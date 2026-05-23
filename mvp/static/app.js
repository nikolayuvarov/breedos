const colors = {
  neutral: '#d5dbe8',
  random: '#b9b0ff',
  phenotype: '#f4b2df',
  genomic: '#a6f0ff',
  aggressive: '#ff9a9a',
  diversity: '#8cb7ff',
  balanced: '#8fffd1',
  balanced_crispr: '#ffd38c',
  ocs_like: '#c2ff8c',
  cross_planner: '#ffdf70',
  edit_introgression: '#ffb070'
};

const labels = {
  neutral: 'Neutral drift',
  random: 'Random baseline',
  phenotype: 'Phenotype truncation',
  genomic: 'Genomic mock',
  aggressive: 'Aggressive',
  diversity: 'Diversity-preserving',
  balanced: 'Balanced',
  balanced_crispr: 'Balanced + CRISPR',
  ocs_like: 'OCS-like',
  cross_planner: 'Cross planner',
  edit_introgression: 'Edit introgression'
};

let currentData = null;
let previousData = null;
// v0.7.6 live histogram: latest AFS snapshot received from /api/simulate/status.
// v0.7.8 — also cache the last-drawn generation key to skip redundant redraws.
// v0.7.13 — keep a client-side queue of snapshots and play them back at a fixed
// UI tempo so the chart looks like an animation even when the backend finishes
// all generations in a few hundred milliseconds. The poll loop is the
// producer (cheap network ops); a separate setTimeout chain is the consumer.
let lastSnapshot = null;
let lastDrawnGeneration = -1;
let snapshotSeq = 0;            // next snapshot index to fetch
let pendingFrames = [];         // FIFO queue of snapshots to render
let playbackTimer = null;       // setTimeout handle for the next frame
const PLAYBACK_FRAME_MS = 90;   // ~11 fps; fast enough to feel live, slow
                                // enough that an 8-generation backend run
                                // takes ~720ms in the UI.

// v0.7.15 budget meter. Mirrors the server-side cap in validateRequest():
//   budget = N × markers × (generations+1) × strategies × replicates ≤ 1.5B
// The meter is the only place the user sees this number before clicking Run;
// without it the only signal is a 400 after submit, which is easy to mistake
// for a stuck client state when reducing one parameter still leaves budget
// over cap. Cap was raised from 800M to 1.5B in v0.7.15 — prod is single-core
// so the upper-bound run is ~30–60s wall-clock, still acceptable for the demo.
const BUDGET_CAP = 1500000000;
const BUDGET_WARN_FRAC = 0.7;
// Inputs that multiply into the budget formula. Highlighted red when over cap
// so the user immediately sees which knobs to turn down.
const BUDGET_INPUT_IDS = ['population_size', 'markers', 'generations', 'replicates'];

function byId(id) { return document.getElementById(id); }

function numberValue(id) {
  const el = byId(id);
  if (!el) return 0;
  const raw = String(el.value || "").trim().replace(",", ".");
  const v = Number(raw);
  return Number.isFinite(v) ? v : 0;
}

function requestFromForm() {
  return {
    dataset: (byId('dataset') && byId('dataset').value) || 'synthetic',
    seed: Math.trunc(numberValue('seed')),
    population_size: Math.trunc(numberValue('population_size')),
    markers: Math.trunc(numberValue('markers')),
    qtl_count: Math.trunc(numberValue('qtl_count')),
    generations: Math.trunc(numberValue('generations')),
    selection_percent: numberValue('selection_percent'),
    heritability: numberValue('heritability'),
    mutation_rate: numberValue('mutation_rate'),
    crispr_enabled: byId('crispr_enabled').checked,
    crispr_edits: Math.trunc(numberValue('crispr_edits')),
    crispr_intro_percent: numberValue('crispr_intro_percent'),
    strategy_set: (byId('strategy_set') && byId('strategy_set').value) || 'core',
    tracked_strategy: (byId('tracked_strategy') && byId('tracked_strategy').value) || '',
    replicates: Math.trunc(numberValue('replicates')),
    worker_count: Math.trunc(numberValue('worker_count')),
    inbreeding_limit: numberValue('inbreeding_limit'),
    diversity_loss_limit: numberValue('diversity_loss_limit'),
    max_inbreeding: numberValue('max_inbreeding'),
    max_diversity_loss: numberValue('max_diversity_loss'),
    max_rare_useful_loss: Math.trunc(numberValue('max_rare_useful_loss')),
    min_genetic_gain: numberValue('min_genetic_gain'),
    min_effective_parents: Math.trunc(numberValue('min_effective_parents')),
    max_combined_risk: numberValue('max_combined_risk')
  };
}

function setFormValues(values) {
  for (const [id, value] of Object.entries(values)) {
    const el = byId(id);
    if (!el) continue;
    if (el.type === 'checkbox') el.checked = Boolean(value);
    else el.value = String(value);
  }
  updateBudgetMeter();
}

const presets = {
  tiny: {
    seed: 20260505,
    population_size: 8,
    markers: 180,
    qtl_count: 18,
    generations: 60,
    selection_percent: 25,
    heritability: 0.45,
    mutation_rate: 0.00002,
    crispr_enabled: true,
    crispr_edits: 3,
    crispr_intro_percent: 25,
    strategy_set: 'advanced',
    replicates: 20,
    worker_count: 0,
    inbreeding_limit: 0.25,
    diversity_loss_limit: 0.30
  },
  balanced: {
    seed: 20260505,
    population_size: 400,
    markers: 900,
    qtl_count: 45,
    generations: 30,
    selection_percent: 10,
    heritability: 0.4,
    mutation_rate: 0.00005,
    crispr_enabled: true,
    crispr_edits: 3,
    crispr_intro_percent: 8,
    strategy_set: 'core',
    replicates: 3,
    worker_count: 0,
    inbreeding_limit: 0.25,
    diversity_loss_limit: 0.30
  },
  large: {
    seed: 20260505,
    population_size: 2500,
    markers: 800,
    qtl_count: 50,
    generations: 35,
    selection_percent: 8,
    heritability: 0.35,
    mutation_rate: 0.00003,
    crispr_enabled: true,
    crispr_edits: 4,
    crispr_intro_percent: 4,
    strategy_set: 'core',
    replicates: 1,
    worker_count: 0,
    inbreeding_limit: 0.25,
    diversity_loss_limit: 0.30
  },
  crispr: {
    seed: 20260505,
    population_size: 240,
    markers: 700,
    qtl_count: 55,
    generations: 36,
    selection_percent: 12,
    heritability: 0.5,
    mutation_rate: 0.00004,
    crispr_enabled: true,
    crispr_edits: 6,
    crispr_intro_percent: 12,
    strategy_set: 'advanced',
    replicates: 3,
    worker_count: 0,
    inbreeding_limit: 0.25,
    diversity_loss_limit: 0.30
  }
};

function setStatus(text, cls) {
  const el = byId('status');
  if (!el) return;
  el.textContent = text || '';
  el.className = 'status ' + (cls || '');
}

