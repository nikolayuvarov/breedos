package main

import (
	"math"
	"testing"
)

// v0.7.20 — Issue 20 + 21. Tests for Ne trajectory formula and inbreeding-
// depression milk-yield cost.

func TestEffectiveNeFromDeltaF_NegativeOrZero(t *testing.T) {
	// ΔF ≤ 0 (no increase in inbreeding) → cap to maxEffectiveNe.
	if got := effectiveNeFromDeltaF(0); got != maxEffectiveNe {
		t.Errorf("ΔF=0 → Ne should cap at %v, got %v", maxEffectiveNe, got)
	}
	if got := effectiveNeFromDeltaF(-0.01); got != maxEffectiveNe {
		t.Errorf("ΔF=-0.01 → Ne should cap at %v, got %v", maxEffectiveNe, got)
	}
}

func TestEffectiveNeFromDeltaF_StandardValues(t *testing.T) {
	cases := []struct {
		dF   float64
		want float64
	}{
		{0.01, 50.0},     // ΔF=1% → Ne=50 (long-term-viability threshold).
		{0.005, 100.0},   // ΔF=0.5% → Ne=100 (FAO vulnerable threshold).
		{0.001, 500.0},   // ΔF=0.1% → Ne=500 (healthy).
		{0.0001, 5000.0}, // ΔF=0.01% → Ne=5000 (capped soon at 10000).
	}
	for _, c := range cases {
		got := effectiveNeFromDeltaF(c.dF)
		if math.Abs(got-c.want) > 1e-6 {
			t.Errorf("ΔF=%v: expected Ne=%v, got %v", c.dF, c.want, got)
		}
	}
}

func TestEffectiveNeFromDeltaF_CapsAtMax(t *testing.T) {
	// Numerically tiny ΔF (just above the 1e-9 threshold) still caps.
	if got := effectiveNeFromDeltaF(1e-10); got != maxEffectiveNe {
		t.Errorf("ΔF=1e-10 → Ne should cap, got %v", got)
	}
	// Slightly larger ΔF that would yield Ne > 10000 should also cap.
	if got := effectiveNeFromDeltaF(1e-8); got != maxEffectiveNe {
		t.Errorf("ΔF=1e-8 → Ne should cap, got %v", got)
	}
}

func TestPopulateNeTrajectory_FirstGenIsCap(t *testing.T) {
	// Generation 0 has no preceding generation; Ne should be the cap.
	metrics := []MetricPoint{
		{Generation: 0, Inbreeding: 0.00},
		{Generation: 1, Inbreeding: 0.01}, // ΔF=0.01 → Ne=50
	}
	populateNeTrajectory(metrics)
	if metrics[0].Ne != maxEffectiveNe {
		t.Errorf("generation 0 Ne should be cap, got %v", metrics[0].Ne)
	}
	if math.Abs(metrics[1].Ne-50.0) > 1e-4 {
		t.Errorf("generation 1: expected Ne ≈ 50, got %v", metrics[1].Ne)
	}
}

func TestPopulateNeTrajectory_MonotonicallyDecreasingInbreeding(t *testing.T) {
	// Typical run: monotonic F increase → Ne shrinks. Check direction.
	metrics := []MetricPoint{
		{Generation: 0, Inbreeding: 0.000},
		{Generation: 1, Inbreeding: 0.001}, // ΔF=0.001 → Ne=500
		{Generation: 2, Inbreeding: 0.005}, // ΔF=0.004 → Ne=125
		{Generation: 3, Inbreeding: 0.015}, // ΔF=0.010 → Ne=50
	}
	populateNeTrajectory(metrics)
	if !(metrics[1].Ne > metrics[2].Ne && metrics[2].Ne > metrics[3].Ne) {
		t.Errorf("Ne should be monotonically decreasing; got %v / %v / %v",
			metrics[1].Ne, metrics[2].Ne, metrics[3].Ne)
	}
}

func TestPopulateNeTrajectory_EmptyOrSingleton(t *testing.T) {
	// Should not panic on empty slice.
	populateNeTrajectory(nil)
	populateNeTrajectory([]MetricPoint{})
	// Single-element slice: only gen 0, no ΔF to compute, Ne == cap.
	metrics := []MetricPoint{{Generation: 0, Inbreeding: 0.0}}
	populateNeTrajectory(metrics)
	if metrics[0].Ne != maxEffectiveNe {
		t.Errorf("singleton: expected cap, got %v", metrics[0].Ne)
	}
}

func TestInbreedingDepressionCostMilkKg_ZeroOrNegative(t *testing.T) {
	if got := inbreedingDepressionCostMilkKg(0, 45); got != 0 {
		t.Errorf("F=0 → cost should be 0, got %v", got)
	}
	if got := inbreedingDepressionCostMilkKg(-0.05, 45); got != 0 {
		t.Errorf("F=-0.05 → cost should be 0 (guard), got %v", got)
	}
	if got := inbreedingDepressionCostMilkKg(0.05, -10); got != 0 {
		t.Errorf("coefficient<=0 → cost should be 0, got %v", got)
	}
}

func TestInbreedingDepressionCostMilkKg_AuditedRange(t *testing.T) {
	// F = 5% → at coefficient 45 kg/F → cost = 225 kg.
	F := 0.05
	got := inbreedingDepressionCostMilkKg(F, holsteinDepressionMilkDefaultKgPerF)
	want := 225.0
	if math.Abs(got-want) > 1e-6 {
		t.Errorf("F=%v at default coef: expected %v, got %v", F, want, got)
	}
	// Range bounds at F=5%: 100 kg (low) to 325 kg (high).
	low := inbreedingDepressionCostMilkKg(F, holsteinDepressionMilkLowKgPerF)
	high := inbreedingDepressionCostMilkKg(F, holsteinDepressionMilkHighKgPerF)
	if math.Abs(low-100.0) > 1e-6 {
		t.Errorf("F=5%% at low coef (20): expected 100 kg, got %v", low)
	}
	if math.Abs(high-325.0) > 1e-6 {
		t.Errorf("F=5%% at high coef (65): expected 325 kg, got %v", high)
	}
}

func TestInbreedingDepressionCostMilkKg_PerGenerationGrowth(t *testing.T) {
	// Cost is linear in F at fixed coefficient.
	coef := holsteinDepressionMilkDefaultKgPerF
	c1 := inbreedingDepressionCostMilkKg(0.01, coef)
	c2 := inbreedingDepressionCostMilkKg(0.02, coef)
	c4 := inbreedingDepressionCostMilkKg(0.04, coef)
	if math.Abs(c2-2*c1) > 1e-6 {
		t.Errorf("doubling F should double cost; c1=%v c2=%v", c1, c2)
	}
	if math.Abs(c4-4*c1) > 1e-6 {
		t.Errorf("quadrupling F should quadruple cost; c1=%v c4=%v", c1, c4)
	}
}
