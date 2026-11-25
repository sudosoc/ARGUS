#!/usr/bin/env bash
# SUDOSOC ARGUS updater: pulls the latest source, rebuilds, reinstalls and refreshes the helper tools.
#
#   ./update.sh                    update everything now (same install mode and options as before)
#   ./update.sh --check            only report whether a newer version exists
#   ./update.sh --fast             skip OS packages (quicker; what the scheduled job uses)
#   ./update.sh --force            rebuild even if already up to date
#   ./update.sh --enable-auto [daily|weekly]   update automatically (systemd timer, or cron)
#   ./update.sh --disable-auto     stop automatic updates
#   ./update.sh --status           show version, install mode and the automatic-update schedule
#   argus -update                  the same as ./update.sh, from anywhere
#
# Local changes in the source tree are stashed for a manual run and left alone (the update is
# skipped) by the scheduled run. If the build fails the previous version stays installed.
# Your reports, saved cases, proxy lists and custom site files are never touched.

set -Eeuo pipefail
SRC_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SRC_DIR"

# ------------------------------------------------------------------ output --
QUIET=0
if [[ -t 1 ]]; then B=$'\e[1m' G=$'\e[32m' Y=$'\e[33m' R=$'\e[31m' N=$'\e[0m'; else B="" G="" Y="" R="" N=""; fi
LOG=""
say()  { local line="$1"; [[ -n "$LOG" ]] && printf '%s %s\n' "$(date '+%F %T')" "$line" >> "$LOG" 2>/dev/null || true; }
info() { say "[*] $*"; [[ $QUIET -eq 1 ]] || printf '%s[*]%s %s\n' "$B" "$N" "$*"; }
ok()   { say "[+] $*"; [[ $QUIET -eq 1 ]] || printf '%s[+]%s %s\n' "$G" "$N" "$*"; }
warn() { say "[!] $*"; printf '%s[!]%s %s\n' "$Y" "$N" "$*" >&2; }
die()  { say "[x] $*"; printf '%s[x]%s %s\n' "$R" "$N" "$*" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

# ----------------------------------------------------------------- options --
CHECK=0 FAST=0 FORCE=0 NOTOOLS=0 ACTION=update AUTO_FREQ=daily
PASS=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    --check) CHECK=1 ;;
    --fast) FAST=1 ;;
    --force) FORCE=1 ;;
    --no-tools) NOTOOLS=1 ;;
    --quiet) QUIET=1 ;;
    --status) ACTION=status ;;
    --disable-auto) ACTION=disable ;;
    --enable-auto) ACTION=enable; if [[ "${2:-}" == daily || "${2:-}" == weekly ]]; then AUTO_FREQ="$2"; shift; fi ;;
    -h|--help) sed -n '2,15p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) PASS+=("$1") ;;                  # anything else goes to install.sh (e.g. --user)
  esac
  shift
done

# ---------------------------------------------------- how it was installed --
MODE="" PREFIX="" DEPS=1 EXTRAS=1 BROWSER=1 DATA_DIR="" BIN_DIR=""
if [[ -f "$SRC_DIR/.argus-install.conf" ]]; then
  while IFS='=' read -r k v; do
    [[ "$k" =~ ^[A-Z_]+$ ]] || continue
    v="${v%\"}"; v="${v#\"}"
    case "$k" in MODE) MODE="$v" ;; PREFIX) PREFIX="$v" ;; DEPS) DEPS="$v" ;; EXTRAS) EXTRAS="$v" ;; BROWSER) BROWSER="$v" ;; DATA_DIR) DATA_DIR="$v" ;; BIN_DIR) BIN_DIR="$v" ;; esac
  done < "$SRC_DIR/.argus-install.conf"
