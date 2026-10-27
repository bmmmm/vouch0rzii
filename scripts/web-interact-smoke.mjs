#!/usr/bin/env node
// Gate: the admin matrix works under clicks and keys. Drives headless Chrome
// over the DevTools protocol on a pipe (fds 3 and 4; no WebSocket, no
// dependencies): swaps two slots, undoes and redoes with the keyboard, turns
// pending roster edits into an undo step, names a free cell with a new
// handle, an existing slot and "handle+k", and keeps a rejected name from
// touching the plan. Prints SKIP and exits 0 when no Chrome is found; set
// CHROME=/path/to/chrome.
//
// Usage: node scripts/web-interact-smoke.mjs [path/to/vouch0rzii-binary]
// Requires a prior `go build -o vouch0rzii .` and `scripts/build-web.sh`.
import { execFileSync, spawn } from 'node:child_process';
import { existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const root = resolve(fileURLToPath(new URL('..', import.meta.url)));
const page = pathToFileURL(join(root, 'web', 'dist', 'index.html')).href;
const cli = resolve(process.argv[2] || join(root, 'vouch0rzii'));
const chrome = process.env.CHROME || [
  '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
  '/usr/bin/google-chrome', '/usr/bin/chromium-browser', '/usr/bin/chromium',
].find(existsSync);
if (!chrome) {
  console.log('SKIP  no Chrome found (set CHROME=/path/to/chrome)');
  process.exit(0);
}
const link = execFileSync(cli, ['-event', join(root, 'events', 'congress2026.json'), '-share', 'admin', '-base', page]).toString().trim();

// --- minimal DevTools client over --remote-debugging-pipe
const profile = mkdtempSync(join(tmpdir(), 'vouch0rzii-cdp-'));
const proc = spawn(chrome, [
  '--headless=new', '--disable-gpu', '--no-sandbox', '--allow-file-access-from-files',
  '--remote-debugging-pipe', `--user-data-dir=${profile}`, '--accept-lang=de', 'about:blank', // the page follows the browser language
], { stdio: ['ignore', 'ignore', 'ignore', 'pipe', 'pipe'] });
const pending = new Map();
let nextId = 0, buf = '';
proc.stdio[4].setEncoding('utf8');
proc.stdio[4].on('data', (chunk) => {
  buf += chunk;
  let i;
  while ((i = buf.indexOf('\0')) >= 0) {
    const msg = JSON.parse(buf.slice(0, i));
    buf = buf.slice(i + 1);
    const p = pending.get(msg.id);
    if (!p) continue;
    pending.delete(msg.id);
    msg.error ? p.reject(new Error(JSON.stringify(msg.error))) : p.resolve(msg.result);
  }
});
proc.on('exit', (code) => { for (const p of pending.values()) p.reject(new Error(`chrome exited (${code})`)); pending.clear(); });
const send = (method, params = {}, sessionId) => new Promise((resolve, reject) => {
  const id = ++nextId;
  const timer = setTimeout(() => { pending.delete(id); reject(new Error(`${method}: no answer within 15s`)); }, 15000);
  pending.set(id, { resolve: (v) => { clearTimeout(timer); resolve(v); }, reject: (e) => { clearTimeout(timer); reject(e); } });
  proc.stdio[3].write(JSON.stringify({ id, method, params, sessionId }) + '\0');
});
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

let failures = 0;
const check = (name, cond, detail = '') => {
  console.log(`${cond ? 'PASS' : 'FAIL'}  ${name}${cond || !detail ? '' : `\n      ${detail}`}`);
  if (!cond) failures++;
};

try {
  const { targetId } = await send('Target.createTarget', { url: link });
  const { sessionId } = await send('Target.attachToTarget', { targetId, flatten: true });
  const js = async (expression) => {
    const r = await send('Runtime.evaluate', { expression, returnByValue: true }, sessionId);
    if (r.exceptionDetails) throw new Error(r.exceptionDetails.exception?.description || r.exceptionDetails.text);
    return r.result.value;
  };
  let ready = false;
  for (let i = 0; i < 400 && !ready; i++) {
    ready = await js('!!globalThis.vouch0rzii && !document.getElementById("result").hidden');
    if (!ready) await sleep(50);
  }
  check('page: plan rendered from the admin link', ready);

  // Helpers run inside the page. Rows are vouchers, the first column a <th>.
  const cell = (d, v) => `document.querySelector('#matrix tbody tr:nth-child(${v}) td:nth-child(${d + 1})')`;
  const text = (d, v) => js(`${cell(d, v)}.firstChild.nodeValue.trim()`);
  const click = (d, v) => js(`${cell(d, v)}.click(); 0`);
  const key = (k, shift = false, on = 'document.body') => js(`${on}.dispatchEvent(new KeyboardEvent('keydown', { key: '${k}', ctrlKey: true, shiftKey: ${shift}, bubbles: true, cancelable: true })); 0`);
  const stats = () => js(`document.getElementById('r-stats').textContent`);
  const roster = () => js(`document.getElementById('roster').value`);
  const disabled = (id) => js(`document.getElementById('${id}').disabled`);
  const pinnedLabels = () => js(`[...document.querySelectorAll('#matrix td.cell.pinned')].map((td) => td.firstChild.nodeValue)`);
  const openFreeCell = () => js(`(() => { const td = document.querySelector('#matrix td.cell.empty'); td.click(); return !!td.querySelector('input.cellname'); })()`);
  const typeInCell = (label) => js(`(() => {
    const input = document.querySelector('#matrix input.cellname');
    input.value = ${JSON.stringify(label)};
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
    return 'ok';
  })()`);
  const nameFreeCell = async (label) => (await openFreeCell()) ? typeInCell(label) : 'no input';

  const a = await text(1, 1), b = await text(1, 2);
  check('matrix: two different slots on day 1', a && b && a !== b, `${a} / ${b}`);
  await click(1, 1);
  await click(1, 2);
  check('swap: click source, click target swaps and pins both', (await text(1, 1)) === b && (await text(1, 2)) === a && (await stats()).includes('2 gepinnt'));
  await key('z');
  check('undo: Ctrl+Z restores the plan without pins', (await text(1, 1)) === a && !(await stats()).includes('gepinnt'));
  await key('z', true);
  check('redo: Ctrl+Shift+Z swaps again', (await text(1, 1)) === b && (await stats()).includes('2 gepinnt'));
  await key('z');
  check('undo again: back to the plain plan, undo button off, redo button on', (await text(1, 1)) === a && (await disabled('undo')) && !(await disabled('redo')));
  await click(1, 1);
  await click(1, 2);
  check('new action after undo clears the redo stack', (await text(1, 1)) === b && (await disabled('redo')));
  await key('z');

  // Pending roster edits become an undo step of their own.
  await js(`document.getElementById('roster').value += '\\tdodo2\\tBekanntschaft\\t\\t\\t\\t\\t\\t\\n'; 0`);
  await key('z');
  check('undo with pending roster edits: edits applied, then taken back', !(await roster()).includes('dodo2') && (await stats()).includes('47 Slots'));
  await key('z', true);
  check('redo brings the roster edit back as a plan', (await roster()).includes('dodo2') && (await stats()).includes('48 Slots'));
  await key('z');
  check('and undo removes it again', (await stats()).includes('47 Slots'));

  // A real mouse click into the open input must not close it.
  check('free cell: click opens an input', await openFreeCell());
  const { x, y } = await js(`(() => { const i = document.querySelector('#matrix input.cellname'); i.value = 'do'; i.scrollIntoView({ block: 'center' }); const r = i.getBoundingClientRect(); return { x: r.x + r.width / 2, y: r.y + r.height / 2 }; })()`);
  for (const type of ['mousePressed', 'mouseReleased']) await send('Input.dispatchMouseEvent', { type, x, y, button: 'left', clickCount: 1 }, sessionId);
  check('free cell: a real click into the input keeps it, its text and the focus', await js(`document.activeElement === document.querySelector('#matrix input.cellname') && document.activeElement.value === 'do'`));
  await js(`document.querySelector('#matrix input.cellname').dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })); 0`);
  check('free cell: Escape closes the input and keeps keyboard focus in the matrix', await js(`!document.querySelector('#matrix input.cellname') && document.activeElement.matches('#matrix td.cell')`));

  check('free cell: typing a new handle adds a person and pins it', (await nameFreeCell('dodo')) === 'ok' && (await stats()).includes('48 Slots') && (await stats()).includes('1 gepinnt') && (await roster()).includes('dodo') && (await pinnedLabels()).includes('dodo'));
  await key('z');
  check('undo: the new person is gone from plan and roster', (await stats()).includes('47 Slots') && !(await roster()).includes('dodo'));
  check('free cell: an existing slot label moves there', (await nameFreeCell('mammut+10')) === 'ok' && (await stats()).includes('47 Slots') && (await stats()).includes('1 gepinnt') && (await pinnedLabels()).includes('mammut+10'));
  check('free cell: "handle+k" raises the +N', (await nameFreeCell('kiwi+1')) === 'ok' && (await stats()).includes('48 Slots') && /\tkiwi\t[^\t]*\t1\t/.test(await roster()));
  check('rejected name: plan and undo buttons stay, the reason is shown above the matrix', (await nameFreeCell('kiwi+3')) === 'ok' && (await stats()).includes('48 Slots')
    && await js(`!document.getElementById('result').hidden && !document.getElementById('undo').disabled && /exceeds the limit/.test(document.querySelector('#messages .error')?.textContent || '') && document.getElementById('messages').previousElementSibling === document.getElementById('r-pinhelp')`));
  check('free cell: handles match case-insensitively (fills the last free cell)', (await nameFreeCell('KIWI+2')) === 'ok' && (await stats()).includes('49 Slots') && /\tkiwi\t[^\t]*\t2\t/.test(await roster()) && !/\tKIWI\t/.test(await roster()));
  await key('z', false, "document.getElementById('roster')");
  check('Ctrl+Z inside the roster textarea is left to the browser', (await stats()).includes('49 Slots'));

  // Bulk status: tick two rows, set "Ticket gekauft" in one step, undo both at once.
  const count = () => js(`document.getElementById('sheets-count').textContent`);
  const badges = (st) => js(`document.querySelectorAll('#sheets tbody tr.st-${st}').length`);
  await js(`[...document.querySelectorAll('#sheets input.pick')].slice(0, 2).forEach((c) => c.click()); 0`);
  check('bulk: two ticked rows are counted', await js(`document.getElementById('pickCount').textContent.startsWith('2 ausgewählt') && !document.querySelector('#bulk button[data-st="bought"]').disabled`));
  await js(`document.querySelector('#bulk button[data-st="bought"]').click(); 0`);
  check('bulk: "Ticket gekauft" set on both, rows tinted, count updated', (await badges('bought')) === 2 && (await count()) === '2 von 49 Tickets gekauft' && await js(`document.querySelectorAll('#matrix td.cell.st-bought').length === 2`));
  await key('z');
  check('bulk: one undo step takes both back', (await badges('bought')) === 0 && (await count()) === '0 von 49 Tickets gekauft');
  check('bulk: the selection survives the undo', await js(`document.querySelectorAll('#sheets input.pick:checked').length === 2`));
  await js(`(() => { document.getElementById('pickNone').click(); const w = document.getElementById('who'); w.value = 'mammut'; w.dispatchEvent(new Event('input')); document.getElementById('pickAll').click(); })(); 0`);
  check('bulk: header checkbox picks the visible (filtered) rows only', await js(`document.getElementById('pickCount').textContent.startsWith('11 ausgewählt')`), // mammut and mammut+1 … +10
    await js(`document.getElementById('pickCount').textContent + ' / visible ' + document.querySelectorAll('#sheets tbody tr:not(.hidden)').length`));
  await js(`document.querySelector('#bulk button[data-st="passed"]').click(); 0`);
  check('bulk: status for the filtered selection, others untouched, passed counts as bought', (await badges('passed')) === 11 && (await badges('open')) === 38 && (await count()) === '11 von 49 Tickets gekauft');
  await js(`document.querySelector('#bulk button[data-st="passed"]').click(); 0`);
  await key('z');
  check('bulk: a click that changes nothing is no undo step', (await badges('passed')) === 0 && (await badges('open')) === 49);
  await js(`(() => { const w = document.getElementById('who'); w.value = 'kiwi'; w.dispatchEvent(new Event('input')); })(); 0`);
  check('bulk: filter change updates the header box and names hidden picks', await js(`!document.getElementById('pickAll').checked && document.getElementById('pickCount').textContent.includes('11 ausgewählt, davon 11 vom Filter ausgeblendet')`));
  await js(`document.getElementById('pickNone').click(); 0`);
  check('bulk: selection cleared, buttons disabled', await js(`document.querySelectorAll('#sheets input.pick:checked').length === 0 && document.querySelector('#bulk button[data-st="open"]').disabled`));
  await js(`document.getElementById('who').value = ''; document.getElementById('who').dispatchEvent(new Event('input')); document.querySelector('#sheets tr[data-label="tapir"] input.pick').click(); 0`);
  await js(`(() => { const r = document.getElementById('roster'); r.value = r.value.split('\\n').filter((l) => !l.includes('\\ttapir\\t')).join('\\n'); document.getElementById('compute').click(); })(); 0`);
  check('bulk: a pick whose slot left the plan is dropped', (await stats()).includes('48 Slots') && await js(`document.getElementById('pickCount').textContent.startsWith('Zeilen ankreuzen')`));
  await key('z');
  check('bulk: the person is back, unticked', (await stats()).includes('49 Slots') && await js(`!document.querySelector('#sheets tr[data-label="tapir"] input.pick').checked`));
  await js(`document.querySelector('#sheets tr[data-label="tapir"] input.pick').click(); 0`);
  const other = execFileSync(cli, ['-event', join(root, 'events', 'congress2026.json'), '-share', 'admin', '-base', page, '-seed', 'other']).toString().trim();
  await js(`location.hash = ${JSON.stringify(other.slice(other.indexOf('#')))}; 0`);
  for (let i = 0; i < 100 && (await js(`document.querySelectorAll('#sheets input.pick').length`)) !== 47; i++) await sleep(50);
  check('new link in the same tab: selection cleared', await js(`document.querySelectorAll('#sheets input.pick:checked').length === 0 && document.getElementById('pickCount').textContent.startsWith('Zeilen ankreuzen')`));

  // Language switch: re-renders in place, keeps pending input, and switching
  // back restores the German text and attributes.
  await js(`document.getElementById('roster').value += 'pending'; 0`);
  const typed = await roster();
  await js(`document.getElementById('lang').click(); 0`);
  check('lang: English re-renders plan, bulk bar and attributes', await js(`document.documentElement.lang === 'en' && document.getElementById('r-stats').textContent.startsWith('7 vouchers · 7 days · 47 slots') && document.getElementById('pickCount').textContent.startsWith('Tick rows') && document.getElementById('copyAdmin').title === 'This link holds the roster, settings and pins' && getComputedStyle(document.querySelector('#compute [lang="de"]')).display === 'none' && document.getElementById('lang').textContent === 'DE'`));
  await js(`document.getElementById('lang').click(); 0`);
  check('lang: German again, pending input untouched', (await roster()) === typed && await js(`document.documentElement.lang === 'de' && document.getElementById('r-stats').textContent.includes('47 Slots') && document.getElementById('copyAdmin').title === 'Dieser Link enthält Roster, Einstellungen und Pins' && document.getElementById('f-seed').placeholder === 'leer = Name' && getComputedStyle(document.querySelector('#compute [lang="en"]')).display === 'none'`));
  await js(`(() => { const r = document.getElementById('roster'); r.value = r.value.replace(/pending$/, ''); })(); 0`);

  // Sign-up lines: a hand-typed line in its own order lands in the roster;
  // messages appear under the button that was clicked, not at the top.
  const msgAfter = (id) => js(`document.getElementById('messages').previousElementSibling === document.getElementById('${id}').parentElement`);
  await js(`document.getElementById('inbox').value = 'bananaaa ; hallo@babna.com ; mitglied ; no ; panda ; nein'; document.getElementById('addInbox').click(); 0`);
  check('inbox: hand-typed line added with e-mail, status and connection', /\tbananaaa\tmember\t\thallo@babna\.com\tpanda\t/.test(await roster()) && (await msgAfter('addInbox')) && await js(`document.querySelector('#messages .warning')?.textContent.includes('"no" ignored')`));
  await js(`document.getElementById('compute').click(); 0`);
  check('inbox: "Plan berechnen" places the new person', (await stats()).includes('48 Slots'));
  await js(`document.getElementById('inbox').value = '; nomail@example.org'; document.getElementById('addInbox').click(); 0`);
  check('inbox: a bad line is reported right under the button', (await msgAfter('addInbox')) && await js(`/line 1: name missing/.test(document.querySelector('#messages .error')?.textContent || '')`));
  await js(`document.getElementById('roster').value += '\\tbroken\\tMitglied\\tx\\t\\t\\t\\t\\t\\n'; document.getElementById('compute').click(); 0`);
  check('roster: a bad row is reported under "Plan berechnen"', (await msgAfter('compute')) && await js(`/not a number/.test(document.querySelector('#messages .error')?.textContent || '')`));
  await js(`document.getElementById('inbox').value = 'zebra; Gast'; document.getElementById('addInbox').click(); 0`);
  check('inbox: a bad roster row is reported under the clicked button', (await msgAfter('addInbox')) && await js(`/not a number/.test(document.querySelector('#messages .error')?.textContent || '')`));
  await js(`(() => { const r = document.getElementById('roster'); r.value = r.value.replace('\\tbroken\\tMitglied\\tx\\t', '\\tbroken\\tMitglied\\t3\\t'); document.getElementById('compute').click(); })(); 0`);
  check('roster: a rule violation (+3 without group) is reported under "Plan berechnen" too', (await msgAfter('compute')) && await js(`/exceeds the limit/.test(document.querySelector('#messages .error')?.textContent || '')`));
  const belowFold = await js(`window.scrollTo(0, 0); document.getElementById('messages').getBoundingClientRect().top > innerHeight`);
  await js(`document.getElementById('lang').click(); 0`);
  check('lang: a Go message on screen gets the English heading and stays put, the page does not jump', belowFold && (await msgAfter('compute')) && (await js(`window.scrollY`)) === 0 && await js(`document.querySelector('#messages .error strong')?.textContent === 'That does not work:' && /exceeds the limit/.test(document.querySelector('#messages .error')?.textContent || '')`));
  await js(`document.getElementById('lang').click(); document.getElementById('f-collectTo').value = ''; document.getElementById('copyCollect').click(); 0`);
  const needTo = () => js(`document.querySelector('#messages .error li')?.textContent`);
  const deNeedTo = await needTo();
  await js(`document.getElementById('lang').click(); 0`);
  const enNeedTo = await needTo();
  await js(`document.getElementById('lang').click(); 0`);
  check('lang: a page message on screen is built again in the other language', deNeedTo.startsWith('Für den Sammel-Link') && enNeedTo === 'The sign-up link needs the organiser address (field above).', `${deNeedTo} / ${enNeedTo}`);

  // The sign-up form writes the canonical English line whatever the page language.
  const collect = execFileSync(cli, ['-event', join(root, 'events', 'congress2026.json'), '-share', 'collect', '-to', 'orga@example.org', '-base', page]).toString().trim();
  await js(`location.hash = ${JSON.stringify(collect.slice(collect.indexOf('#')))}; 0`);
  for (let i = 0; i < 100 && (await js(`document.getElementById('collect').hidden`)); i++) await sleep(50);
  const line = await js(`(() => {
    const set = (id, v) => { document.getElementById(id).value = v; };
    set('c-handle', 'yak'); set('c-email', 'yak@example.org'); set('c-tier', 'guest'); set('c-plus', '1'); set('c-knows', 'panda'); set('c-note', 'late');
    document.getElementById('collectForm').requestSubmit();
    return document.getElementById('c-line').value;
  })()`);
  check('collect: the German page writes the canonical line', line === 'yak; yak@example.org; guest; 1; panda; late', line);

  // Contrast: every visible text and form field against the background it
  // actually sits on, in light and dark, in every mode. WCAG AA: 4.5:1, 3:1
  // for large text; disabled controls are exempt.
  const probe = `(() => {
    const rgba = (s) => { const m = /rgba?\\(([^)]+)\\)/.exec(s); if (!m) return null; const p = m[1].split(/[\\s,\\/]+/).filter(Boolean).map(Number); return [p[0], p[1], p[2], p.length > 3 ? p[3] : 1]; };
    const over = (top, under) => [0, 1, 2].map((i) => top[i] * top[3] + under[i] * (1 - top[3])).concat(1);
    const lum = (c) => { const v = c.slice(0, 3).map((x) => { x /= 255; return x <= 0.03928 ? x / 12.92 : ((x + 0.055) / 1.055) ** 2.4; }); return 0.2126 * v[0] + 0.7152 * v[1] + 0.0722 * v[2]; };
    const ratio = (a, b) => { const [x, y] = [lum(a), lum(b)].sort((p, q) => q - p); return (x + 0.05) / (y + 0.05); };
    const background = (el) => {
      const layers = [];
      for (let e = el; e; e = e.parentElement) { const b = rgba(getComputedStyle(e).backgroundColor); if (b && b[3] > 0) { layers.push(b); if (b[3] >= 1) break; } }
      return layers.reduceRight((under, top) => over(top, under), [255, 255, 255, 1]);
    };
    const field = (el) => el.matches('input:not([type=checkbox]):not([type=radio]):not([type=file]):not([type=hidden]), textarea, select');
    const bad = [];
    for (const el of document.body.querySelectorAll('*')) {
      const own = [...el.childNodes].some((n) => n.nodeType === 3 && n.nodeValue.trim());
      if (!own && !field(el)) continue;
      if (!el.getClientRects().length || el.closest(':disabled, [hidden]')) continue;
      const cs = getComputedStyle(el);
      if (cs.visibility !== 'visible') continue;
      let opacity = 1;
      for (let e = el; e; e = e.parentElement) opacity *= Number(getComputedStyle(e).opacity);
      const placeholder = field(el) && !el.value && el.placeholder; // what shows is the placeholder
      const bg = background(el), fg = rgba(placeholder ? getComputedStyle(el, '::placeholder').color : cs.color);
      const r = ratio(over([fg[0], fg[1], fg[2], fg[3] * opacity], bg), bg);
      const size = parseFloat(cs.fontSize), large = size >= 24 || (size >= 18.66 && Number(cs.fontWeight) >= 700);
      if (r < (large ? 3 : 4.5)) bad.push(el.tagName.toLowerCase() + (el.id ? '#' + el.id : '') + (el.className && typeof el.className === 'string' ? '.' + el.className.trim().replace(/\\s+/g, '.') : '') + ' "' + (el.value || placeholder || el.textContent).trim().slice(0, 24) + '" ' + r.toFixed(2) + ' rgb(' + fg.slice(0, 3).join(', ') + ') on rgb(' + bg.slice(0, 3).map(Math.round).join(', ') + ')');
    }
    return bad;
  })()`;
  const contrast = async (mode) => {
    for (const scheme of ['light', 'dark']) {
      await send('Emulation.setEmulatedMedia', { features: [{ name: 'prefers-color-scheme', value: scheme }] }, sessionId);
      const bad = await js(probe);
      check(`contrast: ${mode}, ${scheme}`, bad.length === 0, bad.slice(0, 8).join('\n      '));
    }
    await send('Emulation.setEmulatedMedia', { features: [] }, sessionId);
  };
  const share = (mode, ...extra) => execFileSync(cli, ['-event', join(root, 'events', 'congress2026.json'), '-share', mode, ...extra, '-base', page]).toString().trim();
  const go = async (url, id) => {
    await js(`location.hash = ${JSON.stringify(url.slice(url.indexOf('#')))}; 0`);
    for (let i = 0; i < 100 && (await js(`document.getElementById('${id}').hidden`)); i++) await sleep(50);
  };
  await js(`document.querySelectorAll('#collect input, #collect textarea').forEach((f) => { f.value = ''; }); 0`); // placeholders show
  await contrast('collect form');
  await go(link, 'result');
  const tinted = await js(`(() => {
    document.querySelectorAll('details').forEach((d) => { d.open = true; });
    ['received', 'bought', 'passed'].forEach((st, i) => { // the table is rebuilt after every status, so look the row up each time
      document.querySelectorAll('#sheets tbody tr')[i].querySelector('input.pick').click();
      document.querySelector('#bulk button[data-st="' + st + '"]').click();
      document.getElementById('pickNone').click();
    });
    document.querySelector('#matrix td.cell:not(.empty)').click();
    document.querySelector('#matrix td.cell.empty').classList.add('dragover'); // throws, and fails the gate, if the plan has no free cell
    // The demo has no handover to someone's own +N; add the mark the page renders for one.
    const li = document.createElement('li');
    li.append(Object.assign(document.createElement('span'), { className: 'mark forbidden-text', textContent: '!' }), ' sample');
    document.querySelector('#handovers ul').append(li);
    document.getElementById('inbox').value = 'zebu; vegan';
    document.getElementById('addInbox').click();
    return ['received', 'bought', 'passed'].map((st) => document.querySelectorAll('#sheets .badge.st-' + st).length + '/' + document.querySelectorAll('#matrix td.cell.st-' + st).length).join(' ');
  })()`);
  check('contrast: admin shows one received, bought and passed slot', tinted === '1/1 1/1 1/1', tinted);
  await contrast('admin with statuses, help, drag target and a warning');
  await js(`document.getElementById('inbox').value = '; x@example.org'; document.getElementById('addInbox').click(); 0`);
  await contrast('admin with an error');
  await send('Emulation.setEmulatedMedia', { media: 'print', features: [{ name: 'prefers-color-scheme', value: 'dark' }] }, sessionId);
  const printed = await js(probe);
  check('print: no dice, super random or nerd panel on paper', await js(`(() => {
    const ids = ['r-random', 'r-nerd', 'entropy', 'r-demo'];
    ids.forEach((id) => { document.getElementById(id).hidden = false; }); // only the print rule may hide them now
    const gone = ids.every((id) => getComputedStyle(document.getElementById(id)).display === 'none');
    ['entropy', 'r-demo'].forEach((id) => { document.getElementById(id).hidden = true; });
    return gone;
  })()`));
  check('contrast: admin printed from a browser in dark mode', printed.length === 0, printed.slice(0, 8).join('\n      '));
  await send('Emulation.setEmulatedMedia', { media: '', features: [] }, sessionId);
  await go(share('board'), 'board');
  await contrast('board');
  await go(share('view'), 'result');
  await js(`(() => { const w = document.getElementById('who'); w.value = 'mammut'; w.dispatchEvent(new Event('input')); })(); 0`);
  check('contrast: viewer highlights the own rows', await js(`document.querySelectorAll('#sheets tr.me').length > 0`));
  await contrast('viewer with own rows');
  await js(`document.getElementById('who').value = ''; 0`);
  await contrast('viewer, empty handle field');
  await send('Page.navigate', { url: page }, sessionId);
  for (let i = 0; i < 200 && !(await js('!!globalThis.vouch0rzii && !document.getElementById("intro").hidden').catch(() => false)); i++) await sleep(50);
  check('contrast: landing page with the intro shown', await js('!document.getElementById("intro").hidden'));
  await contrast('landing page');

  // Start page: the demo is offered only while nothing is typed; a new event
  // is one click away everywhere else.
  const startState = () => js(`({
    demoLinks: [...document.querySelectorAll('.demo-link')].map((n) => !n.hidden),
    newEvent: !document.getElementById('newEvent').hidden, banner: !document.getElementById('demoBanner').hidden,
    home: document.getElementById('home').href,
  })`);
  const fresh = await startState();
  check('start: the untouched start page offers the demo, no new-event button, home link to itself', fresh.demoLinks.length === 3 && fresh.demoLinks.every(Boolean) && !fresh.newEvent && !fresh.banner && fresh.home === page, JSON.stringify(fresh));
  await js(`(() => { const f = document.getElementById('f-name'); f.value = 'mine'; f.dispatchEvent(new Event('input', { bubbles: true })); })(); 0`);
  const typedState = await startState();
  check('start: once something is typed the demo link goes, new event comes', typedState.demoLinks.every((v) => !v) && typedState.newEvent, JSON.stringify(typedState));
  await js(`document.getElementById('f-name').value = ''; 0`);

  // Chance: the draw is made visible. The first plan computed on the page
  // shuffles, opening a link does not; dice and super random work only
  // before the start; a roll is one undo step and the CLI draws the same.
  const isShuffling = () => js(`document.getElementById('matrix').classList.contains('shuffling')`);
  const calm = async () => { for (let i = 0; i < 100 && (await isShuffling()); i++) await sleep(50); };
  const grid = () => js(`[...document.querySelectorAll('#matrix td.cell')].map((td) => td.firstChild ? td.firstChild.nodeValue : '').join('|')`);
  const seedNow = () => js(`document.getElementById('f-seed').value`);
  const demoEvent = JSON.parse(readFileSync(join(root, 'events', 'congress2026.json'), 'utf8'));
  await js(`(() => {
    const ev = ${JSON.stringify(demoEvent)};
    document.getElementById('f-name').value = ev.name; document.getElementById('f-start').value = ev.start;
    document.getElementById('f-vouchers').value = ev.vouchers;
    document.getElementById('roster').value = vouch0rzii.formatRoster(JSON.stringify(ev.people)).tsv;
    document.getElementById('compute').click();
  })(); 0`);
  check('chance: the first plan on the page shuffles its matrix', await isShuffling());
  await calm();
  await js(`document.getElementById('compute').click(); 0`);
  check('chance: computing again does not', !(await isShuffling()));
  await js(`document.getElementById('f-vouchers').value = '0'; document.getElementById('compute').click(); 0`); // the planner rejects it, no plan
  const noPlan = await js(`document.getElementById('result').hidden`);
  await js(`document.getElementById('f-vouchers').value = '7'; document.getElementById('compute').click(); 0`);
  check('chance: nor after a failed compute', noPlan && !(await isShuffling()) && (await js(`!document.getElementById('result').hidden`)));

  await go(link, 'result');
  check('chance: opening a link shows the plan at once, no demo hint', !(await isShuffling()) && (await js(`document.getElementById('r-demo').hidden`)));
  check('chance: dice and super random offered before the start', await js(`['roll', 'superRandom'].every((id) => { const b = document.getElementById(id); return !b.hidden && !b.disabled; })`));
  const before = await grid(), seedBefore = await seedNow();
  await js(`document.getElementById('roll').click(); 0`);
  const during = await isShuffling(), flickers = await js(`document.querySelectorAll('#matrix td.cell.flicker').length`);
  await calm();
  const after = await grid(), seedAfter = await seedNow();
  check('chance: a roll writes a new 13-sign seed and shuffles', during && flickers > 0 && /^[a-km-np-z2-9]{13}$/.test(seedAfter) && seedAfter !== seedBefore && (await js(`document.getElementById('r-seed').textContent`)) === seedAfter, `${during} ${flickers} ${seedAfter}`);
  const noteDe = await js(`document.getElementById('r-randomnote').textContent`);
  await js(`document.getElementById('lang').click(); 0`); // renders the matrix again from the plan
  const noteEn = await js(`document.getElementById('r-randomnote').textContent`);
  await js(`document.getElementById('lang').click(); 0`);
  check('chance: after the animation the matrix is the new plan', after !== before && (await grid()) === after);
  check('chance: the note says it is drawn, in the page language', noteDe.startsWith('Gelost in jeder Stufe') && noteEn.startsWith('Drawn within each tier'), `${noteDe} / ${noteEn}`);
  const rolledEvent = join(profile, 'rolled.json');
  writeFileSync(rolledEvent, JSON.stringify({ ...demoEvent, seed: seedAfter }));
  const cliDay1 = JSON.parse(execFileSync(cli, ['-event', rolledEvent, '-format', 'json']).toString()).grid[0];
  check('chance: the CLI draws the same day 1 from that seed', JSON.stringify(cliDay1) === JSON.stringify(await js(`[...document.querySelectorAll('#matrix tbody tr')].map((tr) => tr.cells[1].firstChild.nodeValue)`)), JSON.stringify(cliDay1));
  await js(`document.getElementById('roll').click(); 0`);
  const rolledAgain = await isShuffling();
  for (const type of ['mousePressed', 'mouseReleased']) await send('Input.dispatchMouseEvent', { type, x: 5, y: 5, button: 'left', clickCount: 1 }, sessionId);
  const skippedAt = { shuffling: await isShuffling(), flicker: await js(`!!document.querySelector('#matrix td.cell.flicker')`), grid: await grid() };
  await js(`document.getElementById('lang').click(); document.getElementById('lang').click(); 0`);
  check('chance: a click skips the animation and leaves the real plan', rolledAgain && !skippedAt.shuffling && !skippedAt.flicker && skippedAt.grid === (await grid()), JSON.stringify({ rolledAgain, ...skippedAt, grid: undefined }));
  await key('z');
  await key('z');
  check('chance: Ctrl+Z brings seed and matrix back', (await seedNow()) === seedBefore && (await grid()) === before);
  const lineups = JSON.parse(execFileSync(cli, ['-event', join(root, 'events', 'congress2026.json'), '-format', 'json']).toString()).draw.lineups;
  check('chance: the nerd panel shows the Go numbers', await js(`(() => { const s = document.getElementById('r-nerdBody').textContent; return s.includes('25 gelost → 25! = ') && s.includes('× 10${lineups.length - 1}'); })()`), lineups);

  const goodRoster = await roster(), seedKept = await seedNow();
  await js(`document.getElementById('roster').value += 'broken\\tmember\\tx\\n'; document.getElementById('roll').click(); 0`);
  check('chance: a roll the form rejects keeps the old seed', (await seedNow()) === seedKept);
  await js(`document.getElementById('roster').value = ${JSON.stringify(goodRoster)}; 0`);

  await send('Emulation.setEmulatedMedia', { features: [{ name: 'prefers-reduced-motion', value: 'reduce' }] }, sessionId);
  await send('Page.navigate', { url: link }, sessionId); // same document: only the hash changes
  await send('Page.reload', {}, sessionId); // a fresh page: no note yet
  await sleep(200);
  for (let i = 0; i < 200 && !(await js('!!globalThis.vouch0rzii && !document.getElementById("result").hidden').catch(() => false)); i++) await sleep(50);
  await js(`document.getElementById('roll').click(); 0`);
  check('chance: with reduced motion a roll recomputes without animation and says so', !(await isShuffling()) && (await seedNow()) !== seedBefore && (await js(`document.getElementById('r-randomnote').textContent.startsWith('Gelost')`)));
  await send('Emulation.setEmulatedMedia', { features: [] }, sessionId);

  // Super random: real pointer moves over the field fill the pot.
  await js(`document.getElementById('superRandom').click(); document.getElementById('entropyPad').scrollIntoView({ block: 'center' }); 0`);
  check('chance: super random announces its panel', await js(`document.getElementById('superRandom').getAttribute('aria-expanded') === 'true'`));
  const padRect = () => js(`(() => { const r = document.getElementById('entropyPad').getBoundingClientRect(); return { x: r.x, y: r.y, w: r.width, h: r.height }; })()`);
  let pad = await padRect();
  const seedPre = await seedNow();
  const move = (i) => send('Input.dispatchMouseEvent', { type: 'mouseMoved', x: pad.x + 8 + (i * 37) % (pad.w - 16), y: pad.y + 8 + (i * 53) % (pad.h - 16) }, sessionId);
  for (let i = 0; i < 40; i++) await move(i);
  await sleep(100);
  const half = await js(`({ open: !document.getElementById('entropy').hidden, bar: document.getElementById('entropyBar').value, max: document.getElementById('entropyBar').max, dots: document.querySelectorAll('#entropyPad .dot').length, preview: document.getElementById('entropySeed').textContent })`);
  check('chance: super random counts moves, draws dots and previews a seed', half.open && half.bar === 40 && half.max === 128 && half.dots === 40 && /^[a-km-np-z2-9]{13}$/.test(half.preview), JSON.stringify(half));
  await contrast('super random open');
  for (const force of [0.2, 0.5, 0.8]) await send('Input.dispatchMouseEvent', { type: 'mouseMoved', x: pad.x + 8 + (39 * 37) % (pad.w - 16), y: pad.y + 8 + (39 * 53) % (pad.h - 16), pointerType: 'pen', force }, sessionId);
  await sleep(50);
  check('chance: a pointer standing still adds nothing', (await js(`document.getElementById('entropyBar').value`)) === 40);
  for (let i = 40; i < 140; i++) await move(i);
  for (let i = 0; i < 40 && !(await js(`document.getElementById('entropy').hidden`)); i++) await sleep(50);
  const superSeed = await seedNow();
  check('chance: 128 moves become the seed and the plan follows', /^[a-km-np-z2-9]{13}$/.test(superSeed) && superSeed !== seedPre && (await js(`document.getElementById('r-seed').textContent`)) === superSeed, superSeed);
  await calm();
  await js(`document.getElementById('superRandom').click(); 0`);
  for (let i = 0; i < 5; i++) await move(i);
  await js(`document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' })); 0`);
  await js(`document.getElementById('superRandom').click(); document.getElementById('superRandom').click(); 0`);
  check('chance: 🖱️ toggles its panel', await js(`document.getElementById('entropy').hidden && document.getElementById('superRandom').getAttribute('aria-expanded') === 'false'`));
  check('chance: Esc closes super random and keeps the seed', (await js(`document.getElementById('entropy').hidden && document.getElementById('superRandom').getAttribute('aria-expanded') === 'false'`)) && (await seedNow()) === superSeed);

  // Only before the start: a pin or a status locks both, also an open super random.
  await js(`document.getElementById('superRandom').click(); document.getElementById('entropyPad').scrollIntoView({ block: 'center' }); 0`);
  pad = await padRect();
  for (let i = 0; i < 20; i++) await move(i);
  const moved20 = await js(`document.getElementById('entropyBar').value`);
  await click(1, 1); await click(1, 2);
  for (let i = 20; i < 140; i++) await move(i);
  await sleep(100);
  check('chance: a pin closes an open super random, the seed stays', moved20 === 20 && (await js(`document.getElementById('entropy').hidden`)) && (await seedNow()) === superSeed, `moves counted before the pin: ${moved20}`);
  check('chance: a pin locks dice and super random, the reason is shown', await js(`['roll', 'superRandom'].every((id) => document.getElementById(id).disabled) && document.getElementById('roll').title.includes('Pins') && document.getElementById('r-randomnote').textContent === document.getElementById('roll').title`));
  await js(`document.getElementById('unpinAll').click(); 0`);
  await js(`document.querySelector('#sheets tbody tr input.pick').click(); document.querySelector('#bulk button[data-st="received"]').click(); 0`);
  check('chance: a status locks them too', await js(`document.getElementById('roll').disabled && document.getElementById('roll').title.includes('Status')`));
  await js(`(() => { // the person with the status leaves the roster: a status on no slot of the plan locks nothing
    const who = document.querySelector('#sheets tbody tr').dataset.label.split('+')[0];
    const r = document.getElementById('roster');
    r.value = r.value.split('\\n').filter((l) => l.split('\\t')[1] !== who).join('\\n');
    document.getElementById('compute').click();
  })(); 0`);
  check('chance: a status on a slot that left the plan locks nothing', await js(`!document.getElementById('roll').disabled`));

  // The demo link opens with its draw and points at the random modes.
  const demoHref = await js(`(() => { const a = [...document.querySelectorAll('a')].find((a) => a.getAttribute('href').startsWith('?demo#s=')); return a && a.getAttribute('href'); })()`);
  await send('Page.navigate', { url: page + demoHref }, sessionId);
  let demoShuffled = false;
  for (let i = 0; i < 200 && !demoShuffled; i++) { demoShuffled = await js(`document.getElementById('matrix').classList.contains('shuffling')`).catch(() => false); if (!demoShuffled) await sleep(20); }
  check('chance: the demo opens on its plan, shuffling, with the hint shown', demoShuffled && (await js(`!document.getElementById('r-demo').hidden && Math.abs(document.getElementById('result').getBoundingClientRect().top) < 5`)));
  const onDemo = await startState();
  check('demo: it keeps its flag and says so, no demo link, new event offered', (await js(`location.search === '?demo' && location.hash.startsWith('#s=')`)) && onDemo.banner && onDemo.newEvent && onDemo.demoLinks.every((v) => !v), JSON.stringify(onDemo));
  await calm();
  await js(`document.getElementById('r-nerd').open = true; 0`);
  await contrast('demo with its hint and the nerd panel');
  await js(`document.getElementById('roll').click(); 0`);
  await calm();
  check('demo: after a roll it is still the demo', await js(`location.search === '?demo' && !document.getElementById('demoBanner').hidden && !document.getElementById('r-demo').hidden`));
  await js(`document.getElementById('f-seed').value = 'pending'; [...document.querySelectorAll('#demoBanner a')].find((a) => a.offsetParent).click(); 0`); // an edit not computed yet
  await sleep(200);
  for (let i = 0; i < 200 && !(await js('!!globalThis.vouch0rzii && !document.getElementById("intro").hidden').catch(() => false)); i++) await sleep(50);
  const leftDemo = { ...(await startState()), ...(await js(`({ href: location.href, name: document.getElementById('f-name').value, roster: document.getElementById('roster').value.trim().split('\\n').length })`)) }; // the header line only
  check('demo: "start your own event" opens the empty start page', leftDemo.href === page && leftDemo.name === '' && leftDemo.roster === 1 && !leftDemo.banner && leftDemo.demoLinks.every(Boolean) && !leftDemo.newEvent, JSON.stringify(leftDemo));
  await js(`history.back(); 0`);
  await sleep(200);
  for (let i = 0; i < 200 && !(await js('!!globalThis.vouch0rzii && !document.getElementById("result").hidden').catch(() => false)); i++) await sleep(50);
  check('demo: Back returns to the demo with the edit made before leaving', await js(`document.getElementById('f-seed').value === 'pending' && location.search === '?demo' && !document.getElementById('demoBanner').hidden`));
  await send('Page.navigate', { url: page + '?demo&view' + share('view').slice(share('view').indexOf('#')) }, sessionId); // a new document; view mode never rewrites the address
  await sleep(200);
  for (let i = 0; i < 200 && !(await js('!!globalThis.vouch0rzii && !document.getElementById("result").hidden').catch(() => false)); i++) await sleep(50);
  const onDemoView = await js(`({ search: location.search, board: document.getElementById('toBoard').href.slice(0, 120), mode: document.getElementById('modeLabel').textContent })`);
  check('chance: links built on a ?demo page carry no demo flag', onDemoView.search === '?demo&view' && !onDemoView.board.includes('?demo'), JSON.stringify(onDemoView));
  await send('Page.navigate', { url: page + demoHref.replace('?demo', '?demo&again') }, sessionId); // a new document, so the demo starts for real
  await sleep(200);
  for (let i = 0; i < 200 && !(await js('!!globalThis.vouch0rzii && !document.getElementById("result").hidden').catch(() => false)); i++) await sleep(50);
  await calm();
  await js(`document.getElementById('superRandom').click(); 0`);
  const demoBefore = await js(`({ hint: !document.getElementById('r-demo').hidden, open: !document.getElementById('entropy').hidden, note: document.getElementById('r-randomnote').textContent })`);
  await js(`location.hash = ${JSON.stringify(link.slice(link.indexOf('#')))}; 0`);
  for (let i = 0; i < 100 && !(await js(`document.getElementById('r-demo').hidden`)); i++) await sleep(50);
  check('chance: another link in the same tab is not the demo, starts without note or open super random', demoBefore.hint && demoBefore.open && demoBefore.note.startsWith('Gelost') && (await js(`location.search === '' && document.getElementById('demoBanner').hidden && document.getElementById('r-demo').hidden && document.getElementById('entropy').hidden && document.getElementById('r-randomnote').textContent === ''`)), JSON.stringify(demoBefore));
  await send('Page.navigate', { url: page + '?demo' }, sessionId); // the flag without a state
  await sleep(200);
  for (let i = 0; i < 200 && !(await js('!!globalThis.vouch0rzii && !document.getElementById("intro").hidden').catch(() => false)); i++) await sleep(50);
  const bareDemo = await startState();
  check('demo: ?demo without a state is the plain start page, the flag leaves the address', !bareDemo.banner && bareDemo.demoLinks.every(Boolean) && !bareDemo.newEvent && (await js(`location.search === ''`)), JSON.stringify(bareDemo));
  await send('Page.navigate', { url: page + '?demo#s=broken' }, sessionId);
  await sleep(200);
  for (let i = 0; i < 200 && !(await js('!!globalThis.vouch0rzii && document.getElementById("loading").hidden').catch(() => false)); i++) await sleep(50);
  check('demo: an unreadable demo link is no demo either', await js(`document.getElementById('demoBanner').hidden && location.search === ''`));
  await send('Page.navigate', { url: page + '?fresh' }, sessionId); // a new document: nothing computed yet
  await sleep(200);
  for (let i = 0; i < 200 && !(await js('!!globalThis.vouch0rzii && !document.getElementById("intro").hidden').catch(() => false)); i++) await sleep(50);
  await js(`(() => {
    document.getElementById('f-name').value = 'solo'; document.getElementById('f-start').value = '2026-10-30';
    document.getElementById('roster').value = 'Name\\tStatus\\nsolo\\tmember\\n';
    document.getElementById('compute').click();
  })(); 0`);
  check('chance: with nothing to draw the first plan does not shuffle and says why', !(await isShuffling()) && (await js(`document.getElementById('roll').disabled && document.getElementById('r-randomnote').textContent.startsWith('Nichts zu losen')`)));
  const afterSolo = await startState(); // values were set without input events: the plan alone ends the start page
  check('start: a computed plan ends the start page, even without typing', afterSolo.demoLinks.every((v) => !v) && afterSolo.newEvent, JSON.stringify(afterSolo));
} catch (e) {
  check(`no exception: ${e.message}`, false);
} finally {
  if (proc.exitCode === null) { proc.kill(); await new Promise((r) => proc.once('exit', r)); } // let Chrome close the profile first
  rmSync(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
}
console.log(failures ? `\n${failures} interaction check(s) failed` : '\nall interaction checks passed');
process.exit(failures ? 1 : 0);