async function runSimulation() {
  const btn = byId('runBtn');
  const request = requestFromForm();
  if (currentData && requestSignature(request) === requestSignature(currentData.request || {})) {
    setStatus('No parameter changes since the current run. Recalculation skipped.', 'ok');
    return;
  }
  // v0.7.15: stop the request from leaving the browser if the run is over the
  // server-side budget cap. Without this the user sees a generic 400, and
  // reducing one parameter while the rest still keep budget over cap looks
  // like a stuck client.
  if (updateBudgetMeter().over) {
    const cells = formatCells(runBudgetCells(request));
    setStatus(`Run budget ${cells} cells exceeds the MVP cap of ${formatCells(BUDGET_CAP)}. Reduce population size, markers, generations, or replicates.`, 'error');
    return;
  }
  const originalText = btn ? btn.textContent : '';
  if (btn) {
    btn.disabled = true;
    setRunButtonProgress(0, 'Starting');
  }
  setStatus('Starting simulation job...', '');
  // v0.7.6 live histogram: reset snapshot state for a fresh run.
  // v0.7.13: also reset the playback queue + seq cursor.
  lastSnapshot = null;
  lastDrawnGeneration = -1;
  snapshotSeq = 0;
  pendingFrames = [];
  if (playbackTimer) {
    clearTimeout(playbackTimer);
    playbackTimer = null;
  }
  resetLiveHistogram();
  try {
    const startRes = await fetch('/api/simulate/start', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(request)
    });
    if (!startRes.ok) {
      const text = await startRes.text();
      throw new Error(text || `HTTP ${startRes.status}`);
    }
    const start = await startRes.json();
    if (!start.job_id) throw new Error('Server did not return a job id');

    let job = null;
    for (;;) {
      // v0.7.13: pass ?since=snapshotSeq so the server returns only new frames
      // we haven't seen yet. The full history is on the server; we accumulate
      // missed frames into a queue that the playback timer consumes at a
      // fixed UI tempo (PLAYBACK_FRAME_MS), so a fast backend looks animated
      // rather than jumping to the final state.
      const statusURL = '/api/simulate/status?id='
        + encodeURIComponent(start.job_id)
        + '&since=' + snapshotSeq;
      const statusRes = await fetch(statusURL, {cache: 'no-store'});
      if (!statusRes.ok) {
        const text = await statusRes.text();
        throw new Error(text || `HTTP ${statusRes.status}`);
      }
      job = await statusRes.json();
      const percent = Number.isFinite(Number(job.percent)) ? Number(job.percent) : 0;
      setRunButtonProgress(percent, job.done ? 'Finishing' : 'Running');
      setStatus(`${job.message || 'Running'} — ${Math.round(percent)}%`, '');
      if (Array.isArray(job.snapshots) && job.snapshots.length > 0) {
        for (const s of job.snapshots) pendingFrames.push(s);
        startHistogramPlayback();
      }
      if (Number.isFinite(Number(job.snapshot_seq))) {
        snapshotSeq = Number(job.snapshot_seq);
      }
      if (job.done) break;
      await sleep(80);
    }

    if (job.error) throw new Error(job.error);
    if (!job.result) throw new Error('Simulation finished without a result');

    const data = job.result;
    setFormValues(data.request || request);
    if (currentData) previousData = currentData;
    currentData = data;
    renderAll(currentData, previousData);
    const changed = previousData ? changedParams(previousData.request || {}, currentData.request || {}) : [];
    const changedText = changed.length ? ' Changed: ' + changed.join('; ') + '.' : '';
    setStatus(previousData ? 'Simulation complete. Dotted lines show the previous run.' + changedText : 'Simulation complete.', 'ok');
  } catch (err) {
    console.error(err);
    setStatus(err.message || String(err), 'error');
  } finally {
    if (btn) {
      btn.disabled = false;
      btn.textContent = originalText || 'Run simulation';
    }
  }
}

function setRunButtonProgress(percent, label) {
  const btn = byId('runBtn');
  if (!btn) return;
  const p = Math.max(0, Math.min(100, Math.round(Number(percent) || 0)));
  btn.textContent = `${label || 'Running'} ${p}%`;
}

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

function clearComparison() {
  previousData = null;
  if (currentData) renderAll(currentData, null);
  setStatus('Previous-run overlay cleared.', 'ok');
}

function randomizeSeed() {
  byId('seed').value = String(Math.floor(1 + Math.random() * 2147483646));
  markDirty('seed changed');
}

function applyPreset(name) {
  if (!presets[name]) return;
  setFormValues(presets[name]);
  const names = {
    tiny: 'Tiny drift demo',
    balanced: 'Balanced default',
    large: 'Large fast demo',
    crispr: 'CRISPR seed demo'
  };
  markDirty(`${names[name] || name} preset loaded`);
}

function renderAll(data, prev) {
  renderDecisionPanel(data);
  renderSummary(data, prev);
  renderNotes(data, prev);
  drawMetricChart('chart_gain', 'legend_gain', data, prev, 'genetic_gain', 'Genetic gain');
  drawMetricChart('chart_diversity', 'legend_diversity', data, prev, 'diversity', 'Diversity');
  drawMetricChart('chart_inbreeding', 'legend_inbreeding', data, prev, 'inbreeding', 'Inbreeding');
  drawMetricChart('chart_drift', 'legend_drift', data, prev, 'allele_drift', 'Allele drift');
  drawMetricChart('chart_lost', 'legend_lost', data, prev, 'rare_useful_lost', 'Rare useful loci lost');
  drawMetricChart('chart_fixed', 'legend_fixed', data, prev, 'fixed_loci', 'Fixed loci');
  drawParetoChart('chart_pareto', data);
  renderEditTable(data.candidate_edits || []);
  renderStrategyTable(data.strategies || [], prev);
}


