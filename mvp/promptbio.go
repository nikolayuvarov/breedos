package main

// v0.7.25 — Promptbio substrate scaffold (Issue 07).
//
// This file defines the type surface the prompt-organism simulator
// will live on, with NO runtime behaviour yet. The point of shipping
// types-only is to:
//   1. Lock the substrate vocabulary used by Issues 01-06 specs.
//   2. Give the next implementation session a concrete starting line.
//   3. Avoid changing the biological simulator path — there is no
//      wiring into runSimulation, aggregateReplicates, or the HTTP
//      mux. The biological release is bit-identical to v0.7.24.
//
// When Issue 07 is implemented, this file is the substrate; the
// concrete operators (Mutate, Recombine, Evaluate) move into a
// separate `promptbio_engine.go`, the HTTP route is wired into main.go,
// and a regression test confirms the biological path is unchanged.
//
// References:
//   - issues-promptbio/07-engine-extension-prompt-organism-mode.md
//   - issues-promptbio/01-prompt-genome-mapper.md (14-locus taxonomy)
//   - ingest-done/04-prompt-dna.md.done lines 440-525

// PromptLocus is one of the 14 named functional positions in a
// prompt-genotype. Order is significant — the codeword for a genotype
// is the locus values in this declaration order.
type PromptLocus int

const (
	LocusTask           PromptLocus = iota // What the response should accomplish.
	LocusSubject                           // What the response is about.
	LocusAudience                          // Whom the response is for.
	LocusContextNiche                      // Surrounding situation / use case.
	LocusDepth                             // How deep the response should go.
	LocusFormatMorph                       // Structure of the response (JSON, list, prose).
	LocusConstraints                       // Hard restrictions on the response.
	LocusEvidenceStd                       // What kind of evidence the response must cite.
	LocusMethod                            // Reasoning procedure to use.
	LocusExamples                          // In-prompt few-shot exemplars.
	LocusTools                             // Tools or external calls available.
	LocusSelfEval                          // Self-checks before emitting.
	LocusFitness                           // Explicit fitness / quality criteria.
	LocusEvolutionRule                     // How the prompt should be improved next.

	numPromptLoci = 14
)

// LocusStatus is the presence state of a locus in a given prompt.
type LocusStatus string

const (
	LocusPresent  LocusStatus = "present"  // Explicit, unambiguous.
	LocusImplicit LocusStatus = "implicit" // Implied but not stated.
	LocusAbsent   LocusStatus = "absent"   // Missing entirely.
)

// PromptGenotype is the substrate-level representation of one prompt.
// Output of Issue 01 (Prompt Genome Mapper); input to Issues 02-05.
type PromptGenotype struct {
	OriginalPrompt    string                       `json:"original_prompt"`
	GenotypeMap       [numPromptLoci]LocusEntry    `json:"genotype_map"`
	MissingGenes      []string                     `json:"missing_genes"`
	ConflictGenes     []ConflictPair               `json:"conflict_genes"`
	ExpectedPhenotype PhenotypePrediction          `json:"expected_phenotype"`
	FragilityPoints   []string                     `json:"fragility_points"`
	MutationPlan      []PromptMutation             `json:"mutation_plan"`
	ImprovedPrompt    string                       `json:"improved_prompt"`
	GenotypeNotation  string                       `json:"genotype_notation"`
	TestProtocol      []TestSpec                   `json:"test_protocol"`
	MapperModel       string                       `json:"mapper_model"`
	MapperSeed        int64                        `json:"mapper_seed"`
}

// LocusEntry is one filled-in slot in the 14-locus map.
type LocusEntry struct {
	Locus      PromptLocus `json:"locus"`
	Status     LocusStatus `json:"status"`
	Fragment   string      `json:"fragment"`
	Effect     string      `json:"expected_phenotype_effect"`
}

// ConflictPair flags two loci with contradictory instructions.
type ConflictPair struct {
	A      PromptLocus `json:"a"`
	B      PromptLocus `json:"b"`
	Reason string      `json:"reason"`
}

// PhenotypePrediction is the expected response shape if the prompt were
// run; populated by Issue 01 and consumed by Issues 02 and 03.
type PhenotypePrediction struct {
	Structure    string `json:"structure"`
	Depth        string `json:"depth"`
	Accuracy     string `json:"accuracy"`
	Concreteness string `json:"concreteness"`
	Style        string `json:"style"`
	Usefulness   string `json:"usefulness"`
	Risks        string `json:"risks"`
}

// PromptMutationKind enumerates the six classes of prompt mutation.
type PromptMutationKind string

const (
	MutationAdd         PromptMutationKind = "addition"
	MutationDelete      PromptMutationKind = "deletion"
	MutationSubstitute  PromptMutationKind = "substitution"
	MutationAmplify     PromptMutationKind = "amplification"
	MutationSuppress    PromptMutationKind = "suppression"
	MutationModularize  PromptMutationKind = "modularization"
)

// PromptMutation is one proposed change to a prompt.
type PromptMutation struct {
	Kind        PromptMutationKind `json:"kind"`
	Locus       PromptLocus        `json:"locus"`
	Description string             `json:"description"`
	Rationale   string             `json:"rationale"`
}

// TestSpec is one of the v0.1 test-protocol entries — concrete enough
// for another agent to execute without clarification.
type TestSpec struct {
	Input          string `json:"input"`
	ExpectedBehave string `json:"expected_behavior"`
	PassFail       string `json:"pass_fail_criteria"`
}

// PromptbioSimRequest is the request shape for /api/promptbio/simulate
// once Issue 07 is implemented. Mirrors SimRequest but for the prompt
// substrate.
type PromptbioSimRequest struct {
	AncestorPrompt    string  `json:"ancestor_prompt"`
	TargetPhenotype   string  `json:"target_phenotype"`
	JudgeModel        string  `json:"judge_model"`
	Population        int     `json:"population"`
	Generations       int     `json:"generations"`
	MutationStrategy  string  `json:"mutation_strategy"`
	Seed              int64   `json:"seed"`
	Replicates        int     `json:"replicates"`
}

// PromptbioVersion is the substrate version tag, surfaced through
// /api/version when the prompt substrate gets wired into a release.
const PromptbioVersion = "v0.1-scaffold"
