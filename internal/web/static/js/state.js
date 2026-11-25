"use strict";

// Copyright (C) 2026 Seif, SUDOSOC. SPDX-License-Identifier: AGPL-3.0-only

// Live scan state: Server-Sent Events feed it, the views read it. Also owns
// the case header, the live collector rows, tabs and the render scheduler.
(function () {
  const { $, el } = UI;

  // Every view registers itself here: UI.views.<tab>() redraws that tab.
  UI.views = {};

  UI.EXPORTS = [
    ["json", "JSON", "Full machine-readable report with integrity digest", "json"],
    ["csv", "CSV", "Spreadsheet-friendly list of findings", "csv"],
    ["txt", "TXT", "Human-readable report", "txt"],
    ["html", "HTML", "Self-contained report with photos; open it and print to PDF", "html"],
    ["pdf", "PDF", "PDF report (needs Chrome, Chromium or Edge on the machine running SUDOSOC ARGUS)", "pdf"],
    ["entities", "Entities CSV", "Every discovered entity (e-mail, domain, username…) as CSV", "csv"],
    ["graphml", "GraphML", "Entity graph for Gephi / yEd / Maltego", "graphml"],
    ["stix", "STIX 2.1", "STIX 2.1 bundle for threat-intel platforms", "json"],
    ["misp", "MISP", "MISP event JSON", "json"],
    ["spiderfoot", "SpiderFoot", "SpiderFoot CSV layout", "csv"],
    ["maltego", "Maltego", "CSV for Maltego's Import Graph from Table", "csv"],
    ["md", "Markdown", "Markdown notes for Obsidian / Logseq", "md"],
    ["sql", "SQLite script", "SQL script that builds a SQLite database of the findings", "sql"],
  ];

  UI.newState = (id) => ({
    id, es: null, findings: [], entities: new Map(), target: {}, report: null, progress: {}, state: "running",
    identities: [], ambiguity: "", sameName: 0, created: Date.now(),
    mod: new Map(), // collector -> {n, hit, err}: live counters
    cat: new Map(), // category -> n
    errs: 0, modT: {}, rev: 0, // rev bumps whenever findings change (cache key for the findings view)
  });

  // ------------------------------------------------------------ scan lifecycle
  UI.openScan = function openScan(id) {
    if (UI.S && UI.S.es) UI.S.es.close();
    const S = (UI.S = UI.newState(id));
    UI.session.set("last", id);
    UI.closeDrawer();
    $("empty").hidden = true;
    $("run").hidden = false;
    $("exportBtn").disabled = true;
    $("bars").replaceChildren();
    $("caseTitle").textContent = "Investigation " + id;
    $("targetChips").replaceChildren();
    UI.setState("running");
    UI.resetViews();
    UI.selectTab(UI.tab || "identity", true);
    UI.paintRun();

    UI.api("/api/scans/" + id).then((s) => {
      S.target = s.target;
      S.created = Date.parse(s.created) || S.created;
      renderTargetChips();
      UI.scheduleRender();
    }).catch(() => {});

    const es = new EventSource("/api/scans/" + id + "/events");
    S.es = es;
    es.addEventListener("finding", (e) => UI.addFinding(JSON.parse(e.data)));
    es.addEventListener("progress", (e) => UI.onProgress(JSON.parse(e.data)));
    es.addEventListener("identities", (e) => UI.onIdentities(JSON.parse(e.data)));
    es.addEventListener("done", (e) => { UI.onDone(JSON.parse(e.data)); es.close(); });
    es.addEventListener("error", (e) => {
      if (e.data) { UI.onFailed(JSON.parse(e.data).error); es.close(); }
      // otherwise a transient network error: EventSource reconnects with Last-Event-ID
    });
    UI.refreshHistory();
  };

  UI.setState = function setState(s) {
    if (UI.S) UI.S.state = s;
    const b = $("state");
    b.className = "state-badge s-" + s;
    b.replaceChildren(UI.icon(s === "running" ? "pulse" : s === "done" ? "check" : "alert"), el("span", { text: s }));
    $("cancelBtn").hidden = s !== "running";
    $("exportBtn").disabled = s === "running" || s === "failed";
    UI.updatePill();
  };

  function renderTargetChips() {
    const t = UI.S.target || {};
    const box = $("targetChips");
    box.replaceChildren();
    const labels = { full_name: "name", username: "user", alias: "alias", email: "email", phone: "phone", domain: "domain", ip: "ip", asn: "asn", wallet: "wallet", company: "company", location: "place", coords: "coords" };
    for (const [k, lab] of Object.entries(labels)) {
      if (t[k]) box.append(el("span", { class: "chip strong" }, el("span", { class: "k", text: lab }), el("span", { text: t[k] })));
    }
    if (t.date_from) box.append(el("span", { class: "chip strong" }, el("span", { class: "k", text: "period" }), el("span", { text: t.date_to && t.date_to !== t.date_from ? t.date_from + " → " + t.date_to : t.date_from })));
    if (t.files && t.files.length) box.append(el("span", { class: "chip strong" }, el("span", { class: "k", text: "files" }), el("span", { text: String(t.files.length) })));
    const label = UI.targetLabel(t);
    if (label) $("caseTitle").textContent = label;
  }

  UI.addFinding = function addFinding(f) {
    const S = UI.S;
    S.findings.push(f);
    S.rev++;
    const m = S.mod.get(f.module) || { n: 0, hit: 0, err: 0 };
    m.n++;
    if (f.status === "found" || f.status === "uncertain") m.hit++;
    if (f.status === "error") { m.err++; S.errs++; }
    S.mod.set(f.module, m);
    const c = f.category || "other";
    S.cat.set(c, (S.cat.get(c) || 0) + 1);
    for (const e of f.entities || []) {
      const key = e.type + ":" + e.value.toLowerCase();
      const have = S.entities.get(key);
      if (!have) S.entities.set(key, { ...e, findings: [f] });
      else { have.strong = have.strong || e.strong; have.findings.push(f); }
    }
    UI.scheduleRender();
  };

  UI.onIdentities = function onIdentities(d) {
    const S = UI.S;
    S.identities = d.identities || [];
    S.ambiguity = d.ambiguity || "";
    S.sameName = d.same_name || 0;
    UI.scheduleRender();
  };

  UI.onProgress = function onProgress(p) {
    const S = UI.S;
    S.progress = p;
    const now = Date.now();
    for (const [mod, [done, total]] of Object.entries(p)) {
      const t = S.modT[mod] || (S.modT[mod] = { start: now, end: 0 });
      if (!t.end && total > 0 && done >= total) t.end = now;
    }
    UI.paintRun();
  };

  UI.onDone = function onDone(rep) {
    const S = UI.S;
    S.report = rep;
    S.target = rep.target || S.target;
    if (rep.identities) { S.identities = rep.identities; S.ambiguity = rep.ambiguity || ""; }
    // Authoritative per-collector durations replace the client-side stopwatch.
    S.dur = {};
    for (const m of rep.modules || []) S.dur[m.name] = (S.dur[m.name] || 0) + (m.duration_ms || 0);
    UI.setState(rep.interrupted ? "cancelled" : "done");
    renderTargetChips();
    UI.paintRun();
    UI.scheduleRender(true);
    UI.refreshHistory();
  };

  UI.onFailed = function onFailed(msg) {
    UI.S.failMsg = msg;
    UI.setState("failed");
    UI.paintRun();
    UI.refreshHistory();
  };

  UI.gapCount = () => {
    const S = UI.S;
    if (!S) return 0;
    return S.report ? (S.report.failures || []).length : S.errs;
  };

  // ------------------------------------------------------- live collector rows
  // Rows are keyed by collector and updated in place, so nothing jumps while
  // results stream in. Collectors are listed in the fixed MODULE_ORDER.
  UI.paintRun = function paintRun() {
    const S = UI.S;
    if (!S) return;
    const bars = $("bars");
    const names = [...new Set([...Object.keys(S.progress), ...S.mod.keys()])].sort((a, b) => UI.modRank(a) - UI.modRank(b) || a.localeCompare(b));
    let sum = 0, nTracked = 0, finished = 0;
    const now = Date.now();
    for (const mod of names) {
      const [done, total] = S.progress[mod] || [0, 0];
      const cnt = S.mod.get(mod) || { n: 0, hit: 0, err: 0 };
      let row = bars.querySelector('[data-mod="' + CSS.escape(mod) + '"]');
      if (!row) {
        row = el("div", { class: "crow", "data-mod": mod, role: "listitem" },
          el("span", { class: "c-ico" }), el("span", { class: "c-name", text: mod }),
          el("span", { class: "meter" }, el("span", { class: "meter-fill" })),
          el("span", { class: "c-prog num" }), el("span", { class: "c-hit num" }), el("span", { class: "c-err num" }), el("span", { class: "c-time num" }));
        bars.append(row);
        // keep fixed collector order even when a new one appears late
        [...bars.children].sort((a, b) => UI.modRank(a.dataset.mod) - UI.modRank(b.dataset.mod) || a.dataset.mod.localeCompare(b.dataset.mod)).forEach((r) => bars.append(r));
      }
      const frac = total > 0 ? Math.min(1, done / total) : 0;
      const isDone = total > 0 && done >= total;
      const t = S.modT[mod];
      const stopped = S.state !== "running";
      const state = isDone || (stopped && S.state === "done") ? "done" : stopped ? "idle" : "run";
      row.className = "crow st-" + state + (cnt.err ? " has-err" : "");
      row.querySelector(".c-ico").replaceChildren(UI.icon(state === "done" ? "check" : state === "run" ? "pulse" : "circle"));
      row.querySelector(".meter-fill").style.width = Math.round((state === "done" && S.state === "done" ? 1 : frac) * 100) + "%";
      row.querySelector(".c-prog").textContent = total ? done + "/" + total : "·";
      row.querySelector(".c-hit").textContent = cnt.hit + " hit" + (cnt.hit === 1 ? "" : "s");
      const errEl = row.querySelector(".c-err");
      errEl.textContent = cnt.err ? cnt.err + " err" : "";
      errEl.title = cnt.err ? "checks that could not complete" : "";
      const ms = S.dur && S.dur[mod] != null ? S.dur[mod] : t ? (t.end || now) - t.start : 0;
      row.querySelector(".c-time").textContent = ms ? UI.dur(ms) : "";
      sum += state === "done" && S.state === "done" ? 1 : frac;
      nTracked++;
      if (isDone || S.state === "done") finished++;
    }
    const overall = S.state === "done" ? 1 : nTracked ? sum / nTracked : 0;
    $("overallFill").style.width = Math.round(overall * 100) + "%";
    const gaps = UI.gapCount();
    const word = { running: "Collecting", done: "Complete", cancelled: "Stopped", failed: "Failed" }[S.state];
    $("overallText").textContent = S.state === "failed" ? "Failed: " + (S.failMsg || "unknown error")
      : `${word} · ${finished}/${nTracked || "…"} collectors · ${UI.n(S.findings.length)} findings · ${UI.n(S.entities.size)} entities`;
    const gp = $("gapPill");
    gp.hidden = gaps === 0;
    gp.replaceChildren(UI.icon("alert"), el("span", { text: UI.n(gaps) + (gaps === 1 ? " coverage gap" : " coverage gaps") }));
    $("elapsed").textContent = UI.dur((S.report ? Date.parse(S.report.finished_at) - Date.parse(S.report.started_at) : now - S.created) || 0);
    // Collapsed by default once finished, unless the analyst chose otherwise.
    const pref = UI.store.get("live", "auto");
    const open = pref === "open" || (pref === "auto" && S.state === "running");
    $("livePanel").hidden = !open;
    $("liveToggle").setAttribute("aria-expanded", String(open));
    UI.updatePill();
  };
  setInterval(() => { if (UI.S && UI.S.state === "running") UI.paintRun(); }, 1000);

  // Tabs and sections are handled by nav.js (two-level navigation).

  // Rendering is debounced while findings stream in; explicit actions render now.
  UI.scheduleRender = (() => {
    let timer = null;
    return function schedule(now) {
      if (now) { clearTimeout(timer); timer = null; UI.render(); return; }
      if (timer) return;
      timer = setTimeout(() => { timer = null; UI.render(); }, 450);
    };
  })();

  UI.render = function render() {
    const S = UI.S;
    if (!S) return;
    UI.paintRun();
    UI.navRender();
  };

  // ------------------------------------------------------------------ photos
  UI.imgSrc = (u) => "/api/img?u=" + encodeURIComponent(u);

  // Pictures found so far: live from image entities, enriched with size / hash /
  // "same picture" group / EXIF once the final report is in.
  UI.photoList = function photoList() {
    const S = UI.S;
    if (!S) return [];
    const rep = new Map(((S.report && S.report.photos) || []).map((p) => [p.url, p]));
    const out = [];
    for (const e of S.entities.values()) {
      if (e.type !== "image") continue;
      const r = rep.get(e.value) || {};
      const sources = (e.findings || []).map((f) => f.source);
      out.push({
        url: e.value, sources: r.sources && r.sources.length ? r.sources : sources, group: r.group || 0, width: r.width, height: r.height,
        bytes: r.bytes, sha256: r.sha256, dhash: r.dhash, error: r.error, gps: r.gps, taken: r.taken, camera: r.camera, meta: r.meta,
      });
    }
    out.sort((a, b) => (b.group > 0) - (a.group > 0) || a.group - b.group || (a.error ? 1 : 0) - (b.error ? 1 : 0));
    return out;
  };

  // Fill the case form from a candidate identity and run again: the way to turn
  // "several people match" into one confident answer.
  UI.refine = function refine(id) {
    const first = (facts) => (facts && facts[0] ? facts[0].value : "");
    const set = (name, value) => { const f = $("f_" + name); if (f && value) f.value = value; };
    set("full_name", first(id.names));
    set("location", first(id.locations));
    set("company", first(id.employers));
    set("email", first(id.emails));
    set("username", (id.usernames || []).map((u) => u.value.split(":")[1]).find(Boolean));
    set("domain", (id.websites || []).map((w) => w.value)[0]);
    UI.showPanel(true);
    UI.panelTab("new");
    UI.showFormError("");
    $("form").dispatchEvent(new Event("input", { bubbles: true }));
    $("startBtn").focus();
    UI.toast("Form filled from this person: review and start");
  };

  UI.resetViews = function resetViews() {
    document.querySelectorAll(".tabpanel").forEach((p) => p.replaceChildren());
    // the graph panel owns static controls; its view re-creates its own content
    for (const fn of UI.resetHooks || []) fn();
  };
  UI.resetHooks = [];

  // ----------------------------------------------------------------- wiring
  $("cancelBtn").addEventListener("click", async () => {
    if (!UI.S) return;
    try { await UI.api("/api/scans/" + UI.S.id + "/cancel", { method: "POST", body: "{}" }); UI.toast("Stopping collectors…"); } catch (e) { UI.toast(e.message); }
  });
  $("liveToggle").addEventListener("click", () => {
    const open = $("livePanel").hidden;
    UI.store.set("live", open ? "open" : "closed");
    UI.paintRun();
  });
  $("exportBtn").addEventListener("click", () => {
    if (!UI.S || $("exportBtn").disabled) return;
    const list = el("div", { class: "menu", role: "menu" });
    for (const [fmt, label, title, ext] of UI.EXPORTS) {
      const item = el("a", { class: "menu-item", role: "menuitem", href: `/api/scans/${UI.S.id}/export?format=${fmt}`, download: "", title },
        UI.icon("download"), el("span", { text: label }), el("span", { class: "menu-sub", text: "." + ext }));
      item.addEventListener("click", () => UI.exportLog && UI.exportLog.add(fmt, label));
      list.append(item);
    }
    UI.popover($("exportBtn"), list, { alignRight: true, role: "menu" });
  });
})();
