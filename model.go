package main

import (
	"encoding/json"
	"fmt"
)

// DiskInfo contains information about a block storage device.
type DiskInfo struct {
	Name      string `json:"name"`
	Model     string `json:"model"`
	SizeBytes uint64 `json:"size_bytes"`
}

// MachineInfo is the complete hardware and OS profile exposed by an agent.
type MachineInfo struct {
	Hostname       string     `json:"hostname"`
	OS             string     `json:"os"`
	Kernel         string     `json:"kernel"`
	Arch           string     `json:"arch"`
	Virtualization string     `json:"virtualization"`
	CPU            string     `json:"cpu"`
	LogicalCPUs    int        `json:"logical_cpus"`
	CPULimit       *float64   `json:"cpu_limit,omitempty"`
	MemoryBytes    uint64     `json:"memory_bytes"`
	GPUs           []string   `json:"gpus"`
	Disks          []DiskInfo `json:"disks"`
	IP             string     `json:"ip,omitempty"`
}

// HostRecord represents a discovered host in the fleet.
type HostRecord struct {
	Hostname string       `json:"hostname"`
	Address  string       `json:"address"`
	Port     int          `json:"port"`
	AllIPs   []string     `json:"all_ips,omitempty"`
	Info     *MachineInfo `json:"info,omitempty"`
	Error    string       `json:"error,omitempty"`
}

// FormatRAMCompact formats memory for the compact table (e.g. "128G", "16G", "512M").
func FormatRAMCompact(bytes uint64) string {
	const gib = 1024 * 1024 * 1024
	const mib = 1024 * 1024

	if bytes >= gib {
		// Round to nearest GiB or one decimal if not close to integer
		gibVal := float64(bytes) / float64(gib)
		if gibVal >= 1.0 {
			rounded := float64(uint64(gibVal + 0.5))
			if (gibVal-rounded) > -0.05 && (gibVal-rounded) < 0.05 {
				return fmt.Sprintf("%.0fG", rounded)
			}
			return fmt.Sprintf("%.1fG", gibVal)
		}
	}
	if bytes >= mib {
		return fmt.Sprintf("%.0fM", float64(bytes)/float64(mib))
	}
	return fmt.Sprintf("%d B", bytes)
}

// FormatRAMDetailed formats memory for the detailed info view (e.g. "128 GiB", "16.0 GiB").
func FormatRAMDetailed(bytes uint64) string {
	const gib = 1024 * 1024 * 1024
	const mib = 1024 * 1024

	if bytes >= gib {
		gibVal := float64(bytes) / float64(gib)
		rounded := float64(uint64(gibVal + 0.5))
		if (gibVal-rounded) > -0.05 && (gibVal-rounded) < 0.05 {
			return fmt.Sprintf("%.0f GiB", rounded)
		}
		return fmt.Sprintf("%.1f GiB", gibVal)
	}
	if bytes >= mib {
		return fmt.Sprintf("%.0f MiB", float64(bytes)/float64(mib))
	}
	return fmt.Sprintf("%d Bytes", bytes)
}

// FormatDiskDetailed formats disk capacity for detailed view (e.g. "3.64 TiB", "931 GiB").
func FormatDiskDetailed(bytes uint64) string {
	const tib = 1024 * 1024 * 1024 * 1024
	const gib = 1024 * 1024 * 1024
	const mib = 1024 * 1024

	if bytes >= tib {
		return fmt.Sprintf("%.2f TiB", float64(bytes)/float64(tib))
	}
	if bytes >= gib {
		return fmt.Sprintf("%d GiB", bytes/gib)
	}
	if bytes >= mib {
		return fmt.Sprintf("%d MiB", bytes/mib)
	}
	return fmt.Sprintf("%d B", bytes)
}

// CleanGPUName cleans redundant prefixes for compact overview (e.g. "NVIDIA GeForce RTX 4090" -> "RTX 4090").
func CleanGPUName(gpu string) string {
	if gpu == "" {
		return "-"
	}
	// Common shortenings for compact table
	clean := gpu
	for _, prefix := range []string{
		"NVIDIA GeForce ",
		"NVIDIA Corporation ",
		"NVIDIA ",
		"Advanced Micro Devices, Inc. [AMD/ATI] ",
		"AMD/ATI ",
		"AMD ",
		"Intel Corporation ",
		"Intel ",
	} {
		if len(clean) > len(prefix) && clean[:len(prefix)] == prefix {
			clean = clean[len(prefix):]
			break
		}
	}
	if clean == "" {
		return gpu
	}
	return clean
}

// MarshalMachineInfo serializes MachineInfo to JSON with clean empty slices.
func MarshalMachineInfo(info *MachineInfo) ([]byte, error) {
	if info.GPUs == nil {
		info.GPUs = []string{}
	}
	if info.Disks == nil {
		info.Disks = []DiskInfo{}
	}
	return json.MarshalIndent(info, "", "  ")
}
