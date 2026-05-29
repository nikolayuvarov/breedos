package main

// v0.7.22 — Issue 22. Methane trait defaults. Constants are documented
// with citation references in comments below. All values came through
// the 2026-05-28 freshness audit; the sign of the milk × methane
// correlation is the one that the original v0.7.18 scoping had wrong.
//
// Key insight (per audit): the *direction* of the correlation depends on
// which methane metric is used.
//
//   * Methane production (MeP)  × milk yield                  : +0.35 (unfavourable)
//   * Methane yield (MeY)       × corrected milk yield        : −0.43 (favourable)
//   * Methane intensity (MeI)   × corrected milk yield        : −0.26 (favourable)
//
// The preset (Issue 25) defaults to MeI so selecting against methane
// also helps milk yield (favourable correlation). A second preset variant
// uses MeP to show the unfavourable trade-off case explicitly.
//
// References:
//   Brito et al. 2022 "Estimates of the genetic contribution to methane
//     emission in dairy cows: a meta-analysis", Sci Rep — h² values.
//   Methane × milk-composition associations — PMC 9404742 (MeY × corrected
//     milk yield = −0.43, MeI × corrected milk yield = −0.26).

const (
	// Heritability defaults (meta-analysis Brito et al. 2022).
	methaneYieldHeritability      = 0.244
	methaneIntensityHeritability  = 0.180
	methaneProductionHeritability = 0.211
	holsteinMilkHeritability      = 0.36

	// Genetic correlations.
	corrMilkYieldMethaneProduction = +0.35 // unfavourable
	corrMilkYieldMethaneYield      = -0.43 // favourable
	corrMilkYieldMethaneIntensity  = -0.26 // favourable
)

// methaneIntensityDefaults builds the 2-trait config used by the "Methane
// (dairy, intensity)" preset (Issue 25). Returns trait list and the matching
// correlation matrix. The two slices stay consistent with each other —
// callers should not mutate either independently.
func methaneIntensityDefaults() ([]TraitConfig, [][]float64) {
	traits := []TraitConfig{
		{Name: "milk_yield", Heritability: holsteinMilkHeritability, QTLCount: 30, EffectScale: 1.0, SelectionWeight: 1.0},
		{Name: "methane_intensity", Heritability: methaneIntensityHeritability, QTLCount: 20, EffectScale: 1.0, SelectionWeight: -0.5},
	}
	corr := [][]float64{
		{1.0, corrMilkYieldMethaneIntensity},
		{corrMilkYieldMethaneIntensity, 1.0},
	}
	return traits, corr
}

// methaneProductionDefaults is the educational-contrast preset: methane
// PRODUCTION × milk yield is positively correlated (unfavourable), so
// selecting against methane drags milk down. This preset lets the operator
// see that trade-off explicitly. Same trait shape, different correlation.
func methaneProductionDefaults() ([]TraitConfig, [][]float64) {
	traits := []TraitConfig{
		{Name: "milk_yield", Heritability: holsteinMilkHeritability, QTLCount: 30, EffectScale: 1.0, SelectionWeight: 1.0},
		{Name: "methane_production", Heritability: methaneProductionHeritability, QTLCount: 20, EffectScale: 1.0, SelectionWeight: -0.5},
	}
	corr := [][]float64{
		{1.0, corrMilkYieldMethaneProduction},
		{corrMilkYieldMethaneProduction, 1.0},
	}
	return traits, corr
}
