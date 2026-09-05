//go:build linux

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Collector gathers machine information.
type Collector struct {
	// Cached static hardware info gathered at boot/startup
	staticArch     string
	staticKernel   string
	staticCPU      string
	staticHostCPUs int
	staticVirt     string
	staticOS       string
	staticOSID     string
	staticGPUs     []string
}

// NewCollector initializes and discovers static machine facts.
func NewCollector() *Collector {
	c := &Collector{}
	c.initStatic()
	return c
}

func (c *Collector) initStatic() {
	c.staticArch = detectArch()
	c.staticKernel = detectKernel()
	c.staticVirt = detectVirtualization()
	c.staticOS, c.staticOSID = detectOS()
	c.staticCPU, c.staticHostCPUs = detectCPU()
	c.staticGPUs = detectGPUs()
}

// Collect returns the complete MachineInfo snapshot.
// It recalculates dynamic fields (memory, disks, IP, container limits).
func (c *Collector) Collect() *MachineInfo {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	hostMem := detectMemTotal()
	hostCPUs := c.staticHostCPUs
	if hostCPUs <= 0 {
		hostCPUs = runtime.NumCPU()
	}

	limits := ReadContainerLimits(hostMem, hostCPUs)

	memBytes := hostMem
	if limits.MemoryBytes > 0 && limits.MemoryBytes < hostMem {
		memBytes = limits.MemoryBytes
	}

	logicalCPUs := hostCPUs
	var cpuLimit *float64
	if limits.CPUs > 0 && limits.CPUs < float64(hostCPUs) {
		cl := limits.CPUs
		cpuLimit = &cl
		logicalCPUs = EffectiveLogicalCPUs(limits.CPUs, hostCPUs)
	}

	disks := detectDisks()
	ip := detectPrimaryIP()

	gpus := c.staticGPUs
	if gpus == nil {
		gpus = []string{}
	}

	return &MachineInfo{
		Hostname:       hostname,
		OS:             c.staticOS,
		Kernel:         c.staticKernel,
		Arch:           c.staticArch,
		Virtualization: c.staticVirt,
		CPU:            c.staticCPU,
		LogicalCPUs:    logicalCPUs,
		CPULimit:       cpuLimit,
		MemoryBytes:    memBytes,
		GPUs:           gpus,
		Disks:          disks,
		IP:             ip,
	}
}

// CollectDynamicOnly updates only dynamic fields (memory, container limits, IP) on an existing info snapshot.
func (c *Collector) UpdateDynamic(info *MachineInfo) {
	hostMem := detectMemTotal()
	hostCPUs := c.staticHostCPUs
	if hostCPUs <= 0 {
		hostCPUs = runtime.NumCPU()
	}

	limits := ReadContainerLimits(hostMem, hostCPUs)
	if limits.MemoryBytes > 0 && limits.MemoryBytes < hostMem {
		info.MemoryBytes = limits.MemoryBytes
	} else {
		info.MemoryBytes = hostMem
	}

	if limits.CPUs > 0 && limits.CPUs < float64(hostCPUs) {
		cl := limits.CPUs
		info.CPULimit = &cl
		info.LogicalCPUs = EffectiveLogicalCPUs(limits.CPUs, hostCPUs)
	} else {
		info.CPULimit = nil
		info.LogicalCPUs = hostCPUs
	}

	info.IP = detectPrimaryIP()
}

// UpdateDisks refreshes the disk inventory.
func (c *Collector) UpdateDisks(info *MachineInfo) {
	info.Disks = detectDisks()
}

// GetOSID returns the short OS identifier (e.g. "ubuntu", "debian", "proxmox") for TXT records.
func (c *Collector) GetOSID() string {
	return c.staticOSID
}

// GetVirt returns the virtualization string for TXT records.
func (c *Collector) GetVirt() string {
	return c.staticVirt
}

func detectArch() string {
	out, err := runCmd(2*time.Second, "uname", "-m")
	if err == nil && out != "" {
		return out
	}
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	default:
		return runtime.GOARCH
	}
}

func detectKernel() string {
	out, err := runCmd(2*time.Second, "uname", "-r")
	if err == nil && out != "" {
		return out
	}
	return "unknown"
}

