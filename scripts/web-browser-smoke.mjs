#!/usr/bin/env node
// Gate: the page works in a real browser. Launches headless Chrome on
// web/dist via file:// (fetch allowed by flag), dumps the DOM once the wasm
// has loaded, and checks the markers of every mode: empty page, demo link,
// admin link, viewer link, collect link (links come from the CLI). Prints
// SKIP and exits 0 when no Chrome is found; set CHROME=/path/to/chrome.
//
// Usage: node scripts/web-browser-smoke.mjs [path/to/vouch0rzii-binary]
// Requires a prior `go build -o vouch0rzii .` and `scripts/build-web.sh`.
import { execFileSync, spawnSync } from 'node:child_process';
import { existsSync } from 'node:fs';
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

const link = (mode, extra = []) =>
  execFileSync(cli, ['-event', join(root, 'events', 'congress2026.json'), '-share', mode, '-base', page, ...extra]).toString().trim();

function dom(url) {
  const r = spawnSync(chrome, [
    '--headless=new', '--disable-gpu', '--no-sandbox', '--allow-file-access-from-files',
    `--user-data-dir=${join(tmpdir(), 'vouch0rzii-chrome')}`, '--virtual-time-budget=10000', '--dump-dom', url,
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
check('admin: live status lists every voucher', (admin.match(/<li class="live"/g) || []).length === 7);
check('admin: status buttons on every sheet row', (admin.match(/<button type="button" class="badge st-open"/g) || []).length === 47);
check('admin: freeze and extra-voucher controls present', admin.includes('id="freezeToday"') && admin.includes('id="f-extra"'));
check('admin: matrix cells keyboard-focusable', (admin.match(/tabindex="0"/g) || []).length >= 47);
check('admin: intro hidden once a plan exists', admin.includes('<p id="intro" class="intro" hidden=""'));
check('admin: share buttons visible', admin.includes('<nav id="sharebar">'));
check('admin: no error box', noError(admin));
const fp = (html) => (html.match(/id="fingerprint"[^>]*>([^<]*)</) || [])[1] || '';
check('admin: four-emoji fingerprint shown', [...fp(admin)].length === 4);

const view = dom(link('view'));
check('view: plan rendered, matrix not editable', planned(view) && !view.includes('class="editable"'));
check('view: frozen (47 pinned)', view.includes('47 gepinnt'));
check('view: status badges, not buttons', (view.match(/<span class="badge st-open"/g) || []).length === 47 && !view.includes('class="badge st-open" title'));
check('view: editor and share buttons hidden', view.includes('<section id="editor" hidden=""') && view.includes('<nav id="sharebar" hidden=""'));
check('view: no error box', noError(view));
check('view: same fingerprint as admin', fp(view) === fp(admin));

const collect = dom(link('collect', ['-to', 'orga@example.org']));
check('collect: form shown with the event name', collect.includes('<section id="collect" class="card">') && collect.includes('<span id="c-event">congress2026</span>'));
check('collect: editor hidden', collect.includes('<section id="editor" hidden=""'));

console.log(failures ? `\n${failures} browser check(s) failed` : '\nall browser checks passed');
process.exit(failures ? 1 : 0);
