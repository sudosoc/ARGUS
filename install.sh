#!/usr/bin/env bash
# Copyright (C) 2026 Seif, SUDOSOC. SPDX-License-Identifier: AGPL-3.0-only
# SUDOSOC ARGUS installer for Linux (Debian/Ubuntu/Kali, Fedora/RHEL, Arch, openSUSE, Alpine).
#
#   ./install.sh                   system-wide install (/usr/local; uses sudo when not root)
#   ./install.sh --user            install for the current user only (~/.local, no root needed)
#   ./install.sh --auto-update     also schedule a daily self-update
#   ./install.sh --doctor          only report what is installed / missing
#   ./install.sh --uninstall       remove what this script installed (see ./uninstall.sh --help)
#
# What it does, end to end:
#   1. installs the OS packages ARGUS and its helper tools need (curl, git, python3 + venv,
#      whois, dig, exiftool, sqlite3, a browser for -browser / PDF reports, theHarvester, whatweb)
#   2. installs Go >= 1.26 if missing (checksum-verified from go.dev)
#   3. builds the static binary and installs it
#   4. installs the optional helper tools into private virtualenvs: maigret, sherlock, holehe
#      (plus phoneinfoga and subfinder release builds) so nothing touches the system Python
#   5. puts the install directory on your PATH, records how it was installed (for update.sh),
#      verifies the result with `argus -doctor`
# Everything is idempotent: run it again at any time to repair or complete an installation.

set -Eeuo pipefail

# ---------------------------------------------------------------- settings --
GO_MIN_MINOR=26   # must match the "go" line in go.mod (dependencies need 1.26)
REPO_URL="${ARGUS_REPO:-https://github.com/sudosoc/ARGUS}"
PHONEINFOGA_REPO="sundowndev/phoneinfoga"
SRC_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" 2>/dev/null && pwd || echo "")"

MODE=""            # system | user (auto-detected unless forced)
PREFIX=""          # override install prefix (bin/ and share/argus/ go under it)
DO_DEPS=1 DO_EXTRAS=1 DO_BROWSER=1 DO_PATH=1 DO_UNINSTALL=0 DO_DOCTOR=0
AUTO=""            # "", daily, weekly: schedule update.sh
SUDO="" PSUDO="" PRIV="" CAN_PKG=1
PM="" TMP="" GO_BIN="" DIED=0
ORIG_ARGS=("$@")
HELPERS=(maigret sherlock holehe h8mail theHarvester)   # tools installed into private virtualenvs

