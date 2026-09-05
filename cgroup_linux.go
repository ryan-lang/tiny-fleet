//go:build linux

package main

import (
	"bufio"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ContainerLimits holds resource limits discovered from cgroups.
type ContainerLimits struct {
	MemoryBytes uint64
	CPUs        float64
}

// ReadContainerLimits inspects cgroup v1 and v2 hierarchies to find active container constraints.
func ReadContainerLimits(hostMemTotal uint64, hostLogicalCPUs int) ContainerLimits {
	limits := ContainerLimits{}

	// 1. Determine cgroup paths from /proc/self/cgroup
	cgroupV2Path, cgroupV1Paths := getSelfCgroupPaths()

	// Check cgroup v2
	v2Mem, v2CPU := checkCgroupV2(cgroupV2Path, hostMemTotal, hostLogicalCPUs)
	if v2Mem > 0 && (limits.MemoryBytes == 0 || v2Mem < limits.MemoryBytes) {
		limits.MemoryBytes = v2Mem
	}
	if v2CPU > 0 && (limits.CPUs == 0 || v2CPU < limits.CPUs) {
		limits.CPUs = v2CPU
	}

	// Check cgroup v1
	v1Mem, v1CPU := checkCgroupV1(cgroupV1Paths, hostMemTotal, hostLogicalCPUs)
	if v1Mem > 0 && (limits.MemoryBytes == 0 || v1Mem < limits.MemoryBytes) {
		limits.MemoryBytes = v1Mem
	}
	if v1CPU > 0 && (limits.CPUs == 0 || v1CPU < limits.CPUs) {
		limits.CPUs = v1CPU
	}

	return limits
}

func getSelfCgroupPaths() (string, map[string]string) {
	v1Paths := make(map[string]string)
	v2Path := ""

	file, err := os.Open("/proc/self/cgroup")
	if err != nil {
		return "", v1Paths
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 {
			continue
		}
		// parts[0]: hierarchy ID, parts[1]: controllers, parts[2]: path
		if parts[0] == "0" && parts[1] == "" {
			v2Path = parts[2]
		} else {
			controllers := strings.Split(parts[1], ",")
			for _, c := range controllers {
				v1Paths[c] = parts[2]
			}
		}
	}
	return v2Path, v1Paths
}

func checkCgroupV2(relPath string, hostMemTotal uint64, hostLogicalCPUs int) (uint64, float64) {
	base := "/sys/fs/cgroup"
	pathsToTry := []string{base}
	if relPath != "" && relPath != "/" {
		curr := filepath.Join(base, relPath)
		for {
			pathsToTry = append([]string{curr}, pathsToTry...)
			if curr == base || curr == "/" || curr == "." {
				break
			}
			curr = filepath.Dir(curr)
		}
	}

	var minMem uint64
	var minCPU float64

	for _, dir := range pathsToTry {
		// Memory
		memMaxFile := filepath.Join(dir, "memory.max")
		if data, err := os.ReadFile(memMaxFile); err == nil {
			str := strings.TrimSpace(string(data))
			if str != "" && str != "max" {
				if val, err := strconv.ParseUint(str, 10, 64); err == nil && val > 0 && val < (1<<62) {
					if hostMemTotal == 0 || val < hostMemTotal {
						if minMem == 0 || val < minMem {
							minMem = val
						}
					}
				}
			}
		}

		// CPU Quota (cpu.max)
		cpuMaxFile := filepath.Join(dir, "cpu.max")
		if data, err := os.ReadFile(cpuMaxFile); err == nil {
			fields := strings.Fields(string(data))
			if len(fields) >= 2 && fields[0] != "max" {
				quota, err1 := strconv.ParseFloat(fields[0], 64)
				period, err2 := strconv.ParseFloat(fields[1], 64)
				if err1 == nil && err2 == nil && quota > 0 && period > 0 {
					cpus := quota / period
					if hostLogicalCPUs == 0 || cpus < float64(hostLogicalCPUs) {
						if minCPU == 0 || cpus < minCPU {
							minCPU = cpus
						}
					}
				}
			}
		}

		// CPU Set (cpuset.cpus.effective or cpuset.cpus)
		for _, name := range []string{"cpuset.cpus.effective", "cpuset.cpus"} {
			cpusetFile := filepath.Join(dir, name)
			if data, err := os.ReadFile(cpusetFile); err == nil {
				str := strings.TrimSpace(string(data))
				if str != "" {
					cnt := parseCPUSet(str)
					if cnt > 0 && (hostLogicalCPUs == 0 || cnt < hostLogicalCPUs) {
						fCnt := float64(cnt)
						if minCPU == 0 || fCnt < minCPU {
							minCPU = fCnt
						}
					}
				}
			}
		}
	}

	return minMem, minCPU
}

func checkCgroupV1(v1Paths map[string]string, hostMemTotal uint64, hostLogicalCPUs int) (uint64, float64) {
	var minMem uint64
	var minCPU float64

	// Memory
	memDir := "/sys/fs/cgroup/memory"
	if p, ok := v1Paths["memory"]; ok && p != "" && p != "/" {
		memDir = filepath.Join(memDir, p)
	}
	memLimitFile := filepath.Join(memDir, "memory.limit_in_bytes")
	if data, err := os.ReadFile(memLimitFile); err == nil {
		str := strings.TrimSpace(string(data))
		if val, err := strconv.ParseUint(str, 10, 64); err == nil && val > 0 && val < (1<<60) {
			if hostMemTotal == 0 || val < hostMemTotal {
				minMem = val
			}
		}
	}

	// CPU Quota
	cpuDir := "/sys/fs/cgroup/cpu,cpuacct"
	if _, err := os.Stat(cpuDir); err != nil {
		cpuDir = "/sys/fs/cgroup/cpu"
	}
	if p, ok := v1Paths["cpu"]; ok && p != "" && p != "/" {
		cpuDir = filepath.Join(cpuDir, p)
	}

	quotaFile := filepath.Join(cpuDir, "cpu.cfs_quota_us")
	periodFile := filepath.Join(cpuDir, "cpu.cfs_period_us")
	quotaBytes, qErr := os.ReadFile(quotaFile)
	periodBytes, pErr := os.ReadFile(periodFile)
	if qErr == nil && pErr == nil {
		quotaStr := strings.TrimSpace(string(quotaBytes))
		periodStr := strings.TrimSpace(string(periodBytes))
		quota, err1 := strconv.ParseFloat(quotaStr, 64)
		period, err2 := strconv.ParseFloat(periodStr, 64)
		if err1 == nil && err2 == nil && quota > 0 && period > 0 {
			cpus := quota / period
			if hostLogicalCPUs == 0 || cpus < float64(hostLogicalCPUs) {
				minCPU = cpus
			}
		}
	}

	// CPU Set
	cpusetDir := "/sys/fs/cgroup/cpuset"
	if p, ok := v1Paths["cpuset"]; ok && p != "" && p != "/" {
		cpusetDir = filepath.Join(cpusetDir, p)
	}
	cpusetFile := filepath.Join(cpusetDir, "cpuset.cpus")
	if data, err := os.ReadFile(cpusetFile); err == nil {
		str := strings.TrimSpace(string(data))
		if str != "" {
			cnt := parseCPUSet(str)
			if cnt > 0 && (hostLogicalCPUs == 0 || cnt < hostLogicalCPUs) {
				fCnt := float64(cnt)
				if minCPU == 0 || fCnt < minCPU {
					minCPU = fCnt
				}
			}
		}
	}

	return minMem, minCPU
}

// parseCPUSet parses cpuset ranges like "0-3,5,7-9" and returns the number of CPUs.
func parseCPUSet(val string) int {
	total := 0
	parts := strings.Split(val, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if dash := strings.Index(part, "-"); dash != -1 {
			start, err1 := strconv.Atoi(part[:dash])
			end, err2 := strconv.Atoi(part[dash+1:])
			if err1 == nil && err2 == nil && end >= start {
				total += (end - start + 1)
			}
		} else {
			if _, err := strconv.Atoi(part); err == nil {
				total++
			}
		}
	}
	return total
}

// EffectiveLogicalCPUs rounds container CPU limit to an integer representation.
func EffectiveLogicalCPUs(cpus float64, fallback int) int {
	if cpus <= 0 {
		return fallback
	}
	// Round half-up
	rounded := int(math.Round(cpus))
	if rounded < 1 {
		return 1
	}
	return rounded
}
