package promptbio

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

func sha256s(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// Issue 30 v2.7 — Truth Maintenance System.
//
// Belief update protocol (§14, source lines 803-818):
//   classify → identify source → check conflict → on user correction
//   deprecate+update+propagate → weak evidence → assumption →
//   high-confidence → fact → unresolved → contradiction → memory write.
//
// Contradiction protocol (§13, source lines 774-782): the seven-step
// pipeline. Critical contradictions emit a clarification_request rather
// than blending.

// ApplyUpdate is the entry point for /api/promptbio/epistemology/update.
// Implements the §14 eight-step protocol over the prior belief state.
func ApplyUpdate(req UpdateRequest) UpdateResponse {
	bs := cloneBeliefState(req.PriorBeliefState)
	if bs.Claims == nil {
		bs.Claims = make(map[string]*Claim)
	}

	newClaim := req.NewClaim
	if newClaim.ID == "" {
		newClaim.ID = "c_" + shortHash(newClaim.Content)
	}
	if newClaim.State == "" {
		newClaim.State = stateForType(newClaim.Type, newClaim.Confidence)
	}

	deprecated := []string{}
	contradictions := []Contradiction{}

	// Step 3 + 4: check conflicts; on user correction deprecate prior.
	conflicting := detectConflicts(&newClaim, bs)
	for _, prior := range conflicting {
		winner := ResolveAuthority(&newClaim, prior)
		ctype := classifyContradiction(prior, &newClaim)
		sev := severityFor(ctype, prior, &newClaim)

		if winner == &newClaim {
			prior.Type = ClaimDeprecated
			prior.State = StateStale
			bs.Deprecated = append(bs.Deprecated, prior.ID)
			deprecated = append(deprecated, prior.ID)
			contradictions = append(contradictions, Contradiction{
				ID:         "x_" + shortHash(prior.ID+newClaim.ID),
				Type:       ctype,
				Severity:   sev,
				ClaimAID:   prior.ID,
				ClaimBID:   newClaim.ID,
				Winner:     newClaim.ID,
				Resolution: "deprecate_prior",
			})
		} else if sev == "critical" {
			contradictions = append(contradictions, Contradiction{
				ID:            "x_" + shortHash(prior.ID+newClaim.ID),
				Type:          ctype,
				Severity:      sev,
				ClaimAID:      prior.ID,
				ClaimBID:      newClaim.ID,
				Resolution:    "clarification_request",
				Clarification: fmt.Sprintf("Two claims conflict and neither dominates: '%s' vs '%s'. Which is current?", trimSnippet(prior.Content, 80), trimSnippet(newClaim.Content, 80)),
			})
			bs.Contradictions = append(bs.Contradictions, prior.ID, newClaim.ID)
		} else {
			contradictions = append(contradictions, Contradiction{
				ID:         "x_" + shortHash(prior.ID+newClaim.ID),
				Type:       ctype,
				Severity:   sev,
				ClaimAID:   prior.ID,
				ClaimBID:   newClaim.ID,
				Winner:     prior.ID,
				Resolution: "keep_prior_until_new_evidence",
			})
		}
		prior.ConflictsWith = appendUnique(prior.ConflictsWith, newClaim.ID)
		newClaim.ConflictsWith = appendUnique(newClaim.ConflictsWith, prior.ID)
	}

	// Step 7: confidence invariant — never promote without new evidence.
	if priorSameContent := findSameContent(&newClaim, bs); priorSameContent != nil {
		if confRank(newClaim.Confidence.Source) > confRank(priorSameContent.Confidence.Source) {
			if len(newClaim.Evidence) == 0 {
				// Reject the promotion: copy the prior's confidence rather
				// than allowing inflation per anti-pattern §18.2.
				newClaim.Confidence = priorSameContent.Confidence
			}
		}
	}

	// Bin into the right slot.
	bs.Claims[newClaim.ID] = &newClaim
	binClaim(&bs, &newClaim)

	// Step 4 propagation: mark recommendations referencing deprecated claims.
	toRevise := propagate(req.Recommendations, deprecated, bs)

	dedup(&bs)
	return UpdateResponse{
		NewBeliefState:          bs,
		DeprecatedClaims:        deprecated,
		RecommendationsToRevise: toRevise,
		Contradictions:          contradictions,
	}
}

func cloneBeliefState(in BeliefState) BeliefState {
	out := BeliefState{
		Objective:      in.Objective,
		KnownFacts:     append([]string{}, in.KnownFacts...),
		Constraints:    append([]string{}, in.Constraints...),
		Assumptions:    append([]string{}, in.Assumptions...),
		Hypotheses:     append([]string{}, in.Hypotheses...),
		Unknowns:       append([]string{}, in.Unknowns...),
		Contradictions: append([]string{}, in.Contradictions...),
		Deprecated:     append([]string{}, in.Deprecated...),
		Claims:         make(map[string]*Claim, len(in.Claims)),
	}
	for k, v := range in.Claims {
		c := *v
		c.Evidence = append([]string{}, v.Evidence...)
		c.ConflictsWith = append([]string{}, v.ConflictsWith...)
		c.UsedFor = append([]string{}, v.UsedFor...)
		out.Claims[k] = &c
	}
	return out
}

func detectConflicts(nc *Claim, bs BeliefState) []*Claim {
	hits := []*Claim{}
	for _, c := range bs.Claims {
		if c.ID == nc.ID {
			continue
		}
		if c.Type == ClaimDeprecated {
			continue
		}
		if claimsContradict(c, nc) {
			hits = append(hits, c)
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].ID < hits[j].ID })
	return hits
}

