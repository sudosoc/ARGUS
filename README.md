<p align="center"><img src="docs/assets/argus-logo.png" alt="SUDOSOC ARGUS: OSINT intelligence platform. See more. Know more." width="340"></p>

<p align="center"><img src="docs/assets/argus-banner.jpg" alt="" width="100%"></p>

# SUDOSOC ARGUS

API-less OSINT reconnaissance in Go. Give it any combination of full name, username, email, phone, domain, IP address, company, ASN, crypto wallet and location; it decides which modules apply, runs them concurrently on a shared worker pool, follows the strongest new leads it discovers, resolves everything it finds into separate people (never blending two people who share a name), and exports reports in 13 formats.

At a glance: **10,600+ username sites** (WhatsMyName + Sherlock + Maigret, in three depth tiers) · **30 modules** · **identity resolution with reliability scores** · **location and time-zone analysis** · **sanctions screening, OpenPGP key lookup, look-alike domains, tracking-ID history, platform-ID timestamps, Bluesky social circle, public code search** · **file, place and period inputs (pictures with OCR, documents, videos, coordinates, dates)** · **API settings with encrypted keys** · **saved cases with comparison** · **SQL / Markdown / STIX / MISP / Maltego / GraphML exports**.

> Use only on subjects you are lawfully permitted to investigate. Results are leads, not proof of identity. Licensed under the [AGPL-3.0](LICENSE); name and artwork reserved (see [Licence](#licence)).

## Screenshots

All screenshots show a **synthetic demonstration case** (an invented person and `example` domains); nothing in them is real data. (Where a picture cannot be loaded, ARGUS shows its emblem as a placeholder; in the capture environment no pictures could be fetched, so the screenshots show placeholders.) The first three are the first-use flow, then one image per main view.

| | |
|---|---|
| ![First start](docs/screenshots/01-first-use-notice.png)<br>**1. First start:** the emblem and a short notice about lawful use. | ![Start page](docs/screenshots/02-start-page.png)<br>**2. Start page:** one clue is enough; every field has an ⓘ hint in English and Arabic. |
| ![File, place and period inputs](docs/screenshots/27-file-inputs.png)<br>**3. File, place and period inputs:** drop pictures, documents or videos; what is found inside is offered as leads. Coordinates and a date range sit beside them. | ![Live scan](docs/screenshots/03-live-scan.png)<br>**4. Live scan:** every collector with progress, hits, errors and time. |
| ![Overview](docs/screenshots/04-overview.png)<br>**5. Overview:** key facts about the subject, risk signals ranked by severity, coverage gaps. | ![Identity](docs/screenshots/05-identity.png)<br>**6. Identity:** who the findings add up to, scored; other candidates kept apart. Sections: Profile, Aliases, Usernames, E-mails, Phones, Digital identifiers, Verification. |
| ![Digital footprint](docs/screenshots/06-footprint.png)<br>**7. Digital footprint:** accounts by platform, web presence, historical footprints, username reuse, public mentions, cached pages, activity and cross-platform matches. | ![Social collectors](docs/screenshots/32-social-collectors.png)<br>**8. Social collectors:** output of facebook-scraper, instagram-osint, Insta OSINT Engine, open-nami, The 3rd Eye and SpiderFoot, when installed. |
| ![Infrastructure](docs/screenshots/07-infrastructure.png)<br>**9. Infrastructure:** WHOIS/RDAP, DNS, subdomains, IP intelligence, ASN, TLS, technology fingerprints, historical records, look-alike domains. | ![Media](docs/screenshots/08-media.png)<br>**10. Media:** downloaded pictures fingerprinted and compared, EXIF, similar images, reverse-search links, media timeline. |
| ![Uploaded files](docs/screenshots/29-uploaded-files.png)<br>**11. Uploaded files:** pictures, documents and videos you supplied, analysed locally: metadata, text, OCR, frames and the leads inside. | ![Relationship graph](docs/screenshots/09-graph.png)<br>**12. Relationship graph:** entities and how each lead was found; responsive canvas with auto-fit, zoom, pan, pinch, a minimap and full screen; also shared identifiers, account relationships, infrastructure links, connection paths and clusters. |
| ![Graph layouts](docs/screenshots/25-graph-layouts.png)<br>**13. Graph layouts:** the same graph as rings by hops, a tree, a circle or groups by type; pinned nodes stay where you put them. | ![Graph full screen](docs/screenshots/26-graph-fullscreen.png)<br>**14. Graph full screen:** full-screen mode with the minimap for large investigations; “Fit all” and “Core” keep it readable. |
| ![Intelligence tree](docs/screenshots/10-intelligence-tree.png)<br>**15. Intelligence tree:** the same case as a tree of leads. | ![Location](docs/screenshots/11-location.png)<br>**16. Location:** evidence clustered on a map, conflicts called out, EXIF and geographic correlation. |
| ![Place you gave](docs/screenshots/30-place-nearby.png)<br>**17. Place you gave:** the coordinates you entered: place name, geotagged pictures and landmarks nearby, map and photo-search links. | ![Timeline](docs/screenshots/12-timeline.png)<br>**18. Timeline:** every dated fact on one axis, plus account activity, domain history and changes. |
| ![Period you gave](docs/screenshots/31-period.png)<br>**19. Period you gave:** news in the window, the closest archive copy of the domain, what happened that day, and period-limited searches. | ![Exposure & risk](docs/screenshots/13-exposure-risk.png)<br>**20. Exposure & risk:** breach intelligence (metadata only), public leaks and a risk assessment with what to do about each signal. |
| ![Findings](docs/screenshots/14-findings.png)<br>**21. Findings:** sortable, filterable, paginated, exportable; every row opens its full evidence. | ![Conflicts](docs/screenshots/15-conflicts.png)<br>**22. Conflicts:** where sources disagree, never silently merged. |
| ![Evidence hashes](docs/screenshots/16-evidence-hashes.png)<br>**23. Evidence hashes:** SHA-256 of every finding, computed in the browser, plus the report's integrity digest. | ![Source coverage](docs/screenshots/17-evidence-coverage.png)<br>**24. Source coverage:** every collector: ran, skipped, checks, hits and errors. |
| ![Missing data](docs/screenshots/18-missing-data.png)<br>**25. Missing data:** what was not found and exactly what input would fill it. | ![Confidence score](docs/screenshots/19-confidence-score.png)<br>**26. Confidence score:** a transparent blend of completeness, evidence quality, corroboration and identity match. |
| ![Reports](docs/screenshots/20-report.png)<br>**27. Reports:** investigation summary, technical, intelligence and evidence reports: copy, download .md or print. | ![Exports](docs/screenshots/21-exports.png)<br>**28. Exports:** HTML, JSON, CSV, Markdown, SQL, JSONL, GraphML; a local export history. |
| ![API settings](docs/screenshots/28-api-settings.png)<br>**29. API settings:** masked keys, Enabled switch and Test connection per provider; model selector. | ![Command palette](docs/screenshots/22-command-palette.png)<br>**30. Command palette:** `Ctrl+K`: search the case or run any command. |
| ![Light theme](docs/screenshots/23-light-theme.png)<br>**31. Light theme:** `t` switches; the emblem keeps its round shape. | ![Phone](docs/screenshots/24-phone.png)<br>**32. Phone:** the same views on a narrow screen: tabs and sections scroll sideways. |
| ![Arabic interface](docs/screenshots/33-arabic-rtl.png)<br>**33. Arabic interface:** Settings → Language; the whole interface and the layout switch to right-to-left. |  |

## Settings: language, API keys and model

The **Settings** button (top bar, or `Ctrl+K` → Settings) holds the interface **language** (English or Arabic; Arabic switches the whole interface and layout to right-to-left, the choice is kept in your browser, reports and exports stay in English) and keys for model providers (Claude, OpenAI, Gemini, a local Ollama or OpenAI-compatible server) and for tools that need one (a Google API key for The 3rd Eye). Each provider has a masked field, an Enabled switch and **Test connection**; pick the provider and type the exact model name under *Model*. Keys are saved in an AES-256-GCM encrypted file under your config directory (`~/.config/argus/settings.enc`, mode 0600; the key sits next to it, or set `ARGUS_SETTINGS_KEY`), are never sent back to the browser (only the last four characters) and never enter reports or cases. Environment variables (`GOOGLE_API_KEY`, `OPENAI_API_KEY`, …) are read when nothing is stored. A tool that needs a key is skipped, with a note in the report, when its provider is off or has no key; otherwise the key is passed to it in its environment variable only. The encryption protects the file from accidental exposure (backups, a stray copy); it cannot protect against someone who already controls your user account. ARGUS has no model-powered feature yet: the model choice is stored and tested for when one is added.

## Uninstall

```bash
./uninstall.sh --dry-run     # show what would be removed
./uninstall.sh               # remove it (asks first)
./uninstall.sh --purge       # ALSO delete your saved cases, settings and API keys (asks you to type DELETE)
```

It removes the `argus` program, the helper tools `install.sh` put in its private data directory (the `maigret`, `sherlock`, `holehe`, `h8mail` and `theHarvester` virtualenvs and their links), `phoneinfoga` and `subfinder` from the bin directory, the automatic-update timer or cron entry, and the PATH lines `install.sh` added to your shell start-up files. It finds user (`~/.local`) and system (`/usr/local`) installs; force one with `--user`, `--system` or `--prefix DIR`. It keeps your saved cases and settings unless you pass `--purge`, and never touches exported reports, the source folder you cloned, or OS packages (it prints the commands for those). `--remove-go` also removes the Go toolchain `install.sh` installed; `--keep-extras` leaves `phoneinfoga` and `subfinder`; `--yes` skips the question. `./install.sh --uninstall` runs the same script. It works from a temporary copy, so it can delete the folder it lives in.

## Documentation

- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md): how the code is organised, what happens in a run, the web server, the front end and the language layer.
- [docs/WRITING-A-MODULE.md](docs/WRITING-A-MODULE.md): add a collector (the contract, an example, every place to wire it in, how to test it offline, wrapping external programs).
- [docs/COMPARING-CASES.md](docs/COMPARING-CASES.md): saved cases, comparing two runs, how to read a difference, comparing from the command line.
- [docs/COVERAGE.md](docs/COVERAGE.md): what is collected per identifier and what is not possible.

## Licence

SUDOSOC ARGUS is free software under the **GNU Affero General Public License v3.0 only** ([LICENSE](LICENSE), `AGPL-3.0-only`). Copyright (C) 2026 Seif, SUDOSOC.

* You may use, study, modify and share it. If you distribute a copy, or let others use a modified version over a network, you must give them the complete source under the same licence; the web interface links to the source for that reason.
* The username-site list is a derivative of CC BY-SA 4.0 data and keeps that licence ([data/sites/LICENSE.md](data/sites/LICENSE.md)); third-party notices are in [NOTICE.md](NOTICE.md).
* **The name SUDOSOC ARGUS, the logos and the artwork are not covered by the licence.** They belong to Seif and SUDOSOC alone; no one may use or imitate them or present anything as coming from or endorsed by SUDOSOC. A modified version must use another name and other artwork ([NOTICE.md](NOTICE.md), *Trademarks and branding*).
* Acceptable use: [ACCEPTABLE-USE.md](ACCEPTABLE-USE.md) (guidance, not a licence term). Security reports: [SECURITY.md](SECURITY.md). Code contributions are not accepted at present: [CONTRIBUTING.md](CONTRIBUTING.md).

## Install (Linux)

SUDOSOC ARGUS targets Linux (Debian/Ubuntu/Kali, Fedora/RHEL, Arch, openSUSE, Alpine; amd64 and arm64). One command does everything:

```bash
git clone https://github.com/sudosoc/ARGUS && cd ARGUS
./install.sh                  # system-wide: /usr/local/bin/argus (uses sudo when not root)
./install.sh --user           # or: no root needed, installs into ~/.local
./install.sh --auto-update    # optional: also keep it updated automatically (daily)
```

(If the executable bit was lost, use `bash install.sh`.) The installer is safe to re-run at any time; it repairs or completes an installation. It:

1. installs the **OS packages**: curl, git, tar, python3 + venv, whois, dig, exiftool, sqlite3, whatweb, a Chrome/Chromium for `-browser` and PDF reports, and the build libraries the Python tools need;
2. installs **Go 1.26+** if missing (checksum-verified from go.dev) and builds a static binary stamped with the commit it was built from;
3. installs the **helper tools** into private virtualenvs under `share/argus/tools/` (nothing touches the system Python): `maigret`, `sherlock`, `holehe`, plus the `phoneinfoga` release build (checksum-verified) and `subfinder`; `theHarvester` where the distribution packages it (Debian/Kali);
4. adds the install directory to your `PATH` if needed, records how it was installed (`install.conf`, used by the updater), and finishes with `argus -doctor`.

Options: `--user`, `--system`, `--prefix DIR`, `--no-deps` (skip OS packages and helper tools), `--no-extras`, `--no-browser`, `--no-path`, `--auto-update[=daily|weekly]`, `--doctor`, `--uninstall`. Installing from a pipe works too: `curl -fsSL https://raw.githubusercontent.com/sudosoc/ARGUS/main/install.sh | bash -s -- --user` clones the source first.

**Check what you have:**

```bash
argus -doctor           # lists every optional tool: found (path) or missing (what it adds, how to get it)
```

ARGUS works without any of the helper tools; each one adds depth (see *Modules*). Missing tools are reported as coverage gaps, never as "nothing found".

### Updating

```bash
argus -update           # from anywhere; same as ./update.sh in the source directory
./update.sh --check     # only say whether a newer version exists
./update.sh --enable-auto weekly   # update automatically (systemd timer, or cron when there is no systemd)
./update.sh --status    # version, install mode, schedule, last log lines
```

The updater pulls the latest source (fast-forward only), rebuilds, reinstalls with the same mode and options as the original install, upgrades the helper tools, verifies the new binary and runs the doctor. If the build fails, the previous version stays installed and the source is put back. Local changes in the source tree are stashed for a manual update and make the scheduled update skip itself. `--fast` skips the OS-package step (the scheduled job uses it). The log is `~/.local/state/argus/update.log`. Your reports, saved cases, proxy lists and custom site files are never touched. Turn automatic updates off with `./update.sh --disable-auto`; `./install.sh --uninstall` removes everything the installer put in place.

Manual build instead (needs Go 1.26+):

```bash
make build              # bin/argus
make linux-arm64        # cross-compile, e.g. for a Raspberry Pi
```

## Usage

```bash
argus -u johndoe -verify -o reports
argus -n "Jane Doe" -e jane@example.com -p +15551234567 -derive -o reports
argus -d example.com -c "Example Inc" -l "Cairo, Egypt" -o reports
argus -ip 203.0.113.10
argus                 # on a terminal with no flags: interactive prompts
argus -list-modules
argus -version
argus -doctor            # which optional tools are installed
argus -update            # pull the latest version and rebuild
```

### Web UI

```bash
argus -web 127.0.0.1:8080
```

It prints a URL containing a random access token; open it in your browser (the token is exchanged once for an HttpOnly, SameSite=Strict cookie).

**Case panel (left).** Fill in whatever you know: every input, collector and option has an **ⓘ** hint (English and Arabic) explaining what turning it on will do. *Aggressive* is on by default; untick it for a quicker, lighter run. Proxies are optional.

**Navigation.** Twelve main views run along the top (Overview, Identity, Footprint, Infrastructure, Media, Graph, Location, Timeline, Exposure, Findings, Evidence, Reports). Each has a left rail of sections with live counts: for example Identity → Profile, Aliases, Usernames, E-mails, Phones, Digital identifiers, Verification; Evidence → Source registry, Evidence library, Source reliability, Confidence, Timestamps, Provenance, SHA-256 hashes, Coverage, Missing data, Validation, Conflicts, Duplicates, Confidence score; Reports → Investigation summary, Technical, Intelligence and Evidence reports, Exports, Export history. Sections with nothing yet are tucked under "Not available yet" and say what input would fill them. Nothing is hidden: every section is computed from the same findings.

**Look and feel.** A restrained dark workstation (black, charcoal, off-white, light gray and two blood reds; no neon) with a matching light theme, a command palette (`Ctrl+K` or `/`: search the case or run any command), `?` for all keyboard shortcuts, `t` theme, `d` density, `1`-`9` main views, and a print stylesheet. The HTML report uses the same palette.

**Workspace (right).** Live progress per collector, then eight views of the same data:

| View | What it gives you |
|---|---|
| **Identity** | Who the findings add up to: the best-matching person as a dossier with confirmation badges, other candidates kept apart, "search again with these details" |
| **Overview** | KPI tiles, key facts about the subject (names, handles, e-mails, phones, domains, IPs, places), notable risk signals (expired TLS, spoofable mail, takeover candidates, stealer-log exposure…), a reliability distribution, findings per collector |
| **Intelligence tree** | Everything organised as Subject → Identity / Contact points / Online accounts / Infrastructure / Organisation / Exposure / Crypto / Search leads, searchable, click any leaf for its evidence |
| **Relationship graph** | Force-directed map of how leads were found: subject → inputs → what the first pass found → what following those leads found. Drag, zoom, click nodes, hide weak leads |
| **Findings** | Paginated, sortable table with collector / category / status / confidence facets, filter chips, group-by, row expansion, multi-select and CSV / JSON export of the selection; scales to thousands of rows |
| **Location** | Where the subject probably is: geocoded evidence clustered on a map, EXIF GPS, activity-hours time zone |
| **Photos** | Gallery of every profile picture found, with "same picture" groups, size/hash, and reverse-image-search links |
| **Coverage & integrity** | Per-collector results, **every check that could not complete** (a coverage gap is not proof of absence), run settings, and a SHA-256 digest of the evidence |

History is kept in memory (last 50 cases); add `-o DIR` to also save every finished report. Exports (JSON / CSV / TXT) include entities, gaps, settings and the integrity digest.

- Binds to loopback by default. On a headless server, use an SSH tunnel: `ssh -L 8080:127.0.0.1:8080 user@server`, then open the printed URL locally.
- Binding to a non-loopback address (e.g. `0.0.0.0:8080`) works but is plain HTTP; put it behind a TLS reverse proxy and keep the token secret. Use `-web-token` to set your own token (16+ characters).
- Protections: required token, Host-header check (anti DNS-rebinding), same-origin + custom-header check on POST (anti CSRF), strict Content-Security-Policy, no external assets, all scraped text rendered as text (no HTML injection).
- The UI exposes only a safe subset of options. Site-list paths, external tools and tools files stay under command-line control (`-sites`, `-tools`, `-tools-file`); `-workers`, `-timeout`, `-proxy` etc. set the defaults for UI scans. `-web-max-scans` limits concurrent scans (default 2).

### Reliability

- **Coverage gaps are first-class.** Blocked, timed-out and offline checks are recorded per collector and shown in the UI and the TXT/JSON reports.
- **Retry pass.** Username probes that fail at connection level (timeouts, DNS) are retried once, gently, before being reported, so a congested link does not silently lose coverage. `-inflight N` caps simultaneous requests (default 110; lower it on weak networks).
- **Site self-test.** `argus -selftest-sites` checks every platform against its known account and a random one, then disables the ones with stale signatures or that report everything as existing (sites merely blocked from your network stay enabled). The list is stored in `$ARGUS_DATA` / `~/.local/share/argus/sites-disabled.json`; `-all-sites` ignores it, deleting the file resets it. Run it occasionally: platforms change.
- **Attribution scoring** keeps unrelated people who share a handle out of the leads that get followed.
- **Integrity digest** (SHA-256 of target, entities and findings) and recorded run settings make a report verifiable and reproducible.

### Nothing a tool prints is dropped

- **Raw output is always kept.** Every external tool run (`maigret`, `sherlock`, `holehe`, `phoneinfoga`, `whois`, `dig`, `theHarvester`, `subfinder`, `whatweb`, custom tools) is stored whole, unparsed, in the report (`tool_runs`), shown in **Coverage → Raw tool output** (with copy and download), written as one file per run in a `<report>.tools/` folder when you export, and loaded into the SQL export (`tool_runs` table). So even where a parser extracts only part of what a tool printed, the original text is there.
- **Every username-site check is listed**: *social: every check* is a full table of site, username, outcome (found, not found, blocked, inconclusive, failed) and HTTP status, so inconclusive and blocked responses are not silently discarded.
- **No result caps.** Parsers no longer limit the number of items, subdomains and fields; Maigret runs on **all** its sites with recursion on and every field it extracts is kept; holehe runs without `--only-used` and reports registered, not registered and rate-limited sites (an unknown result is a coverage gap, not silence); coverage gaps are never cut off (CLI, HTML and UI list all of them).
- **Collectors are not cut short either.** Every username and alias is probed by `profiles` (not just the first three); API page sizes are raised to the largest the services allow (ORCID, Wikidata, Crossref, Bluesky, Mastodon, GitHub repos and commits…); long text values are kept to 5,000 characters; co-hosted domains, ASN prefixes, ad-network accounts, pictures (up to 1,000), archive snapshots (30), documents (100), look-alike candidates (1,500) and resolved subdomains (3,000) are no longer capped at a few dozen; the identity view keeps up to 60 candidates.
- **Honest limits.** A tool's output is captured up to 64 MiB; beyond that the run is flagged *capture limit reached*, never cut quietly. Sites behind anti-bot systems a plain client cannot pass are not probed (ARGUS does not evade protections); `-v` keeps the not-found and error records in exports as well.

### Files, a place and a period as inputs

Besides identifiers, an investigation can start from **files**, a **place** or a **period** (web form: the *Files*, *Period* and *Coordinates* fields; CLI: `-file`, `-coords`, `-date-from`, `-date-to`).

- **Pictures** (JPEG, PNG, GIF, WebP, TIFF…): EXIF, GPS, camera, SHA-256 and perceptual hash, OCR of the text printed in them, and reverse-search links (Google Lens, Yandex, TinEye, Bing, SauceNAO). They join the normal photo analysis: "same picture" groups and GPS on the map.
- **Documents** (PDF, Word, Excel, PowerPoint, OpenDocument, text): hidden metadata (author, last editor, organisation, software, dates), the text, and the e-mails, phone numbers, domains, links and wallets inside it. A CV also yields a name, employers and a place. Every lead found is followed like any other.
- **Videos** (MP4, MOV, MKV, WebM, AVI): needs `ffmpeg`. Up to 12 frames are taken across the video and each is analysed like a picture (OCR, fingerprints); `ffprobe` reads the container metadata (creation time, device, GPS location tag).
- **Place** (`lat,lon`): the place's name (OpenStreetMap), geotagged pictures within 1 km (Wikimedia Commons), landmarks within 2 km (Wikipedia), and links to open map, street-photo and social searches. ARGUS does not query a social network for people near a point.
- **Period** (`YYYY-MM-DD` to `YYYY-MM-DD`): news in the window that names the subject (GDELT), the Internet Archive copy of the domain closest to it, what happened on that day, and searches limited to the period (Google, X). Posts on social networks cannot be searched by date without an account, so they are links for you to open.

Files are read on your machine only; nothing is uploaded to any service. Limits: 100 MB per picture or document, 500 MB per video, 20 files per investigation. OCR reads English and Arabic by default when the Arabic data is installed (`ARGUS_OCR_LANGS` changes it). Example: `argus -file contract.pdf -file photo.jpg -coords 30.0444,31.2357 -date-from 2026-01-10 -date-to 2026-01-12`.

### CLI

At least one identifier is required. Ctrl-C stops the run and still writes a partial report (exit code 130).

Other flags worth knowing (`argus -h` lists all):

| Flag | Effect |
|---|---|
| `-list-modules`, `-version` | list the modules / print the version and exit |
| `-v` | keep not-found and error records in exported reports |
| `-no-color` | plain console output |
| `-pivot-depth N`, `-max-pivots N` | rounds of following discovered leads (default 3; `-aggressive` 4) / leads followed (default 30; `-aggressive` 80) |
| `-retries N`, `-tool-timeout D` | retries on transient errors (default 1) / cap on any external tool's runtime |
| `-proxy-file F` | one proxy URL per line, rotated |
| `-insecure` | skip TLS verification (use only for broken targets you are allowed to test) |
| `-no-wayback` | skip the Internet Archive query in the domain module |
| `-dork-engines`, `-dork-delay`, `-max-dorks` | dorking engines in order (default `brave,bing,duckduckgo,google`), per-engine delay (3s), dork cap (default 200; `-aggressive` 500) |
| `-site-tier core\|extended\|all` | how deep into the username-site list to go (default `all`) |
| `-selftest-sites` | validate every site signature, see *Reliability* |

## Inputs

All optional; each one you add unlocks more sources:

| Input | Flag | Unlocks |
|---|---|---|
| Full name | `-n` | search dorks, Wikidata / ORCID / OpenAlex / GitHub name search |
| Username | `-u` | 500+ site probe, deep profile data (GitHub, Keybase, npm, GitLab, Mastodon, …) |
| Email | `-e` | MX/provider, Gravatar profile, stealer-log exposure, verified web mentions; its domain is investigated too |
| Phone | `-p` | offline analysis, verified web mentions |
| Domain / URL | `-d` | DNS, e-mail security, RDAP, subdomains, website, TLS, archive, exposure |
| IP address | `-ip` | rDNS, ASN, RDAP, geolocation, open ports/CVEs (passive), co-hosted domains, TLS banner |
| Company | `-c` | likely websites (verified), Wikipedia/Wikidata, GitHub org, company dorks |
| Nickname / alias | `-a` | handle variants (`dark.knight`, `darkknight`, …) probed on every platform, plus alias dorks |
| ASN | `-asn` | owner, announced prefixes, BGP neighbours, registration contacts |
| Crypto wallet | `-w` | Bitcoin / Ethereum balance, activity window, ENS name, scam-report dorks |
| Location | `-l` | refines name/company searches (not an identifier on its own) |

A pasted **profile link** in the domain field (`-d https://x.com/handle`) is recognised and treated as a username, not a domain. A phone number without `+` is parsed using the country in `-l`.

**One clue is enough.** Give only a phone number, a nickname, a wallet or a profile link and every module that can use it runs; whatever it finds (emails, domains, IPs, usernames) is followed automatically. Use `-aggressive` (a checkbox, on by default, in the web UI) for maximum coverage: derived usernames, verified hits, pivot depth 3 and up to 25 followed leads, 120 dorks.

## Modules

| Module | Needs | What it does |
|---|---|---|
| `social` | username / alias | Probes **10,600+ platforms** from the WhatsMyName, Sherlock and Maigret lists (embedded, merged and de-duplicated) in three tiers: `core` (about 1,100 popular, well-maintained sites), `extended` (about 3,200) and `all` (the default: adds about 7,400 template-based forums and small communities, so nothing is left out). Several markers per outcome, redirect checks, username patterns and anti-bot error markers; adult sites are opt-in with `-categories nsfw`; adds page title / bio / avatar when present. |
| `phone` | phone | libphonenumber analysis: validity, country, region/area, line type (mobile / landline / VoIP), original carrier, time zones, all formats. |
| `crypto` | wallet | Bitcoin (mempool.space) and Ethereum/EVM (Blockscout): balance, tx counts, activity window, contract status, ENS name. |
| `asn` | ASN | RIPEstat + RDAP: holder, announced prefixes, BGP neighbours, contacts. |
| `profiles` | username | Real profile data from GitHub (profile, SSH keys, **commit e-mails**), Keybase (verified accounts), GitLab, npm (maintainer e-mail), Hacker News, DEV, Docker Hub, Mastodon, Stack Overflow, Lichess, Chess.com, Reddit. |
| `domain` | domain | DNS records; SPF/DMARC/DKIM and mail provider; SaaS used (TXT tokens); RDAP registration and contacts; **subdomains** (crt.sh, certspotter, HackerTarget, TLS SANs) resolved and checked for wildcard DNS; website title/tech/security headers/cookies/tracking IDs; emails, phones and social links on the site; robots.txt, sitemap, security.txt; TLS certificate; Internet Archive URLs; stealer-log exposure counts. |
| `ip` | IP | Reverse DNS, ASN (Team Cymru via DNS), RDAP, geolocation, ports/CVEs already indexed by Shodan InternetDB (no scanning by ARGUS), co-hosted domains, TLS certificate and web banner. |
| `company` | company | Candidate domains (verified against the live page; only the best one is followed), Wikipedia, Wikidata, GitHub organisation. |
| `mentions` | full name | Press articles (GDELT open news index, last ~year) whose headline names the person; headlines that also contain your company, place or e-mail domain are marked as context matches, the rest stay unverified. GDELT rate-limits shared IPs, so it can answer "try again later". |
| `sitefiles` | domain | What a site publishes about itself and its people: humans.txt credits, ads.txt advertising accounts (link a site to its owner and sister sites), the public WordPress author list, RSS / Atom feed authors. |
| `bizreg` | company | Official business registers with open APIs: France (number, address, activity, officers) and Norway (number, address, activity, staff). |
| `devtrace` | username, e-mail | Public developer traces: names and e-mails in an account's recent public commits, e-mails on GPG keys (verified ones flagged), public organisations, main languages and topics, npm / RubyGems maintainer details, PGP keyserver identities for an e-mail. GitHub allows 60 anonymous requests/hour. |
| `docmeta` | domain, document links | Hidden metadata in PDF / Word / Excel / PowerPoint / OpenDocument files the site links to: author, last editor, organisation, software, dates. Often exposes real staff and login names. |
| `records` | full name, domain, IP | SEC EDGAR filings naming the person (needs `-contact you@example.com`, which the SEC requires and which is sent only to sec.gov) and urlscan.io history of a domain or IP. |
| `images` | name or username | Pictures of the subject: avatars of the handle on ~18 services (via unavatar.io) and web image search for the quoted name, filtered to pages whose title has the name. Always shown as unverified candidates, separate from profile photos. |
| `people` | full name, LinkedIn link or pasted text | Candidate profiles as structured records for identity resolution: LinkedIn public view (when allowed), ORCID, Wikidata, OpenAlex, Crossref authors, Wikipedia, GitHub, Fediverse WebFinger, Wayback snapshots of profile links, CV documents, pasted text. Name lookups also cover arXiv, Europe PMC, INSPIRE-HEP, the French company register (officers), MusicBrainz and the wider Stack Exchange network. Username lookups add Lobsters, Duolingo, Launchpad, Hex.pm, MetaCPAN, Kitsu, Minecraft, LeetCode, Codewars, Matrix, Gitea, Wikimedia Commons, 13 link-in-bio hubs and ~27 creative and community platforms. Names match across spellings and scripts (Mohamed / Muhammad / محمد, Bob / Robert). |
| `breach` | e-mail | Which known data breaches the address appears in (XposedOrNot public index): breach names, dates and data types only, never the data itself. |
| `activity` | username | Estimates the owner's time zone from the hours the handle is active on GitHub, Hacker News, Wikipedia, Codeforces, Bluesky, Mastodon and Lichess; shown in the Location tab. |
| `footprint` | email or phone | MX, Gravatar profile (name, linked accounts, sites), infostealer exposure (counts/dates only), phone analysis, and web mentions that are verified by fetching each page. |
| `dorks` | any identifier | Generates dorks from every identifier (site:, filetype:, inurl:, leaks, code, people…) and scrapes Brave / Bing / DuckDuckGo / Google. |
| `iddecode` | pasted profile / post links | Creation time hidden in the IDs of X/Twitter, Discord, TikTok, Instagram, Bluesky, LinkedIn and Mastodon links (offline arithmetic, no request): dates a post, video or account. |
| `keys` | e-mail | The OpenPGP public key published for the address (Web Key Directory, keys.openpgp.org, Proton): fingerprint, algorithm, creation date and the names and other e-mail addresses on its user IDs. |
| `lookalike` | domain | Registered look-alike domains of the target's domain: typos, keyboard neighbours, bit flips, IDN homographs and phishing-style keywords, with addresses and whether they can receive mail. |
| `webhistory` | domain | Analytics, advertising and pixel IDs in archived copies of the site across the years (Internet Archive); an ID that appears on old and new sites ties them to one operator, one that vanishes marks a change of owner. |
| `sanctions` | full name, company | Name screening against the OFAC SDN and UN Security Council consolidated lists (no account). A match is a lead: compare date of birth, nationality and programme. |
| `bskygraph` | username | Bluesky social circle from public follow lists: mutual follows and accounts that many of the subject's follows follow but the subject does not. |
| `codesearch` | e-mail | Public source code that contains the exact address (package metadata, configs, docs, scripts) through grep.app's public endpoint; repository owners become weak leads. |
| `uploads` | uploaded files | Pictures, documents and videos you supply (see *Files, a place and a period as inputs*): metadata, OCR and text, video frames, and the leads inside become findings. Needs no network. |
| `context` | coordinates, dates | A place and a period: place name and nearby geotagged pictures and landmarks (OpenStreetMap, Wikimedia Commons, Wikipedia), news in the period (GDELT), the closest Internet Archive copy of the domain, what happened that day, and ready-made map, photo and period-limited searches. |
| `syswrap` | domain, username | Runs `whois`, `dig`, `theHarvester`, `subfinder`, `whatweb`, `sherlock`, `maigret`, `holehe`, `phoneinfoga` and your custom tools through `os/exec` (those installed). Maigret is read from its own JSON report (names, bios, pictures, linked usernames). |
| `spiderfoot` | domain, e-mail, username, phone | Optional: runs the SpiderFoot CLI (`-u passive`) when installed and keeps every event it returns, grouped by type with the SpiderFoot modules that produced it. |
| `h8mail` | e-mail | Optional: runs h8mail when installed. Only the breach/source names and the number of records are shown; credential values are never stored or displayed. |
| `facebook-scraper`, `instagram-osint`, `insta-osint-engine`, `open-nami`, `the-3rd-eye` | username | Optional social-media collectors, each a separate program you install (`facebook-scraper`, `igosint`, `insta-osint-engine`, `nami`, `the3rdeye`). Their command lines differ between versions, so override with `ARGUS_TOOL_<NAME>_BIN` and `ARGUS_TOOL_<NAME>_ARGS` (for example `ARGUS_TOOL_OPEN_NAMI_ARGS="-u {username}"`). The complete output is kept; URLs, e-mails and phones in it are listed. `the-3rd-eye` runs only when the Google API key is enabled in **Settings**. Use only on public accounts you may examine and within each platform's terms. |

Select modules with `-modules domain,ip`. Modules whose required input is missing are skipped and listed in the summary.

### Photos

Profile pictures from ~22 sources (and Gravatar by e-mail) are downloaded by ARGUS, fingerprinted (SHA-256 + perceptual hash) and compared. The same picture on different sites is reported as a finding and highlighted in the **Photos** tab, with reverse-image-search links (Google Lens, Yandex, TinEye, Bing) per picture. If `tesseract` (or `easyocr`) is installed, text printed in a picture (signs, badges, screenshots) is read and shown with the picture and in Media → Metadata; English and Arabic are used by default when the Arabic data is installed (`ARGUS_OCR_LANGS` changes it). Downloads go through an SSRF-guarded fetcher; the web UI shows them through an authenticated proxy so your browser never contacts third-party hosts. No face recognition is performed.

### What is covered, and what is not

See [docs/COVERAGE.md](docs/COVERAGE.md) for the per-identifier map of sources and limits. Anything that needs a login, CAPTCHA or payment is not automated; the report lists those sources as **manual leads** with pre-filled links.

### Identity resolution: different people stay different

A name or a handle is not an identity. Every profile-like finding (GitHub, Keybase, ORCID, Wikidata, OpenAlex, LinkedIn's public view, personal sites, CVs, Gravatar, …) becomes a **record**, and records are clustered into **identities** only on evidence:

- a shared e-mail, phone, unique ID (ORCID, Wikidata…), linked account or website, or the same profile picture;
- or the same name plus at least two independent matching attributes (place, employer, school, e-mail domain, the same handle on another platform).

Two different accounts on the same platform are never merged by soft evidence (impersonators copy names, places and employers); copies and duplicates are demoted. Each identity is **scored against what you supplied** (name, e-mail, phone, username, domain, company, location, pasted profile link), contradictions lower the score, and every attribute is labelled: ✔ *confirmed* (two or more independent platforms), *self-reported* (the person's own profile) or *1 source*. The **Identity** tab shows the best match as a dossier (photo, role, employers, education, interests, e-mail, accounts, websites, CV), the other candidates separately, and a banner when several people plausibly match, with a button to search again using a candidate's details. Only the best-matching identity's e-mails, accounts and websites are followed automatically; when it is ambiguous, none is.

Sources for a name: ORCID (employments, education, keywords, links), Wikidata (occupation, employer, education, cross-platform IDs, photo), OpenAlex (institutions, research topics), GitHub profiles, personal sites (schema.org Person data) and CV/resume PDFs. A CV is accepted only if its *header* carries the person's name (a CV that merely mentions the name is rejected) and is checked against your e-mail, phone, employer, location and handle.

**LinkedIn.** LinkedIn actively blocks automated access (HTTP 999). ARGUS can read the logged-out public view of a specific profile URL when LinkedIn allows it, slowly and without evading anything, and stops at the first refusal. When LinkedIn refuses, use the manual link, or open the profile yourself and paste its text (or a "Save to PDF" export) into the **Paste text** box (CLI: `-notes-file`); it is parsed into a record and takes part in identity matching. LinkedIn's terms restrict automated access; use this only where you are entitled to.

**`-browser` (opt-in).** If plain HTTP is refused, ARGUS can open the same logged-out public page with the Chrome / Chromium / Edge installed on your machine (headless, default settings, empty profile; set `ARGUS_BROWSER` to pick one). It uses no stealth flags, no cookies and no login, and stops at a sign-in wall or CAPTCHA. Whether LinkedIn serves the page depends on your IP and its current rules; in our own test from a home connection it showed the sign-in wall, so treat this as best effort. The public view contains name, headline, location, roles, schools, about and photo; it never contains e-mail or CV, which LinkedIn shows only to logged-in connections. For those, paste your own logged-in view into the Notes box.

## Report formats

`-formats json,csv,txt,html,pdf,graphml,stix,entities,misp,spiderfoot,maltego,md,sql` (web UI: export buttons). `md` is one Markdown note with front matter and `[[wikilinks]]` for Obsidian / Logseq; `sql` is a SQLite script (`sqlite3 case.db < case.sql`, then `datasette case.db` to browse and query every finding). `html` is a single self-contained file with the identity dossier and embedded photos; open it and print to PDF. `graphml` opens in Gephi, yEd and Maltego; `stix` is a STIX 2.1 bundle; `entities` is a CSV of every discovered entity. `pdf` needs Chrome, Chromium or Edge on the machine. `misp`, `spiderfoot` and `maltego` are import files for those tools.

## Which sites know an e-mail address

With an e-mail, the `syswrap` module runs [holehe](https://github.com/megadose/holehe) (installed by `install.sh` into its own virtualenv) and reports the sites where that address has an account. holehe works by touching each site's sign-up or password-reset form; it is built not to alert the owner, but it does contact those sites, so run it only on addresses you are entitled to check. Without holehe installed the module says so. Pair it with the `breach` module (which leaks an address appeared in) and Gravatar to see an address's footprint.

## Web UI at a glance

The Identity tab opens with one hero card (photo, name, match score, sources, confirmed facts). Every fact carries a trust colour: green = stated by 2+ platforms, amber = the person's own profile or a reliable source, grey = a single third-party source. By default only confirmed and self-reported facts are shown; **Show every detail** reveals the rest. Rows where sources disagree are outlined in red with a **See who says what** button. Other candidates appear as cards side by side; tick two and press **Compare**. Type in the search box (or press `/`) to search every finding, entity, person, place, timeline event and note at once. Add **notes** to a person or to any finding; they are saved with the case and printed in the TXT, HTML and PDF reports. The Location tab shows a weekday × hour activity heatmap (local time or UTC); the Timeline is a horizontal axis; the Relationship graph can be filtered by entity type, coloured by collector, searched, and focused on a node's neighbours. The theme switch (auto / light / dark), collapsible input groups and your option and collector choices are remembered in this browser; the subject's details never are.

## Cases, comparison and source status

Every finished scan in the web UI is saved as a case (`~/.config/argus/cases`, or `$ARGUS_CASES`, files mode 0600). **Saved cases** re-opens one and compares any two runs of a target to show what appeared or disappeared. **Source status** probes the ~23 public services ARGUS relies on, so a blocked source is not mistaken for "nothing found". On any candidate person, **Compare with best match** shows the two side by side. **📋 Copy page for ARGUS** is a bookmarklet: you open a profile yourself while logged in, click it, and paste the copied text into the Paste text box; ARGUS never logs in anywhere.

### Pivoting: findings feed each other

Modules report **entities** (emails, domains, IPs, usernames, names, phones) they discover. After the first pass the engine follows the *strong* ones once (up to `-max-pivots`, default 10): a found email goes to `footprint`, a domain to `domain`, an IP to `ip`, a username to `social` + `profiles`. Results of that second pass are tagged `via email:…` etc. Disable with `-no-pivot`; `-modules` restrictions also apply to the second pass.

### Attribution: a username is not an identity

`torvalds` on npm or Docker Hub is usually someone else. The `profiles` module therefore scores every profile before anything is followed: against the identifiers you supplied (name, email, domain, company, location), against profiles on other platforms (shared name / website / location), and for internal consistency (a commit e-mail that matches the handle). Profiles that don't reach the threshold are reported as **unverified, 40% confidence** and their leads are *not* followed. Supplying more identifiers (even just a full name) raises confidence for genuine matches.

### Data sources and what "API-less" means here

No API keys or accounts are used anywhere. Besides scraping and DNS, the tool reads a handful of **public, unauthenticated web endpoints** that sites serve to anyone: RDAP, crt.sh / certspotter / HackerTarget (subdomains, reverse IP), Internet Archive CDX, Gravatar, GitHub / GitLab / Keybase / npm / Hacker News / Mastodon / Lichess / Chess.com / DEV / Docker Hub / Stack Exchange public profile data, Wikipedia / Wikidata / ORCID / OpenAlex, Shodan InternetDB, ipwho.is, Team Cymru (DNS) and Hudson Rock's free stealer-index counts. Several are rate-limited per IP or go offline at times; failures are reported as errors and never stop a run.

### Username sites: sources, tiers and refreshing

The binary embeds one merged list of 10,600+ sites built from [WhatsMyName](https://github.com/WebBreacher/WhatsMyName) (CC BY-SA 4.0), [Sherlock](https://github.com/sherlock-project/sherlock) (MIT) and [Maigret](https://github.com/soxoj/maigret) (MIT) by `cmd/sitesync`. Where two lists describe the same check the better-maintained signature wins and the other list contributes its popularity rank and known accounts. Sites behind anti-bot systems a plain client cannot pass (JavaScript challenges, TLS fingerprinting, logins) are left out: ARGUS does not evade protections.

| `-site-tier` | Sites | Use |
|---|---|---|
| `core` | about 1,100 | quick checks; popular, well-maintained platforms |
| `extended` (default) | about 3,200 | normal investigations |
| `all` | about 10,600 | adds thousands of template-based forums and small communities; slow, more noise |

`-categories` accepts the platform categories and Maigret's tags (`coding`, `gaming`, `forum`, `ru`…). `-sites FILE` loads your own list (WhatsMyName JSON, a bare array, or `.gz`); a `sites.json` in the data directory (`$ARGUS_DATA`, `$XDG_DATA_HOME/argus`, `~/.local/share/argus`, `/usr/local/share/argus`) overrides the embedded list.

WhatsMyName stopped maintenance on 2026-11-09 and platforms keep changing, so run `argus -selftest-sites` regularly: it disables sites whose signature is stale or that report everything as existing (CI runs it weekly). To rebuild the embedded list: `sh scripts/update-sites.sh` (runs `go run ./cmd/sitesync -fetch`, records source hashes in `data/sites/sources.lock.json`).

Site signatures drift as platforms change. Use `-verify` and treat results as leads.

### Custom syswrap tools

`-tools-file` takes a JSON array (see `configs/tools.example.json`):

```json
[{
  "name": "holehe", "binary": "holehe",
  "args": ["--only-used", "{email}"],
  "pattern": "^\\[\\+\\]\\s+(?P<title>\\S+)",
  "category": "email", "timeout_sec": 90
}]
```

`{username}`, `{email}`, `{domain}` and `{phone}` are substituted into args and also decide when the tool runs. `pattern` is matched per output line; the named groups `title` and `url` feed the finding and any other named group goes into its details. Commands run directly, never through a shell, and inputs that don't pass validation (leading `-`, spaces, shell characters) are dropped.

## Architecture

```
cmd/argus/            CLI: flags, wiring, signal handling, export
internal/
  models/               Target, Finding, Report (shared data types)
  scanner/              Scanner interface + Deps handed to every module
  app/                  Builds a configured engine from one Options value (CLI and web share it)
  web/                  Web UI: JSON API, SSE live results, auth middleware, embedded static/ front end
  engine/               Planner (Enrich/Plan) and Run: dispatch, collect, report
  pool/                 Bounded worker pool shared by all modules
  httpx/                HTTP client: UA rotation, header spoofing, proxies, retry
  output/               Coloured console, JSON / CSV / TXT export
  modules/
    social/             Username enumeration (+ embedded sites_default.json.gz, 10,600+ sites in tiers)
    iddecode/ keys/ lookalike/ webhistory/   Platform-ID timestamps, OpenPGP keys, look-alike domains, tracking-ID history
    sanctions/ bskygraph/ codesearch/        Sanctions screening, Bluesky social circle, public code search
    profiles/           Deep profile data + attribution scoring
    domain/             DNS, RDAP, subdomains, website, TLS, archive, exposure
    ip/                 rDNS, ASN, RDAP, geo, passive ports/CVEs, banner
    company/ people/    Organisation and name look-ups
    dorks/              Dork generation, search-engine scrapers, shared Searcher
    footprint/          Email / phone footprint
    syswrap/            External CLI tool wrappers
  identity/             Identity resolution: records, clustering, scoring, name matching/transliteration
  geo/                  Location analysis: geocoding, evidence clustering, time-zone inference
  media/                Safe (SSRF-guarded) picture download, SHA-256 and perceptual hashing
  browser/              Opt-in headless Chrome/Chromium fetch of public pages (-browser)
  cvparse/ jsonld/      CV text parsing; schema.org Person extraction
  leads/ health/        Manual-lead links per identifier; liveness probes of the public services used
  util/ openpgp/        Shared helpers (JSON fetch, clipping, cleaning); OpenPGP public-key reader and WKD hashing
  sitesync/             Converters + merge for the WhatsMyName / Sherlock / Maigret site lists (cmd/sitesync)
  extract/ probe/ rdap/ intel/   Shared helpers: HTML extraction, web/TLS probes, RDAP client, normalisation
cmd/sitesync/           Builds the embedded site list
configs/  data/  scripts/  .github/workflows/ (CI: gofmt, vet, race tests, arm64 build, weekly site self-test)
```

**Run flow**

1. `engine.Enrich` validates and normalises inputs and derives alias usernames (email local part; name permutations with `-derive`).
2. `engine.Plan` asks each registered module `CanRun(target)` and honours `-modules`.
3. `engine.Run` starts the selected modules concurrently. They push work onto one shared `pool.Pool`, so total in-flight tasks stay at `-workers` however many modules run.
4. Modules call `emit(Finding)`. One collector goroutine counts and stores them, and streams positive hits to the console.
5. The report is sorted (found first), summarised and exported.

**Concurrency rules for modules.** A module's `Scan` runs on its own goroutine, not on a pool worker, so it may submit tasks to `d.Pool` and `wg.Wait()` for them without deadlocking. Tasks must not submit further tasks to the pool. Tasks must honour `ctx`, and `emit` is safe for concurrent use.

**Stealth and evasion.** Each request gets a rotated browser User-Agent, a navigation header set and a random Accept-Language. There is retry with jittered backoff and optional proxy rotation. TLS and HTTP/2 fingerprints are Go's own. Search engines that serve a bot challenge are disabled for the rest of the run; challenges are never solved or bypassed.

## Adding a module

1. Create `internal/modules/<name>/` with a type implementing `scanner.Scanner`:

   ```go
   type Scanner interface {
       Name() string
       Description() string
       CanRun(t models.Target) bool
       Scan(ctx context.Context, t models.Target, d scanner.Deps, emit func(models.Finding)) error
   }
   ```

2. In `Scan`, fan out with `d.Pool.Submit(ctx, fn)`, use `d.HTTP.Do(...)` for requests, call `d.Progress` if you want a progress bar, and emit `models.Finding` values. Use `StatusFound` / `StatusUncertain` for leads, `StatusNotFound` for negatives and `StatusError` for failures. Set `Confidence` honestly (a search hit is about 0.4, a verified match about 0.9).
3. Register it in `internal/app/app.go` (`Build`): `eng.Register(mymod)`. Both the CLI and the web UI pick it up; the engine needs no changes.
4. Add table-driven tests for any parsing. The existing modules test their parsers and matchers offline.

To share rate-limited resources between modules, pass one instance to both. `dorks.Searcher` is shared by `dorks` and `footprint` this way.

## Development

```bash
go vet ./... && go test -race ./...     # also what CI runs
sh scripts/update-sites.sh               # rebuild the embedded username-site list
```

Shared helpers (`util.GetJSON`, `util.Clip`, `util.Put`…) live in `internal/util`; use them instead of private copies. Give a new module a package-level base URL variable so its tests can point it at an `httptest` server.

## Limitations

- **Search engines.** Google and DuckDuckGo usually challenge plain-HTTP scrapers, and Bing may return unrelated pages; Brave works best. Blocked engines are skipped for the rest of the run. The dorks module reports this as errors, and footprint only keeps pages it could fetch and verify. A proxy via `-proxy` may help, but results are never guaranteed.
- **Site signatures.** Social checks depend on each platform's current responses and can produce false positives or negatives.
- **Breach data is names only.** The `breach` module queries the public XposedOrNot index and reports which breaches an e-mail appears in (names, dates, data types). `footprint` and ARGUS never download or store breach contents or credentials.
- **Offline phone analysis.** The calling-code table is partial and does not look up carriers or owners.

## Credits

The username site list merges [WhatsMyName](https://github.com/WebBreacher/WhatsMyName) (CC BY-SA 4.0), [Sherlock](https://github.com/sherlock-project/sherlock) (MIT) and [Maigret](https://github.com/soxoj/maigret) (MIT), see [NOTICE.md](NOTICE.md). Phone metadata comes from Google's libphonenumber via [nyaruka/phonenumbers](https://github.com/nyaruka/phonenumbers).
