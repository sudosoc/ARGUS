# Architecture

This page explains how SUDOSOC ARGUS is put together, so you can find your way around the code, change it, or add to it. For what the tool collects see [COVERAGE.md](COVERAGE.md); to add a collector see [WRITING-A-MODULE.md](WRITING-A-MODULE.md); to compare investigations see [COMPARING-CASES.md](COMPARING-CASES.md).

## The shape of it

```
 cmd/argus                one binary: CLI, web server, doctor, updater
    │
    ├── internal/app      builds an Engine: registers every module, wires proxies, keys, HTTP client
    │        │
    │        ▼
    │   internal/engine   plans which modules run, runs them, follows leads, then the analysis stages
    │        │                      │
    │        │                      └── internal/modules/*   one package per collector (30 of them)
    │        ▼
    │   models.Report ──► internal/output    HTML, JSON, CSV, Markdown, SQL, STIX, MISP, Maltego, GraphML…
    │
    └── internal/web      HTTP API + embedded browser UI (static/), cases, uploads, settings
```

Everything a run produces is one `models.Report`: the target, the findings, the entities, the people (identities), the photos, the timeline and location analysis, the manual leads, the raw tool runs and an integrity digest. The CLI prints it, the web server streams it and saves it as a case, and the exporters write it out. Nothing else is stored.

## Packages

| Package | Role |
|---|---|
| `cmd/argus` | Flags, the interactive prompt, `-web`, `-doctor`, `-update`, `-selftest-sites`. Builds `app.Options` and starts either a scan or the web server. |
| `cmd/sitesync` | Maintainer tool that regenerates the embedded username-site list from WhatsMyName, Sherlock and Maigret. |
| `internal/models` | The data types: `Target`, `Finding`, `Entity`, `Report`, `Identity`, `Photo`, `ToolRun`. No logic, so every other package can import it. |
| `internal/scanner` | The `Scanner` interface every module implements, and `Deps` (HTTP client, worker pool, progress callback). |
| `internal/engine` | Planning, running, pivoting and the post-collection stages (below). |
| `internal/app` | `Build(Options)`: creates the engine and registers modules in one place for both front ends. |
| `internal/modules/*` | The collectors. Each depends only on `models`, `scanner` and shared helpers. |
| `internal/httpx` | The one HTTP client: retries, proxy rotation, body caps, browser-like headers, per-host politeness. |
| `internal/pool` | The shared worker pool all concurrent work runs on. |
| `internal/identity` | Identity resolution: groups records into separate people and scores them against your inputs. |
| `internal/geo` | Geocoding (OpenStreetMap Nominatim) and location clustering. |
| `internal/media` | Safe picture download (SSRF-guarded), hashes, EXIF, OCR, `exiftool`. |
| `internal/files` | Analysis of uploaded pictures, documents and videos. |
| `internal/extract`, `internal/cvparse`, `internal/jsonld`, `internal/doc` | Text extraction: e-mails, phones, links, CV structure, structured data, PDFs. |
| `internal/intel`, `internal/rdap`, `internal/openpgp`, `internal/probe` | Domain/IP helpers, registration data, PGP keys, username-site probing. |
| `internal/insights` | The conclusions stage: brief, next actions, persona, authenticity, associates, hidden links, life timeline, movement and per-fact reasoning, by fixed rules (see [INSIGHTS.md](INSIGHTS.md)). |
| `internal/leads` | The pre-filled manual links for sources that need a login or payment. |
| `internal/output` | Every export format. |
| `internal/settings` | Encrypted API keys and the language/model choice. |
| `internal/web` | The server: JSON API, Server-Sent Events, uploads, cases, settings, the embedded UI. |
| `internal/doctor`, `internal/health`, `internal/sitesync` | Tool check, service reachability, site-list sync. |

## What happens in a run

`engine.Run(ctx, target)`:

1. **Enrich** (`planner.go`). Validate and normalise every input (domains, e-mails, phones, wallets, coordinates, dates), turn a pasted profile link into a username, derive aliases from the e-mail and name, and add the domain behind a non-public e-mail address.
2. **Plan.** Ask each registered module `CanRun(target)`. Modules that cannot run are listed as *skipped* with the reason, not hidden.
3. **Run the first pass.** Every chosen module's `Scan` runs on the shared pool. Modules report through `emit(finding)`; the engine stamps the module name and time, extracts entities from the details, and forwards each finding live (that is what the UI streams).
4. **Pivot.** The strong new entities the first pass found (an e-mail on a GPG key, a domain on a profile, a handle in a bio) become new targets, up to `-pivot-depth` rounds and `-max-pivots` leads. Only the best-matching identity's leads are followed automatically; if the match is ambiguous, none is. Each followed lead records `via` so the graph can show where it came from.
5. **Post-collection stages** (in this order, each a method on `Engine`):
   - `collectEntities`: the de-duplicated list of everything found.
   - `photoStage`: download every picture, hash it, group identical ones.
   - `identityStage`: resolve people, score them, mark confirmed facts.
   - `locationStage`: geocode every location clue, cluster, score, report conflicts.
   - `archiveStage`: Internet Archive first/last capture of profiles and the domain.
   - `cloneStage`, `timelineStage`: copies of pictures and the dated-event timeline.
   - `insights.Build`: the conclusions (brief, actions, persona…), stored in `Report.Insights`.
