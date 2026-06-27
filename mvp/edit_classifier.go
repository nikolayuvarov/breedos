package main

// v0.7.27 — Issue 07. Edit-vs-Cross-vs-Wait classifier.
//
// Operator-facing question: for each candidate locus, is the best move
// to (a) edit it now, (b) propagate it via selection/crossing, or
// (c) wait and validate? The classifier returns the class, a
// machine-readable reason code, a human-readable explanation, a
// suggested introgression posture, and a risk warning string per
// candidate.
//
// The rules are intentionally explicit and ordered. They use only
// quantities already in scope (effect, allele frequency, baseline
// diversity). Future inputs the issue mentions (edit cost,
// validation_confidence, off-target risk from external tools, target
// background lines) plug in as additional rules without restructuring.

import "fmt"

// EditDecision is the structured edit-vs-cross-vs-wait classification.
// Returned per candidate edit and surfaced both in /api/simulate JSON
// (under EditCandidate.Classification) and in the demo edit table.
type EditDecision struct {
	Class                string `json:"class"`                  // "edit" | "cross" | "wait"
	ReasonCode           string `json:"reason_code"`            // machine-readable; stable across versions
	Reason               string `json:"reason"`                 // human-readable, references the numeric inputs that drove the decision
	IntrogressionPosture string `json:"introgression_posture"`  // suggested deployment posture
	RiskWarning          string `json:"risk_warning"`           // separate from EditCandidate.DiversityRisk; this is the edit-specific warning
}

// Thresholds. Centralised so a follow-up calibration pass can tune them
// without hunting through the rule body. The defaults are conservative:
// "edit" requires a clear effect AND a rare allele; the gray zones lean
// to "cross" or "wait" rather than recommending an invasive intervention
// the simulator cannot model fully.
const (
	editClassMarginalEffect      = 0.10 // |effect| below this is marginal → wait/validate.
	editClassLargeEffect         = 0.30 // |effect| at-or-above this is "large".
	editClassHighEffect          = 1.40 // pleiotropy / background-validation flag (matches the legacy rankEditCandidates threshold).
	editClassEditFreqCeiling     = 0.20 // p below this means selection would take many generations → edit candidate.
	editClassCrossFreqFloor      = 0.20 // p at-or-above this means segregating → cross can propagate.
	editClassNearFixation        = 0.92 // p at-or-above this means selection/drift completes the lift; edit adds nothing.
	editClassDiversityBottleneck = 0.15 // population-level diversity below this is a bottleneck; editing into it compounds the risk.
	editClassVeryRareFreq        = 0.10 // p below this is rare enough that high-effect introgression needs an extra-cautious posture.
)

