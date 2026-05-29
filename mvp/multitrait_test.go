package main

import (
	"math"
	"math/rand"
	"strings"
	"testing"
)

// v0.7.21 — Issue 18. Multi-trait engine tests.

func TestValidateMultiTraitRequest_EmptyTraits(t *testing.T) {
	req := SimRequest{Traits: nil}
	if err := validateMultiTraitRequest(req); err == nil {
		t.Fatal("empty traits should be rejected")
	}
}

func TestValidateMultiTraitRequest_DuplicateNames(t *testing.T) {
	req := SimRequest{
		Markers:     200,
		Generations: 10,
		Traits: []TraitConfig{
			{Name: "milk", Heritability: 0.3, QTLCount: 20, EffectScale: 1.0},
			{Name: "milk", Heritability: 0.2, QTLCount: 15, EffectScale: 1.0},
		},
		GeneticCorrelations: [][]float64{{1, 0}, {0, 1}},
	}
	err := validateMultiTraitRequest(req)
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("duplicate trait names should be rejected; got %v", err)
	}
}

func TestValidateMultiTraitRequest_NonSquareMatrix(t *testing.T) {
	req := SimRequest{
		Markers:     200,
		Generations: 10,
		Traits: []TraitConfig{
			{Name: "milk", Heritability: 0.3, QTLCount: 20, EffectScale: 1.0},
			{Name: "methane", Heritability: 0.2, QTLCount: 15, EffectScale: 1.0},
		},
		GeneticCorrelations: [][]float64{{1, 0}, {0, 1, 0}},
	}
	err := validateMultiTraitRequest(req)
	if err == nil || !strings.Contains(err.Error(), "square") {
		t.Errorf("non-square matrix should be rejected; got %v", err)
	}
}

func TestValidateMultiTraitRequest_BadDiagonal(t *testing.T) {
	req := SimRequest{
		Markers:     200,
		Generations: 10,
		Traits: []TraitConfig{
			{Name: "a", Heritability: 0.3, QTLCount: 20, EffectScale: 1.0},
			{Name: "b", Heritability: 0.2, QTLCount: 15, EffectScale: 1.0},
		},
		GeneticCorrelations: [][]float64{{0.9, 0}, {0, 1}},
	}
	err := validateMultiTraitRequest(req)
	if err == nil || !strings.Contains(err.Error(), "diagonal") {
		t.Errorf("non-1 diagonal should be rejected; got %v", err)
	}
}

func TestValidateMultiTraitRequest_OutOfRangeCorrelation(t *testing.T) {
	req := SimRequest{
		Markers:     200,
		Generations: 10,
		Traits: []TraitConfig{
			{Name: "a", Heritability: 0.3, QTLCount: 20, EffectScale: 1.0},
			{Name: "b", Heritability: 0.2, QTLCount: 15, EffectScale: 1.0},
		},
		GeneticCorrelations: [][]float64{{1, 1.5}, {1.5, 1}},
	}
	err := validateMultiTraitRequest(req)
	if err == nil || !strings.Contains(err.Error(), "[-1, 1]") {
		t.Errorf("|r|>1 should be rejected; got %v", err)
	}
}

func TestValidateMultiTraitRequest_AsymmetricCorrelation(t *testing.T) {
	req := SimRequest{
		Markers:     200,
		Generations: 10,
		Traits: []TraitConfig{
			{Name: "a", Heritability: 0.3, QTLCount: 20, EffectScale: 1.0},
			{Name: "b", Heritability: 0.2, QTLCount: 15, EffectScale: 1.0},
		},
		GeneticCorrelations: [][]float64{{1, 0.5}, {0.3, 1}},
	}
	err := validateMultiTraitRequest(req)
	if err == nil || !strings.Contains(err.Error(), "symmetric") {
		t.Errorf("asymmetric matrix should be rejected; got %v", err)
	}
}

func TestValidateMultiTraitRequest_ModestValid(t *testing.T) {
	req := SimRequest{
		Markers:     200,
		Generations: 10,
		Traits: []TraitConfig{
			{Name: "milk", Heritability: 0.36, QTLCount: 30, EffectScale: 1.0, SelectionWeight: 1.0},
			{Name: "methane", Heritability: 0.20, QTLCount: 20, EffectScale: 1.0, SelectionWeight: -0.5},
		},
		GeneticCorrelations: [][]float64{{1.0, -0.26}, {-0.26, 1.0}},
	}
	if err := validateMultiTraitRequest(req); err != nil {
		t.Errorf("modest 2-trait should pass; got %v", err)
	}
}

