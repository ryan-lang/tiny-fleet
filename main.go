package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"
)

var Version = "0.1.1"

func main() {
	if len(os.Args) < 2 {
		// Default command: fleet (discover and show table)
		runFleetOverview(false, 3*time.Second)
		return
	}

	switch os.Args[1] {
	case "agent":
		agentCmd := flag.NewFlagSet("agent", flag.ExitOnError)
		port := agentCmd.Int("port", DefaultPort, "HTTP server listening port")
		agentCmd.Parse(os.Args[2:])
		if err := RunAgentWithSignals(*port); err != nil {
			fmt.Fprintf(os.Stderr, "Agent error: %v\n", err)
			os.Exit(1)
		}

	case "info":
		infoCmd := flag.NewFlagSet("info", flag.ExitOnError)
		asJSON := infoCmd.Bool("json", false, "Output machine-readable JSON")
		timeout := infoCmd.Duration("timeout", 3*time.Second, "Discovery wait timeout")

		// Reorder args so flags can appear before or after host argument
		args := os.Args[2:]
		var targetHost string
		var flagArgs []string
		for _, arg := range args {
			if arg == "--json" || arg == "-json" {
				flagArgs = append(flagArgs, arg)
			} else if len(arg) > 0 && arg[0] == '-' {
				flagArgs = append(flagArgs, arg)
			} else if targetHost == "" {
				targetHost = arg
			} else {
				flagArgs = append(flagArgs, arg)
			}
		}

		infoCmd.Parse(flagArgs)

		if targetHost == "" {
			fmt.Fprintf(os.Stderr, "Usage: fleet info <hostname> [--json]\n")
			os.Exit(1)
		}

		runFleetInfo(targetHost, *asJSON, *timeout)

	case "--json", "-json":
		runFleetOverview(true, 3*time.Second)

	case "version", "-version", "--version", "-v":
		fmt.Printf("fleet version %s\n", Version)

	case "help", "-h", "--help":
		printHelp()

	default:
		// If first arg starts with a dash, parse overview flags
		if os.Args[1][0] == '-' {
			overviewCmd := flag.NewFlagSet("fleet", flag.ExitOnError)
			asJSON := overviewCmd.Bool("json", false, "Output machine-readable JSON")
			timeout := overviewCmd.Duration("timeout", 3*time.Second, "Discovery wait timeout")
			overviewCmd.Parse(os.Args[1:])
			runFleetOverview(*asJSON, *timeout)
			return
		}

		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println(`fleet - zero-configuration LAN inventory tool

Usage:
  fleet                     Browse local network and show fleet table
  fleet --json              Browse local network and output JSON array
  fleet info <host>         Show detailed hardware and OS profile for host
  fleet info <host> --json  Output detailed machine profile as JSON
  fleet agent               Run the background discovery agent
  fleet version             Print version information

Options:
  --json                    Machine-readable JSON output
  --timeout <duration>      mDNS discovery wait duration (default 3s)
  --port <port>             Agent HTTP port (default 9753)`)
}

func runFleetOverview(asJSON bool, timeout time.Duration) {
	ctx := context.Background()
	records, err := DiscoverAll(ctx, timeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Discovery failed: %v\n", err)
		os.Exit(1)
	}

	if asJSON {
		type HostJSON struct {
			MachineInfo
			Address string `json:"address,omitempty"`
			Error   string `json:"error,omitempty"`
		}

		outList := make([]HostJSON, 0, len(records))
		for _, r := range records {
			hj := HostJSON{
				Address: r.Address,
				Error:   r.Error,
			}
			if r.Info != nil {
				hj.MachineInfo = *r.Info
				if hj.IP == "" {
					hj.IP = r.Address
				}
			} else {
				hj.Hostname = r.Hostname
				hj.IP = r.Address
			}
			if hj.GPUs == nil {
				hj.GPUs = []string{}
			}
			if hj.Disks == nil {
				hj.Disks = []DiskInfo{}
			}
			outList = append(outList, hj)
		}

		data, err := json.MarshalIndent(outList, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "JSON marshal error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(data))
		return
	}

	PrintCompactTable(os.Stdout, records)
}

func runFleetInfo(target string, asJSON bool, timeout time.Duration) {
	ctx := context.Background()
	record, err := FindHost(ctx, target, timeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if asJSON {
		if record.Info == nil {
			fmt.Fprintf(os.Stderr, "Error: host '%s' was found on mDNS but failed to report info: %s\n",
				target, record.Error)
			os.Exit(1)
		}
		data, err := MarshalMachineInfo(record.Info)
		if err != nil {
			fmt.Fprintf(os.Stderr, "JSON marshal error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(data))
		return
	}

	PrintDetailedInfo(os.Stdout, record)
}
