package main

// v0.7.31 — Promptogenesis v0.1 Prompt Genome Mapper HTTP surface.
// See ingest-done/handoff-dna-prompt.md.done Section 6 for the
// request/response contract.

import (
	"encoding/json"
	"net/http"
	"strings"

	"breedos-mvp/promptbio"
	"breedos-mvp/promptbio/eval"
)

func promptbioMapHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()
	var req promptbio.MapRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}
	out := promptbio.MapPrompt(req)
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

// v0.7.34 — Issue 03 Prompt Evolution Loop. Grows a population of
// 14-locus PromptOrganisms from a single ancestor, scores across 3
// canonical niches, selects via per-niche top-1 + global top-K,
// iterates N generations, returns lineage tree + niche winners +
// changelog. Deterministic given the seed.
func promptbioEvolveHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()
	var req promptbio.EvolveRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.AncestorPrompt == "" {
		http.Error(w, "ancestor_prompt is required", http.StatusBadRequest)
		return
	}
	out := promptbio.Evolve(req)
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

// v0.7.33 — Issue 07 substrate abstraction: promptbio Simulate.
// Runs the same five core engine moves the biological side runs
// (truncation, balanced, drift, OCS-like, introgression) over the
// prompt-organism substrate using a deterministic placeholder judge.
// Returns a substrate-uniform SimulateResponse.
func promptbioSimulateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()
	var req promptbio.SimulateRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.AncestorPrompt == "" {
		http.Error(w, "ancestor_prompt is required", http.StatusBadRequest)
		return
	}
	// Bound population × generations to keep response time predictable.
	if req.PopulationSize > 200 {
		req.PopulationSize = 200
	}
	if req.Generations > 30 {
		req.Generations = 30
	}
	if req.Replicates > 10 {
		req.Replicates = 10
	}
	out := promptbio.Simulate(req)
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

// v0.7.36 — Issue 16 v1.3 Prompt Evaluation Lab. Two endpoints:
//   POST /api/promptbio/eval/run        → full EvaluationLabReport (13 sections)
//   POST /api/promptbio/eval/regression → pairwise ancestor↔descendant regression report
//
// Real LLM-as-judge integration is queued for v1.4; v0.7.36 ships a
// deterministic genome-map-driven judging stack so the ladder
// P₀ raw → P₁.₁ engineered → P₁.₂ compiled is reproducible.
func promptbioEvalRunHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()
	var req eval.EvalRunRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.TargetPrompt) == "" {
		http.Error(w, "target_prompt is required", http.StatusBadRequest)
		return
	}
	out := eval.Run(req)
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

func promptbioEvalRegressionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()
	var req eval.RegressionRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.AncestorPrompt == "" || req.DescendantPrompt == "" {
		http.Error(w, "ancestor_prompt and descendant_prompt are required", http.StatusBadRequest)
		return
	}
	out := eval.Run(eval.EvalRunRequest{
		TargetPrompt:   req.DescendantPrompt,
		AncestorPrompt: req.AncestorPrompt,
		TaskFamily:     req.TaskFamily,
		CostLambda:     0.1,
	})
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out.Regression)
}

// v0.7.35 — Issue 30 Epistemology & Truth Maintenance. Three endpoints:
//   POST /api/promptbio/epistemology/plan   → full 16-section PromptEpistemologyPlan
//   POST /api/promptbio/epistemology/gate   → runtime-gate verdict + anti-pattern hits
//   POST /api/promptbio/epistemology/update → belief-update protocol (deprecate + propagate)
func promptbioEpistemologyPlanHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()
	var req promptbio.PlanRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}
	out := promptbio.Plan(req)
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

func promptbioEpistemologyGateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()
	var req promptbio.GateRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}
	out := promptbio.RunGate(req)
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

func promptbioEpistemologyUpdateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()
	var req promptbio.UpdateRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}
	out := promptbio.ApplyUpdate(req)
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

// v0.7.32 — Promptogenesis v0.2 Prompt Genome Diff HTTP surface.
// See ingest-done/06-prompt-dna.md.done and handoff Section 6.4.
func promptbioDiffHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()
	var req promptbio.DiffRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Ancestor == "" || req.Descendant == "" {
		http.Error(w, "ancestor and descendant are required", http.StatusBadRequest)
		return
	}
	out := promptbio.DiffPrompts(req)
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}
