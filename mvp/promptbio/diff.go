package promptbio

// v0.2 Prompt Genome Diff. Static, deterministic, no LLM dependency —
// mirrors the v0.1 mapper discipline. Diff is computed against the
// 14-locus vector emitted by MapPrompt, not against raw text — so
// cosmetic rewording with no locus change yields delta_G ≈ 0.
//
// Output shape and the six mutation kinds match handoff Section 6.4 +
// `ingest-done/06-prompt-dna.md.done` (mutation classes / ΔG / ΔZ /
// ΔF and the mutation-ledger schema). v0.2 keeps target-phenotype
// fitness comparison out of scope: ΔF is returned as `nil` with a
// human-readable explanation when no target is supplied, and only as
// a directional sign when one is.
//
// LedgerID is content-addressed sha256(locus|kind|after_fragment) so
// downstream consumers (v0.3 Evolution Loop) can deduplicate identical
// mutations across lineage branches.

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// DiffRequest is the input contract for POST /api/promptbio/diff.
type DiffRequest struct {
	Ancestor    string `json:"ancestor"`
	Descendant  string `json:"descendant"`
	Language    string `json:"language,omitempty"`
	SpeciesHint string `json:"species_hint,omitempty"`
}

// MutationKind is the six-class taxonomy from handoff Section 6.4.
type MutationKind string

const (
	MutationAddition       MutationKind = "addition"
	MutationDeletion       MutationKind = "deletion"
	MutationSubstitution   MutationKind = "substitution"
	MutationAmplification  MutationKind = "amplification"
	MutationSuppression    MutationKind = "suppression"
	MutationModularization MutationKind = "modularization"
)

// GeneChange is one locus-level edit between ancestor and descendant.
// LedgerID is content-addressed and stable across runs.
type GeneChange struct {
	Locus          LocusName    `json:"locus"`
	Kind           MutationKind `json:"kind"`
	BeforeStatus   LocusStatus  `json:"before_status"`
	AfterStatus    LocusStatus  `json:"after_status"`
	BeforeFragment string       `json:"before_fragment,omitempty"`
	AfterFragment  string       `json:"after_fragment,omitempty"`
	Rationale      string       `json:"rationale"`
	LedgerID       string       `json:"ledger_id"`
}

// PhenotypeShift is the v0.2 7-axis prediction of how the descendant's
// response shape shifts relative to the ancestor's. Each axis is an
// integer in {-2, -1, 0, +1, +2} derived heuristically from which
// loci moved status — not from a runtime LLM call. v0.3 will replace
// this static derivation with an empirical reaction-norm measurement.
type PhenotypeShift struct {
	Structure   int `json:"structure"`
	Depth       int `json:"depth"`
	Accuracy    int `json:"accuracy"`
	Concreteness int `json:"concreteness"`
	Style       int `json:"style"`
	Usefulness  int `json:"usefulness"`
	Risks       int `json:"risks"`
}

// PromptDiff is the top-level diff output.
type PromptDiff struct {
	DiffID                 string              `json:"diff_id"`
	AncestorGenotype       GenomeMap           `json:"ancestor_genotype"`
	DescendantGenotype     GenomeMap           `json:"descendant_genotype"`
	GenomicDiff            []GeneChange        `json:"genomic_diff"`
	MutationLedger         []GeneChange        `json:"mutation_ledger"`
	DeltaG                 float64             `json:"delta_g"`
	DeltaZ                 PhenotypeShift      `json:"delta_z"`
	DeltaF                 *float64            `json:"delta_f,omitempty"`
	DeltaFExplanation      string              `json:"delta_f_explanation"`
	Regressions            []string            `json:"regressions"`
	NewRisks               []string            `json:"new_risks"`
	NextMutationSuggestion *MutationSuggestion `json:"next_mutation_suggestion,omitempty"`
}

// locusWeight is the weighting applied to each locus when computing
// delta_G. Core loci (Task, Constraint, Output schema) carry the most
// weight; soft-UX loci carry the least. These are the v0.2 defaults
// and can be tuned per-species in v0.4 Ecology.
var locusWeight = map[LocusName]float64{
	LocusTask:       1.5,
	LocusConstraint: 1.5,
	LocusOutput:     1.5,
	LocusRole:       1.0,
	LocusAudience:   1.0,
	LocusContext:    1.0,
	LocusMethod:     1.0,
	LocusEpistemic:  1.0,
	LocusValidation: 1.0,
	LocusSafety:     1.0,
	LocusTool:       0.7,
	LocusMemory:     0.7,
	LocusUX:         0.5,
	LocusEvolution:  0.5,
}