function renderDecisionPanel(data) {
  const el = byId('decisionPanel');
  if (!el) return;
  const d = data.decision || {};
  const best = findStrategy(data, d.best_risk_adjusted_code);
  const bestGain = findStrategy(data, d.best_gain_code);
  const lowest = findStrategy(data, d.lowest_risk_code);
  const bestFeasible = d.best_feasible_code ? findStrategy(data, d.best_feasible_code) : null;
  const constraintsApplied = Array.isArray(d.constraints_applied) ? d.constraints_applied : [];
  const lines = (d.interpretation || []).map(x => `<li>${escapeHtml(x)}</li>`).join('');
  const pareto = (d.pareto_codes || []).map(code => `<span class="pill">${escapeHtml(labels[code] || code)}</span>`).join(' ');
  const tradeoffs = (d.tradeoffs || []).map(t => `<li><strong>${escapeHtml((labels[t.a] || t.a) + ' ↔ ' + (labels[t.b] || t.b))}</strong> <em>(${escapeHtml(t.theme)})</em><br><span>${escapeHtml(t.note)}</span></li>`).join('');
  const avoid = (d.avoid_strategies || []).map(a => `<li><strong>${escapeHtml(labels[a.code] || a.name)}</strong> — ${escapeHtml(a.reason)}</li>`).join('');
  const warnings = (d.missing_data_warnings || []).map(w => `<li>${escapeHtml(w)}</li>`).join('');
  const assumptions = (d.key_assumptions || []).map(a => `<li>${escapeHtml(a)}</li>`).join('');
  const wrong = (d.what_could_be_wrong || []).map(w => `<li>${escapeHtml(w)}</li>`).join('');
  const limitations = (d.limitations || []).map(l => `<li>${escapeHtml(l)}</li>`).join('');
  const next = d.next_analysis ? `<p class="decision-next"><strong>Recommended next analysis:</strong> ${escapeHtml(d.next_analysis)}</p>` : '';
  const honesty = d.honesty_banner ? `<div class="decision-honesty-banner">${escapeHtml(d.honesty_banner)}</div>` : '';
  const feasibilityCard = constraintsApplied.length
    ? `<div class="decision-card feasible-card"><span>Best feasible</span><strong>${escapeHtml(bestFeasible ? (labels[bestFeasible.code] || bestFeasible.name) : 'none')}</strong><small>${bestFeasible ? 'risk-adjusted score ' + fmt(bestFeasible.final.risk_adjusted_score) : 'no strategy passes all constraints'}</small></div>`
    : '';
  const feasibilityNote = d.feasibility_note
    ? `<div class="feasibility-note${bestFeasible ? '' : ' feasibility-note-empty'}"><strong>Feasibility:</strong> ${escapeHtml(d.feasibility_note)}</div>`
    : '';
  const constraintsChips = constraintsApplied.length
    ? `<div class="constraints-applied"><strong>Constraints applied:</strong> ${constraintsApplied.map(c => `<span class="pill">${escapeHtml(c)}</span>`).join(' ')}</div>`
    : '';
  el.innerHTML = `
    ${honesty}
    <div class="decision-grid${constraintsApplied.length ? ' decision-grid-4' : ''}">
      <div class="decision-card"><span>Recommended</span><strong>${escapeHtml(best ? (labels[best.code] || best.name) : '-')}</strong><small>risk-adjusted score ${best ? fmt(best.final.risk_adjusted_score) : '-'}</small></div>
      <div class="decision-card"><span>Max gain</span><strong>${escapeHtml(bestGain ? (labels[bestGain.code] || bestGain.name) : '-')}</strong><small>${bestGain ? fmt(bestGain.final.genetic_gain) : '-'} final gain</small></div>
      <div class="decision-card"><span>Lowest risk</span><strong>${escapeHtml(lowest ? (labels[lowest.code] || lowest.name) : '-')}</strong><small>${lowest ? fmt(combinedRisk(lowest.final)) : '-'} combined risk</small></div>
      ${feasibilityCard}
    </div>
    ${feasibilityNote}
    ${constraintsChips}
    <ul class="decision-list">${lines}</ul>
    <div class="pareto-list"><strong>Pareto candidates:</strong> ${pareto || '-'}</div>
    ${next}
    ${tradeoffs ? `<details class="decision-section" open><summary><strong>Top trade-offs</strong> (${(d.tradeoffs || []).length})</summary><ul class="decision-list">${tradeoffs}</ul></details>` : ''}
    ${avoid ? `<details class="decision-section" open><summary><strong>Strategies to avoid</strong> (${(d.avoid_strategies || []).length})</summary><ul class="decision-list">${avoid}</ul></details>` : ''}
    ${wrong ? `<details class="decision-section decision-what-could-be-wrong" open><summary><strong>What could make this recommendation wrong?</strong> (${(d.what_could_be_wrong || []).length})</summary><ul class="decision-list">${wrong}</ul></details>` : ''}
    ${warnings ? `<details class="decision-section decision-warnings"><summary><strong>⚠ Missing-data warnings</strong> (${(d.missing_data_warnings || []).length})</summary><ul class="decision-list">${warnings}</ul></details>` : ''}
    ${assumptions ? `<details class="decision-section"><summary><strong>Key assumptions</strong> (${(d.key_assumptions || []).length})</summary><ul class="decision-list">${assumptions}</ul></details>` : ''}
    ${limitations ? `<details class="decision-section decision-limitations"><summary><strong>Model limitations</strong> (${(d.limitations || []).length})</summary><ul class="decision-list">${limitations}</ul></details>` : ''}
  `;
}

function renderNotes(data, prev) {
  const el = byId('notes');
  if (!el) return;
  const notes = [...(data.notes || [])];
  if (prev) {
    notes.unshift('Comparison mode: solid lines are the current run; dotted lines are the previous run.');
    const changed = changedParams(prev.request || {}, data.request || {});
    if (changed.length) notes.unshift('Changed parameters: ' + changed.join('; '));
  }
  const req = data.request || {};
  const budget = (Number(req.population_size) || 0) * (Number(req.markers) || 0) * ((Number(req.generations) || 0) + 1);
  const strategyCount = (data.strategies || []).length;
  const reps = Number(req.replicates) || 1;
  notes.push(`Run budget: ${formatInt(budget * strategyCount * reps)} population-marker-generation-strategy-replicate cells.`);
  el.innerHTML = notes.map(n => `<div class="note-item">${escapeHtml(n)}</div>`).join('');
}

function renderSummary(data, prev) {
  const s = data.strategies || [];
  const bestGain = maxBy(s, x => x.final.genetic_gain);
  const bestDiversity = maxBy(s, x => x.final.diversity);
  const lowestInbreeding = minBy(s, x => x.final.inbreeding);
  const balanced = s.find(x => x.code === 'balanced') || s[0];
  const req = data.request || {};
  const prevBalanced = findStrategy(prev, balanced ? balanced.code : 'balanced');
  const balancedDelta = balanced && prevBalanced ? signedDelta(balanced.final.genetic_gain - prevBalanced.final.genetic_gain) + ' vs previous' : 'no previous run';
  const expectedFlips = expectedMutationFlips(req);
  const cards = [
    ['Best final gain', bestGain ? labels[bestGain.code] : '-', bestGain ? fmt(bestGain.final.genetic_gain) : '-'],
    ['Best diversity', bestDiversity ? labels[bestDiversity.code] : '-', bestDiversity ? fmt(bestDiversity.final.diversity) : '-'],
    ['Lowest inbreeding', lowestInbreeding ? labels[lowestInbreeding.code] : '-', lowestInbreeding ? fmt(lowestInbreeding.final.inbreeding) : '-'],
    ['BreedOS default', balancedDelta, balanced ? fmt(balanced.final.genetic_gain) + ' gain' : '-'],
    ['Population', Number(req.population_size) < 50 ? 'small-pop drift regime' : 'standard demo regime', formatInt(req.population_size)],
    ['Markers / QTL', `${formatInt(req.qtl_count)} causal loci`, `${formatInt(req.markers)} markers`],
    ['Mutation rate', 'per allele per generation', formatParamValue(req.mutation_rate)],
    ['Expected flips', 'per strategy replicate', expectedFlips < 10 ? fmt(expectedFlips) : formatInt(expectedFlips)],
    ['Strategy set', `${(data.strategies || []).length} strategies`, String(req.strategy_set || 'core')],
    ['Replicates', 'Monte Carlo runs per strategy', formatInt(req.replicates || 1)],
    ['Best decision', data.decision ? (labels[data.decision.best_risk_adjusted_code] || data.decision.best_risk_adjusted_name || '-') : '-', data.decision ? (data.decision.best_risk_adjusted_code || '-') : '-']
  ];
  byId('summary').innerHTML = cards.map(([label, name, value]) => `
    <div class="metric-card">
      <div class="label">${escapeHtml(label)}</div>
      <div class="value">${escapeHtml(value)}</div>
      <div class="label">${escapeHtml(name)}</div>
    </div>
  `).join('');
}

