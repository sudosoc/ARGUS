"use strict";

// Copyright (C) 2026 Seif, SUDOSOC. SPDX-License-Identifier: AGPL-3.0-only

// SUDOSOC ARGUS front end: shared foundation. Everything hangs off the global
// `UI` object because there is no bundler; scripts are loaded in order from
// index.html.
//
// Security rule for the whole front end: scraped text (titles, URLs, bios,
// snippets) is only ever inserted with textContent / setAttribute, never with
// innerHTML. Links are limited to http(s) and carry rel="noopener noreferrer".
window.UI = window.UI || {};
(function () {
  const $ = (id) => document.getElementById(id);
  UI.$ = $;

  // The Go server requires this exact value on every state-changing request.
  const HDR = { "Content-Type": "application/json", "X-Requested-With": "argus" };

  // ------------------------------------------------------------------ DOM
  UI.el = function el(tag, props = {}, ...kids) {
    const e = document.createElement(tag);
    for (const [k, v] of Object.entries(props || {})) {
      if (v == null || v === false) continue;
      if (k === "class") e.className = v;
      else if (k === "text") e.textContent = v;
      else if (k === "style") e.style.cssText = v; // CSSOM is allowed by the CSP; a style="" attribute is not
      else e.setAttribute(k, v === true ? "" : v);
    }
    for (const k of kids.flat()) if (k != null && k !== false) e.append(k);
    return e;
  };
  const el = UI.el;

  const SVGNS = "http://www.w3.org/2000/svg";
  /** Icon from the sprite defined once in index.html. */
  UI.icon = function icon(name, cls) {
    const s = document.createElementNS(SVGNS, "svg");
    s.setAttribute("class", "ic" + (cls ? " " + cls : ""));
    s.setAttribute("aria-hidden", "true");
    const u = document.createElementNS(SVGNS, "use");
    u.setAttribute("href", "#i-" + name);
    s.append(u);
    return s;
  };

  /** A button with an icon and optional label. */
  UI.btn = function btn(label, opts = {}) {
    const b = el("button", { type: "button", class: "btn" + (opts.cls ? " " + opts.cls : ""), title: opts.title, "aria-label": opts.aria || (opts.icon && !label ? opts.title : null) });
    if (opts.icon) b.append(UI.icon(opts.icon));
    if (label) b.append(el("span", { text: label }));
    if (opts.on) b.addEventListener("click", opts.on);
    return b;
  };

  // ------------------------------------------------------------- network
  UI.api = async function api(path, opts = {}) {
    const r = await fetch(path, { credentials: "same-origin", ...opts, headers: { ...HDR, ...(opts.headers || {}) } });
    const data = await r.json().catch(() => ({}));
    if (!r.ok) throw new Error(data.error || `HTTP ${r.status}`);
    return data;
  };

  // --------------------------------------------------------- preferences
  UI.store = {
    get(k, d) { try { const v = localStorage.getItem("argus_" + k); return v === null ? d : v; } catch (e) { return d; } },
    set(k, v) { try { localStorage.setItem("argus_" + k, v); } catch (e) { /* private mode */ } },
  };
  // sessionStorage remembers the open case across a reload of this tab only.
  UI.session = {
    get(k) { try { return sessionStorage.getItem("argus_" + k); } catch (e) { return null; } },
    set(k, v) { try { sessionStorage.setItem("argus_" + k, v); } catch (e) { /* ignore */ } },
  };

  // ------------------------------------------------------------- helpers
  UI.safeHref = (u) => { try { const x = new URL(u); return x.protocol === "http:" || x.protocol === "https:" ? x.href : null; } catch { return null; } };
  UI.pct = (c) => Math.round((c || 0) * 100);
  UI.tier = (p) => (p >= 80 ? "hi" : p >= 50 ? "mid" : "lo");
  UI.TIER_LABEL = { hi: "High", mid: "Medium", lo: "Low" };
  UI.clip = (s, n) => (s && s.length > n ? s.slice(0, n) + "…" : s || "");
  UI.debounce = (fn, ms) => { let t; return (...a) => { clearTimeout(t); t = setTimeout(() => fn(...a), ms); }; };
  UI.n = (x) => Number(x || 0).toLocaleString("en-US");
  UI.humanize = (s) => { const t = String(s || "other").replace(/[-_]+/g, " ").trim(); return t.charAt(0).toUpperCase() + t.slice(1); };
  UI.host = (u) => { try { return new URL(u).hostname.replace(/^www\./, ""); } catch { return ""; } };
  UI.dur = (ms) => { const s = Math.max(0, Math.round(ms / 1000)); return s < 60 ? s + "s" : Math.floor(s / 60) + "m " + String(s % 60).padStart(2, "0") + "s"; };
  UI.when = (iso) => { const d = new Date(iso); return isNaN(d) ? "" : d.toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" }); };
  UI.targetLabel = (t) => (t && (t.full_name || t.username || t.alias || t.email || t.domain || t.company || t.ip || t.asn || t.wallet || t.phone || t.coords)) || (t && t.files && t.files.length ? "Uploaded files" : "");

  /** Confidence / score meter: numeric figure sits next to the bar, so meaning never rests on fill alone. */
  UI.meter = function meter(p, withNum) {
    const fill = el("span", { class: "meter-fill" });
    fill.style.width = Math.max(0, Math.min(100, p)) + "%";
    const m = el("span", { class: "meter t-" + UI.tier(p), role: "img", "aria-label": p + " percent" }, fill);
    if (!withNum) return m;
    return el("span", { class: "meter-wrap" }, m, el("span", { class: "num", text: p + "%" }));
  };

  /** Status of a check as icon + word (never colour alone). */
  const STATUS = {
    found: ["check", "Found"],
    uncertain: ["uncertain", "Uncertain"],
    error: ["alert", "Error"],
    not_found: ["minus", "Not found"],
  };
  UI.statusBadge = function statusBadge(status, short) {
    const [ic, label] = STATUS[status] || ["circle", UI.humanize(status)];
    return el("span", { class: "status s-" + (status || "unknown"), title: label }, UI.icon(ic), short ? null : el("span", { text: label }));
  };

  // -------------------------------------------------------------- toast
  UI.toast = function toast(msg) {
    const box = $("toasts");
    if (!box) return;
    const t = el("div", { class: "toast", text: msg });
    box.append(t);
    setTimeout(() => t.remove(), 2600);
  };

  UI.copy = async function copy(text, what) {
    try {
      await navigator.clipboard.writeText(text);
    } catch (e) {
      // Clipboard API is unavailable on plain http origins other than localhost.
      const ta = el("textarea", { class: "offscreen", "aria-hidden": "true" });
      ta.value = text;
      document.body.append(ta);
      ta.select();
      try { document.execCommand("copy"); } catch (e2) { /* ignore */ }
      ta.remove();
    }
    UI.toast("Copied " + (what || "to clipboard"));
  };

  UI.download = function download(name, text, mime) {
    const url = URL.createObjectURL(new Blob([text], { type: mime || "text/plain" }));
    const a = el("a", { href: url, download: name });
    document.body.append(a);
    a.click();
    a.remove();
    setTimeout(() => URL.revokeObjectURL(url), 2000);
  };

  /** Small icon-only copy button. */
  UI.copyBtn = function copyBtn(getText, what) {
    const b = UI.btn("", { icon: "copy", cls: "icon-btn xs", title: "Copy " + (what || "value") });
    b.addEventListener("click", (e) => { e.stopPropagation(); UI.copy(typeof getText === "function" ? getText() : getText, what); });
    return b;
  };

  // ------------------------------------------------------------ taxonomy
  // Finding categories: label + icon. Unknown categories (new collectors ship
  // new ones) fall back to a humanised name and a generic tag icon, so they
  // always render. Add rows here to give a new category its own look.
  const C = (label, icon) => ({ label, icon });
  UI.CATEGORIES = {
    social: C("Social accounts", "user"), profile: C("Profiles", "id"), "email-account": C("E-mail accounts", "mail"),
    "social-links": C("Social links on sites", "link"), "site-people": C("People on sites", "user"), developer: C("Developer traces", "code"),
    dns: C("DNS", "server"), whois: C("Registration", "file"), subdomain: C("Subdomains", "globe"), web: C("Website", "globe"),
    tech: C("Technology", "layers"), tls: C("TLS", "shield"), tracking: C("Tracking IDs", "hash"), contact: C("Contacts", "mail"),
    "email-security": C("E-mail security", "shield"), security: C("Security headers", "shield"), archive: C("Web archive", "clock"),
    exposure: C("Exposure", "alert"), takeover: C("Takeover risk", "alert"), reputation: C("Reputation", "shield"), network: C("Network", "server"),
    geo: C("Geolocation", "pin"), crypto: C("Crypto", "wallet"), company: C("Company", "building"), registry: C("Official registry", "building"),
    reference: C("Reference", "file"), website: C("Websites", "globe"), identity: C("Identity leads", "id"), phone: C("Phone", "phone"),
    "email-domain": C("E-mail domain", "mail"), ip: C("IP", "server"), breach: C("Breach exposure", "alert"), tooling: C("External tools", "tool"),
    photo: C("Photos", "image"), mentions: C("Mentions", "list"), "document-metadata": C("Document metadata", "file"), publication: C("Publications", "file"),
    history: C("History", "clock"), activity: C("Activity", "pulse"),
    // categories added by newer collectors
    idinfo: C("Decoded platform IDs", "hash"), lookalike: C("Look-alike domains", "globe"), sanctions: C("Sanctions & watchlists", "alert"),
    graph: C("Social graph", "graph"), file: C("Uploaded files", "file"), "geo-media": C("Geotagged pictures nearby", "image"), context: C("Place and period", "clock"), "osint-framework": C("OSINT framework events", "layers"), "social-scraper": C("Social collectors", "user"), exif: C("Picture metadata", "camera"), pgp: C("PGP & signing keys", "key"), code: C("Code search", "code"),
  };
  UI.cat = (c) => {
    const key = c || "other";
    return UI.CATEGORIES[key] || { label: UI.humanize(key), icon: "tag" };
  };

  // Entity types: label + icon (+ graph glyph in graph.js).
  UI.TYPES = {
    name: C("Names", "user"), username: C("Handles", "at"), email: C("E-mail addresses", "mail"), phone: C("Phone numbers", "phone"),
    domain: C("Domains", "globe"), ip: C("IP addresses", "server"), organisation: C("Organisations", "building"), location: C("Places", "pin"),
    url: C("URLs", "link"), image: C("Pictures", "image"), asn: C("ASNs", "network"), wallet: C("Wallets", "wallet"),
  };
  UI.type = (t) => UI.TYPES[t] || { label: UI.humanize(t), icon: "hash" };

  // What each collector needs, and the order they are listed in.
  UI.MODULE_NEEDS = {
    social: "username", profiles: "username", domain: "domain", ip: "IP", asn: "ASN", company: "company", people: "full name", images: "name or username",
    activity: "username", breach: "e-mail", sitefiles: "domain", bizreg: "company", devtrace: "username or e-mail", mentions: "full name", docmeta: "domain",
    records: "name, domain or IP", footprint: "e-mail / phone", phone: "phone", crypto: "wallet", dorks: "anything", syswrap: "domain / username",
    iddecode: "username or ID", keys: "e-mail or username", lookalike: "domain", webhistory: "domain", sanctions: "name or company", bskygraph: "username", codesearch: "username, e-mail or domain",
    uploads: "an uploaded file", context: "coordinates or a period",
  };
  UI.MODULE_ORDER = ["social", "profiles", "footprint", "phone", "domain", "ip", "asn", "company", "people", "images", "activity", "breach", "sitefiles", "bizreg", "mentions", "devtrace", "docmeta", "records", "crypto", "iddecode", "keys", "lookalike", "webhistory", "sanctions", "bskygraph", "codesearch", "uploads", "context", "dorks", "syswrap"];
  UI.modRank = (m) => { const i = UI.MODULE_ORDER.indexOf(m); return i < 0 ? 999 : i; };

  // --------------------------------------------------------- hint buttons
  // The little "i": English + Arabic explanation. The tooltip is position:fixed
  // so a scrolling panel never clips it; it is placed next to its button.
  UI.infoBtn = function infoBtn(key) {
    const h = UI.HINTS && UI.HINTS[key];
    if (!h) return null;
    const tip = el("span", { class: "tipbox", role: "tooltip" }, el("span", { class: "en", text: h.en }), el("span", { class: "ar", lang: "ar", dir: "rtl", text: h.ar }));
    const b = el("button", { type: "button", class: "info", "aria-label": "What does this do?" }, "i", tip);
    const place = () => requestAnimationFrame(() => {
      const r = b.getBoundingClientRect();
      const w = Math.min(340, window.innerWidth - 24);
      tip.style.width = w + "px";
      const left = Math.max(12, Math.min(r.left - 8, window.innerWidth - 12 - w));
      let top = r.bottom + 8;
      if (top + tip.offsetHeight > window.innerHeight - 12) top = Math.max(12, r.top - tip.offsetHeight - 8);
      tip.style.left = left + "px";
      tip.style.top = top + "px";
    });
    b.addEventListener("mouseenter", place);
    b.addEventListener("focus", place);
    b.addEventListener("click", (e) => {
      e.preventDefault(); e.stopPropagation();
      const was = b.classList.contains("open");
      closeInfo();
      if (!was) { b.classList.add("open"); place(); }
    });
    return b;
  };
  const closeInfo = () => document.querySelectorAll(".info.open").forEach((x) => x.classList.remove("open"));
  document.addEventListener("click", closeInfo);
  window.addEventListener("scroll", closeInfo, true); // a fixed tooltip would drift from its button
  UI.closeInfo = closeInfo;

  // ---------------------------------------------------------------- popover
  // Tiny anchored popover used by filter menus and the export menu. Only one
  // is open at a time; Escape or an outside click closes it.
  let openPop = null;
  UI.popover = function popover(anchor, content, opts = {}) {
    UI.closePopover();
    const pop = el("div", { class: "popover" + (opts.cls ? " " + opts.cls : ""), role: opts.role || "dialog" }, content);
    document.body.append(pop);
    const r = anchor.getBoundingClientRect();
    const w = pop.offsetWidth;
    pop.style.left = Math.max(8, Math.min(opts.alignRight ? r.right - w : r.left, window.innerWidth - w - 8)) + "px";
    const below = r.bottom + 4;
    pop.style.top = (below + pop.offsetHeight > window.innerHeight - 8 ? Math.max(8, r.top - pop.offsetHeight - 4) : below) + "px";
    openPop = { pop, anchor };
    anchor.setAttribute("aria-expanded", "true");
    return pop;
  };
  UI.closePopover = function closePopover() {
    if (!openPop) return;
    openPop.pop.remove();
    openPop.anchor.setAttribute("aria-expanded", "false");
    openPop = null;
  };
  document.addEventListener("mousedown", (e) => { if (openPop && !openPop.pop.contains(e.target) && !openPop.anchor.contains(e.target)) UI.closePopover(); });
  window.addEventListener("resize", () => UI.closePopover());

  // Escape closes the innermost transient thing first.
  document.addEventListener("keydown", (e) => {
    if (e.key !== "Escape") return;
    if (openPop) { const a = openPop.anchor; UI.closePopover(); a.focus(); return; }
    closeInfo();
  });
})();
