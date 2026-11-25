# Security policy

## Reporting a vulnerability

Please **do not** open a public issue for a security problem. Report it privately through GitHub:
**Security → Report a vulnerability** on the repository (https://github.com/sudosoc/ARGUS/security/advisories/new).

Include what you found, how to reproduce it, the version (`argus -version`) and what you think the impact is.
You will get an answer as soon as the maintainers can; please allow a reasonable time to fix before disclosing publicly.

## What is in scope

The web interface and its API (authentication token, CSRF and origin checks, the upload endpoint, the settings store and
its encrypted keys, the image proxy), the handling of untrusted data from public sources and uploaded files (parsers,
OCR, video and document analysis), the installer and updater, and the way external tools are run.

## Design notes that help you test

* The web server binds to loopback by default and needs a random access token; writes need an `X-Requested-With` header and
  a same-origin request; a strict Content-Security-Policy forbids inline script and external assets.
* Text scraped from the internet is only ever inserted with `textContent`/`setAttribute`, never as HTML.
* External programs run with `os/exec`, never through a shell, with validated arguments and timeouts.
* API keys are kept in an AES-256-GCM encrypted file under the user's config directory and are never sent to the browser.
* Uploaded files are analysed locally; size limits are checked after the file type is determined from its content.

## Supported versions

Only the latest release and `main` receive fixes.