# ------------------------------------------------------------------ output --
if [[ -t 1 ]]; then B=$'\e[1m' G=$'\e[32m' Y=$'\e[33m' R=$'\e[31m' N=$'\e[0m'; else B="" G="" Y="" R="" N=""; fi
info() { printf '%s[*]%s %s\n' "$B" "$N" "$*"; }
ok()   { printf '%s[+]%s %s\n' "$G" "$N" "$*"; }
warn() { printf '%s[!]%s %s\n' "$Y" "$N" "$*" >&2; }
die()  { DIED=1; printf '%s[x]%s %s\n' "$R" "$N" "$*" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

trap 'rc=$?; [[ -n "$TMP" ]] && safe_rm "$TMP"; [[ $rc -ne 0 && $DIED -eq 0 ]] && printf "%s[x]%s install failed (line %s, exit %s). Re-run it: it is safe to repeat.\n" "$R" "$N" "${BASH_LINENO[0]}" "$rc" >&2; exit $rc' EXIT

# safe_rm: only ever delete a clearly-owned, non-trivial path (never "", "/", $HOME or a system dir).
safe_rm() {
  local p
  for p in "$@"; do
    case "$p" in
      ""|/|/usr|/usr/local|/opt|/home|/root|"$HOME"|"$HOME/"|"$HOME/.local"|"$HOME/.local/share") warn "refusing to remove '$p'"; continue ;;
    esac
    [[ "$p" == /* && "$p" == */*/* ]] || { warn "refusing to remove '$p'"; continue; }
    $PRIV rm -rf -- "$p"
  done
}

retry() { # retry N cmd... : transient network errors should not fail an install
  local n="$1" i; shift
  for ((i = 1; i <= n; i++)); do "$@" && return 0; [[ $i -lt $n ]] && sleep $((i * 2)); done
  return 1
}

usage() {
  sed -n '2,20p' "${BASH_SOURCE[0]:-$0}" | sed 's/^# \{0,1\}//'
  cat <<'EOF'

Options:
  --user              install for the current user only (~/.local)
  --system            force a system-wide install (/usr/local)
  --prefix DIR        custom prefix (binary in DIR/bin, data in DIR/share/argus)
  --no-deps           skip OS package installation (also skips the helper tools)
  --no-extras         skip maigret / sherlock / holehe / h8mail / tesseract / phoneinfoga / subfinder / theHarvester / whatweb
  --no-browser        skip the Chrome/Chromium package (needed only for -browser and PDF reports)
  --no-path           do not edit your shell start-up files
  --auto-update[=daily|weekly]   schedule update.sh (default daily)
  --doctor            report installed / missing tools and exit
  --uninstall         remove ARGUS (runs uninstall.sh; add --purge there to also delete saved cases and keys)
  -h, --help          show this help
EOF
}

# -------------------------------------------------------------- arguments --
while [[ $# -gt 0 ]]; do
  case "$1" in
    --user) MODE=user ;;
    --system) MODE=system ;;
    --prefix) PREFIX="${2:?--prefix needs a directory}"; shift ;;
    --no-deps) DO_DEPS=0 ;;
    --no-extras) DO_EXTRAS=0 ;;
    --no-browser) DO_BROWSER=0 ;;
    --no-path) DO_PATH=0 ;;
    --no-sites) ;;                      # accepted for compatibility: the site list is built in
    --auto-update) AUTO=daily ;;
    --auto-update=*) AUTO="${1#*=}" ;;
    --doctor) DO_DOCTOR=1 ;;
    --uninstall) DO_UNINSTALL=1 ;;
    -h|--help) usage; exit 0 ;;
    *) usage >&2; die "unknown option: $1" ;;
  esac
  shift
done
[[ -z "$AUTO" || "$AUTO" == daily || "$AUTO" == weekly ]] || die "--auto-update takes daily or weekly"
[[ "$(uname -s)" == "Linux" ]] || die "this installer is for Linux only (detected $(uname -s))"

# -------------------------------------------- run from a pipe (curl | bash) --
# No source tree next to the script: fetch one and continue from there.
if [[ -z "$SRC_DIR" || ! -f "$SRC_DIR/go.mod" || ! -d "$SRC_DIR/cmd/argus" ]]; then
  have git || die "git is required to download the source (apt install git)"
  checkout="${ARGUS_SRC:-${XDG_DATA_HOME:-$HOME/.local/share}/argus/src}"
  if [[ -d "$checkout/.git" ]]; then git -C "$checkout" pull --ff-only --quiet || true
  else mkdir -p "$(dirname "$checkout")"; info "cloning $REPO_URL"; git clone --quiet "$REPO_URL" "$checkout" || die "cannot clone $REPO_URL"; fi
  exec bash "$checkout/install.sh" "${ORIG_ARGS[@]}"
fi

# ------------------------------------------------------------ mode / paths --
if [[ -z "$MODE" ]]; then
  if [[ $EUID -eq 0 || -n "$PREFIX" && -w "$PREFIX" ]] || have sudo; then MODE=system; else MODE=user; fi
fi
if [[ "$MODE" == system && $EUID -ne 0 && -z "$PREFIX" ]]; then
  if have sudo; then SUDO="sudo"; else warn "no root and no sudo: falling back to --user"; MODE=user; fi
fi
if [[ -n "$PREFIX" && "$MODE" == system && $EUID -ne 0 && ! -w "$PREFIX" ]] && have sudo; then SUDO="sudo"; fi

if [[ -n "$PREFIX" ]]; then
  BIN_DIR="$PREFIX/bin"; DATA_DIR="$PREFIX/share/argus"
elif [[ "$MODE" == system ]]; then
  BIN_DIR=/usr/local/bin; DATA_DIR=/usr/local/share/argus
else
  BIN_DIR="$HOME/.local/bin"; DATA_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/argus"
fi
if [[ "$MODE" == system ]]; then GO_ROOT=/usr/local/go; else GO_ROOT="$HOME/.local/go"; fi
[[ -n "$PREFIX" ]] && GO_ROOT="$PREFIX/go"
TOOLS_DIR="$DATA_DIR/tools"
# Package installs need root regardless of the install mode.
if [[ $EUID -ne 0 ]]; then if have sudo; then PSUDO="sudo"; else CAN_PKG=0; fi; fi
# Privileged commands only when the target is system-owned.
PRIV="$SUDO"; [[ "$MODE" == user && -z "$PREFIX" ]] && PRIV=""

# ------------------------------------------------------------------ doctor --
if [[ $DO_DOCTOR -eq 1 ]]; then
  if [[ -x "$BIN_DIR/argus" ]]; then exec "$BIN_DIR/argus" -doctor; fi
  have argus && exec argus -doctor
  die "argus is not installed yet; run ./install.sh"
fi

# --------------------------------------------------------------- uninstall --
if [[ $DO_UNINSTALL -eq 1 ]]; then
  # the standalone uninstaller does the work (previews, asks, keeps your cases unless --purge)
  [[ -f "$SRC_DIR/uninstall.sh" ]] || die "uninstall.sh not found next to install.sh"
  args=()
  if [[ -n "$PREFIX" ]]; then args+=(--prefix "$PREFIX"); elif [[ "$MODE" == user ]]; then args+=(--user); else args+=(--system); fi
  exec bash "$SRC_DIR/uninstall.sh" "${args[@]}"
fi

# --------------------------------------------------------------------- WSL --
is_wsl() { [[ -n "${WSL_DISTRO_NAME:-}" ]] || grep -qi microsoft /proc/version 2>/dev/null; }

# wsl_preflight: the things that commonly go wrong on Windows Subsystem for Linux, said up front.
wsl_preflight() {
  info "Windows Subsystem for Linux detected (${WSL_DISTRO_NAME:-unknown distribution})"
  grep -qi "wsl2" /proc/version 2>/dev/null || warn "this looks like WSL 1: use WSL 2 (in PowerShell: wsl --set-version <distro> 2); builds and downloads are much slower on WSL 1"
  case "$SRC_DIR" in /mnt/*) warn "the source is on a Windows drive ($SRC_DIR): this is slow and can break file permissions. Clone it inside Linux instead (cd ~ && git clone $REPO_URL)";; esac
  case "$PWD" in /mnt/*) warn "you are on a Windows drive ($PWD). Work from your Linux home (cd ~) for speed and correct permissions" ;; esac
  local mem; mem="$(awk '/MemTotal/ {print int($2/1024)}' /proc/meminfo 2>/dev/null || echo 0)"
  [[ "$mem" -gt 0 && "$mem" -lt 2000 ]] && warn "WSL has only ${mem} MB of memory; the build needs about 2 GB. Raise it in %UserProfile%\\.wslconfig ([wsl2] memory=4GB) and run: wsl --shutdown"
  if ! curl -fsS -m 12 -o /dev/null "https://go.dev/VERSION?m=text" 2>/dev/null; then
    warn "cannot reach go.dev from WSL. Check the Windows network, VPN or proxy; if names do not resolve, try: sudo sh -c 'echo nameserver 1.1.1.1 > /etc/resolv.conf'"
  fi
}

# -------------------------------------------------------- package managers --
detect_pm() {
  PM=""
  for p in apt-get dnf yum pacman zypper apk; do have "$p" && { PM="$p"; return; }; done
}

pm_refresh() {
  case "$PM" in
    apt-get) retry 3 $PSUDO apt-get update -qq ;;
    pacman)  retry 3 $PSUDO pacman -Sy --noconfirm >/dev/null ;;
    zypper)  retry 3 $PSUDO zypper --non-interactive refresh >/dev/null ;;
  esac
}

# pm_install PKG: install one package; returns non-zero if unavailable.
pm_install() {
  case "$PM" in
    apt-get) retry 2 $PSUDO env DEBIAN_FRONTEND=noninteractive DEBCONF_NOWARNINGS=yes apt-get install -y -qq --no-install-recommends "$1" >/dev/null ;;
    dnf)     retry 2 $PSUDO dnf install -y -q "$1" >/dev/null ;;
    yum)     retry 2 $PSUDO yum install -y -q "$1" >/dev/null ;;
    pacman)  retry 2 $PSUDO pacman -S --noconfirm --needed "$1" >/dev/null ;;
    zypper)  retry 2 $PSUDO zypper --non-interactive install "$1" >/dev/null ;;
    apk)     retry 2 $PSUDO apk add --quiet "$1" >/dev/null ;;
    *)       return 1 ;;
  esac
}

# Package names differ per distro family.
pkg_names() { # echo package list for role $1
  case "$1:$PM" in
    base:*)            echo "curl ca-certificates tar gzip git" ;;
    python:apt-get)    echo "python3 python3-venv python3-pip" ;;
    python:dnf|python:yum|python:zypper) echo "python3 python3-pip" ;;
    python:pacman)     echo "python python-pip" ;;
    python:apk)        echo "python3 py3-pip" ;;
    pybuild:apt-get)   echo "gcc python3-dev libffi-dev libxml2-dev libxslt1-dev" ;;
    pybuild:dnf|pybuild:yum) echo "gcc python3-devel libffi-devel libxml2-devel libxslt-devel" ;;
    pybuild:pacman)    echo "gcc libffi libxml2 libxslt" ;;
    pybuild:zypper)    echo "gcc python3-devel libffi-devel libxml2-devel libxslt-devel" ;;
    pybuild:apk)       echo "gcc musl-dev python3-dev libffi-dev libxml2-dev libxslt-dev" ;;
    whois:*)           echo "whois" ;;
    dig:apt-get)       echo "dnsutils" ;;
    dig:dnf|dig:yum|dig:zypper) echo "bind-utils" ;;
    dig:pacman)        echo "bind" ;;
    dig:apk)           echo "bind-tools" ;;
    exiftool:apt-get)  echo "libimage-exiftool-perl" ;;
    exiftool:dnf|exiftool:yum|exiftool:zypper) echo "perl-Image-ExifTool" ;;
    exiftool:pacman)   echo "perl-image-exiftool" ;;
    exiftool:apk)      echo "exiftool" ;;
    tesseract:apt-get) echo "tesseract-ocr tesseract-ocr-ara" ;;
    tesseract:dnf|tesseract:yum) echo "tesseract tesseract-langpack-ara" ;;
    tesseract:pacman)  echo "tesseract tesseract-data-ara tesseract-data-eng" ;;
    tesseract:zypper)  echo "tesseract-ocr tesseract-ocr-traineddata-arabic tesseract-ocr-traineddata-english" ;;
    tesseract:apk)     echo "tesseract-ocr tesseract-ocr-data-ara tesseract-ocr-data-eng" ;;
    ffmpeg:*)          echo "ffmpeg" ;;
    sqlite:apt-get|sqlite:apk) echo "sqlite3" ;;
    sqlite:*)          echo "sqlite" ;;
  esac
}

# install_roles required|optional role...: install every package of each role.
install_roles() {
  local level="$1"; shift
  local role pkg
  for role in "$@"; do
    for pkg in $(pkg_names "$role"); do
      if pm_install "$pkg"; then continue; fi
      if [[ "$level" == required ]]; then die "could not install required package: $pkg"; fi
      warn "optional package unavailable: $pkg"
    done
  done
}

browser_present() {
  have chromium || have chromium-browser || have google-chrome || have google-chrome-stable || have microsoft-edge
}

install_browser() { # Chrome/Chromium is needed only for -browser and PDF reports
  if browser_present; then ok "browser already installed"; return; fi
  local p
  for p in chromium chromium-browser; do
    # on Ubuntu "chromium-browser" is a snap stub that does not work under WSL
    if [[ "$p" == chromium-browser ]] && is_wsl; then continue; fi
    if pm_install "$p" 2>/dev/null && browser_present; then ok "installed $p"; return; fi
  done
  if is_wsl && [[ "$PM" == apt-get && "$(uname -m)" == x86_64 ]]; then
    info "installing Google Chrome (.deb) for -browser and PDF reports"
    if retry 2 curl -fSL -o "$TMP/chrome.deb" "https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb" \
       && $PSUDO env DEBIAN_FRONTEND=noninteractive apt-get install -y -qq "$TMP/chrome.deb" >/dev/null 2>&1 && browser_present; then ok "installed google-chrome"; return; fi
  fi
  warn "no Chrome/Chromium package available (optional: only -browser and PDF reports need it; the html report can be printed to PDF by hand)"
}

# ---------------------------------------------------------------------- Go --
go_ok() { # go_ok BIN : is that go >= 1.$GO_MIN_MINOR ?
  local v minor major
  v="$("$1" env GOVERSION 2>/dev/null || true)"; v="${v#go}"
  major="${v%%.*}"; minor="${v#*.}"; minor="${minor%%[!0-9]*}"
  [[ "$major" =~ ^[0-9]+$ && "$minor" =~ ^[0-9]+$ ]] || return 1
  (( major > 1 || (major == 1 && minor >= GO_MIN_MINOR) ))
}

install_go() {
  local arch ver file want got
  case "$(uname -m)" in
    x86_64|amd64)  arch=amd64 ;;
    aarch64|arm64) arch=arm64 ;;
    armv7l|armv6l) arch=armv6l ;;
    i386|i686)     arch=386 ;;
    *) die "unsupported CPU architecture: $(uname -m)" ;;
  esac
  ver="$(retry 3 curl -fsSL "https://go.dev/VERSION?m=text" | head -n1)"
  [[ "$ver" =~ ^go1\.[0-9]+ ]] || die "could not determine the latest Go version (is go.dev reachable?)"
  file="${ver}.linux-${arch}.tar.gz"
  info "downloading Go ${ver#go} (${arch})"
  retry 3 curl -fSL -o "$TMP/$file" "https://go.dev/dl/$file" || die "Go download failed"
  want="$(retry 3 curl -fsSL "https://dl.google.com/go/$file.sha256" | tr -d '[:space:]')"
  got="$(sha256sum "$TMP/$file" | cut -d' ' -f1)"
  [[ -n "$want" && "$want" == "$got" ]] || die "Go archive checksum mismatch (expected '$want', got '$got')"
  info "installing Go into $GO_ROOT"
  local parent; parent="$(dirname "$GO_ROOT")"
  [[ "$GO_ROOT" == */go ]] || die "unexpected Go directory: $GO_ROOT"
  if [[ "$MODE" == system && -z "$PREFIX" ]]; then PRIV=$SUDO safe_rm "$GO_ROOT"; $SUDO tar -C "$parent" -xzf "$TMP/$file"
  else mkdir -p "$parent"; PRIV="" safe_rm "$GO_ROOT"; tar -C "$parent" -xzf "$TMP/$file"; fi
}