fi
if [[ -z "$MODE" ]]; then if [[ $EUID -eq 0 ]]; then MODE=system; else MODE=user; fi; fi
[[ -n "$DATA_DIR" ]] || { if [[ "$MODE" == system ]]; then DATA_DIR=/usr/local/share/argus; else DATA_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/argus"; fi; }
[[ -n "$BIN_DIR" ]] || { if [[ "$MODE" == system ]]; then BIN_DIR=/usr/local/bin; else BIN_DIR="$HOME/.local/bin"; fi; }
SUDO=""; if [[ "$MODE" == system && $EUID -ne 0 ]]; then if have sudo; then SUDO="sudo"; fi; fi

LOGDIR="${XDG_STATE_HOME:-$HOME/.local/state}/argus"
if mkdir -p "$LOGDIR" 2>/dev/null; then LOG="$LOGDIR/update.log"; fi

# ------------------------------------------------------ automatic updates --
SERVICE=argus-update
CRON_TAG="# argus-auto-update"
use_systemd() { have systemctl && [[ -d /run/systemd/system ]]; }

unit_dir() { if [[ "$MODE" == system ]]; then echo /etc/systemd/system; else echo "${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"; fi; }
sctl() { if [[ "$MODE" == system ]]; then $SUDO systemctl "$@"; else systemctl --user "$@"; fi; }

write_unit() { # write_unit PATH  (content on stdin)
  if [[ "$MODE" == system ]]; then $SUDO tee "$1" >/dev/null; else tee "$1" >/dev/null; fi
}

enable_auto() {
  local cal="$AUTO_FREQ"
  if use_systemd; then
    local d; d="$(unit_dir)"
    if [[ "$MODE" == system ]]; then $SUDO mkdir -p "$d"; else mkdir -p "$d"; fi
    write_unit "$d/$SERVICE.service" <<EOF
[Unit]
Description=Update SUDOSOC ARGUS
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
Environment=HOME=$HOME
ExecStart=/bin/bash "$SRC_DIR/update.sh" --quiet --fast
Nice=10
EOF
    write_unit "$d/$SERVICE.timer" <<EOF
[Unit]
Description=Update SUDOSOC ARGUS ($cal)

[Timer]
OnCalendar=$cal
RandomizedDelaySec=1h
Persistent=true

[Install]
WantedBy=timers.target
EOF
    sctl daemon-reload
    sctl enable --now "$SERVICE.timer" >/dev/null
    ok "automatic update enabled ($cal, systemd timer $SERVICE.timer)"
    if [[ "$MODE" == user ]]; then info "to let it run while you are logged out: sudo loginctl enable-linger $USER"; fi
    return
  fi
  if have crontab; then
    local when="17 4 * * *"; [[ "$cal" == weekly ]] && when="17 4 * * 1"
    local cur; cur="$(crontab_get | grep -v "$CRON_TAG" || true)"
    printf '%s\n%s bash "%s/update.sh" --quiet --fast %s\n' "$cur" "$when" "$SRC_DIR" "$CRON_TAG" | sed '/^$/d' | crontab_set
    ok "automatic update enabled ($cal, cron: $when)"
    return
  fi
  die "neither systemd nor cron is available: schedule \"bash $SRC_DIR/update.sh --quiet --fast\" yourself"
}

crontab_get() { if [[ "$MODE" == system && $EUID -ne 0 && -n "$SUDO" ]]; then $SUDO crontab -l 2>/dev/null || true; else crontab -l 2>/dev/null || true; fi; }
crontab_set() { if [[ "$MODE" == system && $EUID -ne 0 && -n "$SUDO" ]]; then $SUDO crontab -; else crontab -; fi; }

disable_auto() {
  local d f
  if use_systemd; then
    d="$(unit_dir)"
    if [[ -f "$d/$SERVICE.timer" ]]; then
      sctl disable --now "$SERVICE.timer" >/dev/null 2>&1 || true
      for f in "$d/$SERVICE.timer" "$d/$SERVICE.service"; do
        if [[ "$MODE" == system ]]; then $SUDO rm -f -- "$f"; else rm -f -- "$f"; fi
      done
      sctl daemon-reload || true
      ok "automatic update disabled (systemd timer removed)"
    fi
  fi
  if have crontab && crontab_get | grep -q "$CRON_TAG"; then
    crontab_get | grep -v "$CRON_TAG" | crontab_set || true
    ok "automatic update disabled (cron entry removed)"
  fi
  return 0
}

auto_status() {
  if use_systemd && [[ -f "$(unit_dir)/$SERVICE.timer" ]]; then
    echo "automatic update: ON (systemd) $(sctl list-timers "$SERVICE.timer" --no-legend 2>/dev/null | head -1 | sed 's/  */ /g')"
  elif have crontab && crontab_get | grep -q "$CRON_TAG"; then
    echo "automatic update: ON (cron) $(crontab_get | grep "$CRON_TAG" | sed "s/ $CRON_TAG//")"
  else
    echo "automatic update: off   (enable with: ./update.sh --enable-auto)"
  fi
}

case "$ACTION" in
  enable)  enable_auto; exit 0 ;;
  disable) disable_auto; exit 0 ;;
  status)
    echo "SUDOSOC ARGUS source:  $SRC_DIR ($(git rev-parse --short HEAD 2>/dev/null || echo 'not a git checkout'))"
    echo "install mode:          $MODE (binary $BIN_DIR/argus, data $DATA_DIR)"
    if [[ -x "$BIN_DIR/argus" ]]; then echo "installed version:     $("$BIN_DIR/argus" -version)"; fi
    auto_status
    [[ -n "$LOG" && -f "$LOG" ]] && echo "last log lines ($LOG):" && tail -n 5 "$LOG" | sed 's/^/    /'
    exit 0 ;;
