# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.7.18] - 2026-05-24

### Added — EU NGT regulatory classification layer (Issues 13–16)

The EU NGT Regulation (Council adopted 2026-04-21, applies from mid-2028)
splits gene-edited plants into two categories with very different downstream
costs. BreedOS now classifies every planned edit set in real time so the
operator sees the category implication *before* committing to a CRISPR
strategy.

**Issue 13 — classification engine** (`mvp/ngt_classify.go`, ~170 lines + 13 tests):
classifies under the **20/20 rule** (max 20 modifications, each insertion ≤ 20 bp)
plus auto-exclusions (`herbicide_tolerance` and `insecticidal` trait classes
disqualify NGT-1) and donor-source rules (`cross_species` introduces foreign
DNA → disqualifies). Output: `NGT-1` | `NGT-2` | `unclassifiable` (when
inputs are missing). Every result carries a verbatim "Not legal advice"
disclaimer.

**Issue 14 — candidate-edit badge**: every row in the CRISPR edit candidates
table now ends with a colour-coded badge (green NGT-1, orange NGT-2, grey
unclassifiable). Hover/focus tooltip shows reasons, disqualifiers, and the
disclaimer.

**Issue 15 — Regulatory card in the Decision Report area**: a new card
above the candidate-edits table renders the category headline, a list of
reasons, disqualifiers (if any), one paragraph of downstream implications
(registration path, labelling, traceability), and the disclaimer.

**Issue 16 — patent / licensing declaration fields**: optional run-level
fields (`patent_id`, `licensing_status`, `notes`) collapsible inside the
CRISPR form section. When the run is classified NGT-1 *and* `patent_id` is
empty, the Regulatory card surfaces a warning that NGT-1 registration will
require an explicit patent declaration. All fields round-trip into the JSON
export so they propagate to a registration filing.

Schema changes (additive only):

- `SimRequest.NGT` (optional `NGTContext` with `target_trait_class`,
  `donor_source`, `patent_id`, `licensing_status`, `notes`).
- `DecisionSummary.NGT` (optional `*NGTClassification` with `category`,
  `reasons`, `disqualifiers`, `confidence_note`).

Backward compatibility: runs without `req.NGT` produce the existing API
shape unchanged (omitempty); UI hides the Regulatory card and shows a
"—" placeholder badge.

### Changed
- Version strings bumped `v0.7.17` → `v0.7.18` across `main.go`, four landing footers, demo kicker, datasets-page kicker.

## [0.7.17] - 2026-05-24

### Fixed — Sensitivity sweep "looked frozen" on slow prod

v0.7.16 passed a no-op progress callback into the inner simulation, so the
percent bar in the sensitivity sweep panel sat at one value (e.g. 0%, then
20%, then 40%, ...) for the entire duration of one scenario. On prod
(single-core VPS, ~15–30 s per scenario) this read as "frozen" to the
operator who reported the issue with a screenshot during a live sweep.

**Fix:**

- Propagate the inner-simulation progress callback into the sweep so each
  scenario fills its own 20% slice continuously. Outer percent =
  `(scenario_index * 100 + inner_percent) / num_scenarios`.
- Move `Percent` from `int` to `float64` in `SensitivityJobStatus` and the
  in-memory job store so the displayed value can be e.g. `27.4%` instead of
  rounding away the sub-scenario progress.
- Frontend formats one decimal place mid-run (`27.4%`), rounds to integer
  on completion (`100%`).