// DiffPrompts is the v0.2 entry point. It runs MapPrompt on both
// ancestor and descendant, walks the 14-locus vector, classifies each
// status transition into one of the six mutation kinds, computes the
// weighted ΔG, derives the 7-axis ΔZ heuristically, and surfaces
// regressions and the highest-leverage next mutation.
func DiffPrompts(req DiffRequest) PromptDiff {
	ancestor := MapPrompt(MapRequest{
		Prompt:      req.Ancestor,
		Language:    req.Language,
		SpeciesHint: req.SpeciesHint,
	})
	descendant := MapPrompt(MapRequest{
		Prompt:      req.Descendant,
		Language:    req.Language,
		SpeciesHint: req.SpeciesHint,
	})

	byName := func(g GenomeMap) map[LocusName]LocusAssessment {
		out := make(map[LocusName]LocusAssessment, len(g.Loci))
		for _, l := range g.Loci {
			out[l.Name] = l
		}
		return out
	}
	a := byName(ancestor)
	d := byName(descendant)

	changes := []GeneChange{}
	var deltaGNumer, deltaGDenom float64
	for _, name := range allLoci {
		w := locusWeight[name]
		deltaGDenom += w

		before := a[name]
		after := d[name]
		if before.Status == after.Status && before.Evidence == after.Evidence {
			continue
		}

		kind, rationale := classifyTransition(before, after)
		ch := GeneChange{
			Locus:          name,
			Kind:           kind,
			BeforeStatus:   before.Status,
			AfterStatus:    after.Status,
			BeforeFragment: before.Evidence,
			AfterFragment:  after.Evidence,
			Rationale:      rationale,
		}
		ch.LedgerID = ledgerID(ch)
		changes = append(changes, ch)
		deltaGNumer += w * absStatusDelta(before.Status, after.Status)
	}

	deltaG := 0.0
	if deltaGDenom > 0 {
		deltaG = roundTo(deltaGNumer/deltaGDenom, 3)
	}

	regressions := detectRegressions(a, d, changes)
	newRisks := detectNewRisks(a, d, changes)
	deltaZ := derivePhenotypeShift(a, d)
	next := pickNextMutation(d)

	return PromptDiff{
		DiffID:                 newDiffID(req.Ancestor, req.Descendant),
		AncestorGenotype:       ancestor,
		DescendantGenotype:     descendant,
		GenomicDiff:            changes,
		MutationLedger:         changes,
		DeltaG:                 deltaG,
		DeltaZ:                 deltaZ,
		DeltaF:                 nil,
		DeltaFExplanation:      "no target_phenotype supplied; ΔF deferred to v0.3 fitness battery",
		Regressions:            regressions,
		NewRisks:               newRisks,
		NextMutationSuggestion: next,
	}
}

func classifyTransition(before, after LocusAssessment) (MutationKind, string) {
	bs, as := before.Status, after.Status
	switch {
	case bs == LocusConflicting && as != LocusConflicting:
		return MutationModularization, "resolved conflicting locus into " + string(as)
	case as == LocusConflicting:
		return MutationDeletion, "introduced contradiction in " + string(after.Name)
	case (bs == LocusMissing || bs == LocusWeak || bs == LocusNotApplicable) &&
		(as == LocusPresent || as == LocusStrong):
		return MutationAddition, "added " + string(after.Name) + " locus"
	case (bs == LocusPresent || bs == LocusStrong) &&
		(as == LocusMissing || as == LocusWeak || as == LocusNotApplicable):
		return MutationDeletion, "removed " + string(after.Name) + " locus"
	case bs == LocusPresent && as == LocusStrong:
		return MutationAmplification, "amplified " + string(after.Name) + " from present to strong"
	case bs == LocusStrong && as == LocusPresent:
		return MutationSuppression, "suppressed " + string(after.Name) + " from strong to present"
	case bs == as && before.Evidence != after.Evidence:
		return MutationSubstitution, "rewrote " + string(after.Name) + " fragment without status change"
	}
	return MutationSubstitution, "transition " + string(bs) + " → " + string(as)
}

func absStatusDelta(b, a LocusStatus) float64 {
	sb, _ := statusScore(b)
	sa, _ := statusScore(a)
	d := sa - sb
	if d < 0 {
		return -d
	}
	return d
}

