package main

// v0.7.28 — Issue 31. Climate-robustness section tests.
//
// Acceptance matrix from the issue:
//   1. Empty sweep → no section.
//   2. Single-mode sweep (1 scenario) → no comparative text (no section).
//   3. Multi-mode stable → "stays best" wording.
//   4. Multi-mode fragile → alternative-strategy recommendation.

import (
	"strings"
	"testing"
)

func TestClimateRobustness_NonClimateAxisReturnsNil(t *testing.T) {
	req := SensitivityRequest{Axis: "heritability"}
	got := buildClimateRobustnessSection(req, []SensitivityScenario{}, SensitivitySummary{})
	if got != nil {
		t.Errorf("non-climate axis must produce no section, got %+v", got)
	}
}

func TestClimateRobustness_SingleScenarioReturnsNil(t *testing.T) {
	req := SensitivityRequest{
		Axis: axisClimateScenario,
		ClimateValues: []ClimateScenario{{Mode: "normal", Severity: 0}},
	}
	got := buildClimateRobustnessSection(req, []SensitivityScenario{
		{AxisLabel: "normal (sev 0.00)", BestFeasibleCode: "balanced"},
	}, SensitivitySummary{BaselineBestCode: "balanced"})
	if got != nil {
		t.Errorf("single-scenario sweep must produce no section, got %+v", got)
	}
}

func TestClimateRobustness_StableHeadlineUsesStaysBest(t *testing.T) {
	req := SensitivityRequest{
		Axis: axisClimateScenario,
		ClimateValues: []ClimateScenario{
			{Mode: "normal", Severity: 0},
			{Mode: "heat_burst_anthesis", Severity: 1.0},
			{Mode: "drought_terminal", Severity: 0.7},
		},
	}
	scenarios := []SensitivityScenario{
		{AxisLabel: "normal (sev 0.00)", BestFeasibleCode: "balanced", BaselineMatch: true},
		{AxisLabel: "heat_burst_anthesis (sev 1.00)", BestFeasibleCode: "balanced", BaselineMatch: true},
		{AxisLabel: "drought_terminal (sev 0.70)", BestFeasibleCode: "balanced", BaselineMatch: true},
	}
	summary := SensitivitySummary{BaselineBestCode: "balanced"}
	got := buildClimateRobustnessSection(req, scenarios, summary)
	if got == nil {
		t.Fatal("expected section, got nil")
	}
	if !strings.Contains(got.Headline, "stays best in all 3") {
		t.Errorf("expected 'stays best in all 3' wording, got %q", got.Headline)
	}
	if got.FailureModes != "" {
		t.Errorf("no failure modes expected in stable case, got %q", got.FailureModes)
	}
	if got.AlternativeAdvice != "" {
		t.Errorf("no alternative advice in stable case, got %q", got.AlternativeAdvice)
	}
	if got.HonestyCaveat == "" {
		t.Error("honesty caveat must always be present")
	}
}

func TestClimateRobustness_FragileSurfacesAlternative(t *testing.T) {
	req := SensitivityRequest{
		Axis: axisClimateScenario,
		ClimateValues: []ClimateScenario{
			{Mode: "normal", Severity: 0},
			{Mode: "heat_burst_anthesis", Severity: 1.0},
			{Mode: "drought_terminal", Severity: 0.7},
		},
	}
	scenarios := []SensitivityScenario{
		{AxisLabel: "normal (sev 0.00)", BestFeasibleCode: "balanced", BaselineMatch: true},
		{AxisLabel: "heat_burst_anthesis (sev 1.00)", BestFeasibleCode: "diversity"},
		{AxisLabel: "drought_terminal (sev 0.70)", BestFeasibleCode: "diversity"},
	}
	summary := SensitivitySummary{BaselineBestCode: "balanced"}
	got := buildClimateRobustnessSection(req, scenarios, summary)
	if got == nil {
		t.Fatal("expected section, got nil")
	}
	if !strings.Contains(got.Headline, "1 of 3") {
		t.Errorf("expected '1 of 3' wording in fragile headline, got %q", got.Headline)
	}
	if !strings.Contains(got.FailureModes, "heat_burst_anthesis") || !strings.Contains(got.FailureModes, "drought_terminal") {
		t.Errorf("failure modes should name both losing modes, got %q", got.FailureModes)
	}
	if !strings.Contains(got.AlternativeAdvice, "diversity") {
		t.Errorf("alternative advice should name the winning strategy, got %q", got.AlternativeAdvice)
	}
	if got.AncestralAdvice != "" {
		t.Errorf("ancestral paragraph should be absent when ancestral_intro_percent = 0, got %q", got.AncestralAdvice)
	}
	if got.HonestyCaveat == "" {
		t.Error("honesty caveat must always be present")
	}
}

func TestClimateRobustness_AncestralParagraphConditional(t *testing.T) {
	// req with ancestral configured AND ancestral_introgression winning
	// a stressed scenario.
	base := SimRequest{AncestralIntroPercent: 15}
	req := SensitivityRequest{
		Base: base,
		Axis: axisClimateScenario,
		ClimateValues: []ClimateScenario{
			{Mode: "normal", Severity: 0},
			{Mode: "heat_burst_anthesis", Severity: 1.0},
		},
	}
	scenarios := []SensitivityScenario{
		{AxisLabel: "normal (sev 0.00)", BestFeasibleCode: "balanced", BaselineMatch: true},
		{AxisLabel: "heat_burst_anthesis (sev 1.00)", BestFeasibleCode: "ancestral_introgression"},
	}
	summary := SensitivitySummary{BaselineBestCode: "balanced"}
	got := buildClimateRobustnessSection(req, scenarios, summary)
	if got == nil {
		t.Fatal("expected section, got nil")
	}
	if !strings.Contains(got.AncestralAdvice, "15") {
		t.Errorf("ancestral paragraph should reference 15%% configuration, got %q", got.AncestralAdvice)
	}
	if !strings.Contains(got.AncestralAdvice, "heat_burst_anthesis") {
		t.Errorf("ancestral paragraph should name the mode it wins, got %q", got.AncestralAdvice)
	}
}

func TestClimateRobustness_NoBaselineBestCodeRendersWarning(t *testing.T) {
	req := SensitivityRequest{
		Axis: axisClimateScenario,
		ClimateValues: []ClimateScenario{
			{Mode: "normal", Severity: 0},
			{Mode: "heat_burst_anthesis", Severity: 1.0},
		},
	}
	scenarios := []SensitivityScenario{
		{AxisLabel: "normal (sev 0.00)", BestFeasibleCode: ""},
		{AxisLabel: "heat_burst_anthesis (sev 1.00)", BestFeasibleCode: "balanced"},
	}
	summary := SensitivitySummary{BaselineBestCode: ""}
	got := buildClimateRobustnessSection(req, scenarios, summary)
	if got == nil {
		t.Fatal("expected section, got nil")
	}
	if !strings.Contains(strings.ToLower(got.Headline), "no baseline") && !strings.Contains(strings.ToLower(got.Headline), "no feasible") {
		t.Errorf("expected empty-baseline headline to flag the constraint engine, got %q", got.Headline)
	}
}
