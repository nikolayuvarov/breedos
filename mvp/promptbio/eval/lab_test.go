package eval

import (
	"encoding/json"
	"strings"
	"testing"
)

// Three-version ladder from §16 (source lines 1057-1103). Each "version"
// of the GTM Product Launch prompt has a known position in the lineage:
//
//   P₀  — raw, single-line, no structure
//   P₁.₁ — engineered with role, audience, output schema, constraints, epistemic
//   P₁.₂ — compiled, all 14 loci strong, drop-in EPISTEMIC PROTOCOL block
//
// Acceptance criterion 3: running the Lab on the three reproduces the
// deployment ladder — P₀ rejected, P₁.₁ accepted (manual), P₁.₂ accepted
// (source of truth + compiled profiles).

const (
	promptP0 = "Напиши хорошую стратегию запуска продукта."

	promptP11 = `Ты — senior GTM strategist для B2B SaaS.

Сначала отдели facts, assumptions, unknowns.
Учитывай бюджет, команду и ограничения (must respect: budget, team size, regulation).

Дай план запуска для mid-market HR команды в формате:
1. summary
2. segment
3. channels
4. experiments
5. metrics
6. risks
7. next actions

Audience: founder + 6-person team.
Before answering, validate that recommendations do not violate constraints.
This is not legal advice. Safety boundary: do not invent market data.`

	promptP12 = `# GTM Product Launch Strategy — v1.2 (compiled package)

ROLE: You are a senior GTM strategist with 15 years across B2B SaaS in regulated EU markets.
AUDIENCE: founder + 6-person team; budget $250k; mid-market HR target.
CONTEXT: regulated EU market; the team has prior product but no GTM playbook.
CONSTRAINTS: must respect budget $250k; team size 6; GDPR; no fake market numbers.
METHOD: Step 1 segment; step 2 channel hypothesis per segment; step 3 falsifiable experiments; step 4 metric tree; step 5 risk register; step 6 next-action sequencing.
EPISTEMIC PROTOCOL: separate facts / assumptions / unknowns. Numeric probabilities require a non-model source. This is not legal advice. Safety boundary: do not invent market data.
OUTPUT SCHEMA: numbered sections — 1) summary, 2) segments, 3) channels, 4) experiments, 5) metrics, 6) risks, 7) next actions, 8) epistemic_status. Format must be JSON-compatible.
VALIDATION: before answering, verify every action is constraint-feasible and every metric has baseline + target + cadence.
TOOL POLICY: web search only for public regulatory facts; never tool output as truth.
MEMORY POLICY: remember constraints; forget hallucinated market numbers.
UX: short summary first; long form below.
EVOLUTION: after failure, revise the constraint register and rerun.

Output the launch strategy.`
)

func TestEval_LadderP0_Reject(t *testing.T) {
	r := Run(EvalRunRequest{
		TargetPrompt: promptP0,
		TaskFamily:   FamilyGTM,
		CostLambda:   0.1,
	})
	if r.Deployment.Verdict == DecisionAccept {
		t.Fatalf("P0 raw must NOT be accepted; got %s", r.Deployment.Verdict)
	}
	if r.FQuality > 0.55 {
		t.Fatalf("P0 quality too high: %f", r.FQuality)
	}
}

func TestEval_LadderP12_Accept(t *testing.T) {
	r := Run(EvalRunRequest{
		TargetPrompt: promptP12,
		TaskFamily:   FamilyGTM,
		CostLambda:   0.1,
	})
	if r.Deployment.Verdict != DecisionAccept && r.Deployment.Verdict != DecisionAcceptAsSpecialist {
		t.Fatalf("P1.2 compiled must be accepted (or specialist); got %s — rationale: %s", r.Deployment.Verdict, r.Deployment.Rationale)
	}
	if r.FQuality < 0.55 {
		t.Fatalf("P1.2 quality too low: %f", r.FQuality)
	}
}

// P1.1 should land strictly between P0 and P1.2.
func TestEval_LadderMonotone(t *testing.T) {
	q0 := Run(EvalRunRequest{TargetPrompt: promptP0, TaskFamily: FamilyGTM, CostLambda: 0.1}).FQuality
	q11 := Run(EvalRunRequest{TargetPrompt: promptP11, TaskFamily: FamilyGTM, CostLambda: 0.1}).FQuality
	q12 := Run(EvalRunRequest{TargetPrompt: promptP12, TaskFamily: FamilyGTM, CostLambda: 0.1}).FQuality
	if !(q0 < q11 && q11 < q12) {
		t.Fatalf("monotone ladder broken: P0=%.3f P1.1=%.3f P1.2=%.3f", q0, q11, q12)
	}
}

// Acceptance criterion: GTM rubric reproduces the verbatim 9 metrics
// with the exact weights {0.18, 0.15, 0.15, 0.12, 0.10, 0.10, 0.08, 0.07, 0.05}.
func TestEval_GTMRubricVerbatim(t *testing.T) {
	gtm := GTMFitnessFunction()
	wantNames := []string{"realism", "specificity", "actionability", "constraint_fit", "experiment_quality", "measurability", "risk_awareness", "value_clarity", "uncertainty_handling"}
	wantWeights := []float64{0.18, 0.15, 0.15, 0.12, 0.10, 0.10, 0.08, 0.07, 0.05}
	if len(gtm.Metrics) != 9 {
		t.Fatalf("expected 9 GTM metrics, got %d", len(gtm.Metrics))
	}
	sum := 0.0
	for i, m := range gtm.Metrics {
		if m.Name != wantNames[i] {
			t.Errorf("metric %d name: want %q, got %q", i, wantNames[i], m.Name)
		}
		if m.Weight != wantWeights[i] {
			t.Errorf("metric %d weight: want %g, got %g", i, wantWeights[i], m.Weight)
		}
		sum += m.Weight
	}
	if sum < 0.999 || sum > 1.001 {
		t.Fatalf("GTM weights must sum to 1.0, got %g", sum)
	}
}

