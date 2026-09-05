# Tiny Fleet

A zero-configuration LAN inventory tool where every machine announces itself via mDNS/DNS-SD and can be discovered instantly with:

```sh
fleet
```

Example output:

```
HOST          IP             OS                 VIRT   CPU                 RAM    GPU
workstation   192.168.4.10   Ubuntu 26.04 LTS   none   Ryzen 9 9950X3D2   128G   RTX 4090
pve1          192.168.4.20   Proxmox VE 9       none   Xeon E-2288G       64G    -
build01       192.168.4.31   Ubuntu 24.04 LTS   kvm    8 vCPU             32G    -
postgres      192.168.4.32   Debian 13           lxc    4 vCPU             16G    -
```

## Architecture

Tiny Fleet consists of a single binary (`fleet`) that operates in two modes:

1. **`fleet agent`**:
   - Gathers local hardware and OS facts (`/etc/os-release`, `uname`, `lscpu`, `/proc/meminfo`, `lspci`, `lsblk`, cgroup limits).
   - Starts a lightweight HTTP server on port `9753` exposing `/v1/info`.
   - Advertises `_fleet._tcp.local` via mDNS with minimal TXT records (`v=1`, `os=...`, `virt=...`).
   - Refreshes dynamic information periodically (memory and IP every 60s, disks every 5m, hardware at boot).
   - Unregisters cleanly on shutdown.

2. **`fleet` / `fleet info <host>`**:
   - Browses `_fleet._tcp.local` across all local network interfaces.
   - Concurrently queries `http://<host>:9753/v1/info` for each discovered node.
   - Renders a clean compact table or detailed per-host report.
   - Supports `--json` for machine readability and scripting.

No database, no central server, no static host lists. If a machine powers on, it appears automatically. If it shuts down, it unregisters.

---

## Installation

### One-Line Install (Debian / Ubuntu)

Install on any Debian or Ubuntu node with a single command. It downloads the latest release `.deb`, installs dependencies via `apt`, starts `fleet.service`, and deletes the downloaded archive automatically:

```sh
curl -sSL https://raw.githubusercontent.com/ryan-lang/tiny-fleet/main/install.sh | bash
```

Or using `wget`:

```sh
wget -qO- https://raw.githubusercontent.com/ryan-lang/tiny-fleet/main/install.sh | bash
```

### Local Build & Install

Building the Debian package locally:

```sh
make deb
sudo apt install ./tiny-fleet_0.1.1_amd64.deb
```

The package automatically:
- Installs `/usr/bin/fleet`
- Installs `/lib/systemd/system/fleet.service`
- Enables and starts the `fleet.service` background agent

### Manual Installation

Build the binary:

```sh
make build
sudo cp fleet /usr/bin/fleet
sudo cp fleet.service /etc/systemd/system/fleet.service
sudo systemctl daemon-reload
sudo systemctl enable --now fleet.service
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
fleet info workstation
```

Example output:

```
workstation
  Address:     192.168.4.10
  OS:          Ubuntu 26.04 LTS
  Kernel:      6.8.0-137-generic
  Virtualized: no
  CPU:         AMD Ryzen 9 9950X3D2
  CPUs:        32
  RAM:         128 GiB

  GPUs:
    NVIDIA GeForce RTX 4090

  Storage:
    nvme0n1   WD_BLACK SN8100 4TB       3.64 TiB
    nvme1n1   Samsung SSD 980 1TB      931 GiB
```

### Detailed Host Profile (JSON)

```sh
fleet info workstation --json
```

Outputs the raw JSON profile:

```json
{
  "hostname": "workstation",
  "os": "Ubuntu 26.04 LTS",
  "kernel": "6.8.0-137-generic",
  "arch": "x86_64",
  "virtualization": "none",
  "cpu": "AMD Ryzen 9 9950X3D2",
  "logical_cpus": 32,
  "memory_bytes": 137438953472,
  "gpus": [
    "NVIDIA GeForce RTX 4090"
  ],
  "disks": [
    {
      "name": "nvme0n1",
      "model": "WD_BLACK SN8100 4TB",
      "size_bytes": 4000398934016
    },
    {
      "name": "nvme1n1",
      "model": "Samsung SSD 980 1TB",
      "size_bytes": 1000204886016
    }
  ],
  "ip": "192.168.4.10"
}
```

---

## Container & Cgroup Limits

On virtualized hosts (LXC, Docker, KVM, etc.), Tiny Fleet accounts for container resource constraints:
- **Memory**: Checks cgroup v2 (`memory.max`) and cgroup v1 (`memory.limit_in_bytes`). If capped below host memory, the container's allocated memory is reported.
- **CPU Quota & Cpusets**: Checks cgroup v2 (`cpu.max`, `cpuset.cpus.effective`) and cgroup v1 (`cpu.cfs_quota_us`, `cpuset.cpus`). The effective vCPU capacity is displayed in the table (e.g. `4 vCPU`).

---

## Network Requirements

Tiny Fleet requires standard link-local multicast and TCP connectivity:
- **UDP port 5353**: Multicast DNS (`224.0.0.251` / `ff02::fb`) for discovery
- **TCP port 9753**: HTTP `/v1/info` endpoint
