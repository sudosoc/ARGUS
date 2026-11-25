"use strict";

// Case panel: the "new investigation" form, the list of investigations in this
// session and the most recent saved cases.
(function () {
  const { $, el } = UI;

  UI.FIELDS = [
    { group: "Person", items: [
      { name: "full_name", label: "Full name", ph: "John Doe" },
      { name: "username", label: "Username / handle", ph: "johndoe", mono: 1 },
      { name: "alias", label: "Nickname / alias", ph: "Dark Knight" },
      { name: "email", label: "E-mail", ph: "john@example.com", mono: 1 },
      { name: "phone", label: "Phone", ph: "+15551234567", mono: 1 },
    ] },
    { group: "Infrastructure", items: [
      { name: "domain", label: "Domain, website or profile link", ph: "example.com · https://x.com/handle", mono: 1 },
      { name: "ip", label: "IP address", ph: "203.0.113.10", mono: 1 },
      { name: "asn", label: "ASN", ph: "AS13335", mono: 1 },
      { name: "wallet", label: "Crypto wallet", ph: "bc1q… or 0x…", mono: 1 },
    ] },
    { group: "Organisation & place", items: [
      { name: "company", label: "Company / organisation", ph: "Example Inc" },
      { name: "location", label: "Location", ph: "Cairo, Egypt" },
    ] },
  ];

  // ------------------------------------------------------------- panel state
  UI.showPanel = function showPanel(open) {
    const root = document.documentElement;
    if (open) root.removeAttribute("data-panel"); else root.setAttribute("data-panel", "closed");
    $("panelToggle").setAttribute("aria-expanded", String(open));
    UI.store.set("panel", open ? "open" : "closed");
  };
  UI.panelOpen = () => document.documentElement.getAttribute("data-panel") !== "closed";

  UI.panelTab = function panelTab(name) {
    document.querySelectorAll(".ptab").forEach((b) => {
      const on = b.dataset.ptab === name;
      b.classList.toggle("active", on);
      b.setAttribute("aria-selected", String(on));
    });
    document.querySelectorAll(".cp-pane").forEach((p) => (p.hidden = p.dataset.pane !== name));
    if (name === "cases") UI.refreshSaved();
  };

  // --------------------------------------------------------------- form build
  UI.buildForm = function buildForm(info) {
    const root = $("fields");
    root.replaceChildren();

    // Collapsible groups remember whether they are open and how many fields are filled.
    const group = (title, host) => {
      const det = el("details", { class: "fgroup" });
      det.open = UI.store.get("grp_" + title, "1") === "1";
      const filled = el("span", { class: "filled" });
      det.append(el("summary", {}, UI.icon("chevron"), el("span", { class: "g-title", text: title }), filled));
      det.addEventListener("toggle", () => UI.store.set("grp_" + title, det.open ? "1" : "0"));
      det.addEventListener("input", () => {
        const n = [...det.querySelectorAll("input:not([type=checkbox]), textarea")].filter((i) => i.value.trim()).length;
        filled.textContent = n ? n + " filled" : "";
      });
      (host || root).append(det);
      return det;
    };

    for (const g of UI.FIELDS) {
      const det = group(g.group);
      for (const f of g.items) {
        const input = el("input", { name: f.name, id: "f_" + f.name, maxlength: "200", placeholder: f.ph, spellcheck: "false", autocomplete: "off", class: f.mono ? "mono-in" : "" });
        det.append(el("div", { class: "field" }, el("label", { for: "f_" + f.name }, el("span", { text: f.label }), UI.infoBtn(f.name)), input));
      }
    }

    const ev = group("Evidence you already have");
    ev.append(el("div", { class: "field" },
      el("label", { for: "f_notes" }, el("span", { text: "Paste text (CV, LinkedIn page, bio…)" }), UI.infoBtn("notes")),
      el("textarea", { name: "notes", id: "f_notes", rows: "3", maxlength: "30000", class: "notes", placeholder: "Optional. Text you copied from a source SUDOSOC ARGUS cannot open." })));
    // Clipper: the operator logs in and looks at the page themself; this bookmarklet
    // only copies what is on screen so it can be pasted above. SUDOSOC ARGUS never logs in.
    const clip = el("a", { class: "btn sm", title: "Drag this to your bookmarks bar. On a page you are logged in to (e.g. a LinkedIn profile) click it, then paste into the box above." }, UI.icon("copy"), el("span", { text: "Copy page for SUDOSOC ARGUS" }));
    clip.setAttribute("href", "javascript:(()=>{const t=location.href+'\\n\\n'+document.body.innerText;navigator.clipboard.writeText(t).then(()=>alert('Copied '+t.length+' characters for SUDOSOC ARGUS. Paste them into the Paste text box.'),()=>prompt('Copy this text:',t.slice(0,3000)))})()");
    clip.addEventListener("click", (e) => { e.preventDefault(); UI.toast("Drag this button to your bookmarks bar, then click it on a profile you have open"); });
    ev.append(el("div", { class: "hint" }, clip, el("span", { text: "Logged-in pages: you open it, this only copies what you see." })));

    const check = (name, title, sub, checked) => {
      const cb = el("input", { type: "checkbox", name, id: "o_" + name });
      if (checked) cb.checked = true;
      return el("label", { class: "check", for: "o_" + name }, cb, el("span", { class: "t" }, el("b", { text: title }), el("small", { text: sub })), UI.infoBtn(name));
    };
    const tc = info.site_tiers || {};
    const nTier = (k, d) => UI.n(tc[k] || d);
    const tier = el("select", { name: "site_tier", id: "o_site_tier", "aria-label": "Username sites to probe" },
      el("option", { value: "core", text: `Core · ${nTier("core", 1100)} sites · quick` }),
      el("option", { value: "extended", text: `Extended · ${nTier("extended", 3200)} sites · faster pass` }),
      el("option", { value: "all", text: `All · ${nTier("all", 10600)} sites · nothing left out (default)`, selected: "" }));
    const tierNote = el("small", { class: "tier-note" });
    const TIER_NOTES = {
      core: "Popular, well-maintained platforms. Minutes at most; the fewest false leads.",
      extended: "Adds the long tail of ordinary platforms. A faster pass when you are in a hurry.",
      all: "Everything: adds thousands of template-based forums and small communities. Slowest (one request per site per username) and the most leads to verify.",
    };
    const showTier = () => { tierNote.textContent = TIER_NOTES[tier.value] || ""; };
    tier.addEventListener("change", showTier); showTier();
    $("tier").replaceChildren(el("label", { class: "field-tier", for: "o_site_tier" }, el("span", { text: "Username sites" }), UI.infoBtn("site_tier")), tier, tierNote);
    $("intensity").replaceChildren(check("aggressive", "Aggressive", "maximum coverage, slower, many more requests", true));
    $("options").replaceChildren(
      check("verify", "Verify social hits", "canary test removes false positives", false),
      check("derive", "Derive usernames from the name", "jdoe, john.doe, johnd…", false),
      check("no_pivot", "Don't follow discovered leads", "first pass only", false),
      check("browser", "Use local browser for LinkedIn", "headless Chrome/Edge, logged-out view", false),
    );
    $("proxiesInfo").replaceChildren(UI.infoBtn("proxies"));

    // Collectors
    const box = $("modules");
    box.replaceChildren();
    const mods = [...info.modules].sort((a, b) => UI.modRank(a.name) - UI.modRank(b.name) || a.name.localeCompare(b.name));
    const count = $("modCount");
    const sync = () => {
      const all = [...box.querySelectorAll('input[name="mod"]')];
      count.textContent = all.filter((c) => c.checked).length + "/" + all.length + " selected";
    };
    for (const m of mods) {
      const cb = el("input", { type: "checkbox", name: "mod", value: m.name, id: "m_" + m.name, checked: "" });
      box.append(el("label", { class: "check mod", for: "m_" + m.name, title: m.description }, cb,
        el("span", { class: "t" }, el("b", { text: m.name }), el("span", { class: "need", text: UI.MODULE_NEEDS[m.name] || "any input" })),
        UI.infoBtn("mod:" + m.name)));
    }
    $("modAll").onclick = () => { box.querySelectorAll('input[name="mod"]').forEach((c) => (c.checked = true)); saveOpts(); };
    $("modNone").onclick = () => { box.querySelectorAll('input[name="mod"]').forEach((c) => (c.checked = false)); saveOpts(); };

    // Remember options and collector selection (never the subject's details).
    const OPT_NAMES = ["aggressive", "verify", "derive", "no_pivot", "browser"];
    const form = $("form");
    function saveOpts() {
      const o = { mods: [...box.querySelectorAll('input[name="mod"]')].filter((c) => c.checked).map((c) => c.value) };
      for (const n of OPT_NAMES) o[n] = form.elements[n].checked;
      o.site_tier = form.elements["site_tier"].value;
      UI.store.set("opts", JSON.stringify(o));
      sync();
    }
    try {
      const o = JSON.parse(UI.store.get("opts", "null"));
      if (o) {
        for (const n of OPT_NAMES) if (typeof o[n] === "boolean") form.elements[n].checked = o[n];
        if (["core", "extended", "all"].includes(o.site_tier)) form.elements["site_tier"].value = o.site_tier;
        if (Array.isArray(o.mods) && o.mods.length) box.querySelectorAll('input[name="mod"]').forEach((c) => { c.checked = o.mods.includes(c.value); });
      }
    } catch (e) { /* ignore a corrupt saved value */ }
    for (const id of ["intensity", "tier", "options", "modules"]) $(id).addEventListener("change", saveOpts);
    sync();
  };

  UI.showFormError = function showFormError(msg) {
    const b = $("formError");
    b.hidden = !msg;
    b.replaceChildren(...(msg ? [UI.icon("alert"), el("span", { text: msg })] : []));
  };

  UI.bindForm = function bindForm() {
    const submit = async (ev) => {
      ev && ev.preventDefault();
      UI.showFormError("");
      const f = $("form");
      const mods = [...f.querySelectorAll('input[name="mod"]')];
      const chosen = mods.filter((m) => m.checked).map((m) => m.value);
      if (chosen.length === 0) return UI.showFormError("Select at least one collector.");
      const v = (n) => f.elements[n].value;
      const body = {
        full_name: v("full_name"), username: v("username"), alias: v("alias"), email: v("email"), phone: v("phone"),
        domain: v("domain"), ip: v("ip"), asn: v("asn"), wallet: v("wallet"), company: v("company"), location: v("location"), notes: v("notes"),
        modules: chosen.length === mods.length ? [] : chosen,
        aggressive: f.elements["aggressive"].checked, verify: f.elements["verify"].checked,
        derive: f.elements["derive"].checked, browser: f.elements["browser"].checked, no_pivot: f.elements["no_pivot"].checked,
        site_tier: f.elements["site_tier"].value,
        proxies: f.elements["proxies"].value.split("\n").map((s) => s.trim()).filter(Boolean),
      };
      $("startBtn").disabled = true;
      try {
        const s = await UI.api("/api/scans", { method: "POST", body: JSON.stringify(body) });
        UI.openScan(s.id);
        if (window.innerWidth < 1100) UI.showPanel(false);
      } catch (e) {
        UI.showFormError(e.message);
      } finally {
        $("startBtn").disabled = false;
      }
    };
    $("form").addEventListener("submit", submit);
    $("form").addEventListener("keydown", (e) => { if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) submit(e); });
    $("clearBtn").addEventListener("click", () => {
      document.querySelectorAll("#fields input, #fields textarea").forEach((i) => (i.value = ""));
      document.querySelectorAll("#fields .fgroup .filled").forEach((f) => (f.textContent = ""));
      UI.showFormError("");
    });
    document.querySelectorAll(".ptab").forEach((b) => b.addEventListener("click", () => UI.panelTab(b.dataset.ptab)));
    $("panelToggle").addEventListener("click", () => UI.showPanel(!UI.panelOpen()));
    $("scrim").addEventListener("click", () => UI.showPanel(false));
  };

  // ---------------------------------------------------------------- case lists
  const caseItem = (label, meta, state, active, onclick) => {
    const ico = state === "running" ? "pulse" : state === "done" ? "check" : state === "saved" ? "folder" : "alert";
    const b = el("button", { type: "button", class: "case-item" + (active ? " active" : ""), "aria-current": active ? "true" : null },
      UI.icon(ico), el("span", { class: "ci-main" }, el("span", { class: "ci-title", text: label }), el("span", { class: "ci-meta", text: meta })));
    b.addEventListener("click", onclick);
    return el("li", {}, b);
  };

  UI.refreshHistory = async function refreshHistory() {
    let list;
    try { list = await UI.api("/api/scans"); UI.setConn(true); } catch { UI.setConn(false); return; }
    const ul = $("history");
    ul.replaceChildren();
    if (list.length === 0) ul.append(el("li", { class: "empty-line", text: "Nothing run in this session yet." }));
    for (const s of list) {
      const label = UI.targetLabel(s.target) || s.id;
      ul.append(caseItem(label, `${s.state} · ${UI.n(s.hits)} findings`, s.state, UI.S && UI.S.id === s.id, () => { UI.openScan(s.id); if (window.innerWidth < 1100) UI.showPanel(false); }));
    }
    const running = list.filter((s) => s.state === "running").length;
    $("cnt-cases").textContent = String(list.length);
    $("cnt-cases").hidden = list.length === 0;
    UI.runningCount = running;
    UI.updatePill();
  };

  UI.refreshSaved = async function refreshSaved() {
    const ul = $("savedList");
    let list;
    try { list = await UI.api("/api/cases"); } catch (e) { ul.replaceChildren(el("li", { class: "empty-line", text: "Could not list saved cases: " + e.message })); return; }
    ul.replaceChildren();
    if (!list.length) ul.append(el("li", { class: "empty-line", text: "Finished investigations are saved here automatically." }));
    for (const c of list.slice(0, 15)) {
      ul.append(caseItem(c.title || "(no identifiers)", `${UI.when(c.at)} · ${UI.n(c.findings)} findings · ${c.identities} people`, "saved", false, async () => {
        try { const r = await UI.api("/api/cases/" + c.id + "/open", { method: "POST" }); UI.openScan(r.id); if (window.innerWidth < 1100) UI.showPanel(false); } catch (e) { UI.toast(e.message); }
      }));
    }
    $("savedMore").hidden = list.length === 0;
  };
})();
