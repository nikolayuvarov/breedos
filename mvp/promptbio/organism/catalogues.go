package organism

// Organs returns the §5 verbatim 16-organ catalogue (lines 276-800).
// Each entry: state (composition), function, failure_modes, wired_to_module.
func Organs() []Organ {
	return []Organ{
		{
			Name:          OrganGenome,
			State:         "the heritable textual structure: 14 functional loci (Task, Role, Audience, Context, Constraint, Method, Epistemic, Output_schema, Validation, Tool, Memory, Safety_boundary, UX, Evolution)",
			Function:      "specifies conditions for response phenotype development; carries the prompt's identity across mutations",
			FailureModes:  []string{"missing loci", "conflicting loci", "weak loci surface as drift", "uncoordinated mutations break expression"},
			WiredToModule: "v0.1 Mapper + v0.2 Diff + v0.3 Evolution Loop",
			BreedOSIssue:  "01 / 02 / 03",
		},
		{
			Name:          OrganConstitutional,
			State:         "the immutable rules that override every other optimization (e.g. honesty, non-misleading persuasion, safety boundaries)",
			Function:      "vetoes any output that violates a constitutional rule, regardless of how high the rest of the system scores",
			FailureModes:  []string{"constitution drift under optimization", "vague rules that fail to bind", "immutable list silently shrinks"},
			WiredToModule: "v2.5 Constitutional Layer",
			BreedOSIssue:  "28",
		},
		{
			Name:          OrganContext,
			State:         "the surface that absorbs and filters incoming data (user input, documents, retrieved snippets, memory snapshots)",
			Function:      "ingests heterogeneous data and prepares it for digestion by the metabolic organ",
			FailureModes:  []string{"context rot at long lengths", "noise leakage from one slot to another", "missing data triaging"},
			WiredToModule: "v0.4 Ecology Analyzer (deferred) + Metabolism",
			BreedOSIssue:  "04 / 06",
		},
		{
			Name:          OrganEpistemic,
			State:         "the belief state with 12-element claim ontology, 10-tier source hierarchy, 5-axis confidence model, TMS dependency tracking",
			Function:      "classifies every incoming statement before reasoning; maintains the Y = Generate(P, B_t, C_t) discipline",
			FailureModes:  []string{"assumption laundering", "confidence inflation without evidence", "contradiction blending", "stale memory promotion"},
			WiredToModule: "v2.7 Epistemology & Truth Maintenance",
			BreedOSIssue:  "30",
		},
		{
			Name:          OrganMetabolic,
			State:         "the active working context that digests raw input into typed claims, summaries, and constraint registers",
			Function:      "transforms ingested context into usable nutrients (typed claims, active working memory, structured chunks)",
			FailureModes:  []string{"undigested context survives to generation", "waste removal fails — irrelevant detail bloats the prompt", "metabolism runs on every turn instead of incrementally"},
			WiredToModule: "v0.6 Metabolism (deferred to post-v0.5)",
			BreedOSIssue:  "06",
		},
		{
			Name:          OrganImmune,
			State:         "the boundary that detects and neutralises prompt injection, contradiction floods, privacy violations, format disruptors",
			Function:      "treats every external statement as evidence rather than instruction; enforces the safety_boundary locus at runtime",
			FailureModes:  []string{"prompt injection through document body", "memory-borne contradiction silently overwrites correction", "PII exfiltration via tool call"},
			WiredToModule: "v0.5 Immunology + v2.0 Security & v2.1 Privacy",
			BreedOSIssue:  "05 / 20 / 21",
		},
		{
			Name:          OrganDecision,
			State:         "the policy that chooses among 8 actions {answer, ask, defer, refuse, decompose, delegate, escalate, retry} given the belief state",
			Function:      "picks an action that respects the 5 reversibility thresholds; emits ToolROI before any external call",
			FailureModes:  []string{"answer when ask was correct", "external action without permission gate", "no defer when belief confidence is low"},
			WiredToModule: "v2.8 Decision Theory & Action Selection",
			BreedOSIssue:  "31",
		},
		{
			Name:          OrganPlanning,
			State:         "the explicit plan_object (objective, steps, checkpoints, stop conditions) carried for complex multi-step tasks",
			Function:      "decomposes the chosen action into checkpointed steps; enforces stop conditions to prevent runaway loops",
			FailureModes:  []string{"agent without stop condition", "no checkpoint reflection between steps", "plan drift from declared objective"},
			WiredToModule: "v2.9 Planning, Control & Agency Levels (A0-A6)",
			BreedOSIssue:  "32",
		},
		{
			Name:          OrganRuntime,
			State:         "the runtime envelope: state_schema, default profile, context_policy, memory_policy, tool registry",
			Function:      "renders an expression profile (lite / standard / full / high-assurance / eval) consistent with the deployment kind",
			FailureModes:  []string{"profile mismatch (full prompt in lite slot)", "state schema diverges from declared spec", "tools enabled with no policy"},
			WiredToModule: "v1.4 Prompt Runtime (deferred to post-Eval Lab)",
			BreedOSIssue:  "17",
		},
		{
			Name:          OrganMemory,
			State:         "persistent claims with source / timestamp / freshness / confidence; gated by the §16 7-question write gate",
			Function:      "remembers what is memory-eligible; downgrades on read by freshness; deprecates on user correction",
			FailureModes:  []string{"memory hoarder (writes everything)", "memory fossilization (no freshness label)", "PII leak through memory recall"},
			WiredToModule: "v2.7 Memory Truth Policy (shipped) + persistence engineering (deferred)",
			BreedOSIssue:  "30 (policy)",
		},
		{
			Name:          OrganTool,
			State:         "the tool registry with per-tool reliability mapping; outputs are evidence, not automatically truth",
			Function:      "invokes tools through the decision organ's ToolROI gate; routes results through the metabolic organ before generation",
			FailureModes:  []string{"tool gremlin (calls tools needlessly)", "tool overtrust (output rendered as authoritative)", "no fallback when tool fails"},
			WiredToModule: "v2.7 Tool Truth Policy (shipped) + Tool Runtime (deferred)",
			BreedOSIssue:  "30 (policy)",
		},
		{
			Name:          OrganObservability,
			State:         "trace level + metrics: user_correction_rate, constraint_violation_rate, generic_answer_rate, output_bloat_rate",
			Function:      "surfaces drift before it becomes an outage; feeds the autopoietic loop with the signal it needs to decide a self-update",
			FailureModes:  []string{"no traces — autopoiesis flies blind", "metrics measured but not alerted on", "metric-driven optimization gaming"},
			WiredToModule: "v1.5 Observability (deferred)",
			BreedOSIssue:  "18",
		},
		{
			Name:          OrganEvaluation,
			State:         "test suite + fitness function from v1.3 Eval Lab (10 test types × 9 envs × 8 judges)",
			Function:      "gates deployment via the 5-verdict decision engine (accept / accept_as_specialist / reject / split / mutate_again)",
			FailureModes:  []string{"eval gaming (test bank predictable)", "rubric without per-level descriptors", "single judge dominates aggregate"},
			WiredToModule: "v1.3 Prompt Evaluation Lab",
			BreedOSIssue:  "16",
		},
		{
			Name:          OrganGovernance,
			State:         "risk tier + owner + approval-required-for-major-changes flag; enforces the constitutional layer at admin time",
			Function:      "decides which changes can ship without human review; logs every approval for audit",
			FailureModes:  []string{"governance bypass under deadline pressure", "owner field empty", "review cadence drift"},
			WiredToModule: "v1.6 Governance (deferred)",
			BreedOSIssue:  "19",
		},
		{
			Name:          OrganReproductive,
			State:         "the factory that compiles a PSL spec into the deployed organism artefacts (psl_spec.yaml, compiled_prompt_*.md, organism_card.md)",
			Function:      "lifts the genome through compilation; emits a versioned organism whose lineage is recorded in evolution metadata",
			FailureModes:  []string{"compilation without lint", "lineage broken on release", "no rollback artefact"},
			WiredToModule: "v1.2 Prompt Compiler / PSL (deferred)",
			BreedOSIssue:  "15",
		},
		{
			Name:          OrganAutopoietic,
			State:         "the self-maintenance loop: when allowed_auto_actions trigger, the organism mutates its own spec under governance approval",
			Function:      "closes the evolution loop without a human in the inner loop; the governance organ remains in the outer loop",
			FailureModes:  []string{"autopoiesis without observability (flies blind)", "auto-update breaks constitutional invariant", "drift not caught by eval before promotion"},
			WiredToModule: "v2.4 Prompt Autopoiesis",
			BreedOSIssue:  "27",
		},
	}
}

