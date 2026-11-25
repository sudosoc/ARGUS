"use strict";

// Graph: the force-directed map and tree stay; this adds analysis on top of the
// same entity graph: shared identifiers, account relationships, infrastructure
// links, shortest paths between entities and clusters.
(function () {
  const { el } = UI;
  const K = UI.kit;
  const lc = (s) => String(s || "").toLowerCase();

  const connections = { id: "connections", label: "Entity connections", icon: "graph", legacy: "graph", always: true, count: (M) => M.ents.length };
  const tree = { id: "tree", label: "Intelligence tree", icon: "tree", legacy: "tree", always: true };

  const shared = { id: "shared", label: "Shared identifiers", icon: "hash", count: (M) => M.shared.length,
    render(page, M) {
      const rows = M.shared;
      page.append(K.stats([[rows.length, "values seen in 2+ sources", { lead: true }], [rows.filter((r) => r.modules.length >= 2).length, "confirmed by 2+ collectors"], [rows.filter((r) => r.e.type === "email").length, "shared e-mails"], [rows.filter((r) => r.e.type === "domain" || r.e.type === "url").length, "shared sites"]]));
      page.append(K.hint("A value that two independent sources report is stronger than one reported once, and it is what links accounts, sites and people together."));
      page.append(K.panel("Seen by more than one source", "hash", K.opts({ count: rows.length }), K.table([
        { label: "Type", get: (r) => r.e.type }, { label: "Value", get: (r) => r.e.value }, { label: "Sources", cls: "r", get: (r) => r.sources.length }, { label: "Collectors", get: (r) => r.modules.join(", ") }, { label: "Seen in", get: (r) => r.sources.slice(0, 4).join(" · ") + (r.sources.length > 4 ? " …" : "") }],
        rows, { onRow: (r) => UI.showEntity(r.e), empty: "No value was reported by more than one source." })));
    } };

  const accountRel = { id: "accounts", label: "Account relationships", icon: "user", count: (M) => acctGroups(M).length,
    render(page, M) {
      const rows = acctGroups(M);
      page.append(K.stats([[rows.length, "links between accounts", { lead: true }]]));
      page.append(K.panel("Accounts tied together by a shared value", "link", K.opts({ count: rows.length }), K.hint("Two accounts that publish the same e-mail, website, picture or distinctive name are very likely the same person."),
        K.table([{ label: "Shared", get: (r) => r.type + ": " + r.value }, { label: "Accounts", get: (r) => r.accts.map((f) => f.source).join(" · ") }, { label: "Count", cls: "r", get: (r) => r.accts.length }], rows, { empty: "No two accounts share a value." })));
    } };
  function acctGroups(M) {
    const acc = new Set(M.accounts);
    return M.shared.map((s) => ({ type: s.e.type, value: s.e.value, accts: s.findings.filter((f) => acc.has(f)).filter((f, i, a) => a.findIndex((x) => x.source === f.source) === i) })).filter((r) => r.accts.length >= 2);
  }

  const infra = { id: "infra", label: "Infrastructure links", icon: "server", count: (M) => infraRows(M).length,
    render(page, M) {
      const rows = infraRows(M);
      page.append(K.stats([[rows.length, "links", { lead: true }], [M.entsOf("domain").length, "domains"], [M.entsOf("ip").length, "IPs"], [M.entsOf("asn").length, "ASNs"]]));
      page.append(K.panel("How hosts, networks and names connect", "server", K.opts({ count: rows.length }), K.table([{ label: "From", get: (r) => r.a }, { label: "To", get: (r) => r.b }, { label: "Established by", get: (r) => r.via }], rows, { empty: "No infrastructure links. Give a domain or IP." })));
    } };
  function infraRows(M) {
    const out = [], seen = new Set(), INF = new Set(["domain", "ip", "asn"]);
    for (const f of M.hits) {
      if (!["domain", "ip", "asn", "lookalike", "syswrap", "webhistory"].includes(f.module)) continue;
      const es = (f.entities || []).filter((e) => INF.has(e.type));
      if (es.length < 2 || es.length > 60) continue;
      const anchor = es.find((e) => e.type === "domain") || es[0];
      for (const e of es) {
        if (e === anchor || lc(e.value) === lc(anchor.value)) continue;
        const k = lc(anchor.value) + ">" + lc(e.value);
        if (seen.has(k)) continue;
        seen.add(k); out.push({ a: `${anchor.type} ${anchor.value}`, b: `${e.type} ${e.value}`, via: f.source });
      }
    }
    return out;
  }

  // ---- shortest path between two entities
  let pathFrom = "subject", pathTo = "";
  const paths = { id: "paths", label: "Connection paths", icon: "link", always: true,
    render(page, M) {
      const G = M.graph();
      const opts = [["subject", "★ " + (UI.targetLabel(M.target) || "the subject")]];
      const ents = [...M.ents].sort((a, b) => (b.strong ? 1 : 0) - (a.strong ? 1 : 0) || (b.findings || []).length - (a.findings || []).length).slice(0, 400);
      for (const e of ents) opts.push([e.type + ":" + lc(e.value), `${e.type} · ${UI.clip(e.value, 60)}`]);
      if (!pathTo || !G.nodes.has(pathTo)) pathTo = opts[Math.min(1, opts.length - 1)][0];
      if (!G.nodes.has(pathFrom)) pathFrom = "subject";
      const mk = (name, val, onch) => { const s = el("select", { "aria-label": name }); for (const [v, l] of opts) s.append(el("option", { value: v, text: l, selected: v === val ? "" : null })); s.addEventListener("change", () => onch(s.value)); return s; };
      const result = el("div", {});
      const draw = () => {
        result.replaceChildren();
        const path = M.path(pathFrom, pathTo);
        if (pathFrom === pathTo) { result.append(K.hint("Pick two different entities.")); return; }
        if (!path) { result.append(K.empty("link", "No connection found", "These two entities are not linked through any finding.", "run again with more collectors, or follow more leads (-max-pivots).")); return; }
        const chain = el("div", { class: "pathbox" });
        let hops = 0;
        for (let i = 0; i < path.length; i++) {
          const n = G.nodes.get(path[i].key);
          if (n.kind === "finding") continue;
          if (n.kind === "subject") chain.append(el("span", { class: "chip strong" }, UI.icon("eye"), el("span", { class: "chip-v", text: n.label })));
          else chain.append(UI.entityChip({ type: n.type, value: n.value, strong: n.strong }));
          const next = path.slice(i + 1).find((p) => G.nodes.get(p.key).kind !== "finding");
          if (next) { const via = path[i + 1] && G.nodes.get(path[i + 1].key).kind === "finding" ? G.nodes.get(path[i + 1].key).f.source : path[i + 1].via; chain.append(el("span", { class: "arrow", text: "—" + (via ? " " + UI.clip(via, 42) + " " : "") + "→" })); hops++; }
        }
        result.append(K.stats([[hops, "hops", { lead: true }]]), el("div", { style: "margin-top:12px" }, chain));
        if (hops <= 1) result.append(K.hint("Directly connected: one finding mentions both."));
      };
      page.append(K.panel("How are two things connected?", "link", K.hint("Pick two entities (or the subject). SUDOSOC ARGUS finds the shortest chain of findings that links them: the evidence trail from one to the other."),
        el("div", { class: "sel-row" }, el("label", {}, "From", mk("From", pathFrom, (v) => { pathFrom = v; draw(); })), el("label", {}, "To", mk("To", pathTo, (v) => { pathTo = v; draw(); }))), result));
      draw();
      // what is reachable from the subject
      const dist = reach(G);
      page.append(K.panel("Distance from the subject", "graph", K.bars([["directly supplied or derived (1 hop)", dist[1] || 0], ["found by following a lead (2 hops)", dist[2] || 0], ["3 hops", dist[3] || 0], ["4+ hops", dist.more || 0]].filter((r) => r[1] > 0)), K.hint("Entities far from the subject are weaker leads: each hop is a place the trail could have drifted to someone else.")));
    } };
  function reach(G) {
    const d = new Map([["subject", 0]]), q = ["subject"], out = {};
    while (q.length) { const k = q.shift(); for (const { to } of G.adj.get(k)) if (!d.has(to)) { d.set(to, d.get(k) + 1); q.push(to); } }
    for (const [k, v] of d) { if (G.nodes.get(k).kind !== "entity") continue; const h = Math.ceil(v / 2); if (h >= 4) out.more = (out.more || 0) + 1; else out[h] = (out[h] || 0) + 1; }
    return out;
  }

  const clusters = { id: "clusters", label: "Cluster detection", icon: "layers", count: (M) => M.clusters().length,
    render(page, M) {
      const cl = M.clusters();
      page.append(K.stats([[cl.length, "clusters", { lead: true }], [cl.length ? cl[0].length : 0, "entities in the largest"], [M.ents.length - cl.reduce((a, c) => a + c.length, 0), "isolated entities"]]));
      page.append(K.hint("A cluster is a group of entities that findings connect to each other, leaving out the subject. Separate clusters can be separate people, separate organisations or just unconnected leads."));
      cl.slice(0, 20).forEach((c, i) => {
        const types = {};
        for (const n of c) types[n.type] = (types[n.type] || 0) + 1;
        const head = [...c].sort((a, b) => (b.strong ? 1 : 0) - (a.strong ? 1 : 0))[0];
        page.append(K.panel(`Cluster ${i + 1} · around ${UI.clip(head.value, 50)}`, "layers", K.opts({ count: c.length, sub: Object.entries(types).map(([t, n]) => `${n} ${t}`).join(" · ") }), K.entityChips(c.map((n) => ({ type: n.type, value: n.value, strong: n.strong })), 40)));
      });
      if (!cl.length) page.append(K.empty("layers", "No clusters", "No two entities are connected by a finding.", "give more identifiers so findings overlap."));
    } };

  UI.defineTab({ id: "graph", label: "Graph", icon: "graph", count: (M) => M.ents.length, sections: [connections, tree, shared, accountRel, infra, paths, clusters] });
})();
