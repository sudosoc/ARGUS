"use strict";

// Derived data model: one cached object per (case, findings revision) that every
// section of the two-level navigation reads. Nothing here touches the DOM.
// It answers "what do we know?" (accounts, identifiers, shared values, graph,
// conflicts, duplicates, validation) from the findings the server streamed.
(function () {
  const FOUND = new Set(["found", "uncertain"]);
  const cache = { key: "", m: null };

  UI.M = function M() {
    const S = UI.S;
    if (!S) return null;
    const key = [S.id, S.rev, S.report ? 1 : 0, (S.identities || []).length, S.state].join("|");
    if (cache.key !== key) { cache.key = key; cache.m = build(S); }
    return cache.m;
  };

  // ---------------------------------------------------------------- helpers
  const lc = (s) => String(s || "").toLowerCase();
  const ekey = (e) => e.type + ":" + lc(e.value);
  const uniq = (a) => [...new Set(a)];

  /** Normalise a URL for duplicate detection: host without www + path without trailing slash, no query/fragment. */
  UI.normUrl = (u) => {
    try { const x = new URL(u); return (x.hostname.replace(/^www\./, "") + x.pathname.replace(/\/+$/, "")).toLowerCase(); } catch { return ""; }
  };

  // Validation rules for entity values (syntax only: nothing is contacted).
  const RULES = {
    email: (v) => /^[^\s@]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$/.test(v),
    domain: (v) => /^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z][a-z0-9-]{1,62}$/i.test(v),
    ip: (v) => /^(\d{1,3}\.){3}\d{1,3}$/.test(v) ? v.split(".").every((x) => +x <= 255) : /^[0-9a-f:]+$/i.test(v) && v.includes(":"),
    phone: (v) => /^\+?[0-9][0-9 ()-]{5,18}$/.test(v),
    url: (v) => /^https?:\/\/\S+$/.test(v),
    asn: (v) => /^AS\d{1,10}$/i.test(v),
    wallet: (v) => /^(0x[0-9a-fA-F]{40}|bc1[0-9a-z]{20,80}|[13][1-9A-HJ-NP-Za-km-z]{25,34})$/.test(v),
    username: (v) => v.length >= 2 && v.length <= 64 && !/\s/.test(v),
  };
  const FREE = /(^|\.)(gmail|googlemail|yahoo|ymail|outlook|hotmail|live|icloud|me|proton|protonmail|pm|aol|gmx|mail|yandex|zoho)\./i;
  const DISPOSABLE = /(mailinator|guerrillamail|10minutemail|tempmail|trashmail|yopmail|sharklasers)\./i;
  UI.validate = (e) => {
    const rule = RULES[e.type];
    const notes = [];
    let ok = true;
    if (rule && !rule(e.value)) { ok = false; notes.push("does not look like a valid " + e.type); }
    if (e.type === "email") {
      const dom = e.value.split("@")[1] || "";
      if (DISPOSABLE.test(dom + ".")) notes.push("disposable mail provider");
      else if (FREE.test(dom + ".")) notes.push("public mail provider");
      else if (dom) notes.push("own / organisation domain");
    }
    if (e.type === "ip" && /^(10\.|192\.168\.|127\.|172\.(1[6-9]|2\d|3[01])\.)/.test(e.value)) { ok = false; notes.push("private address"); }
    return { ok: ok || !rule, applicable: !!rule, notes };
  };

  function build(S) {
    const rep = S.report || {};
    const target = Object.assign({}, S.target || {}, rep.target || {});
    const findings = S.findings;
    const hits = findings.filter((f) => FOUND.has(f.status));
    const ents = [...S.entities.values()];
    const byType = {};
    for (const e of ents) (byType[e.type] = byType[e.type] || []).push(e);
    const ids = S.identities || [];
    const primary = ids.find((i) => !i.weak && i.primary) || ids.find((i) => !i.weak) || null;

    const M = {
      S, rep, target, findings, hits, ents, byType, ids, primary,
      photos: UI.photoList(),
      notes: rep.notes || {},
      timeline: rep.timeline || [],
      geo: rep.geo || null,
      tz: rep.geo && rep.geo.timezone ? rep.geo.timezone : null,
      modules: rep.modules || [],
      failures: rep.failures || [],
      toolRuns: rep.tool_runs || [],
      leads: rep.manual_leads || [],
      done: S.state !== "running",
    };

    /** Hits of the given modules / categories (status found or uncertain). */
    M.pick = ({ mods, cats, found, any } = {}) => (any ? findings : hits).filter((f) =>
      (!mods || mods.includes(f.module)) && (!cats || cats.includes(f.category || "other")) && (!found || f.status === "found"));
    M.entsOf = (...types) => ents.filter((e) => types.includes(e.type));

    // Accounts: a username-site hit or a deep profile read.
    const seenAcc = new Set();
    M.accounts = hits.filter((f) => (f.module === "social" && f.status === "found") || (f.module === "profiles" && f.category === "profile") ||
      (f.module === "profiles" && f.category === "social") || (f.module === "syswrap" && f.category === "social") || (f.module === "people" && f.category === "profile"))
      .filter((f) => { const k = UI.normUrl(f.url) || f.source; if (seenAcc.has(k)) return false; seenAcc.add(k); return true; });

    // Username each account was found under.
    M.accountUser = (f) => (f.details && (f.details.username || f.details.handle)) || "";

    // ---- entity -> sources (for shared identifiers and correlation)
    M.shared = ents.map((e) => {
      const fs = e.findings || [];
      const sources = uniq(fs.map((f) => f.source));
      const modules = uniq(fs.map((f) => f.module));
      return { e, key: ekey(e), sources, modules, n: fs.length, findings: fs };
    }).filter((x) => x.sources.length >= 2 && x.e.type !== "image").sort((a, b) => b.sources.length - a.sources.length || b.modules.length - a.modules.length);

    // ---- graph: entities and the findings that mention them (bipartite), plus the subject
    let G = null;
    M.graph = () => {
      if (G) return G;
      const nodes = new Map(), adj = new Map();
      const node = (k, d) => { if (!nodes.has(k)) { nodes.set(k, d); adj.set(k, []); } return nodes.get(k); };
      const link = (a, b, via) => { adj.get(a).push({ to: b, via }); adj.get(b).push({ to: a, via }); };
      node("subject", { kind: "subject", label: UI.targetLabel(target) || "subject" });
      for (const e of ents) node(ekey(e), { kind: "entity", type: e.type, value: e.value, strong: !!e.strong });
      let fi = 0;
      for (const f of hits) {
        const keys = (f.entities || []).map(ekey).filter((k) => nodes.has(k));
        const via = (f.via || "").includes(":") ? (f.via.split(":")[0] + ":" + lc(f.via.slice(f.via.indexOf(":") + 1))) : "";
        if (!keys.length && !(via && nodes.has(via))) continue;
        const fk = "f" + fi++;
        node(fk, { kind: "finding", f });
        for (const k of uniq(keys)) link(fk, k, f.source);
        if (via && nodes.has(via) && !keys.includes(via)) link(fk, via, "followed from " + f.via);
      }
      const t = target;
      const inputs = { name: t.full_name, username: t.username, email: t.email, phone: t.phone, domain: t.domain, ip: t.ip, asn: t.asn, wallet: t.wallet, organisation: t.company };
      for (const [type, v] of Object.entries(inputs)) if (v) { const k = type + ":" + lc(v); if (nodes.has(k)) link("subject", k, "supplied by you"); }
      for (const a of t.aliases || []) { const k = "username:" + lc(a); if (nodes.has(k)) link("subject", k, "derived alias"); }
      return (G = { nodes, adj });
    };

    // Shortest path between two node keys (BFS). Returns [{key, via}] or null.
    M.path = (from, to) => {
      const { adj } = M.graph();
      if (!adj.has(from) || !adj.has(to)) return null;
      const prev = new Map([[from, null]]);
      const q = [from];
      while (q.length) {
        const k = q.shift();
        if (k === to) break;
        for (const { to: n, via } of adj.get(k)) if (!prev.has(n)) { prev.set(n, { k, via }); q.push(n); }
      }
      if (!prev.has(to)) return null;
      const out = [];
      for (let k = to; k != null;) { const p = prev.get(k); out.unshift({ key: k, via: p ? p.via : "" }); k = p ? p.k : null; }
      return out;
    };

    // Clusters: connected groups of entities, not counting the subject (which would join everything) and
    // ignoring findings that list dozens of entities at once (bulk lists would glue unrelated groups).
    M.clusters = () => {
      const { nodes, adj } = M.graph();
      const seen = new Set(["subject"]);
      const out = [];
      for (const [k, n] of nodes) {
        if (n.kind !== "entity" || seen.has(k)) continue;
        const comp = [], q = [k];
        seen.add(k);
        while (q.length) {
          const c = q.shift();
          const cn = nodes.get(c);
          if (cn.kind === "entity") comp.push(c);
          for (const { to } of adj.get(c)) {
            if (seen.has(to)) continue;
            const tn = nodes.get(to);
            if (tn.kind === "finding" && adj.get(to).length > 40) continue;
            seen.add(to); q.push(to);
          }
        }
        if (comp.length >= 2) out.push(comp.map((c) => nodes.get(c)));
      }
      return out.sort((a, b) => b.length - a.length);
    };

    // ---- conflicts: things the evidence disagrees on
    M.conflicts = () => {
      const out = [];
      for (const i of ids) for (const c of i.conflicts || []) out.push({ kind: "identity", title: i.label, detail: c });
      if (M.geo && M.geo.conflict) out.push({ kind: "location", title: "Location evidence disagrees", detail: M.geo.conflict });
      if (M.geo && M.geo.given_fit === "differs") out.push({ kind: "location", title: "Different from the place you gave", detail: `You gave ${M.geo.given}; the evidence points to ${M.geo.consensus || "elsewhere"}` });
      if (primary) {
        for (const [label, facts] of [["names", primary.names], ["employers", primary.employers], ["locations", primary.locations]]) {
          const f = (facts || []);
          if (f.length > 1) out.push({ kind: "attribute", title: `Several ${label} for the best match`, detail: f.map((x) => `${x.value} (${(x.sources || []).join(", ")})`).join(" · ") });
        }
      }
      // the same handle on one platform resolving to different display names
      const byUser = new Map();
      for (const f of M.accounts) { const n = f.details && (f.details.name || f.details["page title"]); const u = lc(M.accountUser(f)); if (!n || !u) continue; (byUser.get(u) || byUser.set(u, new Map()).get(u)).set(lc(n), n); }
      for (const [u, names] of byUser) if (names.size > 2) out.push({ kind: "accounts", title: `Handle “${u}” shows ${names.size} different names`, detail: [...names.values()].slice(0, 6).join(" · ") });
      return out;
    };

    // ---- duplicates: the same thing reported more than once
    M.duplicates = () => {
      const groups = [];
      const byUrl = new Map();
      for (const f of hits) { const k = UI.normUrl(f.url); if (!k) continue; (byUrl.get(k) || byUrl.set(k, []).get(k)).push(f); }
      for (const [k, list] of byUrl) if (new Set(list.map((f) => f.module + "|" + f.source)).size > 1) groups.push({ kind: "same page", key: k, items: list });
      const bySrc = new Map();
      for (const f of hits) { const k = f.module + "|" + f.source + "|" + (f.url || ""); (bySrc.get(k) || bySrc.set(k, []).get(k)).push(f); }
      for (const [k, list] of bySrc) if (list.length > 1) groups.push({ kind: "repeated finding", key: k.replace(/\|/g, " · "), items: list });
      const bySha = new Map();
      for (const p of M.photos) if (p.sha256) (bySha.get(p.sha256) || bySha.set(p.sha256, []).get(p.sha256)).push(p);
      for (const [k, list] of bySha) if (list.length > 1) groups.push({ kind: "identical picture", key: k.slice(0, 16) + "…", items: list });
      return groups.sort((a, b) => b.items.length - a.items.length);
    };

    // ---- source reliability per collector
    M.modStats = () => {
      const m = new Map();
      const get = (n) => m.get(n) || m.set(n, { name: n, n: 0, hit: 0, err: 0, conf: 0, first: 0, last: 0 }).get(n);
      for (const f of findings) {
        const s = get(f.module);
        s.n++;
        if (f.status === "found" || f.status === "uncertain") { s.hit++; s.conf += f.confidence || 0; }
        if (f.status === "error") s.err++;
        const t = Date.parse(f.timestamp);
        if (t) { s.first = s.first ? Math.min(s.first, t) : t; s.last = Math.max(s.last, t); }
      }
      for (const r of M.modules) { const s = get(r.name); s.ms = (s.ms || 0) + (r.duration_ms || 0); if (!s.n) { s.n = r.checked || 0; s.hit = r.found || 0; s.err = r.errors || 0; } }
      return [...m.values()].map((s) => Object.assign(s, { avg: s.hit ? s.conf / s.hit : 0, errRate: s.n ? s.err / s.n : 0 })).sort((a, b) => UI.modRank(a.name) - UI.modRank(b.name));
    };

    // Reliability of a source type: how much a hit from it can be trusted on its own.
    UI.RELIABILITY = {
      registry: ["High", "official registers and registries"], bizreg: ["High", "official business registers"], domain: ["High", "DNS, RDAP and TLS are read directly from the infrastructure"],
      ip: ["High", "RDAP, ASN and passive scan data"], asn: ["High", "routing and registration data"], phone: ["High", "offline number analysis"], keys: ["High", "published cryptographic keys"],
      iddecode: ["High", "arithmetic on identifiers, no lookup"], crypto: ["High", "public ledger data"], sanctions: ["Medium", "name match on official lists: needs human comparison"],
      profiles: ["Medium", "profile data the account owner published"], people: ["Medium", "third-party person records"], devtrace: ["Medium", "public developer traces"], social: ["Medium", "an account with this handle exists, not necessarily the same person"],
      activity: ["Medium", "inferred from public timestamps"], webhistory: ["Medium", "archived copies"], footprint: ["Medium", "mixed public sources"], breach: ["Medium", "third-party breach index (names only)"],
      mentions: ["Low", "press mentions of a name that may be shared"], dorks: ["Low", "search-engine results: leads, not evidence"], images: ["Low", "pictures matched by handle or name"], codesearch: ["Low", "text matches in public code"],
      lookalike: ["Medium", "DNS answers; intent is not known"], bskygraph: ["Medium", "public follow lists"], sitefiles: ["Medium", "what a site publishes about itself"], docmeta: ["Medium", "metadata inside public documents"],
      records: ["Medium", "public filings that match a name"], company: ["Medium", "public company data"], syswrap: ["Medium", "output of external tools"],
    };
    return M;
  }
})();
