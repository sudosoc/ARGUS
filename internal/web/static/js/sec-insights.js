"use strict";
// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

// Insights: what the evidence adds up to. Everything here is computed by fixed rules on the server
// (internal/insights) when the investigation finishes: a brief, next actions, a persona, how authentic the
// identity looks, who is around the subject, links nobody stated, a life timeline with anomalies, movement
// and the reasoning behind each fact. Nothing is invented: each line points at evidence in the report.
(function () {
  const { el } = UI;
  const K = UI.kit;
  const INS = (M) => (M.rep && M.rep.insights) || null;
  const pct = (x) => Math.round((x || 0) * 100);
  const kindLabel = { narrow: "Narrow", verify: "Verify", resolve: "Resolve", collect: "Collect", follow: "Follow", defend: "Defend", rerun: "Re-run" };
  const LEVEL = { strong: "Strong match", moderate: "Moderate match", weak: "Weak match", none: "No single person" };

  const wait = (page) => page.append(K.empty("clock", "Insights are written when the investigation finishes", "They are derived from the complete evidence, so they appear as soon as the run ends.", "wait for the run to finish, or open a saved case."));
  const none = (page, icon, title, text, unlock) => page.append(K.empty(icon, title, text, unlock));
  const section = (id, label, icon, render, count) => ({ id, label, icon, group: "Conclusions", always: true, count,
    render(page, M) { const I = INS(M); if (!I) { wait(page); return; } render(page, M, I); } });
  const tag = (text, cls) => el("span", { class: "tag" + (cls ? " " + cls : ""), text });
  const list = (items) => el("ul", { class: "difflist" }, ...items.map((t) => el("li", {}, t)));

  // ---------------------------------------------------------------- brief
  const brief = section("brief", "Brief", "eye", (page, M, I) => {
    const b = I.brief;
    page.append(K.panel(b.headline || "Brief", "eye", el("div", { class: "brief-lvl" }, tag(LEVEL[b.level] || b.level, b.level === "strong" ? "" : "bad")), ...(b.paragraphs || []).map((p) => el("p", { class: "brief-p", text: p }))));
    if ((b.strengths || []).length) page.append(K.panel("What supports it", "check", list(b.strengths)));
    if ((b.concerns || []).length) page.append(K.panel("What to be careful about", "alert", list(b.concerns)));
    if ((I.actions || []).length) {
      const go = UI.btn("See all next actions", { cls: "sm", icon: "list" });
      go.addEventListener("click", () => UI.go("insights", "actions"));
      page.append(K.panel("Do this first", "list", ...I.actions.slice(0, 3).map(actionCard), go));
    }
    page.append(K.hint((I.limits || []).join(" ")));
  }, (M) => (INS(M) ? 1 : 0));

  // -------------------------------------------------------------- actions
  function actionCard(a) {
    const link = a.url ? (() => { const h = UI.safeHref(a.url); return h ? el("a", { class: "btn sm", href: h, target: "_blank", rel: "noopener noreferrer" }, UI.icon("external"), el("span", { text: "Open" })) : null; })() : null;
    return el("div", { class: "act" },
      el("div", { class: "act-h" }, tag(kindLabel[a.kind] || a.kind), el("b", { text: a.title }), el("span", { class: "act-p num", title: "priority", text: String(a.priority) })),
      el("p", { class: "act-w", text: a.why }),
      el("p", { class: "hint", text: "How: " + a.how }),
      el("div", { class: "act-f" }, el("span", { class: "hint", text: "Improves: " + a.gain }), link));
  }
  const actions = section("actions", "Next actions", "list", (page, M, I) => {
    if (!(I.actions || []).length) { none(page, "check", "Nothing more to do", "The evidence is as complete as the supplied inputs allow."); return; }
    page.append(K.hint("Ordered by how much each step improves the result. The number is the priority (1–100)."));
    page.append(K.panel("What to do next", "list", K.opts({ count: I.actions.length }), ...I.actions.map(actionCard)));
  }, (M) => (INS(M) ? (INS(M).actions || []).length : 0));

  // -------------------------------------------------------------- persona
  const persona = section("persona", "Persona", "id", (page, M, I) => {
    const p = I.persona;
    if (!p) { none(page, "id", "No persona yet", "A persona needs a resolved person with public text, code or activity.", "give a username with public activity, or a name plus a place."); return; }
    page.append(K.stats([[p.first_trace ? p.first_trace.slice(0, 4) : "—", "first seen online", { lead: true }], [p.years_online ? p.years_online + " y" : "—", "online for"], [p.accounts || 0, "platforms"], [(p.languages || []).length || "—", "languages"]]));
    if ((p.languages || []).length) page.append(K.panel("Languages written", "list", K.bars(p.languages.map((l) => [l.label, pct(l.share), l.note || ""]), { max: 100 })));
    if ((p.expertise || []).length) page.append(K.panel("Skills and interests", "code", K.bars(p.expertise.map((x) => [x.label, x.n, x.note || ""]))));
    if (p.schedule) {
      const s = p.schedule;
      const rows = [["Time zone", s.zone], ["Quietest hours (probably asleep)", String(s.sleep_from).padStart(2, "0") + ":00 – " + String(s.sleep_to).padStart(2, "0") + ":00 local"], ["Most active hour", String(s.peak_hour).padStart(2, "0") + ":00 local"],
        ["Night activity (22:00–05:00)", pct(s.night_share) + "%"], ["Weekend share", pct(s.weekend_share) + "%"], ["Based on", UI.n(s.events) + " public events"]];
      page.append(K.panel("Daily rhythm", "clock", K.kv(Object.fromEntries(rows)), K.hint("The person is " + s.pattern + ".")));
    }
    if ((p.habits || []).length) page.append(K.panel("Habits seen", "check", list(p.habits)));
    if ((p.exposure || []).length) page.append(K.panel("What their own habits give away", "alert", list(p.exposure), K.hint("This is the part someone can use against the person. It is also what to fix when checking your own exposure.")));
    if ((p.platforms || []).length) page.append(K.panel("Platforms", "globe", K.chips(p.platforms.map((x) => ({ v: x })))));
  }, (M) => (INS(M) && INS(M).persona ? 1 : 0));

  // --------------------------------------------------------- authenticity
  const authenticity = section("authenticity", "Authenticity", "shield", (page, M, I) => {
    const a = I.authenticity;
    if (!a) { none(page, "shield", "No authenticity estimate", "It needs a resolved person.", "give a name with a place or employer, or a username."); return; }
    page.append(K.panel("Does this look like an established real person?", "shield", K.gauge(a.score, a.label, "A set of observable signals, not a verdict. Each factor below says how many points it added or removed from a neutral 50.")));
    page.append(K.panel("Why this score", "list", K.table([{ label: "Signal", get: (f) => f.name }, { label: "Points", cls: "r", get: (f) => (f.effect > 0 ? "+" : "") + f.effect }, { label: "What was seen", get: (f) => f.detail }], a.factors)));
    if ((a.accounts || []).length) page.append(K.panel("Accounts: what is odd or reassuring", "user", K.table([{ label: "Platform", get: (x) => x.platform }, { label: "Notes", get: (x) => x.flags.join(" · ") }], a.accounts, { onRow: (x) => x.url && window.open(UI.safeHref(x.url) || "about:blank", "_blank", "noopener,noreferrer") })));
  }, (M) => (INS(M) && INS(M).authenticity ? 1 : 0));

  // ----------------------------------------------------------- associates
  const associates = section("associates", "Key associates", "network", (page, M, I) => {
    const l = I.associates || [];
    if (!l.length) { none(page, "network", "No associates found", "Associates come from public follow lists, organisations and schools stated in profiles.", "give a Bluesky/GitHub username, or a name with an employer."); return; }
    page.append(K.hint("Ranked by strength: more independent sources and reciprocity raise it. A public connection is a lead about who the subject interacts with, not proof of a relationship."));
    page.append(K.panel("Closest first", "network", K.opts({ count: l.length }), K.table([{ label: "Kind", get: (x) => x.kind }, { label: "Name", get: (x) => x.name }, { label: "Strength", get: (x) => K.conf(x.strength) }, { label: "Why", get: (x) => x.why }], l)));
  }, (M) => (INS(M) ? (INS(M).associates || []).length : 0));

  // ---------------------------------------------------------- hidden links
  const hidden = section("hidden", "Hidden links", "link", (page, M, I) => {
    const l = I.hidden_links || [];
    if (!l.length) { none(page, "link", "No hidden links found", "Nothing connects two accounts beyond what profiles state themselves.", "more accounts give more chances."); return; }
    page.append(K.hint("Pairs of accounts that nothing stated as linked but that share a handle pattern, distinctive wording, a value or a near-identical picture. Strong ones are worth confirming by hand."));
    page.append(K.panel("Possible links", "link", K.opts({ count: l.length }), K.table([{ label: "Account A", get: (x) => x.a }, { label: "Account B", get: (x) => x.b }, { label: "Kind", get: (x) => x.kind }, { label: "Strength", get: (x) => K.conf(x.strength) }, { label: "Evidence", get: (x) => x.evidence.join("; ") }], l)));
  }, (M) => (INS(M) ? (INS(M).hidden_links || []).length : 0));

  // ------------------------------------------------------------------ life
  const life = section("life", "Life timeline", "clock", (page, M, I) => {
    const L = I.life;
    if (!L) { none(page, "clock", "No dated facts", "The timeline is built from dated account creations, registrations, archive captures and pictures.", "give a username, domain or e-mail with dated facts."); return; }
    page.append(K.stats([[L.first ? L.first.slice(0, 4) : "—", "first trace", { lead: true }], [L.last ? L.last.slice(0, 4) : "—", "latest trace"], [L.events.length, "dated facts"], [(L.anomalies || []).length, "unusual stretches", { warn: (L.anomalies || []).length > 0 }]]));
    for (const a of L.anomalies || []) page.append(el("div", { class: "callout" + (a.kind === "recent" ? "" : " warn") }, UI.icon(a.kind === "recent" ? "check" : "alert"), el("span", {}, el("b", { text: a.kind + " · " + a.period + ". " }), a.text)));
    if ((L.per_year || []).length) page.append(K.panel("Facts per year", "clock", K.bars(L.per_year.map((y) => [String(y.year), y.n]))));
    page.append(K.panel("Chronology", "list", K.opts({ count: L.events.length }), K.table([{ label: "Date", get: (e) => e.date }, { label: "Event", get: (e) => e.event }, { label: "Source", get: (e) => e.source }], L.events)));
  }, (M) => (INS(M) && INS(M).life ? INS(M).life.events.length : 0));

  // -------------------------------------------------------------- movement
  const movement = section("movement", "Movement", "pin", (page, M, I) => {
    const m = I.movement;
    if (!m) { none(page, "pin", "No movement evidence", "Movement is read from pictures that carry GPS and a date, and from location evidence.", "find accounts that publish original pictures."); return; }
    page.append(K.stats([[m.home || "—", "home region", { lead: true }], [(m.places || []).length, "places"], [(m.trips || []).length, "moves"], [(m.countries || []).length, "countries"]]));
    if ((m.places || []).length) page.append(K.panel("Places", "pin", K.table([{ label: "Place", get: (p) => p.label }, { label: "Sightings", cls: "r", get: (p) => p.n }, { label: "First", get: (p) => p.first || "" }, { label: "Last", get: (p) => p.last || "" }, { label: "Evidence", get: (p) => (p.sources || []).slice(0, 3).join(" · ") }], m.places)));
    if ((m.trips || []).length) page.append(K.panel("Moves between places (from dated pictures)", "network", K.table([{ label: "Date", get: (t) => t.date }, { label: "From", get: (t) => t.from }, { label: "To", get: (t) => t.to }, { label: "Distance", cls: "r", get: (t) => UI.n(t.km) + " km" }], m.trips)));
    if ((m.countries || []).length) page.append(K.panel("Countries", "globe", K.chips(m.countries.map((c) => ({ v: c })))));
    page.append(K.hint((m.notes || []).join(" ")));
  }, (M) => (INS(M) && INS(M).movement ? (INS(M).movement.places || []).length : 0));

  // --------------------------------------------------------------- explain
  let exFilter = "";
  const explain = section("explain", "Why this confidence", "pulse", (page, M, I) => {
    const l = I.explain || [];
    if (!l.length) { none(page, "pulse", "Nothing to explain", "Explanations are written for each fact about the best-matching person.", "resolve a person first."); return; }
    const counts = {};
    for (const e of l) counts[e.level] = (counts[e.level] || 0) + 1;
    page.append(K.stats([[counts.confirmed || 0, "confirmed", { lead: true }], [counts["self-reported"] || 0, "self-reported"], [counts["single source"] || 0, "single source"], [counts.contested || 0, "contested", { warn: !!counts.contested }]]));
    const sel = el("select", { "aria-label": "Show" }, ...[["", "All facts"], ["confirmed", "Confirmed"], ["contested", "Contested"], ["self-reported", "Self-reported"], ["single source", "Single source"]].map(([v, t]) => el("option", { value: v, text: t, selected: v === exFilter ? "" : null })));
    const body = el("div", {});
    const draw = () => body.replaceChildren(K.table([{ label: "Fact", get: (e) => e.attribute }, { label: "Value", get: (e) => e.value }, { label: "Level", get: (e) => e.level }, { label: "Why it is trusted this much", get: (e) => e.why }, { label: "First trace", get: (e) => e.earliest || "" }], l.filter((e) => !exFilter || e.level === exFilter)));
    sel.addEventListener("change", () => { exFilter = sel.value; draw(); });
    page.append(K.panel("The reasoning behind each fact", "pulse", K.hint("Confirmed = two or more independent platforms state it. Self-reported = only the person's own profile. Single source = one third-party record. Contested = sources disagree."), el("div", { class: "sel-row" }, sel), body));
    draw();
  }, (M) => (INS(M) ? (INS(M).explain || []).length : 0));

  UI.defineTab({ id: "insights", label: "Insights", icon: "eye", count: (M) => (INS(M) ? (INS(M).actions || []).length : 0), sections: [brief, actions, persona, authenticity, associates, hidden, life, movement, explain] });
})();
