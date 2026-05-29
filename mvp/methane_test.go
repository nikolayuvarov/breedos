package main

import (
	"math"
	"math/rand"
	"strings"
	"testing"
)

// v0.7.22 — Issues 19, 22, 23, 26 — tests for the methane defaults,
// N-D Pareto, multi-trait report section, and Holstein synthetic dataset.

func TestMethaneIntensityDefaults_ShapeAndSigns(t *testing.T) {
	traits, corr := methaneIntensityDefaults()
	if len(traits) != 2 {
		t.Fatalf("expected 2 traits, got %d", len(traits))
	}
	if traits[0].Name != "milk_yield" || traits[1].Name != "methane_intensity" {
		t.Errorf("trait order wrong: %s, %s", traits[0].Name, traits[1].Name)
	}
	if traits[0].SelectionWeight <= 0 {
		t.Errorf("milk weight should be positive, got %v", traits[0].SelectionWeight)
	}
	if traits[1].SelectionWeight >= 0 {
		t.Errorf("methane weight should be negative, got %v", traits[1].SelectionWeight)
	}
	if corr[0][1] >= 0 {
		t.Errorf("MeI × milk yield correlation should be negative (favourable), got %v", corr[0][1])
	}
	if math.Abs(corr[0][1]-corr[1][0]) > 1e-9 {
		t.Errorf("correlation matrix not symmetric: %v vs %v", corr[0][1], corr[1][0])
	}
}

func TestMethaneProductionDefaults_UnfavourableSign(t *testing.T) {
	_, corr := methaneProductionDefaults()
	if corr[0][1] <= 0 {
		t.Errorf("MeP × milk yield correlation should be positive (unfavourable), got %v", corr[0][1])
	}
}

func TestMethaneDefaults_ValidPositiveDefinite(t *testing.T) {
	// Both presets must pass Cholesky decomposition (positive-(semi-)definite).
	for _, getter := range []func() ([]TraitConfig, [][]float64){methaneIntensityDefaults, methaneProductionDefaults} {
		_, corr := getter()
		if _, err := choleskyDecomp(corr); err != nil {
			t.Errorf("preset correlation matrix not PD: %v", err)
		}
	}
}

func TestSynthHolsteinFounders_ShapeAndDosage(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	ds := synthHolsteinFounders(20, 100, rng)
	if len(ds.individuals) != 20 {
		t.Errorf("want 20 organisms, got %d", len(ds.individuals))
	}
	if ds.markerCount != 100 {
		t.Errorf("want 100 markers, got %d", ds.markerCount)
	}
	if !ds.isPlaceholder {
		t.Errorf("synthetic dataset must be flagged isPlaceholder=true")
	}
	if !strings.Contains(strings.Join(ds.sourceNotes, " "), "Synthetic Holstein") {
		t.Errorf("sourceNotes should disclose synthetic status")
	}
	// Every dosage must be 0, 1, or 2.
	for i, org := range ds.individuals {
		for m, d := range org.geno {
			if d > 2 {
				t.Fatalf("organism %d marker %d: dosage %d > 2", i, m, d)
			}
		}
	}
}

func TestSynthHolsteinFounders_AFSIsUShaped(t *testing.T) {
	// With Beta(0.5, 0.5) per-marker MAF, the distribution piles up near
	// 0 and 1. Average MAF across many markers should sit close to 0.5
	// (symmetric); but the COUNT of "near-fixed" (p < 0.05 or p > 0.95)
	// markers should be high compared to a uniform distribution.
	rng := rand.New(rand.NewSource(123))
	ds := synthHolsteinFounders(200, 500, rng)
	freq := alleleFreq(ds.individuals, ds.markerCount)
	nearFixed := 0
	for _, p := range freq {
		if p < 0.05 || p > 0.95 {
			nearFixed++
		}
	}
	// Beta(0.5, 0.5) expected mass below 0.05 + above 0.95:
	// 2 * integral(0 to 0.05) Beta(0.5, 0.5) pdf ≈ 2 * (2/π) * arcsin(sqrt(0.05))
	// = (4/π) * arcsin(sqrt(0.05)) ≈ (4/π) * 0.2257 ≈ 0.287.
	// So roughly 29% of markers should be near-fixed.
	frac := float64(nearFixed) / float64(len(freq))
	if frac < 0.15 || frac > 0.45 {
		t.Errorf("near-fixed fraction = %.3f, expected ≈ 0.29 for Beta(0.5, 0.5)", frac)
	}
}

