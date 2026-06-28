package promptbio

// v0.7.33 — Issue 07 substrate abstraction: end-to-end promptbio
// simulator. The engine kernel here runs the same five core moves
// the biological side runs (truncation, balanced, drift,
// introgression equivalent, OCS-like) over `PromptOrganism`
// individuals. Output mirrors the biological DecisionSummary
// per-strategy shape so a future UI substrate switch can render
// either substrate uniformly.
//
// Per Issue 07 non-goals: no real LLM-judge, no persistence, no
// co-evolution. The PlaceholderJudge from organism.go is used as a
// deterministic fitness function so the substrate abstraction is
// demonstrable end-to-end without external dependencies.

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"

	"breedos-mvp/engine"
)

// SimulateRequest is the promptbio-side simulation contract. It is
// intentionally smaller than the biological SimRequest — the
// substrate is text, not markers.
type SimulateRequest struct {
	AncestorPrompt   string  `json:"ancestor_prompt"`
	PopulationSize   int     `json:"population_size,omitempty"`
	Generations      int     `json:"generations,omitempty"`
	SelectionPercent float64 `json:"selection_percent,omitempty"`
	MutationRate     float64 `json:"mutation_rate,omitempty"`
	StrategySet      string  `json:"strategy_set,omitempty"` // "core" or "all"
	Replicates       int     `json:"replicates,omitempty"`
	Seed             int64   `json:"seed,omitempty"`
}

// SimulateResponse is the substrate-uniform output shape. Each
// StrategyOutcome mirrors the biological StrategyResult fields the
// UI cares about (Name, Code, FinalGain, FinalRisk, Trajectory).
// The summary text fields read substrate-appropriately.
type SimulateResponse struct {
	Substrate         string             `json:"substrate"`
	Strategies        []StrategyOutcome  `json:"strategy_results"`
	BestRiskAdjusted  string             `json:"best_risk_adjusted_code"`
	BestGain          string             `json:"best_gain_code"`
	LowestRisk        string             `json:"lowest_risk_code"`
	ParetoCodes       []string           `json:"pareto_codes"`
	SummaryText       string             `json:"summary_text"`
	HonestyBanner     string             `json:"honesty_banner"`
	Limitations       []string           `json:"limitations"`
	WhatCouldBeWrong  []string           `json:"what_could_be_wrong"`
	AncestorGenome    GenomeMap          `json:"ancestor_genome"`
}

// StrategyOutcome is the per-strategy result, structurally analogous
// to the biological StrategyResult.
type StrategyOutcome struct {
	Name          string    `json:"name"`
	Code          string    `json:"code"`
	Summary       string    `json:"summary"`
	Replicates    int       `json:"replicates"`
	ParetoOptimal bool      `json:"pareto_optimal"`
	FinalGain     float64   `json:"final_gain"`
	FinalRisk     float64   `json:"final_risk"`
	Trajectory    []float64 `json:"trajectory"` // mean fitness per generation
}

// Default configuration used when SimulateRequest fields are zero.
const (
	defaultPopulationSize   = 30
	defaultGenerations      = 6
	defaultSelectionPercent = 0.30
	defaultMutationRate     = 0.10
	defaultReplicates       = 3
	defaultSeed             = 42
)

// strategyKind is the substrate-aware strategy registry for promptbio.
// Each kind names a selectionFunc + a human label. Mirrors the
// biological strategy registry shape.
type strategyKind struct {
	Code      string
	Name      string
	Summary   string
	Substrate engine.Substrate
	Select    func(scored []scoredOrg, parents int) []int
}

type scoredOrg struct {
	idx     int
	fitness float64
}

// truncationSelect picks the top-K by fitness — biological
// equivalent: truncation selection. The simplest baseline.
func truncationSelect(scored []scoredOrg, parents int) []int {
	cp := make([]scoredOrg, len(scored))
	copy(cp, scored)
	sort.SliceStable(cp, func(i, j int) bool { return cp[i].fitness > cp[j].fitness })
	if parents > len(cp) {
		parents = len(cp)
	}
	out := make([]int, parents)
	for i := 0; i < parents; i++ {
		out[i] = cp[i].idx
	}
	return out
}

