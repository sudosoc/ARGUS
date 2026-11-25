#!/usr/bin/env bash
# Copyright (C) 2026 Seif, SUDOSOC. SPDX-License-Identifier: AGPL-3.0-only
# Start the SUDOSOC ARGUS web interface and open it in your browser.
#
#   argus-ui                 pick a free port from 8080, make a fresh access token, start, open the browser
#   argus-ui 9000            use port 9000
#   argus-ui -- -max-pivots 50    everything after "--" is passed to argus
#
# Under WSL the page opens in your Windows browser. Press Ctrl+C in this window to stop the server.
# Set ARGUS_UI_NO_OPEN=1 to only print the address.
set -Eeuo pipefail

PORT="${1:-}"
if [[ "$PORT" == "--" || -z "$PORT" || ! "$PORT" =~ ^[0-9]+$ ]]; then PORT=""; else shift; fi
[[ "${1:-}" == "--" ]] && shift
EXTRA=("$@")

command -v argus >/dev/null 2>&1 || { echo "argus is not on your PATH (open a new terminal, or run: export PATH=\"\$HOME/.local/bin:\$PATH\")" >&2; exit 1; }

port_free() { ! (exec 3<>"/dev/tcp/127.0.0.1/$1") 2>/dev/null; }
if [[ -z "$PORT" ]]; then
  for p in $(seq 8080 8120); do port_free "$p" && { PORT="$p"; break; }; done
  [[ -n "$PORT" ]] || { echo "no free port between 8080 and 8120" >&2; exit 1; }
elif ! port_free "$PORT"; then
  echo "port $PORT is already in use" >&2; exit 1
fi

TOKEN="$(head -c 24 /dev/urandom | od -An -tx1 | tr -d ' \n')"
URL="http://127.0.0.1:$PORT/?token=$TOKEN"

open_url() {
  [[ -n "${ARGUS_UI_NO_OPEN:-}" ]] && return 0
  if [[ -n "${WSL_DISTRO_NAME:-}" ]] || grep -qi microsoft /proc/version 2>/dev/null; then
    if command -v wslview >/dev/null 2>&1; then wslview "$URL" >/dev/null 2>&1 && return 0; fi
    if command -v explorer.exe >/dev/null 2>&1; then explorer.exe "$URL" >/dev/null 2>&1 || true; return 0; fi
    if command -v cmd.exe >/dev/null 2>&1; then cmd.exe /c start "" "$URL" >/dev/null 2>&1 || true; return 0; fi
  fi
  if command -v xdg-open >/dev/null 2>&1; then xdg-open "$URL" >/dev/null 2>&1 || true; return 0; fi
  return 1
}

argus -web "127.0.0.1:$PORT" -web-token "$TOKEN" "${EXTRA[@]}" &
PID=$!
trap 'kill "$PID" 2>/dev/null || true; wait "$PID" 2>/dev/null || true' EXIT INT TERM

for _ in $(seq 1 60); do
  kill -0 "$PID" 2>/dev/null || { echo "argus stopped unexpectedly" >&2; exit 1; }
  port_free "$PORT" || break
  sleep 0.25
done

echo
echo "  SUDOSOC ARGUS is running:  $URL"
echo "  (Ctrl+C here stops it. Keep this address private: it contains your access token.)"
echo
open_url || echo "  Open the address above in your browser."
wait "$PID"
