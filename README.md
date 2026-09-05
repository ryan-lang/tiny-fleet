# Tiny Fleet

A zero-configuration LAN inventory tool where every machine announces itself via mDNS/DNS-SD and can be discovered instantly with:

```sh
fleet
```

Example output:

```
HOST          IP             OS                 VIRT   CPU                 RAM    GPU
workstation   192.168.4.10   Ubuntu 26.04 LTS   none   Ryzen 9 9950X3D2   128G   RTX 4090
macbook-pro   192.168.4.15   macOS 15.0         none   Apple M3 Max        64G    Apple M3 Max
pve1          192.168.4.20   Proxmox VE 9       none   Xeon E-2288G       64G    -
build01       192.168.4.31   Ubuntu 24.04 LTS   kvm    8 vCPU             32G    -
postgres      192.168.4.32   Debian 13           lxc    4 vCPU             16G    -
```

## Architecture

Tiny Fleet consists of a single binary (`fleet`) that operates in two modes:

1. **`fleet agent`**:
   - Gathers local hardware and OS facts:
     - **Linux**: `/etc/os-release`, `uname`, `lscpu`, `/proc/meminfo`, `lspci`, `lsblk`, cgroup limits.
     - **macOS**: `sw_vers`, `uname`, `sysctl` (`machdep.cpu.brand_string`, `hw.logicalcpu`, `hw.memsize`), `system_profiler SPDisplaysDataType`, `diskutil` physical drives.
   - Starts a lightweight HTTP server on port `9753` exposing `/v1/info`.
   - Advertises `_fleet._tcp.local` via mDNS with minimal TXT records (`v=1`, `os=...`, `virt=...`).
   - Refreshes dynamic facts periodically (memory and IP every 60s, disks every 5m, hardware at boot).
   - Re-registers mDNS automatically if network IP changes or if network becomes available after boot.
   - Unregisters cleanly on shutdown.

2. **`fleet` / `fleet info <host>`**:
   - Browses `_fleet._tcp.local` across all local network interfaces.
   - Concurrently queries `http://<host>:9753/v1/info` for each discovered node.
   - Renders a clean compact table or detailed per-host report.
   - Supports `--json` for machine readability and scripting.

No database, no central server, no static host lists. If a machine powers on, it appears automatically. If it shuts down, it unregisters.

---

## Installation

### One-Line Install (macOS, Debian, Ubuntu)

Install on any Mac (Apple Silicon or Intel) or Linux node with a single command:

```sh
curl -sSL https://raw.githubusercontent.com/ryan-lang/tiny-fleet/main/install.sh | bash
```

Or using `wget`:

```sh
wget -qO- https://raw.githubusercontent.com/ryan-lang/tiny-fleet/main/install.sh | bash
```

**What it does:**
- **On Linux (Debian/Ubuntu)**: Downloads the release `.deb`, verifies SHA-256 checksums, installs dependencies via `apt`, enables and starts `fleet.service` (systemd), and cleans up temporary files.
- **On macOS**: Detects architecture (`arm64` Apple Silicon or `amd64` Intel), downloads the release tarball, verifies SHA-256 checksums, and installs the binary to `/usr/local/bin/fleet`.

To automatically enable the background agent at boot on macOS during installation, add `--service`:

```sh
curl -sSL https://raw.githubusercontent.com/ryan-lang/tiny-fleet/main/install.sh | bash -s -- --service
```

---

### macOS Service Management (LaunchDaemon)

To run the agent as a system background service at boot on macOS:

```sh
# Copy LaunchDaemon plist
sudo cp io.github.ryan-lang.tiny-fleet.plist /Library/LaunchDaemons/
sudo chown root:wheel /Library/LaunchDaemons/io.github.ryan-lang.tiny-fleet.plist
sudo chmod 0644 /Library/LaunchDaemons/io.github.ryan-lang.tiny-fleet.plist

# Start service
sudo launchctl bootstrap system /Library/LaunchDaemons/io.github.ryan-lang.tiny-fleet.plist

# Stop / uninstall service
sudo launchctl bootout system /Library/LaunchDaemons/io.github.ryan-lang.tiny-fleet.plist
sudo rm /Library/LaunchDaemons/io.github.ryan-lang.tiny-fleet.plist
```

Agent logs are written to `/var/log/tiny-fleet.log`.

---

### Local Build

Build the binary for your current machine:

```sh
make build
```

Build macOS release archives (both Intel and Apple Silicon):

```sh
make darwin
```

Build Debian package:

```sh
make deb
sudo apt install ./tiny-fleet_0.2.0_amd64.deb
```

---

## CLI Usage

### Fleet Overview

```sh
fleet
```

Displays a compact summary of all machines discovered on the local network.

### Machine-Readable Fleet Overview

```sh
fleet --json
```

Outputs a JSON array containing full inventory details for every discovered machine.

### Detailed Host Profile

```sh
fleet info macbook-pro
```

Example output:

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

### Detailed Host Profile (JSON)

```sh
fleet info macbook-pro --json
```

---

## Container & Cgroup Limits (Linux)

On virtualized hosts (LXC, Docker, KVM, etc.), Tiny Fleet accounts for container resource constraints:
- **Memory**: Checks cgroup v2 (`memory.max`) and cgroup v1 (`memory.limit_in_bytes`). If capped below host memory, the container's allocated memory is reported.
- **CPU Quota & Cpusets**: Checks cgroup v2 (`cpu.max`, `cpuset.cpus.effective`) and cgroup v1 (`cpu.cfs_quota_us`, `cpuset.cpus`). The effective vCPU capacity is displayed in the table (e.g. `4 vCPU`).

---

## Network Requirements

Tiny Fleet requires standard link-local multicast and TCP connectivity:
- **UDP port 5353**: Multicast DNS (`224.0.0.251` / `ff02::fb`) for discovery
- **TCP port 9753**: HTTP `/v1/info` endpoint

If local firewalls are enabled:
- **Linux (UFW)**: `sudo ufw allow 5353/udp && sudo ufw allow 9753/tcp`
- **macOS**: Allow incoming connections for `fleet` in System Settings > Network > Firewall, and allow Local Network access when prompted.
