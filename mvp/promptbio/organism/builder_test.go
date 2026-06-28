package organism

import (
	"encoding/json"
	"strings"
	"testing"
)

// Acceptance criterion: 16-organ catalogue with matching state /
// function / failure_modes fields.
func TestOrganism_OrgansCatalogue16(t *testing.T) {
	o := Organs()
	if len(o) != 16 {
		t.Fatalf("expected 16 organs, got %d", len(o))
	}
	seen := map[OrganName]bool{}
	for _, x := range o {
		if seen[x.Name] {
			t.Errorf("duplicate organ: %s", x.Name)
		}
		seen[x.Name] = true
		if x.State == "" || x.Function == "" || len(x.FailureModes) == 0 || x.WiredToModule == "" {
			t.Errorf("organ %s missing required field", x.Name)
		}
	}
	for _, expected := range AllOrgans {
		if !seen[expected] {
			t.Errorf("missing organ: %s", expected)
		}
	}
}

// Acceptance criterion: 7-flow catalogue.
func TestOrganism_FlowsCatalogue7(t *testing.T) {
	f := Flows()
	if len(f) != 7 {
		t.Fatalf("expected 7 flows, got %d", len(f))
	}
	for _, flow := range f {
		if len(flow.Stages) == 0 {
			t.Errorf("flow %s has no stages", flow.Name)
		}
		if flow.Purpose == "" || flow.FailureMode == "" {
			t.Errorf("flow %s missing required field", flow.Name)
		}
	}
}

// Acceptance criterion: 6-loop catalogue with invariants.
func TestOrganism_LoopsCatalogue6(t *testing.T) {
	l := Loops()
	if len(l) != 6 {
		t.Fatalf("expected 6 loops, got %d", len(l))
	}
	for _, loop := range l {
		if loop.Targets == "" || loop.FailureMode == "" {
			t.Errorf("loop %s missing invariant or failure mode", loop.Name)
		}
		if len(loop.Steps) == 0 {
			t.Errorf("loop %s has no steps", loop.Name)
		}
	}
}

// Principles: 10 entries.
func TestOrganism_Principles10(t *testing.T) {
	p := Principles()
	if len(p) != 10 {
		t.Fatalf("expected 10 principles, got %d", len(p))
	}
	for i, pr := range p {
		if pr.ID != i+1 {
			t.Errorf("principle %d has wrong ID %d", i+1, pr.ID)
		}
		if pr.Name == "" || pr.Statement == "" || pr.ViolationFix == "" {
			t.Errorf("principle %d missing field", pr.ID)
		}
	}
}

// Sizes: 3 organism sizes.
func TestOrganism_Sizes3(t *testing.T) {
	s := Sizes()
	if len(s) != 3 {
		t.Fatalf("expected 3 sizes, got %d", len(s))
	}
	wanted := map[OrganismSize]bool{SizeMicro: true, SizeMeso: true, SizeMacro: true}
	for _, x := range s {
		delete(wanted, x.Size)
		if len(x.RequiredOrgans) == 0 || x.Description == "" {
			t.Errorf("size %s missing field", x.Size)
		}
	}
	if len(wanted) != 0 {
		t.Errorf("missing sizes: %v", wanted)
	}
}

// Acceptance criterion: GTM Product Launch worked example reproduces
// the §16 verbatim values (lines 1490-1620).
func TestOrganism_GTMWorkedExample(t *testing.T) {
	r := Build(BuildRequest{
		UseCase:             "GTM Product Launch Strategy for a B2B SaaS targeting mid-market HR teams",
		TargetPhenotype:     "structured launch strategy: segment, channels, experiments, metrics, risks",
		Deployment:          DeploymentMultiTurn,
		RiskLevel:           RiskMedium,
		DataSources:         []string{"user_input", "docs"},
		Tools:               ToolReadOnly,
		Memory:              MemorySession,
		QualityRequirements: QualityProduction,
		Constraints:         Constraints{Cost: "$250k", Privacy: "GDPR"},
	})
	if r.Spec.PromptOrganism.OrganismType != SizeMeso {
		t.Errorf("expected meso organism, got %s", r.Spec.PromptOrganism.OrganismType)
	}
	if r.Spec.PromptOrganism.Species != "GTM Strategy" {
		t.Errorf("expected species GTM Strategy, got %s", r.Spec.PromptOrganism.Species)
	}
	want := []string{"practical_usefulness", "epistemic_integrity", "user_autonomy"}
	for _, v := range want {
		found := false
		for _, x := range r.Spec.Values.Primary {
			if x == v {
				found = true
			}
		}
		if !found {
			t.Errorf("expected value %s in primary values", v)
		}
	}
}

