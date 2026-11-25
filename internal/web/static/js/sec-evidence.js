"use strict";

// Evidence & Coverage: where every fact came from, how far to trust it, what is
// missing, and whether the data is internally consistent.
(function () {
  const { el } = UI;
  const K = UI.kit;
  const lc = (s) => String(s || "").toLowerCase();

  // -------------------------------------------------------------- evidence
  const registry = { id: "registry", label: "Source registry", icon: "layers", group: "Evidence", count: (M) => regRows(M).length,
    render(page, M) {
      const rows = regRows(M);
      page.append(K.stats([[rows.length, "sources", { lead: true }], [new Set(rows.map((r) => r.module)).size, "collectors"], [new Set(rows.map((r) => r.host).filter(Boolean)).size, "distinct sites"]]));
      page.append(K.panel("Every source that produced evidence", "layers", K.opts({ count: rows.length }), K.table([{ label: "Collector", get: (r) => r.module }, { label: "Source", get: (r) => r.source }, { label: "Results", cls: "r", get: (r) => r.n }, { label: "Best confidence", get: (r) => K.conf(r.conf) }, { label: "Site", get: (r) => r.host }], rows, { onRow: (r) => UI.gotoFindings({ mods: [r.module], q: r.source }), empty: "No sources yet." })));
    } };
  function regRows(M) {
    const m = new Map();
    for (const f of M.hits) { if (f.category === "toolrun") continue; const k = f.module + "|" + f.source; const r = m.get(k) || m.set(k, { module: f.module, source: f.source, n: 0, conf: 0, host: "" }).get(k); r.n++; r.conf = Math.max(r.conf, f.confidence || 0); r.host = r.host || UI.host(f.url); }
    return [...m.values()].sort((a, b) => UI.modRank(a.module) - UI.modRank(b.module) || b.conf - a.conf);
  }

  let libQ = "", libMod = "", libConf = 0;
  const library = { id: "library", label: "Evidence library", icon: "file", group: "Evidence", count: (M) => M.hits.length + M.toolRuns.length, always: true,
    render(page, M) {
      const mods = [...new Set(M.hits.map((f) => f.module))].sort((a, b) => UI.modRank(a) - UI.modRank(b));
      const q = el("input", { type: "search", class: "input", placeholder: "Search every piece of evidence…", value: libQ, "aria-label": "Search evidence" });
      const sel = el("select", { "aria-label": "Collector" }, el("option", { value: "", text: "All collectors" }), ...mods.map((m) => el("option", { value: m, text: m, selected: m === libMod ? "" : null })));
      const conf = el("select", { "aria-label": "Minimum confidence" }, ...[[0, "Any confidence"], [0.5, "50% or more"], [0.8, "80% or more"]].map(([v, l]) => el("option", { value: String(v), text: l, selected: v === libConf ? "" : null })));
      const out = el("div", {});
      const draw = () => {
        const t = lc(libQ);
        const list = M.hits.filter((f) => f.category !== "toolrun" && (!libMod || f.module === libMod) && f.confidence >= libConf && (!t || UI.haystack(f).includes(t)));
        out.replaceChildren(K.findings("Evidence", "file", list, { empty: "No evidence matches.", page: 40 }));
      };
      q.addEventListener("input", UI.debounce(() => { libQ = q.value; draw(); }, 200));
      sel.addEventListener("change", () => { libMod = sel.value; draw(); });
      conf.addEventListener("change", () => { libConf = +conf.value; draw(); });
      page.append(K.panel("Evidence library", "file", K.hint("Every finding with the raw detail it was read from. Click one to see its full evidence and its raw source."), el("div", { class: "sel-row" }, q, sel, conf)), out);
      page.append(K.panel("Raw tool output", "tool", K.opts({ count: M.toolRuns.length }), M.toolRuns.length ? K.table([{ label: "Tool", get: (r) => r.tool }, { label: "Command", get: (r) => r.command || "" }, { label: "Size", cls: "r", get: (r) => UI.n(r.bytes || (r.output || "").length) + " bytes" }, { label: "Time", cls: "r", get: (r) => (r.duration_ms ? (r.duration_ms / 1000).toFixed(1) + "s" : "") }], M.toolRuns, { onRow: () => UI.go("evidence", "coverage") }) : K.hint("No external tool ran, or the investigation is still running.")));
      draw();
    } };

  const reliability = { id: "reliability", label: "Source reliability", icon: "shield", group: "Evidence", count: (M) => M.modStats().length,
    render(page, M) {
      const rows = M.modStats().filter((s) => s.n);
      page.append(K.hint("Reliability is how far a result from this kind of source can be trusted on its own, from what the source is (an official register, an account that merely exists, a search result). The measured columns show how each collector behaved in this run."));
      page.append(K.panel("Collectors", "shield", K.opts({ count: rows.length }), K.table([
        { label: "Collector", get: (s) => s.name }, { label: "Source reliability", get: (s) => (UI.RELIABILITY[s.name] ? UI.RELIABILITY[s.name][0] : "—") }, { label: "Why", get: (s) => (UI.RELIABILITY[s.name] ? UI.RELIABILITY[s.name][1] : "") },
        { label: "Hits", cls: "r", get: (s) => s.hit }, { label: "Avg confidence", get: (s) => K.conf(s.avg) }, { label: "Error rate", cls: "r", get: (s) => Math.round(s.errRate * 100) + "%" }], rows, { onRow: (s) => UI.gotoFindings({ mods: [s.name] }) })));
      const ids = M.ids.filter((i) => !i.weak);
      if (ids.length) page.append(K.panel("Facts by trust", "id", K.hint("Facts stated by two independent platforms are confirmed; a person's own profile is self-reported; a single third-party record is the weakest."),
        K.table([{ label: "Person", get: (i) => i.label }, { label: "Confirmed", cls: "r", get: (i) => factCount(i, (f) => f.confirmed) }, { label: "Self-reported", cls: "r", get: (i) => factCount(i, (f) => f.self && !f.confirmed) }, { label: "Single source", cls: "r", get: (i) => factCount(i, (f) => !f.self && !f.confirmed) }], ids)));
    } };
  const factCount = (i, pred) => ["names", "headlines", "employers", "locations", "education", "emails", "phones", "usernames", "websites"].reduce((a, k) => a + (i[k] || []).filter(pred).length, 0);

  const confidence = { id: "confidence", label: "Confidence assessment", icon: "pulse", group: "Evidence", count: (M) => M.hits.length,
    render(page, M) {
      const h = M.hits.filter((f) => f.category !== "toolrun");
      const b = [0, 0, 0, 0, 0];
      for (const f of h) b[Math.min(4, Math.floor((f.confidence || 0) * 5))]++;
      page.append(K.stats([[h.length ? Math.round((h.reduce((a, f) => a + f.confidence, 0) / h.length) * 100) + "%" : "—", "average confidence", { lead: true }], [h.filter((f) => f.confidence >= 0.8).length, "80% or more"], [h.filter((f) => f.confidence < 0.5).length, "below 50%"]]));
      page.append(K.panel("Distribution", "meter", K.bars([["80–100%", b[4]], ["60–80%", b[3]], ["40–60%", b[2]], ["20–40%", b[1]], ["0–20%", b[0]]], { onClick: (l) => UI.gotoFindings({ conf: { "80–100%": 0.8, "60–80%": 0.6, "40–60%": 0.4, "20–40%": 0.2, "0–20%": 0 }[l] }) })));
      const byCat = new Map();
      for (const f of h) { const c = UI.cat(f.category).label; const r = byCat.get(c) || byCat.set(c, { n: 0, s: 0 }).get(c); r.n++; r.s += f.confidence || 0; }
      page.append(K.panel("Average by kind of finding", "layers", K.bars([...byCat.entries()].map(([c, r]) => [c, Math.round((r.s / r.n) * 100), r.n + " findings"]).sort((a, b) => b[1] - a[1]).slice(0, 16), { max: 100 })));
      page.append(K.findings("Least certain findings", "uncertain", [...h].sort((a, b) => a.confidence - b.confidence).slice(0, 15), { empty: "None." }));
      page.append(K.hint("Method: each collector assigns a confidence from how direct its evidence is (an official record 0.9+, a content marker 0.9, a bare status code 0.6, a name match 0.3–0.5). Corroboration across collectors is shown separately under Shared identifiers."));
    } };

  const timestamps = { id: "timestamps", label: "Collection timestamps", icon: "clock", group: "Evidence", count: (M) => M.modStats().length,
    render(page, M) {
      const rows = M.modStats().filter((s) => s.first);
      const r = M.rep;
      page.append(K.stats([[r.started_at ? K.dt(Date.parse(r.started_at)) : K.dt(M.S.created), "started"], [r.finished_at ? K.dt(Date.parse(r.finished_at)) : "running…", "finished"], [rows.length, "collectors with results"]]));
      page.append(K.panel("When each collector produced its evidence", "clock", K.opts({ count: rows.length }), K.table([{ label: "Collector", get: (s) => s.name }, { label: "First result", get: (s) => K.dt(s.first) }, { label: "Last result", get: (s) => K.dt(s.last) }, { label: "Run time", cls: "r", get: (s) => (s.ms ? UI.dur(s.ms) : "") }], rows)));
      page.append(K.hint("All times are UTC. Evidence reflects the public state of each source at that moment; pages and profiles change."));
    } };

  const provenance = { id: "provenance", label: "Data provenance", icon: "network", group: "Evidence", count: (M) => M.hits.length,
    render(page, M) {
      const first = M.hits.filter((f) => !f.via && f.category !== "toolrun"), via = new Map();
      for (const f of M.hits) if (f.via) (via.get(f.via) || via.set(f.via, []).get(f.via)).push(f);
      page.append(K.stats([[first.length, "from what you supplied", { lead: true }], [via.size, "leads followed"], [[...via.values()].reduce((a, l) => a + l.length, 0), "findings from followed leads"]]));
      page.append(K.hint("Provenance is the path a fact took to get here: directly from your input, or by following a lead the first pass discovered (an e-mail, a domain, a handle)."));
      const rows = [...via.entries()].map(([v, l]) => ({ v, n: l.length, mods: [...new Set(l.map((f) => f.module))], ex: l.slice(0, 3).map((f) => f.source) })).sort((a, b) => b.n - a.n);
      page.append(K.panel("Leads that were followed", "network", K.opts({ count: rows.length }), K.table([{ label: "Followed", get: (r) => r.v }, { label: "Findings", cls: "r", get: (r) => r.n }, { label: "Collectors", get: (r) => r.mods.join(", ") }, { label: "Examples", get: (r) => r.ex.join(" · ") }], rows, { onRow: (r) => UI.gotoFindings({ q: r.v }), empty: "No lead was followed (the first pass found nothing new to follow, or following was turned off)." })));
      page.append(K.findings("From your input", "user", first.slice(0, 200), { empty: "None.", page: 30 }));
    } };

  // ------------------------------------------------------------- hashes
  const hashCache = new Map();
  const canon = (f) => JSON.stringify({ module: f.module, source: f.source, category: f.category || "", url: f.url || "", status: f.status, confidence: f.confidence, details: Object.fromEntries(Object.entries(f.details || {}).sort(([a], [b]) => a.localeCompare(b))), entities: (f.entities || []).map((e) => e.type + ":" + e.value).sort() });
  async function sha(text) {
    const d = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(text));
    return [...new Uint8Array(d)].map((b) => b.toString(16).padStart(2, "0")).join("");
  }
  async function hashAll(list, onProgress) {
    let i = 0;
    for (const f of list) {
      if (!hashCache.has(f)) hashCache.set(f, await sha(canon(f)));
      if (++i % 100 === 0 && onProgress) onProgress(i);
    }
  }
  const hashes = { id: "hashes", label: "Evidence hashes", icon: "hash", group: "Evidence", always: true, count: (M) => M.hits.length,
    render(page, M) {
      if (!window.crypto || !crypto.subtle) { page.append(K.empty("alert", "Hashing is not available", "The browser only offers SHA-256 on secure origins (localhost or HTTPS).", "open SUDOSOC ARGUS through http://127.0.0.1 or an HTTPS reverse proxy.")); return; }
      const list = M.hits.filter((f) => f.category !== "toolrun");
      const body = el("div", {});
      const status = el("p", { class: "hint", text: "Computing SHA-256 of each finding…" });
      page.append(K.panel("Integrity digest", "shield", M.rep.integrity ? K.kv({ algorithm: M.rep.integrity.algorithm, "covers": M.rep.integrity.findings + " findings, " + M.rep.integrity.entities + " entities", digest: M.rep.integrity.digest }) : K.hint("The digest of the whole report is recorded when the investigation finishes.")));
      const dl = UI.btn("Download hash manifest (CSV)", { cls: "sm", icon: "download" });
      page.append(K.panel("SHA-256 of each finding", "hash", K.opts({ count: list.length }), K.hint("Each hash is computed in your browser over the finding's canonical form (collector, source, URL, status, confidence, details, entities). Re-computing it later and comparing proves the evidence was not altered."), status, el("div", { class: "rep-actions" }, dl), body));
      const photos = M.photos.filter((p) => p.sha256);
      if (photos.length) page.append(K.panel("Pictures", "image", K.opts({ count: photos.length }), K.table([{ label: "SHA-256", get: (p) => p.sha256 }, { label: "Source", get: (p) => ((p.sources || [])[0] || "").replace(/^[a-z]+: /, "") }, { label: "Size", cls: "r", get: (p) => (p.bytes ? UI.n(p.bytes) + " B" : "") }], photos)));
      (async () => {
        await hashAll(list.slice(0, 3000), (n) => { status.textContent = `Computed ${UI.n(n)} of ${UI.n(Math.min(list.length, 3000))}…`; });
        status.textContent = list.length > 3000 ? "Showing and hashing the first 3,000 findings; the manifest hashes all of them." : "All findings hashed.";
        body.replaceChildren(K.table([{ label: "SHA-256 (first 16)", get: (f) => (hashCache.get(f) || "").slice(0, 16) + "…" }, { label: "Collector", get: (f) => f.module }, { label: "Source", get: (f) => f.source }, { label: "Copy", get: (f) => { const b = UI.copyBtn(hashCache.get(f) || "", "hash"); return b; } }], list.slice(0, 3000), { page: 50 }));
      })();
      dl.addEventListener("click", async () => {
        await hashAll(list);
        const rows = ["sha256,collector,source,url,status,confidence"].concat(list.map((f) => [hashCache.get(f), f.module, f.source, f.url || "", f.status, f.confidence].map((x) => '"' + String(x).replace(/"/g, '""') + '"').join(",")));
        UI.download("evidence-hashes-" + UI.S.id + ".csv", rows.join("\n"), "text/csv");
      });
    } };
  UI.evidenceHash = async (f) => { if (!hashCache.has(f)) hashCache.set(f, await sha(canon(f))); return hashCache.get(f); };

  // ------------------------------------------------------------ coverage
  const coverage = { id: "coverage", label: "Collection coverage", icon: "shield", group: "Coverage & integrity", legacy: "coverage", always: true, count: (M) => M.failures.length };

  const sources = { id: "sources", label: "Source coverage", icon: "server", group: "Coverage & integrity", always: true,
    render(page, M) {
      const ran = new Map(M.modules.map((m) => [m.name, m]));
      const skipped = new Map((M.rep.skipped || []).map((s) => { const [n, ...r] = s.split(": "); return [n, r.join(": ")]; }));
      const names = (UI.info ? UI.info.modules.map((m) => m.name) : []).sort((a, b) => UI.modRank(a) - UI.modRank(b));
      const rows = names.map((n) => { const r = ran.get(n), live = M.S.mod.get(n); return { n, needs: UI.MODULE_NEEDS[n] || "any input", status: r || live ? "ran" : skipped.has(n) ? skipped.get(n) : M.done ? "not run" : "waiting", checked: r ? r.checked : live ? live.n : 0, found: r ? r.found : live ? live.hit : 0, err: r ? r.errors : live ? live.err : 0 }; });
      const ranN = rows.filter((r) => r.status === "ran").length;
      const health = UI.btn("Check which public services are reachable now", { cls: "sm", icon: "pulse" });
      health.addEventListener("click", () => UI.showHealth());
      page.append(K.stats([[ranN + " / " + names.length, "collectors ran", { lead: true }], [rows.filter((r) => r.err > 0).length, "with errors", { warn: rows.some((r) => r.err > 0) }], [rows.filter((r) => r.status !== "ran").length, "did not run"]]));
      page.append(K.panel("Every collector", "server", K.opts({ count: rows.length }), K.table([{ label: "Collector", get: (r) => r.n }, { label: "Needs", get: (r) => r.needs }, { label: "State", get: (r) => r.status }, { label: "Checks", cls: "r", get: (r) => UI.n(r.checked) }, { label: "Hits", cls: "r", get: (r) => UI.n(r.found) }, { label: "Errors", cls: "r", get: (r) => (r.err ? UI.n(r.err) : "") }], rows), health));
    } };

  const missing = { id: "missing", label: "Missing data", icon: "uncertain", group: "Coverage & integrity", always: true,
    render(page, M) {
      const t = M.target, p = M.primary;
      const items = [
        ["A confirmed name", M.entsOf("name").length > 0, "give a name, or an account that states one"],
        ["An e-mail address", M.entsOf("email").length > 0, "give an e-mail, or a handle whose profile or commits publish one"],
        ["A phone number", M.entsOf("phone").length > 0 || !!t.phone, "give a phone number: owners are rarely published"],
        ["Usernames", M.entsOf("username").length > 0 || !!t.username, "give a handle or alias"],
        ["Accounts on platforms", M.accounts.length > 0, "give a username"],
        ["An employer or organisation", !!(p && (p.employers || []).length) || M.entsOf("organisation").length > 0, "give a company, or a profile that states one"],
        ["A location", !!(M.geo && (M.geo.points || []).length), "give a place; accounts, IPs and phones also hint at one"],
        ["A website or domain", M.entsOf("domain").length > 0, "give a domain, or an e-mail on your own domain"],
        ["A profile picture", M.photos.length > 0, "find accounts that publish one"],
        ["Dated events", M.timeline.length > 0, "give a username, domain or e-mail with dated facts"],
        ["A breach check", M.hits.some((f) => f.module === "breach") || M.entsOf("email").length === 0, "give an e-mail address"],
        ["Infrastructure data", M.hits.some((f) => ["domain", "ip", "asn"].includes(f.module)), "give a domain, an IP or an ASN"],
      ];
      const missingN = items.filter((i) => !i[1]).length;
      page.append(K.stats([[items.length - missingN + " / " + items.length, "attributes covered", { lead: true }], [missingN, "missing", { warn: missingN > 0 }], [(M.rep.skipped || []).length, "collectors skipped"], [M.leads.length, "manual leads"]]));
      page.append(K.panel("What the investigation did not find", "uncertain", K.table([{ label: "Attribute", get: (i) => i[0] }, { label: "Found", get: (i) => (i[1] ? "yes" : "no") }, { label: "How to fill it", get: (i) => (i[1] ? "" : i[2]) }], items)));
      if ((M.rep.skipped || []).length) page.append(K.panel("Collectors that did not run", "server", K.table([{ label: "Collector", get: (s) => s.split(": ")[0] }, { label: "Reason", get: (s) => s.split(": ").slice(1).join(": ") }], M.rep.skipped)));
      if (M.leads.length) {
        const b = UI.btn("Open the manual leads", { cls: "sm", icon: "external" });
        b.addEventListener("click", () => UI.go("evidence", "coverage"));
        page.append(K.panel("Sources that need a login, CAPTCHA or payment", "external", K.hint(M.leads.length + " manual leads are listed under Collection coverage."), b));
      }
    } };

  const validation = { id: "validation", label: "Data validation", icon: "check", group: "Coverage & integrity", always: true, count: (M) => invalid(M).length,
    render(page, M) {
      const rows = M.ents.map((e) => ({ e, v: UI.validate(e) })).filter((r) => r.v.applicable);
      const bad = rows.filter((r) => !r.v.ok);
      const byType = {};
      for (const r of rows) { const t = byType[r.e.type] || (byType[r.e.type] = [0, 0]); t[0]++; if (r.v.ok) t[1]++; }
      page.append(K.stats([[rows.length, "values checked", { lead: true }], [rows.length - bad.length, "well-formed"], [bad.length, "malformed or private", { warn: bad.length > 0 }]]));
      page.append(K.hint("Syntax checks only: nothing is contacted. A well-formed value is not proof the mailbox, number or host exists."));
      page.append(K.panel("By type", "layers", K.bars(Object.entries(byType).map(([t, [n, ok]]) => [t, n, ok + " valid"]))));
      page.append(K.panel("Values to review", "alert", K.opts({ count: bad.length }), K.table([{ label: "Type", get: (r) => r.e.type }, { label: "Value", get: (r) => r.e.value }, { label: "Problem", get: (r) => r.v.notes.join(" · ") }, { label: "From", get: (r) => r.e.source || "" }], bad, { onRow: (r) => UI.showEntity(r.e), empty: "Every value is well-formed." })));
    } };
  const invalid = (M) => M.ents.map((e) => ({ e, v: UI.validate(e) })).filter((r) => r.v.applicable && !r.v.ok);

  const conflicts = Object.assign({}, UI.sectionDefs.conflicting, { id: "conflicts", label: "Conflicting information", group: "Coverage & integrity" });

  const duplicates = { id: "duplicates", label: "Duplicate detection", icon: "layers", group: "Coverage & integrity", always: true, count: (M) => M.duplicates().length,
    render(page, M) {
      const d = M.duplicates();
      page.append(K.stats([[d.length, "duplicate groups", { lead: true }], [d.reduce((a, g) => a + g.items.length - 1, 0), "redundant items"], [d.filter((g) => g.kind === "identical picture").length, "identical pictures"]]));
      page.append(K.hint("The same page reached through two collectors, a finding emitted twice, or one picture on several accounts. Duplicates inflate counts; picture duplicates are also evidence two accounts are linked."));
      page.append(K.panel("Duplicates", "layers", K.opts({ count: d.length }), K.table([{ label: "Kind", get: (g) => g.kind }, { label: "Item", get: (g) => g.key }, { label: "Copies", cls: "r", get: (g) => g.items.length }, { label: "Reported by", get: (g) => [...new Set(g.items.map((x) => x.module ? x.module + ": " + x.source : (x.sources || [])[0] || ""))].slice(0, 4).join(" · ") }], d, { empty: "No duplicates detected." })));
    } };

  const score = { id: "score", label: "Confidence score", icon: "pulse", group: "Coverage & integrity", always: true,
    render(page, M) {
      const checked = M.modules.reduce((a, m) => a + (m.checked || 0), 0), errs = M.modules.reduce((a, m) => a + (m.errors || 0), 0);
      const completion = checked ? 1 - errs / checked : 0;
      const h = M.hits.filter((f) => f.category !== "toolrun");
      const quality = h.length ? h.reduce((a, f) => a + f.confidence * (f.status === "found" ? 1 : 0.6), 0) / h.length : 0;
      const corro = M.ents.length ? Math.min(1, M.shared.length / Math.max(5, M.ents.length * 0.25)) : 0;
      const verif = M.primary ? M.primary.score : null;
      const conf = M.conflicts().length;
      const parts = [["Collection completeness", completion, 0.30, `${UI.n(checked - errs)} of ${UI.n(checked)} checks completed`], ["Evidence quality", quality, 0.30, "average confidence of the findings"], ["Corroboration", corro, 0.25, `${UI.n(M.shared.length)} values reported by 2+ sources`]];
      if (verif != null) parts.push(["Identity verification", verif, 0.15, "match score of the best person"]);
      const w = parts.reduce((a, p) => a + p[2], 0);
      let total = (parts.reduce((a, p) => a + p[1] * p[2], 0) / w) * 100;
      const penalty = Math.min(25, conf * 5);
      total = Math.max(0, total - penalty);
      page.append(K.panel("Overall confidence in this investigation", "meter", K.gauge(total, M.done ? "score out of 100" : "score so far (still collecting)", "A transparent blend, not a probability: it says how complete, how well-evidenced and how corroborated the data is, minus " + penalty + " points for " + conf + " unresolved conflicts.")));
      page.append(K.panel("How the score is built", "list", K.table([{ label: "Component", get: (p) => p[0] }, { label: "Value", get: (p) => K.conf(p[1]) }, { label: "Weight", cls: "r", get: (p) => Math.round((p[2] / w) * 100) + "%" }, { label: "Based on", get: (p) => p[3] }], parts)));
    } };

  UI.defineTab({ id: "evidence", label: "Evidence", icon: "shield", count: (M) => M.failures.length, warn: true, sections: [registry, library, reliability, confidence, timestamps, provenance, hashes, coverage, sources, missing, validation, conflicts, duplicates, score] });
})();
