package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/grandcat/zeroconf"
)

const DefaultPort = 9753

// Agent manages the local facts collector, HTTP server, and mDNS announcement.
type Agent struct {
	port       int
	collector  *Collector
	mu         sync.RWMutex
	info       *MachineInfo
	httpServer *http.Server
	mdnsServer *zeroconf.Server
}

// NewAgent creates a new Agent instance.
func NewAgent(port int) *Agent {
	if port <= 0 {
		port = DefaultPort
	}
	collector := NewCollector()
	initialInfo := collector.Collect()

	return &Agent{
		port:      port,
		collector: collector,
		info:      initialInfo,
	}
}

// GetInfo returns a safe copy of the current MachineInfo.
func (a *Agent) GetInfo() *MachineInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()
	copy := *a.info
	if a.info.CPULimit != nil {
		lim := *a.info.CPULimit
		copy.CPULimit = &lim
	}
	copy.GPUs = append([]string(nil), a.info.GPUs...)
	copy.Disks = append([]DiskInfo(nil), a.info.Disks...)
	return &copy
}

// Run starts the agent HTTP server, announces via mDNS, and refreshes dynamic info.
func (a *Agent) Run(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", a.port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to bind HTTP server to %s: %w", addr, err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/info", a.handleV1Info)
	mux.HandleFunc("/", a.handleRoot)

	a.httpServer = &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 1. Start HTTP server
	httpErrCh := make(chan error, 1)
	go func() {
		if err := a.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			httpErrCh <- err
		}
	}()

	currentInfo := a.GetInfo()
	log.Printf("[fleet agent] HTTP server listening on %s", addr)
	log.Printf("[fleet agent] Host: %s (%s, %s, virt: %s)",
		currentInfo.Hostname, currentInfo.OS, currentInfo.CPU, currentInfo.Virtualization)

	// 2. Announce via mDNS
	txtRecords := []string{
		"v=1",
		fmt.Sprintf("os=%s", a.collector.GetOSID()),
		fmt.Sprintf("virt=%s", a.collector.GetVirt()),
	}

	mdnsServer, err := zeroconf.Register(
		currentInfo.Hostname,
		"_fleet._tcp",
		"local.",
		a.port,
		txtRecords,
		nil,
	)
	if err != nil {
		log.Printf("[fleet agent] Warning: mDNS registration error: %v", err)
	} else {
		a.mdnsServer = mdnsServer
		log.Printf("[fleet agent] Registered mDNS service _fleet._tcp.local as '%s' with TXT %v",
			currentInfo.Hostname, txtRecords)
	}

	// 3. Dynamic facts refresh tickers
	ticker60s := time.NewTicker(60 * time.Second)
	ticker5m := time.NewTicker(5 * time.Minute)
	defer ticker60s.Stop()
	defer ticker5m.Stop()

	// 4. Main event loop
	for {
		select {
		case <-ctx.Done():
			log.Println("[fleet agent] Received shutdown signal, terminating...")
			return a.shutdown()
		case err := <-httpErrCh:
			return fmt.Errorf("HTTP server error: %w", err)
		case <-ticker60s.C:
			a.refreshDynamic()
		case <-ticker5m.C:
			a.refreshDisks()
		}
	}
}

func (a *Agent) refreshDynamic() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.collector.UpdateDynamic(a.info)
}

func (a *Agent) refreshDisks() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.collector.UpdateDisks(a.info)
}

func (a *Agent) handleV1Info(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	info := a.GetInfo()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	data, err := MarshalMachineInfo(info)
	if err != nil {
		http.Error(w, "Failed to marshal info", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (a *Agent) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		http.Redirect(w, r, "/v1/info", http.StatusTemporaryRedirect)
		return
	}
	http.NotFound(w, r)
}

func (a *Agent) shutdown() error {
	var firstErr error

	if a.mdnsServer != nil {
		log.Println("[fleet agent] Unregistering mDNS service...")
		a.mdnsServer.Shutdown()
	}

	if a.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		log.Println("[fleet agent] Stopping HTTP server...")
		if err := a.httpServer.Shutdown(ctx); err != nil {
			firstErr = err
		}
	}

	return firstErr
}

// RunAgentWithSignals starts the agent and listens for SIGINT/SIGTERM.
func RunAgentWithSignals(port int) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	agent := NewAgent(port)

	go func() {
		sig := <-sigCh
		log.Printf("[fleet agent] Caught signal %v", sig)
		cancel()
	}()

	return agent.Run(ctx)
}
