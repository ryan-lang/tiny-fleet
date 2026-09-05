#!/usr/bin/env bash
set -euo pipefail

VERSION="${VERSION:-0.1.0}"
ARCH="${ARCH:-amd64}"
PKG_NAME="tiny-fleet_${VERSION}_${ARCH}"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
STAGE_DIR="/tmp/${PKG_NAME}"

echo "==> Building fleet binary for ${ARCH}..."
cd "${ROOT_DIR}"
CGO_ENABLED=0 GOOS=linux GOARCH="${ARCH}" go build -ldflags="-s -w -X main.Version=${VERSION}" -o fleet .

echo "==> Preparing Debian package directory structure at ${STAGE_DIR}..."
rm -rf "${STAGE_DIR}"
mkdir -p "${STAGE_DIR}/DEBIAN"
mkdir -p "${STAGE_DIR}/usr/bin"
mkdir -p "${STAGE_DIR}/lib/systemd/system"

# Binary
cp fleet "${STAGE_DIR}/usr/bin/fleet"
chmod 0755 "${STAGE_DIR}/usr/bin/fleet"

# Systemd service
cp "${ROOT_DIR}/fleet.service" "${STAGE_DIR}/lib/systemd/system/fleet.service"
chmod 0644 "${STAGE_DIR}/lib/systemd/system/fleet.service"

# Debian control file with declared dependencies
cat <<EOF > "${STAGE_DIR}/DEBIAN/control"
Package: tiny-fleet
Version: ${VERSION}
Section: admin
Priority: optional
Architecture: ${ARCH}
Depends: util-linux, pciutils, systemd
Maintainer: Tiny Fleet Maintainers <fleet@local>
Description: Zero-configuration LAN inventory tool
 Fleet is a zero-configuration LAN inventory tool where every machine
 announces itself via mDNS/DNS-SD and can be discovered with a single
 command.
EOF
chmod 0644 "${STAGE_DIR}/DEBIAN/control"

# Post-installation script (enables and starts systemd service if available)
cat <<'EOF' > "${STAGE_DIR}/DEBIAN/postinst"
#!/bin/sh
set -e

if [ "$1" = "configure" ]; then
    if [ -d /run/systemd/system ]; then
        systemctl daemon-reload
        systemctl enable fleet.service
        systemctl restart fleet.service || true
    fi
fi
exit 0
EOF
chmod 0755 "${STAGE_DIR}/DEBIAN/postinst"

# Pre-removal script (stops and disables systemd service)
cat <<'EOF' > "${STAGE_DIR}/DEBIAN/prerm"
#!/bin/sh
set -e

if [ "$1" = "remove" ] || [ "$1" = "deconfigure" ]; then
    if [ -d /run/systemd/system ]; then
        systemctl stop fleet.service || true
        systemctl disable fleet.service || true
    fi
fi
exit 0
EOF
chmod 0755 "${STAGE_DIR}/DEBIAN/prerm"

# Post-removal script (daemon-reload)
cat <<'EOF' > "${STAGE_DIR}/DEBIAN/postrm"
#!/bin/sh
set -e

if [ "$1" = "purge" ] || [ "$1" = "remove" ]; then
    if [ -d /run/systemd/system ]; then
        systemctl daemon-reload || true
    fi
fi
exit 0
EOF
chmod 0755 "${STAGE_DIR}/DEBIAN/postrm"

echo "==> Building .deb package..."
dpkg-deb --build --root-owner-group "${STAGE_DIR}" "${ROOT_DIR}/${PKG_NAME}.deb"

rm -rf "${STAGE_DIR}"
echo "==> Successfully created ${ROOT_DIR}/${PKG_NAME}.deb"
