package promptbio

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Issue 30 v2.7 — Runtime gate, anti-pattern detectors, output block
// renderer. Pre-output check from §19 (source lines 1080-1098); the
// anti-pattern catalogue is §18 (source lines 945-1072).

// RunGate executes the §19 ten-question gate over a candidate output +
// belief state. Returns Pass=false plus failing checks (and the rendered
// Epistemic status block when risk_level demands it).
func RunGate(req GateRequest) GateResult {
	res := GateResult{Pass: true}

	hits := detectAntiPatterns(req)
	res.AntiPatterns = hits

	for _, q := range runtimeGateQuestions {
		check := evaluateGateQuestion(q, req, hits)
		res.Checks = append(res.Checks, check)
		if !check.Pass {
			res.Pass = false
			if res.Reason == "" {
				res.Reason = check.Question + ": " + check.Detail
			}
		}
	}

	// On high-impact outputs the gate MANDATES the Epistemic status block.
	if req.RiskLevel == RiskHigh || req.RiskLevel == RiskMedium {
		if !hasEpistemicStatusBlock(req.CandidateOutput) {
			res.Pass = false
			res.Reason = "high-risk output missing required `Epistemic status` block"
			res.Checks = append(res.Checks, GateCheck{
				Question: "epistemic_status_block_required",
				Pass:     false,
				Detail:   "risk_level=" + string(req.RiskLevel) + " requires the verbatim Epistemic status block",
			})
		}
		res.OutputBlock = RenderEpistemicStatusBlock(req.BeliefState, req.Recommendations)
	}

	return res
}

func evaluateGateQuestion(q string, req GateRequest, hits []AntiPatternHit) GateCheck {
	out := strings.ToLower(req.CandidateOutput)

	switch {
	case strings.Contains(strings.ToLower(q), "traceable to at least one claim"):
		if len(req.BeliefState.Claims) == 0 && req.CandidateOutput != "" {
			return GateCheck{Question: q, Pass: false, Detail: "output present but belief_state has no claims"}
		}
	case strings.Contains(strings.ToLower(q), "assumptions been tagged"):
		if !containsAssumptionTag(req.CandidateOutput, req.BeliefState) {
			if hasUntaggedAssumption(req.BeliefState) && !mentionsWorkingAssumption(out) {
				return GateCheck{Question: q, Pass: false, Detail: "belief_state has untagged assumptions but output lacks `working_assumption` tag"}
			}
		}
	case strings.Contains(strings.ToLower(q), "user_correction"):
		if hasUnpropagatedCorrection(req.BeliefState, req.Recommendations) {
			return GateCheck{Question: q, Pass: false, Detail: "user_correction present but dependent recommendation lacks needs_revision"}
		}
	case strings.Contains(strings.ToLower(q), "numeric probabilities"):
		for _, h := range hits {
			if h.Section == "§18.6" {
				return GateCheck{Question: q, Pass: false, Detail: h.Detail}
			}
		}
	case strings.Contains(strings.ToLower(q), "contradictions surfaced"):
		if len(req.BeliefState.Contradictions) > 0 && !mentionsContradiction(out) {
			return GateCheck{Question: q, Pass: false, Detail: "belief_state has contradictions but output does not surface them"}
		}
	case strings.Contains(strings.ToLower(q), "respect every constraint"):
		// Static-mode check: declared constraints + output that contradicts a constraint claim.
		if violatesConstraint(req.CandidateOutput, req.BeliefState) {
			return GateCheck{Question: q, Pass: false, Detail: "output appears to violate a constraint claim"}
		}
	case strings.Contains(strings.ToLower(q), "stale claims"):
		if hasStale(req.BeliefState) && !mentionsStale(out) {
			return GateCheck{Question: q, Pass: false, Detail: "belief_state has stale claims; output does not flag freshness"}
		}
	case strings.Contains(strings.ToLower(q), "tool_result claims labelled"):
		if hasToolClaim(req.BeliefState) && !mentionsToolEvidence(out) {
			return GateCheck{Question: q, Pass: false, Detail: "tool_result present; output does not label as evidence"}
		}
	case strings.Contains(strings.ToLower(q), "dependency tracking"):
		for _, r := range req.Recommendations {
			if len(r.DependsOn) == 0 {
				return GateCheck{Question: q, Pass: false, Detail: "recommendation " + r.ID + " has empty depends_on"}
			}
			for _, d := range r.DependsOn {
				if c, ok := req.BeliefState.Claims[d]; ok && c.Type == ClaimDeprecated && !r.NeedsRevision {
					return GateCheck{Question: q, Pass: false, Detail: "recommendation " + r.ID + " depends on deprecated " + d + " but not flagged needs_revision"}
				}
			}
		}
	}
	return GateCheck{Question: q, Pass: true}
}

