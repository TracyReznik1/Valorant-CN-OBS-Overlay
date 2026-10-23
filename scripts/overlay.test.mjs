// SPDX-License-Identifier: AGPL-3.0-or-later
// Optional frontend regression checks: node --test scripts/overlay.test.mjs
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {test} from 'node:test';
import vm from 'node:vm';

const html = readFileSync(new URL('../src/cmd/valorant-overlay/static/overlay.html', import.meta.url), 'utf8');
const script = html.match(/<script>([\s\S]*?)<\/script>/)[1];
const good = {rank: '神话II', rr: '350', today_wins: 2, today_losses: 1, last_rr_change: '+22'};

async function page() {
  const elements = new Map();
  function element(id) {
    if (!elements.has(id)) {
      const classes = new Set();
      elements.set(id, {
        style: {}, textContent: '', title: '', hidden: false, src: '', dataset: {},
        removeAttribute(name) { delete this[name]; },
        classList: {
          add(...names) { names.forEach(name => classes.add(name)); },
          remove(...names) { names.forEach(name => classes.delete(name)); },
          toggle(name, on) { if (on) classes.add(name); else classes.delete(name); },
          contains(name) { return classes.has(name); },
        },
      });
    }
    return elements.get(id);
  }
  let response = async () => ({ok: true, json: async () => good});
  let interval;
  const context = vm.createContext({
    document: {getElementById: element, querySelector: element},
    fetch: async (url, options) => {
      assert.equal(url, '/api/state');
      assert.equal(options.cache, 'no-store');
      return response();
    },
    setInterval(callback, ms) { interval = ms; },
  });
  vm.runInContext(script, context);
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(interval, 3000);
  return {
    element,
    refresh: () => vm.runInContext('refresh()', context),
    respond(fn) { response = fn; },
  };
}

test('HTTP errors, invalid JSON and network failures retain display; recovery clears yellow dot', async () => {
  for (const fail of [
    async () => ({ok: false, status: 503}),
    async () => ({ok: true, json: async () => { throw new Error('invalid JSON'); }}),
    async () => { throw new Error('offline'); },
  ]) {
    const p = await page();
    assert.equal(p.element('rr-value').textContent, '350 分');
    p.respond(fail);
    await p.refresh();
    assert.equal(p.element('rank-card').classList.contains('stale'), true);
    assert.equal(p.element('rr-value').textContent, '350 分');
    assert.equal(p.element('.stale-indicator').title, '本机状态读取失败，保留上次显示');
    p.respond(async () => ({ok: true, json: async () => ({...good, rr: '351'})}));
    await p.refresh();
    assert.equal(p.element('rank-card').classList.contains('stale'), false);
    assert.equal(p.element('rr-value').textContent, '351 分');
  }
});

test('API stale and RR parse warning are distinct, visible, and cleared on recovery', async () => {
  const p = await page();
  for (const diagnostic of [
    {stale: true, error: '[val_card] HTTP 502'},
    {stale: false, rr: '--', warning: '[rr_parse] 当前 RR 缺失'},
  ]) {
    p.respond(async () => ({ok: true, json: async () => ({...good, ...diagnostic})}));
    await p.refresh();
    assert.equal(p.element('rank-card').classList.contains('stale'), true);
    assert.equal(p.element('.stale-indicator').title, diagnostic.error || diagnostic.warning);
  }
  p.respond(async () => ({ok: true, json: async () => good}));
  await p.refresh();
  assert.equal(p.element('rank-card').classList.contains('stale'), false);
  assert.equal(p.element('last-change').textContent, '上一局 +22 分');
  assert.equal(p.element('today-record').textContent, '今日 2 胜 / 1 负');
});

test('last-match signs, zero, icon and partial record keep existing behaviour', async () => {
  const p = await page();
  for (const [change, result] of [['+22', 'win'], ['-18', 'loss'], ['0', 'draw'], ['--', 'unknown']]) {
    p.respond(async () => ({ok: true, json: async () => ({...good, last_rr_change: change, today_partial: true, rank_icon: 'https://example.test/icon.png'})}));
    await p.refresh();
    assert.equal(p.element('last-change').textContent, `上一局 ${change === '--' ? '--' : `${change} 分`}`);
    assert.equal(p.element('rank-card').dataset.result, result);
    assert.equal(p.element('today-record').classList.contains('partial'), true);
    assert.equal(p.element('rank-icon').src, 'https://example.test/icon.png');
  }

  // The API's last_result wins over the RR sign for the card/background theme.
  p.respond(async () => ({ok: true, json: async () => ({...good, last_result: '失败', last_rr_change: '+22'})}));
  await p.refresh();
  assert.equal(p.element('rank-card').dataset.result, 'loss');
});

