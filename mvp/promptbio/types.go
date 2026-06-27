// Package promptbio implements the v0.1 Prompt Genome Mapper substrate
// described in `ingest-done/handoff-dna-prompt.md.done`.
//
// Scope per the handoff:
//   - Static heuristic analyzer; NO live LLM dependency.
//   - 14 loci with the codes / order in the handoff (Section 4.2).
//   - Six-status taxonomy (missing / weak / present / strong / conflicting / not_applicable).
//   - Output shape matches the JSON in the handoff (Section 4.5).
//
// Non-goals for v0.1 (per handoff Section 13): live LLM evaluation,
// full evolutionary search, AutoPromptOps, hidden tests, knowledge
// graph, PML certification, complex ontology, metabolism, agent
// runtime, real tool execution.
//
// The biological BreedOS path remains bit-identical; this package is
// imported by the main package only via the HTTP route in
// `mvp/main.go` (POST /api/promptbio/map).
package promptbio

// LocusStatus is the six-state assessment per locus.
type LocusStatus string

const (
	LocusMissing       LocusStatus = "missing"
	LocusWeak          LocusStatus = "weak"
	LocusPresent       LocusStatus = "present"
	LocusStrong        LocusStatus = "strong"
	LocusConflicting   LocusStatus = "conflicting"
	LocusNotApplicable LocusStatus = "not_applicable"
)

// LocusName enumerates the 14 named positions from the handoff's
// Section 4.2 table. Order is significant — Loci slice in GenomeMap
// uses this order.
type LocusName string

const (
	LocusTask       LocusName = "task"
	LocusRole       LocusName = "role"
	LocusAudience   LocusName = "audience"
	LocusContext    LocusName = "context"
	LocusConstraint LocusName = "constraint"
	LocusMethod     LocusName = "method"
	LocusEpistemic  LocusName = "epistemic"
	LocusOutput     LocusName = "output_schema"
	LocusValidation LocusName = "validation"
	LocusTool       LocusName = "tool"
	LocusMemory     LocusName = "memory"
	LocusSafety     LocusName = "safety_boundary"
	LocusUX         LocusName = "ux"
	LocusEvolution  LocusName = "evolution"
)

// allLoci is the canonical iteration order. Don't shuffle — tests
// and clients rely on this ordering for stable diffs.
var allLoci = []LocusName{
	LocusTask,
	LocusRole,
	LocusAudience,
	LocusContext,
	LocusConstraint,
	LocusMethod,
	LocusEpistemic,
	LocusOutput,
	LocusValidation,
	LocusTool,
	LocusMemory,
	LocusSafety,
	LocusUX,
	LocusEvolution,
}

// LocusAssessment is one row in the genome-map output: the status, an
// optional numeric score (per the handoff's missing=0 / weak=1 /
// present=2 / strong=3 / conflicting=-1 mapping), a short evidence
// string ("which fragment of the prompt drove this verdict"), the
// risk if the locus is missing, and the suggested mutation.
type LocusAssessment struct {
	Name              LocusName   `json:"name"`
	Status            LocusStatus `json:"status"`
	Score             *float64    `json:"score,omitempty"`
	Evidence          string      `json:"evidence,omitempty"`
	RiskIfMissing     string      `json:"risk_if_missing,omitempty"`
	SuggestedMutation string      `json:"suggested_mutation,omitempty"`
}

// ExpectedPhenotype is the v0.1 prediction layer. LikelyOutput is one
// or two sentences; FailureModes is the catalog from the handoff
// Section 5.2 example; Confidence is "low" / "medium" / "high" (the
// static analyzer never claims high without strong evidence).
type ExpectedPhenotype struct {
	LikelyOutput string   `json:"likely_output"`
	FailureModes []string `json:"failure_modes"`
	Confidence   string   `json:"confidence"`
}

// MutationSuggestion is one entry in the mutation plan. MutationType
// is one of: addition / deletion / substitution / amplification /
// suppression / modularisation (per handoff Section 6.4). TargetLocus
// is a LocusName string; Patch is a literal text snippet the operator
// can append / replace; Rationale is the one-line "why".
type MutationSuggestion struct {
	MutationType string `json:"mutation_type"`
	TargetLocus  string `json:"target_locus"`
	Patch        string `json:"patch"`
	Rationale    string `json:"rationale"`
}

