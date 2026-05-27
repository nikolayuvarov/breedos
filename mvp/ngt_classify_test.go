package main

import (
	"strings"
	"testing"
)

// Helper: minimal EditCandidate slice of given length.
func nEdits(n int) []EditCandidate {
	out := make([]EditCandidate, n)
	for i := range out {
		out[i] = EditCandidate{Rank: i + 1, Locus: i, Effect: 1.0}
	}
	return out
}

func TestClassifyEditSet_NoEdits(t *testing.T) {
	got := ClassifyEditSet(nil, NGTContext{TargetTraitClass: "productivity", DonorSource: "none"})
	if got.Category != "unclassifiable" {
		t.Errorf("empty edits should be unclassifiable, got %q", got.Category)
	}
	if !strings.Contains(strings.Join(got.Reasons, " "), "No edits") {
		t.Errorf("reasons should mention no edits")
	}
}

func TestClassifyEditSet_MissingTraitClass(t *testing.T) {
	got := ClassifyEditSet(nEdits(3), NGTContext{TargetTraitClass: "", DonorSource: "none"})
	if got.Category != "unclassifiable" {
		t.Errorf("missing trait class should be unclassifiable, got %q", got.Category)
	}
	if !strings.Contains(strings.Join(got.Disqualifiers, " "), "target_trait_class") {
		t.Errorf("disqualifier should mention target_trait_class")
	}
}

func TestClassifyEditSet_InvalidTraitClass(t *testing.T) {
	got := ClassifyEditSet(nEdits(3), NGTContext{TargetTraitClass: "yield_boost", DonorSource: "none"})
	if got.Category != "unclassifiable" {
		t.Errorf("invalid trait class should be unclassifiable, got %q", got.Category)
	}
}

func TestClassifyEditSet_MissingDonorSource(t *testing.T) {
	got := ClassifyEditSet(nEdits(3), NGTContext{TargetTraitClass: "productivity", DonorSource: ""})
	if got.Category != "unclassifiable" {
		t.Errorf("missing donor_source should be unclassifiable, got %q", got.Category)
	}
}

func TestClassifyEditSet_InvalidDonorSource(t *testing.T) {
	got := ClassifyEditSet(nEdits(3), NGTContext{TargetTraitClass: "productivity", DonorSource: "outer-space"})
	if got.Category != "unclassifiable" {
		t.Errorf("invalid donor_source should be unclassifiable, got %q", got.Category)
	}
}

func TestClassifyEditSet_NGT1Happy(t *testing.T) {
	got := ClassifyEditSet(nEdits(5), NGTContext{TargetTraitClass: "productivity", DonorSource: "none"})
	if got.Category != "NGT-1" {
		t.Fatalf("5 edits, productivity, none donor — should be NGT-1; got %q (disq=%v)", got.Category, got.Disqualifiers)
	}
	if len(got.Disqualifiers) != 0 {
		t.Errorf("NGT-1 should have no disqualifiers, got %v", got.Disqualifiers)
	}
	if !strings.Contains(got.ConfidenceNote, "Not legal advice") {
		t.Errorf("ConfidenceNote must contain 'Not legal advice' literal")
	}
}

func TestClassifyEditSet_NGT1AtCountBoundary(t *testing.T) {
	// Exactly 20 — allowed.
	got := ClassifyEditSet(nEdits(20), NGTContext{TargetTraitClass: "quality", DonorSource: "same_species"})
	if got.Category != "NGT-1" {
		t.Errorf("exactly 20 edits should be NGT-1, got %q (disq=%v)", got.Category, got.Disqualifiers)
	}
}

func TestClassifyEditSet_NGT2CountOver(t *testing.T) {
	got := ClassifyEditSet(nEdits(21), NGTContext{TargetTraitClass: "productivity", DonorSource: "none"})
	if got.Category != "NGT-2" {
		t.Fatalf("21 edits should be NGT-2, got %q", got.Category)
	}
	found := false
	for _, d := range got.Disqualifiers {
		if strings.Contains(d, "20-modifications limit") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("disqualifiers should mention 20-modifications limit, got %v", got.Disqualifiers)
	}
}

func TestClassifyEditSet_NGT2HerbicideTolerance(t *testing.T) {
	got := ClassifyEditSet(nEdits(3), NGTContext{TargetTraitClass: "herbicide_tolerance", DonorSource: "none"})
	if got.Category != "NGT-2" {
		t.Errorf("herbicide_tolerance should force NGT-2, got %q", got.Category)
	}
}

func TestClassifyEditSet_NGT2Insecticidal(t *testing.T) {
	got := ClassifyEditSet(nEdits(3), NGTContext{TargetTraitClass: "insecticidal", DonorSource: "none"})
	if got.Category != "NGT-2" {
		t.Errorf("insecticidal should force NGT-2, got %q", got.Category)
	}
}

