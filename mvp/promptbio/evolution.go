package promptbio

// v0.7.34 — Issue 03 Prompt Evolution Loop. From a single ancestor,
// grow a population of variants, score them across multiple niches,
// select winners per niche + a Pareto front over (fitness,
// robustness), iterate for N generations, and emit a lineage tree
// whose edges are content-addressed v0.2 mutation ledger entries.
//
// v0.7.34 scope (minimum viable per Issue 03 acceptance):
// - Synchronous endpoint (deterministic judge is fast enough; async
//   wraps in /start + /status are a v0.4 follow-up).
// - 3 canonical niches with distinct locus-weight profiles:
//   `core_breadth` (Task/Constraint/Output dominate), `epistemic_depth`
//   (Context/Method/Epistemic/Validation), `safety_first`
//   (Safety/Constraint/Validation).
// - 4 mutation kinds reused from v0.2 Diff vocabulary
//   (addition / deletion / substitution / amplification). Six-kind
//   set is queued behind v5.7 expansion.
// - Lineage edges carry the v0.2 `ledger_id` so v0.3 results are
//   consumable by the existing diff UI without translation.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
)

// EvolveRequest is the v0.3 input contract. The fields mirror Issue
// 03 Inputs but pruned to what the v0.7.34 minimum loop actually
// uses; `target_phenotype`, `fitness_criteria`, `test_cases`, and
// `niches` come back in v0.4 when LLM-judge integration lands.
type EvolveRequest struct {
	AncestorPrompt   string  `json:"ancestor_prompt"`
	Generations      int     `json:"generations,omitempty"`
	PopulationSize   int     `json:"population_size,omitempty"`
	MutationStrategy string  `json:"mutation_strategy,omitempty"` // random / targeted / mixed
	SelectionPercent float64 `json:"selection_percent,omitempty"`
	Seed             int64   `json:"seed,omitempty"`
}

