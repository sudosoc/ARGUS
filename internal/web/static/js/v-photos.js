"use strict";

// Photos tab: a gallery of pictures found on profiles. Pictures are fetched by
// the server (never by the browser) and compared; a thick red frame plus a
// "same picture" label marks one used by several sources.
(function () {
  const { $, el } = UI;
  let filter = "all";

  const isCand = (p) => (p.sources || []).length > 0 && (p.sources || []).every((x) => /^images:/.test(x));

  function tile(p) {
    const ph = el("div", { class: "ph" }, el("span", { class: "ph-load", text: "loading…" }));
    const img = el("img", { src: UI.imgSrc(p.url), alt: "picture" });
    img.addEventListener("load", () => ph.replaceChildren(img));
    img.addEventListener("error", () => ph.replaceChildren(el("span", { class: "ph-load", text: "unavailable" })));
    const cap = el("div", { class: "cap" }, el("div", { class: "src", text: UI.clip((p.sources && p.sources[0]) || "picture", 70).replace(/^[a-z]+: /, "") }));
    const tags = el("div", { class: "tags" });
    if (p.group) tags.append(el("span", { class: "tag same" }, UI.icon("link"), "same picture · G" + p.group));
    if (isCand(p)) tags.append(el("span", { class: "tag cand" }, UI.icon("uncertain"), "unverified"));
    if (p.gps) tags.append(el("span", { class: "tag" }, UI.icon("pin"), "GPS"));
    if (tags.children.length) cap.append(tags);
    if ((p.sources || []).length > 1) cap.append(el("div", { class: "m", text: "also: " + UI.clip(p.sources.slice(1).join(" · ").replace(/[a-z]+: /g, ""), 80) }));
    const meta = [p.width ? p.width + "×" + p.height : "", p.camera || ""].filter(Boolean).join(" · ");
    if (meta) cap.append(el("div", { class: "m num", text: meta }));
    const t = el("button", { type: "button", class: "tile" + (p.group ? " grouped" : "") + (isCand(p) ? " candidate" : "") }, ph, cap);
    t.addEventListener("click", () => UI.showPhoto(p));
    return t;
  }

  function photos() {
    const S = UI.S, root = $("tab-photos");
    root.replaceChildren();
    const all = UI.photoList();
    if (!all.length) {
      root.append(UI.emptyState("image", S.state === "running" ? "No pictures yet…" : "No profile pictures were found",
        S.state === "running" ? "Pictures appear as profiles are read." : "Platforms that need a login (Instagram, Facebook, X, LinkedIn) are listed under Coverage → manual leads."));
      return;
    }
    const sets = {
      all: all, grouped: all.filter((p) => p.group), verified: all.filter((p) => !isCand(p)), cand: all.filter(isCand),
    };
    const seg = el("div", { class: "segmented", role: "group", "aria-label": "Show pictures" });
    for (const [k, label] of [["all", "All"], ["grouped", "Same picture"], ["verified", "From accounts"], ["cand", "Unverified"]]) {
      const b = el("button", { type: "button", class: "seg" + (filter === k ? " on" : ""), "aria-pressed": String(filter === k) }, label, el("span", { class: "count", text: String(sets[k].length) }));
      b.addEventListener("click", () => { filter = k; photos(); });
      seg.append(b);
    }
    root.append(el("div", { class: "ptool" }, seg, el("span", { class: "hint", text: "Downloaded by SUDOSOC ARGUS, not your browser. A default avatar, logo or stock image can repeat too: open a picture to confirm it shows a person." })));

    const list = sets[filter];
    if (!list.length) { root.append(UI.emptyState("image", "Nothing in this view", "Choose another filter.")); return; }
    if (filter === "grouped") {
      const byG = new Map();
      for (const p of list) (byG.get(p.group) || byG.set(p.group, []).get(p.group)).push(p);
      for (const [g, ps] of byG) {
        root.append(el("h3", { class: "sec-title" }, UI.icon("link"), `Same picture · group ${g}`, el("span", { class: "count", text: String(ps.length) })));
        const grid = el("div", { class: "gallery" });
        ps.forEach((p) => grid.append(tile(p)));
        root.append(grid);
      }
      return;
    }
    const grid = el("div", { class: "gallery" });
    list.forEach((p) => grid.append(tile(p)));
    root.append(grid);
  }

  UI.views.photos = photos;
})();
