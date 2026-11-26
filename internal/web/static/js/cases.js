"use strict";

// Copyright (C) 2026 Seif, SUDOSOC. SPDX-License-Identifier: AGPL-3.0-only

// Saved cases (re-open, delete, compare two runs), side-by-side comparison of
// two candidate people, and the source-status (health) check.
(function () {
  const { $, el } = UI;
  const picked = new Set();

  // ----------------------------------------------------------- saved cases
  UI.showCases = async function showCases() {
    const body = UI.openDrawer("Saved cases", "stored on this machine");
    body.append(el("p", { class: "hint", text: "Every finished investigation is saved in this machine's user data folder (mode 0600). Tick two cases and press Compare to see what was added or removed between them." }));
    let list;
    try { list = await UI.api("/api/cases"); } catch (e) { body.append(UI.emptyState("alert", "Could not list cases", e.message)); return; }
    if (!list.length) { body.append(UI.emptyState("folder", "No saved cases yet", "They appear here after an investigation finishes.")); return; }
    const cmp = UI.btn("Compare the two ticked cases", { cls: "sm", icon: "layers" });
    cmp.disabled = true;
    const sync = () => { cmp.disabled = picked.size !== 2; };
    cmp.addEventListener("click", () => compareCases([...picked]));
    body.append(el("div", { class: "detail-actions" }, cmp));
    const ul = el("ul", { class: "case-list" });
    for (const c of list) {
      const cb = el("input", { type: "checkbox", id: "cs-" + c.id, "aria-label": "Select for comparison" });
      cb.checked = picked.has(c.id);
      cb.addEventListener("change", () => { cb.checked ? picked.add(c.id) : picked.delete(c.id); if (picked.size > 2) { picked.delete([...picked][0]); showCases(); return; } sync(); });
      const open = UI.btn("Open", { cls: "sm" });
      open.addEventListener("click", async () => {
        try { const r = await UI.api("/api/cases/" + c.id + "/open", { method: "POST" }); UI.closeDrawer(); UI.openScan(r.id); } catch (e) { UI.toast(e.message); }
      });
      const del = UI.btn("Delete", { cls: "sm danger", icon: "trash" });
      del.addEventListener("click", async () => {
        if (!window.confirm("Delete this saved case from disk?")) return;
        try { await UI.api("/api/cases/" + c.id, { method: "DELETE" }); picked.delete(c.id); showCases(); } catch (e) { UI.toast(e.message); }
      });
      ul.append(el("li", { class: "case-row" }, cb, el("div", { class: "cr-main" },
        el("b", { text: c.title || "(no identifiers)" }),
        el("div", { class: "muted", text: `${UI.when(c.at)} · ${UI.n(c.findings)} findings · ${c.identities} people${c.best ? " · best: " + c.best : ""}` }),
        el("div", { class: "chips" }, open, del))));
    }
    body.append(ul);
    sync();
  };

  const entKey = (e) => e.type + ": " + e.value;
  const finKey = (f) => (f.module || "") + " / " + f.source + (f.url ? " · " + f.url : "");

  async function compareCases(ids) {
    const body = UI.openDrawer("Compare cases", "what changed between two runs");
    body.append(el("p", { class: "hint", text: "Loading…" }));
    let a, b;
    try { [a, b] = await Promise.all(ids.map((id) => UI.api("/api/cases/" + id))); } catch (e) { body.replaceChildren(UI.emptyState("alert", "Could not load", e.message)); return; }
    if (new Date(a.started_at) > new Date(b.started_at)) [a, b] = [b, a]; // a = older
    body.replaceChildren();
    body.append(UI.kvTable({ Older: UI.when(a.started_at), Newer: UI.when(b.started_at) }));
    const diff = (xs, ys, key) => {
      const A = new Set(xs.map(key)), B = new Set(ys.map(key));
      return { added: [...B].filter((k) => !A.has(k)), removed: [...A].filter((k) => !B.has(k)) };
    };
    const hits = (r) => (r.findings || []).filter((f) => f.status === "found" || f.status === "uncertain");
    const ga = a.geo && a.geo.consensus, gb = b.geo && b.geo.consensus;
    if (ga !== gb) body.append(el("div", { class: "callout warn" }, UI.icon("alert"), el("span", { text: `Location changed: ${ga || "none"} → ${gb || "none"}` })));
    const la = (a.identities && a.identities[0] || {}).label, lb = (b.identities && b.identities[0] || {}).label;
    if (la !== lb) body.append(el("div", { class: "callout warn" }, UI.icon("alert"), el("span", { text: `Best match changed: ${la || "none"} → ${lb || "none"}` })));
    for (const [title, d] of [["Entities (e-mails, domains, accounts…)", diff(a.entities || [], b.entities || [], entKey)], ["Findings", diff(hits(a), hits(b), finKey)]]) {
      body.append(el("h4", { class: "dsec-title" }, title, el("span", { class: "count", text: `+${d.added.length} / −${d.removed.length}` })));
      const ul = el("ul", { class: "difflist" });
      d.added.forEach((x) => ul.append(el("li", { class: "add" }, el("span", { class: "dmark", text: "+" }), x)));
      d.removed.forEach((x) => ul.append(el("li", { class: "del" }, el("span", { class: "dmark", text: "−" }), x)));
      if (!d.added.length && !d.removed.length) ul.append(el("li", { class: "muted", text: "no change" }));
      body.append(ul);
    }
  }

  // ------------------------------------------------- compare two candidates
  UI.compareIdentities = async function compareIdentities(a, b) {
    const body = UI.openDrawer("Compare people", "how likely they are one person, and what is different");
    body.append(el("div", { class: "skel skel-line" }), el("div", { class: "skel skel-line" }));
    let cmp = null;
    try { cmp = await UI.api("/api/scans/" + UI.S.id + "/compare?a=" + a.id + "&b=" + b.id); } catch (e) { cmp = null; }
    const out = [];
    const VERDICT = { same: "Almost certainly the same person", likely: "Likely the same person", possible: "Possibly the same person", unlikely: "Probably different people" };
    if (cmp) {
      const pct = Math.round(cmp.score * 100);
      out.push(el("div", { class: "cmp-verdict v-" + cmp.verdict },
        el("div", { class: "cv-t" }, el("b", { text: VERDICT[cmp.verdict] || cmp.verdict }), el("span", { class: "num", text: pct + "%" })), UI.meter(pct),
        el("p", { class: "hint", text: "From the signals below: each one is how convincing it would be alone, they are combined, and what speaks against lowers the result. A lead to check, not a verdict." })));
      if ((cmp.photos || []).length) {
        const pp = cmp.photos[0];
        const pic = (u) => el("figure", { class: "cmp-pic" }, UI.queueImg(el("img", { alt: "picture", class: "cmp-img" }, null), u, true));
        out.push(el("div", { class: "cmp-pics" }, pic(pp.a), pic(pp.b),
          el("p", { class: "hint", text: pp.kind === "identical" ? "The same picture." : pp.kind === "near" ? "Almost the same picture (a crop, resize or recolour)." : "Pictures that look alike." + " " + pp.distance + " of 64 fingerprint bits differ." })));
      }
      const rows = (cmp.signals || []).map((s) => el("tr", {}, el("td", { class: "nowrap" }, el("span", { class: "tag " + (s.effect === "against" ? "bad" : ""), text: s.effect === "for" ? "for" : s.effect === "against" ? "against" : "note" })), el("td", { text: s.name }), el("td", { class: "num", text: s.weight ? Math.round(s.weight * 100) + "%" : "" }), el("td", { text: s.detail })));
      if (rows.length) out.push(el("div", { class: "tablewrap" }, el("table", { class: "ctable" }, el("thead", {}, el("tr", {}, ...["", "Signal", "Weight", "What was seen"].map((h) => el("th", { scope: "col", text: h })))), el("tbody", {}, ...rows))));
      else out.push(el("p", { class: "hint", text: "Nothing in common was found." }));
      if ((cmp.advice || []).length) out.push(el("div", { class: "callout" }, UI.icon("check"), el("span", {}, el("b", { text: "To settle it: " }), cmp.advice.join(" "))));
    }
    const rows = [["headlines", "Role"], ["employers", "Employers"], ["locations", "Location"], ["education", "Education"], ["emails", "E-mail"],
      ["phones", "Phone"], ["usernames", "Accounts"], ["websites", "Websites"], ["interests", "Interests"]];
    const tbl = el("table", { class: "ctable cmp" }, el("thead", {}, el("tr", {}, el("th", { scope: "col", text: "" }), el("th", { scope: "col", text: a.label }), el("th", { scope: "col", text: b.label }))));
    const tb = el("tbody");
    const norm = (s) => s.toLowerCase().replace(/[^\p{L}\p{N}]+/gu, " ").trim();
    const cell = (facts, others) => {
      const td = el("td");
      const set = new Set((others || []).map((f) => norm(f.value)));
      for (const f of (facts || []).slice(0, 6)) {
        const same = set.has(norm(f.value));
        td.append(el("div", { class: same ? "same" : "differs" }, el("span", { class: "dmark", text: same ? "=" : "≠" }), f.value));
      }
      return td;
    };
    tb.append(el("tr", {}, el("th", { scope: "row", text: "Match score" }), el("td", { class: "num", text: Math.round(a.score * 100) + "%" }), el("td", { class: "num", text: Math.round(b.score * 100) + "%" })));
    for (const [k, label] of [["names", "Names"], ...rows]) {
      if (!(a[k] || []).length && !(b[k] || []).length) continue;
      tb.append(el("tr", {}, el("th", { scope: "row", text: label }), cell(a[k], b[k]), cell(b[k], a[k])));
    }
    tb.append(el("tr", {}, el("th", { scope: "row", text: "Sources" }), el("td", { text: (a.records || []).map((r) => r.platform).join(" · ") }), el("td", { text: (b.records || []).map((r) => r.platform).join(" · ") })));
    tbl.append(tb);
    out.push(el("h3", { class: "fsec-h", text: "Side by side" }), el("p", { class: "hint", text: "Lines marked = are stated by both. Values on only one side (≠) are what tells the two people apart." }), el("div", { class: "tablewrap" }, tbl));
    body.replaceChildren(...out);
  };

  // --------------------------------------------------------- source status
  UI.showHealth = async function showHealth() {
    const body = UI.openDrawer("Source status", "reachability of the public services used");
    body.append(el("p", { class: "hint", text: "Checking the public services… this takes a few seconds. A source that is down or blocking returns nothing, which is not the same as 'no data exists'." }));
    body.append(el("div", { class: "skel skel-line" }), el("div", { class: "skel skel-line" }), el("div", { class: "skel skel-line" }));
    let rows;
    try { rows = await UI.api("/api/health"); } catch (e) { body.replaceChildren(UI.emptyState("alert", "Status check failed", e.message)); return; }
    const down = rows.filter((r) => !r.ok).length;
    body.replaceChildren(el("div", { class: "dhead" },
      el("b", { class: "num", text: `${rows.length - down}/${rows.length} reachable` }),
      down ? el("span", { class: "tag bad" }, UI.icon("alert"), `${down} not responding`) : el("span", { class: "tag" }, UI.icon("check"), "all reachable")));
    const tbl = el("table", { class: "ctable" }, el("thead", {}, el("tr", {}, ...["Status", "Service", "Used by", "Time"].map((h) => el("th", { scope: "col", text: h })))));
    const tb = el("tbody");
    for (const r of [...rows].sort((a, b) => a.ok - b.ok || a.name.localeCompare(b.name))) {
      tb.append(el("tr", { class: r.ok ? "" : "down" },
        el("td", { class: "nowrap" }, r.ok ? UI.icon("check") : UI.icon("alert"), " " + (r.ok ? "OK " + r.status : "Down")),
        el("td", {}, el("div", { text: r.name }), r.ok ? null : el("div", { class: "hint", text: UI.clip(r.error || "", 100) })),
        el("td", { class: "muted", text: r.used_by }), el("td", { class: "num", text: r.ms + " ms" })));
    }
    tbl.append(tb);
    body.append(el("div", { class: "tablewrap" }, tbl));
  };
})();
