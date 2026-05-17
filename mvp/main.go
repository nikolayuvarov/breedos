package main

import (
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"math"
	"math/rand"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

//go:embed static/*
var embeddedStatic embed.FS

type SimRequest struct {
	Seed               int64   `json:"seed"`
	PopulationSize     int     `json:"population_size"`
	Markers            int     `json:"markers"`
	QTLCount           int     `json:"qtl_count"`
	Generations        int     `json:"generations"`
	SelectionPercent   float64 `json:"selection_percent"`
	Heritability       float64 `json:"heritability"`
	MutationRate       float64 `json:"mutation_rate"`
	CrisprEnabled      bool    `json:"crispr_enabled"`
	CrisprEdits        int     `json:"crispr_edits"`
	CrisprIntroPercent float64 `json:"crispr_intro_percent"`
	StrategySet        string  `json:"strategy_set"`
	Replicates         int     `json:"replicates"`
	WorkerCount        int     `json:"worker_count"`
	InbreedingLimit    float64 `json:"inbreeding_limit"`
	DiversityLossLimit float64 `json:"diversity_loss_limit"`
}

type SimResponse struct {
	Request        SimRequest       `json:"request"`
	Decision       DecisionSummary  `json:"decision"`
	Strategies     []StrategyResult `json:"strategies"`
	CandidateEdits []EditCandidate  `json:"candidate_edits"`
	Notes          []string         `json:"notes"`
}

type DecisionSummary struct {
	BestRiskAdjustedCode string   `json:"best_risk_adjusted_code"`
	BestRiskAdjustedName string   `json:"best_risk_adjusted_name"`
	BestGainCode         string   `json:"best_gain_code"`
	LowestRiskCode       string   `json:"lowest_risk_code"`
	ParetoCodes          []string `json:"pareto_codes"`
	Interpretation       []string `json:"interpretation"`
}

type StrategyResult struct {
	Name          string        `json:"name"`
	Code          string        `json:"code"`
	Summary       string        `json:"summary"`
	Replicates    int           `json:"replicates"`
	ParetoOptimal bool          `json:"pareto_optimal"`
	Metrics       []MetricPoint `json:"metrics"`
	Final         FinalStats    `json:"final"`
}

type MetricPoint struct {
	Generation       int     `json:"generation"`
	TraitMean        float64 `json:"trait_mean"`
	GeneticGain      float64 `json:"genetic_gain"`
	Diversity        float64 `json:"diversity"`
	Inbreeding       float64 `json:"inbreeding"`
	AlleleDrift      float64 `json:"allele_drift"`
	RareUsefulLost   int     `json:"rare_useful_lost"`
	FixedLoci        int     `json:"fixed_loci"`
	FavorableFixed   int     `json:"favorable_fixed"`
	UnfavorableFixed int     `json:"unfavorable_fixed"`
	EffectiveParents int     `json:"effective_parents"`
}

type FinalStats struct {
	TraitMean                    float64 `json:"trait_mean"`
	GeneticGain                  float64 `json:"genetic_gain"`
	Diversity                    float64 `json:"diversity"`
	Inbreeding                   float64 `json:"inbreeding"`
	AlleleDrift                  float64 `json:"allele_drift"`
	RareUsefulLost               int     `json:"rare_useful_lost"`
	FixedLoci                    int     `json:"fixed_loci"`
	FavorableFixed               int     `json:"favorable_fixed"`
	UnfavorableFixed             int     `json:"unfavorable_fixed"`
	EffectiveParents             int     `json:"effective_parents"`
	Replicates                   int     `json:"replicates"`
	GainStd                      float64 `json:"gain_std"`
	DiversityStd                 float64 `json:"diversity_std"`
	InbreedingStd                float64 `json:"inbreeding_std"`
	ProbabilityInbreedingBreach  float64 `json:"probability_inbreeding_breach"`
	ProbabilityDiversityCollapse float64 `json:"probability_diversity_collapse"`
	ProbabilityRareUsefulLoss    float64 `json:"probability_rare_useful_loss"`
	RiskAdjustedScore            float64 `json:"risk_adjusted_score"`
	DecisionRank                 int     `json:"decision_rank"`
	ParetoOptimal                bool    `json:"pareto_optimal"`
	RecommendedNext              string  `json:"recommended_next"`
}

type EditCandidate struct {
	Rank              int     `json:"rank"`
	Locus             int     `json:"locus"`
	Effect            float64 `json:"effect"`
	AlleleFrequency   float64 `json:"allele_frequency"`
	ExpectedGainScore float64 `json:"expected_gain_score"`
	DiversityRisk     string  `json:"diversity_risk"`
	Decision          string  `json:"decision"`
}

type organism struct{ geno []uint8 }

type scoredIndividual struct {
	idx     int
	trait   float64
	genomic float64
	novelty float64
	score   float64
}

type strategyConfig struct {
	Name              string
	Code              string
	Summary           string
	Rule              string
	MatingRule        string
	TraitWeight       float64
	NoveltyWeight     float64
	SimilarityPenalty float64
	ParentMultiplier  float64
	UseCrisprSeed     bool
	ConservativeEdits bool
}

type progressFunc func(percent int, message string)

type SimJobStartResponse struct {
	JobID string `json:"job_id"`
}

type SimJobStatus struct {
	JobID   string       `json:"job_id"`
	Percent int          `json:"percent"`
	Message string       `json:"message"`
	Done    bool         `json:"done"`
	Error   string       `json:"error,omitempty"`
	Result  *SimResponse `json:"result,omitempty"`
}

type simJob struct {
	ID        string
	CreatedAt time.Time
	UpdatedAt time.Time
	Percent   int
	Message   string
	Done      bool
	Error     string
	Result    *SimResponse
}

var simJobStore = struct {
	sync.Mutex
	NextID uint64
	Jobs   map[string]*simJob
}{Jobs: make(map[string]*simJob)}

func main() {
	listen := flag.String("listen", ":8080", "HTTP listen address, for example :8080 or 127.0.0.1:8080")
	flag.Parse()
	staticFS, err := fs.Sub(embeddedStatic, "static")
	if err != nil {
		log.Fatalf("static fs error: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/simulate", simulateHandler)
	mux.HandleFunc("/api/simulate/start", startSimulationJobHandler)
	mux.HandleFunc("/api/simulate/status", simulationJobStatusHandler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if path == "demo" {
			path = "demo.html"
		}
		if path == "customer-discovery" {
			path = "customer_discovery.html"
		}
		if strings.Contains(path, "..") {
			http.Error(w, "bad path", http.StatusBadRequest)
			return
		}
		data, err := fs.ReadFile(staticFS, path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		setContentType(w, path)
		_, _ = w.Write(data)
	})
	server := &http.Server{Addr: *listen, Handler: loggingMiddleware(mux), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("BreedOS MVP server listening on http://%s", normalizeListenForLog(*listen))
	log.Fatal(server.ListenAndServe())
}

func simulateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()
	var req SimRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}
	resp, err := runSimulation(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(resp); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func startSimulationJobHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()
	var req SimRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}
	normalizeRequest(&req)
	strategies := buildStrategyConfigs(req)
	if err := validateRequest(req, len(strategies)); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	id := createSimulationJob()
	go func(jobID string, jobReq SimRequest) {
		resp, err := runSimulationWithProgress(jobReq, func(percent int, message string) { updateSimulationJob(jobID, percent, message, nil, "", false) })
		if err != nil {
			updateSimulationJob(jobID, 100, "failed", nil, err.Error(), true)
			return
		}
		updateSimulationJob(jobID, 100, "complete", &resp, "", true)
	}(id, req)
	writeJSON(w, SimJobStartResponse{JobID: id})
}

func simulationJobStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "use GET", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	status, ok := getSimulationJobStatus(id)
	if !ok {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}
	writeJSON(w, status)
}

