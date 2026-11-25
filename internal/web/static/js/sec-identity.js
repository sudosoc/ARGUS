"use strict";

// Overview and Identity views.
(function () {
  const { el } = UI;
  const K = UI.kit;
  const lc = (s) => String(s || "").toLowerCase();

  // ---- Overview: the dashboard stays as it was
  UI.defineTab({ id: "overview", label: "Overview", icon: "layers", sections: [{ id: "summary", label: "Summary", icon: "layers", legacy: "overview", always: true }] });

  // ---------------------------------------------------------------- helpers
  const usernameRows = (M) => {
    const t = M.target, rows = new Map();
    const get = (u) => { const k = lc(u); if (!rows.has(k)) rows.set(k, { name: u, role: new Set(), platforms: new Set(), n: 0, conf: 0, urls: [] }); return rows.get(k); };
    if (t.username) get(t.username).role.add("supplied");
    for (const a of t.aliases || []) get(a).role.add("derived alias");
    if (t.alias) get(t.alias).role.add("supplied alias");
    for (const e of M.entsOf("username")) { const r = get(e.value); if (!r.role.size) r.role.add("discovered"); if (e.source) r.role.add("found by " + e.source); }
    for (const f of M.accounts) {
      const u = M.accountUser(f); if (!u) continue;
      const r = get(u); r.platforms.add(f.source.replace(/ profile$/i, "")); r.n++; r.conf = Math.max(r.conf, f.confidence || 0); if (f.url) r.urls.push(f.url);
    }
    return [...rows.values()].sort((a, b) => b.platforms.size - a.platforms.size || a.name.localeCompare(b.name));
  };

  // ---------------------------------------------------------------- sections
  const profile = { id: "profile", label: "Profile", icon: "id", legacy: "identity", always: true, count: (M) => M.ids.filter((i) => !i.weak).length };

  const aliases = {
    id: "aliases", label: "Aliases", icon: "tag",
    count: (M) => (M.target.aliases || []).length + (M.target.alias ? 1 : 0) + (M.primary ? (M.primary.names || []).length : 0),
    render(page, M) {
      const t = M.target;
      const names = new Map();
      const add = (v, kind, src) => { if (!v) return; const k = lc(v); const r = names.get(k) || names.set(k, { v, kinds: new Set(), src: new Set() }).get(k); r.kinds.add(kind); if (src) src.forEach((x) => r.src.add(x)); };
      add(t.full_name, "name you supplied");
      add(t.alias, "alias you supplied");
      for (const a of t.aliases || []) add(a, "handle derived by SUDOSOC ARGUS (e-mail or name variants)");
      for (const i of M.ids) for (const f of i.names || []) add(f.value, i.weak ? "name on a weakly-matched account" : "name stated by a profile", f.sources);
      for (const e of M.entsOf("name")) add(e.value, "name found in a finding", e.source ? [e.source] : []);
      const rows = [...names.values()];
      page.append(K.stats([[rows.length, "names & aliases", { lead: true }], [(t.aliases || []).length, "derived handles"], [M.entsOf("name").length, "names found"]]));
      page.append(K.panel("Names and aliases", "tag", K.opts({ count: rows.length }),
        K.hint("Every spelling the investigation used or discovered. Derived handles are what SUDOSOC ARGUS probes on the username sites when it only has a name or an e-mail."),
        K.table([{ label: "Name / alias", get: (r) => r.v }, { label: "Origin", get: (r) => [...r.kinds].join(" · ") }, { label: "Stated by", get: (r) => [...r.src].join(", ") }], rows, { empty: "No aliases yet." })));
    },
  };

  const usernames = {
    id: "usernames", label: "Usernames", icon: "at", count: (M) => usernameRows(M).length,
    render(page, M) {
      const rows = usernameRows(M);
      const reused = rows.filter((r) => r.platforms.size >= 2);
      page.append(K.stats([[rows.length, "handles", { lead: true }], [reused.length, "found on 2+ platforms"], [rows.reduce((a, r) => a + r.platforms.size, 0), "account sightings"]]));
      page.append(K.panel("Handles", "at", K.opts({ count: rows.length }),
        K.table([{ label: "Handle", get: (r) => r.name }, { label: "Role", get: (r) => [...r.role].slice(0, 3).join(" · ") }, { label: "Platforms", cls: "r", get: (r) => r.platforms.size },
          { label: "Where", get: (r) => [...r.platforms].slice(0, 8).join(", ") + (r.platforms.size > 8 ? ` +${r.platforms.size - 8}` : "") }], rows,
          { onRow: (r) => UI.gotoFindings({ q: r.name }), empty: "No usernames." })));
    },
  };

  const mailFlags = (M, addr) => {
    const a = lc(addr), fl = [];
    for (const f of M.hits) {
      if (!lc(JSON.stringify(f.details || {}) + " " + f.source + " " + (f.entities || []).map((e) => e.value).join(" ")).includes(a)) continue;
      if (f.module === "breach") fl.push("breach index");
      else if (f.module === "keys") fl.push("PGP key");
      else if (f.module === "footprint" && /gravatar/i.test(f.source)) fl.push("Gravatar");
      else if (f.module === "footprint" && /stealer|infostealer/i.test(f.source + JSON.stringify(f.details))) fl.push("stealer logs");
      else if (f.module === "syswrap" && /holehe/i.test(f.source)) fl.push("registered accounts");
      else if (f.module === "codesearch") fl.push("public code");
      else if (f.module === "devtrace") fl.push("commits");
    }
    return [...new Set(fl)];
  };

  UI.mailFlags = mailFlags;
  const emails = {
    id: "emails", label: "E-mail addresses", icon: "mail", count: (M) => M.entsOf("email").length,
    render(page, M) {
      const list = M.entsOf("email");
      const rows = list.map((e) => ({ e, v: UI.validate(e), fl: mailFlags(M, e.value) }));
      page.append(K.stats([[list.length, "addresses", { lead: true }], [list.filter((e) => e.strong).length, "strong leads"], [rows.filter((r) => r.fl.length).length, "with public traces"]]));
      page.append(K.panel("Addresses", "mail", K.opts({ count: list.length }),
        K.table([{ label: "Address", get: (r) => r.e.value }, { label: "Kind", get: (r) => r.v.notes.join(" · ") || "—" }, { label: "Lead", get: (r) => (r.e.strong ? "strong" : "weak") },
          { label: "Seen in", get: (r) => (r.e.findings || []).length + " findings" }, { label: "Public traces", get: (r) => r.fl.join(", ") || "—" }], rows,
          { onRow: (r) => UI.showEntity(r.e), empty: "No e-mail addresses found." })));
    },
  };

  const phones = {
    id: "phones", label: "Phone numbers", icon: "phone", count: (M) => M.entsOf("phone").length + (M.target.phone ? 1 : 0),
    render(page, M) {
      const list = M.pick({ mods: ["phone"] });
      const ents = M.entsOf("phone");
      page.append(K.stats([[ents.length + (M.target.phone && !ents.some((e) => e.value === M.target.phone) ? 1 : 0), "numbers", { lead: true }], [list.length, "analysis results"]]));
      if (ents.length) page.append(K.panel("Numbers", "phone", K.opts({ count: ents.length }), K.entityChips(ents)));
      page.append(K.findings("Number analysis", "phone", list, { empty: "No number analysis. Give a phone number (with + and country code) to get country, line type, carrier and time zone.", showModule: false }));
    },
  };

  const identifiers = {
    id: "identifiers", label: "Digital identifiers", icon: "hash",
    count: (M) => idRows(M).length,
    render(page, M) {
      const rows = idRows(M);
      const byKind = {};
      for (const r of rows) byKind[r.kind] = (byKind[r.kind] || 0) + 1;
      page.append(K.stats([[rows.length, "identifiers", { lead: true }], [Object.keys(byKind).length, "kinds"]]));
      page.append(K.panel("Stable identifiers", "hash", K.opts({ count: rows.length }),
        K.hint("Values that identify an account, key or asset on their own: platform IDs, key fingerprints, wallets, analytics IDs, picture hashes."),
        K.table([{ label: "Kind", get: (r) => r.kind }, { label: "Value", get: (r) => r.value }, { label: "From", get: (r) => r.from }], rows, { empty: "No identifiers yet." })));
    },
  };
  function idRows(M) {
    const rows = [];
    for (const i of M.ids) for (const [k, v] of Object.entries(i.ids || {})) rows.push({ kind: k, value: v, from: i.label });
    for (const f of M.hits) {
      const d = f.details || {};
      if (f.category === "pgp" && d.fingerprint) rows.push({ kind: "PGP fingerprint", value: d.fingerprint, from: f.source });
      if (f.category === "idinfo" && d.id) rows.push({ kind: (d.platform || "platform") + " ID", value: d.id, from: f.source });
      if (f.category === "tracking") for (const [k, v] of Object.entries(d)) if (/^(UA-|G-|GTM-|pub-|\d{8,})/.test(String(v).split(/[ ,]/)[0]) || /analytics|pixel|adsense|tag manager|hotjar|metrika/i.test(k)) rows.push({ kind: k, value: String(v).split("\n")[0], from: f.source });
      if (f.module === "profiles" && d.uid) rows.push({ kind: f.source + " user id", value: d.uid, from: f.source });
    }
    for (const e of M.entsOf("wallet", "asn")) rows.push({ kind: e.type, value: e.value, from: e.source || "" });
    for (const p of M.photos) if (p.sha256) rows.push({ kind: "picture SHA-256", value: p.sha256.slice(0, 32) + "…", from: (p.sources || [])[0] || "" });
    const seen = new Set();
    return rows.filter((r) => { const k = r.kind + "|" + r.value; if (seen.has(k)) return false; seen.add(k); return true; });
  }

  const verification = {
    id: "verification", label: "Identity verification", icon: "shield", always: true,
    render(page, M) {
      const p = M.primary;
      if (!p) { page.append(K.empty("shield", "No resolved identity yet", "Verification explains why a person was (or was not) matched to what you supplied.", "supply a name plus a second detail (e-mail, handle, place or employer) so records can be linked and scored.")); return; }
      const facts = ["names", "headlines", "employers", "locations", "education", "emails", "phones", "usernames", "websites"].flatMap((k) => (p[k] || []).map((f) => ({ k, f })));
      const confirmed = facts.filter((x) => x.f.confirmed).length, own = facts.filter((x) => x.f.self && !x.f.confirmed).length;
      page.append(K.panel("Match score", "shield", K.gauge(UI.pct(p.score), "match with what you supplied", p.weak ? "weak: matched only by a shared handle" : null)));
      page.append(K.stats([[confirmed, "confirmed facts (2+ platforms)", { lead: true }], [own, "self-reported"], [facts.length - confirmed - own, "single third-party source"], [(p.conflicts || []).length, "conflicts", { warn: (p.conflicts || []).length > 0 }]]));
      page.append(K.panel("Why this person matches", "check",
        (p.matched || []).length ? K.chips(p.matched.map((m) => ({ k: "matches", v: m }))) : K.hint("No supplied identifier matched directly."),
        (p.reasons || []).length ? el("ul", {}, ...p.reasons.map((r) => el("li", { text: r }))) : null));
      if ((p.conflicts || []).length) page.append(K.panel("Where sources disagree", "alert", el("ul", {}, ...p.conflicts.map((c) => el("li", { text: c })))));
      page.append(K.panel("Evidence behind each fact", "list", K.opts({ count: facts.length }),
        K.table([{ label: "Attribute", get: (x) => x.k }, { label: "Value", get: (x) => x.f.value }, { label: "Status", get: (x) => (x.f.confirmed ? "confirmed" : x.f.self ? "self-reported" : "single source") }, { label: "Stated by", get: (x) => (x.f.sources || []).join(", ") }], facts)));
    },
  };

  UI.defineTab({ id: "identity", label: "Identity", icon: "id", count: (M) => M.ids.filter((i) => !i.weak).length, sections: [profile, aliases, usernames, emails, phones, identifiers, verification] });
})();
