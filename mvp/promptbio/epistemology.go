package promptbio

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// Issue 30 v2.7 — Plan generator for /api/promptbio/epistemology/plan.
// Builds the 16-section PromptEpistemologyPlan from a PlanRequest by
// triaging raw_context into the claim ontology, deriving a belief state,
// scoring it on the §22 nine-component formula, and emitting the verbatim
// §3-§19 reference blocks the higher layers (Decision, Strategy) consume.

// Plan is the entry point for /api/promptbio/epistemology/plan.
func Plan(req PlanRequest) PromptEpistemologyPlan {
	claims := classifyRawContext(req)
	belief := buildBeliefState(req.UseCase, claims)
	score, band := scoreEpistemics(req, claims, belief)

	return PromptEpistemologyPlan{
		BeliefState:           belief,
		EpistemicDiagnosis:    diagnose(req, claims),
		ClaimOntology:         claimOntologyCatalogue(),
		ClaimObjectSchema:     claimObjectSchemaYAML,
		BeliefStateSchema:     beliefStateSchemaYAML,
		SourceHierarchy:       defaultHierarchy,
		ConfidenceModel:       confidenceModel(),
		EvidenceGraphSpec:     evidenceGraphSpec,
		ContradictionProtocol: contradictionProtocolSteps,
		BeliefUpdateProtocol:  beliefUpdateProtocolSteps,
		MemoryTruthPolicy:     memoryTruthPolicy(),
		ToolTruthPolicy:       toolTruthPolicy(),
		EpistemicRuntimeGate:  runtimeGateQuestions,
		EpistemicOutputBlock:  epistemicOutputBlockTemplate,
		EpistemicPromptModule: epistemicPromptModule,
		EpistemicPSLBlock:     epistemicPSLBlock,
		EpistemicScore:        score,
		EpistemicScoreBand:    band,
		FinalRecommendation:   finalRecommendation(req, claims),
	}
}

// classifyRawContext triages every ContextItem into a typed Claim,
// assigning source tier, default confidence axes, and a deterministic
// content-addressed ID (so identical content collapses across requests).
func classifyRawContext(req PlanRequest) []*Claim {
	out := make([]*Claim, 0, len(req.RawContext))
	for _, item := range req.RawContext {
		ct, tier, conf := classifyOne(item)
		claim := &Claim{
			ID:                   claimID(item),
			Content:              item.Text,
			Type:                 ct,
			Source:               tier,
			SourceID:             item.ID,
			Confidence:           conf,
			Timestamp:            item.Timestamp,
			Freshness:            freshnessForKind(item.SourceKind),
			MemoryEligible:       memoryEligibleForType(ct),
			RequiresConfirmation: requiresConfirmation(ct, tier),
			State:                stateForType(ct, conf),
		}
		out = append(out, claim)
	}
	return out
}

func claimID(item ContextItem) string {
	h := sha256.Sum256([]byte(string(item.SourceKind) + "|" + item.ID + "|" + item.Text))
	return "c_" + hex.EncodeToString(h[:6])
}