func TestClassifyEditSet_NGT2CrossSpecies(t *testing.T) {
	got := ClassifyEditSet(nEdits(3), NGTContext{TargetTraitClass: "productivity", DonorSource: "cross_species"})
	if got.Category != "NGT-2" {
		t.Errorf("cross_species donor should force NGT-2, got %q", got.Category)
	}
	found := false
	for _, d := range got.Disqualifiers {
		if strings.Contains(d, "foreign DNA") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("disqualifier should mention foreign DNA, got %v", got.Disqualifiers)
	}
}

func TestClassifyEditSet_NGT2MultipleDisqualifiers(t *testing.T) {
	// 25 edits + insecticidal + cross_species — should accumulate all 3 disqualifiers.
	got := ClassifyEditSet(nEdits(25), NGTContext{TargetTraitClass: "insecticidal", DonorSource: "cross_species"})
	if got.Category != "NGT-2" {
		t.Errorf("multiple disqualifiers should give NGT-2, got %q", got.Category)
	}
	if len(got.Disqualifiers) < 3 {
		t.Errorf("expected ≥3 disqualifiers, got %d: %v", len(got.Disqualifiers), got.Disqualifiers)
	}
}

func TestClassifyEditSet_ConfidenceNoteAlwaysPresent(t *testing.T) {
	cases := []struct {
		edits int
		ctx   NGTContext
	}{
		{0, NGTContext{TargetTraitClass: "productivity", DonorSource: "none"}},
		{5, NGTContext{TargetTraitClass: "productivity", DonorSource: "none"}},
		{5, NGTContext{TargetTraitClass: "insecticidal", DonorSource: "none"}},
		{25, NGTContext{TargetTraitClass: "productivity", DonorSource: "cross_species"}},
		{5, NGTContext{TargetTraitClass: "", DonorSource: ""}},
	}
	for i, c := range cases {
		got := ClassifyEditSet(nEdits(c.edits), c.ctx)
		if !strings.Contains(got.ConfidenceNote, "Not legal advice") {
			t.Errorf("case %d: ConfidenceNote missing 'Not legal advice' literal: %q", i, got.ConfidenceNote)
		}
	}
}

// v0.7.19 — Issue 32. New tests for Path (i) vs Path (ii) split.

// boolPtr is a tiny helper because Go has no literal *bool.
func boolPtr(b bool) *bool { return &b }

func TestClassifyEditSet_DisclaimerCites2026Date(t *testing.T) {
	got := ClassifyEditSet(nEdits(5), NGTContext{TargetTraitClass: "productivity", DonorSource: "none"})
	if !strings.Contains(got.ConfidenceNote, "2026-04-21") {
		t.Errorf("v0.7.19 disclaimer must cite 2026-04-21 adoption date; got %q", got.ConfidenceNote)
	}
}

func TestClassifyEditSet_BackwardCompatDefaultsToSnvOrSmall(t *testing.T) {
	// VariantType omitted ⇒ defaults to snv_or_small Path (i) ⇒ classifier
	// behaves identically to v0.7.18.
	got := ClassifyEditSet(nEdits(5), NGTContext{TargetTraitClass: "productivity", DonorSource: "none"})
	if got.Category != "NGT-1" {
		t.Fatalf("backward-compat: omitted variant_type should yield NGT-1; got %q (disq=%v)", got.Category, got.Disqualifiers)
	}
	// Ensure the Path (i) reason was appended.
	found := false
	for _, r := range got.Reasons {
		if strings.Contains(r, "Path (i)") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a Path (i) reason in result; got %v", got.Reasons)
	}
}

func TestClassifyEditSet_InvalidVariantTypeIsUnclassifiable(t *testing.T) {
	got := ClassifyEditSet(nEdits(5), NGTContext{
		TargetTraitClass: "productivity",
		DonorSource:      "none",
		VariantType:      "frankenedit",
	})
	if got.Category != "unclassifiable" {
		t.Fatalf("invalid variant_type should be unclassifiable; got %q", got.Category)
	}
}

func TestClassifyEditSet_InversionPathIPasses(t *testing.T) {
	// Path (i) inversion: any size, anywhere in genome. No size limit.
	got := ClassifyEditSet(nEdits(5), NGTContext{
		TargetTraitClass: "productivity",
		DonorSource:      "none",
		VariantType:      "inversion",
	})
	if got.Category != "NGT-1" {
		t.Fatalf("inversion should be NGT-1; got %q (disq=%v)", got.Category, got.Disqualifiers)
	}
	found := false
	for _, r := range got.Reasons {
		if strings.Contains(r, "inversion of any size") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected inversion reason; got %v", got.Reasons)
	}
}

