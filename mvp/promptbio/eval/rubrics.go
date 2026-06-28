package eval

// Rubric catalogue — §15 verbatim for GTM; §8 lines 633-665 for coding /
// research / teaching. Weights are sacred (the founder spec quotes them
// numerically and the acceptance criterion checks them verbatim).

// GTMFitnessFunction returns the §15 verbatim 9-metric GTM rubric.
// Weights: 0.18, 0.15, 0.15, 0.12, 0.10, 0.10, 0.08, 0.07, 0.05 (sum = 1.0).
func GTMFitnessFunction() FitnessFunction {
	return FitnessFunction{
		Family: FamilyGTM,
		Metrics: []RubricMetric{
			{
				Name:        "realism",
				Weight:      0.18,
				Description: "Is the strategy plausible given budget, team, market, and timeline?",
				Levels: map[int]string{
					1: "Disconnected from any plausible market or budget reality.",
					3: "Reads as plausible but skips one or two constraints.",
					5: "Calibrated to the supplied constraints; passes a market check.",
				},
			},
			{
				Name:        "specificity",
				Weight:      0.15,
				Description: "Concrete segments, channels, experiments, metrics named — not generic.",
				Levels: map[int]string{
					1: "Generic platitudes (\"do marketing\", \"reach customers\").",
					3: "Some concrete segments named; channels still generic.",
					5: "Named segments, channels, experiments, metrics — operator can act on it tomorrow.",
				},
			},
			{
				Name:        "actionability",
				Weight:      0.15,
				Description: "Whether the recommendation tells the team what to do this week.",
				Levels: map[int]string{
					1: "No actions; only restated context.",
					2: "Vague directions; no owners, no order.",
					3: "Named actions but no sequence or owner.",
					4: "Sequenced actions with implicit owners.",
					5: "Sequenced + owned + with success criteria.",
				},
			},
			{
				Name:        "constraint_fit",
				Weight:      0.12,
				Description: "Respects budget, team size, regulation, timeline as stated.",
				Levels: map[int]string{
					1: "Violates a hard constraint outright.",
					3: "Mentions constraints, then ignores one in the recommendation.",
					5: "Every recommended action is constraint-feasible.",
				},
			},
			{
				Name:        "experiment_quality",
				Weight:      0.10,
				Description: "Are proposed experiments falsifiable, low-cost, time-bounded?",
				Levels: map[int]string{
					1: "No experiments proposed.",
					3: "Experiments named but not falsifiable.",
					5: "Falsifiable, time-bounded, cheap experiments with kill criteria.",
				},
			},
			{
				Name:        "measurability",
				Weight:      0.10,
				Description: "Metrics tied to actions; baseline + target stated.",
				Levels: map[int]string{
					1: "No metrics.",
					3: "Metrics named; no baseline or target.",
					5: "Named metric + baseline + target + measurement cadence.",
				},
			},
			{
				Name:        "risk_awareness",
				Weight:      0.08,
				Description: "Surfaces the failure modes and dependency risks.",
				Levels: map[int]string{
					1: "No risks acknowledged.",
					3: "Generic risks listed (\"competition\", \"market\").",
					5: "Concrete failure modes tied to the recommendation; mitigation per risk.",
				},
			},
			{
				Name:        "value_clarity",
				Weight:      0.07,
				Description: "Clear who benefits, what changes for the customer.",
				Levels: map[int]string{
					1: "No value statement.",
					3: "Generic value proposition.",
					5: "Concrete customer outcome with delta from status quo.",
				},
			},
			{
				Name:        "uncertainty_handling",
				Weight:      0.05,
				Description: "Separates facts from assumptions; flags unknowns.",
				Levels: map[int]string{
					1: "Assumptions presented as facts.",
					3: "Facts and assumptions mentioned but not distinguished.",
					5: "Facts / assumptions / unknowns explicitly separated with confidence labels.",
				},
			},
		},
	}
}

