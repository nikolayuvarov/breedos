package promptbio

// v0.7.33 — Issue 07 substrate abstraction. PromptOrganism is the
// promptbio implementation of `engine.Individual`. The genome is the
// 14-byte locus-status vector from v0.1 (one byte per locus, value
// = LocusStatus encoded as an integer): the canonical "DNA" of a
// prompt in the substrate-agnostic kernel.
//
// Per the Issue 07 non-goal list: no real LLM-judge, no persistence,
// no co-evolution. v0.7.33 ships a placeholder deterministic judge
// so the engine kernel can run end-to-end on the prompt substrate.

import (
	"math/rand"

	"breedos-mvp/engine"
)

// encodeStatus maps a LocusStatus to a single byte. The mapping is
// deliberately compact and contiguous so byte-level mutation
// operators can flip a status by ±1 to step through the ladder.
func encodeStatus(s LocusStatus) byte {
	switch s {
	case LocusMissing:
		return 0
	case LocusWeak:
		return 1
	case LocusPresent:
		return 2
	case LocusStrong:
		return 3
	case LocusConflicting:
		return 4
	case LocusNotApplicable:
		return 5
	}
	return 0
}

func decodeStatus(b byte) LocusStatus {
	switch b {
	case 0:
		return LocusMissing
	case 1:
		return LocusWeak
	case 2:
		return LocusPresent
	case 3:
		return LocusStrong
	case 4:
		return LocusConflicting
	case 5:
		return LocusNotApplicable
	}
	return LocusMissing
}

// PromptOrganism implements engine.Individual over the 14-locus
// status vector. Genome layout is the same locus order as the
// `allLoci` slice in types.go.
type PromptOrganism struct {
	Statuses [14]byte
}

// Genome returns a copy of the 14-byte locus vector. Required by
// engine.Individual.
func (p *PromptOrganism) Genome() []byte {
	out := make([]byte, 14)
	copy(out, p.Statuses[:])
	return out
}

// Clone returns a deep copy. Required by engine.Individual.
func (p *PromptOrganism) Clone() engine.Individual {
	cp := &PromptOrganism{}
	cp.Statuses = p.Statuses
	return cp
}

// FromGenomeMap projects a v0.1 GenomeMap onto a PromptOrganism,
// preserving the locus order. Used to seed a population from an
// ancestor prompt.
func FromGenomeMap(g GenomeMap) *PromptOrganism {
	o := &PromptOrganism{}
	byName := map[LocusName]LocusStatus{}
	for _, l := range g.Loci {
		byName[l.Name] = l.Status
	}
	for i, name := range allLoci {
		if s, ok := byName[name]; ok {
			o.Statuses[i] = encodeStatus(s)
		} else {
			o.Statuses[i] = encodeStatus(LocusMissing)
		}
	}
	return o
}

// MutationStep flips one locus by one step on the status ladder.
// Deterministic given the rng state. Implements engine.MutationOp.
func MutationStep(parent engine.Individual, rng engine.RNG) engine.Individual {
	p, ok := parent.(*PromptOrganism)
	if !ok {
		return parent.Clone()
	}
	child := p.Clone().(*PromptOrganism)
	idx := rng.Intn(14)
	// Choose direction by uniform random in [0, 1).
	if rng.Float64() < 0.5 && child.Statuses[idx] > 0 {
		child.Statuses[idx]--
	} else if child.Statuses[idx] < 3 {
		child.Statuses[idx]++
	}
	return child
}

// RecombineUniform splices loci from two parents at uniform random.
// Implements engine.RecombinationOp. Deterministic given the rng.
func RecombineUniform(a, b engine.Individual, rng engine.RNG) engine.Individual {
	pa, oka := a.(*PromptOrganism)
	pb, okb := b.(*PromptOrganism)
	if !oka || !okb {
		return a.Clone()
	}
	child := &PromptOrganism{}
	for i := 0; i < 14; i++ {
		if rng.Intn(2) == 0 {
			child.Statuses[i] = pa.Statuses[i]
		} else {
			child.Statuses[i] = pb.Statuses[i]
		}
	}
	return child
}

// PlaceholderJudge is the v0.7.33 deterministic fitness function. It
// sums the locus weights times the encoded status value, normalised
// to [0, 1]. The biological-side equivalent is the QTL phenotype
// sum; the structural shape matches so the engine kernel reads
// uniformly across substrates.
//
// `env` is currently unused — added as a parameter so the signature
// matches engine.FitnessFunc for future environment-conditioned
// scoring (Issue 04 Ecology, Issue 05 Immunology).
func PlaceholderJudge(ind engine.Individual, env engine.Environment) float64 {
	p, ok := ind.(*PromptOrganism)
	if !ok {
		return 0
	}
	var weighted, max float64
	for i, name := range allLoci {
		w := locusWeight[name]
		v := float64(p.Statuses[i])
		// Cap at the "strong" value (3) so out-of-range statuses
		// (conflicting=4, not_applicable=5) don't inflate the score.
		if v > 3 {
			v = 0
		}
		weighted += w * v
		max += w * 3
	}
	if max == 0 {
		return 0
	}
	return weighted / max
}

// stdRNG is a thin adapter around `math/rand.Rand` so the engine.RNG
// interface stays tiny. Used by the simulator entry point.
type stdRNG struct{ r *rand.Rand }

func (s *stdRNG) Intn(n int) int     { return s.r.Intn(n) }
func (s *stdRNG) Float64() float64   { return s.r.Float64() }
func newStdRNG(seed int64) engine.RNG { return &stdRNG{r: rand.New(rand.NewSource(seed))} }
