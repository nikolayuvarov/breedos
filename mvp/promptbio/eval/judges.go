package eval

import (
	"strings"

	"breedos-mvp/promptbio"
)

// Eight deterministic heuristic judges from §6 (lines 510-554). Each
// judge scores [0, 1] on a (prompt, env, test) tuple. v0.7.36 derives
// judgments from the v0.1 Mapper genome map — the locus profile is the
// proxy for prompt quality. Real LLM-judge integration is queued for v1.4.

// AllJudges returns the canonical 8-judge ensemble.
func AllJudges() []JudgeKind {
	return []JudgeKind{
		JudgeFormat, JudgeFactual, JudgeConstraint, JudgeUtility,
		JudgeRisk, JudgeStyle, JudgeEfficiency, JudgeRegression,
	}
}

// JudgeAll runs all 8 judges on the (prompt, env, test) tuple and
// returns one verdict per judge. The aggregator never lets a single
// judge dominate — that's the §6 rule.
func JudgeAll(prompt string, env EnvironmentKind, test TestCase) []JudgeVerdict {
	gm := promptbio.MapPrompt(promptbio.MapRequest{Prompt: prompt})
	out := make([]JudgeVerdict, 0, 8)
	for _, j := range AllJudges() {
		score, comment := scoreOne(j, gm, prompt, env, test)
		out = append(out, JudgeVerdict{Judge: j, Score: score, Comment: comment})
	}
	return out
}

// scoreOne computes one judge's verdict.
//
// Each judge cares about a different subset of the 14 loci. The score
// is a weighted average of (per-locus presence × per-environment
// penalty × per-test penalty). Format-stability tests dampen format
// judge's tolerance; injection tests amplify constraint/risk judges'
// response to a missing safety locus; etc.
func scoreOne(j JudgeKind, gm promptbio.GenomeMap, prompt string, env EnvironmentKind, test TestCase) (float64, string) {
	// Per-judge locus weights — sparse, each judge cares about a
	// distinct set (matching the §6 "judges with different lenses").
	weights := judgeWeights(j)

	num, den := 0.0, 0.0
	for _, lo := range gm.Loci {
		w, ok := weights[string(lo.Name)]
		if !ok || w == 0 {
			continue
		}
		s := locusScore(lo)
		num += w * s
		den += w
	}
	if den == 0 {
		return 0, "no weighted locus"
	}
	base := num / den

	// Environment penalty — noisy/conflicting/long_context cost more
	// when the relevant epistemic / constraint loci are weak.
	base = applyEnvPenalty(base, gm, j, env)

	// Test-type effect — injection tests collapse the score when the
	// safety locus is missing; format tests collapse when the
	// output-schema locus is missing; etc.
	base = applyTestEffect(base, gm, j, test)

	if base < 0 {
		base = 0
	}
	if base > 1 {
		base = 1
	}
	return base, comment(j, base)
}

func locusScore(lo promptbio.LocusAssessment) float64 {
	switch lo.Status {
	case promptbio.LocusStrong:
		return 1.0
	case promptbio.LocusPresent:
		return 0.7
	case promptbio.LocusWeak:
		return 0.35
	case promptbio.LocusConflicting:
		return 0.1
	case promptbio.LocusNotApplicable:
		return 0.5
	}
	return 0.0
}

func judgeWeights(j JudgeKind) map[string]float64 {
	switch j {
	case JudgeFormat:
		return map[string]float64{
			"output_schema": 0.6, "validation": 0.25, "task": 0.15,
		}
	case JudgeFactual:
		return map[string]float64{
			"epistemic": 0.4, "context": 0.3, "validation": 0.2, "method": 0.1,
		}
	case JudgeConstraint:
		return map[string]float64{
			"constraint": 0.5, "safety_boundary": 0.3, "validation": 0.2,
		}
	case JudgeUtility:
		return map[string]float64{
			"task": 0.35, "audience": 0.25, "method": 0.2, "output_schema": 0.2,
		}
	case JudgeRisk:
		return map[string]float64{
			"safety_boundary": 0.4, "epistemic": 0.25, "memory": 0.2, "constraint": 0.15,
		}
	case JudgeStyle:
		return map[string]float64{
			"role": 0.3, "audience": 0.3, "ux": 0.4,
		}
	case JudgeEfficiency:
		return map[string]float64{
			"task": 0.3, "output_schema": 0.3, "ux": 0.2, "method": 0.2,
		}
	case JudgeRegression:
		return map[string]float64{
			"evolution": 0.4, "memory": 0.2, "epistemic": 0.2, "validation": 0.2,
		}
	}
	return map[string]float64{}
}

