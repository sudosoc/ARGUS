"use strict";
// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

// Posts: what the accounts the operator opted in have published in public (Mastodon, Bluesky, GitHub, Reddit),
// copied without signing in, and what it adds up to. The analysis is computed on the server (internal/insights)
// and never lists religion, health, sexual orientation or political affiliation.
(function () {
  const { el } = UI;
  const K = UI.kit;
  const INS = (M) => (M.rep && M.rep.insights && M.rep.insights.content) || null;
  const pct = (x) => Math.round((x || 0) * 100);

  // The copied items: from the finished report, or live from the findings that carry them.
  function postsOf(M) {
    if (M.rep && Array.isArray(M.rep.posts)) return M.rep.posts;
    const out = [];
    for (const f of (M.S && M.S.findings) || []) if (Array.isArray(f.posts)) out.push(...f.posts);
    return out.sort((a, b) => String(b.time).localeCompare(String(a.time)));
  }
  function sourcesOf(M) {
    if (M.rep && Array.isArray(M.rep.post_sources)) return M.rep.post_sources;
    const out = [];
    for (const f of (M.S && M.S.findings) || []) if (f.post_source) out.push(f.post_source);
    return out;
  }

  const askFirst = (page) => page.append(K.empty("layers", "No posts were copied", "Posts are copied only from the accounts you choose. In “What will be collected”, tick one of the optional collectors (Mastodon, Bluesky, GitHub or Reddit). Only the username you typed is used; nothing is signed in and nothing private is read.", "start an investigation and tick an optional posts collector."));
  const tag = (text, cls) => el("span", { class: "tag" + (cls ? " " + cls : ""), text });

  // ----------------------------------------------------------------- feed
  const f = { q: "", platform: "", kind: "", shown: 40 };
  function card(p) {
    const href = p.url ? UI.safeHref(p.url) : null;
    const head = el("div", { class: "post-h" }, tag(p.platform), tag(p.kind, p.kind === "boost" ? "weak" : ""), el("span", { class: "muted num", text: K.date(p.time) }),
      el("span", { class: "muted", text: "@" + p.handle }), el("span", { class: "grow" }),
      href ? el("a", { class: "btn xs", href, target: "_blank", rel: "noopener noreferrer", title: "Open the original" }, UI.icon("external")) : null);
    const body = [];
    if (p.kind === "boost" && p.author) body.push(el("div", { class: "muted", text: "shared a post by " + p.author }));
    if (p.reply_to) body.push(el("div", { class: "muted", text: "replying to " + p.reply_to }));
    if (p.text) body.push(el("p", { class: "post-t", text: p.text }));
    const foot = [];
    for (const t of p.tags || []) foot.push(tag("#" + t.replace(/^#/, "")));
    if (p.likes || p.reposts || p.replies) foot.push(el("span", { class: "muted num", text: [p.likes ? p.likes + " likes" : "", p.reposts ? p.reposts + " shares" : "", p.replies ? p.replies + " replies" : ""].filter(Boolean).join(" · ") }));
    const pics = (p.media || []).slice(0, 4).map((u) => { const im = UI.queueImg(el("img", { class: "post-img", alt: "" }), u); im.addEventListener("error", () => im.remove()); return im; });
    return el("article", { class: "post" }, head, ...body, pics.length ? el("div", { class: "post-pics" }, ...pics) : null, foot.length ? el("div", { class: "post-f" }, ...foot) : null);
  }
  const feed = {
    id: "feed", label: "Posts", icon: "layers", group: "Copied", always: true,
    count: (M) => postsOf(M).length,
    render(page, M) {
      const all = postsOf(M);
      if (!all.length) { askFirst(page); return; }
      const platforms = [...new Set(all.map((p) => p.platform))];
      const kinds = [...new Set(all.map((p) => p.kind))];
      const q = el("input", { type: "search", placeholder: "Search the copied text…", "aria-label": "Search the copied text", value: f.q });
      const sp = el("select", { "aria-label": "Place" }, el("option", { value: "", text: "All places" }), ...platforms.map((x) => el("option", { value: x, text: x, selected: x === f.platform ? "" : null })));
      const sk = el("select", { "aria-label": "Kind" }, el("option", { value: "", text: "All kinds" }), ...kinds.map((x) => el("option", { value: x, text: x, selected: x === f.kind ? "" : null })));
      const list = el("div", { class: "postlist" });
      const more = el("button", { type: "button", class: "btn", text: "Show more" });
      const count = el("span", { class: "muted num" });
      const draw = () => {
        const needle = f.q.trim().toLowerCase();
        const rows = all.filter((p) => (!f.platform || p.platform === f.platform) && (!f.kind || p.kind === f.kind) &&
          (!needle || (p.text || "").toLowerCase().includes(needle) || (p.tags || []).some((t) => t.toLowerCase().includes(needle))));
        list.replaceChildren(...rows.slice(0, f.shown).map(card));
        count.textContent = UI.n(rows.length) + " of " + UI.n(all.length);
        more.hidden = rows.length <= f.shown;
        if (!rows.length) list.append(el("p", { class: "hint", text: "Nothing matches." }));
      };
      q.addEventListener("input", () => { f.q = q.value; f.shown = 40; draw(); });
      sp.addEventListener("change", () => { f.platform = sp.value; f.shown = 40; draw(); });
      sk.addEventListener("change", () => { f.kind = sk.value; f.shown = 40; draw(); });
      more.addEventListener("click", () => { f.shown += 40; draw(); });
      page.append(K.hint("Copied as it was public, without signing in. It is evidence kept with this case; it is not analysed for religion, health, sexual orientation or political affiliation."));
      page.append(el("div", { class: "ftool" }, q, sp, sk, count), list, more);
      draw();
    },
  };

  // ------------------------------------------------------------- activity
  const wait = (page) => page.append(K.empty("clock", "The analysis is written when the investigation finishes", "Rhythm, topics, people and writing habits are computed from everything that was copied.", "wait for the run to finish, or open a saved case."));
  const activity = {
    id: "activity", label: "Activity", icon: "clock", group: "Analysis", always: true, count: (M) => (INS(M) ? INS(M).authored : 0),
    render(page, M) {
      const c = INS(M);
      if (!c) { if (postsOf(M).length) wait(page); else askFirst(page); return; }
      if (!c.total) { page.append(K.empty("layers", "Nothing to analyse", "No public items could be copied; the Sources section says why.")); return; }
      page.append(K.stats([[c.total, "items copied", { lead: true }], [c.authored, "written by the person"], [c.first ? c.first.slice(0, 4) : "—", "oldest"], [c.last ? K.date(c.last) : "—", "newest"]]));
      if ((c.kinds || []).length) page.append(K.panel("What the items are", "list", K.bars(c.kinds.map((k) => [k.label, k.n]))));
      if ((c.per_month || []).length) page.append(K.panel("Items per month", "clock", K.bars(c.per_month.map((m) => [m.month, m.n]))));
      if (c.authored >= 5) {
        page.append(K.panel("Hour of the day (UTC)", "clock", K.bars(c.hours.map((n, h) => [String(h).padStart(2, "0") + ":00", n]).filter((r) => r[1] > 0)), K.hint("Peak at " + String(c.peak_hour).padStart(2, "0") + ":00 UTC. Add the person's UTC offset to read local time.")));
        const days = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"];
        page.append(K.panel("Day of the week", "clock", K.bars(c.weekdays.map((n, d) => [days[d], n]))));
      }
      for (const n of c.notes || []) page.append(K.hint(n));
    },
  };

  // --------------------------------------------------------------- topics
  const topics = {
    id: "topics", label: "Topics & people", icon: "hash", group: "Analysis", always: true, count: (M) => (INS(M) ? (INS(M).topics || []).length + (INS(M).interactions || []).length : 0),
    render(page, M) {
      const c = INS(M);
      if (!c) { if (postsOf(M).length) wait(page); else askFirst(page); return; }
      if ((c.topics || []).length) page.append(K.panel("Most used tags and subjects", "hash", K.bars(c.topics.map((t) => ["#" + t.label.replace(/^#/, ""), t.n]))));
      if ((c.interactions || []).length) page.append(K.panel("Who they talk to and share", "network", K.table([{ label: "Account", get: (x) => x.label }, { label: "Score", cls: "r", get: (x) => x.n }, { label: "How", get: (x) => x.note }], c.interactions), K.hint("A reply counts 3, a share 2, a mention 1. Public interaction is a lead about contact, not proof of a relationship.")));
      if ((c.domains || []).length) page.append(K.panel("Sites they link to", "link", K.bars(c.domains.map((d) => [d.label, d.n]))));
      if ((c.languages || []).length) page.append(K.panel("Languages written", "list", K.bars(c.languages.map((l) => [l.label, pct(l.share)]), { max: 100 })));
      if (c.style) {
        const s = c.style;
        page.append(K.panel("Writing habits", "code", K.kv({ "Average length": s.avg_words + " words", "With a link": pct(s.link_share) + "%", "With a picture": pct(s.media_share) + "%", "Asks a question": pct(s.question_share) + "%", "Uses emoji": pct(s.emoji_share) + "%", "Mostly capitals": pct(s.shout_share) + "%", "Answers someone": pct(s.reply_share) + "%" }),
          K.hint("Habits are counted, not interpreted. They are useful to compare two accounts, not to describe a person.")));
      }
      if (!(c.topics || []).length && !(c.interactions || []).length && !(c.domains || []).length) page.append(K.empty("hash", "No topics or people to show", "Too few tagged items, or the ones found were left out by the protected-characteristics rule."));
      for (const n of c.notes || []) page.append(K.hint(n));
    },
  };

  // -------------------------------------------------------------- sources
  const sources = {
    id: "sources", label: "Sources", icon: "tool", group: "Copied", always: true, count: (M) => sourcesOf(M).length,
    render(page, M) {
      const s = sourcesOf(M);
      if (!s.length) { askFirst(page); return; }
      page.append(K.hint("What was copied from each account, and why a place may be partial or empty: a limit, a refusal, or an account that asked not to be shown to people without an account."));
      page.append(K.panel("Where the items came from", "tool", K.table([{ label: "Place", get: (x) => x.platform }, { label: "Account", get: (x) => x.handle }, { label: "Items", cls: "r", get: (x) => x.count },
        { label: "From", get: (x) => K.date(x.oldest) }, { label: "To", get: (x) => K.date(x.newest) }, { label: "Note", get: (x) => x.note || "" }], s,
        { onRow: (x) => { const h = x.url && UI.safeHref(x.url); if (h) window.open(h, "_blank", "noopener,noreferrer"); } })));
    },
  };

  UI.defineTab({ id: "posts", label: "Posts", icon: "layers", count: (M) => postsOf(M).length, sections: [feed, sources, activity, topics] });
})();