function renderEditTable(edits) {
  if (!edits.length) {
    byId('editTable').innerHTML = '<tr><td colspan="7">No candidate edits returned.</td></tr>';
    return;
  }
  byId('editTable').innerHTML = edits.map(e => `
    <tr>
      <td>${e.rank}</td>
      <td>${e.locus}</td>
      <td>${fmt(e.effect)}</td>
      <td>${fmt(e.allele_frequency)}</td>
      <td>${fmt(e.expected_gain_score)}</td>
      <td>${escapeHtml(e.diversity_risk)}</td>
      <td>${escapeHtml(e.decision)}</td>
    </tr>
  `).join('');
}

function renderStrategyTable(strategies, prev) {
  if (!strategies.length) {
    byId('strategyTable').innerHTML = '<tr><td colspan="14">No strategies returned.</td></tr>';
    return;
  }
  byId('strategyTable').innerHTML = strategies.map(s => {
    const p = findStrategy(prev, s.code);
    const delta = p ? [
      `gain ${signedDelta(s.final.genetic_gain - p.final.genetic_gain)}`,
      `div ${signedDelta(s.final.diversity - p.final.diversity)}`,
      `F ${signedDelta(s.final.inbreeding - p.final.inbreeding)}`
    ].join('<br>') : '-';
    const failed = Array.isArray(s.final.failed_constraints) ? s.final.failed_constraints : [];
    const feasibleCell = s.final.feasible
      ? '<span class="feasible-yes" title="Passes all active constraints">✓</span>'
      : (failed.length
        ? `<span class="feasible-no" title="${escapeHtml(failed.join('; '))}">✗ <small>${failed.length}</small></span>`
        : '<span>—</span>');
    const rowClass = s.final.feasible === false && failed.length ? ' class="row-infeasible"' : '';
    return `
      <tr${rowClass}>
        <td><strong style="color:${colors[s.code] || '#fff'}">${escapeHtml(labels[s.code] || s.name)}</strong><br><span>${escapeHtml(s.summary)}</span></td>
        <td>${formatInt(s.final.decision_rank)}</td>
        <td>${fmt(s.final.risk_adjusted_score)}</td>
        <td>${fmt(s.final.genetic_gain)}<br><span>±${fmt(s.final.gain_std)}</span></td>
        <td>${fmt(s.final.diversity)}<br><span>±${fmt(s.final.diversity_std)}</span></td>
        <td>${fmt(s.final.inbreeding)}<br><span>±${fmt(s.final.inbreeding_std)}</span></td>
        <td>${fmt(combinedRisk(s.final))}</td>
        <td>${fmt(s.final.probability_inbreeding_breach)} / ${fmt(s.final.probability_diversity_collapse)} / ${fmt(s.final.probability_rare_useful_loss)}</td>
        <td>${formatInt(s.final.fixed_loci)}</td>
        <td>${formatInt(s.final.effective_parents)}</td>
        <td>${s.final.pareto_optimal ? 'yes' : 'no'}</td>
        <td>${feasibleCell}</td>
        <td>${delta}</td>
        <td>${escapeHtml(s.final.recommended_next)}</td>
      </tr>
    `;
  }).join('');
}


function drawParetoChart(canvasId, data) {
  const canvas = byId(canvasId);
  if (!canvas) return;
  const ctx = canvas.getContext('2d');
  const width = canvas.width;
  const height = canvas.height;
  ctx.clearRect(0, 0, width, height);
  const strategies = data.strategies || [];
  if (!strategies.length) return;
  const margin = {left: 62, right: 24, top: 28, bottom: 52};
  const innerW = width - margin.left - margin.right;
  const innerH = height - margin.top - margin.bottom;
  const gains = strategies.map(s => Number(s.final.genetic_gain));
  const risks = strategies.map(s => combinedRisk(s.final));
  let minX = Math.min(...risks), maxX = Math.max(...risks);
  let minY = Math.min(...gains), maxY = Math.max(...gains);
  if (Math.abs(maxX - minX) < 1e-9) { minX -= 0.1; maxX += 0.1; }
  if (Math.abs(maxY - minY) < 1e-9) { minY -= 1; maxY += 1; }
  const padX = (maxX - minX) * 0.12;
  const padY = (maxY - minY) * 0.12;
  minX = Math.max(0, minX - padX); maxX += padX;
  minY -= padY; maxY += padY;
  const x = v => margin.left + (Number(v) - minX) / (maxX - minX) * innerW;
  const y = v => margin.top + (1 - (Number(v) - minY) / (maxY - minY)) * innerH;
  ctx.fillStyle = 'rgba(255,255,255,0.025)';
  ctx.fillRect(margin.left, margin.top, innerW, innerH);
  ctx.strokeStyle = 'rgba(255,255,255,0.12)';
  ctx.lineWidth = 1;
  ctx.fillStyle = 'rgba(237,248,245,0.72)';
  ctx.font = '12px system-ui, sans-serif';
  ctx.textAlign = 'right'; ctx.textBaseline = 'middle';
  for (let i = 0; i <= 4; i++) {
    const yy = margin.top + innerH * i / 4;
    ctx.beginPath(); ctx.moveTo(margin.left, yy); ctx.lineTo(margin.left + innerW, yy); ctx.stroke();
    ctx.fillText(fmt(maxY - (maxY - minY) * i / 4), margin.left - 8, yy);
  }
  ctx.textAlign = 'center'; ctx.textBaseline = 'top';
  for (let i = 0; i <= 4; i++) {
    const val = minX + (maxX - minX) * i / 4;
    const xx = x(val);
    ctx.fillText(fmt(val), xx, margin.top + innerH + 12);
  }
  ctx.fillStyle = 'rgba(237,248,245,0.90)';
  ctx.textAlign = 'left'; ctx.fillText('Pareto frontier: gain vs combined risk', margin.left, 6);
  ctx.textAlign = 'center'; ctx.fillText('combined risk probability', margin.left + innerW / 2, height - 22);
  ctx.save();
  ctx.translate(16, margin.top + innerH / 2);
  ctx.rotate(-Math.PI / 2);
  ctx.fillText('genetic gain', 0, 0);
  ctx.restore();
  for (const s of strategies) {
    const xx = x(combinedRisk(s.final));
    const yy = y(s.final.genetic_gain);
    ctx.fillStyle = colors[s.code] || '#fff';
    ctx.strokeStyle = s.final.pareto_optimal ? 'rgba(255,255,255,0.95)' : 'rgba(0,0,0,0.25)';
    ctx.lineWidth = s.final.pareto_optimal ? 3 : 1;
    ctx.beginPath(); ctx.arc(xx, yy, s.final.pareto_optimal ? 7 : 5, 0, Math.PI * 2); ctx.fill(); ctx.stroke();
    ctx.fillStyle = 'rgba(237,248,245,0.86)';
    ctx.font = '11px system-ui, sans-serif';
    ctx.textAlign = 'left'; ctx.textBaseline = 'bottom';
    ctx.fillText(labels[s.code] || s.code, xx + 8, yy - 4);
  }
}

