package promptbio

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"
)

// MapPrompt is the v0.1 entry point — a static heuristic analyzer
// from `ingest-done/handoff-dna-prompt.md.done` Section 5.1. No live
// LLM, no API call, no RNG; deterministic given the input.
//
// Each locus is detected by a small set of cue tokens (English +
// Russian for v0.1). Coverage is intentionally narrow: false
// negatives are preferred to false positives — the operator should
// be told a missing locus is missing, not that we think it is
// "present-ish".
func MapPrompt(req MapRequest) GenomeMap {
	prompt := strings.TrimSpace(req.Prompt)
	id := newPromptID(prompt)
	lang := req.Language
	if lang == "" || lang == "auto" {
		lang = detectLanguage(prompt)
	}

	out := GenomeMap{
		PromptID:    id,
		InputPrompt: req.Prompt,
		Language:    lang,
		SpeciesHint: req.SpeciesHint,
		Loci:        make([]LocusAssessment, 0, len(allLoci)),
	}

	// Empty prompt is a hard edge case — every locus is missing and
	// the failure-mode prediction is trivially "no answer at all".
	// Don't run the heuristics; short-circuit so the score is 0.
	if prompt == "" {
		for _, name := range allLoci {
			risk, mut := locusRiskAndMutation(name)
			out.Loci = append(out.Loci, LocusAssessment{
				Name:              name,
				Status:            LocusMissing,
				Score:             pf(0),
				RiskIfMissing:     risk,
				SuggestedMutation: mut,
			})
			out.MissingLoci = append(out.MissingLoci, name)
		}
		out.GenomeScore = 0
		out.ExpectedPhenotype = ExpectedPhenotype{
			LikelyOutput: "Empty prompt — no answer would be generated.",
			FailureModes: []string{"empty_input"},
			Confidence:   "high",
		}
		out.MutationPlan = mutationPlanFromMissing(out.MissingLoci, out.ConflictingLoci)
		out.TestsToRun = suggestedTests(out)
		return out
	}

	lower := strings.ToLower(prompt)

	for _, name := range allLoci {
		assess := assessLocus(name, prompt, lower)
		out.Loci = append(out.Loci, assess)
		switch assess.Status {
		case LocusMissing:
			out.MissingLoci = append(out.MissingLoci, name)
		case LocusConflicting:
			out.ConflictingLoci = append(out.ConflictingLoci, name)
		}
	}

	out.GenomeScore = computeGenomeScore(out.Loci)
	out.ExpectedPhenotype = inferExpectedPhenotype(out.Loci, lower)
	out.MutationPlan = mutationPlanFromMissing(out.MissingLoci, out.ConflictingLoci)
	out.TestsToRun = suggestedTests(out)
	return out
}

// assessLocus runs the v0.1 heuristic detector for one locus. The
// "lower" arg is the cached lowercased prompt — saves repeating the
// allocation per locus.
func assessLocus(name LocusName, prompt, lower string) LocusAssessment {
	risk, mut := locusRiskAndMutation(name)
	base := LocusAssessment{Name: name, RiskIfMissing: risk}
	status, evidence := detectLocus(name, prompt, lower)
	base.Status = status
	base.Evidence = evidence
	if s, ok := statusScore(status); ok {
		base.Score = pf(s)
	}
	// Only attach the mutation suggestion when the locus is weak,
	// missing, or conflicting. A strong/present locus doesn't need a
	// per-row mutation hint (the operator already covered it).
	switch status {
	case LocusMissing, LocusWeak, LocusConflicting:
		base.SuggestedMutation = mut
	}
	return base
}