6. **Finish.** Manual leads, interrupted flag, settings used, the integrity digest (SHA-256 of target, entities and findings).

Everything that can fail is recorded: a check that could not run is a *failure* in the report (shown as a coverage gap), never silence. The complete output of every external tool is kept whole in `tool_runs`.

## Modules

A module is a Go package with a type implementing `scanner.Scanner` (see [WRITING-A-MODULE.md](WRITING-A-MODULE.md)). Modules never call each other; they only read the `Target` and emit `Finding`s. The engine decides when they run. The list lives in one place, `app.Build`, and is the same for the CLI and the web UI.

Two modules wrap other programs. `syswrap` runs external tools (`maigret`, `sherlock`, `holehe`, `theHarvester`, `subfinder`, `whatweb`, `h8mail`, SpiderFoot, the social-media collectors, anything in `-tools-file`) with `os/exec`, never through a shell, with validated arguments, a timeout, and optional API keys passed only in the environment. `uploads` and `context` do not touch the network for the files you supply.

## The web server

`internal/web` serves a small JSON API and the embedded UI.

- **Locked down by default.** Binds to loopback, needs a random access token (exchanged for an HttpOnly SameSite=Strict cookie), rejects foreign `Host`/`Origin`, requires an `X-Requested-With` header on writes, and sends a strict Content-Security-Policy (no inline script, no external assets).
- **Scans.** `POST /api/scans` starts one; `GET /api/scans/{id}/events` streams findings as Server-Sent Events; `GET /api/scans/{id}` returns the current state; exports at `/api/scans/{id}/export?format=…`.
- **Cases.** A finished scan is saved as `<started>_<random>.json` under the user's config directory (`~/.config/argus/cases`, or `ARGUS_CASES`), mode 0600. `GET /api/cases` lists them; `GET /api/cases/{id}` returns a report; `POST /api/cases/{id}/open` re-opens one.
- **Uploads.** `POST /api/uploads` reads the file body, analyses it locally (`internal/files`) and returns the analysis; a scan then refers to the upload by id.
- **Settings.** `GET/PUT /api/settings…` reads and writes the encrypted settings; keys are never returned.
- **Image proxy.** `GET /api/img` serves only pictures a scan discovered (or you uploaded), fetched by the server, so the browser never contacts a third-party host.

## The front end

Plain JavaScript and CSS under `internal/web/static`, no framework and no build step. Everything hangs off one global `UI`.

- `core.js`: helpers, `UI.api`, icons, tooltips, the dictionaries that name modules and categories.
- `state.js`: the live state of one scan (`UI.S`) and the header.
- `model.js`: `UI.M()`, a cached model derived from the findings (accounts, shared identifiers, graph, conflicts, duplicates, validation). Sections read this model; they do not recompute.
- `kit.js`: building blocks (panels, tables, KPI strips, finding lists).
- `nav.js`: the two-level navigation. A main view is registered with `UI.defineTab({id, label, sections:[…]})`; a section is either a legacy full-page view or a `render(page, M)` function.
- `sec-*.js`: one file per main view. `v-*.js`: the original full-page views (identity, graph, photos…).
- `i18n.js`, `i18n-ar.js`: the language layer (below). `settings.js`, `uploads.js`: the settings drawer and file inputs.

**Security rule for the whole front end:** scraped text (titles, URLs, bios, snippets) is only ever inserted with `textContent` or `setAttribute`, never `innerHTML`, and links are limited to `http(s)` with `rel="noopener noreferrer"`.

### Language

English is the source language of every string. In Arabic mode (`Settings → Language`, stored in the browser, page reloads) `i18n.js` sets `lang="ar" dir="rtl"`, replaces the interface's own words found in the dictionary `i18n-ar.js`, and keeps doing so as the page changes (a `MutationObserver`). A dictionary key containing `{}` stands for any carried-over text, for example `"{} findings"`. Data from sources is never translated. To translate a new string, add its exact English text to the dictionary; a Go test checks the dictionary stays consistent.

## Data and privacy

Cases and settings live under the user's config directory with restrictive permissions. Reports include everything a module returned; they contain personal data, so treat the cases folder and exports accordingly. API keys are stored in an AES-256-GCM encrypted file (the key sits beside it, or `ARGUS_SETTINGS_KEY`); they are never sent to the browser and never enter a report.

## Testing

`go test ./...` covers the engine, every module (with local fake servers, fake binaries for external tools), the exporters, settings, uploads and the web API. There is no automated test of the browser UI yet; it is checked by hand and with Playwright scripts.
