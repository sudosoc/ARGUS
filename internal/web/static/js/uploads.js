"use strict";

// File inputs: pictures, documents and videos are sent to the local server, analysed there and
// never leave the machine. Each upload shows what was found and offers its leads (e-mails, phones,
// domains) for the form.
(function () {
  const { el } = UI;
  const FORM_FIELD = { emails: "email", phones: "phone", domains: "domain" };
  const bytes = (n) => (n > 1048576 ? (n / 1048576).toFixed(1) + " MB" : Math.max(1, Math.round(n / 1024)) + " KB");
  let list = null;

  UI.clearUploads = function () { UI.uploads = []; if (list) list.replaceChildren(); };

  UI.buildUploader = function buildUploader() {
    const input = el("input", { type: "file", multiple: "", id: "f_files", class: "visually-hidden", accept: "image/*,video/*,.pdf,.doc,.docx,.xls,.xlsx,.ppt,.pptx,.odt,.ods,.odp,.txt,.md,.csv" });
    const drop = el("label", { class: "dropzone", for: "f_files", tabindex: "0" }, UI.icon("download", "big"),
      el("span", { class: "dz-t", text: "Drop files here or click to choose" }), el("span", { class: "dz-s", text: "pictures · PDF · Word · Excel · PowerPoint · OpenDocument · video · text" }));
    list = el("div", { class: "upl-list" });
    const take = (files) => { for (const f of files) send(f); };
    input.addEventListener("change", () => { take(input.files); input.value = ""; });
    for (const ev of ["dragenter", "dragover"]) drop.addEventListener(ev, (e) => { e.preventDefault(); drop.classList.add("over"); });
    for (const ev of ["dragleave", "drop"]) drop.addEventListener(ev, (e) => { e.preventDefault(); drop.classList.remove("over"); });
    drop.addEventListener("drop", (e) => take(e.dataTransfer.files));
    drop.addEventListener("keydown", (e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); input.click(); } });
    return el("div", { class: "field" }, el("label", {}, el("span", { text: "Pictures, documents, videos" }), UI.infoBtn("files")), input, drop, list);
  };

  async function send(file) {
    const row = el("div", { class: "upl busy" }, el("span", { class: "upl-n", text: file.name }), el("span", { class: "hint", text: "analysing… " + bytes(file.size) }));
    list.append(row);
    try {
      const r = await fetch("/api/uploads", { method: "POST", credentials: "same-origin", headers: { "X-Requested-With": "argus", "X-Filename": encodeURIComponent(file.name), "Content-Type": "application/octet-stream" }, body: file });
      const a = await r.json().catch(() => ({}));
      if (!r.ok) throw new Error(a.error || "HTTP " + r.status);
      UI.uploads.push(a);
      row.replaceWith(card(a));
    } catch (e) {
      row.className = "upl bad";
      row.replaceChildren(el("span", { class: "upl-n", text: file.name }), el("span", { class: "hint", text: "✖ " + e.message }), remove(() => row.remove()));
    }
  }

  const remove = (fn) => { const b = UI.btn("", { cls: "icon-btn xs", icon: "x", title: "Remove" }); b.addEventListener("click", fn); return b; };

  function card(a) {
    const root = el("div", { class: "upl" });
    const facts = [];
    if (a.width) facts.push(`${a.width}×${a.height}`);
    if (a.gps) facts.push("GPS " + a.gps);
    if (a.duration_seconds) facts.push(Math.round(a.duration_seconds) + " s");
    if (a.frames && a.frames.length) facts.push(a.frames.length + " frames");
    if (a.text_chars) facts.push(UI.n(a.text_chars) + " chars of text");
    const author = a.meta && (a.meta.author || a["person_name"]);
    if (author) facts.push("author: " + author);
    root.append(el("div", { class: "upl-h" }, UI.icon(a.kind === "image" ? "image" : a.kind === "video" ? "camera" : "file"), el("span", { class: "upl-n", text: a.name }), el("span", { class: "tag", text: a.kind }), el("span", { class: "hint", text: bytes(a.size) }),
      remove(() => { UI.uploads = UI.uploads.filter((u) => u.id !== a.id); root.remove(); })));
    if (facts.length) root.append(el("div", { class: "hint", text: facts.join(" · ") }));
    for (const n of a.notes || []) root.append(el("div", { class: "hint warn", text: "⚠ " + n }));
    const leads = el("div", { class: "chips" });
    for (const [k, field] of Object.entries(FORM_FIELD)) {
      for (const v of (a[k] || []).slice(0, 6)) {
        const b = el("button", { type: "button", class: "chip", title: "Put this in the " + field + " field" }, UI.icon(field === "email" ? "mail" : field === "phone" ? "phone" : "globe"), el("span", { class: "chip-v", text: v }));
        b.addEventListener("click", () => { const inp = document.getElementById("f_" + field); if (inp && !inp.value) { inp.value = v; inp.dispatchEvent(new Event("input", { bubbles: true })); } else UI.toast("The " + field + " field is already filled"); });
        leads.append(b);
      }
    }
    if (a.person_name) leads.append(el("button", { type: "button", class: "chip", onclick: null, title: "Put this in the full name field", "data-fill": "full_name" }, UI.icon("user"), el("span", { class: "chip-v", text: a.person_name })));
    leads.querySelectorAll("[data-fill]").forEach((b) => b.addEventListener("click", () => { const inp = document.getElementById("f_full_name"); if (inp && !inp.value) { inp.value = a.person_name; inp.dispatchEvent(new Event("input", { bubbles: true })); } }));
    if (leads.children.length) root.append(el("div", { class: "hint", text: "Found inside (click to use):" }), leads);
    root.append(el("div", { class: "hint", text: "Everything found in it, including its e-mails, phones and domains, is followed up when you start the investigation." }));
    return root;
  }
})();
