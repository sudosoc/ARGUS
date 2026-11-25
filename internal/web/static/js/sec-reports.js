"use strict";

// Copyright (C) 2026 Seif, SUDOSOC. SPDX-License-Identifier: AGPL-3.0-only

// Reports: the investigation written up as a document, in four forms, plus the
// server-side exports and a local history of what was exported.
(function () {
  const { el } = UI;
  const K = UI.kit;

  // ------------------------------------------------------------ export history
  const histKey = () => "argus.exports." + (UI.S ? UI.S.id : "");
  const readHist = () => { try { return JSON.parse(localStorage.getItem(histKey()) || "[]"); } catch (e) { return []; } };
  UI.exportLog = {
    add(fmt, label) {
      try { const h = readHist(); h.unshift({ at: Date.now(), fmt, label: label || fmt }); localStorage.setItem(histKey(), JSON.stringify(h.slice(0, 100))); } catch (e) { /* storage unavailable */ }
      if (UI.tab === "reports") UI.scheduleRender(true);
    },
    list: readHist,
  };

  // ------------------------------------------------------------ blocks
  const targetLine = (t) => Object.entries(t || {}).filter(([, v]) => v).map(([k, v]) => `${k}: ${v}`).join(" · ") || "(none)";
  const top = (a, n) => a.slice(0, n);
  const acc = (M) => M.accounts.map((f) => [f.source, M.accountUser ? M.accountUser(f) || "" : "", f.url || "", Math.round((f.confidence || 0) * 100) + "%"]);

  function header(M, title) {
    const r = M.rep;
    return [
      { h: 1, t: title },
      { kv: { Target: targetLine(M.target), Started: r.started_at || K.dt(M.S.created), Finished: r.finished_at || "still running", Findings: String(M.hits.length), "Report digest (SHA-256)": r.integrity ? r.integrity.digest : "recorded when the run finishes" } },
    ];
  }
  function personBlocks(M) {
    const p = M.primary;
    if (!p) return [{ p: "No person could be built from the evidence." }];
    const b = [{ h: 2, t: "Subject" }, { kv: { Name: (p.names || []).map((x) => x.value).join(" / ") || p.label, "Match score": Math.round((p.score || 0) * 100) + "%", Headline: (p.headlines || []).map((x) => x.value).join("; "), Employers: (p.employers || []).map((x) => x.value).join("; "), Locations: (p.locations || []).map((x) => x.value).join("; "), Emails: (p.emails || []).map((x) => x.value).join("; "), Phones: (p.phones || []).map((x) => x.value).join("; "), Usernames: (p.usernames || []).map((x) => x.value).join("; ") } }];
    return b;
  }
  const conclusionText = (M) => (M.notes && (M.notes.conclusions || M.notes["conclusions"])) || "";

  function summaryBlocks(M) {
    const b = header(M, "Investigation summary");
    b.push(...personBlocks(M));
    b.push({ h: 2, t: "At a glance" }, { ul: [
      `${M.accounts.length} accounts found across ${new Set(M.accounts.map((f) => f.source)).size} platforms`,
      `${M.ents.length} entities, ${M.shared.length} of them reported by two or more sources`,
      `${M.photos.length} pictures, ${M.timeline.length} dated events, ${(M.geo && M.geo.points ? M.geo.points.length : 0)} location points`,
      `${M.conflicts().length} conflicts, ${M.duplicates().length} duplicate groups`,
    ] });
    const c = conclusionText(M);
    if (c) b.push({ h: 2, t: "Analyst conclusions" }, { p: c });
    return b;
  }
  function technicalBlocks(M) {
    const b = header(M, "Technical report");
    b.push({ h: 2, t: "Collectors" }, { table: { head: ["Collector", "Checks", "Hits", "Errors", "Avg confidence"], rows: M.modStats().map((s) => [s.name, s.n, s.hit, s.err, Math.round((s.avg || 0) * 100) + "%"]) } });
    if ((M.rep.skipped || []).length) b.push({ h: 2, t: "Collectors that did not run" }, { ul: M.rep.skipped });
    if (M.failures.length) b.push({ h: 2, t: "Failures" }, { p: `${M.failures.length} checks failed. First 50:` }, { table: { head: ["Source", "Error"], rows: top(M.failures, 50).map((f) => [f.source || f.module || "", String(f.error || f.message || "").slice(0, 200)]) } });
    if (M.toolRuns.length) b.push({ h: 2, t: "External tools" }, { table: { head: ["Tool", "Command", "Bytes"], rows: M.toolRuns.map((r) => [r.tool, r.command || "", (r.output || "").length]) } });
    return b;
  }
  function intelBlocks(M) {
    const b = header(M, "Intelligence report");
    b.push(...personBlocks(M));
    b.push({ h: 2, t: "Accounts" }, M.accounts.length ? { table: { head: ["Platform", "Handle", "URL", "Confidence"], rows: top(acc(M), 200) } } : { p: "No accounts found." });
    if (M.shared.length) b.push({ h: 2, t: "Values reported by several sources" }, { table: { head: ["Type", "Value", "Sources"], rows: top(M.shared, 100).map((s) => [s.type || (s.e && s.e.type) || "", s.value || (s.e && s.e.value) || "", s.n || (s.sources ? s.sources.length : "")]) } });
    const c = M.conflicts();
    if (c.length) b.push({ h: 2, t: "Conflicting information" }, { ul: c.map((x) => x.label || x.title || JSON.stringify(x)).slice(0, 50) });
    if (M.timeline.length) b.push({ h: 2, t: "Timeline" }, { table: { head: ["Date", "Event", "Source"], rows: top(M.timeline, 100).map((e) => [K.date(e.date || (e.t ? new Date(e.t).toISOString() : "")), e.label || e.title || e.text || "", e.source || e.module || ""]) } });
    const c2 = conclusionText(M);
    if (c2) b.push({ h: 2, t: "Analyst conclusions" }, { p: c2 });
    return b;
  }
  function evidenceBlocks(M) {
    const b = header(M, "Evidence report");
    const h = M.hits.filter((f) => f.category !== "toolrun");
    b.push({ h: 2, t: "Evidence" }, { p: `${h.length} findings. Each row names the collector, the source page and the confidence assigned.` },
      { table: { head: ["Collector", "Source", "Category", "Confidence", "URL"], rows: top(h, 500).map((f) => [f.module, f.source, UI.cat(f.category).label, Math.round((f.confidence || 0) * 100) + "%", f.url || ""]) } });
    if (h.length > 500) b.push({ p: `Showing the first 500 of ${h.length}; the full set is in the JSON and SQL exports.` });
    if (M.rep.integrity) b.push({ h: 2, t: "Integrity" }, { kv: { Algorithm: M.rep.integrity.algorithm, Digest: M.rep.integrity.digest, Covers: `${M.rep.integrity.findings} findings, ${M.rep.integrity.entities} entities` } });
    return b;
  }

  const REPORTS = {
    summary: ["Investigation summary", "Who or what was found, in one page.", summaryBlocks],
    technical: ["Technical report", "How the run went: collectors, failures, external tools.", technicalBlocks],
    intelligence: ["Intelligence report", "Accounts, shared identifiers, conflicts and timeline.", intelBlocks],
    evidence: ["Evidence report", "Every finding with its source, for review or hand-over.", evidenceBlocks],
  };

  const report = (id) => ({ id, label: REPORTS[id][0], icon: "file", group: "Write-ups", always: true,
    render(page, M) {
      const [title, desc, build] = REPORTS[id];
      const blocks = build(M);
      const md = K.blocksToMarkdown(blocks);
      const name = `argus-${id}-${UI.S.id}`;
      const copy = UI.copyBtn(md, "Copy Markdown");
      const dl = UI.btn("Download .md", { cls: "sm", icon: "download" });
      dl.addEventListener("click", () => { UI.download(name + ".md", md, "text/markdown"); UI.exportLog.add("md", title); });
      const pr = UI.btn("Print / save as PDF", { cls: "sm", icon: "file" });
      pr.addEventListener("click", () => { UI.exportLog.add("print", title); window.print(); });
      page.append(K.panel(title, "file", K.hint(desc + (M.done ? "" : " The investigation is still running, so this report is a snapshot of what is known so far.")), el("div", { class: "rep-actions" }, copy, dl, pr)));
      page.append(el("section", { class: "panel report-panel" }, K.blocksToNode(blocks)));
    } });

  const FORMATS = [["html", "HTML report", "One self-contained page you can open anywhere."], ["json", "JSON", "Everything, for scripts and other tools."], ["csv", "CSV", "Findings as a spreadsheet."], ["md", "Markdown", "The report as text."], ["sql", "SQL", "Load the data into a database."], ["jsonl", "JSONL", "One finding per line."], ["graphml", "GraphML", "The relationship graph for Gephi / yEd."]];
  const exportsSec = { id: "exports", label: "Exports", icon: "download", group: "Files", always: true,
    render(page, M) {
      page.append(K.panel("Download the whole investigation", "download", K.hint("These are produced by the server from the same data shown here. Raw tool output is included."),
        el("div", { class: "leadlist" }, ...FORMATS.map(([f, label, why]) => {
          const a = el("a", { href: `/api/scans/${UI.S.id}/export?format=${f}`, download: "", text: label });
          a.addEventListener("click", () => UI.exportLog.add(f, label));
          return el("div", { class: "lead" }, a, el("span", { class: "why", text: why }));
        }))));
    } };

  const history = { id: "history", label: "Export history", icon: "clock", group: "Files", always: true, count: () => readHist().length,
    render(page) {
      const h = readHist();
      page.append(K.panel("What was exported from this browser", "clock", K.opts({ count: h.length }), K.hint("Kept only in this browser, for this case."),
        K.table([{ label: "When", get: (x) => K.dt(x.at) }, { label: "Format", get: (x) => x.fmt }, { label: "What", get: (x) => x.label }], h, { empty: "Nothing exported yet." })));
      if (h.length) { const c = UI.btn("Clear history", { cls: "sm", icon: "trash" }); c.addEventListener("click", () => { try { localStorage.removeItem(histKey()); } catch (e) { /* ignore */ } UI.scheduleRender(true); }); page.append(c); }
    } };

  UI.defineTab({ id: "reports", label: "Reports", icon: "file", sections: [report("summary"), report("technical"), report("intelligence"), report("evidence"), exportsSec, history] });
})();