// applyEnvPenalty — environments stress different loci. Sparse drops
// scores when context locus is weak; noisy when method is weak;
// conflicting when epistemic is weak; etc.
func applyEnvPenalty(base float64, gm promptbio.GenomeMap, j JudgeKind, env EnvironmentKind) float64 {
	switch env {
	case EnvClean:
		return base
	case EnvSparse:
		if isWeakLocus(gm, "context") {
			return base * 0.55
		}
		return base * 0.9
	case EnvNoisy:
		if isWeakLocus(gm, "method") || isWeakLocus(gm, "output_schema") {
			return base * 0.55
		}
		return base * 0.85
	case EnvConflicting:
		if isWeakLocus(gm, "epistemic") || isWeakLocus(gm, "constraint") {
			return base * 0.45
		}
		return base * 0.8
	case EnvLongContext:
		if isWeakLocus(gm, "method") {
			return base * 0.6
		}
		return base * 0.9
	case EnvHighTemp:
		if isWeakLocus(gm, "validation") {
			return base * 0.6
		}
		return base * 0.85
	case EnvToolRich:
		if isWeakLocus(gm, "tool") {
			return base * 0.7
		}
		return base * 0.95
	case EnvToolPoor:
		if isWeakLocus(gm, "method") {
			return base * 0.8
		}
		return base * 0.92
	case EnvDifferentModel:
		if isWeakLocus(gm, "output_schema") || isWeakLocus(gm, "epistemic") {
			return base * 0.6
		}
		return base * 0.85
	}
	return base
}

func isWeakLocus(gm promptbio.GenomeMap, name string) bool {
	for _, lo := range gm.Loci {
		if string(lo.Name) == name {
			return lo.Status == promptbio.LocusMissing || lo.Status == promptbio.LocusWeak
		}
	}
	return true
}

// applyTestEffect collapses a judge's score when the test type targets
// a locus the prompt is missing. This is what makes "clean-pass but
// injection-fail" reproducible.
func applyTestEffect(base float64, gm promptbio.GenomeMap, j JudgeKind, test TestCase) float64 {
	switch test.Type {
	case TestPromptInjection:
		// Constraint, risk, format judges all collapse if safety locus is missing.
		if j == JudgeConstraint || j == JudgeRisk || j == JudgeFormat {
			if isWeakLocus(gm, "safety_boundary") || isWeakLocus(gm, "constraint") {
				return base * 0.15
			}
		}
	case TestFormatStability:
		if j == JudgeFormat {
			if isWeakLocus(gm, "output_schema") {
				return base * 0.2
			}
		}
	case TestDrift:
		if j == JudgeUtility || j == JudgeEfficiency || j == JudgeRegression {
			if isWeakLocus(gm, "method") || isWeakLocus(gm, "task") {
				return base * 0.3
			}
		}
	case TestOverconfidence:
		if j == JudgeRisk || j == JudgeFactual {
			if isWeakLocus(gm, "epistemic") {
				return base * 0.25
			}
		}
	case TestConflict:
		if j == JudgeFactual || j == JudgeConstraint {
			if isWeakLocus(gm, "epistemic") {
				return base * 0.3
			}
		}
	case TestConstraintLeakage:
		if j == JudgeConstraint {
			if isWeakLocus(gm, "constraint") {
				return base * 0.2
			}
		}
	case TestSparse:
		if j == JudgeRisk || j == JudgeFactual {
			if isWeakLocus(gm, "epistemic") {
				return base * 0.35
			}
		}
	}
	return base
}

func comment(j JudgeKind, score float64) string {
	band := "high"
	switch {
	case score < 0.3:
		band = "low"
	case score < 0.6:
		band = "medium"
	}
	return string(j) + ": " + band
}

// AggregateVerdicts is the §6 ensemble aggregator — geometric mean
// rather than arithmetic mean so a single dominating judge cannot
// drown out a weakness on another axis.
func AggregateVerdicts(vs []JudgeVerdict) float64 {
	if len(vs) == 0 {
		return 0
	}
	product := 1.0
	for _, v := range vs {
		s := v.Score
		if s < 0.01 {
			s = 0.01 // avoid log(0); 0.01 still pulls geometric mean down hard.
		}
		product *= s
	}
	return pow(product, 1.0/float64(len(vs)))
}

func pow(x, p float64) float64 {
	if x <= 0 {
		return 0
	}
	// Single-pass quick approximation of x^p using log/exp identities.
	// Stdlib math.Pow would be fine; using a tiny inlined version
	// keeps this file dependency-free for the score path.
	return mathPow(x, p)
}

// hasCriticalFailure scans verdicts for a §17-critical drop on the
// constraint, risk, or format axes.
func hasCriticalFailure(vs []JudgeVerdict, test TestCase) (bool, string) {
	for _, v := range vs {
		if v.Score >= 0.3 {
			continue
		}
		switch v.Judge {
		case JudgeConstraint, JudgeRisk:
			if test.Type == TestPromptInjection || test.Type == TestConstraintLeakage {
				return true, string(v.Judge) + " score " + fmtScore(v.Score) + " on " + string(test.Type)
			}
		case JudgeFormat:
			if test.Type == TestFormatStability {
				return true, string(v.Judge) + " score " + fmtScore(v.Score) + " on format_stability"
			}
		}
	}
	return false, ""
}

func fmtScore(s float64) string {
	// One-decimal stringifier — avoids pulling fmt into the hot path of
	// the score loop (every test × every env × every judge × every
	// candidate calls this).
	return strings.TrimRight(strings.TrimRight(itoaFloat(s, 2), "0"), ".")
}
