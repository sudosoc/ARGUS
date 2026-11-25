"use strict";

// Relationship graph: the subject, the identifiers it was given and every
// entity discovered, linked by where each lead came from. A small
// force-directed layout on <canvas>, no libraries. Monochrome: node type is
// told by shape, strength by filled vs outlined, and red marks the subject and
// strong links. Exports PNG and SVG without leaving the browser.
(function () {
  const { $, el } = UI;

  // Entity type -> marker shape and grey level (0 = lightest). Unknown types fall back to a small circle.
  const GLYPH = {
    subject: ["circle", 0], name: ["circle", 0], username: ["diamond", 1], email: ["square", 0], phone: ["cross", 1],
    domain: ["hexagon", 0], ip: ["triangle-down", 2], organisation: ["pentagon", 1], location: ["circle", 3],
    asn: ["hexagon", 3], wallet: ["square", 2], url: ["circle", 3],
  };
  const glyph = (t) => GLYPH[t] || ["circle", 2];
  const INPUT_TYPE = { full_name: "name", username: "username", alias: "username", email: "email", phone: "phone", domain: "domain", ip: "ip", asn: "asn", wallet: "wallet", company: "organisation", location: "location" };
  const MODULE_INPUT = {
    social: ["username"], profiles: ["username"], footprint: ["email", "phone"], phone: ["phone"], domain: ["domain"], ip: ["ip"],
    asn: ["asn"], company: ["organisation"], people: ["name"], images: ["name", "username"], activity: ["username"], breach: ["email"],
    sitefiles: ["domain"], bizreg: ["organisation"], devtrace: ["username", "email"], mentions: ["name"], docmeta: ["domain"],
    records: ["name", "domain", "ip"], crypto: ["wallet"], syswrap: ["domain", "username"], dorks: [],
  };
  const MAX_NODES = 800;

  let canvas = null, ctx = null, tip = null;
  let nodes = [], edges = [], byId = new Map();
  let view = { x: 0, y: 0, k: 1 };
  let alpha = 1, raf = null, hover = null, selected = null, drag = null, showWeak = true, fitted = false, touched = false;
  let groupLabels = [], layoutMode = "force", settling = false, coreMode = false, capNote = "", shell = null, mini = null, mctx = null, mmap = null, showMini = null;
  const ptrs = new Map();
  let pinch = null, lastStats = "";
  const hiddenTypes = new Set();
  let shadeBy = "type", findText = "", typesPresent = new Map();
  let T = {}; // design tokens resolved for canvas drawing

  const collectorOf = (n) => (n.source || "input").split(/[:/]/)[0].trim() || "input";
  const grays = () => [T.text, T.text2, T.text3, T.text4];
  const levelOf = (n) => {
    if (shadeBy === "collector" && n.type !== "subject") { let h = 0; for (const ch of collectorOf(n)) h = (h * 31 + ch.charCodeAt(0)) >>> 0; return h % 4; }
    return glyph(n.type)[1];
  };
  const key = (type, value) => type + ":" + String(value).toLowerCase();

  function readTokens() {
    T = { text: UI.token("--text"), text2: UI.token("--text-2"), text3: UI.token("--text-3"), text4: UI.token("--text-4"), line: UI.token("--line-strong"),
      accent: UI.token("--accent"), strong: UI.token("--emph-strong"), emph: UI.token("--emph"), bg: UI.token("--surface-1") };
  }

  // ------------------------------------------------------------------ model
  function build() {
    const S = UI.S;
    const nm = new Map(), em = [];
    const add = (type, value, extra) => {
      const id = key(type, value);
      let n = nm.get(id);
      if (!n) { n = { id, type, value, strong: false, input: false, deg: 0 }; nm.set(id, n); }
      Object.assign(n, extra && Object.fromEntries(Object.entries(extra).filter(([, v]) => v)));
      return n;
    };
    const t = S.target || {};
    const subject = add("subject", "subject", { strong: true, input: true });
    subject.label = UI.targetLabel(t) || "subject";
    const inputsByType = {};
    for (const [field, type] of Object.entries(INPUT_TYPE)) {
      if (!t[field]) continue;
      const n = add(type, t[field], { strong: true, input: true });
      (inputsByType[type] = inputsByType[type] || []).push(n);
      if (type !== "location") em.push([subject, n, "input"]);
    }
    for (const e of S.entities.values()) if (e.type !== "image") add(e.type, e.value, { strong: e.strong, source: e.source });

    const parentFor = (f) => {
      if (f.via) { const p = nm.get(String(f.via).toLowerCase()); if (p) return p; }
      for (const type of MODULE_INPUT[f.module] || []) if (inputsByType[type]) return inputsByType[type][0];
      return subject;
    };
    for (const f of S.findings) {
      if (!f.entities || !f.entities.length) continue;
      const parent = parentFor(f);
      for (const e of f.entities) {
        const n = nm.get(key(e.type, e.value));
        if (n && n !== parent) em.push([parent, n, f.module]);
      }
    }
    const seen = new Set();
    let ed = [];
    for (const [a, b, label] of em) {
      const k = a.id + ">" + b.id;
      if (seen.has(k) || seen.has(b.id + ">" + a.id)) continue;
      seen.add(k);
      ed.push({ a, b, label });
      a.deg++; b.deg++;
    }
    let list = [...nm.values()];
    typesPresent = new Map();
    for (const n of list) typesPresent.set(n.type, (typesPresent.get(n.type) || 0) + 1);
    if (hiddenTypes.size) {
      const keep = new Set(list.filter((n) => n.type === "subject" || !hiddenTypes.has(n.type)).map((n) => n.id));
      list = list.filter((n) => keep.has(n.id));
      ed = ed.filter((e) => keep.has(e.a.id) && keep.has(e.b.id));
    }
    if (!showWeak) {
      const keep = new Set(list.filter((n) => n.strong || n.input).map((n) => n.id));
      list = list.filter((n) => keep.has(n.id));
      ed = ed.filter((e) => keep.has(e.a.id) && keep.has(e.b.id));
    }
    let capped = false;
    if (list.length > MAX_NODES) {
      capped = true;
      list.sort((x, y) => (y.input - x.input) || (y.strong - x.strong) || (y.deg - x.deg));
      list = list.slice(0, MAX_NODES);
      const ids = new Set(list.map((n) => n.id));
      ed = ed.filter((e) => ids.has(e.a.id) && ids.has(e.b.id));
    }
    const prev = byId;
    const w = canvas.clientWidth || 800, h = canvas.clientHeight || 500;
    list.forEach((n, i) => {
      const p = prev.get(n.id);
      if (p) { n.x = p.x; n.y = p.y; n.vx = p.vx; n.vy = p.vy; n.fixed = p.fixed; }
      else {
        const ang = (i / Math.max(1, list.length)) * Math.PI * 2, r = n.input ? 60 : 160 + Math.random() * 80;
        n.x = w / 2 + Math.cos(ang) * r; n.y = h / 2 + Math.sin(ang) * r; n.vx = 0; n.vy = 0;
      }
      if (n.type === "subject") { if (!p) { n.x = w / 2; n.y = h / 2; } n.fixed = true; }
    });
    nodes = list; edges = ed; byId = new Map(list.map((n) => [n.id, n]));
    legend(capped);
  }

  function legend(capped) {
    const box = $("graphLegend");
    box.replaceChildren();
    if (shadeBy === "collector") {
      box.append(el("span", { class: "hint", text: "Grey level = collector that found the lead (four levels, cycling). Shape still shows the type." }));
    }
    for (const [type, n] of typesPresent) {
      if (type === "subject") continue;
      const off = hiddenTypes.has(type);
      const b = el("button", { type: "button", class: "chip lg-chip" + (off ? " off" : ""), "aria-pressed": String(!off), title: (off ? "Show " : "Hide ") + type },
        UI.shapeIcon(glyph(type)[0]), el("span", { class: "chip-v", text: type }), el("span", { class: "count", text: String(n) }));
      b.addEventListener("click", () => { off ? hiddenTypes.delete(type) : hiddenTypes.add(type); build(); kick(0.7); });
      box.append(b);
    }
    box.append(el("span", { class: "chip static" }, el("span", { class: "subjdot" }), el("span", { class: "chip-v", text: "subject" })),
      el("span", { class: "chip static" }, UI.shapeIcon("circle"), el("span", { class: "chip-v", text: "filled = strong lead" })),
      el("span", { class: "chip static" }, UI.shapeIcon("circle", true), el("span", { class: "chip-v", text: "outline = weak lead" })));
    capNote = capped ? " · strongest shown" : "";
    stats();
  }
  function stats() {
    const el2 = $("graphStats");
    if (!el2) return;
    const t = `${nodes.length} nodes · ${edges.length} links${capNote} · zoom ${Math.round(view.k * 100)}%`;
    if (t !== lastStats) { lastStats = t; el2.textContent = t; }
  }

  // ---------------------------------------------------------------- physics
  function step() {
    const n = nodes.length;
    const CELL = 300, grid = new Map();
    const hk = (cx, cy) => (cx + 4096) * 8192 + (cy + 4096);
    for (let i = 0; i < n; i++) {
      const a = nodes[i]; a.i = i;
      const k = hk(Math.floor(a.x / CELL), Math.floor(a.y / CELL));
      const c = grid.get(k); if (c) c.push(a); else grid.set(k, [a]);
    }
    for (let i = 0; i < n; i++) {
      const a = nodes[i], cx = Math.floor(a.x / CELL), cy = Math.floor(a.y / CELL);
      for (let gx = -1; gx <= 1; gx++) for (let gy = -1; gy <= 1; gy++) {
        const c = grid.get(hk(cx + gx, cy + gy));
        if (!c) continue;
        for (const b of c) {
          if (b.i <= i) continue;
          let dx = a.x - b.x, dy = a.y - b.y, d2 = dx * dx + dy * dy;
          if (d2 < 0.01) { dx = Math.random() - 0.5; dy = Math.random() - 0.5; d2 = 0.5; }
          if (d2 > 90000) continue;
          const f = (2600 * alpha) / d2, d = Math.sqrt(d2);
          dx = (dx / d) * f; dy = (dy / d) * f;
          a.vx += dx; a.vy += dy; b.vx -= dx; b.vy -= dy;
        }
      }
    }
    for (const e of edges) {
      const dx = e.b.x - e.a.x, dy = e.b.y - e.a.y, d = Math.sqrt(dx * dx + dy * dy) || 1;
      const len = e.a.type === "subject" ? 95 : 70, f = (d - len) * 0.02 * alpha * 4;
      const fx = (dx / d) * f, fy = (dy / d) * f;
      e.a.vx += fx; e.a.vy += fy; e.b.vx -= fx; e.b.vy -= fy;
    }
    const cx = (canvas.clientWidth || 800) / 2, cy = (canvas.clientHeight || 500) / 2;
    for (const a of nodes) {
      a.vx += (cx - a.x) * 0.0016 * alpha * 4; a.vy += (cy - a.y) * 0.0016 * alpha * 4;
      if (a.fixed) { a.vx = a.vy = 0; continue; }
      a.vx *= 0.82; a.vy *= 0.82;
      a.x = Math.max(-3500, Math.min(3500, a.x + Math.max(-30, Math.min(30, a.vx)))); a.y = Math.max(-3500, Math.min(3500, a.y + Math.max(-30, Math.min(30, a.vy))));
    }
    alpha *= 0.985;
  }

  function loop() {
    raf = null;
    let busy = false;
    if (layoutMode === "force") {
      if (alpha > 0.02) { step(); step(); busy = true; }
    } else if (settling) {
      let moved = 0;
      for (const n of nodes) {
        if (n.tx == null) continue;
        const dx = n.tx - n.x, dy = n.ty - n.y;
        n.x += dx * 0.2; n.y += dy * 0.2;
        moved = Math.max(moved, Math.abs(dx), Math.abs(dy));
      }
      if (moved < 0.6) { for (const n of nodes) if (n.tx != null) { n.x = n.tx; n.y = n.ty; } settling = false; } else busy = true;
    }
    // Frame the graph once the layout has settled, unless the analyst already moved the view.
    const calm = layoutMode === "force" ? alpha < 0.25 : !settling;
    if (!fitted && !touched && calm) { autoFit(); fitted = true; }
    draw();
    if (busy || drag) raf = requestAnimationFrame(loop);
  }
  const kick = (a) => { alpha = Math.max(alpha, a); if (!raf) raf = requestAnimationFrame(loop); };

  // ---------------------------------------------------------------- drawing
  const radius = (n) => (n.type === "subject" ? 13 : n.input ? 9 : 4.5 + Math.min(7, Math.sqrt(n.deg) * 2));
  const pathCache = new Map();
  const pathFor = (shape, r) => { const k = shape + Math.round(r * 2); let p = pathCache.get(k); if (!p) { p = new Path2D(UI.shapePath(shape, r)); pathCache.set(k, p); } return p; };

  function drawNode(g, n, dim, k) {
    const r = Math.max(radius(n), Math.round(3.2 / k));
    g.save();
    g.translate(n.x, n.y);
    g.globalAlpha = dim ? 0.15 : 1;
    if (n.type === "subject") {
      g.beginPath(); g.arc(0, 0, r, 0, Math.PI * 2); g.fillStyle = T.accent; g.fill();
      g.lineWidth = 2.5 / k; g.strokeStyle = T.strong; g.stroke();
      g.beginPath(); g.arc(0, 0, r * 0.38, 0, Math.PI * 2); g.fillStyle = T.text; g.fill();
    } else {
      const col = grays()[levelOf(n)];
      const p = pathFor(glyph(n.type)[0], r);
      if (n.strong || n.input) { g.fillStyle = col; g.fill(p); }
      else { g.lineWidth = 1.5 / k; g.strokeStyle = col; g.stroke(p); g.globalAlpha = dim ? 0.15 : 0.9; }
      if (n.input) { g.lineWidth = 2 / k; g.strokeStyle = T.strong; g.beginPath(); g.arc(0, 0, r + 3, 0, Math.PI * 2); g.stroke(); }
    }
    g.restore();
  }

  function draw() {
    if (!canvas) return;
    const dpr = window.devicePixelRatio || 1;
    const w = canvas.clientWidth, h = canvas.clientHeight;
    if (canvas.width !== Math.round(w * dpr) || canvas.height !== Math.round(h * dpr)) { canvas.width = Math.round(w * dpr); canvas.height = Math.round(h * dpr); }
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, w, h);
    ctx.save();
    ctx.translate(view.x, view.y); ctx.scale(view.k, view.k);

    const focus = selected || null;
    const near = new Set();
    if (focus) { near.add(focus); for (const e of edges) { if (e.a === focus) near.add(e.b); if (e.b === focus) near.add(e.a); } }
    const q = findText;
    const matches = (n) => q && String(n.value || n.label || "").toLowerCase().includes(q);

    for (const e of edges) {
      const strong = (e.a.input || e.a.strong) && (e.b.input || e.b.strong);
      const hot = hover && (e.a === hover || e.b === hover);
      ctx.strokeStyle = hot || (strong && e.a.type === "subject") ? T.strong : T.line;
      ctx.lineWidth = (hot ? 2 : strong ? 1.4 : 1) / view.k;
      ctx.globalAlpha = hot ? 1 : focus ? (e.a === focus || e.b === focus ? 0.95 : 0.08) : strong ? 0.85 : 0.5;
      ctx.beginPath(); ctx.moveTo(e.a.x, e.a.y); ctx.lineTo(e.b.x, e.b.y); ctx.stroke();
    }
    ctx.globalAlpha = 1;
    for (const n of nodes) {
      drawNode(ctx, n, focus && !near.has(n), view.k);
      const r = radius(n);
      if (matches(n)) { ctx.strokeStyle = T.emph; ctx.lineWidth = 2.5 / view.k; ctx.beginPath(); ctx.arc(n.x, n.y, r + 8, 0, Math.PI * 2); ctx.stroke(); }
      if (n === selected || n === hover) { ctx.strokeStyle = T.text; ctx.lineWidth = 2 / view.k; ctx.beginPath(); ctx.arc(n.x, n.y, r + 5, 0, Math.PI * 2); ctx.stroke(); }
    }
    if (layoutMode === "clusters" && groupLabels.length) {
      ctx.font = `600 ${14 / Math.max(0.6, Math.min(1.6, view.k))}px system-ui, sans-serif`; ctx.textAlign = "center"; ctx.textBaseline = "alphabetic"; ctx.fillStyle = T.text2;
      for (const g of groupLabels) ctx.fillText(g.text, g.x, g.y);
      ctx.textAlign = "start";
    }
    ctx.font = `${11 / Math.max(0.7, Math.min(1.4, view.k))}px ui-monospace, Menlo, Consolas, monospace`;
    ctx.textBaseline = "middle";
    for (const n of nodes) {
      const show = n.input || n === hover || n === selected || (near.has(n) && focus) || matches(n) || (view.k > 0.9 && (n.strong || n.deg >= 3)) || view.k > 1.6;
      if (!show) continue;
      const label = n.type === "subject" ? n.label : n.value;
      const text = label.length > 34 ? label.slice(0, 33) + "…" : label;
      const x = n.x + radius(n) + 6;
      // a halo keeps labels legible where they cross lines
      ctx.lineWidth = 3 / view.k; ctx.strokeStyle = T.bg; ctx.lineJoin = "round"; ctx.strokeText(text, x, n.y);
      ctx.fillStyle = T.text; ctx.fillText(text, x, n.y);
    }
    ctx.restore();
    stats();
    drawMini();
  }

  // --------------------------------------------------------------- view ops
  const labelOf = (n) => (n.type === "subject" ? n.label : String(n.value || ""));
  /** Bounding box of the nodes, including the room their labels need on the right. */
  function bounds(list) {
    let x0 = Infinity, y0 = Infinity, x1 = -Infinity, y1 = -Infinity;
    for (const n of list) {
      const r = radius(n) + 6;
      const lw = n.input || n.strong || n.type === "subject" ? Math.min(34, labelOf(n).length) * 6.6 + 10 : 0;
      x0 = Math.min(x0, n.x - r); y0 = Math.min(y0, n.y - r); x1 = Math.max(x1, n.x + r + lw); y1 = Math.max(y1, n.y + r);
    }
    return { x0, y0, x1, y1 };
  }
  function hops() {
    const adj = new Map(nodes.map((n) => [n, []]));
    for (const e of edges) { adj.get(e.a).push(e.b); adj.get(e.b).push(e.a); }
    const root = nodes.find((n) => n.type === "subject") || nodes[0];
    const lvl = new Map([[root, 0]]), par = new Map(), order = [root];
    for (let i = 0; i < order.length; i++) for (const b of adj.get(order[i])) if (!lvl.has(b)) { lvl.set(b, lvl.get(order[i]) + 1); par.set(b, order[i]); order.push(b); }
    let max = 0; for (const v of lvl.values()) max = Math.max(max, v);
    const loose = nodes.filter((n) => !lvl.has(n));
    for (const n of loose) lvl.set(n, max + 1);
    const kids = new Map(nodes.map((n) => [n, []]));
    for (const [c, pr] of par) kids.get(pr).push(c);
    return { lvl, par, root, max: loose.length ? max + 1 : max, order, kids, loose };
  }
  /** mode "all": every node on screen. mode "core": the subject and what is within two hops, at a readable size. */
  function fit(mode, quiet) {
    if (!nodes.length || !canvas) return;
    const w = canvas.clientWidth, h = canvas.clientHeight;
    let list = nodes;
    if (mode === "core") {
      // the subject and the ~45 nodes nearest to it, whatever the layout
      const root = nodes.find((n) => n.type === "subject") || nodes[0];
      list = [...nodes].sort((p, q) => Math.hypot(p.x - root.x, p.y - root.y) - Math.hypot(q.x - root.x, q.y - root.y)).slice(0, 45);
    }
    const b = bounds(list), pad = 28;
    const bw = Math.max(60, b.x1 - b.x0), bh = Math.max(60, b.y1 - b.y0);
    const lo = mode === "core" ? 0.55 : 0.06;
    const k = Math.max(lo, Math.min(1.8, Math.min((w - pad * 2) / bw, (h - pad * 2) / bh)));
    view.k = k; view.x = w / 2 - ((b.x0 + b.x1) / 2) * k; view.y = h / 2 - ((b.y0 + b.y1) / 2) * k;
    coreMode = mode === "core";
    if (!quiet) { touched = false; draw(); }
  }
  /** First framing: everything if it stays legible, else the core around the subject. */
  function autoFit() {
    if (!nodes.length) return;
    const w = canvas.clientWidth, h = canvas.clientHeight, b = bounds(nodes);
    const kAll = Math.min((w - 56) / Math.max(60, b.x1 - b.x0), (h - 56) / Math.max(60, b.y1 - b.y0));
    if (kAll < 0.34 && nodes.length > 40 && layoutMode === "force") { fit("core", true); UI.toast("Large graph: showing the core around the subject. “Fit all” shows everything."); }
    else fit("all", true);
  }
  /** Keep part of the graph on screen whatever the analyst does. */
  function clampView() {
    if (!nodes.length) return;
    const w = canvas.clientWidth, h = canvas.clientHeight, b = bounds(nodes), m = 90;
    const sx0 = b.x0 * view.k + view.x, sx1 = b.x1 * view.k + view.x, sy0 = b.y0 * view.k + view.y, sy1 = b.y1 * view.k + view.y;
    if (sx1 < m) view.x += m - sx1; else if (sx0 > w - m) view.x -= sx0 - (w - m);
    if (sy1 < m) view.y += m - sy1; else if (sy0 > h - m) view.y -= sy0 - (h - m);
  }
  function zoomAt(px, py, f) {
    const k = Math.max(0.05, Math.min(4, view.k * f));
    view.x = px - ((px - view.x) / view.k) * k; view.y = py - ((py - view.y) / view.k) * k; view.k = k;
    touched = true; clampView(); draw();
  }
  const zoom = (f) => zoomAt(canvas.clientWidth / 2, canvas.clientHeight / 2, f);

  // ---------------------------------------------------------------- layouts
  /** Nodes not connected to the subject: a tidy block off to one side (radial: below, tree: under the tree). */
  function placeLoose(list, r, set, byType, y0, flat) {
    if (!list.length) return;
    const sorted = [...list].sort(byType), cols = Math.max(1, Math.ceil(Math.sqrt(sorted.length / 2)));
    sorted.forEach((n, i) => {
      const cx = (i % cols) * 150 - ((cols - 1) * 75), cy = Math.floor(i / cols) * 28;
      if (flat) set(n, cx, y0 + cy); else set(n, cx, r + cy);
    });
  }
  /** Static layouts compute a target for every node; nodes glide there. "force" keeps the live physics. */
  function applyLayout(mode) {
    layoutMode = mode;
    for (const n of nodes) { n.tx = n.ty = null; }
    if (mode === "force") { groupLabels = []; for (const n of nodes) if (n.type !== "subject") n.fixed = false; fitted = false; touched = false; kick(0.8); return; }
    const { lvl, root, order, kids, loose } = hops();
    const set = (n, x, y) => { n.tx = x; n.ty = y; };
    const typeRank = (n) => (n.input ? 0 : n.strong ? 1 : 2);
    const byType = (a, b) => (typeRank(a) - typeRank(b)) || (a.type < b.type ? -1 : a.type > b.type ? 1 : String(a.value).localeCompare(String(b.value)));
    for (const list of kids.values()) list.sort(byType);
    const weight = new Map();
    const wOf = (n) => { if (weight.has(n)) return weight.get(n); const ks = kids.get(n); const w = ks.length ? ks.reduce((t, k) => t + wOf(k), 0) : 1; weight.set(n, w); return w; };
    groupLabels = [];
    if (mode === "radial") {
      // each subtree owns an angular sector sized by its leaves; rings are spaced so no ring is crowded
      const levels = new Map();
      for (const n of order) levels.set(lvl.get(n), (levels.get(lvl.get(n)) || 0) + 1);
      const R = new Map([[0, 0]]), tracks = new Map();
      let prev = 0;
      for (let l = 1; l <= Math.max(...levels.keys(), 1); l++) {
        const cnt = levels.get(l) || 0, t = Math.min(4, Math.max(1, Math.ceil(cnt / 90)));
        tracks.set(l, t);
        prev = Math.max(prev + (l === 1 ? 190 : 140), (cnt * 26) / (2 * Math.PI * t));
        R.set(l, prev);
      }
      const seen = new Map();
      const go = (n, a0, a1) => {
        const l = lvl.get(n), a = (a0 + a1) / 2;
        if (l === 0) set(n, 0, 0);
        else {
          const i = seen.get(l) || 0; seen.set(l, i + 1);
          const t = tracks.get(l), r = R.get(l) + (t > 1 ? ((i % t) - (t - 1) / 2) * 22 : 0);
          set(n, Math.cos(a) * r, Math.sin(a) * r);
        }
        const ks = kids.get(n), tot = ks.reduce((t, k) => t + wOf(k), 0);
        let c = a0;
        for (const k of ks) { const span = ((a1 - a0) * wOf(k)) / tot; go(k, c, c + span); c += span; }
      };
      go(root, -Math.PI / 2, (3 * Math.PI) / 2);
      placeLoose(loose, (R.get(Math.max(...R.keys())) || 300) + 160, set, byType);
    } else if (mode === "tree") {
      // left to right; a hub's many leaves are laid out as a block of columns, not one tall line
      const COLW = 250, ROWH = 26;
      // place(n, x, y0) -> [height, width]; a node's leaves form a block of columns, and many branches are wrapped into several columns too
      const place = (n, x, y0) => {
        const ks = kids.get(n);
        if (!ks.length) { set(n, x, y0 + ROWH / 2); return [ROWH, 0]; }
        const leaves = ks.filter((k) => !kids.get(k).length), branches = ks.filter((k) => kids.get(k).length);
        let y = y0, wid = COLW; const ys = [];
        if (leaves.length) {
          const cols = Math.max(1, Math.ceil(leaves.length / 12)), rows = Math.ceil(leaves.length / cols);
          leaves.forEach((k, i) => set(k, x + COLW + Math.floor(i / rows) * 190, y + (i % rows) * ROWH + ROWH / 2));
          ys.push(y + ROWH / 2, y + (rows - 1) * ROWH + ROWH / 2); y += rows * ROWH + 12;
          wid = COLW + (cols - 1) * 190 + 150;
        }
        if (branches.length) {
          const K = Math.min(6, Math.max(1, Math.ceil(branches.length / 8)));
          const colY = new Array(K).fill(y), colX = new Array(K).fill(0);
          // first pass sizes, masonry second: shortest column gets the next branch
          let cellW = 0;
          const sized = branches.map((b) => { const r = place(b, x + COLW, 0); cellW = Math.max(cellW, r[1] + COLW); return [b, r[0]]; });
          sized.forEach(([b, h], i) => {
            let c = 0; for (let j = 1; j < K; j++) if (colY[j] < colY[c]) c = j;
            const dx = c * cellW, dy = colY[c];
            // re-place at the final spot (cheap: subtrees are small)
            place(b, x + COLW + dx, dy);
            ys.push(b.ty); colY[c] += h + 12; wid = Math.max(wid, COLW + dx + cellW);
          });
          y = Math.max(...colY);
        }
        set(n, x, (Math.min(...ys) + Math.max(...ys)) / 2);
        return [Math.max(ROWH, y - y0 - 12), wid];
      };
      const [h] = place(root, 0, 0);
      const dy = -(h / 2);
      for (const n of order) n.ty += dy;
      if (loose.length) placeLoose(loose, 0, set, byType, h / 2 + 90, true);
    } else if (mode === "circle") {
      // BFS order keeps a node next to what it was found from
      const ring = order.slice(1).concat([...loose].sort(byType)), r = Math.max(240, (ring.length * 26) / (2 * Math.PI));
      set(root, 0, 0);
      ring.forEach((n, i) => { const a = (i / ring.length) * Math.PI * 2 - Math.PI / 2; set(n, Math.cos(a) * r, Math.sin(a) * r); });
    } else if (mode === "clusters") {
      const groups = new Map();
      for (const n of nodes) if (n !== root) (groups.get(n.type) || groups.set(n.type, []).get(n.type)).push(n);
      const order2 = [...groups.entries()].sort((a, b) => b[1].length - a[1].length);
      const gr = order2.map(([, l]) => 26 * Math.sqrt(l.length) + 34);
      const circ = gr.reduce((t, g) => t + g * 2 + 30, 0), Rg = Math.max(300, circ / (2 * Math.PI) + Math.max(0, ...gr) * 0.6);
      set(root, 0, 0);
      let acc = 0;
      order2.forEach(([type, list], gi) => {
        const share = (gr[gi] * 2 + 30) / circ, a = (acc + share / 2) * Math.PI * 2 - Math.PI / 2; acc += share;
        const gx = Math.cos(a) * Rg, gy = Math.sin(a) * Rg;
        list.sort(byType).forEach((n, i) => { const t = i * 2.39996, d = 24 * Math.sqrt(i + 0.5); set(n, gx + Math.cos(t) * d, gy + Math.sin(t) * d); });
        groupLabels.push({ x: gx, y: gy - gr[gi] - 4, text: `${type} · ${list.length}` });
      });
    }
    for (const n of nodes) if (n.fixed && n.type !== "subject") { /* dragged by hand: stays where the analyst put it */ n.tx = n.x; n.ty = n.y; }
    settling = true; fitted = false; touched = false;
    kick(0);
  }

  // ---------------------------------------------------------------- minimap
  const MM = { w: 168, h: 116 };
  function drawMini() {
    if (!mini || mini.hidden) return;
    const dpr = window.devicePixelRatio || 1, W = mini.clientWidth || MM.w, H = mini.clientHeight || MM.h;
    if (mini.width !== Math.round(W * dpr)) { mini.width = Math.round(W * dpr); mini.height = Math.round(H * dpr); }
    mctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    mctx.clearRect(0, 0, W, H);
    if (!nodes.length) return;
    const b = bounds(nodes), bw = Math.max(60, b.x1 - b.x0), bh = Math.max(60, b.y1 - b.y0);
    const w = canvas.clientWidth, h = canvas.clientHeight;
    // include the viewport so it never leaves the map
    const vx0 = -view.x / view.k, vy0 = -view.y / view.k, vx1 = vx0 + w / view.k, vy1 = vy0 + h / view.k;
    const x0 = Math.min(b.x0, vx0), y0 = Math.min(b.y0, vy0), x1 = Math.max(b.x1, vx1), y1 = Math.max(b.y1, vy1);
    const s = Math.min((W - 8) / (x1 - x0), (H - 8) / (y1 - y0)), ox = (W - (x1 - x0) * s) / 2 - x0 * s, oy = (H - (y1 - y0) * s) / 2 - y0 * s;
    mmap = { s, ox, oy };
    if (edges.length < 600) {
      mctx.strokeStyle = T.line; mctx.globalAlpha = 0.45; mctx.lineWidth = 0.6; mctx.beginPath();
      for (const e of edges) { mctx.moveTo(e.a.x * s + ox, e.a.y * s + oy); mctx.lineTo(e.b.x * s + ox, e.b.y * s + oy); }
      mctx.stroke();
    }
    mctx.globalAlpha = 1;
    for (const n of nodes) {
      const sub = n.type === "subject";
      mctx.fillStyle = sub ? T.strong : n.strong || n.input ? T.text2 : T.text4;
      const r = sub ? 3 : n.strong || n.input ? 1.8 : 1.2;
      mctx.fillRect(n.x * s + ox - r, n.y * s + oy - r, r * 2, r * 2);
    }
    mctx.strokeStyle = T.strong; mctx.lineWidth = 1.5;
    mctx.strokeRect(vx0 * s + ox, vy0 * s + oy, (vx1 - vx0) * s, (vy1 - vy0) * s);
  }
  function miniMove(ev) {
    if (!mmap) return;
    const r = mini.getBoundingClientRect(), mx = ev.clientX - r.left, my = ev.clientY - r.top;
    const wx = (mx - mmap.ox) / mmap.s, wy = (my - mmap.oy) / mmap.s;
    view.x = canvas.clientWidth / 2 - wx * view.k; view.y = canvas.clientHeight / 2 - wy * view.k;
    touched = true; clampView(); draw();
  }
  function setMini(on) {
    showMini = on;
    if (mini) { mini.hidden = !on; if (on) drawMini(); }
    const b = document.getElementById("graphMiniBtn");
    if (b) b.setAttribute("aria-pressed", String(!!on));
  }

  // -------------------------------------------------------------- fullscreen
  function isFull() { return document.fullscreenElement === shell || shell.classList.contains("is-full"); }
  function toggleFull() {
    if (isFull()) { if (document.fullscreenElement) document.exitFullscreen(); shell.classList.remove("is-full"); }
    else if (shell.requestFullscreen) shell.requestFullscreen().catch(() => shell.classList.add("is-full"));
    else shell.classList.add("is-full");
    setTimeout(() => { if (!touched) fitted = false; kick(0); draw(); }, 200);
  }

  function focusNode(n) {
    selected = n;
    const w = canvas.clientWidth, h = canvas.clientHeight;
    view.k = Math.max(view.k, 1.2); view.x = w / 2 - n.x * view.k; view.y = h / 2 - n.y * view.k;
    touched = true; clampView(); draw();
  }

  // ----------------------------------------------------------------- export
  function exportPNG() {
    const out = document.createElement("canvas");
    out.width = canvas.width; out.height = canvas.height;
    const g = out.getContext("2d");
    g.fillStyle = T.bg; g.fillRect(0, 0, out.width, out.height);
    g.drawImage(canvas, 0, 0);
    out.toBlob((b) => {
      if (!b) return UI.toast("Could not render the picture");
      const url = URL.createObjectURL(b);
      const a = el("a", { href: url, download: "argus_graph_" + UI.S.id + ".png" });
      document.body.append(a); a.click(); a.remove();
      setTimeout(() => URL.revokeObjectURL(url), 2000);
    });
  }
  function exportSVG() {
    if (!nodes.length) return UI.toast("Nothing to export");
    let x0 = Infinity, y0 = Infinity, x1 = -Infinity, y1 = -Infinity;
    for (const n of nodes) { x0 = Math.min(x0, n.x - 20); y0 = Math.min(y0, n.y - 20); x1 = Math.max(x1, n.x + 220); y1 = Math.max(y1, n.y + 20); }
    const root = UI.svg("svg", { xmlns: "http://www.w3.org/2000/svg", viewBox: `${x0} ${y0} ${x1 - x0} ${y1 - y0}`, width: Math.round(x1 - x0), height: Math.round(y1 - y0) });
    root.append(UI.svg("rect", { x: x0, y: y0, width: x1 - x0, height: y1 - y0, fill: T.bg }));
    for (const e of edges) root.append(UI.svg("line", { x1: e.a.x, y1: e.a.y, x2: e.b.x, y2: e.b.y, stroke: e.a.type === "subject" ? T.strong : T.line, "stroke-width": 1 }));
    for (const n of nodes) {
      const r = radius(n);
      if (n.type === "subject") {
        root.append(UI.svg("circle", { cx: n.x, cy: n.y, r, fill: T.accent, stroke: T.strong, "stroke-width": 2.5 }));
      } else {
        const col = grays()[levelOf(n)];
        const filled = n.strong || n.input;
        root.append(UI.svg("path", { d: UI.shapePath(glyph(n.type)[0], r), transform: `translate(${n.x} ${n.y})`, fill: filled ? col : "none", stroke: col, "stroke-width": 1.5 }));
      }
      const label = n.type === "subject" ? n.label : n.value;
      if (n.input || n.strong || n.deg >= 3) {
        const t = UI.svg("text", { x: n.x + r + 6, y: n.y + 4, fill: T.text, "font-family": "monospace", "font-size": 11 });
        t.textContent = label.length > 34 ? label.slice(0, 33) + "…" : label;
        root.append(t);
      }
    }
    UI.download("argus_graph_" + UI.S.id + ".svg", '<?xml version="1.0" encoding="UTF-8"?>\n' + new XMLSerializer().serializeToString(root), "image/svg+xml");
  }

  // ------------------------------------------------------- connected nodes
  /** Called by the entity drawer: list what the node is linked to. */
  UI.graphNeighbours = function graphNeighbours(ent, body) {
    const n = byId.get(key(ent.type, ent.value));
    if (!n) return;
    const list = [];
    for (const e of edges) { if (e.a === n) list.push([e.b, e.label]); else if (e.b === n) list.push([e.a, e.label]); }
    if (!list.length) return;
    const chips = el("div", { class: "chips" });
    for (const [m, via] of list) {
      const b = el("button", { type: "button", class: "chip", title: "via " + via }, UI.icon(UI.type(m.type).icon), el("span", { class: "k", text: m.type }), el("span", { class: "chip-v", text: (m.type === "subject" ? m.label : m.value).slice(0, 48) }));
      b.addEventListener("click", () => { selected = m; draw(); if (m.type !== "subject") UI.showEntity({ type: m.type, value: m.value, strong: m.strong, source: m.source }); });
      chips.append(b);
    }
    body.append(UI.section(`Connected to (${list.length})`, chips));
  };

  // ------------------------------------------------------------ interaction
  const toWorld = (px, py) => [(px - view.x) / view.k, (py - view.y) / view.k];
  function pick(px, py) {
    const [x, y] = toWorld(px, py);
    let best = null, bd = Infinity;
    for (const n of nodes) {
      const d = Math.hypot(n.x - x, n.y - y);
      if (d <= radius(n) + 6 / view.k && d < bd) { best = n; bd = d; }
    }
    return best;
  }
  const local = (ev) => { const r = canvas.getBoundingClientRect(); return [ev.clientX - r.left, ev.clientY - r.top]; };

  function bindCanvas() {
    const pinchMid = () => { const [a, b] = [...ptrs.values()]; return [(a[0] + b[0]) / 2, (a[1] + b[1]) / 2, Math.hypot(a[0] - b[0], a[1] - b[1]) || 1]; };
    canvas.addEventListener("pointerdown", (ev) => {
      const [px, py] = local(ev);
      ptrs.set(ev.pointerId, [px, py]);
      canvas.setPointerCapture(ev.pointerId);
      if (ptrs.size === 2) {
        drag = null; canvas.classList.remove("drag");
        const [mx, my, d] = pinchMid();
        pinch = { d, k: view.k, wx: (mx - view.x) / view.k, wy: (my - view.y) / view.k };
        return;
      }
      const n = pick(px, py);
      drag = { n, sx: px, sy: py, ox: view.x, oy: view.y, moved: false };
      if (!n) canvas.classList.add("drag");
      kick(0.3);
    });
    canvas.addEventListener("pointermove", (ev) => {
      const [px, py] = local(ev);
      if (ptrs.has(ev.pointerId)) ptrs.set(ev.pointerId, [px, py]);
      if (pinch && ptrs.size === 2) {
        const [mx, my, d] = pinchMid();
        const k = Math.max(0.05, Math.min(4, pinch.k * (d / pinch.d)));
        view.k = k; view.x = mx - pinch.wx * k; view.y = my - pinch.wy * k; touched = true; clampView(); draw();
        return;
      }
      if (drag) {
        if (Math.hypot(px - drag.sx, py - drag.sy) > 3) { drag.moved = true; touched = true; }
        if (drag.n && drag.moved) {
          const [x, y] = toWorld(px, py); drag.n.x = x; drag.n.y = y; drag.n.fixed = true; drag.n.vx = drag.n.vy = 0;
          if (layoutMode !== "force") { drag.n.tx = x; drag.n.ty = y; }
          kick(layoutMode === "force" ? 0.3 : 0);
        } else if (!drag.n) { view.x = drag.ox + (px - drag.sx); view.y = drag.oy + (py - drag.sy); clampView(); draw(); }
        return;
      }
      const n = pick(px, py);
      if (n !== hover) { hover = n; draw(); }
      if (n) {
        tip.hidden = false;
        tip.textContent = n.type === "subject" ? "subject" : `${n.type}: ${n.value}\n${n.strong ? "strong lead" : "weak lead"}${n.source ? " · from " + n.source : ""}`;
        tip.style.left = Math.max(4, Math.min(px + 14, canvas.clientWidth - 260)) + "px"; tip.style.top = Math.min(py + 14, canvas.clientHeight - 60) + "px";
      } else tip.hidden = true;
    });
    const up = (ev) => {
      ptrs.delete(ev.pointerId);
      if (pinch) { if (ptrs.size < 2) pinch = null; drag = null; canvas.classList.remove("drag"); return; }
      if (drag && drag.n && !drag.moved) {
        selected = drag.n;
        if (drag.n.type !== "subject") UI.showEntity({ type: drag.n.type, value: drag.n.value, strong: drag.n.strong, source: drag.n.source });
      } else if (drag && !drag.n && !drag.moved) selected = null;
      canvas.classList.remove("drag");
      drag = null; draw();
    };
    canvas.addEventListener("pointerup", up);
    canvas.addEventListener("pointercancel", up);
    canvas.addEventListener("pointerleave", () => { tip.hidden = true; if (hover) { hover = null; draw(); } });
    canvas.addEventListener("dblclick", (ev) => { const n = pick(...local(ev)); if (n && n.type !== "subject") { n.fixed = false; if (layoutMode === "force") kick(0.5); } else if (!n) zoomAt(...local(ev), 1.6); });
    canvas.addEventListener("wheel", (ev) => {
      ev.preventDefault();
      const [px, py] = local(ev);
      zoomAt(px, py, Math.exp(-ev.deltaY * (ev.ctrlKey ? 0.01 : 0.0016)));
    }, { passive: false });
    canvas.addEventListener("keydown", (e) => {
      const step = 70;
      if (e.key === "+" || e.key === "=") { zoom(1.25); e.preventDefault(); }
      else if (e.key === "-") { zoom(1 / 1.25); e.preventDefault(); }
      else if (e.key === "0") { fit("all"); e.preventDefault(); }
      else if (e.key === "f" || e.key === "F") { toggleFull(); e.preventDefault(); }
      else if (e.key === "ArrowLeft") { view.x += step; touched = true; clampView(); draw(); e.preventDefault(); }
      else if (e.key === "ArrowRight") { view.x -= step; touched = true; clampView(); draw(); e.preventDefault(); }
      else if (e.key === "ArrowUp") { view.y += step; touched = true; clampView(); draw(); e.preventDefault(); }
      else if (e.key === "ArrowDown") { view.y -= step; touched = true; clampView(); draw(); e.preventDefault(); }
    });
    new ResizeObserver(() => { if (!touched && fitted) fit(coreMode ? "core" : "all", true); draw(); }).observe(canvas);
    document.addEventListener("fullscreenchange", () => { if (!shell) return; shell.classList.toggle("is-full", false); if (!touched) fitted = false; kick(0); draw(); });
    document.addEventListener("keydown", (e) => { if (e.key === "Escape" && shell && shell.classList.contains("is-full")) { shell.classList.remove("is-full"); if (!touched) fitted = false; kick(0); } });
  }

  // ------------------------------------------------------------------ panel
  function buildPanel(root) {
    const find = el("input", { type: "search", placeholder: "Find a node…", "aria-label": "Find a node in the graph", autocomplete: "off" });
    find.addEventListener("input", () => { findText = find.value.trim().toLowerCase(); draw(); });
    find.addEventListener("keydown", (e) => {
      if (e.key !== "Enter" || !findText) return;
      const hit = nodes.find((n) => String(n.value || n.label || "").toLowerCase().includes(findText));
      if (hit) { focusNode(hit); UI.toast("Focused: " + UI.clip(hit.value || hit.label, 40)); } else UI.toast("No node matches");
    });
    const shade = el("select", { "aria-label": "Shade nodes by" }, el("option", { value: "type", text: "Shade by type" }), el("option", { value: "collector", text: "Shade by collector" }));
    shade.addEventListener("change", () => { shadeBy = shade.value; legend(); draw(); });
    const weak = el("input", { type: "checkbox", id: "graphWeak", checked: "" });
    weak.addEventListener("change", () => { showWeak = weak.checked; build(); kick(0.7); });
    const layoutSel = el("select", { "aria-label": "Layout" },
      ...[["force", "Layout: organic (live)"], ["radial", "Layout: rings by hops"], ["tree", "Layout: tree, left to right"], ["circle", "Layout: circle"], ["clusters", "Layout: grouped by type"]].map(([v, t]) => el("option", { value: v, text: t })));
    layoutSel.addEventListener("change", () => applyLayout(layoutSel.value));
    const miniBtn = UI.btn("Map", { cls: "lbl-hide", icon: "map", title: "Show or hide the minimap", on: () => setMini(!(mini && !mini.hidden)) });
    miniBtn.id = "graphMiniBtn";
    const bar = el("div", { class: "graph-bar" },
      el("div", { class: "searchbox" }, UI.icon("search"), find), shade, layoutSel,
      el("label", { class: "check sm", for: "graphWeak" }, weak, el("span", { text: "weak leads" })),
      el("span", { class: "grow" }),
      el("div", { class: "btn-group", role: "group", "aria-label": "View" },
        UI.btn("", { icon: "minus", cls: "icon-btn", title: "Zoom out ( - )", on: () => zoom(1 / 1.25) }),
        UI.btn("", { icon: "plus", cls: "icon-btn", title: "Zoom in ( + )", on: () => zoom(1.25) }),
        UI.btn("Fit all", { icon: "fit", title: "Put every node on screen ( 0 )", on: () => fit("all") }),
        UI.btn("Core", { icon: "user", title: "Zoom to the subject and what is within two hops, at a readable size", on: () => fit("core") }),
        miniBtn,
        UI.btn("", { icon: "expand", cls: "icon-btn", title: "Full screen ( f )", on: toggleFull }),
        UI.btn("Re-layout", { cls: "lbl-hide", icon: "graph", title: "Release pinned nodes and settle the layout again", on: () => { nodes.forEach((n) => { if (n.type !== "subject") n.fixed = false; }); applyLayout(layoutMode); } })),
      el("div", { class: "btn-group", role: "group", "aria-label": "Export" },
        UI.btn("PNG", { cls: "lbl-hide", icon: "download", title: "Save the current view as a PNG picture", on: exportPNG }),
        UI.btn("SVG", { cls: "lbl-hide", icon: "download", title: "Save the whole graph as an SVG drawing", on: exportSVG })));
    canvas = el("canvas", { id: "graph", tabindex: "0", role: "img", "aria-label": "Relationship graph of discovered entities. The intelligence tree lists the same items as text." });
    tip = el("div", { class: "graph-tip", hidden: true });
    mini = el("canvas", { class: "graph-mini", "aria-hidden": "true", hidden: true });
    mini.addEventListener("pointerdown", (ev) => { mini.setPointerCapture(ev.pointerId); miniMove(ev); mini.dataset.down = "1"; });
    mini.addEventListener("pointermove", (ev) => { if (mini.dataset.down) miniMove(ev); });
    mini.addEventListener("pointerup", () => { delete mini.dataset.down; });
    mctx = mini.getContext("2d");
    shell = el("div", { class: "graph-shell" }, bar, el("div", { id: "graphLegend", class: "chips legend" }),
      el("div", { class: "graph-wrap" }, canvas, mini, tip, el("div", { class: "graph-stats num", id: "graphStats" })));
    root.append(shell,
      el("p", { class: "hint pad", text: "Edges show where a lead came from: the subject → what the first pass found → what following those leads found. Drag to pan, wheel or pinch to zoom, arrows to move, select a node for details, double-click a node to release it or empty space to zoom in. Pick another layout to see the same graph as rings, a tree, a circle or groups; pinned nodes stay where you put them." }));
    ctx = canvas.getContext("2d");
    bindCanvas();
  }

  function update() {
    const root = $("tab-graph");
    if (!root.querySelector("canvas")) {
      nodes = []; edges = []; byId = new Map(); view = { x: 0, y: 0, k: 1 }; hover = selected = null; fitted = false; touched = false; layoutMode = "force"; settling = false; coreMode = false; showMini = null;
      buildPanel(root);
    }
    readTokens();
    const before = nodes.length;
    build();
    if (showMini === null) setMini(nodes.length > 40 && canvas.clientWidth > 560);
    if (layoutMode !== "force" && nodes.length !== before) applyLayout(layoutMode);
    else kick(nodes.length !== before ? 0.7 : 0.15);
    if (nodes.length !== before) fitted = touched; // new nodes: re-frame unless the analyst has taken control of the view
  }

  UI.views.graph = update;
  UI.graphRedraw = () => { if (canvas) { readTokens(); draw(); } };
})();
