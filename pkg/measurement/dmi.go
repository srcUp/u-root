package measurement

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/TrenchBoot/tpmtool/pkg/tpm"
	"github.com/digitalocean/go-smbios/smbios"
	"log"
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
	log.Printf("New DMI Collector initialized\n")
	var dc = new(DmiCollector)
	err := json.Unmarshal(config, &dc)
	if err != nil {
		return nil, err
	}
	return dc, nil
}

// below table is from man dmidecode
// couldn't use enum because of space in names
// const (
// BIOS dmiTypeId = iota
var type_table = map[string]uint8{
	"BIOS":                             0,
	"System":                           1,
	"Base Board":                       2,
	"Chassis":                          3,
	"Processor":                        4,
	"Memory Controller":                5,
	"Memory Module":                    6,
	"Cache":                            7,
	"Port Connector":                   8,
	"System Slots":                     9,
	"On Board Devices":                 10,
	"OEM Strings":                      11,
	"System Configuration Options":     12,
	"BIOS Language":                    13,
	"Group Associations":               14,
	"System Event Log":                 15,
	"Physical Memory Array":            16,
	"Memory Device":                    17,
	"32-bit Memory Error":              18,
	"Memory Array Mapped Address":      19,
	"Memory Device Mapped Address":     20,
	"Built-in Pointing Device":         21,
	"Portable Battery":                 22,
	"System Reset":                     23,
	"Hardware Security":                24,
	"System Power Controls":            25,
	"Voltage Probe":                    26,
	"Cooling Device":                   27,
	"Temperature Probe":                28,
	"Electrical Current Probe":         29,
	"Out-of-band Remote Access":        30,
	"Boot Integrity Services":          31,
	"System Boot":                      32,
	"64-bit Memory Error":              33,
	"Management Device":                34,
	"Management Device Component":      35,
	"Management Device Threshold Data": 36,
	"Memory Channel":                   37,
	"IPMI Device":                      38,
	"Power Supply":                     39,
	"Additional Information":           40,
	"Onboard Device":                   41,
}

func (s *DmiCollector) Collect(t *tpm.TPM) error {
	log.Printf("DMI Collector: Entering \n")
	if s.Type != "dmi" {
		return errors.New("Invalid type passed to a DmiCollector method")
	}

	// Find SMBIOS data in operating system-specific location.
	rc, _, err := smbios.Stream()
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
			if k.Header.Type != type_table[label] {
				continue
			}

			log.Printf("DMI Collector: Hashing %s information\n", label)
			b := new(bytes.Buffer)
			for _, str := range k.Strings {
				// log.Printf("Storing %s\n", str)
				b.WriteString(str)
			}

			// log.Printf("%T", *t)
			// TODO: Extract and Measure specific "Fields" of a FieldCluster on user's request.
			// For example: for BIOS type(type=0), currently we measure entire output
			// but in future we could measure individual fields like bios-vendor, bios-version etc.
			if e := (*t).Measure(pcrIndex, b.Bytes()); e != nil {
				return e
			}
		}
	}

	return nil
}
