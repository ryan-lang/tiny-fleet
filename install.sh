#!/usr/bin/env bash
set -euo pipefail

REPO="ryan-lang/tiny-fleet"
VERSION="${VERSION:-latest}"
ENABLE_SERVICE="${ENABLE_SERVICE:-false}"

# Parse optional arguments
for arg in "$@"; do
    case "${arg}" in
        --service|--enable-service)
            ENABLE_SERVICE="true"
            ;;
        --help|-h)
            echo "Tiny Fleet Installer"
            echo "Usage: install.sh [--service]"
            echo "  --service, --enable-service: Automatically configure and start background service"
            exit 0
            ;;
    esac
done

# Determine OS
OS="$(uname -s)"
case "${OS}" in
    Linux)
        TARGET_OS="linux"
        ;;
    Darwin)
        TARGET_OS="darwin"
        ;;
    *)
        echo "Error: unsupported operating system: ${OS}" >&2
        exit 1
        ;;
esac

# Determine architecture
RAW_ARCH="$(uname -m)"
case "${RAW_ARCH}" in
    x86_64|amd64)
        TARGET_ARCH="amd64"
        ;;
    arm64|aarch64)
        TARGET_ARCH="arm64"
        ;;
    *)
        echo "Error: unsupported architecture: ${RAW_ARCH}" >&2
        exit 1
        ;;
esac

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

echo "==> Fetching latest Tiny Fleet release info for ${TARGET_OS}/${TARGET_ARCH}..."

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
        VERSION="0.2.0"
    fi
fi

CLEAN_VER="${VERSION#v}"

# Setup portable temporary directory and cleanup trap
TMP_DIR="$(mktemp -d 2>/dev/null || mktemp -d -t 'tiny-fleet')"
trap 'rm -rf "${TMP_DIR}"' EXIT INT TERM

download_file() {
    local url="$1"
    local dest="$2"
    if [ "${downloader}" = "curl" ]; then
        curl -fsSL "${url}" -o "${dest}"
    else
        wget -qO "${dest}" "${url}"
    fi
}

verify_checksum() {
    local file_path="$1"
    local filename
    filename="$(basename "${file_path}")"
    local sums_url="https://github.com/${REPO}/releases/download/v${CLEAN_VER}/checksums.txt"
    local sums_file="${TMP_DIR}/checksums.txt"

    if download_file "${sums_url}" "${sums_file}" 2>/dev/null; then
        local expected_sum
        expected_sum="$(grep -E "[[:space:]]${filename}\$" "${sums_file}" | awk '{print $1}' || true)"
        if [ -n "${expected_sum}" ]; then
            local actual_sum=""
            if command -v sha256sum >/dev/null 2>&1; then
                actual_sum="$(sha256sum "${file_path}" | awk '{print $1}')"
            elif command -v shasum >/dev/null 2>&1; then
                actual_sum="$(shasum -a 256 "${file_path}" | awk '{print $1}')"
            fi
            if [ -n "${actual_sum}" ]; then
                if [ "${actual_sum}" != "${expected_sum}" ]; then
                    echo "Error: SHA-256 checksum mismatch for ${filename}!" >&2
                    echo "Expected: ${expected_sum}" >&2
                    echo "Actual:   ${actual_sum}" >&2
                    exit 1
                fi
                echo "==> Verified SHA-256 checksum for ${filename}."
            fi
        fi
    fi
}

SUDO=""
get_sudo() {
    if [ "$(id -u)" -ne 0 ]; then
        if command -v sudo >/dev/null 2>&1; then
            SUDO="sudo"
        else
            echo "Error: root privileges or sudo required." >&2
            exit 1
        fi
    fi
}

