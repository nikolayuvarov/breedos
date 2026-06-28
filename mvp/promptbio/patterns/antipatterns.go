package patterns

import (
	"strings"

	"breedos-mvp/promptbio/organism"
)

// DetectAntiPatterns is the §1741-1929 anti-pattern linter. Each
// detector inspects the supplied organism spec and emits one
// AntiPatternFinding per anti-pattern detected.
//
// Each anti-pattern is detected from a single symptom that is easy to
// catch from the spec shape; deeper detection (e.g. drift on
// production telemetry) is queued for v1.4 Runtime.
func DetectAntiPatterns(req AntiPatternRequest) AntiPatternResponse {
	findings := []AntiPatternFinding{}
	spec := req.Spec

	// §18.1 God Prompt — one giant prompt with all 16 organs squeezed
	// into a single expression profile and no expression_profiles split.
	if len(spec.ExpressionProfiles) == 1 && spec.PromptOrganism.OrganismType == organism.SizeMacro {
		findings = append(findings, AntiPatternFinding{
			Name:      AntiGodPrompt,
			Symptom:   "macro-size organism with a single expression profile",
			Treatment: "split into expression profiles (lite / standard / full / high_assurance / eval) per §11",
			Evidence:  "expression_profiles has 1 entry; size=macro",
		})
	}

	// §18.2 Confident Oracle — belief state disabled while risk_tier ≥ medium.
	risk := strings.ToLower(spec.Governance.RiskTier)
	if !spec.Epistemology.BeliefStateEnabled && (risk == "medium" || risk == "high") {
		findings = append(findings, AntiPatternFinding{
			Name:      AntiConfidentOracle,
			Symptom:   "risk_tier ≥ medium but epistemology.belief_state_enabled is false",
			Treatment: "enable belief state; populate claim_types; gate output with epistemic status block on medium/high risk",
			Evidence:  "risk_tier=" + risk + "; belief_state_enabled=false",
		})
	}

	// §18.3 Tool Gremlin — tools registered without decision.tool_roi_required.
	if len(spec.Runtime.Tools) > 0 && !spec.Decision.ToolROIRequired {
		findings = append(findings, AntiPatternFinding{
			Name:      AntiToolGremlin,
			Symptom:   "tools registered without ToolROI gate",
			Treatment: "set decision.tool_roi_required=true; require a stated reason before every tool call",
			Evidence:  "tools=" + strings.Join(spec.Runtime.Tools, ",") + "; tool_roi_required=false",
		})
	}

	// §18.4 Memory Hoarder — memory enabled without write-gate / freshness language.
	if spec.Runtime.MemoryPolicy != "" && !strings.Contains(strings.ToLower(spec.Runtime.MemoryPolicy), "no memory") {
		lowMem := strings.ToLower(spec.Runtime.MemoryPolicy)
		if !strings.Contains(lowMem, "gate") && !strings.Contains(lowMem, "freshness") {
			findings = append(findings, AntiPatternFinding{
				Name:      AntiMemoryHoarder,
				Symptom:   "memory enabled but no write gate / freshness policy declared",
				Treatment: "add the §16 7-question write gate + freshness on read to memory_policy",
				Evidence:  "memory_policy='" + spec.Runtime.MemoryPolicy + "'",
			})
		}
	}

	// §18.5 Eval Gamer — autopoiesis enabled but evaluation.test_suite empty.
	if spec.Autopoiesis.Level != organism.AutopoiesisNone && spec.Evaluation.TestSuite == "" {
		findings = append(findings, AntiPatternFinding{
			Name:      AntiEvalGamer,
			Symptom:   "autopoiesis enabled without an eval lab gate",
			Treatment: "wire evaluation.test_suite to the v1.3 Eval Lab before enabling autopoiesis",
			Evidence:  "autopoiesis.level=" + string(spec.Autopoiesis.Level) + "; evaluation.test_suite empty",
		})
	}

	// §18.6 Paper Shield — constitution is empty/thin (< 2 immutables) on risk_tier ≥ medium.
	if (risk == "medium" || risk == "high") && len(spec.Constitution.Immutable) < 2 {
		findings = append(findings, AntiPatternFinding{
			Name:      AntiPaperShield,
			Symptom:   "constitution too thin for the risk tier",
			Treatment: "add at least 3 immutable rules covering honesty, constraint respect, and contradiction handling",
			Evidence:  "risk_tier=" + risk + "; constitution.immutable has " + itoa(len(spec.Constitution.Immutable)) + " entries",
		})
	}

	// §18.7 Format Tyrant — homeostasis.format_check on but output schema unstated in genome.
	if spec.Homeostasis.FormatCheck && !strings.Contains(strings.ToLower(spec.Genome.Spec), "output_schema") {
		findings = append(findings, AntiPatternFinding{
			Name:      AntiFormatTyrant,
			Symptom:   "format_check enabled but genome doesn't declare an output_schema",
			Treatment: "add output_schema to genome.spec OR drop format_check",
			Evidence:  "genome.spec doesn't mention output_schema",
		})
	}

	// §18.8 Agent Without Stop Condition — tools + no stop conditions.
	if len(spec.Runtime.Tools) > 0 && !spec.Planning.StopConditionsRequired {
		findings = append(findings, AntiPatternFinding{
			Name:      AntiAgentWithoutStopCond,
			Symptom:   "tools registered without explicit stop conditions",
			Treatment: "set planning.stop_conditions_required=true; declare per-loop termination criteria",
			Evidence:  "tools=" + strings.Join(spec.Runtime.Tools, ",") + "; planning.stop_conditions_required=false",
		})
	}

	// §18.9 Strategy Fantasist — no constraint check on a meso/macro organism.
	if spec.PromptOrganism.OrganismType != organism.SizeMicro && !spec.Homeostasis.ConstraintCheck {
		findings = append(findings, AntiPatternFinding{
			Name:      AntiStrategyFantasist,
			Symptom:   "non-micro organism without constraint_check",
			Treatment: "set homeostasis.constraint_check=true; declare the constraint register in genome",
			Evidence:  "organism_type=" + string(spec.PromptOrganism.OrganismType) + "; constraint_check=false",
		})
	}

	// §18.10 Document Puppet — context organ enabled but immunity.prompt_injection_boundary off.
	if spec.Runtime.ContextPolicy != "" && !spec.Immunity.PromptInjectionBoundary && strings.Contains(strings.ToLower(spec.Runtime.ContextPolicy), "docs") {
		findings = append(findings, AntiPatternFinding{
			Name:      AntiDocumentPuppet,
			Symptom:   "document sources in context without injection boundary",
			Treatment: "set immunity.prompt_injection_boundary=true; treat document body as data, never instruction",
			Evidence:  "context_policy mentions docs; prompt_injection_boundary=false",
		})
	}

	return AntiPatternResponse{
		Findings: findings,
		Pass:     len(findings) == 0,
	}
}

