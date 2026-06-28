// Package eval implements the v1.3 Prompt Evaluation Lab.
// See ingest-done/17-prompt-dna.md.done §§3-20 for the source spec.
//
// Core formula:
//
//	F_system(P, E, T) = Score(Y = Run(P, E, T))
//	F(P)             = E_{E,T}[Score(Y)]
//	Robustness(P)    = 1 − Var_{E,T}(Score(Y))
//	F_net            = F_quality − λ·Cost
//
// v0.7.36 ships the substrate end-to-end with a deterministic heuristic
// judging stack (operating on the v0.1 Mapper genome). Real LLM judges
// are queued for v1.4 Runtime; the Lab is the gate that decides whether
// a compiled prompt earns deployment.
package eval

// TestType is the §4 ten-element test taxonomy (lines 235-411).
type TestType string

const (
	TestClean             TestType = "clean"
	TestSparse            TestType = "sparse"
	TestNoisyContext      TestType = "noisy_context"
	TestConflict          TestType = "conflict"
	TestPromptInjection   TestType = "prompt_injection"
	TestConstraintLeakage TestType = "constraint_leakage"
	TestFormatStability   TestType = "format_stability"
	TestDrift             TestType = "drift"
	TestOverconfidence    TestType = "overconfidence"
	TestReproduction      TestType = "reproduction"
)

var allTestTypes = []TestType{
	TestClean, TestSparse, TestNoisyContext, TestConflict, TestPromptInjection,
	TestConstraintLeakage, TestFormatStability, TestDrift, TestOverconfidence, TestReproduction,
}

// EnvironmentKind is the §10 environment matrix (lines 708-718).
type EnvironmentKind string

const (
	EnvClean        EnvironmentKind = "clean"
	EnvSparse       EnvironmentKind = "sparse"
	EnvNoisy        EnvironmentKind = "noisy"
	EnvConflicting  EnvironmentKind = "conflicting"
	EnvLongContext  EnvironmentKind = "long_context"
	EnvHighTemp     EnvironmentKind = "high_temperature"
	EnvToolRich     EnvironmentKind = "tool_rich"
	EnvToolPoor     EnvironmentKind = "tool_poor"
	EnvDifferentModel EnvironmentKind = "different_model"
)

var allEnvironments = []EnvironmentKind{
	EnvClean, EnvSparse, EnvNoisy, EnvConflicting, EnvLongContext,
	EnvHighTemp, EnvToolRich, EnvToolPoor, EnvDifferentModel,
}

// TaskFamily picks the rubric weight vector (§8, lines 596-665).
type TaskFamily string

const (
	FamilyGTM      TaskFamily = "gtm_product_launch"
	FamilyCoding   TaskFamily = "coding_prompt"
	FamilyResearch TaskFamily = "research_prompt"
	FamilyTeaching TaskFamily = "teaching_prompt"
)

// RubricMetric is one row of the fitness function (§7, 558-593).
// Levels maps score (1, 3, 5) to verbatim descriptors.
type RubricMetric struct {
	Name        string             `json:"name"`
	Weight      float64            `json:"weight"`
	Description string             `json:"description"`
	Levels      map[int]string     `json:"levels"`
}

// FitnessFunction is the per-family rubric bundle (§15, 945-1007).
type FitnessFunction struct {
	Family  TaskFamily     `json:"family"`
	Metrics []RubricMetric `json:"metrics"`
}

// TestCase is one entry in the §4 test bank.
type TestCase struct {
	ID              string   `json:"id"`
	Type            TestType `json:"type"`
	Input           string   `json:"input"`
	Expected        []string `json:"expected"`
	FailureCondition string  `json:"failure_condition"`
	PassCriteria    string   `json:"pass_criteria"`
	MetricsAffected []string `json:"metrics_affected"`
}

// JudgeKind is one of the §6 eight specialised judges.
type JudgeKind string

const (
	JudgeFormat     JudgeKind = "format_judge"
	JudgeFactual    JudgeKind = "factual_judge"
	JudgeConstraint JudgeKind = "constraint_judge"
	JudgeUtility    JudgeKind = "utility_judge"
	JudgeRisk       JudgeKind = "risk_judge"
	JudgeStyle      JudgeKind = "style_judge"
	JudgeEfficiency JudgeKind = "efficiency_judge"
	JudgeRegression JudgeKind = "regression_judge"
)

// JudgeVerdict is one judge's score on a (prompt, test) pair.
type JudgeVerdict struct {
	Judge   JudgeKind `json:"judge"`
	Score   float64   `json:"score"`
	Comment string    `json:"comment,omitempty"`
}

// TestResult is the per-test verdict bundle.
type TestResult struct {
	TestID     string         `json:"test_id"`
	TestType   TestType       `json:"test_type"`
	Pass       bool           `json:"pass"`
	Score      float64        `json:"score"`
	Verdicts   []JudgeVerdict `json:"verdicts"`
	Failures   []string       `json:"failures,omitempty"`
}

// EnvResult bundles all per-environment results.
type EnvResult struct {
	Env         EnvironmentKind `json:"env"`
	TestResults []TestResult    `json:"test_results"`
	MeanScore   float64         `json:"mean_score"`
}

// ScoreMatrixRow is one row of the §9 score matrix (677-693).
type ScoreMatrixRow struct {
	Prompt          string             `json:"prompt"`
	PerEnvScore     map[string]float64 `json:"per_env_score"`
	AvgScore        float64            `json:"avg_score"`
	Robustness      float64            `json:"robustness"`
	CriticalFailure string             `json:"critical_failure,omitempty"`
}

