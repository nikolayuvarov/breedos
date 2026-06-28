package promptbio

import (
	"encoding/json"
	"strings"
	"testing"
)

// Issue 30 v2.7 test battery — §24 (source lines 1294-1428) plus the two
// additional tests the meta-prompt mandates at lines 1755-1756
// (stale_source, unsupported_market_claim). Acceptance criterion: ≥9/10 pass.

// Test 1: assumption_laundering — a plausible user_claim must not graduate
// into a fact in the output, and the gate must catch it when it does.
func TestEpi_AssumptionLaundering(t *testing.T) {
	bs := classifyAndBuild("plan", []ContextItem{
		{ID: "1", Text: "Assuming the user wants the Pro tier.", SourceKind: DSUserInput},
	})
	gate := RunGate(GateRequest{
		BeliefState:     bs,
		CandidateOutput: "The user wants the Pro tier. We will ship it.",
		RiskLevel:       RiskMedium,
	})
	if gate.Pass {
		t.Fatal("gate must FAIL: assumption surfaced in output without working_assumption tag")
	}
	if !hasAntiPattern(gate, "assumption_laundering") {
		t.Fatal("expected assumption_laundering anti-pattern hit")
	}
}

// Test 2: user_correction — must deprecate prior and mark dependent
// recommendations as needs_revision in the same response cycle.
func TestEpi_UserCorrection(t *testing.T) {
	bs := classifyAndBuild("plan", []ContextItem{
		{ID: "1", Text: "Deadline is Friday.", SourceKind: DSMemory, Timestamp: "2026-01-01"},
	})
	priorClaimID := ""
	for id := range bs.Claims {
		priorClaimID = id
		break
	}
	recs := []Recommendation{{ID: "r1", Content: "Ship by Friday.", DependsOn: []string{priorClaimID}}}

	upd := ApplyUpdate(UpdateRequest{
		PriorBeliefState: bs,
		NewClaim: Claim{
			Content: "Actually deadline is Wednesday.",
			Type:    ClaimUserClaim,
			Source:  SrcCurrentUserCorrection,
			Confidence: ConfidenceAxes{
				Source: ConfHigh, Interpretation: ConfHigh, Inference: ConfMedium, Action: ConfMedium, Freshness: ConfHigh,
			},
		},
		Recommendations: recs,
	})
	if len(upd.DeprecatedClaims) == 0 {
		t.Fatal("user_correction must deprecate prior claim")
	}
	if len(upd.RecommendationsToRevise) == 0 || !upd.RecommendationsToRevise[0].NeedsRevision {
		t.Fatal("dependent recommendation must be flagged needs_revision")
	}
}

// Test 3: conflict_handling — two equally-authoritative claims surface as
// contradiction; neither is silently blended.
func TestEpi_ConflictHandling(t *testing.T) {
	bs := classifyAndBuild("plan", []ContextItem{
		{ID: "1", Text: "Price is $99.", SourceKind: DSUserInput},
	})
	upd := ApplyUpdate(UpdateRequest{
		PriorBeliefState: bs,
		NewClaim: Claim{
			Content: "Price is $149.",
			Type:    ClaimUserClaim,
			Source:  SrcCurrentUserFact,
			Confidence: ConfidenceAxes{Source: ConfHigh, Interpretation: ConfHigh, Inference: ConfMedium, Action: ConfMedium, Freshness: ConfHigh},
		},
	})
	if len(upd.Contradictions) == 0 {
		t.Fatal("equal-authority conflicting claims must surface as contradiction")
	}
}

// Test 4: fake_precision — numeric probability without a non-model source
// at high/medium confidence is flagged by the gate.
func TestEpi_FakePrecision(t *testing.T) {
	bs := classifyAndBuild("plan", []ContextItem{
		{ID: "1", Text: "Pricing rumours.", SourceKind: DSWeb},
	})
	gate := RunGate(GateRequest{
		BeliefState:     bs,
		CandidateOutput: "There is a 73% probability the user wants Pro tier.",
		RiskLevel:       RiskMedium,
	})
	if gate.Pass {
		t.Fatal("gate must FAIL: 73% probability with only web-snippet evidence")
	}
	if !hasAntiPattern(gate, "fake_precision") {
		t.Fatal("expected fake_precision anti-pattern hit")
	}
}

// Test 5: tool_overtrust — tool_result rendered with authoritative
// language (no `evidence` marker) triggers the §18.8 detector.
func TestEpi_ToolOvertrust(t *testing.T) {
	bs := classifyAndBuild("plan", []ContextItem{
		{ID: "1", Text: "calc returned 42.", SourceKind: DSTool},
	})
	gate := RunGate(GateRequest{
		BeliefState:     bs,
		CandidateOutput: "The answer is 42. Epistemic status: …",
		RiskLevel:       RiskMedium,
	})
	if !hasAntiPattern(gate, "tool_overtrust") {
		t.Fatal("expected tool_overtrust anti-pattern hit")
	}
}

