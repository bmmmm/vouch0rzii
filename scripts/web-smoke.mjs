#!/usr/bin/env node
// Gate: the web build must produce exactly what the CLI produces.
//
// Loads web/dist/vouch0rzii.wasm through the same wasm_exec.js the browser
// uses, plans the example event via globalThis.vouch0rzii.plan and compares
// the JSON byte for byte with `vouch0rzii -format json`. Further cases: a
// broken event surfaces as ok:false with errors (never an exception), the
// roster codec round-trips, and a share link encodes and decodes.
//
// Usage: node scripts/web-smoke.mjs [path/to/vouch0rzii-binary]   (default ./vouch0rzii)
// Requires a prior `go build -o vouch0rzii .` and `scripts/build-web.sh`.
import { createRequire } from 'node:module';
import { execFileSync } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
const root = resolve(fileURLToPath(new URL('..', import.meta.url)));
const dist = join(root, 'web', 'dist');
const cli = resolve(process.argv[2] || join(root, 'vouch0rzii'));
const eventPath = join(root, 'events', 'congress2026.json');

const refJSON = execFileSync(cli, ['-event', eventPath, '-format', 'json']).toString();

globalThis.fs = require('node:fs');
globalThis.path = require('node:path');
require(join(dist, 'wasm_exec.js'));
const go = new globalThis.Go();
const { instance } = await WebAssembly.instantiate(readFileSync(join(dist, 'vouch0rzii.wasm')), go.importObject);
go.run(instance); // never resolves; the Go side parks in select {}
for (let i = 0; i < 100 && !globalThis.vouch0rzii; i++) await new Promise((r) => setTimeout(r, 10));
const bridge = globalThis.vouch0rzii;
if (!bridge) throw new Error('vouch0rzii global not registered');

let failures = 0;
const check = (name, cond, detail = '') => {
  console.log(`${cond ? 'PASS' : 'FAIL'}  ${name}${cond || !detail ? '' : `\n      ${detail}`}`);
  if (!cond) failures++;
};

const eventJSON = readFileSync(eventPath, 'utf8');
const planned = bridge.plan(eventJSON, '[]');
check('plan() ok', planned.ok, JSON.stringify(planned.errors));
check('plan JSON equals CLI -format json byte for byte', planned.exportJson === refJSON);
check('plan returns markdown/csv/svg', planned.markdown.includes('47 slots') && planned.markdown.startsWith('# congress2026') && planned.csv.startsWith('Voucher;') && planned.svg.startsWith('<svg '));

// The same after the start: the first three days pinned and frozen, voucher 3
// lost from day 2, so its pins there are dropped with a warning.
const ref = JSON.parse(refJSON);
const startedEvent = { ...JSON.parse(readFileSync(eventPath, 'utf8')), frozen: ref.days[2].date, lost: [{ voucher: 3, date: ref.days[1].date }] };
const pastPins = ref.sheets.filter((s) => s.day <= 3).map(({ label, day, voucher }) => ({ label, day, voucher }));
const tmp = mkdtempSync(join(tmpdir(), 'vouch0rzii-smoke-'));
writeFileSync(join(tmp, 'event.json'), JSON.stringify(startedEvent));
writeFileSync(join(tmp, 'pins.json'), JSON.stringify(pastPins));
const startedRef = execFileSync(cli, ['-event', join(tmp, 'event.json'), '-pins', join(tmp, 'pins.json'), '-format', 'json']).toString();
rmSync(tmp, { recursive: true });
const startedPlan = bridge.plan(JSON.stringify(startedEvent), JSON.stringify(pastPins));
check('after the start (frozen, lost, pins): plan JSON equals the CLI byte for byte', startedPlan.ok && startedPlan.exportJson === startedRef, JSON.stringify(startedPlan.errors));
const sx = startedPlan.ok ? JSON.parse(startedPlan.exportJson) : { voucherLost: [], warnings: [] };
check('after the start: voucher 3 lost from day 2, its pins there dropped', sx.voucherLost[2] === 2 && sx.warnings.some((w) => w.endsWith('voucher 3 is lost from day 2, dropped')), JSON.stringify(sx.warnings));

const broken = bridge.plan('{"name":"x","vouchers":0,"start":"nope","people":[]}', '');
check('broken event → ok:false with errors, no exception', broken.ok === false && broken.errors.length >= 3, JSON.stringify(broken.errors));

const ev = JSON.parse(eventJSON);
const tsv = bridge.formatRoster(JSON.stringify(ev.people));
const parsed = bridge.parseRoster(tsv.tsv);
check('roster TSV round trip', parsed.ok && JSON.stringify(JSON.parse(parsed.peopleJson)) === JSON.stringify(ev.people), JSON.stringify(parsed.errors));

const signup = bridge.parseSignup('bananaaa ; hallo@babna.com ; mitglied ; no ; panda ; nein\n');
check('parseSignup reads a hand-typed line by content', signup.ok && JSON.stringify(JSON.parse(signup.peopleJson)) === '[{"id":"bananaaa","email":"hallo@babna.com","tier":"member","note":"panda"}]' && signup.warnings.length === 2, JSON.stringify(signup));
const note = bridge.parseSignup('anna; Gast; vegan');
check('parseSignup keeps an unknown word as the note', note.ok && JSON.parse(note.peopleJson)[0].note === 'vegan' && note.warnings.length === 0, JSON.stringify(note));
check('parseSignup names the line in errors', !bridge.parseSignup('\n; x@example.org').ok && bridge.parseSignup('\n; x@example.org').errors[0].startsWith('line 2:'));

const state = { v: 1, mode: 'view', event: ev, pins: JSON.parse(planned.exportJson).pins };
const enc = bridge.encodeState(JSON.stringify(state));
check('encodeState ok', enc.ok, JSON.stringify(enc.errors));
const dec = bridge.decodeState(enc.encoded);
check('decodeState round trip', dec.ok && JSON.parse(dec.stateJson).pins.length === 47 && JSON.parse(dec.stateJson).mode === 'view');
const bad = bridge.decodeState('not-a-state');
check('decodeState rejects garbage', bad.ok === false && bad.errors.length === 1);
check('version string present', typeof bridge.version === 'string' && bridge.version.length > 0);

// --- privacy contract (README: nothing leaves the page except through the
// links, mails and downloads the user triggers; the fragment is the only store)
const html = readFileSync(join(root, 'web', 'index.html'), 'utf8');
const js = readFileSync(join(root, 'web', 'app.js'), 'utf8');
const csp = "default-src 'none'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; form-action 'none'";
check('privacy: CSP pinned literally', html.includes(`content="${csp}"`));
check('privacy: no storage APIs', !/localStorage|sessionStorage|indexedDB|document\.cookie|caches\./.test(js));
check('privacy: fetch only loads the wasm', (js.match(/fetch\(/g) || []).length === 2 && !/fetch\((?!'vouch0rzii\.wasm')/.test(js));
check('privacy: no beacons, sockets or XHR', !/sendBeacon|WebSocket|XMLHttpRequest|EventSource/.test(js));
check('privacy: no external resources in the page', !/<(script|link|img|iframe)[^>]+(src|href)="(https?:)?\/\//.test(html));
check('privacy: one HTML string sink, the plan SVG', (js.match(/innerHTML/g) || []).length === 1 && js.includes('innerHTML = plan.svg'));

console.log(failures ? `\n${failures} check(s) failed` : '\nall web smoke checks passed');
process.exit(failures ? 1 : 0);
