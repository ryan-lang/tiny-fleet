package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/grandcat/zeroconf"
)

func TestEndToEndDiscoveryAndInfo(t *testing.T) {
	// 1. Create two fake fleet agents
	info1 := MachineInfo{
		Hostname:       "node-alpha",
		OS:             "Ubuntu 24.04 LTS",
		Kernel:         "6.8.0-generic",
		Arch:           "x86_64",
		Virtualization: "none",
		CPU:            "AMD Ryzen 9 7950X",
		LogicalCPUs:    16,
		MemoryBytes:    32 * 1024 * 1024 * 1024,
		GPUs:           []string{"NVIDIA GeForce RTX 4090"},
		Disks: []DiskInfo{
			{Name: "nvme0n1", Model: "Samsung SSD 980 1TB", SizeBytes: 1000204886016},
		},
		IP: "127.0.0.1",
	}

	info2 := MachineInfo{
		Hostname:       "node-beta",
		OS:             "Debian 13",
		Kernel:         "6.6.0-amd64",
		Arch:           "x86_64",
		Virtualization: "lxc",
		CPU:            "AMD Ryzen 9 7950X",
		LogicalCPUs:    4,
		MemoryBytes:    16 * 1024 * 1024 * 1024,
		GPUs:           []string{},
		Disks:          []DiskInfo{},
		IP:             "127.0.0.1",
	}

	server1 := http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(info1)
		}),
	}
	server2 := http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(info2)
		}),
	}

	l1, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer l1.Close()
	port1 := l1.Addr().(*net.TCPAddr).Port
	go server1.Serve(l1)
	defer server1.Close()

	l2, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer l2.Close()
	port2 := l2.Addr().(*net.TCPAddr).Port
	go server2.Serve(l2)
	defer server2.Close()

	// Register with mDNS
	mdns1, err := zeroconf.Register("node-alpha", "_fleet._tcp", "local.", port1, []string{"v=1", "os=ubuntu", "virt=none"}, nil)
	if err != nil {
		t.Logf("Skipping mDNS registration test if multicast not supported: %v", err)
		return
	}
	defer mdns1.Shutdown()

	mdns2, err := zeroconf.Register("node-beta", "_fleet._tcp", "local.", port2, []string{"v=1", "os=debian", "virt=lxc"}, nil)
	if err != nil {
		t.Logf("Skipping mDNS registration test: %v", err)
		return
	}
	defer mdns2.Shutdown()

	// Allow mDNS announcements to settle
	time.Sleep(500 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	records, err := DiscoverAll(ctx, 2*time.Second)
	if err != nil {
		t.Fatalf("DiscoverAll failed: %v", err)
	}

	foundAlpha := false
	foundBeta := false
	for _, r := range records {
		if r.Hostname == "node-alpha" {
			foundAlpha = true
			if r.Info == nil {
				t.Errorf("node-alpha info is nil, error: %s", r.Error)
			} else {
				if r.Info.OS != "Ubuntu 24.04 LTS" {
					t.Errorf("node-alpha OS = %s; want Ubuntu 24.04 LTS", r.Info.OS)
				}
				if len(r.Info.GPUs) != 1 || r.Info.GPUs[0] != "NVIDIA GeForce RTX 4090" {
					t.Errorf("node-alpha GPUs = %v; want [NVIDIA GeForce RTX 4090]", r.Info.GPUs)
				}
			}
		}
		if r.Hostname == "node-beta" {
			foundBeta = true
			if r.Info == nil {
				t.Errorf("node-beta info is nil, error: %s", r.Error)
			} else {
				if r.Info.Virtualization != "lxc" {
					t.Errorf("node-beta virt = %s; want lxc", r.Info.Virtualization)
				}
				if r.Info.LogicalCPUs != 4 {
					t.Errorf("node-beta CPUs = %d; want 4", r.Info.LogicalCPUs)
				}
			}
		}
	}

	if !foundAlpha {
		t.Errorf("node-alpha was not discovered via mDNS")
	}
	if !foundBeta {
		t.Errorf("node-beta was not discovered via mDNS")
	}

	// Test FindHost
	targetRecord, err := FindHost(context.Background(), "node-alpha", 2*time.Second)
	if err != nil {
		t.Fatalf("FindHost failed: %v", err)
	}
	if targetRecord.Hostname != "node-alpha" {
		t.Errorf("targetRecord hostname = %s; want node-alpha", targetRecord.Hostname)
	}

	// Test PrintCompactTable with live discovered records
	var buf os.File
	_ = buf
	PrintCompactTable(os.Stdout, records)
	fmt.Println()
	PrintDetailedInfo(os.Stdout, targetRecord)
}