ensure_go() {
  local c
  for c in "$(command -v go 2>/dev/null || true)" "$GO_ROOT/bin/go" /usr/local/go/bin/go "$HOME/.local/go/bin/go"; do
    if [[ -n "$c" && -x "$c" ]] && go_ok "$c"; then GO_BIN="$c"; ok "using $("$c" version)"; return; fi
  done
  info "Go >= 1.$GO_MIN_MINOR not found"
  install_go
  GO_BIN="$GO_ROOT/bin/go"
  go_ok "$GO_BIN" || die "Go installation failed"
  ok "installed $("$GO_BIN" version)"
}

# ------------------------------------------------------------------- build --
build_argus() {
  info "building argus"
  local commit date
  commit="$(git -C "$SRC_DIR" rev-parse --short HEAD 2>/dev/null || echo "")"
  date="$(date -u +%Y-%m-%d)"
  ( cd "$SRC_DIR"
    export PATH="$(dirname "$GO_BIN"):$PATH" CGO_ENABLED=0 GOFLAGS=-trimpath
    retry 3 "$GO_BIN" mod download || die "cannot download the Go dependencies (is proxy.golang.org reachable?)"
    "$GO_BIN" build -ldflags "-s -w -X github.com/sudosoc/argus/internal/engine.Commit=$commit -X github.com/sudosoc/argus/internal/engine.BuildDate=$date" \
      -o "$TMP/argus" ./cmd/argus )
  ok "build complete"
}

