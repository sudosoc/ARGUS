"use strict";

// Copyright (C) 2026 Seif, SUDOSOC. SPDX-License-Identifier: AGPL-3.0-only

// Identity tab: the dossier of the best-matching person (confirmed facts apart
// from unconfirmed ones), other candidates side by side, accounts by area,
// weak handle-only matches and the dated timeline.
(function () {
  const { $, el } = UI;

  const FACT_ROWS = [
    ["headlines", "Role"], ["employers", "Employers"], ["locations", "Location"], ["education", "Education"],
    ["interests", "Interests"], ["emails", "E-mail"], ["phones", "Phone"], ["usernames", "Accounts"], ["websites", "Websites"],
  ];
  const ALL_KEYS = ["names", "headlines", "employers", "locations", "education", "interests", "emails", "phones", "usernames", "websites"];
  const MONO_ROWS = new Set(["emails", "phones", "usernames"]);
  const cmpPick = new Set();
  let cmpChanged = () => {};
  let showAllUnconfirmed = false;

  const isSure = (f) => f.confirmed || f.self;
  const trustTxt = (f) => (f.trust ? " · reliability " + Math.round(f.trust * 100) + "%" : "");

  /** Trust marker: icon + words + weight, so it never relies on colour. */
  function factBadge(f) {
    const srcs = (f.sources || []).join(", ");
    if (f.confirmed) return el("span", { class: "tbadge t-confirmed", title: "Stated by independent sources: " + srcs + trustTxt(f) }, UI.icon("check"), `${f.sources.length} sources`);
    if (f.self) return el("span", { class: "tbadge t-self", title: "Stated in the person's own profile (" + srcs + ")" + trustTxt(f) }, UI.icon("user"), "self-reported");
    return el("span", { class: "tbadge t-single", title: "One third-party source: " + srcs + trustTxt(f) }, UI.icon("uncertain"), "1 source");
  }

  function factItem(f, mono) {
    const href = /^https?:\/\//.test(f.value) ? UI.safeHref(f.value) : null;
    const v = href ? el("a", { class: "v", href, target: "_blank", rel: "noopener noreferrer", text: f.value }) : el("span", { class: "v" + (mono ? " mono" : ""), text: f.value });
    return el("div", { class: "fact " + (f.confirmed ? "f-confirmed" : f.self ? "f-self" : "f-single") }, v, factBadge(f), UI.copyBtn(f.value, "value"));
  }

  function showConflict(label, facts) {
    const body = UI.openDrawer("Sources disagree", label);
    body.append(el("p", { class: "hint", text: "Each value below is stated by the listed sources. They may describe a move, an old address, or different people." }));
    for (const f of facts) body.append(el("div", { class: "conflict-item" }, el("b", { text: f.value }), el("div", { class: "muted", text: (f.confirmed ? "confirmed · " : "stated · ") + (f.sources || []).join(" · ") })));
    $("drawerClose").focus();
  }

  function scoreFig(id) {
    const p = Math.round(id.score * 100);
    return el("div", { class: "scorefig t-" + UI.tier(p) },
      el("div", { class: "sf-top" }, el("span", { class: "sf-n num", text: String(p) }), el("span", { class: "sf-of", text: "/ 100" })),
      el("div", { class: "sf-l", text: "match score" }), UI.meter(p));
  }

  function photoButton(id, size) {
    const photo = (id.photos || [])[0];
    const ph = el("button", { type: "button", class: "avatar " + size, title: photo ? "Open photo" : "No photo found", "aria-label": photo ? "Open photo" : "No photo found" }, UI.icon("user"));
    if (photo) {
      const img = el("img", { src: UI.imgSrc(photo), alt: "profile picture" });
      img.addEventListener("load", () => ph.replaceChildren(img));
      ph.addEventListener("click", () => UI.showPhoto({ url: photo, sources: id.records.map((r) => r.platform), group: 0 }));
    } else ph.disabled = true;
    return ph;
  }

  function dossier(id) {
    const card = el("article", { class: "panel hero-card", "aria-label": "Identity dossier" });
    const left = el("div", { class: "hero-left" }, photoButton(id, "xl"));
    const extra = el("div", { class: "thumbs" });
    for (const u of (id.photos || []).slice(1, 9)) {
      const t = el("button", { type: "button", class: "thumb", "aria-label": "Open photo" });
      const im = el("img", { src: UI.imgSrc(u), alt: "" });
      im.addEventListener("error", () => t.remove());
      t.append(im);
      t.addEventListener("click", () => UI.showPhoto({ url: u, sources: id.records.map((r) => r.platform), group: 0 }));
      extra.append(t);
    }
    if (extra.children.length) left.append(extra);

    const conflictRows = new Set();
    for (const c of id.conflicts || []) { if (/different locations/.test(c)) conflictRows.add("locations"); if (/names differ/.test(c)) conflictRows.add("names"); }
    const confirmedN = ALL_KEYS.reduce((n, k) => n + (id[k] || []).filter((f) => f.confirmed).length, 0);
    const stat = (n, l) => el("div", { class: "stat" }, el("span", { class: "n num", text: String(n) }), el("span", { class: "l", text: l }));
    const chips = el("div", { class: "chips" });
    for (const m of id.matched || []) chips.append(el("span", { class: "chip strong" }, UI.icon("check"), el("span", { class: "k", text: "matches" }), el("span", { class: "chip-v", text: m })));

    const main = el("div", { class: "hero-main" },
      el("div", { class: "eyebrow", text: id.primary ? "Best match" : "Candidate" }),
      el("h2", { class: "hero-name", text: (id.names[0] && id.names[0].value) || "Unnamed profile" }),
      el("div", { class: "hero-sub", text: [(id.headlines[0] || {}).value, (id.locations[0] || {}).value].filter(Boolean).join(" · ") }),
      el("div", { class: "hero-stats" }, stat(id.records.length, "sources"), stat(confirmedN, "confirmed facts"), stat((id.usernames || []).length, "accounts"), stat((id.emails || []).length, "e-mails")),
      chips);
    card.append(el("div", { class: "hero" }, left, main, scoreFig(id)));

    // ---- facts: confirmed / self-reported vs single-source, clearly apart
    const sureBlock = el("div", { class: "factblock fb-sure" });
    const unsureBlock = el("div", { class: "factblock fb-unsure" });
    let nSure = 0, nUnsure = 0;
    const addRow = (key, label, facts, mono) => {
      const sure = facts.filter(isSure), unsure = facts.filter((f) => !isSure(f));
      const conflict = conflictRows.has(key);
      const mk = (list, cls) => {
        const row = el("div", { class: "frow" + (conflict && cls === "sure" ? " conflict-row" : "") }, el("div", { class: "k", text: label }));
        const vals = el("div", { class: "fvals" });
        const cap = key === "interests" ? 14 : 6;
        list.slice(0, cap).forEach((f) => vals.append(factItem(f, mono)));
        if (list.length > cap) {
          const more = UI.btn(`+${list.length - cap} more`, { cls: "sm ghost" });
          more.addEventListener("click", () => more.replaceWith(...list.slice(cap).map((f) => factItem(f, mono))));
          vals.append(more);
        }
        if (conflict && cls === "sure") {
          const b = UI.btn("See who says what", { cls: "sm ghost" });
          b.addEventListener("click", () => showConflict(label, facts));
          vals.append(el("div", { class: "cnote" }, UI.icon("alert"), "sources disagree here", b));
        }
        row.append(vals);
        return row;
      };
      if (sure.length) { sureBlock.append(mk(sure, "sure")); nSure += sure.length; }
      if (unsure.length) { unsureBlock.append(mk(unsure, "unsure")); nUnsure += unsure.length; }
    };
    if (conflictRows.has("names")) addRow("names", "Names", id.names, false);
    for (const [key, label] of FACT_ROWS) if ((id[key] || []).length) addRow(key, label, id[key], MONO_ROWS.has(key));
    if (id.bio) sureBlock.append(el("div", { class: "frow" }, el("div", { class: "k", text: "About" }), el("div", { class: "bio", text: UI.clip(id.bio, 600) })));
    if ((id.documents || []).length) {
      const box = el("div", { class: "fvals" });
      for (const d of id.documents) { const h = UI.safeHref(d); box.append(h ? el("a", { href: h, target: "_blank", rel: "noopener noreferrer", class: "wrap", text: d }) : el("span", { text: d })); }
      sureBlock.append(el("div", { class: "frow" }, el("div", { class: "k", text: "CV / resume" }), box));
    }

    const legend = el("div", { class: "trust-legend", "aria-label": "How to read the markers" },
      el("span", { class: "tbadge t-confirmed" }, UI.icon("check"), "confirmed by 2+ platforms"),
      el("span", { class: "tbadge t-self" }, UI.icon("user"), "own profile"),
      el("span", { class: "tbadge t-single" }, UI.icon("uncertain"), "single third-party source"));

    const cols = el("div", { class: "factcols" });
    const sureCol = el("section", { class: "factcol" }, el("h3", {}, "Confirmed & self-reported ", el("span", { class: "count", text: String(nSure) })),
      nSure ? sureBlock : el("p", { class: "hint", text: "Nothing is confirmed by two platforms or stated in the person's own profile yet." }));
    cols.append(sureCol);
    if (nUnsure) {
      const open = showAllUnconfirmed || !nSure;
      const col = el("section", { class: "factcol unconfirmed" }, el("h3", {}, "Unconfirmed ", el("span", { class: "count", text: String(nUnsure) })));
      if (open) col.append(unsureBlock);
      const tog = UI.btn(open ? "Hide unconfirmed facts" : `Show ${nUnsure} unconfirmed fact${nUnsure === 1 ? "" : "s"}`, { cls: "sm" });
      tog.addEventListener("click", () => { showAllUnconfirmed = !showAllUnconfirmed; UI.scheduleRender(true); });
      if (nSure) col.append(tog);
      if (!open) col.append(el("p", { class: "hint", text: "Each of these is stated by one third-party source only: treat as leads." }));
      cols.append(col);
    }
    card.append(legend, cols);

    for (const c of id.conflicts || []) card.append(el("div", { class: "callout warn" }, UI.icon("alert"), el("span", { text: c })));
    if ((id.reasons || []).length) {
      const ul = el("ul", { class: "plain" });
      for (const r of id.reasons) ul.append(el("li", { text: r }));
      card.append(el("details", { class: "fold" }, el("summary", { text: "Why these sources are treated as the same person" }), ul));
    }
    const chips2 = el("div", { class: "chips" });
    for (const r of id.records) {
      const href = UI.safeHref(r.url);
      chips2.append(href ? el("a", { class: "chip", href, target: "_blank", rel: "noopener noreferrer", title: r.source }, UI.icon("external"), el("span", { class: "k", text: r.self ? "own" : "record" }), el("span", { class: "chip-v", text: r.platform }))
        : el("span", { class: "chip" }, el("span", { class: "chip-v", text: r.platform })));
    }
    card.append(el("details", { class: "fold" }, el("summary", { text: `Sources (${id.records.length})` }), chips2));

    const act = el("div", { class: "actions" });
    const b = UI.btn("Use these details to search again", { cls: "sm", icon: "search", title: "Fills the form with this person's name, location, employer, e-mail, account and website" });
    b.addEventListener("click", () => UI.refine(id));
    const cb = el("input", { type: "checkbox", id: "pick-" + id.id });
    cb.checked = cmpPick.has(id.id);
    cb.addEventListener("change", () => { cb.checked ? cmpPick.add(id.id) : cmpPick.delete(id.id); cmpChanged(); });
    act.append(b, el("label", { class: "check sm", for: "pick-" + id.id }, cb, el("span", { text: "pick for comparison" })));
    card.append(act);
    card.append(el("details", { class: "fold", open: !!UI.notes.get("identity") }, el("summary", { text: "Your notes on this person" }), UI.notes.box("identity", "What you verified, doubts, next steps…")));
    return card;
  }

  /** Compact card for another candidate, laid out so several read side by side. */
  function candidate(id, best, topId) {
    const c = el("article", { class: "ccard" + (best ? " best" : "") });
    const p = Math.round(id.score * 100);
    c.append(el("div", { class: "ctop" }, photoButton(id, "md"),
      el("div", { class: "cmain" }, el("div", { class: "cn", text: id.label }), el("div", { class: "cscore" }, el("span", { class: "num", text: p + "%" }), UI.meter(p)))));
    const have = new Set();
    if (topId) for (const k of ["employers", "locations", "education"]) for (const f of topId[k] || []) have.add(f.value.toLowerCase());
    const ul = el("ul", { class: "plain" });
    for (const [k, lab] of [["employers", "works at"], ["locations", "in"], ["education", "studied at"], ["emails", "e-mail"]]) {
      const f = (id[k] || [])[0];
      if (!f) continue;
      const differs = topId && !have.has(f.value.toLowerCase()) && k !== "emails";
      ul.append(el("li", { class: differs ? "diff" : "", title: differs ? "differs from the best match" : "" }, differs ? el("span", { class: "neq", text: "≠ " }) : null, `${lab} ${f.value}`));
    }
    if (ul.childNodes.length) c.append(ul);
    c.append(el("div", { class: "csrc", text: id.records.map((r) => r.platform).join(" · ") }));
    const row = el("div", { class: "pickrow" });
    const use = UI.btn("This is the person", { cls: "sm", title: "Search again with these details" });
    use.addEventListener("click", () => UI.refine(id));
    const open = UI.btn("Full profile", { cls: "sm ghost" });
    open.addEventListener("click", () => c.replaceWith(dossier(id)));
    const cb = el("input", { type: "checkbox", id: "cmp-" + id.id });
    cb.checked = cmpPick.has(id.id);
    cb.addEventListener("change", () => { cb.checked ? cmpPick.add(id.id) : cmpPick.delete(id.id); cmpChanged(); });
    row.append(use, open, el("label", { class: "check sm", for: "cmp-" + id.id }, cb, el("span", { text: "compare" })));
    c.append(row);
    return c;
  }

  /** What to add to learn more or to tighten the match. */
  function tipsPanel(top, S) {
    const n = (k) => (top[k] || []).length;
    const t = S.target || {};
    const sites = UI.info && UI.info.social_sites ? UI.n(UI.info.social_sites) : "many";
    const tips = [];
    const ambiguous = !!S.ambiguity || S.sameName > 1 || !top.primary;
    if (ambiguous && !t.location) tips.push("Add a location (city or country): it is the quickest way to separate people who share the name.");
    if (ambiguous && !t.company) tips.push("Add an employer or school: profiles that mention it are ranked first.");
    if (!t.email && n("emails")) tips.push("Candidate e-mail found: " + top.emails[0].value + ". If it is right, enter it in the E-mail field to confirm the match and pull its Gravatar, GitHub and leak data.");
    if (!t.username && n("usernames")) tips.push("Account found: " + top.usernames[0].value + ". Enter it as Username to scan " + sites + " sites for the same handle.");
    if (!t.domain && n("websites")) tips.push("Website found: " + top.websites[0].value + ". Enter its domain to map DNS, subdomains, technologies and contacts.");
    if (!n("photos")) tips.push("No verified photo. Check Photos → unverified candidates, or use the reverse image links on any picture.");
    if (!(t.notes || "").trim() && !(t.links || []).length) tips.push("Open the person's LinkedIn yourself and paste the page text into the Paste text box (e-mail, CV and full history only appear to logged-in viewers).");
    if (!tips.length) return null;
    const ul = el("ul", { class: "plain" });
    tips.slice(0, 5).forEach((x) => ul.append(el("li", { text: x })));
    return el("details", { class: "panel fold-panel", open: ambiguous }, el("summary", {}, UI.icon("chevron"), el("span", { text: "To learn more or tighten the match" })), ul);
  }

  /**
   * Accounts found under the handle, grouped by what they are for (coding, music,
   * gaming…). They exist under the handle: not proof that all belong to the subject.
   */
  function areasPanel(S) {
    const groups = new Map();
    const seen = new Set();
    for (const f of S.findings) {
      if (!f.url || (f.status !== "found" && f.status !== "uncertain")) continue;
      let area = "";
      if (f.module === "social") area = f.category || "other";
      else if (f.details && f.details.area) area = f.details.area;
      else continue;
      if (seen.has(f.url)) continue;
      seen.add(f.url);
      if (!groups.has(area)) groups.set(area, []);
      groups.get(area).push(f);
    }
    if (groups.size === 0) return null;
    const total = [...groups.values()].reduce((n, g) => n + g.length, 0);
    const wrap = el("details", { class: "panel fold-panel", open: true },
      el("summary", {}, UI.icon("chevron"), el("span", { text: `Accounts by area (${UI.n(total)} across ${groups.size} areas)` })),
      el("p", { class: "hint", text: "Accounts that exist under the handle(s) you gave, grouped by purpose. They show where the person is active, but a handle can belong to someone else: confirm with the photo, name or bio on each." }));
    const grid = el("div", { class: "area-grid" });
    [...groups.entries()].sort((a, b) => b[1].length - a[1].length).forEach(([area, fs]) => {
      const box = el("div", { class: "area" }, el("h4", {}, UI.icon(UI.cat(area).icon), el("span", { text: UI.cat(area).label }), el("span", { class: "count", text: String(fs.length) })));
      const add = (f) => {
        const href = UI.safeHref(f.url);
        box.append(href ? el("a", { href, target: "_blank", rel: "noopener noreferrer", text: f.source.replace(/ profile$/, ""), title: f.url }) : el("span", { text: f.source }));
      };
      fs.slice(0, 8).forEach(add);
      if (fs.length > 8) {
        const more = UI.btn(`+${fs.length - 8} more`, { cls: "sm ghost" });
        more.addEventListener("click", () => { more.remove(); fs.slice(8).forEach(add); });
        box.append(more);
      }
      grid.append(box);
    });
    wrap.append(grid);
    return wrap;
  }

  function skeleton() {
    return el("div", { class: "panel skel-card", "aria-busy": "true", "aria-label": "Loading" }, el("div", { class: "skel skel-photo" }),
      el("div", { class: "skel-lines" }, ...[40, 70, 55, 90, 75].map((w) => { const l = el("div", { class: "skel skel-line" }); l.style.width = w + "%"; return l; })));
  }

  function identityView() {
    const S = UI.S, root = $("tab-identity");
    root.replaceChildren();
    const all = S.identities || [];
    const ids = all.filter((i) => !i.weak);
    const weak = all.filter((i) => i.weak);
    if (!ids.length && weak.length) ids.push(...weak.splice(0, 1));
    if (!ids.length) {
      if (S.state === "running") {
        root.append(skeleton(), el("p", { class: "hint pad", text: "Collecting… as soon as collectors return profile data (LinkedIn, ORCID, GitHub, Wikidata, personal sites, CVs) the people they describe appear here, kept apart." }));
      } else {
        root.append(UI.emptyState("id", "No person profiles were found", "Give a full name plus a location, employer or e-mail, paste a LinkedIn/CV text, or a username, and run again."));
      }
      return;
    }
    if (S.ambiguity) {
      root.append(el("div", { class: "callout warn" }, UI.icon("alert"), el("span", {}, el("b", { text: "Several people match. " }), S.ambiguity,
        " Their facts are shown separately, never merged, and none of their e-mails or accounts is followed until you choose one.")));
    } else if (S.sameName > 1) {
      root.append(el("div", { class: "callout" }, UI.icon("user"), el("span", {}, el("b", { text: `${S.sameName} people share this name. ` }), "The best match is shown first; the others are listed beside it.")));
    }
    const top = ids[0];
    if (top.primary) root.append(dossier(top));
    else root.append(el("p", { class: "hint pad", text: "No single person is a clear match yet. Candidates, best guess first:" }));

    const rest = top.primary ? ids.slice(1) : ids;
    if (rest.length) {
      const wrap = el("section", { class: "panel" }, el("h3", { class: "ph" }, top.primary ? `Other people matching` : `Candidates`, el("span", { class: "count", text: String(rest.length) })));
      wrap.append(el("p", { class: "hint", text: "Tick two people (or one person plus the best match) and a Compare bar appears at the bottom of the screen." }));
      const list = el("div", { class: "cand-grid" });
      rest.forEach((id, i) => list.append(candidate(id, !top.primary && i === 0, top.primary ? top : null)));
      wrap.append(list);
      root.append(wrap);
    }

    // floating compare bar: appears as soon as something is ticked
    const bar = el("div", { class: "cmp-bar", hidden: true, role: "region", "aria-label": "Compare people" });
    const msg = el("span", { class: "cmp-msg" });
    const go = UI.btn("Compare", { cls: "primary" });
    const clr = UI.btn("Clear", { cls: "ghost sm" });
    bar.append(msg, go, clr);
    const picked = () => ids.filter((i) => cmpPick.has(i.id));
    cmpChanged = () => {
      const p = picked();
      bar.hidden = p.length === 0;
      go.disabled = p.length < 1 || p.length > 2 || (p.length === 1 && !(top.primary && p[0].id !== top.id));
      msg.textContent = p.length === 0 ? "" : p.length === 1 ? (top.primary && p[0].id !== top.id ? `Compare "${p[0].label}" with the best match "${top.label}", or tick one more person` : "Tick one more person to compare") : p.length === 2 ? `Compare "${p[0].label}" with "${p[1].label}"` : "Tick at most two people";
    };
    go.addEventListener("click", () => {
      const p = picked();
      if (p.length === 2) UI.compareIdentities(p[0], p[1]);
      else if (p.length === 1 && top.primary && p[0].id !== top.id) UI.compareIdentities(top, p[0]);
    });
    clr.addEventListener("click", () => { cmpPick.clear(); root.querySelectorAll("input[type=checkbox][id^=pick-], input[type=checkbox][id^=cmp-]").forEach((c) => { c.checked = false; }); cmpChanged(); });
    root.append(bar);
    cmpChanged();

    const tp = tipsPanel(top, S);
    if (tp) root.append(tp);
    const ap = areasPanel(S);
    if (ap) root.append(ap);
    if (weak.length) {
      const chips = el("div", { class: "chips" });
      for (const id of weak) {
        const r = id.records[0] || {};
        const href = UI.safeHref(r.url);
        const label = (id.usernames[0] || {}).value || r.platform;
        chips.append(href ? el("a", { class: "chip weak", href, target: "_blank", rel: "noopener noreferrer" }, el("span", { class: "k", text: r.platform }), el("span", { class: "chip-v", text: label })) : el("span", { class: "chip weak", text: label }));
      }
      root.append(el("details", { class: "panel fold-panel" }, el("summary", {}, UI.icon("chevron"), el("span", { text: `Accounts that only share the handle (${weak.length}): probably other people` })),
        el("p", { class: "hint", text: "These accounts have no name and nothing else tying them to your subject. They are shown for completeness and are never followed." }), chips));
    }
    if (S.report && (S.report.timeline || []).length) root.append(UI.timelinePanel(S.report.timeline));
  }

  /** Shared empty state: icon, headline, one line of guidance. */
  UI.emptyState = function emptyState(icon, title, text) {
    return el("div", { class: "empty-state" }, UI.icon(icon, "big"), el("h3", { text: title }), el("p", { text }));
  };

  UI.views.identity = identityView;
})();
