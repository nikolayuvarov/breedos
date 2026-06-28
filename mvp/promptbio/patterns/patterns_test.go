package patterns

import (
	"encoding/json"
	"testing"

	"breedos-mvp/promptbio/organism"
)

// Acceptance: 12 canonical patterns + each card has all 20 fields populated.
func TestPatterns_12CardsAll20Fields(t *testing.T) {
	cards := Cards()
	if len(cards) != 12 {
		t.Fatalf("expected 12 patterns, got %d", len(cards))
	}
	seen := map[PatternName]bool{}
	for _, c := range cards {
		seen[c.Name] = true
		if c.Intent == "" || len(c.UseWhen) == 0 || len(c.AvoidWhen) == 0 {
			t.Errorf("%s missing Intent/UseWhen/AvoidWhen", c.Name)
		}
		if c.OrganismSize == "" || c.AgencyLevel == "" {
			t.Errorf("%s missing OrganismSize/AgencyLevel", c.Name)
		}
		if len(c.CoreOrgans) == 0 {
			t.Errorf("%s missing CoreOrgans", c.Name)
		}
		if c.Genome == "" || c.Epistemology == "" || c.DecisionPlanning == "" || c.Runtime == "" || c.Memory == "" || c.Tools == "" {
			t.Errorf("%s missing one of Genome/Epistemology/DecisionPlanning/Runtime/Memory/Tools", c.Name)
		}
		if c.OutputPhenotype == "" || len(c.FailureModes) == 0 || len(c.RequiredTests) == 0 {
			t.Errorf("%s missing OutputPhenotype/FailureModes/RequiredTests", c.Name)
		}
		if c.MinimalPromptSkeleton == "" || c.ProductionArchitecture == "" {
			t.Errorf("%s missing MinimalPromptSkeleton/ProductionArchitecture", c.Name)
		}
		if c.EvolutionPath == "" {
			t.Errorf("%s missing EvolutionPath", c.Name)
		}
	}
	for _, want := range AllPatterns {
		if !seen[want] {
			t.Errorf("missing pattern: %s", want)
		}
	}
}

// Acceptance: every pattern card's CoreOrgans is a subset of the v3.0
// 16-organ catalogue.
func TestPatterns_CoreOrgansSubsetOfV3(t *testing.T) {
	validOrgans := map[organism.OrganName]bool{}
	for _, o := range organism.AllOrgans {
		validOrgans[o] = true
	}
	for _, c := range Cards() {
		for _, o := range c.CoreOrgans {
			if !validOrgans[o] {
				t.Errorf("%s references unknown organ %s", c.Name, o)
			}
		}
	}
}

// Acceptance: selection matrix returns canonical patterns for known niches.
func TestPatterns_SelectorMatrixRouting(t *testing.T) {
	cases := []struct {
		niche string
		want  PatternName
	}{
		{"explain a topic", PatternExplainerCell},
		{"synthesise sources", PatternResearchSynthesizer},
		{"GTM strategy", PatternStrategyAdvisor},
		{"review a strategy", PatternStrategyCritic},
		{"fix a bug", PatternCodeRepair},
		{"review a contract", PatternDocumentReview},
		{"analyse data", PatternDataAnalysis},
		{"judge an answer", PatternEvalJudge},
		{"track a learner", PatternMemoryAwareCompanion},
		{"compliance advice", PatternHighAssuranceAdvisor},
		{"manage a portfolio", PatternAutopoieticEcosystem},
	}
	for _, tc := range cases {
		got := SelectPattern(SelectorRequest{TaskNiche: tc.niche})
		if got.RecommendedPattern != tc.want {
			t.Errorf("niche %q → expected %s, got %s", tc.niche, tc.want, got.RecommendedPattern)
		}
	}
}

// Acceptance: overrides — tools=external + deployment=agent → Agentic.
func TestPatterns_SelectorOverrides(t *testing.T) {
	r := SelectPattern(SelectorRequest{TaskNiche: "explain a topic", Tools: organism.ToolExternal})
	if r.RecommendedPattern != PatternAgenticToolUsing {
		t.Errorf("tools=external should override to Agentic Tool-Using, got %s", r.RecommendedPattern)
	}
	r = SelectPattern(SelectorRequest{TaskNiche: "research", QualityLevel: organism.QualityHighAssurance})
	if r.RecommendedPattern != PatternHighAssuranceAdvisor {
		t.Errorf("quality=high-assurance should override to High-Assurance Advisor, got %s", r.RecommendedPattern)
	}
	r = SelectPattern(SelectorRequest{TaskNiche: "strategy", Memory: organism.MemoryLongTerm})
	if r.RecommendedPattern != PatternMemoryAwareCompanion {
		t.Errorf("memory=long-term should override to Memory-Aware Companion, got %s", r.RecommendedPattern)
	}
}

