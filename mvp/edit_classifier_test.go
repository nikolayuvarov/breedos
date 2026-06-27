package main

// v0.7.27 — Issue 07. Edit-vs-cross-vs-wait classifier tests.
//
// Coverage matrix:
//   - Low-frequency high-effect candidate → EDIT (the canonical "edit"
//     case the issue calls out).
//   - High-frequency low-marginal-value candidate → WAIT (NEAR_FIXATION
//     when p ≥ 0.92; MARGINAL_EFFECT when effect is tiny).
//   - High-risk candidate → WAIT (BOTTLENECK_RISK when diversity is low
//     and allele is rare).
//   - High-effect very-rare → EDIT with the cautious "≤2%" posture.
//   - Mid-band segregating → CROSS.
//   - Mid-band rare → EDIT with the standard ~5% posture.
//   - summarizeEditDecisions counts each class correctly and produces
//     a sensible headline for each special case (all-EDIT / all-CROSS /
//     all-WAIT / mixed / empty).

import (
	"strings"
	"testing"
)

func TestClassify_LargeEffectRareAllele_Edit(t *testing.T) {
	d := classifyEditCandidate(0.50, 0.05, 0.30)
	if d.Class != "edit" {
		t.Fatalf("Class = %q, want edit", d.Class)
	}
	if d.ReasonCode != "LARGE_EFFECT_RARE_ALLELE" {
		t.Errorf("ReasonCode = %q, want LARGE_EFFECT_RARE_ALLELE", d.ReasonCode)
	}
	if !strings.Contains(strings.ToLower(d.IntrogressionPosture), "5") {
		t.Errorf("standard posture missing 5%% mention: %q", d.IntrogressionPosture)
	}
}

func TestClassify_VeryHighEffectVeryRare_EditCautious(t *testing.T) {
	d := classifyEditCandidate(1.80, 0.03, 0.30)
	if d.Class != "edit" {
		t.Fatalf("Class = %q, want edit", d.Class)
	}
	if d.ReasonCode != "LARGE_EFFECT_RARE_ALLELE" {
		t.Errorf("ReasonCode = %q, want LARGE_EFFECT_RARE_ALLELE", d.ReasonCode)
	}
	if !strings.Contains(d.IntrogressionPosture, "2%") {
		t.Errorf("expected cautious ≤2%% posture for very-high-effect very-rare, got %q", d.IntrogressionPosture)
	}
	if !strings.Contains(d.RiskWarning, "pleiotropy") {
		t.Errorf("expected pleiotropy warning for high-effect introgression, got %q", d.RiskWarning)
	}
}

func TestClassify_HighFrequencyMarginal_Wait_NearFixation(t *testing.T) {
	d := classifyEditCandidate(0.40, 0.95, 0.30)
	if d.Class != "wait" {
		t.Fatalf("Class = %q, want wait", d.Class)
	}
	if d.ReasonCode != "NEAR_FIXATION" {
		t.Errorf("ReasonCode = %q, want NEAR_FIXATION", d.ReasonCode)
	}
}

func TestClassify_LowEffect_Wait_MarginalEffect(t *testing.T) {
	d := classifyEditCandidate(0.05, 0.10, 0.30)
	if d.Class != "wait" {
		t.Fatalf("Class = %q, want wait", d.Class)
	}
	if d.ReasonCode != "MARGINAL_EFFECT" {
		t.Errorf("ReasonCode = %q, want MARGINAL_EFFECT", d.ReasonCode)
	}
}

func TestClassify_HighRisk_Wait_BottleneckRisk(t *testing.T) {
	d := classifyEditCandidate(0.60, 0.05, 0.10) // diversity below 0.15 floor, p < 0.10.
	if d.Class != "wait" {
		t.Fatalf("Class = %q, want wait", d.Class)
	}
	if d.ReasonCode != "BOTTLENECK_RISK" {
		t.Errorf("ReasonCode = %q, want BOTTLENECK_RISK", d.ReasonCode)
	}
	if !strings.Contains(d.RiskWarning, "narrow") {
		t.Errorf("expected 'narrow' in risk warning, got %q", d.RiskWarning)
	}
}

