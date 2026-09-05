package main

import (
	"bytes"
	"context"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
)

func runCmd(timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return strings.TrimSpace(stdout.String()), err
}

func detectPrimaryIP() string {
	// Attempt outbound route resolution
	conn, err := net.DialTimeout("udp", "8.8.8.8:80", 1*time.Second)
	if err == nil {
		defer conn.Close()
		if udpAddr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
			ip := udpAddr.IP.String()
			if ip != "" && !udpAddr.IP.IsLoopback() {
				return ip
			}
		}
	}

	// Fallback to scanning interfaces
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	var firstIPv6 string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if ip4 := ip.To4(); ip4 != nil {
				return ip4.String()
			}
			if firstIPv6 == "" && ip.IsGlobalUnicast() {
				firstIPv6 = ip.String()
			}
		}
	}

	return firstIPv6
}
