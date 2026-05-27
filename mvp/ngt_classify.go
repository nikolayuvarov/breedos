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
	ngtDisclaimer        = "Not legal advice. Classification is a planning aid based on Annex I of the EU NGT Regulation (Council adoption 2026-04-21, applies from mid-2028). Final regulatory category is determined by competent authorities."
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

	// v0.7.19 — Issue 32. VariantType narrows the NGT-1 path:
	//   ""              → defaults to "snv_or_small" (Path (i), backward-compat).
	//   "snv_or_small"  → Path (i): substitution/insertion ≤ 20 nt anywhere.
	//   "inversion"     → Path (i): any-size inversion anywhere in genome.
	//   "deletion"      → Path (i): any-size deletion anywhere.
	//   "gene_pool_insertion" → Path (ii): any-size contiguous DNA from the
	//                   breeder's gene pool; REQUIRES EndogenousGeneInterrupted
	//                   to be set and false to pass NGT-1.
	VariantType string `json:"variant_type,omitempty"`

	// v0.7.19 — Issue 32. Only meaningful when VariantType == "gene_pool_insertion".
	// nil = "not yet evaluated" → classifier returns unclassifiable.
	// true = at least one planned insertion locus disrupts an endogenous gene
	//        → disqualifies NGT-1 (per Annex I Path (ii)).
	// false = operator has confirmed no endogenous gene is disrupted → NGT-1 eligible.
	EndogenousGeneInterrupted *bool `json:"endogenous_gene_interrupted,omitempty"`

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

// v0.7.19 — Issue 32. Recognised variant_type values.
var validNGTVariantTypes = map[string]bool{
	"snv_or_small":        true,
	"inversion":           true,
	"deletion":            true,
	"gene_pool_insertion": true,
}

// genePoolDonorSources are the donor_source values that are consistent with
// a Path (ii) gene_pool_insertion. cross_species and none are not — neither
// represents an allele drawn from the breeder's gene pool as Annex I defines
// it (same species or any species capable of being cross-bred including via
// advanced breeding techniques).
var genePoolDonorSources = map[string]bool{
	"same_species":   true,
	"same_gene_pool": true,
}

// ngtTraitClassExclusions maps trait-class values that disqualify NGT-1
// even when the 20/20 numerical limits are satisfied. (Article on
// excluded categories — see file header references.)
var ngtTraitClassExclusions = map[string]string{
	"herbicide_tolerance": "target trait class 'herbicide_tolerance' is excluded from NGT-1 by regulation",
	"insecticidal":        "target trait class 'insecticidal' is excluded from NGT-1 by regulation",
}

// ClassifyEditSet evaluates the planned edit set under the EU NGT
// Regulation Annex I criteria. It is pure: same inputs → same output.
//
// Annex I distinguishes two NGT-1 paths inside the 20-modification envelope:
//
//	Path (i)  — deletions or inversions of any number of nucleotides, OR
//	            insertions/substitutions of ≤ 20 arbitrary nucleotides,
//	            anywhere in the genome.
//	Path (ii) — insertions/substitutions of any-sized contiguous DNA from the
//	            breeder's gene pool — only if no endogenous gene is disrupted.
//
// The MVP models all edits at marker-level (SNV-equivalent). The classifier
// represents the path choice through ctx.VariantType:
//   - "snv_or_small" (default) / "inversion" / "deletion" → Path (i)
//   - "gene_pool_insertion"                               → Path (ii)
//
// The 20-modification count rule applies to either path; Path (ii) additionally
// requires EndogenousGeneInterrupted to be set and false. When operator hasn't
// declared either VariantType or EndogenousGeneInterrupted for a gene_pool_
// insertion the classifier returns unclassifiable — it must never silently
// guess on a Path (ii) case.
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

	// Rule 1: count ≤ 20 (applies to both Path (i) and Path (ii)).
	if n > maxModificationsNGT1 {
		disq = append(disq, fmt.Sprintf("count exceeds %d-modifications limit (%d edits)", maxModificationsNGT1, n))
	} else {
		reasons = append(reasons, fmt.Sprintf("%d edits — within the %d-modifications NGT-1 limit", n, maxModificationsNGT1))
	}

	// v0.7.19 — Issue 32. Variant-type validation. Empty defaults to
	// "snv_or_small" (Path (i)) for backward compatibility with v0.7.18
	// payloads that omit the field.
	variantType := ctx.VariantType
	if variantType == "" {
		variantType = "snv_or_small"
	}
	if !validNGTVariantTypes[variantType] {
		return NGTClassification{
			Category:       "unclassifiable",
			Reasons:        reasons,
			Disqualifiers:  append(disq, fmt.Sprintf("variant_type %q is not a recognised value (use one of snv_or_small, inversion, deletion, gene_pool_insertion)", ctx.VariantType)),
			ConfidenceNote: ngtDisclaimer,
		}
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

	// v0.7.19 — Issue 32. Path-specific checks.
	switch variantType {
	case "snv_or_small":
		reasons = append(reasons, "Path (i): SNV / small insertion-substitution; insertions implicitly ≤ 20 nt at marker resolution.")
	case "inversion":
		reasons = append(reasons, "Path (i): inversion of any size, anywhere in genome — allowed.")
	case "deletion":
		reasons = append(reasons, "Path (i): deletion of any size, anywhere in genome — allowed.")
	case "gene_pool_insertion":
		// Path (ii): donor must be gene-pool; endogenous gene check is mandatory.
		if !genePoolDonorSources[ctx.DonorSource] {
			disq = append(disq, fmt.Sprintf("Path (ii) gene-pool insertion requires donor_source 'same_species' or 'same_gene_pool'; got %q", ctx.DonorSource))
		}
		if ctx.EndogenousGeneInterrupted == nil {
			return NGTClassification{
				Category:       "unclassifiable",
				Reasons:        reasons,
				Disqualifiers:  append(disq, "Path (ii) gene-pool insertion requires endogenous_gene_interrupted to be declared (true/false) — operator must confirm no endogenous gene is disrupted before NGT-1 can be granted"),
				ConfidenceNote: ngtDisclaimer,
			}
		}
		if *ctx.EndogenousGeneInterrupted {
			disq = append(disq, "Path (ii) gene-pool insertion disrupts an endogenous gene — excluded from NGT-1 by Annex I (\"shall not disrupt any endogenous genes\")")
		} else {
			reasons = append(reasons, "Path (ii) gene-pool insertion: operator declared no endogenous gene is disrupted.")
		}
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