// CodingFitnessFunction returns the §8 lines 633-643 coding rubric.
// Weights: correctness 0.35, executability 0.20, edge_cases 0.15,
// security 0.10, maintainability 0.10, clarity 0.05, efficiency 0.05.
func CodingFitnessFunction() FitnessFunction {
	return FitnessFunction{
		Family: FamilyCoding,
		Metrics: []RubricMetric{
			{Name: "correctness", Weight: 0.35, Description: "Solves the problem as specified.",
				Levels: map[int]string{1: "Wrong answer.", 3: "Partly correct.", 5: "Fully correct."}},
			{Name: "executability", Weight: 0.20, Description: "Runs as-is.",
				Levels: map[int]string{1: "Does not compile / run.", 3: "Runs with minor edits.", 5: "Runs as written."}},
			{Name: "edge_cases", Weight: 0.15, Description: "Handles boundary conditions.",
				Levels: map[int]string{1: "Ignores edges.", 3: "Some edges handled.", 5: "All declared edges handled."}},
			{Name: "security", Weight: 0.10, Description: "No injection, leak, or unsafe call.",
				Levels: map[int]string{1: "Unsafe.", 3: "Safe under happy path.", 5: "Safe under hostile input."}},
			{Name: "maintainability", Weight: 0.10, Description: "Readable, named, testable.",
				Levels: map[int]string{1: "Unreadable.", 3: "Readable.", 5: "Readable + testable + named."}},
			{Name: "clarity", Weight: 0.05, Description: "Names and structure convey intent.",
				Levels: map[int]string{1: "Cryptic.", 3: "OK.", 5: "Self-documenting."}},
			{Name: "efficiency", Weight: 0.05, Description: "Reasonable big-O for the problem class.",
				Levels: map[int]string{1: "Wasteful.", 3: "Acceptable.", 5: "Optimal for class."}},
		},
	}
}

// ResearchFitnessFunction returns the §8 lines 644-653 research rubric.
func ResearchFitnessFunction() FitnessFunction {
	return FitnessFunction{
		Family: FamilyResearch,
		Metrics: []RubricMetric{
			{Name: "claim_grounding", Weight: 0.25, Description: "Every claim cites or labels source."},
			{Name: "novelty", Weight: 0.15, Description: "Adds something not in the input."},
			{Name: "rigor", Weight: 0.15, Description: "Method named, assumptions stated."},
			{Name: "scope_clarity", Weight: 0.10, Description: "Inside vs outside scope marked."},
			{Name: "uncertainty_handling", Weight: 0.15, Description: "Facts / assumptions / unknowns separated."},
			{Name: "evidence_quality", Weight: 0.10, Description: "Sources are checkable."},
			{Name: "synthesis", Weight: 0.05, Description: "Pieces composed, not just listed."},
			{Name: "reproducibility", Weight: 0.05, Description: "Steps + data + tools stated."},
		},
	}
}

// TeachingFitnessFunction returns the §8 lines 656-665 teaching rubric.
func TeachingFitnessFunction() FitnessFunction {
	return FitnessFunction{
		Family: FamilyTeaching,
		Metrics: []RubricMetric{
			{Name: "audience_fit", Weight: 0.20, Description: "Right level for the named learner."},
			{Name: "scaffolding", Weight: 0.20, Description: "Builds from known to new."},
			{Name: "example_quality", Weight: 0.15, Description: "Concrete, illustrative examples."},
			{Name: "checks_for_understanding", Weight: 0.15, Description: "Questions/exercises that test."},
			{Name: "clarity", Weight: 0.10, Description: "Jargon defined; metaphor accurate."},
			{Name: "engagement", Weight: 0.10, Description: "Holds attention; respects time."},
			{Name: "next_steps", Weight: 0.05, Description: "What to learn next named."},
			{Name: "factual_accuracy", Weight: 0.05, Description: "No errors; correct sources."},
		},
	}
}

// ResolveFitness returns the rubric for the given task family. Unknown
// families fall back to GTM (the most thoroughly verbatim'd in the spec).
func ResolveFitness(family TaskFamily) FitnessFunction {
	switch family {
	case FamilyCoding:
		return CodingFitnessFunction()
	case FamilyResearch:
		return ResearchFitnessFunction()
	case FamilyTeaching:
		return TeachingFitnessFunction()
	}
	return GTMFitnessFunction()
}
