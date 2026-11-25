"use strict";

// Copyright (C) 2026 Seif, SUDOSOC. SPDX-License-Identifier: AGPL-3.0-only

// Location tab: best-supported place, an evidence map (schematic outline, or
// real OpenStreetMap tiles fetched through this server), the evidence table and
// the timezone estimated from activity hours. Evidence kinds are told apart by
// marker shape, not colour.
(function () {
  const { $, el } = UI;

  // coarse continent outlines [lon, lat]: enough to orient a dot, not a cartographic map
  const LAND = [
    [[-168,66],[-156,71],[-125,70],[-95,72],[-80,73],[-62,60],[-55,50],[-67,45],[-76,35],[-81,25],[-97,26],[-105,20],[-95,16],[-83,9],[-80,8],[-92,14],[-105,22],[-115,30],[-124,40],[-125,49],[-135,58],[-150,60],[-165,60]],
    [[-80,8],[-60,10],[-50,0],[-35,-6],[-40,-22],[-48,-28],[-58,-38],[-65,-42],[-68,-52],[-74,-50],[-72,-30],[-71,-18],[-81,-5]],
    [[-10,36],[-9,43],[-2,48],[5,53],[8,57],[12,56],[20,60],[25,66],[30,70],[60,69],[80,73],[110,77],[140,72],[170,70],[180,66],[160,58],[140,52],[135,43],[122,40],[122,30],[110,20],[106,10],[100,2],[98,15],[92,22],[80,15],[77,8],[72,20],[66,25],[57,25],[52,16],[43,12],[35,28],[35,36],[27,37],[20,40],[15,38],[8,44],[-1,37]],
    [[-17,21],[-10,32],[10,37],[32,31],[35,28],[43,12],[51,12],[40,-3],[40,-15],[33,-26],[20,-35],[14,-25],[12,-5],[9,4],[-8,4],[-17,14]],
    [[114,-22],[122,-18],[136,-12],[142,-11],[146,-19],[153,-26],[150,-37],[140,-38],[131,-31],[115,-34]],
    [[-55,60],[-45,60],[-22,70],[-20,82],[-60,82],[-70,76]],
  ];
  // evidence kind -> [marker shape, label]
  const KIND = {
    "self-declared": ["circle", "stated in a profile"],
    exif: ["square", "GPS inside a picture"],
    organisation: ["diamond", "company / employer address"],
    ip: ["triangle", "IP geolocation"],
    phone: ["cross", "phone number region"],
    activity: ["hexagon", "activity hours"],
    given: ["pentagon", "location you gave"],
  };
  const kind = (k) => KIND[k] || ["circle", k || "evidence"];

  function mark(g, shape, x, y, r, fill, stroke, lw) {
    g.save();
    g.translate(x, y);
    const p = new Path2D(UI.shapePath(shape, r));
    g.globalAlpha = 0.5; g.fillStyle = fill; g.fill(p);
    g.globalAlpha = 1; g.strokeStyle = stroke; g.lineWidth = lw; g.stroke(p);
    g.restore();
  }

  function sizeCanvas(canvas, ratio) {
    const dpr = window.devicePixelRatio || 1;
    const W = canvas.clientWidth || 800, H = Math.round(W * ratio);
    canvas.width = W * dpr; canvas.height = H * dpr; canvas.style.height = H + "px";
    const g = canvas.getContext("2d");
    g.setTransform(dpr, 0, 0, dpr, 0, 0);
    return { g, W, H };
  }

  function label(g, text, x, y, W) {
    g.font = "600 12px system-ui, sans-serif";
    g.textBaseline = "middle";
    const tw = g.measureText(text).width;
    const lx = x + 22 + tw + 12 > W ? x - 22 - tw - 12 : x + 22;
    g.fillStyle = UI.token("--surface-1"); g.globalAlpha = 0.92; g.fillRect(lx, y - 11, tw + 12, 22); g.globalAlpha = 1;
    g.strokeStyle = UI.token("--line"); g.lineWidth = 1; g.strokeRect(lx + 0.5, y - 10.5, tw + 11, 21);
    g.fillStyle = UI.token("--text"); g.fillText(text, lx + 6, y);
  }

  function drawMap(canvas, geo) {
    const { g, W, H } = sizeCanvas(canvas, 0.5);
    const line = UI.token("--line"), well = UI.token("--well"), strong = UI.token("--emph-strong"), text = UI.token("--text"), land = UI.token("--surface-3");
    const X = (lon) => ((lon + 180) / 360) * W, Y = (lat) => ((90 - lat) / 180) * H;
    g.fillStyle = well; g.fillRect(0, 0, W, H);
    g.strokeStyle = line; g.lineWidth = 0.5;
    for (let lon = -150; lon <= 180; lon += 30) { g.beginPath(); g.moveTo(X(lon), 0); g.lineTo(X(lon), H); g.stroke(); }
    for (let lat = -60; lat <= 60; lat += 30) { g.beginPath(); g.moveTo(0, Y(lat)); g.lineTo(W, Y(lat)); g.stroke(); }
    g.fillStyle = land;
    for (const poly of LAND) {
      g.beginPath();
      poly.forEach(([lo, la], i) => (i ? g.lineTo(X(lo), Y(la)) : g.moveTo(X(lo), Y(la))));
      g.closePath(); g.fill();
    }
    // timezone band: the estimated offset is a vertical strip 15 degrees wide
    if (geo.timezone) {
      const c = geo.timezone.utc_offset * 15;
      g.fillStyle = strong; g.globalAlpha = 0.12; g.fillRect(X(c - 7.5), 0, X(c + 7.5) - X(c - 7.5), H); g.globalAlpha = 1;
    }
    for (const p of geo.points || []) mark(g, kind(p.kind)[0], X(p.lon), Y(p.lat), 4 + p.weight * 6, text, text, 1.4);
    const top = (geo.clusters || [])[0];
    if (top) {
      g.beginPath(); g.arc(X(top.lon), Y(top.lat), 15, 0, Math.PI * 2);
      g.strokeStyle = strong; g.lineWidth = 2.5; g.stroke();
      label(g, top.label, X(top.lon), Y(top.lat), W);
    }
  }

  // Real map: OpenStreetMap tiles fetched through this server (/api/tile),
  // desaturated so they sit inside the palette, with the evidence markers on top.
  function drawTiles(canvas, geo, onStatus) {
    const pts = geo.points || [];
    if (!pts.length) return;
    const { g, W, H } = sizeCanvas(canvas, 0.55);
    const dark = document.documentElement.getAttribute("data-theme") !== "light";
    const lat2y = (lat, z) => ((1 - Math.log(Math.tan((lat * Math.PI) / 180) + 1 / Math.cos((lat * Math.PI) / 180)) / Math.PI) / 2) * 256 * 2 ** z;
    const lon2x = (lon, z) => ((lon + 180) / 360) * 256 * 2 ** z;
    const lats = pts.map((p) => p.lat), lons = pts.map((p) => p.lon);
    const minLat = Math.min(...lats), maxLat = Math.max(...lats), minLon = Math.min(...lons), maxLon = Math.max(...lons);
    let z = 11;
    for (; z > 1; z--) {
      if (Math.abs(lon2x(maxLon, z) - lon2x(minLon, z)) < W * 0.8 && Math.abs(lat2y(minLat, z) - lat2y(maxLat, z)) < H * 0.8) break;
    }
    const cx = (lon2x(minLon, z) + lon2x(maxLon, z)) / 2, cy = (lat2y(minLat, z) + lat2y(maxLat, z)) / 2;
    const x0 = cx - W / 2, y0 = cy - H / 2;
    g.fillStyle = UI.token("--well"); g.fillRect(0, 0, W, H);
    const n = 2 ** z;
    let pending = 0, failed = 0;
    const markers = () => {
      g.filter = "none";
      const fg = dark ? UI.token("--text") : UI.token("--bg");
      for (const p of pts) mark(g, kind(p.kind)[0], lon2x(p.lon, z) - x0, lat2y(p.lat, z) - y0, 5 + p.weight * 6, fg, fg, 1.6);
      const top = (geo.clusters || [])[0];
      if (top) {
        const x = lon2x(top.lon, z) - x0, y = lat2y(top.lat, z) - y0;
        g.beginPath(); g.arc(x, y, 16, 0, Math.PI * 2); g.strokeStyle = UI.token("--emph-strong"); g.lineWidth = 3; g.stroke();
        label(g, top.label, x, y, W);
      }
      g.font = "10px system-ui"; g.fillStyle = fg; g.globalAlpha = 0.85; g.textBaseline = "alphabetic";
      g.fillText("© OpenStreetMap contributors", 6, H - 8); g.globalAlpha = 1;
      onStatus(failed ? failed + " tile(s) could not be loaded." : "");
    };
    const tx0 = Math.floor(x0 / 256), tx1 = Math.floor((x0 + W) / 256), ty0 = Math.floor(y0 / 256), ty1 = Math.floor((y0 + H) / 256);
    for (let tx = tx0; tx <= tx1; tx++) {
      for (let ty = ty0; ty <= ty1; ty++) {
        if (ty < 0 || ty >= n) continue;
        pending++;
        const img = new Image();
        const done = () => { if (--pending === 0) markers(); };
        img.onload = () => {
          g.filter = dark ? "grayscale(1) invert(1) brightness(0.8) contrast(1.1)" : "grayscale(1) contrast(0.95)";
          g.drawImage(img, tx * 256 - x0, ty * 256 - y0);
          done();
        };
        img.onerror = () => { failed++; done(); };
        img.src = `/api/tile/${z}/${((tx % n) + n) % n}/${ty}`;
      }
    }
    if (pending === 0) markers();
  }

  const osmLink = (p) => `https://www.openstreetmap.org/?mlat=${p.lat}&mlon=${p.lon}#map=${p.precision === "country" ? 5 : p.precision === "region" ? 7 : 11}/${p.lat}/${p.lon}`;

  function location() {
    const S = UI.S, root = $("tab-location");
    root.replaceChildren();
    const geo = S.report && S.report.geo;
    if (!geo) {
      root.append(UI.emptyState("map", S.state === "running" ? "Location analysis runs when the scan finishes" : "No location evidence was found",
        S.state === "running" ? "Once all profiles, IP/phone data, pictures and activity hours are in, the evidence is geocoded and clustered here."
          : "Give a username (activity hours), an IP or phone number, or a full name with some context, and run again."));
      return;
    }
    const top = (geo.clusters || [])[0];
    const head = el("section", { class: "panel loc-head" }, el("div", { class: "eyebrow", text: "Most likely location" }),
      el("h2", { class: "loc-main", text: top ? top.label : "Not determined" }));
    if (top) {
      const p = Math.round(top.score * 100);
      head.append(el("div", { class: "loc-support" }, el("span", { class: "num strongnum", text: p + "%" }), el("span", { class: "muted", text: "support" }), UI.meter(p),
        el("span", { class: "hint", text: "from " + (top.kinds || []).map((k) => kind(k)[1]).join(", ") })));
    }
    if (geo.given) {
      const fit = { agrees: ["check", "agrees with the location you gave (" + geo.given + ")"], differs: ["alert", "differs from the location you gave (" + geo.given + ")"] }[geo.given_fit];
      if (fit) head.append(el("div", { class: "callout" + (geo.given_fit === "differs" ? " warn" : "") }, UI.icon(fit[0]), el("span", { text: fit[1] })));
    }
    if (geo.conflict) head.append(el("div", { class: "callout warn" }, UI.icon("alert"), el("span", { text: geo.conflict })));
    for (const n of geo.notes || []) head.append(el("div", { class: "hint", text: "• " + n }));
    root.append(head);

    const mapPanel = el("section", { class: "panel" }, el("h3", { class: "ph" }, UI.icon("map"), "Evidence map"));
    const canvas = el("canvas", { class: "locmap", role: "img", "aria-label": "Map of location evidence" });
    mapPanel.append(canvas);
    const legend = el("div", { class: "chips legend" });
    for (const [k, [shape, lab]] of Object.entries(KIND)) legend.append(el("span", { class: "chip" }, UI.shapeIcon(shape), el("span", { class: "chip-v", text: lab })));
    legend.append(el("span", { class: "chip ring" }, el("span", { class: "ringdot" }), el("span", { class: "chip-v", text: "best-supported place" })));
    const status = el("span", { class: "hint" });
    const mapBtn = UI.btn("Load street map", { icon: "map", cls: "sm", title: "The map tiles are downloaded by SUDOSOC ARGUS from tile.openstreetmap.org (they reveal which area you are viewing to OpenStreetMap, nothing about the subject). Zoom stops at city level." });
    mapBtn.addEventListener("click", () => { mapBtn.disabled = true; status.textContent = "Loading tiles…"; drawTiles(canvas, geo, (m) => { status.textContent = m; }); });
    mapPanel.append(el("div", { class: "maptools" }, mapBtn, status), legend,
      el("p", { class: "hint", text: "Marker size = how much that evidence is trusted. Red ring = best-supported place. Shaded strip = timezone estimated from activity hours. Outlines are schematic; the street map loads OpenStreetMap tiles through this server." }));
    root.append(mapPanel);
    requestAnimationFrame(() => drawMap(canvas, geo));

    const two = el("div", { class: "two" });
    if ((geo.clusters || []).length > 1) {
      const p = el("section", { class: "panel" }, el("h3", { class: "ph" }, UI.icon("pin"), "Candidate places", el("span", { class: "count", text: String(geo.clusters.length) })));
      for (const c of geo.clusters) {
        const s = Math.round(c.score * 100);
        p.append(el("div", { class: "cluster" }, el("span", { class: "num cl-n", text: s + "%" }), UI.meter(s),
          el("span", { class: "cl-t" }, el("b", { text: c.label }), el("span", { class: "muted", text: "  " + (c.kinds || []).join(" · ") + " · " + c.points.length + " clue" + (c.points.length === 1 ? "" : "s") }))));
      }
      two.append(p);
    }
    if (geo.timezone) {
      const tz = geo.timezone;
      two.append(el("section", { class: "panel" }, el("h3", { class: "ph" }, UI.icon("clock"), "Timezone from activity hours"),
        el("p", {}, el("b", { text: tz.label }), ` · confidence ${Math.round(tz.confidence * 100)}% · ${UI.n(tz.events)} public events (${(tz.sources || []).join(", ")})`),
        el("p", { class: "muted", text: "Typical places: " + ((tz.regions || []).join("; ") || "—") }),
        UI.hoursChart(tz), UI.heatmap(tz) || document.createTextNode(""),
        el("p", { class: "hint", text: "Bars = events per hour (UTC). Hatched bars = the quiet window assumed to be local night (00:00–07:00). Shift work, automation, bots and travel weaken this signal." })));
    }
    if (two.children.length) root.append(two);

    const tbl = el("table", { class: "ctable" }, el("thead", {}, el("tr", {}, ...["Type", "As found", "Resolved to", "Precision", "Trust", "Sources"].map((h) => el("th", { scope: "col", text: h })))));
    const tb = el("tbody");
    for (const p of geo.points) {
      const href = UI.safeHref(osmLink(p));
      const resolved = [p.city, p.region, p.country].filter(Boolean).join(", ") || p.place || "map";
      const w = Math.round(p.weight * 100);
      tb.append(el("tr", {},
        el("td", { class: "nowrap" }, UI.shapeIcon(kind(p.kind)[0]), " " + p.kind),
        el("td", { text: p.label }),
        el("td", {}, href ? el("a", { href, target: "_blank", rel: "noopener noreferrer", text: resolved }) : document.createTextNode(resolved)),
        el("td", { text: p.precision }),
        el("td", {}, el("span", { class: "meter-wrap" }, UI.meter(w), el("span", { class: "num", text: w + "%" }))),
        el("td", { class: "muted", text: (p.sources || []).slice(0, 3).join(" · ") })));
    }
    tbl.append(tb);
    root.append(el("section", { class: "panel" }, el("h3", { class: "ph" }, UI.icon("list"), "Evidence", el("span", { class: "count", text: String(geo.points.length) })), el("div", { class: "tablewrap" }, tbl)));
    root.append(el("p", { class: "hint pad", text: "Location here comes from what is publicly stated or technically visible, and is shown at city / region precision. IP and phone data describe an ISP or number range, not a person's home. SUDOSOC ARGUS does not search for street addresses." }));
  }

  UI.views.location = location;
  window.addEventListener("resize", UI.debounce(() => { if (UI.S && UI.tab === "location") UI.scheduleRender(true); }, 200));
})();