func containsAssumptionTag(s string, bs BeliefState) bool {
	low := strings.ToLower(s)
	return strings.Contains(low, "working_assumption") || strings.Contains(low, "assumption:")
}
func hasUntaggedAssumption(bs BeliefState) bool {
	return len(bs.Assumptions) > 0
}
func mentionsWorkingAssumption(s string) bool {
	return strings.Contains(s, "working_assumption") || strings.Contains(s, "assumption:")
}
func mentionsContradiction(s string) bool {
	return strings.Contains(s, "contradiction") || strings.Contains(s, "conflict")
}
func mentionsStale(s string) bool {
	return strings.Contains(s, "stale") || strings.Contains(s, "freshness")
}
func mentionsToolEvidence(s string) bool {
	return strings.Contains(s, "evidence") || strings.Contains(s, "tool_result")
}
func hasStale(bs BeliefState) bool {
	for _, c := range bs.Claims {
		if c.State == StateStale {
			return true
		}
	}
	return false
}
func hasToolClaim(bs BeliefState) bool {
	for _, c := range bs.Claims {
		if c.Type == ClaimToolResult {
			return true
		}
	}
	return false
}

func hasUnpropagatedCorrection(bs BeliefState, recs []Recommendation) bool {
	hasCorrection := false
	for _, c := range bs.Claims {
		if c.Source == SrcCurrentUserCorrection {
			hasCorrection = true
			break
		}
	}
	if !hasCorrection || len(bs.Deprecated) == 0 {
		return false
	}
	deprecatedSet := map[string]struct{}{}
	for _, d := range bs.Deprecated {
		deprecatedSet[d] = struct{}{}
	}
	for _, r := range recs {
		for _, d := range r.DependsOn {
			if _, dep := deprecatedSet[d]; dep && !r.NeedsRevision {
				return true
			}
		}
	}
	return false
}

func violatesConstraint(output string, bs BeliefState) bool {
	low := strings.ToLower(output)
	for _, id := range bs.Constraints {
		c, ok := bs.Claims[id]
		if !ok {
			continue
		}
		txt := strings.ToLower(c.Content)
		if strings.HasPrefix(txt, "must not ") || strings.HasPrefix(txt, "no ") {
			banned := strings.TrimPrefix(strings.TrimPrefix(txt, "must not "), "no ")
			banned = strings.TrimSpace(banned)
			if banned != "" && strings.Contains(low, banned) {
				return true
			}
		}
	}
	return false
}

func hasEpistemicStatusBlock(s string) bool {
	low := strings.ToLower(s)
	return strings.Contains(low, "epistemic status")
}

