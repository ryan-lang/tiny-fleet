package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintCompactTable(t *testing.T) {
	// Recreate the four machines from the specification example
	records := []*HostRecord{
		{
			Hostname: "workstation",
			Address:  "192.168.4.10",
			Info: &MachineInfo{
				Hostname:       "workstation",
				OS:             "Ubuntu 26.04 LTS",
				Virtualization: "none",
				CPU:            "AMD Ryzen 9 9950X3D2",
				LogicalCPUs:    32,
				MemoryBytes:    128 * 1024 * 1024 * 1024,
				GPUs:           []string{"NVIDIA GeForce RTX 4090"},
			},
		},
		{
			Hostname: "pve1",
			Address:  "192.168.4.20",
			Info: &MachineInfo{
				Hostname:       "pve1",
				OS:             "Proxmox VE 9",
				Virtualization: "none",
				CPU:            "Intel(R) Xeon(R) CPU E-2288G @ 3.70GHz",
				LogicalCPUs:    16,
				MemoryBytes:    64 * 1024 * 1024 * 1024,
				GPUs:           []string{},
			},
		},
		{
			Hostname: "build01",
			Address:  "192.168.4.31",
			Info: &MachineInfo{
				Hostname:       "build01",
				OS:             "Ubuntu 24.04 LTS",
				Virtualization: "kvm",
				CPU:            "AMD Ryzen 9 7950X",
				LogicalCPUs:    8,
				MemoryBytes:    32 * 1024 * 1024 * 1024,
				GPUs:           []string{},
			},
		},
		{
			Hostname: "postgres",
			Address:  "192.168.4.32",
			Info: &MachineInfo{
				Hostname:       "postgres",
				OS:             "Debian 13",
				Virtualization: "lxc",
				CPU:            "AMD Ryzen 9 7950X",
				LogicalCPUs:    4,
				MemoryBytes:    16 * 1024 * 1024 * 1024,
				GPUs:           []string{},
			},
		},
	}

	var buf bytes.Buffer
	PrintCompactTable(&buf, records)
	out := buf.String()

	// Verify Header
	if !strings.Contains(out, "HOST") || !strings.Contains(out, "VIRT") || !strings.Contains(out, "GPU") {
		t.Errorf("missing headers in output:\n%s", out)
	}

	// Verify workstation row
	if !strings.Contains(out, "workstation") || !strings.Contains(out, "192.168.4.10") ||
		!strings.Contains(out, "128G") || !strings.Contains(out, "RTX 4090") {
		t.Errorf("missing workstation details in output:\n%s", out)
	}

	// Verify build01 virtualized CPU displays as "8 vCPU"
	if !strings.Contains(out, "8 vCPU") {
		t.Errorf("expected '8 vCPU' for build01 in output:\n%s", out)
	}

	// Verify postgres virtualized CPU displays as "4 vCPU"
	if !strings.Contains(out, "4 vCPU") {
		t.Errorf("expected '4 vCPU' for postgres in output:\n%s", out)
	}

	// Verify pve1 shows "-" for GPU
	if !strings.Contains(out, "Proxmox VE 9") {
		t.Errorf("expected 'Proxmox VE 9' for pve1 in output:\n%s", out)
	}
}

func TestPrintDetailedInfo(t *testing.T) {
	record := &HostRecord{
		Hostname: "workstation",
		Address:  "192.168.4.10",
		Info: &MachineInfo{
			Hostname:       "workstation",
			OS:             "Ubuntu 26.04 LTS",
			Kernel:         "6.x",
			Virtualization: "none",
			CPU:            "AMD Ryzen 9 9950X3D2",
			LogicalCPUs:    32,
			MemoryBytes:    128 * 1024 * 1024 * 1024,
			GPUs:           []string{"NVIDIA GeForce RTX 4090"},
			Disks: []DiskInfo{
				{
					Name:      "nvme0n1",
					Model:     "WD_BLACK SN8100 4TB",
					SizeBytes: 4000398934016,
				},
				{
					Name:      "nvme1n1",
					Model:     "Samsung SSD 980 1TB",
					SizeBytes: 1000204886016,
				},
			},
		},
	}

	var buf bytes.Buffer
	PrintDetailedInfo(&buf, record)
	out := buf.String()

	expectedSubstrings := []string{
		"workstation",
		"Address:     192.168.4.10",
		"OS:          Ubuntu 26.04 LTS",
		"Kernel:      6.x",
		"Virtualized: no",
		"CPU:         AMD Ryzen 9 9950X3D2",
		"CPUs:        32",
		"RAM:         128 GiB",
		"NVIDIA GeForce RTX 4090",
		"nvme0n1",
		"WD_BLACK SN8100 4TB",
		"3.64 TiB",
		"nvme1n1",
		"Samsung SSD 980 1TB",
		"931 GiB",
	}

	for _, sub := range expectedSubstrings {
		if !strings.Contains(out, sub) {
			t.Errorf("detailed info missing expected text %q:\n%s", sub, out)
		}
	}
}