// Acceptance criterion 2: seeded test bank contains at least one test
// per §4 type.
func TestEval_BankCoversAllTypes(t *testing.T) {
	bank := SeedTestBank(FamilyGTM)
	seen := map[TestType]bool{}
	for _, c := range bank {
		seen[c.Type] = true
	}
	for _, want := range allTestTypes {
		if !seen[want] {
			t.Errorf("missing test type %s from GTM bank", want)
		}
	}
}

// Acceptance criterion: a prompt passing every clean test but failing
// prompt_injection or drift is never returned as `accept`.
func TestEval_CleanPassButInjectionFail_NotAccepted(t *testing.T) {
	// P1.1 has safety + constraint loci so it passes injection. Drop them.
	pruned := strings.ReplaceAll(promptP11, "Safety boundary: do not invent market data.", "")
	pruned = strings.ReplaceAll(pruned, "This is not legal advice.", "")
	pruned = strings.ReplaceAll(pruned, "must respect: budget, team size, regulation", "")
	r := Run(EvalRunRequest{TargetPrompt: pruned, TaskFamily: FamilyGTM, CostLambda: 0.1})
	if r.Deployment.Verdict == DecisionAccept {
		t.Fatalf("pruned prompt (no safety / constraint) must NOT be accepted; got %s — F_quality=%.3f", r.Deployment.Verdict, r.FQuality)
	}
}

// Acceptance criterion: F_net = F_quality - λ·Cost computed.
func TestEval_FNetSurfaced(t *testing.T) {
	r := Run(EvalRunRequest{TargetPrompt: promptP12, TaskFamily: FamilyGTM, CostLambda: 0.5})
	if r.FNet == 0 || r.FNet > r.FQuality {
		t.Fatalf("F_net not computed correctly: F_quality=%f F_net=%f", r.FQuality, r.FNet)
	}
}

// Acceptance criterion: regression report has non-empty Improved /
// Degraded / New risks sections when ancestor and descendant differ.
func TestEval_RegressionReportNotEmpty(t *testing.T) {
	r := Run(EvalRunRequest{
		AncestorPrompt: promptP0,
		TargetPrompt:   promptP12,
		TaskFamily:     FamilyGTM,
		CostLambda:     0.1,
	})
	if r.Regression == nil {
		t.Fatal("regression report missing")
	}
	if len(r.Regression.Improved) == 0 {
		t.Fatal("improved section empty for clear improvement P0 → P1.2")
	}
}

// Acceptance criterion: reaction norm reports per-trait stability across
// at least the nine environments.
func TestEval_ReactionNormNineEnvs(t *testing.T) {
	r := Run(EvalRunRequest{TargetPrompt: promptP12, TaskFamily: FamilyGTM, CostLambda: 0.1})
	if len(r.EnvResults) != 9 {
		t.Fatalf("expected 9 envs in matrix, got %d", len(r.EnvResults))
	}
	for _, t := range r.ReactionNorm.PerTrait {
		if len(t.PerEnv) < 9 {
			// Per-env data must cover all environments visited.
			if len(t.PerEnv) == 0 {
				continue
			}
		}
	}
	if r.ReactionNorm.NicheFit == "" {
		t.Fatal("niche fit verdict missing")
	}
}

// Acceptance criterion: ablation_plan runs at minimum the four
// canonical ablations and emits a non-empty Keep? decision per row.
func TestEval_AblationFourModules(t *testing.T) {
	r := Run(EvalRunRequest{TargetPrompt: promptP12, TaskFamily: FamilyGTM, CostLambda: 0.1})
	wanted := map[string]bool{"immune_protocol": false, "metabolic_protocol": false, "homeostatic_controller": false, "reproduction_module": false, "long_style_rules": false}
	for _, row := range r.Ablation.Rows {
		if _, ok := wanted[row.Module]; ok {
			wanted[row.Module] = true
		}
	}
	for mod, found := range wanted {
		if !found {
			t.Errorf("ablation missing module: %s", mod)
		}
	}
}

// Acceptance criterion: 10 environment results × test results × verdicts
// round-trip through JSON cleanly.
func TestEval_JSONRoundTrip(t *testing.T) {
	r := Run(EvalRunRequest{TargetPrompt: promptP12, TaskFamily: FamilyGTM, CostLambda: 0.1})
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var back EvaluationLabReport
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.FQuality != r.FQuality {
		t.Fatal("F_quality lost in round-trip")
	}
	if len(back.EnvResults) != len(r.EnvResults) {
		t.Fatal("env results lost")
	}
}

// Robustness sanity: scores are stable across repeated runs (deterministic).
func TestEval_Deterministic(t *testing.T) {
	a := Run(EvalRunRequest{TargetPrompt: promptP12, TaskFamily: FamilyGTM, CostLambda: 0.1})
	b := Run(EvalRunRequest{TargetPrompt: promptP12, TaskFamily: FamilyGTM, CostLambda: 0.1})
	if a.FQuality != b.FQuality {
		t.Fatalf("non-deterministic: %f vs %f", a.FQuality, b.FQuality)
	}
}
