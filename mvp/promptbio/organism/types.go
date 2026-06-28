// Package organism implements the v3.0 Unified Prompt Organism Architecture.
// See ingest-done/35-3.0-prompt-dna.md.done for the canonical source.
//
// Three structural catalogues land verbatim:
//
//   - 16 organs (§5)            — Genome, Constitutional, Context, Epistemic,
//                                  Metabolic, Immune/Security/Privacy, Decision,
//                                  Planning/Control, Runtime, Memory, Tool,
//                                  Observability, Evaluation, Governance,
//                                  Reproductive/Factory, Autopoietic.
//   - 7 information flows (§6)  — input, epistemic, decision, planning,
//                                  output, feedback, reproductive.
//   - 6 control loops (§13)     — homeostatic, epistemic, agentic,
//                                  evaluation, governance, autopoietic.
//
// Plus 10 architectural principles (§20) enforced as build-time checks
// and 3 organism sizes (micro / meso / macro) with required-organ
// gates from §11. The substrate every higher abstraction (v3.1
// Patterns, v3.2 Composition, future v3.x layers) reuses.
package organism

// OrganName is the canonical 16-organ enum (§5 lines 276-800).
type OrganName string

const (
	OrganGenome         OrganName = "genome"
	OrganConstitutional OrganName = "constitutional"
	OrganContext        OrganName = "context"
	OrganEpistemic      OrganName = "epistemic"
	OrganMetabolic      OrganName = "metabolic"
	OrganImmune         OrganName = "immune"
	OrganDecision       OrganName = "decision"
	OrganPlanning       OrganName = "planning"
	OrganRuntime        OrganName = "runtime"
	OrganMemory         OrganName = "memory"
	OrganTool           OrganName = "tool"
	OrganObservability  OrganName = "observability"
	OrganEvaluation     OrganName = "evaluation"
	OrganGovernance     OrganName = "governance"
	OrganReproductive   OrganName = "reproductive"
	OrganAutopoietic    OrganName = "autopoietic"
)

// AllOrgans is the canonical iteration order — stable across catalogues
// JSON for diff-based testing.
var AllOrgans = []OrganName{
	OrganGenome, OrganConstitutional, OrganContext, OrganEpistemic,
	OrganMetabolic, OrganImmune, OrganDecision, OrganPlanning,
	OrganRuntime, OrganMemory, OrganTool, OrganObservability,
	OrganEvaluation, OrganGovernance, OrganReproductive, OrganAutopoietic,
}

// Organ describes one element of the §5 catalogue.
type Organ struct {
	Name          OrganName `json:"name"`
	State         string    `json:"state"`
	Function      string    `json:"function"`
	FailureModes  []string  `json:"failure_modes"`
	WiredToModule string    `json:"wired_to_module"`
	BreedOSIssue  string    `json:"breedos_issue,omitempty"`
}

// FlowName is the 7-flow enum (§6 lines 803-883).
type FlowName string

const (
	FlowInput        FlowName = "input"
	FlowEpistemic    FlowName = "epistemic"
	FlowDecision     FlowName = "decision"
	FlowPlanning     FlowName = "planning"
	FlowOutput       FlowName = "output"
	FlowFeedback     FlowName = "feedback"
	FlowReproductive FlowName = "reproductive"
)

var AllFlows = []FlowName{
	FlowInput, FlowEpistemic, FlowDecision, FlowPlanning,
	FlowOutput, FlowFeedback, FlowReproductive,
}

// Flow describes one of the 7 information-flow chains.
type Flow struct {
	Name        FlowName    `json:"name"`
	Stages      []OrganName `json:"stages"`
	Purpose     string      `json:"purpose"`
	FailureMode string      `json:"failure_mode"`
}

// LoopName is the 6-control-loop enum (§13 lines 1194-1306).
type LoopName string

const (
	LoopHomeostatic LoopName = "homeostatic"
	LoopEpistemic   LoopName = "epistemic"
	LoopAgentic     LoopName = "agentic"
	LoopEvaluation  LoopName = "evaluation"
	LoopGovernance  LoopName = "governance"
	LoopAutopoietic LoopName = "autopoietic"
)

var AllLoops = []LoopName{
	LoopHomeostatic, LoopEpistemic, LoopAgentic,
	LoopEvaluation, LoopGovernance, LoopAutopoietic,
}

// Loop describes one of the 6 control loops.
type Loop struct {
	Name        LoopName    `json:"name"`
	Targets     string      `json:"target_invariant"`
	Steps       []string    `json:"steps"`
	Organs      []OrganName `json:"organs"`
	FailureMode string      `json:"failure_mode"`
}

// Principle is the 10-element §20 catalogue of architectural rules.
type Principle struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Statement   string `json:"statement"`
	Enforces    string `json:"enforces"`
	ViolationFix string `json:"violation_fix"`
}

