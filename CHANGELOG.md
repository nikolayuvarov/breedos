# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.7.8] - 2026-05-22

### Fixed — Demo-grid width / top hero narrower than bottom

Top hero cards (honesty banner, "Selection Strategy Simulator" title) rendered visibly narrower than the bottom two-column `.demo-grid`. Root cause: the `1fr` right column of `.demo-grid` was being forced to its min-content size by the embedded 900px-wide canvases (chart_gain, chart_pareto, the new chart_histogram). Without `min-width: 0` on grid items, CSS Grid expands `1fr` past its parent's width to accommodate min-content. The result: `.demo-grid` was ~30 px wider than `.wrap`.

Fix: `.demo-grid > * { min-width: 0; }` and an explicit `min-width: 0` on `.results`. Both columns now honor the parent `.wrap` width; the demo-grid and the top hero cards line up.

### Fixed — Live histogram delay at start + jitter at end

Three small fixes addressing the user-reported delay + twitching:

- **Initial generation-0 snapshot.** The backend emits a snapshot of the founder population *before* the worker pool starts running generations, so the live histogram has content the instant the client begins polling — no perceived startup delay waiting for generation 1 to finish.
- **Stable Y-axis.** `drawLiveHistogram` now anchors the Y-axis to the total marker count (sum of bins, constant within a run) instead of the dynamic per-snapshot max. Bars represent the fraction of markers in each frequency bin and the scale never rescales — no visible "jumping" as alleles drift toward fixation in late generations.
- **Dedup'd redraws.** The polling loop now keeps a `lastDrawnGeneration` counter and only redraws when the snapshot's generation actually advances. Repeated polls reading the same final snapshot no longer trigger `clearRect` + redraw, which was causing subtle jitter near run end.
- **Snappier polling (120 ms → 80 ms).** Tighter loop interval for crisper per-generation updates. The status endpoint is cheap; no meaningful extra server load.

### Added — Tracked-strategy picker for the live histogram

New `tracked_strategy` field on `SimRequest` (string; default `""` / `"auto"` = prefer "balanced" else first configured strategy). When set to a strategy code (e.g. `"aggressive"`, `"ocs_like"`), the live AFS histogram tracks that strategy instead of the default. If the selected code isn't part of the current strategy set (e.g. an advanced-only code while running `strategy_set: "core"`), the server falls back to auto behaviour — no error.

Demo form gains a "Live histogram tracks" `<select>` below the strategy-set dropdown with all 11 strategy codes plus "auto". `requestSignature` and `changedParams` include `tracked_strategy` so cache invalidation works when the operator changes only the picked strategy.

### Changed
- `runSimulation` and `runSimulationWithProgress` still wrap `runSimulationWithCallbacks` (unchanged signatures).
- Version strings bumped `v0.7.7` → `v0.7.8` in `main.go` run notes, all four landing footers, and the demo kicker.

### Not changed
- Wheat fetch script unchanged in this release. (The CerealsDB endpoint returned a 503 through the proxy during one fetch attempt; retry after CerealsDB is healthy.)
- Histogram concurrency model (single tracked strategy, single replicate, mutex-protected snapshot writes) unchanged.

## [0.7.7] - 2026-05-22

### Fixed — Misleading language switcher on demo

The language switcher previously appeared in the demo nav too, but clicking RU/ES/UZ on demo threw the user to the localized landing — not to a translated demo (the demo is intentionally English-only, one source of truth for the technical surface). The user reasonably read this as a broken promise.

Fixes:
- Language switcher removed from `demo.html` nav. The demo now shows only the "Landing" link back to /. Users who want to switch language do so from the landing page.
- Each localized landing (`index-ru.html`, `index-es.html`, `index-uz.html`) gains a small italic `.lang-note` directly under the demo CTA stating that the demo and Decision Report are English. Removes the surprise of clicking "Запустить демо" on the Russian landing and arriving on an English page.

### Changed
- New `.lang-note` style in `style.css` — subtle muted italic note, max-width 640 px.
- Version strings bumped v0.7.6 → v0.7.7 in `main.go` run notes, all four landing footers, and the demo kicker.

### Not changed
- The landing-page language switcher remains exactly as in v0.7.6 — that one IS honest (clicking RU on the English landing takes you to the actual Russian landing).
- Histogram, wheat fetcher, dataset loader, constraint engine, honesty layer, self-update — all unchanged.