// Acceptance criterion: autopoiesis != none + no observability → fails Principle 6.
func TestOrganism_Principle6Violation(t *testing.T) {
	spec := PromptOrganismSpec{
		PromptOrganism: Identity{ID: "x", Name: "y", OrganismType: SizeMacro},
		Values:         ValuesSpec{Primary: []string{"epistemic_integrity"}},
		Constitution:   ConstitutionSpec{Immutable: []string{"never invent"}},
		Genome:         GenomeSpec{Spec: "v0.1", CoreGenes: []string{"task"}},
		Epistemology:   EpistemologySpec{BeliefStateEnabled: true, ClaimTypes: []string{"fact"}},
		Decision:       DecisionSpec{Policy: "8-action"},
		Runtime:        RuntimeSpec{StateSchema: "v0.7.37", DefaultProfile: "full"},
		Homeostasis:    HomeostasisSpec{ObjectiveLock: true, ConstraintCheck: true},
		Autopoiesis:    AutopoiesisSpec{Level: AutopoiesisAssistedSelfMaintenance},
		Lifecycle:      LifecycleSpec{Version: "v0.1", ReviewCadence: "annual"},
	}
	report := Validate(spec, SizeMacro)
	found := false
	for _, v := range report.PrincipleViolations {
		if v.PrincipleID == 6 {
			found = true
		}
	}
	if !found {
		t.Fatal("expected Principle 6 violation (autopoiesis without observability)")
	}
}

// Acceptance criterion: tools registered, no external_actions policy
// and no permission gate → fails §18 agent formula.
func TestOrganism_AgentFormulaIncomplete(t *testing.T) {
	spec := PromptOrganismSpec{
		PromptOrganism: Identity{ID: "x", Name: "y", OrganismType: SizeMacro},
		Values:         ValuesSpec{Primary: []string{"epistemic_integrity"}},
		Constitution:   ConstitutionSpec{Immutable: []string{"never invent"}},
		Genome:         GenomeSpec{Spec: "v0.1", CoreGenes: []string{"task"}},
		Epistemology:   EpistemologySpec{BeliefStateEnabled: true, ClaimTypes: []string{"fact"}},
		Decision:       DecisionSpec{Policy: "8-action", ExternalActions: "none"},
		Runtime:        RuntimeSpec{StateSchema: "v0.7.37", DefaultProfile: "full", Tools: []string{"filesystem:write"}},
		Homeostasis:    HomeostasisSpec{ObjectiveLock: true, ConstraintCheck: true},
		Lifecycle:      LifecycleSpec{Version: "v0.1", ReviewCadence: "annual"},
	}
	report := Validate(spec, SizeMacro)
	if report.ClassificationVerdict != "agent_incomplete" {
		t.Fatalf("expected agent_incomplete, got %s", report.ClassificationVerdict)
	}
}

// Acceptance criterion: spec with all organs but no values/constitution
// /belief state is classified as workflow per §17.
func TestOrganism_WorkflowClassification(t *testing.T) {
	spec := PromptOrganismSpec{
		PromptOrganism: Identity{ID: "x", Name: "y", OrganismType: SizeMeso},
		Genome:         GenomeSpec{Spec: "v0.1", CoreGenes: []string{"task"}},
		Runtime:        RuntimeSpec{StateSchema: "v0.7.37", DefaultProfile: "lite"},
		Lifecycle:      LifecycleSpec{Version: "v0.1", ReviewCadence: "annual"},
	}
	report := Validate(spec, SizeMeso)
	if report.ClassificationVerdict != "workflow" {
		t.Fatalf("expected workflow classification, got %s", report.ClassificationVerdict)
	}
}