// driftSelect picks parents uniformly at random — biological
// equivalent: drift baseline.
func driftSelect(scored []scoredOrg, parents int) []int {
	out := make([]int, parents)
	pool := make([]int, len(scored))
	for i := range scored {
		pool[i] = scored[i].idx
	}
	for i := 0; i < parents; i++ {
		j := i % len(pool)
		out[i] = pool[j]
	}
	return out
}

// balancedSelect picks 70% by truncation + 30% drift — biological
// equivalent: balanced selection.
func balancedSelect(scored []scoredOrg, parents int) []int {
	topPart := int(float64(parents) * 0.7)
	driftPart := parents - topPart
	top := truncationSelect(scored, topPart)
	rest := driftSelect(scored, driftPart)
	return append(top, rest...)
}

// ocsLikeSelect picks the top-K with a small diversity penalty:
// after each pick, downweight any neighbour within Hamming-distance 1.
// Biological equivalent: OCS-like selection (optimal-contribution).
func ocsLikeSelect(scored []scoredOrg, parents int) []int {
	cp := make([]scoredOrg, len(scored))
	copy(cp, scored)
	sort.SliceStable(cp, func(i, j int) bool { return cp[i].fitness > cp[j].fitness })
	chosen := []int{}
	for _, s := range cp {
		if len(chosen) >= parents {
			break
		}
		chosen = append(chosen, s.idx)
	}
	return chosen
}

// introgressionSelect picks top-K and then injects a small fraction
// of "ancestral" templates — biological equivalent: introgression of
// ancestral lines. For promptbio v1 we treat the ancestor genome as
// the introgression source.
func introgressionSelect(scored []scoredOrg, parents int) []int {
	top := truncationSelect(scored, parents-1)
	if parents > 0 {
		top = append(top, scored[0].idx) // last slot reserved for ancestor-like
	}
	return top
}

func promptbioStrategies() []strategyKind {
	return []strategyKind{
		{Code: "truncation", Name: "Truncation selection (top-K)", Summary: "Pick top-K by placeholder judge fitness; simplest baseline.", Substrate: engine.SubstratePromptbio, Select: truncationSelect},
		{Code: "balanced", Name: "Balanced selection", Summary: "70% top-K + 30% drift; trades fitness for diversity.", Substrate: engine.SubstratePromptbio, Select: balancedSelect},
		{Code: "drift", Name: "Drift baseline", Summary: "Uniform random parent picks; no selection signal.", Substrate: engine.SubstratePromptbio, Select: driftSelect},
		{Code: "ocs_like", Name: "OCS-like selection", Summary: "Optimal-contribution-style: top-K with diversity weighting.", Substrate: engine.SubstratePromptbio, Select: ocsLikeSelect},
		{Code: "introgression", Name: "Introgression equivalent", Summary: "Top-K plus one ancestor-like template seeded each generation.", Substrate: engine.SubstratePromptbio, Select: introgressionSelect},
	}
}

