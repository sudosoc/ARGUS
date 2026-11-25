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
  const MAX_NODES = 320;

  let canvas = null, ctx = null, tip = null;
  let nodes = [], edges = [], byId = new Map();
  let view = { x: 0, y: 0, k: 1 };
  let alpha = 1, raf = null, hover = null, selected = null, drag = null, showWeak = true, fitted = false, touched = false;
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
      if (n.type === "subject") { n.x = w / 2; n.y = h / 2; n.fixed = true; }
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
    $("graphStats").textContent = `${nodes.length} nodes · ${edges.length} links` + (capped ? " · strongest shown" : "");
  }

  // ---------------------------------------------------------------- physics
  function step() {
    const n = nodes.length;
    for (let i = 0; i < n; i++) {
      const a = nodes[i];
      for (let j = i + 1; j < n; j++) {
        const b = nodes[j];
        let dx = a.x - b.x, dy = a.y - b.y, d2 = dx * dx + dy * dy;
        if (d2 < 0.01) { dx = Math.random() - 0.5; dy = Math.random() - 0.5; d2 = 0.5; }
        if (d2 > 90000) continue;
        const f = (2600 * alpha) / d2, d = Math.sqrt(d2);
        dx = (dx / d) * f; dy = (dy / d) * f;
        a.vx += dx; a.vy += dy; b.vx -= dx; b.vy -= dy;
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
      a.x += Math.max(-30, Math.min(30, a.vx)); a.y += Math.max(-30, Math.min(30, a.vy));
    }
    alpha *= 0.985;
  }

  function loop() {
    raf = null;
    if (alpha > 0.02) { step(); step(); }
    // Frame the whole graph once the layout has mostly settled, unless the analyst already moved the view.
    if (!fitted && !touched && alpha < 0.25) { fit(true); fitted = true; }
    draw();
    if (alpha > 0.02 || drag) raf = requestAnimationFrame(loop);
  }
  const kick = (a) => { alpha = Math.max(alpha, a); if (!raf) raf = requestAnimationFrame(loop); };

  // ---------------------------------------------------------------- drawing
  const radius = (n) => (n.type === "subject" ? 13 : n.input ? 9 : 4.5 + Math.min(7, Math.sqrt(n.deg) * 2));
  const pathCache = new Map();
  const pathFor = (shape, r) => { const k = shape + Math.round(r * 2); let p = pathCache.get(k); if (!p) { p = new Path2D(UI.shapePath(shape, r)); pathCache.set(k, p); } return p; };

  function drawNode(g, n, dim, k) {
    const r = radius(n);
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
  }

  // --------------------------------------------------------------- view ops
  function fit(quiet) {
    if (!nodes.length) return;
    const w = canvas.clientWidth, h = canvas.clientHeight;
    let x0 = Infinity, y0 = Infinity, x1 = -Infinity, y1 = -Infinity;
    for (const n of nodes) { x0 = Math.min(x0, n.x); y0 = Math.min(y0, n.y); x1 = Math.max(x1, n.x); y1 = Math.max(y1, n.y); }
    const pad = 60, bw = Math.max(40, x1 - x0), bh = Math.max(40, y1 - y0);
    const k = Math.max(0.3, Math.min(2, Math.min((w - pad * 2) / bw, (h - pad * 2) / bh)));
    view.k = k; view.x = w / 2 - ((x0 + x1) / 2) * k; view.y = h / 2 - ((y0 + y1) / 2) * k;
    if (!quiet) { touched = false; draw(); }
  }
  function zoom(f) {
    const w = canvas.clientWidth, h = canvas.clientHeight;
    const k = Math.max(0.2, Math.min(4, view.k * f));
    view.x = w / 2 - ((w / 2 - view.x) / view.k) * k; view.y = h / 2 - ((h / 2 - view.y) / view.k) * k; view.k = k;
    touched = true; draw();
  }
  function focusNode(n) {
    selected = n;
    const w = canvas.clientWidth, h = canvas.clientHeight;
    view.k = Math.max(view.k, 1.2); view.x = w / 2 - n.x * view.k; view.y = h / 2 - n.y * view.k;
    touched = true; draw();
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
    for (const [m, via] of list.slice(0, 40)) {
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
    canvas.addEventListener("pointerdown", (ev) => {
      const [px, py] = local(ev);
      const n = pick(px, py);
      canvas.setPointerCapture(ev.pointerId);
      drag = { n, sx: px, sy: py, ox: view.x, oy: view.y, moved: false };
      if (!n) canvas.classList.add("drag");
      kick(0.3);
    });
    canvas.addEventListener("pointermove", (ev) => {
      const [px, py] = local(ev);
      if (drag) {
        if (Math.hypot(px - drag.sx, py - drag.sy) > 3) { drag.moved = true; touched = true; }
        if (drag.n && drag.moved) { const [x, y] = toWorld(px, py); drag.n.x = x; drag.n.y = y; drag.n.fixed = true; drag.n.vx = drag.n.vy = 0; kick(0.3); }
        else if (!drag.n) { view.x = drag.ox + (px - drag.sx); view.y = drag.oy + (py - drag.sy); draw(); }
        return;
      }
      const n = pick(px, py);
      if (n !== hover) { hover = n; draw(); }
      if (n) {
        tip.hidden = false;
        tip.textContent = n.type === "subject" ? "subject" : `${n.type}: ${n.value}\n${n.strong ? "strong lead" : "weak lead"}${n.source ? " · from " + n.source : ""}`;
        tip.style.left = Math.min(px + 14, canvas.clientWidth - 260) + "px"; tip.style.top = py + 14 + "px";
      } else tip.hidden = true;
    });
    canvas.addEventListener("pointerup", () => {
      if (drag && drag.n && !drag.moved) {
        selected = drag.n;
        if (drag.n.type !== "subject") UI.showEntity({ type: drag.n.type, value: drag.n.value, strong: drag.n.strong, source: drag.n.source });
      } else if (drag && !drag.n && !drag.moved) selected = null;
      canvas.classList.remove("drag");
      drag = null; draw();
    });
    canvas.addEventListener("pointerleave", () => { tip.hidden = true; if (hover) { hover = null; draw(); } });
    canvas.addEventListener("dblclick", (ev) => { const n = pick(...local(ev)); if (n && n.type !== "subject") { n.fixed = false; kick(0.5); } });
    canvas.addEventListener("wheel", (ev) => {
      ev.preventDefault();
      const [px, py] = local(ev);
      const k = Math.max(0.2, Math.min(4, view.k * (ev.deltaY < 0 ? 1.12 : 1 / 1.12)));
      view.x = px - ((px - view.x) / view.k) * k; view.y = py - ((py - view.y) / view.k) * k; view.k = k;
      touched = true; draw();
    }, { passive: false });
    canvas.addEventListener("keydown", (e) => {
      if (e.key === "+" || e.key === "=") { zoom(1.25); e.preventDefault(); }
      else if (e.key === "-") { zoom(1 / 1.25); e.preventDefault(); }
      else if (e.key === "0") { fit(); e.preventDefault(); }
    });
    new ResizeObserver(() => draw()).observe(canvas);
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
    const bar = el("div", { class: "graph-bar" },
      el("div", { class: "searchbox" }, UI.icon("search"), find), shade,
      el("label", { class: "check sm", for: "graphWeak" }, weak, el("span", { text: "weak leads" })),
      el("span", { class: "grow" }),
      el("div", { class: "btn-group", role: "group", "aria-label": "View" },
        UI.btn("", { icon: "minus", cls: "icon-btn", title: "Zoom out ( - )", on: () => zoom(1 / 1.25) }),
        UI.btn("", { icon: "plus", cls: "icon-btn", title: "Zoom in ( + )", on: () => zoom(1.25) }),
        UI.btn("Fit", { icon: "fit", title: "Fit the whole graph on screen ( 0 )", on: () => fit() }),
        UI.btn("Re-layout", { icon: "graph", title: "Release pinned nodes and settle the layout again", on: () => { nodes.forEach((n) => { if (n.type !== "subject") n.fixed = false; }); fitted = false; touched = false; kick(0.9); } })),
      el("div", { class: "btn-group", role: "group", "aria-label": "Export" },
        UI.btn("PNG", { icon: "download", title: "Save the current view as a PNG picture", on: exportPNG }),
        UI.btn("SVG", { icon: "download", title: "Save the whole graph as an SVG drawing", on: exportSVG })));
    canvas = el("canvas", { id: "graph", tabindex: "0", role: "img", "aria-label": "Relationship graph of discovered entities. The intelligence tree lists the same items as text." });
    tip = el("div", { class: "graph-tip", hidden: true });
    root.append(bar, el("div", { id: "graphLegend", class: "chips legend" }),
      el("div", { class: "graph-wrap" }, canvas, tip, el("div", { class: "graph-stats num", id: "graphStats" })),
      el("p", { class: "hint pad", text: "Edges show where a lead came from: the subject → what the first pass found → what following those leads found. Drag to pan, wheel to zoom, select a node for details, double-click to release a pinned node." }));
    ctx = canvas.getContext("2d");
    bindCanvas();
  }

  function update() {
    const root = $("tab-graph");
    if (!root.querySelector("canvas")) {
      nodes = []; edges = []; byId = new Map(); view = { x: 0, y: 0, k: 1 }; hover = selected = null; fitted = false; touched = false;
      buildPanel(root);
    }
    readTokens();
    const before = nodes.length;
    build();
    kick(nodes.length !== before ? 0.7 : 0.15);
    if (nodes.length !== before) fitted = touched; // new nodes: re-frame unless the analyst has taken control of the view
  }

  UI.views.graph = update;
  UI.graphRedraw = () => { if (canvas) { readTokens(); draw(); } };
})();