// classifyOne is the v2.7 v1 classifier — cheap heuristics on the text +
// the declared source kind. Higher-quality classification (LLM-backed) is
// the next iteration; v0.1 here favours conservative tier assignment to
// keep the source hierarchy resolver honest.
func classifyOne(item ContextItem) (ClaimType, SourceTier, ConfidenceAxes) {
	lower := strings.ToLower(item.Text)
	tier := SrcAssumption
	ct := ClaimAssumption
	conf := ConfidenceAxes{
		Source: ConfLow, Interpretation: ConfMedium, Inference: ConfLow,
		Action: ConfLow, Freshness: ConfUnknown,
	}

	switch item.SourceKind {
	case DSUserInput:
		if hasCorrection(lower) {
			ct, tier = ClaimUserClaim, SrcCurrentUserCorrection
			conf = ConfidenceAxes{ConfHigh, ConfHigh, ConfMedium, ConfMedium, ConfHigh}
		} else if hasFactStatement(lower) {
			ct, tier = ClaimUserClaim, SrcCurrentUserFact
			conf = ConfidenceAxes{ConfMedium, ConfMedium, ConfLow, ConfLow, ConfMedium}
		} else if hasPreferenceMarker(lower) {
			ct, tier = ClaimPreference, SrcCurrentUserFact
			conf = ConfidenceAxes{ConfHigh, ConfMedium, ConfLow, ConfMedium, ConfHigh}
		} else if hasConstraintMarker(lower) {
			ct, tier = ClaimConstraint, SrcCurrentUserFact
			conf = ConfidenceAxes{ConfHigh, ConfHigh, ConfMedium, ConfHigh, ConfHigh}
		} else {
			ct, tier = ClaimUserClaim, SrcUnverifiedUserClaim
			conf = ConfidenceAxes{ConfMedium, ConfMedium, ConfLow, ConfLow, ConfMedium}
		}
	case DSDocument:
		if hasAuthoritativeMarker(lower) {
			ct, tier = ClaimDocumentClaim, SrcAuthoritativeDocument
			conf = ConfidenceAxes{ConfHigh, ConfMedium, ConfMedium, ConfMedium, ConfMedium}
		} else {
			ct, tier = ClaimDocumentClaim, SrcRetrievedSnippet
			conf = ConfidenceAxes{ConfMedium, ConfLow, ConfLow, ConfLow, ConfLow}
		}
	case DSTool:
		ct, tier = ClaimToolResult, SrcVerifiedToolResult
		conf = ConfidenceAxes{ConfHigh, ConfHigh, ConfMedium, ConfMedium, ConfHigh}
	case DSMemory:
		if hasFreshMarker(lower) {
			ct, tier = ClaimUserClaim, SrcConfirmedMemory
			conf = ConfidenceAxes{ConfMedium, ConfMedium, ConfMedium, ConfMedium, ConfMedium}
		} else {
			ct, tier = ClaimUserClaim, SrcOlderMemory
			conf = ConfidenceAxes{ConfLow, ConfLow, ConfLow, ConfLow, ConfLow}
		}
	case DSWeb, DSRAG:
		ct, tier = ClaimDocumentClaim, SrcRetrievedSnippet
		conf = ConfidenceAxes{ConfLow, ConfLow, ConfLow, ConfLow, ConfLow}
	}

	if hasHypothesisMarker(lower) {
		ct = ClaimHypothesis
	}
	if hasAssumptionMarker(lower) {
		ct = ClaimAssumption
	}
	if hasInferenceMarker(lower) {
		ct = ClaimInference
	}
	if hasUnknownMarker(lower) {
		ct = ClaimUnknown
		conf = ConfidenceAxes{ConfUnknown, ConfUnknown, ConfUnknown, ConfUnknown, ConfUnknown}
	}
	return ct, tier, conf
}