install_files() {
  info "installing to $BIN_DIR and $DATA_DIR"
  $PRIV install -d -m 0755 "$BIN_DIR" "$DATA_DIR"
  $PRIV install -m 0755 "$TMP/argus" "$BIN_DIR/argus.new"
  $PRIV mv -f "$BIN_DIR/argus.new" "$BIN_DIR/argus"          # atomic: never leaves a half-written binary
  if [[ -d "$SRC_DIR/configs" ]]; then $PRIV cp -r "$SRC_DIR/configs/." "$DATA_DIR/"; fi
  if [[ -f "$SRC_DIR/scripts/argus-ui.sh" ]]; then $PRIV install -m 0755 "$SRC_DIR/scripts/argus-ui.sh" "$BIN_DIR/argus-ui"; fi
  ok "installed $BIN_DIR/argus"
}

# Record how this copy was installed so update.sh and `argus -update` can repeat it.
write_conf() {
  local conf="$TMP/install.conf"
  {
    echo "# written by install.sh $(date -u +%FT%TZ)"
    echo "SRC_DIR=\"$SRC_DIR\""
    echo "MODE=$MODE"
    echo "PREFIX=\"$PREFIX\""
    echo "BIN_DIR=\"$BIN_DIR\""
    echo "DATA_DIR=\"$DATA_DIR\""
    echo "GO_ROOT=\"$GO_ROOT\""
    echo "DEPS=$DO_DEPS"
    echo "EXTRAS=$DO_EXTRAS"
    echo "BROWSER=$DO_BROWSER"
  } > "$conf"
  $PRIV install -m 0644 "$conf" "$DATA_DIR/install.conf"
  cp "$conf" "$SRC_DIR/.argus-install.conf" 2>/dev/null || true
}

