// SPDX-License-Identifier: GPL-3.0-or-later
//
// Page logic for vouch0rzii. All planning happens in the Go wasm build
// (globalThis.vouch0rzii); this file only moves data between the form, the
// URL fragment and the DOM. The fragment "#s=…" is the whole state (see
// internal/plan/share.go); modes admin/view/board/collect are views on it,
// not access control. The live status per slot is curated by admins and
// travels in the same state.
'use strict';
(() => {
  const $ = (sel) => document.querySelector(sel);
  const el = (tag, attrs = {}, ...children) => {
    const n = document.createElement(tag);
    for (const [k, v] of Object.entries(attrs)) {
      if (v === null || v === undefined || v === false) continue;
      if (k === 'class') n.className = v;
      else if (k === 'dataset') Object.assign(n.dataset, v);
      else if (k.startsWith('on')) n.addEventListener(k.slice(2), v);
      else n.setAttribute(k, v === true ? '' : v);
    }
    for (const c of children.flat()) {
      if (c === null || c === undefined || c === false) continue;
      n.append(c.nodeType ? c : document.createTextNode(String(c)));
    }
    return n;
  };

  // ---------- language. Static text sits in index.html in both languages;
  // TEXT holds what this file builds. The choice follows the browser and
  // the header button, and stays out of the link.
  const TEXT = {
    de: {
      title: 'vouch0rzii — Voucher-Kettenplaner',
      mode: { admin: 'Admin', view: 'Ansicht', board: 'Board', collect: 'Anmeldung' },
      status: { open: 'offen', received: 'Voucher da', bought: 'Ticket gekauft', passed: 'weitergegeben' },
      orga: 'Orga',
      nothingToUndo: 'Nichts rückgängig zu machen', undone: 'Rückgängig', nothingToRedo: 'Nichts zu wiederholen', redone: 'Wiederholt',
      errorsHead: 'Das geht so nicht:', warningsHead: 'Hinweise:',
      copied: (what) => `${what} kopiert`, copyFailed: 'Kopieren fehlgeschlagen',
      linkUnreadable: (e) => `Der Link konnte nicht gelesen werden: ${e}`,
      cellAria: 'Handle für diese Zelle',
      newStatus: 'Neuer Status',
      chainDone: 'Kette abgeschlossen',
      moving: (to, from, day) => `unterwegs zu ${to}, ${from} hat weitergegeben, Kauf am ${day}`,
      holding: (who, st) => `bei ${who} (${st})`, holdingNext: (who, day) => `, als Nächstes ${who} am ${day}`, lastStop: ', letzte Station',
      atOrga: (who, day) => `noch bei der Orga, Start ${who} am ${day}`, noSlots: 'ohne Slots',
      statusStand: (n, at) => `${n} Slot(s) mit Status, Stand ${at}.`,
      noStatus: 'Noch kein Status gesetzt. Überfällig wird erst markiert, sobald der erste Status gesetzt ist.',
      publicHead: 'Freier Verkauf: ', publicText: (n) => `${n} Ticket(s) passen nicht mehr in die Voucher-Phase und müssen regulär gekauft werden: `,
      overdue: 'überfällig',
      startOn: (day) => `Start am ${day}`, phaseOver: 'Voucher-Phase beendet', lastDayWas: (day) => `Letzter Plantag war ${day}`,
      dayOf: (n, total) => `Tag ${n} von ${total}`, endAt: (day) => ` · Ende ${day}`,
      kpiMoving: 'Voucher unterwegs', kpiDone: 'Ketten abgeschlossen', kpiFree: 'Slots frei',
      nobody: 'niemand', nextOn: (day) => `Als Nächstes, ${day}`, next: 'Als Nächstes', fingerprint: (fp) => ` Kennung ${fp}.`,
      bought: (n, total) => `${n} von ${total} Tickets gekauft`,
      stats: (v, s) => `${v} Vouchers · ${s.days} Tage · ${s.slots} Slots · ${s.handovers} Übergaben, davon ${s.known} unter Buddies, ${s.unknown} neue Verbindungen` +
        (s.forbidden ? `, ${s.forbidden} direkt an eigene Begleitung` : '') + (s.pinned ? ` · ${s.pinned} gepinnt` : ''),
      handoverAt: (at) => ` · Übergabe ${at}`,
      order: (seed) => `Reihenfolge: erst Mitglieder, dann häufige Gäste, dann Bekanntschaften; innerhalb jeder Gruppe entscheidet das Los mit Seed „${seed}" (gleicher Seed, gleiche Losung). Danach die Begleitungen rundenweise: alle +1, dann alle +2 und so weiter. Eine Zahl in der Spalte „Order" überstimmt das Los.`,
      blockPins: 'Gewürfelt wird nur vor dem Start: es gibt schon Pins.',
      blockStatus: 'Gewürfelt wird nur vor dem Start: es gibt schon einen Status.',
      blockNothing: 'Nichts zu losen: alle haben eine feste Reihenfolge.',
      drawing: 'Lose werden gezogen …',
      drawn: 'Gelost in jeder Stufe, dann jeder Tag nach Bekanntschaften optimiert.',
      entropyInfo: (n, of, bits) => `${n} von ${of} Bewegungen · ≈ ${bits} Bit gesammelt`,
      tierNames: { member: 'Mitglieder', guest: 'Häufige Gäste', acquaintance: 'Bekanntschaften' },
      nerdChain: (seed) => `Seed „${seed}" → FNV-1a-64-Hash, ein zweiter setzt ihn mit Salz fort → PCG-Generator → Fisher–Yates-Mischen. Jede Stufe hat ihren eigenen Generator (Seed + Stufe), darum verschiebt eine neue Anmeldung bei den Gästen niemanden bei den Mitgliedern.`,
      nerdTier: (name, x, orders) => [`${name}: ${x.drawn} gelost → ${x.drawn}! = `, orders, x.orders === '1' ? ' Reihenfolge' : ' Reihenfolgen', x.fixed ? ` (${x.fixed} mit fester Reihenfolge)` : ''],
      nerdTotal: (lineups) => ['Zusammen ', lineups, ' mögliche Losergebnisse (Ausgestiegene werden mitgelost, damit niemand verrutscht).'],
      nerdSeeds: '🎲 nimmt 65 Bit aus dem Kryptozufall des Browsers (32¹³ ≈ 3,7 × 10¹⁹ Seeds), 🖱️ mischt zusätzlich deine Bewegungen per SHA-256 hinein. Den Generator erreichen davon nur 64 Bit: höchstens 2⁶⁴ ≈ 1,8 × 10¹⁹ Startzustände, für alle Stufen zusammen. Gibt es mehr Losergebnisse, erreicht ein Wurf nur einen Teil von ihnen.',
      nerdHonest: 'Gleicher Seed, gleiche Aufstellung, auf jedem Gerät; mit der CLI kann es jede Person nachrechnen. Gewürfelt wird nur vor dem Start, ohne Pins und Status, damit niemand laufende Ketten umwirft. Wer oft würfelt, könnte sich eine Aufstellung aussuchen; welche gilt, zeigt der Seed im Link.',
      phase: (day, days, free) => `Voucher-Phase bis ${day} (${days} Tage) · ${free} Slots frei`,
      noEnd: (free) => `Kein Ende der Voucher-Phase gesetzt · ${free} Slots frei im Plan`,
      addDay: '+ Tag', free: 'frei', notYet: 'Voucher noch nicht verfügbar', fromOrga: 'bekommt den Voucher von der Orga',
      freeCell: 'Voucher verfügbar, noch niemand eingeplant', unpin: 'Pin lösen',
      evening: (day, date, at) => `${day} (${date}), ${at} Uhr`,
      forbidden: 'direkt an eigene Begleitung', known: 'Buddies, kennen sich schon', stranger: 'neue Verbindung',
      buysOn: (day) => `, kauft am ${day}`, oneDay: 'Nur ein Tag, keine Übergaben.',
      mail: (to, label, date, v, ev, at, next, nextMail, from) => [
        `Hallo ${to}${label !== to ? ` (für ${label})` : ''},`, '',
        `am ${date} bekommst du von mir Voucher ${v} für ${ev}:`, '', '<CODE HIER EINFÜGEN>', '',
        `Kauf bitte an dem Tag dein Ticket und gib den neuen Voucher um ${at} Uhr weiter` +
          (next ? ` an ${next}${nextMail ? ` (${nextMail})` : ''}.` : '. Du bist die letzte Station dieser Kette.'),
        '', `Viele Grüße, ${from}`,
      ],
      mailSubject: (v, ev) => `Voucher ${v} für ${ev}`,
      pickAll: 'Alle sichtbaren Zeilen auswählen', pickRow: (label) => `${label} auswählen`,
      heads: ['Slot', 'Kontakt', 'Tag', 'Voucher', 'bekommt von', 'gibt weiter an', 'Status', ''],
      nextStatus: 'Klick: nächster Status', chainEnds: '— Kette endet',
      report: 'melden', reportTitle: 'Statusmeldung an die Orga vorbereiten', mailTitle: 'Mail an die nächste Person vorbereiten',
      linkTitle: (who) => `Viewer-Link, der die Zeilen von ${who} hervorhebt`, linkFor: (who) => `Link für ${who}`,
      picked: (n, hidden) => `${n} ausgewählt${hidden ? `, davon ${hidden} vom Filter ausgeblendet` : ''}, Status setzen:`,
      pickHint: 'Zeilen ankreuzen, dann Status für alle setzen:',
      fileUnreadable: (e) => `Datei nicht lesbar: ${e}`, skipped: (list) => `Schon vorhanden, übersprungen: ${list}`,
      added: (n) => `${n} Person(en) hinzugefügt, jetzt „Plan berechnen"`, nothingAdded: 'Nichts Neues hinzugefügt',
      noneGone: 'Noch kein Tag vergangen, nichts festzunageln', frozen: (n) => `${n} Slot(s) bis heute festgenagelt`,
      adminLink: (fp) => `Admin-Link, Stand ${fp},`, viewLink: (fp) => `Viewer-Link, Stand ${fp},`, boardLink: (fp) => `Board-Link, Stand ${fp},`,
      collectLink: 'Sammel-Link', needTo: 'Für den Sammel-Link braucht es die Orga-Adresse (Feld oben).', needName: 'Für den Sammel-Link braucht es den Event-Namen.',
      line: 'Zeile', theEvent: 'das Event',
      loadFailed: (e) => `Der Planer konnte nicht geladen werden. Der Browser braucht WebAssembly. ${e}`,
    },
    en: {
      title: 'vouch0rzii — voucher chain planner',
      mode: { admin: 'Admin', view: 'View', board: 'Board', collect: 'Sign-up' },
      status: { open: 'open', received: 'voucher received', bought: 'ticket bought', passed: 'passed on' },
      orga: 'organisers',
      nothingToUndo: 'Nothing to undo', undone: 'Undone', nothingToRedo: 'Nothing to redo', redone: 'Redone',
      errorsHead: 'That does not work:', warningsHead: 'Notes:',
      copied: (what) => `${what} copied`, copyFailed: 'Copying failed',
      linkUnreadable: (e) => `The link could not be read: ${e}`,
      cellAria: 'Handle for this cell',
      newStatus: 'New status',
      chainDone: 'chain complete',
      moving: (to, from, day) => `on its way to ${to}, ${from} passed it on, buys on ${day}`,
      holding: (who, st) => `with ${who} (${st})`, holdingNext: (who, day) => `, next ${who} on ${day}`, lastStop: ', last stop',
      atOrga: (who, day) => `still with the organisers, starts with ${who} on ${day}`, noSlots: 'no slots',
      statusStand: (n, at) => `${n} slot(s) with a status, as of ${at}.`,
      noStatus: 'No status set yet. Overdue is only marked once the first status is set.',
      publicHead: 'Public sale: ', publicText: (n) => `${n} ticket(s) no longer fit into the voucher phase and have to be bought regularly: `,
      overdue: 'overdue',
      startOn: (day) => `Starts on ${day}`, phaseOver: 'Voucher phase over', lastDayWas: (day) => `Last plan day was ${day}`,
      dayOf: (n, total) => `Day ${n} of ${total}`, endAt: (day) => ` · ends ${day}`,
      kpiMoving: 'vouchers on their way', kpiDone: 'chains complete', kpiFree: 'slots free',
      nobody: 'nobody', nextOn: (day) => `Next, ${day}`, next: 'Next', fingerprint: (fp) => ` Fingerprint ${fp}.`,
      bought: (n, total) => `${n} of ${total} tickets bought`,
      stats: (v, s) => `${v} vouchers · ${s.days} days · ${s.slots} slots · ${s.handovers} handovers, ${s.known} of them between buddies, ${s.unknown} new connections` +
        (s.forbidden ? `, ${s.forbidden} straight to one's own companion` : '') + (s.pinned ? ` · ${s.pinned} pinned` : ''),
      handoverAt: (at) => ` · handover ${at}`,
      order: (seed) => `Order: members first, then frequent guests, then acquaintances; within each tier the draw with seed "${seed}" decides (same seed, same draw). Then the companions round by round: every +1, then every +2 and so on. A number in "Order" overrides the draw.`,
      blockPins: 'Rolling is only for the start: there are pins already.',
      blockStatus: 'Rolling is only for the start: there is a status already.',
      blockNothing: 'Nothing to draw: everyone has a fixed order.',
      drawing: 'Drawing lots …',
      drawn: 'Drawn within each tier, then every day optimised for people who know each other.',
      entropyInfo: (n, of, bits) => `${n} of ${of} movements · ≈ ${bits} bits collected`,
      tierNames: { member: 'Members', guest: 'Frequent guests', acquaintance: 'Acquaintances' },
      nerdChain: (seed) => `Seed "${seed}" → FNV-1a 64 hash, a second one continues it with a salt → PCG generator → Fisher–Yates shuffle. Every tier has its own generator (seed + tier), so a new guest never moves a member.`,
      nerdTier: (name, x, orders) => [`${name}: ${x.drawn} drawn → ${x.drawn}! = `, orders, x.orders === '1' ? ' order' : ' orders', x.fixed ? ` (${x.fixed} with a fixed order)` : ''],
      nerdTotal: (lineups) => ['Together ', lineups, ' possible results of the draw (people who left are drawn too, so nobody shifts).'],
      nerdSeeds: '🎲 takes 65 bits from the browser\'s crypto randomness (32¹³ ≈ 3.7 × 10¹⁹ seeds), 🖱️ also mixes in your movements through SHA-256. Only 64 of those bits reach the generator: at most 2⁶⁴ ≈ 1.8 × 10¹⁹ starting states, for all tiers together. With more results than that, a roll reaches only part of them.',
      nerdHonest: 'Same seed, same lineup, on every device; anyone can recompute it with the CLI. Rolling is only possible before the start, without pins or status, so nobody upsets running chains. Rolling many times could pick a lineup; the seed in the link shows which one counts.',
      phase: (day, days, free) => `Voucher phase until ${day} (${days} days) · ${free} slots free`,
      noEnd: (free) => `No end of the voucher phase set · ${free} slots free in the plan`,
      addDay: '+ day', free: 'free', notYet: 'Voucher not available yet', fromOrga: 'gets the voucher from the organisers',
      freeCell: 'Voucher available, nobody planned yet', unpin: 'Release pin',
      evening: (day, date, at) => `${day} (${date}), ${at}`,
      forbidden: "straight to one's own companion", known: 'buddies, already know each other', stranger: 'new connection',
      buysOn: (day) => `, buys on ${day}`, oneDay: 'Only one day, no handovers.',
      mail: (to, label, date, v, ev, at, next, nextMail, from) => [
        `Hello ${to}${label !== to ? ` (for ${label})` : ''},`, '',
        `on ${date} you get voucher ${v} for ${ev} from me:`, '', '<PASTE CODE HERE>', '',
        `Please buy your ticket on that day and pass the new voucher on at ${at}` +
          (next ? ` to ${next}${nextMail ? ` (${nextMail})` : ''}.` : '. You are the last stop of this chain.'),
        '', `Best, ${from}`,
      ],
      mailSubject: (v, ev) => `Voucher ${v} for ${ev}`,
      pickAll: 'Select all visible rows', pickRow: (label) => `Select ${label}`,
      heads: ['Slot', 'Contact', 'Day', 'Voucher', 'gets it from', 'passes it to', 'Status', ''],
      nextStatus: 'Click: next status', chainEnds: '— chain ends',
      report: 'report', reportTitle: 'Prepare a status report to the organisers', mailTitle: 'Prepare a mail to the next person',
      linkTitle: (who) => `Viewer link that highlights the rows of ${who}`, linkFor: (who) => `Link for ${who}`,
      picked: (n, hidden) => `${n} selected${hidden ? `, ${hidden} of them hidden by the filter` : ''}, set status:`,
      pickHint: 'Tick rows, then set a status for all of them:',
      fileUnreadable: (e) => `File not readable: ${e}`, skipped: (list) => `Already there, skipped: ${list}`,
      added: (n) => `${n} person(s) added, now "Compute plan"`, nothingAdded: 'Nothing new added',
      noneGone: 'No day has passed yet, nothing to pin', frozen: (n) => `${n} slot(s) pinned until today`,
      adminLink: (fp) => `Admin link, state ${fp},`, viewLink: (fp) => `Viewer link, state ${fp},`, boardLink: (fp) => `Board link, state ${fp},`,
      collectLink: 'Sign-up link', needTo: 'The sign-up link needs the organiser address (field above).', needName: 'The sign-up link needs the event name.',
      line: 'Line', theEvent: 'the event',
      loadFailed: (e) => `The planner could not be loaded. The browser needs WebAssembly. ${e}`,
    },
  };
  let lang = 'de';
  const t = (key, ...args) => { const v = TEXT[lang][key]; return typeof v === 'function' ? v(...args) : v; };
  // Go writes English labels ("Fri 30/10", "organisers"); the German page relabels.
  const dayName = (d) => {
    if (lang === 'en' || !d.date) return d.label;
    const [y, m, day] = d.date.split('-');
    return `${['So', 'Mo', 'Di', 'Mi', 'Do', 'Fr', 'Sa'][new Date(y, m - 1, day).getDay()]} ${day}.${m}.`;
  };

  const DEFAULT_EVENT = { name: '', vouchers: 7, start: '', weekends: false, plusMax: 2, seed: '', handover: '23:00', people: [] };
  const STATUS = ['open', 'received', 'bought', 'passed'];
  const statusLabel = (st) => t('status')[st];

  const emptyState = () => ({ v: 1, mode: 'admin', event: { ...DEFAULT_EVENT }, pins: [], collect: { to: '' }, status: {}, statusAt: '', me: '' });
  let bridge = null;
  let state = emptyState();
  let plan = null;      // last export plus exportJson/markdown/csv/svg
  let selected = null;  // cell chosen as move source: {label, day, voucher}
  // ?demo marks the demo link (scripts/build-web.sh): it shows the draw on
  // open. A display flag only; links built here never carry it.
  let demo = new URLSearchParams(location.search).has('demo'); // cleared by the first change
  let drawShown = false; // the first plan computed or opened on this page

  // ---------- undo / redo: in memory, admin only; the link always holds the current state
  const past = [], future = [];
  // canon serialises with sorted keys, so the same data in another key order
  // (Go JSON vs. objects built here) is not mistaken for a change.
  const canon = (v) => Array.isArray(v) ? `[${v.map(canon).join(',')}]`
    : v && typeof v === 'object' ? `{${Object.keys(v).sort().map((k) => `${JSON.stringify(k)}:${canon(v[k])}`).join(',')}}` : JSON.stringify(v);
  const snapshot = () => canon({ event: state.event, pins: state.pins, collect: state.collect, status: state.status, statusAt: state.statusAt });
  // mutate runs fn and remembers the state before it, if fn changed anything.
  function mutate(fn) {
    const before = snapshot();
    fn();
    if (snapshot() === before) return;
    demo = false; // from here on it is the admin's own plan
    past.push(before);
    if (past.length > 100) past.shift();
    future.length = 0;
  }
  function restore(s) {
    Object.assign(state, JSON.parse(s));
    selected = null;
    eventToForm();
    peopleToRoster(state.event.people);
    computePlan({ fromForm: false });
  }
  // Both first apply what is typed in the form, so pending roster edits
  // become a step of their own instead of being overwritten.
  function undo() {
    if (!computePlan()) return;
    if (!past.length) { toast(t('nothingToUndo')); return; }
    future.push(snapshot());
    restore(past.pop());
    toast(t('undone'));
  }
  function redo() {
    if (!computePlan()) return;
    if (!future.length) { toast(t('nothingToRedo')); return; }
    past.push(snapshot());
    restore(future.pop());
    toast(t('redone'));
  }

  // ---------- wasm
  async function loadWasm() {
    const go = new Go();
    let result;
    try {
      result = await WebAssembly.instantiateStreaming(fetch('vouch0rzii.wasm'), go.importObject);
    } catch {
      const buf = await (await fetch('vouch0rzii.wasm')).arrayBuffer();
      result = await WebAssembly.instantiate(buf, go.importObject);
    }
    go.run(result.instance); // resolves only when the Go program exits; it never does
    for (let i = 0; i < 200 && !globalThis.vouch0rzii; i++) await new Promise((r) => setTimeout(r, 10));
    if (!globalThis.vouch0rzii) throw new Error('vouch0rzii global not registered');
    return globalThis.vouch0rzii;
  }

  // ---------- messages, clipboard, files
  // showMessages renders errors and warnings right after the control that
  // caused them (anchor), where the admin is looking; without an anchor the
  // box sits at the top of the page. A message is a string from Go or a
  // function that builds page text, so a language switch can build it again.
  let shown = [[], []];
  function showMessages(errors = [], warnings = [], anchor = null, scroll = true) {
    shown = [errors, warnings];
    const box = $('#messages');
    (anchor || $('#loading')).after(box);
    box.replaceChildren();
    const li = (m) => el('li', {}, typeof m === 'function' ? m() : m);
    if (errors.length) box.append(el('div', { class: 'error' }, el('strong', {}, t('errorsHead')), el('ul', {}, errors.map(li))));
    if (warnings.length) box.append(el('div', { class: 'warning' }, el('strong', {}, t('warningsHead')), el('ul', {}, warnings.map(li))));
    if (errors.length && scroll) box.scrollIntoView({ block: 'nearest' });
  }
  let toastTimer = 0;
  function toast(msg) {
    const t = $('#toast');
    t.textContent = msg;
    t.hidden = false;
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => { t.hidden = true; }, 1800);
  }
  async function copy(text, what) {
    try {
      await navigator.clipboard.writeText(text);
      toast(t('copied', what));
    } catch {
      toast(t('copyFailed'));
    }
  }
  function download(name, text, type) {
    const a = el('a', { href: URL.createObjectURL(new Blob([text], { type })), download: name });
    document.body.append(a);
    a.click();
    a.remove();
  }
  function mailto(to, subject, body) {
    return `mailto:${to}?subject=${encodeURIComponent(subject)}&body=${encodeURIComponent(body)}`;
  }
  function fmtDate(d) {
    const p = (n) => String(n).padStart(2, '0');
    return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`;
  }
  const today = () => fmtDate(new Date());
  function fmtAt(iso) {
    const d = new Date(iso);
    return isNaN(d) ? iso : d.toLocaleString(lang === 'de' ? 'de-DE' : 'en-GB', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' });
  }

  // ---------- state <-> URL
  function encodeState(st) {
    const r = bridge.encodeState(JSON.stringify(st));
    if (!r.ok) throw new Error(r.errors.join('; '));
    return r.encoded;
  }
  function readHash() {
    const m = location.hash.match(/^#s=([A-Za-z0-9_-]+)$/);
    if (!m) return null;
    const r = bridge.decodeState(m[1]);
    if (!r.ok) {
      showMessages([() => t('linkUnreadable', r.errors.join('; '))]);
      return null;
    }
    return JSON.parse(r.stateJson);
  }
  function writeHash() {
    if (state.mode !== 'admin') return;
    try { history.replaceState(null, '', location.href.split(/[?#]/)[0] + '#s=' + encodeState(state)); } catch (e) { showMessages([String(e)]); } // drops ?demo
  }
  function linkFor(mode, extra) {
    return location.href.split(/[?#]/)[0] + '#s=' + encodeState({ ...state, mode, ...extra }); // never the ?demo flag
  }

  // ---------- form <-> state.event
  // readForm returns the settings as typed, without touching state; only a
  // successful computePlan commits them, so links and the fingerprint never
  // mix a new form with an old plan.
  function readForm() {
    const ev = { ...state.event };
    ev.name = $('#f-name').value.trim();
    ev.vouchers = Number($('#f-vouchers').value) || 0;
    ev.start = $('#f-start').value;
    ev.end = $('#f-end').value;
    if (!ev.end) delete ev.end;
    ev.weekends = $('#f-weekends').checked;
    ev.plusMax = Number($('#f-plusMax').value) || 0;
    ev.seed = $('#f-seed').value.trim();
    ev.handover = $('#f-handover').value.trim();
    const extra = $('#f-extra').value.split(/[,\s]+/).filter(Boolean).map((tok) => {
      const m = tok.match(/^(\d{4}-\d{2}-\d{2})(?::(\d+))?$/);
      return m ? { date: m[1], count: Number(m[2] || 1) } : { date: tok, count: 1 }; // Go rejects a bad date
    });
    if (extra.length) ev.extra = extra; else delete ev.extra;
    return { event: ev, to: $('#f-collectTo').value.trim() };
  }
  function eventToForm() {
    const ev = state.event;
    $('#f-name').value = ev.name || '';
    $('#f-vouchers').value = ev.vouchers || 7;
    $('#f-start').value = ev.start || '';
    $('#f-end').value = ev.end || '';
    $('#f-weekends').checked = !!ev.weekends;
    $('#f-plusMax').value = ev.plusMax || 2;
    $('#f-seed').value = ev.seed || '';
    $('#f-handover').value = ev.handover || '23:00';
    $('#f-extra').value = (ev.extra || []).map((x) => `${x.date}:${x.count}`).join(', ');
    $('#f-collectTo').value = (state.collect && state.collect.to) || '';
  }
  // rosterToPeople reads the textarea; null means errors were shown, next to
  // anchor.
  function rosterToPeople(anchor = $('#compute').parentElement) {
    const r = bridge.parseRoster($('#roster').value);
    if (!r.ok) {
      showMessages(r.errors, r.warnings, anchor);
      return null;
    }
    return { people: JSON.parse(r.peopleJson), warnings: r.warnings };
  }
  function peopleToRoster(people) {
    const r = bridge.formatRoster(JSON.stringify(people || []));
    $('#roster').value = r.ok ? r.tsv : '';
  }

  // ---------- planning
  function computePlan({ fromForm = true, shuffle = false } = {}) {
    let warnings = [];
    const anchor = fromForm ? $('#compute').parentElement : null; // form problems show under "Compute plan"
    if (fromForm) {
      const f = readForm();
      const r = rosterToPeople();
      if (!r) return false;
      mutate(() => { state.event = { ...f.event, people: r.people }; state.collect = { to: f.to }; });
      warnings = r.warnings;
    }
    const r = bridge.plan(JSON.stringify(state.event), JSON.stringify(state.pins));
    if (!r.ok) {
      showMessages(r.errors, warnings.concat(r.warnings), anchor);
      plan = null;
      renderResult();
      return false;
    }
    plan = { ...JSON.parse(r.exportJson), exportJson: r.exportJson, markdown: r.markdown, csv: r.csv, svg: r.svg };
    showMessages([], warnings.concat(plan.warnings), anchor);
    renderResult();
    if (shuffle) shuffleMatrix();
    drawShown = true;
    writeHash();
    return true;
  }
  function loadEvent(ev) {
    mutate(() => { state = { ...emptyState(), event: { ...DEFAULT_EVENT, ...ev } }; });
    selected = null;
    picked.clear();
    eventToForm();
    peopleToRoster(state.event.people);
    computePlan({ fromForm: false });
  }

  // ---------- pins (manual moves)
  const withPin = (pins, label, day, voucher) => [...pins.filter((p) => p.label !== label), { label, day, voucher }];
  function setPin(label, day, voucher) {
    state.pins = withPin(state.pins, label, day, voucher);
  }
  function unpin(label) {
    mutate(() => { state.pins = state.pins.filter((p) => p.label !== label); });
    selected = null;
    computePlan({ fromForm: false });
  }
  function positions() {
    const pos = new Map();
    plan.grid.forEach((row, d) => row.forEach((l, v) => { if (l) pos.set(l, { label: l, day: d + 1, voucher: v + 1 }); }));
    return pos;
  }
  function moveSlot(label, target) {
    const src = positions().get(label);
    if (!src || (src.day === target.day && src.voucher === target.voucher)) { selected = null; renderMatrix(); return; }
    mutate(() => {
      if (target.label && target.label !== label) setPin(target.label, src.day, src.voucher);
      setPin(label, target.day, target.voucher);
    });
    selected = null;
    computePlan({ fromForm: false });
  }
  function onCellPick(cell, td) {
    if (!selected) {
      if (cell.label) { selected = cell; renderMatrix(); } else if (!cell.extra) nameCell(td, cell);
      return;
    }
    if (selected.label === cell.label) { selected = null; renderMatrix(); return; }
    moveSlot(selected.label, cell);
  }
  // nameCell turns a free cell into an input. Enter commits, Escape cancels,
  // any re-render of the matrix discards it; assignLabel resolves the text.
  function nameCell(td, cell) {
    const input = el('input', { class: 'cellname', placeholder: 'Handle', 'aria-label': t('cellAria') });
    const refocus = () => { const c = $(`#matrix tbody tr:nth-child(${cell.voucher}) td:nth-child(${cell.day + 1})`); if (c) c.focus(); };
    input.addEventListener('click', (e) => e.stopPropagation()); // not a second pick of the cell
    input.addEventListener('keydown', (e) => {
      e.stopPropagation();
      const label = input.value.trim();
      if (e.key === 'Enter' && label) { if (assignLabel(label, cell)) refocus(); else renderMatrix(); } // on failure the message keeps the scroll position
      else if (e.key === 'Enter' || e.key === 'Escape') { renderMatrix(); refocus(); }
    });
    td.replaceChildren(input);
    input.focus();
  }
  // assignLabel pins a typed label to a cell: an existing slot moves there,
  // "handle+k" raises that person's +N to k, an unknown handle joins the
  // roster as Bekanntschaft. Pending form edits are applied first; the new
  // roster is tried on a copy, so a rejected name changes nothing and the
  // reasons stay on screen. Returns whether the label was applied.
  function assignLabel(label, cell) {
    if (!computePlan()) return false;
    const m = label.match(/^(.+)\+(\d+)$/);
    const base = (m ? m[1] : label).trim(), k = m ? Number(m[2]) : 0;
    if (!base) return false;
    const people = structuredClone(state.event.people);
    let p = people.find((q) => q.id.toLowerCase() === base.toLowerCase());
    if (!p) { p = { id: base, tier: 'acquaintance' }; people.push(p); }
    if (k) p.plus = Math.max(p.plus || 0, k);
    const pins = withPin(state.pins, k ? `${p.id}+${k}` : p.id, cell.day, cell.voucher);
    const trial = bridge.plan(JSON.stringify({ ...state.event, people }), JSON.stringify(pins));
    if (!trial.ok) { showMessages(trial.errors, trial.warnings, $('#r-pinhelp')); return false; }
    mutate(() => { state.event = { ...state.event, people }; state.pins = pins; });
    selected = null;
    peopleToRoster(people);
    computePlan({ fromForm: false });
    return true;
  }

  // ---------- live status (curated by admins, carried in the link)
  const statusOf = (label) => (STATUS.includes(state.status[label]) ? state.status[label] : 'open');
  // setStatus sets one status for several slots in a single undo step; a
  // click that changes nothing records nothing.
  function setStatus(labels, st) {
    labels = labels.filter((l) => statusOf(l) !== st);
    if (!labels.length) return;
    mutate(() => {
      for (const l of labels) { if (st === 'open') delete state.status[l]; else state.status[l] = st; }
      state.statusAt = new Date().toISOString();
    });
    writeHash();
    renderResult();
  }
  const boughtCount = () => plan.sheets.filter((s) => ['bought', 'passed'].includes(statusOf(s.label))).length;
  const boughtText = () => t('bought', boughtCount(), plan.sheets.length);
  const cycleStatus = (label) => setStatus([label], STATUS[(STATUS.indexOf(statusOf(label)) + 1) % STATUS.length]);
  const picked = new Set(); // slot labels ticked in the sheet table for a bulk status action
  function reportMail(sheet) {
    const to = state.collect && state.collect.to;
    if (!to) return null;
    const next = STATUS[Math.min(STATUS.indexOf(statusOf(sheet.label)) + 1, STATUS.length - 1)];
    const body = `Slot: ${sheet.label}\nVoucher: ${sheet.voucher}\n${t('newStatus')}: ${statusLabel(next)}\nPlan: ${plan.fingerprint}\n`;
    return mailto(to, `vouch0rzii ${plan.event}: ${sheet.label} ${statusLabel(next)}`, body);
  }
  // voucherState reads the curated status of one chain: where the voucher is
  // (kind: done, moving, holding, orga, none) and whether it is overdue.
  // Overdue marks only once the sale is being tracked (track).
  function voucherState(v, now, track) {
    const chain = plan.sheets.filter((s) => s.voucher === v); // in day order
    const st = chain.map((s) => statusOf(s.label));
    const holder = Math.max(st.lastIndexOf('received'), st.lastIndexOf('bought'));
    const passed = st.lastIndexOf('passed');
    const day = (s) => dayName(plan.days[s.day - 1]);
    if (passed > holder) {
      const next = chain[passed + 1];
      if (!next) return { v, kind: 'done', text: t('chainDone') };
      return { v, kind: 'moving', text: t('moving', next.label, chain[passed].label, day(next)), late: track && next.date < now };
    }
    if (holder >= 0) {
      const h = chain[holder];
      const next = chain.slice(holder + 1).find((s) => statusOf(s.label) === 'open');
      return {
        v, kind: 'holding',
        text: t('holding', h.label, statusLabel(st[holder])) + (next ? t('holdingNext', next.label, day(next)) : t('lastStop')),
        late: track && h.date < now && !(st[holder] === 'bought' && !next), // a bought last station is done, not late
      };
    }
    if (chain.length) return { v, kind: 'orga', text: t('atOrga', chain[0].label, day(chain[0])), late: track && chain[0].date < now };
    return { v, kind: 'none', text: t('noSlots') };
  }
  // trackedCount is the number of statuses that belong to a slot of this plan;
  // statuses of renamed or vanished slots neither count nor start tracking.
  function trackedCount() {
    const labels = new Set(plan.sheets.map((s) => s.label));
    return Object.keys(state.status).filter((l) => labels.has(l)).length;
  }
  const tracking = () => trackedCount() > 0;
  function statusStand() {
    const n = trackedCount();
    return n ? t('statusStand', n, fmtAt(state.statusAt)) : t('noStatus');
  }
  const publicBox = (box) => {
    box.hidden = !plan.public.length;
    box.replaceChildren(el('strong', {}, t('publicHead')), t('publicText', plan.public.length), el('b', {}, plan.public.join(', ')));
  };
  function renderLive() {
    const now = today(), track = tracking();
    const items = [];
    for (let v = 1; v <= plan.vouchers; v++) {
      const s = voucherState(v, now, track);
      items.push(el('li', { class: s.late ? 'live overdue' : 'live' }, `Voucher ${v}: ${s.text}`, s.late ? ` — ${t('overdue')}` : ''));
    }
    $('#liveList').replaceChildren(...items);
    $('#liveAt').textContent = statusStand();
  }

  // ---------- board: the viewer's data as a dashboard for everyone
  function renderBoard() {
    const now = today(), track = tracking();
    const sheets = plan.sheets;
    const first = plan.days[0];
    // Plan day number of a date, counting past the grid up to the end.
    const planDay = (date) => {
      const [y, m, d] = first.date.split('-').map(Number);
      let n = 0;
      for (const t = new Date(y, m - 1, d); fmtDate(t) <= date; t.setDate(t.getDate() + 1)) if (state.event.weekends || t.getDay() % 6) n++; // % 6: Sunday 0 and Saturday 6 are skipped
      return n;
    };
    const total = plan.end.day || plan.days.length;
    const last = plan.days[plan.days.length - 1];
    const ended = plan.end.day ? now > plan.end.date : now > last.date;
    $('#b-title').textContent = plan.event;
    $('#b-day').textContent = now < first.date ? t('startOn', dayName(first))
      : ended ? (plan.end.day ? t('phaseOver') : t('lastDayWas', dayName(last)))
        : t('dayOf', planDay(now), total) + (plan.end.day ? t('endAt', dayName(plan.end)) : '');
    const states = [];
    for (let v = 1; v <= plan.vouchers; v++) states.push(voucherState(v, now, track));
    const bar = $('#b-bar');
    bar.max = sheets.length || 1;
    bar.value = boughtCount();
    $('#b-progress').textContent = boughtText();
    $('#b-kpis').replaceChildren(...[
      [t('kpiMoving'), states.filter((s) => s.kind === 'moving').length],
      [t('kpiDone'), states.filter((s) => s.kind === 'done').length],
      [t('overdue'), states.filter((s) => s.late).length],
      [t('kpiFree'), plan.stats.free],
    ].map(([k, n]) => el('div', { class: 'kpi' }, el('b', {}, n), el('span', {}, k))));
    $('#b-vouchers').replaceChildren(...states.map((s) => el('li', { class: `tile ${s.kind}${s.late ? ' late' : ''}` }, el('b', {}, `Voucher ${s.v}`), s.text, s.late && el('small', { class: 'late' }, t('overdue')))));
    const nextDay = plan.days.find((d) => d.date > now);
    const who = (list) => list.length ? list.map((s) => el('li', {}, el('span', { class: `badge st-${statusOf(s.label)}` }, statusLabel(statusOf(s.label))), ` ${s.label} · Voucher ${s.voucher}`)) : [el('li', { class: 'hint' }, t('nobody'))];
    $('#b-today').replaceChildren(...who(sheets.filter((s) => s.date === now)));
    $('#b-nextLabel').textContent = nextDay ? t('nextOn', dayName(nextDay)) : t('next');
    $('#b-next').replaceChildren(...who(nextDay ? sheets.filter((s) => s.day === nextDay.day) : []));
    publicBox($('#b-public'));
    $('#b-at').textContent = statusStand() + t('fingerprint', plan.fingerprint);
    $('#b-plan').href = linkFor('view', {});
  }

  // ---------- rendering
  function renderResult() {
    const board = state.mode === 'board';
    $('#result').hidden = !plan || board;
    $('#board').hidden = !plan || !board;
    $('#intro').hidden = !!plan;
    $('#fingerprint').textContent = plan ? plan.fingerprint : '';
    if (!plan) return;
    if (board) { renderBoard(); return; }
    const s = plan.stats;
    $('#r-title').textContent = `Plan ${plan.event}`;
    $('#r-stats').textContent =
      t('stats', plan.vouchers, s) + t('handoverAt', plan.handover);
    $('#r-order').textContent = t('order', state.event.seed || state.event.name);
    $('#r-phase').textContent = plan.end.day
      ? t('phase', dayName(plan.end), plan.end.day, s.free)
      : t('noEnd', s.free);
    publicBox($('#r-public'));
    const toBoard = $('#toBoard');
    toBoard.href = linkFor('board', { pins: plan.pins });
    if (state.mode === 'admin') toBoard.target = '_blank'; // keep the admin tab and its unsaved edits
    else toBoard.removeAttribute('target');
    $('#r-pinhelp').hidden = state.mode !== 'admin';
    $('#undo').disabled = !past.length;
    $('#redo').disabled = !future.length;
    renderLive();
    renderMatrix();
    $('#diagram').innerHTML = plan.svg; // our own SVG, labels escaped by the Go side
    // The bold texts are the day headers, one per plan day, the only Go text
    // in the diagram that is not a handle.
    $('#diagram').querySelectorAll('text[font-weight="600"]').forEach((n, i) => { n.textContent = dayName(plan.days[i]); });
    renderHandovers();
    renderSheets();
    renderChance();
  }

  // ---------- chance: shuffle animation, dice, super random, nerd panel.
  // Nothing here plans: a roll only writes a new seed and recomputes; the
  // numbers come from Go (plan.draw); Math.random only moves the animation.
  const SEED_ABC = 'abcdefghijkmnpqrstuvwxyz23456789'; // 32 signs, no l o 0 1
  const SEED_LEN = 13; // 65 bits; 64 of them reach the generator (nerd panel)
  const ENTROPY_SAMPLES = 128; // ≈ 0.5 bit per moved sample, a cautious guess
  const reducedMotion = () => matchMedia('(prefers-reduced-motion: reduce)').matches;
  const toSeed = (bytes) => {
    let acc = 0, bits = 0, out = '';
    for (const b of bytes) {
      acc = (acc << 8) | b; bits += 8;
      while (bits >= 5 && out.length < SEED_LEN) { bits -= 5; out += SEED_ABC[(acc >> bits) & 31]; }
      acc &= (1 << bits) - 1;
    }
    return out;
  };
  // Only before the start: no pins and no status on a slot of the plan.
  const randomBlock = () => state.pins.length ? 'blockPins' : tracking() ? 'blockStatus' : plan && plan.draw && plan.draw.lineups === '1' ? 'blockNothing' : '';
  // big shows a decimal string exactly up to 15 digits, beyond as m × 10^e
  // with three significant digits, rounded.
  const big = (s) => {
    if (s.length <= 15) return Number(s).toLocaleString(lang);
    let m = Math.round(Number(s.slice(0, 4)) / 10) / 100, e = s.length - 1;
    if (m >= 10) { m /= 10; e++; }
    return el('span', {}, `≈ ${m.toLocaleString(lang, { minimumFractionDigits: 2, maximumFractionDigits: 2 })} × 10`, el('sup', {}, e));
  };
  // The note next to the dice says what the draw is doing or why the dice
  // are locked; it keeps its key so a language switch can say it again.
  let noteKey = '';
  const note = (key) => { noteKey = key; $('#r-randomnote').textContent = key ? t(key) : ''; };
  let entropyRefresh = () => {};

  function renderChance() {
    const admin = state.mode === 'admin';
    $('#r-random').hidden = $('#r-nerd').hidden = !plan.draw; // a wasm from before the draw export
    if (!plan.draw) return;
    const block = randomBlock();
    for (const b of [$('#roll'), $('#superRandom')]) {
      b.hidden = !admin || (b.id === 'superRandom' && !(globalThis.crypto && crypto.subtle));
      b.disabled = !!block;
      b.title = block ? t(block) : '';
    }
    if (block || !admin) closeEntropy(); // a pin, a status or another link ends super random
    note(admin && block ? block : noteKey.startsWith('block') ? '' : noteKey);
    entropyRefresh();
    $('#r-seed').textContent = plan.draw.seed;
    $('#r-demo').hidden = !(demo && admin);
    const d = plan.draw, names = t('tierNames');
    $('#r-nerdBody').replaceChildren(
      el('p', {}, t('nerdChain', d.seed)),
      el('ul', {}, d.tiers.filter((x) => x.drawn || x.fixed).map((x) => el('li', {}, t('nerdTier', names[x.tier] || x.tier, x, big(x.orders))))),
      el('p', {}, t('nerdTotal', big(d.lineups))),
      el('p', {}, t('nerdSeeds')),
      el('p', {}, t('nerdHonest')));
  }

  // shuffleMatrix lets every filled cell flicker through the plan's labels,
  // then locks the real plan in day by day; a click or key skips it.
  let shuffling = null;
  const stopShuffle = () => { if (shuffling) shuffling(); };
  function shuffleMatrix() {
    stopShuffle();
    if (randomBlock() === 'blockNothing') return; // nothing was drawn, nothing to show
    if (reducedMotion()) { note('drawn'); return; }
    const table = $('#matrix');
    const cells = [...table.querySelectorAll('td.cell:not(.empty):not(.extra)')].filter((td) => td.firstChild && td.firstChild.nodeType === 3);
    if (!cells.length) return;
    const labels = cells.map((td) => td.firstChild.nodeValue);
    const step = Math.min(150, 1100 / plan.days.length);
    const start = performance.now();
    let timer = 0;
    const stop = () => {
      clearInterval(timer);
      cells.forEach((td, i) => { td.firstChild.nodeValue = labels[i]; td.classList.remove('flicker'); });
      table.classList.remove('shuffling');
      table.removeAttribute('aria-busy');
      removeEventListener('pointerdown', stop, true);
      removeEventListener('keydown', stop, true);
      note('drawn');
      shuffling = null;
    };
    const tick = () => {
      const now = performance.now() - start;
      let open = 0;
      cells.forEach((td, i) => {
        if (now >= 350 + (td.cellIndex - 1) * step) { // day columns start at cellIndex 1
          if (td.classList.contains('flicker')) { td.firstChild.nodeValue = labels[i]; td.classList.replace('flicker', 'settle'); }
          return;
        }
        open++;
        td.classList.add('flicker');
        td.firstChild.nodeValue = labels[Math.floor(Math.random() * labels.length)];
      });
      if (!open) stop();
    };
    table.classList.add('shuffling');
    table.setAttribute('aria-busy', 'true');
    note('drawing');
    addEventListener('pointerdown', stop, true);
    addEventListener('keydown', stop, true);
    shuffling = stop;
    tick();
    timer = setInterval(tick, 70);
  }

  // useSeed writes the seed into the form and recomputes through it, so the
  // roll is one undo step and pending form edits come along.
  function useSeed(seed) {
    const was = $('#f-seed').value;
    $('#f-seed').value = seed;
    selected = null;
    if (!computePlan({ shuffle: true })) { $('#f-seed').value = was; return; } // the form has an error; keep the old seed

    const dice = $('#roll .dice');
    if (!reducedMotion()) { dice.classList.remove('spin'); void dice.offsetWidth; dice.classList.add('spin'); }
  }
  const roll = () => { closeEntropy(); useSeed(toSeed(crypto.getRandomValues(new Uint8Array(9)))); };

  // Super random: the pointer's path, with time and pressure, is hashed
  // together with 32 crypto bytes, so the seed is never weaker than a roll.
  let closeEntropy = () => {};
  function openEntropy() {
    if (randomBlock()) return;
    closeEntropy();
    const pool = crypto.getRandomValues(new Uint8Array(32));
    const samples = [];
    const pad = $('#entropyPad');
    let last = '', previewAt = 0, done = false, cancelled = false;
    pad.replaceChildren();
    $('#entropy').hidden = false;
    $('#entropyBar').max = ENTROPY_SAMPLES;
    $('#superRandom').setAttribute('aria-expanded', 'true');
    const digest = async () => {
      const moves = new Uint8Array(new Float64Array(samples.flat()).buffer);
      const buf = new Uint8Array(pool.length + moves.length);
      buf.set(pool);
      buf.set(moves, pool.length);
      return new Uint8Array(await crypto.subtle.digest('SHA-256', buf));
    };
    const info = () => {
      $('#entropyBar').value = samples.length;
      $('#entropyInfo').textContent = t('entropyInfo', samples.length, ENTROPY_SAMPLES, Math.floor(samples.length / 2));
    };
    const onMove = async (e) => {
      if (done) return;
      const r = pad.getBoundingClientRect();
      const x = e.clientX - r.left, y = e.clientY - r.top;
      if (`${x},${y}` === last) return;
      last = `${x},${y}`;
      samples.push([x, y, performance.now(), e.pressure || 0]);
      const dot = el('span', { class: 'dot' });
      dot.style.left = `${x}px`; // CSSOM, allowed by style-src 'self'
      dot.style.top = `${y}px`;
      pad.append(dot);
      if (pad.childElementCount > 60) pad.firstElementChild.remove();
      info();
      if (samples.length >= ENTROPY_SAMPLES) {
        done = true;
        const seed = toSeed(await digest());
        if (cancelled) return; // closed while hashing
        closeEntropy();
        useSeed(seed);
      } else if (samples.length - previewAt >= 8) {
        previewAt = samples.length;
        const preview = toSeed(await digest());
        if (!done) $('#entropySeed').textContent = preview;
      }
    };
    const onKey = (e) => { if (e.key === 'Escape') closeEntropy(); };
    pad.addEventListener('pointermove', onMove);
    document.addEventListener('keydown', onKey);
    closeEntropy = () => {
      done = cancelled = true;
      pad.removeEventListener('pointermove', onMove);
      document.removeEventListener('keydown', onKey);
      $('#entropy').hidden = true;
      $('#entropySeed').textContent = '';
      $('#superRandom').setAttribute('aria-expanded', 'false');
      closeEntropy = entropyRefresh = () => {};
    };
    entropyRefresh = info;
    info();
  }

  function renderMatrix() {
    stopShuffle();
    const table = $('#matrix');
    const editable = state.mode === 'admin';
    table.classList.toggle('editable', editable);
    const now = today();
    const pos = positions();
    const incoming = new Map(); // "day-voucher" of the receiving cell → handover
    for (const h of plan.handovers) incoming.set(`${pos.get(h.to).day}-${h.voucher}`, h);
    const pinned = new Set(state.pins.map((p) => p.label));
    const days = plan.days.slice();
    if (editable && !(plan.end.day && days.length >= plan.end.day)) days.push({ day: days.length + 1, date: '', label: t('addDay'), extra: true }); // no day after the end

    const head = el('tr', {}, el('th', {}, 'Voucher'), days.map((d) => el('th', { class: d.extra ? 'extra' : d.date === now ? 'today' : null }, dayName(d), d.date && el('small', {}, d.date))));
    const rows = [];
    for (let v = 1; v <= plan.vouchers; v++) {
      const tds = days.map((d) => {
        const label = d.extra ? '' : (plan.grid[d.day - 1][v - 1] || '');
        const na = plan.voucherFrom[v - 1] > d.day; // voucher does not exist yet (also in the "+ Tag" column)
        if (na) return el('td', { class: 'cell na', title: t('notYet') });
        const cell = { label, day: d.day, voucher: v };
        const h = incoming.get(`${d.day}-${v}`);
        const cls = ['cell'];
        if (!label && !d.extra) cls.push('empty');
        if (d.extra) cls.push('extra');
        if (d.date === now) cls.push('today');
        if (h) cls.push(h.forbidden ? 'forbidden' : h.known ? 'known' : 'stranger');
        if (label && pinned.has(label)) cls.push('pinned');
        if (label && statusOf(label) !== 'open') cls.push('st-' + statusOf(label));
        if (selected && label && selected.label === label) cls.push('selected');
        const title = label ? `${h ? `${h.from} → ${h.to}` : t('fromOrga')} · ${statusLabel(statusOf(label))}` : d.extra ? '' : t('freeCell');
        const td = el('td', { class: cls.join(' '), draggable: editable && label ? 'true' : null, tabindex: editable ? '0' : null, title }, label || (d.extra ? '·' : t('free')));
        if (editable) {
          if (d.extra) cell.extra = true;
          td.addEventListener('click', () => onCellPick(cell, td));
          td.addEventListener('keydown', (e) => {
            if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onCellPick(cell, td); } else if (e.key === 'Escape' && selected) { selected = null; renderMatrix(); }
          });
          if (label) {
            td.addEventListener('dragstart', (e) => { selected = cell; e.dataTransfer.setData('text/plain', label); e.dataTransfer.effectAllowed = 'move'; });
            td.addEventListener('dragend', () => { if (selected) { selected = null; renderMatrix(); } });
            if (pinned.has(label)) {
              td.append(el('button', { class: 'unpin', type: 'button', title: t('unpin'), onclick: (e) => { e.stopPropagation(); unpin(label); } }, '×'));
            }
          }
          td.addEventListener('dragover', (e) => { e.preventDefault(); td.classList.add('dragover'); });
          td.addEventListener('dragleave', () => td.classList.remove('dragover'));
          td.addEventListener('drop', (e) => {
            e.preventDefault();
            const label = e.dataTransfer.getData('text/plain');
            if (label) moveSlot(label, cell);
          });
        }
        return td;
      });
      rows.push(el('tr', {}, el('th', {}, v), tds));
    }
    table.replaceChildren(el('thead', {}, head), el('tbody', {}, rows));
  }

  function renderHandovers() {
    const box = $('#handovers');
    box.replaceChildren();
    const pos = positions();
    let day = 0, list = null;
    for (const h of plan.handovers) {
      if (h.day !== day) {
        day = h.day;
        box.append(el('h3', {}, t('evening', dayName(plan.days[day - 1]), h.date, plan.handover)));
        list = el('ul');
        box.append(list);
      }
      const mark = h.forbidden ? el('span', { class: 'mark forbidden-text', title: t('forbidden') }, '!') : h.known ? el('span', { class: 'mark known-text', title: t('known') }, '✓') : el('span', { class: 'mark', title: t('stranger') }, '·');
      const toDay = pos.get(h.to).day;
      list.append(el('li', {}, mark, ` Voucher ${h.voucher}: ${h.from} → ${h.to}`, h.toEmail && ` <${h.toEmail}>`, toDay !== h.day + 1 && t('buysOn', dayName(plan.days[toDay - 1]))));
    }
    if (!plan.handovers.length) box.append(el('p', { class: 'hint' }, t('oneDay')));
  }

  function mailFor(sheet) {
    const next = sheet.to && plan.sheets.find((s) => s.label === sheet.to);
    if (!next || !next.email) return null;
    const body = t('mail', next.person, next.label, next.date, sheet.voucher, plan.event, plan.handover, next.to, next.toEmail, sheet.person).join('\n');
    return mailto(next.email, t('mailSubject', sheet.voucher, plan.event), body);
  }

  function renderSheets() {
    const table = $('#sheets');
    const admin = state.mode === 'admin';
    const labels = new Set(plan.sheets.map((s) => s.label));
    for (const l of picked) if (!labels.has(l)) picked.delete(l);
    // The first holder of a chain gets the voucher from the organisers; a
    // person whose handle happens to be "organisers" keeps it.
    const firstDay = new Map();
    for (const s of plan.sheets) if (!(firstDay.get(s.voucher) <= s.day)) firstDay.set(s.voucher, s.day);
    const fromName = (s) => (firstDay.get(s.voucher) === s.day ? t('orga') : s.from);
    $('#sheets-count').textContent = boughtText();
    const pickAll = admin && el('input', { type: 'checkbox', id: 'pickAll', title: t('pickAll'), onchange: () => {
      const visible = [...table.querySelectorAll('tbody tr:not(.hidden)')].map((tr) => tr.dataset.label);
      if (visible.every((l) => picked.has(l))) for (const l of visible) picked.delete(l); else for (const l of visible) picked.add(l);
      renderSheets();
      $('#pickAll').focus();
    } });
    const head = el('tr', {}, admin && el('th', {}, pickAll), t('heads').map((h) => el('th', {}, h)));
    const rows = plan.sheets.map((s) => {
      const mail = mailFor(s);
      const st = statusOf(s.label);
      const badge = admin
        ? el('button', { type: 'button', class: `badge st-${st}`, title: t('nextStatus'), onclick: () => cycleStatus(s.label) }, statusLabel(st))
        : el('span', { class: `badge st-${st}` }, statusLabel(st));
      const report = !admin && reportMail(s);
      return el('tr', { class: `st-${st}`, dataset: { label: s.label, who: `${s.label} ${s.person} ${s.email || ''}`.toLowerCase(), person: s.person.toLowerCase() } },
        admin && el('td', {}, el('input', { type: 'checkbox', class: 'pick', 'aria-label': t('pickRow', s.label), checked: picked.has(s.label), onchange: (e) => { if (e.target.checked) picked.add(s.label); else picked.delete(s.label); renderBulk(); } })),
        el('td', {}, s.label, s.pinned ? ' 📌' : ''),
        el('td', {}, s.person, s.email && el('small', {}, el('br'), s.email)),
        el('td', {}, dayName(plan.days[s.day - 1]), el('small', {}, el('br'), s.date)),
        el('td', {}, s.voucher),
        el('td', {}, fromName(s), s.fromEmail && el('small', {}, el('br'), s.fromEmail)),
        el('td', {}, s.to || t('chainEnds'), s.toEmail && el('small', {}, el('br'), s.toEmail)),
        el('td', {}, badge, report && [' ', el('a', { href: report, title: t('reportTitle') }, t('report'))]),
        el('td', {}, mail && el('a', { href: mail, title: t('mailTitle') }, 'Mail'),
          admin && [' ', el('button', { type: 'button', class: 'small', title: t('linkTitle', s.person), onclick: () => copy(linkFor('view', { pins: plan.pins, me: s.person }), t('linkFor', s.person)) }, 'Link')]),
      );
    });
    table.replaceChildren(el('thead', {}, head), el('tbody', {}, rows));
    filterSheets();
    renderBulk();
  }
  function renderBulk() {
    const bar = $('#bulk');
    bar.hidden = state.mode !== 'admin';
    const visible = [...$('#sheets').querySelectorAll('tbody tr:not(.hidden)')];
    const hidden = picked.size - visible.filter((tr) => picked.has(tr.dataset.label)).length;
    $('#pickCount').textContent = picked.size
      ? t('picked', picked.size, hidden)
      : t('pickHint');
    for (const b of bar.querySelectorAll('button')) b.disabled = !picked.size;
    const all = $('#pickAll');
    if (all) all.checked = visible.length > 0 && visible.every((tr) => picked.has(tr.dataset.label));
  }
  function filterSheets() {
    const q = $('#who').value.trim().toLowerCase();
    const exact = !!state.me && q === state.me.toLowerCase(); // personal link: this person only, not every substring match
    for (const tr of $('#sheets').querySelectorAll('tbody tr')) {
      const hit = q && (exact ? tr.dataset.person === q : tr.dataset.who.includes(q));
      tr.classList.toggle('me', !!hit);
      tr.classList.toggle('hidden', !!q && !hit);
    }
  }

  // ---------- modes
  function applyMode() {
    const m = state.mode;
    $('#modeLabel').textContent = t('mode')[m] || m;
    $('#editor').hidden = m !== 'admin';
    $('#sharebar').hidden = m !== 'admin';
    $('#collect').hidden = m !== 'collect';
    $('#result').hidden = true;
    $('#board').hidden = true;
    if (m === 'collect') {
      $('#c-event').textContent = state.event.name || t('theEvent');
      $('#c-plus').max = String(state.event.plusMax || 2);
      return;
    }
    eventToForm();
    peopleToRoster(state.event.people);
    if (state.me) $('#who').value = state.me;
    if (state.event.people && state.event.people.length) computePlan({ fromForm: false });
  }

  function wireAdmin() {
    $('#compute').addEventListener('click', () => { selected = null; computePlan({ shuffle: !drawShown }); }); // the first plan shows its draw
    $('#settings').addEventListener('submit', (e) => { e.preventDefault(); computePlan({ shuffle: !drawShown }); });
    $('#roll').addEventListener('click', roll);
    $('#superRandom').addEventListener('click', () => { if ($('#entropy').hidden) openEntropy(); else closeEntropy(); });
    $('#entropyCancel').addEventListener('click', () => closeEntropy());
    $('#loadFile').addEventListener('change', async (e) => {
      const f = e.target.files[0];
      if (!f) return;
      try {
        loadEvent(JSON.parse(await f.text()));
      } catch (err) {
        showMessages([() => t('fileUnreadable', err)]);
      }
      e.target.value = '';
    });
    $('#saveFile').addEventListener('click', () => {
      const f = readForm();
      const r = rosterToPeople();
      if (!r) return;
      download(`${f.event.name || 'event'}.json`, JSON.stringify({ ...f.event, people: r.people }, null, 2) + '\n', 'application/json');
    });
    $('#addInbox').addEventListener('click', () => {
      const near = $('#addInbox').parentElement;
      if (!$('#inbox').value.trim()) return;
      let current = [];
      if ($('#roster').value.trim()) {
        const cur = rosterToPeople(near);
        if (!cur) return;
        current = cur.people;
      }
      const r = bridge.parseSignup($('#inbox').value, JSON.stringify(current.map((p) => p.id)));
      if (!r.ok) { showMessages(r.errors, r.warnings, near); return; }
      const have = new Set(current.map((p) => p.id.toLowerCase()));
      const skipped = [];
      let added = 0;
      for (const p of JSON.parse(r.peopleJson)) {
        if (have.has(p.id.toLowerCase())) { skipped.push(p.id); continue; }
        have.add(p.id.toLowerCase());
        current.push(p);
        added++;
      }
      peopleToRoster(current);
      $('#inbox').value = '';
      showMessages([], r.warnings.concat(skipped.length ? [() => t('skipped', skipped.join(', '))] : []), near);
      toast(added ? t('added', added) : t('nothingAdded'));
    });
    $('#unpinAll').addEventListener('click', () => { mutate(() => { state.pins = []; }); selected = null; computePlan({ fromForm: false }); });
    $('#freezeToday').addEventListener('click', () => {
      if (!plan) return;
      const now = today();
      const gone = plan.sheets.filter((s) => s.date <= now);
      if (!gone.length) { toast(t('noneGone')); return; }
      mutate(() => { for (const s of gone) setPin(s.label, s.day, s.voucher); });
      selected = null;
      computePlan({ fromForm: false });
      toast(t('frozen', gone.length));
    });
    $('#undo').addEventListener('click', undo);
    $('#redo').addEventListener('click', redo);
    for (const b of $('#bulk').querySelectorAll('button[data-st]')) b.addEventListener('click', () => setStatus([...picked], b.dataset.st));
    $('#pickNone').addEventListener('click', () => { picked.clear(); renderSheets(); });
    document.addEventListener('keydown', (e) => { // Ctrl/Cmd+Z, Ctrl/Cmd+Shift+Z or Ctrl/Cmd+Y outside of inputs
      if (state.mode !== 'admin' || !(e.ctrlKey || e.metaKey) || e.altKey || (e.target instanceof Element && e.target.matches('input:not([type=checkbox]), textarea, select'))) return;
      const k = e.key.toLowerCase();
      if (k === 'z') { e.preventDefault(); if (e.shiftKey) redo(); else undo(); } else if (k === 'y') { e.preventDefault(); redo(); }
    });

    $('#copyAdmin').addEventListener('click', () => { if (computePlan()) copy(linkFor('admin', {}), t('adminLink', plan.fingerprint)); });
    $('#copyView').addEventListener('click', () => { if (computePlan()) copy(linkFor('view', { pins: plan.pins }), t('viewLink', plan.fingerprint)); });
    $('#copyBoard').addEventListener('click', () => { if (computePlan()) copy(linkFor('board', { pins: plan.pins }), t('boardLink', plan.fingerprint)); });
    $('#copyCollect').addEventListener('click', () => {
      const f = readForm();
      if (!f.to) { showMessages([() => t('needTo')]); return; }
      if (!f.event.name) { showMessages([() => t('needName')]); return; }
      copy(linkFor('collect', { event: f.event, collect: { to: f.to } }), t('collectLink'));
    });
  }

  function wireResult() {
    $('#copyMd').addEventListener('click', () => plan && copy(plan.markdown, 'Markdown'));
    $('#dlCsv').addEventListener('click', () => plan && download(`${plan.event}-plan.csv`, plan.csv, 'text/csv'));
    $('#dlJson').addEventListener('click', () => plan && download(`${plan.event}-plan.json`, plan.exportJson, 'application/json'));
    $('#dlSvg').addEventListener('click', () => plan && download(`${plan.event}-plan.svg`, plan.svg, 'image/svg+xml'));
    $('#who').addEventListener('input', () => { filterSheets(); renderBulk(); });
  }

  function wireCollect() {
    $('#collectForm').addEventListener('submit', (e) => {
      e.preventDefault();
      const clean = (s) => s.replace(/[;\n\r\t]/g, ' ').trim();
      const handle = clean($('#c-handle').value);
      const line = [
        handle, clean($('#c-email').value), $('#c-tier').value, String(Number($('#c-plus').value) || 0),
        clean($('#c-knows').value), clean($('#c-note').value),
      ].join('; ');
      $('#c-line').value = line;
      $('#collectOut').hidden = false;
      const to = (state.collect && state.collect.to) || '';
      const a = $('#c-mail');
      a.hidden = !to;
      if (to) a.href = mailto(to, `vouch0rzii ${state.event.name || ''}: ${handle}`.trim(), line + '\n');
    });
    $('#c-copy').addEventListener('click', () => copy($('#c-line').value, t('line')));
  }

  // ---------- language switch
  function browserLang() {
    for (const l of navigator.languages || [navigator.language]) {
      const p = String(l).slice(0, 2).toLowerCase();
      if (p === 'de' || p === 'en') return p;
    }
    return 'en';
  }
  // setLang shows one language: CSS hides the other's static text, the
  // data-en-* attributes swap in (the German originals are kept beside
  // them), and whatever this file rendered is rendered again.
  function setLang(l) {
    lang = l;
    document.documentElement.lang = l;
    document.title = t('title');
    for (const n of document.querySelectorAll('[data-en-title], [data-en-placeholder], [data-en-text]')) {
      for (const [prop, key] of [['title', 'Title'], ['placeholder', 'Placeholder'], ['textContent', 'Text']]) {
        if (n.dataset[`en${key}`] === undefined) continue;
        if (n.dataset[`de${key}`] === undefined) n.dataset[`de${key}`] = n[prop];
        n[prop] = n.dataset[`${l}${key}`];
      }
    }
    $('#lang').textContent = l === 'de' ? 'EN' : 'DE';
    if (bridge) $('#modeLabel').textContent = t('mode')[state.mode] || state.mode; // the mode is known once the link is read
    if (state.mode === 'collect') $('#c-event').textContent = state.event.name || t('theEvent');
    if (plan) renderResult();
    if (shown.flat().length) showMessages(...shown, $('#messages').previousElementSibling, false);
  }

  async function main() {
    setLang(browserLang());
    $('#lang').addEventListener('click', () => setLang(lang === 'de' ? 'en' : 'de'));
    try {
      bridge = await loadWasm();
    } catch (e) {
      $('#loading').replaceChildren(el('span', { lang: 'de' }, TEXT.de.loadFailed(e)), el('span', { lang: 'en' }, TEXT.en.loadFailed(e)));
      return;
    }
    $('#loading').hidden = true;
    $('#version').textContent = 'Version ' + bridge.version;
    wireAdmin();
    wireResult();
    wireCollect();
    const boot = () => {
      closeEntropy(); // a new link starts without an open super random
      noteKey = '';
      const st = readHash();
      if (st) {
        state = {
          ...emptyState(), mode: st.mode || 'admin', event: { ...DEFAULT_EVENT, ...(st.event || {}) },
          pins: st.pins || [], collect: st.collect || { to: '' }, status: st.status || {}, statusAt: st.statusAt || '', me: st.me || '',
        };
      }
      selected = null;
      plan = null;
      picked.clear();
      past.length = future.length = 0; // a new link is a new baseline
      applyMode();
    };
    const opensDemo = demo;
    boot();
    if (opensDemo && plan && state.mode === 'admin') { // the demo opens on its plan, shuffling
      $('#result').scrollIntoView({ block: 'start' });
      shuffleMatrix();
    }
    window.addEventListener('hashchange', () => { demo = false; boot(); }); // another link is not the demo
  }

  main();
})();