## [0.7.6] - 2026-05-22

### Added — Live allele-frequency-spectrum (AFS) histogram

While a simulation runs, the demo now shows a small live histogram below the Pareto chart that updates per generation. It bucketed the current allele frequencies of ONE tracked strategy (preferring `balanced`; otherwise the first strategy in the run) into 10 bins of width 0.1 and displays them as a bar chart. The final snapshot stays visible after the run completes.

Backend additions (`mvp/main.go`):
- `AFSSnapshot` struct with `generation`, `total_generations`, `strategy_code`, `strategy_name`, and `[10]int` bins.
- `snapshotFunc` callback type threaded through `runSimulationWithCallbacks` (new entry point; `runSimulation` and `runSimulationWithProgress` are preserved as thin wrappers so all existing tests pass).
- `simJob.LatestSnapshot` + `SimJobStatus.latest_snapshot` (JSON, `omitempty` for nullable client-side).
- `afsBinsFromPop` helper next to `alleleFreq`.

The decision engine picks ONE tracked strategy and only its replicate 0 emits snapshots — this avoids concurrent writes from parallel workers entirely. Snapshot writes still go through the existing `simJobStore.Mutex` for defense-in-depth. Verified with `go test -race ./...` (clean).

Frontend (`app.js`):
- `lastSnapshot` state, `drawLiveHistogram(snap)`, `resetLiveHistogram()`.
- The existing 120 ms `/api/simulate/status` polling loop now consumes `job.latest_snapshot` and redraws the canvas on each poll. No new polling cadence.
- Bar colour reuses the strategy's color from the existing `colors` map.

UI:
- New `.histogram-card` between the Pareto chart and CRISPR card (`demo.html`).
- Heading "Live allele-frequency spectrum — &lt;strategy&gt; generation N/M", ~800 × 160 canvas, explanatory note.
- `.histogram-card` and `.histogram-label` styles in `style.css`.

### Added — Wheat data fetcher (`tools/data/fetch_wheat_t3.py`)

Standalone Python 3 script (no external dependencies) that downloads a public wheat genotype subset and writes a BreedOS founder-CSV. Output dataset name: `wheat_t3`.

Default source: **CerealsDB 35K Wheat Breeders' Array** (University of Bristol — fully public, no auth required). Auto-detects ZIP / CSV / TSV / VCF (gzip-aware). Defaults to `--n 500 --m 5000 --maf 0.05`; output ~5 MB.

Supports `--source <url>` override so the operator can point it at a manually-downloaded T3/Wheat VCF (T3 requires a free account for the genotype-download endpoint) or the Watkins 12.7× WGS VCF (Cheng et al. 2024).

Hexaploidy is handled by treating per-locus diploid calls (AA / AB / BB from arrays, or 0/0 / 0/1 / 1/1 from VCF callers) as 0/1/2 dosage — documented in the script header and in the output CSV `# Ploidy note:` block.

Demo dropdown extended with `Wheat (T3 / CerealsDB)` option. As with Arabidopsis, the operator runs the fetcher locally, then `deploy_breedos.sh` uploads the resulting CSV to the server alongside the binary (size-skip).

### Added — Marketing localization (Russian, Spanish B1, Uzbek)

Three new landing pages alongside English:

- `/ru` → `index-ru.html` — Russian (native register, professional but not academic).
- `/es` → `index-es.html` — Spanish at **CEFR B1** level: simple tenses (presente, pretérito, pretérito perfecto, futuro simple), short sentences (≤ 20 words), common vocabulary, no subjunctive where avoidable.
- `/uz` → `index-uz.html` — Uzbek (Latin script, standard since 2018). Recommend founder review for terminology choices (`naslchilik` vs `seleksiya`, `belgi` vs `xususiyat`, etc.).

The demo and the Decision Report stay English (technical content; one source of truth). The nav on every page (landing + demo) has a four-way language switcher (`EN · RU · ES · UZ`) with `.active` styling on the current language.

Go routes added in `main.go`: `/ru`, `/es`, `/uz` serve the respective HTML; everything else routes unchanged.

CSS: new `.lang-switcher` block in `style.css` (subtle separator, hover, active highlight in accent colour).

