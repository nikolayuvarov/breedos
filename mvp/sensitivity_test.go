package main

import (
	"testing"
)

func baseSimRequestForSens() SimRequest {
	return SimRequest{
		Seed: 1, PopulationSize: 80, Markers: 200, QTLCount: 15,
		Generations: 12, SelectionPercent: 15, Heritability: 0.5,
		MutationRate: 0.0001, CrisprEnabled: true, CrisprEdits: 2,
		CrisprIntroPercent: 10, StrategySet: "core", Replicates: 2,
		WorkerCount: 0, InbreedingLimit: 0.25, DiversityLossLimit: 0.30,
	}
}

func TestValidateSensitivityRejectsUnknownAxis(t *testing.T) {
	req := SensitivityRequest{Base: baseSimRequestForSens(), Axis: "nonsense", Values: []float64{0.5}}
	if err := validateSensitivityRequest(req); err == nil {
		t.Fatal("expected error for unknown axis, got nil")
	}
}

func TestValidateSensitivityRejectsEmptyValues(t *testing.T) {
	req := SensitivityRequest{Base: baseSimRequestForSens(), Axis: "heritability", Values: nil}
	if err := validateSensitivityRequest(req); err == nil {
		t.Fatal("expected error for empty values, got nil")
	}
}

func TestValidateSensitivityEnforcesMaxValues(t *testing.T) {
	req := SensitivityRequest{
		Base: baseSimRequestForSens(), Axis: "heritability",
		Values: []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6},
	}
	if err := validateSensitivityRequest(req); err == nil {
		t.Fatalf("expected error for >%d values, got nil", sensitivityMaxValues)
	}
}

func TestValidateSensitivityRejectsBadAxisValue(t *testing.T) {
	// h² must be in [0,1] per validateRequest; 1.5 should fail.
	req := SensitivityRequest{Base: baseSimRequestForSens(), Axis: "heritability", Values: []float64{0.5, 1.5}}
	if err := validateSensitivityRequest(req); err == nil {
		t.Fatal("expected error for out-of-range axis value, got nil")
	}
}

func TestValidateSensitivityBudgetCap(t *testing.T) {
	base := baseSimRequestForSens()
	// Push base to ~300M cells per run × 5 → 1.5B cap edge.
	base.PopulationSize = 1000
	base.Markers = 1000
	base.Generations = 60
	req := SensitivityRequest{Base: base, Axis: "heritability", Values: []float64{0.2, 0.35, 0.5, 0.65, 0.8}}
	if err := validateSensitivityRequest(req); err == nil {
		t.Fatal("expected budget cap violation, got nil")
	}
}

func TestValidateSensitivityAcceptsModestSweep(t *testing.T) {
	req := SensitivityRequest{
		Base: baseSimRequestForSens(), Axis: "heritability",
		Values: []float64{0.3, 0.5, 0.7},
	}
	if err := validateSensitivityRequest(req); err != nil {
		t.Fatalf("modest sweep should pass validation, got %v", err)
	}
}

func TestNearestIndex(t *testing.T) {
	cases := []struct {
		values []float64
		target float64
		want   int
	}{
		{[]float64{0.2, 0.35, 0.5, 0.65, 0.8}, 0.5, 2},
		{[]float64{0.2, 0.35, 0.5, 0.65, 0.8}, 0.6, 3}, // 0.65 closer than 0.5
		{[]float64{0.2, 0.35, 0.5, 0.65, 0.8}, 0.99, 4},
		{[]float64{0.2, 0.35, 0.5, 0.65, 0.8}, 0.0, 0},
		{nil, 0.5, -1},
	}
	for i, c := range cases {
		got := nearestIndex(c.values, c.target)
		if got != c.want {
			t.Errorf("case %d: nearestIndex(%v, %v) = %d, want %d", i, c.values, c.target, got, c.want)
		}
	}
}

