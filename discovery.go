package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
)

// FetchInfoClient handles fetching MachineInfo over HTTP.
type FetchInfoClient struct {
	client *http.Client
}

// NewFetchInfoClient returns an HTTP client configured safely for local discovery.
func NewFetchInfoClient() *FetchInfoClient {
	transport := &http.Transport{
		Proxy: nil, // Do not inherit proxy settings for LAN discovery
		DialContext: (&net.Dialer{
			Timeout:   2 * time.Second,
			KeepAlive: 10 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   2 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &FetchInfoClient{
		client: &http.Client{
			Transport: transport,
			Timeout:   3 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse // Do not follow redirects
			},
		},
	}
}

// FetchInfo attempts to retrieve MachineInfo from the given IP addresses and port.
func (f *FetchInfoClient) FetchInfo(ctx context.Context, ips []net.IP, port int) (*MachineInfo, string, error) {
	if len(ips) == 0 {
		return nil, "", fmt.Errorf("no IP addresses available for host")
	}

	// Prioritize IPv4 addresses first, then IPv6
	orderedIPs := make([]net.IP, 0, len(ips))
	for _, ip := range ips {
		if ip.To4() != nil {
			orderedIPs = append(orderedIPs, ip)
		}
	}
	for _, ip := range ips {
		if ip.To4() == nil {
			orderedIPs = append(orderedIPs, ip)
		}
	}

	var lastErr error
	for _, ip := range orderedIPs {
		var hostPort string
		if ip.To4() != nil {
			hostPort = fmt.Sprintf("%s:%d", ip.String(), port)
		} else {
			hostPort = fmt.Sprintf("[%s]:%d", ip.String(), port)
		}

		url := fmt.Sprintf("http://%s/v1/info", hostPort)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			lastErr = err
			continue
		}

		resp, err := f.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
			continue
		}

		// Bound response size to 1MB
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		var info MachineInfo
		if err := json.Unmarshal(body, &info); err != nil {
			lastErr = fmt.Errorf("malformed JSON from %s: %w", url, err)
			continue
		}

		if info.IP == "" {
			info.IP = ip.String()
		}

		return &info, ip.String(), nil
	}

	return nil, "", lastErr
}

// DiscoverAll browses _fleet._tcp.local for the specified duration and returns all reachable hosts.
func DiscoverAll(ctx context.Context, waitTime time.Duration) ([]*HostRecord, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize mDNS resolver: %w", err)
	}

	entries := make(chan *zeroconf.ServiceEntry, 64)
	browseCtx, cancelBrowse := context.WithTimeout(ctx, waitTime)
	defer cancelBrowse()

	err = resolver.Browse(browseCtx, "_fleet._tcp", "local.", entries)
	if err != nil {
		return nil, fmt.Errorf("failed to browse mDNS services: %w", err)
	}

	// Map of instance name -> ServiceEntry
	foundMap := make(map[string]*zeroconf.ServiceEntry)
	for {
		select {
		case entry, ok := <-entries:
			if !ok {
				goto fetchPhase
			}
			if entry == nil || entry.Instance == "" {
				continue
			}
			inst := entry.Instance
			if existing, exists := foundMap[inst]; !exists {
				foundMap[inst] = entry
			} else {
				// Merge IP addresses if discovered via different interfaces
				existing.AddrIPv4 = appendUniqueIPs(existing.AddrIPv4, entry.AddrIPv4)
				existing.AddrIPv6 = appendUniqueIPs(existing.AddrIPv6, entry.AddrIPv6)
			}
		case <-browseCtx.Done():
			goto fetchPhase
		}
	}

fetchPhase:
	fetcher := NewFetchInfoClient()
	records := make([]*HostRecord, 0, len(foundMap))
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, entry := range foundMap {
		wg.Add(1)
		go func(e *zeroconf.ServiceEntry) {
			defer wg.Done()

			allIPs := make([]net.IP, 0, len(e.AddrIPv4)+len(e.AddrIPv6))
			allIPs = append(allIPs, e.AddrIPv4...)
			allIPs = append(allIPs, e.AddrIPv6...)

			allIPStrings := make([]string, 0, len(allIPs))
			for _, ip := range allIPs {
				allIPStrings = append(allIPStrings, ip.String())
			}

			rec := &HostRecord{
				Hostname: e.Instance,
				Port:     e.Port,
				AllIPs:   allIPStrings,
			}

			// Parse TXT records for fallback hints
			var fallbackOS, fallbackVirt string
			for _, txt := range e.Text {
				if strings.HasPrefix(txt, "os=") {
					fallbackOS = strings.TrimPrefix(txt, "os=")
				} else if strings.HasPrefix(txt, "virt=") {
					fallbackVirt = strings.TrimPrefix(txt, "virt=")
				}
			}

			fetchCtx, cancelFetch := context.WithTimeout(ctx, 3*time.Second)
			defer cancelFetch()

			info, usedIP, err := fetcher.FetchInfo(fetchCtx, allIPs, e.Port)
			if err != nil {
				rec.Error = err.Error()
				if len(allIPStrings) > 0 {
					rec.Address = allIPStrings[0]
				}
				// Provide fallback info so row can still be displayed
				rec.Info = &MachineInfo{
					Hostname:       e.Instance,
					OS:             fallbackOS,
					Virtualization: fallbackVirt,
					IP:             rec.Address,
				}
			} else {
				rec.Address = usedIP
				rec.Info = info
			}

			mu.Lock()
			records = append(records, rec)
			mu.Unlock()
		}(entry)
	}

	wg.Wait()

	sort.Slice(records, func(i, j int) bool {
		return strings.ToLower(records[i].Hostname) < strings.ToLower(records[j].Hostname)
	})

	return records, nil
}

// FindHost locates a specific host by instance name or hostname.
func FindHost(ctx context.Context, target string, waitTime time.Duration) (*HostRecord, error) {
	// Discover all within window and locate target
	records, err := DiscoverAll(ctx, waitTime)
	if err != nil {
		return nil, err
	}

	var matches []*HostRecord
	for _, r := range records {
		if strings.EqualFold(r.Hostname, target) ||
			(r.Info != nil && strings.EqualFold(r.Info.Hostname, target)) {
			matches = append(matches, r)
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("host '%s' not found on local network", target)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("ambiguous host '%s': found %d matching machines", target, len(matches))
	}

	return matches[0], nil
}

func appendUniqueIPs(base []net.IP, toAdd []net.IP) []net.IP {
	for _, ip := range toAdd {
		exists := false
		for _, b := range base {
			if b.Equal(ip) {
				exists = true
				break
			}
		}
		if !exists {
			base = append(base, ip)
		}
	}
	return base
}