# ------------------------------------------------------- python helper tools --
# Each tool lives in its own virtualenv under $DATA_DIR/tools, with a symlink in
# $BIN_DIR: nothing touches the system Python (PEP 668 safe) and update.sh can
# upgrade them. A tool the user already has (pipx, apt, …) is left alone.
PYBIN=""
# venv_ok PY: this interpreter can create a virtualenv with pip (Debian/Ubuntu ship that in a separate package).
venv_ok() { "$1" -c 'import ensurepip, venv' >/dev/null 2>&1; }
py_ver() { "$1" -c 'import sys; print("%d.%d" % sys.version_info[:2])' 2>/dev/null; }
py_minor_ok() { "$1" -c "import sys; sys.exit(0 if sys.version_info >= (3, $2) else 1)" >/dev/null 2>&1; }

# pick_python: choose the interpreter for the helper tools. Older, widely tested versions first (the tools' dependencies
# often lag behind the newest Python), then whatever python3 is. Installs python3-venv when that is what is missing.
pick_python() {
  [[ -n "$PYBIN" ]] && return 0
  local c cand=""
  if [[ -n "${ARGUS_PYTHON:-}" ]] && have "$ARGUS_PYTHON"; then cand="$ARGUS_PYTHON"
  else
    for c in python3.12 python3.11 python3.13 python3.10 python3; do
      have "$c" && py_minor_ok "$c" 10 && { cand="$c"; break; }
    done
  fi
  [[ -n "$cand" ]] || return 1
  if ! venv_ok "$cand"; then
    info "python venv support is missing for $cand: installing it"
    local mm; mm="$(py_ver "$cand")"
    install_roles optional python >/dev/null 2>&1 || true            # python3 python3-venv python3-pip (apt)
    venv_ok "$cand" || { [[ "$PM" == apt-get ]] && pm_install "python${mm}-venv" >/dev/null 2>&1 || true; }
    venv_ok "$cand" || { [[ "$PM" == dnf || "$PM" == yum ]] && pm_install python3-pip >/dev/null 2>&1 || true; }
    if ! venv_ok "$cand"; then
      warn "$cand cannot create virtual environments. Fix: sudo apt install python${mm}-venv   (or python3-venv), then run install.sh again"
      return 1
    fi
  fi
  PYBIN="$(command -v "$cand")"
  info "helper tools will use $PYBIN (Python $(py_ver "$PYBIN"))"
}
python_ok() { pick_python; }

