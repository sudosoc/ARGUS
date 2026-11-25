# Writing a module

A module (a *collector*) takes what the operator supplied and reports what it found about it. This guide builds one from scratch, then lists everywhere a new module must be mentioned. Read [ARCHITECTURE.md](ARCHITECTURE.md) first if you have not.

## The contract

```go
type Scanner interface {
    Name() string                      // short unique id, used by -modules
    Description() string               // one line, shown in the UI and -list-modules
    CanRun(t models.Target) bool       // does this module apply to these inputs?
    Scan(ctx context.Context, t models.Target, d scanner.Deps, emit func(models.Finding)) error
}
```

Rules for `Scan`:

- Honour `ctx`: stop promptly when it is cancelled.
- Do concurrent work on `d.Pool`, not with bare goroutines; use `d.HTTP` for every web request (retries, proxies, size caps and politeness are handled there).
- Report only through `emit`, which is safe for concurrent use, and never after `Scan` returns.
- Call `d.Progress(name, done, total)` as units of work complete if you can; it feeds the live table.
- Return an error only for a failure of the module as a whole. A single check that failed is an `emit` of a finding with `Status: models.StatusError`.

## A finding

```go
models.Finding{
    Source:     "what produced it, shown as the title",   // e.g. "DNS records example.org"
    Category:   "dns",                                      // groups it in the UI
    URL:        "https://…",                                // where to verify it, if any
    Status:     models.StatusFound,                         // found | uncertain | not_found | error
    Confidence: 0.9,                                        // 0..1, how direct the evidence is
    Details:    map[string]string{"A": "203.0.113.5"},      // everything the source returned
    Entities:   []models.Entity{{Type: models.EntIP, Value: "203.0.113.5"}}, // leads to follow
}
```

Honest confidence matters more than a big number. A guide: an official record 0.9+, a content marker found on a page 0.9, a bare "HTTP 200" 0.6, a name match 0.3–0.5. Report `not_found` and `error` too: the report keeps them as coverage information. Put **everything** the source returned in `Details` (do not summarise and discard); values are shown as text, never as HTML.

`Entities` are what the engine may *pivot* on: e-mails, domains, usernames, phones, names, locations, organisations, IPs, URLs and picture URLs (`models.Ent*`). Mark one `Strong: true` only when it is certainly the subject's own (an e-mail on the subject's own verified profile); a weak entity is listed but not followed automatically.

## An example

```go
package example

import (
    "context"

    "github.com/sudosoc/argus/internal/httpx"
    "github.com/sudosoc/argus/internal/models"
    "github.com/sudosoc/argus/internal/scanner"
)

type Scanner struct{}

var _ scanner.Scanner = (*Scanner)(nil) // compile-time check

func New() *Scanner { return &Scanner{} }

func (s *Scanner) Name() string        { return "example" }
func (s *Scanner) Description() string { return "looks the username up in the Example directory" }
func (s *Scanner) CanRun(t models.Target) bool { return t.Username != "" }

func (s *Scanner) Scan(ctx context.Context, t models.Target, d scanner.Deps, emit func(models.Finding)) error {
    resp, err := d.HTTP.Do(ctx, httpx.Request{URL: "https://example.org/api/users/" + t.Username})
    if err != nil {
        emit(models.Finding{Source: "Example directory", Category: "profile", Status: models.StatusError,
            Details: map[string]string{"error": err.Error()}})
        return nil
    }
    if resp.StatusCode != 200 {
        emit(models.Finding{Source: "Example directory", Category: "profile", Status: models.StatusNotFound})
        return nil
    }
    emit(models.Finding{
        Source: "Example directory: " + t.Username, Category: "profile", Status: models.StatusFound, Confidence: 0.8,
        URL: "https://example.org/u/" + t.Username, Details: map[string]string{"raw": string(resp.Body)},
    })
    return nil
}
```

## Wiring it in

1. **Register it** in `internal/app/app.go`, `Build`: `eng.Register(example.New())`. This is the only place the engine learns about a module.
2. **Tell the UI about it** in `internal/web/static/js/core.js`:
   - `UI.MODULE_NEEDS["example"] = "username"` (what input it needs),
   - add `"example"` to `UI.MODULE_ORDER` (where it sorts),
   - `UI.CATEGORIES` gets an entry for any new `Category` (label and icon), otherwise it is shown with a humanised name.
3. **Rate its source** in `js/model.js`, `UI.RELIABILITY["example"] = ["Medium", "why"]`, which feeds Evidence → Source reliability.
4. **Describe it** in the README module table (`| \`example\` | username | … |`).
5. **Give it a place in the UI** if its results deserve one: a section in the relevant `sec-*.js` file using `K.section(id, label, icon, pick)`, where `pick` filters `M.hits`. Without it the findings still appear under Findings and Evidence.
6. **Translate** any new interface text in `js/i18n-ar.js` (exact English string as key).

If the module should also run on pivoted leads, nothing more is needed: the engine plans every registered module against each new target with `CanRun`.

## Testing a module

Tests must not need the internet. Point the module at a local server:

- Make the endpoints package variables (see `internal/modules/contextual`: `var endpoints = struct{…}{…}`), set them to an `httptest.Server` URL in the test, and restore them afterwards.
- For a module that runs a program, put a small fake executable on `PATH` in a temporary directory (see `internal/modules/syswrap/keys_test.go`).
- Build the HTTP client with `httpx.New(httpx.Options{Timeout: …})` and the pool with `pool.New(workers, queue)`.

Check at least: `CanRun` true and false, one found result, one not-found, one error, and that nothing is emitted after `ctx` is cancelled. Run `gofmt -l .`, `go vet ./...`, `go test -race ./...`.

## Wrapping an external program

Prefer a native module. If a program is the right tool, add a `Tool` in `internal/modules/syswrap` (see `tools.go`, `extras.go`, `social.go`): its arguments come from validated inputs only, it runs through `os/exec` with a timeout, and the complete stdout is kept automatically as a raw record. If it needs an API key, set `APIProvider` to a provider id from `internal/settings`: the tool is then skipped when that provider is off or has no key, and the key is passed in the provider's environment variable. Do not parse what you cannot see: if you do not know a program's output format, list the URLs, e-mails and phones in it (`parseGeneric`) and keep the raw text.

## Do's and don'ts

- Use only public, unauthenticated sources. Do not log in, solve CAPTCHAs, evade blocks or search for home addresses. A source that needs those goes into `internal/leads` as a pre-filled manual link instead.
- Never invent data and never hide a failure.
- Keep personal data out of logs and out of requests to services that do not need it.