### Changed
- Version strings bumped `v0.7.5` → `v0.7.6` in `main.go` run notes, all four landing footers (`index.html`, `index-ru.html`, `index-es.html`, `index-uz.html`), and the demo kicker.
- `runSimulation` and `runSimulationWithProgress` are now thin wrappers around the new `runSimulationWithCallbacks` (which also accepts a `snapshotFunc`). API and existing test signatures unchanged.

### Followup ideas (deferred)
- A select control above the live histogram letting the user pick which strategy to track. Today the choice is fixed at run-start (prefer `balanced`).
- Detect language preference via `Accept-Language` or cookie and redirect `/` accordingly; today users land on English and switch manually.
- Founder review of the Uzbek translation for terminology choices.

## [0.7.5] - 2026-05-22

### Changed — External real-data CSVs (not embedded in binary)

Large founder-data CSVs (Arabidopsis 1001 and any future maize / wheat panels) are no longer embedded into the binary via `//go:embed`. They live as separate files on the server alongside the binary in `<bindir>/data/<name>.csv`. The binary stays small (~6.5 MB); the data deploys independently.

`mvp/dataset.go` lookup order for a `dataset=<name>` request:

1. `<bindir>/data/<name>.csv` (external — operator-deployed).
2. Embedded `data/<name>.csv` (only if explicitly added to the `//go:embed` directive — currently only the placeholder is included).
3. Embedded `data/example_founders.csv` (placeholder fallback so the dropdown still works on fresh installs).

Parsed datasets are cached in a package-level map (`datasetCache`); repeat requests within the same binary instance reuse the parsed matrix instead of re-reading and re-parsing the file. A 100 MB CSV would otherwise cost ~1 s of parse time per simulation; with the cache, only the first request pays.

The `//go:embed` directive is narrowed from `data/*.csv` to `data/example_founders.csv` so that adding new CSVs to `mvp/data/` locally does NOT bloat the compiled binary.

### Changed — `.gitignore`

`mvp/data/*.csv` is now gitignored, with `mvp/data/example_founders.csv` whitelisted. Large CSVs produced by `tools/data/fetch_*.py` stay local; only the placeholder ships in git.

### Changed — `deploy_breedos.sh`

Before uploading the binary, the deploy script now uploads external data files conditionally:

- Iterates `mvp/data/*.csv` and selects files that are gitignored (i.e., external).
- Creates `<bindir>/data/` on the remote via SSH.
- For each external file, compares local size with remote size (`ssh stat -c %s`).
- Uploads via `scp` only when remote is missing or sizes differ.
- Skips upload when remote and local sizes match (byte-exact).

This keeps repeat deploys fast: the 100 MB Arabidopsis CSV is uploaded once and skipped on every subsequent deploy unless it changes.

### Added — Test for external-precedence

`TestExternalDatasetTakesPrecedenceOverEmbedded` writes a CSV into `<testbin-dir>/data/arabidopsis1001.csv` and verifies that `loadDataset` reads the external file (not the embedded placeholder), sets `ds.external = true`, and clears the placeholder flag.

### Changed — Notes language

`buildNotes` now distinguishes three cases when reporting the founder-population source:

- Placeholder fixture (warning).
- External real-data file (`external = true`).
- Embedded real-data file (only possible if a CSV is explicitly bundled — rare).

### Changed
- Version strings bumped `v0.7.4` → `v0.7.5` in `main.go` run notes, `index.html` footer, and `demo.html` kicker.
- Run-notes top description rewritten to mention the external-data deploy semantics.

### Migration notes
- Existing deployments with v0.7.4 had the (small) `arabidopsis1001.csv` embedded if the operator ran the fetcher before building. After upgrading to v0.7.5: on first run, the loader will NOT find the embedded data (new embed pattern), will look for `<bindir>/data/arabidopsis1001.csv`, and if missing, fall back to the placeholder. The deploy script handles this transition: it uploads the local CSV (gitignored) to `<bindir>/data/` before swapping the binary.
- Repos that imported `mvp/data/<some-csv>` into git inadvertently: the file will become gitignored. Remove from index with `git rm --cached`.

## [0.7.4] - 2026-05-22

### Added — Real-data founder population loader (closes `issues-breedos/04`)

The simulator can now load real founder genotypes from an embedded CSV instead of generating a synthetic population. The on-ramp for the Arabidopsis 1001 Genomes Project is the first wired-up dataset; the same loader works for any matrix in the BreedOS founder-CSV format.