// detectAntiPatterns runs the §18 nine static checks. Each hit fails the
// gate and is included in GateResult.AntiPatterns.
func detectAntiPatterns(req GateRequest) []AntiPatternHit {
	hits := []AntiPatternHit{}
	out := req.CandidateOutput
	low := strings.ToLower(out)

	// §18.1 assumption_laundering — assumption in belief_state appears in
	// output without `working_assumption` tag.
	if len(req.BeliefState.Assumptions) > 0 && !mentionsWorkingAssumption(low) && len(out) > 0 {
		for _, aid := range req.BeliefState.Assumptions {
			c, ok := req.BeliefState.Claims[aid]
			if !ok {
				continue
			}
			subject := subjectOf(c.Content)
			if subject != "" && strings.Contains(low, subject) {
				hits = append(hits, AntiPatternHit{
					Name:    "assumption_laundering",
					Section: "§18.1",
					Detail:  "assumption '" + trimSnippet(c.Content, 60) + "' surfaces in output without `working_assumption` tag",
					ClaimID: c.ID,
				})
				break
			}
		}
	}

	// §18.6 fake_precision — numeric probabilities not backed by
	// non-model-inference source with high/medium confidence.
	if numericProbRe.MatchString(out) {
		backed := false
		for _, c := range req.BeliefState.Claims {
			if c.Source != SrcModelInference && (c.Confidence.Source == ConfHigh || c.Confidence.Source == ConfMedium) {
				if numericProbRe.MatchString(c.Content) {
					backed = true
					break
				}
			}
		}
		if !backed {
			hits = append(hits, AntiPatternHit{
				Name:    "fake_precision",
				Section: "§18.6",
				Detail:  "numeric probability in output without supporting non-model-inference source at confidence ≥ medium",
			})
		}
	}

	// §18.2 confidence_inflation — caught at update-time in tms.go, but
	// also detectable when output uses high-confidence language for an
	// assumption-typed claim.
	if hasHighConfidenceLanguage(low) {
		for _, aid := range req.BeliefState.Assumptions {
			c, ok := req.BeliefState.Claims[aid]
			if !ok {
				continue
			}
			subject := subjectOf(c.Content)
			if subject != "" && strings.Contains(low, subject) {
				hits = append(hits, AntiPatternHit{
					Name:    "confidence_inflation",
					Section: "§18.2",
					Detail:  "assumption rendered with high-confidence language",
					ClaimID: c.ID,
				})
				break
			}
		}
	}

	// §18.3 source_flattening — multiple source tiers present, output omits provenance.
	if sourceTierCount(req.BeliefState) > 1 && !strings.Contains(low, "source") && !strings.Contains(low, "per ") {
		hits = append(hits, AntiPatternHit{
			Name:    "source_flattening",
			Section: "§18.3",
			Detail:  "multiple source tiers in belief_state; output does not record provenance",
		})
	}

	// §18.4 memory_fossilization — older_memory claim present without
	// freshness label downstream.
	for _, c := range req.BeliefState.Claims {
		if c.Source == SrcOlderMemory && c.Freshness == "" {
			hits = append(hits, AntiPatternHit{
				Name:    "memory_fossilization",
				Section: "§18.4",
				Detail:  "older_memory claim lacks freshness label",
				ClaimID: c.ID,
			})
			break
		}
	}

	// §18.5 contradiction_blending — contradictions exist; output doesn't surface them.
	if len(req.BeliefState.Contradictions) > 0 && !mentionsContradiction(low) {
		hits = append(hits, AntiPatternHit{
			Name:    "contradiction_blending",
			Section: "§18.5",
			Detail:  "belief_state has contradictions; output appears to blend",
		})
	}

	// §18.7 citation_laundering — quoted-looking phrase without source provenance.
	if quotedSnippetRe.MatchString(out) && !strings.Contains(low, "source") && !strings.Contains(low, "per ") {
		hits = append(hits, AntiPatternHit{
			Name:    "citation_laundering",
			Section: "§18.7",
			Detail:  "quoted snippet present without source attribution",
		})
	}

	// §18.8 tool_overtrust — tool_result present, output uses authoritative language without "tool" or "evidence" marker.
	if hasToolClaim(req.BeliefState) && hasAuthoritativeLanguage(low) && !mentionsToolEvidence(low) {
		hits = append(hits, AntiPatternHit{
			Name:    "tool_overtrust",
			Section: "§18.8",
			Detail:  "tool_result claim rendered as authoritative truth",
		})
	}

	// §18.9 self_reference_as_evidence — model_inference claim cited as evidence by another claim.
	for _, c := range req.BeliefState.Claims {
		if c.Type == ClaimRecommendation {
			for _, eid := range c.Evidence {
				if ec, ok := req.BeliefState.Claims[eid]; ok && ec.Source == SrcModelInference {
					hits = append(hits, AntiPatternHit{
						Name:    "model_self_reference_as_evidence",
						Section: "§18.9",
						Detail:  "recommendation " + c.ID + " cites model_inference claim " + ec.ID + " as evidence",
						ClaimID: c.ID,
					})
					break
				}
			}
		}
	}

	sort.Slice(hits, func(i, j int) bool { return hits[i].Section < hits[j].Section })
	return hits
}

var (
	numericProbRe   = regexp.MustCompile(`\b\d{1,3}(?:\.\d+)?\s*%|\b0?\.\d+\s+(probability|likelihood)`)
	quotedSnippetRe = regexp.MustCompile(`"[^"]{8,}"`)
)

