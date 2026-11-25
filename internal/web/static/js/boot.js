"use strict";

// Copyright (C) 2026 Seif, SUDOSOC. SPDX-License-Identifier: AGPL-3.0-only

// Start-up: load server info, build the form, restore the last case.
(async function boot() {
  const { $ } = UI;
  // On narrow screens the case panel is an overlay: start closed unless the analyst opened it.
  if (window.innerWidth < 1100) UI.showPanel(false);
  UI.panelTab("new");
  try {
    const info = await UI.api("/api/info");
    UI.info = info;
    $("ver").textContent = "v" + info.version;
    $("siteCount").textContent = UI.n(info.social_sites) + " social sites · " + info.modules.length + " collectors";
    UI.buildForm(info);
    UI.setConn(true);
  } catch (e) {
    UI.setConn(false);
    UI.showFormError("Could not reach the server: " + e.message);
  }
  UI.bindForm();
  UI.refreshHistory();
  setInterval(UI.refreshHistory, 4000);
  const tab = UI.session.get("tab");
  if (tab && UI.TABS.some((t) => t.id === tab)) UI.tab = tab;
  const last = UI.session.get("last");
  if (last) {
    try { await UI.api("/api/scans/" + last); UI.openScan(last); } catch (e) { /* the server was restarted: that scan no longer exists */ }
  }
})();
