package promptbio

// Issue 30 v2.7 — Epistemology & Truth Maintenance schemas.
//
// Schemas mirror ingest-done/32-2.7-prompt-dna.md.done sections §3-§5
// verbatim where called for (Claim YAML §4, Belief State YAML §5,
// confidence axes §7, source hierarchy §6). The Go shapes are the
// canonical representation; YAML/JSON emission is downstream.

// ClaimType is the 12-element claim ontology from §4 (source lines 222-449).
type ClaimType string

const (
	ClaimFact           ClaimType = "fact"
	ClaimUserClaim      ClaimType = "user_claim"
	ClaimDocumentClaim  ClaimType = "document_claim"
	ClaimToolResult     ClaimType = "tool_result"
	ClaimAssumption     ClaimType = "assumption"
	ClaimInference      ClaimType = "inference"
	ClaimHypothesis     ClaimType = "hypothesis"
	ClaimRecommendation ClaimType = "recommendation"
	ClaimPreference     ClaimType = "preference"
	ClaimConstraint     ClaimType = "constraint"
	ClaimUnknown        ClaimType = "unknown"
	ClaimDeprecated     ClaimType = "deprecated"
)

var allClaimTypes = []ClaimType{
	ClaimFact, ClaimUserClaim, ClaimDocumentClaim, ClaimToolResult,
	ClaimAssumption, ClaimInference, ClaimHypothesis, ClaimRecommendation,
	ClaimPreference, ClaimConstraint, ClaimUnknown, ClaimDeprecated,
}

// ConfidenceLabel is the §7 qualitative label set. Numeric probabilities
// are explicitly disallowed (anti-pattern §18.6).
type ConfidenceLabel string

const (
	ConfHigh             ConfidenceLabel = "high"
	ConfMedium           ConfidenceLabel = "medium"
	ConfLow              ConfidenceLabel = "low"
	ConfUnknown          ConfidenceLabel = "unknown"
	ConfNeedsVerification ConfidenceLabel = "needs_verification"
)

// ConfidenceAxes are the five orthogonal axes from §7 (source lines 573-646).
type ConfidenceAxes struct {
	Source         ConfidenceLabel `json:"source"`
	Interpretation ConfidenceLabel `json:"interpretation"`
	Inference      ConfidenceLabel `json:"inference"`
	Action         ConfidenceLabel `json:"action"`
	Freshness      ConfidenceLabel `json:"freshness"`
}

// SourceTier is the §6 ten-tier authority ladder. Index 0 is strongest.
type SourceTier string

const (
	SrcCurrentUserCorrection SourceTier = "current_user_correction"
	SrcCurrentUserFact       SourceTier = "current_user_fact"
	SrcVerifiedToolResult    SourceTier = "verified_tool_result"
	SrcAuthoritativeDocument SourceTier = "authoritative_document"
	SrcConfirmedMemory       SourceTier = "confirmed_memory"
	SrcOlderMemory           SourceTier = "older_memory"
	SrcUnverifiedUserClaim   SourceTier = "unverified_user_claim"
	SrcRetrievedSnippet      SourceTier = "retrieved_snippet"
	SrcModelInference        SourceTier = "model_inference"
	SrcAssumption            SourceTier = "assumption"
)

var defaultHierarchy = []SourceTier{
	SrcCurrentUserCorrection, SrcCurrentUserFact, SrcVerifiedToolResult,
	SrcAuthoritativeDocument, SrcConfirmedMemory, SrcOlderMemory,
	SrcUnverifiedUserClaim, SrcRetrievedSnippet, SrcModelInference, SrcAssumption,
}

// EpistemicState is the §11 emitted-claim tagging set (source lines 828-869).
type EpistemicState string

const (
	StateVerified     EpistemicState = "Verified"
	StateSupported    EpistemicState = "Supported"
	StateAssumed      EpistemicState = "Assumed"
	StateHypothetical EpistemicState = "Hypothetical"
	StateSpeculative  EpistemicState = "Speculative"
	StateContradicted EpistemicState = "Contradicted"
	StateStale        EpistemicState = "Stale"
	StateUnknown      EpistemicState = "Unknown"
)

// Claim mirrors the §4 YAML object schema (source lines 457-475) verbatim.
type Claim struct {
	ID                    string          `json:"id"`
	Content               string          `json:"content"`
	Type                  ClaimType       `json:"type"`
	Source                SourceTier      `json:"source"`
	SourceID              string          `json:"source_id,omitempty"`
	Confidence            ConfidenceAxes  `json:"confidence"`
	Scope                 string          `json:"scope,omitempty"`
	Timestamp             string          `json:"timestamp,omitempty"`
	Freshness             ConfidenceLabel `json:"freshness,omitempty"`
	Evidence              []string        `json:"evidence,omitempty"`
	ConflictsWith         []string        `json:"conflicts_with,omitempty"`
	UsedFor               []string        `json:"used_for,omitempty"`
	MemoryEligible        bool            `json:"memory_eligible"`
	RequiresConfirmation  bool            `json:"requires_confirmation"`
	State                 EpistemicState  `json:"state,omitempty"`
}