py_install() { # py_install TOOL PIP-PACKAGE
  local tool="$1" pkg="$2" venv="$TOOLS_DIR/$1" log="$TMP/pip-$1.log"
  if have "$tool" && [[ ! -d "$venv" ]]; then ok "$tool already installed ($(command -v "$tool"))"; return 0; fi
  pick_python || return 1
  $PRIV mkdir -p "$TOOLS_DIR"
  # a half-made virtualenv (a failed earlier run leaves bin/python but no pip) is removed and made again
  if [[ ! -x "$venv/bin/python" || ! -x "$venv/bin/pip" ]]; then
    safe_rm "$venv"
    $PRIV "$PYBIN" -m venv "$venv" >"$log" 2>&1 && [[ -x "$venv/bin/pip" ]] || { warn "cannot create a virtualenv for $tool: $(tail -n 2 "$log" | tr '\n' ' ')"; safe_rm "$venv"; return 1; }
  fi
  if retry 2 $PRIV "$venv/bin/pip" install -q --disable-pip-version-check --upgrade pip wheel >"$log" 2>&1 \
     && retry 2 $PRIV "$venv/bin/pip" install -q --disable-pip-version-check --upgrade "$pkg" >>"$log" 2>&1 \
     && [[ -x "$venv/bin/$tool" ]]; then
    $PRIV ln -sf "$venv/bin/$tool" "$BIN_DIR/$tool"
    ok "installed $tool"
  else
    warn "could not install $tool with Python $(py_ver "$PYBIN"). Last lines of pip's output:"
    tail -n 6 "$log" 2>/dev/null | sed 's/^/      /' >&2
    safe_rm "$venv"
    warn "  (try another Python: ARGUS_PYTHON=python3.12 ./install.sh, or: $PYBIN -m venv v && v/bin/pip install $pkg)"
    return 1
  fi
}