func createSimulationJob() string {
	now := time.Now()
	simJobStore.Lock()
	defer simJobStore.Unlock()
	cleanupSimulationJobsLocked(now)
	simJobStore.NextID++
	id := fmt.Sprintf("job-%d-%d", now.UnixNano(), simJobStore.NextID)
	simJobStore.Jobs[id] = &simJob{ID: id, CreatedAt: now, UpdatedAt: now, Percent: 0, Message: "queued"}
	return id
}

func updateSimulationJob(id string, percent int, message string, result *SimResponse, errText string, done bool) {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	now := time.Now()
	simJobStore.Lock()
	defer simJobStore.Unlock()
	job := simJobStore.Jobs[id]
	if job == nil {
		return
	}
	if percent < job.Percent && !done {
		percent = job.Percent
	}
	job.Percent = percent
	if message != "" {
		job.Message = message
	}
	job.Result = result
	job.Error = errText
	job.Done = done
	job.UpdatedAt = now
}

func getSimulationJobStatus(id string) (SimJobStatus, bool) {
	now := time.Now()
	simJobStore.Lock()
	defer simJobStore.Unlock()
	cleanupSimulationJobsLocked(now)
	job := simJobStore.Jobs[id]
	if job == nil {
		return SimJobStatus{}, false
	}
	return SimJobStatus{JobID: job.ID, Percent: job.Percent, Message: job.Message, Done: job.Done, Error: job.Error, Result: job.Result}, true
}

func cleanupSimulationJobsLocked(now time.Time) {
	for id, job := range simJobStore.Jobs {
		if now.Sub(job.UpdatedAt) > 30*time.Minute {
			delete(simJobStore.Jobs, id)
		}
	}
}
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func runSimulation(req SimRequest) (SimResponse, error) { return runSimulationWithProgress(req, nil) }

func runSimulationWithProgress(req SimRequest, progress progressFunc) (SimResponse, error) {
	reportProgress(progress, 1, "normalizing request")
	normalizeRequest(&req)
	strategies := buildStrategyConfigs(req)
	if err := validateRequest(req, len(strategies)); err != nil {
		return SimResponse{}, err
	}
	rng := rand.New(rand.NewSource(req.Seed))
	effects := makeEffects(req.Markers, req.QTLCount, rng)
	initial := makeInitialPopulation(req.PopulationSize, req.Markers, rng)
	baseFreq := alleleFreq(initial, req.Markers)
	baseDiversity := diversityFromFreq(baseFreq)
	baseMean := meanGeneticValue(initial, effects)
	rareUsefulAtStart := rareUsefulLoci(baseFreq, effects)
	candidates := rankEditCandidates(baseFreq, effects, req.CrisprEdits)
	reportProgress(progress, 5, "initial population, candidate edits, and strategy set ready")
	results := simulateStrategiesDecisionEngine(req, strategies, initial, effects, baseFreq, baseDiversity, baseMean, rareUsefulAtStart, candidates, progress)
	annotateDecisionScores(results)
	decision := buildDecisionSummary(results)
	reportProgress(progress, 98, "building response")
	return SimResponse{Request: req, Decision: decision, Strategies: results, CandidateEdits: candidates, Notes: buildNotes(req, len(strategies), baseDiversity)}, nil
}

func buildNotes(req SimRequest, strategyCount int, baseDiversity float64) []string {
	workers := effectiveWorkerCount(req.WorkerCount, strategyCount*req.Replicates)
	notes := []string{
		"This MVP is a decision-layer simulator, not a wet-lab protocol and not a CRISPR guide/off-target design tool.",
		"BreedOS v0.6.3 now runs Monte Carlo replicates, computes risk probabilities, ranks strategies, and marks Pareto-optimal trade-offs.",
		"The CRISPR part is intentionally minimal: it shows how candidate edits can be prioritized and injected into strategy simulation without providing laboratory instructions.",
		fmt.Sprintf("The engine runs %d strategies × %d replicates = %d simulation jobs through a worker pool of %d workers.", strategyCount, req.Replicates, strategyCount*req.Replicates, workers),
		fmt.Sprintf("Risk thresholds: inbreeding breach ≥ %.2f; diversity collapse means diversity loss ≥ %.2f relative to baseline diversity %.4f.", req.InbreedingLimit, req.DiversityLossLimit, baseDiversity),
	}
	if req.StrategySet == "advanced" {
		notes = append(notes, "Advanced strategy set enabled: includes neutral/random baselines, phenotype/genomic selection mockups, OCS-like diversity constraint, cross planning, and edit-aware introgression.")
	} else {
		notes = append(notes, "Core strategy set enabled: includes a neutral drift baseline plus aggressive, diversity-preserving, balanced, and CRISPR-aware balanced strategies when CRISPR is enabled.")
	}
	if req.PopulationSize < 50 {
		notes = append(notes, "Small-population mode is enabled: N < 50 is intentionally allowed to expose stochastic drift, rapid fixation, and founder effects.")
	}
	if req.PopulationSize < 10 {
		notes = append(notes, "N < 10 is a stress-test regime, not a realistic breeding program. It is useful for showing why population size and diversity constraints matter.")
	}
	baseParents := int(math.Round(float64(req.PopulationSize) * req.SelectionPercent / 100.0))
	if baseParents < 2 {
		notes = append(notes, "Selection intensity would yield fewer than two parents; the simulator floors the selected parent pool to two when possible.")
	}
	expectedMutations := float64(req.PopulationSize) * float64(req.Markers) * 2 * float64(req.Generations) * req.MutationRate
	if req.MutationRate == 0 {
		notes = append(notes, "Mutation rate is 0: no de novo mutations are injected; changes come only from segregation, drift, and selection.")
	} else {
		notes = append(notes, fmt.Sprintf("Mutation rate %.8g means roughly %.1f expected allele flips per strategy replicate across the whole run before selection effects.", req.MutationRate, expectedMutations))
		if expectedMutations < 1 {
			notes = append(notes, "Mutation effect may be visually invisible: expected mutation count is below one per strategy replicate. Increase generations, markers, population size, or mutation rate to see it.")
		}
	}
	if simulationBudget(req, strategyCount) > 300000000 {
		notes = append(notes, "Large simulation: v0.6.3 uses a budget guard and worker pool. Production BreedOS should move heavy runs to durable queued workers.")
	}
	return notes
}
func reportProgress(progress progressFunc, percent int, message string) {
	if progress != nil {
		progress(percent, message)
	}
}

