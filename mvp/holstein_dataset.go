package main

// v0.7.22 — Issue 19. Synthetic Holstein-flavoured founder population.
// Real Holstein VCFs from the 1000 Bull Genomes Project (Run 8 public via
// NCBI BioProject PRJEB42783, Run 9 controlled access via CNGB) require
// operator-side download and are too large to embed. This MVP loader
// builds a synthetic founder population whose allele-frequency-spectrum
// (AFS) mimics published Holstein data — U-shaped MAF distribution
// reflecting selective sweeps and small Ne (43–66) — while staying
// explicit about being synthetic.
//
// Honesty layer: the dataset reports isPlaceholder=true and the
// sourceNotes carry a verbatim disclosure. The /datasets page surfaces
// the same disclosure.

import (
	"fmt"
	"math"
	"math/rand"
)

// synthHolsteinFounders generates n diploid organisms with `markers`
// markers each, drawn from Holstein-flavoured allele frequencies.
//
// AFS shape: U-shaped MAF distribution via Beta(0.5, 0.5). In a fully
// neutral / large-population scenario MAF tends to be uniform; in a
// bottlenecked / selected population the distribution piles up near
// 0 and 1 (alleles drift to fixation, then mutate back rare). Beta(0.5,
// 0.5) captures that qualitatively without committing to a specific
// Holstein generation cohort.
//
// For each marker we sample allele frequency p ~ Beta(0.5, 0.5), then
// draw each organism's dosage at that marker from Binomial(2, p) — i.e.
// independent Hardy-Weinberg sampling. The resulting dataset is
// statistically Holstein-shaped but does NOT represent any specific bull
// or herd. Honest about this in sourceNotes.
func synthHolsteinFounders(n, markers int, rng *rand.Rand) *loadedDataset {
	if n < 2 {
		n = 2
	}
	if markers < 1 {
		markers = 1
	}
	pop := make([]organism, n)
	for i := range pop {
		pop[i] = organism{geno: make([]uint8, markers)}
	}
	// Sample one allele frequency per marker from Beta(0.5, 0.5). Beta
	// random variates via the standard trick: U^(1/a) / (U^(1/a) + V^(1/b))
	// is awkward — use the inverse-CDF approximation via two gamma draws.
	// Simpler: use rejection sampling on Beta(0.5, 0.5) shape. For MVP just
	// use math/rand's Beta-ish approximation by mixing of uniforms biased
	// toward 0 and 1.
	for m := 0; m < markers; m++ {
		p := sampleBetaHalfHalf(rng)
		// Hardy-Weinberg sampling: each allele copy is Bernoulli(p), two
		// copies per diploid organism, dosage = 0/1/2.
		for i := 0; i < n; i++ {
			var d uint8
			if rng.Float64() < p {
				d++
			}
			if rng.Float64() < p {
				d++
			}
			pop[i].geno[m] = d
		}
	}
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("synth_holstein_%d", i+1)
	}
	notes := []string{
		"Synthetic Holstein-flavoured founder dataset. NOT real bovine genotypes — this is a generator that mimics published Holstein AFS shape (Beta(0.5, 0.5) MAF, Hardy-Weinberg dosage sampling).",
		"Real Holstein data: 1000 Bull Genomes Run 8 (public, NCBI BioProject PRJEB42783) or Run 9 (controlled access, CNGB). See /datasets page for operator-side acquisition.",
		"Selection / recombination / mutation in subsequent generations are still simulated; only the founder genotypes are synthesised here.",
	}
	return &loadedDataset{
		individuals:   pop,
		markerCount:   markers,
		accessionIDs:  ids,
		isPlaceholder: true,
		sourceFile:    "synthetic://holstein_synthetic",
		sourceNotes:   notes,
	}
}

// sampleBetaHalfHalf returns a Beta(0.5, 0.5) variate. This is the
// arcsine distribution: equivalent to sin²(πU/2) for U ~ Uniform(0, 1).
func sampleBetaHalfHalf(rng *rand.Rand) float64 {
	u := rng.Float64()
	s := math.Sin(math.Pi * u / 2.0)
	return s * s
}

// holsteinSynthGenSeed returns the RNG used to build the synthetic
// Holstein dataset. Fixed seed so the cached dataset is deterministic;
// subsampleDataset later applies the per-request seed for sampling N×M.
func holsteinSynthGenSeed() *rand.Rand {
	return rand.New(rand.NewSource(0x1d05_2026))
}
