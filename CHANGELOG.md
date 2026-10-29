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
- `membersFirst` in the event (page: "members first"): every member buys
  their own slot on the same day as without groups, and a group's chain
  starts on its first day, but no earlier than the day after the last
  member's day, pinned members included. Combines with `oneChain`. In the
  example all 25 members buy by day 4 instead of day 5, and mammut's ten +N
  run on days 4 to 7 on three vouchers instead of two; 7 instead of 9
  handovers stay within the group. A member added after "Pin until today"
  comes first as well: without `oneChain` the rest of the group's chain then
  crowds onto the remaining days (in the example, pinned until day 5, onto
  day 7 on six vouchers), with it as many handovers stay within the group
  as before (8 in the example). Without `membersFirst` plans stay as they
  are.
- `lost` in the event (page: "lost" at a slot in the sheet table, also
  after the fact): a voucher whose chain broke carries nobody from the
  first plan day on or after the date. The page pins the past as "Pin until
  today" does, except that voucher from the slot's day on; its slots from
  there are planned again after today, in priority order, on the vouchers
  left, and once every voucher is lost the rest goes to the public sale.
  Pins on closed cells are dropped with a warning. The matrix hatches the
  closed cells, the live status says since when, "found" takes the mark
  back. The JSON export has `voucherLost` (first day without the voucher,
  0 = not lost), the Markdown a line per lost voucher, and `free` leaves
  the closed cells out.
- The page locks what would move the past once the sale has started (a pin,
  a status or a lost voucher): start, weekends, `every`, the seed and, while
  it stands in for the seed, the name. A change that gives the existing
  vouchers other numbers or start days, moves or drops a pin whose slot is
  still in the roster, or does not compute is rejected with the reason and
  the plan stays; other changes go through and say how many planned slots
  moved.
- `frozen` in the event: days up to this date take pinned slots only.
  "Pin until today" sets it, "Release all pins" clears it.
- The page marks the plan as outdated as soon as settings or roster differ
  from it, and blocks its links, exports, matrix and sheet actions, undo
  and printing until "Compute plan" ran.

### Changed

- The page explains × on a pin on a frozen day: nobody bought there, the
  slot is planned again after the frozen days and the cell stays free.
- Groups keep their voucher: a person with the group flag gets their slots
  day after day on as few vouchers as the plan's length allows, so the
  voucher stays in one mailbox instead of passing through strangers. The new
  event setting `oneChain` ("one voucher per group") gives each group a
  single voucher and lets the plan grow up to the end of the voucher phase.
  Who gets into the plan still follows the tiers; only days move. In the
  example mammut's eleven slots run on two vouchers instead of six.
- The assignment decides each day from the days before it and the pins
  only, counting the handover into each voucher's next pin and, to break
  ties, the cheapest handover to tomorrow's slots; the 2-opt pass that
  changed earlier days afterwards is gone. Pinning the past ("Pin until
  today") keeps the rest of the plan, pins by hand after today included
  (0 of 20,000 random rosters changed, 51 before). A slot no longer lands
  right before a pinned slot of the same person (without the group flag)
  when another voucher is free, among equal choices a voucher does not idle
  before its pin, and a group goes on with its own voucher rather than one
  fresh from the organisers. Plans with pins by hand may hand a voucher
  straight to someone's own +N a little more often (about 6 % more in
  random rosters, warned as "no alternative found"). A day with few slots
  and many vouchers is solved much faster.
- Plans come out differently than with 0.1.0, also without pins. For an
  event already under way, "Pin until today" before upgrading keeps the
  days that have passed.
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

### Fixed

- "Pin until today" closes the past: it also sets `frozen` in the event to
  today, and days up to `frozen` take pinned slots only. Before, a cell
  left free in the past (beside a group's chain with `oneChain`, at the end
  of the plan) took the next slot once someone joined, and a group slot
  could move back into it: in the example with `oneChain`, pinned until
  day 9, a new member landed on day 7, now on day 10. `free` counts the
  days after `frozen` only. "Release all pins" clears `frozen` as well, and
  the saved event file leaves it out, as it leaves out the pins. Since
  `frozen` is a setting, freezing changes the fingerprint.
- A group's chain takes its targets from the day after its first slot
  instead of that day, which the slot already holds: with more slots than
  days left, a slot lost its target and fell back to its turn. Plans with
  groups can come out differently: 49 to 65 of 1,500 random rosters, 55 of
  200 other seeds for the example; the example itself stays byte for byte.
- A compute that fails no longer empties the page: the last plan and state
  stay, with the reason (a loaded file, undo, a lost mark near the 400-day
  limit). A rejected change shows the roster's warnings too.
- After the start, a pin the plan already did not honour (a link made by
  hand) no longer blocks every change.
- Typing into a free cell while a roster edit does not compute no longer
  throws.
- Empty cells on frozen days read "closed" instead of "free".
- `frozen` is trimmed like the other dates, and when frozen days use up all
  400 plan days the error says so.
- The order of `lost` entries no longer changes the fingerprint or the
  exports: they are sorted by voucher.


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
