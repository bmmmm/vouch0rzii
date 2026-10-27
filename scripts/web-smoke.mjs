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
import { readFileSync } from 'node:fs';
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
check('plan returns markdown/csv', planned.markdown.includes('47 slots') && planned.markdown.startsWith('# congress2026') && planned.csv.startsWith('Voucher;'));

const broken = bridge.plan('{"name":"x","vouchers":0,"start":"nope","people":[]}', '');
check('broken event → ok:false with errors, no exception', broken.ok === false && broken.errors.length >= 3, JSON.stringify(broken.errors));

const ev = JSON.parse(eventJSON);
const tsv = bridge.formatRoster(JSON.stringify(ev.people));
const parsed = bridge.parseRoster(tsv.tsv);
check('roster TSV round trip', parsed.ok && JSON.stringify(JSON.parse(parsed.peopleJson)) === JSON.stringify(ev.people), JSON.stringify(parsed.errors));

const state = { v: 1, mode: 'view', event: ev, pins: JSON.parse(planned.exportJson).pins };
const enc = bridge.encodeState(JSON.stringify(state));
check('encodeState ok', enc.ok, JSON.stringify(enc.errors));
const dec = bridge.decodeState(enc.encoded);
check('decodeState round trip', dec.ok && JSON.parse(dec.stateJson).pins.length === 47 && JSON.parse(dec.stateJson).mode === 'view');
const bad = bridge.decodeState('not-a-state');
check('decodeState rejects garbage', bad.ok === false && bad.errors.length === 1);
check('version string present', typeof bridge.version === 'string' && bridge.version.length > 0);

console.log(failures ? `\n${failures} check(s) failed` : '\nall web smoke checks passed');
process.exit(failures ? 1 : 0);