test('game rank icon badge, last-known icon and tier accent', async () => {
  const p = await page();
  p.respond(async () => ({ok: true, json: async () => ({...good, rank: '铂金I', rr: '82', rank_icon: ''})}));
  await p.refresh();
  assert.equal(p.element('rank-placeholder').hidden, false);
  assert.equal(p.element('rank-icon').hidden, true);
  assert.equal(p.element('rank-card').dataset.tier, 'platinum');

  p.respond(async () => ({ok: true, json: async () => ({...good, rank: '辐能战魂', rr: '1250', rank_icon: 'https://example.test/radiant.png'})}));
  await p.refresh();
  assert.equal(p.element('rank-placeholder').hidden, true);
  assert.equal(p.element('rank-icon').hidden, false);
  assert.equal(p.element('rank-card').dataset.tier, 'radiant');

  // A transient missing icon for the same rank keeps the last game icon.
  p.respond(async () => ({ok: true, json: async () => ({...good, rank: '辐能战魂', rr: '1250', rank_icon: ''})}));
  await p.refresh();
  assert.equal(p.element('rank-icon').hidden, false);
  assert.equal(p.element('rank-placeholder').hidden, true);

  p.element('rank-icon').onerror();
  assert.equal(p.element('rank-icon').hidden, true);
  assert.equal(p.element('rank-placeholder').hidden, false);

  await p.refresh();
  assert.equal(p.element('rank-icon').hidden, true);
  assert.equal(p.element('rank-placeholder').hidden, false);
});

test('win and loss streak badges trigger card animation state', async () => {
  const p = await page();
  p.respond(async () => ({ok: true, json: async () => ({...good, streak_type: 'win', streak_count: 3})}));
  await p.refresh();
  assert.equal(p.element('streak-badge').hidden, false);
  assert.equal(p.element('streak-badge').textContent, '3 连胜');
  assert.equal(p.element('streak-badge').className, 'streak-badge win');
  assert.equal(p.element('rank-card').dataset.streak, 'win');

  p.respond(async () => ({ok: true, json: async () => ({...good, streak_type: 'loss', streak_count: 2})}));
  await p.refresh();
  assert.equal(p.element('streak-badge').textContent, '2 连败');
  assert.equal(p.element('streak-badge').className, 'streak-badge loss');
  assert.equal(p.element('rank-card').dataset.streak, 'loss');

  p.respond(async () => ({ok: true, json: async () => ({...good, streak_type: 'win', streak_count: 1})}));
  await p.refresh();
  assert.equal(p.element('streak-badge').hidden, true);
  assert.equal(p.element('rank-card').dataset.streak, 'none');
});

test('rank/RR matrix preserves the independent API values through the actual page script', async t => {
  let cases;
  if (process.env.VALORANT_OVERLAY_MATRIX_RESULTS) {
    // Consume the actual HTTP-state JSON emitted by the Go integration matrix.
    const report = JSON.parse(readFileSync(process.env.VALORANT_OVERLAY_MATRIX_RESULTS, 'utf8'));
    assert.equal(report.synthetic, true);
    assert.equal(report.count, 1501);
    assert.equal(report.cases.length, report.count);
    cases = report.cases;
  } else {
    // Standalone frontend check, sharing the same declared fixture data.
    const fixture = JSON.parse(readFileSync(new URL('../src/cmd/valorant-overlay/testdata/rank_rr_matrix.json', import.meta.url), 'utf8'));
    assert.equal(fixture.synthetic, true);
    cases = fixture.groups.flatMap(group => group.ranks.flatMap(rank => group.points.map(rr => ({
      case_id: `${group.id}/${rank}/${rr}`,
      expected_rank: rank, expected_rr: String(rr),
      state: {...good, rank, rr: String(rr), today_wins: 1, today_losses: 1},
    }))));
  }
  const p = await page();
  for (const item of cases) {
    p.respond(async () => ({ok: true, json: async () => item.state}));
    await p.refresh();
    assert.equal(p.element('rank-name').textContent, item.expected_rank, item.case_id);
    assert.equal(p.element('rr-value').textContent, `${item.expected_rr} 分`, item.case_id);
    assert.equal(p.element('rank-card').classList.contains('stale'), false, item.case_id);
    assert.equal(p.element('today-record').textContent, '今日 1 胜 / 1 负', item.case_id);
    assert.equal(p.element('last-change').textContent, '上一局 +22 分', item.case_id);
  }
  t.diagnostic(`checked ${cases.length} rank/RR states; no rank inferred from RR`);
});