func buildStrategyConfigs(req SimRequest) []strategyConfig {
	strategies := []strategyConfig{
		{Name: "Neutral drift baseline", Code: "neutral", Summary: "No intentional selection. This baseline isolates drift, fixation, and mutation effects.", Rule: "no_selection", MatingRule: "random", ParentMultiplier: 1.0},
		{Name: "Aggressive selection", Code: "aggressive", Summary: "Maximizes short-term trait gain. Useful as a warning baseline: fast progress can consume diversity and future optionality.", Rule: "phenotype", MatingRule: "random", TraitWeight: 1.0, ParentMultiplier: 0.75},
		{Name: "Diversity-preserving selection", Code: "diversity", Summary: "Keeps more genetically unusual candidates in the parent pool. Slower gain, lower bottleneck risk.", Rule: "score", MatingRule: "random", TraitWeight: 0.55, NoveltyWeight: 1.25, ParentMultiplier: 2.25},
		{Name: "Balanced strategy", Code: "balanced", Summary: "Trades off near-term gain and long-term genetic optionality. This is the default BreedOS posture.", Rule: "score", MatingRule: "random", TraitWeight: 0.85, NoveltyWeight: 0.50, ParentMultiplier: 1.60},
	}
	if req.CrisprEnabled && req.CrisprEdits > 0 {
		strategies = append(strategies, strategyConfig{Name: "Balanced + CRISPR seed", Code: "balanced_crispr", Summary: "Demonstrates edit-aware integration: prioritize beneficial loci, seed them into part of the founding population, then run balanced selection.", Rule: "score", MatingRule: "random", TraitWeight: 0.85, NoveltyWeight: 0.50, ParentMultiplier: 1.60, UseCrisprSeed: true})
	}
	if req.StrategySet != "advanced" {
		return strategies
	}
	strategies = append(strategies,
		strategyConfig{Name: "Random parent baseline", Code: "random", Summary: "Randomly selects the configured parent fraction. Useful to separate selection value from drift noise.", Rule: "random", MatingRule: "random", ParentMultiplier: 1.0},
		strategyConfig{Name: "Phenotype truncation selection", Code: "phenotype", Summary: "Classic truncation selection on noisy phenotype. Strong baseline for practical breeding programs.", Rule: "phenotype", MatingRule: "random", TraitWeight: 1.0, ParentMultiplier: 1.0},
		strategyConfig{Name: "Genomic selection mock", Code: "genomic", Summary: "Selects by a simplified predicted breeding value. In production this is where GBLUP/Bayesian/ML models plug in.", Rule: "genomic", MatingRule: "random", TraitWeight: 1.0, ParentMultiplier: 1.0},
		strategyConfig{Name: "OCS-like constrained selection", Code: "ocs_like", Summary: "Approximates optimum contribution selection: gain is pursued under a similarity/diversity penalty.", Rule: "ocs", MatingRule: "random", TraitWeight: 0.90, NoveltyWeight: 0.25, SimilarityPenalty: 0.85, ParentMultiplier: 1.80},
		strategyConfig{Name: "Cross planner", Code: "cross_planner", Summary: "Not only who to keep, but who to cross. Uses balanced parent choice plus more distant mating pairs.", Rule: "score", MatingRule: "diverse_pairs", TraitWeight: 0.82, NoveltyWeight: 0.60, ParentMultiplier: 1.80},
	)
	if req.CrisprEnabled && req.CrisprEdits > 0 {
		strategies = append(strategies, strategyConfig{Name: "Edit-aware introgression planner", Code: "edit_introgression", Summary: "Seeds lower-risk candidate edits, then spreads them through diversity-aware parent choice and diverse mate allocation.", Rule: "ocs", MatingRule: "diverse_pairs", TraitWeight: 0.88, NoveltyWeight: 0.35, SimilarityPenalty: 0.75, ParentMultiplier: 1.90, UseCrisprSeed: true, ConservativeEdits: true})
	}
	return strategies
}

type strategyTask struct {
	StrategyIndex int
	Replicate     int
	Config        strategyConfig
}
type strategyTaskResult struct {
	StrategyIndex int
	Replicate     int
	Config        strategyConfig
	Result        StrategyResult
}

