package main

import (
	"reflect"
	"testing"
)

func TestParseSwVers(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantOS     string
		wantOSID   string
	}{
		{
			name: "macOS 14 Sonoma",
			input: `ProductName:		macOS
ProductVersion:		14.4.1
BuildVersion:		23E224`,
			wantOS:   "macOS 14.4.1",
			wantOSID: "macos",
		},
		{
			name: "macOS 15 Sequoia",
			input: `ProductName:	macOS
ProductVersion:	15.0
BuildVersion:	24A335`,
			wantOS:   "macOS 15.0",
			wantOSID: "macos",
		},
		{
			name: "macOS Server / legacy",
			input: `ProductName:	Mac OS X
ProductVersion:	10.15.7
BuildVersion:	19H2`,
			wantOS:   "Mac OS X 10.15.7",
			wantOSID: "macos",
		},
		{
			name:     "Empty output fallback",
			input:    "",
			wantOS:   "macOS",
			wantOSID: "macos",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOS, gotOSID := parseSwVers(tt.input)
			if gotOS != tt.wantOS || gotOSID != tt.wantOSID {
				t.Errorf("parseSwVers() = (%q, %q), want (%q, %q)", gotOS, gotOSID, tt.wantOS, tt.wantOSID)
			}
		})
	}
}