// v0.7.6 live histogram: render the per-generation AFS bins as a bar chart.
// `snap` is the latest_snapshot object from /api/simulate/status: it carries
// {generation, total_generations, strategy_code, strategy_name, bins[10]}.
function drawLiveHistogram(snap) {
  const canvas = byId('chart_histogram');
  if (!canvas) return;
  const label = byId('histogramLabel');
  if (!snap || !Array.isArray(snap.bins) || snap.bins.length === 0) return;
  const total = Number(snap.total_generations) || 0;
  const gen = Number(snap.generation) || 0;
  const niceName = labels[snap.strategy_code] || snap.strategy_name || snap.strategy_code || 'strategy';
  if (label) {
    label.textContent = `Live allele-frequency spectrum — ${niceName}, generation ${gen} / ${total}`;
  }
  const ctx = canvas.getContext('2d');
  const width = canvas.width;
  const height = canvas.height;
  ctx.clearRect(0, 0, width, height);
  const margin = {left: 44, right: 16, top: 14, bottom: 30};
  const innerW = width - margin.left - margin.right;
  const innerH = height - margin.top - margin.bottom;
  const bins = snap.bins.map(v => Number(v) || 0);
  // v0.7.8: fix Y-axis to total markers (sum of bins). Bins always sum to M,
  // so this gives a stable scale across all generations — no rescaling
  // jitter as alleles drift toward fixation. Bars represent fraction of
  // markers in each frequency bin.
  const totalMarkers = bins.reduce((a, b) => a + b, 0);
  const maxV = Math.max(1, totalMarkers);
  ctx.fillStyle = 'rgba(255,255,255,0.025)';
  ctx.fillRect(margin.left, margin.top, innerW, innerH);
  ctx.strokeStyle = 'rgba(255,255,255,0.10)';
  ctx.lineWidth = 1;
  ctx.fillStyle = 'rgba(237,248,245,0.72)';
  ctx.font = '11px system-ui, sans-serif';
  ctx.textAlign = 'right';
  ctx.textBaseline = 'middle';
  for (let i = 0; i <= 4; i++) {
    const yy = margin.top + innerH * i / 4;
    ctx.beginPath();
    ctx.moveTo(margin.left, yy);
    ctx.lineTo(margin.left + innerW, yy);
    ctx.stroke();
    const val = Math.round(maxV - (maxV * i / 4));
    ctx.fillText(String(val), margin.left - 6, yy);
  }
  const barColor = colors[snap.strategy_code] || '#8fffd1';
  const barW = innerW / bins.length;
  for (let k = 0; k < bins.length; k++) {
    const h = innerH * (bins[k] / maxV);
    const x = margin.left + k * barW;
    const y = margin.top + innerH - h;
    ctx.fillStyle = barColor;
    ctx.globalAlpha = 0.85;
    ctx.fillRect(x + 2, y, barW - 4, Math.max(1, h));
    ctx.globalAlpha = 1;
  }
  ctx.fillStyle = 'rgba(237,248,245,0.72)';
  ctx.textAlign = 'center';
  ctx.textBaseline = 'top';
  for (let k = 0; k <= bins.length; k++) {
    const tickX = margin.left + k * barW;
    const labelVal = (k / bins.length).toFixed(1);
    ctx.fillText(labelVal, tickX, margin.top + innerH + 6);
  }
  ctx.fillStyle = 'rgba(237,248,245,0.86)';
  ctx.textAlign = 'left';
  ctx.fillText('markers per allele-frequency bin', margin.left, 2);
}

// v0.7.13 — playback timer that pulls one snapshot at a time from
// pendingFrames and draws it. Independent of the polling loop, so it
// continues drawing the queued tail after the simulation completes.
function startHistogramPlayback() {
  if (playbackTimer) return; // already running
  playNextHistogramFrame();
}
function playNextHistogramFrame() {
  playbackTimer = null;
  if (pendingFrames.length === 0) return;
  const snap = pendingFrames.shift();
  if (snap) {
    const incomingGen = Number(snap.generation);
    if (incomingGen !== lastDrawnGeneration) {
      lastSnapshot = snap;
      lastDrawnGeneration = incomingGen;
      drawLiveHistogram(snap);
    }
  }
  if (pendingFrames.length > 0) {
    playbackTimer = setTimeout(playNextHistogramFrame, PLAYBACK_FRAME_MS);
  }
}

// resetLiveHistogram clears the canvas and resets the label so consecutive runs
// don't briefly show stale data. v0.7.6 live histogram.
function resetLiveHistogram() {
  const canvas = byId('chart_histogram');
  if (canvas) {
    const ctx = canvas.getContext('2d');
    ctx.clearRect(0, 0, canvas.width, canvas.height);
  }
  const label = byId('histogramLabel');
  if (label) label.textContent = 'Waiting for first generation snapshot…';
}

function drawMetricChart(canvasId, legendId, data, prev, metric, title) {
  const canvas = byId(canvasId);
  if (!canvas) return;
  const ctx = canvas.getContext('2d');
  const width = canvas.width;
  const height = canvas.height;
  ctx.clearRect(0, 0, width, height);

  const strategies = data.strategies || [];
  const prevStrategies = prev ? (prev.strategies || []) : [];
  const margin = {left: 58, right: 18, top: 18, bottom: 42};
  const innerW = width - margin.left - margin.right;
  const innerH = height - margin.top - margin.bottom;

  let all = [];
  let maxGen = 0;
  function collect(series) {
    for (const s of series) {
      for (const p of s.metrics || []) {
        const val = Number(p[metric]);
        if (Number.isFinite(val)) all.push(val);
        maxGen = Math.max(maxGen, Number(p.generation));
      }
    }
  }
  collect(strategies);
  collect(prevStrategies);
  if (!all.length) return;
  let minY = Math.min(...all);
  let maxY = Math.max(...all);
  if (Math.abs(maxY - minY) < 1e-9) {
    minY -= 1;
    maxY += 1;
  }
  const pad = (maxY - minY) * 0.08;
  minY -= pad;
  maxY += pad;

  function x(gen) {
    return margin.left + (Number(gen) / Math.max(1, maxGen)) * innerW;
  }
  function y(v) {
    return margin.top + (1 - (Number(v) - minY) / (maxY - minY)) * innerH;
  }

  ctx.fillStyle = 'rgba(255,255,255,0.025)';
  ctx.fillRect(margin.left, margin.top, innerW, innerH);

  ctx.strokeStyle = 'rgba(255,255,255,0.10)';
  ctx.lineWidth = 1;
  ctx.fillStyle = 'rgba(237,248,245,0.72)';
  ctx.font = '12px system-ui, sans-serif';
  ctx.textAlign = 'right';
  ctx.textBaseline = 'middle';
  ctx.setLineDash([]);
  for (let i = 0; i <= 4; i++) {
    const yy = margin.top + innerH * i / 4;
    ctx.beginPath();
    ctx.moveTo(margin.left, yy);
    ctx.lineTo(margin.left + innerW, yy);
    ctx.stroke();
    const val = maxY - (maxY - minY) * i / 4;
    ctx.fillText(fmt(val), margin.left - 8, yy);
  }
  ctx.textAlign = 'center';
  ctx.textBaseline = 'top';
  for (let i = 0; i <= 5; i++) {
    const gen = Math.round(maxGen * i / 5);
    const xx = x(gen);
    ctx.fillText(String(gen), xx, margin.top + innerH + 12);
  }
  ctx.fillStyle = 'rgba(237,248,245,0.86)';
  ctx.textAlign = 'left';
  ctx.fillText(title, margin.left, 2);
  ctx.textAlign = 'center';
  ctx.fillText('generation', margin.left + innerW / 2, height - 18);

  drawSeries(ctx, prevStrategies, metric, x, y, true);
  drawSeries(ctx, strategies, metric, x, y, false);

  const currentLegend = strategies.map(s => `
    <span><i class="dot" style="background:${colors[s.code] || '#fff'}"></i>${escapeHtml(labels[s.code] || s.name)}</span>
  `).join('');
  const previousLegend = prevStrategies.length ? '<span><i class="dash-sample"></i>previous run</span>' : '';
  byId(legendId).innerHTML = currentLegend + previousLegend;
}