`SimRequest` adds:

- `dataset` (string) — `"synthetic"` (default, current behaviour) or `"arabidopsis1001"` (loads from embedded CSV).

New module `mvp/dataset.go`:

- `loadDataset(name)` reads the matching `data/<name>.csv` via `//go:embed`, falling back to `data/example_founders.csv` so the dropdown still works even before the user runs the fetch script. Comment lines starting with `#` are ignored except for `# placeholder: true`, which marks the file as the non-real placeholder.
- `parseDatasetCSV` performs strict 0/1/2 validation; out-of-range values fail loudly.
- `subsampleDataset(ds, n, m, rng)` samples up to `n` accessions and the first `m` markers; called from `runSimulationWithProgress` so the existing `population_size` / `markers` knobs continue to scope the run.

When a dataset is selected, the simulator:

- Replaces the generated founder population with the loaded one.
- Overrides `req.PopulationSize` and `req.Markers` to match the loaded matrix.
- Adds a run note that either announces the real data source ("Founder population loaded from real-data file ...") or warns clearly that the placeholder is in use ("⚠ Dataset 'arabidopsis1001' is the embedded PLACEHOLDER fixture — run tools/data/fetch_arabidopsis_1001.py and rebuild ...").
- Updates the honesty banner, `key_assumptions`, and `limitations` to acknowledge real founders while making clear that selection / recombination / mutation in subsequent generations remain synthetic.

### Added — Python fetch script (`tools/data/fetch_arabidopsis_1001.py`)

A standalone Python 3 script (no external dependencies) that:

- Streams the 1001 Genomes v3.1 VCF (gzip-aware, HTTP or local file).
- Filters to biallelic SNPs with `--maf` ≥ 0.05 and `--max-missing` ≤ 0.10.
- Samples `--n` accessions and `--m` markers deterministically (`--seed`).
- Imputes any remaining missing genotypes at sampled sites with mean-dose rounding.
- Writes a BreedOS founder-CSV to `breedos/mvp/data/arabidopsis1001.csv`.

The Go loader picks up the new CSV on the next binary rebuild (`./mvp/build.sh ...`). See `tools/data/README.md` for the workflow.

### Added — Placeholder fixture

`breedos/mvp/data/example_founders.csv` ships a 60 × 200 synthetic-but-real-format matrix so that the dataset dropdown is exercised by tests and the live demo without requiring the user to run the fetch script first. The file is marked `# placeholder: true` and the simulator emits a prominent warning note when it's in use.

### Added — Dataset dropdown in demo

`demo.html` exposes a "Founder population" dropdown at the top of the simulation-inputs form with two options: `Synthetic (generated)` (default) and `Arabidopsis 1001 (subset)`. The selection is passed in the `dataset` request field; `requestSignature` and `changedParams` include it so cache invalidation behaves correctly when the operator switches data sources.

### Added — Tests

`mvp/main_test.go` adds four tests:

- `TestParseDatasetCSVRoundTrip` — parses a tiny 3 × 3 CSV with comments and the `placeholder: true` marker; verifies parsed values, accession IDs, and the placeholder flag.
- `TestParseDatasetCSVRejectsOutOfRange` — guarantees that a value outside `0..2` causes a parse error.
- `TestLoadDatasetFallsBackToPlaceholder` — verifies the loader fallback path when the requested dataset file doesn't exist.
- `TestDatasetRoutedThroughSimulation` — runs an end-to-end simulation with `dataset: "arabidopsis1001"` (which falls back to the placeholder) and confirms the placeholder warning appears in the response notes.

### Changed
- Version strings bumped `v0.7.3` → `v0.7.4` in `main.go` run notes, `index.html` footer, and `demo.html` kicker.
- `buildNotes` signature now accepts an optional `*loadedDataset` so the dataset-related warning/announcement can be emitted alongside the existing notes.

### Not in this release
- The simulator does NOT ship real Arabidopsis 1001 Genomes data — that data is large and the user fetches it locally. The binary ships with the placeholder fixture so all code paths and the UI dropdown are exercisable on first install. To use real data: run `tools/data/fetch_arabidopsis_1001.py`, then rebuild.
- No streaming visualisation — that lands separately in v0.7.5.
- No new selection strategies; constraint engine (v0.7.3), honesty layer (v0.7.2), and self-update mechanism (v0.7.1) unchanged.

