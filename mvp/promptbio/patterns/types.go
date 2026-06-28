// Package patterns implements the v3.1 Prompt Organism Design Patterns.
// See ingest-done/36-3.1-prompt-dna.md.done for the canonical source.
//
// v3.1 is zoology — a catalogue of 12 repeatable architectural patterns
// for distinct classes of tasks, built on top of the v3.0 anatomy
// (Issue 33). The slogan: "Right pattern before right prompt."
//
// Formula: Task Niche → Organism Pattern → Prompt System Architecture.
package patterns

import (
	"breedos-mvp/promptbio/organism"
)

// PatternName is the canonical 12-element enum (§3.1).
type PatternName string

const (
	PatternExplainerCell        PatternName = "Explainer Cell"
	PatternResearchSynthesizer  PatternName = "Research Synthesizer"
	PatternStrategyAdvisor      PatternName = "Strategy Advisor"
	PatternStrategyCritic       PatternName = "Strategy Critic"
	PatternCodeRepair           PatternName = "Code Repair"
	PatternDocumentReview       PatternName = "Document Review"
	PatternDataAnalysis         PatternName = "Data Analysis"
	PatternEvalJudge            PatternName = "Eval Judge"
	PatternMemoryAwareCompanion PatternName = "Memory-Aware Companion"
	PatternAgenticToolUsing     PatternName = "Agentic Tool-Using"
	PatternHighAssuranceAdvisor PatternName = "High-Assurance Advisor"
	PatternAutopoieticEcosystem PatternName = "Autopoietic Ecosystem"
)

// AllPatterns is the canonical iteration order.
var AllPatterns = []PatternName{
	PatternExplainerCell, PatternResearchSynthesizer, PatternStrategyAdvisor,
	PatternStrategyCritic, PatternCodeRepair, PatternDocumentReview,
	PatternDataAnalysis, PatternEvalJudge, PatternMemoryAwareCompanion,
	PatternAgenticToolUsing, PatternHighAssuranceAdvisor, PatternAutopoieticEcosystem,
}

// AgencyLevel is the A0-A6 scale fixed in earlier promptbio modules.
type AgencyLevel string

const (
	AgencyA0 AgencyLevel = "A0" // pure information, no action
	AgencyA1 AgencyLevel = "A1" // single-call retrieval
	AgencyA2 AgencyLevel = "A2" // bounded plan + bounded loop
	AgencyA3 AgencyLevel = "A3" // multi-step with checkpoints
	AgencyA4 AgencyLevel = "A4" // delegated sub-agents
	AgencyA5 AgencyLevel = "A5" // self-modifying spec under governance
	AgencyA6 AgencyLevel = "A6" // autonomous ecosystem
)

// AntiPatternName is the canonical 10-element §18 catalogue.
type AntiPatternName string

const (
	AntiGodPrompt              AntiPatternName = "God Prompt"
	AntiConfidentOracle        AntiPatternName = "Confident Oracle"
	AntiToolGremlin            AntiPatternName = "Tool Gremlin"
	AntiMemoryHoarder          AntiPatternName = "Memory Hoarder"
	AntiEvalGamer              AntiPatternName = "Eval Gamer"
	AntiPaperShield            AntiPatternName = "Paper Shield"
	AntiFormatTyrant           AntiPatternName = "Format Tyrant"
	AntiAgentWithoutStopCond   AntiPatternName = "Agent Without Stop Condition"
	AntiStrategyFantasist      AntiPatternName = "Strategy Fantasist"
	AntiDocumentPuppet         AntiPatternName = "Document Puppet"
)

var AllAntiPatterns = []AntiPatternName{
	AntiGodPrompt, AntiConfidentOracle, AntiToolGremlin, AntiMemoryHoarder,
	AntiEvalGamer, AntiPaperShield, AntiFormatTyrant, AntiAgentWithoutStopCond,
	AntiStrategyFantasist, AntiDocumentPuppet,
}

// PatternCard is the 20-field schema for one pattern catalogue entry.
type PatternCard struct {
	Name                     PatternName           `json:"name"`
	Intent                   string                `json:"intent"`
	UseWhen                  []string              `json:"use_when"`
	AvoidWhen                []string              `json:"avoid_when"`
	OrganismSize             organism.OrganismSize `json:"organism_size"`
	AgencyLevel              AgencyLevel           `json:"agency_level"`
	CoreOrgans               []organism.OrganName  `json:"core_organs"`
	OptionalOrgans           []organism.OrganName  `json:"optional_organs"`
	Genome                   string                `json:"genome"`
	Epistemology             string                `json:"epistemology"`
	DecisionPlanning         string                `json:"decision_planning"`
	Runtime                  string                `json:"runtime"`
	Memory                   string                `json:"memory"`
	Tools                    string                `json:"tools"`
	OutputPhenotype          string                `json:"output_phenotype"`
	FailureModes             []string              `json:"failure_modes"`
	RequiredTests            []string              `json:"required_tests"`
	MinimalPromptSkeleton    string                `json:"minimal_prompt_skeleton"`
	ProductionArchitecture   string                `json:"production_architecture"`
	AntiPatterns             []AntiPatternName     `json:"anti_patterns"`
	EvolutionPath            string                `json:"evolution_path"`
}