function drawSeries(ctx, strategies, metric, x, y, dashed) {
  ctx.save();
  ctx.globalAlpha = dashed ? 0.38 : 1;
  ctx.setLineDash(dashed ? [7, 7] : []);
  for (const s of strategies) {
    const pts = s.metrics || [];
    if (!pts.length) continue;
    ctx.strokeStyle = colors[s.code] || '#ffffff';
    ctx.lineWidth = dashed ? 2 : 3;
    ctx.beginPath();
    pts.forEach((p, idx) => {
      const xx = x(p.generation);
      const yy = y(p[metric]);
      if (idx === 0) ctx.moveTo(xx, yy); else ctx.lineTo(xx, yy);
    });
    ctx.stroke();
    if (!dashed) {
      const last = pts[pts.length - 1];
      ctx.fillStyle = colors[s.code] || '#ffffff';
      ctx.beginPath();
      ctx.arc(x(last.generation), y(last[metric]), 4, 0, Math.PI * 2);
      ctx.fill();
    }
  }
  ctx.restore();
}

function exportJSON() {
  if (!currentData) return;
  downloadBlob(JSON.stringify(currentData, null, 2), 'breedos-run.json', 'application/json');
}

async function copySummary() {
  if (!currentData) return;
  const serverText = currentData && currentData.decision && currentData.decision.summary_text;
  const lines = serverText && serverText.length > 0 ? serverText : buildSummaryText(currentData, previousData);
  try {
    await navigator.clipboard.writeText(lines);
    setStatus('Run summary copied to clipboard.', 'ok');
  } catch (err) {
    setStatus('Clipboard copy failed: ' + (err.message || String(err)), 'error');
  }
}

function buildSummaryText(data, prev) {
  const req = data.request || {};
  const lines = [
    'BreedOS simulation summary',
    `Population=${req.population_size}, markers=${req.markers}, QTL=${req.qtl_count}, generations=${req.generations}, seed=${req.seed}, mutation_rate=${formatParamValue(req.mutation_rate)}, expected_flips_per_strategy=${fmt(expectedMutationFlips(req))}`,
    ''
  ];
  for (const s of data.strategies || []) {
    const p = findStrategy(prev, s.code);
    const delta = p ? `, delta_gain=${signedDelta(s.final.genetic_gain - p.final.genetic_gain)}` : '';
    lines.push(`${labels[s.code] || s.name}: gain=${fmt(s.final.genetic_gain)}, diversity=${fmt(s.final.diversity)}, inbreeding=${fmt(s.final.inbreeding)}, fixed_loci=${s.final.fixed_loci}${delta}`);
  }
  return lines.join('\n');
}

function downloadBlob(text, filename, type) {
  const blob = new Blob([text], {type});
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}


function requestSignature(req) {
  if (!req) return '';
  return JSON.stringify({
    dataset: String(req.dataset || 'synthetic'),
    seed: Number(req.seed) || 0,
    population_size: Number(req.population_size) || 0,
    markers: Number(req.markers) || 0,
    qtl_count: Number(req.qtl_count) || 0,
    generations: Number(req.generations) || 0,
    selection_percent: Number(req.selection_percent) || 0,
    heritability: Number(req.heritability) || 0,
    mutation_rate: Number(req.mutation_rate) || 0,
    crispr_enabled: Boolean(req.crispr_enabled),
    crispr_edits: Number(req.crispr_edits) || 0,
    crispr_intro_percent: Number(req.crispr_intro_percent) || 0,
    strategy_set: String(req.strategy_set || 'core'),
    tracked_strategy: String(req.tracked_strategy || ''),
    replicates: Number(req.replicates) || 0,
    worker_count: Number(req.worker_count) || 0,
    inbreeding_limit: Number(req.inbreeding_limit) || 0,
    diversity_loss_limit: Number(req.diversity_loss_limit) || 0,
    max_inbreeding: Number(req.max_inbreeding) || 0,
    max_diversity_loss: Number(req.max_diversity_loss) || 0,
    max_rare_useful_loss: Number(req.max_rare_useful_loss) || 0,
    min_genetic_gain: Number(req.min_genetic_gain) || 0,
    min_effective_parents: Number(req.min_effective_parents) || 0,
    max_combined_risk: Number(req.max_combined_risk) || 0
  });
}

function changedParams(prevReq, curReq) {
  const fields = [
    ['dataset', 'dataset'],
    ['seed', 'seed'],
    ['population_size', 'N'],
    ['markers', 'markers'],
    ['qtl_count', 'QTL'],
    ['generations', 'generations'],
    ['selection_percent', 'selection %'],
    ['heritability', 'heritability'],
    ['mutation_rate', 'mutation rate'],
    ['crispr_enabled', 'CRISPR enabled'],
    ['crispr_edits', 'CRISPR edits'],
    ['crispr_intro_percent', 'edit intro %'],
    ['strategy_set', 'strategy set'],
    ['tracked_strategy', 'tracked strategy'],
    ['replicates', 'replicates'],
    ['worker_count', 'worker count'],
    ['inbreeding_limit', 'inbreeding limit'],
    ['diversity_loss_limit', 'diversity loss limit'],
    ['max_inbreeding', 'max inbreeding'],
    ['max_diversity_loss', 'max diversity loss'],
    ['max_rare_useful_loss', 'max rare-useful loss'],
    ['min_genetic_gain', 'min genetic gain'],
    ['min_effective_parents', 'min effective parents'],
    ['max_combined_risk', 'max combined risk']
  ];
  const out = [];
  for (const [key, label] of fields) {
    const a = prevReq ? prevReq[key] : undefined;
    const b = curReq ? curReq[key] : undefined;
    if (String(a) !== String(b)) out.push(`${label}: ${formatParamValue(a)} → ${formatParamValue(b)}`);
  }
  return out;
}

function combinedRisk(final) {
  if (!final) return 0;
  return 0.45 * (Number(final.probability_inbreeding_breach) || 0) +
         0.35 * (Number(final.probability_diversity_collapse) || 0) +
         0.20 * (Number(final.probability_rare_useful_loss) || 0);
}

