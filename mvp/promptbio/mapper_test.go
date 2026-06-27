package promptbio

// v0.1 Prompt Genome Mapper tests.
// Coverage targets from handoff Section 12 "Definition of Done":
//   - raw prompt gets low score (Section 5.2);
//   - structured prompt gets higher score (Section 5.3);
//   - 14 loci returned;
//   - mutation plan generated;
//   - missing loci surfaced;
//   - failure modes generated.

import (
	"encoding/json"
	"testing"
)

func TestMapPrompt_RawGTM_LowScore(t *testing.T) {
	// Verbatim test prompt from handoff Section 5.2.
	g := MapPrompt(MapRequest{Prompt: "Напиши хорошую стратегию запуска продукта."})

	if len(g.Loci) != 14 {
		t.Fatalf("expected 14 loci, got %d", len(g.Loci))
	}
	if g.GenomeScore >= 0.30 {
		t.Errorf("raw GTM prompt should score LOW (< 0.30), got %.3f", g.GenomeScore)
	}
	if len(g.MissingLoci) < 8 {
		t.Errorf("raw GTM prompt should surface ≥ 8 missing loci, got %d", len(g.MissingLoci))
	}
	if len(g.MutationPlan) == 0 {
		t.Errorf("mutation plan must be non-empty when loci are missing")
	}
	if len(g.ExpectedPhenotype.FailureModes) == 0 {
		t.Errorf("failure modes must be non-empty for a low-score prompt")
	}
	if g.PromptID == "" {
		t.Errorf("prompt_id must be set")
	}
	if g.Language != "ru" {
		t.Errorf("expected language=ru for the Russian sample, got %q", g.Language)
	}
}

func TestMapPrompt_StructuredGTM_HigherScore(t *testing.T) {
	// Verbatim structured prompt from handoff Section 5.3.
	raw := MapPrompt(MapRequest{Prompt: "Напиши хорошую стратегию запуска продукта."})
	structured := MapPrompt(MapRequest{Prompt: "Ты — senior GTM strategist. Сначала отдели facts, assumptions, unknowns. Учитывай бюджет, команду и ограничения. Дай план запуска в формате: summary, segment, channels, experiments, metrics, risks, next actions. Перед финалом проверь, что рекомендации не нарушают constraints."})

	if structured.GenomeScore <= raw.GenomeScore {
		t.Errorf("structured prompt must score HIGHER than raw, got %.3f vs raw=%.3f", structured.GenomeScore, raw.GenomeScore)
	}

	// Per the handoff's expected results for the structured prompt:
	// role: strong, epistemic: strong, output_schema: strong.
	mustBeAtLeast := map[LocusName]LocusStatus{
		LocusRole:       LocusStrong,
		LocusEpistemic:  LocusStrong,
		LocusOutput:     LocusStrong,
		LocusConstraint: LocusPresent,
		LocusValidation: LocusPresent,
	}
	statusByName := map[LocusName]LocusStatus{}
	for _, l := range structured.Loci {
		statusByName[l.Name] = l.Status
	}
	for name, want := range mustBeAtLeast {
		got := statusByName[name]
		if statusRank(got) < statusRank(want) {
			t.Errorf("locus %q: got status %q, want at least %q", name, got, want)
		}
	}
}

func TestMapPrompt_EmptyPrompt_AllMissing(t *testing.T) {
	g := MapPrompt(MapRequest{Prompt: ""})
	if g.GenomeScore != 0 {
		t.Errorf("empty prompt must score 0, got %.3f", g.GenomeScore)
	}
	if len(g.MissingLoci) != 14 {
		t.Errorf("empty prompt should mark all 14 loci missing, got %d", len(g.MissingLoci))
	}
	if g.ExpectedPhenotype.Confidence != "high" {
		t.Errorf("empty-prompt confidence should be high, got %q", g.ExpectedPhenotype.Confidence)
	}
}

func TestMapPrompt_DeterministicPromptID(t *testing.T) {
	a := MapPrompt(MapRequest{Prompt: "Write a strategy."})
	b := MapPrompt(MapRequest{Prompt: "Write a strategy."})
	if a.PromptID != b.PromptID {
		t.Errorf("prompt_id should be deterministic for same input, got %q vs %q", a.PromptID, b.PromptID)
	}
	if a.GenomeScore != b.GenomeScore {
		t.Errorf("genome_score should be deterministic for same input, got %.6f vs %.6f", a.GenomeScore, b.GenomeScore)
	}
}

func TestMapPrompt_DetectsEnglishLanguage(t *testing.T) {
	g := MapPrompt(MapRequest{Prompt: "Write a product launch strategy for an enterprise SaaS audience."})
	if g.Language != "en" {
		t.Errorf("expected language=en, got %q", g.Language)
	}
}

func TestMapPrompt_JSONSerialisationStable(t *testing.T) {
	g := MapPrompt(MapRequest{Prompt: "Write a short summary."})
	b, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if len(b) == 0 {
		t.Fatal("empty JSON output")
	}
	// Round-trip through json.Unmarshal to confirm the public shape is sound.
	var back GenomeMap
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if back.GenomeScore != g.GenomeScore {
		t.Errorf("round-trip score drift: %v → %v", g.GenomeScore, back.GenomeScore)
	}
	if len(back.Loci) != len(g.Loci) {
		t.Errorf("round-trip loci length drift: %d → %d", len(g.Loci), len(back.Loci))
	}
}

func TestMapPrompt_RoleDetection(t *testing.T) {
	g := MapPrompt(MapRequest{Prompt: "You are a senior research scientist. Summarise the paper."})
	for _, l := range g.Loci {
		if l.Name == LocusRole && l.Status != LocusStrong {
			t.Errorf("expected Role=strong with 'you are a senior', got %q", l.Status)
		}
	}
}

func TestMapPrompt_OutputSchemaDetectsStrong(t *testing.T) {
	// Mirrors the Section 5.3 cue: the structured GTM prompt's
	// "summary, segment, channels, experiments, metrics, risks, next actions"
	// schema should classify Output as strong.
	g := MapPrompt(MapRequest{Prompt: "Plan in the format: summary, segment, channels, experiments, metrics, risks, next actions."})
	for _, l := range g.Loci {
		if l.Name == LocusOutput && l.Status != LocusStrong {
			t.Errorf("expected Output=strong with named-sections schema, got %q", l.Status)
		}
	}
}

func TestMapPrompt_MutationPlanCoversMissingLoci(t *testing.T) {
	g := MapPrompt(MapRequest{Prompt: "Write a strategy."})
	missing := map[LocusName]bool{}
	for _, n := range g.MissingLoci {
		missing[n] = true
	}
	covered := 0
	for _, m := range g.MutationPlan {
		if missing[LocusName(m.TargetLocus)] {
			covered++
		}
	}
	if covered != len(missing) {
		t.Errorf("mutation plan should cover every missing locus: covered=%d, missing=%d", covered, len(missing))
	}
}

func TestMapPrompt_TestsToRunAlwaysNonEmpty(t *testing.T) {
	g := MapPrompt(MapRequest{Prompt: "Write a strategy."})
	if len(g.TestsToRun) == 0 {
		t.Error("tests_to_run must always have at least one entry")
	}
}

// ---- helper ----

// statusRank gives a comparable order: missing < weak < present < strong.
// not_applicable and conflicting are off-axis; treat as 0.
func statusRank(s LocusStatus) int {
	switch s {
	case LocusMissing:
		return 0
	case LocusWeak:
		return 1
	case LocusPresent:
		return 2
	case LocusStrong:
		return 3
	}
	return 0
}