func simulateStrategiesDecisionEngine(req SimRequest, strategies []strategyConfig, initial []organism, effects []float64, baseFreq []float64, baseDiversity, baseMean float64, rareUsefulAtStart []int, candidates []EditCandidate, progress progressFunc) []StrategyResult {
	strategyCount := len(strategies)
	jobCount := strategyCount * req.Replicates
	resultsByStrategy := make([][]StrategyResult, strategyCount)
	totalSteps := jobCount * req.Generations
	if totalSteps < 1 {
		totalSteps = 1
	}
	workers := effectiveWorkerCount(req.WorkerCount, jobCount)
	reportProgress(progress, 6, fmt.Sprintf("parallel decision engine: %d strategies × %d replicates, %d workers", strategyCount, req.Replicates, workers))
	tasks := make(chan strategyTask)
	out := make(chan strategyTaskResult, jobCount)
	var completedSteps int64
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range tasks {
				strategyPop := clonePopulation(initial)
				editSet := candidates
				if task.Config.ConservativeEdits {
					editSet = conservativeEditCandidates(candidates)
				}
				if task.Config.UseCrisprSeed {
					applyCrisprSeed(strategyPop, editSet, req.CrisprIntroPercent, rand.New(rand.NewSource(req.Seed+int64(900000+task.StrategyIndex*1009+task.Replicate*97))))
				}
				strategyRNG := rand.New(rand.NewSource(req.Seed + int64(1000+task.StrategyIndex*31337+task.Replicate*7919)))
				res := simulateStrategy(req, task.Config, strategyPop, effects, baseFreq, baseDiversity, baseMean, rareUsefulAtStart, strategyRNG, func(gen int) {
					done := atomic.AddInt64(&completedSteps, 1)
					percent := 6 + int(math.Round(float64(done)*91.0/float64(totalSteps)))
					reportProgress(progress, percent, fmt.Sprintf("parallel run: %d/%d strategy-generations complete; %s replicate %d/%d generation %d/%d", done, totalSteps, task.Config.Name, task.Replicate+1, req.Replicates, gen, req.Generations))
				})
				out <- strategyTaskResult{StrategyIndex: task.StrategyIndex, Replicate: task.Replicate, Config: task.Config, Result: res}
			}
		}()
	}
	go func() {
		for i, cfg := range strategies {
			for rep := 0; rep < req.Replicates; rep++ {
				tasks <- strategyTask{StrategyIndex: i, Replicate: rep, Config: cfg}
			}
		}
		close(tasks)
		wg.Wait()
		close(out)
	}()
	for item := range out {
		resultsByStrategy[item.StrategyIndex] = append(resultsByStrategy[item.StrategyIndex], item.Result)
	}
	aggregated := make([]StrategyResult, strategyCount)
	for i, cfg := range strategies {
		aggregated[i] = aggregateReplicates(req, cfg, resultsByStrategy[i], baseDiversity)
	}
	return aggregated
}

func effectiveWorkerCount(requested, jobCount int) int {
	if jobCount < 1 {
		return 1
	}
	workers := requested
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers < 1 {
		workers = 1
	}
	if workers > 64 {
		workers = 64
	}
	if workers > jobCount {
		workers = jobCount
	}
	return workers
}
func conservativeEditCandidates(edits []EditCandidate) []EditCandidate {
	out := make([]EditCandidate, 0, len(edits))
	for _, e := range edits {
		if strings.HasPrefix(e.DiversityRisk, "low") || strings.HasPrefix(e.DiversityRisk, "medium:") {
			out = append(out, e)
		}
		if len(out) >= 3 {
			break
		}
	}
	if len(out) == 0 && len(edits) > 0 {
		out = append(out, edits[0])
	}
	return out
}

func aggregateReplicates(req SimRequest, cfg strategyConfig, reps []StrategyResult, baseDiversity float64) StrategyResult {
	if len(reps) == 0 {
		return StrategyResult{Name: cfg.Name, Code: cfg.Code, Summary: cfg.Summary}
	}
	metricCount := len(reps[0].Metrics)
	metrics := make([]MetricPoint, metricCount)
	for genIdx := 0; genIdx < metricCount; genIdx++ {
		var trait, gain, diversity, inbreeding, drift, lost, fixed, favFixed, unfavFixed, parents float64
		for _, rep := range reps {
			p := rep.Metrics[genIdx]
			trait += p.TraitMean
			gain += p.GeneticGain
			diversity += p.Diversity
			inbreeding += p.Inbreeding
			drift += p.AlleleDrift
			lost += float64(p.RareUsefulLost)
			fixed += float64(p.FixedLoci)
			favFixed += float64(p.FavorableFixed)
			unfavFixed += float64(p.UnfavorableFixed)
			parents += float64(p.EffectiveParents)
		}
		den := float64(len(reps))
		metrics[genIdx] = MetricPoint{Generation: reps[0].Metrics[genIdx].Generation, TraitMean: round4(trait / den), GeneticGain: round4(gain / den), Diversity: round4(diversity / den), Inbreeding: round4(inbreeding / den), AlleleDrift: round4(drift / den), RareUsefulLost: int(math.Round(lost / den)), FixedLoci: int(math.Round(fixed / den)), FavorableFixed: int(math.Round(favFixed / den)), UnfavorableFixed: int(math.Round(unfavFixed / den)), EffectiveParents: int(math.Round(parents / den))}
	}
	gains, divs, inbr := make([]float64, 0, len(reps)), make([]float64, 0, len(reps)), make([]float64, 0, len(reps))
	rareLossEvents, inbreedingBreaches, diversityCollapses := 0, 0, 0
	for _, rep := range reps {
		f := rep.Final
		gains = append(gains, f.GeneticGain)
		divs = append(divs, f.Diversity)
		inbr = append(inbr, f.Inbreeding)
		if f.RareUsefulLost > 0 {
			rareLossEvents++
		}
		if f.Inbreeding >= req.InbreedingLimit {
			inbreedingBreaches++
		}
		diversityLoss := 0.0
		if baseDiversity > 0 {
			diversityLoss = (baseDiversity - f.Diversity) / baseDiversity
		}
		if diversityLoss >= req.DiversityLossLimit {
			diversityCollapses++
		}
	}
	finalMetric := metrics[len(metrics)-1]
	final := FinalStats{TraitMean: finalMetric.TraitMean, GeneticGain: finalMetric.GeneticGain, Diversity: finalMetric.Diversity, Inbreeding: finalMetric.Inbreeding, AlleleDrift: finalMetric.AlleleDrift, RareUsefulLost: finalMetric.RareUsefulLost, FixedLoci: finalMetric.FixedLoci, FavorableFixed: finalMetric.FavorableFixed, UnfavorableFixed: finalMetric.UnfavorableFixed, EffectiveParents: finalMetric.EffectiveParents, Replicates: len(reps), GainStd: round4(stddev(gains)), DiversityStd: round4(stddev(divs)), InbreedingStd: round4(stddev(inbr)), ProbabilityInbreedingBreach: round4(float64(inbreedingBreaches) / float64(len(reps))), ProbabilityDiversityCollapse: round4(float64(diversityCollapses) / float64(len(reps))), ProbabilityRareUsefulLoss: round4(float64(rareLossEvents) / float64(len(reps)))}
	final.RecommendedNext = recommendationFor(cfg.Code, final)
	return StrategyResult{Name: cfg.Name, Code: cfg.Code, Summary: cfg.Summary, Replicates: len(reps), Metrics: metrics, Final: final}
}