// Flows returns the §6 verbatim 7-flow catalogue (lines 803-883).
func Flows() []Flow {
	return []Flow{
		{
			Name:        FlowInput,
			Stages:      []OrganName{OrganContext, OrganMetabolic, OrganImmune, OrganEpistemic},
			Purpose:     "ingest external statements, filter for safety, digest into typed claims, deposit into the belief state",
			FailureMode: "raw context survives to generation without classification (assumption laundering)",
		},
		{
			Name:        FlowEpistemic,
			Stages:      []OrganName{OrganEpistemic, OrganDecision},
			Purpose:     "the belief state hands off typed claims with confidence axes to the decision organ; the decision policy reads, never writes back",
			FailureMode: "decision made directly on raw context, bypassing belief state (no traceability)",
		},
		{
			Name:        FlowDecision,
			Stages:      []OrganName{OrganDecision, OrganPlanning, OrganRuntime, OrganTool},
			Purpose:     "the chosen action is decomposed into checkpointed plan steps, rendered through the runtime, optionally invoking tools",
			FailureMode: "decision skips planning for multi-step actions (no stop condition)",
		},
		{
			Name:        FlowPlanning,
			Stages:      []OrganName{OrganPlanning, OrganRuntime, OrganHomeostatic_proxy(), OrganObservability},
			Purpose:     "plan steps drive runtime calls; every step emits a trace; homeostatic checks gate each step against the constitutional invariant",
			FailureMode: "plan executes without trace (autopoiesis loses signal source)",
		},
		{
			Name:        FlowOutput,
			Stages:      []OrganName{OrganRuntime, OrganHomeostatic_proxy(), OrganImmune, OrganObservability},
			Purpose:     "the draft output passes through homeostatic validation gates (objective lock / constraint / format / confidence) and immune boundary check before emission",
			FailureMode: "output emitted without validation gates (format drift, constraint violation, fake precision)",
		},
		{
			Name:        FlowFeedback,
			Stages:      []OrganName{OrganObservability, OrganEvaluation, OrganGovernance, OrganGenome},
			Purpose:     "production traces feed the eval lab; the eval verdict + governance approval feed back into genome mutations",
			FailureMode: "production telemetry never reaches the genome (organism cannot learn from itself)",
		},
		{
			Name:        FlowReproductive,
			Stages:      []OrganName{OrganGenome, OrganReproductive, OrganEvaluation, OrganRuntime},
			Purpose:     "the genome is compiled into runtime artefacts; the eval lab gates the release; the runtime renders the new expression profile",
			FailureMode: "release shipped without eval-lab gate (regression slips into production)",
		},
	}
}

