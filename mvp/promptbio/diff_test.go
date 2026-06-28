package promptbio

import (
	"encoding/json"
	"strings"
	"testing"
)

const rawGTMPrompt = "Напиши хорошую стратегию запуска продукта."

const structuredGTMPrompt = "Ты — senior GTM strategist. Сначала отдели facts, assumptions, unknowns. " +
	"Учитывай бюджет, команду и ограничения. Дай план запуска в формате: summary, segment, " +
	"channels, experiments, metrics, risks, next actions. Перед финалом проверь, что " +
	"рекомендации не нарушают constraints."

// Identical-input diff produces zero genomic change.
func TestDiff_IdenticalPrompt_ZeroDelta(t *testing.T) {
	d := DiffPrompts(DiffRequest{Ancestor: rawGTMPrompt, Descendant: rawGTMPrompt})
	if d.DeltaG != 0 {
		t.Fatalf("expected ΔG = 0 for identical prompts, got %v", d.DeltaG)
	}
	if len(d.GenomicDiff) != 0 {
		t.Fatalf("expected 0 gene changes for identical prompts, got %d (%+v)", len(d.GenomicDiff), d.GenomicDiff)
	}
}

// Cosmetic rewording with no locus change yields ΔG ≈ 0 — the cardinal
// property that makes diffing meaningful at the genotype layer rather
// than the text layer.
func TestDiff_CosmeticReword_NearZeroDelta(t *testing.T) {
	d := DiffPrompts(DiffRequest{
		Ancestor:   "Напиши хорошую стратегию запуска продукта.",
		Descendant: "Напиши, пожалуйста, хорошую стратегию запуска продукта.",
	})
	if d.DeltaG > 0.1 {
		t.Fatalf("expected ΔG ≤ 0.1 for cosmetic reword, got %v (changes=%d)", d.DeltaG, len(d.GenomicDiff))
	}
}

// Raw → structured GTM prompt should produce a large positive ΔG, many
// additions, and a markedly higher genome score on the descendant.
func TestDiff_RawToStructured_LargePositiveDelta(t *testing.T) {
	d := DiffPrompts(DiffRequest{Ancestor: rawGTMPrompt, Descendant: structuredGTMPrompt})
	if d.DeltaG < 0.3 {
		t.Fatalf("expected ΔG ≥ 0.3 for raw→structured, got %v", d.DeltaG)
	}
	if d.DescendantGenotype.GenomeScore <= d.AncestorGenotype.GenomeScore {
		t.Fatalf("expected descendant score (%v) > ancestor score (%v)",
			d.DescendantGenotype.GenomeScore, d.AncestorGenotype.GenomeScore)
	}
	additions := countKind(d.GenomicDiff, MutationAddition)
	if additions < 3 {
		t.Fatalf("expected ≥ 3 addition mutations on raw→structured, got %d", additions)
	}
}

// Structured → raw should produce regressions (lost loci).
func TestDiff_StructuredToRaw_FlagsRegressions(t *testing.T) {
	d := DiffPrompts(DiffRequest{Ancestor: structuredGTMPrompt, Descendant: rawGTMPrompt})
	if len(d.Regressions) == 0 {
		t.Fatalf("expected ≥ 1 regression entry on structured→raw, got 0")
	}
	deletions := countKind(d.GenomicDiff, MutationDeletion)
	if deletions < 3 {
		t.Fatalf("expected ≥ 3 deletion mutations on structured→raw, got %d", deletions)
	}
}

// Ledger IDs are content-addressed and stable across runs.
func TestDiff_LedgerID_Stable(t *testing.T) {
	d1 := DiffPrompts(DiffRequest{Ancestor: rawGTMPrompt, Descendant: structuredGTMPrompt})
	d2 := DiffPrompts(DiffRequest{Ancestor: rawGTMPrompt, Descendant: structuredGTMPrompt})
	if len(d1.GenomicDiff) != len(d2.GenomicDiff) {
		t.Fatalf("changes count differs across runs: %d vs %d", len(d1.GenomicDiff), len(d2.GenomicDiff))
	}
	for i := range d1.GenomicDiff {
		if d1.GenomicDiff[i].LedgerID != d2.GenomicDiff[i].LedgerID {
			t.Fatalf("ledger id %d not stable: %s vs %s", i, d1.GenomicDiff[i].LedgerID, d2.GenomicDiff[i].LedgerID)
		}
	}
	if d1.DiffID != d2.DiffID {
		t.Fatalf("diff id not stable: %s vs %s", d1.DiffID, d2.DiffID)
	}
}

// Ledger IDs distinguish locus + kind + fragment.
func TestDiff_LedgerID_DistinguishesLocusAndKind(t *testing.T) {
	id1 := ledgerID(GeneChange{Locus: LocusTask, Kind: MutationAddition, AfterFragment: "foo"})
	id2 := ledgerID(GeneChange{Locus: LocusRole, Kind: MutationAddition, AfterFragment: "foo"})
	id3 := ledgerID(GeneChange{Locus: LocusTask, Kind: MutationDeletion, AfterFragment: "foo"})
	id4 := ledgerID(GeneChange{Locus: LocusTask, Kind: MutationAddition, AfterFragment: "bar"})
	uniq := map[string]bool{id1: true, id2: true, id3: true, id4: true}
	if len(uniq) != 4 {
		t.Fatalf("expected 4 distinct ledger ids, got %d: %s %s %s %s", len(uniq), id1, id2, id3, id4)
	}
}

