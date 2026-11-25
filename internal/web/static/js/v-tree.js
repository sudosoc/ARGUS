"use strict";

// Copyright (C) 2026 Seif, SUDOSOC. SPDX-License-Identifier: AGPL-3.0-only

// Intelligence tree: everything collected, organised subject → section →
// type/category → item. Groups are built lazily when opened and long groups
// are paged, so thousands of items stay fast.
(function () {
  const { $, el } = UI;

  // Sections decide where entities and categories land. Categories nobody
  // claims (new collectors add some) fall into the last, catch-all section.
  const SECTIONS = [
    { id: "identity", icon: "user", label: "Identity", ents: ["name", "username"], cats: ["idinfo", "exif", "photo", "site-people", "developer", "publication", "document-metadata", "history", "activity"] },
    { id: "contact", icon: "mail", label: "Contact points", ents: ["email", "phone"], cats: ["phone", "contact", "breach"] },
    { id: "accounts", icon: "link", label: "Online accounts & profiles", ents: [], cats: ["social", "profile", "email-account", "social-links", "graph"] },
    { id: "infra", icon: "globe", label: "Infrastructure", ents: ["domain", "ip", "asn"], cats: ["dns", "whois", "subdomain", "tls", "web", "tech", "tracking", "email-security", "security", "archive", "network", "geo", "ip", "email-domain", "lookalike"] },
    { id: "org", icon: "building", label: "Organisation & places", ents: ["organisation", "location"], cats: ["company", "registry", "reference", "website"] },
    { id: "exposure", icon: "alert", label: "Exposure & risk", ents: [], cats: ["exposure", "takeover", "reputation", "sanctions"] },
    { id: "crypto", icon: "wallet", label: "Crypto", ents: ["wallet"], cats: ["crypto"] },
    { id: "other", icon: "search", label: "Search leads, mentions & other", ents: ["url"], cats: null },
  ];
  const PAGE = 200;

  let query = "";
  const open = new Set(); // ids of groups the analyst opened (kept across re-renders)

  const leaf = (children, onclick) => {
    const b = el("button", { type: "button", class: "leaf" }, ...children);
    b.addEventListener("click", onclick);
    return b;
  };

  function findingLeaf(f) {
    const p = UI.pct(f.confidence);
    const kids = [
      UI.statusBadge(f.status, true),
      el("span", { class: "v", text: UI.clip(f.source, 90) }),
      el("span", { class: "pct t-" + UI.tier(p), text: p + "%" }),
      el("span", { class: "tag", text: f.module }),
    ];
    if (f.via) kids.push(el("span", { class: "tag via", text: "via " + UI.clip(f.via, 40) }));
    const h = UI.host(f.url);
    if (h) kids.push(el("span", { class: "m", text: h }));
    return leaf(kids, () => UI.showFinding(f));
  }

  function entityLeaf(e) {
    const kids = [el("span", { class: "v", text: e.value }),
      el("span", { class: "tag" + (e.strong ? " strong" : ""), text: e.strong ? "strong lead" : "weak lead" })];
    if (e.source) kids.push(el("span", { class: "m", text: "from " + UI.clip(e.source, 60) }));
    return leaf(kids, () => UI.showEntity(e));
  }

  function group(id, icon, label, count, builder, startOpen) {
    const d = el("details", { class: "tgroup", "data-id": id });
    if (startOpen || open.has(id)) d.open = true;
    d.append(el("summary", {}, UI.icon("chevron", "caret"), UI.icon(icon, "gico"), el("span", { class: "nm", text: label }), el("span", { class: "count", text: UI.n(count) })));
    let built = false;
    const build = () => { if (!built) { built = true; const body = el("div", { class: "tbody" }); builder(body); d.append(body); } };
    if (d.open) build();
    d.addEventListener("toggle", () => { if (d.open) { open.add(id); build(); } else open.delete(id); });
    return d;
  }

  function addLeaves(parent, items, make) {
    const next = (from) => {
      for (const it of items.slice(from, from + PAGE)) parent.append(make(it));
      if (items.length > from + PAGE) {
        const more = UI.btn(`Show ${Math.min(PAGE, items.length - from - PAGE)} more of ${UI.n(items.length - from - PAGE)} remaining`, { cls: "sm more" });
        more.addEventListener("click", () => { more.remove(); next(from + PAGE); });
        parent.append(more);
      }
    };
    next(0);
  }

  function ensureBar(root) {
    if (root.querySelector(".treebar")) return;
    const search = el("input", { type: "search", class: "grow-in", placeholder: "Search the tree…", "aria-label": "Search the tree", autocomplete: "off" });
    search.addEventListener("input", UI.debounce(() => { query = search.value; build(); }, 250));
    const exp = UI.btn("Expand all", { cls: "sm", icon: "plus" });
    const col = UI.btn("Collapse all", { cls: "sm", icon: "minus" });
    exp.addEventListener("click", () => { SECTIONS.forEach((s) => open.add("sec:" + s.id)); build(true); });
    col.addEventListener("click", () => { open.clear(); open.add("root"); build(); });
    root.append(el("div", { class: "treebar" }, el("div", { class: "searchbox" }, UI.icon("search"), search), exp, col, el("span", { class: "hint", text: "Select any item for its full evidence." })), el("div", { id: "treeBody", class: "tree" }));
  }

  function build(expandAll) {
    const S = UI.S;
    const body = $("treeBody");
    if (!body) return;
    const q = query.toLowerCase();
    body.replaceChildren();
    const ents = [...S.entities.values()].filter((e) => !q || (e.value + " " + e.type).toLowerCase().includes(q));
    const finds = S.findings.filter((f) => !q || (f.source + " " + (f.url || "") + " " + f.module + " " + JSON.stringify(f.details || {})).toLowerCase().includes(q));
    const known = new Set(SECTIONS.flatMap((s) => s.cats || []));
    const t = S.target || {};
    const subject = UI.targetLabel(t) || "Subject";

    if (!ents.length && !finds.length) {
      body.append(UI.emptyState("tree", q ? "No match in the tree" : S.state === "running" ? "Collecting…" : "Nothing collected", q ? "Try a shorter search." : "Entities and findings appear here as they are found."));
      return;
    }
    const rootNode = group("root", "eye", "Subject · " + UI.clip(subject, 60), finds.length + ents.length, (d) => {
      for (const s of SECTIONS) {
        const sEnts = ents.filter((e) => s.ents.includes(e.type) || (s.cats === null && !SECTIONS.some((x) => x.ents.includes(e.type))));
        const sFinds = finds.filter((f) => (s.cats === null ? !known.has(f.category) : s.cats.includes(f.category)));
        const total = sEnts.length + sFinds.length;
        if (!total) continue;
        d.append(group("sec:" + s.id, s.icon, s.label, total, (sd) => {
          const types = [...new Set(sEnts.map((e) => e.type))];
          for (const type of types) {
            const list = sEnts.filter((e) => e.type === type).sort((a, b) => (b.strong - a.strong) || a.value.localeCompare(b.value));
            sd.append(group("ent:" + s.id + ":" + type, UI.type(type).icon, UI.type(type).label, list.length, (gd) => addLeaves(gd, list, entityLeaf), list.length <= 12 || expandAll || !!q));
          }
          const cats = {};
          for (const f of sFinds) (cats[f.category || "other"] = cats[f.category || "other"] || []).push(f);
          for (const c of Object.keys(cats).sort((a, b) => cats[b].length - cats[a].length)) {
            const list = cats[c].sort((a, b) => b.confidence - a.confidence);
            sd.append(group("cat:" + s.id + ":" + c, UI.cat(c).icon, UI.cat(c).label, list.length, (gd) => addLeaves(gd, list, findingLeaf), list.length <= 6 || expandAll || !!q));
          }
        }, true));
      }
    }, true);
    body.append(rootNode);
  }

  UI.views.tree = function tree() {
    const root = $("tab-tree");
    ensureBar(root);
    build();
  };
  UI.resetHooks.push(() => { query = ""; open.clear(); open.add("root"); SECTIONS.forEach((s) => open.add("sec:" + s.id)); });
})();
