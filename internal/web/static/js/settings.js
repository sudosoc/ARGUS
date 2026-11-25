"use strict";

// API settings: keys for the model providers and for tools that need one. The server keeps keys in an
// encrypted file and never sends one back: this page only ever sees whether a key exists and its last four characters.
(function () {
  const { el } = UI;

  const LLM_LABELS = { anthropic: "Claude (Anthropic)", openai: "OpenAI", gemini: "Google Gemini", local: "Local model" };

  UI.showSettings = async function showSettings() {
    const body = UI.openDrawer("Settings", "language, keys for model providers and for tools that need one");
    let data;
    try { data = await UI.api("/api/settings"); } catch (e) { body.replaceChildren(UI.emptyState("alert", "Could not load the settings", e.message)); return; }
    draw(body, data);
  };

  function draw(body, data) {
    const rerender = (d) => draw(body, d);
    body.replaceChildren(el("p", { class: "hint", text: "Keys are stored in an encrypted file in your config directory and are never shown again after saving; environment variables are read too. A tool that needs a key is skipped when its provider is off or has no key." }));

    // ---- interface language (stored in this browser; the page reloads)
    const langSel = el("select", { "aria-label": "Interface language", translate: "no" },
      el("option", { value: "en", text: "English", selected: UI.lang === "en" ? "" : null }), el("option", { value: "ar", text: "العربية", selected: UI.lang === "ar" ? "" : null }));
    langSel.addEventListener("change", () => UI.setLang(langSel.value));
    body.append(el("section", { class: "panel" }, el("h3", { class: "ph" }, UI.icon("globe"), "Language"),
      el("div", { class: "sel-row" }, langSel),
      el("p", { class: "hint", text: "Choose the language of the interface. Arabic switches the layout to right-to-left. Reports and exported files stay in English." })));

    // ---- model selector
    const llm = data.providers.filter((p) => p.kind === "llm");
    const prov = el("select", { "aria-label": "Model provider" }, ...llm.map((p) => el("option", { value: p.id, text: LLM_LABELS[p.id] || p.label, selected: p.id === (data.llm.provider || "anthropic") ? "" : null })));
    const model = el("input", { type: "text", class: "input", placeholder: "exact model name", value: data.llm.model || "", "aria-label": "Exact model name", autocomplete: "off", spellcheck: "false" });
    const defaultFor = () => (llm.find((p) => p.id === prov.value) || {}).default_model || "";
    prov.addEventListener("change", () => { model.placeholder = defaultFor() || "exact model name"; });
    model.placeholder = defaultFor() || "exact model name";
    const saveSel = UI.btn("Use this model", { cls: "sm", icon: "check" });
    saveSel.addEventListener("click", async () => {
      try { rerender(await UI.api("/api/settings/llm", { method: "PUT", body: JSON.stringify({ provider: prov.value, model: model.value.trim() || defaultFor() }) })); UI.toast("Model saved"); } catch (e) { UI.toast(e.message); }
    });
    body.append(el("section", { class: "panel" }, el("h3", { class: "ph" }, UI.icon("tool"), "Model"),
      el("div", { class: "sel-row" }, prov, model, saveSel),
      el("p", { class: "hint", text: "Pick the provider, then type the exact model name (for example the one shown in the provider's console). ARGUS stores your choice and tests it below; it sends nothing to a model unless a feature that uses one is turned on." })));

    for (const p of data.providers) body.append(card(p, rerender));
  }

  function card(p, rerender) {
    const key = el("input", { type: "password", class: "input", placeholder: p.has_key ? p.key_hint + (p.key_source === "env" ? "  (from environment)" : "  (saved)") : p.needs_key ? "paste the API key" : "no key needed", autocomplete: "new-password", spellcheck: "false", "aria-label": p.label + " API key", disabled: p.needs_key ? null : "" });
    const on = el("input", { type: "checkbox", checked: p.enabled ? "" : null, id: "en_" + p.id });
    const model = p.kind === "llm" ? el("input", { type: "text", class: "input", value: p.model || "", placeholder: p.default_model || "model name", "aria-label": p.label + " model" }) : null;
    const base = p.id === "local" ? el("input", { type: "text", class: "input", value: p.base_url || "", placeholder: p.default_base, "aria-label": "Local server address" }) : null;
    const status = el("span", { class: "hint", role: "status" });

    const payload = (withKey) => {
      const o = { enabled: on.checked };
      if (withKey && key.value.trim()) o.key = key.value.trim();
      if (model) o.model = model.value.trim();
      if (base) o.base_url = base.value.trim();
      return o;
    };
    const save = UI.btn("Save", { cls: "sm", icon: "check" });
    save.addEventListener("click", async () => {
      try { rerender(await UI.api("/api/settings/" + p.id, { method: "PUT", body: JSON.stringify(payload(true)) })); UI.toast(p.label + " saved"); } catch (e) { status.textContent = e.message; }
    });
    const test = UI.btn("Test connection", { cls: "sm", icon: "pulse" });
    test.addEventListener("click", async () => {
      test.disabled = true; status.className = "hint"; status.textContent = "Testing…";
      try {
        const r = await UI.api("/api/settings/" + p.id + "/test", { method: "POST", body: JSON.stringify(payload(true)) });
        status.className = r.ok ? "hint ok" : "hint bad"; status.textContent = (r.ok ? "✔ " : "✖ ") + r.message;
      } catch (e) { status.className = "hint bad"; status.textContent = "✖ " + e.message; }
      test.disabled = false;
    });
    const clear = p.has_key && p.key_source === "stored" ? UI.btn("Remove key", { cls: "sm", icon: "trash" }) : null;
    if (clear) clear.addEventListener("click", async () => { try { rerender(await UI.api("/api/settings/" + p.id, { method: "PUT", body: JSON.stringify({ key: "", enabled: false }) })); UI.toast("Key removed"); } catch (e) { status.textContent = e.message; } });

    return el("section", { class: "panel" },
      el("h3", { class: "ph" }, UI.icon("key"), p.label, p.has_key ? el("span", { class: "tag" }, UI.icon("check"), "key set") : el("span", { class: "tag" }, "no key"), p.kind === "tool" ? el("span", { class: "tag" }, "for tools") : null),
      el("div", { class: "sel-row" }, key, el("label", { class: "check sm", for: "en_" + p.id }, on, el("span", { text: "Enabled" }))),
      model || base ? el("div", { class: "sel-row" }, base, model) : null,
      el("div", { class: "rep-actions" }, save, test, clear, status),
      p.help ? el("p", { class: "hint", text: p.help + (p.env_var ? " · environment variable: " + p.env_var : "") }) : null);
  }

  document.addEventListener("DOMContentLoaded", () => { const b = document.getElementById("settingsBtn"); if (b) b.addEventListener("click", () => UI.showSettings()); });
})();
