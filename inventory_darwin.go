//go:build darwin

package main

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Collector gathers machine information on macOS (Darwin).
type Collector struct {
	// Cached static hardware info gathered at startup
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
	c.staticArch = detectDarwinArch()
	c.staticKernel = detectDarwinKernel()
	c.staticVirt = detectDarwinVirt()
	c.staticOS, c.staticOSID = detectDarwinOS()
	c.staticCPU, c.staticHostCPUs = detectDarwinCPU()
	c.staticGPUs = detectDarwinGPUs()
}

// Collect returns the complete MachineInfo snapshot.
func (c *Collector) Collect() *MachineInfo {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	memBytes := detectDarwinMemTotal()
	hostCPUs := c.staticHostCPUs
	if hostCPUs <= 0 {
		hostCPUs = runtime.NumCPU()
	}

	disks := detectDarwinDisks()
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
		LogicalCPUs:    hostCPUs,
		CPULimit:       nil, // macOS bare metal / launchd has no Linux cgroups
		MemoryBytes:    memBytes,
		GPUs:           gpus,
		Disks:          disks,
		IP:             ip,
	}
}

// UpdateDynamic updates dynamic fields (memory, IP) on an existing info snapshot.
func (c *Collector) UpdateDynamic(info *MachineInfo) {
	mem := detectDarwinMemTotal()
	if mem > 0 {
		info.MemoryBytes = mem
	}
	info.IP = detectPrimaryIP()
}

// UpdateDisks refreshes the disk inventory.
func (c *Collector) UpdateDisks(info *MachineInfo) {
	info.Disks = detectDarwinDisks()
}

// GetOSID returns the short OS identifier ("macos") for TXT records.
func (c *Collector) GetOSID() string {
	return c.staticOSID
}

// GetVirt returns the virtualization string for TXT records.
func (c *Collector) GetVirt() string {
	return c.staticVirt
}

func detectDarwinArch() string {
	out, err := runCmd(2*time.Second, "uname", "-m")
	if err == nil && out != "" {
		return out
	}
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "arm64"
	default:
		return runtime.GOARCH
	}
}

func detectDarwinKernel() string {
	out, err := runCmd(2*time.Second, "uname", "-r")
	if err == nil && out != "" {
		return out
	}
	return "unknown"
}

func detectDarwinOS() (string, string) {
	out, err := runCmd(2*time.Second, "sw_vers")
	if err == nil && out != "" {
		return parseSwVers(out)
	}
	return "macOS", "macos"
}

func detectDarwinCPU() (string, int) {
	brand, _ := runCmd(2*time.Second, "sysctl", "-n", "machdep.cpu.brand_string")
	hwModel, _ := runCmd(2*time.Second, "sysctl", "-n", "hw.model")
	cpuName := parseSysctlCPU(brand, hwModel, runtime.GOARCH)

	logicalCPUs := runtime.NumCPU()
	if cpusStr, err := runCmd(2*time.Second, "sysctl", "-n", "hw.logicalcpu"); err == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(cpusStr)); err == nil && n > 0 {
			logicalCPUs = n
		}
	}

	return cpuName, logicalCPUs
}

func detectDarwinMemTotal() uint64 {
	out, err := runCmd(2*time.Second, "sysctl", "-n", "hw.memsize")
	if err == nil {
		if mem := parseSysctlMem(out); mem > 0 {
			return mem
		}
	}
	return 0
}

func detectDarwinVirt() string {
	hvVmm, _ := runCmd(2*time.Second, "sysctl", "-n", "kern.hv_vmm_present")
	hwModel, _ := runCmd(2*time.Second, "sysctl", "-n", "hw.model")
	cpuFeat, _ := runCmd(2*time.Second, "sysctl", "-n", "machdep.cpu.features")
	return parseDarwinVirt(hvVmm, hwModel, cpuFeat)
}

func detectDarwinGPUs() []string {
	// First attempt JSON format with system_profiler
	outJSON, err := runCmd(4*time.Second, "system_profiler", "-json", "SPDisplaysDataType")
	if err == nil && len(outJSON) > 0 {
		gpus := parseSystemProfilerGPUs([]byte(outJSON), "")
		if len(gpus) > 0 {
			return gpus
		}
	}

	// Fallback to text format
	outText, err := runCmd(4*time.Second, "system_profiler", "SPDisplaysDataType")
	if err == nil && len(outText) > 0 {
		gpus := parseSystemProfilerGPUs(nil, outText)
		if len(gpus) > 0 {
			return gpus
		}
	}

	return []string{}
}

func detectDarwinDisks() []DiskInfo {
	listOut, err := runCmd(3*time.Second, "diskutil", "list")
	if err != nil && listOut == "" {
		return []DiskInfo{}
	}

	diskIDs := parsePhysicalDisks(listOut)
	var disks []DiskInfo

	for _, diskID := range diskIDs {
		infoOut, err := runCmd(2*time.Second, "diskutil", "info", diskID)
		if err != nil && infoOut == "" {
			continue
		}
		name, model, size := parseDiskutilInfo(infoOut)
		if name == "" {
			name = diskID
		}
		if size > 0 {
			disks = append(disks, DiskInfo{
				Name:      name,
				Model:     model,
				SizeBytes: size,
			})
		}
	}

	if disks == nil {
		disks = []DiskInfo{}
	}
	return disks
}
