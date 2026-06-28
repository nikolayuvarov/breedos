package main

// v0.7.31 — Promptogenesis v0.1 Prompt Genome Mapper HTTP surface.
// See ingest-done/handoff-dna-prompt.md.done Section 6 for the
// request/response contract.

import (
	"encoding/json"
	"net/http"

	"breedos-mvp/promptbio"
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
