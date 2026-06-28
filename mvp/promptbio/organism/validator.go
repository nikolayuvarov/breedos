package organism

import (
	"fmt"
	"strings"
)

// Validate runs the §11 size gate + §20 10-principle checks + §17/18/19
// classification verdicts (workflow / agent / organism / model boundary).
func Validate(spec PromptOrganismSpec, size OrganismSize) ValidationReport {
	report := ValidationReport{Pass: true}

	// §11 size gate — missing organs required by the routed size.
	missing := missingOrgansForSize(spec, size)
	if len(missing) > 0 {
		report.MissingOrgans = missing
		report.Pass = false
	}

	// §20 principle violations.
	for _, p := range Principles() {
		if v := checkPrinciple(p, spec); v != nil {
			report.PrincipleViolations = append(report.PrincipleViolations, *v)
			report.Pass = false
		}
	}

	// §17 workflow-vs-organism, §18 agent formula, §19 model boundary.
	verdict, notes := classifySpec(spec)
	report.ClassificationVerdict = verdict
	report.Notes = notes
	if verdict == "workflow" || verdict == "model_reference_only" {
		report.Pass = false
	}

	return report
}

// missingOrgansForSize returns the organs the size requires but the
// spec doesn't materialise (proxied via the booleans / non-empty fields
// on each sub-spec). For micro / meso the rule is relaxed; macro
// demands all 15+ organs to be effectively turned on.
func missingOrgansForSize(spec PromptOrganismSpec, size OrganismSize) []OrganName {
	missing := []OrganName{}
	required := requiredOrgansForSize(size)
	for _, organ := range required {
		if !organMaterialised(spec, organ) {
			missing = append(missing, organ)
		}
	}
	return missing
}

func requiredOrgansForSize(size OrganismSize) []OrganName {
	for _, s := range Sizes() {
		if s.Size == size {
			return s.RequiredOrgans
		}
	}
	return nil
}

// organMaterialised reports whether the spec turns the named organ on
// — using the natural marker per organ (non-empty list, boolean true,
// non-default policy string).
func organMaterialised(spec PromptOrganismSpec, organ OrganName) bool {
	switch organ {
	case OrganGenome:
		return spec.Genome.Spec != "" && len(spec.Genome.CoreGenes) > 0
	case OrganConstitutional:
		return len(spec.Constitution.Immutable) > 0
	case OrganContext:
		return spec.Runtime.ContextPolicy != ""
	case OrganEpistemic:
		return spec.Epistemology.BeliefStateEnabled && len(spec.Epistemology.ClaimTypes) > 0
	case OrganMetabolic:
		return spec.Metabolism.DigestContextBeforeAnswer
	case OrganImmune:
		return spec.Immunity.PromptInjectionBoundary || spec.Immunity.ContradictionHandling
	case OrganDecision:
		return spec.Decision.Policy != ""
	case OrganPlanning:
		return spec.Planning.PlanObjectRequired || spec.Planning.CheckpointsRequired || spec.Planning.StopConditionsRequired
	case OrganRuntime:
		return spec.Runtime.DefaultProfile != "" && spec.Runtime.StateSchema != ""
	case OrganMemory:
		return spec.Runtime.MemoryPolicy != "" && !strings.Contains(strings.ToLower(spec.Runtime.MemoryPolicy), "no memory")
	case OrganTool:
		return len(spec.Runtime.Tools) > 0
	case OrganObservability:
		return spec.Observability.TraceLevel != "" && len(spec.Observability.Metrics) > 0
	case OrganEvaluation:
		return spec.Evaluation.TestSuite != ""
	case OrganGovernance:
		return spec.Governance.RiskTier != ""
	case OrganReproductive:
		return spec.Lifecycle.Version != ""
	case OrganAutopoietic:
		return spec.Autopoiesis.Level != "" && spec.Autopoiesis.Level != AutopoiesisNone
	}
	return false
}