// Test 6: memory_fossilization — older_memory without freshness label.
func TestEpi_MemoryFossilization(t *testing.T) {
	bs := classifyAndBuild("plan", []ContextItem{
		{ID: "1", Text: "Plan from Q3 was launch in October.", SourceKind: DSMemory},
	})
	// Strip freshness on the memory claim to simulate an unlabelled fossil.
	for _, c := range bs.Claims {
		c.Source = SrcOlderMemory
		c.Freshness = ""
	}
	gate := RunGate(GateRequest{
		BeliefState:     bs,
		CandidateOutput: "Per the plan, we launch in October. Epistemic status: …",
		RiskLevel:       RiskMedium,
	})
	if !hasAntiPattern(gate, "memory_fossilization") {
		t.Fatal("expected memory_fossilization anti-pattern hit")
	}
}

// Test 7: citation_laundering — quoted phrase without source attribution.
func TestEpi_CitationLaundering(t *testing.T) {
	bs := classifyAndBuild("plan", []ContextItem{
		{ID: "1", Text: "Some background.", SourceKind: DSDocument},
	})
	gate := RunGate(GateRequest{
		BeliefState:     bs,
		CandidateOutput: `Industry experts say: "the market will double by 2027 across all segments". Epistemic status: …`,
		RiskLevel:       RiskMedium,
	})
	if !hasAntiPattern(gate, "citation_laundering") {
		t.Fatal("expected citation_laundering anti-pattern hit")
	}
}

// Test 8: recommendation_dependency — every Recommendation lists depends_on;
// when a dependency deprecates, the recommendation is flagged.
func TestEpi_RecommendationDependency(t *testing.T) {
	bs := classifyAndBuild("plan", []ContextItem{
		{ID: "1", Text: "Customer prefers email.", SourceKind: DSUserInput},
	})
	priorID := ""
	for id := range bs.Claims {
		priorID = id
		break
	}
	recs := []Recommendation{{ID: "r1", Content: "Notify by email.", DependsOn: []string{priorID}}}
	upd := ApplyUpdate(UpdateRequest{
		PriorBeliefState: bs,
		NewClaim: Claim{
			Content:    "Customer prefers Slack instead.",
			Type:       ClaimUserClaim,
			Source:     SrcCurrentUserCorrection,
			Confidence: ConfidenceAxes{Source: ConfHigh, Interpretation: ConfHigh, Inference: ConfMedium, Action: ConfMedium, Freshness: ConfHigh},
		},
		Recommendations: recs,
	})
	if len(upd.RecommendationsToRevise) == 0 || !upd.RecommendationsToRevise[0].NeedsRevision {
		t.Fatal("recommendation depending on a deprecated claim must be flagged needs_revision")
	}
}

// Test 9: stale_source — claim with low freshness is gated until refreshed.
func TestEpi_StaleSource(t *testing.T) {
	bs := classifyAndBuild("plan", []ContextItem{
		{ID: "1", Text: "Old plan said October launch.", SourceKind: DSMemory},
	})
	for _, c := range bs.Claims {
		c.State = StateStale
	}
	gate := RunGate(GateRequest{
		BeliefState:     bs,
		CandidateOutput: "We launch in October per plan. Epistemic status: …",
		RiskLevel:       RiskMedium,
	})
	// Either the stale-claims check or the memory_fossilization detector should fire.
	if gate.Pass {
		t.Fatal("gate must FAIL: stale claim used without freshness call-out")
	}
}

// Test 10: unsupported_market_claim — numeric market projection without
// authoritative source is the canonical §27 GTM failure mode.
func TestEpi_UnsupportedMarketClaim(t *testing.T) {
	bs := classifyAndBuild("plan", []ContextItem{
		{ID: "1", Text: "TAM speculation in a blog post.", SourceKind: DSWeb},
	})
	gate := RunGate(GateRequest{
		BeliefState:     bs,
		CandidateOutput: "TAM will reach $12B by 2027, growing 28% YoY. Epistemic status: …",
		RiskLevel:       RiskMedium,
	})
	if gate.Pass {
		t.Fatal("gate must FAIL: market-size numeric claim from web-snippet only")
	}
	if !hasAntiPattern(gate, "fake_precision") {
		t.Fatal("expected fake_precision hit on $12B / 28% YoY")
	}
}

// — Supporting acceptance criteria, not part of the 10-test battery —

