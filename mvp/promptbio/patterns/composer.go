package patterns

import (
	"strings"

	"breedos-mvp/promptbio/organism"
)

// Compose merges N patterns into one composition plan, runs the §1688-1737
// 8 composition rules, and emits any RuleViolation(s).
//
// The plan unions Core organs + Optional organs across the input patterns,
// picks the largest organism size, the highest agency level, and the
// union of anti-pattern risks.
func Compose(req ComposeRequest) CompositionPlan {
	plan := CompositionPlan{
		Patterns:       append([]PatternName(nil), req.Patterns...),
		AntiPatterns:   []AntiPatternName{},
		Violations:     []RuleViolation{},
	}
	if len(req.Patterns) == 0 {
		plan.Violations = append(plan.Violations, RuleViolation{
			RuleID:   0,
			RuleName: "non_empty_composition",
			Detail:   "no patterns supplied",
			Fix:      "name at least one pattern from the canonical 12",
		})
		return plan
	}

	requiredSet := map[organism.OrganName]bool{}
	optionalSet := map[organism.OrganName]bool{}
	antiSet := map[AntiPatternName]bool{}
	size := organism.SizeMicro
	agency := AgencyA0

	for _, p := range req.Patterns {
		card := lookupCard(p)
		for _, o := range card.CoreOrgans {
			requiredSet[o] = true
		}
		for _, o := range card.OptionalOrgans {
			optionalSet[o] = true
		}
		for _, a := range card.AntiPatterns {
			antiSet[a] = true
		}
		if sizeRank(card.OrganismSize) > sizeRank(size) {
			size = card.OrganismSize
		}
		if agencyRank(card.AgencyLevel) > agencyRank(agency) {
			agency = card.AgencyLevel
		}
	}

	for o := range requiredSet {
		plan.RequiredOrgans = append(plan.RequiredOrgans, o)
		delete(optionalSet, o)
	}
	for o := range optionalSet {
		plan.OptionalOrgans = append(plan.OptionalOrgans, o)
	}
	for a := range antiSet {
		plan.AntiPatterns = append(plan.AntiPatterns, a)
	}
	plan.OrganismSize = size
	plan.AgencyLevel = agency

	plan.Violations = checkCompositionRules(plan, req.Patterns)
	plan.Pass = len(plan.Violations) == 0
	return plan
}

func sizeRank(s organism.OrganismSize) int {
	switch s {
	case organism.SizeMacro:
		return 3
	case organism.SizeMeso:
		return 2
	case organism.SizeMicro:
		return 1
	}
	return 0
}

func agencyRank(a AgencyLevel) int {
	switch a {
	case AgencyA6:
		return 6
	case AgencyA5:
		return 5
	case AgencyA4:
		return 4
	case AgencyA3:
		return 3
	case AgencyA2:
		return 2
	case AgencyA1:
		return 1
	}
	return 0
}

func containsPattern(xs []PatternName, x PatternName) bool {
	for _, p := range xs {
		if p == x {
			return true
		}
	}
	return false
}

func containsOrgan(xs []organism.OrganName, x organism.OrganName) bool {
	for _, o := range xs {
		if o == x {
			return true
		}
	}
	return false
}

// CompositionRules returns the §1688-1737 8 rules.
func CompositionRules() []CompositionRule {
	return []CompositionRule{
		{1, "no_agency_without_control", "Tool-using patterns must compose with stop conditions, ToolROI gates, and a permission policy."},
		{2, "no_memory_without_truth_or_privacy", "Memory-using patterns must compose with the memory truth policy + PII minimisation."},
		{3, "no_research_without_source_evaluation", "Research synthesis must compose with source-tier hierarchy + confidence axes."},
		{4, "no_strategy_without_constraints", "Strategy patterns must compose with a constraint register; generic advice is not a strategy."},
		{5, "no_critique_without_repair", "Critic patterns compose with an Advisor / Repair pattern to close the loop."},
		{6, "no_high_assurance_on_low_risk", "Do not over-engineer: high-assurance overhead has a cost; reserve for regulated / safety-critical."},
		{7, "no_one_giant_prompt", "If the composition crosses 3 patterns, split into a multi-pattern organism with explicit hand-offs (v3.2)."},
		{8, "stable_core_adaptive_pattern", "Genome + Constitution + Values are version-pinned; pattern choice is the adaptive periphery."},
	}
}

