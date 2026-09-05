# Tiny Fleet — Quick Start Guide

Get your LAN inventory up and running in under two minutes across your Linux servers and macOS machines.

---

## 1. Quick Install

### Option A: One-Line Installer (Recommended)

Run on any machine on your network (Linux Debian/Ubuntu, or macOS Apple Silicon / Intel):

```sh
curl -sSL https://raw.githubusercontent.com/ryan-lang/tiny-fleet/main/install.sh | bash
```

Or using `wget`:

```sh
wget -qO- https://raw.githubusercontent.com/ryan-lang/tiny-fleet/main/install.sh | bash
```

- **Linux (Debian/Ubuntu)**: Installs the `.deb` package via `apt` and automatically starts `fleet.service`.
- **macOS (Apple Silicon & Intel)**: Installs `fleet` to `/usr/local/bin/fleet`. To also enable the background agent at boot:
  ```sh
  curl -sSL https://raw.githubusercontent.com/ryan-lang/tiny-fleet/main/install.sh | bash -s -- --service
  ```

---

### Option B: Build From Source

Clone the repo and run:

```sh
# Build binary for current platform
make build

# Or package both macOS Intel and Apple Silicon tarballs
make darwin

# Or package Debian .deb
make deb
```

---

## 2. Managing the Background Agent

### On Linux (systemd)

```sh
sudo systemctl status fleet.service
sudo systemctl restart fleet.service
sudo journalctl -u fleet.service -f
```

### On macOS (launchd)

```sh
# Start background service
sudo cp io.github.ryan-lang.tiny-fleet.plist /Library/LaunchDaemons/
sudo launchctl bootstrap system /Library/LaunchDaemons/io.github.ryan-lang.tiny-fleet.plist

# Inspect logs
tail -f /var/log/tiny-fleet.log

# Stop service
sudo launchctl bootout system /Library/LaunchDaemons/io.github.ryan-lang.tiny-fleet.plist
```

### Running in Foreground (Any Machine)

You can always run the agent directly in a terminal without a background daemon:

```sh
fleet agent
```

---

## 3. Discover Your Fleet

On any machine on the same LAN (Mac or Linux), run:

### Compact Overview

```sh
fleet
```

Output:

```
HOST          IP             OS                 VIRT   CPU                 RAM    GPU
workstation   192.168.4.10   Ubuntu 26.04 LTS   none   Ryzen 9 9950X3D2   128G   RTX 4090
macbook-pro   192.168.4.15   macOS 15.0         none   Apple M3 Max        64G    Apple M3 Max
pve1          192.168.4.20   Proxmox VE 9       none   Xeon E-2288G       64G    -
build01       192.168.4.31   Ubuntu 24.04 LTS   kvm    8 vCPU             32G    -
postgres      192.168.4.32   Debian 13           lxc    4 vCPU             16G    -
```

### Detailed Host Profile

```sh
fleet info macbook-pro
```

Output:

```
macbook-pro
  Address:     192.168.4.15
  OS:          macOS 15.0
  Kernel:      24.0.0
  Virtualized: no
  CPU:         Apple M3 Max
  CPUs:        16
  RAM:         64 GiB

  GPUs:
    Apple M3 Max

  Storage:
    disk0   APPLE SSD AP1024Z   931 GiB
```

### Scripting with JSON

```sh
# List all discovered hosts
fleet --json | jq -r '.[].hostname'

# Extract IP, OS, and RAM for all nodes
fleet --json | jq '.[] | {host: .hostname, os: .os, ip: .ip, ram_gb: (.memory_bytes / 1073741824)}'

# Inspect raw facts for one machine
fleet info macbook-pro --json
```

---

## 4. Network Checklist

Tiny Fleet requires no central database or registration token. Ensure the following network traffic is permitted on your LAN:

- **UDP Port 5353**: Multicast DNS (`224.0.0.251` / `ff02::fb`)
- **TCP Port 9753**: HTTP facts endpoint (`/v1/info`)

Firewall configuration if needed:
- **Linux (UFW)**: `sudo ufw allow 5353/udp && sudo ufw allow 9753/tcp`
- **macOS**: Allow incoming connections when prompted or via System Settings > Network > Firewall.
