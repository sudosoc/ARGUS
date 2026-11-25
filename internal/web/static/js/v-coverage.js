"use strict";

// Coverage & integrity tab: what ran, what could not, what to open by hand,
// how the run was configured and the digest that proves the data was not edited.
(function () {
  const { $, el } = UI;

  const stat = (n, l, warn) => el("div", { class: "kpi static" + (warn ? " warn" : "") }, el("span", { class: "n num", text: UI.n(n) }), el("span", { class: "l", text: l }));

  function coverage() {
    const S = UI.S, root = $("tab-coverage");
    root.replaceChildren();
    root.append(el("div", { class: "callout warn" }, UI.icon("alert"), el("span", {}, el("b", { text: "Read this before relying on the results. " }),
      "Public sources rate-limit, block and go offline. A check that could not run is listed below as a gap: the absence of a result is not evidence that nothing exists. Re-run later, use a proxy, or check manually.")));

    const rep = S.report;
    if (!rep) {
      root.append(el("section", { class: "panel" }, el("h3", { class: "ph" }, "Collectors"),
        UI.emptyState("shield", "Coverage is reported when the investigation finishes", S.errs ? `${UI.n(S.errs)} check${S.errs === 1 ? "" : "s"} could not complete so far.` : "Per-collector results, gaps and the integrity digest appear here.")));
      return;
    }

    const mods = rep.modules || [];
    const tot = mods.reduce((a, m) => ({ checked: a.checked + (m.checked || 0), found: a.found + (m.found || 0), errors: a.errors + (m.errors || 0), ms: a.ms + (m.duration_ms || 0) }), { checked: 0, found: 0, errors: 0, ms: 0 });
    const fails = rep.failures || [];
    root.append(el("div", { class: "kpis" },
      stat(mods.length, "collector runs"), stat(tot.checked, "checks made"), stat(tot.found, "hits"), stat(tot.errors, "errors", tot.errors > 0), stat(fails.length, "coverage gaps", fails.length > 0), stat((rep.skipped || []).length, "not run")));

    // ---- collectors
    const table = el("table", { class: "ctable" }, el("thead", {}, el("tr", {}, ...[["Collector", ""], ["Followed lead", ""], ["Checked", "r"], ["Found", "r"], ["Errors", "r"], ["Time", "r"]].map(([h, c]) => el("th", { class: c, scope: "col", text: h })))));
    const tb = el("tbody");
    for (const m of [...mods].sort((a, b) => UI.modRank(a.name) - UI.modRank(b.name) || (a.via || "").localeCompare(b.via || ""))) {
      const bad = m.errors || m.error;
      tb.append(el("tr", {},
        el("td", { class: "mono", text: m.name }), el("td", { class: "mono muted", text: m.via || "—" }),
        el("td", { class: "r num", text: UI.n(m.checked) }), el("td", { class: "r num", text: UI.n(m.found) }),
        el("td", { class: "r num" + (bad ? " bad" : "") }, bad ? UI.icon("alert") : null, String(m.errors) + (m.error ? " · " + UI.clip(m.error, 80) : "")),
        el("td", { class: "r num", text: (m.duration_ms / 1000).toFixed(1) + "s" })));
    }
    table.append(tb);
    const mp = el("section", { class: "panel" }, el("h3", { class: "ph" }, UI.icon("layers"), "Collectors"), el("div", { class: "tablewrap" }, table));
    if ((rep.skipped || []).length) mp.append(el("p", { class: "hint", text: "Not run: " + rep.skipped.join(" · ") }));
    root.append(mp);

    // ---- raw tool output: everything every tool printed, unparsed and unclipped
    const runs = rep.tool_runs || [];
    if (runs.length) {
      const total = runs.reduce((a, r) => a + (r.bytes || (r.output || "").length), 0);
      const rp = el("section", { class: "panel" }, el("h3", { class: "ph" }, UI.icon("file"), "Raw tool output", el("span", { class: "count", text: String(runs.length) })),
        el("p", { class: "hint", text: `The complete, unparsed output of every external tool and the full table of every username-site check (${UI.n(total)} bytes). Parsers turn it into findings; this is the original, so nothing a tool printed can be lost. It is also in the JSON report and in the .tools folder next to exported files.` }));
      for (const r of runs) {
        const det = el("details", { class: "gap rawrun" }, el("summary", {}, UI.icon("chevron", "caret"), el("span", { class: "tag mod", text: r.tool }),
          el("b", { class: "num", text: UI.n(r.bytes || (r.output || "").length) + " bytes" }),
          r.duration_ms ? el("span", { class: "muted gap-why", text: (r.duration_ms / 1000).toFixed(1) + "s" }) : null,
          r.truncated ? el("span", { class: "tag bad", text: "capture limit reached" }) : null,
          r.error ? el("span", { class: "tag bad", text: "exited with an error" }) : null));
        det.addEventListener("toggle", () => {
          if (!det.open || det.dataset.built) return;
          det.dataset.built = "1";
          const body = el("div", { class: "rawbody" });
          if (r.command) body.append(el("div", { class: "rawcmd mono", text: "$ " + r.command }));
          if (r.error) body.append(el("div", { class: "rawcmd mono bad", text: r.error }));
          body.append(el("pre", { class: "rawout mono", text: r.output || "(no output)" }));
          if (r.stderr) body.append(el("div", { class: "rawcmd muted", text: "stderr" }), el("pre", { class: "rawout mono", text: r.stderr }));
          const dl = el("button", { type: "button", class: "btn small", text: "Download .txt" });
          dl.addEventListener("click", () => {
            const blob = new Blob([(r.command ? "$ " + r.command + "\n\n" : "") + (r.output || "") + (r.stderr ? "\n\n--- stderr ---\n" + r.stderr : "")], { type: "text/plain" });
            const a = el("a", { href: URL.createObjectURL(blob), download: r.tool.replace(/[^A-Za-z0-9._-]+/g, "_") + ".txt" });
            document.body.append(a); a.click(); a.remove();
          });
          body.append(el("div", { class: "rawtools" }, UI.copyBtn(r.output || "", "output"), dl));
          det.append(body);
        });
        rp.append(det);
      }
      root.append(rp);
    }

    // ---- gaps
    const gp = el("section", { class: "panel" }, el("h3", { class: "ph" }, UI.icon("alert"), "Coverage gaps", el("span", { class: "count" + (fails.length ? " warn" : ""), text: String(fails.length) })));
    if (!fails.length) gp.append(el("p", { class: "hint", text: "Every check that ran completed." }));
    const byMod = {};
    for (const f of fails) (byMod[f.module] = byMod[f.module] || []).push(f);
    for (const [m, list] of Object.entries(byMod).sort((a, b) => b[1].length - a[1].length)) {
      const reasons = {};
      for (const f of list) { const r = UI.clip((f.error || "unknown").replace(/\d+/g, "#"), 70); reasons[r] = (reasons[r] || 0) + 1; }
      const top = Object.entries(reasons).sort((a, b) => b[1] - a[1]).slice(0, 2).map(([r, n]) => `${n}× ${r}`).join(" · ");
      const det = el("details", { class: "gap" }, el("summary", {}, UI.icon("chevron", "caret"), el("span", { class: "tag mod", text: m }), el("b", { class: "num", text: UI.n(list.length) }), el("span", { class: "muted gap-why", text: top })));
      const kv = el("dl", { class: "kv" });
      // every gap is listed (built when the group is opened, so thousands stay fast)
      det.addEventListener("toggle", () => {
        if (!det.open || kv.childElementCount) return;
        for (const f of list) kv.append(el("dt", { text: f.source }), el("dd", { text: (f.error || "") + (f.via ? "  (via " + f.via + ")" : "") }));
      });
      det.append(kv);
      gp.append(det);
    }
    root.append(gp);

    // ---- manual leads
    const leads = rep.manual_leads || [];
    if (leads.length) {
      const lp = el("section", { class: "panel" }, el("h3", { class: "ph" }, UI.icon("external"), "Manual leads", el("span", { class: "count", text: String(leads.length) })),
        el("p", { class: "hint", text: "Worth opening by hand. These sources need a login, CAPTCHA or payment, so SUDOSOC ARGUS does not (and will not try to) automate them. The links are pre-filled with what you gave it." }));
      const byGroup = {};
      for (const l of leads) (byGroup[l.group] = byGroup[l.group] || []).push(l);
      for (const [g, list] of Object.entries(byGroup)) {
        const grid = el("div", { class: "leadlist" });
        for (const l of list) {
          const href = UI.safeHref(l.url);
          grid.append(el("div", { class: "lead" }, href ? el("a", { href, target: "_blank", rel: "noopener noreferrer", text: l.label }) : el("span", { text: l.label }), el("span", { class: "why", text: l.why })));
        }
        lp.append(el("div", { class: "leadgroup" }, el("h4", { text: g }), grid));
      }
      root.append(lp);
    }

    // ---- settings + integrity
    const row = el("div", { class: "two" });
    row.append(el("section", { class: "panel" }, el("h3", { class: "ph" }, UI.icon("sliders"), "Run settings (reproducibility)"),
      UI.kvTable(Object.assign({ started: rep.started_at, finished: rep.finished_at, "tool version": rep.version }, rep.settings || {}))));
    const integ = el("section", { class: "panel" }, el("h3", { class: "ph" }, UI.icon("shield"), "Integrity"));
    if (rep.integrity) {
      integ.append(UI.kvTable({ algorithm: rep.integrity.algorithm, "findings / entities": rep.integrity.findings + " / " + rep.integrity.entities }),
        el("p", { class: "hint", text: "SHA-256 of the target, entities and findings in the exported JSON. Store it with the report; any later edit changes it." }),
        el("div", { class: "digest" }, el("code", { text: rep.integrity.digest }), UI.copyBtn(rep.integrity.digest, "digest")));
    } else integ.append(el("p", { class: "hint", text: "No integrity digest was recorded for this run." }));
    row.append(integ);
    root.append(row);
  }

  UI.views.coverage = coverage;
})();
