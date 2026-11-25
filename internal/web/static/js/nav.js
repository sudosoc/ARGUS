"use strict";

// Two-level navigation. The top bar holds the main views (Identity, Footprint,
// Infrastructure…); each view has a left rail of sections with counts. A section
// is either one of the original full-page views (`legacy`) or a page built from
// the derived model by a render function. Sections with nothing to show are
// tucked under "Not available yet" instead of cluttering the rail, and say what
// would fill them when opened.
(function () {
  const { $, el } = UI;
  const TABS = [];

  /**
   * Register a main view.
   * sections: [{ id, label, icon, group?, legacy?, render(page, M)?, count?(M) -> number, always?: true }]
   * count() === 0 marks a section as empty (listed under "Not available yet") unless `always`.
   */
  UI.defineTab = function defineTab(def) {
    const i = TABS.findIndex((t) => t.id === def.id);
    if (i >= 0) TABS[i] = def; else TABS.push(def);
  };
  UI.TAB_ORDER = ["overview", "identity", "footprint", "infrastructure", "media", "graph", "location", "timeline", "exposure", "findings", "evidence", "reports"];
  const ordered = () => [...TABS].sort((a, b) => UI.TAB_ORDER.indexOf(a.id) - UI.TAB_ORDER.indexOf(b.id));
  Object.defineProperty(UI, "TABS", { get: () => ordered().map((t) => ({ id: t.id, label: t.label, icon: t.icon })) });

  // Old tab names (used by links elsewhere in the UI) -> [tab, section].
  const ALIAS = {
    identity: ["identity", "profile"], overview: ["overview", "summary"], tree: ["graph", "tree"], graph: ["graph", "connections"], location: ["location", "geo"],
    photos: ["media", "collection"], findings: ["findings", "all"], coverage: ["evidence", "coverage"],
  };

  const state = { tab: null, sec: {}, key: "" };
  const tabDef = (id) => TABS.find((t) => t.id === id);
  const sectionsOf = (t) => t.sections;

  function secCount(s, M) {
    if (!s.count) return null;
    try { return s.count(M); } catch (e) { return null; }
  }
  const isEmpty = (s, M) => !s.always && s.count && secCount(s, M) === 0;

  function pickSection(t, M) {
    let id = state.sec[t.id] || UI.session.get("sec_" + t.id);
    let s = t.sections.find((x) => x.id === id);
    if (!s) s = t.sections.find((x) => !isEmpty(x, M)) || t.sections[0];
    return s;
  }

  /** Go to a main view / section. `name` may be an old tab name. */
  UI.go = function go(name, sec, silent) {
    let tab = name;
    if (ALIAS[name] && !sec) [tab, sec] = ALIAS[name];
    if (!tabDef(tab)) tab = "overview";
    state.tab = tab;
    UI.tab = tab;
    if (sec) { state.sec[tab] = sec; UI.session.set("sec_" + tab, sec); }
    UI.session.set("tab", tab);
    if (!silent && $("workspace")) $("workspace").scrollTop = 0;
    UI.scheduleRender(true);
  };
  UI.selectTab = (name, silent) => UI.go(name, null, silent);

  UI.currentSection = () => { const t = tabDef(state.tab); return t ? pickSection(t, UI.M()).id : null; };

  // ---------------------------------------------------------------- drawing
  function drawTabs(M) {
    const bar = $("tabs");
    if (!bar.dataset.built) {
      bar.dataset.built = "1";
      for (const t of ordered()) {
        const b = el("button", { class: "tab", type: "button", role: "tab", "data-tab": t.id, "aria-selected": "false", tabindex: "-1", title: t.label, "aria-label": t.label }, UI.icon(t.icon), el("span", { class: "tl", text: t.label }), el("span", { class: "count", "data-count": t.id, hidden: true }));
        b.addEventListener("click", () => UI.go(t.id));
        b.addEventListener("keydown", (e) => {
          const d = e.key === "ArrowRight" ? 1 : e.key === "ArrowLeft" ? -1 : 0;
          if (!d) return;
          e.preventDefault();
          const all = [...bar.querySelectorAll(".tab")];
          const n = all[(all.indexOf(b) + d + all.length) % all.length];
          n.focus(); UI.go(n.dataset.tab, null, true);
        });
        bar.append(b);
      }
    }
    for (const t of ordered()) {
      const b = bar.querySelector(`[data-tab="${t.id}"]`);
      const on = t.id === state.tab;
      b.classList.toggle("active", on);
      b.setAttribute("aria-selected", on ? "true" : "false");
      b.tabIndex = on ? 0 : -1;
      if (on && b.offsetParent) bar.scrollLeft = Math.max(0, b.offsetLeft - (bar.clientWidth - b.offsetWidth) / 2);
      const c = b.querySelector(".count");
      let n = null;
      if (t.count) { try { n = t.count(M); } catch (e) { n = null; } }
      c.hidden = !n;
      c.textContent = n ? UI.n(n) : "";
      c.classList.toggle("warn", !!t.warn && !!n);
    }
  }

  function drawSubnav(t, cur, M) {
    const nav = $("subnav");
    const secs = sectionsOf(t);
    nav.hidden = secs.length < 2;
    $("tabbody").classList.toggle("nosub", secs.length < 2);
    const sig = t.id + "|" + cur.id + "|" + secs.map((s) => secCount(s, M)).join(",");
    if (nav.dataset.sig === sig) return;
    nav.dataset.sig = sig;
    nav.replaceChildren();
    nav.append(el("div", { class: "sn-title" }, UI.icon(t.icon), el("span", { text: t.label })));
    const mk = (s, muted) => {
      const n = secCount(s, M);
      const b = el("button", { type: "button", class: "sub" + (s.id === cur.id ? " active" : "") + (muted ? " muted" : ""), "data-sec": s.id, "aria-current": s.id === cur.id ? "page" : null },
        s.icon ? UI.icon(s.icon) : null, el("span", { class: "sl", text: s.label }), n ? el("span", { class: "count", text: UI.n(n) }) : null);
      b.addEventListener("click", () => UI.go(t.id, s.id));
      return b;
    };
    let group = null;
    const empties = [];
    for (const s of secs) {
      if (isEmpty(s, M) && s.id !== cur.id) { empties.push(s); continue; }
      if (s.group && s.group !== group) { group = s.group; nav.append(el("div", { class: "sn-group", text: group })); }
      nav.append(mk(s, false));
    }
    if (empties.length) {
      const d = el("details", { class: "sn-empty" }, el("summary", {}, UI.icon("chevron", "caret"), `Not available yet (${empties.length})`));
      for (const s of empties) d.append(mk(s, true));
      nav.append(d);
    }
  }

  let mountedLegacy = null;
  function unmountLegacy() {
    if (!mountedLegacy) return;
    const root = $("tab-" + mountedLegacy);
    root.hidden = true;
    $("legacy").append(root);
    mountedLegacy = null;
  }

  UI.navRender = function navRender() {
    const M = UI.M();
    if (!M) return;
    if (!state.tab || !tabDef(state.tab)) state.tab = tabDef(UI.session.get("tab")) ? UI.session.get("tab") : "overview";
    UI.tab = state.tab;
    const t = tabDef(state.tab);
    const cur = pickSection(t, M);
    drawTabs(M);
    drawSubnav(t, cur, M);

    const host = $("subhost");
    const key = t.id + ":" + cur.id + ":" + UI.S.id;
    if (host.dataset.key !== key) {
      unmountLegacy();
      host.replaceChildren();
      host.dataset.key = key;
      if (cur.legacy) {
        const root = $("tab-" + cur.legacy);
        root.hidden = false;
        host.append(root);
        mountedLegacy = cur.legacy;
      }
    }
    if (cur.legacy) {
      const v = UI.views[cur.legacy];
      if (v) v();
      return;
    }
    const keep = host.scrollTop;
    const page = el("div", { class: "secpage" });
    try { cur.render(page, M); }
    catch (e) { page.replaceChildren(UI.kit.panel("This section could not be drawn", "alert", UI.kit.hint(String(e && e.message || e)))); console.error(e); }
    host.replaceChildren(page);
    host.scrollTop = keep;
  };

  UI.navReset = function navReset() { const h = $("subhost"); if (h) { unmountLegacy(); h.replaceChildren(); h.dataset.key = ""; } const n = $("subnav"); if (n) n.dataset.sig = ""; };
  (UI.resetHooks = UI.resetHooks || []).push(UI.navReset);
})();

// The sticky sub-navigation sits right under the sticky header, whatever its height.
(function () {
  const rh = document.querySelector(".runhead");
  if (!rh || !window.ResizeObserver) return;
  new ResizeObserver(() => document.documentElement.style.setProperty("--runhead-h", rh.offsetHeight + "px")).observe(rh);
})();
