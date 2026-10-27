# Changelog

Notable changes to vouch0rzii. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); this project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html). While the major
version is 0, behaviour may change in a minor release — the entries below say
what to check.

## [Unreleased]

## [0.1.0] - 2026-10-27

### Added

- Planner: tiers (members, guests, acquaintances, then +1, +2, overflow),
  seeded draw per tier, minimal days, per-day assignment that favours
  handovers between people who know each other, 2-opt around pins.
- Pins: move or swap slots; viewer links are frozen; "Bis heute festnageln".
- Mid-run changes: extra vouchers that start later, people who leave keep
  only pinned slots.
- Live status per slot (offen → Voucher da → Ticket gekauft →
  weitergegeben), curated by admins, carried in the link, with per-voucher
  summary and overdue marks; status reports by mail.
- Share links: admin, viewer, collect (sign-up form), personal viewer links;
  four-emoji fingerprint of the data version.
- Outputs: Markdown, CSV, JSON, SVG flow diagram; handover mails.
- CLI (`-format md|csv|json|svg|event`, `-share`), wasm page, demo link.
- Gates: Go tests, CLI == wasm smoke with privacy contract, headless-Chrome
  DOM gate.