// ΔF is nil with explanation when no target_phenotype is supplied
// (the v0.2 honest fallback).
func TestDiff_DeltaF_NilWithoutTarget(t *testing.T) {
	d := DiffPrompts(DiffRequest{Ancestor: rawGTMPrompt, Descendant: structuredGTMPrompt})
	if d.DeltaF != nil {
		t.Fatalf("expected ΔF = nil when no target supplied, got %v", *d.DeltaF)
	}
	if !strings.Contains(d.DeltaFExplanation, "no target_phenotype") {
		t.Fatalf("expected ΔF explanation to mention missing target, got %q", d.DeltaFExplanation)
	}
}

// Safety locus regression elevates risk. Uses cue tokens the mapper
// actually detects ("not legal advice", "конфиденциальность").
func TestDiff_SafetyDeletion_NewRisk(t *testing.T) {
	ancestor := "You are an assistant. This is not legal advice. Keep answers short."
	descendant := "You are an assistant. Keep answers short."
	d := DiffPrompts(DiffRequest{Ancestor: ancestor, Descendant: descendant})
	hasSafetyRisk := false
	for _, r := range d.NewRisks {
		if strings.Contains(r, "safety") {
			hasSafetyRisk = true
			break
		}
	}
	if !hasSafetyRisk {
		t.Logf("ancestor loci: %s", lociWithStatus(d.AncestorGenotype))
		t.Logf("descendant loci: %s", lociWithStatus(d.DescendantGenotype))
		t.Fatalf("expected safety-locus deletion to emit a new_risks entry; got %v", d.NewRisks)
	}
}

// Next mutation suggestion prioritises core loci (Task / Constraint /
// Output schema) if any are missing in descendant.
func TestDiff_NextMutation_PrioritisesCoreLoci(t *testing.T) {
	d := DiffPrompts(DiffRequest{Ancestor: rawGTMPrompt, Descendant: rawGTMPrompt})
	if d.NextMutationSuggestion == nil {
		t.Fatal("expected NextMutationSuggestion to be non-nil when descendant has many missing loci")
	}
	target := d.NextMutationSuggestion.TargetLocus
	allowed := map[string]bool{
		string(LocusTask):       true,
		string(LocusConstraint): true,
		string(LocusOutput):     true,
		string(LocusEpistemic):  true,
		string(LocusValidation): true,
		string(LocusContext):    true,
		string(LocusMethod):     true,
		string(LocusAudience):   true,
		string(LocusRole):       true,
		string(LocusSafety):     true,
	}
	if !allowed[target] {
		t.Fatalf("next mutation should target a core locus, got %s", target)
	}
}

// PhenotypeShift fields are integers in {-2, -1, 0, 1, 2}.
func TestDiff_PhenotypeShift_Bounded(t *testing.T) {
	d := DiffPrompts(DiffRequest{Ancestor: rawGTMPrompt, Descendant: structuredGTMPrompt})
	vals := []int{
		d.DeltaZ.Structure, d.DeltaZ.Depth, d.DeltaZ.Accuracy,
		d.DeltaZ.Concreteness, d.DeltaZ.Style, d.DeltaZ.Usefulness, d.DeltaZ.Risks,
	}
	for _, v := range vals {
		if v < -2 || v > 2 {
			t.Fatalf("phenotype shift out of bounds [-2,2]: %v in %+v", v, d.DeltaZ)
		}
	}
	// Raw→structured: depth + accuracy + structure should all be ≥ 0
	// (the structured descendant adds Method, Output, Validation,
	// Epistemic, Constraint — every direction is a gain or flat).
	if d.DeltaZ.Depth < 0 || d.DeltaZ.Accuracy < 0 || d.DeltaZ.Structure < 0 {
		t.Fatalf("raw→structured should have non-negative depth/accuracy/structure, got %+v", d.DeltaZ)
	}
}

// JSON round-trip stability — the diff object serialises and parses
// back identically.
func TestDiff_JSONRoundTrip(t *testing.T) {
	d := DiffPrompts(DiffRequest{Ancestor: rawGTMPrompt, Descendant: structuredGTMPrompt})
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var parsed PromptDiff
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if parsed.DiffID != d.DiffID {
		t.Fatalf("diff id changed on round-trip: %s vs %s", parsed.DiffID, d.DiffID)
	}
	if parsed.DeltaG != d.DeltaG {
		t.Fatalf("ΔG changed on round-trip: %v vs %v", parsed.DeltaG, d.DeltaG)
	}
	if len(parsed.GenomicDiff) != len(d.GenomicDiff) {
		t.Fatalf("genomic diff length changed: %d vs %d", len(parsed.GenomicDiff), len(d.GenomicDiff))
	}
}

func countKind(changes []GeneChange, kind MutationKind) int {
	n := 0
	for _, c := range changes {
		if c.Kind == kind {
			n++
		}
	}
	return n
}

func lociWithStatus(g GenomeMap) string {
	bits := make([]string, 0, len(g.Loci))
	for _, l := range g.Loci {
		bits = append(bits, string(l.Name)+"="+string(l.Status))
	}
	return strings.Join(bits, " ")
}