func TestClassifyEditSet_DeletionPathIPasses(t *testing.T) {
	got := ClassifyEditSet(nEdits(5), NGTContext{
		TargetTraitClass: "productivity",
		DonorSource:      "none",
		VariantType:      "deletion",
	})
	if got.Category != "NGT-1" {
		t.Fatalf("deletion should be NGT-1; got %q (disq=%v)", got.Category, got.Disqualifiers)
	}
}

func TestClassifyEditSet_GenePoolUnconfirmedIsUnclassifiable(t *testing.T) {
	// Path (ii): EndogenousGeneInterrupted = nil ⇒ unclassifiable.
	got := ClassifyEditSet(nEdits(5), NGTContext{
		TargetTraitClass:          "productivity",
		DonorSource:               "same_species",
		VariantType:               "gene_pool_insertion",
		EndogenousGeneInterrupted: nil,
	})
	if got.Category != "unclassifiable" {
		t.Fatalf("Path (ii) without endogenous_gene_interrupted should be unclassifiable; got %q", got.Category)
	}
	found := false
	for _, d := range got.Disqualifiers {
		if strings.Contains(d, "endogenous_gene_interrupted") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("disqualifier should mention endogenous_gene_interrupted; got %v", got.Disqualifiers)
	}
}

func TestClassifyEditSet_GenePoolInterruptedIsNGT2(t *testing.T) {
	// Path (ii): EndogenousGeneInterrupted = true ⇒ disqualified, NGT-2.
	// This is the MAJOR v0.7.18 silent-false-positive case that this issue fixes.
	got := ClassifyEditSet(nEdits(5), NGTContext{
		TargetTraitClass:          "productivity",
		DonorSource:               "same_species",
		VariantType:               "gene_pool_insertion",
		EndogenousGeneInterrupted: boolPtr(true),
	})
	if got.Category != "NGT-2" {
		t.Fatalf("interrupted endogenous gene must give NGT-2; got %q (disq=%v)", got.Category, got.Disqualifiers)
	}
	found := false
	for _, d := range got.Disqualifiers {
		if strings.Contains(d, "endogenous gene") || strings.Contains(d, "endogenous genes") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("disqualifier should mention endogenous gene disruption; got %v", got.Disqualifiers)
	}
}

func TestClassifyEditSet_GenePoolClearIsNGT1(t *testing.T) {
	// Path (ii): EndogenousGeneInterrupted = false ⇒ NGT-1 eligible.
	got := ClassifyEditSet(nEdits(5), NGTContext{
		TargetTraitClass:          "productivity",
		DonorSource:               "same_gene_pool",
		VariantType:               "gene_pool_insertion",
		EndogenousGeneInterrupted: boolPtr(false),
	})
	if got.Category != "NGT-1" {
		t.Fatalf("gene_pool_insertion with no endogenous-gene interruption should be NGT-1; got %q (disq=%v)", got.Category, got.Disqualifiers)
	}
}

func TestClassifyEditSet_GenePoolRequiresGenePoolDonor(t *testing.T) {
	// Path (ii) with donor_source=none is contradictory — gene_pool_insertion
	// implies gene-pool provenance. Should be NGT-2 (disqualified) not silently
	// accepted.
	got := ClassifyEditSet(nEdits(5), NGTContext{
		TargetTraitClass:          "productivity",
		DonorSource:               "none",
		VariantType:               "gene_pool_insertion",
		EndogenousGeneInterrupted: boolPtr(false),
	})
	if got.Category != "NGT-2" {
		t.Fatalf("gene_pool_insertion with donor_source=none should be NGT-2; got %q", got.Category)
	}
	found := false
	for _, d := range got.Disqualifiers {
		if strings.Contains(d, "Path (ii) gene-pool insertion requires") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("disqualifier should explain Path (ii) donor requirement; got %v", got.Disqualifiers)
	}
}

func TestClassifyEditSet_GenePoolWithCrossSpeciesDonorFails(t *testing.T) {
	// gene_pool_insertion + cross_species donor: TWO disqualifiers — cross_species
	// is foreign DNA AND donor is not gene-pool. Both should fire.
	got := ClassifyEditSet(nEdits(5), NGTContext{
		TargetTraitClass:          "productivity",
		DonorSource:               "cross_species",
		VariantType:               "gene_pool_insertion",
		EndogenousGeneInterrupted: boolPtr(false),
	})
	if got.Category != "NGT-2" {
		t.Fatalf("gene_pool_insertion + cross_species should be NGT-2; got %q", got.Category)
	}
	if len(got.Disqualifiers) < 2 {
		t.Errorf("expected ≥2 disqualifiers (cross_species AND Path (ii) donor requirement), got %d: %v", len(got.Disqualifiers), got.Disqualifiers)
	}
}
