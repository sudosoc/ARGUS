"use strict";

// Findings tab: a dense, filterable, sortable, paginated table that stays fast
// with tens of thousands of rows. Filters are faceted (collector, category,
// status, confidence, text) and shown as removable chips; rows expand in place
// to the full evidence; rows can be selected and exported client-side.
(function () {
  const { $, el } = UI;

  const STATUSES = ["found", "uncertain", "error", "not_found"];
  const SORTS = {
    status: (a, b) => STATUSES.indexOf(a.status) - STATUSES.indexOf(b.status),
    module: (a, b) => a.module.localeCompare(b.module),
    source: (a, b) => a.source.localeCompare(b.source),
    category: (a, b) => (a.category || "other").localeCompare(b.category || "other"),
    conf: (a, b) => a.confidence - b.confidence,
  };
  // Detail keys that are bookkeeping rather than evidence are listed last.
  const MINOR = new Set(["note", "md5", "http_status", "avatar", "username", "rank", "engine", "query", "host", "handle"]);

  const F = {
    q: "", mods: new Set(), cats: new Set(), stats: new Set(), conf: 0,
    sort: "conf", dir: -1, group: "none", page: 0, size: 100,
    sel: new Set(), open: new Set(),
  };
  UI.findingsFilter = F;

  UI.gotoFindings = function gotoFindings(over = {}) {
    Object.assign(F, { q: "", conf: 0, page: 0 }, { mods: new Set(over.mods || []), cats: new Set(over.cats || []), stats: new Set(over.stats || []) }, over.q != null ? { q: over.q } : {}, over.conf != null ? { conf: over.conf } : {});
    UI.selectTab("findings");
  };

  // Lower-cased searchable text per finding, computed once.
  const hay = new WeakMap();
  const haystack = (f) => {
    let h = hay.get(f);
    if (h == null) {
      h = (f.source + " " + (f.url || "") + " " + f.module + " " + (f.category || "") + " " + JSON.stringify(f.details || {}) + " " + (f.entities || []).map((e) => e.value).join(" ")).toLowerCase();
      hay.set(f, h);
    }
    return h;
  };

  UI.haystack = haystack;

  // Filtered + sorted list, recomputed only when findings or filters change.
  let cache = { key: "", list: [] };
  function filtered() {
    const S = UI.S;
    const key = [S.id, S.rev, F.q, [...F.mods].join(), [...F.cats].join(), [...F.stats].join(), F.conf, F.sort, F.dir, F.group].join("|");
    if (cache.key === key) return cache.list;
    const q = F.q.trim().toLowerCase();
    const list = S.findings.filter((f) => {
      if (F.mods.size && !F.mods.has(f.module)) return false;
      if (F.cats.size && !F.cats.has(f.category || "other")) return false;
      if (F.stats.size && !F.stats.has(f.status)) return false;
      if (f.confidence < F.conf) return false;
      return !q || haystack(f).includes(q);
    });
    const cmp = SORTS[F.sort];
    const gkey = F.group === "module" ? (f) => f.module : F.group === "category" ? (f) => UI.cat(f.category).label : null;
    list.sort((a, b) => {
      if (gkey) { const g = gkey(a).localeCompare(gkey(b)); if (g) return g; }
      return cmp(a, b) * F.dir || b.confidence - a.confidence;
    });
    cache = { key, list };
    return list;
  }

  const csvCell = (v) => {
    let s = String(v == null ? "" : v);
    if (/^[=+\-@\t\r]/.test(s)) s = "'" + s; // neutralise spreadsheet formula injection
    return /[",\n\r]/.test(s) ? '"' + s.replace(/"/g, '""') + '"' : s;
  };
  function toCSV(list) {
    const rows = [["module", "source", "category", "status", "confidence", "url", "via", "details"]];
    for (const f of list) rows.push([f.module, f.source, f.category || "", f.status, UI.pct(f.confidence), f.url || "", f.via || "", JSON.stringify(f.details || {})]);
    return rows.map((r) => r.map(csvCell).join(",")).join("\r\n");
  }

  // ------------------------------------------------------------------ facets
  function facetCounts(kind) {
    const m = new Map();
    for (const f of UI.S.findings) {
      const k = kind === "mods" ? f.module : kind === "cats" ? f.category || "other" : f.status;
      m.set(k, (m.get(k) || 0) + 1);
    }
    return m;
  }
  const facetLabel = (kind, k) => (kind === "cats" ? UI.cat(k).label : kind === "stats" ? UI.humanize(k) : k);

  function openFacet(kind, anchor) {
    const counts = facetCounts(kind);
    const keys = [...counts.keys()].sort((a, b) => (kind === "stats" ? STATUSES.indexOf(a) - STATUSES.indexOf(b) : counts.get(b) - counts.get(a) || a.localeCompare(b)));
    const box = el("div", { class: "facet" });
    const list = el("div", { class: "facet-list", role: "group" });
    const rows = [];
    for (const k of keys) {
      const cb = el("input", { type: "checkbox" });
      cb.checked = F[kind].has(k);
      cb.addEventListener("change", () => { cb.checked ? F[kind].add(k) : F[kind].delete(k); F.page = 0; paint(); });
      const row = el("label", { class: "check" }, cb,
        kind === "cats" ? UI.icon(UI.cat(k).icon) : kind === "stats" ? UI.statusBadge(k, true) : null,
        el("span", { class: "t", text: facetLabel(kind, k) }), el("span", { class: "num muted", text: UI.n(counts.get(k)) }));
      rows.push([row, facetLabel(kind, k).toLowerCase()]);
      list.append(row);
    }
    if (keys.length > 10) {
      const s = el("input", { type: "search", placeholder: "Filter list…", "aria-label": "Filter list" });
      s.addEventListener("input", () => { const q = s.value.toLowerCase(); rows.forEach(([r, t]) => (r.hidden = !!q && !t.includes(q))); });
      box.append(s);
    }
    const foot = el("div", { class: "facet-foot" }, UI.btn("Clear", { cls: "sm ghost", on: () => { F[kind].clear(); F.page = 0; UI.closePopover(); paint(); } }));
    box.append(list, foot);
    UI.popover(anchor, box);
  }

  // --------------------------------------------------------------- selection
  function exportMenu(anchor) {
    const S = UI.S;
    const all = filtered();
    const sel = [...F.sel].filter((f) => S.findings.includes(f));
    const target = sel.length ? sel : all;
    const what = sel.length ? `${UI.n(sel.length)} selected` : `${UI.n(all.length)} filtered`;
    const mk = (label, sub, fn) => {
      const b = el("button", { type: "button", class: "menu-item", role: "menuitem" }, UI.icon("download"), el("span", { text: label }), el("span", { class: "menu-sub", text: sub }));
      b.addEventListener("click", () => { UI.closePopover(); fn(); });
      return b;
    };
    const base = "argus_findings_" + S.id;
    const menu = el("div", { class: "menu", role: "menu" },
      el("div", { class: "menu-head", text: "Export " + what }),
      mk("JSON", ".json", () => UI.download(base + ".json", JSON.stringify(target, null, 2), "application/json")),
      mk("CSV", ".csv", () => UI.download(base + ".csv", toCSV(target), "text/csv")),
      mk("Copy URLs", "clipboard", () => UI.copy(target.map((f) => f.url).filter(Boolean).join("\n"), target.length + " URLs")),
      mk("Copy as JSON", "clipboard", () => UI.copy(JSON.stringify(target, null, 2), "JSON")));
    UI.popover(anchor, menu, { role: "menu", alignRight: true });
  }

  // ------------------------------------------------------------------- table
  function evidenceCell(f) {
    const td = el("td", { class: "c-evid" });
    const entries = Object.entries(f.details || {}).sort((a, b) => MINOR.has(a[0]) - MINOR.has(b[0]));
    for (const [k, v] of entries.slice(0, 2)) td.append(el("div", { class: "ev-line" }, el("span", { class: "ev-k", text: k }), el("span", { text: UI.clip(String(v), 120) })));
    if (entries.length > 2) td.append(el("div", { class: "hint", text: `+${entries.length - 2} more` }));
    return td;
  }

  function detailRow(f, cols) {
    const meta = { collector: f.module, category: UI.cat(f.category).label, status: f.status, confidence: UI.pct(f.confidence) + "%" };
    if (f.via) meta["found by following"] = f.via;
    if (f.timestamp) meta.collected = f.timestamp;
    if (f.url) meta.url = f.url;
    const left = el("div", {}, el("h4", { text: "Overview" }), UI.kvTable(meta));
    const right = el("div", {});
    if (f.details && Object.keys(f.details).length) right.append(el("h4", { text: "Evidence" }), UI.kvTable(f.details));
    if (f.entities && f.entities.length) {
      const chips = el("div", { class: "chips" });
      for (const e of f.entities.slice(0, 30)) chips.append(UI.entityChip(e));
      right.append(el("h4", { text: "Leads discovered here" }), chips);
    }
    const actions = el("div", { class: "detail-actions" },
      UI.btn("Copy value", { cls: "sm", icon: "copy", on: () => UI.copy(UI.findingValue(f), "value") }),
      UI.btn("Copy as JSON", { cls: "sm", icon: "copy", on: () => UI.copy(JSON.stringify(f, null, 2), "JSON") }),
      UI.btn("Notes & raw record", { cls: "sm", icon: "file", on: () => UI.showFinding(f) }));
    const href = UI.safeHref(f.url);
    if (href) actions.append(el("a", { class: "btn sm", href, target: "_blank", rel: "noopener noreferrer" }, UI.icon("external"), el("span", { text: "Open source" })));
    return el("tr", { class: "detail" }, el("td", { colspan: String(cols) }, el("div", { class: "detail-grid" }, left, right), actions));
  }

  function toggleOpen(f, tr) {
    if (F.open.has(f)) { F.open.delete(f); tr.nextSibling && tr.nextSibling.classList.contains("detail") && tr.nextSibling.remove(); }
    else { F.open.add(f); tr.after(detailRow(f, 6)); }
    tr.setAttribute("aria-expanded", String(F.open.has(f)));
    tr.querySelector(".expander").classList.toggle("on", F.open.has(f));
  }

  function row(f) {
    const p = UI.pct(f.confidence);
    const href = UI.safeHref(f.url);
    const cb = el("input", { type: "checkbox", "aria-label": "Select finding" });
    cb.checked = F.sel.has(f);
    cb.addEventListener("change", () => { cb.checked ? F.sel.add(f) : F.sel.delete(f); tr.classList.toggle("sel", cb.checked); paintSelection(); });
    cb.addEventListener("click", (e) => e.stopPropagation());
    const title = el("div", { class: "ftitle" },
      el("span", { class: "expander" + (F.open.has(f) ? " on" : "") }, UI.icon("chevron")),
      href ? el("a", { href, target: "_blank", rel: "noopener noreferrer", text: UI.clip(f.source, 120), title: f.url }) : el("span", { text: UI.clip(f.source, 120) }));
    const cat = UI.cat(f.category);
    const sub = el("div", { class: "fsub" }, el("span", { class: "tag" }, UI.icon(cat.icon), cat.label));
    if (f.via) sub.append(el("span", { class: "tag via", text: "via " + UI.clip(f.via, 40) }));
    if (href && f.url !== f.source) sub.append(el("span", { class: "furl", text: UI.clip(UI.host(f.url) || f.url, 60) }));
    const tr = el("tr", { class: "frow-main" + (F.sel.has(f) ? " sel" : ""), tabindex: "-1", "aria-expanded": String(F.open.has(f)) },
      el("td", { class: "c-sel" }, cb), el("td", { class: "c-status" }, UI.statusBadge(f.status)),
      el("td", { class: "c-mod" }, el("span", { class: "tag mod", text: f.module })),
      el("td", { class: "c-find" }, title, sub), evidenceCell(f),
      el("td", { class: "c-conf" }, el("span", { class: "num", text: p + "%" }), UI.meter(p)));
    tr.addEventListener("click", (e) => { if (!e.target.closest("a, button.chip, input")) toggleOpen(f, tr); });
    return tr;
  }

  function paintSelection() {
    const bar = $("fSelbar");
    const n = F.sel.size;
    bar.hidden = n === 0;
    $("fSelText").textContent = `${UI.n(n)} selected`;
  }

  // ---------------------------------------------------------------- chrome
  function build() {
    const root = $("tab-findings");
    if (root.querySelector(".ftool")) return;
    const q = el("input", { type: "search", id: "fq", placeholder: "Filter findings…  (evidence, URL, collector)", "aria-label": "Filter findings", autocomplete: "off" });
    q.addEventListener("input", UI.debounce(() => { F.q = q.value; F.page = 0; paint(); }, 200));
    const facet = (kind, label, icon) => {
      const b = el("button", { type: "button", class: "btn facet-btn", "aria-haspopup": "dialog", "aria-expanded": "false", "data-kind": kind }, UI.icon(icon), el("span", { text: label }), el("span", { class: "count fcount", hidden: true }));
      b.addEventListener("click", () => { if (b.getAttribute("aria-expanded") === "true") UI.closePopover(); else openFacet(kind, b); });
      return b;
    };
    const sel = (id, label, opts, onchange) => {
      const s = el("select", { id, "aria-label": label }, ...opts.map(([v, t]) => el("option", { value: v, text: t })));
      s.addEventListener("change", () => { onchange(s.value); F.page = 0; paint(); });
      return s;
    };
    const tool = el("div", { class: "ftool" },
      el("div", { class: "searchbox grow" }, UI.icon("search"), q),
      facet("mods", "Collector", "layers"), facet("cats", "Category", "tag"), facet("stats", "Status", "check"),
      sel("fConf", "Minimum confidence", [["0", "Any confidence"], ["0.5", "≥ 50%"], ["0.7", "≥ 70%"], ["0.8", "≥ 80%"], ["0.9", "≥ 90%"]], (v) => (F.conf = parseFloat(v))),
      sel("fGroup", "Group by", [["none", "No grouping"], ["module", "Group by collector"], ["category", "Group by category"]], (v) => (F.group = v)),
      UI.btn("Export", { icon: "download", cls: "", title: "Export selected or filtered findings", on: (e) => exportMenu(e.currentTarget) }));
    const chips = el("div", { class: "fchips", id: "fChips", "aria-live": "polite" });
    const selbar = el("div", { class: "selbar", id: "fSelbar", hidden: true }, el("span", { id: "fSelText" }),
      UI.btn("Select all filtered", { cls: "sm", on: () => { filtered().forEach((f) => F.sel.add(f)); paint(); } }),
      UI.btn("Clear selection", { cls: "sm ghost", on: () => { F.sel.clear(); paint(); } }));

    const thead = el("tr", {}, el("th", { class: "c-sel" }, (() => {
      const cb = el("input", { type: "checkbox", id: "fAll", "aria-label": "Select all on this page" });
      cb.addEventListener("change", () => { pageRows().forEach((f) => (cb.checked ? F.sel.add(f) : F.sel.delete(f))); paint(); });
      return cb;
    })()));
    for (const [key, label, cls] of [["status", "Status", "c-status"], ["module", "Collector", "c-mod"], ["source", "Finding", "c-find"]]) thead.append(sortTh(key, label, cls));
    thead.append(el("th", { class: "c-evid", text: "Evidence" }), sortTh("conf", "Confidence", "c-conf"));
    const table = el("table", { class: "ftable" }, el("thead", {}, thead), el("tbody", { id: "rows" }));
    const wrap = el("div", { class: "tablewrap" }, table);
    wrap.addEventListener("keydown", navKeys);
    root.append(tool, chips, selbar, el("div", { class: "pager top", id: "pagerTop" }), wrap, el("div", { class: "pager", id: "pagerBottom" }));
  }

  function sortTh(key, label, cls) {
    const b = el("button", { type: "button", class: "th-btn", "data-sort": key }, el("span", { text: label }), UI.icon("sort"));
    b.addEventListener("click", () => { if (F.sort === key) F.dir = -F.dir; else { F.sort = key; F.dir = key === "conf" ? -1 : 1; } F.page = 0; paint(); });
    return el("th", { class: cls, scope: "col", "data-sort": key }, b);
  }

  function pageRows() {
    const list = filtered();
    const pages = Math.max(1, Math.ceil(list.length / F.size));
    F.page = Math.min(F.page, pages - 1);
    return list.slice(F.page * F.size, F.page * F.size + F.size);
  }

  // j / k move between rows, Enter opens, x selects (when the table has focus).
  function navKeys(e) {
    const tr = e.target.closest && e.target.closest("tr.frow-main");
    const rows = [...$("rows").querySelectorAll("tr.frow-main")];
    if (!rows.length) return;
    const i = tr ? rows.indexOf(tr) : -1;
    const go = (n) => { const r = rows[Math.max(0, Math.min(rows.length - 1, n))]; rows.forEach((x) => (x.tabIndex = -1)); r.tabIndex = 0; r.focus(); e.preventDefault(); };
    if (e.target.matches("input, select, textarea")) return;
    if (e.key === "j" || e.key === "ArrowDown") go(i + 1);
    else if (e.key === "k" || e.key === "ArrowUp") go(i - 1);
    else if (tr && (e.key === "Enter" || e.key === " ") && e.target === tr) { e.preventDefault(); tr.click(); }
    else if (tr && e.key === "x") { const cb = tr.querySelector("input"); cb.checked = !cb.checked; cb.dispatchEvent(new Event("change")); }
  }

  function pager(box, total) {
    const pages = Math.max(1, Math.ceil(total / F.size));
    box.replaceChildren();
    if (total === 0) return;
    const from = F.page * F.size + 1, to = Math.min(total, (F.page + 1) * F.size);
    const prev = UI.btn("Previous", { cls: "sm", icon: "chevron-l", on: () => { F.page--; paint(true); } });
    const next = UI.btn("Next", { cls: "sm", on: () => { F.page++; paint(true); } });
    next.append(UI.icon("chevron"));
    prev.disabled = F.page === 0;
    next.disabled = F.page >= pages - 1;
    const size = el("select", { "aria-label": "Rows per page" }, ...[50, 100, 250, 500].map((n) => el("option", { value: String(n), text: n + " / page" })));
    size.value = String(F.size);
    size.addEventListener("change", () => { F.size = parseInt(size.value, 10); F.page = 0; paint(true); });
    box.append(el("span", { class: "pg-info num", text: `${UI.n(from)}–${UI.n(to)} of ${UI.n(total)}` }), prev, el("span", { class: "pg-page num", text: `Page ${F.page + 1} / ${pages}` }), next, size);
  }

  function paint(scrollTop) {
    const S = UI.S;
    build();
    const list = filtered();
    const rows = pageRows();
    const body = $("rows");
    body.replaceChildren();

    // header controls reflect state
    $("fq").value = F.q;
    $("fConf").value = String(F.conf);
    $("fGroup").value = F.group;
    for (const b of document.querySelectorAll(".facet-btn")) {
      const n = F[b.dataset.kind].size;
      const c = b.querySelector(".fcount");
      c.hidden = !n;
      c.textContent = String(n);
      b.classList.toggle("active", n > 0);
    }
    document.querySelectorAll("#tab-findings th[data-sort]").forEach((th) => {
      const on = th.dataset.sort === F.sort;
      th.setAttribute("aria-sort", on ? (F.dir > 0 ? "ascending" : "descending") : "none");
      th.classList.toggle("sorted", on);
    });

    // active filter chips
    const chips = $("fChips");
    chips.replaceChildren(el("span", { class: "fcount-text num", text: `${UI.n(list.length)} of ${UI.n(S.findings.length)} findings` }));
    const chip = (label, remove) => {
      const b = el("button", { type: "button", class: "chip removable", "aria-label": "Remove filter " + label }, el("span", { text: label }), UI.icon("x"));
      b.addEventListener("click", () => { remove(); F.page = 0; paint(); });
      chips.append(b);
    };
    if (F.q) chip(`text: ${UI.clip(F.q, 30)}`, () => (F.q = ""));
    F.mods.forEach((m) => chip("collector: " + m, () => F.mods.delete(m)));
    F.cats.forEach((c) => chip("category: " + UI.cat(c).label, () => F.cats.delete(c)));
    F.stats.forEach((s) => chip("status: " + s, () => F.stats.delete(s)));
    if (F.conf) chip(`confidence ≥ ${Math.round(F.conf * 100)}%`, () => (F.conf = 0));
    if (chips.children.length > 2) chips.append(UI.btn("Clear all", { cls: "sm ghost", on: () => { Object.assign(F, { q: "", conf: 0, page: 0 }); F.mods.clear(); F.cats.clear(); F.stats.clear(); paint(); } }));

    // body
    if (!rows.length) {
      body.append(el("tr", {}, el("td", { colspan: "6" }, UI.emptyState("search",
        S.findings.length ? "No findings match these filters" : S.state === "running" ? "Waiting for the first findings…" : "No findings",
        S.findings.length ? "Loosen or clear a filter to see more." : "Results stream in here as collectors report."))));
    }
    let lastGroup = null;
    const counts = new Map();
    if (F.group !== "none") for (const f of list) { const k = F.group === "module" ? f.module : UI.cat(f.category).label; counts.set(k, (counts.get(k) || 0) + 1); }
    for (const f of rows) {
      if (F.group !== "none") {
        const g = F.group === "module" ? f.module : UI.cat(f.category).label;
        if (g !== lastGroup) {
          lastGroup = g;
          body.append(el("tr", { class: "grouprow" }, el("th", { colspan: "6", scope: "colgroup" }, F.group === "category" ? UI.icon(UI.cat(f.category).icon) : UI.icon("layers"), el("span", { text: g }), el("span", { class: "count", text: UI.n(counts.get(g)) }))));
        }
      }
      const tr = row(f);
      body.append(tr);
      if (F.open.has(f)) body.append(detailRow(f, 6));
    }
    const first = body.querySelector("tr.frow-main");
    if (first) first.tabIndex = 0;
    $("fAll").checked = rows.length > 0 && rows.every((f) => F.sel.has(f));
    pager($("pagerTop"), list.length);
    pager($("pagerBottom"), list.length);
    paintSelection();
    if (scrollTop) $("workspace").scrollTop = 0;
  }

  UI.views.findings = () => paint(false);
  UI.resetHooks.push(() => {
    Object.assign(F, { q: "", conf: 0, page: 0, group: "none", sort: "conf", dir: -1 });
    F.mods.clear(); F.cats.clear(); F.stats.clear(); F.sel.clear(); F.open.clear();
    cache = { key: "", list: [] };
  });
})();