// Acceptance: GTM worked example (§2115-2162). The selector + composer
// should route a GTM Product Launch query (medium risk, session memory)
// to Strategy Advisor as the lead pattern.
func TestPatterns_GTMWorkedExample(t *testing.T) {
	rec := SelectPattern(SelectorRequest{
		TaskNiche:       "GTM Product Launch Strategy",
		TargetPhenotype: "structured launch strategy",
		RiskLevel:       organism.RiskMedium,
		Memory:          organism.MemorySession,
		Tools:           organism.ToolReadOnly,
		Deployment:      organism.DeploymentMultiTurn,
		QualityLevel:    organism.QualityProduction,
	})
	if rec.RecommendedPattern != PatternStrategyAdvisor {
		t.Fatalf("expected Strategy Advisor for GTM Product Launch, got %s", rec.RecommendedPattern)
	}
	// Composition with Strategy Critic + Eval Judge should still pass.
	plan := Compose(ComposeRequest{Patterns: []PatternName{PatternStrategyAdvisor, PatternStrategyCritic, PatternEvalJudge}})
	if !plan.Pass {
		t.Logf("composition violations: %+v", plan.Violations)
		t.Fatal("GTM composition (Advisor + Critic + Eval Judge) should pass all 8 rules")
	}
}

// Acceptance: composition rule violations fire when expected.
func TestPatterns_Rule1_AgencyWithoutControl(t *testing.T) {
	// Agentic Tool-Using alone — composition lacks planning organ in core_organs
	// unless we add it. But Agentic's own card already includes planning. So
	// this test verifies the catalogue is structurally honest.
	plan := Compose(ComposeRequest{Patterns: []PatternName{PatternAgenticToolUsing}})
	// Agentic alone: it brings planning + governance + immune itself; should pass Rule 1
	// but may trip Rule 7 (single-pattern is fine — only > 3 trips Rule 7).
	for _, v := range plan.Violations {
		if v.RuleID == 1 {
			t.Errorf("Agentic Tool-Using alone should satisfy Rule 1; violation: %v", v)
		}
	}
}

func TestPatterns_Rule5_CritiqueWithoutRepair(t *testing.T) {
	plan := Compose(ComposeRequest{Patterns: []PatternName{PatternStrategyCritic}})
	found := false
	for _, v := range plan.Violations {
		if v.RuleID == 5 {
			found = true
		}
	}
	if !found {
		t.Fatal("Strategy Critic alone must violate Rule 5 (no critique without repair)")
	}
}

func TestPatterns_Rule6_HighAssuranceOnLowRisk(t *testing.T) {
	plan := Compose(ComposeRequest{Patterns: []PatternName{PatternHighAssuranceAdvisor, PatternExplainerCell}})
	found := false
	for _, v := range plan.Violations {
		if v.RuleID == 6 {
			found = true
		}
	}
	if !found {
		t.Fatal("High-Assurance + Explainer Cell must violate Rule 6")
	}
}

func TestPatterns_Rule7_GiantPrompt(t *testing.T) {
	plan := Compose(ComposeRequest{Patterns: []PatternName{
		PatternStrategyAdvisor, PatternStrategyCritic, PatternEvalJudge, PatternHighAssuranceAdvisor,
	}})
	found := false
	for _, v := range plan.Violations {
		if v.RuleID == 7 {
			found = true
		}
	}
	if !found {
		t.Fatal("4+ patterns must violate Rule 7 (no one giant prompt)")
	}
}

// Anti-pattern detectors should fire on canonical bad-spec fixtures.
func TestPatterns_AntiGodPrompt(t *testing.T) {
	spec := organism.PromptOrganismSpec{
		PromptOrganism: organism.Identity{ID: "x", OrganismType: organism.SizeMacro},
		ExpressionProfiles: []string{"full"},
	}
	r := DetectAntiPatterns(AntiPatternRequest{Spec: spec})
	if !hasFinding(r, AntiGodPrompt) {
		t.Fatal("expected God Prompt finding on macro + 1 profile")
	}
}

func TestPatterns_AntiConfidentOracle(t *testing.T) {
	spec := organism.PromptOrganismSpec{
		Governance:    organism.GovernanceSpec{RiskTier: "high"},
		Epistemology:  organism.EpistemologySpec{BeliefStateEnabled: false},
	}
	r := DetectAntiPatterns(AntiPatternRequest{Spec: spec})
	if !hasFinding(r, AntiConfidentOracle) {
		t.Fatal("expected Confident Oracle finding on high risk + no belief state")
	}
}

func TestPatterns_AntiToolGremlin(t *testing.T) {
	spec := organism.PromptOrganismSpec{
		Runtime:  organism.RuntimeSpec{Tools: []string{"web_search"}},
		Decision: organism.DecisionSpec{ToolROIRequired: false},
	}
	r := DetectAntiPatterns(AntiPatternRequest{Spec: spec})
	if !hasFinding(r, AntiToolGremlin) {
		t.Fatal("expected Tool Gremlin on tools + no ROI gate")
	}
}

