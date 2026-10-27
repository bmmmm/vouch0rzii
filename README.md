# vouch0rzii

Voucher chain planner for event presales (Congress, Easterhegg, Camp, …).

A group holds a handful of vouchers. Each voucher buys one ticket per day and
is handed on by e-mail in the evening, so every voucher is a chain of people.
vouch0rzii turns a roster into that plan: who holds which voucher on which day
and hands it to whom, with as few days as possible and as many handovers as
possible between people who already know each other.

Everything runs in the browser (Go compiled to WebAssembly) or on the command
line with the same code. Nothing is sent to a server: the share link *is* the
data, and your browser history keeps it like any other link.

## Rules the planner follows

1. **As few days as possible.** With `V` vouchers from day 1 and `N` ticket
   slots the plan has `ceil(N / V)` days; extra vouchers, pins, people who
   leave and the end of the voucher phase (rule 8) change that, the grid
   shows the result. Weekends are skipped unless the event says otherwise.
2. **Tiers.** Named people first — members, then frequent guests, then
   acquaintances. After all named people come everybody's +1, then everybody's
   +2, each round ordered by the sponsor's position. Within a tier the order is
   drawn by lot with a fixed seed (reproducible; change the seed for a new
   draw). A manual `Reihenfolge`/`order` number overrides the lot.
3. **+N is capped at 2** unless the person carries the group flag (they
   represent a larger group). Their surplus (+3, +4, …) comes last. A person
   never hands the voucher directly to their own +N — unless they have the
   group flag, then their block may run back to back. If such a handover
   cannot be avoided (one voucher, one person with +1) it is used and reported
   as a warning.
4. **Connections.** For each day the assignment person → voucher is chosen so
   that as many handovers as possible happen between people who know each
   other (`Kennt`/`knows`). +N slots stand in for their sponsor. Without manual
   pins this day-by-day optimum is already the global optimum; with pins a
   2-opt pass cleans up around them. The page calls these handovers
   *Buddies*, all others *neue Verbindung*.
5. **Credit card** is shown as an emergency fallback only and never affects
   the plan (instant SEPA transfers are the default).
6. **Pins.** In the admin view a slot can be moved to another cell or swapped
   with another slot. Pins are kept in the link and respected before the
   automatic placement fills the rest. A voucher may idle for a day; the
   handover then goes to the next holder and says on which day they buy.
7. **Mid-run changes.** "Bis heute festnageln" pins every slot up to today,
   so roster edits only change the future. Someone who got a voucher
   elsewhere is marked `Ausgestiegen`/`out`: they stay in the draw (nobody
   else moves), their pinned slots remain, their other slots vanish and the
   people behind move up. Vouchers that arrive later go into `extra`
   (`date:count`); those chains start fresh from the organisers on that day.
8. **End of the voucher phase.** `end` (`Ende`) is the last day vouchers
   work. The plan fills only up to it; slots that do not fit are listed as
   *Freier Verkauf* / `public` in priority order and are bought in the public
   sale. Pins after the end are dropped with a warning. Free voucher days
   are marked *frei* in the grid and counted (`free`): until the end, or
   within the grid when no end is set, so organisers see how many more
   slots they can still hand out. Extra vouchers dated after the end are
   reported as unused.

## Roster columns

Paste from a spreadsheet; the header row is matched by name, German or
English, in any order:

| Column | Aliases | Values |
|---|---|---|
| `Reihenfolge` | order | manual position within the tier, empty = lot |
| `Name` | handle | unique; must not contain `+ ; , \|` |
| `Kreditkarte` | card | ja/nein, x, 1/0 |
| `Status` | tier, "Mitglied / (Häufiger) Gast / Bekanntschaft" | Mitglied, Gast, Bekanntschaft |
| `+N` | plus | number of extra people |
| `E-Mail` | email, mail | contact for the handover |
| `Kennt` | knows | handles, comma separated |
| `Gruppe` | group | ja = may exceed +N max, block allowed |
| `Ausgestiegen` | out, raus, left | ja = gives up all slots that are not pinned |
| `Notiz` | note | free text, never part of viewer links |

