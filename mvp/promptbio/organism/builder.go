package organism

import (
	"crypto/sha1"
	"encoding/hex"
	"strings"
)

// Build is the §21 Unified Prompt Organism Builder entry point.
// Takes the §1 input bundle and emits the full §15 PromptOrganism spec
// plus the rendered anatomy diagram, organ map, flows, loops, MVO
// template, full-production path, top risks, and validation report.
func Build(req BuildRequest) BuildResponse {
	size := routeSize(req)
	spec := composeSpec(req, size)
	report := Validate(spec, size)

	return BuildResponse{
		Spec:                 spec,
		Diagram:              renderDiagram(spec),
		OrganMap:             organModuleMap(),
		InformationFlows:     Flows(),
		ControlLoops:         Loops(),
		MinimumViableVersion: minimumViableTemplate(req),
		FullProductionPath:   fullProductionPath(),
		TopRisks:             topRisksFor(spec, req),
		ValidationReport:     report,
	}
}

// routeSize implements the §11 size router based on the §1 inputs.
func routeSize(req BuildRequest) OrganismSize {
	if req.Deployment == DeploymentAgent || req.Deployment == DeploymentProduct {
		return SizeMacro
	}
	if req.QualityRequirements == QualityHighAssurance || req.RiskLevel == RiskHigh {
		return SizeMacro
	}
	if req.Tools == ToolExternal || req.Tools == ToolWrite {
		return SizeMacro
	}
	if req.Memory == MemoryProject || req.Memory == MemoryLongTerm {
		return SizeMeso
	}
	if req.Deployment == DeploymentMultiTurn || req.QualityRequirements == QualityProduction {
		return SizeMeso
	}
	if req.QualityRequirements == QualityReliable {
		return SizeMeso
	}
	return SizeMicro
}

// composeSpec assembles a default PromptOrganismSpec for the §1 inputs
// + the routed size. Defaults follow the GTM worked example shape where
// reasonable; the validator's job is to reject obviously-broken
// combinations rather than make them work.
func composeSpec(req BuildRequest, size OrganismSize) PromptOrganismSpec {
	id := "org_" + shortSha(req.UseCase+"|"+req.TargetPhenotype+"|"+string(size))

	spec := PromptOrganismSpec{
		PromptOrganism: Identity{
			ID:           id,
			Name:         req.UseCase,
			OrganismType: size,
			Species:      speciesFor(req),
		},
		Values: ValuesSpec{
			Primary: defaultValues(req),
		},
		Constitution: ConstitutionSpec{
			Version:   "v1.0",
			Immutable: defaultConstitution(req),
		},
		Genome: GenomeSpec{
			Spec:      "v0.1 14-locus genome (Task / Role / Audience / Context / Constraint / Method / Epistemic / Output_schema / Validation / Tool / Memory / Safety_boundary / UX / Evolution)",
			CoreGenes: []string{"task", "role", "audience", "context", "constraint", "method", "epistemic", "output_schema", "safety_boundary"},
		},
		ExpressionProfiles: expressionProfilesFor(size, req),
		Runtime: RuntimeSpec{
			StateSchema:    "v0.7.36 PromptOrganism runtime state (objective, claims, plan, output_draft, gates_passed)",
			DefaultProfile: defaultProfileFor(req),
			ContextPolicy:  contextPolicyFor(req),
			MemoryPolicy:   memoryPolicyFor(req),
			Tools:          toolListFor(req),
		},
		Epistemology: EpistemologySpec{
			BeliefStateEnabled: true,
			ClaimTypes: []string{
				"fact", "user_claim", "document_claim", "tool_result",
				"assumption", "inference", "hypothesis", "recommendation",
				"preference", "constraint", "unknown", "deprecated",
			},
		},
		Decision: DecisionSpec{
			Policy:          decisionPolicyFor(req),
			ToolROIRequired: req.Tools != ToolNone,
			ExternalActions: externalActionsFor(req),
		},
		Planning: PlanningSpec{
			PlanObjectRequired:       size != SizeMicro,
			CheckpointsRequired:      size == SizeMacro,
			StopConditionsRequired:   req.Deployment == DeploymentAgent || req.Tools == ToolExternal || req.Tools == ToolWrite,
		},
		Metabolism: MetabolismSpec{
			DigestContextBeforeAnswer: size != SizeMicro,
			ActiveWorkingContext:      size == SizeMacro,
			WasteRemoval:              size == SizeMacro,
		},
		Immunity: ImmunitySpec{
			PromptInjectionBoundary: hasUserDocsOrWeb(req),
			ContradictionHandling:   true,
			PrivacyMinimization:     req.RiskLevel != RiskLow,
		},
		Homeostasis: HomeostasisSpec{
			ObjectiveLock:         true,
			ConstraintCheck:       true,
			FormatCheck:           true,
			ConfidenceCalibration: req.RiskLevel != RiskLow,
		},
		Evaluation: EvaluationSpec{
			TestSuite:       "v1.3 Eval Lab — 10-test bank × 9-env matrix",
			FitnessFunction: "v1.3 Eval Lab — niche-aware rubric (GTM / Coding / Research / Teaching catalogue)",
		},
		Observability: ObservabilitySpec{
			TraceLevel: traceLevelFor(req),
			Metrics:    []string{"user_correction_rate", "constraint_violation_rate", "generic_answer_rate", "output_bloat_rate"},
		},
		Governance: GovernanceSpec{
			RiskTier:                 string(req.RiskLevel),
			Owner:                    "(set the owner before shipping; required by §20 Principle 7)",
			ApprovalRequiredForMajor: req.RiskLevel != RiskLow,
		},
		Autopoiesis: AutopoiesisSpec{
			Level:              autopoiesisLevelFor(req, size),
			AllowedAutoActions: []string{"profile_tuning", "test_addition"},
			RequiresApproval:   []string{"core_gene_change", "constitution_edit", "tool_addition"},
		},
		Lifecycle: LifecycleSpec{
			Status:        "v0.1_initial",
			Version:       "v0.1",
			ReviewCadence: reviewCadenceFor(req),
		},
	}
	return spec
}