// BeliefState mirrors the §5 YAML schema (source lines 486-518) verbatim:
// the eight top-level slots are exposed as separated claim ID slices so the
// renderer can group them in the four-pane inspector without re-classifying.
type BeliefState struct {
	Objective      string             `json:"objective"`
	KnownFacts     []string           `json:"known_facts"`
	Constraints    []string           `json:"constraints"`
	Assumptions    []string           `json:"assumptions"`
	Hypotheses     []string           `json:"hypotheses"`
	Unknowns       []string           `json:"unknowns"`
	Contradictions []string           `json:"contradictions"`
	Deprecated     []string           `json:"deprecated"`
	Claims         map[string]*Claim  `json:"claims"`
}

// Recommendation mirrors §10 (source lines 725-733): every recommendation
// declares the claim IDs it depends on so TMS can mark it `needs_revision`
// when a dependency is deprecated or its confidence drops.
type Recommendation struct {
	ID            string   `json:"id"`
	Content       string   `json:"content"`
	DependsOn     []string `json:"depends_on"`
	NeedsRevision bool     `json:"needs_revision"`
	Weak          bool     `json:"weak"`
	Path          []string `json:"path,omitempty"`
}

// EvidenceEdge represents one directed edge in the §9 evidence DAG
// (claim → evidence → inference → recommendation).
type EvidenceEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

// ContradictionType enumerates the §13 nine contradiction kinds
// (source lines 760-770).
type ContradictionType string

const (
	ContradictionFactFact            ContradictionType = "fact_vs_fact"
	ContradictionFactUser            ContradictionType = "fact_vs_user_claim"
	ContradictionUserUser            ContradictionType = "user_claim_vs_user_claim"
	ContradictionUserMemory          ContradictionType = "user_correction_vs_memory"
	ContradictionDocDoc              ContradictionType = "doc_vs_doc"
	ContradictionToolDoc             ContradictionType = "tool_result_vs_doc"
	ContradictionUserInference       ContradictionType = "user_claim_vs_model_inference"
	ContradictionAssumptionEvidence  ContradictionType = "assumption_vs_new_evidence"
	ContradictionRecommendationFact  ContradictionType = "recommendation_vs_new_fact"
)

// Contradiction is one detected conflict + the §13 protocol resolution.
type Contradiction struct {
	ID             string            `json:"id"`
	Type           ContradictionType `json:"type"`
	Severity       string            `json:"severity"`
	ClaimAID       string            `json:"claim_a_id"`
	ClaimBID       string            `json:"claim_b_id"`
	Winner         string            `json:"winner,omitempty"`
	Resolution     string            `json:"resolution"`
	Clarification  string            `json:"clarification_request,omitempty"`
	Affected       []string          `json:"affected_recommendations,omitempty"`
}

// AntiPatternHit is one §18 anti-pattern detection. Each fails the runtime gate.
type AntiPatternHit struct {
	Name    string `json:"name"`
	Section string `json:"section"`
	Detail  string `json:"detail"`
	ClaimID string `json:"claim_id,omitempty"`
}

// GateResult is the §19 runtime gate verdict.
type GateResult struct {
	Pass          bool             `json:"pass"`
	Checks        []GateCheck      `json:"checks"`
	AntiPatterns  []AntiPatternHit `json:"anti_patterns,omitempty"`
	OutputBlock   string           `json:"epistemic_status_block,omitempty"`
	Reason        string           `json:"reason,omitempty"`
}

// GateCheck is one of the ten §19 pre-output checks (source lines 1080-1091).
type GateCheck struct {
	Question string `json:"question"`
	Pass     bool   `json:"pass"`
	Detail   string `json:"detail,omitempty"`
}

// DataSourceKind is the §2 declared input-source taxonomy.
type DataSourceKind string

const (
	DSUserInput DataSourceKind = "user_input"
	DSDocument  DataSourceKind = "document"
	DSTool      DataSourceKind = "tool"
	DSMemory    DataSourceKind = "memory"
	DSWeb       DataSourceKind = "web"
	DSRAG       DataSourceKind = "RAG"
)

// DataSource is one declared input channel for the plan.
type DataSource struct {
	Kind        DataSourceKind `json:"kind"`
	Description string         `json:"description,omitempty"`
}

// ContextItem is one §3 raw_context entry — the unclassified incoming
// text that the gate must triage into the claim ontology before generation.
type ContextItem struct {
	ID         string         `json:"id"`
	Text       string         `json:"text"`
	SourceKind DataSourceKind `json:"source_kind"`
	Timestamp  string         `json:"timestamp,omitempty"`
}

// RiskLevel is the §6 declared risk level for the use case.
type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

// MemoryMode is the §16 memory operating mode.
type MemoryMode string

const (
	MemDisabled  MemoryMode = "disabled"
	MemSession   MemoryMode = "session"
	MemProject   MemoryMode = "project"
	MemLongTerm  MemoryMode = "long_term"
)