// detectLocus is the heart of the v0.1 analyzer. Returns the status
// + a short evidence string (the cue token that matched, when one
// did) so the operator can see why the locus was flagged.
func detectLocus(name LocusName, prompt, lower string) (LocusStatus, string) {
	switch name {
	case LocusTask:
		// "Strong" needs an imperative verb in present-form at the
		// start of a clause AND a deliverable noun. Common cues —
		// English: write, build, create, plan, design, explain,
		// summarise, analyse, propose. Russian: напиши, сделай,
		// построй, спроектируй, объясни, проанализируй, предложи.
		if w := firstMatch(lower, []string{
			"write", "build", "create", "plan", "design", "explain",
			"summarise", "summarize", "analyse", "analyze", "propose",
			"draft", "produce", "outline", "output the", "deliver",
			"напиши", "сделай", "построй", "спроектируй", "объясни",
			"проанализируй", "предложи", "составь", "разработай",
			"выведи", "выдай",
		}); w != "" {
			// "Strong" requires an explicit deliverable noun nearby.
			if containsAny(lower, []string{
				"strategy", "plan", "doc", "doc.", "answer", "code",
				"script", "report", "summary", "tutorial",
				"стратегию", "стратегия", "план", "ответ", "код",
				"скрипт", "отчёт", "отчет", "резюме",
			}) {
				return LocusStrong, w
			}
			return LocusPresent, w
		}
		return LocusMissing, ""
	case LocusRole:
		// Role anchors. "Strong" if the role is named explicitly
		// with seniority/specialism.
		if w := firstMatch(lower, []string{
			"act as", "you are", "as a senior", "as an expert",
			"ты — ", "ты—", "ты - ", "ты –",
			"выступи в роли", "выступай как",
		}); w != "" {
			return LocusStrong, w
		}
		return LocusMissing, ""
	case LocusAudience:
		if w := firstMatch(lower, []string{
			"audience", "for a junior", "for a senior", "for an investor",
			"for a beginner", "for an expert", "for a non-technical",
			"for a customer",
			"для junior", "для senior", "для инвестора",
			"для новичка", "для эксперта", "для клиента",
			"для аудитории", "целевая аудитория",
		}); w != "" {
			return LocusPresent, w
		}
		return LocusMissing, ""
	case LocusContext:
		// Context cues = explicit pointer to data the model should use.
		if w := firstMatch(lower, []string{
			"given the following", "given the data", "given this",
			"based on the data", "context:", "background:",
			"input data", "the document below", "user data",
			"мои данные", "контекст:", "исходные данные",
			"учитывай", "учти", "ниже приведён", "ниже приведен",
		}); w != "" {
			return LocusPresent, w
		}
		return LocusMissing, ""
	case LocusConstraint:
		if w := firstMatch(lower, []string{
			"constraint", "constraints", "must not", "do not",
			"avoid", "without", "limited to", "budget",
			"нельзя", "ограничения", "без ", "не должен",
			"учитывай бюджет", "ограничен",
		}); w != "" {
			return LocusPresent, w
		}
		return LocusMissing, ""
	case LocusMethod:
		if w := firstMatch(lower, []string{
			"step 1", "step by step", "first, ", "then ", "finally, ",
			"method:", "approach:", "process:",
			"шаг 1", "шаг за шагом", "сначала", "затем", "наконец",
			"метод:", "подход:",
		}); w != "" {
			return LocusPresent, w
		}
		return LocusMissing, ""
	case LocusEpistemic:
		if w := firstMatch(lower, []string{
			"facts", "assumptions", "unknowns", "confidence",
			"уверенность", "источник", "sources", "uncertainty",
			"отдели facts", "отдели факт", "разделяй",
		}); w != "" {
			if containsAll(lower, []string{"facts", "assumptions"}) ||
				containsAll(lower, []string{"факт", "предполож"}) {
				return LocusStrong, w
			}
			return LocusPresent, w
		}
		return LocusMissing, ""
	case LocusOutput:
		// Output schema cues. "Strong" if a structured schema is
		// declared (JSON, numbered sections, named blocks).
		if w := firstMatch(lower, []string{
			"format:", "schema:", "json", "yaml", "markdown",
			"return a json", "as a table", "in the format",
			"summary, ", "summary,", "формат:", "схема:",
			"в формате", "в виде json", "в виде таблицы",
			"summary,", "следующих разделах", "разделы:",
		}); w != "" {
			if containsAny(lower, []string{
				"summary,", "summary, ", "summary, segment", "next actions",
				"sections:", "разделы:", "metrics, risks",
			}) {
				return LocusStrong, w
			}
			return LocusPresent, w
		}
		return LocusMissing, ""
	case LocusValidation:
		if w := firstMatch(lower, []string{
			"before final", "final check", "validate", "verify",
			"перед финал", "проверь", "проверьте", "валидируй",
			"check that", "double-check",
		}); w != "" {
			return LocusPresent, w
		}
		return LocusMissing, ""
	case LocusTool:
		// Tool locus is "not_applicable" by default — most prompts
		// don't need to discuss tools. Mark present only when the
		// prompt explicitly names tools.
		if w := firstMatch(lower, []string{
			"use the search tool", "use tools", "call the api",
			"use the web", "execute code", "run the tool",
			"используй поиск", "используй tool", "вызови инструмент",
		}); w != "" {
			return LocusPresent, w
		}
		return LocusNotApplicable, ""
	case LocusMemory:
		if w := firstMatch(lower, []string{
			"remember", "save to memory", "persist", "memory:",
			"project memory", "forget", "ignore previous",
			"запомни", "сохрани в памяти", "забудь", "игнорируй",
		}); w != "" {
			return LocusPresent, w
		}
		return LocusNotApplicable, ""
	case LocusSafety:
		if w := firstMatch(lower, []string{
			"not legal advice", "not medical advice", "not financial advice",
			"safety", "privacy", "no pii", "do not store",
			"не является юридическим", "не является медицинским",
			"конфиденциальность", "безопасность",
		}); w != "" {
			return LocusPresent, w
		}
		return LocusNotApplicable, ""
	case LocusUX:
		if w := firstMatch(lower, []string{
			"short", "concise", "summary first", "tldr", "tl;dr",
			"keep it short", "no more than",
			"коротко", "кратко", "не длиннее", "summary first",
		}); w != "" {
			return LocusPresent, w
		}
		return LocusMissing, ""
	case LocusEvolution:
		if w := firstMatch(lower, []string{
			"if it fails", "revise", "iterate", "feedback loop",
			"learn from", "after failure",
			"если не получится", "пересмотри", "уточни после",
			"улучшай", "доработай",
		}); w != "" {
			return LocusPresent, w
		}
		return LocusMissing, ""
	}
	return LocusMissing, ""
}