func TestParseSysctlCPU(t *testing.T) {
	tests := []struct {
		name    string
		brand   string
		hwModel string
		arch    string
		want    string
	}{
		{
			name:    "Apple M1 Pro",
			brand:   "Apple M1 Pro",
			hwModel: "MacBookPro18,3",
			arch:    "arm64",
			want:    "Apple M1 Pro",
		},
		{
			name:    "Apple M3 Max with extra spaces",
			brand:   "Apple  M3   Max",
			hwModel: "Mac15,9",
			arch:    "arm64",
			want:    "Apple M3 Max",
		},
		{
			name:    "Intel Core i9",
			brand:   "Intel(R) Core(TM) i9-9880H CPU @ 2.30GHz",
			hwModel: "MacBookPro16,1",
			arch:    "amd64",
			want:    "Intel(R) Core(TM) i9-9880H CPU @ 2.30GHz",
		},
		{
			name:    "Empty brand fallback to hwModel",
			brand:   "",
			hwModel: "Macmini9,1",
			arch:    "arm64",
			want:    "Macmini9,1",
		},
		{
			name:    "Empty brand and hwModel on arm64",
			brand:   "",
			hwModel: "",
			arch:    "arm64",
			want:    "Apple Silicon",
		},
		{
			name:    "Empty brand and hwModel on amd64",
			brand:   "",
			hwModel: "",
			arch:    "amd64",
			want:    "Intel",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSysctlCPU(tt.brand, tt.hwModel, tt.arch)
			if got != tt.want {
				t.Errorf("parseSysctlCPU() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseSysctlMem(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  uint64
	}{
		{"16 GB", "17179869184\n", 17179869184},
		{"32 GB", "34359738368", 34359738368},
		{"64 GB", "68719476736\n", 68719476736},
		{"Invalid", "invalid", 0},
		{"Empty", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSysctlMem(tt.input)
			if got != tt.want {
				t.Errorf("parseSysctlMem(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseDarwinVirt(t *testing.T) {
	tests := []struct {
		name        string
		hvVmm       string
		hwModel     string
		cpuFeatures string
		want        string
	}{
		{
			name:        "Bare metal M1 Mac",
			hvVmm:       "0",
			hwModel:     "MacBookPro18,1",
			cpuFeatures: "",
			want:        "none",
		},
		{
			name:        "Apple Virtualization Framework / Tart",
			hvVmm:       "1",
			hwModel:     "VirtualMac2,1",
			cpuFeatures: "",
			want:        "apple-vm",
		},
		{
			name:        "Parallels Desktop VM",
			hvVmm:       "1",
			hwModel:     "ParallelsARM1,1",
			cpuFeatures: "",
			want:        "parallels",
		},
		{
			name:        "VMware Fusion VM",
			hvVmm:       "1",
			hwModel:     "VMware7,1",
			cpuFeatures: "",
			want:        "vmware",
		},
		{
			name:        "QEMU / UTM VM",
			hvVmm:       "1",
			hwModel:     "QEMU",
			cpuFeatures: "",
			want:        "qemu",
		},
		{
			name:        "Generic hypervisor without model match",
			hvVmm:       "1",
			hwModel:     "GenericDevice",
			cpuFeatures: "",
			want:        "vm",
		},
		{
			name:        "Intel Mac VM with VMM cpu feature",
			hvVmm:       "0",
			hwModel:     "MacBookPro15,1",
			cpuFeatures: "FPU VME DE PSE TSC MSR PAE MCE CX8 APIC SEP MTRR PGE MCA CMOV PAT PSE36 CLFSH DS ACPI MMX FXSR SSE SSE2 SS HTT TM PBE SSE3 PCLMULQDQ DTES64 MON DSCPL VMX EST TM2 SSSE3 FMA CX16 TPR PDCM SSE4.1 SSE4.2 x2APIC MOVBE POPCNT AES PCID XSAVE OSXSAVE SEGLIM64 TSCTMR AVX1.0 RDRAND F16C VMM",
			want:        "vm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDarwinVirt(tt.hvVmm, tt.hwModel, tt.cpuFeatures)
			if got != tt.want {
				t.Errorf("parseDarwinVirt() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseSystemProfilerGPUs(t *testing.T) {
	// 1. Apple Silicon M1 Pro JSON
	m1JSON := []byte(`{
  "SPDisplaysDataType" : [
    {
      "_name" : "Apple M1 Pro",
      "spdisplays_ndrvs" : [
        {
          "_name" : "Built-in Display",
          "_spdisplays_display-vendor-id" : "610",
          "_spdisplays_display-product-id" : "a048",
          "_spdisplays_resolution" : "3024 x 1964"
        }
      ],
      "spdisplays_vendor" : "sppci_vendor_Apple"
    }
  ]
}`)
	gotM1 := parseSystemProfilerGPUs(m1JSON, "")
	wantM1 := []string{"Apple M1 Pro"}
	if !reflect.DeepEqual(gotM1, wantM1) {
		t.Errorf("parseSystemProfilerGPUs(M1) = %v, want %v", gotM1, wantM1)
	}

	// 2. Dual-GPU Intel MacBook Pro (Intel UHD 630 + AMD Radeon Pro 5500M)
	dualJSON := []byte(`{
  "SPDisplaysDataType" : [
    {
      "_name" : "Intel UHD Graphics 630",
      "spdisplays_vendor" : "sppci_vendor_intel"
    },
    {
      "_name" : "AMD Radeon Pro 5500M",
      "spdisplays_vendor" : "sppci_vendor_amd",
      "spdisplays_vram" : "4 GB"
    }
  ]
}`)
	gotDual := parseSystemProfilerGPUs(dualJSON, "")
	wantDual := []string{"Intel UHD Graphics 630", "AMD Radeon Pro 5500M"}
	if !reflect.DeepEqual(gotDual, wantDual) {
		t.Errorf("parseSystemProfilerGPUs(Dual) = %v, want %v", gotDual, wantDual)
	}

	// 3. Text fallback
	textOut := `Graphics/Displays:

    Apple M2 Max:

      Chipset Model: Apple M2 Max
      Type: GPU
      Bus: Built-In
      Total Number of Cores: 38
`
	gotText := parseSystemProfilerGPUs(nil, textOut)
	wantText := []string{"Apple M2 Max"}
	if !reflect.DeepEqual(gotText, wantText) {
		t.Errorf("parseSystemProfilerGPUs(text) = %v, want %v", gotText, wantText)
	}
}

func TestParsePhysicalDisks(t *testing.T) {
	diskutilListOutput := `/dev/disk0 (internal, physical):
   #:                       TYPE NAME                    SIZE       IDENTIFIER
   0:      GUID_partition_scheme                        *1.0 TB     disk0
   1:                        EFI EFI                     314.6 MB   disk0s1
   2:                 Apple_APFS Container disk3         1.0 TB     disk0s2

/dev/disk1 (external, physical):
   #:                       TYPE NAME                    SIZE       IDENTIFIER
   0:      GUID_partition_scheme                        *2.0 TB     disk1
   1:               Apple_APFS Container disk4         2.0 TB     disk1s1

/dev/disk2 (disk image):
   #:                       TYPE NAME                    SIZE       IDENTIFIER
   0:                            Installer              +2.5 GB     disk2

/dev/disk3 (synthesized):
   #:                       TYPE NAME                    SIZE       IDENTIFIER
   0:      APFS Container Scheme -                      +1.0 TB     disk3
                                 Physical Store disk0s2
   1:                APFS Volume Macintosh HD - Data     245.0 GB   disk3s1

/dev/disk4 (synthesized):
   #:                       TYPE NAME                    SIZE       IDENTIFIER
   0:      APFS Container Scheme -                      +2.0 TB     disk4
                                 Physical Store disk1s1
   1:                APFS Volume External Backup         500.0 GB   disk4s1
`

	got := parsePhysicalDisks(diskutilListOutput)
	want := []string{"disk0", "disk1"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("parsePhysicalDisks() = %v, want %v", got, want)
	}
}

func TestParseDiskutilInfo(t *testing.T) {
	diskutilInfoOutput := `   Device Identifier:        disk0
   Device Node:              /dev/disk0
   Whole:                    Yes
   Part of Whole:            disk0
   Device / Media Name:      APPLE SSD AP1024Z
   Volume Name:              Not applicable (no file system)
   Mounted:                  No
   File System:              None

   Disk Size:                1.0 TB (1000204886016 Bytes) (exactly 1953525168 512-Byte-Units)
   Device Block Size:        4096 Bytes

   Device Location:          Internal
   Removable Media:          Fixed

   Solid State:              Yes
   Virtual:                  No
`

	name, model, size := parseDiskutilInfo(diskutilInfoOutput)

	if name != "disk0" {
		t.Errorf("name = %q, want %q", name, "disk0")
	}
	if model != "APPLE SSD AP1024Z" {
		t.Errorf("model = %q, want %q", model, "APPLE SSD AP1024Z")
	}
	if size != 1000204886016 {
		t.Errorf("size = %d, want %d", size, uint64(1000204886016))
	}
}
