"use strict";

// Exposure & breaches. Only metadata is ever collected: breach names, dates,
// data types and counts. Never the leaked data or any credential.
(function () {
  const { el } = UI;
  const K = UI.kit;
  const split = (s) => String(s || "").split(/[,;]\s*/).map((x) => x.trim()).filter(Boolean);

  const isBreach = (f) => f.module === "breach" || f.category === "breach";
  const isStealer = (f) => /stealer/i.test(f.source + JSON.stringify(f.details || {}));

  const breaches = { id: "breaches", label: "Breach intelligence", icon: "alert", count: (M) => M.hits.filter(isBreach).length,
    render(page, M) {
      const list = M.hits.filter(isBreach);
      const names = new Set(list.flatMap((f) => split((f.details || {}).names)));
      page.append(K.stats([[names.size, "breaches named", { lead: true, warn: names.size > 0 }], [list.length, "breach findings"], [new Set(list.flatMap((f) => split((f.details || {})["years exposed"]))).size, "years of exposure"]]));
      page.append(K.hint("Breach names, dates and the kinds of data exposed come from a public breach index. SUDOSOC ARGUS never retrieves the leaked data or any password."));
      page.append(K.findings("Known breaches", "alert", list, { empty: "No breach found. Give an e-mail address: it is checked against a public breach index (absence is not proof of safety).", showModule: false }));
    } };

  const exposedMails = { id: "emails", label: "Exposed e-mails", icon: "mail", count: (M) => mailRows(M).filter((r) => r.exposed).length,
    render(page, M) {
      const rows = mailRows(M);
      page.append(K.stats([[rows.filter((r) => r.exposed).length, "exposed addresses", { lead: true, warn: rows.some((r) => r.exposed) }], [rows.length, "addresses checked"], [rows.filter((r) => r.stealer).length, "in stealer logs"]]));
      page.append(K.panel("E-mail addresses and where they surface", "mail", K.opts({ count: rows.length }), K.table([{ label: "Address", get: (r) => r.e.value }, { label: "Known breach", get: (r) => (r.breach ? "yes" : "no") }, { label: "Stealer logs", get: (r) => (r.stealer ? "yes" : "no") }, { label: "Other public traces", get: (r) => r.other.join(", ") || "—" }], rows, { onRow: (r) => UI.showEntity(r.e), empty: "No e-mail address to assess." })));
    } };
  function mailRows(M) {
    return M.entsOf("email").map((e) => { const fl = UI.mailFlags(M, e.value); return { e, breach: fl.includes("breach index"), stealer: fl.includes("stealer logs"), exposed: fl.includes("breach index") || fl.includes("stealer logs"), other: fl.filter((x) => x !== "breach index" && x !== "stealer logs") }; });
  }

  const dataTypes = { id: "datatypes", label: "Exposed data types", icon: "list", count: (M) => typeCounts(M).length,
    render(page, M) {
      const rows = typeCounts(M);
      page.append(K.stats([[rows.length, "kinds of data exposed", { lead: true }]]));
      page.append(K.hint("Only the categories (for example “Passwords”, “Phone numbers”) are listed: they say what an attacker could have, not the values."));
      page.append(K.panel("What kinds of data were exposed", "list", K.bars(rows.slice(0, 30)), rows.length ? null : K.hint("No breach reported data types.")));
    } };
  function typeCounts(M) {
    const c = new Map();
    for (const f of M.hits.filter(isBreach)) for (const t of split((f.details || {})["data types exposed"])) c.set(t, (c.get(t) || 0) + 1);
    return [...c.entries()].sort((a, b) => b[1] - a[1]);
  }

  const leaks = { id: "leaks", label: "Public data leaks", icon: "code", count: (M) => leakList(M).length,
    render(page, M) {
      const list = leakList(M);
      page.append(K.stats([[list.length, "exposure findings", { lead: true, warn: list.some((f) => f.category === "exposure" || f.category === "takeover") }], [list.filter((f) => f.module === "codesearch").length, "in public code"], [list.filter((f) => f.category === "takeover").length, "takeover candidates"]]));
      page.append(K.findings("Where data is exposed in public", "code", list, { empty: "No public exposure found. Domains are checked for sensitive archived URLs and dangling DNS; e-mails for stealer logs and public code." }));
    } };
  const leakList = (M) => M.hits.filter((f) => ["exposure", "takeover", "code"].includes(f.category) || f.module === "codesearch" || isStealer(f));

  const history = { id: "history", label: "Exposure history", icon: "clock", count: (M) => histRows(M).length,
    render(page, M) {
      const rows = histRows(M);
      page.append(K.stats([[rows.length, "dated exposures", { lead: true }]]));
      page.append(K.panel("When data was exposed", "clock", K.opts({ count: rows.length }), K.table([{ label: "When", get: (r) => r.when }, { label: "What", get: (r) => r.what }, { label: "Source", get: (r) => r.source }], rows, { empty: "No dated exposure." })));
    } };
  function histRows(M) {
    const rows = [];
    for (const f of M.hits) {
      const d = f.details || {};
      if (isBreach(f) && d["years exposed"]) for (const y of split(d["years exposed"])) rows.push({ when: y, what: "appears in a breach dataset" + (d.names ? " (" + UI.clip(d.names, 70) + ")" : ""), source: f.source });
      else if (isStealer(f)) { const m = JSON.stringify(d).match(/\d{4}-\d{2}-\d{2}/g); if (m) rows.push({ when: m[m.length - 1], what: "infostealer log entry (latest)", source: f.source }); }
    }
    return rows.sort((a, b) => b.when.localeCompare(a.when));
  }

  const ADVICE = [
    [/stealer|infostealer/i, "Assume every password on that device was exposed: change them, enable multi-factor authentication, and scan the device."],
    [/breach/i, "Change any password reused with that address, enable multi-factor authentication and watch for phishing that uses the leaked details."],
    [/\.git|exposed|sensitive|env|backup/i, "Remove or block the exposed path, then rotate any secret that may have been in it."],
    [/takeover|CNAME/i, "Remove the dangling DNS record or reclaim the service it points to."],
    [/spoof|SPF|DMARC/i, "Publish SPF ending in -all and a DMARC policy of quarantine or reject."],
    [/TLS|certificate/i, "Renew or replace the certificate and make sure it covers every host name in use."],
    [/header|HTTPS/i, "Force HTTPS with HSTS and add the missing security headers."],
    [/sanction|watch/i, "Compare date of birth, nationality and address by hand before drawing any conclusion: a name match is not an identification."],
    [/look-alike/i, "Check registration date and content of look-alike domains; consider defensive registration or a takedown request if one is hostile."],
  ];
  const risk = { id: "risk", label: "Risk assessment", icon: "shield", always: true,
    render(page, M) {
      const sig = [];
      for (const f of M.hits) { const s = UI.signalFor(f); if (s) sig.push({ ...s, f }); }
      const rank = { high: 0, med: 1, info: 2 };
      sig.sort((a, b) => rank[a.sev] - rank[b.sev]);
      const hi = sig.filter((s) => s.sev === "high").length, med = sig.filter((s) => s.sev === "med").length;
      const level = hi >= 1 ? "High" : med >= 3 ? "Elevated" : med >= 1 ? "Moderate" : "Low";
      const score = Math.min(100, hi * 25 + med * 8);
      page.append(K.panel("Overall exposure", "shield", K.gauge(score, "exposure level: " + level, "Counts high-severity findings (25 points each) and medium ones (8 each). It measures what is exposed, not who the subject is.")));
      page.append(K.stats([[hi, "high", { warn: hi > 0 }], [med, "medium"], [sig.filter((s) => s.sev === "info").length, "informational"]]));
      if (!sig.length) { page.append(K.empty("shield", "No risk signals", "Nothing exposed was found in what was checked.", "give a domain, an IP or an e-mail so exposure checks can run.")); return; }
      page.append(K.panel("Signals and what to do", "alert", K.opts({ count: sig.length }), K.table([{ label: "Severity", get: (s) => s.sev === "high" ? "High" : s.sev === "med" ? "Medium" : "Info" }, { label: "Signal", get: (s) => s.title }, { label: "Detail", get: (s) => s.detail }, { label: "Recommended action", get: (s) => { const a = ADVICE.find(([re]) => re.test(s.title + " " + s.detail)); return a ? a[1] : "Review the finding."; } }], sig, { onRow: (s) => UI.showFinding(s.f) })));
    } };

  UI.defineTab({ id: "exposure", label: "Exposure", icon: "alert", warn: true, count: (M) => mailRows(M).filter((r) => r.exposed).length + M.hits.filter((f) => ["exposure", "takeover"].includes(f.category)).length, sections: [breaches, exposedMails, dataTypes, leaks, history, risk] });
})();