func TestClassify_Segregating_Cross(t *testing.T) {
	d := classifyEditCandidate(0.40, 0.45, 0.30)
	if d.Class != "cross" {
		t.Fatalf("Class = %q, want cross", d.Class)
	}
	if d.ReasonCode != "ALREADY_SEGREGATING" {
		t.Errorf("ReasonCode = %q, want ALREADY_SEGREGATING", d.ReasonCode)
	}
}

func TestClassify_MidBandRareModerate_Edit(t *testing.T) {
	d := classifyEditCandidate(0.20, 0.15, 0.30) // effect ≥ marginal but < large; p < edit ceiling.
	if d.Class != "edit" {
		t.Fatalf("Class = %q, want edit", d.Class)
	}
	if d.ReasonCode != "MID_BAND_RARE_FAVOUR_EDIT" {
		t.Errorf("ReasonCode = %q, want MID_BAND_RARE_FAVOUR_EDIT", d.ReasonCode)
	}
}

func TestSummarize_AllEdit(t *testing.T) {
	cs := []EditCandidate{
		{Classification: &EditDecision{Class: "edit"}},
		{Classification: &EditDecision{Class: "edit"}},
		{Classification: &EditDecision{Class: "edit"}},
	}
	s := summarizeEditDecisions(cs)
	if s.EditCount != 3 || s.CrossCount != 0 || s.WaitCount != 0 {
		t.Errorf("counts wrong: %+v", s)
	}
	if !strings.Contains(s.Headline, "All 3") || !strings.Contains(s.Headline, "EDIT") {
		t.Errorf("headline wrong: %q", s.Headline)
	}
}

func TestSummarize_MixedHeadline(t *testing.T) {
	cs := []EditCandidate{
		{Classification: &EditDecision{Class: "edit"}},
		{Classification: &EditDecision{Class: "cross"}},
		{Classification: &EditDecision{Class: "cross"}},
		{Classification: &EditDecision{Class: "wait"}},
	}
	s := summarizeEditDecisions(cs)
	if s.EditCount != 1 || s.CrossCount != 2 || s.WaitCount != 1 || s.TotalCandidates != 4 {
		t.Errorf("counts wrong: %+v", s)
	}
	if !strings.Contains(s.Headline, "1 EDIT") || !strings.Contains(s.Headline, "2 CROSS") || !strings.Contains(s.Headline, "1 WAIT") {
		t.Errorf("mixed headline wrong: %q", s.Headline)
	}
}

func TestSummarize_Empty(t *testing.T) {
	s := summarizeEditDecisions(nil)
	if s.TotalCandidates != 0 {
		t.Errorf("expected 0 candidates, got %d", s.TotalCandidates)
	}
	if !strings.Contains(s.Headline, "No candidate edits") {
		t.Errorf("empty headline wrong: %q", s.Headline)
	}
}

func TestRankEditCandidates_AttachesClassification(t *testing.T) {
	freq := []float64{0.05, 0.50, 0.95, 0.10}
	effects := []float64{0.6, 0.4, 0.5, 0.05}
	got := rankEditCandidates(freq, effects, 4, 0.30)
	if len(got) == 0 {
		t.Fatalf("expected ranked candidates")
	}
	for _, c := range got {
		if c.Classification == nil {
			t.Fatalf("candidate at locus %d has no Classification", c.Locus)
		}
		if c.Classification.Class == "" || c.Classification.ReasonCode == "" {
			t.Errorf("candidate at locus %d has empty class/reason: %+v", c.Locus, c.Classification)
		}
		// Decision (legacy) must align with new class.
		switch c.Classification.Class {
		case "edit":
			if !strings.Contains(strings.ToLower(c.Decision), "seed edit") {
				t.Errorf("legacy decision for EDIT not aligned: %q", c.Decision)
			}
		case "cross":
			if !strings.Contains(strings.ToLower(c.Decision), "selection") {
				t.Errorf("legacy decision for CROSS not aligned: %q", c.Decision)
			}
		case "wait":
			if !strings.Contains(strings.ToLower(c.Decision), "defer") {
				t.Errorf("legacy decision for WAIT not aligned: %q", c.Decision)
			}
		}
	}
}
