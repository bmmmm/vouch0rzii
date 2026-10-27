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

  const DEFAULT_EVENT = { name: '', vouchers: 7, start: '', weekends: false, plusMax: 2, seed: '', handover: '23:00', people: [] };
  const MODE_LABEL = { admin: 'Admin', view: 'Ansicht', board: 'Board', collect: 'Anmeldung' };
  const COLLECT_HEADER = 'Name;E-Mail;Status;+N;Kreditkarte;Kennt;Notiz';
  const STATUS = ['open', 'received', 'bought', 'passed'];
  const STATUS_LABEL = { open: 'offen', received: 'Voucher da', bought: 'Ticket gekauft', passed: 'weitergegeben' };

  const emptyState = () => ({ v: 1, mode: 'admin', event: { ...DEFAULT_EVENT }, pins: [], collect: { to: '' }, status: {}, statusAt: '', me: '' });
  let bridge = null;
  let state = emptyState();
  let plan = null;      // last export plus exportJson/markdown/csv/svg
  let selected = null;  // cell chosen as move source: {label, day, voucher}

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
  function showMessages(errors = [], warnings = []) {
    const box = $('#messages');
    box.replaceChildren();
    if (errors.length) box.append(el('div', { class: 'error' }, el('strong', {}, 'Das geht so nicht:'), el('ul', {}, errors.map((e) => el('li', {}, e)))));
    if (warnings.length) box.append(el('div', { class: 'warning' }, el('strong', {}, 'Hinweise:'), el('ul', {}, warnings.map((w) => el('li', {}, w)))));
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
      toast(`${what} kopiert`);
    } catch {
      toast('Kopieren fehlgeschlagen');
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
    return isNaN(d) ? iso : d.toLocaleString('de-DE', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' });
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
      showMessages(['Der Link konnte nicht gelesen werden: ' + r.errors.join('; ')]);
      return null;
    }
    return JSON.parse(r.stateJson);
  }
  function writeHash() {
    if (state.mode !== 'admin') return;
    try { history.replaceState(null, '', '#s=' + encodeState(state)); } catch (e) { showMessages([String(e)]); }
  }
  function linkFor(mode, extra) {
    return location.href.split('#')[0] + '#s=' + encodeState({ ...state, mode, ...extra });
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
  // rosterToPeople reads the textarea; null means errors were shown.
  function rosterToPeople() {
    const r = bridge.parseRoster($('#roster').value);
    if (!r.ok) {
      showMessages(r.errors, r.warnings);
      return null;
    }
    return { people: JSON.parse(r.peopleJson), warnings: r.warnings };
  }
  function peopleToRoster(people) {
    const r = bridge.formatRoster(JSON.stringify(people || []));
    $('#roster').value = r.ok ? r.tsv : '';
  }

  // ---------- planning
  function computePlan({ fromForm } = { fromForm: true }) {
    let warnings = [];
    if (fromForm) {
      const f = readForm();
      const r = rosterToPeople();
      if (!r) return false;
      state.event = { ...f.event, people: r.people };
      state.collect = { to: f.to };
      warnings = r.warnings;
    }
    const r = bridge.plan(JSON.stringify(state.event), JSON.stringify(state.pins));
    if (!r.ok) {
      showMessages(r.errors, warnings.concat(r.warnings));
      plan = null;
      renderResult();
      return false;
    }
    plan = { ...JSON.parse(r.exportJson), exportJson: r.exportJson, markdown: r.markdown, csv: r.csv, svg: r.svg };
    showMessages([], warnings.concat(plan.warnings));
    renderResult();
    writeHash();
    return true;
  }
  function loadEvent(ev) {
    state = { ...emptyState(), event: { ...DEFAULT_EVENT, ...ev } };
    selected = null;
    eventToForm();
    peopleToRoster(state.event.people);
    computePlan({ fromForm: false });
  }

  // ---------- pins (manual moves)
  function setPin(label, day, voucher) {
    const i = state.pins.findIndex((p) => p.label === label);
    const pin = { label, day, voucher };
    if (i >= 0) state.pins[i] = pin; else state.pins.push(pin);
  }
  function unpin(label) {
    state.pins = state.pins.filter((p) => p.label !== label);
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
    if (target.label && target.label !== label) setPin(target.label, src.day, src.voucher);
    setPin(label, target.day, target.voucher);
    selected = null;
    computePlan({ fromForm: false });
  }
  function onCellPick(cell) {
    if (!selected) {
      if (!cell.label) return;
      selected = cell;
      renderMatrix();
      return;
    }
    if (selected.label === cell.label) { selected = null; renderMatrix(); return; }
    moveSlot(selected.label, cell);
  }

  // ---------- live status (curated by admins, carried in the link)
  const statusOf = (label) => (STATUS.includes(state.status[label]) ? state.status[label] : 'open');
  function cycleStatus(label) {
    const next = STATUS[(STATUS.indexOf(statusOf(label)) + 1) % STATUS.length];
    if (next === 'open') delete state.status[label]; else state.status[label] = next;
    state.statusAt = new Date().toISOString();
    writeHash();
    renderResult();
  }
  function reportMail(sheet) {
    const to = state.collect && state.collect.to;
    if (!to) return null;
    const next = STATUS[Math.min(STATUS.indexOf(statusOf(sheet.label)) + 1, STATUS.length - 1)];
    const body = `Slot: ${sheet.label}\nVoucher: ${sheet.voucher}\nNeuer Status: ${STATUS_LABEL[next]}\nPlan: ${plan.fingerprint}\n`;
    return mailto(to, `vouch0rzii ${plan.event}: ${sheet.label} ${STATUS_LABEL[next]}`, body);
  }
  // voucherState reads the curated status of one chain: where the voucher is
  // (kind: done, moving, holding, orga, none) and whether it is overdue.
  // Overdue marks only once the sale is being tracked (track).
  function voucherState(v, now, track) {
    const chain = plan.sheets.filter((s) => s.voucher === v); // in day order
    const st = chain.map((s) => statusOf(s.label));
    const holder = Math.max(st.lastIndexOf('received'), st.lastIndexOf('bought'));
    const passed = st.lastIndexOf('passed');
    const day = (s) => plan.days[s.day - 1].label;
    if (passed > holder) {
      const next = chain[passed + 1];
      if (!next) return { v, kind: 'done', text: 'Kette abgeschlossen' };
      return { v, kind: 'moving', text: `unterwegs zu ${next.label}, ${chain[passed].label} hat weitergegeben, Kauf am ${day(next)}`, late: track && next.date < now };
    }
    if (holder >= 0) {
      const h = chain[holder];
      const next = chain.slice(holder + 1).find((s) => statusOf(s.label) === 'open');
      return {
        v, kind: 'holding',
        text: `bei ${h.label} (${STATUS_LABEL[st[holder]]})` + (next ? `, als Nächstes ${next.label} am ${day(next)}` : ', letzte Station'),
        late: track && h.date < now && !(st[holder] === 'bought' && !next), // a bought last station is done, not late
      };
    }
    if (chain.length) return { v, kind: 'orga', text: `noch bei der Orga, Start ${chain[0].label} am ${day(chain[0])}`, late: track && chain[0].date < now };
    return { v, kind: 'none', text: 'ohne Slots' };
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
    return n ? `${n} Slot(s) mit Status, Stand ${fmtAt(state.statusAt)}.` : 'Noch kein Status gesetzt. Überfällig wird erst markiert, sobald der erste Status gesetzt ist.';
  }
  const publicBox = (box) => {
    box.hidden = !plan.public.length;
    box.replaceChildren(el('strong', {}, 'Freier Verkauf: '), `${plan.public.length} Ticket(s) passen nicht mehr in die Voucher-Phase und müssen regulär gekauft werden: `, el('b', {}, plan.public.join(', ')));
  };
  function renderLive() {
    const now = today(), track = tracking();
    const items = [];
    for (let v = 1; v <= plan.vouchers; v++) {
      const s = voucherState(v, now, track);
      items.push(el('li', { class: s.late ? 'live overdue' : 'live' }, `Voucher ${v}: ${s.text}`, s.late ? ' — überfällig' : ''));
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
    $('#b-day').textContent = now < first.date ? `Start am ${first.label}`
      : ended ? (plan.end.day ? 'Voucher-Phase beendet' : `Letzter Plantag war ${last.label}`)
        : `Tag ${planDay(now)} von ${total}` + (plan.end.day ? ` · Ende ${plan.end.label}` : '');
    const states = [];
    for (let v = 1; v <= plan.vouchers; v++) states.push(voucherState(v, now, track));
    const bought = sheets.filter((s) => ['bought', 'passed'].includes(statusOf(s.label))).length;
    const bar = $('#b-bar');
    bar.max = sheets.length || 1;
    bar.value = bought;
    $('#b-progress').textContent = `${bought} von ${sheets.length} Tickets gekauft`;
    $('#b-kpis').replaceChildren(...[
      ['Voucher unterwegs', states.filter((s) => s.kind === 'moving').length],
      ['Ketten abgeschlossen', states.filter((s) => s.kind === 'done').length],
      ['überfällig', states.filter((s) => s.late).length],
      ['Slots frei', plan.stats.free],
    ].map(([k, n]) => el('div', { class: 'kpi' }, el('b', {}, n), el('span', {}, k))));
    $('#b-vouchers').replaceChildren(...states.map((s) => el('li', { class: `tile ${s.kind}${s.late ? ' late' : ''}` }, el('b', {}, `Voucher ${s.v}`), s.text, s.late && el('small', { class: 'late' }, 'überfällig'))));
    const nextDay = plan.days.find((d) => d.date > now);
    const who = (list) => list.length ? list.map((s) => el('li', {}, el('span', { class: `badge st-${statusOf(s.label)}` }, STATUS_LABEL[statusOf(s.label)]), ` ${s.label} · Voucher ${s.voucher}`)) : [el('li', { class: 'hint' }, 'niemand')];
    $('#b-today').replaceChildren(...who(sheets.filter((s) => s.date === now)));
    $('#b-nextLabel').textContent = nextDay ? `Als Nächstes, ${nextDay.label}` : 'Als Nächstes';
    $('#b-next').replaceChildren(...who(nextDay ? sheets.filter((s) => s.day === nextDay.day) : []));
    publicBox($('#b-public'));
    $('#b-at').textContent = `${statusStand()} Kennung ${plan.fingerprint}.`;
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
      `${plan.vouchers} Vouchers · ${s.days} Tage · ${s.slots} Slots · ${s.handovers} Übergaben, davon ${s.known} unter Buddies, ${s.unknown} neue Verbindungen` +
      (s.forbidden ? `, ${s.forbidden} direkt an eigene Begleitung` : '') + (s.pinned ? ` · ${s.pinned} gepinnt` : '') + ` · Übergabe ${plan.handover}`;
    $('#r-phase').textContent = plan.end.day
      ? `Voucher-Phase bis ${plan.end.label} (${plan.end.day} Tage) · ${s.free} Slots frei`
      : `Kein Ende der Voucher-Phase gesetzt · ${s.free} Slots frei im Plan`;
    publicBox($('#r-public'));
    const toBoard = $('#toBoard'); // admins copy the board link instead; switching would drop unsaved edits
    toBoard.hidden = state.mode === 'admin';
    if (!toBoard.hidden) toBoard.href = linkFor('board', { pins: plan.pins });
    $('#r-pinhelp').hidden = state.mode !== 'admin';
    renderLive();
    renderMatrix();
    $('#diagram').innerHTML = plan.svg; // our own SVG, labels escaped by the Go side
    renderHandovers();
    renderSheets();
  }

  function renderMatrix() {
    const table = $('#matrix');
    const editable = state.mode === 'admin';
    table.classList.toggle('editable', editable);
    const now = today();
    const pos = positions();
    const incoming = new Map(); // "day-voucher" of the receiving cell → handover
    for (const h of plan.handovers) incoming.set(`${pos.get(h.to).day}-${h.voucher}`, h);
    const pinned = new Set(state.pins.map((p) => p.label));
    const days = plan.days.slice();
    if (editable && !(plan.end.day && days.length >= plan.end.day)) days.push({ day: days.length + 1, date: '', label: '+ Tag', extra: true }); // no day after the end

    const head = el('tr', {}, el('th', {}, 'Voucher'), days.map((d) => el('th', { class: d.extra ? 'extra' : d.date === now ? 'today' : null }, d.label, d.date && el('small', {}, d.date))));
    const rows = [];
    for (let v = 1; v <= plan.vouchers; v++) {
      const tds = days.map((d) => {
        const label = d.extra ? '' : (plan.grid[d.day - 1][v - 1] || '');
        const na = plan.voucherFrom[v - 1] > d.day; // voucher does not exist yet (also in the "+ Tag" column)
        if (na) return el('td', { class: 'cell na', title: 'Voucher noch nicht verfügbar' });
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
        const title = label ? `${h ? `${h.from} → ${h.to}` : 'bekommt den Voucher von der Orga'} · ${STATUS_LABEL[statusOf(label)]}` : d.extra ? '' : 'Voucher verfügbar, noch niemand eingeplant';
        const td = el('td', { class: cls.join(' '), draggable: editable && label ? 'true' : null, tabindex: editable ? '0' : null, title }, label || (d.extra ? '·' : 'frei'));
        if (editable) {
          td.addEventListener('click', () => onCellPick(cell));
          td.addEventListener('keydown', (e) => {
            if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onCellPick(cell); } else if (e.key === 'Escape' && selected) { selected = null; renderMatrix(); }
          });
          if (label) {
            td.addEventListener('dragstart', (e) => { selected = cell; e.dataTransfer.setData('text/plain', label); e.dataTransfer.effectAllowed = 'move'; });
            td.addEventListener('dragend', () => { if (selected) { selected = null; renderMatrix(); } });
            if (pinned.has(label)) {
              td.append(el('button', { class: 'unpin', type: 'button', title: 'Pin lösen', onclick: (e) => { e.stopPropagation(); unpin(label); } }, '×'));
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
        box.append(el('h3', {}, `${plan.days[day - 1].label} (${h.date}), ${plan.handover} Uhr`));
        list = el('ul');
        box.append(list);
      }
      const mark = h.forbidden ? el('span', { class: 'mark forbidden-text', title: 'direkt an eigene Begleitung' }, '!') : h.known ? el('span', { class: 'mark known-text', title: 'Buddies, kennen sich schon' }, '✓') : el('span', { class: 'mark', title: 'neue Verbindung' }, '·');
      const toDay = pos.get(h.to).day;
      list.append(el('li', {}, mark, ` Voucher ${h.voucher}: ${h.from} → ${h.to}`, h.toEmail && ` <${h.toEmail}>`, toDay !== h.day + 1 && `, kauft am ${plan.days[toDay - 1].label}`));
    }
    if (!plan.handovers.length) box.append(el('p', { class: 'hint' }, 'Nur ein Tag, keine Übergaben.'));
  }

  function mailFor(sheet) {
    const next = sheet.to && plan.sheets.find((s) => s.label === sheet.to);
    if (!next || !next.email) return null;
    const body = [
      `Hallo ${next.person}${next.label !== next.person ? ` (für ${next.label})` : ''},`,
      '',
      `am ${next.date} bekommst du von mir Voucher ${sheet.voucher} für ${plan.event}:`,
      '',
      '<CODE HIER EINFÜGEN>',
      '',
      `Kauf bitte an dem Tag dein Ticket und gib den neuen Voucher um ${plan.handover} Uhr weiter` +
        (next.to ? ` an ${next.to}${next.toEmail ? ` (${next.toEmail})` : ''}.` : '. Du bist die letzte Station dieser Kette.'),
      '',
      `Viele Grüße, ${sheet.person}`,
    ].join('\n');
    return mailto(next.email, `Voucher ${sheet.voucher} für ${plan.event}`, body);
  }

  function renderSheets() {
    const table = $('#sheets');
    const admin = state.mode === 'admin';
    const head = el('tr', {}, ['Slot', 'Kontakt', 'Tag', 'Voucher', 'bekommt von', 'gibt weiter an', 'Status', ''].map((h) => el('th', {}, h)));
    const rows = plan.sheets.map((s) => {
      const mail = mailFor(s);
      const st = statusOf(s.label);
      const badge = admin
        ? el('button', { type: 'button', class: `badge st-${st}`, title: 'Klick: nächster Status', onclick: () => cycleStatus(s.label) }, STATUS_LABEL[st])
        : el('span', { class: `badge st-${st}` }, STATUS_LABEL[st]);
      const report = !admin && reportMail(s);
      return el('tr', { dataset: { who: `${s.label} ${s.person} ${s.email || ''}`.toLowerCase(), person: s.person.toLowerCase() } },
        el('td', {}, s.label, s.pinned ? ' 📌' : ''),
        el('td', {}, s.person, s.email && el('small', {}, el('br'), s.email), s.card ? el('small', { title: 'Kreditkarte für den Notfall' }, ' 💳') : ''),
        el('td', {}, plan.days[s.day - 1].label, el('small', {}, el('br'), s.date)),
        el('td', {}, s.voucher),
        el('td', {}, s.from, s.fromEmail && el('small', {}, el('br'), s.fromEmail)),
        el('td', {}, s.to || '— Kette endet', s.toEmail && el('small', {}, el('br'), s.toEmail)),
        el('td', {}, badge, report && [' ', el('a', { href: report, title: 'Statusmeldung an die Orga vorbereiten' }, 'melden')]),
        el('td', {}, mail && el('a', { href: mail, title: 'Mail an die nächste Person vorbereiten' }, 'Mail'),
          admin && [' ', el('button', { type: 'button', class: 'small', title: `Viewer-Link, der die Zeilen von ${s.person} hervorhebt`, onclick: () => copy(linkFor('view', { pins: plan.pins, me: s.person }), `Link für ${s.person}`) }, 'Link')]),
      );
    });
    table.replaceChildren(el('thead', {}, head), el('tbody', {}, rows));
    filterSheets();
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
    $('#modeLabel').textContent = MODE_LABEL[m] || m;
    $('#editor').hidden = m !== 'admin';
    $('#sharebar').hidden = m !== 'admin';
    $('#collect').hidden = m !== 'collect';
    $('#result').hidden = true;
    $('#board').hidden = true;
    if (m === 'collect') {
      $('#c-event').textContent = state.event.name || 'das Event';
      $('#c-plus').max = String(state.event.plusMax || 2);
      return;
    }
    eventToForm();
    peopleToRoster(state.event.people);
    if (state.me) $('#who').value = state.me;
    if (state.event.people && state.event.people.length) computePlan({ fromForm: false });
  }

  function wireAdmin() {
    $('#compute').addEventListener('click', () => { selected = null; computePlan(); });
    $('#settings').addEventListener('submit', (e) => { e.preventDefault(); computePlan(); });
    $('#loadFile').addEventListener('change', async (e) => {
      const f = e.target.files[0];
      if (!f) return;
      try {
        loadEvent(JSON.parse(await f.text()));
      } catch (err) {
        showMessages([`Datei nicht lesbar: ${err}`]);
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
      const lines = $('#inbox').value.split('\n').map((l) => l.trim()).filter(Boolean);
      if (!lines.length) return;
      const r = bridge.parseRoster([COLLECT_HEADER, ...lines].join('\n'));
      if (!r.ok) { showMessages(r.errors, r.warnings); return; }
      let current = [];
      if ($('#roster').value.trim()) {
        const cur = rosterToPeople();
        if (!cur) return;
        current = cur.people;
      }
      const have = new Set(current.map((p) => p.id.toLowerCase()));
      const skipped = [];
      for (const p of JSON.parse(r.peopleJson)) {
        if (have.has(p.id.toLowerCase())) { skipped.push(p.id); continue; }
        have.add(p.id.toLowerCase());
        current.push(p);
      }
      peopleToRoster(current);
      $('#inbox').value = '';
      showMessages([], skipped.length ? [`Schon vorhanden, übersprungen: ${skipped.join(', ')}`] : []);
      toast(`${lines.length - skipped.length} Person(en) hinzugefügt`);
    });
    $('#unpinAll').addEventListener('click', () => { state.pins = []; selected = null; computePlan({ fromForm: false }); });
    $('#freezeToday').addEventListener('click', () => {
      if (!plan) return;
      const now = today();
      const past = plan.sheets.filter((s) => s.date <= now);
      if (!past.length) { toast('Noch kein Tag vergangen, nichts festzunageln'); return; }
      for (const s of past) setPin(s.label, s.day, s.voucher);
      selected = null;
      computePlan({ fromForm: false });
      toast(`${past.length} Slot(s) bis heute festgenagelt`);
    });

    $('#copyAdmin').addEventListener('click', () => { if (computePlan()) copy(linkFor('admin', {}), `Admin-Link, Stand ${plan.fingerprint},`); });
    $('#copyView').addEventListener('click', () => { if (computePlan()) copy(linkFor('view', { pins: plan.pins }), `Viewer-Link, Stand ${plan.fingerprint},`); });
    $('#copyBoard').addEventListener('click', () => { if (computePlan()) copy(linkFor('board', { pins: plan.pins }), `Board-Link, Stand ${plan.fingerprint},`); });
    $('#copyCollect').addEventListener('click', () => {
      const f = readForm();
      if (!f.to) { showMessages(['Für den Sammel-Link braucht es die Orga-Adresse (Feld oben).']); return; }
      if (!f.event.name) { showMessages(['Für den Sammel-Link braucht es den Event-Namen.']); return; }
      copy(linkFor('collect', { event: f.event, collect: { to: f.to } }), 'Sammel-Link');
    });
  }

  function wireResult() {
    $('#copyMd').addEventListener('click', () => plan && copy(plan.markdown, 'Markdown'));
    $('#dlCsv').addEventListener('click', () => plan && download(`${plan.event}-plan.csv`, plan.csv, 'text/csv'));
    $('#dlJson').addEventListener('click', () => plan && download(`${plan.event}-plan.json`, plan.exportJson, 'application/json'));
    $('#dlSvg').addEventListener('click', () => plan && download(`${plan.event}-plan.svg`, plan.svg, 'image/svg+xml'));
    $('#who').addEventListener('input', filterSheets);
  }

  function wireCollect() {
    $('#collectForm').addEventListener('submit', (e) => {
      e.preventDefault();
      const clean = (s) => s.replace(/[;\n\r\t]/g, ' ').trim();
      const handle = clean($('#c-handle').value);
      const line = [
        handle, clean($('#c-email').value), $('#c-tier').value, String(Number($('#c-plus').value) || 0),
        $('#c-card').checked ? 'ja' : 'nein', clean($('#c-knows').value), clean($('#c-note').value),
      ].join('; ');
      $('#c-line').value = line;
      $('#collectOut').hidden = false;
      const to = (state.collect && state.collect.to) || '';
      const a = $('#c-mail');
      a.hidden = !to;
      if (to) a.href = mailto(to, `vouch0rzii ${state.event.name || ''}: ${handle}`.trim(), line + '\n');
    });
    $('#c-copy').addEventListener('click', () => copy($('#c-line').value, 'Zeile'));
  }

  async function main() {
    try {
      bridge = await loadWasm();
    } catch (e) {
      $('#loading').textContent = 'Der Planer konnte nicht geladen werden. Der Browser braucht WebAssembly. ' + e;
      return;
    }
    $('#loading').hidden = true;
    $('#version').textContent = 'Version ' + bridge.version;
    wireAdmin();
    wireResult();
    wireCollect();
    const boot = () => {
      const st = readHash();
      if (st) {
        state = {
          ...emptyState(), mode: st.mode || 'admin', event: { ...DEFAULT_EVENT, ...(st.event || {}) },
          pins: st.pins || [], collect: st.collect || { to: '' }, status: st.status || {}, statusAt: st.statusAt || '', me: st.me || '',
        };
      }
      selected = null;
      plan = null;
      applyMode();
    };
    boot();
    window.addEventListener('hashchange', boot);
  }

  main();
})();
