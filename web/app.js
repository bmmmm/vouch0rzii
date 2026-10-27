// SPDX-License-Identifier: GPL-3.0-or-later
//
// Page logic for vouch0rzii. All planning happens in the Go wasm build
// (globalThis.vouch0rzii); this file only moves data between the form, the
// URL fragment and the DOM. The fragment "#s=…" is the whole state (see
// internal/plan/share.go); modes admin/view/collect are views on it, not
// access control.
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
  const MODE_LABEL = { admin: 'Admin', view: 'Ansicht', collect: 'Anmeldung' };
  const COLLECT_HEADER = 'Name;E-Mail;Status;+N;Kreditkarte;Kennt;Notiz';

  let api = null;
  let state = { v: 1, mode: 'admin', event: { ...DEFAULT_EVENT }, pins: [], collect: { to: '' } };
  let plan = null;      // last export (+ text/markdown/csv)
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

  // ---------- messages
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
      const ta = el('textarea', {}, text);
      document.body.append(ta);
      ta.select();
      const ok = document.execCommand('copy');
      ta.remove();
      toast(ok ? `${what} kopiert` : 'Kopieren fehlgeschlagen');
    }
  }
  function download(name, text, type) {
    const a = el('a', { href: URL.createObjectURL(new Blob([text], { type })), download: name });
    document.body.append(a);
    a.click();
    a.remove();
  }

  // ---------- state <-> URL
  function baseURL() { return location.href.split('#')[0]; }
  function encodeState(st) {
    const r = api.encodeState(JSON.stringify(st));
    if (!r.ok) throw new Error(r.errors.join('; '));
    return r.encoded;
  }
  function readHash() {
    const m = location.hash.match(/^#s=([A-Za-z0-9_-]+)$/);
    if (!m) return null;
    const r = api.decodeState(m[1]);
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
    return baseURL() + '#s=' + encodeState({ ...state, mode, ...extra });
  }

  // ---------- form <-> state.event
  function formToEvent() {
    const ev = state.event;
    ev.name = $('#f-name').value.trim();
    ev.vouchers = Number($('#f-vouchers').value) || 0;
    ev.start = $('#f-start').value;
    ev.weekends = $('#f-weekends').checked;
    ev.plusMax = Number($('#f-plusMax').value) || 0;
    ev.seed = $('#f-seed').value.trim();
    ev.handover = $('#f-handover').value.trim();
    state.collect = { to: $('#f-collectTo').value.trim() };
  }
  function eventToForm() {
    const ev = state.event;
    $('#f-name').value = ev.name || '';
    $('#f-vouchers').value = ev.vouchers || 7;
    $('#f-start').value = ev.start || '';
    $('#f-weekends').checked = !!ev.weekends;
    $('#f-plusMax').value = ev.plusMax || 2;
    $('#f-seed').value = ev.seed || '';
    $('#f-handover').value = ev.handover || '23:00';
    $('#f-collectTo').value = (state.collect && state.collect.to) || '';
  }
  function rosterToPeople() {
    const r = api.parseRoster($('#roster').value);
    if (!r.ok) {
      showMessages(r.errors, r.warnings);
      return null;
    }
    if (r.warnings.length) showMessages([], r.warnings);
    return JSON.parse(r.peopleJson);
  }
  function peopleToRoster(people) {
    const r = api.formatRoster(JSON.stringify(people || []));
    $('#roster').value = r.ok ? r.tsv : '';
  }

  // ---------- planning
  function computePlan({ fromForm } = { fromForm: true }) {
    if (fromForm) {
      formToEvent();
      const people = rosterToPeople();
      if (!people) return false;
      state.event.people = people;
    }
    const r = api.plan(JSON.stringify(state.event), JSON.stringify(state.pins));
    if (!r.ok) {
      showMessages(r.errors, r.warnings);
      plan = null;
      renderResult();
      return false;
    }
    plan = { ...JSON.parse(r.exportJson), text: r.text, markdown: r.markdown, csv: r.csv };
    showMessages([], plan.warnings);
    renderResult();
    writeHash();
    return true;
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
  function cellOf(label) {
    for (let d = 0; d < plan.grid.length; d++) {
      const v = plan.grid[d].indexOf(label);
      if (v >= 0) return { label, day: d + 1, voucher: v + 1 };
    }
    return null;
  }
  function moveSlot(label, target) {
    const src = cellOf(label);
    if (!src || (src.day === target.day && src.voucher === target.voucher)) { selected = null; renderResult(); return; }
    if (target.label && target.label !== label) setPin(target.label, src.day, src.voucher);
    setPin(label, target.day, target.voucher);
    selected = null;
    computePlan({ fromForm: false });
  }
  function onCellPick(cell) {
    if (state.mode !== 'admin' || !plan) return;
    if (!selected) {
      if (!cell.label) return;
      selected = cell;
      renderMatrix();
      return;
    }
    if (selected.label === cell.label) { selected = null; renderMatrix(); return; }
    moveSlot(selected.label, cell);
  }

  // ---------- rendering
  function renderResult() {
    const show = !!plan;
    $('#result').hidden = !show;
    if (!show) return;
    const s = plan.stats;
    $('#r-title').textContent = `Plan ${plan.event}`;
    $('#r-stats').textContent =
      `${plan.vouchers} Vouchers · ${s.days} Tage · ${s.slots} Slots · ${s.handovers} Übergaben, davon ${s.known} unter Bekannten, ${s.unknown} unter Fremden` +
      (s.forbidden ? `, ${s.forbidden} direkt an eigene +N` : '') + (s.pinned ? ` · ${s.pinned} gepinnt` : '') + ` · Übergabe ${plan.handover}`;
    $('#r-pinhelp').hidden = state.mode !== 'admin';
    renderMatrix();
    renderHandovers();
    renderSheets();
  }

  function renderMatrix() {
    const table = $('#matrix');
    const editable = state.mode === 'admin';
    table.classList.toggle('editable', editable);
    const incoming = new Map(); // "day-voucher" of the receiving cell → handover
    for (const h of plan.handovers) incoming.set(`${h.day + 1}-${h.voucher}`, h);
    const pinned = new Set(plan.pins.filter((p) => state.pins.some((q) => q.label === p.label)).map((p) => p.label));
    const days = plan.days.slice();
    if (editable) days.push({ day: days.length + 1, date: '', label: '+ Tag', extra: true });

    const head = el('tr', {}, el('th', {}, 'Voucher'), days.map((d) => el('th', {}, d.label, d.date && el('small', {}, d.date))));
    const rows = [];
    for (let v = 1; v <= plan.vouchers; v++) {
      const tds = days.map((d) => {
        const label = d.extra ? '' : (plan.grid[d.day - 1][v - 1] || '');
        const cell = { label, day: d.day, voucher: v };
        const h = incoming.get(`${d.day}-${v}`);
        const cls = ['cell'];
        if (!label) cls.push('empty');
        if (d.extra) cls.push('extra');
        if (h) cls.push(h.forbidden ? 'forbidden' : h.known ? 'known' : 'stranger');
        if (label && pinned.has(label)) cls.push('pinned');
        if (selected && selected.label === label && label) cls.push('selected');
        const td = el('td', { class: cls.join(' '), draggable: editable && !!label, title: h ? `${h.from} → ${h.to}` : (label ? 'bekommt den Voucher von der Orga' : '') },
          label || (editable ? '·' : ''));
        if (editable) {
          td.addEventListener('click', () => onCellPick(cell));
          if (label) {
            td.addEventListener('dragstart', (e) => { selected = cell; e.dataTransfer.setData('text/plain', label); e.dataTransfer.effectAllowed = 'move'; });
            if (pinned.has(label)) {
              td.append(el('button', { class: 'unpin', type: 'button', title: 'Pin lösen', onclick: (e) => { e.stopPropagation(); unpin(label); } }, '×'));
            }
          }
          td.addEventListener('dragover', (e) => { e.preventDefault(); td.classList.add('dragover'); });
          td.addEventListener('dragleave', () => td.classList.remove('dragover'));
          td.addEventListener('drop', (e) => {
            e.preventDefault();
            td.classList.remove('dragover');
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
    let day = 0, list = null;
    for (const h of plan.handovers) {
      if (h.day !== day) {
        day = h.day;
        box.append(el('h3', {}, `${plan.days[day - 1].label} (${h.date}), ${plan.handover} Uhr`));
        list = el('ul');
        box.append(list);
      }
      const mark = h.forbidden ? el('span', { class: 'mark forbidden-text', title: 'direkt an eigene +N' }, '!') : h.known ? el('span', { class: 'mark known-text', title: 'kennen sich' }, '✓') : el('span', { class: 'mark' }, '·');
      list.append(el('li', {}, mark, ` Voucher ${h.voucher}: ${h.from} → ${h.to}`, h.toEmail && el('span', { class: 'muted' }, ` <${h.toEmail}>`)));
    }
    if (!plan.handovers.length) box.append(el('p', { class: 'hint' }, 'Nur ein Tag, keine Übergaben.'));
  }

  function mailtoFor(sheet) {
    if (!sheet.to) return null;
    const next = plan.sheets.find((s) => s.label === sheet.to);
    if (!next || !next.email) return null;
    const subject = `Voucher ${sheet.voucher} für ${plan.event}`;
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
    return `mailto:${encodeURIComponent(next.email)}?subject=${encodeURIComponent(subject)}&body=${encodeURIComponent(body)}`;
  }

  function renderSheets() {
    const table = $('#sheets');
    const head = el('tr', {}, ['Slot', 'Kontakt', 'Tag', 'Voucher', 'bekommt von', 'gibt weiter an', ''].map((h) => el('th', {}, h)));
    const rows = plan.sheets.map((s) => {
      const mail = mailtoFor(s);
      return el('tr', { dataset: { who: `${s.label} ${s.person} ${s.email || ''}`.toLowerCase() } },
        el('td', {}, s.label, s.pinned ? ' 📌' : ''),
        el('td', {}, s.person, s.email && el('small', {}, el('br'), s.email), s.card ? el('small', { title: 'Kreditkarte für den Notfall' }, ' 💳') : ''),
        el('td', {}, `${plan.days[s.day - 1].label}`, el('small', {}, el('br'), s.date)),
        el('td', {}, s.voucher),
        el('td', {}, s.from, s.fromEmail && el('small', {}, el('br'), s.fromEmail)),
        el('td', {}, s.to || '— Kette endet', s.toEmail && el('small', {}, el('br'), s.toEmail)),
        el('td', {}, mail && el('a', { href: mail, title: 'Mail an die nächste Person vorbereiten' }, 'Mail')),
      );
    });
    table.replaceChildren(el('thead', {}, head), el('tbody', {}, rows));
    filterSheets();
  }
  function filterSheets() {
    const q = $('#who').value.trim().toLowerCase();
    for (const tr of $('#sheets').querySelectorAll('tbody tr')) {
      const hit = q && tr.dataset.who.includes(q);
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
    if (m === 'collect') {
      $('#c-event').textContent = state.event.name || 'das Event';
      $('#c-plus').max = String(state.event.plusMax || 2);
      return;
    }
    eventToForm();
    peopleToRoster(state.event.people);
    if (state.event.people && state.event.people.length) computePlan({ fromForm: false });
  }

  async function loadExamples() {
    try {
      const names = await (await fetch('events/index.json')).json();
      $('#examples').replaceChildren(names.map((n) => el('option', { value: n }, n)));
    } catch {
      $('#examples').replaceChildren(el('option', { value: '' }, 'keine Beispiele'));
    }
  }

  function wireAdmin() {
    $('#compute').addEventListener('click', () => { selected = null; computePlan(); });
    $('#settings').addEventListener('submit', (e) => { e.preventDefault(); computePlan(); });
    $('#loadExample').addEventListener('click', async () => {
      const name = $('#examples').value;
      if (!name) return;
      try {
        const ev = await (await fetch(`events/${name}.json`)).json();
        state.event = { ...DEFAULT_EVENT, ...ev };
        state.pins = [];
        eventToForm();
        peopleToRoster(state.event.people);
        computePlan({ fromForm: false });
      } catch (e) {
        showMessages([`Beispiel ${name} konnte nicht geladen werden: ${e}`]);
      }
    });
    $('#loadFile').addEventListener('change', async (e) => {
      const f = e.target.files[0];
      if (!f) return;
      try {
        const ev = JSON.parse(await f.text());
        state.event = { ...DEFAULT_EVENT, ...ev };
        state.pins = [];
        eventToForm();
        peopleToRoster(state.event.people);
        computePlan({ fromForm: false });
      } catch (err) {
        showMessages([`Datei nicht lesbar: ${err}`]);
      }
      e.target.value = '';
    });
    $('#saveFile').addEventListener('click', () => {
      formToEvent();
      const people = rosterToPeople();
      if (!people) return;
      state.event.people = people;
      download(`${state.event.name || 'event'}.json`, JSON.stringify(state.event, null, 2) + '\n', 'application/json');
    });
    $('#addInbox').addEventListener('click', () => {
      const lines = $('#inbox').value.split('\n').map((l) => l.trim()).filter(Boolean);
      if (!lines.length) return;
      const r = api.parseRoster([COLLECT_HEADER, ...lines].join('\n'));
      if (!r.ok) { showMessages(r.errors, r.warnings); return; }
      const current = $('#roster').value.trim() ? rosterToPeople() : [];
      if (!current) return;
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

    $('#copyAdmin').addEventListener('click', () => { if (computePlan()) copy(linkFor('admin', {}), 'Admin-Link'); });
    $('#copyView').addEventListener('click', () => { if (computePlan()) copy(linkFor('view', { pins: plan.pins }), 'Viewer-Link'); });
    $('#copyCollect').addEventListener('click', () => {
      formToEvent();
      if (!state.collect.to) { showMessages(['Für den Sammel-Link braucht es die Orga-Adresse (Feld oben).']); return; }
      if (!state.event.name) { showMessages(['Für den Sammel-Link braucht es den Event-Namen.']); return; }
      copy(linkFor('collect', {}), 'Sammel-Link');
    });
  }

  function wireResult() {
    $('#copyMd').addEventListener('click', () => plan && copy(plan.markdown, 'Markdown'));
    $('#copyText').addEventListener('click', () => plan && copy(plan.text, 'Text'));
    $('#dlCsv').addEventListener('click', () => plan && download(`${plan.event}-plan.csv`, plan.csv, 'text/csv'));
    $('#dlJson').addEventListener('click', () => plan && download(`${plan.event}-plan.json`, plan.exportJson || JSON.stringify(plan, null, 2), 'application/json'));
    $('#who').addEventListener('input', filterSheets);
  }

  function wireCollect() {
    $('#collectForm').addEventListener('submit', (e) => {
      e.preventDefault();
      const clean = (s) => s.replace(/[;\n\r\t]/g, ' ').trim();
      const line = [
        clean($('#c-handle').value), clean($('#c-email').value), $('#c-tier').value, String(Number($('#c-plus').value) || 0),
        $('#c-card').checked ? 'ja' : 'nein', clean($('#c-knows').value), clean($('#c-note').value),
      ].join('; ');
      $('#c-line').value = line;
      $('#collectOut').hidden = false;
      const to = (state.collect && state.collect.to) || '';
      const subject = `vouch0rzii ${state.event.name || ''}: ${clean($('#c-handle').value)}`.trim();
      const a = $('#c-mail');
      if (to) {
        a.href = `mailto:${encodeURIComponent(to)}?subject=${encodeURIComponent(subject)}&body=${encodeURIComponent(line + '\n')}`;
        a.hidden = false;
      } else {
        a.hidden = true;
      }
    });
    $('#c-copy').addEventListener('click', () => copy($('#c-line').value, 'Zeile'));
  }

  async function main() {
    try {
      api = await loadWasm();
    } catch (e) {
      $('#loading').textContent = 'Der Planer konnte nicht geladen werden. Der Browser braucht WebAssembly. ' + e;
      return;
    }
    $('#loading').hidden = true;
    $('#version').textContent = 'Version ' + api.version;
    wireAdmin();
    wireResult();
    wireCollect();
    loadExamples();
    const boot = () => {
      const st = readHash();
      if (st) {
        state = { v: 1, mode: st.mode || 'admin', event: { ...DEFAULT_EVENT, ...(st.event || {}) }, pins: st.pins || [], collect: st.collect || { to: '' } };
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