func detectVirtualization() string {
	// systemd-detect-virt returns 0 on virtualized, 1 on "none"
	out, _ := runCmd(2*time.Second, "systemd-detect-virt")
	clean := strings.ToLower(strings.TrimSpace(out))
	if clean != "" {
		if clean == "none" {
			return "none"
		}
		return clean
	}

	// Fallbacks if systemd-detect-virt is missing
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return "docker"
	}
	if data, err := os.ReadFile("/run/systemd/container"); err == nil {
		v := strings.TrimSpace(string(data))
		if v != "" {
			return v
		}
	}
	if data, err := os.ReadFile("/sys/class/dmi/id/product_name"); err == nil {
		p := strings.ToLower(string(data))
		if strings.Contains(p, "kvm") || strings.Contains(p, "qemu") {
			return "kvm"
		}
		if strings.Contains(p, "vmware") {
			return "vmware"
		}
		if strings.Contains(p, "virtualbox") {
			return "oracle"
		}
	}

	return "none"
}

func detectOS() (prettyName string, osID string) {
	// 1. Check for Proxmox VE first
	if pveOut, err := runCmd(2*time.Second, "pveversion"); err == nil && strings.HasPrefix(pveOut, "pve-manager/") {
		parts := strings.Split(pveOut, "/")
		if len(parts) >= 2 {
			verParts := strings.Split(parts[1], ".")
			if len(verParts) >= 1 {
				major := verParts[0]
				return fmt.Sprintf("Proxmox VE %s", major), "proxmox"
			}
		}
		return "Proxmox VE", "proxmox"
	}

	// 2. Parse /etc/os-release or /usr/lib/os-release
	paths := []string{"/etc/os-release", "/usr/lib/os-release"}
	for _, p := range paths {
		if f, err := os.Open(p); err == nil {
			defer f.Close()
			scanner := bufio.NewScanner(f)
			rawPretty := ""
			rawID := ""
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if strings.HasPrefix(line, "PRETTY_NAME=") {
					rawPretty = unquote(strings.TrimPrefix(line, "PRETTY_NAME="))
				} else if strings.HasPrefix(line, "ID=") {
					rawID = unquote(strings.TrimPrefix(line, "ID="))
				}
			}
			if rawPretty != "" {
				if rawID == "" {
					rawID = strings.ToLower(strings.Fields(rawPretty)[0])
				}
				return rawPretty, rawID
			}
		}
	}

	return "Linux", "linux"
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		return s[1 : len(s)-1]
	}
	return s
}

func detectCPU() (model string, count int) {
	count = runtime.NumCPU()

	// Try lscpu --json first
	if out, err := runCmd(2*time.Second, "lscpu", "--json"); err == nil {
		var lscpuData struct {
			Lscpu []struct {
				Field string `json:"field"`
				Data  string `json:"data"`
			} `json:"lscpu"`
		}
		if err := json.Unmarshal([]byte(out), &lscpuData); err == nil {
			for _, item := range lscpuData.Lscpu {
				if strings.EqualFold(item.Field, "Model name:") {
					model = cleanCPUModel(item.Data)
				} else if strings.EqualFold(item.Field, "CPU(s):") {
					if c, err := strconv.Atoi(item.Data); err == nil && c > 0 {
						count = c
					}
				}
			}
		}
	}

	// Fallback to /proc/cpuinfo
	if model == "" {
		if file, err := os.Open("/proc/cpuinfo"); err == nil {
			defer file.Close()
			scanner := bufio.NewScanner(file)
			cpuCount := 0
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "model name") {
					if colon := strings.Index(line, ":"); colon != -1 && model == "" {
						model = cleanCPUModel(line[colon+1:])
					}
				} else if strings.HasPrefix(line, "processor") {
					cpuCount++
				}
			}
			if count <= 0 && cpuCount > 0 {
				count = cpuCount
			}
		}
	}

	if model == "" {
		model = fmt.Sprintf("CPU (%s)", detectArch())
	}
	if count <= 0 {
		count = 1
	}

	return model, count
}

func cleanCPUModel(m string) string {
	m = strings.TrimSpace(m)
	// Replace multiple spaces with single space
	fields := strings.Fields(m)
	return strings.Join(fields, " ")
}

func detectMemTotal() uint64 {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, err := strconv.ParseUint(fields[1], 10, 64)
				if err == nil {
					return kb * 1024
				}
			}
			break
		}
	}
	return 0
}