func hasHighConfidenceLanguage(s string) bool {
	for _, k := range []string{"definitely", "certainly", "without doubt", "guaranteed", "always", "never fails"} {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

func hasAuthoritativeLanguage(s string) bool {
	for _, k := range []string{"the answer is", "the result is", "the truth is"} {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

func sourceTierCount(bs BeliefState) int {
	seen := map[SourceTier]struct{}{}
	for _, c := range bs.Claims {
		seen[c.Source] = struct{}{}
	}
	return len(seen)
}

// RenderEpistemicStatusBlock builds the §20 verbatim block for a given belief_state.
func RenderEpistemicStatusBlock(bs BeliefState, recs []Recommendation) string {
	var sb strings.Builder
	sb.WriteString("Epistemic status:\n")
	sb.WriteString("- Known facts: " + describeClaims(bs, bs.KnownFacts) + "\n")
	sb.WriteString("- Working assumptions: " + describeClaims(bs, bs.Assumptions) + "\n")
	sb.WriteString("- Unverified claims: " + describeUnverified(bs) + "\n")
	sb.WriteString("- Key unknowns: " + describeClaims(bs, bs.Unknowns) + "\n")
	sb.WriteString("- Contradictions: " + describeContradictions(bs) + "\n")

	overall, strongest, weakest := overallConfidence(bs)
	sb.WriteString(fmt.Sprintf("- Confidence (overall/strongest/weakest): %s / %s / %s\n", overall, strongest, weakest))
	sb.WriteString("- What would change the recommendation: " + describeWhatWouldChange(bs, recs) + "\n")
	return sb.String()
}

func describeClaims(bs BeliefState, ids []string) string {
	if len(ids) == 0 {
		return "(none)"
	}
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		if c, ok := bs.Claims[id]; ok {
			parts = append(parts, trimSnippet(c.Content, 60))
		}
	}
	if len(parts) == 0 {
		return "(none)"
	}
	return strings.Join(parts, "; ")
}

func describeUnverified(bs BeliefState) string {
	parts := []string{}
	for _, c := range bs.Claims {
		if c.RequiresConfirmation && c.Type != ClaimDeprecated {
			parts = append(parts, trimSnippet(c.Content, 60))
		}
	}
	if len(parts) == 0 {
		return "(none)"
	}
	sort.Strings(parts)
	return strings.Join(parts, "; ")
}

func describeContradictions(bs BeliefState) string {
	if len(bs.Contradictions) == 0 {
		return "(none)"
	}
	parts := []string{}
	for _, id := range bs.Contradictions {
		if c, ok := bs.Claims[id]; ok {
			parts = append(parts, trimSnippet(c.Content, 60))
		}
	}
	return strings.Join(parts, " ⇄ ")
}

func overallConfidence(bs BeliefState) (overall, strongest, weakest string) {
	if len(bs.Claims) == 0 {
		return "unknown", "unknown", "unknown"
	}
	strongVal, weakVal := -1, 99
	overallSum, n := 0, 0
	for _, c := range bs.Claims {
		r := confRank(c.Confidence.Source)
		overallSum += r
		n++
		if r > strongVal {
			strongVal = r
			strongest = string(c.Confidence.Source)
		}
		if r < weakVal {
			weakVal = r
			weakest = string(c.Confidence.Source)
		}
	}
	avg := overallSum / max1(n)
	switch {
	case avg >= 4:
		overall = string(ConfHigh)
	case avg >= 3:
		overall = string(ConfMedium)
	case avg >= 2:
		overall = string(ConfLow)
	default:
		overall = string(ConfUnknown)
	}
	return
}

func describeWhatWouldChange(bs BeliefState, recs []Recommendation) string {
	parts := []string{}
	for _, c := range bs.Claims {
		if c.Type == ClaimAssumption {
			parts = append(parts, "if '"+trimSnippet(c.Content, 50)+"' is contradicted by evidence")
		}
	}
	for _, r := range recs {
		if len(r.DependsOn) > 0 {
			parts = append(parts, "if any claim in depends_on of "+r.ID+" is deprecated")
		}
	}
	if len(parts) == 0 {
		return "(no listed triggers; treat all evidence as potentially decisive)"
	}
	sort.Strings(parts)
	if len(parts) > 3 {
		parts = parts[:3]
	}
	return strings.Join(parts, "; ")
}

func max1(n int) int {
	if n == 0 {
		return 1
	}
	return n
}
