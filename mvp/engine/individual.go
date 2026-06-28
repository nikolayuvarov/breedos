// Package engine defines the substrate-agnostic contracts for BreedOS
// simulation. The biological kernel in `breedos/mvp/main.go` continues
// to use its own concrete `organism` type for bit-identical output and
// does not import this package; new substrates (Promptbio, future
// substrates) implement `Individual` here and reuse the same selection
// / mutation / recombination interface shape.
//
// Per Issue 07: substrate is plug-in, not the other way around — the
// engine package never imports from any substrate package. The
// biological path stays bit-identical to v0.7.32; this package
// existing alongside it is purely additive.
package engine

// Substrate is the kind of organism a simulation is operating on.
// New substrates can be added here without touching any existing
// code paths.
type Substrate string

const (
	SubstrateBiology   Substrate = "biology"
	SubstratePromptbio Substrate = "promptbio"
)

// Individual is the minimum contract every substrate must satisfy.
// Genome returns the substrate-specific byte sequence — for biology
// it is a marker array; for promptbio it is a 14-byte locus-status
// vector. Clone produces a deep copy so mutation operators can mutate
// in place without aliasing the parent.
type Individual interface {
	Genome() []byte
	Clone() Individual
}

// FitnessFunc returns a scalar fitness for an Individual under a
// substrate-specific environment. Higher is better. The biological
// kernel uses a concrete equivalent; promptbio uses a placeholder
// deterministic judge until a real LLM-judge integration is wired
// (post-v0.7.x).
type FitnessFunc func(ind Individual, env Environment) float64

// Environment is intentionally opaque — each substrate defines its
// own concrete environment type (model + history + tools for
// promptbio; weather + climate scenario for biology). Use a type
// assertion in the FitnessFunc.
type Environment interface{}

// MutationOp produces a mutated child from one parent. The op is
// expected to be deterministic given (parent, rng_state). For
// promptbio v1 the canonical mutations are locus add / remove /
// strengthen / suppress — six in total matching v0.2 Diff's kind
// catalog.
type MutationOp func(parent Individual, rng RNG) Individual

// RecombinationOp produces a child by splicing genome segments from
// two parents. Deterministic given (a, b, rng_state).
type RecombinationOp func(a, b Individual, rng RNG) Individual

// RNG is a minimal random-number-source interface that each
// substrate can wrap around `math/rand.Rand` or another source. The
// abstraction lets tests inject a deterministic counter-based RNG
// for reproducibility checks.
type RNG interface {
	Intn(n int) int
	Float64() float64
}

// StrategyTag is the substrate-aware key in the strategy registry.
// A biological run with `SubstrateBiology` cannot pick a promptbio
// strategy and vice versa; the UI substrate switch filters the
// available choices.
type StrategyTag struct {
	Substrate Substrate
	Code      string
	Name      string
}
