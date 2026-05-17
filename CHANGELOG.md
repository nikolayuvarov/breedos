# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.6.2] - 2026-05-17

### Fixed
- Corrected the diversity-collapse probability calculation in `aggregateReplicates`. The previous version compared the inbreeding coefficient against the diversity-loss limit, which effectively duplicated the inbreeding-breach metric. The fix now computes the relative diversity loss `(baseDiversity − finalDiversity) / baseDiversity` and compares it against the limit, so the probability reflects actual diversity collapse rather than re-measuring inbreeding.

### Changed
- Updated MVP version strings (landing footer, demo kicker, run notes) from `v0.6.1` to `v0.6.2`.

## [0.6.1] - 2026-05-16

### Changed
- Core thesis statement refined to acknowledge that prediction is part of breeding's complexity rather than separate from it. New wording: "Breeding is not only a prediction problem. It is, more fundamentally, a multi-generation control problem over an evolving population." The previous absolutist framing risked being read as dismissive of genomic prediction work (rrBLUP, BGLR, deep-learning predictors) by domain reviewers.
- Updated MVP version strings in landing-page footer, demo-page kicker, and run-notes from `v0.6` to `v0.6.1`.

## [0.6.0] - 2026-05-15

### Added
- Monte Carlo replicates per strategy.
- Worker-pool execution for `strategies × replicates` jobs.
- Core and advanced strategy sets.
- Neutral drift and random parent baselines.
- Phenotype truncation selection.
- Genomic selection mock (placeholder for GBLUP/Bayesian/ML predictors).
- OCS-like constrained selection (gain under a similarity/diversity penalty).
- Cross planner strategy with more distant mate allocation.
- Edit-aware introgression planner for minimal CRISPR decision-layer integration.
- Risk probabilities: inbreeding breach, diversity collapse, rare useful allele loss.
- Risk-adjusted score, decision rank, and decision summary panel.
- Pareto gain/risk chart.
- Tests for advanced strategy set, risk probabilities, decision ranks, and Pareto summary.

### Changed
- Demo reframed as an early decision engine rather than a single-strategy simulator.

## [0.5.0]

### Added
- Parallel strategy execution: independent strategy simulations run concurrently and results are returned in canonical order.
- Aggregate parallel strategy-generation progress reporting.
- Tests for parallel execution notes and strategy order.

### Changed
- After a run, the browser form is updated with the normalized request returned by the server, making no-change detection reliable.

### Removed
- Automatic page-load calculation: the demo now waits for an explicit Run simulation click.
- No-change run guard: if the current form matches the last completed run, pressing Run simulation does not start a backend job.

## [0.4.0]

### Added
- Asynchronous API endpoints: `POST /api/simulate/start` and `GET /api/simulate/status?id=<job_id>`.
- Real backend job progress: the Run simulation button shows percentage while the server computes.
- Request dirty-state behavior: changing any input shows "Press Run simulation to update graphs."
- Regression coverage confirming a `mutation_rate` change triggers a fresh run on manual submit.
- Progress callback tests.

### Removed
- Auto-run checkbox and automatic recalculation after field changes.

### Notes
- `POST /api/simulate` is retained as a synchronous endpoint for tests and direct API use.

## [0.3.0]

### Fixed
- `mutation_rate=0` is no longer treated as missing and is no longer rewritten to the default.
- `crispr_edits=0` now disables CRISPR candidate ranking instead of silently returning three candidates.

### Added
- Expected mutation-flip notes and summary cards so small mutation-rate changes are easier to interpret.
- Changed-parameter notes: when only mutation rate changes, the run notes explicitly show the previous and current values.
- Frontend support for comma decimal input normalization.
- Regression tests for zero mutation rate and disabled CRISPR edits.

## [0.2.0]

### Added
- Previous-run dotted chart overlays.
- Field-level tooltips on the demo form.
- Fixed-loci metrics and chart.
- Presets, random-seed button, copy-summary, JSON export, and run notes.
- `.gitignore` rules to exclude local build artifacts.

### Changed
- Population-size minimum lowered to 2 to expose small-population drift and fixation.
- Population-size maximum raised to 5000, protected by a global simulation budget guard.

## [0.1.0]

### Added
- First runnable BreedOS MVP: landing page, simulation demo, and CRISPR-aware seed strategy.
