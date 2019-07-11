package measurement

import (
	"errors"
	"fmt"
	"github.com/TrenchBoot/tpmtool/pkg/tpm"
	"github.com/digitalocean/go-smbios/smbios"
)

// DMI Events are expected to be a COMBINED_EVENT extend, as such the json
// definition is designed to allow clusters of DMI fields/strings.
//
// Example json:
//	{
//		"type": "dmi",
//		[
//			{
//				"label": "BIOS",
//				"fields": [
//					"bios-vendor",
//					"bios-version",
//					"bios-release-date"
//				]
//			}
//			{
//				"label": "System",
//				"fields": [
//					"system-manufacturer",
//					"system-product-name",
//					"system-version"
//				]
//			}
//		]
//	}
type fieldCluster struct {
	Label  string   `json:"label"`
	Fields []string `json:"fields"`
}

type DmiCollector struct {
	Type     string         `json:"type"`
	Clusters []fieldCluster `json:"events"`
}

func NewDmiCollector(config []byte) (Collector, error) {
	return new(DmiCollector), nil
}

const (
	PcrIndex uint32 = 23
)


// below table is from man dmidecode
type dmiTypeId int

// couldn't use const because of space in names
const (
	BIOS dmiTypeId = iota
    System
    Base Board
    Chassis
	Processor
    Memory Controller
    Memory Module
    Cache
    Port Connector
    System Slots
    On Board Devices
    OEM Strings
    System Configuration Options
    BIOS Language
    Group Associations
    System Event Log
    Physical Memory Array
    Memory Device
    32-bit Memory Error
    Memory Array Mapped Address
    Memory Device Mapped Address
    Built-in Pointing Device
    Portable Battery
    System Reset
    Hardware Security
    System Power Controls
    Voltage Probe
    Cooling Device
    Temperature Probe
    Electrical Current Probe
    Out-of-band Remote Access
    Boot Integrity Services
    System Boot
    64-bit Memory Error
    Management Device
    Management Device Component
    Management Device Threshold Data
    Memory Channel
    IPMI Device
    Power Supply
	Additional Information
	Onboard Device
)

func (s *DmiCollector) Collect(t *tpm.TPM) error {
	if s.Type != "dmi" {
		return errors.New("Invalid type passed to a DmiCollector method")
	}

	// Find SMBIOS data in operating system-specific location.
	rc, ep, err := smbios.Stream()
	if err != nil {
		return fmt.Errorf("failed to open stream: %v", err)
	}

	// Be sure to close the stream!
	defer rc.Close()

	// Decode SMBIOS structures from the stream.
	d := smbios.NewDecoder(rc)
	data, err := d.Decode()
	if err != nil {
		return fmt.Errorf("failed to decode structures: %v", err)
	}

	var labels []string // collect all types entered by user in one slice
	for _, fieldCluster := range s.Clusters {
		labels = append(labels, fieldCluster.Label)
	}

	for _, k := range data { // k ==> data for each dmi type
		// Only look at types mentioned in policy file.
		for _, label := range labels {
			if k.Type != label {
				continue
			}

			fmt.Printf("Hashing %s information\n", k.Type)
			// TODO: Measure specific parts of s structure on user's request.
			// For example: for BIOS type(type=0), currently we measure entire output
			// but in future we could measure individual parts like bios-vendor, bios-version etc.
			t.Measure(PcrIndex, k) // keep extending same pcr .
		}
	}

	return nil
}
