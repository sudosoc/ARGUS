#!/usr/bin/env bash
# Copyright (C) 2026 Seif, SUDOSOC. SPDX-License-Identifier: AGPL-3.0-only
# SUDOSOC ARGUS uninstaller for Linux. Removes what install.sh put on the machine.
#
#   ./uninstall.sh                 show what will be removed, ask, then remove (finds user and system installs)
#   ./uninstall.sh --dry-run       only show what would be removed
#   ./uninstall.sh --yes           do not ask
#   ./uninstall.sh --purge         ALSO delete your saved cases, API settings and keys, and logs (personal data)
#   ./uninstall.sh --user | --system | --prefix DIR   uninstall only that install
#   ./uninstall.sh --keep-extras   leave phoneinfoga and subfinder in the bin directory
#   ./uninstall.sh --remove-go     also remove the Go toolchain install.sh put in the install location
#   ./uninstall.sh --remove-python-tools   (default: helper tools in the private data directory are always removed)
#
# By default it removes: the argus program, the helper tools install.sh put in its private data directory
# (maigret, sherlock, holehe, h8mail, theHarvester virtualenvs and their links), phoneinfoga and subfinder in the
# bin directory, the automatic-update schedule, and the PATH lines install.sh added to your shell start-up files.
#
# It never removes: your saved cases, settings and API keys (unless --purge), reports you exported, the source
# folder you cloned, OS packages (whois, dig, exiftool, tesseract, ffmpeg, chromium, python3 ...), or Go (unless
# --remove-go). It prints the commands to remove the OS packages if you want them gone too.

set -Eeuo pipefail

if [[ -t 1 ]]; then B=$'\e[1m' G=$'\e[32m' Y=$'\e[33m' R=$'\e[31m' N=$'\e[0m'; else B="" G="" Y="" R="" N=""; fi
info() { printf '%s[*]%s %s\n' "$B" "$N" "$*"; }
ok()   { printf '%s[+]%s %s\n' "$G" "$N" "$*"; }
warn() { printf '%s[!]%s %s\n' "$Y" "$N" "$*" >&2; }
die()  { printf '%s[x]%s %s\n' "$R" "$N" "$*" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

[[ "$(uname -s)" == "Linux" ]] || die "this uninstaller is for Linux only (detected $(uname -s))"

SELF="$(readlink -f "${BASH_SOURCE[0]:-$0}" 2>/dev/null || echo "")"
SRC_DIR=""; [[ -n "$SELF" ]] && SRC_DIR="$(dirname "$SELF")"

# ------------------------------------------------------------------ options --
ORIG_ARGS=("$@")
DRY=0 YES=0 PURGE=0 KEEP_EXTRAS=0 REMOVE_GO=0 FORCE_MODE="" PREFIX=""
usage() { sed -n '2,22p' "${BASH_SOURCE[0]:-$0}" | sed 's/^# \{0,1\}//'; }
while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run) DRY=1 ;;
    -y|--yes) YES=1 ;;
    --purge) PURGE=1 ;;
    --keep-extras) KEEP_EXTRAS=1 ;;
    --remove-go) REMOVE_GO=1 ;;
    --remove-python-tools) ;;                       # accepted for clarity: always done
    --user) FORCE_MODE=user ;;
    --system) FORCE_MODE=system ;;
    --prefix) PREFIX="${2:?--prefix needs a directory}"; FORCE_MODE=prefix; shift ;;
    -h|--help) usage; exit 0 ;;
    *) usage >&2; die "unknown option: $1" ;;
  esac
  shift
done

# --------------------------------------------- run from a safe copy of itself --
# The script may live inside a directory it is about to delete (the private data directory when ARGUS was
# installed with a pipe). Bash reads scripts as it goes, so continue from a temporary copy.
if [[ -z "${ARGUS_UNINSTALL_COPY:-}" && -n "$SELF" ]]; then
  cp_dir="$(mktemp -d)"; cp "$SELF" "$cp_dir/uninstall.sh"
  ARGUS_UNINSTALL_COPY="$cp_dir" ARGUS_UNINSTALL_SRC="$SRC_DIR" exec bash "$cp_dir/uninstall.sh" "${ORIG_ARGS[@]}"
