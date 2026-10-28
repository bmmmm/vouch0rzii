# Changelog

Notable changes to vouch0rzii. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); this project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html). While the major
version is 0, behaviour may change in a minor release — the entries below say
what to check.

## [Unreleased]

### Added

- `every` in the event (page: "Voucher every … days"): plan days from one
  buying day of a chain to the next, 1 to 14, default 1. Without weekends the
  voucher process rests on Saturday and Sunday, so `every` counts weekdays.
  Handovers are dated on the evening the new voucher is expected; the JSON
  export gives each handover a `label`. `"every": 1` is the default and keeps
  the fingerprint.
- The page warns when a change of start, weekends or `every` moves pinned
  slots to other dates.

### Changed

- Groups keep their voucher: a person with the group flag gets their slots
  day after day on as few vouchers as the plan's length allows, so the
  voucher stays in one mailbox instead of passing through strangers. The new
  event setting `oneChain` ("one voucher per group") gives each group a
  single voucher and lets the plan grow up to the end of the voucher phase.
  Who gets into the plan still follows the tiers; only days move. In the
  example mammut's eleven slots run on two vouchers instead of six.
- The assignment counts the handover into a pin on the next day: a slot no
  longer lands right before a pinned slot of the same person (without the
  group flag) when another voucher is free, and among equal choices a
  voucher does not idle before its pin. A group goes on with its own
  voucher rather than one fresh from the organisers.
- Exports: `stats.kept` and `handovers[].kept` (handovers within a group)
  replace `stats.known`, `stats.unknown` and `handovers[].known`; the SVG
  arrow kinds are `kept`, `pass` and `forbidden`; the legend shows only
  "group keeps the voucher".

### Removed

- `Knows` (Kennt): the roster column, the sign-up field and the preference
  for handovers between people who know each other. The sign-up line is now
  `Name; Email; Status; +N; Note`; `parseSignup` takes the text only.
  Event files with `knows` are rejected, a `Kennt` column is ignored with a
  warning, and older sign-up lines with six or seven fields are read by
  content, their handles ending up in the note.


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
- The page in German and English: it follows the browser's language, a
  header button switches; the choice is not part of the link. Everything the
  planner writes is English (day labels `Fri 30/10`, roster text, sign-up
  line `Name; Email; Status; +N; Knows; Note`, `organisers`); German headers
  and values are still read.
- The draw made visible: the first plan, every roll and the demo shuffle the
  matrix before the real plan locks in day by day (a click skips; off with
  reduced motion). Before the start, without pins or status, admins can
  roll a new seed (🎲) or collect one from pointer movement (🖱️ super
  random); a roll is one undo step. "How random?" explains seed, hashes,
  generator and the number of possible results of the draw; the JSON export
  carries them in `draw`, as exact decimals.
- Start page and demo: the demo link only shows on the untouched start page;
  the demo page says it is the demo in a banner, also after edits and
  reloads; the title and "＋ New event" lead back to an empty start page.
- Sign-up lines typed by hand: a single word that names nobody known stays
  the note; known names count anywhere on the line, alone or as a further
  list, each handle once; words kept as note are reported in one warning
  per line. Someone who knows their own handle gets a warning.
- Contrast in the light and the dark scheme: text and placeholders reach
  WCAG AA (4.5:1) on the start page, the sign-up form, the admin view, the
  board and the viewer, measured by the browser gate; field borders, the
  selection ring, the stranger mark and the diagram's lines are set to 3:1.
  Printing uses the light colours.
- Gates: Go tests, CLI == wasm smoke with privacy contract, headless-Chrome
  DOM gate, DevTools interaction gate (clicks, keys, pointer moves,
  contrast in both schemes).

[Unreleased]: https://github.com/bmmmm/vouch0rzii/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/bmmmm/vouch0rzii/releases/tag/v0.1.0