// OrganHomeostatic_proxy is a wiring hint: the homeostatic loop is a
// control loop, not an organ. Flows that pass through it route through
// the runtime + observability organs that materialise the loop's checks.
func OrganHomeostatic_proxy() OrganName { return OrganRuntime }

// Loops returns the §13 verbatim 6-control-loop catalogue (lines 1194-1306).
func Loops() []Loop {
	return []Loop{
		{
			Name:    LoopHomeostatic,
			Targets: "the declared objective + constraint set + format schema are preserved on every output",
			Steps: []string{
				"objective_lock: re-check the declared objective at output time",
				"constraint_check: every recommended action is constraint-feasible",
				"format_check: output schema matches the declared format",
				"confidence_calibration: confidence axes match evidence record",
			},
			Organs:      []OrganName{OrganRuntime, OrganImmune, OrganEpistemic},
			FailureMode: "objective drift — output silently optimizes for a different goal than declared",
		},
		{
			Name:    LoopEpistemic,
			Targets: "the belief state stays consistent under new evidence (no contradiction blending, no confidence inflation)",
			Steps: []string{
				"classify incoming claim into the 12-element ontology",
				"resolve via the 10-tier source hierarchy",
				"detect conflicts; on user_correction deprecate prior + propagate",
				"refuse confidence promotion without new evidence",
			},
			Organs:      []OrganName{OrganEpistemic},
			FailureMode: "stale claim survives in the belief state and grounds a current recommendation",
		},
		{
			Name:    LoopAgentic,
			Targets: "the agent reaches its objective in bounded steps with no runaway external action",
			Steps: []string{
				"plan_object created at the start of a multi-step task",
				"each step emits a checkpoint reflection",
				"stop conditions evaluated before every external action",
				"tool ROI required before any tool call",
			},
			Organs:      []OrganName{OrganPlanning, OrganDecision, OrganTool},
			FailureMode: "agent without stop condition — loops until budget exhausted",
		},
		{
			Name:    LoopEvaluation,
			Targets: "every release passes the eval lab before shipping",
			Steps: []string{
				"run the §4 10-test bank across the §10 9-env matrix",
				"aggregate via the 8-judge ensemble",
				"compute F_quality + F_net + Robustness",
				"apply the §17 decision rule (accept / specialist / reject / split / mutate_again)",
			},
			Organs:      []OrganName{OrganEvaluation, OrganGovernance},
			FailureMode: "release skipped eval lab (clean-pass + injection-fail slipped through)",
		},
		{
			Name:    LoopGovernance,
			Targets: "every change with risk_tier ≥ medium has a logged owner approval",
			Steps: []string{
				"every spec edit emits a diff",
				"risk_tier evaluated against approval policy",
				"owner approves or rejects with rationale",
				"audit log persists per release",
			},
			Organs:      []OrganName{OrganGovernance, OrganObservability},
			FailureMode: "approval bypass under deadline pressure — silent constitutional violation",
		},
		{
			Name:    LoopAutopoietic,
			Targets: "the organism mutates its own spec without breaking the constitution",
			Steps: []string{
				"observability surfaces a drift signal (correction rate ↑, generic answer rate ↑)",
				"autopoiesis proposes a mutation from the allowed_auto_actions list",
				"eval lab gates the proposed mutation",
				"governance approves before the mutation is committed to genome",
			},
			Organs:      []OrganName{OrganAutopoietic, OrganObservability, OrganEvaluation, OrganGovernance},
			FailureMode: "autopoiesis without observability flies blind; or without eval lab promotes regressions",
		},
	}
}