// AntiPatternsCatalogue returns the §18 verbatim catalogue.
func AntiPatternsCatalogue() []AntiPatternEntry {
	return []AntiPatternEntry{
		{AntiGodPrompt, "one giant prompt trying to cover every case in a single expression profile", "split into expression profiles per §11; route via runtime"},
		{AntiConfidentOracle, "model issues confident answers without a belief state or evidence axis", "wire the v2.7 belief state; gate output with the epistemic status block"},
		{AntiToolGremlin, "tools called needlessly without ROI calculation", "set decision.tool_roi_required=true; require a stated reason before every call"},
		{AntiMemoryHoarder, "memory persists every utterance with no write gate", "add the §16 7-question write gate; freshness on read"},
		{AntiEvalGamer, "test bank predictable; output styled to pass eval", "rotate hidden tests; use the v1.3 Eval Lab adversarial battery"},
		{AntiPaperShield, "constitutional layer present but vacuous (too few immutables)", "author at least 3 immutables covering honesty, constraint respect, contradiction handling"},
		{AntiFormatTyrant, "format checks enforced but the underlying genome lacks an output_schema", "add output_schema to genome.spec, or drop format_check"},
		{AntiAgentWithoutStopCond, "agent loops until budget is exhausted", "declare planning.stop_conditions_required=true; per-loop termination criteria"},
		{AntiStrategyFantasist, "strategy ignores stated constraints", "set homeostasis.constraint_check=true; declare the constraint register in genome"},
		{AntiDocumentPuppet, "document body overrides system instructions via prompt injection", "set immunity.prompt_injection_boundary=true; treat document as data"},
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}
