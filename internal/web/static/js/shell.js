"use strict";

// The application shell: top bar controls (theme, density, status pill), the
// command palette / global search, keyboard shortcuts and their help overlay,
// and the first-use notice.
(function () {
  const { $, el } = UI;
  const root = document.documentElement;

  // ------------------------------------------------------------ theme/density
  function applyTheme(t) {
    root.setAttribute("data-theme", t);
    UI.store.set("theme", t);
    const b = $("themeBtn");
    b.replaceChildren(UI.icon(t === "dark" ? "moon" : "sun"));
    b.title = `Theme: ${t} (t). Switch to ${t === "dark" ? "light" : "dark"}`;
    b.setAttribute("aria-label", b.title);
    if (UI.S) { UI.scheduleRender(true); UI.graphRedraw && UI.graphRedraw(); }
  }
  UI.toggleTheme = () => applyTheme(root.getAttribute("data-theme") === "dark" ? "light" : "dark");
  function applyDensity(d) {
    root.setAttribute("data-density", d);
    UI.store.set("density", d);
    const b = $("densityBtn");
    b.title = `Density: ${d} (d). Switch to ${d === "compact" ? "comfortable" : "compact"}`;
    b.setAttribute("aria-label", b.title);
    b.setAttribute("aria-pressed", String(d === "compact"));
  }
  UI.toggleDensity = () => applyDensity(root.getAttribute("data-density") === "compact" ? "comfortable" : "compact");

  // ------------------------------------------------------------ status pieces
  UI.setConn = function setConn(ok) {
    const c = $("conn");
    c.className = "conn " + (ok ? "ok" : "bad");
    c.replaceChildren(UI.icon(ok ? "check" : "alert"), el("span", { text: ok ? "Connected" : "Server unreachable" }));
  };

  // Top-bar pill: progress of the open investigation, or how many are running.
  UI.updatePill = function updatePill() {
    const pill = $("scanPill");
    const S = UI.S;
    const running = S && S.state === "running";
    const others = (UI.runningCount || 0) - (running ? 1 : 0);
    if (!running && others <= 0) { pill.hidden = true; return; }
    pill.hidden = false;
    const fill = $("pillFill");
    const w = running ? parseInt($("overallFill").style.width, 10) || 0 : 0;
    fill.style.width = w + "%";
    $("pillText").textContent = running
      ? `Collecting · ${UI.n(S.findings.length)} findings` + (others > 0 ? ` · +${others} more` : "")
      : `${others} investigation${others === 1 ? "" : "s"} running`;
  };

  // ------------------------------------------------------------- shortcuts
  // [group, keys, description]. "a | b" lists alternatives, "a+b" is a chord.
  const SHORTCUTS = [
    ["General", "Ctrl+K | /", "Command palette: search this case or run any command"],
    ["General", "?", "Show this help"],
    ["General", "Esc", "Close the palette, a menu, the details drawer or this help"],
    ["General", "t", "Switch between dark and light theme"],
    ["General", "d", "Switch between compact and comfortable density"],
    ["Navigate", "1–8", "Identity, Overview, Tree, Graph, Location, Photos, Findings, Coverage"],
    ["Navigate", "← | →", "Move between tabs when a tab has focus"],
    ["Navigate", "[", "Show or hide the left case panel"],
    ["Investigation", "n", "New investigation: focus the first field of the form"],
    ["Investigation", "Ctrl+Enter", "Start the investigation (from any form field)"],
    ["Findings table", "j | k", "Next / previous row"],
    ["Findings table", "Enter", "Expand the focused row to its full evidence"],
    ["Findings table", "x", "Select the focused row"],
    ["Relationship graph", "+ | -", "Zoom in / out (graph focused)"],
    ["Relationship graph", "0", "Fit the graph to the screen (graph focused)"],
  ];

  function showHelp() {
    const wrap = el("div", { class: "shortcuts" });
    let group = null;
    for (const [g, keys, desc] of SHORTCUTS) {
      if (g !== group) { group = g; wrap.append(el("h3", { class: "sc-group", text: g })); }
      const k = el("span", { class: "sc-keys" });
      keys.split(" | ").forEach((alt, i) => {
        if (i) k.append(el("span", { class: "plus", text: "or" }));
        alt.split("+").forEach((key, j) => { if (j) k.append(el("span", { class: "plus", text: "+" })); k.append(el("kbd", { text: key })); });
      });
      wrap.append(el("div", { class: "sc-row" }, k, el("div", { class: "sc-desc", text: desc })));
    }
    overlay("Keyboard shortcuts", el("div", { class: "modal-body" }, wrap), { id: "helpOverlay" });
  }

  // --------------------------------------------------------- overlay helper
  let overlayEl = null, overlayOpener = null;
  function overlay(title, content, opts = {}) {
    closeOverlay();
    overlayOpener = document.activeElement;
    const close = UI.btn("", { icon: "x", cls: "icon-btn", title: "Close (Esc)" });
    close.addEventListener("click", closeOverlay);
    const box = el("div", { class: "modal " + (opts.cls || ""), role: "dialog", "aria-modal": "true", "aria-label": title }, el("header", { class: "modal-head" }, el("h2", { text: title }), close), content);
    overlayEl = el("div", { class: "overlay", id: opts.id || "overlay" }, box);
    overlayEl.addEventListener("mousedown", (e) => { if (e.target === overlayEl) closeOverlay(); });
    $("overlays").append(overlayEl);
    close.focus();
    return box;
  }
  function closeOverlay() {
    if (!overlayEl) return;
    overlayEl.remove();
    overlayEl = null;
    if (overlayOpener && document.contains(overlayOpener)) overlayOpener.focus();
    overlayOpener = null;
  }
  UI.closeOverlay = closeOverlay;

  // ----------------------------------------------------------- command palette
  function commands() {
    const S = UI.S;
    const list = [];
    const add = (label, icon, run, keys, group, extra) => list.push({ label, icon, run, keys, group: group || "Commands", hay: (label + " " + (extra || "")).toLowerCase() });
    if (S) UI.TABS.forEach((t, i) => add("Go to " + t.label, t.icon, () => UI.selectTab(t.id), String(i + 1), "Navigate", "tab view"));
    add("New investigation", "plus", () => { UI.showPanel(true); UI.panelTab("new"); setTimeout(() => $("f_full_name") && $("f_full_name").focus(), 30); }, "n", "Commands", "start form");
    add("Saved cases", "folder", () => UI.showCases(), "", "Commands", "open compare history");
    add("Source status", "pulse", () => UI.showHealth(), "", "Commands", "health reachable services");
    add("Toggle case panel", "panel", () => UI.showPanel(!UI.panelOpen()), "[", "Commands", "sidebar left");
    add("Switch theme (dark / light)", "moon", () => UI.toggleTheme(), "t", "Commands", "dark light colour");
    add("Switch density (compact / comfortable)", "rows", () => UI.toggleDensity(), "d", "Commands", "rows spacing");
    add("Keyboard shortcuts", "help", showHelp, "?", "Commands", "help keys");
    if (S && S.state === "running") add("Stop the running investigation", "stop", () => $("cancelBtn").click(), "", "Commands", "cancel");
    if (S && S.state !== "running" && S.state !== "failed") for (const [fmt, label] of UI.EXPORTS) {
      add("Export " + label, "download", () => { const a = el("a", { href: `/api/scans/${S.id}/export?format=${fmt}`, download: "" }); document.body.append(a); a.click(); a.remove(); }, "", "Export", "download report " + fmt);
    }
    return list;
  }

  function dataResults(q) {
    const S = UI.S;
    const out = [];
    if (!S || q.length < 2) return out;
    const lq = q.toLowerCase();
    const push = (group, label, sub, icon, run) => out.push({ label, sub, icon, run, group });

    let nF = 0, shown = 0;
    for (const f of S.findings) {
      if (!UI.haystack(f).includes(lq)) continue;
      nF++;
      if (shown < 8) { shown++; push("Findings", f.source, f.module + " · " + UI.cat(f.category).label, UI.cat(f.category).icon, () => UI.showFinding(f)); }
    }
    if (nF > shown) push("Findings", `See all ${UI.n(nF)} matching findings`, "open the Findings tab with this filter", "list", () => UI.gotoFindings({ q }));

    let nE = 0;
    for (const e of S.entities.values()) {
      if (!e.value.toLowerCase().includes(lq)) continue;
      if (nE++ < 6) push("Entities", e.value, e.type + (e.strong ? " · strong lead" : " · weak lead"), UI.type(e.type).icon, () => UI.showEntity(e));
    }
    let nP = 0;
    for (const id of S.identities || []) {
      for (const k of ["names", "headlines", "employers", "locations", "education", "interests", "emails", "phones", "usernames", "websites"]) {
        for (const f of id[k] || []) if (nP < 6 && f.value.toLowerCase().includes(lq)) { nP++; push("People", f.value, id.label + " · " + k, "user", () => UI.selectTab("identity")); }
      }
    }
    const rep = S.report || {};
    ((rep.geo && rep.geo.points) || []).filter((p) => (p.label + " " + (p.place || "")).toLowerCase().includes(lq)).slice(0, 4)
      .forEach((p) => push("Locations", p.label + (p.place ? " → " + p.place : ""), p.kind, "pin", () => UI.selectTab("location")));
    (rep.timeline || []).filter((e) => (e.event + " " + e.source).toLowerCase().includes(lq)).slice(0, 4)
      .forEach((e) => push("Timeline", e.event, e.date + " · " + e.source, "clock", () => UI.selectTab("identity")));
    Object.entries(rep.notes || {}).filter(([, t]) => t.toLowerCase().includes(lq)).slice(0, 3)
      .forEach(([, t]) => push("Notes", UI.clip(t, 90), "analyst note", "file", () => {}));
    return out;
  }

  function openPalette(initial) {
    closeOverlay();
    UI.closePopover();
    let results = [], active = 0;
    const input = el("input", { type: "text", class: "pal-input", role: "combobox", "aria-expanded": "true", "aria-controls": "palList", "aria-autocomplete": "list", placeholder: "Search this case, or type a command…", autocomplete: "off", spellcheck: "false", "aria-label": "Search or run a command" });
    const list = el("ul", { class: "pal-list", id: "palList", role: "listbox", "aria-label": "Results" });
    const empty = el("div", { class: "pal-empty", hidden: true, text: "Nothing matches." });
    const foot = el("div", { class: "pal-foot" }, el("span", {}, el("kbd", { text: "↑" }), el("kbd", { text: "↓" }), " move"), el("span", {}, el("kbd", { text: "Enter" }), " select"), el("span", {}, el("kbd", { text: "Esc" }), " close"), UI.S ? el("span", { class: "grow" }) : el("span", { class: "grow", text: "Open or start an investigation to search its data." }));
    const box = overlay("Command palette", el("div", { class: "pal" }, el("div", { class: "pal-bar" }, UI.icon("search"), input), list, empty, foot), { cls: "palette", id: "paletteOverlay" });
    box.querySelector(".modal-head").hidden = true;

    const render = () => {
      const q = input.value.trim();
      const lq = q.toLowerCase();
      const cmds = commands().filter((c) => !lq || c.hay.includes(lq) || lq.split(/\s+/).every((w) => c.hay.includes(w)));
      results = [...(lq ? dataResults(q) : []), ...cmds];
      if (q && UI.S && !results.some((r) => r.group === "Findings")) results.push({ group: "Findings", label: `Filter the findings table for “${UI.clip(q, 40)}”`, sub: "", icon: "filter", run: () => UI.gotoFindings({ q }) });
      active = Math.min(active, Math.max(0, results.length - 1));
      list.replaceChildren();
      let g = null;
      results.forEach((r, i) => {
        if (r.group !== g) { g = r.group; list.append(el("li", { class: "pal-group", role: "presentation", text: g })); }
        const li = el("li", { class: "pal-item" + (i === active ? " active" : ""), role: "option", id: "pal-" + i, "aria-selected": String(i === active) },
          UI.icon(r.icon || "tag"), el("span", { class: "pi-main" }, el("span", { class: "pi-t", text: r.label }), r.sub ? el("span", { class: "pi-s", text: r.sub }) : null), r.keys ? el("kbd", { text: r.keys }) : null);
        li.addEventListener("mousemove", () => { if (active !== i) { active = i; mark(); } });
        li.addEventListener("click", () => choose(i));
        list.append(li);
      });
      empty.hidden = results.length > 0;
      input.setAttribute("aria-activedescendant", results.length ? "pal-" + active : "");
    };
    const mark = () => {
      list.querySelectorAll(".pal-item").forEach((li) => { const on = li.id === "pal-" + active; li.classList.toggle("active", on); li.setAttribute("aria-selected", String(on)); if (on) li.scrollIntoView({ block: "nearest" }); });
      input.setAttribute("aria-activedescendant", "pal-" + active);
    };
    const choose = (i) => { const r = results[i]; if (!r) return; closeOverlay(); r.run(); };
    input.addEventListener("input", UI.debounce(() => { active = 0; render(); }, 90));
    input.addEventListener("keydown", (e) => {
      if (e.key === "ArrowDown") { active = Math.min(results.length - 1, active + 1); mark(); e.preventDefault(); }
      else if (e.key === "ArrowUp") { active = Math.max(0, active - 1); mark(); e.preventDefault(); }
      else if (e.key === "Enter") { choose(active); e.preventDefault(); }
      else if (e.key === "Tab") e.preventDefault(); // the palette is a single-field dialog
    });
    if (initial) input.value = initial;
    render();
    input.focus();
  }
  UI.openPalette = openPalette;

  // ---------------------------------------------------------- global keyboard
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape" && overlayEl) { closeOverlay(); e.preventDefault(); return; }
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "k") { e.preventDefault(); overlayEl && overlayEl.id === "paletteOverlay" ? closeOverlay() : openPalette(); return; }
    if (e.ctrlKey || e.metaKey || e.altKey) return;
    if (document.querySelector(".modal-bg")) return; // first-use notice is open
    const tag = (document.activeElement && document.activeElement.tagName) || "";
    if (/^(INPUT|TEXTAREA|SELECT)$/.test(tag) || (document.activeElement && document.activeElement.isContentEditable)) return;
    if (overlayEl) return;
    const k = e.key;
    if (k === "/") { e.preventDefault(); openPalette(); }
    else if (k === "?") { e.preventDefault(); showHelp(); }
    else if (k === "[") UI.showPanel(!UI.panelOpen());
    else if (k === "n") { e.preventDefault(); UI.showPanel(true); UI.panelTab("new"); const f = $("f_full_name"); if (f) f.focus(); }
    else if (k === "t") UI.toggleTheme();
    else if (k === "d") UI.toggleDensity();
    else if (/^[1-8]$/.test(k) && UI.S) UI.selectTab(UI.TABS[Number(k) - 1].id);
  });

  // ---------------------------------------------------------------- top bar
  $("paletteBtn").addEventListener("click", () => openPalette());
  $("casesBtn").addEventListener("click", () => UI.showCases());
  $("savedMore").addEventListener("click", () => UI.showCases());
  $("healthBtn").addEventListener("click", () => UI.showHealth());
  $("helpBtn").addEventListener("click", showHelp);
  $("themeBtn").addEventListener("click", UI.toggleTheme);
  $("densityBtn").addEventListener("click", UI.toggleDensity);
  $("scanPill").addEventListener("click", () => { if (UI.S) { UI.store.set("live", "open"); UI.paintRun(); $("workspace").scrollTop = 0; } });
  applyTheme(root.getAttribute("data-theme") || "dark");
  applyDensity(root.getAttribute("data-density") || "compact");

  // ------------------------------------------------------- first-use notice
  if (UI.store.get("consent_v1", "") !== "yes") {
    const bg = el("div", { class: "modal-bg", role: "dialog", "aria-modal": "true", "aria-labelledby": "consentTitle" });
    const cb = el("input", { type: "checkbox", id: "consentCb" });
    const go = UI.btn("I understand, continue", { cls: "primary" });
    go.disabled = true;
    cb.addEventListener("change", () => { go.disabled = !cb.checked; });
    go.addEventListener("click", () => { UI.store.set("consent_v1", "yes"); bg.remove(); });
    bg.append(el("div", { class: "modal consent" },
      el("div", { class: "consent-hero", "aria-hidden": "true" },
        el("img", { class: "consent-eyes", src: "/img/hero-eyes.webp", alt: "" }),
        el("img", { class: "consent-emblem", src: "/img/argus-emblem.webp", alt: "" })),
      el("h2", { id: "consentTitle", text: "Before you start" }),
      el("p", { text: "SUDOSOC ARGUS gathers information that is already public. It is for lawful, authorised work such as security research, due diligence, journalism, and checking your own exposure." }),
      el("ul", {},
        el("li", { text: "Everything it shows is a lead. A name is not an identity: verify before you act on or share anything." }),
        el("li", { text: "It does not log in anywhere, solve CAPTCHAs, or look for home addresses. Do not use it to stalk, harass or profile private people." }),
        el("li", { text: "You are responsible for following the law and each site's terms where you operate." })),
      el("label", { class: "check consent-check", for: "consentCb" }, cb, el("span", { class: "t" }, el("b", { text: "I will use SUDOSOC ARGUS only for lawful, authorised purposes." }))),
      el("div", { class: "actions" }, go)));
    bg.addEventListener("keydown", (e) => {
      if (e.key !== "Tab") return;
      const f = [cb, go].filter((x) => !x.disabled);
      const i = f.indexOf(document.activeElement);
      e.preventDefault();
      f[(i + (e.shiftKey ? -1 : 1) + f.length) % f.length].focus();
    });
    document.body.append(bg);
    setTimeout(() => cb.focus(), 50);
  }
})();