func detectGPUs() []string {
	var gpus []string
	seen := make(map[string]bool)

	// 1. Check nvidia-smi if available for precise marketing names
	if out, err := runCmd(2*time.Second, "nvidia-smi", "--query-gpu=name", "--format=csv,noheader"); err == nil {
		scanner := bufio.NewScanner(strings.NewReader(out))
		for scanner.Scan() {
			name := strings.TrimSpace(scanner.Text())
			if name != "" && !seen[name] {
				gpus = append(gpus, name)
				seen[name] = true
			}
		}
	}

	// 2. Check lspci -mm
	if out, err := runCmd(2*time.Second, "lspci", "-mm"); err == nil {
		scanner := bufio.NewScanner(strings.NewReader(out))
		reBracket := regexp.MustCompile(`\[(.*?)\]`)
		for scanner.Scan() {
			line := scanner.Text()
			// lspci -mm format: Slot "Class" "Vendor" "Device" -rRev -pProgIF "SVendor" "SDevice"
			parts := parseLspciMM(line)
			if len(parts) >= 4 {
				class := strings.ToLower(parts[1])
				if strings.Contains(class, "vga") || strings.Contains(class, "3d") || strings.Contains(class, "display") {
					vendor := parts[2]
					device := parts[3]

					// If device has bracket [GeForce RTX 4090], extract that
					match := reBracket.FindStringSubmatch(device)
					cleanDev := device
					if len(match) >= 2 && match[1] != "" {
						cleanDev = match[1]
					}

					// Build descriptive name
					fullName := vendor + " " + cleanDev
					fullName = cleanGPUVendorName(fullName)

					// Check if already covered by nvidia-smi
					alreadyCovered := false
					for _, existing := range gpus {
						if strings.Contains(strings.ToLower(fullName), strings.ToLower(existing)) ||
							strings.Contains(strings.ToLower(existing), strings.ToLower(cleanDev)) {
							alreadyCovered = true
							break
						}
					}
					if !alreadyCovered && !seen[fullName] {
						gpus = append(gpus, fullName)
						seen[fullName] = true
					}
				}
			}
		}
	}

	if gpus == nil {
		gpus = []string{}
	}
	return gpus
}

func parseLspciMM(line string) []string {
	var parts []string
	inQuotes := false
	var cur strings.Builder

	for i := 0; i < len(line); i++ {
		c := line[i]
		if c == '"' {
			inQuotes = !inQuotes
		} else if c == ' ' && !inQuotes {
			if cur.Len() > 0 {
				parts = append(parts, cur.String())
				cur.Reset()
			}
		} else {
			cur.WriteByte(c)
		}
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	return parts
}

func cleanGPUVendorName(name string) string {
	name = strings.ReplaceAll(name, "Corporation", "")
	name = strings.ReplaceAll(name, "Advanced Micro Devices, Inc. [AMD/ATI]", "AMD")
	name = strings.ReplaceAll(name, "AMD/ATI", "AMD")
	name = strings.ReplaceAll(name, "Co., Ltd.", "")
	return strings.Join(strings.Fields(name), " ")
}

func detectDisks() []DiskInfo {
	var disks []DiskInfo

	// 1. Try lsblk --json --bytes -d -o NAME,MODEL,SIZE,TYPE
	if out, err := runCmd(3*time.Second, "lsblk", "--json", "--bytes", "-d", "-o", "NAME,MODEL,SIZE,TYPE"); err == nil {
		var res struct {
			Blockdevices []struct {
				Name  string  `json:"name"`
				Model *string `json:"model"`
				Size  uint64  `json:"size"`
				Type  string  `json:"type"`
			} `json:"blockdevices"`
		}
		if err := json.Unmarshal([]byte(out), &res); err == nil {
			for _, bd := range res.Blockdevices {
				if bd.Type == "disk" && bd.Size > 0 {
					model := ""
					if bd.Model != nil {
						model = strings.TrimSpace(*bd.Model)
					}
					if model == "" {
						model = readSysBlockModel(bd.Name)
					}
					disks = append(disks, DiskInfo{
						Name:      bd.Name,
						Model:     model,
						SizeBytes: bd.Size,
					})
				}
			}
			if len(disks) > 0 {
				return disks
			}
		}
	}

	// 2. Fallback to /sys/block inspection
	entries, err := os.ReadDir("/sys/block")
	if err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") || strings.HasPrefix(name, "zram") {
				continue
			}
			sizeFile := filepath.Join("/sys/block", name, "size")
			sizeData, err := os.ReadFile(sizeFile)
			if err != nil {
				continue
			}
			sectors, err := strconv.ParseUint(strings.TrimSpace(string(sizeData)), 10, 64)
			if err != nil || sectors == 0 {
				continue
			}
			bytes := sectors * 512
			model := readSysBlockModel(name)

			disks = append(disks, DiskInfo{
				Name:      name,
				Model:     model,
				SizeBytes: bytes,
			})
		}
	}

	if disks == nil {
		disks = []DiskInfo{}
	}
	return disks
}

func readSysBlockModel(devName string) string {
	paths := []string{
		filepath.Join("/sys/block", devName, "device", "model"),
		filepath.Join("/sys/block", devName, "device", "name"),
	}
	for _, p := range paths {
		if data, err := os.ReadFile(p); err == nil {
			m := strings.TrimSpace(string(data))
			if m != "" {
				return m
			}
		}
	}
	return ""
}

