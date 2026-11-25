"use strict";

// Copyright (C) 2026 Seif, SUDOSOC. SPDX-License-Identifier: AGPL-3.0-only

// Findings & analysis: the full table, plus the same evidence cut by how much it can be trusted.
(function () {
  const { el } = UI;
  const K = UI.kit;
  const weight = (f) => { const r = (UI.RELIABILITY[f.module] || ["Medium"])[0]; return r === "High" ? 1 : r === "Medium" ? 0.75 : 0.5; };

  const all = { id: "all", label: "All findings", icon: "list", legacy: "findings", always: true, count: (M) => M.S.findings.length };

  const keyList = (M) => {
    const seen = new Set();
    return [...M.hits].filter((f) => !["toolrun", "tooling"].includes(f.category) && f.module !== "dorks").sort((a, b) => b.confidence * weight(b) - a.confidence * weight(a))
      .filter((f) => { const k = f.module + "|" + f.source; if (seen.has(k)) return false; seen.add(k); return true; }).slice(0, 30);
  };
  const key = { id: "key", label: "Key findings", icon: "alert", count: (M) => keyList(M).length,
    render(page, M) { page.append(K.hint("The most reliable and most informative results: confidence weighted by how trustworthy the source type is. One entry per source."), K.findings("Key findings", "alert", keyList(M), { empty: "Nothing yet." })); } };

  const verifiedList = (M) => M.hits.filter((f) => f.status === "found" && f.confidence >= 0.8 && f.category !== "toolrun");
  const verified = { id: "verified", label: "Verified findings", icon: "check", count: (M) => verifiedList(M).length,
    render(page, M) { page.append(K.hint("Found with 80% confidence or more: read directly from an authoritative source, or confirmed by a content marker. Still verify before you act."), K.findings("Verified findings", "check", verifiedList(M), { empty: "No finding reached 80% confidence." })); } };

  const potentialList = (M) => M.hits.filter((f) => f.status === "uncertain" || (f.status === "found" && f.confidence < 0.6));
  const potential = { id: "potential", label: "Potential findings", icon: "uncertain", count: (M) => potentialList(M).length,
    render(page, M) { page.append(K.hint("Leads that might belong to someone else: a handle match without a content marker, a name match, a search-engine hit. Verify each before relying on it."), K.findings("Potential findings", "uncertain", potentialList(M), { empty: "No tentative findings." })); } };

  const corrList = (M) => {
    const strong = new Map();
    for (const s of M.shared) for (const f of s.findings) strong.set(f, (strong.get(f) || 0) + s.sources.length - 1);
    return [...strong.entries()].filter(([f]) => f.category !== "toolrun").sort((a, b) => b[1] - a[1]).map(([f, n]) => Object.assign({}, f, { __corr: n, __orig: f }));
  };
  const correlated = { id: "correlated", label: "Correlated findings", icon: "link", count: (M) => corrList(M).length,
    render(page, M) {
      const list = corrList(M);
      page.append(K.hint("Findings that mention a value (e-mail, site, handle, picture) also reported by another source. The more they overlap, the stronger the link."));
      const p = K.findings("Correlated findings", "link", list.map((x) => x.__orig), { empty: "No finding overlaps with another." });
      page.append(p);
    } };

  const conflicting = { id: "conflicting", label: "Conflicting findings", icon: "alert", count: (M) => M.conflicts().length,
    render(page, M) {
      const c = M.conflicts();
      page.append(K.stats([[c.length, "disagreements", { lead: true, warn: c.length > 0 }]]));
      if (!c.length) { page.append(K.empty("check", "No conflicts", "The evidence found does not contradict itself.", "")); return; }
      page.append(K.panel("Where sources disagree", "alert", K.opts({ count: c.length }), K.table([{ label: "Kind", get: (x) => x.kind }, { label: "About", get: (x) => x.title }, { label: "Detail", get: (x) => x.detail }], c)));
    } };

  const notes = { id: "notes", label: "Analyst notes", icon: "file", always: true, count: (M) => Object.keys(M.notes).filter((k) => k !== "conclusions").length,
    render(page, M) {
      const entries = Object.entries(M.notes).filter(([k]) => k !== "conclusions");
      page.append(K.panel("Case note", "file", K.hint("A free note for the whole case. Saved with it and included in the reports."), UI.notes.box("case", "What you know that the tool cannot: context, decisions, follow-ups…")));
      page.append(K.panel("All notes", "list", K.opts({ count: entries.length }), entries.length ? K.table([{ label: "About", get: ([k]) => k === "identity" ? "The best match" : k.startsWith("finding:") ? "Finding: " + k.slice(8).replace(/\|/g, " / ") : k }, { label: "Note", get: ([, t]) => t }], entries) : K.hint("No notes yet. Open any finding or person to add one.")));
    } };

  const conclusions = { id: "conclusions", label: "Conclusions", icon: "check", always: true,
    render(page, M) {
      const box = UI.notes.box("conclusions", "Write your conclusions: who or what the evidence points to, how confident you are, what remains unknown…");
      const draft = UI.btn("Draft from the evidence", { cls: "sm", icon: "file" });
      draft.addEventListener("click", () => {
        const ta = box.querySelector("textarea");
        if (!ta) return;
        const p = M.primary, c = M.conflicts();
        const lines = [
          `Subject: ${UI.targetLabel(M.target) || "unspecified"}.`,
          p ? `Best match: ${p.label} (${UI.pct(p.score)}% match with the supplied details; ${p.weak ? "weak" : "supported by " + (p.records || []).length + " records"}).` : "No single person could be resolved from the evidence.",
          `${UI.n(verifiedList(M).length)} findings reached 80% confidence; ${UI.n(potentialList(M).length)} remain tentative.`,
          M.geo && M.geo.consensus ? `Most likely location: ${M.geo.consensus}.` : "",
          c.length ? `Unresolved disagreements: ${c.length} (see Conflicting findings).` : "No conflicting evidence was found.",
          M.failures.length ? `${UI.n(M.failures.length)} checks could not complete, so the absence of a result is not evidence of absence.` : "",
          "", "Unknown / to verify: ",
        ].filter((x) => x !== "");
        ta.value = (ta.value ? ta.value + "\n\n" : "") + lines.join("\n");
        ta.dispatchEvent(new Event("input", { bubbles: true }));
      });
      page.append(K.panel("Conclusions", "check", K.hint("Your reading of the evidence. It is saved with the case and printed in the intelligence report. The draft is only a starting point built from the numbers."), box, UI.notes.ready() ? draft : null));
    } };

  UI.defineTab({ id: "findings", label: "Findings", icon: "list", count: (M) => M.S.findings.length, sections: [all, key, verified, potential, correlated, conflicting, notes, conclusions] });
  UI.findingLists = { verifiedList, potentialList, keyList };
  UI.sectionDefs = Object.assign(UI.sectionDefs || {}, { conflicting });
})();