// GenomeMap is the top-level output of MapPrompt.
type GenomeMap struct {
	PromptID          string               `json:"prompt_id"`
	InputPrompt       string               `json:"input_prompt"`
	Language          string               `json:"language,omitempty"`
	SpeciesHint       string               `json:"species_hint,omitempty"`
	GenomeScore       float64              `json:"genome_score"`
	Loci              []LocusAssessment    `json:"loci"`
	MissingLoci       []LocusName          `json:"missing_loci"`
	ConflictingLoci   []LocusName          `json:"conflicting_loci"`
	ExpectedPhenotype ExpectedPhenotype    `json:"expected_phenotype"`
	MutationPlan      []MutationSuggestion `json:"mutation_plan"`
	TestsToRun        []string             `json:"tests_to_run"`
}

// MapRequest mirrors the handoff's Section 6.2 request shape.
// Language defaults to "auto" (the analyzer detects ru vs en
// heuristically from the prompt text). SpeciesHint is a tag like
// "strategy" / "research" / "coding" / "document" / "agent" /
// "memory" / "eval" / "explainer" — currently informational only;
// v0.2 may weight loci by species.
type MapRequest struct {
	Prompt      string `json:"prompt"`
	Language    string `json:"language,omitempty"`
	SpeciesHint string `json:"species_hint,omitempty"`
}

// statusScore returns the numeric mapping from the handoff Section
// 4.3. not_applicable contributes nothing to the genome score (it is
// represented as nil in the Score field of the assessment so JSON
// emits "score": null or omits it).
func statusScore(s LocusStatus) (float64, bool) {
	switch s {
	case LocusMissing:
		return 0, true
	case LocusWeak:
		return 1, true
	case LocusPresent:
		return 2, true
	case LocusStrong:
		return 3, true
	case LocusConflicting:
		return -1, true
	case LocusNotApplicable:
		return 0, false
	}
	return 0, false
}

// locusRiskAndMutation returns canonical risk + suggested-mutation
// strings for a missing locus. Pulled from handoff Section 4.2 table
// and Section 5.2 mutation list. Used by the analyzer when a locus is
// flagged missing or weak.
func locusRiskAndMutation(name LocusName) (risk, mutation string) {
	switch name {
	case LocusTask:
		return "generic or wrong task", "add an imperative task verb naming the deliverable"
	case LocusRole:
		return "shallow or unstable expertise", "add a role anchor (e.g. \"Ты — senior … strategist\")"
	case LocusAudience:
		return "wrong level, tone, detail", "name the audience (e.g. \"for an investor / for a junior dev\")"
	case LocusContext:
		return "hallucination, generic output", "add a context slot for the user's situation, data, or product"
	case LocusConstraint:
		return "constraint leakage", "add a constraint register (\"must respect: budget, team size, regulation\")"
	case LocusMethod:
		return "unstructured reasoning", "add a method block (e.g. \"step 1 do X, step 2 do Y\")"
	case LocusEpistemic:
		return "assumptions as facts", "add an epistemic protocol (separate facts / assumptions / unknowns)"
	case LocusOutput:
		return "format drift", "add an output schema (numbered sections / JSON / table)"
	case LocusValidation:
		return "unchecked failure", "add a final-check step (\"before answering, verify constraints\")"
	case LocusTool:
		return "tool overuse / underuse", "name allowed tools and the rule for when to call them"
	case LocusMemory:
		return "stale memory, privacy risk", "state what to remember and what to forget"
	case LocusSafety:
		return "unsafe or misleading output", "name the safety boundary (e.g. \"not legal advice\")"
	case LocusUX:
		return "too long, too many questions", "add a length / clarity hint (\"short summary first\")"
	case LocusEvolution:
		return "no learning loop", "add an evolution rule (\"after failure, revise the constraint register\")"
	}
	return "", ""
}