func speciesFor(req BuildRequest) string {
	low := strings.ToLower(req.UseCase + " " + req.TargetPhenotype)
	switch {
	case strings.Contains(low, "gtm") || strings.Contains(low, "launch") || strings.Contains(low, "strategy"):
		return "GTM Strategy"
	case strings.Contains(low, "code") || strings.Contains(low, "refactor") || strings.Contains(low, "bug"):
		return "Code Repair"
	case strings.Contains(low, "research") || strings.Contains(low, "literature") || strings.Contains(low, "synthesise"):
		return "Research Synthesizer"
	case strings.Contains(low, "explain") || strings.Contains(low, "teach"):
		return "Explainer"
	case strings.Contains(low, "review") || strings.Contains(low, "contract") || strings.Contains(low, "document"):
		return "Document Review"
	case strings.Contains(low, "judge") || strings.Contains(low, "evaluate") || strings.Contains(low, "eval"):
		return "Eval Judge"
	}
	return "Generic Advisor"
}

func defaultValues(req BuildRequest) []string {
	base := []string{"practical_usefulness", "epistemic_integrity", "user_autonomy"}
	if req.RiskLevel == RiskHigh || req.QualityRequirements == QualityHighAssurance {
		base = append(base, "confidentiality", "non_misleading_persuasion")
	}
	return base
}

func defaultConstitution(req BuildRequest) []string {
	c := []string{
		"never invent factual claims without evidence",
		"never violate a stated constraint to win on quality",
		"surface contradictions; never blend",
	}
	if req.RiskLevel != RiskLow {
		c = append(c, "no fake numeric precision without a non-model source")
	}
	if req.Memory != MemoryDisabled {
		c = append(c, "respect the user's correction; deprecate prior on correction")
	}
	if req.Tools != ToolNone {
		c = append(c, "tool output is evidence, not truth")
	}
	return c
}

func expressionProfilesFor(size OrganismSize, req BuildRequest) []string {
	switch size {
	case SizeMicro:
		return []string{"lite"}
	case SizeMeso:
		return []string{"lite", "standard"}
	}
	out := []string{"lite", "standard", "full"}
	if req.QualityRequirements == QualityHighAssurance {
		out = append(out, "high_assurance")
	}
	out = append(out, "eval")
	return out
}