if [ "${TARGET_OS}" = "linux" ]; then
    get_sudo
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/v${CLEAN_VER}/tiny-fleet_${CLEAN_VER}_${TARGET_ARCH}.deb"
    PKG_FILE="${TMP_DIR}/tiny-fleet_${CLEAN_VER}_${TARGET_ARCH}.deb"

    echo "==> Downloading ${DOWNLOAD_URL}..."
    download_file "${DOWNLOAD_URL}" "${PKG_FILE}"
    verify_checksum "${PKG_FILE}"

    echo "==> Installing Tiny Fleet via apt..."
    $SUDO apt-get update -qq
    $SUDO apt-get install -y "${PKG_FILE}"

    echo "==> Installation complete! Checking fleet agent status..."
    if command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]; then
        if systemctl is-active --quiet fleet.service; then
            echo "==> fleet.service is active and running."
        else
            echo "==> Note: fleet.service is installed. Start it with: sudo systemctl start fleet.service"
        fi
    fi

elif [ "${TARGET_OS}" = "darwin" ]; then
    ARCHIVE_NAME="tiny-fleet_${CLEAN_VER}_darwin_${TARGET_ARCH}.tar.gz"
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/v${CLEAN_VER}/${ARCHIVE_NAME}"
    ARCHIVE_FILE="${TMP_DIR}/${ARCHIVE_NAME}"

    echo "==> Downloading ${DOWNLOAD_URL}..."
    download_file "${DOWNLOAD_URL}" "${ARCHIVE_FILE}"
    verify_checksum "${ARCHIVE_FILE}"

    echo "==> Extracting archive..."
    tar -xzf "${ARCHIVE_FILE}" -C "${TMP_DIR}"

    INSTALL_DIR="/usr/local/bin"
    if [ ! -d "${INSTALL_DIR}" ] || [ ! -w "${INSTALL_DIR}" ]; then
        get_sudo
        $SUDO mkdir -p "${INSTALL_DIR}"
        $SUDO cp "${TMP_DIR}/fleet" "${INSTALL_DIR}/fleet"
        $SUDO chmod 0755 "${INSTALL_DIR}/fleet"
    else
        cp "${TMP_DIR}/fleet" "${INSTALL_DIR}/fleet"
        chmod 0755 "${INSTALL_DIR}/fleet"
    fi
    echo "==> Installed fleet to ${INSTALL_DIR}/fleet"

    # LaunchDaemon setup
    PLIST_FILE="io.github.ryan-lang.tiny-fleet.plist"
    if [ -f "${TMP_DIR}/${PLIST_FILE}" ]; then
        if [ "${ENABLE_SERVICE}" = "true" ]; then
            get_sudo
            echo "==> Configuring LaunchDaemon..."
            $SUDO cp "${TMP_DIR}/${PLIST_FILE}" "/Library/LaunchDaemons/${PLIST_FILE}"
            $SUDO chown root:wheel "/Library/LaunchDaemons/${PLIST_FILE}"
            $SUDO chmod 0644 "/Library/LaunchDaemons/${PLIST_FILE}"
            $SUDO launchctl bootout system "/Library/LaunchDaemons/${PLIST_FILE}" 2>/dev/null || true
            $SUDO launchctl bootstrap system "/Library/LaunchDaemons/${PLIST_FILE}" 2>/dev/null || $SUDO launchctl load -w "/Library/LaunchDaemons/${PLIST_FILE}"
            echo "==> Tiny Fleet agent LaunchDaemon enabled and started!"
        else
            echo
            echo "==> Background Service (Optional):"
            echo "To run the Tiny Fleet agent in the background at boot via launchd:"
            echo "  sudo cp \"${TMP_DIR}/${PLIST_FILE}\" /Library/LaunchDaemons/ (or copy from repo)"
            echo "  sudo launchctl bootstrap system /Library/LaunchDaemons/${PLIST_FILE}"
            echo "Or re-run the installer with: curl -fsSL ... | bash -s -- --service"
        fi
    fi
fi

echo
echo "Tiny Fleet installed successfully! Run 'fleet' to discover local machines."
