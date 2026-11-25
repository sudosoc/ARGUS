"use strict";

// Copyright (C) 2026 Seif, SUDOSOC. SPDX-License-Identifier: AGPL-3.0-only

// Runs synchronously from <head> so the saved theme and density are applied
// before first paint (no flash of the wrong theme). The keys share the
// "argus_" prefix used by the rest of the front end (see core.js).
(function () {
  const root = document.documentElement;
  const get = (k, d) => { try { return localStorage.getItem("argus_" + k) || d; } catch (e) { return d; } };
  root.setAttribute("data-theme", get("theme", "dark") === "light" ? "light" : "dark");
  root.setAttribute("data-density", get("density", "compact") === "comfortable" ? "comfortable" : "compact");
  if (get("panel", "open") === "closed") root.setAttribute("data-panel", "closed");
})();
