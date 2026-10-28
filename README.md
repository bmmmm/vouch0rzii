# vouch0rzii

Voucher chain planner for event presales (Congress, Easterhegg, Camp, …).

A group holds a handful of vouchers. Each voucher buys one ticket per day and
is handed on by e-mail in the evening, so every voucher is a chain of people.
vouch0rzii turns a roster into that plan: who holds which voucher on which day
and hands it to whom, with as few days as possible, and groups keep their
voucher.

Everything runs in the browser (Go compiled to WebAssembly) or on the command
line with the same code. Nothing is sent to a server: the share link *is* the
data, and your browser history keeps it like any other link.

## Rules the planner follows

1. **As few days as possible.** With `V` vouchers from day 1 and `N` ticket
   slots the plan has `ceil(N / V)` days; extra vouchers, pins, people who
   leave, one voucher per group (rule 4) and the end of the voucher phase
   (rule 7) change that, the grid shows the result. Weekends are skipped
   unless the event says otherwise.
   A "day" is a buying day: with `every` set to 2 (the new voucher only
   arrives the day after next) buying days are two plan days apart, and a
   handover is listed on the evening the new voucher is expected. Without
   weekends the voucher process rests on Saturday and Sunday too, so plan
   days are weekdays: bought on Friday, handed over on Monday evening.
