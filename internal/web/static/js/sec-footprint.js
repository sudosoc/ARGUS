"use strict";

// Copyright (C) 2026 Seif, SUDOSOC. SPDX-License-Identifier: AGPL-3.0-only

// Footprint: the public trace of the subject on the web and on social platforms.
(function () {
  const { el } = UI;
  const K = UI.kit;
  const lc = (s) => String(s || "").toLowerCase();

  const WEB_CATS = ["web", "website", "tech", "security", "contact", "social-links", "site-people", "document-metadata", "email-security", "reference", "publication", "registry", "company"];
  const MENTION_MODS = ["mentions", "codesearch", "dorks", "records"];
  const ARCHIVE = (f) => f.category === "archive" || f.category === "history" || f.module === "webhistory" || /archive\.org|wayback/i.test(f.url || "");

  const webPresence = { id: "web", label: "Website presence", icon: "globe", group: "Digital footprint", count: (M) => M.pick({ cats: WEB_CATS }).filter((f) => f.module !== "dorks").length,
    render(page, M) {
      const list = M.pick({ cats: WEB_CATS }).filter((f) => f.module !== "dorks");
      const domains = M.entsOf("domain");
      page.append(K.stats([[domains.length, "domains", { lead: true }], [list.length, "web findings"], [M.pick({ cats: ["site-people"] }).length, "people on sites"], [M.pick({ cats: ["social-links"] }).length, "social links on sites"]]));
      page.append(K.findings("What the sites say about themselves", "globe", list, { empty: "No website findings. Give a domain (or an e-mail on your own domain) to read its site, contacts, people and published metadata." }));
    } };

  const publicAccounts = { id: "accounts", label: "Public accounts", icon: "user", group: "Digital footprint", count: (M) => M.accounts.length,
    render(page, M) {
      const by = new Map();
      for (const f of M.accounts) { const k = f.module === "social" ? UI.cat(f.category).label : "Deep profile"; by.set(k, (by.get(k) || 0) + 1); }
      page.append(K.stats([[M.accounts.length, "accounts", { lead: true }], [new Set(M.accounts.map((f) => UI.host(f.url))).size, "distinct sites"], [M.accounts.filter((f) => f.confidence >= 0.8).length, "high confidence"]]));
      if (by.size) page.append(K.panel("By kind of site", "layers", K.bars([...by.entries()].sort((a, b) => b[1] - a[1]))));
      page.append(K.findings("Accounts", "user", [...M.accounts].sort((a, b) => b.confidence - a.confidence), { empty: "No accounts yet. Give a username (or a name/e-mail the planner can derive one from).", showModule: false, page: 60 }));
    } };

  const historical = { id: "historical", label: "Historical footprints", icon: "clock", group: "Digital footprint",
    count: (M) => M.hits.filter(ARCHIVE).length + M.timeline.filter((e) => /archive|first|wayback|account created|joined/i.test(e.event + e.source)).length,
    render(page, M) {
      const list = M.hits.filter(ARCHIVE);
      const ev = M.timeline.filter((e) => /archive|first|wayback|account created|joined|registered|created/i.test(e.event + " " + e.source));
      page.append(K.stats([[list.length, "archive findings", { lead: true }], [ev.length, "dated events"]]));
      if (ev.length) page.append(K.panel("Dated traces", "clock", K.opts({ count: ev.length }), K.table([{ label: "Date", get: (e) => e.date }, { label: "Event", get: (e) => e.event }, { label: "Source", get: (e) => e.source }], [...ev].sort((a, b) => a.date.localeCompare(b.date)))));
      page.append(K.findings("From the web archive", "clock", list, { empty: "No archive history. Give a domain or profile link: the Internet Archive is asked when each was first and last captured." }));
    } };

  const reuse = { id: "reuse", label: "Username reuse", icon: "at", group: "Digital footprint", count: (M) => reuseRows(M).length,
    render(page, M) {
      const rows = reuseRows(M);
      page.append(K.stats([[rows.length, "handles on 2+ sites", { lead: true }], [rows.filter((r) => r.sites.size >= 5).length, "on 5+ sites"], [rows.reduce((a, r) => Math.max(a, r.sites.size), 0), "widest reuse"]]));
      page.append(K.panel("Handles reused across platforms", "at", K.opts({ count: rows.length }),
        K.hint("People tend to reuse a handle. A handle on many unrelated sites ties those accounts together; a common handle (john, admin) proves little."),
        K.table([{ label: "Handle", get: (r) => r.name }, { label: "Sites", cls: "r", get: (r) => r.sites.size }, { label: "Where", get: (r) => [...r.sites].slice(0, 12).join(", ") + (r.sites.size > 12 ? " …" : "") }], rows, { onRow: (r) => UI.gotoFindings({ q: r.name }), empty: "No handle was found on more than one site." })));
    } };
  function reuseRows(M) {
    const m = new Map();
    for (const f of M.accounts) { const u = M.accountUser(f); if (!u) continue; const k = lc(u); (m.get(k) || m.set(k, { name: u, sites: new Set() }).get(k)).sites.add(f.source.replace(/ profile$/i, "")); }
    return [...m.values()].filter((r) => r.sites.size >= 2).sort((a, b) => b.sites.size - a.sites.size);
  }

  const mentions = { id: "mentions", label: "Public mentions", icon: "list", group: "Digital footprint", count: (M) => M.pick({ mods: MENTION_MODS }).length,
    render(page, M) {
      const list = M.pick({ mods: MENTION_MODS });
      page.append(K.stats([["" + M.pick({ mods: ["mentions"] }).length, "press mentions", { lead: true }], [M.pick({ mods: ["codesearch"] }).length, "code matches"], [M.pick({ mods: ["dorks"] }).length, "search results"], [M.pick({ mods: ["records"] }).length, "filings & scans"]]));
      page.append(K.findings("Where the name, e-mail or domain is mentioned", "list", list, { empty: "No public mentions. Names, e-mails and domains are searched in news, public code, filings and search engines." }));
    } };

  const cached = { id: "cached", label: "Cached & archived pages", icon: "file", group: "Digital footprint",
    count: (M) => M.hits.filter((f) => ARCHIVE(f) || /archive\.(org|ph|today)/i.test(JSON.stringify(f.details || {}))).length,
    render(page, M) {
      const list = M.hits.filter((f) => ARCHIVE(f) || /archive\.(org|ph|today)/i.test(JSON.stringify(f.details || {})));
      page.append(K.findings("Archived copies", "file", list, { empty: "No archived pages found." }));
      const urls = [...new Set(M.accounts.map((f) => f.url).filter(Boolean))].slice(0, 30);
      if (urls.length) page.append(K.panel("Look up an archived copy by hand", "external", K.hint("Open the Wayback Machine for each profile found."), K.links(urls.map((u) => ({ label: u, url: "https://web.archive.org/web/*/" + u, why: "Wayback Machine" })))));
    } };

  // ---- Social profiles
  const platforms = { id: "platforms", label: "Platform discovery", icon: "layers", group: "Social profiles", count: (M) => M.accounts.length,
    render(page, M) {
      const rows = [...M.accounts].sort((a, b) => b.confidence - a.confidence);
      const cats = new Map();
      for (const f of rows) { const c = UI.cat(f.category).label; cats.set(c, (cats.get(c) || 0) + 1); }
      page.append(K.stats([[rows.length, "platforms with an account", { lead: true }], [rows.filter((f) => f.status === "found").length, "confirmed present"], [cats.size, "kinds of site"]]));
      if (cats.size > 1) page.append(K.panel("Kinds of site", "layers", K.bars([...cats.entries()].sort((a, b) => b[1] - a[1]).slice(0, 14))));
      page.append(K.panel("Where an account exists", "user", K.opts({ count: rows.length }),
        K.table([{ label: "Platform", get: (f) => f.source }, { label: "Handle", get: (f) => M.accountUser(f) }, { label: "Confidence", get: (f) => K.conf(f.confidence) }, { label: "Profile", get: (f) => (f.url ? (() => { const h = UI.safeHref(f.url); return h ? el("a", { href: h, target: "_blank", rel: "noopener noreferrer", text: UI.host(f.url) }) : f.url; })() : "") }], rows,
          { onRow: (f) => UI.showFinding(f), empty: "No accounts discovered." })));
    } };

  const analysis = { id: "analysis", label: "Profile analysis", icon: "id", group: "Social profiles",
    count: (M) => profileRows(M).length,
    render(page, M) {
      const rows = profileRows(M);
      page.append(K.stats([[rows.length, "profiles with data", { lead: true }], [rows.filter((r) => r.bio).length, "with a bio"], [rows.filter((r) => r.loc).length, "with a place"], [rows.filter((r) => r.avatar).length, "with a picture"]]));
      page.append(K.panel("What each profile reveals", "id", K.opts({ count: rows.length }),
        K.table([{ label: "Platform", get: (r) => r.f.source }, { label: "Name", get: (r) => r.name }, { label: "Place", get: (r) => r.loc }, { label: "Bio", get: (r) => UI.clip(r.bio, 140) }, { label: "Completeness", get: (r) => K.conf(r.fill / 4) }], rows, { onRow: (r) => UI.showFinding(r.f), empty: "No profile carried a name, bio or place." })));
    } };
  function profileRows(M) {
    return M.accounts.map((f) => {
      const d = f.details || {};
      const name = d.name || d["display name"] || d["page title"] || "", bio = d.bio || d.description || d.about || "", loc = d.location || d.city || "", avatar = d.avatar || d.image || "";
      return { f, name, bio, loc, avatar, fill: [name, bio, loc, avatar].filter(Boolean).length };
    }).filter((r) => r.fill > 0).sort((a, b) => b.fill - a.fill);
  }

  const connections = { id: "connections", label: "Account connections", icon: "link", group: "Social profiles",
    count: (M) => M.hits.filter(isLink).length,
    render(page, M) {
      const list = M.hits.filter(isLink);
      page.append(K.stats([[list.length, "links between accounts", { lead: true }], [M.pick({ cats: ["graph"] }).length, "social-graph results"], [M.hits.filter((f) => /link hub/i.test(f.source)).length, "link hubs"]]));
      page.append(K.findings("Accounts that point at each other", "link", list, { empty: "No connections found. Link hubs (Linktree…), verified cross-links (Keybase, GitHub) and follow lists are read when a username is given." }));
    } };
  const isLink = (f) => f.category === "social-links" || f.category === "graph" || /link hub|keybase|identity proofs|linked accounts|commit identities/i.test(f.source) || (f.details && (f.details["linked accounts"] || f.details.links));

  const activity = { id: "activity", label: "Public activity", icon: "pulse", group: "Social profiles",
    count: (M) => M.pick({ mods: ["activity"] }).length,
    render(page, M) {
      const list = M.pick({ mods: ["activity"] });
      page.append(K.stats([[list.length, "sources with enough events", { lead: true }], [list.reduce((a, f) => a + (+f.details.events || 0), 0), "public events read"], [M.tz ? M.tz.label : "—", "estimated time zone"]]));
      page.append(K.hint("SUDOSOC ARGUS reads when public events happened, never what was written: no post text is collected."));
      if (M.tz) { const h = UI.heatmap(M.tz); if (h) page.append(K.panel("When the subject is active", "clock", h)); const hc = UI.hoursChart(M.tz); if (hc) page.append(K.panel("Activity by hour", "pulse", hc)); }
      page.append(K.findings("Activity by source", "pulse", list, { empty: "No public activity read. Needs a username with public events (GitHub, Hacker News, Reddit, Wikipedia, Bluesky, Mastodon, Lichess…).", showModule: false }));
    } };

  const interactions = { id: "interactions", label: "Public interactions", icon: "network", group: "Social profiles",
    count: (M) => M.hits.filter((f) => f.module === "bskygraph" || f.category === "developer" || f.module === "devtrace").length,
    render(page, M) {
      const g = M.hits.filter((f) => f.module === "bskygraph"), d = M.hits.filter((f) => f.module === "devtrace" || f.category === "developer");
      page.append(K.stats([[g.length, "social circles", { lead: true }], [d.length, "developer traces"]]));
      page.append(K.findings("Social circle (follow graph)", "network", g, { empty: "No social circle: needs a Bluesky account with public follow lists.", showModule: false }));
      page.append(K.findings("Developer interactions (commits, organisations, packages)", "code", d, { empty: "No developer traces.", showModule: false }));
    } };

  const cross = { id: "cross", label: "Cross-platform matching", icon: "layers", group: "Social profiles", always: true,
    render(page, M) {
      const groups = new Map();
      for (const p of M.photos) if (p.group) (groups.get(p.group) || groups.set(p.group, []).get(p.group)).push(p);
      const ids = M.ids.filter((i) => !i.weak);
      page.append(K.stats([[ids.length, "resolved people", { lead: true }], [groups.size, "pictures used on 2+ sources"], [reuseRows(M).length, "reused handles"], [M.shared.filter((s) => s.modules.length >= 2).length, "values seen by 2+ collectors"]]));
      for (const i of ids.slice(0, 6)) {
        page.append(K.panel(i.label + (i.primary ? " (best match)" : ""), "id", K.opts({ count: (i.records || []).length }),
          (i.reasons || []).length ? el("ul", {}, ...i.reasons.map((r) => el("li", { text: r }))) : K.hint("Records linked by their own details."),
          K.table([{ label: "Platform", get: (r) => r.platform }, { label: "Source", get: (r) => r.source }, { label: "Own profile", get: (r) => (r.self ? "yes" : "") }], i.records || [])));
      }
      if (groups.size) page.append(K.panel("The same picture on several sources", "image", ...[...groups.entries()].map(([g, list]) => el("div", { class: "hint" }, `G${g}: ` + list.flatMap((p) => p.sources || []).map((s) => s.replace(/^[a-z]+: /, "")).filter((x, i, a) => a.indexOf(x) === i).join(" · ")))));
      if (!ids.length && !groups.size) page.append(K.empty("layers", "Nothing to match yet", "Matching links accounts that share an e-mail, website, picture or distinctive name.", "give a name plus a second detail, or enough accounts for pictures and handles to repeat."));
    } };

  const collectors = K.section("collectors", "Social collectors & frameworks", "user", (M) => M.hits.filter((f) => f.category === "social-scraper" || f.category === "osint-framework"), { group: "Social profiles",
    intro: "Output of the optional social-media collectors (facebook-scraper, instagram-osint, Insta OSINT Engine, open-nami, The 3rd Eye) and of SpiderFoot. The complete raw output of each run is under Evidence → Collection coverage.",
    empty: "None of these tools ran.", unlock: "install the tool, and for The 3rd Eye turn on the Google API key in API settings." });
  UI.defineTab({ id: "footprint", label: "Footprint", icon: "globe", count: (M) => M.accounts.length, sections: [collectors, webPresence, publicAccounts, historical, reuse, mentions, cached, platforms, analysis, connections, activity, interactions, cross] });
})();
