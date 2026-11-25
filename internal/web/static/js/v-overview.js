"use strict";

// Copyright (C) 2026 Seif, SUDOSOC. SPDX-License-Identifier: AGPL-3.0-only

// Overview tab: a dashboard. KPI strip, key facts, risk signals ranked by
// severity, reliability distribution, per-collector coverage, activity heatmap
// and timeline. Counts always drill down into the findings table.
(function () {
  const { $, el } = UI;

  const TYPE_ORDER = ["name", "username", "email", "phone", "domain", "ip", "organisation", "location", "url", "asn", "wallet"];
  const ACCOUNT_CATS = new Set(["social", "profile", "email-account", "social-links"]);
  const SEV = { high: { rank: 0, label: "High", icon: "alert" }, med: { rank: 1, label: "Medium", icon: "uncertain" }, info: { rank: 2, label: "Info", icon: "check" } };

  // Turn a finding into a risk signal, or null. Search-engine hits are leads,
  // not confirmed exposure, so they never raise a flag.
  function signalFor(f) {
    const d = f.details || {};
    if (f.module === "dorks") return null;
    const hit = f.status === "found" || f.status === "uncertain";
    switch (f.category) {
      case "exposure":
        return { sev: "high", title: f.source, detail: Object.entries(d).filter(([k]) => k !== "note").slice(0, 2).map(([k, v]) => k + ": " + UI.clip(v, 80)).join(" · ") };
      case "takeover":
        return { sev: "high", title: f.source, detail: "CNAME → " + (d["CNAME target"] || "") + " (" + (d.platform || "") + ")" };
      case "sanctions":
        return hit ? { sev: "high", title: "Possible sanctions / watch-list match: verify", detail: f.source + " · " + UI.clip((Object.values(d)[0] || "").split("\n")[0], 100) } : null;
      case "lookalike":
        return hit ? { sev: "med", title: f.source, detail: "look-alike domain" + (d.registered ? " · registered " + d.registered : "") } : null;
      case "breach":
        return hit ? { sev: "med", title: f.source, detail: UI.clip(Object.values(d)[0] || "appears in a known breach", 120) } : null;
      case "reputation":
        if (d["internet noise (mass scanner)"] === "yes") return { sev: "med", title: "IP is internet background noise / scanner", detail: d.classification || d.message || "" };
        if (d["known benign service (RIOT)"] === "yes") return { sev: "info", title: "IP belongs to a known benign service", detail: d["actor / service"] || "" };
        return null;
      case "email-security":
        if (/missing|permissive/i.test((d.SPF || "") + (d["SPF policy"] || "") + (d.DMARC || ""))) return { sev: "med", title: "Mail spoofing possible", detail: [d["SPF policy"] || d.SPF, d.DMARC === "missing" ? "DMARC missing" : ""].filter(Boolean).join(" · ") };
        if (d["DMARC policy"] === "none") return { sev: "med", title: "DMARC in monitor-only mode", detail: "policy p=none does not block spoofed mail" };
        return null;
      case "security":
        if ((d.missing || "").split(",").filter(Boolean).length >= 3) return { sev: "med", title: "Several security headers missing", detail: d.missing };
        if (d.transport) return { sev: "med", title: "Site not forced to HTTPS", detail: d.transport };
        return null;
      case "tls":
        if (/^EXPIRED/.test(d.status || "")) return { sev: "high", title: "TLS certificate expired", detail: d.status };
        if (/^expires in/.test(d.status || "")) return { sev: "med", title: "TLS certificate expires soon", detail: d.status };
        if (d.note) return { sev: "med", title: "Self-signed TLS certificate", detail: d.subject || "" };
        return null;
      default:
        return null;
    }
  }
  UI.signalFor = signalFor;

  function kpi(label, n, opts, onclick) {
    const b = el("button", { type: "button", class: "kpi" + (opts.warn ? " warn" : "") + (opts.lead ? " lead" : ""), "aria-label": `${UI.n(n)} ${label}` },
      el("span", { class: "n num", text: UI.n(n) }), el("span", { class: "l" }, opts.icon ? UI.icon(opts.icon) : null, label));
    b.addEventListener("click", onclick);
    return b;
  }

  function panel(title, icon, ...kids) {
    return el("section", { class: "panel" }, el("h3", { class: "ph" }, icon ? UI.icon(icon) : null, title), ...kids);
  }

  function overview() {
    const S = UI.S, root = $("tab-overview");
    root.replaceChildren();
    const ents = [...S.entities.values()];
    const byType = (t) => ents.filter((e) => e.type === t);
    const accounts = S.findings.filter((f) => ACCOUNT_CATS.has(f.category) && f.url).length;
    const subs = S.findings.filter((f) => f.category === "subdomain" && f.url).length;
    const signals = S.findings.map((f) => ({ f, s: signalFor(f) })).filter((x) => x.s).sort((a, b) => SEV[a.s.sev].rank - SEV[b.s.sev].rank);
    const risky = signals.filter((x) => x.s.sev !== "info").length;
    const gaps = UI.gapCount();

    if (!S.findings.length && S.state !== "running") {
      root.append(UI.emptyState("layers", "Nothing to summarise", "This investigation produced no findings. Check the Coverage tab for checks that could not run."));
      return;
    }

    // ---- KPI strip
    const kp = el("div", { class: "kpis" });
    kp.append(
      kpi("findings", S.findings.length, { lead: true, icon: "list" }, () => UI.gotoFindings()),
      kpi("entities", ents.length, { icon: "hash" }, () => UI.selectTab("tree")),
      kpi("accounts", accounts, { icon: "user" }, () => UI.gotoFindings({ cats: ["social"] })),
      kpi("e-mails", byType("email").length, { icon: "mail" }, () => UI.selectTab("tree")),
      kpi("domains", byType("domain").length + subs, { icon: "globe" }, () => UI.gotoFindings({ cats: ["subdomain"] })),
      kpi("IPs", byType("ip").length, { icon: "server" }, () => UI.selectTab("tree")),
      kpi("photos", byType("image").length, { icon: "image" }, () => UI.selectTab("photos")),
      kpi("risk signals", risky, { warn: risky > 0, icon: "alert" }, () => $("sigPanel") && $("sigPanel").scrollIntoView({ behavior: "smooth", block: "start" })),
      kpi("coverage gaps", gaps, { warn: gaps > 0, icon: "shield" }, () => UI.selectTab("coverage")),
    );
    root.append(kp);

    const grid = el("div", { class: "dash" });

    // ---- key facts
    const facts = panel("Key facts about the subject", "id");
    facts.classList.add("d-facts");
    const rows = el("div", { class: "kfacts" });
    let any = false;
    const photos = UI.photoList().filter((p) => !p.error);
    if (photos.length) {
      any = true;
      const th = el("div", { class: "thumbs" });
      for (const p of photos.slice(0, 8)) {
        const im = el("img", { src: UI.imgSrc(p.url), alt: "" });
        const b = el("button", { type: "button", class: "thumb", title: (p.sources || []).join(" · "), "aria-label": "Open photo" }, im);
        im.addEventListener("error", () => { b.remove(); if (!th.querySelector(".thumb")) th.closest(".krow") && th.closest(".krow").remove(); }); // host removed the picture or blocked the download
        b.addEventListener("click", () => UI.showPhoto(p));
        th.append(b);
      }
      if (photos.length > 8) { const more = UI.btn("+" + (photos.length - 8), { cls: "sm" }); more.addEventListener("click", () => UI.selectTab("photos")); th.append(more); }
      rows.append(el("div", { class: "krow" }, el("div", { class: "k" }, UI.icon("image"), "Photos"), th));
    }
    for (const t of TYPE_ORDER) {
      const list = byType(t).sort((a, b) => (b.strong - a.strong) || a.value.localeCompare(b.value));
      if (!list.length) continue;
      any = true;
      const chips = el("div", { class: "chips" });
      for (const e of list.slice(0, 8)) chips.append(UI.entityChip(e));
      if (list.length > 8) {
        const more = UI.btn(`+${list.length - 8} more`, { cls: "sm" });
        more.addEventListener("click", () => UI.selectTab("tree"));
        chips.append(more);
      }
      rows.append(el("div", { class: "krow" }, el("div", { class: "k" }, UI.icon(UI.type(t).icon), UI.type(t).label, el("span", { class: "count", text: String(list.length) })), chips));
    }
    if (!any) rows.append(el("p", { class: "hint", text: S.state === "running" ? "Collecting… leads appear here as they are found." : "No new entities were discovered." }));
    facts.append(rows);

    // ---- risk signals
    const sig = panel("Risk signals", "alert");
    sig.querySelector(".ph").append(el("span", { class: "ph-sub", text: "ranked by severity" }));
    sig.id = "sigPanel";
    sig.classList.add("d-signals");
    if (!signals.length) sig.append(el("p", { class: "hint", text: S.state === "running" ? "Nothing notable yet." : "No risk signals detected in the collected data." }));
    const list = el("div", { class: "signals" });
    for (const { f, s } of signals.slice(0, 12)) {
      const sv = SEV[s.sev];
      const row = el("button", { type: "button", class: "signal" },
        el("span", { class: "sev sev-" + s.sev }, UI.icon(sv.icon), sv.label),
        el("span", { class: "sg-main" }, el("span", { class: "sg-t", text: UI.clip(s.title, 90) }), el("span", { class: "sg-d", text: UI.clip(s.detail, 140) })),
        el("span", { class: "tag", text: f.module }));
      row.addEventListener("click", () => UI.showFinding(f));
      list.append(row);
    }
    sig.append(list);
    if (signals.length > 12) sig.append(el("p", { class: "hint", text: `${signals.length - 12} more lower-ranked signals are in the findings table.` }));

    // ---- reliability distribution
    const dist = { hi: 0, mid: 0, lo: 0 };
    for (const f of S.findings) dist[UI.tier(UI.pct(f.confidence))]++;
    const total = Math.max(1, S.findings.length);
    const dd = el("div", { class: "dist" });
    for (const [k, lab, range] of [["hi", "High", "≥ 80%"], ["mid", "Medium", "50–79%"], ["lo", "Low", "< 50%"]]) {
      const fill = el("span", { class: "meter-fill" });
      fill.style.width = Math.round((dist[k] / total) * 100) + "%";
      const row = el("button", { type: "button", class: "drow" }, el("span", { class: "dl" }, el("b", { text: lab }), el("span", { class: "muted", text: range })),
        el("span", { class: "meter t-" + k }, fill), el("span", { class: "num", text: UI.n(dist[k]) }), el("span", { class: "num muted", text: Math.round((dist[k] / total) * 100) + "%" }));
      row.addEventListener("click", () => UI.gotoFindings({ conf: k === "hi" ? 0.8 : k === "mid" ? 0.5 : 0 }));
      dd.append(row);
    }
    const rel = panel("How reliable is it?", "shield", dd, el("p", { class: "hint", text: "High = verified by an official source or cross-checked. Low = a name/handle match that may belong to someone else." }));
    rel.classList.add("d-rel");

    // ---- per-collector coverage
    const cov = panel("Collector coverage", "layers");
    cov.classList.add("d-cov");
    const runs = new Map();
    for (const m of (S.report && S.report.modules) || []) {
      const r = runs.get(m.name) || { checked: 0, found: 0, errors: 0 };
      r.checked += m.checked || 0; r.found += m.found || 0; r.errors += m.errors || 0;
      runs.set(m.name, r);
    }
    if (!runs.size) for (const [m, c] of S.mod) runs.set(m, { checked: c.n, found: c.hit, errors: c.err });
    const cl = el("div", { class: "covbars" });
    const cmax = Math.max(1, ...[...runs.values()].map((r) => Math.max(r.checked, r.found + r.errors)));
    for (const [m, r] of [...runs.entries()].sort((a, b) => b[1].found - a[1].found || b[1].checked - a[1].checked)) {
      const basis = Math.max(r.checked, r.found + r.errors);
      const track = el("span", { class: "cov-track" });
      track.style.width = Math.max(4, Math.round((basis / cmax) * 100)) + "%";
      const seg = (cls, n) => { const s = el("span", { class: cls }); s.style.width = (basis ? (n / basis) * 100 : 0) + "%"; return s; };
      track.append(seg("seg-hit", r.found), seg("seg-err", r.errors));
      const row = el("button", { type: "button", class: "covrow", title: `${m}: ${r.found} hits, ${r.errors} errors, ${r.checked} checked` },
        el("span", { class: "mono cv-name", text: m }), el("span", { class: "cv-bar" }, track),
        el("span", { class: "num cv-n", text: `${UI.n(r.found)}` }), el("span", { class: "num cv-e" + (r.errors ? " has" : ""), text: r.errors ? UI.n(r.errors) + " err" : "" }));
      row.addEventListener("click", () => UI.gotoFindings({ mods: [m] }));
      cl.append(row);
    }
    if (!runs.size) cl.append(el("p", { class: "hint", text: "No collector output yet." }));
    cov.append(cl, el("div", { class: "cov-legend hint" }, el("span", { class: "lg lg-hit" }), "hits", el("span", { class: "lg lg-err" }), "errors", el("span", { class: "lg lg-rest" }), "checked, nothing found"));

    // ---- activity heatmap
    const act = panel("Activity pattern", "pulse");
    act.classList.add("d-act");
    const tz = S.report && S.report.geo && S.report.geo.timezone;
    const hm = tz && UI.heatmap(tz);
    if (hm) act.append(hm);
    else act.append(el("p", { class: "hint", text: S.state === "running" ? "Activity hours are analysed when the scan finishes." : "No activity pattern: it needs a username with public activity (GitHub, Hacker News, Wikipedia…)." }));

    // ---- timeline
    const tlp = panel("Timeline", "clock");
    tlp.classList.add("d-tl");
    const tl = (S.report && S.report.timeline) || [];
    if (tl.length) {
      tlp.append(UI.miniTimeline(tl));
      const more = UI.btn("Open full timeline", { cls: "sm ghost" });
      more.addEventListener("click", () => UI.selectTab("identity"));
      tlp.append(more);
    } else tlp.append(el("p", { class: "hint", text: "No dated events yet." }));

    grid.append(facts, sig, rel, cov, act, tlp);
    root.append(grid);
  }

  UI.views.overview = overview;
})();
