# Tiny Fleet — Quick Start Guide

Get your LAN inventory up and running in under two minutes across your Debian and Ubuntu servers.

---

## 1. Build the Package

Run `make deb` on your build or management machine:

```sh
cd /path/to/tiny-fleet
make deb
```

This compiles a static Go binary and packages `tiny-fleet_0.1.0_amd64.deb`.

---

## 2. Deploy Agents to Your Machines

Copy the `.deb` to each machine on your local network (e.g. via `scp`):

```sh
scp tiny-fleet_0.1.0_amd64.deb user@target-node:/tmp/
```

On each machine, install with `apt`:

```sh
sudo apt update
sudo apt install /tmp/tiny-fleet_0.1.0_amd64.deb
```

> **Why `apt` instead of `dpkg -i`?**  
> `apt` automatically installs declared hardware dependencies (`util-linux`, `pciutils`, `systemd`) if any are missing.

The package immediately enables and starts the systemd service:

```sh
systemctl status fleet.service
```

---

## 3. Alternative: Running Without Installation

You can also run the agent directly on any machine without installing the package:

```sh
# Start agent in background or terminal
./fleet agent &
```

---

## 4. Discover Your Fleet

On any machine on the same LAN (workstation, laptop, or server), run:

### Compact Overview

```sh
fleet
```

Output:

```
HOST          IP             OS                 VIRT   CPU                 RAM    GPU
workstation   192.168.4.10   Ubuntu 26.04 LTS   none   Ryzen 9 9950X3D2   128G   RTX 4090
pve1          192.168.4.20   Proxmox VE 9       none   Xeon E-2288G       64G    -
build01       192.168.4.31   Ubuntu 24.04 LTS   kvm    8 vCPU             32G    -
postgres      192.168.4.32   Debian 13           lxc    4 vCPU             16G    -
```

### Detailed Host Profile

```sh
fleet info build01
```

Output:

```
build01
  Address:     192.168.4.31
  OS:          Ubuntu 24.04.4 LTS
  Kernel:      6.8.0-137-generic
  Virtualized: kvm
  CPU:         AMD Ryzen 9 7950X
  CPUs:        8
  RAM:         32 GiB

  GPUs:
    -

  Storage:
    nvme0n1    Samsung SSD 980 PRO 2TB    1.82 TiB
```

### Scripting with JSON

```sh
# List all discovered hosts
fleet --json | jq -r '.[].hostname'

# Extract IP and RAM for all nodes
fleet --json | jq '.[] | {host: .hostname, ip: .ip, ram_gb: (.memory_bytes / 1073741824)}'

# Inspect raw facts for one machine
fleet info build01 --json
```

---

## 5. Network Checklist

Tiny Fleet requires no central database or registration token. Ensure the following network traffic is permitted on your LAN:

- **UDP Port 5353**: Multicast DNS (`224.0.0.251` / `ff02::fb`)
- **TCP Port 9753**: HTTP facts endpoint (`/v1/info`)

If a machine has a local firewall (e.g. UFW):

```sh
sudo ufw allow 5353/udp
sudo ufw allow 9753/tcp
```