// OrganismSize is the §11 enum.
type OrganismSize string

const (
	SizeMicro OrganismSize = "micro"
	SizeMeso  OrganismSize = "meso"
	SizeMacro OrganismSize = "macro"
)

// DeploymentKind is the input §1 envelope enum.
type DeploymentKind string

const (
	DeploymentSingleTurn DeploymentKind = "single-turn"
	DeploymentMultiTurn  DeploymentKind = "multi-turn"
	DeploymentAgent      DeploymentKind = "agent"
	DeploymentProduct    DeploymentKind = "product"
	DeploymentAPI        DeploymentKind = "API"
	DeploymentInternal   DeploymentKind = "internal"
)

// RiskLevel is the §1 risk-tier enum.
type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

// QualityRequirements is the §1 gate-tier enum.
type QualityRequirements string

const (
	QualityDraft         QualityRequirements = "draft"
	QualityReliable      QualityRequirements = "reliable"
	QualityProduction    QualityRequirements = "production"
	QualityHighAssurance QualityRequirements = "high-assurance"
)

// ToolEnvelope is the §1 agency-tool enum.
type ToolEnvelope string

const (
	ToolNone      ToolEnvelope = "none"
	ToolReadOnly  ToolEnvelope = "read-only"
	ToolWrite     ToolEnvelope = "write"
	ToolExternal  ToolEnvelope = "external_actions"
)

// MemoryMode is the §1 memory-depth enum.
type MemoryMode string

const (
	MemoryDisabled MemoryMode = "disabled"
	MemorySession  MemoryMode = "session"
	MemoryProject  MemoryMode = "project"
	MemoryLongTerm MemoryMode = "long-term"
)

// AutopoiesisLevel is the §5.16 enum.
type AutopoiesisLevel string

const (
	AutopoiesisNone                   AutopoiesisLevel = "none"
	AutopoiesisAssistedSelfMaintenance AutopoiesisLevel = "assisted_self_maintenance"
	AutopoiesisSemiAutomatic           AutopoiesisLevel = "semi_automatic"
	AutopoiesisAutomatic               AutopoiesisLevel = "automatic"
)

// BuildRequest is the §21 builder input contract.
type BuildRequest struct {
	UseCase             string              `json:"use_case"`
	TargetPhenotype     string              `json:"target_phenotype"`
	Deployment          DeploymentKind      `json:"deployment"`
	RiskLevel           RiskLevel           `json:"risk_level"`
	DataSources         []string            `json:"data_sources"`
	Tools               ToolEnvelope        `json:"tools"`
	Memory              MemoryMode          `json:"memory"`
	QualityRequirements QualityRequirements `json:"quality_requirements"`
	Constraints         Constraints         `json:"constraints"`
	CurrentArtefact     string              `json:"current_prompt_or_system,omitempty"`
}

// Constraints holds the hard-limit envelope.
type Constraints struct {
	Cost     string `json:"cost,omitempty"`
	Latency  string `json:"latency,omitempty"`
	Privacy  string `json:"privacy,omitempty"`
	Security string `json:"security,omitempty"`
	Format   string `json:"format,omitempty"`
}

// PromptOrganismSpec is the §15 canonical YAML schema (lines 1368-1486).
type PromptOrganismSpec struct {
	PromptOrganism Identity        `json:"prompt_organism"`
	Values         ValuesSpec      `json:"values"`
	Constitution   ConstitutionSpec `json:"constitution"`
	Genome         GenomeSpec      `json:"genome"`
	ExpressionProfiles []string    `json:"expression_profiles"`
	Runtime        RuntimeSpec     `json:"runtime"`
	Epistemology   EpistemologySpec `json:"epistemology"`
	Decision       DecisionSpec    `json:"decision"`
	Planning       PlanningSpec    `json:"planning"`
	Metabolism     MetabolismSpec  `json:"metabolism"`
	Immunity       ImmunitySpec    `json:"immunity"`
	Homeostasis    HomeostasisSpec `json:"homeostasis"`
	Evaluation     EvaluationSpec  `json:"evaluation"`
	Observability  ObservabilitySpec `json:"observability"`
	Governance     GovernanceSpec  `json:"governance"`
	Autopoiesis    AutopoiesisSpec `json:"autopoiesis"`
	Lifecycle      LifecycleSpec   `json:"lifecycle"`
}

// Identity is the §15 top identity block.
type Identity struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	OrganismType OrganismSize `json:"organism_type"`
	Species      string       `json:"species"`
}

type ValuesSpec struct {
	Primary []string `json:"primary"`
}

type ConstitutionSpec struct {
	Version   string   `json:"version"`
	Immutable []string `json:"immutable"`
}

type GenomeSpec struct {
	Spec      string   `json:"spec"`
	CoreGenes []string `json:"core_genes"`
}

