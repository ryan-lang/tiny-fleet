#!/usr/bin/env bash
set -euo pipefail

REPO="ryan-lang/tiny-fleet"
VERSION="${VERSION:-latest}"

# Determine architecture
ARCH="$(uname -m)"
case "${ARCH}" in
    x86_64|amd64)
        DEB_ARCH="amd64"
        ;;
    aarch64|arm64)
        DEB_ARCH="arm64"
        ;;
    *)
        echo "Error: unsupported architecture: ${ARCH}" >&2
        exit 1
        ;;
esac

# Check for root/sudo
SUDO=""
if [ "$(id -u)" -ne 0 ]; then
    if command -v sudo >/dev/null 2>&1; then
        SUDO="sudo"
    else
        echo "Error: root privileges or sudo required to install package." >&2
        exit 1
    fi
fi

# Determine download tool
downloader=""
if command -v curl >/dev/null 2>&1; then
    downloader="curl"
elif command -v wget >/dev/null 2>&1; then
    downloader="wget"
else
    echo "Error: neither curl nor wget found. Please install one first." >&2
    exit 1
fi

echo "==> Fetching latest Tiny Fleet release info for ${DEB_ARCH}..."

if [ "${VERSION}" = "latest" ]; then
    LATEST_TAG=""
    if [ "${downloader}" = "curl" ]; then
        LATEST_TAG="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | grep -m1 '"tag_name":' | cut -d '"' -f 4 || true)"
    else
        LATEST_TAG="$(wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | grep -m1 '"tag_name":' | cut -d '"' -f 4 || true)"
    fi
    if [ -n "${LATEST_TAG}" ]; then
        VERSION="${LATEST_TAG#v}"
    else
        VERSION="0.1.0"
    fi
fi

CLEAN_VER="${VERSION#v}"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/v${CLEAN_VER}/tiny-fleet_${CLEAN_VER}_${DEB_ARCH}.deb"

# Create a temporary file and ensure cleanup on exit
TMP_DEB="$(mktemp --suffix=.deb 2>/dev/null || mktemp /tmp/tiny-fleet-XXXXXX.deb)"
trap 'rm -f "${TMP_DEB}"' EXIT INT TERM

echo "==> Downloading ${DOWNLOAD_URL}..."
if [ "${downloader}" = "curl" ]; then
    curl -fsSL "${DOWNLOAD_URL}" -o "${TMP_DEB}"
else
    wget -qO "${TMP_DEB}" "${DOWNLOAD_URL}"
fi

echo "==> Installing Tiny Fleet and dependencies via apt..."
$SUDO apt-get update -qq
$SUDO apt-get install -y "${TMP_DEB}"

echo "==> Installation complete! Checking fleet agent status..."
if command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]; then
    if systemctl is-active --quiet fleet.service; then
        echo "==> fleet.service is active and running."
    else
        echo "==> Note: fleet.service is installed. Start it with: sudo systemctl start fleet.service"
    fi
fi

echo
echo "Tiny Fleet installed successfully! Run 'fleet' to discover local machines."
