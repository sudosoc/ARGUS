"use strict";

// Infrastructure: everything learned about domains, hosts and networks.
(function () {
  const K = UI.kit;
  const hostOf = (v) => String(v || "").toLowerCase();

  /** A section that is a stats strip plus a finding list selected by a predicate. */
  UI.listSection = (id, label, icon, pick, { empty, unlock, stats, group, intro, showModule = true } = {}) => ({
    id, label, icon, group,
    count: (M) => pick(M).length,
    render(page, M) {
      const list = pick(M);
      if (!list.length) { page.append(K.empty(icon, "Nothing here yet", empty || "No findings in this group.", unlock)); return; }
      if (intro) page.append(K.hint(intro));
      const st = stats ? stats(M, list) : [[list.length, "findings", { lead: true }], [new Set(list.map((f) => f.source)).size, "sources"], [list.filter((f) => f.confidence >= 0.8).length, "high confidence"]];
      page.append(K.stats(st));
      page.append(K.findings(label, icon, list, { showModule }));
    },
  });
  const L = UI.listSection;

  const whois = L("whois", "WHOIS / RDAP", "file", (M) => M.hits.filter((f) => f.category === "whois" || f.category === "registry" && f.module !== "bizreg" || (f.module === "syswrap" && /^whois/i.test(f.source))), {
    empty: "No registration data.", unlock: "give a domain, an IP or an ASN: RDAP is read for the registrar, dates, name servers and contacts.", intro: "Registration records: who registered the name or network, when, and through whom. Privacy services hide the person; the dates and name servers still help.",
  });
  const dns = L("dns", "DNS records", "server", (M) => M.hits.filter((f) => ["dns", "email-security", "email-domain"].includes(f.category) || (f.module === "syswrap" && /^dig/i.test(f.source))), {
    empty: "No DNS records.", unlock: "give a domain (or an e-mail on your own domain).", intro: "A, MX, TXT and NS records, mail security (SPF, DMARC, DKIM) and the services a domain's TXT records reveal.",
  });
  const subdomains = L("subdomains", "Subdomains", "globe", (M) => M.hits.filter((f) => f.category === "subdomain" || f.category === "takeover" || (f.module === "syswrap" && /subfinder|theharvester/i.test(f.source))), {
    empty: "No subdomains.", unlock: "give a domain: certificate-transparency logs and passive DNS list its hosts.", stats: (M, list) => [[M.entsOf("domain").length, "domains and hosts", { lead: true }], [list.length, "findings"], [list.filter((f) => f.category === "takeover").length, "takeover candidates", { warn: list.some((f) => f.category === "takeover") }]],
  });
  const ip = L("ip", "IP intelligence", "server", (M) => M.hits.filter((f) => f.module === "ip" || f.category === "ip" || f.category === "reputation" || f.category === "network" && f.module !== "asn"), {
    empty: "No IP data.", unlock: "give an IP address (or a domain: its addresses are followed).", stats: (M, list) => [[M.entsOf("ip").length, "addresses", { lead: true }], [list.length, "findings"]],
  });
  const asn = L("asn", "ASN & hosting", "network", (M) => M.hits.filter((f) => f.module === "asn" || /asn|hosting|autonomous|bgp/i.test(f.source) || (f.details && (f.details.asn || f.details.ASN))), {
    empty: "No network data.", unlock: "give an AS number or an IP: the holder, announced prefixes and neighbours are read from RIPEstat and RDAP.",
  });
  const tls = L("tls", "SSL / TLS", "shield", (M) => M.hits.filter((f) => f.category === "tls" || f.category === "security"), {
    empty: "No TLS data.", unlock: "give a domain or an IP with a web server: the certificate and security headers are read.", intro: "The certificate a server presents, its validity and names, and the hardening headers of the website.",
  });
  const tech = L("tech", "Technology fingerprinting", "layers", (M) => M.hits.filter((f) => f.category === "tech" || f.category === "tracking" || (f.module === "syswrap" && /whatweb/i.test(f.source))), {
    empty: "No technology detected.", unlock: "give a domain: headers, cookies, page source and analytics IDs reveal the stack (and sibling sites sharing an ID).",
  });
  const hist = L("historical", "Historical records", "clock", (M) => M.hits.filter((f) => f.category === "archive" || f.category === "history" || f.module === "webhistory" || (f.category === "tls" && /history|previous|first/i.test(JSON.stringify(f.details || {})))), {
    empty: "No historical records.", unlock: "give a domain: archived copies, analytics-ID history and certificate history are read.", intro: "Public history of a domain: archived pages, analytics IDs over the years and certificates. Passive-DNS history needs paid sources and is not collected.",
  });
  const lookalike = L("lookalike", "Look-alike domains", "globe", (M) => M.hits.filter((f) => f.category === "lookalike"), {
    empty: "No registered look-alike domains.", unlock: "give a domain: typo, homograph and keyword variants are resolved to see which are registered.",
  });

  UI.defineTab({ id: "infrastructure", label: "Infrastructure", icon: "server", count: (M) => M.entsOf("domain", "ip", "asn").length, sections: [whois, dns, subdomains, ip, asn, tls, tech, hist, lookalike] });
})();
