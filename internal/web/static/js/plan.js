"use strict";

// Copyright (C) 2026 Seif, SUDOSOC. SPDX-License-Identifier: AGPL-3.0-only

// "What will be collected": shown before every investigation. It proposes the smallest set of
// collectors that answers the chosen question, says which of the entered details each would read and
// where they would be sent, and lets the operator tick or untick collectors before anything runs.
(function () {
  const { el } = UI;

  const KIND = { name: "name", username: "username", email: "e-mail", phone: "phone", domain: "domain", ip: "IP", company: "company", wallet: "wallet", asn: "ASN",
    location: "location", files: "files", notes: "pasted text", coords: "coordinates", period: "period" };
  const WEIGHT = { light: "few requests", medium: "some requests", heavy: "many requests" };

  // Resolves with { modules, goal } when the operator confirms, or null when they cancel.
  UI.confirmPlan = function confirmPlan(body, allowed) {
    return new Promise(async (resolve) => {
      let done = false;
      const finish = (v) => { if (done) return; done = true; resolve(v); };
      const drawer = UI.openDrawer("What will be collected", "the smallest set that answers the question, before anything is sent");
      drawer.append(el("div", { class: "skel skel-line" }), el("div", { class: "skel skel-line" }));
      let plan;
      try { plan = await UI.api("/api/plan", { method: "POST", body: JSON.stringify(body) }); } catch (e) {
        drawer.replaceChildren(UI.emptyState("alert", "Could not prepare the plan", e.message));
        return finish(null);
      }
      const picked = new Set(plan.collectors.filter((c) => c.selected && (!allowed || allowed.has(c.name))).map((c) => c.name));
      const boxes = new Map();
      const total = el("b", { class: "num" });
      const startBtn = el("button", { type: "button", class: "btn primary" });
      const refresh = () => {
        const n = [...boxes.values()].filter((b) => b.checked).length;
        startBtn.textContent = n ? "Start with " + n + (n === 1 ? " collector" : " collectors") : "Nothing selected";
        startBtn.disabled = !n;
        let req = 0;
        for (const c of plan.collectors) if (boxes.get(c.name) && boxes.get(c.name).checked) req += c.requests || ({ light: 6, medium: 25, heavy: 60 }[c.weight] || 10);
        total.textContent = "about " + UI.n(req) + " requests";
      };
      const row = (c) => {
        const cb = el("input", { type: "checkbox", id: "pl_" + c.name });
        cb.checked = picked.has(c.name);
        cb.addEventListener("change", refresh);
        boxes.set(c.name, cb);
        const uses = c.uses.length ? c.uses.map((u) => KIND[u] || u).join(", ") : "nothing you entered";
        return el("label", { class: "plan-row", for: "pl_" + c.name },
          cb,
          el("span", { class: "plan-main" },
            el("b", { text: c.name }), el("span", { class: "plan-q", text: c.question }),
            el("span", { class: "plan-meta" },
              el("span", { class: "tag" }, "reads: " + uses),
              el("span", { class: "tag" }, c.sends_to ? "sends to: " + c.sends_to : "stays on this machine"),
              el("span", { class: "tag w-" + c.weight }, WEIGHT[c.weight] || c.weight + (c.requests ? " · " + UI.n(c.requests) : "")))));
      };
      const goalSel = el("select", { id: "planGoal", "aria-label": "Question" }, ...plan.goals.map((g) => el("option", { value: g.id, text: g.label, selected: g.id === plan.goal ? "" : null })));
      goalSel.addEventListener("change", () => {
        // choosing another question re-plans from the form
        const f = document.getElementById("goalSel");
        if (f) { f.value = goalSel.value; f.dispatchEvent(new Event("change", { bubbles: true })); }
        finish(null);
        UI.closeDrawer();
        setTimeout(() => document.getElementById("startBtn").click(), 120);
      });
      const goalNow = plan.goals.find((g) => g.id === plan.goal) || plan.goals[0];
      const need = plan.collectors.filter((c) => c.selected && (!allowed || allowed.has(c.name)));
      const optional = plan.collectors.filter((c) => c.optional);
      const spare = plan.collectors.filter((c) => !need.includes(c) && !c.optional);
      const sup = plan.supplied.map((k) => KIND[k] || k).join(", ");

      const startRow = el("div", { class: "plan-foot" },
        el("span", { class: "plan-total" }, "Estimated: ", total),
        el("span", { class: "grow" }),
        el("button", { type: "button", class: "btn", text: "Cancel" }),
        startBtn);
      startRow.querySelector("button.btn:not(.primary)").addEventListener("click", () => { UI.closeDrawer(); finish(null); });
      startBtn.addEventListener("click", () => {
        const names = [...boxes.entries()].filter(([, b]) => b.checked).map(([n]) => n);
        UI.closeDrawer();
        finish({ modules: names, goal: plan.goal, all: names.length === plan.collectors.length });
      });

      drawer.replaceChildren(
        el("div", { class: "plan-q0" }, el("label", { for: "planGoal", class: "muted", text: "Question" }), goalSel, el("p", { class: "hint", text: goalNow.question })),
        el("p", { class: "hint", text: "Using what you entered (" + (sup || "nothing") + "). Collectors that need something you did not enter are not listed." }),
        el("h3", { class: "fsec-h", text: "Will run (" + need.length + ")" }),
        need.length ? el("div", { class: "plan-list" }, ...need.map(row)) : el("p", { class: "hint", text: "Nothing in this question applies to what you entered. Add a detail, or choose another question." }),
        optional.length ? el("div", { class: "plan-opt" }, el("h3", { class: "fsec-h", text: "Optional: copy public posts (off unless you tick)" }),
          el("p", { class: "hint", text: "Copies what the account publicly shows to anyone without an account: no sign-in, no keys. Only the username you typed is used, never a guess. Accounts that asked not to be shown to visitors are skipped, and each place is asked a handful of times, one request a second." }),
          el("div", { class: "plan-list" }, ...optional.map(row))) : null,
        spare.length ? el("details", { class: "plan-spare" }, el("summary", { text: "Also available, not needed for this question (" + spare.length + ")" }), el("div", { class: "plan-list" }, ...spare.map(row))) : null,
        plan.skipped.length ? el("details", { class: "plan-spare" }, el("summary", { text: "Skipped: they need details you did not enter (" + plan.skipped.length + ")" }), el("p", { class: "hint", text: plan.skipped.map((s) => s.name).join(", ") })) : null,
        el("div", { class: "plan-never" }, UI.icon("shield"), el("span", { text: "Never collected or analysed, even if it appears in the evidence: " + plan.never.join("; ") + "." })),
        startRow);
      refresh();
    });
  };
})();