// classifyEditCandidate evaluates one locus given its QTL effect, its
// current allele frequency, and the population's baseline diversity.
// Caller filters non-positive effects upstream (rankEditCandidates);
// classifier defends against bad input anyway.
func classifyEditCandidate(effect, allelFreq, baseDiversity float64) EditDecision {
	e := effect
	if e < 0 {
		e = -e
	}
	p := allelFreq

	switch {
	// Rule 1 — WAIT: allele is essentially fixed in the favourable direction.
	case p >= editClassNearFixation:
		return EditDecision{
			Class:                "wait",
			ReasonCode:           "NEAR_FIXATION",
			Reason:               fmt.Sprintf("Favourable allele is already at frequency %.2f; ongoing selection or drift completes the lift. Edit adds no meaningful gain.", p),
			IntrogressionPosture: "skip edit — population trajectory will complete the lift on its own",
			RiskWarning:          "low — opportunity cost of editing exceeds expected benefit",
		}

	// Rule 2 — WAIT: effect is below the practical-gain threshold.
	case e < editClassMarginalEffect:
		return EditDecision{
			Class:                "wait",
			ReasonCode:           "MARGINAL_EFFECT",
			Reason:               fmt.Sprintf("QTL effect %.3f is below the practical-gain threshold (%.2f standardised units); validation cost likely exceeds expected benefit.", effect, editClassMarginalEffect),
			IntrogressionPosture: "defer until validated against real phenotype data and effect estimate is refined",
			RiskWarning:          "low — marginal candidate, validation cost is the dominant concern",
		}

	// Rule 3 — WAIT: editing into a bottleneck. Rare allele on top of a
	// narrow founder set compounds the bottleneck risk; restore diversity
	// (outcross / introgress unrelated germplasm) before editing.
	case baseDiversity < editClassDiversityBottleneck && p < editClassVeryRareFreq:
		return EditDecision{
			Class:                "wait",
			ReasonCode:           "BOTTLENECK_RISK",
			Reason:               fmt.Sprintf("Founder diversity %.3f is below the %.2f bottleneck floor and allele is rare (p = %.3f); editing on top of a narrow population would compound the bottleneck.", baseDiversity, editClassDiversityBottleneck, p),
			IntrogressionPosture: "defer until diversity is restored via outcrossing or unrelated-germplasm introgression",
			RiskWarning:          "medium-high: editing into a narrow founder set risks a one-line-derived collapse",
		}

	// Rule 4 — EDIT: large effect on a rare allele. Selection alone
	// would take many generations to lift; the edit produces immediate
	// progress that the cross path cannot match in a feasible horizon.
	case e >= editClassLargeEffect && p < editClassEditFreqCeiling:
		posture := "seed edit into 5–10% of founders, then propagate via balanced selection"
		risk := "low–medium: standard introgression risks (background effects, allele × environment uncertainty)"
		if e > editClassHighEffect && p < editClassVeryRareFreq {
			posture = "seed edit into ≤2% of founders only — high-effect very-rare introgression carries pleiotropy + bottleneck risk"
			risk = "medium: validate pleiotropy and background effects before scaling beyond a pilot block"
		}
		return EditDecision{
			Class:                "edit",
			ReasonCode:           "LARGE_EFFECT_RARE_ALLELE",
			Reason:               fmt.Sprintf("Effect %.3f is large and allele is rare (p = %.3f); selection alone would need many generations to lift this allele, editing produces immediate progress.", effect, p),
			IntrogressionPosture: posture,
			RiskWarning:          risk,
		}

	// Rule 5 — CROSS: favourable allele already segregating at workable
	// frequency. Selection / recombination propagate it without an edit.
	case p >= editClassCrossFreqFloor && p < editClassNearFixation:
		return EditDecision{
			Class:                "cross",
			ReasonCode:           "ALREADY_SEGREGATING",
			Reason:               fmt.Sprintf("Favourable allele segregates at p = %.2f with effect %.3f; selection or crossing propagates it without an edit.", p, effect),
			IntrogressionPosture: "skip edit — use selection or directed crossing to raise the allele",
			RiskWarning:          "low — established route via selection; no edit-specific risk",
		}

	// Rule 6 — EDIT (mid-band): rare allele, moderate effect. The gray
	// band where editing wins on time-to-fixation against pure selection
	// but does not warrant the high-effect posture.
	case p < editClassEditFreqCeiling:
		return EditDecision{
			Class:                "edit",
			ReasonCode:           "MID_BAND_RARE_FAVOUR_EDIT",
			Reason:               fmt.Sprintf("Allele is rare (p = %.3f) and effect %.3f is above the marginal threshold; editing is faster than selection alone in this band.", p, effect),
			IntrogressionPosture: "seed edit into ~5% of founders, then propagate via balanced selection",
			RiskWarning:          "low–medium: standard introgression risks",
		}

	// Default — CROSS: everything else falls through to "let selection do it".
	default:
		return EditDecision{
			Class:                "cross",
			ReasonCode:           "DEFAULT_CROSS",
			Reason:               fmt.Sprintf("Effect %.3f at frequency %.2f does not meet the edit criteria; default to selection/crossing.", effect, p),
			IntrogressionPosture: "use selection — no edit warranted under current thresholds",
			RiskWarning:          "low",
		}
	}
}

// SummarizeEditDecisions counts class membership across a set of
// candidate edits. Used by the Decision Report and Notes section.
type EditDecisionSummary struct {
	TotalCandidates int    `json:"total_candidates"`
	EditCount       int    `json:"edit_count"`
	CrossCount      int    `json:"cross_count"`
	WaitCount       int    `json:"wait_count"`
	Headline        string `json:"headline"` // one-sentence aggregate verdict.
}

func summarizeEditDecisions(candidates []EditCandidate) EditDecisionSummary {
	out := EditDecisionSummary{TotalCandidates: len(candidates)}
	for _, c := range candidates {
		if c.Classification == nil {
			continue
		}
		switch c.Classification.Class {
		case "edit":
			out.EditCount++
		case "cross":
			out.CrossCount++
		case "wait":
			out.WaitCount++
		}
	}
	switch {
	case out.TotalCandidates == 0:
		out.Headline = "No candidate edits ranked — set crispr_edits > 0 to populate the edit decision table."
	case out.EditCount == out.TotalCandidates:
		out.Headline = fmt.Sprintf("All %d ranked candidates classify as EDIT — large-effect rare alleles where selection alone is too slow.", out.TotalCandidates)
	case out.CrossCount == out.TotalCandidates:
		out.Headline = fmt.Sprintf("All %d ranked candidates classify as CROSS/SELECT — the favourable alleles already segregate; editing adds no time advantage.", out.TotalCandidates)
	case out.WaitCount == out.TotalCandidates:
		out.Headline = fmt.Sprintf("All %d ranked candidates classify as WAIT — effects are marginal, alleles are near fixation, or the population is too narrow to safely edit.", out.TotalCandidates)
	default:
		out.Headline = fmt.Sprintf("Edit decision mix: %d EDIT / %d CROSS / %d WAIT (out of %d candidates).", out.EditCount, out.CrossCount, out.WaitCount, out.TotalCandidates)
	}
	return out
}
