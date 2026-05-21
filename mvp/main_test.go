package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestMutationRateZeroIsPreserved(t *testing.T) {
	req := SimRequest{Seed: 123, PopulationSize: 12, Markers: 40, QTLCount: 8, Generations: 8, SelectionPercent: 25, Heritability: 0.5, MutationRate: 0, CrisprEnabled: true, CrisprEdits: 2, Replicates: 1}
	resp, err := runSimulation(req)
	if err != nil {
		t.Fatalf("runSimulation failed: %v", err)
	}
	if resp.Request.MutationRate != 0 {
		t.Fatalf("mutation rate was normalized incorrectly: got %g, want 0", resp.Request.MutationRate)
	}
	joined := strings.Join(resp.Notes, "\n")
	if !strings.Contains(joined, "Mutation rate is 0") {
		t.Fatalf("expected zero-mutation note, got: %s", joined)
	}
}

func TestCrisprEditsZeroDisablesCandidatesAndCrisprStrategy(t *testing.T) {
	req := SimRequest{Seed: 123, PopulationSize: 40, Markers: 80, QTLCount: 10, Generations: 5, SelectionPercent: 20, Heritability: 0.5, MutationRate: 0.0001, CrisprEnabled: true, CrisprEdits: 0, CrisprIntroPercent: 0, Replicates: 1}
	resp, err := runSimulation(req)
	if err != nil {
		t.Fatalf("runSimulation failed: %v", err)
	}
	if len(resp.CandidateEdits) != 0 {
		t.Fatalf("expected no edit candidates when crispr_edits=0, got %d", len(resp.CandidateEdits))
	}
	for _, s := range resp.Strategies {
		if strings.Contains(s.Code, "crispr") || strings.Contains(s.Code, "edit_introgression") {
			t.Fatalf("CRISPR/edit strategy should be disabled when crispr_edits=0, got %s", s.Code)
		}
	}
}

func TestMutationRateChangeIsAcceptedOnManualRun(t *testing.T) {
	base := SimRequest{Seed: 98765, PopulationSize: 40, Markers: 120, QTLCount: 20, Generations: 30, SelectionPercent: 20, Heritability: 0.5, MutationRate: 0, Replicates: 1}
	withoutMutations, err := runSimulation(base)
	if err != nil {
		t.Fatalf("runSimulation without mutations failed: %v", err)
	}
	withMutationReq := base
	withMutationReq.MutationRate = 0.01
	withMutations, err := runSimulation(withMutationReq)
	if err != nil {
		t.Fatalf("runSimulation with mutations failed: %v", err)
	}
	if withoutMutations.Request.MutationRate != 0 {
		t.Fatalf("expected first run mutation rate echo 0, got %g", withoutMutations.Request.MutationRate)
	}
	if withMutations.Request.MutationRate != 0.01 {
		t.Fatalf("expected second run mutation rate echo 0.01, got %g", withMutations.Request.MutationRate)
	}
	if len(withoutMutations.Strategies) == 0 || len(withMutations.Strategies) == 0 {
		t.Fatalf("expected strategies in both runs")
	}
	before := withoutMutations.Strategies[0].Final
	after := withMutations.Strategies[0].Final
	if before.GeneticGain == after.GeneticGain && before.Diversity == after.Diversity && before.FixedLoci == after.FixedLoci {
		t.Fatalf("mutation rate change was accepted but produced effectively identical final stats; before=%+v after=%+v", before, after)
	}
}