fi
[[ -n "${ARGUS_UNINSTALL_SRC:-}" ]] && SRC_DIR="$ARGUS_UNINSTALL_SRC"
cleanup() { [[ -n "${ARGUS_UNINSTALL_COPY:-}" && -d "${ARGUS_UNINSTALL_COPY}" ]] && rm -rf -- "${ARGUS_UNINSTALL_COPY}" 2>/dev/null || true; }
trap cleanup EXIT

# safe_rm: only ever delete a clearly-owned, non-trivial path (never "", "/", $HOME or a system directory).
safe_path() {
  local p="$1"
  case "$p" in
    ""|/|/usr|/usr/local|/usr/local/bin|/usr/bin|/bin|/opt|/home|/root|/etc|/var|"$HOME"|"$HOME/"|"$HOME/.local"|"$HOME/.local/bin"|"$HOME/.local/share"|"$HOME/.config"|"$HOME/.local/state") return 1 ;;
  esac
  [[ "$p" == /* && "$p" == */*/* ]]
}

# ------------------------------------------------------------- the plan ------
# Each action is "kind|path|sudo(0/1)|note"; nothing is removed until the plan is shown.
PLAN=()
add() { PLAN+=("$1|$2|$3|${4:-}"); }
NOTES=()

SUDO=""; if [[ $EUID -ne 0 ]] && have sudo; then SUDO="sudo"; fi

# $1 mode (user|system|prefix) $2 bin dir $3 data dir $4 go root
plan_install() {
  local mode="$1" bin="$2" data="$3" goroot="$4" sudo=0 t tools
  [[ "$mode" != user ]] && sudo=1
  [[ $EUID -eq 0 ]] && sudo=0
  # a prefix install owned by the user needs no sudo
  if [[ "$mode" == prefix && -w "$bin" ]]; then sudo=0; fi
  [[ -e "$bin/argus" || -d "$data" ]] || return 1
  tools="$data/tools"
  for t in maigret sherlock holehe h8mail theHarvester theharvester; do
    if [[ -L "$bin/$t" && "$(readlink "$bin/$t")" == "$tools/"* ]]; then add file "$bin/$t" $sudo "helper tool link"; fi
  done
  [[ -e "$bin/argus" ]] && add file "$bin/argus" $sudo "the argus program"
  [[ -e "$bin/argus-ui" ]] && add file "$bin/argus-ui" $sudo "the argus-ui launcher"
  if [[ $KEEP_EXTRAS -eq 0 ]]; then
    [[ -f "$bin/phoneinfoga" ]] && add file "$bin/phoneinfoga" $sudo "phoneinfoga (installed by install.sh if it was missing)"
    [[ -f "$bin/subfinder" ]] && add file "$bin/subfinder" $sudo "subfinder (installed by install.sh if it was missing)"
  fi
  [[ -d "$data" ]] && add dir "$data" $sudo "private data: helper tools (virtualenvs), and the source copy if installed from a pipe"
  if [[ $REMOVE_GO -eq 1 && -x "$goroot/bin/go" && -f "$goroot/VERSION" ]]; then add dir "$goroot" $sudo "the Go toolchain"; fi
  return 0
}

FOUND=0
case "$FORCE_MODE" in
  user)   plan_install user "$HOME/.local/bin" "${XDG_DATA_HOME:-$HOME/.local/share}/argus" "$HOME/.local/go" && FOUND=1 || true ;;
  system) plan_install system /usr/local/bin /usr/local/share/argus /usr/local/go && FOUND=1 || true ;;
  prefix) plan_install prefix "$PREFIX/bin" "$PREFIX/share/argus" "$PREFIX/go" && FOUND=1 || true ;;
  *)
    # the install recorded next to the source tree wins; otherwise look in both default places
    if [[ -n "$SRC_DIR" && -f "$SRC_DIR/.argus-install.conf" ]]; then
      cm="" cp="" cd="" cb=""
      while IFS='=' read -r k v; do
        [[ "$k" =~ ^[A-Z_]+$ ]] || continue; v="${v%\"}"; v="${v#\"}"
        case "$k" in MODE) cm="$v" ;; PREFIX) cp="$v" ;; DATA_DIR) cd="$v" ;; BIN_DIR) cb="$v" ;; esac
      done < "$SRC_DIR/.argus-install.conf"
      if [[ -n "$cb" && -n "$cd" ]]; then
        goroot=/usr/local/go; [[ "$cm" == user ]] && goroot="$HOME/.local/go"; [[ -n "$cp" ]] && goroot="$cp/go"
        plan_install "${cm:-user}" "$cb" "$cd" "$goroot" && FOUND=1 || true
      fi
    fi
    if [[ $FOUND -eq 0 ]]; then
      plan_install user "$HOME/.local/bin" "${XDG_DATA_HOME:-$HOME/.local/share}/argus" "$HOME/.local/go" && FOUND=1 || true
      plan_install system /usr/local/bin /usr/local/share/argus /usr/local/go && FOUND=1 || true
    fi
    ;;
esac

# any other copy on PATH the installer could have made (informational only)
if have argus; then
  here="$(command -v argus)"
  listed=0; for a in "${PLAN[@]:-}"; do [[ "$a" == "file|$here|"* ]] && listed=1; done
  [[ $listed -eq 0 ]] && NOTES+=("another argus is on your PATH at $here: it was not installed by this script, so it was left alone")
fi

# automatic updates (systemd timer / cron entry)
AUTO_NOTE=""
for scope in user system; do
  if [[ "$scope" == user ]]; then d="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"; else d=/etc/systemd/system; fi
  if [[ -f "$d/argus-update.timer" || -f "$d/argus-update.service" ]]; then
    s=0; [[ "$scope" == system && $EUID -ne 0 ]] && s=1
    add autosys "$d#$scope" $s "automatic-update schedule (systemd timer)"
  fi
done
cron_has() { have crontab && { crontab -l 2>/dev/null | grep -q "# argus-auto-update"; }; }
if cron_has; then add cron "user" 0 "automatic-update schedule (your crontab)"; fi
if [[ $EUID -ne 0 && -n "$SUDO" ]] && have crontab && $SUDO -n true 2>/dev/null && { $SUDO crontab -l 2>/dev/null | grep -q "# argus-auto-update"; }; then
  add cron "system" 1 "automatic-update schedule (root crontab)"
fi

# PATH lines added by install.sh
for rc in "$HOME/.profile" "$HOME/.bashrc" "$HOME/.zshrc"; do
  if [[ -f "$rc" ]] && grep -q "# added by SUDOSOC ARGUS" "$rc" 2>/dev/null; then add rcline "$rc" 0 "PATH line added by install.sh"; fi
done

# install record next to the source tree
if [[ -n "$SRC_DIR" && -f "$SRC_DIR/.argus-install.conf" ]]; then add file "$SRC_DIR/.argus-install.conf" 0 "install record (update.sh uses it)"; fi

# personal data
CASES="${ARGUS_CASES:-${XDG_CONFIG_HOME:-$HOME/.config}/argus/cases}"
CONF_DIR="${ARGUS_SETTINGS_DIR:-${XDG_CONFIG_HOME:-$HOME/.config}/argus}"
STATE_DIR="${XDG_STATE_HOME:-$HOME/.local/state}/argus"
KEPT=()
if [[ $PURGE -eq 1 ]]; then
  [[ -d "$CASES" ]] && add dir "$CASES" 0 "YOUR SAVED CASES (personal data)"
  [[ -d "$CONF_DIR" ]] && add dir "$CONF_DIR" 0 "YOUR SETTINGS AND ENCRYPTED API KEYS (and the key file beside them)"
  [[ -d "$STATE_DIR" ]] && add dir "$STATE_DIR" 0 "update log"
else
  [[ -d "$CASES" ]] && KEPT+=("saved cases: $CASES")
  [[ -d "$CONF_DIR" ]] && KEPT+=("settings and API keys: $CONF_DIR")
  [[ -d "$STATE_DIR" ]] && KEPT+=("update log: $STATE_DIR")
fi

if [[ ${#PLAN[@]} -eq 0 ]]; then
  info "nothing to remove: no ARGUS installation was found."
  [[ ${#KEPT[@]} -gt 0 ]] && { info "your data is still on disk (use --purge to delete it):"; printf '    %s\n' "${KEPT[@]}"; }
  exit 0
fi

# ------------------------------------------------------------- show + ask -----
info "this will remove:"
for a in "${PLAN[@]}"; do IFS='|' read -r kind path sudo note <<<"$a"; printf '    %-9s %s   (%s)\n' "$kind" "${path//#/ [scope: }" "$note"; done
if [[ ${#KEPT[@]} -gt 0 ]]; then info "and will KEEP (use --purge to delete):"; printf '    %s\n' "${KEPT[@]}"; fi
for n in "${NOTES[@]:-}"; do [[ -n "$n" ]] && warn "$n"; done
[[ $DRY -eq 1 ]] && { info "dry run: nothing was removed."; exit 0; }

if [[ $YES -eq 0 ]]; then
  [[ -t 0 ]] || die "not a terminal: pass --yes to confirm (or --dry-run to preview)"
  read -r -p "Remove these? [y/N] " ans
  [[ "$ans" == y || "$ans" == Y || "$ans" == yes ]] || { info "cancelled."; exit 0; }
  if [[ $PURGE -eq 1 ]]; then
    warn "--purge deletes your saved cases and API keys. This cannot be undone."
    read -r -p "Type DELETE to confirm: " ans2
    [[ "$ans2" == DELETE ]] || { info "cancelled."; exit 0; }
  fi
fi

# ----------------------------------------------------------------- do it ------
priv() { if [[ "$1" == 1 && $EUID -ne 0 ]]; then shift; if [[ -n "$SUDO" ]]; then $SUDO "$@"; else return 1; fi; else shift; "$@"; fi; }
FAILED=0
for a in "${PLAN[@]}"; do
  IFS='|' read -r kind path sudo note <<<"$a"
  case "$kind" in
    file)
      if priv "$sudo" rm -f -- "$path"; then ok "removed $path"; else warn "could not remove $path (needs sudo?)"; FAILED=1; fi ;;
    dir)
      if safe_path "$path"; then
        if priv "$sudo" rm -rf -- "$path"; then ok "removed $path"; else warn "could not remove $path"; FAILED=1; fi
      else warn "refusing to remove '$path'"; FAILED=1; fi ;;
    autosys)
      d="${path%%#*}"; scope="${path##*#}"
      if [[ "$scope" == user ]]; then
        have systemctl && systemctl --user disable --now argus-update.timer >/dev/null 2>&1 || true
        rm -f -- "$d/argus-update.timer" "$d/argus-update.service"; have systemctl && systemctl --user daemon-reload >/dev/null 2>&1 || true
      else
        have systemctl && priv 1 systemctl disable --now argus-update.timer >/dev/null 2>&1 || true
        priv 1 rm -f -- "$d/argus-update.timer" "$d/argus-update.service"; have systemctl && priv 1 systemctl daemon-reload >/dev/null 2>&1 || true
      fi
      ok "removed the automatic-update timer ($scope)" ;;
    cron)
      if [[ "$path" == system ]]; then { $SUDO crontab -l 2>/dev/null | grep -v "# argus-auto-update" | $SUDO crontab -; } && ok "removed the root crontab entry" || warn "could not edit the root crontab"
      else { crontab -l 2>/dev/null | grep -v "# argus-auto-update" | crontab -; } && ok "removed the crontab entry" || warn "could not edit your crontab"; fi ;;
    rcline)
      sed -i '/# added by SUDOSOC ARGUS/,+1d' "$path" && ok "cleaned $path" || { warn "could not edit $path"; FAILED=1; } ;;
  esac
done

echo
if [[ $FAILED -eq 0 ]]; then ok "SUDOSOC ARGUS was removed."; else warn "finished with some items that could not be removed (see above)."; fi
if [[ ${#KEPT[@]} -gt 0 ]]; then info "kept (your data): $(printf '%s; ' "${KEPT[@]}")"; fi
[[ -n "$SRC_DIR" && -f "$SRC_DIR/go.mod" ]] && info "the source folder $SRC_DIR was left in place: delete it yourself if you no longer need it."
cat <<'EOF'
[*] OS packages that install.sh may have added were NOT removed (other software can depend on them). To remove them:
      Debian/Ubuntu/Kali:  sudo apt remove --autoremove tesseract-ocr tesseract-ocr-ara ffmpeg exiftool whatweb theharvester
      Fedora/RHEL:         sudo dnf remove tesseract ffmpeg perl-Image-ExifTool
      Arch:                sudo pacman -Rns tesseract ffmpeg perl-image-exiftool
    (keep python3, git, curl, whois and dnsutils unless you are sure nothing else needs them.)
[*] Open a new shell so your PATH is refreshed.
EOF
exit $FAILED