// SelectorRequest is the input contract for /api/promptbio/pattern-select.
type SelectorRequest struct {
	TaskNiche       string                       `json:"task_niche"`
	TargetPhenotype string                       `json:"target_phenotype"`
	RiskLevel       organism.RiskLevel           `json:"risk_level"`
	DataSources     []string                     `json:"data_sources"`
	Tools           organism.ToolEnvelope        `json:"tools"`
	Memory          organism.MemoryMode          `json:"memory"`
	Deployment      organism.DeploymentKind      `json:"deployment"`
	QualityLevel    organism.QualityRequirements `json:"quality_level"`
}

// PatternRecommendation is the §3.1 11-section output bundle.
type PatternRecommendation struct {
	RecommendedPattern    PatternName            `json:"recommended_pattern"`
	WhyThisPattern        string                 `json:"why_this_pattern"`
	OrganismSize          organism.OrganismSize  `json:"organism_size"`
	AgencyLevel           AgencyLevel            `json:"agency_level"`
	RequiredOrgans        []organism.OrganName   `json:"required_organs"`
	OptionalOrgans        []organism.OrganName   `json:"optional_organs"`
	AntiPatternRisks      []AntiPatternName      `json:"anti_pattern_risks"`
	MinimalPromptSkeleton string                 `json:"minimal_prompt_skeleton"`
	RequiredTests         []string               `json:"required_tests"`
	NextAction            string                 `json:"next_action"`
	Card                  PatternCard            `json:"card"`
}

// ComposeRequest is the body for /api/promptbio/pattern-compose.
type ComposeRequest struct {
	Patterns []PatternName `json:"patterns"`
}

// RuleViolation is one composition-rule failure.
type RuleViolation struct {
	RuleID   int    `json:"rule_id"`
	RuleName string `json:"rule_name"`
	Detail   string `json:"detail"`
	Fix      string `json:"fix"`
}

// CompositionPlan is the result of composing N patterns into one organism.
type CompositionPlan struct {
	Patterns        []PatternName        `json:"patterns"`
	RequiredOrgans  []organism.OrganName `json:"required_organs"`
	OptionalOrgans  []organism.OrganName `json:"optional_organs"`
	OrganismSize    organism.OrganismSize `json:"organism_size"`
	AgencyLevel     AgencyLevel          `json:"agency_level"`
	AntiPatterns    []AntiPatternName    `json:"anti_pattern_risks"`
	Violations      []RuleViolation      `json:"violations"`
	Pass            bool                 `json:"pass"`
}

// AntiPatternFinding is one detection result from detect_anti_patterns.
type AntiPatternFinding struct {
	Name      AntiPatternName `json:"name"`
	Symptom   string          `json:"symptom"`
	Treatment string          `json:"treatment"`
	Evidence  string          `json:"evidence"`
}

// AntiPatternRequest runs the linter against an organism spec.
type AntiPatternRequest struct {
	Spec organism.PromptOrganismSpec `json:"spec"`
}

// AntiPatternResponse bundles findings.
type AntiPatternResponse struct {
	Findings []AntiPatternFinding `json:"findings"`
	Pass     bool                 `json:"pass"`
}

// CataloguesResponse is GET /api/promptbio/patterns.
type CataloguesResponse struct {
	Patterns         []PatternCard         `json:"patterns"`
	SelectionMatrix  []SelectionMatrixRow  `json:"pattern_selection_matrix"`
	CompositionRules []CompositionRule     `json:"composition_rules"`
	AntiPatterns     []AntiPatternEntry    `json:"anti_patterns"`
}

// SelectionMatrixRow is one task→pattern mapping (§1577-1592).
type SelectionMatrixRow struct {
	Niche   string      `json:"niche"`
	Pattern PatternName `json:"pattern"`
	Why     string      `json:"why"`
}

// CompositionRule is one of the §1688-1737 8 rules.
type CompositionRule struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Statement string `json:"statement"`
}

// AntiPatternEntry is one §1741-1929 anti-pattern catalogue row.
type AntiPatternEntry struct {
	Name      AntiPatternName `json:"name"`
	Symptom   string          `json:"symptom"`
	Treatment string          `json:"treatment"`
}
