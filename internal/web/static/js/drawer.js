"use strict";

// The details drawer (findings, entities, pictures, saved cases, search…),
// analyst notes and the small renderers they share.
(function () {
  const { $, el } = UI;

  // ------------------------------------------------------------------ drawer
  let opener = null;
  UI.openDrawer = function openDrawer(title, kicker) {
    if ($("drawer").hidden) opener = document.activeElement;
    $("drawerTitle").textContent = title;
    $("drawerKicker").textContent = kicker || "";
    const body = $("drawerBody");
    body.replaceChildren();
    body.scrollTop = 0;
    $("drawer").hidden = false;
    document.body.classList.add("has-drawer");
    return body;
  };
  UI.closeDrawer = function closeDrawer() {
    if ($("drawer").hidden) return;
    $("drawer").hidden = true;
    document.body.classList.remove("has-drawer");
    if (opener && document.contains(opener)) opener.focus();
    opener = null;
  };
  $("drawerClose").addEventListener("click", UI.closeDrawer);
  document.addEventListener("keydown", (e) => { if (e.key === "Escape" && !$("drawer").hidden && !document.querySelector(".overlay, .popover")) UI.closeDrawer(); });

  const section = (title, ...kids) => el("section", { class: "dsec" }, el("h4", { text: title }), ...kids);
  UI.section = section;

  /** Key/value list. URLs become safe links; long values wrap. */
  UI.kvTable = function kvTable(obj) {
    const dl = el("dl", { class: "kv" });
    for (const [k, v] of Object.entries(obj || {})) {
      dl.append(el("dt", { text: k }));
      const s = String(v);
      const href = /^https?:\/\/\S+$/.test(s) ? UI.safeHref(s) : null;
      dl.append(el("dd", {}, href ? el("a", { href, target: "_blank", rel: "noopener noreferrer", text: s }) : document.createTextNode(s)));
    }
    return dl;
  };

  UI.entityChip = function entityChip(e) {
    const b = el("button", { type: "button", class: "chip " + (e.strong ? "strong" : "weak"), title: (e.strong ? "Strong lead" : "Weak lead") + (e.source ? " · " + e.source : "") },
      UI.icon(UI.type(e.type).icon), el("span", { class: "k", text: e.type }), el("span", { class: "chip-v", text: e.value }));
    b.addEventListener("click", () => UI.showEntity(e));
    return b;
  };

  /** The value an analyst would copy from a finding: its URL, else its title. */
  UI.findingValue = (f) => f.url || f.source;
  UI.findingKey = (f) => [f.module, f.source, f.url || ""].join("|");

  // ---------------------------------------------------------------- finding
  UI.showFinding = function showFinding(f) {
    const body = UI.openDrawer(f.source, f.module + " · " + UI.cat(f.category).label);
    const p = UI.pct(f.confidence);
    body.append(el("div", { class: "dhead" }, UI.statusBadge(f.status), UI.meter(p, true),
      el("span", { class: "grow" }), UI.copyBtn(() => UI.findingValue(f), "value"), UI.copyBtn(() => JSON.stringify(f, null, 2), "JSON")));
    const meta = { collector: f.module, category: f.category || "—", status: f.status, confidence: p + "%" };
    if (f.via) meta["found by following"] = f.via;
    if (f.timestamp) meta.collected = f.timestamp;
    if (f.url) meta.url = f.url;
    body.append(section("Overview", UI.kvTable(meta)));
    if (f.details && Object.keys(f.details).length) body.append(section("Evidence", UI.kvTable(f.details)));
    if (f.entities && f.entities.length) {
      const chips = el("div", { class: "chips" });
      for (const e of f.entities) chips.append(UI.entityChip(e));
      body.append(section("Leads discovered here", chips));
    }
    body.append(section("Your notes", UI.notes.box("finding:" + UI.findingKey(f), "Why this matters, what you verified…")));
    body.append(section("Raw record", el("details", { class: "raw" }, el("summary", { text: "Show JSON" }), el("pre", { class: "mono", text: JSON.stringify(f, null, 2) }))));
    $("drawerClose").focus();
  };

  // ----------------------------------------------------------------- entity
  UI.showEntity = function showEntity(e) {
    const body = UI.openDrawer(e.value, UI.type(e.type).label.replace(/s$/, ""));
    const key = e.type + ":" + e.value.toLowerCase();
    const full = (UI.S && UI.S.entities.get(key)) || e;
    body.append(el("div", { class: "dhead" }, el("span", { class: "tag " + (full.strong ? "strong" : "") }, UI.icon(full.strong ? "check" : "uncertain"), full.strong ? "Strong lead" : "Weak lead"),
      el("span", { class: "grow" }), UI.copyBtn(e.value, "value")));
    body.append(section("Entity", UI.kvTable({
      type: e.type, value: e.value,
      attribution: full.strong ? "strong lead (followed automatically)" : "weak lead (listed, not followed)",
      "first seen in": full.source || "—",
    })));
    const finder = (list) => {
      const box = el("div", { class: "chips" });
      for (const f of list) {
        const b = el("button", { type: "button", class: "chip", title: f.source }, el("span", { class: "k", text: f.module }), el("span", { class: "chip-v", text: UI.clip(f.source, 60) }));
        b.addEventListener("click", () => UI.showFinding(f));
        box.append(b);
      }
      return box;
    };
    const where = full.findings || [];
    body.append(section(`Mentioned in ${UI.n(where.length)} finding${where.length === 1 ? "" : "s"}`, finder(where.slice(0, 40)),
      where.length > 40 ? el("p", { class: "hint", text: `${where.length - 40} more: use the findings table search.` }) : null));
    const via = UI.S ? UI.S.findings.filter((f) => (f.via || "").toLowerCase() === key).slice(0, 40) : [];
    if (via.length) body.append(section("Found by investigating it", finder(via)));
    $("drawerClose").focus();
    if (UI.graphNeighbours) UI.graphNeighbours(e, body);
  };

  // ------------------------------------------------------------------ photo
  UI.showPhoto = function showPhoto(p) {
    const body = UI.openDrawer("Photo", p.group ? "Same picture · group " + p.group : "");
    const img = el("img", { class: "bigimg", alt: "profile picture", src: UI.imgSrc(p.url) });
    img.addEventListener("error", () => img.replaceWith(el("div", { class: "empty-line", text: "The image could not be loaded (the host may have removed it or blocked the download)." })));
    body.append(img);
    const meta = { "found at": (p.sources || []).join("  ·  ") || "—" };
    if (p.width) meta.size = p.width + " × " + p.height + " px, " + Math.round((p.bytes || 0) / 1024) + " KB";
    if (p.group) meta["same picture"] = "group " + p.group + ": identical or near-identical picture used by several sources";
    if (p.camera) meta.camera = p.camera;
    if (p.taken) meta["taken"] = p.taken;
    if (p.gps) meta.gps = p.gps;
    for (const [k, v] of Object.entries(p.meta || {})) meta[k] = v; // exiftool: Software, Artist, SerialNumber, Make, Model…
    if (p.sha256) meta.sha256 = p.sha256;
    if (p.dhash) meta["perceptual hash"] = p.dhash;
    body.append(section("Details", UI.kvTable(meta)));
    const u = encodeURIComponent(p.url);
    const links = [
      ["Google Lens", "https://lens.google.com/uploadbyurl?url=" + u],
      ["Yandex Images", "https://yandex.com/images/search?rpt=imageview&url=" + u],
      ["TinEye", "https://tineye.com/search?url=" + u],
      ["Bing Visual Search", "https://www.bing.com/images/search?view=detailv2&iss=sbi&q=imgurl:" + u],
    ];
    const row = el("div", { class: "chips" });
    for (const [label, href] of links) row.append(el("a", { class: "btn sm", href, target: "_blank", rel: "noopener noreferrer" }, UI.icon("external"), el("span", { text: label })));
    body.append(section("Reverse image search (opens an external site)", row,
      el("p", { class: "hint", text: "Sends this picture's address to that service. Reverse search finds other places the same photo appears: the strongest way to tie accounts together or spot a stolen photo." })));
    const href = UI.safeHref(p.url);
    if (href) body.append(section("Original", el("a", { href, target: "_blank", rel: "noopener noreferrer", class: "wrap", text: p.url })));
    $("drawerClose").focus();
  };

  // ------------------------------------------------------------------ notes
  // Analyst notes are saved with the case on the server (and appear in reports).
  const saveTimers = new Map();
  UI.notes = {
    get(key) { const r = UI.S && UI.S.report; return (r && r.notes && r.notes[key]) || ""; },
    ready() { return !!(UI.S && UI.S.report && UI.S.state !== "running"); },
    set(key, text, status) {
      const S = UI.S;
      if (!S || !S.report) return;
      S.report.notes = S.report.notes || {};
      if (text.trim()) S.report.notes[key] = text; else delete S.report.notes[key];
      clearTimeout(saveTimers.get(key));
      const id = S.id;
      saveTimers.set(key, setTimeout(async () => {
        try { await UI.api(`/api/scans/${id}/notes`, { method: "POST", body: JSON.stringify({ key, text }) }); if (status) status.textContent = "Saved with the case"; }
        catch (e) { if (status) status.textContent = "Could not save: " + e.message; }
      }, 700));
    },
    box(key, placeholder) {
      const wrap = el("div", { class: "notewrap" });
      if (!UI.notes.ready()) {
        wrap.append(el("div", { class: "hint", text: "Notes can be added once the scan has finished." }));
        return wrap;
      }
      const ta = el("textarea", { class: "notebox", maxlength: "4000", "aria-label": "Analyst note", placeholder: placeholder || "Your note (saved with the case and included in the reports)…" });
      ta.value = UI.notes.get(key);
      const st = el("span", { class: "hint" });
      ta.addEventListener("input", () => { st.textContent = "Saving…"; UI.notes.set(key, ta.value, st); });
      wrap.append(ta, st);
      return wrap;
    },
  };
})();
