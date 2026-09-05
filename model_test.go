package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFormatRAMCompact(t *testing.T) {
	tests := []struct {
		bytes    uint64
		expected string
	}{
		{128 * 1024 * 1024 * 1024, "128G"},
		{64 * 1024 * 1024 * 1024, "64G"},
		{32 * 1024 * 1024 * 1024, "32G"},
		{16 * 1024 * 1024 * 1024, "16G"},
		{8 * 1024 * 1024 * 1024, "8G"},
		{512 * 1024 * 1024, "512M"},
		{1000, "1000 B"},
	}

	for _, tt := range tests {
		actual := FormatRAMCompact(tt.bytes)
		if actual != tt.expected {
			t.Errorf("FormatRAMCompact(%d) = %s; want %s", tt.bytes, actual, tt.expected)
		}
	}
}

func TestFormatRAMDetailed(t *testing.T) {
	tests := []struct {
		bytes    uint64
		expected string
	}{
		{128 * 1024 * 1024 * 1024, "128 GiB"},
		{16 * 1024 * 1024 * 1024, "16 GiB"},
		{512 * 1024 * 1024, "512 MiB"},
	}

	for _, tt := range tests {
		actual := FormatRAMDetailed(tt.bytes)
		if actual != tt.expected {
			t.Errorf("FormatRAMDetailed(%d) = %s; want %s", tt.bytes, actual, tt.expected)
		}
	}
}

func TestFormatDiskDetailed(t *testing.T) {
	tests := []struct {
		bytes    uint64
		expected string
	}{
		{4000398934016, "3.64 TiB"},
		{1000204886016, "931 GiB"},
		{511868665856, "476 GiB"},
	}

	for _, tt := range tests {
		actual := FormatDiskDetailed(tt.bytes)
		if actual != tt.expected {
			t.Errorf("FormatDiskDetailed(%d) = %s; want %s", tt.bytes, actual, tt.expected)
		}
	}
}

func TestCleanGPUName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"NVIDIA GeForce RTX 4090", "RTX 4090"},
		{"Intel Corporation CometLake-S GT2 [UHD Graphics 630]", "CometLake-S GT2 [UHD Graphics 630]"},
		{"AMD/ATI Radeon RX 7900 XTX", "Radeon RX 7900 XTX"},
		{"", "-"},
	}

	for _, tt := range tests {
		actual := CleanGPUName(tt.input)
		if actual != tt.expected {
			t.Errorf("CleanGPUName(%q) = %q; want %q", tt.input, actual, tt.expected)
		}
	}
}

func TestMarshalMachineInfo(t *testing.T) {
	info := &MachineInfo{
		Hostname: "test-node",
		GPUs:     nil, // must serialize as []
		Disks:    nil, // must serialize as []
	}

	data, err := MarshalMachineInfo(info)
	if err != nil {
		t.Fatalf("MarshalMachineInfo failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	gpus, ok := parsed["gpus"].([]interface{})
	if !ok || len(gpus) != 0 {
		t.Errorf("expected empty array for gpus, got %#v", parsed["gpus"])
	}

	disks, ok := parsed["disks"].([]interface{})
	if !ok || len(disks) != 0 {
		t.Errorf("expected empty array for disks, got %#v", parsed["disks"])
	}

	// Verify indentation
	str := string(data)
	if !strings.Contains(str, "\n  \"hostname\": \"test-node\"") {
		t.Errorf("expected indented JSON, got: %s", str)
	}
}