// computeGenomeScore implements the v0.1 score from Section 4.4:
// normalised average of applicable locus scores, mapped to [0, 1].
// not_applicable rows are skipped. The raw range is [-1, 3] per
// locus, normalised here against the max possible score (3 × number
// of applicable loci) so 1.0 means "every applicable locus strong",
// 0.0 means "every applicable locus missing", negative is mapped to
// 0 (clamped).
func computeGenomeScore(loci []LocusAssessment) float64 {
	applicable := 0
	total := 0.0
	for _, l := range loci {
		if l.Score == nil {
			continue
		}
		applicable++
		total += *l.Score
	}
	if applicable == 0 {
		return 0
	}
	max := float64(applicable * 3)
	if max == 0 {
		return 0
	}
	score := total / max
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}
	// Round to 3 decimals so JSON output is stable across runs.
	return roundTo(score, 3)
}

// inferExpectedPhenotype is a small lookup that walks the assessed
// loci and emits the failure modes from handoff Section 5.2 when the
// matching loci are missing/weak. Confidence stays "medium" — v0.1
// is a static analyzer, not a runtime predictor.
func inferExpectedPhenotype(loci []LocusAssessment, lower string) ExpectedPhenotype {
	failures := make([]string, 0)
	hasContext := false
	hasConstraint := false
	hasEpistemic := false
	hasOutput := false
	hasValidation := false
	hasUX := false
	taskStatus := LocusMissing
	for _, l := range loci {
		switch l.Name {
		case LocusContext:
			hasContext = l.Status != LocusMissing && l.Status != LocusNotApplicable
		case LocusConstraint:
			hasConstraint = l.Status != LocusMissing && l.Status != LocusNotApplicable
		case LocusEpistemic:
			hasEpistemic = l.Status != LocusMissing && l.Status != LocusNotApplicable
		case LocusOutput:
			hasOutput = l.Status != LocusMissing && l.Status != LocusNotApplicable
		case LocusValidation:
			hasValidation = l.Status != LocusMissing && l.Status != LocusNotApplicable
		case LocusUX:
			hasUX = l.Status != LocusMissing && l.Status != LocusNotApplicable
		case LocusTask:
			taskStatus = l.Status
		}
	}
	if !hasContext {
		failures = append(failures, "generic_answer")
	}
	if !hasEpistemic {
		failures = append(failures, "assumption_as_fact")
	}
	if !hasConstraint && taskStatus != LocusMissing {
		failures = append(failures, "constraint_leakage_if_constraints_exist_later")
	}
	if !hasOutput {
		failures = append(failures, "format_drift")
	}
	if !hasValidation {
		failures = append(failures, "unchecked_failure")
	}
	if !hasUX {
		failures = append(failures, "verbose_or_unfocused_answer")
	}
	// Strategy-style prompts have a characteristic failure: no concrete
	// metrics or next actions surface even when the task is "write a
	// strategy". Detect via cue token.
	if containsAny(lower, []string{"strategy", "стратеги", "plan", "план запуска"}) && !hasOutput {
		failures = append(failures, "no_next_action")
		failures = append(failures, "no_metrics")
	}
	if len(failures) == 0 {
		failures = append(failures, "none_obvious_at_v0.1_heuristic")
	}
	conf := "medium"
	if taskStatus == LocusMissing {
		conf = "low"
	}
	likely := "Static analyzer cannot predict the literal answer; failure-mode list above is the v0.1 estimate."
	return ExpectedPhenotype{
		LikelyOutput: likely,
		FailureModes: failures,
		Confidence:   conf,
	}
}