// claimsContradict is the v0.1 detector — two claims conflict when they
// share the same canonical subject but assert different values, OR when
// the new claim is explicitly marked as a correction.
func claimsContradict(a, b *Claim) bool {
	if a.Source == SrcCurrentUserCorrection || b.Source == SrcCurrentUserCorrection {
		return sameSubject(a.Content, b.Content) && !equalIgnoreCase(a.Content, b.Content)
	}
	if a.Type == ClaimDeprecated || b.Type == ClaimDeprecated {
		return false
	}
	if sameSubject(a.Content, b.Content) && !equalIgnoreCase(a.Content, b.Content) {
		return true
	}
	return false
}

// sameSubject extracts a crude "subject" token sequence — the first
// 4 lowercase words minus stop-words. Good enough for the test battery's
// "deadline is Friday" vs "deadline is Wednesday" pattern; richer
// subject extraction is a downstream concern.
var stopWords = map[string]struct{}{
	"the": {}, "a": {}, "an": {}, "is": {}, "are": {}, "was": {}, "were": {},
	"be": {}, "been": {}, "being": {}, "am": {}, "do": {}, "does": {}, "did": {},
	"i": {}, "we": {}, "our": {}, "your": {}, "my": {}, "of": {}, "to": {},
	"in": {}, "on": {}, "at": {}, "by": {}, "for": {}, "and": {}, "or": {},
	"this": {}, "that": {}, "it": {}, "its": {},
	// Filler / discourse markers — these come before the real subject when
	// the user issues a correction or hedge.
	"actually": {}, "really": {}, "honestly": {}, "basically": {}, "literally": {},
	"perhaps": {}, "maybe": {}, "probably": {},
	"assuming": {}, "supposing": {},
	"per": {}, "according": {},
	"so": {}, "well": {}, "okay": {}, "ok": {},
}

// subjectOf returns the first non-stop, non-marker content token —
// the *subject* of the sentence. The contradiction detector then asks
// whether two claims are about the same subject, not whether their full
// tail matches (the tail is exactly what changes between the claims).
func subjectOf(s string) string {
	for _, p := range strings.Fields(strings.ToLower(s)) {
		p = strings.Trim(p, ".,;:!?\"'()$%")
		if p == "" {
			continue
		}
		if _, skip := stopWords[p]; skip {
			continue
		}
		return p
	}
	return ""
}

func sameSubject(a, b string) bool {
	sa := subjectOf(a)
	sb := subjectOf(b)
	return sa != "" && sa == sb
}

