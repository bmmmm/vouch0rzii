# vouch0rzii — agent instructions

Voucher chain planner for event presales: a Go core, a CLI, and a static page
(Go compiled to WebAssembly) whose URL fragment is the whole state.

## Architecture

- `internal/plan` — everything that computes:
  `model.go` types and limits · `event.go` JSON format and `Normalize` ·
  `tsv.go` roster paste codec · `schedule.go` draw per tier, voucher start
  days, placement with pins · `hungarian.go` + `assign.go` per-day
  assignment and the 2-opt pass · `plan.go` `Compute`, handovers across idle
  days, `Freeze` · `render.go` `Export`/JSON/Markdown/CSV · `svg.go` flow
  diagram · `fingerprint.go` four-emoji data version · `share.go` link state
  codec and modes (admin, view, collect).
- `main.go` — CLI over `run()`, tested in `main_test.go`.
- `cmd/vouch0rzii-wasm` — browser bridge; every call returns
  `{ok, errors, warnings, …}` and never throws (panics are caught).
- `web/` — `index.html`, `app.js`, `style.css`; no framework, no external
  resources. `scripts/build-web.sh` assembles `web/dist/` (gitignored) and
  injects the demo link. `app.js` only moves data between form, fragment
  and DOM; it never plans.
- `events/congress2026.json` — pseudonymised example: demo, smoke reference,
  CLI tests.

## Build & test

```sh
gofmt -l . ; go vet ./... ; GOOS=js GOARCH=wasm go vet ./cmd/vouch0rzii-wasm
go test ./...
go build -o vouch0rzii . && scripts/build-web.sh
node scripts/web-smoke.mjs            # wasm == CLI byte for byte, privacy contract
node scripts/web-browser-smoke.mjs    # headless Chrome renders every mode; skips without Chrome
```

All of it gates a commit; CI runs the same. Add one mutant per new operand
and watch a test go red before trusting it.

## Conventions

- Standard library only. A dependency needs a one-line reason in the PR.
- The link is the database. No storage APIs, no fetch besides the wasm, no
  external resources, the CSP stays exactly as pinned in the smoke. Modes are
  views, not access control; viewer links are frozen (every slot pinned) and
  never carry notes; collect links carry no roster.
- CLI and wasm share `internal/plan` and must stay byte-equal on JSON. The
  `Export` field names are the page's API.
- Deterministic: the draw depends only on seed and tier membership; the same
  input yields the same plan on every platform.
- Limits in `model.go` bound everything a link can carry. Keep them when you
  add inputs.
- UI strings German, code, comments and docs English, commits conventional
  (`feat:`, `fix:`, `refactor!:` …).
- Page changes count as unverified until the browser gate ran.