# GitHub release binary with a verified checksum (phoneinfoga).
install_phoneinfoga() {
  if have phoneinfoga; then ok "phoneinfoga already installed"; return; fi
  local arch file base want got
  case "$(uname -m)" in x86_64|amd64) arch=x86_64 ;; aarch64|arm64) arch=arm64 ;; armv7l|armv6l) arch=armv6 ;; i386|i686) arch=i386 ;; *) return 0 ;; esac
  file="phoneinfoga_Linux_${arch}.tar.gz"
  base="https://github.com/$PHONEINFOGA_REPO/releases/latest/download"
  if retry 2 curl -fsSL -o "$TMP/$file" "$base/$file"; then
    if retry 2 curl -fsSL -o "$TMP/pi-checksums.txt" "$base/phoneinfoga_checksums.txt" 2>/dev/null; then
      want="$(grep " $file\$" "$TMP/pi-checksums.txt" | cut -d' ' -f1)"; got="$(sha256sum "$TMP/$file" | cut -d' ' -f1)"
      if [[ -n "$want" && "$want" != "$got" ]]; then warn "phoneinfoga checksum mismatch: not installed"; return; fi
    fi
    mkdir -p "$TMP/pi" && tar -C "$TMP/pi" -xzf "$TMP/$file" 2>/dev/null || true
    if [[ -f "$TMP/pi/phoneinfoga" ]] && $PRIV install -m 0755 "$TMP/pi/phoneinfoga" "$BIN_DIR/phoneinfoga"; then ok "installed phoneinfoga"; return; fi
  fi
  warn "could not install phoneinfoga (optional: https://github.com/$PHONEINFOGA_REPO/releases)"
}

install_extras() {
  info "installing optional helper tools"
  have whois    || install_roles optional whois
  have dig      || install_roles optional dig
  have exiftool || install_roles optional exiftool
  have ffmpeg || install_roles optional ffmpeg      # video frames and metadata for uploaded videos
  have tesseract || install_roles optional tesseract   # reads text printed in downloaded pictures (OCR)
  have sqlite3  || install_roles optional sqlite
  have python3  || install_roles optional python

  if python_ok; then
    install_roles optional pybuild
    py_install maigret maigret || true
    py_install sherlock sherlock-project || true
    py_install holehe holehe || true
    py_install h8mail h8mail || true   # breach lookup (source names only are shown, never credentials)
  else
    warn "no usable Python >= 3.10 with venv support: skipping maigret, sherlock, holehe and h8mail"
  fi

  install_phoneinfoga

  if have theHarvester || have theharvester; then ok "theHarvester already installed"
  elif [[ "$PM" == apt-get ]] && pm_install theharvester 2>/dev/null; then ok "installed theHarvester"
  elif [[ -n "$PYBIN" ]] && py_minor_ok "$PYBIN" 12 \
       && py_install theHarvester "git+https://github.com/laramies/theHarvester.git" 2>/dev/null; then :
  else warn "theHarvester not installed: it is packaged for Debian/Kali (apt install theharvester) and its pip build needs Python >= 3.12 (optional)"; fi

  if have whatweb; then ok "whatweb already installed"
  elif pm_install whatweb 2>/dev/null; then ok "installed whatweb"
  else warn "whatweb not available from the package manager (optional)"; fi

  if have subfinder; then ok "subfinder already installed"
  elif [[ -n "$GO_BIN" ]]; then
    info "installing subfinder with go install (optional, may take a minute)"
    if PATH="$(dirname "$GO_BIN"):$PATH" GOBIN="$TMP/gobin" retry 2 "$GO_BIN" install github.com/projectdiscovery/subfinder/v2/cmd/subfinder@latest >/dev/null 2>&1 \
       && $PRIV install -m 0755 "$TMP/gobin/subfinder" "$BIN_DIR/subfinder"; then ok "installed subfinder"
    else warn "could not install subfinder (optional: go install github.com/projectdiscovery/subfinder/v2/cmd/subfinder@latest)"; fi
  fi
}