## [0.7.3] - 2026-05-21

### Added — Constraint engine and feasible-strategy selection (closes `issues-breedos/03`)

The simulator now evaluates each strategy against user-supplied program constraints and surfaces a separate "best feasible" recommendation in the decision report.

`SimRequest` adds six optional constraint fields (zero = no constraint):

- `max_inbreeding` — hard cap on mean final inbreeding coefficient.
- `max_diversity_loss` — hard cap on diversity loss as a fraction of baseline (e.g., 0.30 = lose at most 30%).
- `max_rare_useful_loss` — hard cap on count of rare-useful loci lost.
- `min_genetic_gain` — floor on final mean genetic gain.
- `min_effective_parents` — floor on effective parent count.
- `max_combined_risk` — hard cap on the combined risk score (weighted inbreeding-breach / diversity-collapse / rare-allele-loss probabilities).

`FinalStats` per strategy adds:

- `feasible` (bool) — true if all active constraints pass for that strategy's mean outcome.
- `failed_constraints` (string slice) — human-readable list of which constraints were violated (e.g., `"inbreeding 0.3812 > max 0.2500"`).

`DecisionSummary` adds:

- `best_feasible_code` / `best_feasible_name` — top risk-adjusted strategy among feasible ones (empty when none feasible or no constraints applied).
- `feasibility_note` — explanation that either names the best feasible strategy or identifies the most-binding constraint when nothing passes.
- `constraints_applied` — human-readable list of the constraints that were evaluated for this run (e.g., `"max inbreeding ≤ 0.2500"`).

When no constraints are supplied, behaviour is identical to v0.7.2 — every strategy is treated as feasible and the decision report explicitly notes "No hard constraints supplied".

### Added — Constraint inputs in demo

`demo.html` exposes a new collapsible "Program constraints" form section with the six fields. Each input defaults to 0 (no constraint). Numeric inputs validate as non-negative.

### Added — Feasibility in decision report and strategy table

`renderDecisionPanel` (`app.js`) now renders:

- A "Best feasible" card alongside "Recommended" / "Max gain" / "Lowest risk".
- A `feasibility-note` block summarising how many strategies passed and the most-binding constraint when none did.
- A `constraints-applied` chip row showing the list of evaluated constraints.

`renderStrategyTable` (`app.js`) adds a feasibility column showing ✓ / ✗ plus a tooltip listing the failed constraints. Infeasible rows are visually de-emphasised so the user reads feasible options first.

### Added — Tests for the constraint engine

`mvp/main_test.go` adds three test cases:

- `TestConstraintEngineFeasibleStrategyExists` — sets a permissive `min_genetic_gain` floor and verifies at least one strategy is feasible and `BestFeasibleCode` is populated.
- `TestConstraintEngineNoFeasibleStrategy` — sets an impossibly tight `max_inbreeding` and verifies `BestFeasibleCode` is empty and `FeasibilityNote` references the binding constraint.
- `TestConstraintEngineAggressiveRejectedByRiskCap` — sets a tight `max_combined_risk` that aggressive selection cannot meet; verifies aggressive ends in `failed_constraints` while a more balanced strategy passes.

### Changed
- `buildSummaryText` now mentions feasibility status when constraints are active.
- `Interpretation` list in DecisionSummary explicitly states whether constraints were supplied and which strategy is best feasible.
- Run-notes mention v0.7.3 in the budget-guard line and main feature description.
- Version strings bumped `v0.7.2` → `v0.7.3` in `main.go` run notes, `index.html` footer, and `demo.html` kicker.