esac

# ----------------------------------------------------------------- locking --
if have flock; then
  exec 9>"$SRC_DIR/.update.lock"
  flock -n 9 || { warn "another update is already running"; exit 0; }
fi

# ------------------------------------------------------------ preconditions --
have git || die "git is required (apt install git / dnf install git / pacman -S git)"
[[ -d .git ]] || die "$SRC_DIR is not a git checkout. Re-install with: git clone https://github.com/sudosoc/ARGUS && cd ARGUS && ./install.sh"

old="$(git rev-parse --short HEAD)"
branch="$(git rev-parse --abbrev-ref HEAD)"
[[ "$branch" != HEAD ]] || die "detached HEAD: check out a branch first (git checkout main)"
info "current version: $old ($(git log -1 --format=%cs))"
info "fetching updates..."
retry_fetch() { local i; for i in 1 2 3; do git fetch --quiet origin && return 0; sleep $((i * 3)); done; return 1; }
retry_fetch || die "cannot reach the remote (network down?)"
behind="$(git rev-list --count "HEAD..origin/$branch" 2>/dev/null || echo 0)"
dirty=0; if ! git diff --quiet || ! git diff --cached --quiet; then dirty=1; fi

installed_commit=""
if [[ -x "$BIN_DIR/argus" ]]; then installed_commit="$("$BIN_DIR/argus" -version 2>/dev/null | sed -n 's/.*(\([0-9a-f]\{7,\})[,)].*/\1/p')"; fi

if [[ "$behind" -gt 0 ]]; then
  info "$behind new commit(s):"
  git --no-pager log --oneline "HEAD..origin/$branch" | head -20 | sed 's/^/    /'
fi
[[ $CHECK -eq 1 ]] && { [[ "$behind" -eq 0 ]] && ok "already up to date" || info "run ./update.sh to install them"; exit 0; }