## Links and modes

The page keeps its whole state in the URL fragment (`#s=…`,
base64url-encoded deflated JSON). Three modes, all views on the same data —
**not** access control: whoever has a link has what is in it. The header
shows a four-emoji fingerprint of the current data (settings, roster,
placement); admin and viewer links of the same state show the same emoji,
so two people can tell at a glance whether they look at the same plan.

- **Admin**: settings, roster, pins and the organiser's address. Edit, compute,
  pin cells, copy links.
- **Viewer**: settings, roster without notes, the organisers' address, the
  live status and the frozen plan (every placed slot pinned), so it renders
  identically on any later version. Filter "Ich bin …" to find your row; the
  Mail link prepares the handover mail to the next person, "melden" a status
  report to the organisers. Admins can copy a personal viewer link per
  person ("Link" in the sheet table) that highlights that person's rows.
  The viewer prints as a clean sheet: matrix, handovers, who-when-whom.
- **Sammeln / collect**: a sign-up form for people interested in a voucher.
  It carries only the event settings and the organiser's address; the result
  is one line (`Name; E-Mail; Status; +N; Kreditkarte; Kennt; Notiz`) that is
  mailed or copied to the organiser, who pastes it into "Zeilen aus
  Sammel-Mails hinzufügen".

## Live status

There is no server, so the status is curated: admins click a slot's status
through *offen → Voucher da → Ticket gekauft → weitergegeben* in the sheet
table, the status and its timestamp travel in the link, and a re-shared
viewer link shows everyone where each voucher is, who is next, and what is
overdue (date before today, status not advanced; marked only once the first
status has been set, so an untouched plan is never "overdue"). Participants report their
status with the "melden" mail link; admins enter it. Two admins compare
fingerprint and status timestamp to see who holds the newer link.

## Demo and example

`events/congress2026.json` is a pseudonymised real roster (25 animals, 22
extra slots, 7 vouchers → 7 days, voucher phase of 11 days with 30 slots
free). Status, credit card, e-mail and
connections are **placeholders**; the real values live only in your own links
and files. The page's "Demo öffnen" link is an admin link of this roster,
generated at build time; the file itself is not part of the site.

## Command line

```
go install github.com/bmmmm/vouch0rzii@latest

vouch0rzii -event events/congress2026.json              # Markdown: matrix + handovers
vouch0rzii -event events/congress2026.json -format csv   # csv | json | svg | event
vouch0rzii -event e.json -roster paste.tsv -format event # spreadsheet paste → event file
vouch0rzii -event e.json -pins pins.json -seed other     # manual pins, new draw
vouch0rzii -event e.json -share view                     # frozen viewer link
vouch0rzii -event e.json -share collect -to orga@example.org
```

Exit status 2 means the input was rejected; the reasons are on stderr.

## Development

```
go test ./... && go vet ./... && GOOS=js GOARCH=wasm go vet ./cmd/vouch0rzii-wasm
go build -o vouch0rzii . && scripts/build-web.sh && node scripts/web-smoke.mjs
node scripts/web-browser-smoke.mjs             # headless Chrome over web/dist, all three modes
cd web/dist && python3 -m http.server 8077     # then open http://localhost:8077/ (file:// cannot fetch)
```

`scripts/web-smoke.mjs` proves the wasm build plans byte-equal to the CLI and
pins the privacy contract (CSP, no storage, no external resources);
`scripts/web-browser-smoke.mjs` loads the page in headless Chrome and checks
the DOM of the empty page and of a demo, an admin, a viewer and a collect
link. Agent instructions live in `AGENTS.md`, the security policy in
`SECURITY.md`, notable changes in `CHANGELOG.md`.
Layout: `internal/plan` (model, codecs, scheduling, assignment, rendering,
share state), `main.go` (CLI), `cmd/vouch0rzii-wasm` (browser bridge),
`web/` (page), `events/` (examples).

## License

GPL-3.0-or-later, see `LICENSE`.