func TestMultiTraitDominates_StrictNoStrict(t *testing.T) {
	// a strictly better on trait 0; equal elsewhere → dominates.
	a := StrategyResult{
		Final: FinalStats{Diversity: 0.5, Inbreeding: 0.1},
		PerTraitMetrics: [][]MetricPoint{
			{{GeneticGain: 10}},
			{{GeneticGain: 5}},
		},
	}
	b := StrategyResult{
		Final: FinalStats{Diversity: 0.5, Inbreeding: 0.1},
		PerTraitMetrics: [][]MetricPoint{
			{{GeneticGain: 8}},
			{{GeneticGain: 5}},
		},
	}
	if !multiTraitDominates(a, b, 2) {
		t.Errorf("a should dominate b (a strictly better on trait 0)")
	}
	if multiTraitDominates(b, a, 2) {
		t.Errorf("b should not dominate a")
	}
}

func TestMultiTraitDominates_TraitTradeoff(t *testing.T) {
	// a > b on trait 0, b > a on trait 1 → neither dominates.
	a := StrategyResult{
		Final: FinalStats{Diversity: 0.5, Inbreeding: 0.1},
		PerTraitMetrics: [][]MetricPoint{
			{{GeneticGain: 10}},
			{{GeneticGain: 3}},
		},
	}
	b := StrategyResult{
		Final: FinalStats{Diversity: 0.5, Inbreeding: 0.1},
		PerTraitMetrics: [][]MetricPoint{
			{{GeneticGain: 8}},
			{{GeneticGain: 6}},
		},
	}
	if multiTraitDominates(a, b, 2) {
		t.Errorf("tradeoff: a should NOT dominate b")
	}
	if multiTraitDominates(b, a, 2) {
		t.Errorf("tradeoff: b should NOT dominate a")
	}
}

func TestMultiTraitDominates_BetterDiversityWorseInbreeding(t *testing.T) {
	// a has higher diversity but also higher inbreeding → not dominate.
	a := StrategyResult{
		Final: FinalStats{Diversity: 0.6, Inbreeding: 0.2},
		PerTraitMetrics: [][]MetricPoint{
			{{GeneticGain: 10}},
		},
	}
	b := StrategyResult{
		Final: FinalStats{Diversity: 0.5, Inbreeding: 0.1},
		PerTraitMetrics: [][]MetricPoint{
			{{GeneticGain: 10}},
		},
	}
	if multiTraitDominates(a, b, 1) {
		t.Errorf("a should NOT dominate (higher inbreeding)")
	}
	if multiTraitDominates(b, a, 1) {
		t.Errorf("b should NOT dominate (lower diversity)")
	}
}

func TestRunMultiTraitSimulation_PerTraitReportSectionAppears(t *testing.T) {
	traits, corr := methaneIntensityDefaults()
	req := SimRequest{
		Seed:               42,
		PopulationSize:     50,
		Markers:            120,
		QTLCount:           20,
		Generations:        4,
		SelectionPercent:   15,
		Heritability:       0.5,
		MutationRate:       0.0001,
		StrategySet:        "core",
		Replicates:         2,
		InbreedingLimit:    0.25,
		DiversityLossLimit: 0.30,
		Traits:             traits,
		GeneticCorrelations: corr,
	}
	resp, err := runMultiTraitSimulation(req, func(int, string) {}, func(AFSSnapshot) {})
	if err != nil {
		t.Fatalf("multi-trait simulation: %v", err)
	}
	found := false
	for _, line := range resp.Decision.Interpretation {
		if strings.Contains(line, "Multi-trait trade-off") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Multi-trait trade-off section missing from interpretation")
	}
}

func TestRunMultiTraitSimulation_HolsteinSyntheticDataset(t *testing.T) {
	// Verify the multi-trait simulator runs on a synthetic Holstein founder.
	traits, corr := methaneIntensityDefaults()
	req := SimRequest{
		Seed:               7,
		PopulationSize:     30,
		Markers:            60,
		QTLCount:           15,
		Generations:        3,
		SelectionPercent:   15,
		Heritability:       0.5,
		MutationRate:       0.0001,
		StrategySet:        "core",
		Replicates:         1,
		InbreedingLimit:    0.30,
		DiversityLossLimit: 0.35,
		Dataset:            "holstein_synthetic",
		Traits:             traits,
		GeneticCorrelations: corr,
	}
	resp, err := runMultiTraitSimulation(req, func(int, string) {}, func(AFSSnapshot) {})
	if err != nil {
		t.Fatalf("Holstein-synthetic + multi-trait: %v", err)
	}
	if len(resp.Strategies) == 0 {
		t.Error("no strategies returned")
	}
	if resp.Decision.PerTraitGain == nil {
		t.Error("PerTraitGain should be populated")
	}
}
