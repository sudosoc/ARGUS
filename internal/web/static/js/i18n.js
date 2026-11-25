"use strict";

// Copyright (C) 2026 Seif, SUDOSOC. SPDX-License-Identifier: AGPL-3.0-only

// Interface language (English or Arabic). English is the source language of every string in the UI; in
// Arabic mode a translator walks the page, replaces text and tooltips found in the dictionary (i18n-ar.js)
// and keeps doing so as the page changes, and the layout switches to right-to-left. Data coming from
// sources (names, URLs, findings) is never translated, only the interface's own words.
// The choice is stored in this browser; changing it reloads the page.
(function () {
  window.UI = window.UI || {};
  const KEY = "argus_lang";
  let lang = "en";
  try { lang = localStorage.getItem(KEY) === "ar" ? "ar" : "en"; } catch (e) { /* storage unavailable */ }
  UI.lang = lang;
  UI.setLang = (l) => { try { localStorage.setItem(KEY, l === "ar" ? "ar" : "en"); } catch (e) { /* ignore */ } location.reload(); };
  if (lang !== "ar") return;

  const root = document.documentElement;
  root.lang = "ar";
  root.dir = "rtl";

  const exact = new Map();
  const tmpl = [];
  for (const [k, v] of Object.entries(UI.AR || {})) {
    if (k.includes("{}")) {
      const re = new RegExp("^" + k.split("{}").map((s) => s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")).join("(.+?)") + "$");
      tmpl.push([re, v]);
    } else exact.set(k, v);
  }
  // longer templates first so "{} of {} findings" wins over "{} of {}"
  tmpl.sort((a, b) => b[0].source.length - a[0].source.length);

  function one(s) {
    if (exact.has(s)) return exact.get(s);
    for (const [re, v] of tmpl) {
      const m = re.exec(s);
      if (m) { let i = 1; return v.replace(/\{\}/g, () => m[i++] ?? ""); }
    }
    return null;
  }
  /** Translate a string, or return null when nothing in it is known. */
  function tr(s) {
    const r = one(s);
    if (r != null) return r;
    if (s.includes(" · ")) {
      const parts = s.split(" · ");
      let hit = false;
      const out = parts.map((p) => { const t = one(p.trim()); if (t != null) { hit = true; return t; } return p; });
      if (hit) return out.join(" · ");
    }
    return null;
  }
  UI.tr = (s) => { const r = tr(String(s).trim()); return r == null ? s : r; };

  const SKIP = new Set(["SCRIPT", "STYLE", "TEXTAREA", "CODE", "PRE", "CANVAS", "SVG", "svg"]);
  const ATTRS = ["title", "placeholder", "aria-label", "alt"];
  const skipEl = (e) => e && e.closest && e.closest("[translate=no], .rawout, .rawcmd, .report-doc");

  function text(n) {
    const v = n.nodeValue;
    if (!v || !/[A-Za-z]/.test(v)) return;
    const t = v.trim();
    if (!t) return;
    const p = n.parentNode;
    const r = p && p.classList && p.classList.contains("k") && UI.AR_K && UI.AR_K[t.toLowerCase()] ? UI.AR_K[t.toLowerCase()] : tr(t);
    if (r != null && r !== t) { n.nodeValue = v.replace(t, r); if (p && p.classList) p.classList.add("ar"); }
  }
  function attrs(e) {
    for (const a of ATTRS) {
      if (!e.hasAttribute(a)) continue;
      const v = e.getAttribute(a), r = tr(v.trim());
      if (r != null && r !== v) e.setAttribute(a, r);
    }
  }
  function walk(n) {
    if (n.nodeType === 3) { if (!skipEl(n.parentNode)) text(n); return; }
    if (n.nodeType !== 1 || SKIP.has(n.tagName) || skipEl(n)) return;
    attrs(n);
    for (let c = n.firstChild; c; c = c.nextSibling) walk(c);
  }

  function start() {
    walk(document.body);
    new MutationObserver((muts) => {
      for (const m of muts) {
        if (m.type === "childList") m.addedNodes.forEach(walk);
        else if (m.type === "characterData") { if (!skipEl(m.target.parentNode)) text(m.target); }
        else if (m.type === "attributes") attrs(m.target);
      }
    }).observe(document.body, { childList: true, subtree: true, characterData: true, attributes: true, attributeFilter: ATTRS });
    document.title = UI.tr(document.title);
  }
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", start); else start();
})();