func annotateDecisionScores(results []StrategyResult) {
	if len(results) == 0 {
		return
	}
	gainMin, gainMax := minMaxStrategy(results, func(s StrategyResult) float64 { return s.Final.GeneticGain })
	divMin, divMax := minMaxStrategy(results, func(s StrategyResult) float64 { return s.Final.Diversity })
	inbMin, inbMax := minMaxStrategy(results, func(s StrategyResult) float64 { return s.Final.Inbreeding })
	riskMin, riskMax := minMaxStrategy(results, func(s StrategyResult) float64 { return combinedRisk(s.Final) })
	lostMin, lostMax := minMaxStrategy(results, func(s StrategyResult) float64 { return float64(s.Final.RareUsefulLost) })
	for i := range results {
		f := results[i].Final
		gain := norm(f.GeneticGain, gainMin, gainMax)
		div := norm(f.Diversity, divMin, divMax)
		lowInb := 1.0 - norm(f.Inbreeding, inbMin, inbMax)
		lowRisk := 1.0 - norm(combinedRisk(f), riskMin, riskMax)
		lowLost := 1.0 - norm(float64(f.RareUsefulLost), lostMin, lostMax)
		results[i].Final.RiskAdjustedScore = round4(0.40*gain + 0.22*div + 0.18*lowInb + 0.15*lowRisk + 0.05*lowLost)
	}
	for i := range results {
		pareto := true
		for j := range results {
			if i != j && dominates(results[j].Final, results[i].Final) {
				pareto = false
				break
			}
		}
		results[i].ParetoOptimal = pareto
		results[i].Final.ParetoOptimal = pareto
	}
	type ranked struct {
		idx   int
		score float64
	}
	ranks := make([]ranked, len(results))
	for i := range results {
		ranks[i] = ranked{i, results[i].Final.RiskAdjustedScore}
	}
	sort.Slice(ranks, func(i, j int) bool {
		if ranks[i].score == ranks[j].score {
			return results[ranks[i].idx].Final.GeneticGain > results[ranks[j].idx].Final.GeneticGain
		}
		return ranks[i].score > ranks[j].score
	})
	for rank, item := range ranks {
		results[item.idx].Final.DecisionRank = rank + 1
	}
}
func dominates(a, b FinalStats) bool {
	betterOrEqual := a.GeneticGain >= b.GeneticGain && a.Diversity >= b.Diversity && a.Inbreeding <= b.Inbreeding && a.ProbabilityDiversityCollapse <= b.ProbabilityDiversityCollapse && a.ProbabilityRareUsefulLoss <= b.ProbabilityRareUsefulLoss
	strictlyBetter := a.GeneticGain > b.GeneticGain || a.Diversity > b.Diversity || a.Inbreeding < b.Inbreeding || a.ProbabilityDiversityCollapse < b.ProbabilityDiversityCollapse || a.ProbabilityRareUsefulLoss < b.ProbabilityRareUsefulLoss
	return betterOrEqual && strictlyBetter
}
func combinedRisk(f FinalStats) float64 {
	return 0.45*f.ProbabilityInbreedingBreach + 0.35*f.ProbabilityDiversityCollapse + 0.20*f.ProbabilityRareUsefulLoss
}