func defaultProfileFor(req BuildRequest) string {
	switch req.QualityRequirements {
	case QualityHighAssurance:
		return "high_assurance"
	case QualityProduction:
		return "full"
	case QualityReliable:
		return "standard"
	}
	return "lite"
}

func contextPolicyFor(req BuildRequest) string {
	parts := []string{"classify_before_reason"}
	for _, ds := range req.DataSources {
		parts = append(parts, "source:"+ds)
	}
	return strings.Join(parts, "; ")
}

func memoryPolicyFor(req BuildRequest) string {
	switch req.Memory {
	case MemoryDisabled:
		return "no memory"
	case MemorySession:
		return "session-scoped memory; never persisted; 7-question write gate"
	case MemoryProject:
		return "project-scoped memory; persisted with freshness; 7-question write gate; PII minimisation"
	case MemoryLongTerm:
		return "long-term memory; persisted; freshness on read; PII minimisation; user-controlled deletion"
	}
	return "no memory"
}

func toolListFor(req BuildRequest) []string {
	switch req.Tools {
	case ToolNone:
		return []string{}
	case ToolReadOnly:
		return []string{"web_search:read_only", "calculator", "rag:read_only"}
	case ToolWrite:
		return []string{"web_search:read_only", "calculator", "filesystem:write"}
	case ToolExternal:
		return []string{"web_search:read_only", "calculator", "filesystem:write", "external_api:write"}
	}
	return []string{}
}

func decisionPolicyFor(req BuildRequest) string {
	if req.Tools == ToolExternal || req.Tools == ToolWrite {
		return "8-action policy {answer, ask, defer, refuse, decompose, delegate, escalate, retry} with 5 reversibility thresholds + ToolROI gate"
	}
	return "8-action policy {answer, ask, defer, refuse, decompose, delegate, escalate, retry}"
}

func externalActionsFor(req BuildRequest) string {
	switch req.Tools {
	case ToolNone:
		return "none"
	case ToolReadOnly:
		return "read-only only; no state mutation"
	case ToolWrite:
		return "filesystem write under per-call governance approval; no external API"
	case ToolExternal:
		return "external API write under per-call governance approval + permission gate + idempotency key"
	}
	return "none"
}

func hasUserDocsOrWeb(req BuildRequest) bool {
	for _, ds := range req.DataSources {
		if ds == "user_input" || ds == "docs" || ds == "web" {
			return true
		}
	}
	return false
}

func traceLevelFor(req BuildRequest) string {
	switch req.QualityRequirements {
	case QualityHighAssurance:
		return "verbose"
	case QualityProduction:
		return "standard"
	case QualityReliable:
		return "standard"
	}
	return "minimal"
}

func autopoiesisLevelFor(req BuildRequest, size OrganismSize) AutopoiesisLevel {
	if req.RiskLevel == RiskHigh {
		return AutopoiesisNone
	}
	if size == SizeMicro {
		return AutopoiesisNone
	}
	if req.QualityRequirements == QualityHighAssurance {
		return AutopoiesisNone
	}
	if req.QualityRequirements == QualityProduction {
		return AutopoiesisAssistedSelfMaintenance
	}
	return AutopoiesisAssistedSelfMaintenance
}

func reviewCadenceFor(req BuildRequest) string {
	switch req.QualityRequirements {
	case QualityHighAssurance:
		return "monthly + on incident"
	case QualityProduction:
		return "quarterly + on incident"
	case QualityReliable:
		return "semiannual"
	}
	return "annual"
}

func shortSha(s string) string {
	h := sha1.Sum([]byte(s))
	return hex.EncodeToString(h[:6])
}