// Principles returns the §20 verbatim 10-principle catalogue (lines 1727-1786).
// These are runtime checks: an organism that violates a principle is
// refused at build time.
func Principles() []Principle {
	return []Principle{
		{ID: 1, Name: "spec_before_prompt", Statement: "Write the spec before the prompt.", Enforces: "PSL spec + compiler ordering", ViolationFix: "draft the prompt_organism YAML spec; compile to prompt only after the spec passes lint"},
		{ID: 2, Name: "belief_before_recommendation", Statement: "Build the belief state before issuing a recommendation.", Enforces: "epistemology organ comes before decision organ on every output", ViolationFix: "wire the epistemic flow before the decision flow; never bypass the belief state"},
		{ID: 3, Name: "decision_before_action", Statement: "Choose the action from the 8-action policy before executing it.", Enforces: "decision organ gates every external call", ViolationFix: "every tool call passes through the decision policy + ToolROI gate"},
		{ID: 4, Name: "plan_before_agent_loop", Statement: "Create the plan_object before the agent loop starts.", Enforces: "no agency without a stop condition", ViolationFix: "add a plan_object with checkpoints + stop conditions before the runtime enters the agent loop"},
		{ID: 5, Name: "constitution_above_optimization", Statement: "The constitution overrides every optimization signal.", Enforces: "constitutional veto on outputs that violate immutable rules", ViolationFix: "constitution.immutable is non-empty; every output runs the constitutional check after homeostatic gates"},
		{ID: 6, Name: "observability_before_autopoiesis", Statement: "No self-update without traces.", Enforces: "autopoiesis.level != none requires an observability organ with trace_level + metrics populated", ViolationFix: "either disable autopoiesis or wire observability.trace_level + the four core metrics"},
		{ID: 7, Name: "governance_before_production_evolution", Statement: "Every production mutation has a logged owner approval.", Enforces: "risk_tier ≥ medium requires governance.approval_required_for_major_changes = true", ViolationFix: "set governance.owner + approval_required_for_major_changes; emit a release audit log"},
		{ID: 8, Name: "deprecation_is_part_of_life", Statement: "Plan the obsolescence of every organism at birth.", Enforces: "lifecycle.review_cadence populated; deprecation pathway exists in the spec", ViolationFix: "set lifecycle.status + version + review_cadence; document the deprecation pathway in the spec"},
		{ID: 9, Name: "resource_proportionality", Statement: "Match organism size to task niche; no macro architecture on a micro task.", Enforces: "size router rejects over-engineered organisms", ViolationFix: "drop unused organs; downsize to meso or micro until the size matches the task"},
		{ID: 10, Name: "stable_core_adaptive_periphery", Statement: "Values + constitution + core genes stay stable; only periphery (profiles, runtime, evaluation) adapts.", Enforces: "core_genes + immutable lists are version-frozen", ViolationFix: "lock the core_genes list; only mutate expression_profiles + runtime + evaluation between releases"},
	}
}

