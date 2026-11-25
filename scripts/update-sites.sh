#!/bin/sh
# Rebuild the embedded username-site list (WhatsMyName + Sherlock + Maigret).
# Usage: scripts/update-sites.sh          (needs Go and network access)
# Writes internal/modules/social/sites_default.json.gz and data/sites/sources.lock.json;
# rebuild the binary afterwards (make build).
set -eu
cd "$(dirname "$0")/.."
go run ./cmd/sitesync -fetch