// Acceptance criterion: empty spec → model_reference_only classification.
func TestOrganism_ModelBoundary(t *testing.T) {
	spec := PromptOrganismSpec{
		PromptOrganism: Identity{ID: "x", Name: "y"},
		Lifecycle:      LifecycleSpec{Version: "v0.1"},
	}
	report := Validate(spec, SizeMicro)
	if report.ClassificationVerdict != "model_reference_only" {
		t.Fatalf("expected model_reference_only, got %s", report.ClassificationVerdict)
	}
}

// Acceptance criterion: §14 MVO template renders 8 steps.
func TestOrganism_MVO8Steps(t *testing.T) {
	r := Build(BuildRequest{
		UseCase:         "Explain a topic to a junior dev",
		TargetPhenotype: "structured explanation with examples",
		Deployment:      DeploymentSingleTurn,
		RiskLevel:       RiskLow,
		QualityRequirements: QualityDraft,
	})
	if len(r.MinimumViableVersion) != 8 {
		t.Fatalf("expected 8-step MVO, got %d", len(r.MinimumViableVersion))
	}
}

// Acceptance criterion: §23 16-step production path.
func TestOrganism_FullProductionPath16Steps(t *testing.T) {
	r := Build(BuildRequest{
		UseCase:             "GTM Product Launch",
		TargetPhenotype:     "production strategy",
		Deployment:          DeploymentProduct,
		RiskLevel:           RiskMedium,
		QualityRequirements: QualityProduction,
	})
	if len(r.FullProductionPath) != 16 {
		t.Fatalf("expected 16-step production path, got %d", len(r.FullProductionPath))
	}
}

// Acceptance criterion: organ_map wires each of the 16 organs to a non-empty
// module reference.
func TestOrganism_OrganMapComplete(t *testing.T) {
	r := Build(BuildRequest{
		UseCase: "x", TargetPhenotype: "y",
	})
	for _, organ := range AllOrgans {
		if r.OrganMap[organ] == "" {
			t.Errorf("organ %s has no module wiring", organ)
		}
	}
}

// JSON round-trip.
func TestOrganism_JSONRoundTrip(t *testing.T) {
	r := Build(BuildRequest{
		UseCase: "x", TargetPhenotype: "y",
		Deployment: DeploymentMultiTurn, RiskLevel: RiskMedium, QualityRequirements: QualityProduction,
	})
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var back BuildResponse
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Spec.PromptOrganism.OrganismType != r.Spec.PromptOrganism.OrganismType {
		t.Fatal("organism type lost")
	}
	if len(back.InformationFlows) != 7 {
		t.Fatal("flows lost")
	}
}

// Diagram contains canonical anchor strings from §12.
func TestOrganism_DiagramAnatomy(t *testing.T) {
	r := Build(BuildRequest{
		UseCase: "GTM Product Launch", TargetPhenotype: "strategy",
		Deployment: DeploymentMultiTurn, RiskLevel: RiskMedium, QualityRequirements: QualityProduction,
	})
	for _, want := range []string{"VALUES", "CONSTITUTION", "GENOME", "RUNTIME", "BELIEF STATE", "DECISION", "VALIDATION GATES", "OBSERVABILITY", "EVALUATION", "AUTOPOIESIS"} {
		if !strings.Contains(r.Diagram, want) {
			t.Errorf("diagram missing %q", want)
		}
	}
}

// Determinism.
func TestOrganism_Deterministic(t *testing.T) {
	a := Build(BuildRequest{UseCase: "x", TargetPhenotype: "y", Deployment: DeploymentMultiTurn, QualityRequirements: QualityProduction})
	b := Build(BuildRequest{UseCase: "x", TargetPhenotype: "y", Deployment: DeploymentMultiTurn, QualityRequirements: QualityProduction})
	if a.Spec.PromptOrganism.ID != b.Spec.PromptOrganism.ID {
		t.Fatalf("non-deterministic: %s vs %s", a.Spec.PromptOrganism.ID, b.Spec.PromptOrganism.ID)
	}
}