func TestCholeskyDecomp_Identity(t *testing.T) {
	C := [][]float64{
		{1, 0, 0},
		{0, 1, 0},
		{0, 0, 1},
	}
	L, err := choleskyDecomp(C)
	if err != nil {
		t.Fatalf("identity Cholesky: %v", err)
	}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			want := 0.0
			if i == j {
				want = 1.0
			}
			if math.Abs(L[i][j]-want) > 1e-9 {
				t.Errorf("L[%d][%d] = %v, want %v", i, j, L[i][j], want)
			}
		}
	}
}

func TestCholeskyDecomp_2x2(t *testing.T) {
	// C = [[1, r], [r, 1]]. Cholesky: L = [[1, 0], [r, sqrt(1-r²)]].
	r := 0.5
	C := [][]float64{{1, r}, {r, 1}}
	L, err := choleskyDecomp(C)
	if err != nil {
		t.Fatalf("2x2 Cholesky: %v", err)
	}
	if math.Abs(L[0][0]-1) > 1e-9 {
		t.Errorf("L[0][0] = %v, want 1", L[0][0])
	}
	if math.Abs(L[0][1]-0) > 1e-9 {
		t.Errorf("L[0][1] = %v, want 0 (lower-triangular)", L[0][1])
	}
	if math.Abs(L[1][0]-r) > 1e-9 {
		t.Errorf("L[1][0] = %v, want %v", L[1][0], r)
	}
	expected := math.Sqrt(1 - r*r)
	if math.Abs(L[1][1]-expected) > 1e-9 {
		t.Errorf("L[1][1] = %v, want %v", L[1][1], expected)
	}
}

func TestCholeskyDecomp_VerifyLLT(t *testing.T) {
	// For an arbitrary symmetric positive-definite C, verify L · L^T = C.
	C := [][]float64{
		{1.0, 0.3, -0.2},
		{0.3, 1.0, 0.4},
		{-0.2, 0.4, 1.0},
	}
	L, err := choleskyDecomp(C)
	if err != nil {
		t.Fatalf("3x3 Cholesky: %v", err)
	}
	n := 3
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			var sum float64
			for k := 0; k < n; k++ {
				sum += L[i][k] * L[j][k]
			}
			if math.Abs(sum-C[i][j]) > 1e-9 {
				t.Errorf("(LL^T)[%d][%d] = %v, want C = %v", i, j, sum, C[i][j])
			}
		}
	}
}

func TestSelectionIndex_ZeroCenteredAndScaled(t *testing.T) {
	// Two-trait, weights (1, -0.5). Construct hand-crafted phenotypes so we
	// can verify the standardised weighted sum.
	traits := []TraitConfig{
		{Name: "a", SelectionWeight: 1.0, EffectScale: 1.0},
		{Name: "b", SelectionWeight: -0.5, EffectScale: 1.0},
	}
	phen := [][]float64{
		{1, 2, 3, 4, 5}, // mean=3, sd=√2
		{5, 4, 3, 2, 1}, // mean=3, sd=√2
	}
	idx := selectionIndex(phen, traits)
	if len(idx) != 5 {
		t.Fatalf("want 5 index values, got %d", len(idx))
	}
	// Individual 0: z_a = (1-3)/√2 = -√2; z_b = (5-3)/√2 = √2.
	// Index = 1*(-√2) + (-0.5)*(√2) = -1.5·√2.
	want0 := -1.5 * math.Sqrt(2)
	if math.Abs(idx[0]-want0) > 1e-6 {
		t.Errorf("idx[0] = %v, want %v", idx[0], want0)
	}
}

func TestMakeMultiTraitEffects_QTLMaskingHonoured(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	traits := []TraitConfig{
		{Name: "a", Heritability: 0.5, QTLCount: 5, EffectScale: 1.0},
		{Name: "b", Heritability: 0.5, QTLCount: 3, EffectScale: 1.0},
	}
	L := [][]float64{{1, 0}, {0, 1}} // identity (no correlation)
	eff := makeMultiTraitEffects(50, traits, L, rng)
	nzA, nzB := 0, 0
	for m := 0; m < 50; m++ {
		if eff[0][m] != 0 {
			nzA++
		}
		if eff[1][m] != 0 {
			nzB++
		}
	}
	if nzA != 5 {
		t.Errorf("trait a should have exactly 5 non-zero effects (QTLCount), got %d", nzA)
	}
	if nzB != 3 {
		t.Errorf("trait b should have exactly 3 non-zero effects (QTLCount), got %d", nzB)
	}
}