func TestAssembleSensitivityResultStableVerdict(t *testing.T) {
	req := SensitivityRequest{
		Base: baseSimRequestForSens(), Axis: "heritability",
		Values: []float64{0.3, 0.5, 0.7},
	}
	scenarios := []SensitivityScenario{
		{AxisValue: 0.3, BestFeasibleCode: "balanced", BestFeasibleName: "Balanced"},
		{AxisValue: 0.5, BestFeasibleCode: "balanced", BestFeasibleName: "Balanced"},
		{AxisValue: 0.7, BestFeasibleCode: "balanced", BestFeasibleName: "Balanced"},
	}
	result := assembleSensitivityResult(req, scenarios)
	if result.Summary.Verdict != "stable" {
		t.Errorf("expected 'stable' verdict, got %q", result.Summary.Verdict)
	}
	if !result.Summary.Stable {
		t.Errorf("expected Stable=true")
	}
	if result.Summary.StrategySwitches != 0 {
		t.Errorf("expected 0 switches, got %d", result.Summary.StrategySwitches)
	}
	for i, s := range result.Scenarios {
		if !s.BaselineMatch {
			t.Errorf("scenario %d should match baseline", i)
		}
	}
}

func TestAssembleSensitivityResultFragileVerdict(t *testing.T) {
	req := SensitivityRequest{
		Base: baseSimRequestForSens(), Axis: "heritability",
		Values: []float64{0.3, 0.5, 0.7},
	}
	scenarios := []SensitivityScenario{
		{AxisValue: 0.3, BestFeasibleCode: "diversity", BestFeasibleName: "Diversity"},
		{AxisValue: 0.5, BestFeasibleCode: "balanced", BestFeasibleName: "Balanced"},
		{AxisValue: 0.7, BestFeasibleCode: "aggressive", BestFeasibleName: "Aggressive"},
	}
	result := assembleSensitivityResult(req, scenarios)
	if result.Summary.Verdict != "fragile" {
		t.Errorf("expected 'fragile' verdict, got %q", result.Summary.Verdict)
	}
	if result.Summary.Stable {
		t.Errorf("expected Stable=false")
	}
	if result.Summary.StrategySwitches != 2 {
		t.Errorf("expected 2 switches (h²=0.3 and 0.7 differ from baseline=balanced), got %d", result.Summary.StrategySwitches)
	}
	if result.Summary.BaselineBestCode != "balanced" {
		t.Errorf("baseline should be the scenario nearest h²=0.5 = balanced, got %q", result.Summary.BaselineBestCode)
	}
}

func TestAssembleSensitivityResultInconclusiveWhenAllInfeasible(t *testing.T) {
	req := SensitivityRequest{
		Base: baseSimRequestForSens(), Axis: "heritability",
		Values: []float64{0.3, 0.5, 0.7},
	}
	scenarios := []SensitivityScenario{
		{AxisValue: 0.3, BestFeasibleCode: ""},
		{AxisValue: 0.5, BestFeasibleCode: ""},
		{AxisValue: 0.7, BestFeasibleCode: ""},
	}
	result := assembleSensitivityResult(req, scenarios)
	if result.Summary.Verdict != "inconclusive" {
		t.Errorf("expected 'inconclusive' verdict, got %q", result.Summary.Verdict)
	}
}

func TestSummarizeScenarioFallsBackToBestRiskAdjusted(t *testing.T) {
	// When constraint engine is off, BestFeasibleCode is "" and the
	// summarizer should fall back to BestRiskAdjustedCode.
	resp := SimResponse{
		Decision: DecisionSummary{
			BestRiskAdjustedCode: "balanced",
			BestFeasibleCode:     "",
			FeasibilityNote:      "No hard constraints applied.",
		},
		Strategies: []StrategyResult{
			{Code: "balanced", Name: "Balanced",
				Final: FinalStats{GeneticGain: 12.5, Diversity: 0.45, Inbreeding: 0.18}},
		},
	}
	got := summarizeScenario(0.5, resp)
	if got.BestFeasibleCode != "balanced" {
		t.Errorf("fallback should populate BestFeasibleCode='balanced', got %q", got.BestFeasibleCode)
	}
	if got.BestFeasibleName != "Balanced" {
		t.Errorf("fallback should populate name, got %q", got.BestFeasibleName)
	}
	if got.GeneticGain != 12.5 {
		t.Errorf("fallback should still copy metrics, got gain=%v", got.GeneticGain)
	}
}
