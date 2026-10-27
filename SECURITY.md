# Security Policy

vouch0rzii plans who holds which voucher on which day. The page keeps names,
e-mail addresses and the plan in the URL fragment and nowhere else; the
security-relevant failure modes are data leaving the page, a crafted link or
roster breaking the page, and a plan that silently differs between the page
and the CLI.

## In scope

- **Data flow of the web version:** anything in
  <https://bmmmm.github.io/vouch0rzii/> that moves roster data off the page
  other than through the links, mails and downloads the user triggers — a
  request carrying data, a write to storage, a resource pulled from another
  origin. The contract rests on README "Links and modes" and is pinned by
  `scripts/web-smoke.mjs` (privacy checks) and the CSP in `web/index.html`.
- **Crafted links or rosters:** input that crashes or hangs the page or the
  CLI, escapes the limits in `internal/plan/model.go`, or injects markup
  through a handle (labels are escaped in the SVG and never set as HTML).
- **Plan integrity:** a difference between the CLI's JSON and the page's for
  the same input, or a handover missing from the lists while present in the
  grid.

## Out of scope

- Whoever holds a link holds its data by design; links are not access
  control, the README says so. Sharing a link with the wrong person is not a
  vulnerability of the tool.
- The mail clients, messengers and browsers the links travel through.
- The ticket shop the vouchers are used in.

## Reporting

Please do **not** open a public issue for a suspected vulnerability.

- Preferred: GitHub private vulnerability reporting —
  <https://github.com/bmmmm/vouch0rzii/security/advisories/new>
- Alternatively: email `hi@brtsz.de`

You'll get an acknowledgement within a few days. Please include the link or
event file that triggers the problem (pseudonymised is fine), the browser or
CLI version, and what you observed.