// ClaimTypeDescriptor records the four meta-prompt fields per claim type
// (source lines 1666-1671).
type ClaimTypeDescriptor struct {
	Type            ClaimType `json:"type"`
	Definition      string    `json:"definition"`
	Example         string    `json:"example"`
	MemoryEligible  bool      `json:"memory_eligible"`
	OutputHandling  string    `json:"output_handling"`
}

// PromptEpistemologyPlan is the full §28 v2.7 plan output (16 sections).
// BeliefState is an extension over the §28 contract — the plan endpoint
// returns the derived belief_state so the UI can render the four-pane
// inspector without a separate /update round-trip.
type PromptEpistemologyPlan struct {
	BeliefState               BeliefState            `json:"belief_state"`
	EpistemicDiagnosis        EpistemicDiagnosis     `json:"epistemic_diagnosis"`
	ClaimOntology             []ClaimTypeDescriptor  `json:"claim_ontology"`
	ClaimObjectSchema         string                 `json:"claim_object_schema"`
	BeliefStateSchema         string                 `json:"belief_state_schema"`
	SourceHierarchy           []SourceTier           `json:"source_hierarchy"`
	ConfidenceModel           ConfidenceModel        `json:"confidence_model"`
	EvidenceGraphSpec         string                 `json:"evidence_graph_spec"`
	ContradictionProtocol     []string               `json:"contradiction_protocol"`
	BeliefUpdateProtocol      []string               `json:"belief_update_protocol"`
	MemoryTruthPolicy         MemoryTruthPolicy      `json:"memory_truth_policy"`
	ToolTruthPolicy           map[string]string      `json:"tool_truth_policy"`
	EpistemicRuntimeGate      []string               `json:"epistemic_runtime_gate"`
	EpistemicOutputBlock      string                 `json:"epistemic_output_block_template"`
	EpistemicPromptModule     string                 `json:"epistemic_prompt_module"`
	EpistemicPSLBlock         string                 `json:"epistemic_psl_block"`
	EpistemicScore            float64                `json:"epistemic_score"`
	EpistemicScoreBand        string                 `json:"epistemic_score_band"`
	FinalRecommendation       FinalRecommendation    `json:"final_recommendation"`
}

// EpistemicDiagnosis is the first §28 output section.
type EpistemicDiagnosis struct {
	MainEpistemicRisk        string `json:"main_epistemic_risk"`
	ClaimsLikelyConfused     string `json:"claims_likely_confused"`
	RequiredTMSLevel         string `json:"required_truth_maintenance_level"`
}

// ConfidenceModel is the §7 axes + label set + invariants.
type ConfidenceModel struct {
	Axes                       []string          `json:"axes"`
	Labels                     []ConfidenceLabel `json:"labels"`
	NoFakePrecision            bool              `json:"no_fake_precision"`
	ConfidenceRequiresEvidence bool              `json:"confidence_requires_evidence"`
}

// MemoryTruthPolicy is the §16 verbatim seven-rule list plus the write gate.
type MemoryTruthPolicy struct {
	Rules         []string `json:"rules"`
	WriteGate     []string `json:"write_gate_questions"`
}

// FinalRecommendation is the §28 last output section.
type FinalRecommendation struct {
	MostDangerousConfusion string `json:"most_dangerous_claim_type_confusion"`
	FirstRuleToInstall     string `json:"first_truth_maintenance_rule_to_install"`
	FirstTestToRun         string `json:"first_epistemic_test_to_run"`
}

// PlanRequest is the §6 input contract.
type PlanRequest struct {
	PromptSystemDescription string         `json:"prompt_system_description"`
	UseCase                 string         `json:"use_case"`
	DataSources             []DataSource   `json:"data_sources"`
	RiskLevel               RiskLevel      `json:"risk_level"`
	MemoryMode              MemoryMode     `json:"memory_mode"`
	Tools                   []string       `json:"tools"`
	KnownEpistemicRisks     []string       `json:"known_epistemic_risks"`
	RawContext              []ContextItem  `json:"raw_context"`
}

// GateRequest runs the runtime gate over a (belief_state, candidate_output) pair.
type GateRequest struct {
	BeliefState     BeliefState      `json:"belief_state"`
	CandidateOutput string           `json:"candidate_output"`
	Recommendations []Recommendation `json:"recommendations,omitempty"`
	RiskLevel       RiskLevel        `json:"risk_level"`
}

// UpdateRequest applies a new claim through the §14 belief update protocol.
type UpdateRequest struct {
	PriorBeliefState BeliefState      `json:"prior_belief_state"`
	NewClaim         Claim            `json:"new_claim"`
	Recommendations  []Recommendation `json:"recommendations,omitempty"`
}

// UpdateResponse is the new state + the cascade effects.
type UpdateResponse struct {
	NewBeliefState           BeliefState      `json:"new_belief_state"`
	DeprecatedClaims         []string         `json:"deprecated_claims"`
	RecommendationsToRevise  []Recommendation `json:"recommendations_to_revise"`
	Contradictions           []Contradiction  `json:"contradictions"`
}