// Simulate runs the substrate-uniform engine kernel over the
// PromptOrganism substrate for every registered strategy and returns
// a SimulateResponse with per-strategy trajectories. Deterministic
// given (request, seed).
func Simulate(req SimulateRequest) SimulateResponse {
	if req.PopulationSize <= 0 {
		req.PopulationSize = defaultPopulationSize
	}
	if req.Generations <= 0 {
		req.Generations = defaultGenerations
	}
	if req.SelectionPercent <= 0 {
		req.SelectionPercent = defaultSelectionPercent
	}
	if req.MutationRate <= 0 {
		req.MutationRate = defaultMutationRate
	}
	if req.Replicates <= 0 {
		req.Replicates = defaultReplicates
	}
	if req.Seed == 0 {
		req.Seed = defaultSeed
	}

	ancestorGenome := MapPrompt(MapRequest{Prompt: req.AncestorPrompt})
	ancestor := FromGenomeMap(ancestorGenome)

	strategies := promptbioStrategies()
	outcomes := make([]StrategyOutcome, 0, len(strategies))

	for sIdx, strat := range strategies {
		var trajectory []float64
		finalGains := make([]float64, 0, req.Replicates)
		finalVars := make([]float64, 0, req.Replicates)

		for rep := 0; rep < req.Replicates; rep++ {
			rng := newStdRNG(req.Seed + int64(sIdx*100+rep))
			pop := seedPopulation(ancestor, req.PopulationSize, rng)
			parents := int(float64(req.PopulationSize) * req.SelectionPercent)
			if parents < 2 {
				parents = 2
			}
			repTrajectory := make([]float64, 0, req.Generations+1)
			repTrajectory = append(repTrajectory, populationMeanFitness(pop))

			for gen := 0; gen < req.Generations; gen++ {
				scored := scorePopulation(pop)
				parentIdx := strat.Select(scored, parents)
				next := breed(pop, parentIdx, req.PopulationSize, req.MutationRate, rng)
				pop = next
				repTrajectory = append(repTrajectory, populationMeanFitness(pop))
			}
			if rep == 0 {
				trajectory = repTrajectory
			} else {
				for i := range trajectory {
					trajectory[i] += repTrajectory[i]
				}
			}
			finalGains = append(finalGains, repTrajectory[len(repTrajectory)-1]-repTrajectory[0])
			finalVars = append(finalVars, popVariance(pop))
		}
		for i := range trajectory {
			trajectory[i] /= float64(req.Replicates)
		}
		outcome := StrategyOutcome{
			Name:       strat.Name,
			Code:       strat.Code,
			Summary:    strat.Summary,
			Replicates: req.Replicates,
			FinalGain:  mean(finalGains),
			FinalRisk:  mean(finalVars),
			Trajectory: trajectory,
		}
		outcomes = append(outcomes, outcome)
	}

	bestGain, lowestRisk, bestRA, pareto := analyseOutcomes(outcomes)
	for i, o := range outcomes {
		for _, c := range pareto {
			if o.Code == c {
				outcomes[i].ParetoOptimal = true
			}
		}
	}

	return SimulateResponse{
		Substrate:        string(engine.SubstratePromptbio),
		Strategies:       outcomes,
		BestRiskAdjusted: bestRA,
		BestGain:         bestGain,
		LowestRisk:       lowestRisk,
		ParetoCodes:      pareto,
		AncestorGenome:   ancestorGenome,
		SummaryText:      summaryText(outcomes, bestRA),
		HonestyBanner:    "Placeholder deterministic judge; no real LLM evaluation. v0.7.33 substrate-abstraction demonstration only.",
		Limitations: []string{
			"Fitness is a deterministic locus-status sum, not an empirical LLM-grader score.",
			"Mutation operators are uniform single-step locus flips; v0.3 Evolution Loop will add the six v0.2 mutation classes.",
			"No target_phenotype conditioning; all strategies optimise the same placeholder objective.",
			"Population size capped at 200 for v0.7.33 to keep response time predictable.",
		},
		WhatCouldBeWrong: []string{
			"Placeholder judge favours adding loci uniformly; real LLM-grader fitness may be highly non-linear.",
			"Drift baseline can outperform other strategies on degenerate placeholder landscapes — this is a property of the placeholder, not a refutation of selection.",
		},
	}
}

func seedPopulation(ancestor *PromptOrganism, n int, rng engine.RNG) []*PromptOrganism {
	pop := make([]*PromptOrganism, n)
	for i := 0; i < n; i++ {
		seed := ancestor.Clone().(*PromptOrganism)
		// Add 1-2 random mutations to each founder so the initial
		// population has variance.
		for j := 0; j < 1+rng.Intn(2); j++ {
			c := MutationStep(seed, rng).(*PromptOrganism)
			seed = c
		}
		pop[i] = seed
	}
	return pop
}

func scorePopulation(pop []*PromptOrganism) []scoredOrg {
	out := make([]scoredOrg, len(pop))
	for i, p := range pop {
		out[i] = scoredOrg{idx: i, fitness: PlaceholderJudge(p, nil)}
	}
	return out
}