# --------------------------------------------------------------- pull + build --
stashed=0
if [[ "$behind" -gt 0 ]]; then
  if [[ $dirty -eq 1 ]]; then
    if [[ $QUIET -eq 1 ]]; then warn "local changes in $SRC_DIR: scheduled update skipped (run ./update.sh by hand)"; exit 0; fi
    warn "you have local changes; stashing them for the update"
    git stash push --quiet -m "argus update $(date +%F-%H%M)"; stashed=1
  fi
  git pull --ff-only --quiet origin "$branch" || { [[ $stashed -eq 1 ]] && git stash pop --quiet || true; die "cannot fast-forward (your branch has diverged); resolve it with: git pull"; }
elif [[ $FORCE -eq 0 && -n "$installed_commit" && "$installed_commit" == "$old" ]]; then
  ok "already up to date ($old)"
  SKIP_BUILD=1
fi

new="$(git rev-parse --short HEAD)"
args=()
[[ "$MODE" == system ]] && args+=(--system) || args+=(--user)
[[ -n "$PREFIX" ]] && args+=(--prefix "$PREFIX")
[[ "$DEPS" == 0 || $FAST -eq 1 ]] && args+=(--no-deps)
[[ "$EXTRAS" == 0 ]] && args+=(--no-extras)
[[ "$BROWSER" == 0 ]] && args+=(--no-browser)
args+=(--no-path "${PASS[@]}")

if [[ "${SKIP_BUILD:-0}" -eq 0 ]]; then
  info "rebuilding and reinstalling ($([[ " ${args[*]} " == *" --no-deps "* ]] && echo 'OS packages skipped' || echo 'OS packages and helper tools refreshed'))..."
  if ! bash ./install.sh "${args[@]}" 2>&1 | { if [[ $QUIET -eq 1 ]]; then cat >> "${LOG:-/dev/null}"; else cat; fi; }; then
    warn "the update failed: restoring the previous version of the source ($old)"
    git reset --hard "$old" >/dev/null 2>&1 || true
    [[ $stashed -eq 1 ]] && git stash pop --quiet 2>/dev/null || true
    die "update failed; the previously installed version is unchanged"
  fi
fi
[[ $stashed -eq 1 ]] && { git stash pop --quiet || warn "stash pop conflicted; your changes are in 'git stash list'"; }

# ------------------------------------------------ refresh the helper tools --
upgrade_tools() {
  local venv tool pkg
  if [[ -d "$DATA_DIR/tools" ]]; then
    for venv in "$DATA_DIR/tools"/*/; do
      tool="$(basename "$venv")"
      case "$tool" in
        sherlock) pkg=sherlock-project ;;
        theHarvester) pkg="git+https://github.com/laramies/theHarvester.git" ;;
        *) pkg="$tool" ;;
      esac
      [[ -x "$venv/bin/pip" ]] || continue
      if $SUDO "$venv/bin/pip" install -q --disable-pip-version-check --upgrade "$pkg" >/dev/null 2>&1; then ok "helper tool up to date: $tool"
      else warn "could not upgrade $tool"; fi
    done
  fi
  if have pipx; then
    for pkg in maigret sherlock-project holehe; do
      if pipx list --short 2>/dev/null | grep -q "^$pkg "; then pipx upgrade "$pkg" >/dev/null 2>&1 && ok "helper tool up to date: $pkg (pipx)" || true; fi
    done
  fi
}
if [[ $NOTOOLS -eq 0 ]]; then upgrade_tools; fi

# ------------------------------------------------------------------- verify --
if [[ -x "$BIN_DIR/argus" ]]; then
  "$BIN_DIR/argus" -version >/dev/null || die "the installed binary does not run"
  ok "$("$BIN_DIR/argus" -version)"
  [[ $QUIET -eq 1 ]] || PATH="$BIN_DIR:$PATH" "$BIN_DIR/argus" -doctor | sed -n '/^Optional tools/,$p'
fi
if [[ "$new" != "$old" ]]; then ok "updated $old -> $new"; else ok "no code changes (still $old)"; fi
[[ $QUIET -eq 1 ]] || auto_status