# --------------------------------------------------------------------- PATH --
ensure_path() {
  [[ $DO_PATH -eq 1 ]] || return 0
  if echo ":$PATH:" | grep -q ":$BIN_DIR:"; then return 0; fi
  local rc line="export PATH=\"$BIN_DIR:\$PATH\"" added=0
  for rc in "$HOME/.profile" "$HOME/.bashrc" "$HOME/.zshrc"; do
    [[ -f "$rc" ]] || continue
    if grep -q "# added by SUDOSOC ARGUS" "$rc" 2>/dev/null; then added=1; continue; fi
    printf '\n# added by SUDOSOC ARGUS\n%s\n' "$line" >> "$rc"; added=1
  done
  if [[ $added -eq 1 ]]; then warn "$BIN_DIR was not on your PATH: added it to your shell start-up files. Open a new terminal, or run: export PATH=\"$BIN_DIR:\$PATH\""
  else warn "$BIN_DIR is not on your PATH. Add: $line"; fi
}

# --------------------------------------------------------------------- run --
main() {
  TMP="$(mktemp -d)"
  info "SUDOSOC ARGUS installer   mode: $MODE   binary: $BIN_DIR/argus   data: $DATA_DIR"

  if is_wsl; then wsl_preflight; fi
  detect_pm
  if [[ $DO_DEPS -eq 1 ]]; then
    if [[ -z "$PM" ]]; then warn "no supported package manager found; assuming curl, tar, git and python3 are present"
    elif [[ $CAN_PKG -eq 0 ]]; then warn "no root/sudo: skipping OS packages (curl, git, tar and gzip must already exist)"; PM=""
    else
      info "package manager: $PM"
      pm_refresh
      install_roles required base
    fi
  fi
  have curl || die "curl is required"
  have tar  || die "tar is required"

  ensure_go
  build_argus
  install_files
  write_conf
  if [[ $DO_DEPS -eq 1 && -n "$PM" ]]; then
    if [[ $DO_EXTRAS -eq 1 ]]; then install_extras; fi
    if [[ $DO_BROWSER -eq 1 ]]; then install_browser; fi
  fi
  ensure_path

  # ---- verify
  local bin="$BIN_DIR/argus"
  "$bin" -version >/dev/null || die "the installed binary does not run"
  ok "$("$bin" -version)"
  if [[ -n "$AUTO" ]]; then bash "$SRC_DIR/update.sh" --enable-auto "$AUTO" || warn "could not schedule the automatic update"; fi
  echo
  PATH="$BIN_DIR:$PATH" "$bin" -doctor | sed -n '/^Optional tools/,$p'

  echo
  ok "done."
  cat <<EOT

Try it:
  argus-ui                           start the browser UI and open it (under WSL: in Windows)
  argus -web 127.0.0.1:8080          browser UI by hand (prints a URL with your access token)
  argus -u johndoe -verify -o reports
  argus -n "Jane Doe" -e jane@example.com -derive -o reports
  argus -doctor                      what is installed / missing
  argus -update                      pull the latest version, rebuild and refresh the helper tools
  ./install.sh --auto-update         keep it updated automatically

Use only on subjects you are lawfully permitted to investigate.
EOT
}

main