// ReactionNormReport is per-trait stability across the env matrix (§10).
type ReactionNormReport struct {
	Prompt        string                          `json:"prompt"`
	PerTrait      map[string]TraitStability       `json:"per_trait"`
	StableTraits  []string                        `json:"stable_traits"`
	FloatTraits   []string                        `json:"float_traits"`
	BreakingEnvs  []EnvironmentKind               `json:"breaking_environments"`
	NicheFit      string                          `json:"niche_fit"`
}

// TraitStability summarises one trait's variance across environments.
type TraitStability struct {
	Trait      string                       `json:"trait"`
	Variance   float64                      `json:"variance"`
	MeanScore  float64                      `json:"mean_score"`
	WorstEnv   EnvironmentKind              `json:"worst_env"`
	WorstScore float64                      `json:"worst_score"`
	PerEnv     map[EnvironmentKind]float64  `json:"per_env"`
}

// RegressionReport is the §11 verbatim template output (779-803).
type RegressionReport struct {
	AncestorID  string   `json:"ancestor_id"`
	DescendantID string  `json:"descendant_id"`
	Improved    []string `json:"improved"`
	Degraded    []string `json:"degraded"`
	Unchanged   []string `json:"unchanged"`
	NewRisks    []string `json:"new_risks"`
	Decision    string   `json:"decision"`
}

// AblationRow is one row of the §13 ablation table.
type AblationRow struct {
	Module        string  `json:"module"`
	ExpectedLoss  string  `json:"expected_loss"`
	ActualLoss    float64 `json:"actual_loss"`
	Keep          bool    `json:"keep"`
	Detail        string  `json:"detail,omitempty"`
}

// AblationReport bundles the §13 runner output.
type AblationReport struct {
	Prompt string        `json:"prompt"`
	Rows   []AblationRow `json:"rows"`
}

// CostBreakdown is §14 cost telemetry (902-937).
type CostBreakdown struct {
	PromptLength       int     `json:"prompt_length"`
	OutputLength       int     `json:"output_length_estimate"`
	ToolCalls          int     `json:"tool_calls"`
	LatencyMS          int     `json:"latency_ms_estimate"`
	SlotFillingBurden  float64 `json:"slot_filling_burden"`
	UserBurden         float64 `json:"user_burden"`
	MaintenanceComplex float64 `json:"maintenance_complexity"`
	TestCount          int     `json:"test_count"`
	DependencyCount    int     `json:"dependency_count"`
	NormalisedCost     float64 `json:"normalised_cost"`
}

// Decision is the §17 deployment verdict (1110-1156).
type Decision string

const (
	DecisionAccept            Decision = "accept"
	DecisionAcceptAsSpecialist Decision = "accept_as_specialist"
	DecisionReject            Decision = "reject"
	DecisionSplitIntoProfiles Decision = "split_into_profiles"
	DecisionMutateAgain       Decision = "mutate_again"
)

// DeploymentDecision is the §17 record.
type DeploymentDecision struct {
	Verdict          Decision `json:"verdict"`
	Rationale        string   `json:"rationale"`
	CriticalFailures []string `json:"critical_failures,omitempty"`
	NicheScoping     string   `json:"niche_scoping,omitempty"`
	Profiles         []string `json:"profiles,omitempty"`
}

// NextMutation is one of the §18 three targeted mutation suggestions.
type NextMutation struct {
	TargetLocus string `json:"target_locus"`
	Kind        string `json:"kind"`
	Rationale   string `json:"rationale"`
}

// EvaluationLabReport is the §18 13-section bundle.
type EvaluationLabReport struct {
	LabID            string             `json:"lab_id"`
	TargetPrompt     string             `json:"target_prompt"`
	AncestorPrompt   string             `json:"ancestor_prompt,omitempty"`
	TaskFamily       TaskFamily         `json:"task_family"`
	EvaluationMode   string             `json:"evaluation_mode"`
	Fitness          FitnessFunction    `json:"fitness"`
	TestBank         []TestCase         `json:"test_bank"`
	EnvironmentMatrix []EnvironmentKind `json:"environment_matrix"`
	EnvResults       []EnvResult        `json:"env_results"`
	ScoreMatrix      []ScoreMatrixRow   `json:"score_matrix"`
	ReactionNorm     ReactionNormReport `json:"reaction_norm"`
	Regression       *RegressionReport  `json:"regression,omitempty"`
	Ablation         AblationReport     `json:"ablation"`
	Cost             CostBreakdown      `json:"cost"`
	FQuality         float64            `json:"f_quality"`
	FNet             float64            `json:"f_net"`
	CostLambda       float64            `json:"cost_lambda"`
	Deployment       DeploymentDecision `json:"deployment"`
	NextMutations    []NextMutation     `json:"next_mutations"`
}

// EvalRunRequest is the §15 request contract.
type EvalRunRequest struct {
	TargetPrompt   string          `json:"target_prompt"`
	AncestorPrompt string          `json:"ancestor_prompt,omitempty"`
	TaskFamily     TaskFamily      `json:"task_family"`
	EvaluationMode string          `json:"evaluation_mode"`
	CostLambda     float64         `json:"cost_lambda"`
	ExtraTests     []TestCase      `json:"extra_tests,omitempty"`
}

// RegressionRequest is the pairwise endpoint contract.
type RegressionRequest struct {
	AncestorPrompt   string     `json:"ancestor_prompt"`
	DescendantPrompt string     `json:"descendant_prompt"`
	TaskFamily       TaskFamily `json:"task_family"`
}