### Not in this release
- No new selection strategies (out of scope per the issue).
- No full mathematical optimisation (per the issue's non-goals).
- Honesty layer (v0.7.2) and self-update mechanism (v0.7.1) unchanged.

## [0.7.2] - 2026-05-21

### Added — Scientific Honesty / Trust Layer (closes `issues-breedos/06`)

The Decision Report now includes three honesty-oriented fields and the demo carries a visible banner so domain users and reviewers can see the scope and limits of the simulator at a glance.

`DecisionSummary` response object adds:

- `honesty_banner` — one-line banner: "Decision-layer simulator on synthetic data — minimal CRISPR demo (when enabled) — not a deployable recommendation without your own genotype/phenotype data and domain review."
- `limitations` — explicit modelling limits: diploid biallelic markers, simplified inheritance, additive trait architecture (no dominance / epistasis / pleiotropy), no GxE, mock genomic-prediction signal, no real germplasm or pedigree ingested, user-set risk thresholds. CRISPR-mode adds explicit "not guide-RNA design / not off-target scoring / not regulatory feasibility" lines.
- `what_could_be_wrong` — context-aware list of scenarios that would invalidate the recommendation: risk-adjusted-vs-max-gain disagreement under a different inbreeding tolerance, low replicate count, small population, mis-estimated heritability, non-additive trait architecture, population substructure, infeasible selection intensity, CRISPR off-target / regulatory hurdles.

### Added — Demo honesty banner

`demo.html` carries a visible `.honesty-banner` above the title card that states the simulator's scope ("Decision-layer simulator on synthetic data. Not a wet-lab protocol, not a guide-RNA designer, not a deployable recommendation without your own genotype/phenotype data and domain review.") and points readers to the new Decision Report sections.

### Added — Decision Report sections

`renderDecisionPanel` in `app.js` now renders two new `<details>` blocks in the Decision Report:

- **What could make this recommendation wrong?** — open by default; orange accent (`decision-what-could-be-wrong` class).
- **Model limitations** — collapsed by default; blue accent (`decision-limitations` class).

A small inline honesty banner is also rendered at the top of the Decision Report panel itself (`decision-honesty-banner` class), so users who jump straight to the report still see the scope statement.

### Changed
- `buildNotes` in `main.go` now reflects v0.7.2 in the run-notes string and budget-guard note.
- Version strings bumped `v0.7.1` → `v0.7.2` in `main.go` run notes, landing footer (`index.html`), and demo kicker (`demo.html`).

### Operational changes (from previous Unreleased)
- `deploy_breedos.sh` now reads its defaults from a local `.env` file next to the script (gitignored). A new tracked `.env.example` documents the format. Override precedence: positional `$1` > `BREEDOS_DEPLOY_TARGET` env var (current shell or `.env`). When no target is configured and no `.env` exists, the script prints actionable instructions for creating one and exits non-zero. Help text is shown on `-h` / `--help` / `help`.

### Not in this release
- No algorithm changes (selection, simulation, Pareto, risk, self-update contract all unchanged from v0.7.1).
- No API breaking changes — the three new `DecisionSummary` fields are additive.
- Domain-expert review (issues-breedos/11) and constraint engine (issues-breedos/03) remain open.

## [0.7.1] - 2026-05-21

### Added — Self-update module (`mvp/selfupdate.go`)

The running binary now watches for a sibling file named `<binary>.UPDATE`. When the file appears, the running process:

1. Ensures the candidate is executable (chmods if needed).
2. Runs the candidate with `--self-check` and verifies it prints the literal token `OK` on stdout. If not, the candidate is left in place for inspection and the watcher continues polling — the running service does NOT go down.
3. Renames the running binary to `<binary>.bak.<YYYYMMDDHHmmss>`.
4. Renames the candidate into the running binary's name.
5. Exits with non-zero so that systemd (`Restart=on-failure` in the unit file generated by `install.sh`) restarts the now-new binary.

All steps fail loudly to journal/stderr and either skip the swap or attempt a rollback. The candidate `.UPDATE` file is left in place when self-check or swap fails so that a human can inspect.

### Added — `--self-check` flag

`main.go` accepts `--self-check` (or `-self-check`); the binary prints `OK` and exits 0. This is the contract the self-update watcher uses to validate any candidate binary.

### Added — `deploy_breedos.sh`

Build a portable static binary via `mvp/build.sh`, verify it locally with `--self-check`, then `scp` to the remote host as `<binary>.UPDATE`. Target is parameter or `BREEDOS_DEPLOY_TARGET` env var. The running service on the remote host detects the `.UPDATE` file within ~60 s and performs the swap.

### Added — tests

- `TestPerformSwapRenamesBinaryAndCreatesBackup` — exercises the rename/swap logic in a temp directory; verifies the running path holds the update content, the `.UPDATE` file is gone, and a backup file matching `breedos.bak.*` contains the previous content.
- `TestEnsureExecutableSetsExecBit` — verifies the helper sets the owner execute bit on a non-executable file.

### Changed
- `main.go` adds the `os` package import, parses the `--self-check` flag (before any work), and starts the self-update watcher before `ListenAndServe`.
- Watcher polling interval defaults to 60 seconds; self-check command timeout is 10 seconds. Both are constants in `mvp/selfupdate.go`.
- Version strings bumped `v0.7.0` → `v0.7.1` in main.go run notes, landing footer, and demo kicker.

### Operational note
The very first deployment to a host running v0.7.0 or earlier must be done manually (the older binary does not know about the `.UPDATE` contract). Subsequent deploys to a v0.7.1+ host can use `deploy_breedos.sh`.

### Out of scope
- Signed/verified updates — the contract here is `--self-check` returns `OK`; that is *not* cryptographic verification. For a hostile environment, future work should add signed binaries and signature verification.
- Rolling deploys across multiple hosts — `deploy_breedos.sh` ships to one target per invocation.

## [0.7.0] - 2026-05-21

### Added — Decision Report (closes `issues-breedos/02`)

`DecisionSummary` response object now includes six new structured fields:

- `tradeoffs` — up to three pairwise observations comparing top strategies on gain vs. risk, risk-adjusted vs. min-risk, and Pareto/aggressive-vs-diversity. Each entry has `a`, `b`, `theme`, and `note`.
- `avoid_strategies` — strategies with combined risk ≥ 0.5 that are *not* Pareto-optimal. Each entry lists which risk-probability thresholds were breached.
- `key_assumptions` — explicit list of modelling assumptions this run depends on (synthetic population, mock genomic-selection signal, additive trait architecture, heritability, selection percent, risk thresholds, CRISPR seed parameters when enabled).
- `missing_data_warnings` — dynamic flags for replicates < 5, population < 50 or < 10, extreme heritability, mutation_rate = 0, short horizon, low baseline diversity.
- `next_analysis` — heuristic suggestion for the next experiment (vary seed, tighten constraints, raise replicates, sweep selection intensity, etc.) depending on the shape of this run's outcome.
- `summary_text` — single paragraph copy-pasteable export of the recommendation, max-gain / lowest-risk strategies, top trade-off, avoid list, first caveat, and next analysis.

Frontend:

- `renderDecisionPanel` now renders the new fields as collapsible `<details>` sections (Top trade-offs and Strategies to avoid open by default; Missing-data warnings shown with warning styling; Key assumptions collapsed by default).
- "Recommended next analysis" callout block prominently displayed.
- "Copy summary" button now prefers `decision.summary_text` from the server, falling back to the client-side `buildSummaryText` for older responses.
- `style.css` adds `.decision-section`, `.decision-next`, `.decision-warnings` styles.

Tests:

- `TestDecisionReportPopulatesNewFields` — verifies all six new fields are populated for a basic run.
- `TestDecisionReportFlagsHighRiskAggressiveOrIncludesItInTradeoff` — verifies aggressive strategy appears in AvoidStrategies or Tradeoffs in a small-N high-pressure scenario.
- `TestDecisionReportSummaryReferencesRecommendedStrategy` — verifies `summary_text` references the recommended strategy by name and `next_analysis` matches one of the expected heuristic keywords.

### Changed
- `buildDecisionSummary` signature changed to take `(req SimRequest, results []StrategyResult, baseDiversity float64)` (was `(results []StrategyResult)`). Single internal call site updated.
- Version strings bumped `v0.6.7` → `v0.7.0` in main.go run notes, landing footer, and demo kicker.

### Non-goals (deferred)
- Best-feasible-strategy field — deferred to v0.7.1 alongside the constraint engine in `issues-breedos/03`.
- Tighter assumption/missing-data integration with a "scientific honesty" trust layer — deferred to v0.7.2 with `issues-breedos/06`.
- PDF export and LLM-generated explanation — out of scope per the issue.

## [0.6.7] - 2026-05-21

### Fixed
- Removed two residual references to the deleted `customer-discovery` page that v0.6.6 missed:
  - `mvp/static/index.html` line 138: the secondary CTA button "Open customer discovery page" linking to `/customer-discovery` (would have 404'd on the live deployment). Replaced with a "View on GitHub" link.
  - `README.md` project-structure tree: removed the `customer_discovery.html` entry from the static/ listing.

### Changed
- Updated MVP version strings (landing footer, demo kicker, run notes) from `v0.6.6` to `v0.6.7`.

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