func ledgerID(ch GeneChange) string {
	h := sha256.Sum256([]byte(string(ch.Locus) + "|" + string(ch.Kind) + "|" + ch.AfterFragment))
	return "m_" + hex.EncodeToString(h[:8])
}

func newDiffID(ancestor, descendant string) string {
	h := sha256.Sum256([]byte(ancestor + "\n--\n" + descendant))
	return "d_" + hex.EncodeToString(h[:8])
}

func detectRegressions(a, d map[LocusName]LocusAssessment, changes []GeneChange) []string {
	out := []string{}
	for _, ch := range changes {
		if ch.Kind == MutationDeletion || ch.Kind == MutationSuppression {
			before := a[ch.Locus]
			after := d[ch.Locus]
			risk, _ := locusRiskAndMutation(ch.Locus)
			out = append(out, "lost "+string(ch.Locus)+" ("+string(before.Status)+" → "+string(after.Status)+"); risk: "+risk)
		}
	}
	return out
}

func detectNewRisks(a, d map[LocusName]LocusAssessment, changes []GeneChange) []string {
	out := []string{}
	for _, ch := range changes {
		if ch.AfterStatus == LocusConflicting && ch.BeforeStatus != LocusConflicting {
			out = append(out, "introduced conflicting "+string(ch.Locus)+" — competing instructions detected in descendant")
		}
	}
	for _, name := range allLoci {
		before := a[name]
		after := d[name]
		hadIt := before.Status == LocusPresent || before.Status == LocusStrong
		lostIt := after.Status == LocusMissing || after.Status == LocusWeak || after.Status == LocusNotApplicable
		if hadIt && lostIt && name == LocusSafety {
			out = append(out, "safety boundary weakened — unsafe output risk now elevated")
		}
		if hadIt && lostIt && name == LocusValidation {
			out = append(out, "validation step lost — unchecked-failure risk now elevated")
		}
	}
	return out
}

func derivePhenotypeShift(a, d map[LocusName]LocusAssessment) PhenotypeShift {
	axisDelta := func(loci ...LocusName) int {
		var sum float64
		for _, l := range loci {
			sum += statusNumeric(d[l].Status) - statusNumeric(a[l].Status)
		}
		avg := sum / float64(len(loci))
		switch {
		case avg >= 1.5:
			return 2
		case avg >= 0.5:
			return 1
		case avg <= -1.5:
			return -2
		case avg <= -0.5:
			return -1
		}
		return 0
	}
	risksDelta := -axisDelta(LocusSafety, LocusConstraint, LocusTool)
	return PhenotypeShift{
		Structure:    axisDelta(LocusOutput, LocusMethod),
		Depth:        axisDelta(LocusContext, LocusConstraint, LocusValidation),
		Accuracy:     axisDelta(LocusEpistemic, LocusValidation),
		Concreteness: axisDelta(LocusContext, LocusConstraint),
		Style:        axisDelta(LocusAudience, LocusRole, LocusUX),
		Usefulness:   axisDelta(LocusTask, LocusAudience, LocusValidation),
		Risks:        risksDelta,
	}
}

func statusNumeric(s LocusStatus) float64 {
	switch s {
	case LocusStrong:
		return 3
	case LocusPresent:
		return 2
	case LocusWeak:
		return 1
	case LocusConflicting:
		return -1
	}
	return 0
}

func pickNextMutation(descendant map[LocusName]LocusAssessment) *MutationSuggestion {
	prioritised := []LocusName{
		LocusTask,
		LocusConstraint,
		LocusOutput,
		LocusEpistemic,
		LocusValidation,
		LocusContext,
		LocusMethod,
		LocusAudience,
		LocusRole,
		LocusSafety,
	}
	for _, name := range prioritised {
		a, ok := descendant[name]
		if !ok {
			continue
		}
		if a.Status == LocusMissing || a.Status == LocusWeak {
			_, mutation := locusRiskAndMutation(name)
			return &MutationSuggestion{
				MutationType: string(MutationAddition),
				TargetLocus:  string(name),
				Patch:        mutation,
				Rationale:    "highest-leverage missing locus in descendant",
			}
		}
	}
	return nil
}

// joinKinds is a small helper for diagnostic strings.
func joinKinds(kinds []MutationKind) string {
	bits := make([]string, len(kinds))
	for i, k := range kinds {
		bits[i] = string(k)
	}
	return strings.Join(bits, ", ")
}