func TestProgressCallbackAdvances(t *testing.T) {
	req := SimRequest{Seed: 4321, PopulationSize: 20, Markers: 60, QTLCount: 10, Generations: 8, SelectionPercent: 25, Heritability: 0.5, MutationRate: 0.001, Replicates: 2}
	var mu sync.Mutex
	maxPercent, calls := 0, 0
	emptyMessage, parallelMessage := false, false
	_, err := runSimulationWithProgress(req, func(percent int, message string) {
		mu.Lock()
		defer mu.Unlock()
		calls++
		if percent > maxPercent {
			maxPercent = percent
		}
		if message == "" {
			emptyMessage = true
		}
		if strings.Contains(strings.ToLower(message), "parallel") || strings.Contains(strings.ToLower(message), "worker") {
			parallelMessage = true
		}
	})
	if err != nil {
		t.Fatalf("runSimulationWithProgress failed: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if emptyMessage {
		t.Fatalf("progress message must not be empty")
	}
	if !parallelMessage {
		t.Fatalf("expected at least one progress message to mention parallel/worker execution")
	}
	if calls < 3 {
		t.Fatalf("expected multiple progress calls, got %d", calls)
	}
	if maxPercent < 98 {
		t.Fatalf("expected progress to reach at least 98 before completion, got %d", maxPercent)
	}
}

func TestCoreStrategyOrderAndCount(t *testing.T) {
	req := SimRequest{Seed: 555, PopulationSize: 30, Markers: 80, QTLCount: 12, Generations: 6, SelectionPercent: 20, Heritability: 0.5, MutationRate: 0.0001, CrisprEnabled: true, CrisprEdits: 2, CrisprIntroPercent: 10, Replicates: 1}
	resp, err := runSimulation(req)
	if err != nil {
		t.Fatalf("runSimulation failed: %v", err)
	}
	want := []string{"neutral", "aggressive", "diversity", "balanced", "balanced_crispr"}
	if len(resp.Strategies) != len(want) {
		t.Fatalf("strategy count mismatch: got %d want %d", len(resp.Strategies), len(want))
	}
	for i, code := range want {
		if resp.Strategies[i].Code != code {
			t.Fatalf("strategy order mismatch at %d: got %q want %q", i, resp.Strategies[i].Code, code)
		}
	}
	joined := strings.Join(resp.Notes, "\n")
	if !strings.Contains(strings.ToLower(joined), "worker pool") && !strings.Contains(strings.ToLower(joined), "parallel") {
		t.Fatalf("expected notes to mention parallel/worker execution, got: %s", joined)
	}
}

func TestAdvancedDecisionEngineReturnsRanksAndRisks(t *testing.T) {
	req := SimRequest{Seed: 777, PopulationSize: 24, Markers: 70, QTLCount: 12, Generations: 7, SelectionPercent: 25, Heritability: 0.45, MutationRate: 0.0002, CrisprEnabled: true, CrisprEdits: 3, CrisprIntroPercent: 10, StrategySet: "advanced", Replicates: 2, WorkerCount: 2, InbreedingLimit: 0.25, DiversityLossLimit: 0.30}
	resp, err := runSimulation(req)
	if err != nil {
		t.Fatalf("runSimulation failed: %v", err)
	}
	codes := map[string]bool{}
	for _, s := range resp.Strategies {
		codes[s.Code] = true
		if s.Replicates != 2 || s.Final.Replicates != 2 {
			t.Fatalf("expected strategy %s to aggregate 2 replicates, got result=%d final=%d", s.Code, s.Replicates, s.Final.Replicates)
		}
		if s.Final.DecisionRank <= 0 {
			t.Fatalf("expected decision rank for %s", s.Code)
		}
		if s.Final.ProbabilityInbreedingBreach < 0 || s.Final.ProbabilityInbreedingBreach > 1 {
			t.Fatalf("risk probability out of range for %s", s.Code)
		}
	}
	for _, required := range []string{"genomic", "ocs_like", "cross_planner", "edit_introgression"} {
		if !codes[required] {
			t.Fatalf("advanced response missing strategy %s", required)
		}
	}
	if resp.Decision.BestRiskAdjustedCode == "" || len(resp.Decision.ParetoCodes) == 0 {
		t.Fatalf("expected decision summary and Pareto codes: %+v", resp.Decision)
	}
}

func TestDecisionReportPopulatesNewFields(t *testing.T) {
	req := SimRequest{Seed: 555, PopulationSize: 24, Markers: 60, QTLCount: 10, Generations: 8, SelectionPercent: 20, Heritability: 0.5, MutationRate: 0.0001, StrategySet: "core", Replicates: 3, InbreedingLimit: 0.25, DiversityLossLimit: 0.30}
	resp, err := runSimulation(req)
	if err != nil {
		t.Fatalf("runSimulation failed: %v", err)
	}
	d := resp.Decision
	if d.SummaryText == "" {
		t.Fatalf("SummaryText must not be empty")
	}
	if d.NextAnalysis == "" {
		t.Fatalf("NextAnalysis must not be empty")
	}
	if len(d.KeyAssumptions) == 0 {
		t.Fatalf("KeyAssumptions must not be empty")
	}
	joined := strings.Join(d.KeyAssumptions, " ")
	if !strings.Contains(joined, "Heritability") && !strings.Contains(joined, "h²") {
		t.Fatalf("KeyAssumptions should mention heritability: %v", d.KeyAssumptions)
	}
	if d.Tradeoffs == nil {
		t.Fatalf("Tradeoffs must be initialized (non-nil), even if empty")
	}
	if d.AvoidStrategies == nil {
		t.Fatalf("AvoidStrategies must be initialized (non-nil), even if empty")
	}
	if d.MissingDataWarnings == nil {
		t.Fatalf("MissingDataWarnings must be initialized (non-nil), even if empty")
	}
	if !strings.Contains(d.SummaryText, "Recommended") {
		t.Fatalf("SummaryText should mention Recommended: %q", d.SummaryText)
	}
}

func TestDecisionReportFlagsHighRiskAggressiveOrIncludesItInTradeoff(t *testing.T) {
	req := SimRequest{Seed: 31337, PopulationSize: 12, Markers: 60, QTLCount: 10, Generations: 25, SelectionPercent: 15, Heritability: 0.5, MutationRate: 0.0001, StrategySet: "core", Replicates: 3, InbreedingLimit: 0.25, DiversityLossLimit: 0.30}
	resp, err := runSimulation(req)
	if err != nil {
		t.Fatalf("runSimulation failed: %v", err)
	}
	flagged := false
	for _, a := range resp.Decision.AvoidStrategies {
		if a.Code == "aggressive" {
			flagged = true
			break
		}
	}
	for _, tr := range resp.Decision.Tradeoffs {
		if tr.A == "aggressive" || tr.B == "aggressive" {
			flagged = true
			break
		}
	}
	if !flagged {
		t.Fatalf("expected aggressive to appear in AvoidStrategies or Tradeoffs (high-pressure small-N scenario); got decision=%+v", resp.Decision)
	}
}

func TestDecisionReportSummaryReferencesRecommendedStrategy(t *testing.T) {
	req := SimRequest{Seed: 777, PopulationSize: 50, Markers: 80, QTLCount: 12, Generations: 15, SelectionPercent: 20, Heritability: 0.5, MutationRate: 0.0001, StrategySet: "advanced", Replicates: 3, InbreedingLimit: 0.25, DiversityLossLimit: 0.30}
	resp, err := runSimulation(req)
	if err != nil {
		t.Fatalf("runSimulation failed: %v", err)
	}
	d := resp.Decision
	bestName := ""
	for _, s := range resp.Strategies {
		if s.Code == d.BestRiskAdjustedCode {
			bestName = s.Name
			break
		}
	}
	if bestName == "" {
		t.Fatalf("best risk-adjusted strategy '%s' not found in results", d.BestRiskAdjustedCode)
	}
	if !strings.Contains(d.SummaryText, bestName) {
		t.Fatalf("SummaryText must reference recommended strategy name '%s'; got: %s", bestName, d.SummaryText)
	}
	keywords := []string{"replicates", "Pareto", "seed", "intensity", "constraints", "robustness", "robust", "narrow"}
	found := false
	for _, kw := range keywords {
		if strings.Contains(d.NextAnalysis, kw) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("NextAnalysis should contain at least one of %v; got: %s", keywords, d.NextAnalysis)
	}
}

func TestPerformSwapRenamesBinaryAndCreatesBackup(t *testing.T) {
	dir := t.TempDir()
	currentPath := filepath.Join(dir, "breedos")
	updatePath := currentPath + ".UPDATE"

	if err := os.WriteFile(currentPath, []byte("OLD"), 0o755); err != nil {
		t.Fatalf("write current: %v", err)
	}
	if err := os.WriteFile(updatePath, []byte("NEW"), 0o755); err != nil {
		t.Fatalf("write update: %v", err)
	}

	if err := performSwap(currentPath, updatePath); err != nil {
		t.Fatalf("performSwap returned error: %v", err)
	}

	data, err := os.ReadFile(currentPath)
	if err != nil {
		t.Fatalf("read current after swap: %v", err)
	}
	if string(data) != "NEW" {
		t.Fatalf("current path should now hold update content; got %q", string(data))
	}

	if _, err := os.Stat(updatePath); !os.IsNotExist(err) {
		t.Fatalf("update path should be gone after swap; stat err: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	var backupName string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "breedos.bak.") {
			backupName = e.Name()
			break
		}
	}
	if backupName == "" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("no backup file matching 'breedos.bak.*' found; dir contains: %v", names)
	}
	backupData, err := os.ReadFile(filepath.Join(dir, backupName))
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(backupData) != "OLD" {
		t.Fatalf("backup should contain old binary content; got %q", string(backupData))
	}
}

func TestEnsureExecutableSetsExecBit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "candidate")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := ensureExecutable(path); err != nil {
		t.Fatalf("ensureExecutable: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Fatalf("owner exec bit should be set after ensureExecutable; got mode %o", info.Mode().Perm())
	}
}
