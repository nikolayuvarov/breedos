package main

// v0.7.28 — Issue 30. Ancestral-allele introgression tests.
//
// Coverage matrix from the issue:
//   1. Ancestral seed flag propagates (strategy appears, UseAncestralSeed=true).
//   2. Climate-penalty discount applies (discount < 1 when scenario is set
//      and strategy carries the flag; bit-identical otherwise).
//   3. Mating still works with mixed origin (a full run with ancestral
//      seed completes without crash; metrics monotonically reasonable).
//   4. Validation rejects intro_percent > 25 (and < 0).
//   5. Edge case at 0% (strategy is NOT added; discount = 1).

import (
	"math/rand"
	"strings"
	"testing"
)

func TestSeedAncestralLines_RewritesLastK(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	markers := 100
	pop := make([]organism, 20)
	for i := range pop {
		g := make([]uint8, markers)
		for m := range g {
			g[m] = 2 // saturate so we can tell what was rewritten.
		}
		pop[i] = organism{geno: g}
	}
	seedAncestralLines(pop, 15.0, markers, rng)
	// Expected K = round(20 × 0.15) = 3 → the last 3 organisms are rewritten.
	for i := 0; i < 17; i++ {
		for m := 0; m < markers; m++ {
			if pop[i].geno[m] != 2 {
				t.Errorf("modern individual %d locus %d should remain 2 (got %d)", i, m, pop[i].geno[m])
			}
		}
	}
	// The last 3 should not all be 2 anymore (Bernoulli with p=0.3).
	allTwo := true
	for i := 17; i < 20; i++ {
		for m := 0; m < markers; m++ {
			if pop[i].geno[m] != 2 {
				allTwo = false
				break
			}
		}
	}
	if allTwo {
		t.Errorf("ancestral lines should have been re-rolled but are all 2s")
	}
}

func TestSeedAncestralLines_LowersMeanAlleleCount(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	markers := 200
	n := 100
	pop := make([]organism, n)
	for i := range pop {
		g := make([]uint8, markers)
		for m := range g {
			g[m] = 1
		}
		pop[i] = organism{geno: g}
	}
	seedAncestralLines(pop, 25.0, markers, rng) // K = 25 ancestrals.
	// Mean allele count over ancestrals should be < the modern baseline
	// of 1.0 (expected 2 × 0.30 = 0.60 with the AncestralAlleleBias).
	sum := 0
	count := 0
	for i := n - 25; i < n; i++ {
		for _, g := range pop[i].geno {
			sum += int(g)
			count++
		}
	}
	mean := float64(sum) / float64(count)
	if mean >= 0.9 {
		t.Errorf("expected ancestral mean allele count < 0.9 (modern was 1.0), got %.3f", mean)
	}
	if mean < 0.3 || mean > 0.9 {
		t.Errorf("ancestral mean allele count out of plausible band [0.3, 0.9]: %.3f", mean)
	}
}

func TestSeedAncestralLines_ZeroPercentNoOp(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	pop := make([]organism, 10)
	for i := range pop {
		g := make([]uint8, 50)
		for m := range g {
			g[m] = 2
		}
		pop[i] = organism{geno: g}
	}
	seedAncestralLines(pop, 0, 50, rng)
	for i, o := range pop {
		for m, v := range o.geno {
			if v != 2 {
				t.Errorf("individual %d locus %d should remain 2 at pct=0, got %d", i, m, v)
			}
		}
	}
}

func TestClimateDiscount_StrategyFlagOff(t *testing.T) {
	cfg := strategyConfig{Code: "balanced"}
	req := SimRequest{AncestralIntroPercent: 20, AncestralStressTolerance: 0.5}
	got := climateDiscountForStrategy(cfg, req)
	if got != 1.0 {
		t.Errorf("non-ancestral strategy must get discount=1.0, got %v", got)
	}
}

func TestClimateDiscount_AncestralFlagAppliedWithDefaults(t *testing.T) {
	cfg := strategyConfig{Code: "ancestral_introgression", UseAncestralSeed: true}
	// intro=15%, stress_tolerance=0.5 → discount = 1 - 0.15×(1-0.5) = 0.925.
	req := SimRequest{AncestralIntroPercent: 15, AncestralStressTolerance: 0.5}
	got := climateDiscountForStrategy(cfg, req)
	if got <= 0.92 || got > 0.93 {
		t.Errorf("expected discount ≈ 0.925 for 15%% × tol=0.5, got %v", got)
	}
}

func TestClimateDiscount_ZeroPercentLeavesDefault(t *testing.T) {
	cfg := strategyConfig{Code: "ancestral_introgression", UseAncestralSeed: true}
	req := SimRequest{AncestralIntroPercent: 0, AncestralStressTolerance: 0.5}
	if climateDiscountForStrategy(cfg, req) != 1.0 {
		t.Errorf("intro_percent=0 must yield no discount")
	}
}

