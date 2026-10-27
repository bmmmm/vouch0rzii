#!/usr/bin/env node
// Gate: the page works in a real browser. Launches headless Chrome on
// web/dist via file:// (fetch allowed by flag), dumps the DOM once the wasm
// has loaded, and checks the markers of every mode: empty page, demo link,
// admin link, viewer link, board link (also with statuses), collect link
// (links come from the CLI; the state codec is base64url(deflate(json)), so
// node can read and rewrite them). Prints SKIP and exits 0 when no Chrome is
// found; set CHROME=/path/to/chrome.
//
// Usage: node scripts/web-browser-smoke.mjs [path/to/vouch0rzii-binary]
// Requires a prior `go build -o vouch0rzii .` and `scripts/build-web.sh`.
import { execFileSync, spawnSync } from 'node:child_process';
import { existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { deflateRawSync, inflateRawSync } from 'node:zlib';

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

const demoEvent = join(root, 'events', 'congress2026.json');
const link = (mode, extra = [], event = demoEvent) =>
  execFileSync(cli, ['-event', event, '-share', mode, '-base', page, ...extra]).toString().trim();
const decode = (href) => JSON.parse(inflateRawSync(Buffer.from(href.split('#s=')[1], 'base64url')).toString());
const withStatus = (href, status) =>
  page + '#s=' + deflateRawSync(JSON.stringify({ ...decode(href), status, statusAt: '2026-10-27T12:00:00Z' })).toString('base64url');
const hrefOf = (html, id) => (html.match(new RegExp(`id="${id}" class="button" href="([^"]+)"`)) || [])[1] || '';

function dom(url, lang = 'de') { // the page follows the browser language; pinned so checks do not depend on the machine
  const r = spawnSync(chrome, [
    '--headless=new', '--disable-gpu', '--no-sandbox', '--allow-file-access-from-files',
    `--user-data-dir=${join(tmpdir(), 'vouch0rzii-chrome')}`, `--accept-lang=${lang}`, '--virtual-time-budget=10000', '--dump-dom', url,
  ], { encoding: 'utf8', maxBuffer: 64 << 20 });
  if (r.status !== 0) throw new Error(`chrome exit ${r.status}: ${r.stderr}`);
  return r.stdout;
}

let failures = 0;
const check = (name, cond) => {
  console.log(`${cond ? 'PASS' : 'FAIL'}  ${name}`);
  if (!cond) failures++;
};
const noError = (html) => !html.includes('class="error"');
const planned = (html) => html.includes('7 Vouchers · 7 Tage · 47 Slots');

const home = dom(page);
check('empty page: wasm loaded, loading note hidden', home.includes('<p id="loading" hidden=""'));
check('empty page: version shown', /Version \S+/.test(home));
const demo = (home.match(/href="(#s=[A-Za-z0-9_-]+)"/) || [])[1];
check('empty page: demo link present', !!demo);
check('empty page: intro visible', home.includes('<p id="intro" class="intro">'));
check('empty page: no error box', noError(home));

const demoDom = dom(page + demo);
check('demo: plan rendered in admin mode', planned(demoDom) && demoDom.includes('<table id="matrix" class="editable">'));
check('demo: diagram inlined', demoDom.includes('<div class="scroll" id="diagram"><svg'));

const admin = dom(link('admin'));
check('admin: plan rendered from the link', planned(admin));
check('admin: matrix editable and shows the group slot', admin.includes('<table id="matrix" class="editable">') && admin.includes('mammut+10'));
check('admin: one sheet row per slot', (admin.match(/data-who=/g) || []).length === 47);
check('admin: live status lists every voucher', (admin.match(/<li class="live/g) || []).length === 7);
check('admin: status buttons on every sheet row', (admin.match(/<button type="button" class="badge st-open"/g) || []).length === 47);
check('admin: a pick box per row, bulk bar shown, bought counter', (admin.match(/<input type="checkbox" class="pick"/g) || []).length === 47 && admin.includes('<div id="bulk" class="row">') && admin.includes('<span id="sheets-count" class="stats">0 von 47 Tickets gekauft</span>'));
check('admin: freeze and extra-voucher controls present', admin.includes('id="freezeToday"') && admin.includes('id="f-extra"'));
check('admin: matrix cells keyboard-focusable', (admin.match(/tabindex="0"/g) || []).length >= 47);
check('admin: intro hidden once a plan exists', admin.includes('<p id="intro" class="intro" hidden=""'));
check('admin: voucher phase summary, free cells marked, nothing for the public sale', admin.includes('Voucher-Phase bis Fr 13.11. (11 Tage) · 30 Slots frei') && (admin.match(/>frei<\/td>/g) || []).length === 2 && admin.includes('<div id="r-public" class="warning" hidden="">'));
check('admin: "+ Tag" column offered while the phase has room', admin.includes('>+ Tag<'));
check('admin: the draw is explained with the seed', admin.includes('entscheidet das Los mit Seed „congress2026"'));
check('admin: share buttons visible', admin.includes('<nav id="sharebar">'));
check('admin: board hidden, "Board" opens a board state in a new tab', admin.includes('<section id="board" class="card" hidden="">') && /id="toBoard" class="button" href="[^"]+"[^>]* rel="noopener"[^>]* target="_blank"/.test(admin) && decode(hrefOf(admin, 'toBoard')).mode === 'board');
check('admin: no error box', noError(admin));
check('admin: German page relabels the organisers and formats German day labels', (admin.match(/<td>Orga<\/td>/g) || []).length === 7 && (admin.match(/>Orga</g) || []).length === 7 && !admin.includes('>organisers<') && admin.includes('>Fr 30.10.<small>2026-10-30<') && admin.includes('font-weight="600">Fr 30.10.<') && !admin.includes('font-weight="600">Fri 30/10<'));
check('admin: a German browser gets the German page', admin.includes('<html lang="de">') && admin.includes('<title>vouch0rzii — Voucher-Kettenplaner</title>'));
const fp = (html) => (html.match(/id="fingerprint"[^>]*>([^<]*)</) || [])[1] || '';
check('admin: four-emoji fingerprint shown', [...fp(admin)].length === 4);

const view = dom(link('view'));
check('view: plan rendered, matrix not editable', planned(view) && !view.includes('class="editable"'));
check('view: frozen (47 pinned)', view.includes('47 gepinnt'));
check('view: status badges, not buttons', (view.match(/<span class="badge st-open"/g) || []).length === 47 && !view.includes('class="badge st-open" title'));
check('view: no pick boxes, bulk bar hidden, counter shown', !view.includes('class="pick"') && view.includes('<div id="bulk" class="row" hidden="">') && view.includes('0 von 47 Tickets gekauft'));
check('view: the draw is explained with the seed', view.includes('entscheidet das Los mit Seed „congress2026"'));
check('view: editor and share buttons hidden', view.includes('<section id="editor" hidden=""') && view.includes('<nav id="sharebar" hidden=""'));
check('view: no error box', noError(view));
check('view: same fingerprint as admin', fp(view) === fp(admin));
check('view: board hidden, switch leads to a board state of the same plan in this tab', view.includes('<section id="board" class="card" hidden="">') && !/id="toBoard"[^>]*target=/.test(view) && (() => { const st = decode(hrefOf(view, 'toBoard')); return st.mode === 'board' && st.pins.length === 47; })());

const board = dom(link('board'));
check('board: dashboard shown, plan and editor hidden', board.includes('<section id="board" class="card">') && board.includes('<section id="result" hidden="">') && board.includes('<section id="editor" hidden=""'));
check('board: progress over all placed slots, one tile per voucher', board.includes('0 von 47 Tickets gekauft') && (board.match(/<li class="tile orga">/g) || []).length === 7 && board.includes('<progress id="b-bar" max="47" value="0">'));
check('board: free slots, same fingerprint, "Zum Plan" is a view state', board.includes('<b>30</b><span>Slots frei</span>') && decode(hrefOf(board, 'b-plan')).mode === 'view' && fp(board) === fp(admin) && noError(board));
// Statuses: fennek passed voucher 1 on, quokka holds it; nacktmull bought on
// voucher 2 (holding); the rest is still with the organisers. Tickets bought
// counts bought and passed; overdue depends on the date and is not asserted.
const live = dom(withStatus(link('board'), { fennek: 'passed', quokka: 'received', nacktmull: 'bought' }));
check('board with statuses: progress, holding tiles, status stand', live.includes('2 von 47 Tickets gekauft') && live.includes('<progress id="b-bar" max="47" value="2">') && (live.match(/<li class="tile holding/g) || []).length === 2 && (live.match(/<li class="tile orga/g) || []).length === 5 && live.includes('3 Slot(s) mit Status, Stand'));

// The same roster with the voucher phase ending after five days: 35 slots
// fit, twelve go to the public sale, no "+ Tag" column past the end. And
// without any end: the open-ended path every older link takes.
const scratch = mkdtempSync(join(tmpdir(), 'vouch0rzii-'));
const variant = (name, patch) => {
  const { end, ...ev } = JSON.parse(readFileSync(demoEvent, 'utf8'));
  const file = join(scratch, name);
  writeFileSync(file, JSON.stringify({ ...ev, ...patch }));
  return dom(link('admin', [], file));
};
const early = variant('early.json', { end: '2026-11-05' });
check('early end: plan stops at the end of the voucher phase', early.includes('7 Vouchers · 5 Tage · 47 Slots') && early.includes('Voucher-Phase bis Do 05.11. (5 Tage) · 0 Slots frei'));
check('early end: public sale box lists the tickets that did not fit', early.includes('<div id="r-public" class="warning">') && early.includes('12 Ticket(s) passen nicht mehr') && (early.match(/data-who=/g) || []).length === 35);
check('early end: no day after the end offered', !early.includes('>+ Tag<') && noError(early));
const open = variant('open.json', { seed: 'würfel' }); // a seed that differs from the name
check('no end: open-ended summary, free cells, "+ Tag" offered', open.includes('Kein Ende der Voucher-Phase gesetzt · 2 Slots frei im Plan') && (open.match(/>frei<\/td>/g) || []).length === 2 && open.includes('>+ Tag<') && open.includes('<div id="r-public" class="warning" hidden="">'));
check('no end: the explanation names the seed, not the event name', open.includes('entscheidet das Los mit Seed „würfel"'));
// A chain that starts later and a member whose handle is "organisers": the
// German page says "Orga" for each first holder, the member keeps the name.
const people = JSON.parse(readFileSync(demoEvent, 'utf8')).people;
const late = variant('late.json', { extra: [{ date: '2026-11-03', count: 1 }], people: [{ id: 'organisers', email: 'organisers@example.org', tier: 'member' }, ...people] });
check('late chain: "Orga" for the eight first holders only, the member "organisers" keeps the name', (late.match(/>Orga</g) || []).length === 8 && late.includes('<td>organisers<small>') && noError(late));
rmSync(scratch, { recursive: true });

const collect = dom(link('collect', ['-to', 'orga@example.org']));
check('collect: form shown with the event name', collect.includes('<section id="collect" class="card">') && collect.includes('<span id="c-event">congress2026</span>'));
check('collect: editor hidden', collect.includes('<section id="editor" hidden=""'));

// An English browser gets the English page: <html lang> picks the static
// text, app.js renders its own strings, day labels and attributes in English.
const en = dom(link('admin'), 'en-US');
check('en: page language follows the browser', en.includes('<html lang="en">') && en.includes('<title>vouch0rzii — voucher chain planner</title>'));
check('en: plan summary, phase and free cells in English', en.includes('7 vouchers · 7 days · 47 slots') && en.includes('Voucher phase until Fri 13/11 (11 days) · 30 slots free') && (en.match(/>free<\/td>/g) || []).length === 2 && en.includes('>+ day<'));
check('en: draw, counter and status badges in English', en.includes('the draw with seed "congress2026" decides') && en.includes('0 of 47 tickets bought') && (en.match(/class="badge st-open"[^>]*>open</g) || []).length === 47);
check('en: attributes swapped, German kept beside them', en.includes('title="This link holds the roster, settings and pins"') && en.includes('data-de-title="Dieser Link enthält Roster, Einstellungen und Pins"') && en.includes('>DE</button>'));
check('en: first holders get it from the organisers', (en.match(/<td>organisers<\/td>/g) || []).length === 7 && !en.includes('>Orga<') && en.includes('font-weight="600">Fri 30/10<'));
check('en: same plan as German', fp(en) === fp(admin) && noError(en));
const enBoard = dom(link('board'), 'en-US');
check('en: board in English', enBoard.includes('0 of 47 tickets bought') && enBoard.includes('<b>30</b><span>slots free</span>'));
const enCollect = dom(link('collect', ['-to', 'orga@example.org']), 'en-US');
check('en: collect form options in English', enCollect.includes('<option value="guest" data-en-text="frequent guest" data-de-text="häufiger Gast">frequent guest</option>'));

console.log(failures ? `\n${failures} browser check(s) failed` : '\nall browser checks passed');
process.exit(failures ? 1 : 0);