func buildDecisionSummary(results []StrategyResult) DecisionSummary {
	if len(results) == 0 {
		return DecisionSummary{}
	}
	bestRiskIdx, bestGainIdx, lowestRiskIdx := 0, 0, 0
	for i := range results {
		if results[i].Final.RiskAdjustedScore > results[bestRiskIdx].Final.RiskAdjustedScore {
			bestRiskIdx = i
		}
		if results[i].Final.GeneticGain > results[bestGainIdx].Final.GeneticGain {
			bestGainIdx = i
		}
		if combinedRisk(results[i].Final) < combinedRisk(results[lowestRiskIdx].Final) {
			lowestRiskIdx = i
		}
	}
	pareto := make([]string, 0)
	for _, s := range results {
		if s.ParetoOptimal {
			pareto = append(pareto, s.Code)
		}
	}
	best, bestGain, lowest := results[bestRiskIdx], results[bestGainIdx], results[lowestRiskIdx]
	return DecisionSummary{BestRiskAdjustedCode: best.Code, BestRiskAdjustedName: best.Name, BestGainCode: bestGain.Code, LowestRiskCode: lowest.Code, ParetoCodes: pareto, Interpretation: []string{fmt.Sprintf("Recommended risk-adjusted strategy: %s (score %.4f, rank #%d).", best.Name, best.Final.RiskAdjustedScore, best.Final.DecisionRank), fmt.Sprintf("Maximum final gain is produced by %s, but compare its risk probabilities before treating it as deployable.", bestGain.Name), fmt.Sprintf("Lowest combined risk is produced by %s.", lowest.Name), "Use the Pareto chart to choose a trade-off, not a single metric. Real BreedOS should optimize under explicit constraints supplied by the breeding program."}}
}
func minMaxStrategy(results []StrategyResult, fn func(StrategyResult) float64) (float64, float64) {
	minV, maxV := math.Inf(1), math.Inf(-1)
	for _, s := range results {
		v := fn(s)
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}
	return minV, maxV
}
func norm(v, minV, maxV float64) float64 {
	if math.Abs(maxV-minV) < 1e-12 {
		return 0.5
	}
	x := (v - minV) / (maxV - minV)
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

func normalizeRequest(req *SimRequest) {
	if req.Seed == 0 {
		req.Seed = time.Now().UnixNano() % 1000000000
	}
	if req.PopulationSize == 0 {
		req.PopulationSize = 400
	}
	if req.Markers == 0 {
		req.Markers = 900
	}
	if req.QTLCount == 0 {
		req.QTLCount = 45
	}
	if req.Generations == 0 {
		req.Generations = 30
	}
	if req.SelectionPercent == 0 {
		req.SelectionPercent = 10
	}
	if req.Heritability == 0 {
		req.Heritability = 0.4
	}
	if req.StrategySet == "" {
		req.StrategySet = "core"
	}
	req.StrategySet = strings.ToLower(strings.TrimSpace(req.StrategySet))
	if req.Replicates == 0 {
		req.Replicates = 3
	}
	if req.InbreedingLimit == 0 {
		req.InbreedingLimit = 0.25
	}
	if req.DiversityLossLimit == 0 {
		req.DiversityLossLimit = 0.30
	}
}
func validateRequest(req SimRequest, strategyCount int) error {
	if req.PopulationSize < 2 || req.PopulationSize > 5000 {
		return errors.New("population_size must be between 2 and 5000")
	}
	if req.Markers < 10 || req.Markers > 5000 {
		return errors.New("markers must be between 10 and 5000")
	}
	if req.QTLCount < 1 || req.QTLCount > req.Markers {
		return errors.New("qtl_count must be between 1 and markers")
	}
	if req.Generations < 1 || req.Generations > 200 {
		return errors.New("generations must be between 1 and 200")
	}
	if req.SelectionPercent <= 0 || req.SelectionPercent > 100 {
		return errors.New("selection_percent must be in (0, 100]")
	}
	if req.Heritability <= 0 || req.Heritability > 1 {
		return errors.New("heritability must be in (0, 1]")
	}
	if req.MutationRate < 0 || req.MutationRate > 0.01 {
		return errors.New("mutation_rate must be between 0 and 0.01")
	}
	if req.CrisprEdits < 0 || req.CrisprEdits > 20 {
		return errors.New("crispr_edits must be between 0 and 20")
	}
	if req.CrisprIntroPercent < 0 || req.CrisprIntroPercent > 100 {
		return errors.New("crispr_intro_percent must be between 0 and 100")
	}
	if req.StrategySet != "core" && req.StrategySet != "advanced" {
		return errors.New("strategy_set must be core or advanced")
	}
	if req.Replicates < 1 || req.Replicates > 100 {
		return errors.New("replicates must be between 1 and 100")
	}
	if req.WorkerCount < 0 || req.WorkerCount > 64 {
		return errors.New("worker_count must be between 0 and 64; 0 means auto")
	}
	if req.InbreedingLimit <= 0 || req.InbreedingLimit > 1 {
		return errors.New("inbreeding_limit must be in (0, 1]")
	}
	if req.DiversityLossLimit <= 0 || req.DiversityLossLimit > 1 {
		return errors.New("diversity_loss_limit must be in (0, 1]")
	}
	budget := simulationBudget(req, strategyCount)
	if budget > 800000000 {
		return fmt.Errorf("simulation budget too high: population_size * markers * (generations + 1) * strategies * replicates must be <= 800,000,000 for the MVP; got %d", budget)
	}
	return nil
}
func simulationBudget(req SimRequest, strategyCount int) int64 {
	return int64(req.PopulationSize) * int64(req.Markers) * int64(req.Generations+1) * int64(strategyCount) * int64(req.Replicates)
}

func makeEffects(markers, qtl int, rng *rand.Rand) []float64 {
	effects := make([]float64, markers)
	used := make(map[int]bool, qtl)
	for len(used) < qtl {
		idx := rng.Intn(markers)
		if used[idx] {
			continue
		}
		used[idx] = true
		v := rng.NormFloat64()
		if math.Abs(v) < 0.15 {
			if v < 0 {
				v = -0.15
			} else {
				v = 0.15
			}
		}
		effects[idx] = v
	}
	return effects
}
func makeInitialPopulation(n, markers int, rng *rand.Rand) []organism {
	freq := make([]float64, markers)
	for m := 0; m < markers; m++ {
		freq[m] = 0.03 + rng.Float64()*0.94
	}
	pop := make([]organism, n)
	for i := range pop {
		g := make([]uint8, markers)
		for m := 0; m < markers; m++ {
			var x uint8
			if rng.Float64() < freq[m] {
				x++
			}
			if rng.Float64() < freq[m] {
				x++
			}
			g[m] = x
		}
		pop[i] = organism{geno: g}
	}
	return pop
}
func clonePopulation(pop []organism) []organism {
	out := make([]organism, len(pop))
	for i := range pop {
		g := make([]uint8, len(pop[i].geno))
		copy(g, pop[i].geno)
		out[i] = organism{geno: g}
	}
	return out
}

func simulateStrategy(req SimRequest, cfg strategyConfig, pop []organism, effects []float64, baseFreq []float64, baseDiversity, baseMean float64, rareUsefulAtStart []int, rng *rand.Rand, progress func(gen int)) StrategyResult {
	metrics := make([]MetricPoint, 0, req.Generations+1)
	metrics = append(metrics, computeMetrics(0, pop, effects, baseFreq, baseDiversity, baseMean, rareUsefulAtStart, 0))
	for gen := 1; gen <= req.Generations; gen++ {
		parents := selectParents(pop, effects, req, cfg, rng)
		pop = makeNextGeneration(pop, parents, req.Markers, req.MutationRate, rng, cfg.MatingRule)
		metrics = append(metrics, computeMetrics(gen, pop, effects, baseFreq, baseDiversity, baseMean, rareUsefulAtStart, len(parents)))
		if progress != nil {
			progress(gen)
		}
	}
	finalMetric := metrics[len(metrics)-1]
	final := finalMetricToFinal(finalMetric)
	final.Replicates = 1
	final.RecommendedNext = recommendationFor(cfg.Code, final)
	return StrategyResult{Name: cfg.Name, Code: cfg.Code, Summary: cfg.Summary, Replicates: 1, Metrics: metrics, Final: final}
}
func finalMetricToFinal(m MetricPoint) FinalStats {
	return FinalStats{TraitMean: m.TraitMean, GeneticGain: m.GeneticGain, Diversity: m.Diversity, Inbreeding: m.Inbreeding, AlleleDrift: m.AlleleDrift, RareUsefulLost: m.RareUsefulLost, FixedLoci: m.FixedLoci, FavorableFixed: m.FavorableFixed, UnfavorableFixed: m.UnfavorableFixed, EffectiveParents: m.EffectiveParents}
}

func selectParents(pop []organism, effects []float64, req SimRequest, cfg strategyConfig, rng *rand.Rand) []int {
	n := len(pop)
	if cfg.Rule == "no_selection" {
		parents := make([]int, n)
		for i := range parents {
			parents[i] = i
		}
		return parents
	}
	baseCount := int(math.Round(float64(n) * req.SelectionPercent / 100.0))
	parentCount := int(math.Round(float64(baseCount) * cfg.ParentMultiplier))
	if parentCount < 2 {
		parentCount = 2
	}
	if parentCount > n {
		parentCount = n
	}
	if cfg.Rule == "random" {
		perm := rng.Perm(n)
		return append([]int(nil), perm[:parentCount]...)
	}
	freq := alleleFreq(pop, req.Markers)
	gvals := geneticValues(pop, effects)
	meanG, varG := meanVar(gvals)
	stdG := math.Sqrt(math.Max(varG, 1e-9))
	varEnv := 0.0
	if req.Heritability < 1.0 {
		varEnv = varG * (1.0 - req.Heritability) / req.Heritability
	}
	stdEnv := math.Sqrt(math.Max(varEnv, 1e-9))
	novelties := make([]float64, n)
	for i := range pop {
		var novelty float64
		for m, gv := range pop[i].geno {
			p := freq[m]
			x := float64(gv) / 2.0
			d := x - p
			novelty += d * d
		}
		novelties[i] = novelty / float64(req.Markers)
	}
	meanN, varN := meanVar(novelties)
	stdN := math.Sqrt(math.Max(varN, 1e-9))
	scored := make([]scoredIndividual, n)
	for i := range pop {
		phenotype := gvals[i] + rng.NormFloat64()*stdEnv
		zTrait := (phenotype - meanG) / stdG
		zNovelty := (novelties[i] - meanN) / stdN
		genomicPred := gvals[i]
		if cfg.Rule == "genomic" {
			accuracy := math.Sqrt(math.Max(0.01, req.Heritability))
			genomicPred = meanG + accuracy*(gvals[i]-meanG) + rng.NormFloat64()*stdG*(1.0-accuracy+0.10)
		}
		var score float64
		switch cfg.Rule {
		case "phenotype":
			score = zTrait
		case "genomic":
			score = (genomicPred - meanG) / stdG
		default:
			score = cfg.TraitWeight*zTrait + cfg.NoveltyWeight*zNovelty
		}
		scored[i] = scoredIndividual{idx: i, trait: phenotype, genomic: genomicPred, novelty: novelties[i], score: score}
	}
	sort.Slice(scored, func(i, j int) bool { return scored[i].score > scored[j].score })
	if cfg.Rule == "ocs" {
		return selectOCSLike(pop, scored, parentCount, cfg.SimilarityPenalty)
	}
	parents := make([]int, parentCount)
	for i := 0; i < parentCount; i++ {
		parents[i] = scored[i].idx
	}
	return parents
}
func selectOCSLike(pop []organism, scored []scoredIndividual, parentCount int, penalty float64) []int {
	if parentCount >= len(scored) {
		parents := make([]int, len(scored))
		for i := range scored {
			parents[i] = scored[i].idx
		}
		return parents
	}
	poolCount := parentCount * 5
	if poolCount < parentCount {
		poolCount = parentCount
	}
	if poolCount > len(scored) {
		poolCount = len(scored)
	}
	if poolCount > 600 {
		poolCount = 600
	}
	selected := []int{scored[0].idx}
	used := map[int]bool{scored[0].idx: true}
	for len(selected) < parentCount {
		bestIdx, bestScore := -1, math.Inf(-1)
		for i := 0; i < poolCount; i++ {
			candidate := scored[i].idx
			if used[candidate] {
				continue
			}
			value := scored[i].score - penalty*avgSimilarityToSelected(pop, candidate, selected)
			if value > bestScore {
				bestScore = value
				bestIdx = candidate
			}
		}
		if bestIdx < 0 {
			break
		}
		selected = append(selected, bestIdx)
		used[bestIdx] = true
	}
	return selected
}
func avgSimilarityToSelected(pop []organism, candidate int, selected []int) float64 {
	if len(selected) == 0 {
		return 0
	}
	var sum float64
	for _, idx := range selected {
		sum += genotypeSimilarity(pop[candidate].geno, pop[idx].geno)
	}
	return sum / float64(len(selected))
}
func genotypeSimilarity(a, b []uint8) float64 {
	if len(a) == 0 {
		return 0
	}
	var sim float64
	for i := range a {
		sim += 1.0 - math.Abs(float64(a[i])-float64(b[i]))/2.0
	}
	return sim / float64(len(a))
}
func genotypeDistance(a, b []uint8) float64 { return 1.0 - genotypeSimilarity(a, b) }
func makeNextGeneration(pop []organism, parents []int, markers int, mutationRate float64, rng *rand.Rand, matingRule string) []organism {
	n := len(pop)
	next := make([]organism, n)
	for i := 0; i < n; i++ {
		p1 := parents[rng.Intn(len(parents))]
		p2 := chooseSecondParent(pop, parents, p1, rng, matingRule)
		g := make([]uint8, markers)
		for m := 0; m < markers; m++ {
			a := drawGamete(pop[p1].geno[m], rng)
			b := drawGamete(pop[p2].geno[m], rng)
			if mutationRate > 0 && rng.Float64() < mutationRate {
				a = 1 - a
			}
			if mutationRate > 0 && rng.Float64() < mutationRate {
				b = 1 - b
			}
			g[m] = a + b
		}
		next[i] = organism{geno: g}
	}
	return next
}
func chooseSecondParent(pop []organism, parents []int, p1 int, rng *rand.Rand, matingRule string) int {
	if len(parents) == 1 {
		return p1
	}
	if matingRule != "diverse_pairs" {
		p2 := parents[rng.Intn(len(parents))]
		for p2 == p1 {
			p2 = parents[rng.Intn(len(parents))]
		}
		return p2
	}
	best, bestDist := -1, math.Inf(-1)
	samples := 12
	if samples > len(parents)-1 {
		samples = len(parents) - 1
	}
	for s := 0; s < samples; s++ {
		p2 := parents[rng.Intn(len(parents))]
		for p2 == p1 {
			p2 = parents[rng.Intn(len(parents))]
		}
		d := genotypeDistance(pop[p1].geno, pop[p2].geno)
		if d > bestDist {
			bestDist = d
			best = p2
		}
	}
	if best >= 0 {
		return best
	}
	return parents[0]
}
func drawGamete(genotype uint8, rng *rand.Rand) uint8 {
	switch genotype {
	case 0:
		return 0
	case 2:
		return 1
	default:
		if rng.Intn(2) == 0 {
			return 0
		}
		return 1
	}
}

func computeMetrics(gen int, pop []organism, effects []float64, baseFreq []float64, baseDiversity, baseMean float64, rareUsefulAtStart []int, effectiveParents int) MetricPoint {
	freq := alleleFreq(pop, len(effects))
	div := diversityFromFreq(freq)
	trait := meanGeneticValue(pop, effects)
	drift := meanAbsDiff(freq, baseFreq)
	fixed, favorableFixed, unfavorableFixed := fixationStats(freq, effects)
	inbreeding := 0.0
	if baseDiversity > 0 {
		inbreeding = 1.0 - div/baseDiversity
	}
	if inbreeding < 0 {
		inbreeding = 0
	}
	lost := countRareUsefulLost(freq, rareUsefulAtStart)
	return MetricPoint{Generation: gen, TraitMean: round4(trait), GeneticGain: round4(trait - baseMean), Diversity: round4(div), Inbreeding: round4(inbreeding), AlleleDrift: round4(drift), RareUsefulLost: lost, FixedLoci: fixed, FavorableFixed: favorableFixed, UnfavorableFixed: unfavorableFixed, EffectiveParents: effectiveParents}
}
func fixationStats(freq []float64, effects []float64) (fixed, favorableFixed, unfavorableFixed int) {
	for i, p := range freq {
		fixedToZero, fixedToOne := p <= 0.001, p >= 0.999
		if !fixedToZero && !fixedToOne {
			continue
		}
		fixed++
		if effects[i] == 0 {
			continue
		}
		if effects[i] > 0 {
			if fixedToOne {
				favorableFixed++
			} else {
				unfavorableFixed++
			}
			continue
		}
		if fixedToZero {
			favorableFixed++
		} else {
			unfavorableFixed++
		}
	}
	return fixed, favorableFixed, unfavorableFixed
}
func geneticValues(pop []organism, effects []float64) []float64 {
	out := make([]float64, len(pop))
	for i := range pop {
		var v float64
		for m, g := range pop[i].geno {
			if effects[m] != 0 {
				v += float64(g) * effects[m]
			}
		}
		out[i] = v
	}
	return out
}
func meanGeneticValue(pop []organism, effects []float64) float64 {
	vals := geneticValues(pop, effects)
	mean, _ := meanVar(vals)
	return mean
}
func alleleFreq(pop []organism, markers int) []float64 {
	freq := make([]float64, markers)
	denom := float64(len(pop) * 2)
	for i := range pop {
		for m, g := range pop[i].geno {
			freq[m] += float64(g)
		}
	}
	for m := range freq {
		freq[m] /= denom
	}
	return freq
}
func diversityFromFreq(freq []float64) float64 {
	var s float64
	for _, p := range freq {
		s += 2.0 * p * (1.0 - p)
	}
	return s / float64(len(freq))
}
func meanAbsDiff(a, b []float64) float64 {
	var s float64
	for i := range a {
		s += math.Abs(a[i] - b[i])
	}
	return s / float64(len(a))
}
func meanVar(vals []float64) (float64, float64) {
	if len(vals) == 0 {
		return 0, 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	mean := sum / float64(len(vals))
	var ss float64
	for _, v := range vals {
		d := v - mean
		ss += d * d
	}
	return mean, ss / float64(len(vals))
}
func stddev(vals []float64) float64 {
	_, variance := meanVar(vals)
	return math.Sqrt(math.Max(0, variance))
}
func rareUsefulLoci(freq []float64, effects []float64) []int {
	out := make([]int, 0)
	for i := range freq {
		if effects[i] > 0 && freq[i] > 0.02 && freq[i] < 0.25 {
			out = append(out, i)
		}
	}
	return out
}
func countRareUsefulLost(freq []float64, loci []int) int {
	lost := 0
	for _, idx := range loci {
		if idx >= 0 && idx < len(freq) && freq[idx] < 0.01 {
			lost++
		}
	}
	return lost
}
func rankEditCandidates(freq []float64, effects []float64, maxEdits int) []EditCandidate {
	if maxEdits <= 0 {
		return nil
	}
	type item struct {
		locus int
		score float64
	}
	items := make([]item, 0)
	for i := range effects {
		if effects[i] <= 0 {
			continue
		}
		p := freq[i]
		score := effects[i] * (1.0 - p)
		if score > 0 {
			items = append(items, item{i, score})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].score > items[j].score })
	if len(items) > maxEdits {
		items = items[:maxEdits]
	}
	out := make([]EditCandidate, 0, len(items))
	for rank, it := range items {
		p := freq[it.locus]
		risk := "low"
		if p < 0.05 {
			risk = "medium: avoid single-founder bottleneck"
		}
		if effects[it.locus] > 1.4 && p < 0.10 {
			risk = "medium-high: validate pleiotropy and background effects"
		}
		decision := "seed edit into limited founders, then test under balanced selection"
		if p > 0.70 {
			decision = "prefer selection/crossing; edit adds limited marginal value"
		}
		out = append(out, EditCandidate{Rank: rank + 1, Locus: it.locus, Effect: round4(effects[it.locus]), AlleleFrequency: round4(p), ExpectedGainScore: round4(it.score), DiversityRisk: risk, Decision: decision})
	}
	return out
}
func applyCrisprSeed(pop []organism, edits []EditCandidate, introPercent float64, rng *rand.Rand) {
	if introPercent <= 0 || len(edits) == 0 {
		return
	}
	count := int(math.Round(float64(len(pop)) * introPercent / 100.0))
	if count < 1 {
		count = 1
	}
	if count > len(pop) {
		count = len(pop)
	}
	perm := rng.Perm(len(pop))
	for i := 0; i < count; i++ {
		ind := perm[i]
		for _, edit := range edits {
			if edit.Locus >= 0 && edit.Locus < len(pop[ind].geno) {
				pop[ind].geno[edit.Locus] = 2
			}
		}
	}
}

func recommendationFor(code string, f FinalStats) string {
	riskText := ""
	if f.ProbabilityInbreedingBreach >= 0.5 || f.ProbabilityDiversityCollapse >= 0.5 {
		riskText = " High risk probability: treat as non-deployable without stronger constraints."
	}
	switch code {
	case "neutral":
		return "Use as a baseline only; it reveals drift without intentional gain." + riskText
	case "random":
		return "Use as a stochastic baseline; a real strategy should beat it on risk-adjusted score." + riskText
	case "phenotype":
		return "Good practical baseline; compare against genomic and OCS-like strategies under noise." + riskText
	case "genomic":
		return "Promising when marker/phenotype data support prediction; replace mock predictor with real GEBV models." + riskText
	case "ocs_like":
		return "Strong early BreedOS candidate: gain is pursued under explicit diversity pressure." + riskText
	case "cross_planner":
		return "Use when mating allocation matters; next step is real pedigree and kinship-aware cross optimization." + riskText
	case "edit_introgression":
		return "Use as the CRISPR-aware decision demo: edit candidates still require biological validation and partner lab work." + riskText
	case "aggressive":
		if f.Inbreeding > 0.35 || f.Diversity < 0.20 {
			return "Use only as a short-term upper-bound baseline; add diversity constraints before real deployment." + riskText
		}
		return "Good short-term gain; still run a diversity-constrained scenario before choosing parents." + riskText
	case "diversity":
		return "Use when the program is bottlenecked, rare alleles are strategic, or future environments are uncertain." + riskText
	case "balanced_crispr":
		return "Use as a CRISPR-integration demo: validate candidate edits, seed narrowly, then propagate through balanced selection." + riskText
	default:
		return "Recommended default: preserve enough diversity while still producing visible trait gain." + riskText
	}
}
func round4(v float64) float64 { return math.Round(v*10000) / 10000 }
func setContentType(w http.ResponseWriter, path string) {
	if strings.HasSuffix(path, ".html") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		return
	}
	if strings.HasSuffix(path, ".css") {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		return
	}
	if strings.HasSuffix(path, ".js") {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
}
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Truncate(time.Millisecond))
	})
}
func normalizeListenForLog(s string) string {
	if strings.HasPrefix(s, ":") {
		return "127.0.0.1" + s
	}
	return s
}
func _unusedFmtGuard() { fmt.Print("") }