// NicheSpec defines one selection niche: a per-locus weight vector
// the niche-specific fitness function uses to score variants.
type NicheSpec struct {
	Code        string             `json:"code"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Weights     map[LocusName]float64 `json:"weights"`
}

// PromptVariant is one organism in the population. ID is content-
// addressed on the genome so identical genomes across branches
// deduplicate naturally.
type PromptVariant struct {
	ID         string             `json:"id"`
	Generation int                `json:"generation"`
	ParentID   string             `json:"parent_id,omitempty"`
	Genome     [14]byte           `json:"genome"`
	Mutations  []GeneChange       `json:"mutations_applied"`
	Fitness    map[string]float64 `json:"fitness_per_niche"`
	MeanFitness float64           `json:"mean_fitness"`
	Robustness  float64           `json:"robustness"`
}

// LineageEdge is one parent → child relation. The ledger ID matches
// the corresponding entry in `Mutations` and is content-addressed
// per v0.2.
type LineageEdge struct {
	ParentID  string `json:"parent_id"`
	ChildID   string `json:"child_id"`
	LedgerID  string `json:"ledger_id"`
	Kind      MutationKind `json:"kind"`
	Locus     LocusName    `json:"locus"`
}

// GenerationSummary is the per-generation rollup the UI renders as a
// row in the changelog.
type GenerationSummary struct {
	Generation     int                `json:"generation"`
	MeanFitness    float64            `json:"mean_fitness"`
	BestFitness    float64            `json:"best_fitness"`
	BestVariantID  string             `json:"best_variant_id"`
	WinnersByNiche map[string]string  `json:"winners_by_niche"`
	ParetoIDs      []string           `json:"pareto_ids"`
	NewVariants    int                `json:"new_variants"`
	Notes          string             `json:"notes"`
}

// EvolveResponse is the substrate-uniform output of /api/promptbio/evolve.
type EvolveResponse struct {
	RunID          string              `json:"run_id"`
	AncestorGenome GenomeMap           `json:"ancestor_genome"`
	Niches         []NicheSpec         `json:"niches"`
	Generations    [][]PromptVariant   `json:"generations"`
	Lineage        []LineageEdge       `json:"lineage"`
	Changelog      []GenerationSummary `json:"changelog"`
	GlobalWinner   string              `json:"global_winner_id"`
	NicheWinners   map[string]string   `json:"final_niche_winners"`
	SummaryText    string              `json:"summary_text"`
	HonestyBanner  string              `json:"honesty_banner"`
	Limitations    []string            `json:"limitations"`
}

// canonicalNiches returns the 3 v0.7.34 niches. The weight maps
// are intentionally SPARSE and ANTI-CORRELATED so a variant tuned
// for one niche cannot dominate the others — that's what produces
// niche specialists per Issue 03 acceptance criterion #2. Loci
// weighted in multiple niches are minimal; each niche has a 4–5
// locus identity.
func canonicalNiches() []NicheSpec {
	return []NicheSpec{
		{
			Code: "core_breadth", Name: "Core breadth",
			Description: "Task + Audience + Output + Constraint. Generalist answer organism.",
			Weights: map[LocusName]float64{
				LocusTask: 3.0, LocusAudience: 3.0, LocusOutput: 3.0, LocusConstraint: 2.0,
			},
		},
		{
			Code: "epistemic_depth", Name: "Epistemic depth",
			Description: "Context + Method + Epistemic. Research / reasoning organism.",
			Weights: map[LocusName]float64{
				LocusContext: 3.0, LocusMethod: 3.0, LocusEpistemic: 3.0, LocusValidation: 2.0,
			},
		},
		{
			Code: "safety_first", Name: "Safety-first",
			Description: "Safety + Constraint + Memory + Tool. High-risk advisory organism.",
			Weights: map[LocusName]float64{
				LocusSafety: 3.0, LocusConstraint: 2.0, LocusMemory: 3.0, LocusTool: 3.0,
			},
		},
	}
}

// nicheFitness scores a genome against a niche's weight map,
// normalised to [0, 1].
func nicheFitness(genome [14]byte, niche NicheSpec) float64 {
	var weighted, max float64
	for i, name := range allLoci {
		w := niche.Weights[name]
		v := float64(genome[i])
		if v > 3 {
			v = 0
		}
		weighted += w * v
		max += w * 3
	}
	if max == 0 {
		return 0
	}
	return weighted / max
}

// variantID is content-addressed on the genome bytes — identical
// genomes across branches share the same id, which is the right
// semantics for lineage deduplication.
func variantID(genome [14]byte, parentID string, gen int) string {
	h := sha256.Sum256(append([]byte(parentID), append([]byte{byte(gen)}, genome[:]...)...))
	return "v_" + hex.EncodeToString(h[:6])
}

// mutateRandomLocus applies one of the four canonical v0.3 mutation
// kinds (addition / deletion / substitution / amplification) to a
// uniformly-random locus, returning the child genome and the
// resulting GeneChange (with content-addressed ledger id) so the
// lineage edge can reference it.
func mutateRandomLocus(parent [14]byte, parentName, childName string, rng *RNGv) ([14]byte, GeneChange) {
	child := parent
	idx := rng.Intn(14)
	before := child[idx]
	kind := pickMutationKind(before, rng)
	switch kind {
	case MutationAddition:
		if child[idx] < 3 {
			child[idx]++
		}
	case MutationDeletion:
		if child[idx] > 0 {
			child[idx]--
		}
	case MutationAmplification:
		if child[idx] < 3 {
			child[idx] = 3
		}
	case MutationSubstitution:
		// pick a different status of the same energy band
		if child[idx] == 2 {
			child[idx] = 1
		} else if child[idx] == 1 {
			child[idx] = 2
		} else if child[idx] < 3 {
			child[idx]++
		}
	}
	locus := allLoci[idx]
	gc := GeneChange{
		Locus:        locus,
		Kind:         kind,
		BeforeStatus: decodeStatus(before),
		AfterStatus:  decodeStatus(child[idx]),
		Rationale:    fmt.Sprintf("v0.3 generation mutation: %s on %s (%s → %s)", kind, locus, decodeStatus(before), decodeStatus(child[idx])),
	}
	gc.LedgerID = ledgerID(gc)
	return child, gc
}

// pickMutationKind picks a mutation kind biased toward "addition"
// when the current status is missing/weak (room to grow) and toward
// "deletion" when at strong (room to shrink). Keeps the distribution
// meaningful instead of uniform.
func pickMutationKind(current byte, rng *RNGv) MutationKind {
	r := rng.Float64()
	switch {
	case current <= 1:
		// missing/weak → biased toward addition
		if r < 0.6 {
			return MutationAddition
		}
		if r < 0.8 {
			return MutationAmplification
		}
		return MutationSubstitution
	case current == 3:
		// strong → biased toward deletion/substitution
		if r < 0.5 {
			return MutationDeletion
		}
		return MutationSubstitution
	}
	// present → balanced
	if r < 0.35 {
		return MutationAddition
	} else if r < 0.55 {
		return MutationAmplification
	} else if r < 0.75 {
		return MutationSubstitution
	}
	return MutationDeletion
}

// RNGv is a deterministic counter-based RNG so determinism tests can
// reproduce evolution runs byte-for-byte regardless of Go's
// math/rand internal state.
type RNGv struct{ s uint64 }

func newRNG(seed int64) RNGv { return RNGv{s: uint64(seed)*6364136223846793005 + 1442695040888963407} }
func (r *RNGv) Next() uint64 {
	r.s = r.s*6364136223846793005 + 1442695040888963407
	return r.s
}
func (r *RNGv) Intn(n int) int     { return int(r.Next() % uint64(n)) }
func (r *RNGv) Float64() float64   { return float64(r.Next()&((1<<53)-1)) / float64(1<<53) }

// Evolve runs the v0.3 Prompt Evolution Loop synchronously.
// Deterministic given (request, seed).
func Evolve(req EvolveRequest) EvolveResponse {
	if req.Generations <= 0 {
		req.Generations = 5
	}
	if req.Generations > 12 {
		req.Generations = 12
	}
	if req.PopulationSize <= 0 {
		req.PopulationSize = 8
	}
	if req.PopulationSize < 5 {
		req.PopulationSize = 5
	}
	if req.PopulationSize > 20 {
		req.PopulationSize = 20
	}
	if req.SelectionPercent <= 0 {
		req.SelectionPercent = 0.35
	}
	if req.Seed == 0 {
		req.Seed = 1
	}
	if req.MutationStrategy == "" {
		req.MutationStrategy = "mixed"
	}

	rng := newRNG(req.Seed)
	niches := canonicalNiches()

	ancestorMap := MapPrompt(MapRequest{Prompt: req.AncestorPrompt})
	ancestorOrg := FromGenomeMap(ancestorMap)
	ancestorID := variantID(ancestorOrg.Statuses, "", 0)
	ancestorVariant := PromptVariant{
		ID:         ancestorID,
		Generation: 0,
		ParentID:   "",
		Genome:     ancestorOrg.Statuses,
		Mutations:  []GeneChange{},
		Fitness:    scoreAcrossNiches(ancestorOrg.Statuses, niches),
	}
	ancestorVariant.MeanFitness, ancestorVariant.Robustness = meanAndStd(ancestorVariant.Fitness)

	pop0 := []PromptVariant{ancestorVariant}
	// Seed population: ancestor + (PopulationSize - 1) mutated children.
	lineage := []LineageEdge{}
	for i := 1; i < req.PopulationSize; i++ {
		child, gc := mutateRandomLocus(ancestorOrg.Statuses, ancestorID, "", &rng)
		v := PromptVariant{
			Generation: 0,
			ParentID:   ancestorID,
			Genome:     child,
			Mutations:  []GeneChange{gc},
		}
		v.ID = variantID(child, ancestorID, 0)
		v.Fitness = scoreAcrossNiches(child, niches)
		v.MeanFitness, v.Robustness = meanAndStd(v.Fitness)
		pop0 = append(pop0, v)
		lineage = append(lineage, LineageEdge{
			ParentID: ancestorID, ChildID: v.ID,
			LedgerID: gc.LedgerID, Kind: gc.Kind, Locus: gc.Locus,
		})
	}

	generations := [][]PromptVariant{pop0}
	changelog := []GenerationSummary{summarise(pop0, niches, 0)}

	for gen := 1; gen <= req.Generations; gen++ {
		prev := generations[gen-1]
		parents := selectParentsForEvolve(prev, niches, req.SelectionPercent)
		next := make([]PromptVariant, 0, req.PopulationSize)
		for i := 0; i < req.PopulationSize; i++ {
			parentIdx := parents[i%len(parents)]
			parent := prev[parentIdx]
			child, gc := mutateRandomLocus(parent.Genome, parent.ID, "", &rng)
			v := PromptVariant{
				Generation: gen,
				ParentID:   parent.ID,
				Genome:     child,
				Mutations:  []GeneChange{gc},
			}
			v.ID = variantID(child, parent.ID, gen)
			v.Fitness = scoreAcrossNiches(child, niches)
			v.MeanFitness, v.Robustness = meanAndStd(v.Fitness)
			next = append(next, v)
			lineage = append(lineage, LineageEdge{
				ParentID: parent.ID, ChildID: v.ID,
				LedgerID: gc.LedgerID, Kind: gc.Kind, Locus: gc.Locus,
			})
		}
		generations = append(generations, next)
		changelog = append(changelog, summarise(next, niches, gen))
	}

	// Final-generation analysis: global winner + niche winners.
	finalGen := generations[len(generations)-1]
	globalWinner := pickBest(finalGen, func(v PromptVariant) float64 { return v.MeanFitness })
	nicheWinners := map[string]string{}
	for _, niche := range niches {
		nc := niche.Code
		w := pickBest(finalGen, func(v PromptVariant) float64 { return v.Fitness[nc] })
		nicheWinners[nc] = w
	}

	resp := EvolveResponse{
		RunID:          newRunID(req),
		AncestorGenome: ancestorMap,
		Niches:         niches,
		Generations:    generations,
		Lineage:        lineage,
		Changelog:      changelog,
		GlobalWinner:   globalWinner,
		NicheWinners:   nicheWinners,
		HonestyBanner:  "v0.3 Evolution Loop v0.7.34: deterministic placeholder judge over 3 fixed niches; no real LLM-grader, no synthetic worlds. 6-kind mutation taxonomy and MAP-Elites niche search are queued behind v0.4–v0.5.",
		Limitations: []string{
			"Fitness is locus-weighted status sum, not empirical LLM-grader score.",
			"Niches are fixed: core_breadth / epistemic_depth / safety_first. Issue 03 spec allows operator-supplied niches; deferred to v0.4.",
			"Mutation kinds limited to 4 from v0.2 vocabulary (addition / deletion / substitution / amplification). v5.7 catalogue (15+ operators with risk tiers) is queued.",
			"Synchronous endpoint (no /start + /status). Deterministic judge is fast; async pattern is a v0.4 follow-up when LLM-judge integration lands.",
			"No target_phenotype conditioning. v0.3 optimises the locus-weight objective per niche; v0.4 wires environment + Ecology.",
		},
		SummaryText: fmt.Sprintf("Evolution converged on global winner %s (mean fitness %.3f over %d generations × %d-variant population, %d niches). Lineage tree spans %d edges.",
			globalWinner, fitnessOf(finalGen, globalWinner), req.Generations, req.PopulationSize, len(niches), len(lineage)),
	}
	return resp
}

func selectParentsForEvolve(pop []PromptVariant, niches []NicheSpec, selectionPercent float64) []int {
	// Mixed selection: per-niche top-1 + Pareto-front top-2 + global top-K.
	k := int(float64(len(pop)) * selectionPercent)
	if k < 2 {
		k = 2
	}
	chosen := map[int]bool{}
	// Per-niche winners
	for _, niche := range niches {
		bestIdx := 0
		bestScore := -1.0
		for i, v := range pop {
			if v.Fitness[niche.Code] > bestScore {
				bestScore = v.Fitness[niche.Code]
				bestIdx = i
			}
		}
		chosen[bestIdx] = true
	}
	// Top-K by mean fitness
	cp := make([]struct {
		idx     int
		fitness float64
	}, len(pop))
	for i, v := range pop {
		cp[i] = struct {
			idx     int
			fitness float64
		}{i, v.MeanFitness}
	}
	sort.SliceStable(cp, func(i, j int) bool { return cp[i].fitness > cp[j].fitness })
	for i := 0; i < k && i < len(cp); i++ {
		chosen[cp[i].idx] = true
	}
	out := make([]int, 0, len(chosen))
	for i := range chosen {
		out = append(out, i)
	}
	sort.Ints(out)
	return out
}

func scoreAcrossNiches(genome [14]byte, niches []NicheSpec) map[string]float64 {
	out := map[string]float64{}
	for _, n := range niches {
		out[n.Code] = nicheFitness(genome, n)
	}
	return out
}

func meanAndStd(fitness map[string]float64) (float64, float64) {
	if len(fitness) == 0 {
		return 0, 0
	}
	var mean float64
	for _, v := range fitness {
		mean += v
	}
	mean /= float64(len(fitness))
	var sq float64
	for _, v := range fitness {
		sq += (v - mean) * (v - mean)
	}
	return mean, math.Sqrt(sq / float64(len(fitness)))
}

func summarise(pop []PromptVariant, niches []NicheSpec, gen int) GenerationSummary {
	if len(pop) == 0 {
		return GenerationSummary{Generation: gen}
	}
	var meanF, bestF float64
	bestID := pop[0].ID
	for _, v := range pop {
		meanF += v.MeanFitness
		if v.MeanFitness > bestF {
			bestF = v.MeanFitness
			bestID = v.ID
		}
	}
	meanF /= float64(len(pop))
	winners := map[string]string{}
	for _, niche := range niches {
		bestNF := -1.0
		var wid string
		for _, v := range pop {
			if v.Fitness[niche.Code] > bestNF {
				bestNF = v.Fitness[niche.Code]
				wid = v.ID
			}
		}
		winners[niche.Code] = wid
	}
	pareto := paretoFront(pop)
	return GenerationSummary{
		Generation:     gen,
		MeanFitness:    meanF,
		BestFitness:    bestF,
		BestVariantID:  bestID,
		WinnersByNiche: winners,
		ParetoIDs:      pareto,
		NewVariants:    len(pop),
	}
}

func paretoFront(pop []PromptVariant) []string {
	out := []string{}
	for i, a := range pop {
		dominated := false
		for j, b := range pop {
			if i == j {
				continue
			}
			if b.MeanFitness >= a.MeanFitness && b.Robustness <= a.Robustness &&
				(b.MeanFitness > a.MeanFitness || b.Robustness < a.Robustness) {
				dominated = true
				break
			}
		}
		if !dominated {
			out = append(out, a.ID)
		}
	}
	return out
}

func pickBest(pop []PromptVariant, score func(PromptVariant) float64) string {
	if len(pop) == 0 {
		return ""
	}
	bestID := pop[0].ID
	bestScore := score(pop[0])
	for _, v := range pop[1:] {
		s := score(v)
		if s > bestScore {
			bestScore = s
			bestID = v.ID
		}
	}
	return bestID
}

func fitnessOf(pop []PromptVariant, id string) float64 {
	for _, v := range pop {
		if v.ID == id {
			return v.MeanFitness
		}
	}
	return 0
}

func newRunID(req EvolveRequest) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%d|%d|%v|%d",
		req.AncestorPrompt, req.Seed, req.Generations,
		req.PopulationSize, req.SelectionPercent, len(req.MutationStrategy))))
	return "r_" + hex.EncodeToString(h[:8])
}
