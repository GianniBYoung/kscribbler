#!/bin/sh
# Over-the-air update for kscribbler.
#
# Usage: upgrade.sh [stable|testing|<tag>] [--force]
#   stable   latest published release (default)
#   testing  rolling pre-release built from a branch by the testing workflow
#   <tag>    a specific release, e.g. v1.4.0 (pin or roll back)
# Without an argument, UPDATE_CHANNEL from config.env is used.
# --force reinstalls even when the build is already installed.
#
# NickelMenu's cmd_output kills commands after ~10s, so unless run from a terminal
# the download continues in the background and Nickel prompts to install when done.

REPO="https://github.com/GianniBYoung/kscribbler/releases"
KSDIR="/mnt/onboard/.adds/kscribbler"
CONFIG="$KSDIR/config.env"
INSTALLED="$KSDIR/installed-checksums.txt"
TGZ="/mnt/onboard/.kobo/KoboRoot.tgz"
TMP="/tmp/kscribbler-update"
LOG="$KSDIR/kscribbler.log"

CHANNEL=""
FORCE=0
for arg in "$@"; do
  case "$arg" in
    --force) FORCE=1 ;;
    *) CHANNEL="$arg" ;;
  esac
done

if [ -z "$CHANNEL" ] && [ -f "$CONFIG" ]; then
  CHANNEL=$(sed -n 's/^UPDATE_CHANNEL="\{0,1\}\([^"]*\)"\{0,1\}[[:space:]]*$/\1/p' "$CONFIG" | tail -n 1)
fi
CHANNEL=${CHANNEL:-stable}

case "$CHANNEL" in
  stable) BASE="$REPO/latest/download" ;;
  *) BASE="$REPO/download/$CHANNEL" ;;
esac

if ! ping -c 1 8.8.8.8 > /dev/null 2>&1; then
  echo "no internet detected"
  exit 1
fi

rm -rf "$TMP" && mkdir -p "$TMP"
if ! wget -q -O "$TMP/checksums.txt" "$BASE/checksums.txt"; then
  echo "no '$CHANNEL' build found"
  exit 1
fi

# Builds installed before installed-checksums.txt existed: compare against the running version's release.
if [ ! -f "$INSTALLED" ]; then
  CURRENT_VERSION=$(/opt/bin/kscribbler --version)
  wget -q -O "$INSTALLED" "$REPO/download/$CURRENT_VERSION/checksums.txt" || rm -f "$INSTALLED"
fi

if [ "$FORCE" -eq 0 ] && [ -f "$INSTALLED" ] && cmp -s "$TMP/checksums.txt" "$INSTALLED"; then
  echo "kscribbler is already up to date ($CHANNEL)"
  exit 0
fi

install_build() {
  if ! wget -q -O "$TMP/KoboRoot.tgz" "$BASE/KoboRoot.tgz"; then
    echo "download failed"
    return 1
  fi

  EXPECTED=$(grep 'KoboRoot.tgz' "$TMP/checksums.txt" | cut -d ' ' -f 1)
  if command -v sha256sum > /dev/null 2>&1; then
    ACTUAL=$(sha256sum "$TMP/KoboRoot.tgz" | cut -d ' ' -f 1)
    if [ -z "$EXPECTED" ] || [ "$EXPECTED" != "$ACTUAL" ]; then
      echo "checksum mismatch, not installing"
      return 1
    fi
  else
    echo "warning: sha256sum not available, skipping checksum verification"
  fi

  mv "$TMP/KoboRoot.tgz" "$TGZ" && cp "$TMP/checksums.txt" "$INSTALLED" || return 1
  dbus-send --system /com/kobo/nickel com.kobo.nickel.NICKEL.upgradeCheck
  echo "Downloaded $CHANNEL build. Please trigger a kobo sync and follow any on-screen instructions."
}

if [ -t 1 ]; then
  install_build
  exit $?
fi

echo "Downloading $CHANNEL build in the background."
echo "Your Kobo will prompt to install when it's ready (progress in kscribbler.log)."
( echo "$(date) upgrade.sh: installing $CHANNEL build"; install_build ) >> "$LOG" 2>&1 < /dev/null &