function expectedMutationFlips(req) {
  return (Number(req.population_size) || 0) * (Number(req.markers) || 0) * 2 * (Number(req.generations) || 0) * (Number(req.mutation_rate) || 0);
}

// Mirrors buildStrategyConfigs() in main.go. Keep in sync if strategies change.
function strategyCountForRequest(req) {
  const advanced = String(req.strategy_set || 'core') === 'advanced';
  const base = advanced ? 9 : 4;
  const crisprActive = Boolean(req.crispr_enabled) && (Number(req.crispr_edits) || 0) > 0;
  if (!crisprActive) return base;
  return base + (advanced ? 2 : 1);
}

function runBudgetCells(req) {
  const n = Number(req.population_size) || 0;
  const m = Number(req.markers) || 0;
  const g = (Number(req.generations) || 0) + 1;
  const s = strategyCountForRequest(req);
  const r = Math.max(1, Number(req.replicates) || 0);
  return n * m * g * s * r;
}

function formatCells(n) {
  if (!Number.isFinite(n) || n <= 0) return '0';
  if (n >= 1e9) return (n / 1e9).toFixed(n >= 1e10 ? 1 : 2) + 'B';
  if (n >= 1e6) return (n / 1e6).toFixed(n >= 1e8 ? 0 : 1) + 'M';
  if (n >= 1e3) return (n / 1e3).toFixed(0) + 'k';
  return String(Math.round(n));
}

// Returns { over: bool } so callers (runSimulation) can pre-flight the request.
function updateBudgetMeter() {
  const el = byId('budgetMeter');
  if (!el) return { over: false };
  const req = requestFromForm();
  const budget = runBudgetCells(req);
  const strategies = strategyCountForRequest(req);
  const over = budget > BUDGET_CAP;
  const warn = !over && budget > BUDGET_CAP * BUDGET_WARN_FRAC;
  el.classList.remove('warn', 'over');
  if (over) el.classList.add('over');
  else if (warn) el.classList.add('warn');
  const formula = `${req.population_size || 0} N × ${req.markers || 0} markers × ${(Number(req.generations) || 0) + 1} gens × ${strategies} strategies × ${Math.max(1, Number(req.replicates) || 0)} reps`;
  let hint;
  if (over) {
    hint = `Over the ${formatCells(BUDGET_CAP)} cap — Run is disabled. Reduce population, markers, generations, or replicates.`;
  } else if (warn) {
    hint = `Near the ${formatCells(BUDGET_CAP)} cap — close to MVP limit.`;
  } else {
    hint = `MVP cap is ${formatCells(BUDGET_CAP)} cells per run.`;
  }
  el.innerHTML = `Run budget: <strong>${formatCells(budget)} cells</strong> <small>${formula}</small><small>${hint}</small>`;
  const btn = byId('runBtn');
  if (btn) {
    if (over) btn.setAttribute('disabled', 'disabled');
    else btn.removeAttribute('disabled');
  }
  // Highlight every input that multiplies into the budget, so the user can
  // see at a glance which knobs to turn down. We don't single one out — they
  // all contribute multiplicatively, any of them works.
  for (const id of BUDGET_INPUT_IDS) {
    const input = byId(id);
    if (!input) continue;
    if (over) input.classList.add('over-budget');
    else input.classList.remove('over-budget');
  }
  return { over };
}

function formatParamValue(v) {
  if (v === null || v === undefined) return '-';
  if (typeof v === 'boolean') return v ? 'yes' : 'no';
  const n = Number(v);
  if (!Number.isFinite(n)) return String(v);
  if (Math.abs(n) > 0 && Math.abs(n) < 0.0001) return n.toExponential(3);
  if (Math.abs(n) < 1) return n.toFixed(8).replace(/0+$/,'').replace(/\.$/,'');
  return fmt(n);
}

function markDirty(reason) {
  updateBudgetMeter();
  const req = requestFromForm();
  const currentSig = currentData ? requestSignature(currentData.request || {}) : '';
  const nextSig = requestSignature(req);
  if (currentData && nextSig === currentSig) {
    setStatus('Parameters match the current run.', 'ok');
    return;
  }
  const prefix = reason || 'Parameters changed';
  setStatus(`${prefix}. Press Run simulation to update graphs.`, 'dirty');
}

function findStrategy(data, code) {
  if (!data || !code) return null;
  return (data.strategies || []).find(x => x.code === code) || null;
}

function maxBy(arr, fn) {
  let best = null;
  let bestVal = -Infinity;
  for (const x of arr || []) {
    const v = fn(x);
    if (v > bestVal) { best = x; bestVal = v; }
  }
  return best;
}

function minBy(arr, fn) {
  let best = null;
  let bestVal = Infinity;
  for (const x of arr || []) {
    const v = fn(x);
    if (v < bestVal) { best = x; bestVal = v; }
  }
  return best;
}

function fmt(v) {
  if (v === null || v === undefined || Number.isNaN(Number(v))) return '-';
  const n = Number(v);
  if (Math.abs(n) >= 100) return n.toFixed(1);
  if (Math.abs(n) >= 10) return n.toFixed(2);
  return n.toFixed(4).replace(/0+$/,'').replace(/\.$/,'');
}

function signedDelta(v) {
  if (v === null || v === undefined || Number.isNaN(Number(v))) return '-';
  const n = Number(v);
  const sign = n > 0 ? '+' : '';
  return sign + fmt(n);
}

function formatInt(v) {
  const n = Number(v);
  if (!Number.isFinite(n)) return '-';
  return Math.round(n).toLocaleString('en-US');
}

function escapeHtml(s) {
  return String(s)
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#039;');
}

// v0.7.16 — sensitivity sweep client. Mirrors the API shape exposed by
// sensitivity.go. Defaults per axis are chosen to bracket common breeder
// questions ("what if h² is lower than assumed?", "what if we cut selection
// intensity?", "what if we only have 10 generations?"). 5 values max because
// the server caps at sensitivityMaxValues and the table stays readable.
const SENS_DEFAULT_VALUES = {
  heritability:      '0.2, 0.35, 0.5, 0.65, 0.8',
  selection_percent: '5, 10, 15, 20, 30',
  generations:       '10, 20, 30, 50, 80'
};
const SENS_AXIS_LABEL = {
  heritability:      'h²',
  selection_percent: 'sel %',
  generations:       'gens'
};
const SENS_AXIS_FORMAT = {
  heritability:      v => v.toFixed(2),
  selection_percent: v => v.toFixed(0) + '%',
  generations:       v => String(Math.round(v))
};
let sensRunInFlight = false;

function parseSensValues(text) {
  return String(text || '')
    .split(/[,\s]+/)
    .map(s => s.trim().replace(',', '.'))
    .filter(s => s.length > 0)
    .map(s => Number(s))
    .filter(v => Number.isFinite(v));
}

function sensitivityCostCells(req, values) {
  const single = runBudgetCells(req);
  return single * Math.max(1, values.length);
}