func TestEpi_ConfidenceNeverInflatesWithoutEvidence(t *testing.T) {
	bs := classifyAndBuild("plan", []ContextItem{
		{ID: "1", Text: "Deadline is Friday.", SourceKind: DSUserInput},
	})
	priorConf := ConfidenceAxes{}
	for _, c := range bs.Claims {
		priorConf = c.Confidence
	}
	upd := ApplyUpdate(UpdateRequest{
		PriorBeliefState: bs,
		NewClaim: Claim{
			Content:    "Deadline is Friday.",
			Type:       ClaimFact,
			Source:     SrcModelInference,
			Confidence: ConfidenceAxes{Source: ConfHigh, Interpretation: ConfHigh, Inference: ConfHigh, Action: ConfHigh, Freshness: ConfHigh},
			Evidence:   nil,
		},
	})
	// New claim's confidence must NOT be higher than prior since no evidence was attached.
	var newClaim *Claim
	for _, c := range upd.NewBeliefState.Claims {
		if c.Content == "Deadline is Friday." && c.Source == SrcModelInference {
			newClaim = c
		}
	}
	if newClaim == nil {
		t.Skip("did not find injected claim in belief state")
	}
	if confRank(newClaim.Confidence.Source) > confRank(priorConf.Source) {
		t.Fatalf("confidence inflated without evidence: %s → %s", priorConf.Source, newClaim.Confidence.Source)
	}
}

func TestEpi_HighRiskRequiresStatusBlock(t *testing.T) {
	bs := classifyAndBuild("plan", []ContextItem{
		{ID: "1", Text: "Customer wants Pro.", SourceKind: DSUserInput},
	})
	gate := RunGate(GateRequest{
		BeliefState:     bs,
		CandidateOutput: "Ship Pro tier.",
		RiskLevel:       RiskHigh,
	})
	if gate.Pass {
		t.Fatal("high-risk output without Epistemic status block must FAIL")
	}
}

func TestEpi_PlanReturnsAllSixteenSections(t *testing.T) {
	plan := Plan(PlanRequest{
		UseCase:    "GTM strategy",
		RiskLevel:  RiskMedium,
		MemoryMode: MemSession,
		RawContext: []ContextItem{
			{ID: "1", Text: "Customer churn is 4%.", SourceKind: DSDocument},
			{ID: "2", Text: "We must ship by Friday.", SourceKind: DSUserInput},
		},
	})
	if plan.EpistemicScore <= 0 {
		t.Fatal("score must be > 0 for non-empty input")
	}
	if len(plan.ClaimOntology) != 12 {
		t.Fatalf("expected 12 claim types, got %d", len(plan.ClaimOntology))
	}
	if len(plan.SourceHierarchy) != 10 {
		t.Fatalf("expected 10 source tiers, got %d", len(plan.SourceHierarchy))
	}
	if len(plan.EpistemicRuntimeGate) != 10 {
		t.Fatalf("expected 10 gate questions, got %d", len(plan.EpistemicRuntimeGate))
	}
	if len(plan.ContradictionProtocol) != 7 {
		t.Fatalf("expected 7-step contradiction protocol, got %d", len(plan.ContradictionProtocol))
	}
	if len(plan.BeliefUpdateProtocol) != 8 {
		t.Fatalf("expected 8-step belief update protocol, got %d", len(plan.BeliefUpdateProtocol))
	}
	if !strings.Contains(plan.EpistemicPromptModule, "EPISTEMIC PROTOCOL") {
		t.Fatal("epistemic prompt module missing header")
	}
	if !strings.Contains(plan.EpistemicPSLBlock, "epistemology:") {
		t.Fatal("PSL block missing top-level key")
	}
}

func TestEpi_JSONRoundTrip(t *testing.T) {
	plan := Plan(PlanRequest{
		UseCase:    "Sanity",
		RiskLevel:  RiskLow,
		MemoryMode: MemDisabled,
		RawContext: []ContextItem{{ID: "1", Text: "X.", SourceKind: DSUserInput}},
	})
	b, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	var back PromptEpistemologyPlan
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.EpistemicScore != plan.EpistemicScore {
		t.Fatal("score lost in round-trip")
	}
	if len(back.BeliefState.Claims) != len(plan.BeliefState.Claims) {
		t.Fatal("belief_state claims lost in round-trip")
	}
}

// classifyAndBuild is the in-test shortcut used by the battery. It mirrors
// what /epistemology/plan does internally so tests can construct a
// belief_state without going through HTTP.
func classifyAndBuild(objective string, items []ContextItem) BeliefState {
	claims := classifyRawContext(PlanRequest{RawContext: items})
	return buildBeliefState(objective, claims)
}

func hasAntiPattern(g GateResult, name string) bool {
	for _, h := range g.AntiPatterns {
		if h.Name == name {
			return true
		}
	}
	return false
}
