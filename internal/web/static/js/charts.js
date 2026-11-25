"use strict";

// Copyright (C) 2026 Seif, SUDOSOC. SPDX-License-Identifier: AGPL-3.0-only

// Small chart builders shared by several views. Everything is plain DOM/CSS or
// inline SVG in the product palette: intensity is expressed by red level and
// by size, never by extra hues.
(function () {
  const { el } = UI;
  const SVGNS = "http://www.w3.org/2000/svg";
  const svg = (tag, attrs = {}, ...kids) => {
    const n = document.createElementNS(SVGNS, tag);
    for (const [k, v] of Object.entries(attrs)) n.setAttribute(k, String(v));
    for (const k of kids) if (k) n.append(k);
    return n;
  };
  UI.svg = svg;

  /** Weekday x hour heatmap of public activity, in the person's estimated local time or UTC. */
  UI.heatmap = function heatmap(tz) {
    const g = tz && tz.grid_utc;
    if (!g || !g.some((row) => row && row.some((v) => v > 0))) return null;
    let local = true;
    const wrap = el("div", { class: "heat-wrap" });
    const btn = UI.btn("", { cls: "sm" });
    const grid = el("div", { class: "heat", role: "img", "aria-label": "Activity by weekday and hour" });
    const days = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];
    function draw() {
      btn.replaceChildren(el("span", { text: local ? `Local time (${tz.label}) · switch to UTC` : "UTC · switch to local time" }));
      const off = local ? tz.utc_offset : 0;
      const m = Array.from({ length: 7 }, () => new Array(24).fill(0));
      for (let d = 0; d < 7; d++) for (let h = 0; h < 24; h++) {
        const v = (g[d] || [])[h] || 0;
        if (!v) continue;
        const hl = h + off;
        const dd = (((d + Math.floor(hl / 24)) % 7) + 7) % 7, hh = ((hl % 24) + 24) % 24;
        m[dd][hh] += v;
      }
      const max = Math.max(1, ...m.flat());
      grid.replaceChildren(el("div", {}));
      for (let h = 0; h < 24; h++) grid.append(el("div", { class: "hh", text: h % 3 === 0 ? String(h) : "" }));
      m.forEach((row, d) => {
        grid.append(el("div", { class: "hl", text: days[d] }));
        row.forEach((v, h) => {
          const c = el("div", { class: "hc", title: `${days[d]} ${String(h).padStart(2, "0")}:00 · ${v} event${v === 1 ? "" : "s"}` });
          if (v) c.style.background = `color-mix(in srgb, var(--emph) ${Math.max(14, Math.round((v / max) * 100))}%, var(--surface-2))`;
          grid.append(c);
        });
      });
    }
    btn.addEventListener("click", () => { local = !local; draw(); });
    draw();
    wrap.append(grid, el("div", { class: "heat-ctl" }, btn, el("span", { class: "hint", text: "Brighter red = more public activity in that hour." })));
    return wrap;
  };

  /** Events per UTC hour; the assumed local-night window is hatched. */
  UI.hoursChart = function hoursChart(tz) {
    const max = Math.max(1, ...tz.hours_utc);
    const quiet = new Set();
    for (let h = 0; h < 7; h++) quiet.add((((h - tz.utc_offset) % 24) + 24) % 24); // local 00:00-07:00 shifted back by the offset
    const wrap = el("div", { class: "hours", role: "img", "aria-label": "Events per hour (UTC)" });
    tz.hours_utc.forEach((n, h) => {
      const bar = el("div", { class: "hbar" + (quiet.has(h) ? " quiet" : ""), title: `${String(h).padStart(2, "0")}:00 UTC · ${n} events${quiet.has(h) ? " · assumed local night" : ""}` });
      const fill = el("div", { class: "hfill" });
      fill.style.height = Math.max(2, Math.round((n / max) * 100)) + "%";
      bar.append(fill, el("span", { text: String(h) }));
      wrap.append(bar);
    });
    return wrap;
  };


  /**
   * Marker shapes as SVG path strings centred on 0,0. The same string feeds
   * SVG <path d> and canvas Path2D, so legends, maps and the graph share one
   * vocabulary: meaning is carried by shape, not by hue.
   */
  const poly = (n, r, rot) => {
    const pts = [];
    for (let i = 0; i < n; i++) { const a = rot + (i * 2 * Math.PI) / n; pts.push((Math.cos(a) * r).toFixed(2) + " " + (Math.sin(a) * r).toFixed(2)); }
    return "M" + pts.join("L") + "Z";
  };
  const SHAPES = {
    circle: (r) => `M${r} 0A${r} ${r} 0 1 1 ${-r} 0A${r} ${r} 0 1 1 ${r} 0Z`,
    square: (r) => { const q = (r * 0.86).toFixed(2); return `M${-q} ${-q}H${q}V${q}H${-q}Z`; },
    diamond: (r) => poly(4, r * 1.15, -Math.PI / 2),
    triangle: (r) => poly(3, r * 1.15, -Math.PI / 2),
    "triangle-down": (r) => poly(3, r * 1.15, Math.PI / 2),
    hexagon: (r) => poly(6, r, 0),
    pentagon: (r) => poly(5, r, -Math.PI / 2),
    cross: (r) => { const a = (r * 0.4).toFixed(2), b = r.toFixed(2); return `M${-a} ${-b}H${a}V${-a}H${b}V${a}H${a}V${b}H${-a}V${a}H${-b}V${-a}H${-a}Z`; },
  };
  UI.shapePath = (name, r) => (SHAPES[name] || SHAPES.circle)(r);
  /** Small SVG glyph of a marker shape (filled with currentColor). */
  UI.shapeIcon = function shapeIcon(name, hollow) {
    const s = svg("svg", { class: "shape", viewBox: "-8 -8 16 16", "aria-hidden": "true" });
    const p = svg("path", { d: UI.shapePath(name, 6) });
    if (hollow) { p.setAttribute("fill", "none"); p.setAttribute("stroke", "currentColor"); p.setAttribute("stroke-width", "1.6"); }
    s.append(p);
    return s;
  };
  /** Read a design token (CSS custom property) for canvas drawing. */
  UI.token = (name) => getComputedStyle(document.documentElement).getPropertyValue(name).trim();

  const ms = (d) => Date.parse(d + "T00:00:00Z");

  /** Full horizontal timeline: dots on an axis, click a dot to read what happened there. */
  UI.timelinePanel = function timelinePanel(tl) {
    const p = el("section", { class: "panel" }, el("h3", { class: "ph" }, UI.icon("clock"), "Timeline", el("span", { class: "count", text: String(tl.length) })),
      el("p", { class: "hint", text: "Oldest to newest. Dots that fall close together are grouped; select one to read what happened there." }));
    const t0 = ms(tl[0].date);
    const span = Math.max(ms(tl[tl.length - 1].date) - t0, 30 * 86400000);
    const W = Math.max(680, Math.min(2600, tl.length * 46));
    const axis = el("div", { class: "htl" });
    axis.style.width = W + "px";
    const xOf = (t) => ((t - t0) / span) * (W - 24) + 12;
    const y0 = new Date(t0).getUTCFullYear(), y1 = new Date(t0 + span).getUTCFullYear();
    const tick = (x, label) => { const t = el("div", { class: "tick" }, el("span", { text: label })); t.style.left = x + "px"; axis.append(t); };
    if (y1 - y0 >= 1) { for (let y = y0 + 1; y <= y1; y++) tick(xOf(Date.UTC(y, 0, 1)), String(y)); tick(xOf(t0), tl[0].date.slice(0, 7)); }
    else { tick(xOf(t0), tl[0].date); tick(xOf(t0 + span), tl[tl.length - 1].date); }
    // cluster events whose dots would overlap
    const groups = [];
    for (const e of tl) {
      const x = xOf(ms(e.date));
      const g = groups[groups.length - 1];
      if (g && x - g.x < 16) g.items.push(e); else groups.push({ x, items: [e] });
    }
    const list = el("div", { class: "tl-list" });
    const show = (g, dot) => {
      axis.querySelectorAll(".ev.sel").forEach((d) => d.classList.remove("sel"));
      dot.classList.add("sel");
      list.replaceChildren();
      for (const e of g.items) {
        const href = UI.safeHref(e.url);
        list.append(el("div", { class: "tl-row" }, el("span", { class: "tl-date num", text: e.date }),
          el("span", {}, el("b", { text: e.event }), document.createTextNode(" · "), href ? el("a", { href, target: "_blank", rel: "noopener noreferrer", text: e.source }) : document.createTextNode(e.source))));
      }
    };
    groups.forEach((g, i) => {
      const press = g.items.some((e) => /press/.test(e.event));
      const dot = el("button", { type: "button", class: "ev" + (g.items.length > 1 ? " big" : "") + (press ? " sq" : ""),
        title: g.items.length > 1 ? `${g.items.length} events around ${g.items[0].date}` : `${g.items[0].date} · ${g.items[0].event}`, "aria-label": g.items[0].date + " " + g.items[0].event });
      dot.style.left = g.x + "px";
      dot.addEventListener("click", () => show(g, dot));
      axis.append(dot);
      if (i === groups.length - 1) show(g, dot);
    });
    p.append(el("div", { class: "tl-wrap" }, axis), list);
    return p;
  };

  /** Compact timeline for the overview: events per year as bars plus the latest few. */
  UI.miniTimeline = function miniTimeline(tl) {
    const box = el("div", { class: "mini-tl" });
    const years = new Map();
    for (const e of tl) { const y = e.date.slice(0, 4); years.set(y, (years.get(y) || 0) + 1); }
    const keys = [...years.keys()].sort();
    const max = Math.max(...years.values());
    const bars = el("div", { class: "yearbars", role: "img", "aria-label": "Events per year" });
    for (const y of keys.slice(-16)) {
      const b = el("div", { class: "yb", title: `${y}: ${years.get(y)} event${years.get(y) === 1 ? "" : "s"}` });
      const f = el("div", { class: "yb-fill" });
      f.style.height = Math.max(8, Math.round((years.get(y) / max) * 100)) + "%";
      b.append(f, el("span", { text: y.slice(2) }));
      bars.append(b);
    }
    box.append(bars);
    const recent = [...tl].sort((a, b) => b.date.localeCompare(a.date)).slice(0, 5);
    for (const e of recent) box.append(el("div", { class: "tl-row" }, el("span", { class: "tl-date num", text: e.date }), el("span", { text: UI.clip(e.event, 70) + " · " + UI.clip(e.source, 30) })));
    return box;
  };
})();