func TestPatterns_AntiMemoryHoarder(t *testing.T) {
	spec := organism.PromptOrganismSpec{
		Runtime: organism.RuntimeSpec{MemoryPolicy: "remember everything across sessions"},
	}
	r := DetectAntiPatterns(AntiPatternRequest{Spec: spec})
	if !hasFinding(r, AntiMemoryHoarder) {
		t.Fatal("expected Memory Hoarder on memory without write gate / freshness")
	}
}

func TestPatterns_AntiEvalGamer(t *testing.T) {
	spec := organism.PromptOrganismSpec{
		Autopoiesis: organism.AutopoiesisSpec{Level: organism.AutopoiesisAssistedSelfMaintenance},
		Evaluation:  organism.EvaluationSpec{TestSuite: ""},
	}
	r := DetectAntiPatterns(AntiPatternRequest{Spec: spec})
	if !hasFinding(r, AntiEvalGamer) {
		t.Fatal("expected Eval Gamer on autopoiesis + no test suite")
	}
}

func TestPatterns_AntiPaperShield(t *testing.T) {
	spec := organism.PromptOrganismSpec{
		Governance:   organism.GovernanceSpec{RiskTier: "high"},
		Constitution: organism.ConstitutionSpec{Immutable: []string{"one rule"}},
	}
	r := DetectAntiPatterns(AntiPatternRequest{Spec: spec})
	if !hasFinding(r, AntiPaperShield) {
		t.Fatal("expected Paper Shield on high risk + thin constitution")
	}
}

func TestPatterns_AntiFormatTyrant(t *testing.T) {
	spec := organism.PromptOrganismSpec{
		Genome:      organism.GenomeSpec{Spec: "task=explain · role=teacher · method=scaffolded"},
		Homeostasis: organism.HomeostasisSpec{FormatCheck: true},
	}
	r := DetectAntiPatterns(AntiPatternRequest{Spec: spec})
	if !hasFinding(r, AntiFormatTyrant) {
		t.Fatal("expected Format Tyrant on format_check + no output_schema in genome")
	}
}

func TestPatterns_AntiAgentWithoutStopCond(t *testing.T) {
	spec := organism.PromptOrganismSpec{
		Runtime:  organism.RuntimeSpec{Tools: []string{"web_search", "filesystem"}},
		Planning: organism.PlanningSpec{StopConditionsRequired: false},
	}
	r := DetectAntiPatterns(AntiPatternRequest{Spec: spec})
	if !hasFinding(r, AntiAgentWithoutStopCond) {
		t.Fatal("expected Agent Without Stop Condition on tools + no stop conditions")
	}
}

func TestPatterns_AntiStrategyFantasist(t *testing.T) {
	spec := organism.PromptOrganismSpec{
		PromptOrganism: organism.Identity{OrganismType: organism.SizeMeso},
		Homeostasis:    organism.HomeostasisSpec{ConstraintCheck: false},
	}
	r := DetectAntiPatterns(AntiPatternRequest{Spec: spec})
	if !hasFinding(r, AntiStrategyFantasist) {
		t.Fatal("expected Strategy Fantasist on meso + no constraint check")
	}
}

func TestPatterns_AntiDocumentPuppet(t *testing.T) {
	spec := organism.PromptOrganismSpec{
		Runtime:  organism.RuntimeSpec{ContextPolicy: "source:docs; source:user_input"},
		Immunity: organism.ImmunitySpec{PromptInjectionBoundary: false},
	}
	r := DetectAntiPatterns(AntiPatternRequest{Spec: spec})
	if !hasFinding(r, AntiDocumentPuppet) {
		t.Fatal("expected Document Puppet on docs context + no injection boundary")
	}
}

// Catalogue: 10 anti-patterns + 8 composition rules + selection matrix non-empty.
func TestPatterns_Catalogues(t *testing.T) {
	if len(AntiPatternsCatalogue()) != 10 {
		t.Fatalf("expected 10 anti-patterns in catalogue, got %d", len(AntiPatternsCatalogue()))
	}
	if len(CompositionRules()) != 8 {
		t.Fatalf("expected 8 composition rules, got %d", len(CompositionRules()))
	}
	if len(SelectionMatrix()) < 12 {
		t.Fatalf("expected ≥12 selection matrix rows, got %d", len(SelectionMatrix()))
	}
}

// JSON round-trip.
func TestPatterns_JSONRoundTrip(t *testing.T) {
	rec := SelectPattern(SelectorRequest{TaskNiche: "research"})
	b, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	var back PatternRecommendation
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.RecommendedPattern != rec.RecommendedPattern {
		t.Fatal("recommendation lost")
	}
}

// Deterministic across re-runs.
func TestPatterns_Deterministic(t *testing.T) {
	a := SelectPattern(SelectorRequest{TaskNiche: "fix a bug"})
	b := SelectPattern(SelectorRequest{TaskNiche: "fix a bug"})
	if a.RecommendedPattern != b.RecommendedPattern {
		t.Fatal("non-deterministic")
	}
}

func hasFinding(r AntiPatternResponse, name AntiPatternName) bool {
	for _, f := range r.Findings {
		if f.Name == name {
			return true
		}
	}
	return false
}
