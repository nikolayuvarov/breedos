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
    replicates: Math.trunc(numberValue('replicates')),
    worker_count: Math.trunc(numberValue('worker_count')),
    inbreeding_limit: numberValue('inbreeding_limit'),
    diversity_loss_limit: numberValue('diversity_loss_limit')
  };
}

function setFormValues(values) {
  for (const [id, value] of Object.entries(values)) {
    const el = byId(id);
    if (!el) continue;
    if (el.type === 'checkbox') el.checked = Boolean(value);
    else el.value = String(value);
  }
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
  const originalText = btn ? btn.textContent : '';
  if (btn) {
    btn.disabled = true;
    setRunButtonProgress(0, 'Starting');
  }
  setStatus('Starting simulation job...', '');
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
      const statusRes = await fetch('/api/simulate/status?id=' + encodeURIComponent(start.job_id), {cache: 'no-store'});
      if (!statusRes.ok) {
        const text = await statusRes.text();
        throw new Error(text || `HTTP ${statusRes.status}`);
      }
      job = await statusRes.json();
      const percent = Number.isFinite(Number(job.percent)) ? Number(job.percent) : 0;
      setRunButtonProgress(percent, job.done ? 'Finishing' : 'Running');
      setStatus(`${job.message || 'Running'} — ${Math.round(percent)}%`, '');
      if (job.done) break;
      await sleep(120);
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
  el.innerHTML = `
    ${honesty}
    <div class="decision-grid">
      <div class="decision-card"><span>Recommended</span><strong>${escapeHtml(best ? (labels[best.code] || best.name) : '-')}</strong><small>risk-adjusted score ${best ? fmt(best.final.risk_adjusted_score) : '-'}</small></div>
      <div class="decision-card"><span>Max gain</span><strong>${escapeHtml(bestGain ? (labels[bestGain.code] || bestGain.name) : '-')}</strong><small>${bestGain ? fmt(bestGain.final.genetic_gain) : '-'} final gain</small></div>
      <div class="decision-card"><span>Lowest risk</span><strong>${escapeHtml(lowest ? (labels[lowest.code] || lowest.name) : '-')}</strong><small>${lowest ? fmt(combinedRisk(lowest.final)) : '-'} combined risk</small></div>
    </div>
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
    byId('strategyTable').innerHTML = '<tr><td colspan="13">No strategies returned.</td></tr>';
    return;
  }
  byId('strategyTable').innerHTML = strategies.map(s => {
    const p = findStrategy(prev, s.code);
    const delta = p ? [
      `gain ${signedDelta(s.final.genetic_gain - p.final.genetic_gain)}`,
      `div ${signedDelta(s.final.diversity - p.final.diversity)}`,
      `F ${signedDelta(s.final.inbreeding - p.final.inbreeding)}`
    ].join('<br>') : '-';
    return `
      <tr>
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
    replicates: Number(req.replicates) || 0,
    worker_count: Number(req.worker_count) || 0,
    inbreeding_limit: Number(req.inbreeding_limit) || 0,
    diversity_loss_limit: Number(req.diversity_loss_limit) || 0
  });
}

function changedParams(prevReq, curReq) {
  const fields = [
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
    ['replicates', 'replicates'],
    ['worker_count', 'worker count'],
    ['inbreeding_limit', 'inbreeding limit'],
    ['diversity_loss_limit', 'diversity loss limit']
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
  });
  setStatus('Ready. Press Run simulation to calculate.', '');
});