func mutationPlanFromMissing(missing, conflicting []LocusName) []MutationSuggestion {
	out := make([]MutationSuggestion, 0, len(missing)+len(conflicting))
	for _, name := range missing {
		_, mut := locusRiskAndMutation(name)
		out = append(out, MutationSuggestion{
			MutationType: "addition",
			TargetLocus:  string(name),
			Patch:        mut,
			Rationale:    fmt.Sprintf("Locus %q is missing — addition mutation introduces it.", string(name)),
		})
	}
	for _, name := range conflicting {
		out = append(out, MutationSuggestion{
			MutationType: "suppression",
			TargetLocus:  string(name),
			Patch:        "remove or rewrite the conflicting clause",
			Rationale:    fmt.Sprintf("Locus %q carries a contradictory instruction — suppression mutation resolves it.", string(name)),
		})
	}
	return out
}

// suggestedTests gives the operator 3–5 concrete checks that
// distinguish a baseline answer from the mutated-prompt answer.
// Pulled from the locus statuses; deterministic given input.
func suggestedTests(g GenomeMap) []string {
	tests := make([]string, 0, 6)
	hasOutput := false
	hasConstraint := false
	for _, l := range g.Loci {
		if l.Name == LocusOutput && (l.Status == LocusPresent || l.Status == LocusStrong) {
			hasOutput = true
		}
		if l.Name == LocusConstraint && (l.Status == LocusPresent || l.Status == LocusStrong) {
			hasConstraint = true
		}
	}
	if !hasOutput {
		tests = append(tests, "Run the prompt; check whether the answer follows a stated structure (sections / JSON / table). Baseline failure mode: format drift.")
	}
	if !hasConstraint {
		tests = append(tests, "Run the prompt with a constraint scenario (e.g. \"team of 2, 4-week deadline\"); check whether the answer respects the constraint without prompting.")
	}
	tests = append(tests, "Run the prompt twice with different temperatures; check whether the response shape stays stable (low reaction-norm variance).")
	tests = append(tests, "Apply the suggested mutation plan; re-score with MapPrompt; verify the genome score increases.")
	if len(g.MissingLoci) > 0 {
		tests = append(tests, fmt.Sprintf("Spot-check that the answer addresses each of: %s.", joinLoci(g.MissingLoci)))
	}
	return tests
}

// ---- helpers -------------------------------------------------------

func firstMatch(haystack string, needles []string) string {
	for _, n := range needles {
		if strings.Contains(haystack, n) {
			return n
		}
	}
	return ""
}

func containsAny(haystack string, needles []string) bool { return firstMatch(haystack, needles) != "" }
func containsAll(haystack string, needles []string) bool {
	for _, n := range needles {
		if !strings.Contains(haystack, n) {
			return false
		}
	}
	return true
}

func detectLanguage(prompt string) string {
	cyr := 0
	lat := 0
	for _, r := range prompt {
		switch {
		case unicode.Is(unicode.Cyrillic, r):
			cyr++
		case unicode.Is(unicode.Latin, r):
			lat++
		}
	}
	if cyr > lat {
		return "ru"
	}
	if lat > 0 {
		return "en"
	}
	return "unknown"
}

func newPromptID(prompt string) string {
	// Stable id derived from the prompt content. v0.1 doesn't need
	// uniqueness across runs of the same prompt — the id is for
	// client correlation, not storage.
	h := sha1.Sum([]byte(prompt))
	return "p_" + hex.EncodeToString(h[:6])
}

func pf(v float64) *float64 { return &v }

func roundTo(v float64, places int) float64 {
	shift := 1.0
	for i := 0; i < places; i++ {
		shift *= 10
	}
	if v >= 0 {
		return float64(int64(v*shift+0.5)) / shift
	}
	return float64(int64(v*shift-0.5)) / shift
}

func joinLoci(loci []LocusName) string {
	bits := make([]string, len(loci))
	for i, l := range loci {
		bits[i] = string(l)
	}
	return strings.Join(bits, ", ")
}