func equalIgnoreCase(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func classifyContradiction(prior, neu *Claim) ContradictionType {
	switch {
	case neu.Source == SrcCurrentUserCorrection && (prior.Source == SrcConfirmedMemory || prior.Source == SrcOlderMemory):
		return ContradictionUserMemory
	case prior.Type == ClaimFact && neu.Type == ClaimFact:
		return ContradictionFactFact
	case (prior.Type == ClaimFact && neu.Type == ClaimUserClaim) || (prior.Type == ClaimUserClaim && neu.Type == ClaimFact):
		return ContradictionFactUser
	case prior.Type == ClaimUserClaim && neu.Type == ClaimUserClaim:
		return ContradictionUserUser
	case prior.Type == ClaimDocumentClaim && neu.Type == ClaimDocumentClaim:
		return ContradictionDocDoc
	case (prior.Type == ClaimToolResult && neu.Type == ClaimDocumentClaim) || (prior.Type == ClaimDocumentClaim && neu.Type == ClaimToolResult):
		return ContradictionToolDoc
	case (prior.Type == ClaimUserClaim && neu.Type == ClaimInference) || (prior.Type == ClaimInference && neu.Type == ClaimUserClaim):
		return ContradictionUserInference
	case prior.Type == ClaimAssumption || neu.Type == ClaimAssumption:
		return ContradictionAssumptionEvidence
	case prior.Type == ClaimRecommendation || neu.Type == ClaimRecommendation:
		return ContradictionRecommendationFact
	}
	return ContradictionFactFact
}

func severityFor(t ContradictionType, prior, neu *Claim) string {
	switch t {
	case ContradictionUserMemory, ContradictionFactFact, ContradictionFactUser:
		return "high"
	case ContradictionDocDoc, ContradictionToolDoc:
		return "high"
	case ContradictionRecommendationFact:
		return "high"
	}
	// Critical iff neither source dominates AND both claims are high-confidence.
	if tierIndex(prior.Source) == tierIndex(neu.Source) &&
		prior.Confidence.Source == ConfHigh && neu.Confidence.Source == ConfHigh {
		return "critical"
	}
	return "medium"
}

func findSameContent(nc *Claim, bs BeliefState) *Claim {
	for _, c := range bs.Claims {
		if c.ID == nc.ID {
			continue
		}
		if equalIgnoreCase(c.Content, nc.Content) {
			return c
		}
	}
	return nil
}

func binClaim(bs *BeliefState, c *Claim) {
	switch c.Type {
	case ClaimDeprecated:
		bs.Deprecated = appendUnique(bs.Deprecated, c.ID)
	case ClaimFact, ClaimUserClaim, ClaimDocumentClaim, ClaimToolResult, ClaimInference:
		bs.KnownFacts = appendUnique(bs.KnownFacts, c.ID)
	case ClaimConstraint, ClaimPreference:
		bs.Constraints = appendUnique(bs.Constraints, c.ID)
	case ClaimAssumption:
		bs.Assumptions = appendUnique(bs.Assumptions, c.ID)
	case ClaimHypothesis:
		bs.Hypotheses = appendUnique(bs.Hypotheses, c.ID)
	case ClaimUnknown:
		bs.Unknowns = appendUnique(bs.Unknowns, c.ID)
	}
}

func dedup(bs *BeliefState) {
	bs.KnownFacts = removeDeprecated(bs.KnownFacts, bs)
	bs.Constraints = removeDeprecated(bs.Constraints, bs)
	bs.Assumptions = removeDeprecated(bs.Assumptions, bs)
	bs.Hypotheses = removeDeprecated(bs.Hypotheses, bs)
	bs.Unknowns = removeDeprecated(bs.Unknowns, bs)
}

func removeDeprecated(ids []string, bs *BeliefState) []string {
	out := ids[:0]
	for _, id := range ids {
		c := bs.Claims[id]
		if c != nil && c.Type == ClaimDeprecated {
			continue
		}
		out = append(out, id)
	}
	return out
}

func appendUnique(xs []string, v string) []string {
	for _, x := range xs {
		if x == v {
			return xs
		}
	}
	return append(xs, v)
}

func propagate(recs []Recommendation, deprecatedIDs []string, bs BeliefState) []Recommendation {
	if len(deprecatedIDs) == 0 {
		return recs
	}
	dep := map[string]struct{}{}
	for _, id := range deprecatedIDs {
		dep[id] = struct{}{}
	}
	out := make([]Recommendation, len(recs))
	copy(out, recs)
	hits := []Recommendation{}
	for i := range out {
		for _, d := range out[i].DependsOn {
			if _, ok := dep[d]; ok {
				out[i].NeedsRevision = true
				hits = append(hits, out[i])
				break
			}
		}
	}
	return hits
}

// trimSnippet bounds a claim content for the clarification_request text.
func trimSnippet(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func shortHash(s string) string {
	h := sha256s(s)
	if len(h) > 12 {
		return h[:12]
	}
	return h
}
