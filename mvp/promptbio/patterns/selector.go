package patterns

import (
	"strings"

	"breedos-mvp/promptbio/organism"
)

// SelectPattern is the §3.1 Design Pattern Selector entry point.
// Layers heuristics over the §1577-1592 task → pattern selection matrix.
func SelectPattern(req SelectorRequest) PatternRecommendation {
	pat := pickPattern(req)
	card := lookupCard(pat)
	return PatternRecommendation{
		RecommendedPattern:    pat,
		WhyThisPattern:        rationaleFor(pat, req),
		OrganismSize:          card.OrganismSize,
		AgencyLevel:           card.AgencyLevel,
		RequiredOrgans:        card.CoreOrgans,
		OptionalOrgans:        card.OptionalOrgans,
		AntiPatternRisks:      card.AntiPatterns,
		MinimalPromptSkeleton: card.MinimalPromptSkeleton,
		RequiredTests:         card.RequiredTests,
		NextAction:            nextActionFor(pat, req),
		Card:                  card,
	}
}

// pickPattern routes the inputs to one of the 12 canonical patterns.
//
// Override priority (top to bottom; first match wins):
//   1. Tools=external_actions OR Deployment=agent      → Agentic Tool-Using
//   2. Quality=high-assurance OR Risk=high             → High-Assurance Advisor
//   3. Memory ∈ {project, long-term}                   → Memory-Aware Companion
//   4. Niche keyword match                             → matrix row
//   5. Default                                          → Strategy Advisor
func pickPattern(req SelectorRequest) PatternName {
	if req.Tools == organism.ToolExternal || req.Deployment == organism.DeploymentAgent {
		return PatternAgenticToolUsing
	}
	if req.QualityLevel == organism.QualityHighAssurance || req.RiskLevel == organism.RiskHigh {
		return PatternHighAssuranceAdvisor
	}
	if req.Memory == organism.MemoryProject || req.Memory == organism.MemoryLongTerm {
		return PatternMemoryAwareCompanion
	}
	low := strings.ToLower(req.TaskNiche + " " + req.TargetPhenotype)
	for _, row := range SelectionMatrix() {
		if strings.Contains(low, strings.ToLower(row.Niche)) {
			return row.Pattern
		}
	}
	return PatternStrategyAdvisor
}

func lookupCard(name PatternName) PatternCard {
	for _, c := range Cards() {
		if c.Name == name {
			return c
		}
	}
	return Cards()[0]
}

func rationaleFor(pat PatternName, req SelectorRequest) string {
	parts := []string{string(pat) + " fits:"}
	if req.Tools == organism.ToolExternal {
		parts = append(parts, "external_actions tools require an agent loop")
	}
	if req.QualityLevel == organism.QualityHighAssurance {
		parts = append(parts, "high-assurance quality gates source provenance + audit")
	}
	if req.RiskLevel == organism.RiskHigh {
		parts = append(parts, "high risk gates constitutional dominance over speed")
	}
	if req.Memory == organism.MemoryProject || req.Memory == organism.MemoryLongTerm {
		parts = append(parts, "persistent memory drives continuity and deprecation discipline")
	}
	if len(parts) == 1 {
		parts = append(parts, "default niche route from the §1577 selection matrix")
	}
	return strings.Join(parts, " · ")
}

func nextActionFor(pat PatternName, req SelectorRequest) string {
	switch pat {
	case PatternAgenticToolUsing:
		return "design_runtime · wire planning organ with explicit stop conditions; gate every external_actions call through governance"
	case PatternHighAssuranceAdvisor:
		return "design_security · author constitutional immutables + observability traces before first release"
	case PatternMemoryAwareCompanion:
		return "design_PSL · author memory_truth_policy with write gate + freshness on read + PII minimisation"
	case PatternAutopoieticEcosystem:
		return "design_runtime · validate observability is live + governance log is wired before enabling autopoiesis"
	}
	return "design_PSL · draft the PSL spec from the minimal_prompt_skeleton; gate via Eval Lab before release"
}

// SelectionMatrix encodes §1577-1592 task → pattern matrix verbatim.
// Niche keywords are matched case-insensitive against task_niche + target_phenotype.
func SelectionMatrix() []SelectionMatrixRow {
	return []SelectionMatrixRow{
		{"explain a topic", PatternExplainerCell, "audience-bounded one-shot explanation"},
		{"teach", PatternExplainerCell, "audience-bounded knowledge transfer"},
		{"synthesise sources", PatternResearchSynthesizer, "multi-source synthesis with source quality stratification"},
		{"research", PatternResearchSynthesizer, "grounded multi-document synthesis"},
		{"literature review", PatternResearchSynthesizer, "source-stratified review"},
		// Critic-class rows must come first so they win the substring race
		// against the broader "strategy" / "launch" rows below.
		{"critique", PatternStrategyCritic, "adversarial review of an existing strategy"},
		{"red team", PatternStrategyCritic, "outside-view critique"},
		{"red-team", PatternStrategyCritic, "outside-view critique"},
		{"review a strategy", PatternStrategyCritic, "adversarial strategy review"},
		{"strategy", PatternStrategyAdvisor, "actionable recommendation under constraints"},
		{"launch", PatternStrategyAdvisor, "GTM / launch playbook under constraints"},
		{"gtm", PatternStrategyAdvisor, "GTM / launch playbook under constraints"},
		{"fix a bug", PatternCodeRepair, "failing test → minimal patch"},
		{"repair code", PatternCodeRepair, "failing test → minimal patch"},
		{"debug", PatternCodeRepair, "failing test → minimal patch"},
		{"code", PatternCodeRepair, "fix or extend code with tests"},
		{"review a contract", PatternDocumentReview, "adversarial document read for risks"},
		{"review a paper", PatternDocumentReview, "adversarial document read for risks"},
		{"audit", PatternDocumentReview, "adversarial document read"},
		{"analyse data", PatternDataAnalysis, "hypothesis → method → code → interpretation"},
		{"analyze data", PatternDataAnalysis, "hypothesis → method → code → interpretation"},
		{"data analysis", PatternDataAnalysis, "hypothesis → method → code → interpretation"},
		{"judge an answer", PatternEvalJudge, "rubric-anchored scoring"},
		{"score", PatternEvalJudge, "rubric-anchored scoring"},
		{"evaluate", PatternEvalJudge, "rubric-anchored scoring"},
		{"track a learner", PatternMemoryAwareCompanion, "multi-session continuity"},
		{"continuity", PatternMemoryAwareCompanion, "multi-session continuity"},
		{"manage a portfolio", PatternAutopoieticEcosystem, "self-maintaining organism portfolio under governance"},
		{"portfolio management", PatternAutopoieticEcosystem, "self-maintaining organism portfolio under governance"},
		{"agent", PatternAgenticToolUsing, "objective + tool loop + stop conditions"},
		{"automation", PatternAgenticToolUsing, "objective + tool loop + stop conditions"},
		{"compliance advice", PatternHighAssuranceAdvisor, "regulated context with audit trail"},
		{"legal advice", PatternHighAssuranceAdvisor, "regulated context with audit trail"},
		{"medical advice", PatternHighAssuranceAdvisor, "regulated context with audit trail"},
	}
}
