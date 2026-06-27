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
