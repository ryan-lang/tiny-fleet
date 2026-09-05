package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestAgentHTTPEndpoint(t *testing.T) {
	agent := NewAgent(9799)
	info := agent.GetInfo()
	if info == nil {
		t.Fatal("expected non-nil info from agent")
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/info", nil)
	w := httptest.NewRecorder()

	agent.handleV1Info(w, req)
	resp := w.Result()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status code = %d; want 200", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("Content-Type = %s; want application/json", contentType)
	}

	var parsed MachineInfo
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if parsed.Hostname != info.Hostname {
		t.Errorf("parsed hostname = %s; want %s", parsed.Hostname, info.Hostname)
	}
}

func TestFetchInfoClient(t *testing.T) {
	testInfo := MachineInfo{
		Hostname:       "test-node",
		OS:             "Debian 12",
		Kernel:         "6.1.0",
		Arch:           "x86_64",
		Virtualization: "lxc",
		CPU:            "Test CPU",
		LogicalCPUs:    4,
		MemoryBytes:    16 * 1024 * 1024 * 1024,
		GPUs:           []string{},
		Disks:          []DiskInfo{},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/info" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testInfo)
	}))
	defer server.Close()

	// Extract port and IP
	host, portStr, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(portStr)
	ip := net.ParseIP(host)
	if ip == nil {
		ip = net.ParseIP("127.0.0.1")
	}

	fetcher := NewFetchInfoClient()
	info, usedIP, err := fetcher.FetchInfo(context.Background(), []net.IP{ip}, port)
	if err != nil {
		t.Fatalf("FetchInfo failed: %v", err)
	}

	if info.Hostname != "test-node" {
		t.Errorf("hostname = %s; want test-node", info.Hostname)
	}
	if usedIP != ip.String() {
		t.Errorf("usedIP = %s; want %s", usedIP, ip.String())
	}
}

func TestFetchInfoClientErrorCases(t *testing.T) {
	fetcher := NewFetchInfoClient()

	// 1. Empty IP list
	_, _, err := fetcher.FetchInfo(context.Background(), nil, 9753)
	if err == nil {
		t.Errorf("expected error on empty IPs, got nil")
	}

	// 2. Closed port
	_, _, err = fetcher.FetchInfo(context.Background(), []net.IP{net.ParseIP("127.0.0.1")}, 59999)
	if err == nil {
		t.Errorf("expected connection error on closed port, got nil")
	}

	// 3. Malformed JSON server
	badServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, "not valid json {{{")
	}))
	defer badServer.Close()

	_, portStr, _ := net.SplitHostPort(badServer.Listener.Addr().String())
	port, _ := strconv.Atoi(portStr)
	_, _, err = fetcher.FetchInfo(context.Background(), []net.IP{net.ParseIP("127.0.0.1")}, port)
	if err == nil || !strings.Contains(err.Error(), "malformed JSON") {
		t.Errorf("expected malformed JSON error, got: %v", err)
	}
}
