# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.6.6] - 2026-05-18

### Removed
- Customer-discovery page from public deployment (`mvp/static/customer_discovery.html`, the `/customer-discovery` route handler in `mvp/main.go`, and the corresponding `<a>` navigation links in `index.html` / `demo.html`). The page contained an internal customer-discovery framework (target segments and interview questions) intended for the founder's own workflow, not for public/investor visibility. Removing it focuses the landing on product narrative and the interactive demo only.

### Changed
- Updated MVP version strings (landing footer, demo kicker, run notes) from `v0.6.5` to `v0.6.6`.
- README "Demo pages" list no longer lists `/customer-discovery`.

## [0.6.5] - 2026-05-18

### Added
- `mvp/build.sh` — portable static-binary build script. Uses `CGO_ENABLED=0 go build -trimpath -ldflags='-s -w'` to produce a fully static binary with no glibc dependency, runnable on any reasonably modern Linux regardless of the build host's glibc version. Default output is `../breedos` (next to `install.sh`).
- `install.sh` pre-flight check: runs the binary briefly and parses common dynamic-loader errors (`GLIBC`, `symbol not found`, `cannot load shared object`, `exec format error`). Fails fast with a clear remediation pointing to `./mvp/build.sh` or `CGO_ENABLED=0 go build`.
- `install.sh` post-start verification: after `systemctl start`, polls `systemctl is-active` for up to 5 seconds; if the service does not reach `active`, dumps recent journal output and aborts with a clear error instead of silently reporting success.

### Changed
- `install.sh` refactored from positional arguments to a flag-based CLI: `--binary`, `--service`, `--args`, `--user`, `--workdir`, `--description`, `--non-interactive`, `--force` (with short aliases `-b -s -a -u -w -d -y -f`). Each install/uninstall/info subcommand parses its own flags.
- `install.sh` no longer hard-codes binary-specific runtime defaults. The previous version assumed `-listen 0.0.0.0:8080` for breedos; now an empty `--args` means empty args and the binary uses its own defaults. The systemd unit `Description=` is generic (`<service> service`) by default, overridable via `--description`. The `Documentation=` field is no longer hard-coded to the breedos repository URL.
- Updated README "Run as a systemd service" section: now references `mvp/build.sh`, shows both interactive and non-interactive install invocations, and documents the empty-args / binary-defaults convention.
- Updated MVP version strings (landing footer, demo kicker, run notes) from `v0.6.4` to `v0.6.5`.

## [0.6.4] - 2026-05-17

### Added
- `install.sh` — systemd-service installer for the BreedOS MVP binary. Generic enough to manage any Go binary placed next to it, but tuned for breedos defaults (binds `0.0.0.0:8080`, working directory = the script's directory). Supports subcommands `install` / `uninstall` / `info` / `help`. Interactive prompts for arguments, run user, and working directory. Prints status/log/control commands after install and starts the service.
- README section "Run as a systemd service" with end-to-end deployment recipe.

### Changed
- Updated MVP version strings (landing footer, demo kicker, run notes) from `v0.6.3` to `v0.6.4`.

## [0.6.3] - 2026-05-17

### Changed
- Landing page (`mvp/static/index.html`) refined for public release at the canonical domain:
  - Title and meta description rewritten to match the canonical one-liner ("decision engine for selection strategies in CRISPR-enabled crop breeding").
  - Dropped the "AI" prefix from the hero kicker for consistency with README, CHANGELOG, and the public pack.
  - Replaced the secondary hero CTA "Read the pitch" with "View on GitHub" pointing directly to the source repository.
  - Genomic Selection Planner module flagged as roadmap; description softened to acknowledge that production integration is scheduled for the program build (MVP currently demonstrates the kernel only).
  - CRISPR section heading softened: "CRISPR gives breeders a limited but powerful write function" (was "biology a write function") and the body adds an explicit non-replacement framing against Benchling, Synthego, and CRISPResso.
  - Footer adds a direct "View source on GitHub" link.
- Updated MVP version strings (landing footer, demo kicker, run notes) from `v0.6.2` to `v0.6.3`.

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