func checkCompositionRules(plan CompositionPlan, patterns []PatternName) []RuleViolation {
	v := []RuleViolation{}

	// Rule 1: no_agency_without_control
	if containsPattern(patterns, PatternAgenticToolUsing) {
		needed := []organism.OrganName{organism.OrganPlanning, organism.OrganGovernance, organism.OrganImmune}
		for _, o := range needed {
			if !containsOrgan(plan.RequiredOrgans, o) {
				v = append(v, RuleViolation{
					RuleID: 1, RuleName: "no_agency_without_control",
					Detail: "Agentic Tool-Using requires " + string(o) + " in the composition",
					Fix:    "add a pattern that brings " + string(o) + " (Strategy Advisor brings governance; High-Assurance Advisor brings immune; Code Repair brings planning)",
				})
			}
		}
	}

	// Rule 2: no_memory_without_truth_or_privacy
	if containsPattern(patterns, PatternMemoryAwareCompanion) {
		if !containsOrgan(plan.RequiredOrgans, organism.OrganImmune) {
			v = append(v, RuleViolation{
				RuleID: 2, RuleName: "no_memory_without_truth_or_privacy",
				Detail: "memory-using pattern composed without an immune organ (PII minimisation)",
				Fix:    "compose with High-Assurance Advisor or Document Review to bring the immune organ",
			})
		}
	}

	// Rule 3: no_research_without_source_evaluation
	if containsPattern(patterns, PatternResearchSynthesizer) {
		if !containsOrgan(plan.RequiredOrgans, organism.OrganEpistemic) {
			v = append(v, RuleViolation{
				RuleID: 3, RuleName: "no_research_without_source_evaluation",
				Detail: "Research Synthesizer composed without epistemic organ",
				Fix:    "ensure the v3.0 epistemic organ is in core_organs of at least one composed pattern",
			})
		}
	}

	// Rule 4: no_strategy_without_constraints
	if containsPattern(patterns, PatternStrategyAdvisor) {
		if !containsOrgan(plan.RequiredOrgans, organism.OrganGovernance) && !containsOrgan(plan.RequiredOrgans, organism.OrganConstitutional) {
			v = append(v, RuleViolation{
				RuleID: 4, RuleName: "no_strategy_without_constraints",
				Detail: "Strategy Advisor composed without governance / constitutional anchor for the constraint register",
				Fix:    "compose with Eval Judge or High-Assurance Advisor to bring governance",
			})
		}
	}

	// Rule 5: no_critique_without_repair
	if containsPattern(patterns, PatternStrategyCritic) && !containsAnyPattern(patterns, []PatternName{PatternStrategyAdvisor, PatternCodeRepair}) {
		v = append(v, RuleViolation{
			RuleID: 5, RuleName: "no_critique_without_repair",
			Detail: "Strategy Critic composed without a repair pattern (Strategy Advisor / Code Repair)",
			Fix:    "add Strategy Advisor or Code Repair to close the critique → repair loop",
		})
	}

	// Rule 6: no_high_assurance_on_low_risk — heuristic: if HighAssuranceAdvisor
	// is the ONLY pattern in the composition and the task family is non-regulated,
	// flag. v0.7.38 cannot easily reason about niche regulation, so we apply a
	// conservative version: HA Advisor alone is fine; HA Advisor + Explainer Cell
	// only triggers when there's no constitution organ (over-engineered hint).
	if containsPattern(patterns, PatternHighAssuranceAdvisor) && containsPattern(patterns, PatternExplainerCell) {
		v = append(v, RuleViolation{
			RuleID: 6, RuleName: "no_high_assurance_on_low_risk",
			Detail: "High-Assurance overhead composed with Explainer Cell (audience-bounded one-shot — typically low-risk)",
			Fix:    "drop High-Assurance unless the audience operates in a regulated domain",
		})
	}

	// Rule 7: no_one_giant_prompt — composition larger than 3 patterns.
	if len(patterns) > 3 {
		v = append(v, RuleViolation{
			RuleID: 7, RuleName: "no_one_giant_prompt",
			Detail: "composition crosses 3 patterns — single-organism implementation will become a God Prompt",
			Fix:    "split into a multi-pattern organism with explicit hand-offs (queued for v3.2)",
		})
	}

	// Rule 8: stable_core_adaptive_pattern — always require Genome + Constitution in core_organs.
	if !containsOrgan(plan.RequiredOrgans, organism.OrganGenome) || !containsOrgan(plan.RequiredOrgans, organism.OrganConstitutional) {
		// Soft hint when the composition doesn't enforce both. Skip when only Explainer Cell (micro) is composed alone.
		isOnlyExplainer := len(patterns) == 1 && patterns[0] == PatternExplainerCell
		if !isOnlyExplainer {
			missing := []string{}
			if !containsOrgan(plan.RequiredOrgans, organism.OrganGenome) {
				missing = append(missing, "genome")
			}
			if !containsOrgan(plan.RequiredOrgans, organism.OrganConstitutional) {
				missing = append(missing, "constitutional")
			}
			v = append(v, RuleViolation{
				RuleID: 8, RuleName: "stable_core_adaptive_pattern",
				Detail: "stable core missing — " + strings.Join(missing, " + ") + " organ(s) not anchored",
				Fix:    "compose with a pattern that brings the missing stable-core organ (Strategy Advisor / High-Assurance Advisor / Eval Judge)",
			})
		}
	}

	return v
}

func containsAnyPattern(xs []PatternName, candidates []PatternName) bool {
	for _, c := range candidates {
		if containsPattern(xs, c) {
			return true
		}
	}
	return false
}
