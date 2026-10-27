# Changelog

Notable changes to vouch0rzii. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); this project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html). While the major
version is 0, behaviour may change in a minor release — the entries below say
what to check.

## [Unreleased]

### Added

- The page in German and English: it follows the browser's language, a
  header button switches; the choice is not part of the link.

### Changed

- Everything the planner writes is English: day labels (`Fri 30/10`), the
  roster text (`Order Name Status +N Email Knows Group Out Note`, `member`,
  `yes`/`no`), the sign-up line (`Name; Email; Status; +N; Knows; Note`) and
  `from` of a chain's first holder (`organisers` instead of `Orga`). German
  headers and values are still read. Check: scripts that match day labels,
  the old roster header or `from` = `Orga`.
- The wasm call `parseSignup` takes the roster's handles as a second
  argument, a JSON string (`'["bob"]'`); an array is rejected.

### Fixed

- Sign-up lines typed by hand: a single word that names nobody known stays
  the note instead of being dropped as an unknown handle; known names count
  anywhere on the line, alone or as a further list, each handle once. Words
  kept as note are reported in one warning per line.
- Someone who knows their own handle gets a warning, as the docs said.

## [0.1.0] - 2026-10-27

### Added

- Planner: tiers (members, guests, acquaintances, then +1, +2, overflow),
  seeded draw per tier, minimal days, per-day assignment that favours
  handovers between people who know each other, 2-opt around pins.
- Pins: move or swap slots, type a handle into a free cell (existing slot,
  `handle+k`, or a new acquaintance); undo and redo (Ctrl+Z, Ctrl+Shift+Z,
  buttons); viewer links are frozen; "Pin until today".
- Mid-run changes: extra vouchers that start later, people who leave keep
  only pinned slots.
- End of the voucher phase (`end`): the plan stops there, tickets that do
  not fit are listed for the public sale (`public`), free voucher days are
  marked *free* and counted (`free`).
- Live status per slot (open → voucher received → ticket bought →
  passed on), curated by admins, carried in the link, with per-voucher
  summary and overdue marks; status reports by mail. Rows in the sheet table
  can be ticked (header box: all visible rows) and set to one status in a
  single step; rows are tinted by status and a counter shows tickets bought.
- Credit card removed from roster, sign-up line, table and export; an old
  `Kreditkarte` column is ignored, event files with `card` need the field
  removed. The plan card, roster hint and sign-up form explain the draw.
- Sign-up lines pasted by hand are read by content (e-mail at `@`, tier
  word, number, handles, note) when they are not in the form's
  order; roster and sign-up problems are reported under "Compute plan" or
  the sign-up button, rejected cell names above the matrix.
- Share links: admin, viewer, board (dashboard for everyone: progress, a
  tile per voucher, today and next, free slots, public sale), collect
  (sign-up form), personal viewer links; four-emoji fingerprint of the data
  version.
- Outputs: Markdown, CSV, JSON, SVG flow diagram; handover mails.
- CLI (`-format md|csv|json|svg|event`, `-share`), wasm page, demo link.
- Gates: Go tests, CLI == wasm smoke with privacy contract, headless-Chrome
  DOM gate, DevTools interaction gate for the admin matrix.
