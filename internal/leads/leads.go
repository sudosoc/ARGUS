// Package leads lists, for each identifier, the sources that matter but cannot
// be queried automatically without an account, a CAPTCHA, or payment. argus
// never works around those barriers; instead it hands the operator ready-made
// links so nothing is silently skipped.
package leads

import (
	"net/url"
	"strings"

	"github.com/sudosoc/argus/internal/models"
)

func q(s string) string { return url.QueryEscape(s) }
func p(s string) string { return url.PathEscape(s) }

// Build returns the manual follow-ups for t, grouped by identifier type.
func Build(t models.Target) []models.Lead {
	var out []models.Lead
	add := func(group, label, u, why string) {
		out = append(out, models.Lead{Group: group, Label: label, URL: u, Why: why})
	}

	for _, u := range t.Usernames() {
		if len(out) > 400 {
			break
		}
		add("username", "Instagram", "https://www.instagram.com/"+p(u)+"/", "needs login to browse reliably")
		add("username", "X / Twitter", "https://x.com/"+p(u), "blocks anonymous scraping")
		add("username", "Facebook", "https://www.facebook.com/"+p(u), "needs login")
		add("username", "TikTok", "https://www.tiktok.com/@"+p(u), "anti-bot protected")
		add("username", "YouTube", "https://www.youtube.com/@"+p(u), "JavaScript-rendered")
		add("username", "Threads", "https://www.threads.net/@"+p(u), "needs login")
		add("username", "Snapchat", "https://www.snapchat.com/add/"+p(u), "app-first")
		add("username", "LinkedIn (search)", "https://www.linkedin.com/search/results/all/?keywords="+q(u), "needs login")
		add("username", "Web search", "https://www.google.com/search?q="+q(`"`+u+`"`), "search engines block scraping")
		add("username", "Namechk", "https://namechk.com/", "type the handle; checks 100+ platforms and domains, needs a human check")
		add("username", "Social Searcher", "https://www.social-searcher.com/search-users/?q="+q(u), "free tier is limited")
		add("username", "Reddit", "https://www.reddit.com/user/"+p(u), "often blocks scripted access")
		add("username", "Telegram", "https://t.me/"+p(u), "public profile page, JavaScript")
		add("username", "Telegram channel search", "https://www.google.com/search?q="+q("site:t.me "+u), "Telegago-style search of public channels")
		add("username", "GitHub code search", "https://github.com/search?type=code&q="+q(`"`+u+`"`), "needs a GitHub login")
		add("username", "Pastes and leaks (web)", "https://www.google.com/search?q="+q(`"`+u+`" (site:pastebin.com OR site:gist.github.com OR site:ghostbin.com)`), "search engines block scraping")
		break // the primary handle is enough; aliases are covered by the automatic checks
	}
	if t.FullName != "" {
		add("name", "LinkedIn people", "https://www.linkedin.com/search/results/people/?keywords="+q(t.FullName), "needs login")
		add("name", "Facebook people", "https://www.facebook.com/search/people/?q="+q(t.FullName), "needs login")
		add("name", "X / Twitter people", "https://x.com/search?f=user&q="+q(t.FullName), "needs login")
		add("name", "Web search", "https://www.google.com/search?q="+q(`"`+t.FullName+`"`), "search engines block scraping")
		add("name", "Image search", "https://www.google.com/search?tbm=isch&q="+q(`"`+t.FullName+`"`), "visual identification is manual")
		add("name", "Name variant search", "https://bellingcat.github.io/name-variant-search/", "type the name; opens searches for spelling variants")
		add("name", "OpenSanctions", "https://www.opensanctions.org/search/?q="+q(t.FullName), "sanctions and PEP database; the automatic check covers OFAC and the UN only")
		add("name", "ICIJ Offshore Leaks", "https://offshoreleaks.icij.org/search?q="+q(t.FullName), "Panama / Pandora / Paradise papers; interactive")
		add("name", "OCCRP Aleph", "https://aleph.occrp.org/search?q="+q(t.FullName), "leaks, registries, sanctions; needs an account for full data")
		add("name", "LittleSis", "https://littlesis.org/search?q="+q(t.FullName), "links between people in politics and business")
		add("name", "OpenSecrets", "https://www.opensecrets.org/search?q="+q(t.FullName)+"&type=donors", "US campaign donations")
		add("name", "Court records (UK)", "https://www.thelawpages.com/court-cases/court-case-search.php?mode=1", "search by name; interactive")
		add("name", "192.com (UK)", "https://www.192.com/people/search/?name="+q(t.FullName), "UK electoral roll based; paid for details")
		add("name", "FastPeopleSearch (US)", "https://www.fastpeoplesearch.com/name/"+p(strings.ReplaceAll(strings.ToLower(t.FullName), " ", "-")), "US people search; private individuals, use with care")
		add("name", "OSINT tools map by country", "https://cipher387.github.io/osintmap/", "yellow/white pages, registries and court search per country")
		add("name", "Google Scholar", "https://scholar.google.com/scholar?q="+q(`author:"`+t.FullName+`"`), "CAPTCHA protected")
	}
	if t.FullName != "" || len(t.Usernames()) > 0 {
		add("image", "Google Lens", "https://lens.google.com/", "upload or paste a picture found on a profile")
		add("image", "Yandex Images", "https://yandex.com/images/", "best general reverse search for faces and places; upload a picture")
		add("image", "TinEye", "https://tineye.com/", "exact and altered copies of a picture")
		add("image", "Bing Visual Search", "https://www.bing.com/visualsearch", "upload a picture")
		add("image", "PimEyes", "https://pimeyes.com/en", "face search engine; paid for source links; a human must decide whether a match is the same person")
		add("image", "FaceCheck.ID", "https://facecheck.id/", "face search across social media; paid; verify every match by hand")
		add("image", "Search4Faces", "https://search4faces.com/", "face search on VK, OK, TikTok, Clubhouse")
	}
	if t.Alias != "" {
		add("alias", "Web search", "https://www.google.com/search?q="+q(`"`+t.Alias+`"`), "search engines block scraping")
	}
	if t.Email != "" {
		add("email", "Epieos (accounts by e-mail)", "https://epieos.com/?q="+q(t.Email)+"&t=email", "CAPTCHA / JavaScript")
		add("email", "Have I Been Pwned", "https://haveibeenpwned.com/account/"+p(t.Email), "breach lookup needs a human check")
		add("email", "IntelX", "https://intelx.io/?s="+q(t.Email), "paid / account for full results")
		add("email", "DeHashed", "https://dehashed.com/search?query="+q(t.Email), "paid; breach data")
		add("email", "Gravatar check", "https://gravatar.com/site/check/"+p(t.Email), "profile picture and linked accounts if one exists")
		add("email", "Google account (Epieos)", "https://epieos.com/?q="+q(t.Email)+"&t=email", "Google profile picture and public Maps reviews; CAPTCHA")
		add("email", "Code search (GitHub)", "https://github.com/search?type=code&q="+q(`"`+t.Email+`"`), "needs a GitHub login")
		add("email", "Pastes (web)", "https://www.google.com/search?q="+q(`"`+t.Email+`" (site:pastebin.com OR site:gist.github.com)`), "search engines block scraping")
		add("email", "Web search", "https://www.google.com/search?q="+q(`"`+t.Email+`"`), "search engines block scraping")
	}
	if t.Phone != "" {
		digits := strings.TrimPrefix(t.Phone, "+")
		add("phone", "WhatsApp", "https://wa.me/"+p(digits), "shows a profile photo only if the number is registered and public")
		add("phone", "Telegram", "https://t.me/+"+p(digits), "same")
		add("phone", "Truecaller", "https://www.truecaller.com/search/global/"+p(digits), "needs login")
		add("phone", "Sync.ME", "https://sync.me/search/?number="+q(t.Phone), "needs login")
		add("phone", "GetContact", "https://www.getcontact.com/en/", "crowd-sourced names; needs the app and an account")
		add("phone", "HaveIBeenZuckered", "https://haveibeenzuckered.com/", "was the number in the 2021 Facebook data leak")
		add("phone", "Signal / Viber / Line", "https://www.google.com/search?q="+q(`"`+t.Phone+`"`), "messengers show a name or photo only inside their apps")
		add("phone", "Web search", "https://www.google.com/search?q="+q(`"`+t.Phone+`"`), "search engines block scraping")
	}
	if t.Domain != "" {
		d := t.Domain
		add("domain", "VirusTotal", "https://www.virustotal.com/gui/domain/"+p(d), "needs an account for full data")
		add("domain", "urlscan.io", "https://urlscan.io/search/#"+q("domain:"+d), "JavaScript app")
		add("domain", "SecurityTrails", "https://securitytrails.com/domain/"+p(d)+"/dns", "historical DNS is paid")
		add("domain", "Shodan", "https://www.shodan.io/search?query="+q("hostname:"+d), "needs an account")
		add("domain", "Censys", "https://search.censys.io/search?q="+q(d), "needs an account")
		add("domain", "BuiltWith", "https://builtwith.com/"+p(d), "full history is paid")
		add("domain", "Wayback Machine", "https://web.archive.org/web/*/"+d, "interactive calendar")
		add("domain", "crt.sh certificates", "https://crt.sh/?q=%25."+q(d), "every certificate ever issued, including internal host names")
		add("domain", "ICANN lookup", "https://lookup.icann.org/en/lookup?name="+q(d), "current registration data")
		add("domain", "Whoxy reverse WHOIS", "https://www.whoxy.com/", "search the registrant name or e-mail to find sister domains; API is paid")
		add("domain", "DNSDumpster", "https://dnsdumpster.com/", "type the domain; interactive map of DNS")
		add("domain", "PublicWWW (source search)", "https://publicwww.com/websites/"+q(`"`+d+`"`)+"/", "sites whose source mentions the domain; free tier is limited")
		add("domain", "Archive.today", "https://archive.ph/search/?q="+q(d), "snapshots that the Wayback Machine may not hold")
		add("domain", "Lumen (takedown notices)", "https://lumendatabase.org/notices/search?term="+q(d), "legal complaints that name the domain")
		add("domain", "ViewDNS reverse WHOIS", "https://viewdns.info/reversewhois/?q="+q(d), "CAPTCHA")
	}
	if t.IP != "" {
		add("ip", "Shodan", "https://www.shodan.io/host/"+p(t.IP), "full banners need an account")
		add("ip", "Censys", "https://search.censys.io/hosts/"+p(t.IP), "needs an account")
		add("ip", "VirusTotal", "https://www.virustotal.com/gui/ip-address/"+p(t.IP), "needs an account")
		add("ip", "AbuseIPDB", "https://www.abuseipdb.com/check/"+p(t.IP), "reports need an account")
		add("ip", "AlienVault OTX", "https://otx.alienvault.com/indicator/ip/"+p(t.IP), "needs an account")
		add("ip", "BGP.HE.net", "https://bgp.he.net/ip/"+p(t.IP), "interactive")
	}
	if t.ASN != "" {
		n := strings.TrimPrefix(t.ASN, "AS")
		add("asn", "BGP.HE.net", "https://bgp.he.net/AS"+p(n), "interactive")
		add("asn", "PeeringDB", "https://www.peeringdb.com/asn/"+p(n), "interactive")
	}
	if t.Company != "" {
		add("company", "OpenCorporates", "https://opencorporates.com/companies?q="+q(t.Company), "CAPTCHA protected")
		add("company", "LinkedIn companies", "https://www.linkedin.com/search/results/companies/?keywords="+q(t.Company), "needs login")
		add("company", "Crunchbase", "https://www.crunchbase.com/textsearch?q="+q(t.Company), "anti-bot protected")
		add("company", "SEC EDGAR", "https://www.sec.gov/cgi-bin/browse-edgar?action=getcompany&company="+q(t.Company), "US filings; needs a contact User-Agent")
		add("company", "OpenSanctions", "https://www.opensanctions.org/search/?q="+q(t.Company), "sanctions, PEPs and related companies")
		add("company", "ICIJ Offshore Leaks", "https://offshoreleaks.icij.org/search?q="+q(t.Company), "offshore entities and officers; interactive")
		add("company", "OCCRP Aleph", "https://aleph.occrp.org/search?q="+q(t.Company), "registries and leaks; account needed for full data")
		add("company", "Companies House (UK)", "https://find-and-update.company-information.service.gov.uk/search?q="+q(t.Company), "UK officers and filings")
		add("company", "North Data (EU)", "https://www.northdata.com/?q="+q(t.Company), "EU company registers, partly paid")
		add("company", "ImportYeti", "https://www.importyeti.com/search?q="+q(t.Company), "US customs shipping records")
		add("company", "Open Ownership", "https://register.openownership.org/search?q="+q(t.Company), "beneficial ownership registers")
		add("company", "Web search", "https://www.google.com/search?q="+q(`"`+t.Company+`"`), "search engines block scraping")
	}
	if t.Wallet != "" {
		w := t.Wallet
		add("wallet", "Chainabuse", "https://www.chainabuse.com/address/"+p(w), "scam reports; JavaScript app")
		add("wallet", "ScamSearch", "https://scamsearch.io/search_report?searchoption=all&search="+q(w), "needs an account")
		add("wallet", "Breadcrumbs / OKLink", "https://www.oklink.com/multi-search#key="+q(w), "multi-chain explorer, JavaScript app")
		if strings.HasPrefix(w, "0x") {
			add("wallet", "Etherscan", "https://etherscan.io/address/"+p(w), "labels and token holdings (CAPTCHA)")
		} else {
			add("wallet", "WalletExplorer", "https://www.walletexplorer.com/address/"+p(w), "clusters wallets by owner")
		}
	}
	return out
}