type RuntimeSpec struct {
	StateSchema    string   `json:"state_schema"`
	DefaultProfile string   `json:"default_profile"`
	ContextPolicy  string   `json:"context_policy"`
	MemoryPolicy   string   `json:"memory_policy"`
	Tools          []string `json:"tools"`
}

type EpistemologySpec struct {
	BeliefStateEnabled bool     `json:"belief_state_enabled"`
	ClaimTypes         []string `json:"claim_types"`
}

type DecisionSpec struct {
	Policy           string   `json:"policy"`
	ToolROIRequired  bool     `json:"tool_roi_required"`
	ExternalActions  string   `json:"external_actions"`
}

type PlanningSpec struct {
	PlanObjectRequired       bool `json:"plan_object_required_for_complex_tasks"`
	CheckpointsRequired      bool `json:"checkpoints_required"`
	StopConditionsRequired   bool `json:"stop_conditions_required"`
}

type MetabolismSpec struct {
	DigestContextBeforeAnswer bool `json:"digest_context_before_answer"`
	ActiveWorkingContext      bool `json:"active_working_context"`
	WasteRemoval              bool `json:"waste_removal"`
}

type ImmunitySpec struct {
	PromptInjectionBoundary bool `json:"prompt_injection_boundary"`
	ContradictionHandling   bool `json:"contradiction_handling"`
	PrivacyMinimization     bool `json:"privacy_minimization"`
}

type HomeostasisSpec struct {
	ObjectiveLock         bool `json:"objective_lock"`
	ConstraintCheck       bool `json:"constraint_check"`
	FormatCheck           bool `json:"format_check"`
	ConfidenceCalibration bool `json:"confidence_calibration"`
}

type EvaluationSpec struct {
	TestSuite       string `json:"test_suite"`
	FitnessFunction string `json:"fitness_function"`
}

type ObservabilitySpec struct {
	TraceLevel string   `json:"trace_level"`
	Metrics    []string `json:"metrics"`
}

type GovernanceSpec struct {
	RiskTier                 string `json:"risk_tier"`
	Owner                    string `json:"owner"`
	ApprovalRequiredForMajor bool   `json:"approval_required_for_major_changes"`
}

type AutopoiesisSpec struct {
	Level             AutopoiesisLevel `json:"level"`
	AllowedAutoActions []string        `json:"allowed_auto_actions"`
	RequiresApproval   []string        `json:"requires_approval"`
}

type LifecycleSpec struct {
	Status        string `json:"status"`
	Version       string `json:"version"`
	ReviewCadence string `json:"review_cadence"`
}

// BuildResponse is the §21 builder output bundle.
type BuildResponse struct {
	Spec                  PromptOrganismSpec `json:"spec"`
	Diagram               string             `json:"organism_diagram"`
	OrganMap              map[OrganName]string `json:"organ_map"`
	InformationFlows      []Flow             `json:"information_flows"`
	ControlLoops          []Loop             `json:"control_loops"`
	MinimumViableVersion  []string           `json:"minimum_viable_version"`
	FullProductionPath    []string           `json:"full_production_path"`
	TopRisks              []TopRisk          `json:"top_risks"`
	ValidationReport      ValidationReport   `json:"validation_report"`
}

// TopRisk pairs a failure mode with its repair.
type TopRisk struct {
	FailureMode string `json:"failure_mode"`
	Repair      string `json:"repair"`
}

// ValidationReport bundles principle violations and classification verdicts.
type ValidationReport struct {
	Pass                 bool                `json:"pass"`
	PrincipleViolations  []PrincipleViolation `json:"principle_violations,omitempty"`
	MissingOrgans        []OrganName         `json:"missing_organs,omitempty"`
	ClassificationVerdict string              `json:"classification_verdict"`
	Notes                []string            `json:"notes,omitempty"`
}

// PrincipleViolation explains which §20 principle was breached + how to fix.
type PrincipleViolation struct {
	PrincipleID  int    `json:"principle_id"`
	PrincipleName string `json:"principle_name"`
	Detail       string `json:"detail"`
	Fix          string `json:"fix"`
}

// ValidateRequest is the body for /organism/validate.
type ValidateRequest struct {
	Spec PromptOrganismSpec `json:"spec"`
}

// CataloguesResponse is the body returned by GET /organism/catalogues.
type CataloguesResponse struct {
	Organs     []Organ     `json:"organs"`
	Flows      []Flow      `json:"flows"`
	Loops      []Loop      `json:"loops"`
	Principles []Principle `json:"principles"`
	Sizes      []SizeSpec  `json:"sizes"`
}

// SizeSpec describes one of the 3 organism sizes plus its required-organ gate.
type SizeSpec struct {
	Size            OrganismSize `json:"size"`
	Description     string       `json:"description"`
	RequiredOrgans  []OrganName  `json:"required_organs"`
	TypicalUseCases []string     `json:"typical_use_cases"`
}