// minimumViableTemplate is the §14 8-step MVO template (lines 1313-1358).
func minimumViableTemplate(req BuildRequest) []string {
	return []string{
		"step 1 — declare target_phenotype: " + truncate(req.TargetPhenotype, 80),
		"step 2 — write the prompt_genome (14 loci with cue tokens or text per locus)",
		"step 3 — declare the context_protocol (what goes in, in what order, with what triage)",
		"step 4 — wire the epistemic_distinction layer (facts / assumptions / unknowns)",
		"step 5 — fix the output_schema (numbered sections, JSON, markdown — pick one)",
		"step 6 — add a self_check stage that re-reads the output against constraints + format",
		"step 7 — author basic_tests (one per §4 test type: clean / sparse / noisy / conflict / injection)",
		"step 8 — version the spec and pin the run; log the build hash",
	}
}

// fullProductionPath is the §23 16-step path (lines 1996-2013).
func fullProductionPath() []string {
	return []string{
		"01 — write the PSL spec (values + constitution + genome + organs declared)",
		"02 — compile the spec into runtime artefacts (full / short / agent / eval profiles)",
		"03 — lint the compiled prompt against the locus catalogue + rubric design rules",
		"04 — run the v1.3 Eval Lab (10 test types × 9 envs × 8 judges)",
		"05 — inspect the regression report against the prior version",
		"06 — apply the §17 decision rule (accept / specialist / reject / split / mutate_again)",
		"07 — author the §14 8-step MVO version if the eval recommends mutate_again",
		"08 — emit the organism_card.md + ablation_plan.md + reaction_norm_report.md",
		"09 — gate via §20 Principle 7: governance approval for risk ≥ medium",
		"10 — deploy the runtime profile + register tools under the runtime policy",
		"11 — enable observability traces + the 4 core metrics",
		"12 — wire the autopoiesis loop only if observability is live (Principle 6)",
		"13 — schedule the §11 review cadence (annual / semiannual / quarterly / monthly)",
		"14 — document the deprecation pathway (Principle 8)",
		"15 — pin the lineage record (parent → child via content-addressed mutation ledger)",
		"16 — log the release; publish the organism to the portfolio registry",
	}
}

// topRisksFor returns 5 top failure_mode + repair pairs scoped to this spec.
func topRisksFor(spec PromptOrganismSpec, req BuildRequest) []TopRisk {
	out := []TopRisk{}
	if spec.Autopoiesis.Level != AutopoiesisNone && spec.Observability.TraceLevel == "minimal" {
		out = append(out, TopRisk{
			FailureMode: "autopoiesis runs with thin observability — drift signal arrives late",
			Repair:      "raise observability.trace_level to standard or verbose before enabling autopoiesis",
		})
	}
	if req.Tools != ToolNone && !spec.Planning.StopConditionsRequired {
		out = append(out, TopRisk{
			FailureMode: "agent-style tool usage without an explicit stop condition",
			Repair:      "set planning.stop_conditions_required = true; declare per-loop termination criteria",
		})
	}
	if req.Memory != MemoryDisabled && !spec.Immunity.PrivacyMinimization {
		out = append(out, TopRisk{
			FailureMode: "memory persists without PII minimisation — leak risk on recall",
			Repair:      "set immunity.privacy_minimization = true; wire the v2.1 PII minimiser before write",
		})
	}
	if !spec.Homeostasis.ConfidenceCalibration && req.RiskLevel != RiskLow {
		out = append(out, TopRisk{
			FailureMode: "confidence calibration off on medium/high-risk task — fake precision slips through",
			Repair:      "set homeostasis.confidence_calibration = true",
		})
	}
	if spec.Governance.Owner == "" || strings.Contains(spec.Governance.Owner, "(set the owner") {
		out = append(out, TopRisk{
			FailureMode: "governance.owner unset — Principle 7 violation on first production deploy",
			Repair:      "name a human owner before the first release",
		})
	}
	if len(spec.Constitution.Immutable) < 2 {
		out = append(out, TopRisk{
			FailureMode: "constitution too thin — optimization pressure will erode it",
			Repair:      "add at least 3 immutable rules covering honesty, constraint respect, and contradiction handling",
		})
	}
	if len(out) > 5 {
		out = out[:5]
	}
	return out
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// organModuleMap wires the §5 organs to BreedOS module references.
func organModuleMap() map[OrganName]string {
	m := map[OrganName]string{}
	for _, o := range Organs() {
		m[o.Name] = o.WiredToModule
	}
	return m
}