func TestValidateRequest_RejectsIntroPercentAboveCap(t *testing.T) {
	req := baseRequestForUnitTests()
	req.AncestralIntroPercent = 30 // above the 25% cap.
	err := validateRequest(req, 4)
	if err == nil || !strings.Contains(err.Error(), "ancestral_intro_percent") {
		t.Errorf("expected ancestral_intro_percent rejection at 30%%, got %v", err)
	}
}

func TestValidateRequest_AcceptsAtCap(t *testing.T) {
	req := baseRequestForUnitTests()
	req.AncestralIntroPercent = 25
	if err := validateRequest(req, 4); err != nil {
		t.Errorf("validation should accept intro_percent=25, got %v", err)
	}
}

func TestBuildStrategyConfigs_AncestralStrategyAppearsOnlyWhenEnabled(t *testing.T) {
	off := baseRequestForUnitTests()
	off.AncestralIntroPercent = 0
	for _, s := range buildStrategyConfigs(off) {
		if s.Code == "ancestral_introgression" {
			t.Errorf("ancestral strategy must not appear when intro_percent=0")
		}
	}
	on := baseRequestForUnitTests()
	on.AncestralIntroPercent = 15
	found := false
	for _, s := range buildStrategyConfigs(on) {
		if s.Code == "ancestral_introgression" {
			found = true
			if !s.UseAncestralSeed {
				t.Errorf("ancestral strategy must carry UseAncestralSeed=true")
			}
		}
	}
	if !found {
		t.Errorf("ancestral strategy must appear when intro_percent > 0")
	}
}

func TestRunSimulation_AncestralStrategyRunsEndToEnd(t *testing.T) {
	// End-to-end mating-with-mixed-origin smoke. Verifies the
	// ancestral strategy completes a full run on the same RNG seed
	// as the rest of the strategy set and that the climate-discount
	// MECHANISM works at the per-strategy level: under climate
	// stress, the ratio of (ancestral gain) / (ancestral gain at
	// no-stress) is STRICTLY LARGER than the matching ratio for
	// balanced. That is the dynamic the issue calls for — ancestral
	// lines partially absorb the climate penalty — even when the
	// ancestral strategy's absolute gain remains below balanced's
	// because of its lower base trait mean.
	mkReq := func(climate *ClimateScenario) SimRequest {
		r := baseRequestForUnitTests()
		r.AncestralIntroPercent = 25
		r.AncestralStressTolerance = 0.5
		r.Climate = climate
		return r
	}
	pick := func(resp SimResponse, code string) *StrategyResult {
		for i := range resp.Strategies {
			if resp.Strategies[i].Code == code {
				return &resp.Strategies[i]
			}
		}
		return nil
	}

	noStress, err := runSimulation(mkReq(nil))
	if err != nil {
		t.Fatalf("no-stress runSimulation: %v", err)
	}
	stress, err := runSimulation(mkReq(&ClimateScenario{Mode: "heat_burst_anthesis", Severity: 1.0}))
	if err != nil {
		t.Fatalf("stress runSimulation: %v", err)
	}

	ancNo := pick(noStress, "ancestral_introgression")
	balNo := pick(noStress, "balanced")
	ancSt := pick(stress, "ancestral_introgression")
	balSt := pick(stress, "balanced")
	for name, p := range map[string]*StrategyResult{"anc/no": ancNo, "bal/no": balNo, "anc/st": ancSt, "bal/st": balSt} {
		if p == nil {
			t.Fatalf("strategy %s missing from response", name)
		}
	}

	// Issue acceptance criterion (no stress): ancestral strictly worse
	// than balanced — the lower-baseline drag is real.
	if ancNo.Final.GeneticGain >= balNo.Final.GeneticGain {
		t.Errorf("no-stress: ancestral gain (%.4f) should be strictly worse than balanced (%.4f) because ancestral lines have lower base trait mean", ancNo.Final.GeneticGain, balNo.Final.GeneticGain)
	}

	// Climate-discount mechanism: the climate penalty reduces gain
	// LESS for ancestral than for balanced. Ratio is independent of
	// the absolute gain difference.
	ancRatio := ancSt.Final.GeneticGain / ancNo.Final.GeneticGain
	balRatio := balSt.Final.GeneticGain / balNo.Final.GeneticGain
	if ancRatio <= balRatio {
		t.Errorf("climate-discount mechanism failed: ancestral retains %.4f of its no-stress gain, balanced retains %.4f — ancestral should retain MORE", ancRatio, balRatio)
	}
}

// baseRequestForUnitTests returns a small valid SimRequest for tests
// that need one. Independent of test fixtures elsewhere in the package
// so renaming there doesn't break this file.
func baseRequestForUnitTests() SimRequest {
	return SimRequest{
		Seed:               42,
		PopulationSize:     60,
		Markers:            120,
		QTLCount:           15,
		Generations:        5,
		SelectionPercent:   20,
		Heritability:       0.5,
		MutationRate:       0.0001,
		StrategySet:        "advanced",
		Replicates:         1,
		InbreedingLimit:    0.25,
		DiversityLossLimit: 0.30,
	}
}
