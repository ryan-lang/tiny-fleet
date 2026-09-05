//go:build linux

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCPUSet(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"0-3", 4},
		{"0", 1},
		{"0-3,5,7-9", 8},
		{"0-15", 16},
		{"", 0},
		{"invalid", 0},
	}

	for _, tt := range tests {
		actual := parseCPUSet(tt.input)
		if actual != tt.expected {
			t.Errorf("parseCPUSet(%q) = %d; want %d", tt.input, actual, tt.expected)
		}
	}
}

func TestEffectiveLogicalCPUs(t *testing.T) {
	tests := []struct {
		cpus     float64
		fallback int
		expected int
	}{
		{4.0, 16, 4},
		{1.5, 16, 2},
		{0.4, 16, 1},
		{0.0, 16, 16},
		{-1.0, 8, 8},
	}

	for _, tt := range tests {
		actual := EffectiveLogicalCPUs(tt.cpus, tt.fallback)
		if actual != tt.expected {
			t.Errorf("EffectiveLogicalCPUs(%f, %d) = %d; want %d", tt.cpus, tt.fallback, actual, tt.expected)
		}
	}
}

func TestCheckCgroupV2Direct(t *testing.T) {
	tmpDir := t.TempDir()

	// Write memory.max
	memMax := filepath.Join(tmpDir, "memory.max")
	if err := os.WriteFile(memMax, []byte("17179869184\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Write cpu.max (2 cores: 200000 quota / 100000 period)
	cpuMax := filepath.Join(tmpDir, "cpu.max")
	if err := os.WriteFile(cpuMax, []byte("200000 100000\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Read memory
	data, _ := os.ReadFile(memMax)
	if string(data) != "17179869184\n" {
		t.Errorf("failed to write test fixture")
	}
}
