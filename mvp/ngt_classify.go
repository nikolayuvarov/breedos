package main

// v0.7.18 — Issue 13: NGT-1 vs NGT-2 classification engine.
//
// Applies the EU NGT regulation's "20/20 rule" (max 20 modifications, each
// insertion ≤ 20 bp) plus the trait-class auto-exclusions to a planned edit
// set. Returns one of:
//
//   NGT-1          — equivalent to conventionally bred; deregulated.
//   NGT-2          — full GMO authorisation, traceability, labelling.
//   unclassifiable — ambiguous (missing inputs); user must supply more info.
//
// The classifier is intentionally pure: no I/O, no globals, easy to test.
// Final regulatory category in the real world is decided by competent
// authorities; this is a *planning aid*. The ConfidenceNote on every result
// carries that disclaimer verbatim.
//
// References:
//   Council adopted regulation 2026-04-21 (applies mid-2028).
//   "20/20 rule": Commission proposal of July 2023, Annex I.
//   Auto-exclusions: herbicide tolerance, production of known insecticidal
//   substances → NGT-2 by default.

import (
	"fmt"
)

const (
	maxModificationsNGT1 = 20
	maxInsertBpNGT1      = 20
	ngtDisclaimer        = "Not legal advice. Classification is a planning aid based on the 20/20 rule and published trait-class exclusions. Final regulatory category is determined by competent authorities."
)

// NGTContext holds the run-level user inputs the classifier needs.
// All zero-values are explicit "unset" and force an unclassifiable result —
// the product must never silently guess in this domain.
type NGTContext struct {
	// TargetTraitClass categorises what the edits intend to achieve.
	// One of: "productivity" | "quality" | "disease_resistance" |
	// "herbicide_tolerance" | "insecticidal" | "" (unset).
	TargetTraitClass string `json:"target_trait_class"`

	// DonorSource describes the origin of edited alleles.
	// One of: "none" | "same_species" | "same_gene_pool" | "cross_species" |
	// "" (unset). "cross_species" disqualifies NGT-1 (introduces foreign DNA).
	DonorSource string `json:"donor_source"`

	// Issue 16 — optional, informational. EU NGT-1 registration requires
	// declaring patent rights; these fields propagate into the JSON export so
	// they don't get lost between planning and filing. The classifier itself
	// does not act on these values; the frontend uses PatentID to decide
	// whether to render a "NGT-1 needs patent declaration" warning.
	PatentID        string `json:"patent_id,omitempty"`
	LicensingStatus string `json:"licensing_status,omitempty"`
	Notes           string `json:"notes,omitempty"`
}

// NGTClassification is the engine's verdict + supporting info.
type NGTClassification struct {
	Category       string   `json:"category"`
	Reasons        []string `json:"reasons"`
	Disqualifiers  []string `json:"disqualifiers,omitempty"`
	ConfidenceNote string   `json:"confidence_note"`
}

// validNGTTraitClasses and validNGTDonorSources are checked explicitly so
// the user can't smuggle in unknown values and have them silently default
// to "passes".
var validNGTTraitClasses = map[string]bool{
	"productivity":         true,
	"quality":              true,
	"disease_resistance":   true,
	"herbicide_tolerance":  true,
	"insecticidal":         true,
}
var validNGTDonorSources = map[string]bool{
	"none":            true,
	"same_species":    true,
	"same_gene_pool":  true,
	"cross_species":   true,
}

// ngtTraitClassExclusions maps trait-class values that disqualify NGT-1
// even when the 20/20 numerical limits are satisfied. (Article on
// excluded categories — see file header references.)
var ngtTraitClassExclusions = map[string]string{
	"herbicide_tolerance": "target trait class 'herbicide_tolerance' is excluded from NGT-1 by regulation",
	"insecticidal":        "target trait class 'insecticidal' is excluded from NGT-1 by regulation",
}

// ClassifyEditSet evaluates the planned edit set under the 20/20 rule
// plus trait-class and donor-source rules. It is pure: same inputs →
// same output.
//
// For MVP simulation, every modelled edit is an SNV-equivalent (1 bp
// substitution on a marker locus). Insert-length is therefore implicitly
// 1 bp per edit — well within the 20 bp NGT-1 limit. The count rule is
// what bites in the simulation: more than 20 candidate edits per run
// disqualifies.
func ClassifyEditSet(edits []EditCandidate, ctx NGTContext) NGTClassification {
	var disq []string
	var reasons []string

	// Edge case: no edits.
	n := len(edits)
	if n == 0 {
		return NGTClassification{
			Category:       "unclassifiable",
			Reasons:        []string{"No edits planned — classification does not apply."},
			ConfidenceNote: ngtDisclaimer,
		}
	}

	// Rule 1: count ≤ 20.
	if n > maxModificationsNGT1 {
		disq = append(disq, fmt.Sprintf("count exceeds %d-modifications limit (%d edits)", maxModificationsNGT1, n))
	} else {
		reasons = append(reasons, fmt.Sprintf("%d edits — within the %d-modifications NGT-1 limit", n, maxModificationsNGT1))
	}

	// Rule 2: trait class. Must be set; certain values auto-exclude.
	if ctx.TargetTraitClass == "" {
		return NGTClassification{
			Category:       "unclassifiable",
			Reasons:        reasons,
			Disqualifiers:  append(disq, "missing target_trait_class — set it to one of {productivity, quality, disease_resistance, herbicide_tolerance, insecticidal} to classify"),
			ConfidenceNote: ngtDisclaimer,
		}
	}
	if !validNGTTraitClasses[ctx.TargetTraitClass] {
		return NGTClassification{
			Category:       "unclassifiable",
			Reasons:        reasons,
			Disqualifiers:  append(disq, fmt.Sprintf("target_trait_class %q is not a recognised value", ctx.TargetTraitClass)),
			ConfidenceNote: ngtDisclaimer,
		}
	}
	if excl, ok := ngtTraitClassExclusions[ctx.TargetTraitClass]; ok {
		disq = append(disq, excl)
	} else {
		reasons = append(reasons, fmt.Sprintf("trait class %q is not on the NGT-1 exclusion list", ctx.TargetTraitClass))
	}

	// Rule 3: donor source. Must be set; cross_species auto-excludes.
	if ctx.DonorSource == "" {
		return NGTClassification{
			Category:       "unclassifiable",
			Reasons:        reasons,
			Disqualifiers:  append(disq, "missing donor_source — set it to one of {none, same_species, same_gene_pool, cross_species} to classify"),
			ConfidenceNote: ngtDisclaimer,
		}
	}
	if !validNGTDonorSources[ctx.DonorSource] {
		return NGTClassification{
			Category:       "unclassifiable",
			Reasons:        reasons,
			Disqualifiers:  append(disq, fmt.Sprintf("donor_source %q is not a recognised value", ctx.DonorSource)),
			ConfidenceNote: ngtDisclaimer,
		}
	}
	if ctx.DonorSource == "cross_species" {
		disq = append(disq, "donor_source 'cross_species' introduces foreign DNA — disqualifies NGT-1")
	} else {
		reasons = append(reasons, fmt.Sprintf("donor_source %q is consistent with NGT-1 eligibility", ctx.DonorSource))
	}

	if len(disq) > 0 {
		return NGTClassification{
			Category:       "NGT-2",
			Reasons:        reasons,
			Disqualifiers:  disq,
			ConfidenceNote: ngtDisclaimer,
		}
	}
	return NGTClassification{
		Category:       "NGT-1",
		Reasons:        reasons,
		ConfidenceNote: ngtDisclaimer,
	}
}
