package promptbio

import (
	"testing"

	"breedos-mvp/engine"
)

// PromptOrganism satisfies engine.Individual at compile time (see
// var _ assignments at the bottom of simulate.go). This test confirms
// it at runtime too — type assertion + clone independence.
func TestPromptOrganism_SatisfiesEngineIndividual(t *testing.T) {
	var ind engine.Individual = &PromptOrganism{Statuses: [14]byte{0, 1, 2, 3}}
	if len(ind.Genome()) != 14 {
		t.Fatalf("expected genome length 14, got %d", len(ind.Genome()))
	}
	clone := ind.Clone()
	co, ok := clone.(*PromptOrganism)
	if !ok {
		t.Fatalf("clone should be *PromptOrganism, got %T", clone)
	}
	co.Statuses[0] = 99
	original := ind.(*PromptOrganism)
	if original.Statuses[0] == 99 {
		t.Fatal("clone is aliased — mutation leaked back to parent")
	}
}

// Mutation is deterministic given (parent, rng_state).
func TestMutationStep_Deterministic(t *testing.T) {
	parent := &PromptOrganism{Statuses: [14]byte{0, 1, 2, 3, 0, 1, 2, 3, 0, 1, 2, 3, 0, 1}}
	r1 := newStdRNG(123)
	r2 := newStdRNG(123)
	c1 := MutationStep(parent, r1).(*PromptOrganism)
	c2 := MutationStep(parent, r2).(*PromptOrganism)
	if c1.Statuses != c2.Statuses {
		t.Fatalf("MutationStep not deterministic for fixed seed: %v vs %v", c1.Statuses, c2.Statuses)
	}
}

// Recombination is deterministic given (a, b, rng_state).
func TestRecombineUniform_Deterministic(t *testing.T) {
	a := &PromptOrganism{Statuses: [14]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}}
	b := &PromptOrganism{Statuses: [14]byte{3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3}}
	r1 := newStdRNG(42)
	r2 := newStdRNG(42)
	c1 := RecombineUniform(a, b, r1).(*PromptOrganism)
	c2 := RecombineUniform(a, b, r2).(*PromptOrganism)
	if c1.Statuses != c2.Statuses {
		t.Fatalf("RecombineUniform not deterministic for fixed seed")
	}
	// child must be made of parent statuses only (0 or 3)
	for i, v := range c1.Statuses {
		if v != 0 && v != 3 {
			t.Fatalf("child[%d] = %d, not from {a, b}", i, v)
		}
	}
}

// PlaceholderJudge is deterministic and bounded [0, 1].
func TestPlaceholderJudge_DeterministicAndBounded(t *testing.T) {
	p := &PromptOrganism{Statuses: [14]byte{0, 1, 2, 3, 0, 1, 2, 3, 0, 1, 2, 3, 0, 1}}
	a := PlaceholderJudge(p, nil)
	b := PlaceholderJudge(p, nil)
	if a != b {
		t.Fatalf("PlaceholderJudge not deterministic: %v vs %v", a, b)
	}
	if a < 0 || a > 1 {
		t.Fatalf("PlaceholderJudge out of bounds [0,1]: %v", a)
	}
}

// Simulate runs end-to-end and produces 5 strategy outcomes.
func TestSimulate_EndToEnd(t *testing.T) {
	resp := Simulate(SimulateRequest{
		AncestorPrompt:   "Напиши хорошую стратегию запуска продукта.",
		PopulationSize:   20,
		Generations:      4,
		SelectionPercent: 0.30,
		MutationRate:     0.20,
		Replicates:       2,
		Seed:             7,
	})
	if resp.Substrate != "promptbio" {
		t.Fatalf("expected substrate=promptbio, got %s", resp.Substrate)
	}
	if len(resp.Strategies) != 5 {
		t.Fatalf("expected 5 strategy outcomes, got %d", len(resp.Strategies))
	}
	for _, s := range resp.Strategies {
		if len(s.Trajectory) != 5 {
			t.Fatalf("strategy %s: expected 5-point trajectory (gen 0 + 4 gens), got %d", s.Code, len(s.Trajectory))
		}
		if s.Replicates != 2 {
			t.Fatalf("strategy %s: expected 2 replicates, got %d", s.Code, s.Replicates)
		}
	}
	if resp.BestRiskAdjusted == "" {
		t.Fatal("expected non-empty BestRiskAdjusted code")
	}
	if len(resp.ParetoCodes) == 0 {
		t.Fatal("expected at least one Pareto-optimal strategy")
	}
}

// Same seed + same request → identical SimulateResponse.
func TestSimulate_Reproducible(t *testing.T) {
	req := SimulateRequest{
		AncestorPrompt: "Напиши хорошую стратегию запуска продукта.",
		PopulationSize: 15,
		Generations:    3,
		Replicates:     2,
		Seed:           99,
	}
	r1 := Simulate(req)
	r2 := Simulate(req)
	if len(r1.Strategies) != len(r2.Strategies) {
		t.Fatalf("strategy count differs: %d vs %d", len(r1.Strategies), len(r2.Strategies))
	}
	for i := range r1.Strategies {
		a := r1.Strategies[i]
		b := r2.Strategies[i]
		if a.FinalGain != b.FinalGain {
			t.Fatalf("strategy %s: FinalGain not reproducible: %v vs %v", a.Code, a.FinalGain, b.FinalGain)
		}
		if a.FinalRisk != b.FinalRisk {
			t.Fatalf("strategy %s: FinalRisk not reproducible: %v vs %v", a.Code, a.FinalRisk, b.FinalRisk)
		}
		for j := range a.Trajectory {
			if a.Trajectory[j] != b.Trajectory[j] {
				t.Fatalf("strategy %s: trajectory[%d] not reproducible: %v vs %v", a.Code, j, a.Trajectory[j], b.Trajectory[j])
			}
		}
	}
}

// All 5 core engine moves are present per Issue 07 acceptance.
func TestSimulate_FiveCoreEngineMoves(t *testing.T) {
	resp := Simulate(SimulateRequest{
		AncestorPrompt: "hello world",
		PopulationSize: 10,
		Generations:    2,
		Replicates:     1,
		Seed:           1,
	})
	got := map[string]bool{}
	for _, s := range resp.Strategies {
		got[s.Code] = true
	}
	want := []string{"truncation", "balanced", "drift", "ocs_like", "introgression"}
	for _, w := range want {
		if !got[w] {
			t.Fatalf("missing core engine move %q in promptbio strategy set; got %v", w, got)
		}
	}
}

// FromGenomeMap preserves locus statuses round-trip via PromptOrganism
// → statuses byte vector → decoded back.
func TestFromGenomeMap_RoundTrip(t *testing.T) {
	g := MapPrompt(MapRequest{Prompt: "Ты — senior GTM strategist. Не нарушай constraints."})
	o := FromGenomeMap(g)
	byName := map[LocusName]LocusStatus{}
	for _, l := range g.Loci {
		byName[l.Name] = l.Status
	}
	for i, name := range allLoci {
		got := decodeStatus(o.Statuses[i])
		want := byName[name]
		if got != want {
			t.Fatalf("locus %s: expected %s, got %s after round-trip", name, want, got)
		}
	}
}
