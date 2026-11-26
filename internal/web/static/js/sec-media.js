"use strict";

// Copyright (C) 2026 Seif, SUDOSOC. SPDX-License-Identifier: AGPL-3.0-only

// Media intelligence: pictures, their metadata, similarity and where to search them.
(function () {
  const { el } = UI;
  const K = UI.kit;

  // Hamming distance of two 64-bit hex dHashes.
  const hamming = (a, b) => {
    if (!a || !b || a.length !== b.length) return 64;
    let x = BigInt("0x" + a) ^ BigInt("0x" + b), n = 0;
    while (x) { n += Number(x & 1n); x >>= 1n; }
    return n;
  };

  const collection = { id: "collection", label: "Image collection", icon: "image", legacy: "photos", always: true, count: (M) => M.photos.length };

  const exif = { id: "exif", label: "Metadata / EXIF", icon: "camera",
    count: (M) => M.photos.filter(hasMeta).length + M.pick({ cats: ["document-metadata", "exif"] }).length,
    render(page, M) {
      const rows = M.photos.filter(hasMeta);
      const docs = M.pick({ cats: ["document-metadata", "exif"] });
      page.append(K.stats([[rows.length, "pictures with metadata", { lead: true }], [rows.filter((p) => p.gps).length, "with GPS"], [rows.filter((p) => p.camera).length, "with device info"], [docs.length, "documents with metadata"]]));
      page.append(K.hint("Most platforms strip EXIF when a picture is uploaded. Personal sites, CV photos and shared documents often keep it: device, software, author, serial numbers and GPS."));
      page.append(K.panel("Picture metadata", "camera", K.opts({ count: rows.length }), K.table([
        { label: "Picture", get: (p) => (p.sources || [])[0] ? String(p.sources[0]).replace(/^[a-z]+: /, "") : UI.host(p.url) }, { label: "Device", get: (p) => p.camera || "" }, { label: "Taken", get: (p) => p.taken || "" },
        { label: "GPS", get: (p) => p.gps || "" }, { label: "Software / author", get: (p) => [p.meta && p.meta.Software, p.meta && (p.meta.Artist || p.meta.Creator)].filter(Boolean).join(" · ") },
        { label: "Serial", get: (p) => (p.meta && (p.meta.SerialNumber || p.meta.BodySerialNumber)) || "" }, { label: "Text in picture (OCR)", get: (p) => (p.meta && p.meta.OCRText ? UI.clip(p.meta.OCRText.replace(/\s+/g, " "), 90) : "") }], rows, { onRow: (p) => UI.showPhoto(p), empty: "No picture carried metadata." })));
      page.append(K.findings("Documents and files with metadata", "file", docs, { empty: "No document metadata.", showModule: false }));
    } };
  const hasMeta = (p) => p.gps || p.taken || p.camera || (p.meta && Object.keys(p.meta).length);

  const similarity = { id: "similarity", label: "Image similarity", icon: "layers", count: (M) => M.photos.filter((p) => p.dhash).length > 1 ? sim(M).groups.length + sim(M).near.length : 0,
    render(page, M) {
      const { groups, near } = sim(M);
      page.append(K.stats([[M.photos.length, "pictures", { lead: true }], [groups.length, "identical groups"], [near.length, "near-duplicates"]]));
      page.append(K.hint("Pictures are compared by a perceptual hash: the same photo at another size or compression matches. A photo shared by several accounts is strong evidence they belong together."));
      for (const g of groups) page.append(K.panel("Same picture · G" + g.id, "image", K.opts({ count: g.items.length }), K.table([{ label: "Source", get: (p) => (p.sources || []).join(" · ").replace(/[a-z]+: /g, "") }, { label: "Size", get: (p) => (p.width ? p.width + "×" + p.height : "") }], g.items, { onRow: (p) => UI.showPhoto(p) })));
      if (near.length) page.append(K.panel("Very similar (not identical)", "uncertain", K.opts({ count: near.length }), K.table([{ label: "Picture A", get: (n) => (n.a.sources || [])[0] || UI.host(n.a.url) }, { label: "Picture B", get: (n) => (n.b.sources || [])[0] || UI.host(n.b.url) }, { label: "Distance", cls: "r", get: (n) => n.d + " / 64" }], near)));
      if (!groups.length && !near.length) page.append(K.empty("image", "No similar pictures", "Pictures were compared and none repeats.", "more accounts with profile pictures raise the chance of a match."));
    } };
  function sim(M) {
    const byGroup = new Map();
    for (const p of M.photos) if (p.group) (byGroup.get(p.group) || byGroup.set(p.group, []).get(p.group)).push(p);
    const groups = [...byGroup.entries()].map(([id, items]) => ({ id, items }));
    const near = [], list = M.photos.filter((p) => p.dhash);
    for (let i = 0; i < list.length && i < 300; i++) for (let j = i + 1; j < list.length && j < 300; j++) {
      const a = list[i], b = list[j];
      if (a.group && a.group === b.group) continue;
      const d = hamming(a.dhash, b.dhash);
      if (d <= 10) near.push({ a, b, d });
    }
    return { groups, near: near.sort((x, y) => x.d - y.d) };
  }

  const reverse = { id: "reverse", label: "Reverse image search", icon: "search", count: (M) => M.photos.length, always: true,
    render(page, M) {
      if (!M.photos.length) { page.append(K.empty("search", "No pictures to search", "Reverse image search needs a picture.", "find accounts with profile pictures, or paste a picture URL as a profile link.")); return; }
      page.append(K.hint("SUDOSOC ARGUS does not upload anything. Each link opens a search engine with the picture's public address; a person decides whether a match is the same person or place."));
      const engines = [["Google Lens", (u) => "https://lens.google.com/uploadbyurl?url=" + encodeURIComponent(u)], ["Yandex", (u) => "https://yandex.com/images/search?rpt=imageview&url=" + encodeURIComponent(u)], ["TinEye", (u) => "https://tineye.com/search?url=" + encodeURIComponent(u)], ["SauceNAO", (u) => "https://saucenao.com/search.php?url=" + encodeURIComponent(u)], ["Bing", (u) => "https://www.bing.com/images/search?view=detailv2&iss=sbi&q=imgurl:" + encodeURIComponent(u)]];
      for (const p of M.photos.slice(0, 40)) {
        const row = el("div", { class: "revrow" }, UI.queueImg(el("img", { class: "revthumb", alt: "" }), p.url),
          el("div", { class: "revmain" }, el("div", { class: "fr-title", text: ((p.sources || [])[0] || UI.host(p.url)).replace(/^[a-z]+: /, "") }), K.links(engines.map(([n, f]) => ({ label: n, url: f(p.url) })))));
        page.append(el("div", { class: "panel" }, row));
      }
      const manual = M.leads.filter((l) => l.group === "image");
      if (manual.length) page.append(K.panel("Face and image search engines", "external", K.hint("Upload a picture yourself. Treat a face match as a lead: verify by hand."), K.links(manual.map((l) => ({ label: l.label, url: l.url, why: l.why })))));
    } };

  const timeline = { id: "timeline", label: "Media timeline", icon: "clock", count: (M) => M.photos.filter((p) => p.taken).length,
    render(page, M) {
      const rows = M.photos.filter((p) => p.taken).map((p) => ({ p, t: String(p.taken).replace(/^(\d{4}):(\d\d):(\d\d)/, "$1-$2-$3") })).sort((a, b) => a.t.localeCompare(b.t));
      page.append(K.stats([[rows.length, "dated pictures", { lead: true }], [rows.length ? rows[0].t.slice(0, 10) : "—", "earliest"], [rows.length ? rows[rows.length - 1].t.slice(0, 10) : "—", "latest"]]));
      page.append(K.panel("When pictures were taken", "clock", K.table([{ label: "Taken", get: (r) => r.t }, { label: "Device", get: (r) => r.p.camera || "" }, { label: "Source", get: (r) => ((r.p.sources || [])[0] || "").replace(/^[a-z]+: /, "") }, { label: "GPS", get: (r) => r.p.gps || "" }], rows, { onRow: (r) => UI.showPhoto(r.p), empty: "No picture carried a capture date." })));
    } };

  const uploaded = K.section("files", "Uploaded files", "file", (M) => M.hits.filter((f) => f.module === "uploads"), { showModule: false,
    intro: "Files you supplied, analysed on this machine: hidden metadata, extracted text, video frames and the leads found inside. Open one to read everything, including the full text.",
    empty: "No file was supplied.", unlock: "drop a picture, document or video into Files when you start an investigation, or use -file on the command line." });
  UI.defineTab({ id: "media", label: "Media", icon: "image", count: (M) => M.photos.length, sections: [collection, uploaded, exif, similarity, reverse, timeline] });
})();