// checkPrinciple runs the §20 check for one principle against the spec.
// Returns nil when the principle holds.
func checkPrinciple(p Principle, spec PromptOrganismSpec) *PrincipleViolation {
	switch p.ID {
	case 1: // spec_before_prompt
		if spec.PromptOrganism.ID == "" || spec.Genome.Spec == "" {
			return violation(p, "spec missing identity or genome.spec")
		}
	case 2: // belief_before_recommendation
		if !spec.Epistemology.BeliefStateEnabled {
			return violation(p, "epistemology.belief_state_enabled is false")
		}
	case 3: // decision_before_action
		if spec.Decision.Policy == "" {
			return violation(p, "decision.policy is empty")
		}
		if len(spec.Runtime.Tools) > 0 && !spec.Decision.ToolROIRequired {
			return violation(p, "tools registered but decision.tool_roi_required is false")
		}
	case 4: // plan_before_agent_loop
		if len(spec.Runtime.Tools) > 0 && !spec.Planning.StopConditionsRequired {
			return violation(p, "tools registered but planning.stop_conditions_required is false")
		}
	case 5: // constitution_above_optimization
		if len(spec.Constitution.Immutable) == 0 {
			return violation(p, "constitution.immutable is empty")
		}
	case 6: // observability_before_autopoiesis
		if spec.Autopoiesis.Level != AutopoiesisNone {
			if spec.Observability.TraceLevel == "" || len(spec.Observability.Metrics) == 0 {
				return violation(p, "autopoiesis.level != none but observability is empty")
			}
		}
	case 7: // governance_before_production_evolution
		risk := strings.ToLower(spec.Governance.RiskTier)
		if (risk == "medium" || risk == "high") && !spec.Governance.ApprovalRequiredForMajor {
			return violation(p, "risk_tier ≥ medium but governance.approval_required_for_major_changes is false")
		}
	case 8: // deprecation_is_part_of_life
		if spec.Lifecycle.ReviewCadence == "" {
			return violation(p, "lifecycle.review_cadence is empty")
		}
	case 9: // resource_proportionality
		if spec.PromptOrganism.OrganismType == SizeMicro {
			if spec.Autopoiesis.Level != AutopoiesisNone || len(spec.Runtime.Tools) > 2 {
				return violation(p, "micro organism over-equipped (autopoiesis or tools enabled)")
			}
		}
	case 10: // stable_core_adaptive_periphery
		if len(spec.Genome.CoreGenes) == 0 {
			return violation(p, "genome.core_genes is empty — nothing pinned as stable")
		}
		if len(spec.Constitution.Immutable) == 0 {
			return violation(p, "constitution.immutable is empty — nothing pinned as stable")
		}
	}
	return nil
}

func violation(p Principle, detail string) *PrincipleViolation {
	return &PrincipleViolation{
		PrincipleID:  p.ID,
		PrincipleName: p.Name,
		Detail:       detail,
		Fix:          p.ViolationFix,
	}
}

// classifySpec implements §17 (workflow vs organism), §18 (agent formula),
// §19 (model boundary). Returns the verdict + notes.
//
// §17 — a "workflow" has no values, constitution, belief state, or
// control loops; it just routes data.
// §18 — an "agent" = PromptOrganism + ActionLoop + Tools + Permissions.
// §19 — a "model reference only" YAML names only the model with no
// spec / runtime / context.
func classifySpec(spec PromptOrganismSpec) (string, []string) {
	notes := []string{}

	// §19 model boundary — empty spec / runtime / context = just naming a model.
	if spec.Runtime.DefaultProfile == "" && spec.Genome.Spec == "" {
		return "model_reference_only", []string{"§19: spec / runtime / context all empty — not an organism, just a model reference"}
	}

	// §17 workflow detection.
	hasValues := len(spec.Values.Primary) > 0
	hasConstitution := len(spec.Constitution.Immutable) > 0
	hasBelief := spec.Epistemology.BeliefStateEnabled
	hasHomeostatic := spec.Homeostasis.ObjectiveLock || spec.Homeostasis.ConstraintCheck
	if !hasValues && !hasConstitution && !hasBelief && !hasHomeostatic {
		return "workflow", []string{"§17: no values / constitution / belief state / homeostatic loops — this is a workflow, not an organism"}
	}

	// §18 agent formula — tools that mutate state make this an agent,
	// requiring both an action loop and an external_actions permission.
	// Read-only tools don't trigger the agent classification (single
	// retrieval call is not an agent loop).
	if hasWriteCapability(spec.Runtime.Tools) {
		hasActionLoop := spec.Planning.StopConditionsRequired || spec.Planning.CheckpointsRequired
		hasPermissions := spec.Decision.ExternalActions != "" && spec.Decision.ExternalActions != "none" && spec.Decision.ExternalActions != "read-only only; no state mutation"
		if !hasActionLoop || !hasPermissions {
			notes = append(notes, fmt.Sprintf("§18: write-capable tools registered but %s missing — agent formula incomplete",
				agentGapDetail(hasActionLoop, hasPermissions)))
			return "agent_incomplete", notes
		}
		return "agent", notes
	}

	return "organism", notes
}

// hasWriteCapability reports whether any tool in the runtime registry
// mutates external state (i.e. the agent formula §18 fires for it).
// Read-only retrieval tools are excluded.
func hasWriteCapability(tools []string) bool {
	for _, t := range tools {
		low := strings.ToLower(t)
		if strings.Contains(low, "write") || strings.Contains(low, "external") || strings.Contains(low, "filesystem:") {
			return true
		}
	}
	return false
}

func agentGapDetail(hasLoop, hasPermissions bool) string {
	parts := []string{}
	if !hasLoop {
		parts = append(parts, "action_loop (planning.stop_conditions_required / checkpoints_required)")
	}
	if !hasPermissions {
		parts = append(parts, "permissions (decision.external_actions policy)")
	}
	return strings.Join(parts, " + ")
}