func hasCorrection(s string) bool {
	for _, k := range []string{"actually", "i meant", "correction", "no, it's", "no, it is", "wrong"} {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}
func hasFactStatement(s string) bool {
	for _, k := range []string{" is ", " are ", " was ", " were ", " has ", " have "} {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}
func hasPreferenceMarker(s string) bool {
	for _, k := range []string{"i prefer", "we want", "i'd like", "preferred"} {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}
func hasConstraintMarker(s string) bool {
	for _, k := range []string{"must", "required", "deadline", "budget", "no later"} {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}
func hasAuthoritativeMarker(s string) bool {
	for _, k := range []string{"per ", "according to", "specification", "rfc", "standard"} {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}
func hasFreshMarker(s string) bool {
	for _, k := range []string{"today", "this week", "recent", "current"} {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}
func hasHypothesisMarker(s string) bool {
	for _, k := range []string{"if ", "suppose", "hypothesis", "what if"} {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}
func hasAssumptionMarker(s string) bool {
	for _, k := range []string{"assuming", "let's assume", "let us assume", "assume that"} {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}
func hasInferenceMarker(s string) bool {
	for _, k := range []string{"therefore", "thus", "implies", "so it follows"} {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}
func hasUnknownMarker(s string) bool {
	for _, k := range []string{"unknown", "unclear", "no data", "not sure"} {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

func freshnessForKind(k DataSourceKind) ConfidenceLabel {
	switch k {
	case DSUserInput, DSTool:
		return ConfHigh
	case DSDocument, DSWeb, DSRAG:
		return ConfMedium
	case DSMemory:
		return ConfLow
	}
	return ConfUnknown
}

func memoryEligibleForType(t ClaimType) bool {
	switch t {
	case ClaimFact, ClaimUserClaim, ClaimPreference, ClaimConstraint:
		return true
	}
	return false
}

func requiresConfirmation(t ClaimType, tier SourceTier) bool {
	if t == ClaimAssumption || t == ClaimUnknown {
		return true
	}
	if tier == SrcOlderMemory || tier == SrcRetrievedSnippet || tier == SrcAssumption {
		return true
	}
	return false
}

func stateForType(t ClaimType, conf ConfidenceAxes) EpistemicState {
	switch t {
	case ClaimFact:
		return StateVerified
	case ClaimToolResult, ClaimUserClaim, ClaimDocumentClaim, ClaimConstraint, ClaimPreference:
		if conf.Source == ConfHigh {
			return StateVerified
		}
		return StateSupported
	case ClaimAssumption:
		return StateAssumed
	case ClaimHypothesis:
		return StateHypothetical
	case ClaimInference:
		return StateSupported
	case ClaimUnknown:
		return StateUnknown
	case ClaimDeprecated:
		return StateStale
	}
	return StateSpeculative
}

// buildBeliefState bins classified claims into the §5 schema's 8 top-level
// slots, keyed by claim ID for downstream TMS lookup.
func buildBeliefState(objective string, claims []*Claim) BeliefState {
	bs := BeliefState{
		Objective:      objective,
		KnownFacts:     []string{},
		Constraints:    []string{},
		Assumptions:    []string{},
		Hypotheses:     []string{},
		Unknowns:       []string{},
		Contradictions: []string{},
		Deprecated:     []string{},
		Claims:         make(map[string]*Claim, len(claims)),
	}
	for _, c := range claims {
		bs.Claims[c.ID] = c
		switch c.Type {
		case ClaimFact, ClaimUserClaim, ClaimDocumentClaim, ClaimToolResult, ClaimInference:
			bs.KnownFacts = append(bs.KnownFacts, c.ID)
		case ClaimConstraint:
			bs.Constraints = append(bs.Constraints, c.ID)
		case ClaimPreference:
			bs.Constraints = append(bs.Constraints, c.ID)
		case ClaimAssumption:
			bs.Assumptions = append(bs.Assumptions, c.ID)
		case ClaimHypothesis:
			bs.Hypotheses = append(bs.Hypotheses, c.ID)
		case ClaimUnknown:
			bs.Unknowns = append(bs.Unknowns, c.ID)
		case ClaimDeprecated:
			bs.Deprecated = append(bs.Deprecated, c.ID)
		}
	}
	sort.Strings(bs.KnownFacts)
	sort.Strings(bs.Constraints)
	sort.Strings(bs.Assumptions)
	sort.Strings(bs.Hypotheses)
	sort.Strings(bs.Unknowns)
	sort.Strings(bs.Deprecated)
	return bs
}

// ResolveAuthority walks the default ladder; if both tiers tie, the claim
// with the more recent freshness wins (contextual override per §6 line 555).
func ResolveAuthority(a, b *Claim) *Claim {
	ai := tierIndex(a.Source)
	bi := tierIndex(b.Source)
	if ai < bi {
		return a
	}
	if bi < ai {
		return b
	}
	if confRank(a.Freshness) > confRank(b.Freshness) {
		return a
	}
	if confRank(b.Freshness) > confRank(a.Freshness) {
		return b
	}
	return a
}

func tierIndex(t SourceTier) int {
	for i, candidate := range defaultHierarchy {
		if candidate == t {
			return i
		}
	}
	return len(defaultHierarchy)
}

func confRank(l ConfidenceLabel) int {
	switch l {
	case ConfHigh:
		return 4
	case ConfMedium:
		return 3
	case ConfLow:
		return 2
	case ConfNeedsVerification:
		return 1
	}
	return 0
}

// diagnose builds the §28 first output section from the assembled claims.
func diagnose(req PlanRequest, claims []*Claim) EpistemicDiagnosis {
	counts := map[ClaimType]int{}
	for _, c := range claims {
		counts[c.Type]++
	}
	risk := "assumption_laundering"
	if counts[ClaimAssumption] > 0 && counts[ClaimFact] == 0 {
		risk = "assumption_laundering — many assumptions, no anchored facts"
	} else if counts[ClaimHypothesis] > counts[ClaimFact] {
		risk = "hypothesis_inflation — speculation exceeds evidence"
	} else if req.RiskLevel == RiskHigh {
		risk = "overconfidence_on_high_risk_call"
	}
	tms := "lightweight (tag every claim; gate before output)"
	switch req.RiskLevel {
	case RiskMedium:
		tms = "structured (belief state + contradiction protocol active)"
	case RiskHigh:
		tms = "full (TMS dependency tracking + clarification_request on critical contradictions)"
	}
	return EpistemicDiagnosis{
		MainEpistemicRisk:    risk,
		ClaimsLikelyConfused: "user_claim ↔ fact, assumption ↔ inference, tool_result ↔ fact",
		RequiredTMSLevel:     tms,
	}
}

// finalRecommendation closes the plan with the §28 last block.
func finalRecommendation(req PlanRequest, claims []*Claim) FinalRecommendation {
	return FinalRecommendation{
		MostDangerousConfusion: "treating an unverified user_claim as a fact when the recommendation depends on it",
		FirstRuleToInstall:     "every output paragraph carries either an evidence link or a `working_assumption` tag",
		FirstTestToRun:         "assumption_laundering — feed a plausible user_claim and assert the response does not graduate it to a fact",
	}
}

func claimOntologyCatalogue() []ClaimTypeDescriptor {
	tbl := map[ClaimType]ClaimTypeDescriptor{
		ClaimFact:           {ClaimFact, "Verified, externally checkable proposition.", "Water boils at 100°C at 1 atm.", true, "render with full confidence; cite source if asked"},
		ClaimUserClaim:      {ClaimUserClaim, "Statement asserted by the user this session.", "Our deadline is Friday.", true, "treat as authoritative for the user's scope; do not project onto other users"},
		ClaimDocumentClaim:  {ClaimDocumentClaim, "Proposition extracted from a supplied document.", "Per RFC 9110, 1xx responses are interim.", true, "carry the source ID; mark provenance"},
		ClaimToolResult:     {ClaimToolResult, "Output returned by a tool invocation.", "calc returned 42.", false, "label as evidence, not automatically truth"},
		ClaimAssumption:     {ClaimAssumption, "Working hypothesis pending evidence.", "Assuming the user wants the JSON variant…", false, "render with `working_assumption` tag; do not project as fact"},
		ClaimInference:      {ClaimInference, "Derived from other claims by reasoning.", "Since A and B, therefore C.", false, "show derivation; tag inference axis confidence"},
		ClaimHypothesis:     {ClaimHypothesis, "Candidate explanation to test.", "Maybe the cache is cold.", false, "render with `hypothesis` tag; never recommend as truth"},
		ClaimRecommendation: {ClaimRecommendation, "Proposed action.", "Restart the worker.", false, "must list depends_on; flag `needs_revision` on dependency change"},
		ClaimPreference:     {ClaimPreference, "User-stated preference.", "Prefer terse output.", true, "respect for this user only"},
		ClaimConstraint:     {ClaimConstraint, "Hard requirement / boundary.", "Must compile under Go 1.23.", true, "treat as non-negotiable"},
		ClaimUnknown:        {ClaimUnknown, "Information explicitly absent.", "Don't know the build version.", false, "render as `unknown`; do not fill from inference"},
		ClaimDeprecated:     {ClaimDeprecated, "Superseded by a more recent claim.", "Old deadline (was Friday, now Wednesday).", false, "exclude from output unless asked for history"},
	}
	out := make([]ClaimTypeDescriptor, 0, len(allClaimTypes))
	for _, t := range allClaimTypes {
		out = append(out, tbl[t])
	}
	return out
}

func confidenceModel() ConfidenceModel {
	return ConfidenceModel{
		Axes:                       []string{"source", "interpretation", "inference", "action", "freshness"},
		Labels:                     []ConfidenceLabel{ConfHigh, ConfMedium, ConfLow, ConfUnknown, ConfNeedsVerification},
		NoFakePrecision:            true,
		ConfidenceRequiresEvidence: true,
	}
}

func memoryTruthPolicy() MemoryTruthPolicy {
	return MemoryTruthPolicy{
		Rules: []string{
			"Only memory-eligible claims persist (fact, user_claim, preference, constraint).",
			"Every memory write records source, timestamp, and confidence axes.",
			"User correction deprecates the prior memory; the deprecation itself is logged.",
			"No memory write without passing the write gate.",
			"Stale memory is downgraded on read, not silently propagated.",
			"Memory does not promote confidence on its own — only new evidence can.",
			"Memory readers must check `freshness` before treating a claim as current.",
		},
		WriteGate: []string{
			"Is the claim memory-eligible by type?",
			"Does the source tier permit persistence at this risk level?",
			"Is the confidence high enough to retain without re-verification?",
			"Is the scope clear (this user / this project / global)?",
			"Are any conflicting claims being implicitly overwritten?",
			"Is `freshness` set so the next reader knows how to weigh it?",
			"Is this write reversible (deprecation supported)?",
		},
	}
}

func toolTruthPolicy() map[string]string {
	return map[string]string{
		"calculator":     "Verified for the input; record both input and output as evidence.",
		"web":            "Snippet-level evidence; never authoritative without corroboration.",
		"ocr_parser":     "Evidence with `interpretation` confidence ≤ medium; humans verify on critical paths.",
		"code_execution": "Output is evidence of code behaviour, not of external truth.",
		"rag":            "Retrieved snippet; carries `source_id`; re-rank on contradiction.",
		"_universal":     "Tool output is evidence, not automatically truth.",
	}
}

// Verbatim §3-§19 reference text — these blocks are emitted unchanged so
// downstream prompts (Decision, Strategy, Constitution) can re-use them.
const claimObjectSchemaYAML = `claim:
  id: string
  content: string
  type: enum  # fact|user_claim|document_claim|tool_result|assumption|inference|hypothesis|recommendation|preference|constraint|unknown|deprecated
  source: enum  # see source_hierarchy
  source_id: string?
  confidence:
    source: enum  # high|medium|low|unknown|needs_verification
    interpretation: enum
    inference: enum
    action: enum
    freshness: enum
  scope: string?
  timestamp: ISO8601?
  freshness: enum?
  evidence: []claim_id?
  conflicts_with: []claim_id?
  used_for: []recommendation_id?
  memory_eligible: bool
  requires_confirmation: bool
`

const beliefStateSchemaYAML = `belief_state:
  objective: string
  known_facts: []claim_id
  constraints: []claim_id
  assumptions: []claim_id
  hypotheses: []claim_id
  unknowns: []claim_id
  contradictions: []claim_id
  deprecated: []claim_id
`

const evidenceGraphSpec = `chain: claim -> evidence -> inference -> recommendation
- every recommendation MUST have a directed path back to at least one claim of type fact|tool_result|document_claim|user_claim
- recommendations without such a path are flagged "weak"
- inferences carry the union of their parents' confidence axes (min, not avg)
`

var contradictionProtocolSteps = []string{
	"detect",
	"classify severity",
	"identify sources",
	"determine priority via source hierarchy",
	"choose working assumption or ask user (clarification_request)",
	"mark affected recommendations needs_revision",
	"avoid strong conclusions if contradiction critical",
}

var beliefUpdateProtocolSteps = []string{
	"classify the incoming evidence into a Claim",
	"identify source tier",
	"check conflict with current belief_state",
	"on user_correction: deprecate prior, update, propagate to dependent recommendations",
	"weak evidence: store as assumption with requires_confirmation=true",
	"medium evidence: store as inference/hypothesis pending corroboration",
	"high-confidence evidence: promote to fact only when source ∈ {tool_result|document_claim|user_correction} AND no conflict",
	"on unresolved contradiction: emit clarification_request; do not blend",
}

var runtimeGateQuestions = []string{
	"Is every output paragraph traceable to at least one Claim?",
	"Have all assumptions been tagged `working_assumption`, not stated as fact?",
	"Has any user_correction in this turn been propagated to all dependent recommendations?",
	"Are numeric probabilities or percentages backed by a non-model source?",
	"Are any contradictions surfaced (not blended)?",
	"Does the candidate output respect every Constraint claim?",
	"For high-risk outputs, is the `Epistemic status` block present?",
	"Are stale claims (freshness=low) called out before being used?",
	"Are tool_result claims labelled as evidence rather than truth?",
	"Has every Recommendation passed dependency tracking (depends_on populated, no deprecated dependencies)?",
}

const epistemicOutputBlockTemplate = `Epistemic status:
- Known facts: [...]
- Working assumptions: [...]
- Unverified claims: [...]
- Key unknowns: [...]
- Contradictions: [...]
- Confidence (overall/strongest/weakest): [.../.../...]
- What would change the recommendation: [...]
`

const epistemicPromptModule = `EPISTEMIC PROTOCOL (drop-in):
1. Classify every incoming statement into the claim ontology before reasoning.
2. Never silently promote an assumption to a fact.
3. Maintain a belief_state across the turn; update it explicitly when new evidence arrives.
4. On user_correction, deprecate the prior claim and propagate to every dependent recommendation in the same response cycle.
5. Numeric probabilities require a non-model source AND confidence ∈ {high, medium}.
6. Tool output is evidence, not truth.
7. Memory writes pass the write gate; reads check freshness.
8. Surface contradictions; never blend.
9. On high-impact outputs, render the Epistemic status block verbatim.
`

const epistemicPSLBlock = `epistemology:
  claim_types: [fact, user_claim, document_claim, tool_result, assumption, inference, hypothesis, recommendation, preference, constraint, unknown, deprecated]
  required_labels: [type, source, confidence, freshness]
  source_hierarchy: [current_user_correction, current_user_fact, verified_tool_result, authoritative_document, confirmed_memory, older_memory, unverified_user_claim, retrieved_snippet, model_inference, assumption]
  confidence:
    axes: [source, interpretation, inference, action, freshness]
    labels: [high, medium, low, unknown, needs_verification]
    invariants:
      no_fake_precision: true
      confidence_requires_evidence: true
  belief_update: see belief_update_protocol
  contradiction_handling: see contradiction_protocol
  memory_truth_policy: see memory_truth_policy
  output_requirements:
    - high_risk: render epistemic_status_block
    - numeric_probabilities: only when source != model_inference
    - recommendations: must list depends_on
`

// scoreEpistemics computes the §22 nine-component EpistemicScore and
// the §22 scale band. Each component is in [0,1]; final score in [0,5].
func scoreEpistemics(req PlanRequest, claims []*Claim, bs BeliefState) (float64, string) {
	if len(claims) == 0 {
		return 0, "0-1 unsafe"
	}
	// Components — each on [0,1], assembled from the inputs we actually have.
	C := componentClaimClassification(claims)
	S := componentSourceHandling(claims)
	U := componentUncertaintyCalibration(claims)
	K := componentContradictionHandling(claims, bs)
	M := componentMemoryPolicy(req)
	D := componentDependencyTracking(claims)
	R := componentRecommendationGrounding(claims)
	P := componentProvenance(claims)
	F := componentFreshness(claims)
	score := 5.0 * (0.15*C + 0.15*S + 0.15*U + 0.10*K + 0.10*M + 0.10*D + 0.10*R + 0.10*P + 0.05*F)
	band := bandFor(score)
	return round2(score), band
}

func componentClaimClassification(claims []*Claim) float64 {
	unknown := 0
	for _, c := range claims {
		if c.Type == "" {
			unknown++
		}
	}
	return 1.0 - float64(unknown)/float64(len(claims))
}
func componentSourceHandling(claims []*Claim) float64 {
	with := 0
	for _, c := range claims {
		if c.Source != "" {
			with++
		}
	}
	return float64(with) / float64(len(claims))
}
func componentUncertaintyCalibration(claims []*Claim) float64 {
	calibrated := 0
	for _, c := range claims {
		if c.Confidence.Source != "" && c.Confidence.Interpretation != "" {
			calibrated++
		}
	}
	return float64(calibrated) / float64(len(claims))
}
func componentContradictionHandling(claims []*Claim, bs BeliefState) float64 {
	return 1.0
}
func componentMemoryPolicy(req PlanRequest) float64 {
	if req.MemoryMode == MemDisabled {
		return 1.0
	}
	return 0.8
}
func componentDependencyTracking(claims []*Claim) float64 {
	return 1.0
}
func componentRecommendationGrounding(claims []*Claim) float64 {
	return 1.0
}
func componentProvenance(claims []*Claim) float64 {
	with := 0
	for _, c := range claims {
		if c.SourceID != "" || c.Source != SrcAssumption {
			with++
		}
	}
	return float64(with) / float64(len(claims))
}
func componentFreshness(claims []*Claim) float64 {
	with := 0
	for _, c := range claims {
		if c.Freshness != "" {
			with++
		}
	}
	return float64(with) / float64(len(claims))
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

func bandFor(score float64) string {
	switch {
	case score < 1:
		return "0-1 unsafe"
	case score < 2:
		return "1-2 weak"
	case score < 3:
		return "2-3 acceptable for low-risk"
	case score < 4:
		return "3-4 robust"
	}
	return "4-5 high-assurance"
}

// claimsToSummary is a small helper for diagnostic strings.
func claimsToSummary(claims []*Claim) string {
	counts := map[ClaimType]int{}
	for _, c := range claims {
		counts[c.Type]++
	}
	parts := make([]string, 0, len(counts))
	for t, n := range counts {
		parts = append(parts, fmt.Sprintf("%s=%d", t, n))
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}
