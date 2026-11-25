#!/bin/sh
# Copyright (C) 2026 Seif, SUDOSOC. SPDX-License-Identifier: AGPL-3.0-only
# Rebuild the embedded username-site list (WhatsMyName + Sherlock + Maigret).
# Usage: scripts/update-sites.sh          (needs Go and network access)
# Writes internal/modules/social/sites_default.json.gz and data/sites/sources.lock.json;
# rebuild the binary afterwards (make build).
set -eu
cd "$(dirname "$0")/.."
go run ./cmd/sitesync -fetch
