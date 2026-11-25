"use strict";

// Timeline: events, activity patterns, domain history, changes between runs.
(function () {
  const { el } = UI;
  const K = UI.kit;

  const events = { id: "events", label: "Event timeline", icon: "clock", count: (M) => M.timeline.length,
    render(page, M) {
      const tl = [...M.timeline].sort((a, b) => a.date.localeCompare(b.date));
      if (!tl.length) { page.append(K.empty("clock", "No dated events yet", "The timeline is built when the investigation finishes from every dated fact: account creation, registrations, first archive captures, pictures.", "run with a username, domain or e-mail so dated facts exist.")); return; }
      page.append(K.stats([[tl.length, "events", { lead: true }], [tl[0].date, "earliest"], [tl[tl.length - 1].date, "latest"], [new Set(tl.map((e) => e.source)).size, "sources"]]));
      page.append(UI.timelinePanel(tl));
      page.append(K.panel("Every event", "list", K.opts({ count: tl.length }), K.table([{ label: "Date", get: (e) => e.date }, { label: "Event", get: (e) => e.event }, { label: "Source", get: (e) => e.source }, { label: "Link", get: (e) => { const h = e.url && UI.safeHref(e.url); return h ? el("a", { href: h, target: "_blank", rel: "noopener noreferrer", text: UI.host(e.url) }) : ""; } }], [...tl].reverse())));
    } };

  const activity = { id: "activity", label: "Account activity", icon: "pulse", count: (M) => M.pick({ mods: ["activity"] }).length,
    render(page, M) {
      const list = M.pick({ mods: ["activity"] });
      if (M.tz) {
        page.append(K.stats([[M.tz.label, "estimated time zone", { lead: true }], [UI.pct(M.tz.confidence) + "%", "confidence"], [M.tz.events, "public events read"], [(M.tz.regions || []).slice(0, 2).join(", ") || "—", "likely regions"]]));
        const h = UI.heatmap(M.tz); if (h) page.append(K.panel("Weekday × hour", "clock", h));
        const hc = UI.hoursChart(M.tz); if (hc) page.append(K.panel("By hour", "pulse", hc));
      }
      page.append(K.findings("Per platform", "pulse", list, { empty: "No account had enough public events (10 or more) to read a pattern.", showModule: false }));
    } };

  const DATEKEY = /(creat|regist|expir|updat|first|last|valid|not.?before|not.?after|born|joined|since|issued|captur|latest)/i;
  const DATEVAL = /\b(\d{4}-\d{2}-\d{2})/;
  function domainRows(M) {
    const rows = [];
    for (const f of M.hits) {
      if (!["domain", "ip", "asn", "webhistory", "company", "bizreg", "records"].includes(f.module) && !["whois", "tls", "archive", "history", "tracking"].includes(f.category)) continue;
      for (const [k, v] of Object.entries(f.details || {})) {
        if (!DATEKEY.test(k)) continue;
        const m = String(v).match(DATEVAL);
        if (m) rows.push({ date: m[1], what: k, value: UI.clip(String(v).split("\n")[0], 90), source: f.source, f });
      }
      if (f.module === "webhistory" && f.details["IDs by period seen"]) rows.push({ date: "", what: "analytics IDs by period", value: UI.clip(f.details["IDs by period seen"].split("\n").slice(0, 3).join(" | "), 160), source: f.source, f });
    }
    return rows.sort((a, b) => (a.date || "9").localeCompare(b.date || "9"));
  }
  const domain = { id: "domain", label: "Domain history", icon: "globe", count: (M) => domainRows(M).length,
    render(page, M) {
      const rows = domainRows(M);
      page.append(K.stats([[rows.length, "dated facts", { lead: true }], [new Set(rows.map((r) => r.source)).size, "sources"]]));
      page.append(K.panel("Registration, certificates, archives and company facts", "globe", K.opts({ count: rows.length }), K.table([{ label: "Date", get: (r) => r.date || "over time" }, { label: "What", get: (r) => r.what }, { label: "Value", get: (r) => r.value }, { label: "Source", get: (r) => r.source }], rows, { onRow: (r) => UI.showFinding(r.f), empty: "No dated domain facts. Give a domain." })));
    } };

  let prev = { id: "", list: null, err: "" };
  const changes = { id: "changes", label: "Historical changes", icon: "layers", always: true,
    render(page, M) {
      page.append(K.hint("Runs of the same target are saved on this machine. Compare them to see what appeared or disappeared: new accounts, domains that dropped a tracker, people who changed employer."));
      const webChanges = M.hits.filter((f) => f.module === "webhistory" && f.details && f.details["no longer present in the latest capture"]);
      if (webChanges.length) page.append(K.panel("Changes inside the archived history", "clock", ...webChanges.map((f) => el("div", { class: "hint" }, f.details.domain + ": " + f.details["no longer present in the latest capture"]))));
      const box = el("div", {});
      page.append(K.panel("Earlier runs of this target", "folder", box));
      const draw = (list) => {
        const label = UI.targetLabel(M.target);
        const rows = (list || []).filter((c) => c.id !== UI.S.id && c.title === label);
        if (!rows.length) { box.replaceChildren(el("p", { class: "empty-line", text: "No earlier run of this target is saved." })); return; }
        const open = UI.btn("Open the comparison view", { cls: "sm", icon: "layers" });
        open.addEventListener("click", () => UI.showCases());
        box.replaceChildren(K.table([{ label: "When", get: (c) => UI.when(c.at) }, { label: "Findings", cls: "r", get: (c) => UI.n(c.findings) }, { label: "Change vs this run", cls: "r", get: (c) => { const d = M.hits.length - c.findings; return (d > 0 ? "+" : "") + d; } }, { label: "People", cls: "r", get: (c) => c.identities }, { label: "Best match", get: (c) => c.best || "" }], rows), open);
      };
      if (prev.list && prev.id === UI.S.id) draw(prev.list);
      else UI.api("/api/cases").then((l) => { prev = { id: UI.S.id, list: l }; draw(l); }).catch((e) => box.replaceChildren(el("p", { class: "empty-line", text: "Could not list saved cases: " + e.message })));
    } };

  const correlationSec = { id: "correlation", label: "Event correlation", icon: "network", count: (M) => bursts(M).length,
    render(page, M) {
      const b = bursts(M);
      page.append(K.stats([[b.length, "periods with activity from 2+ sources", { lead: true }]]));
      page.append(K.hint("Months where several independent sources report dated events at the same time: a new job, a move, a new project, an incident."));
      page.append(K.panel("Busy periods", "network", K.opts({ count: b.length }), K.table([{ label: "Month", get: (x) => x.m }, { label: "Events", cls: "r", get: (x) => x.n }, { label: "Sources", get: (x) => [...x.src].slice(0, 5).join(" · ") }, { label: "What", get: (x) => x.ev.slice(0, 3).join(" · ") }], b, { empty: "No month has events from more than one source." })));
    } };
  function bursts(M) {
    const m = new Map();
    for (const e of M.timeline) { const k = (e.date || "").slice(0, 7); if (k.length < 7) continue; const r = m.get(k) || m.set(k, { m: k, n: 0, src: new Set(), ev: [] }).get(k); r.n++; r.src.add(e.source); r.ev.push(e.event); }
    return [...m.values()].filter((r) => r.src.size >= 2).sort((a, b) => b.m.localeCompare(a.m));
  }

  const period = K.section("period", "Period you gave", "clock", (M) => M.hits.filter((f) => f.module === "context" && f.category !== "geo" && f.category !== "geo-media"), { showModule: false,
    intro: "News that names the subject in the period, the closest Internet Archive copy of the domain, what happened on those days, and searches limited to the period to open yourself.",
    empty: "No period was supplied.", unlock: "enter a From date (and optionally To) in the form, or use -date-from / -date-to." });
  UI.defineTab({ id: "timeline", label: "Timeline", icon: "clock", count: (M) => M.timeline.length, sections: [events, period, activity, domain, changes, correlationSec] });
})();