function updateSensBudgetMeter() {
  const el = byId('sensBudget');
  if (!el) return { over: false };
  const axis = byId('sensAxis') ? byId('sensAxis').value : 'heritability';
  const values = parseSensValues(byId('sensValues') ? byId('sensValues').value : '');
  const req = requestFromForm();
  const sweep = sensitivityCostCells(req, values);
  const over = sweep > BUDGET_CAP;
  const warn = !over && sweep > BUDGET_CAP * BUDGET_WARN_FRAC;
  el.classList.remove('warn', 'over');
  if (over) el.classList.add('over');
  else if (warn) el.classList.add('warn');
  const single = runBudgetCells(req);
  const formula = `${values.length || 0} scenarios × ${formatCells(single)} cells per run`;
  let hint;
  if (values.length === 0) {
    hint = 'Enter 1–5 axis values.';
  } else if (values.length > 5) {
    hint = `Only 5 values per sweep are allowed (got ${values.length}).`;
  } else if (over) {
    hint = `Over the ${formatCells(BUDGET_CAP)} cap — reduce number of values, or shrink base run.`;
  } else if (warn) {
    hint = `Near the ${formatCells(BUDGET_CAP)} cap.`;
  } else {
    hint = `Sum stays within the ${formatCells(BUDGET_CAP)} cap.`;
  }
  el.innerHTML = `Sweep budget: <strong>${formatCells(sweep)} cells</strong> <small>${formula}</small><small>${hint}</small>`;
  const btn = byId('sensRunBtn');
  if (btn) {
    if (over || values.length === 0 || values.length > 5 || sensRunInFlight) btn.setAttribute('disabled', 'disabled');
    else btn.removeAttribute('disabled');
  }
  return { over, values };
}

function setSensStatus(text, cls) {
  const el = byId('sensStatus');
  if (!el) return;
  el.textContent = text || '';
  el.className = 'status ' + (cls || '');
}

async function runSensitivitySweep() {
  if (sensRunInFlight) return;
  const axis = byId('sensAxis').value;
  const meter = updateSensBudgetMeter();
  if (meter.over) {
    setSensStatus(`Sweep budget exceeds the ${formatCells(BUDGET_CAP)} cap. Reduce number of values or shrink base run.`, 'error');
    return;
  }
  if (!meter.values || meter.values.length === 0) {
    setSensStatus('Enter at least one axis value (comma-separated).', 'error');
    return;
  }
  sensRunInFlight = true;
  const btn = byId('sensRunBtn');
  const originalLabel = btn ? btn.textContent : '';
  if (btn) { btn.disabled = true; btn.textContent = 'Starting...'; }
  // Hide previous result so the user doesn't conflate runs.
  byId('sensVerdict').hidden = true;
  byId('sensTableWrap').hidden = true;
  setSensStatus('Starting sweep...', '');
  try {
    const startRes = await fetch('/api/sensitivity/start', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({base: requestFromForm(), axis: axis, values: meter.values})
    });
    if (!startRes.ok) {
      const text = await startRes.text();
      throw new Error(text || `HTTP ${startRes.status}`);
    }
    const start = await startRes.json();
    let job = null;
    for (;;) {
      const res = await fetch('/api/sensitivity/status?id=' + encodeURIComponent(start.job_id), {cache: 'no-store'});
      if (!res.ok) { throw new Error(await res.text() || `HTTP ${res.status}`); }
      job = await res.json();
      setSensStatus(`${job.message || 'Running'} — ${job.percent || 0}%`, '');
      if (btn) btn.textContent = `Sweep ${job.percent || 0}%`;
      if (job.done) break;
      await sleep(150);
    }
    if (job.error) throw new Error(job.error);
    if (!job.result) throw new Error('Sweep finished without a result');
    renderSensResult(axis, job.result);
    setSensStatus('Sweep complete.', 'ok');
  } catch (err) {
    console.error(err);
    setSensStatus(err.message || String(err), 'error');
  } finally {
    sensRunInFlight = false;
    if (btn) { btn.disabled = false; btn.textContent = originalLabel || 'Run sweep'; }
    updateSensBudgetMeter();
  }
}

function renderSensResult(axis, result) {
  const verdictEl = byId('sensVerdict');
  const wrap = byId('sensTableWrap');
  const body = byId('sensTable');
  const summary = result.summary || {};
  const verdict = summary.verdict || 'inconclusive';
  verdictEl.classList.remove('stable', 'fragile', 'inconclusive');
  verdictEl.classList.add(verdict);
  const verdictLabel = verdict.toUpperCase();
  const notes = Array.isArray(summary.notes) ? summary.notes.join(' ') : '';
  verdictEl.innerHTML = `<strong>${verdictLabel}</strong> — ${escapeHtml(notes)}`;
  verdictEl.hidden = false;
  const fmt = SENS_AXIS_FORMAT[axis] || (v => String(v));
  const rows = (result.scenarios || []).map(s => {
    const matchCell = s.best_feasible_code === ''
      ? '<span class="sens-match-no">infeasible</span>'
      : (s.baseline_match
          ? '<span class="sens-match-yes">yes</span>'
          : `<span class="sens-match-no">no (→ ${escapeHtml(s.best_feasible_code)})</span>`);
    return '<tr>'
      + `<td>${fmt(Number(s.axis_value))}</td>`
      + `<td>${escapeHtml(s.best_feasible_name || s.best_feasible_code || '—')}</td>`
      + `<td>${signedDelta(s.genetic_gain)}</td>`
      + `<td>${Number(s.diversity).toFixed(3)}</td>`
      + `<td>${Number(s.inbreeding).toFixed(3)}</td>`
      + `<td>${Number(s.combined_risk).toFixed(3)}</td>`
      + `<td>${matchCell}</td>`
      + '</tr>';
  }).join('');
  body.innerHTML = rows;
  wrap.hidden = false;
}

window.addEventListener('DOMContentLoaded', () => {
  const runBtn = byId('runBtn');
  if (!runBtn) return;
  runBtn.addEventListener('click', runSimulation);
  byId('clearCompareBtn').addEventListener('click', clearComparison);
  byId('randomSeedBtn').addEventListener('click', randomizeSeed);
  byId('exportJsonBtn').addEventListener('click', exportJSON);
  byId('copySummaryBtn').addEventListener('click', copySummary);
  document.querySelectorAll('[data-preset]').forEach(btn => {
    btn.addEventListener('click', () => applyPreset(btn.getAttribute('data-preset')));
  });
  document.querySelectorAll('input, select').forEach(input => {
    input.addEventListener('change', () => markDirty(`${input.id} changed`));
    // 'input' fires while typing — keeps the budget meter reactive without
    // waiting for blur. markDirty itself stays on 'change' so the status text
    // doesn't churn on every keystroke.
    input.addEventListener('input', () => {
      updateBudgetMeter();
      updateSensBudgetMeter();
    });
  });
  updateBudgetMeter();
  // v0.7.16 sensitivity panel.
  const sensAxis = byId('sensAxis');
  const sensValues = byId('sensValues');
  const sensRunBtn = byId('sensRunBtn');
  if (sensAxis && sensValues && sensRunBtn) {
    sensAxis.addEventListener('change', () => {
      sensValues.value = SENS_DEFAULT_VALUES[sensAxis.value] || '';
      updateSensBudgetMeter();
    });
    sensValues.addEventListener('input', updateSensBudgetMeter);
    sensRunBtn.addEventListener('click', runSensitivitySweep);
    updateSensBudgetMeter();
  }
  setStatus('Ready. Press Run simulation to calculate.', '');
});