- Inner runner's own message string (`parallel run: 334/600 strategy-
  generations complete; ...`) is appended in parentheses so the operator
  sees that work is happening inside the scenario, not just at boundaries.

### Changed
- Version strings bumped `v0.7.16` → `v0.7.17` across `main.go`, four landing footers, demo kicker, datasets-page kicker.

## [0.7.16] - 2026-05-23

### Added — Sensitivity sweep (Issue 09)

Runs the same configuration across up to 5 values of one axis (heritability, selection percent, or generations horizon) and compares the best-feasible strategy across scenarios. The recommendation gets a verdict:

- **stable** — same strategy wins in every scenario; recommendation is robust to changes in that axis within the sampled range.
- **fragile** — strategy switches in at least one scenario; the single-run recommendation may not hold if the axis differs from the baseline.
- **inconclusive** — all scenarios infeasible; loosen constraints.

The UI lives under the Decision engine output: axis dropdown, comma-separated values (with sensible defaults per axis), Run sweep button, sweep-budget meter, results table (gain / diversity / inbreeding / combined risk / match-to-baseline per scenario), and the verdict banner.

API:
- `POST /api/sensitivity/start` → `{job_id}`
- `GET /api/sensitivity/status?id=X` → `{percent, message, done, result?}`

Budget cap: the **sum** of per-scenario budgets must be ≤ 1.5B cells (same cap as a single run, applied to the sweep total). Client-side pre-flight blocks oversized sweeps before submit. Server-side validation runs every scenario through `validateRequest` upfront so axis-specific range violations (e.g. h² outside [0,1]) get a per-scenario error message instead of a mid-sweep failure.

Backend in `mvp/sensitivity.go` (~250 lines). Tests in `mvp/sensitivity_test.go` cover validation rejects, baseline-nearest indexing, stable/fragile/inconclusive verdict logic, and the `BestFeasibleCode → BestRiskAdjustedCode` fallback when no constraints are applied.

### Changed
- Version strings bumped `v0.7.15` → `v0.7.16` across `main.go`, four landing footers, demo kicker, datasets-page kicker.

## [0.7.15] - 2026-05-22

### Added — Live budget meter under Run

`mvp/static/demo.html` shows a budget meter directly below the Run button. It prints the current run budget in cells (`N × markers × (generations+1) × strategies × replicates`), the formula breakdown, and the cap. The meter is reactive — updates on every keystroke via an `input` listener, not just on `change` — so the user sees the impact of typing as it happens.

States:
- **OK** (< 70% of cap): muted text, green budget number, Run enabled.
- **Warn** (70–100% of cap): warn-yellow budget number, Run still enabled.
- **Over** (> cap): red budget number, Run button disabled, and the four numeric inputs that multiply into the budget (`population_size`, `markers`, `generations`, `replicates`) get a red `over-budget` outline so the user can immediately see which knobs to turn down.

Pre-flight check in `runSimulation()` short-circuits over-budget submits so the request never leaves the browser — previously the only signal was a generic 400 after submit, which was easy to mistake for stuck client state when reducing one parameter still left the run over cap.

### Changed — Budget cap raised 800M → 1.5B

`validateRequest()` in `main.go` now caps `budget` at 1,500,000,000 (was 800,000,000). Rationale: 800M was set in the initial v0.6 commit without recorded justification; benchmarks show wall-clock cost is ~2.2–2.7 ns per cell on dev (multi-core), and the prod box is a single-core VPS. Raising to 1.5B keeps the upper-bound run at ~30–60 s on prod — long but tolerable for a public demo. The JS `BUDGET_CAP` constant in `app.js` is kept in sync.

### Changed
- Version strings bumped `v0.7.14` → `v0.7.15` across `main.go`, all four landing footers, demo kicker, and datasets-page kicker.

## [0.7.14] - 2026-05-22

### Fixed — Live histogram stuck on gen-0 (real root cause)

The v0.7.13 snapshot queue + playback shipped the right architecture but the user reported the chart still stuck on the first generation. DevTools network log showed the cursor frozen at `?since=1` for 50+ polls.

**Root cause (this time for real):** the worker pool enqueued tasks in `[strategy_index][replicate]` order. The tracked task (default "balanced") sat at queue position `trackIdx × replicates` (= 10 in the default core+CRISPR set), so on a slow production host the tracked task didn't START for 10-15 seconds while other strategies ran. The frontend got the gen-0 snapshot (emitted before the worker pool starts) and nothing else until then.

**Fix:** enqueue the tracked task FIRST. A worker grabs it immediately, snapshots start arriving on poll #1. The rest of the queue is enqueued in the usual order, skipping the already-queued tracked entry. Combined with the v0.7.13 client-side queue + playback, the user now sees the chart animate from poll #1 onwards even when total run time is 15+ seconds.

### Added — Favicon

`mvp/static/favicon.ico` — 32×32 32bpp ICO with a stylised mint-green "B" on a dark teal background, matching the landing-page accent palette. Generated by `tools/data/make_favicon.py` (stdlib-only Python, no PIL/cairo). All six HTML pages now reference it via `<link rel="icon" type="image/x-icon" href="/favicon.ico">`; the Go static handler emits `Content-Type: image/x-icon` for `.ico` paths.

### Changed
- Version strings bumped `v0.7.13` → `v0.7.14` across `main.go`, all four landing footers, demo kicker, and datasets-page kicker.

## [0.7.13] - 2026-05-22

### Fixed — Live histogram drops frames (snapshot queue + client playback)

Diagnosis: the simJob stored only one `LatestSnapshot`. The browser polled every 80 ms but the simulation often emitted all 15-30 per-generation snapshots in well under 80 ms, so most snapshots were overwritten before they were ever sent. The chart visibly jumped from generation 0 to whatever the latest single snapshot happened to be — not an animation.

Fix:

- **Backend.** `simJob` now keeps `Snapshots []AFSSnapshot` (the full history) plus a derived `SnapshotSeq = len(Snapshots)`. `updateSimulationJobSnapshot` appends to the slice instead of overwriting a single field. The `/api/simulate/status` handler accepts `?since=<N>` and returns `Snapshots[since:]` plus the new `snapshot_seq`. The legacy `latest_snapshot` field is still emitted for backwards compatibility.

- **Frontend.** The poll loop is the producer: it asks for `?since=snapshotSeq` and appends the returned `snapshots[]` to a client-side queue `pendingFrames[]`. An independent `setTimeout` chain (`playNextHistogramFrame`) is the consumer: it dequeues one frame and draws it every `PLAYBACK_FRAME_MS` (90 ms). The playback continues running after the poll loop exits, draining the queue. A backend that finishes 20 generations in 300 ms now plays back as a ~1.8 s animation in the UI — visibly live.

- **Smoke verification.** A 15-generation run produces `snapshot_seq=16` (generations 0-15 inclusive) and `?since=10` returns only the trailing 6 frames. Confirmed locally.

### Not changed
- Concurrency model: still one tracked strategy + replicate 0 only; `simJobStore.Mutex` guards the slice write/read.
- `latest_snapshot` is still emitted (omitempty) so older clients keep working.
- SSE / true streaming endpoint deferred — the polling+queue approach is a small diff with no new transport layer.

### Changed
- Version strings bumped `v0.7.12` → `v0.7.13` across `main.go`, all four landing footers, the demo kicker, and the datasets-page kicker.

## [0.7.12] - 2026-05-22

### Added — Public-wheat datasets registry

A curated list of public wheat genotype datasets is now bundled with BreedOS and exposed at **`/datasets`**.

`breedos/datasets/` (new folder):
- `README.md` — usage notes + fetch commands. Tracked.
- `.gitignore` — ignores everything except the README and itself. Tracked.
- Raw archive files (`*.vcf`, `*.zip`, `*.xlsb`, ...) — **gitignored**. Fetched locally per the README; not redistributed by BreedOS.

`breedos/mvp/datasets-manifest.json` (new tracked file, embedded into the binary via `//go:embed`):
- Six entries spanning small + large + manual-only sources:
  - `wheat_durum_figshare_259` — Figshare durum wheat 259 × 7817, VCFv4.2, 8.45 MB, full upload.
  - `wheat_dryad_159_55k` — Dryad 55K SNPs × 159 wheat, .xlsb, 20.4 MB, manual download (Dryad blocks programmatic).
  - `wheat_dryad_pakistani_37k` — Dryad Pakistani 37K, .xlsx set, 30.2 MB, manual download.
  - `wheat_inrae_1000_exomes` — INRAE 1000 wheat exome ZIP, 2.25 GB, truncated to 100 MB on server.
  - `wheat_zenodo_28m_v21` — Zenodo same SNPs lifted to RefSeq v2.1, 9.2 GB VCF, truncated to 100 MB on server.
  - `wheat_watkins_g2b` — Watkins G2B portal landraces+cultivars, per-chromosome VCFs, manual.

