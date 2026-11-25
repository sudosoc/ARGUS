"use strict";

// Building blocks for the section pages of the two-level navigation: panels,
// KPI strips, tables, finding lists, empty states. Sections are written as a
// few calls to these, so every page looks and behaves the same.
(function () {
  const { el } = UI;
  const K = (UI.kit = {});

  K.page = (...kids) => el("div", { class: "secpage" }, ...kids.flat().filter(Boolean));

  K.panel = (title, icon, ...kids) => {
    let count = null, sub = null;
    kids = kids.flat().filter((k) => {
      if (k && k.__opts) { ({ count = count, sub = sub } = k.__opts); return false; }
      return true;
    });
    return el("section", { class: "panel" },
      el("h3", { class: "ph" }, icon ? UI.icon(icon) : null, title, count != null ? el("span", { class: "count", text: UI.n(count) }) : null, sub ? el("span", { class: "ph-sub", text: sub }) : null),
      ...kids);
  };
  K.opts = (o) => ({ __opts: o });

  K.hint = (t) => el("p", { class: "hint", text: t });

  /** KPI strip: [[n, label, {warn, icon}], ...] */
  K.stats = (items) => el("div", { class: "kpis" }, ...items.filter(Boolean).map(([n, label, o = {}]) =>
    el("div", { class: "kpi static" + (o.warn ? " warn" : "") + (o.lead ? " lead" : "") }, el("span", { class: "n num", text: typeof n === "number" ? UI.n(n) : String(n) }), el("span", { class: "l" }, o.icon ? UI.icon(o.icon) : null, label))));

  /** Empty state that also says what would fill the page. */
  K.empty = (icon, title, text, unlock) => el("div", { class: "empty-state" }, UI.icon(icon, "big"), el("h3", { text: title }), el("p", { text }),
    unlock ? el("p", { class: "unlock" }, el("b", { text: "To fill this page: " }), unlock) : null);

  /** Meter + percent. */
  K.conf = (c) => UI.meter ? UI.meter(UI.pct(c), true) : el("span", { text: UI.pct(c) + "%" });

  K.chips = (list, onClick) => el("div", { class: "chips" }, ...list.map((x) => {
    const b = el("span", { class: "chip" }, x.icon ? UI.icon(x.icon) : null, x.k ? el("span", { class: "k", text: x.k }) : null, el("span", { class: "chip-v", text: x.v }));
    return b;
  }));

  K.entityChips = (ents, max = 60) => {
    const box = el("div", { class: "chips" });
    ents.slice(0, max).forEach((e) => box.append(UI.entityChip(e)));
    if (ents.length > max) box.append(el("span", { class: "hint", text: `+${UI.n(ents.length - max)} more` }));
    return box;
  };

  /**
   * Table. cols: [{label, get(row) -> string|Node, cls, sort(row)}]. Rows paginate by 100.
   * onRow(row) makes rows clickable.
   */
  K.table = (cols, rows, { onRow, page = 100, empty = "Nothing to show.", cls = "" } = {}) => {
    if (!rows.length) return el("p", { class: "empty-line", text: empty });
    let shown = page;
    const wrap = el("div", { class: "tablewrap" });
    const table = el("table", { class: "ctable " + cls });
    table.append(el("thead", {}, el("tr", {}, ...cols.map((c) => el("th", { scope: "col", class: c.cls || "", text: c.label })))));
    const tb = el("tbody");
    table.append(tb);
    const more = UI.btn("", { cls: "sm" });
    const draw = () => {
      tb.replaceChildren();
      for (const r of rows.slice(0, shown)) {
        const tr = el("tr", { class: onRow ? "clickable" : "" });
        for (const c of cols) {
          const v = c.get(r);
          tr.append(el("td", { class: c.cls || "" }, v instanceof Node ? v : document.createTextNode(v == null ? "" : String(v))));
        }
        if (onRow) { tr.tabIndex = 0; tr.addEventListener("click", () => onRow(r)); tr.addEventListener("keydown", (e) => { if (e.key === "Enter") onRow(r); }); }
        tb.append(tr);
      }
      more.hidden = shown >= rows.length;
      more.replaceChildren(el("span", { text: `Show ${Math.min(page, rows.length - shown)} more (${UI.n(rows.length - shown)} left)` }));
    };
    more.addEventListener("click", () => { shown += page; draw(); });
    draw();
    wrap.append(table);
    return el("div", {}, wrap, more);
  };

  /** One finding as a compact row: status, source, category, confidence and its first details. */
  K.findingRow = (f, { showModule = true } = {}) => {
    const d = Object.entries(f.details || {}).filter(([k]) => !["note", "http_status", "md5"].includes(k)).slice(0, 3);
    const row = el("button", { type: "button", class: "frow2" },
      el("span", { class: "fr-status" }, UI.statusBadge(f.status, true)),
      el("span", { class: "fr-main" },
        el("span", { class: "fr-title", text: f.source }),
        el("span", { class: "fr-meta" }, showModule ? el("span", { class: "tag mod", text: f.module }) : null, el("span", { class: "tag" }, UI.icon(UI.cat(f.category).icon), UI.cat(f.category).label),
          f.via ? el("span", { class: "tag via", text: "via " + f.via }) : null),
        d.length ? el("span", { class: "fr-det" }, d.map(([k, v]) => el("span", { class: "fr-kv" }, el("i", { text: k + " " }), UI.clip(String(v).replace(/\s+/g, " "), 110)))) : null),
      el("span", { class: "fr-conf" }, K.conf(f.confidence)));
    row.addEventListener("click", () => UI.showFinding(f));
    return row;
  };

  /** Panel with a paginated list of findings. */
  K.findings = (title, icon, list, { empty = "No findings in this group.", page = 50, showModule = true, sub } = {}) => {
    const p = K.panel(title, icon, K.opts({ count: list.length, sub }));
    if (!list.length) { p.append(el("p", { class: "empty-line", text: empty })); return p; }
    const box = el("div", { class: "flist" });
    let shown = page;
    const more = UI.btn("", { cls: "sm" });
    const draw = () => {
      box.replaceChildren(...list.slice(0, shown).map((f) => K.findingRow(f, { showModule })));
      more.hidden = shown >= list.length;
      more.replaceChildren(el("span", { text: `Show ${Math.min(page, list.length - shown)} more (${UI.n(list.length - shown)} left)` }));
    };
    more.addEventListener("click", () => { shown += page; draw(); });
    draw();
    p.append(box, more);
    return p;
  };

  /** Horizontal bars: [[label, n], ...] */
  K.bars = (rows, { max, onClick } = {}) => {
    const m = max || Math.max(1, ...rows.map((r) => r[1]));
    return el("div", { class: "hbars" }, ...rows.map(([label, n, sub]) => {
      const r = el("div", { class: "kbar" + (onClick ? " clickable" : ""), role: onClick ? "button" : null, tabindex: onClick ? 0 : null },
        el("span", { class: "hb-l", text: label }), el("span", { class: "hb-t" }, el("span", { class: "hb-f", style: `width:${Math.round((n / m) * 100)}%` })), el("span", { class: "hb-n num", text: UI.n(n) + (sub ? " · " + sub : "") }));
      if (onClick) r.addEventListener("click", () => onClick(label));
      return r;
    }));
  };

  /** Links the analyst can open: [{label, url, why}] */
  K.links = (list) => el("div", { class: "leadlist" }, ...list.map((l) => {
    const href = UI.safeHref(l.url);
    return el("div", { class: "lead" }, href ? el("a", { href, target: "_blank", rel: "noopener noreferrer", text: l.label }) : el("span", { text: l.label }), l.why ? el("span", { class: "why", text: l.why }) : null);
  }));

  K.kv = (o) => UI.kvTable(o);

  /** Short timestamp / date helpers. */
  K.date = (iso) => (iso ? String(iso).slice(0, 10) : "");
  K.dt = (ms) => (ms ? new Date(ms).toISOString().replace("T", " ").slice(0, 19) + "Z" : "");

  /** Score gauge: big number + bar. */
  K.gauge = (score, label, note) => el("div", { class: "gauge" },
    el("div", { class: "g-n num", text: String(Math.round(score)) }), el("div", { class: "g-body" }, el("div", { class: "g-l", text: label }), el("div", { class: "g-bar" }, el("span", { class: "g-fill", style: `width:${Math.max(2, Math.min(100, score))}%` })), note ? el("div", { class: "hint", text: note }) : null));

  /** A section that lists the findings `pick(M)` returns, with a short KPI strip. */
  K.section = (id, label, icon, pick, { group, empty, intro, unlock, showModule = true } = {}) => ({
    id, label, icon, group,
    count: (M) => pick(M).length,
    render(page, M) {
      const list = pick(M);
      if (!list.length) { page.append(K.empty(icon, "Nothing here yet", empty || "No findings in this group.", unlock)); return; }
      if (intro) page.append(K.hint(intro));
      page.append(K.stats([[list.length, "findings", { lead: true }], [new Set(list.map((f) => f.source)).size, "sources"], [list.filter((f) => f.confidence >= 0.8).length, "high confidence"]]));
      page.append(K.findings(label, icon, list, { showModule }));
    },
  });

  /** Blocks -> DOM and Markdown, so a report page and its download are the same content. */
  K.blocksToNode = (blocks) => {
    const root = el("div", { class: "report-doc" });
    for (const b of blocks) {
      if (b.h) root.append(el("h" + b.h, { text: b.t }));
      else if (b.p) root.append(el("p", { text: b.p }));
      else if (b.ul) root.append(el("ul", {}, ...b.ul.map((x) => el("li", { text: x }))));
      else if (b.table) {
        const t = el("table", { class: "ctable" }, el("thead", {}, el("tr", {}, ...b.table.head.map((h) => el("th", { text: h })))), el("tbody", {}, ...b.table.rows.map((r) => el("tr", {}, ...r.map((c) => el("td", { text: String(c) }))))));
        root.append(el("div", { class: "tablewrap" }, t));
      } else if (b.kv) root.append(UI.kvTable(b.kv));
    }
    return root;
  };
  const esc = (s) => String(s).replace(/\|/g, "\\|").replace(/\n/g, " ");
  K.blocksToMarkdown = (blocks) => blocks.map((b) => {
    if (b.h) return "#".repeat(b.h) + " " + b.t + "\n";
    if (b.p) return b.p + "\n";
    if (b.ul) return b.ul.map((x) => "- " + x).join("\n") + "\n";
    if (b.table) return "| " + b.table.head.map(esc).join(" | ") + " |\n|" + b.table.head.map(() => "---").join("|") + "|\n" + b.table.rows.map((r) => "| " + r.map(esc).join(" | ") + " |").join("\n") + "\n";
    if (b.kv) return Object.entries(b.kv).map(([k, v]) => `- **${k}:** ${String(v).replace(/\n/g, " ")}`).join("\n") + "\n";
    return "";
  }).join("\n");
})();
