package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

func parseSwVers(out string) (osName, osID string) {
	osID = "macos"
	version := ""
	productName := "macOS"

	lines := strings.Split(out, "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "ProductName":
			if val != "" {
				productName = val
			}
		case "ProductVersion":
			version = val
		}
	}

	if version != "" {
		osName = fmt.Sprintf("%s %s", productName, version)
	} else {
		osName = productName
	}
	return osName, osID
}

func parseSysctlCPU(brand string, hwModel string, arch string) string {
	brand = strings.TrimSpace(brand)
	if brand != "" {
		brand = regexp.MustCompile(`\s+`).ReplaceAllString(brand, " ")
		return brand
	}
	hwModel = strings.TrimSpace(hwModel)
	if hwModel != "" {
		return hwModel
	}
	if arch == "arm64" {
		return "Apple Silicon"
	}
	return "Intel"
}

func parseSysctlMem(out string) uint64 {
	out = strings.TrimSpace(out)
	if bytesVal, err := strconv.ParseUint(out, 10, 64); err == nil {
		return bytesVal
	}
	return 0
}

func parseDarwinVirt(hvVmm string, hwModel string, cpuFeatures string) string {
	hvVmm = strings.TrimSpace(hvVmm)
	if hvVmm == "1" {
		modelLower := strings.ToLower(hwModel)
		switch {
		case strings.Contains(modelLower, "virtualmac"):
			return "apple-vm"
		case strings.Contains(modelLower, "parallels"):
			return "parallels"
		case strings.Contains(modelLower, "vmware"):
			return "vmware"
		case strings.Contains(modelLower, "qemu"):
			return "qemu"
		case strings.Contains(modelLower, "virtualbox"):
			return "virtualbox"
		default:
			return "vm"
		}
	}

	modelLower := strings.ToLower(hwModel)
	switch {
	case strings.Contains(modelLower, "virtualmac"):
		return "apple-vm"
	case strings.Contains(modelLower, "parallels"):
		return "parallels"
	case strings.Contains(modelLower, "vmware"):
		return "vmware"
	case strings.Contains(modelLower, "qemu"):
		return "qemu"
	case strings.Contains(modelLower, "virtualbox"):
		return "virtualbox"
	}

	featUpper := strings.ToUpper(cpuFeatures)
	if strings.Contains(featUpper, "VMM") {
		return "vm"
	}

	return "none"
}

type spDisplaysData struct {
	Displays []struct {
		Name         string `json:"_name"`
		Model        string `json:"sppci_model"`
		ModelAlt     string `json:"sppci_model_name"`
		ChipsetModel string `json:"spdisplays_chipset_model"`
	} `json:"SPDisplaysDataType"`
}

func parseSystemProfilerGPUs(jsonBytes []byte, textFallback string) []string {
	var gpus []string
	seen := make(map[string]bool)

	if len(jsonBytes) > 0 {
		var data spDisplaysData
		if err := json.Unmarshal(jsonBytes, &data); err == nil && len(data.Displays) > 0 {
			for _, d := range data.Displays {
				name := strings.TrimSpace(d.Name)
				if name == "" {
					name = strings.TrimSpace(d.Model)
				}
				if name == "" {
					name = strings.TrimSpace(d.ModelAlt)
				}
				if name == "" {
					name = strings.TrimSpace(d.ChipsetModel)
				}
				if name != "" && !seen[name] {
					seen[name] = true
					gpus = append(gpus, name)
				}
			}
			if len(gpus) > 0 {
				return gpus
			}
		}
	}

	// Text fallback: look for "Chipset Model: <name>"
	re := regexp.MustCompile(`(?i)Chipset Model:\s*(.+)`)
	matches := re.FindAllStringSubmatch(textFallback, -1)
	for _, m := range matches {
		if len(m) > 1 {
			name := strings.TrimSpace(m[1])
			if name != "" && !seen[name] {
				seen[name] = true
				gpus = append(gpus, name)
			}
		}
	}

	return gpus
}

// parsePhysicalDisks finds identifiers for physical whole disks from "diskutil list" output.
// It ignores APFS synthesized containers, volumes, partitions, and disk images.
func parsePhysicalDisks(listOut string) []string {
	var disks []string
	seen := make(map[string]bool)

	// Pattern matches headers like:
	// /dev/disk0 (internal, physical):
	// /dev/disk2 (external, physical):
	re := regexp.MustCompile(`(?m)^/dev/(disk\d+)\s*\([^)]*physical[^)]*\):`)
	matches := re.FindAllStringSubmatch(listOut, -1)
	for _, m := range matches {
		if len(m) > 1 {
			diskID := m[1]
			if !seen[diskID] {
				seen[diskID] = true
				disks = append(disks, diskID)
			}
		}
	}

	// Fallback: if no physical tag was found, look for /dev/disk0 directly
	if len(disks) == 0 {
		reFallback := regexp.MustCompile(`(?m)^/dev/(disk0)\b`)
		if m := reFallback.FindStringSubmatch(listOut); len(m) > 1 {
			disks = append(disks, m[1])
		}
	}

	return disks
}

func parseDiskutilInfo(infoOut string) (name string, model string, size uint64) {
	lines := strings.Split(infoOut, "\n")
	reBytes := regexp.MustCompile(`\((\d+)\s+Bytes\)`)

	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "Device Identifier":
			name = val
		case "Device / Media Name", "Media Name":
			if model == "" {
				model = val
			}
		case "Device Model":
			if val != "" {
				model = val
			}
		case "Disk Size":
			if match := reBytes.FindStringSubmatch(val); len(match) > 1 {
				if parsed, err := strconv.ParseUint(match[1], 10, 64); err == nil {
					size = parsed
				}
			}
		}
	}

	return name, model, size
}
