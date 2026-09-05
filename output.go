package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
)

// PrintCompactTable writes the fleet overview table to the specified writer.
func PrintCompactTable(w io.Writer, records []*HostRecord) {
	if len(records) == 0 {
		fmt.Fprintln(w, "No fleet hosts discovered.")
		return
	}

	tw := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	fmt.Fprintln(tw, "HOST\tIP\tOS\tVIRT\tCPU\tRAM\tGPU")

	for _, r := range records {
		host := r.Hostname
		ip := r.Address
		if ip == "" {
			ip = "-"
		}

		osName := "-"
		virt := "none"
		cpu := "-"
		ram := "-"
		gpu := "-"

		if r.Info != nil {
			if r.Info.OS != "" {
				osName = r.Info.OS
			}
			if r.Info.Virtualization != "" {
				virt = r.Info.Virtualization
			}

			// CPU formatting:
			if virt != "none" && virt != "" {
				// In virtualized / container environments, show vCPU count
				if r.Info.CPULimit != nil {
					if *r.Info.CPULimit == float64(int(*r.Info.CPULimit)) {
						cpu = fmt.Sprintf("%.0f vCPU", *r.Info.CPULimit)
					} else {
						cpu = fmt.Sprintf("%.1f vCPU", *r.Info.CPULimit)
					}
				} else if r.Info.LogicalCPUs > 0 {
					cpu = fmt.Sprintf("%d vCPU", r.Info.LogicalCPUs)
				} else if r.Info.CPU != "" {
					cpu = CleanCPUCompact(r.Info.CPU)
				}
			} else {
				// Bare metal
				if r.Info.CPU != "" {
					cpu = CleanCPUCompact(r.Info.CPU)
				}
			}

			if r.Info.MemoryBytes > 0 {
				ram = FormatRAMCompact(r.Info.MemoryBytes)
			}

			if len(r.Info.GPUs) > 0 {
				gpu = CleanGPUName(r.Info.GPUs[0])
			}
		}

		if r.Error != "" && r.Info == nil {
			cpu = "[unreachable]"
		}

		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			host, ip, osName, virt, cpu, ram, gpu)
	}

	tw.Flush()
}

// PrintDetailedInfo writes detailed info for a single host to stdout.
func PrintDetailedInfo(w io.Writer, r *HostRecord) {
	info := r.Info
	if info == nil {
		fmt.Fprintf(w, "%s\n  Error: %s\n", r.Hostname, r.Error)
		return
	}

	address := r.Address
	if address == "" && info.IP != "" {
		address = info.IP
	}
	if address == "" {
		address = "-"
	}

	virtDisplay := "no"
	if info.Virtualization != "" && info.Virtualization != "none" {
		virtDisplay = info.Virtualization
	}

	fmt.Fprintf(w, "%s\n", r.Hostname)
	fmt.Fprintf(w, "  Address:     %s\n", address)
	fmt.Fprintf(w, "  OS:          %s\n", info.OS)
	fmt.Fprintf(w, "  Kernel:      %s\n", info.Kernel)
	fmt.Fprintf(w, "  Virtualized: %s\n", virtDisplay)
	fmt.Fprintf(w, "  CPU:         %s\n", info.CPU)

	if info.CPULimit != nil {
		fmt.Fprintf(w, "  CPUs:        %d (limit: %.1f)\n", info.LogicalCPUs, *info.CPULimit)
	} else {
		fmt.Fprintf(w, "  CPUs:        %d\n", info.LogicalCPUs)
	}

	fmt.Fprintf(w, "  RAM:         %s\n", FormatRAMDetailed(info.MemoryBytes))
	fmt.Fprintln(w)

	// GPUs section
	fmt.Fprintln(w, "  GPUs:")
	if len(info.GPUs) == 0 {
		fmt.Fprintln(w, "    -")
	} else {
		for _, g := range info.GPUs {
			fmt.Fprintf(w, "    %s\n", g)
		}
	}
	fmt.Fprintln(w)

	// Storage section
	fmt.Fprintln(w, "  Storage:")
	if len(info.Disks) == 0 {
		fmt.Fprintln(w, "    -")
	} else {
		tw := tabwriter.NewWriter(w, 0, 8, 4, ' ', 0)
		for _, d := range info.Disks {
			model := d.Model
			if model == "" {
				model = "-"
			}
			sizeStr := FormatDiskDetailed(d.SizeBytes)
			fmt.Fprintf(tw, "    %s\t%s\t%s\n", d.Name, model, sizeStr)
		}
		tw.Flush()
	}
}

// CleanCPUCompact cleans up CPU model for the compact table.
func CleanCPUCompact(m string) string {
	clean := m
	// Strip Intel(R) Core(TM), AMD, etc.
	clean = strings.ReplaceAll(clean, "(R)", "")
	clean = strings.ReplaceAll(clean, "(TM)", "")
	clean = strings.ReplaceAll(clean, "CPU", "")
	for _, p := range []string{"AMD ", "Intel "} {
		if strings.HasPrefix(clean, p) {
			clean = strings.TrimPrefix(clean, p)
		}
	}
	// If it has @ 3.xxGHz, remove it to save column space
	if idx := strings.Index(clean, "@"); idx != -1 {
		clean = clean[:idx]
	}
	return strings.TrimSpace(strings.Join(strings.Fields(clean), " "))
}

// WriteJSON writes any data as indented JSON to stdout.
func WriteJSON(data interface{}) error {
	bytes, err := MarshalMachineInfoSafe(data)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(append(bytes, '\n'))
	return err
}

func MarshalMachineInfoSafe(v interface{}) ([]byte, error) {
	switch val := v.(type) {
	case *MachineInfo:
		return MarshalMachineInfo(val)
	case MachineInfo:
		return MarshalMachineInfo(&val)
	default:
		return nil, fmt.Errorf("unsupported type")
	}
}