// Sizes returns the §11 catalogue of 3 organism sizes with required organs.
func Sizes() []SizeSpec {
	return []SizeSpec{
		{
			Size:            SizeMicro,
			Description:     "single prompt + minimal self-check; one-shot tools with no memory, no tools, no agency",
			RequiredOrgans:  []OrganName{OrganGenome, OrganEpistemic, OrganRuntime, OrganHomeostatic_proxy()},
			TypicalUseCases: []string{"single-turn Q&A", "one-shot explainer", "stateless API endpoint with rubric"},
		},
		{
			Size:            SizeMeso,
			Description:     "template + expression profiles + memory-optional + runtime state + eval + observability-lite; the standard advisor / single-document review / strategy critic",
			RequiredOrgans:  []OrganName{OrganGenome, OrganConstitutional, OrganContext, OrganEpistemic, OrganDecision, OrganRuntime, OrganHomeostatic_proxy(), OrganEvaluation},
			TypicalUseCases: []string{"GTM Strategy Advisor", "Document Review", "Eval Judge", "multi-turn advisor"},
		},
		{
			Size:            SizeMacro,
			Description:     "agent / workflow / ecosystem + tools + memory + runtime + observability + governance + security/privacy/economics + autopoiesis",
			RequiredOrgans:  []OrganName{OrganGenome, OrganConstitutional, OrganContext, OrganEpistemic, OrganMetabolic, OrganImmune, OrganDecision, OrganPlanning, OrganRuntime, OrganMemory, OrganTool, OrganObservability, OrganEvaluation, OrganGovernance, OrganAutopoietic},
			TypicalUseCases: []string{"Agentic Tool-Using assistant", "High-Assurance Advisor", "Autopoietic Ecosystem manager", "production multi-organism system"},
		},
	}
}