2. **Tiers.** Named people first — members, then frequent guests, then
   acquaintances. After all named people come everybody's +1, then everybody's
   +2, each round ordered by the sponsor's position. Within a tier the order is
   drawn by lot with a fixed seed (reproducible; change the seed for a new
   draw). A manual `Order` number overrides the lot. The page shows the draw:
   the first plan and every new roll shuffle the matrix before it locks in
   day by day. Before the start (no pins, no status) admins can roll a new
   seed (🎲, the browser's crypto randomness) or collect one from mouse or
   finger movement (🖱️ super random, hashed with crypto bytes through
   SHA-256); "How random?" shows the seed's path to the generator and how
   many orders each tier's lot allows. The JSON export carries the same numbers in
   `draw`.
3. **+N is capped at 2** unless the person carries the group flag (they
   represent a larger group). Their surplus (+3, +4, …) comes last. A person
   never hands the voucher directly to their own +N — unless they have the
   group flag (rule 4). If the planner finds no way around such a handover
   (one voucher, one person with +1) it is used and reported as a warning.
4. **Groups keep their voucher.** +N slots stand in for their sponsor and
   share the sponsor's address, so a voucher that stays with a group needs
   no handover. From a group's first day its slots run day after day on as
   few vouchers as the plan's length allows: one chain from that day,
   further chains ending on the last day; the plan keeps its length. With
   `oneChain` ("one voucher per group" on the page) a group keeps a single
   voucher and the plan may grow, up to the end of the voucher phase. With
   `membersFirst` ("members first") every member buys their own slot on the
   same day as without groups: a group's chain starts on the day after the
   last member's day, pinned members included, and the group's slots whose
   turn came before stay where they are. Who
   gets into the plan at all still follows rules 2 and 3; only the days
   move. For each day the assignment person → voucher keeps every group on
   the voucher it holds, unless a voucher would otherwise go straight to
   someone's own +N (rule 3). Each day is decided from the days before it
   and the pins only. The page marks these cells *group keeps the voucher*.
5. **Pins.** In the admin view a slot can be moved to another cell or swapped
   with another slot. Pins are kept in the link and respected before the
   automatic placement fills the rest. A voucher may idle for a day; the
   handover then goes to the next holder and says on which day they buy.
   A free cell takes a typed handle: an existing slot moves there,
   `handle+2` raises that person's +N, a new handle joins the roster as
   acquaintance, pinned to the cell. Ctrl+Z / Ctrl+Shift+Z (Cmd on a Mac)
   or the buttons above the matrix undo and redo moves, pins, status and
   roster changes while the tab is open; the link always carries the
   current state.
6. **Mid-run changes.** "Pin until today" pins every slot up to today,
   so roster edits only change the future; pinning alone changes
   nothing. Someone who got a voucher
   elsewhere is marked `Out`: they stay in the draw (nobody
   else moves), their pinned slots remain, their other slots vanish and the
   people behind move up. Vouchers that arrive later go into `extra`
   (`date:count`); those chains start fresh from the organisers on that day.
7. **End of the voucher phase.** `end` is the last day vouchers
   work. The plan fills only up to it; slots that do not fit are listed as
   *public sale* (`public`) in priority order and are bought in the public
   sale. Pins after the end are dropped with a warning. Free voucher days
   are marked *free* in the grid and counted (`free`): until the end, or
   within the grid when no end is set, so organisers see how many more
   slots they can still hand out. Extra vouchers dated after the end are
   reported as unused.

## Roster columns

Paste from a spreadsheet; the header row is matched by name in any order.
The planner writes the English names and values; the German aliases are
read too, so a German sheet can be pasted as it is:

| Column | Aliases | Values |
|---|---|---|
| `Order` | Reihenfolge | manual position within the tier, empty = lot |
| `Name` | handle | unique; must not contain `+ ; , \|` |
| `Status` | tier, Mitglied, "Mitglied / (Häufiger) Gast / Bekanntschaft" | member, guest, acquaintance (Mitglied, Gast, Bekanntschaft) |
| `+N` | plus | number of extra people |
| `Email` | E-Mail, mail | contact for the handover |
| `Group` | Gruppe | yes = may exceed +N max, keeps its voucher (yes/no, ja/nein, x, 1/0) |
| `Out` | Ausgestiegen, raus, left | yes = gives up all slots that are not pinned (yes/no, ja/nein, x, 1/0) |
| `Note` | Notiz | free text, never part of viewer links |

## Links and modes

The page keeps its whole state in the URL fragment (`#s=…`,
base64url-encoded deflated JSON). Four modes, all views on the same data —
**not** access control: whoever has a link has what is in it. The header
shows a four-emoji fingerprint of the current data (settings, roster,
placement); admin and viewer links of the same state show the same emoji,
so two people can tell at a glance whether they look at the same plan.

- **Admin**: settings, roster, pins and the organiser's address. Edit, compute,
  pin cells, copy links.
- **Viewer**: settings, roster without notes, the organisers' address, the
  live status and the frozen plan (every placed slot pinned), so it renders
  identically on any later version. Filter "I am …" to find your row; the
  Mail link prepares the handover mail to the next person, "report" a status
  report to the organisers. Admins can copy a personal viewer link per
  person ("Link" in the sheet table) that highlights that person's rows.
  The viewer prints as a clean sheet: matrix, handovers, who-when-whom.
- **Board**: the viewer's data as a dashboard for everyone who wants to
  follow along: progress bar of tickets bought, a tile per voucher (where it
  is, who is next, overdue), today's and the next day's buyers, free slots,
  the public-sale list, and a link to the full plan. "Board" in the plan
  opens it (admins in a new tab, so their edits stay); "Copy board link"
  gives admins the link to share.
- **Sign-up (collect)**: a sign-up form for people interested in a voucher.
  It carries only the event settings and the organiser's address; the result
  is one line (`Name; Email; Status; +N; Note`, status as member, guest or
  acquaintance) that is mailed or copied to the organiser, who pastes it
  into "Add lines from sign-up mails". Lines typed by hand may put the
  fields after the name in any order: the e-mail is the field with `@`, the
  status a tier word, +N a number; a second e-mail, status or number and
  yes/no words are ignored with a warning, the rest becomes the note. Problems with the roster or with sign-up lines
  are reported under "Compute plan" or under the sign-up button; a cell
  name the planner rejects is reported above the matrix.

## Live status

There is no server, so the status is curated: admins click a slot's status
through *open → voucher received → ticket bought → passed on* in the sheet
table, or tick several rows (the header box takes every visible row, so the
"I am" filter selects a person's slots) and set one status for all of
them in a single undo step. Rows are tinted by status and the table header
counts tickets bought. The status and its timestamp travel in the link, and a re-shared
viewer link shows everyone where each voucher is, who is next, and what is
overdue (date before today, status not advanced; marked only once the first
status has been set, so an untouched plan is never "overdue"). Participants report their
status with the "report" mail link; admins enter it. Two admins compare
fingerprint and status timestamp to see who holds the newer link.

## Demo and example

`events/congress2026.json` is a pseudonymised real roster (25 animals, 22
extra slots, 7 vouchers → 7 days, voucher phase of 11 days with 30 slots
free). Status and e-mail are **placeholders**; the real values live only in your own links
and files. The page's demo link is an admin link of this roster,
generated at build time; the file itself is not part of the site. It is
offered only on the untouched start page, so nobody works in the demo by
mistake: the demo opens on its shuffling plan and a banner says it is the
demo for as long as `?demo` is in the address bar (links copied from it
never carry the flag). The title and "＋ New event" always lead back to an
empty start page; "New event" first puts pending edits into the link if
they compute, so the browser's Back returns to the last computed state.

## Command line

```
go install github.com/bmmmm/vouch0rzii@latest

vouch0rzii -event events/congress2026.json              # Markdown: matrix + handovers
vouch0rzii -event events/congress2026.json -format csv   # csv | json | svg | event
vouch0rzii -event e.json -roster paste.tsv -format event # spreadsheet paste → event file
vouch0rzii -event e.json -pins pins.json -seed other     # manual pins, new draw
vouch0rzii -event e.json -share view                     # frozen viewer link (board: dashboard)
vouch0rzii -event e.json -share collect -to orga@example.org
```

Exit status 2 means the input was rejected; the reasons are on stderr.

## Development

```
go test ./... && go vet ./... && GOOS=js GOARCH=wasm go vet ./cmd/vouch0rzii-wasm
go build -o vouch0rzii . && scripts/build-web.sh && node scripts/web-smoke.mjs
node scripts/web-browser-smoke.mjs             # headless Chrome over web/dist, all modes
node scripts/web-interact-smoke.mjs            # clicks, typing and undo in the admin matrix
cd web/dist && python3 -m http.server 8077     # then open http://localhost:8077/ (file:// cannot fetch)
```

`scripts/web-smoke.mjs` proves the wasm build plans byte-equal to the CLI and
pins the privacy contract (CSP, no storage, no external resources);
`scripts/web-browser-smoke.mjs` loads the page in headless Chrome and checks
the DOM of the empty page and of a demo, an admin, a viewer, a board and a
collect link; `scripts/web-interact-smoke.mjs` drives the admin matrix over
the DevTools protocol (swap, undo, redo, naming a free cell). Agent instructions live in `AGENTS.md`, the security policy in
`SECURITY.md`, notable changes in `CHANGELOG.md`.
Layout: `internal/plan` (model, codecs, scheduling, assignment, rendering,
share state), `main.go` (CLI), `cmd/vouch0rzii-wasm` (browser bridge),
`web/` (page), `events/` (examples).

## License

GPL-3.0-or-later, see `LICENSE`.