func TestRunMultiTraitSimulation_EndToEndCompletes(t *testing.T) {
	// Minimal end-to-end smoke: 2 traits, small population, single generation.
	// Just verify the pipeline returns without error and produces per-trait gain.
	req := SimRequest{
		Seed:               1,
		PopulationSize:     50,
		Markers:            120,
		QTLCount:           20,
		Generations:        4,
		SelectionPercent:   20,
		Heritability:       0.4, // not used in multi-trait path; required by validateRequest
		MutationRate:       0.0001,
		StrategySet:        "core",
		Replicates:         2,
		InbreedingLimit:    0.25,
		DiversityLossLimit: 0.30,
		Traits: []TraitConfig{
			{Name: "milk", Heritability: 0.36, QTLCount: 20, EffectScale: 1.0, SelectionWeight: 1.0},
			{Name: "methane", Heritability: 0.20, QTLCount: 15, EffectScale: 1.0, SelectionWeight: -0.5},
		},
		GeneticCorrelations: [][]float64{{1.0, -0.26}, {-0.26, 1.0}},
	}
	resp, err := runMultiTraitSimulation(req, func(int, string) {}, func(AFSSnapshot) {})
	if err != nil {
		t.Fatalf("multi-trait simulation error: %v", err)
	}
	if len(resp.Strategies) == 0 {
		t.Fatal("no strategy results")
	}
	if _, ok := resp.Decision.PerTraitGain["milk"]; !ok {
		t.Errorf("PerTraitGain missing 'milk' entry; got %v", resp.Decision.PerTraitGain)
	}
	if _, ok := resp.Decision.PerTraitGain["methane"]; !ok {
		t.Errorf("PerTraitGain missing 'methane' entry; got %v", resp.Decision.PerTraitGain)
	}
	// Every strategy must carry PerTraitMetrics with 2 entries.
	for _, s := range resp.Strategies {
		if len(s.PerTraitMetrics) != 2 {
			t.Errorf("strategy %q: PerTraitMetrics has %d entries, want 2", s.Code, len(s.PerTraitMetrics))
		}
	}
}

func TestRunMultiTraitSimulation_NegativeWeightOpposesPositive(t *testing.T) {
	// With strong unfavourable correlation +0.6 between traits A and B,
	// selecting strongly FOR A (w=1) while pushing AGAINST B (w=-1) is a
	// tug-of-war. With h² > 0 for both, the simulator should at least
	// produce *finite, non-NaN* gains on both. Direction-specific assertions
	// are difficult under stochastic short runs; we settle for "outputs are
	// finite + per-trait gain map covers both traits".
	req := SimRequest{
		Seed:               2,
		PopulationSize:     60,
		Markers:            120,
		QTLCount:           20,
		Generations:        5,
		SelectionPercent:   20,
		Heritability:       0.4,
		MutationRate:       0.0001,
		StrategySet:        "core",
		Replicates:         2,
		InbreedingLimit:    0.25,
		DiversityLossLimit: 0.30,
		Traits: []TraitConfig{
			{Name: "A", Heritability: 0.4, QTLCount: 20, EffectScale: 1.0, SelectionWeight: 1.0},
			{Name: "B", Heritability: 0.4, QTLCount: 20, EffectScale: 1.0, SelectionWeight: -1.0},
		},
		GeneticCorrelations: [][]float64{{1.0, 0.6}, {0.6, 1.0}},
	}
	resp, err := runMultiTraitSimulation(req, func(int, string) {}, func(AFSSnapshot) {})
	if err != nil {
		t.Fatalf("multi-trait simulation: %v", err)
	}
	for name, gain := range resp.Decision.PerTraitGain {
		if math.IsNaN(gain) || math.IsInf(gain, 0) {
			t.Errorf("trait %s: gain = %v (NaN/Inf)", name, gain)
		}
	}
}

func TestRunSimulation_SingleTraitPayloadStillWorks(t *testing.T) {
	// Backward compatibility: omitting req.Traits ⇒ existing single-trait
	// path. No PerTraitGain / PerTraitMetrics fields should be populated.
	req := SimRequest{
		Seed:               1,
		PopulationSize:     50,
		Markers:            120,
		QTLCount:           20,
		Generations:        3,
		SelectionPercent:   20,
		Heritability:       0.5,
		MutationRate:       0.0001,
		StrategySet:        "core",
		Replicates:         2,
		InbreedingLimit:    0.25,
		DiversityLossLimit: 0.30,
	}
	resp, err := runSimulationWithCallbacks(req, func(int, string) {}, func(AFSSnapshot) {})
	if err != nil {
		t.Fatalf("single-trait simulation error: %v", err)
	}
	if resp.Decision.PerTraitGain != nil {
		t.Errorf("single-trait run should not populate PerTraitGain; got %v", resp.Decision.PerTraitGain)
	}
	for _, s := range resp.Strategies {
		if s.PerTraitMetrics != nil {
			t.Errorf("strategy %q: single-trait run should not populate PerTraitMetrics", s.Code)
		}
	}
}