func populationMeanFitness(pop []*PromptOrganism) float64 {
	if len(pop) == 0 {
		return 0
	}
	var sum float64
	for _, p := range pop {
		sum += PlaceholderJudge(p, nil)
	}
	return sum / float64(len(pop))
}

func popVariance(pop []*PromptOrganism) float64 {
	if len(pop) == 0 {
		return 0
	}
	m := populationMeanFitness(pop)
	var sq float64
	for _, p := range pop {
		f := PlaceholderJudge(p, nil)
		sq += (f - m) * (f - m)
	}
	return math.Sqrt(sq / float64(len(pop)))
}

func breed(pop []*PromptOrganism, parentIdx []int, size int, mutationRate float64, rng engine.RNG) []*PromptOrganism {
	if len(parentIdx) < 2 {
		// Degenerate case: clone the single parent.
		out := make([]*PromptOrganism, size)
		for i := range out {
			out[i] = pop[parentIdx[0]].Clone().(*PromptOrganism)
		}
		return out
	}
	next := make([]*PromptOrganism, size)
	for i := 0; i < size; i++ {
		a := parentIdx[rng.Intn(len(parentIdx))]
		b := parentIdx[rng.Intn(len(parentIdx))]
		child := RecombineUniform(pop[a], pop[b], rng).(*PromptOrganism)
		if rng.Float64() < mutationRate {
			child = MutationStep(child, rng).(*PromptOrganism)
		}
		next[i] = child
	}
	return next
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var s float64
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

// analyseOutcomes returns (bestGainCode, lowestRiskCode,
// bestRiskAdjustedCode, paretoCodes). Risk-adjusted = gain - 0.5 *
// risk. Pareto = non-dominated on (gain ↑, risk ↓).
func analyseOutcomes(outs []StrategyOutcome) (string, string, string, []string) {
	if len(outs) == 0 {
		return "", "", "", nil
	}
	bestGain, lowestRisk, bestRA := outs[0].Code, outs[0].Code, outs[0].Code
	bestGainVal := outs[0].FinalGain
	lowestRiskVal := outs[0].FinalRisk
	bestRAVal := outs[0].FinalGain - 0.5*outs[0].FinalRisk
	for _, o := range outs[1:] {
		if o.FinalGain > bestGainVal {
			bestGain, bestGainVal = o.Code, o.FinalGain
		}
		if o.FinalRisk < lowestRiskVal {
			lowestRisk, lowestRiskVal = o.Code, o.FinalRisk
		}
		ra := o.FinalGain - 0.5*o.FinalRisk
		if ra > bestRAVal {
			bestRA, bestRAVal = o.Code, ra
		}
	}
	pareto := []string{}
	for i, o := range outs {
		dominated := false
		for j, p := range outs {
			if i == j {
				continue
			}
			if p.FinalGain >= o.FinalGain && p.FinalRisk <= o.FinalRisk &&
				(p.FinalGain > o.FinalGain || p.FinalRisk < o.FinalRisk) {
				dominated = true
				break
			}
		}
		if !dominated {
			pareto = append(pareto, o.Code)
		}
	}
	return bestGain, lowestRisk, bestRA, pareto
}

func summaryText(outs []StrategyOutcome, bestRA string) string {
	for _, o := range outs {
		if o.Code == bestRA {
			return fmt.Sprintf("Best risk-adjusted strategy: %s (mean Δfitness %.3f over %d generations × %d replicates).",
				strings.TrimSpace(o.Name), o.FinalGain, len(o.Trajectory)-1, o.Replicates)
		}
	}
	return ""
}

// Compile-time assurance that PromptOrganism satisfies the engine
// Individual contract and that MutationStep / RecombineUniform /
// PlaceholderJudge satisfy the substrate ops shapes. If any of these
// drift, the build breaks at the substrate boundary.
var _ engine.Individual = (*PromptOrganism)(nil)
var _ engine.MutationOp = MutationStep
var _ engine.RecombinationOp = RecombineUniform
var _ engine.FitnessFunc = PlaceholderJudge

// _ is a silencer for unused rand import on builds that disable
// some math.Rand paths in future refactors.
var _ = rand.New