### Added — `/datasets` page and `/api/datasets` endpoint

`mvp/datasets_api.go`:
- `GET /api/datasets` reads the embedded manifest, merges it with the current on-server file sizes from `<bindir>/data/datasets/`, returns combined JSON. Each entry carries: id, name, full `size_bytes`, current `deployed_bytes`, `status` (`full` / `truncated` / `manual` / `missing` / `stale`), category, deploy strategy, source URL, landing URL, license, content description.

`mvp/static/datasets.html`:
- Plain-JS page that fetches `/api/datasets` on load and renders a table with columns: Name, Original size, On server, Status, Accessions × markers, Format, License, Source, Content.

### Changed — `deploy_breedos.sh`

The deploy script now iterates `mvp/datasets-manifest.json` entries and uploads files from `breedos/datasets/` to `<server>/data/datasets/<filename>` per the declared `deploy_strategy`:
- `full` → `scp` whole file (size-skip applies).
- `truncate_head_100mb` → upload only `head -c 100MB` (truncation cap is the manifest's `deploy_truncate_mb`).
- `manual` / `large_external` → skipped.

Set `BREEDOS_DEPLOY_FULL_LARGE=1` to override truncation and upload the full local file (use after a server-disk upgrade).

### Changed
- Version strings bumped `v0.7.11` → `v0.7.12` in `main.go` run notes, all four landing footers, and the demo kicker.

### Not in this release
- Big-dataset local downloads via the sandbox proxy timed out at partial bytes (848 MB of the INRAE 2.25 GB, 461 MB of the Zenodo 9.2 GB). The partials are valid file prefixes — the 100 MB truncation for server deploy works from them. Operator should re-fetch full archives on their workstation for a complete local copy.
- Dryad files (55K + Pakistani 37K) cannot be fetched programmatically from this sandbox (HTTP 403). Marked `deploy_strategy: "manual"` in the manifest; operator visits the landing URL and saves files into `breedos/datasets/` before the next deploy.

## [0.7.11] - 2026-05-22

### Fixed — Demo shell width alignment

The demo page now uses one explicit demo-width container (`--demo-max: 1340px`) for the navigation, honesty banner, title card, and bottom workbench. This fixes the remaining visual mismatch where the top of `/demo` was constrained by the generic landing-page `.wrap` width (`1180px`) while the lower simulation workbench appeared wider.

Added nested `min-width: 0` guards for demo cards, result panels, grid children, chart cards, histogram cards, and table wrappers so charts/tables shrink or scroll inside the shared shell instead of widening the page.

### Changed
- Demo kicker and version strings bumped `v0.7.10` → `v0.7.11`.

## [0.7.10] - 2026-05-22

### Fixed — demo-grid width (definitive)

`.demo-grid` switched from CSS Grid (`grid-template-columns: 360px 1fr` / later `minmax(0, 1fr)`) to Flexbox:

```css
.demo-grid {
  display: flex;
  flex-direction: row;
  gap: 18px;
  align-items: flex-start;
}
.demo-grid > .sticky-panel { flex: 0 0 360px; }
.demo-grid > .results { flex: 1 1 0%; min-width: 0; }
```

`flex: 1 1 0%` + `min-width: 0` is the classic shrinkable-fill pattern and constrains the flex container to its parent width without the track-sizing surprises Grid was producing. Verified on production v0.7.9 that the earlier `minmax(0, 1fr)` + `min-width: 0` on grid items was still rendering the demo-grid visibly wider than the top hero / title cards.

The `@media (max-width: 980px)` rule was updated to use `flex-direction: column` for narrow viewports (sticky panel collapses to full-width stacked above results).

### Reverted — Cache-control hacks from v0.7.9

v0.7.9 introduced `Cache-Control: no-cache, must-revalidate` on the Go static handler and `?v=v0.7.9` query strings on every `<link rel="stylesheet">`. These were a misdiagnosis (the user works in dev mode with cache disabled; the width issue was the real CSS bug, not a stale cache). Removed both:

- Static handler returns to default no-cache-header behaviour. If real cache headers are wanted later, that's a separate, deliberate decision with its own scope.
- All five HTML files reference `/style.css` (no query string).

### Changed
- Version strings bumped `v0.7.9` → `v0.7.10` across `main.go`, all four landing footers, and the demo kicker.

## [0.7.9] - 2026-05-22

### Fixed — Demo-grid width (proper fix)

The v0.7.8 attempt (`min-width: 0` on grid items) wasn't sufficient. Stronger fix:

- `grid-template-columns: 360px 1fr` → `360px minmax(0, 1fr)`. Bare `1fr` resolves its minimum size to the column's min-content (which was being inflated by 900-px-wide canvases and a 14-column strategy table). `minmax(0, 1fr)` explicitly caps the minimum at 0, so the column never expands the grid past the parent `.wrap`.
- `max-width: 100%` belt-and-suspenders on `.demo-grid`.

### Fixed — Browser cache serving stale CSS / JS across deploys

The Go static handler did not set any cache headers, so browsers heuristic-cached `/style.css`, `/app.js`, and `/index.html` for several minutes. After a deploy, users would still see the previous version's styles even though the new binary was serving fresh content.

Two-pronged fix:

- The static handler now emits `Cache-Control: no-cache, must-revalidate` on every response. Browsers will revalidate on each load. (Bandwidth impact is small; the assets are tens of KB and we always send 200, but at least we are never stale.)
- All five HTML files reference `<link rel="stylesheet" href="/style.css?v=v0.7.9">`. The query string busts the URL-keyed cache for already-cached browsers that load the new HTML first.

### Changed
- Version strings bumped `v0.7.8` → `v0.7.9` in `main.go` run notes, all four landing footers, and the demo kicker.

### Background
- The user reported that after the v0.7.8 deploy the demo-grid was still visibly wider than the top hero cards. Diagnosed two contributing causes: (a) `min-width: 0` on grid items isn't enough when the track itself is `1fr` (track resolves min size to min-content), and (b) browser cache likely served the v0.7.7 CSS even though the v0.7.8 binary was serving fresh content. This release addresses both.

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
