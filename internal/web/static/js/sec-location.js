"use strict";

// Copyright (C) 2026 Seif, SUDOSOC. SPDX-License-Identifier: AGPL-3.0-only

// Location: the map view stays; this adds EXIF locations and a correlation view.
(function () {
  const { el } = UI;
  const K = UI.kit;

  const geo = { id: "geo", label: "Geolocation & map", icon: "map", legacy: "location", always: true, count: (M) => (M.geo ? (M.geo.points || []).length : 0) };

  const osm = (lat, lon) => `https://www.openstreetmap.org/?mlat=${lat}&mlon=${lon}#map=14/${lat}/${lon}`;
  const exifRows = (M) => {
    const rows = [];
    for (const p of M.photos) if (p.gps) { const [lat, lon] = p.gps.split(",").map(Number); rows.push({ what: "picture", where: (p.sources || [])[0] || UI.host(p.url), lat, lon, taken: p.taken || "", camera: p.camera || "" }); }
    for (const f of M.pick({ cats: ["exif", "document-metadata"] })) { const g = (f.details || {}).gps || (f.details || {}).GPS; if (g && /-?\d+\.\d+\s*,\s*-?\d+\.\d+/.test(g)) { const [lat, lon] = g.split(",").map(Number); rows.push({ what: "document", where: f.source, lat, lon, taken: (f.details || {}).created || "", camera: (f.details || {}).software || "" }); } }
    return rows;
  };

  const exif = { id: "exif", label: "EXIF location data", icon: "camera", count: (M) => exifRows(M).length,
    render(page, M) {
      const rows = exifRows(M);
      page.append(K.stats([[rows.length, "GPS positions in files", { lead: true }], [new Set(rows.map((r) => r.lat.toFixed(2) + r.lon.toFixed(2))).size, "distinct places"]]));
      page.append(K.hint("These are coordinates embedded in a file by the device that made it. They are the most precise location evidence there is, and the easiest to fake or to have been copied from elsewhere."));
      page.append(K.panel("Positions found inside files", "pin", K.opts({ count: rows.length }), K.table([{ label: "From", get: (r) => r.what + " · " + r.where }, { label: "Latitude", get: (r) => r.lat.toFixed(5) }, { label: "Longitude", get: (r) => r.lon.toFixed(5) }, { label: "Taken", get: (r) => r.taken }, { label: "Device / software", get: (r) => r.camera },
        { label: "Map", get: (r) => el("a", { href: osm(r.lat, r.lon), target: "_blank", rel: "noopener noreferrer", text: "OpenStreetMap" }) }], rows, { empty: "No file carried GPS coordinates." })));
    } };

  const correlation = { id: "correlation", label: "Geographic correlation", icon: "layers", count: (M) => (M.geo ? (M.geo.clusters || []).length : 0),
    render(page, M) {
      const g = M.geo;
      if (!g) { page.append(K.empty("map", "No location analysis", "Location is assessed when the investigation finishes.", "give a place, or let accounts, IPs and phone numbers provide location clues.")); return; }
      const cl = g.clusters || [], pts = g.points || [];
      page.append(K.stats([[g.consensus || "—", "most likely place", { lead: true }], [cl.length, "clusters of evidence"], [pts.length, "evidence points"], [g.given_fit === "differs" ? "differs" : g.given_fit === "agrees" ? "agrees" : "—", "vs the place you gave", { warn: g.given_fit === "differs" }]]));
      if (g.conflict) page.append(el("div", { class: "callout warn" }, UI.icon("alert"), el("span", {}, el("b", { text: "Evidence disagrees. " }), g.conflict)));
      page.append(K.panel("Where the evidence agrees", "layers", K.opts({ count: cl.length }), K.table([{ label: "Place", get: (c) => c.label }, { label: "Country", get: (c) => c.country || "" }, { label: "Support", get: (c) => K.conf(c.score) }, { label: "Evidence kinds", get: (c) => (c.kinds || []).join(" · ") }, { label: "Points", cls: "r", get: (c) => (c.points || []).length }], cl, { empty: "No clusters." })));
      // which kinds of evidence say which country
      const kinds = {}, countries = new Set();
      for (const p of pts) { const c = p.country || p.place || p.label; countries.add(c); (kinds[p.kind] = kinds[p.kind] || new Set()).add(c); }
      page.append(K.panel("What each kind of evidence says", "list", K.table([{ label: "Evidence", get: (r) => r.k }, { label: "Points to", get: (r) => [...r.v].slice(0, 6).join(" · ") }, { label: "Agrees with consensus", get: (r) => (g.consensus && [...r.v].some((x) => g.consensus.toLowerCase().includes(String(x).toLowerCase().split(",")[0])) ? "yes" : "no") }],
        Object.entries(kinds).map(([k, v]) => ({ k, v })))));
      page.append(K.panel("All points", "pin", K.opts({ count: pts.length }), K.table([{ label: "As found", get: (p) => p.label }, { label: "Resolved to", get: (p) => p.place || "" }, { label: "Kind", get: (p) => p.kind }, { label: "Precision", get: (p) => p.precision }, { label: "Sources", get: (p) => (p.sources || []).slice(0, 3).join(" · ") }], pts)));
      for (const n of g.notes || []) page.append(K.hint(n));
    } };

  const place = K.section("place", "Place you gave & nearby", "pin", (M) => M.hits.filter((f) => f.module === "context" && (f.category === "geo" || f.category === "geo-media")), { showModule: false,
    intro: "The coordinates you supplied: the place's name, geotagged pictures and landmarks near it, and links to open map and photo searches. This is context about the place, not evidence about the subject.",
    empty: "No coordinates were supplied.", unlock: "enter Coordinates (lat, lon) in the form, or use -coords on the command line." });
  UI.defineTab({ id: "location", label: "Location", icon: "map", count: (M) => (M.geo ? (M.geo.points || []).length : 0), sections: [geo, place, exif, correlation] });
})();
