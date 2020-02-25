// Copyright 2019 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package measurement

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	// "io/ioutil"
	"log"
	// "path/filepath"
	// "strconv"
	"strings"

	"github.com/u-root/u-root/pkg/smbios"
	// "github.com/digitalocean/go-smbios/smbios"
	slaunch "github.com/u-root/u-root/pkg/securelaunch"
	"github.com/u-root/u-root/pkg/securelaunch/tpm"
)

type fieldCluster struct {
	Label  string   `json:"label"`
	Fields []string `json:"fields"`
}

/* describes the "dmi" portion of policy file */
type DmiCollector struct {
	Type     string         `json:"type"`
	Clusters []fieldCluster `json:"events"`
}

/*
 * NewDmiCollector extracts the "dmi" portion from the policy file.
 * initializes a new DmiCollector structure.
 * returns error if unmarshalling of DmiCollector fails
 */
func NewDmiCollector(config []byte) (Collector, error) {
	slaunch.Debug("New DMI Collector initialized")
	var dc = new(DmiCollector)
	err := json.Unmarshal(config, &dc)
	if err != nil {
		return nil, err
	}
	return dc, nil
}

//const (
//	sysfsPath = "/sys/firmware/dmi/tables"
//)

/*
 * below look up table is from man dmidecode.
 * used to lookup the dmi type parsed from policy file.
 * e.g if policy file contains BIOS, this table would return 0.
 */
var typeTable = map[string]uint8{
	"bios":                             0,
	"system":                           1,
	"base board":                       2,
	"chassis":                          3,
	"processor":                        4,
	"memory controller":                5,
	"memory module":                    6,
	"cache":                            7,
	"port connector":                   8,
	"system slots":                     9,
	"on board devices":                 10,
	"oem strings":                      11,
	"system configuration options":     12,
	"bios language":                    13,
	"group associations":               14,
	"system event log":                 15,
	"physical memory array":            16,
	"memory device":                    17,
	"32-bit memory error":              18,
	"memory array mapped address":      19,
	"memory device mapped address":     20,
	"built-in pointing device":         21,
	"portable battery":                 22,
	"system reset":                     23,
	"hardware security":                24,
	"system power controls":            25,
	"voltage probe":                    26,
	"cooling device":                   27,
	"temperature probe":                28,
	"electrical current probe":         29,
	"out-of-band remote access":        30,
	"boot integrity services":          31,
	"system boot":                      32,
	"64-bit memory error":              33,
	"management device":                34,
	"management device component":      35,
	"management device threshold data": 36,
	"memory channel":                   37,
	"ipmi device":                      38,
	"power supply":                     39,
	"additional information":           40,
	"onboard device":                   41,
}

//var (
//	typeGroups = map[string][]uint8{
//		"bios":      {0, 13},
//		"system":    {1, 12, 15, 23, 32},
//		"baseboard": {2, 10, 41},
//		"chassis":   {3},
//		"processor": {4},
//		"memory":    {5, 6, 16, 17},
//		"cache":     {7},
//		"connector": {8},
//		"slot":      {9},
//	}
//)

// parseTypeFilter parses the --type argument(s) and returns a set of types that should be included.
//func parseTypeFilter(typeStrings []string) (map[smbios.TableType]bool, error) {
//	types := map[smbios.TableType]bool{}
//	for _, ts := range typeStrings {
//		log.Println("type=", ts)
//		if tg, ok := typeGroups[strings.ToLower(ts)]; ok {
//			for _, t := range tg {
//				log.Println("matched type in table=", t)
//				types[smbios.TableType(t)] = true
//			}
//		} else {
//			log.Println("I was in else loop")
//			u, err := strconv.ParseUint(ts, 0, 8)
//			if err != nil {
//				return nil, fmt.Errorf("Invalid type: %s", ts)
//			}
//			types[smbios.TableType(uint8(u))] = true
//		}
//	}
//	return types, nil
//}

//looks up type in table and sets the corresponding entry in map to true.
func parseTypeFilter(typeStrings []string) (map[smbios.TableType]bool, error) {
	types := map[smbios.TableType]bool{}
	for _, ts := range typeStrings {
		if tg, ok := typeTable[strings.ToLower(ts)]; ok {
			// log.Println("matched type in table=", tg)
			types[smbios.TableType(tg)] = true
		}
	}
	return types, nil
}

//// getData returns SMBIOS entry point and DMI table data.
//// read from sysfsPath (smbios_entry_point and DMI files respectively).
//func getData() ([]byte, []byte, error) {
//	log.Println("Reading SMBIOS/DMI data from sysfs.")
//	entry, err := ioutil.ReadFile(filepath.Join(sysfsPath, "smbios_entry_point"))
//	if err != nil {
//		return nil, nil, fmt.Errorf("error reading DMI data: %v", err)
//	}
//	data, err := ioutil.ReadFile(filepath.Join(sysfsPath, "DMI"))
//	if err != nil {
//		return nil, nil, fmt.Errorf("error reading DMI data: %v", err)
//	}
//	return entry, data, nil
//}

/*
 * Collect satisfies collector interface. It calls
 * 1. smbios package to get all smbios data,
 * 2. then, filters smbios data based on type provided in policy file, and
 * 3. the filtered data is then measured into the tpmHandle (tpm device).
 */
func (s *DmiCollector) Collect(tpmHandle io.ReadWriteCloser) error {
	slaunch.Debug("DMI Collector: Entering ")
	if s.Type != "dmi" {
		return errors.New("invalid type passed to a DmiCollector method")
	}

	var labels []string // collect all types entered by user in one slice
	for _, fieldCluster := range s.Clusters {
		labels = append(labels, fieldCluster.Label)
	}

	slaunch.Debug("DMI Collector: len(labels)=%d", len(labels))

	// lables would be []{BIOS, Chassis, Processor}
	typeFilter, err := parseTypeFilter(labels)
	if err != nil {
		return fmt.Errorf("invalid --type: %v", err)
	}

	slaunch.Debug("DMI Collector: len(typeFilter)=%d", len(typeFilter))

	//	entryData, tableData, err := getData()
	//	if err != nil {
	//		return fmt.Errorf("error parsing loading data: %v", err)
	//	}

	si, err := smbios.FromSysfs()
	if err != nil {
		return fmt.Errorf("error parsing loading data: %v", err)
	}

	//	si, err := smbios.ParseInfo(entryData, tableData)
	//	if err != nil {
	//		return fmt.Errorf("error parsing data: %v", err)
	//	}

	slaunch.Debug("DMI Collector: len(s.Tables)=%d", len(si.Tables))
	for _, t := range si.Tables {
		if len(typeFilter) != 0 && !typeFilter[t.Type] {
			// log.Println("skipping type=", t.Type)
			continue
		}

		// log.Println("parsing type=", t.Type)
		pt, err := smbios.ParseTypedTable(t)
		if err != nil {
			log.Printf("DMI Collector: skipping type %s, err=%v", t.Type, err)
			//if err == smbios.ErrUnsupportedTableType {
			//	log.Println("skipping unsupported type", t.Type)
			//} else {
			//	log.Printf("DMI Collector: err = %s", err)
			//}
			continue
			// Print as raw table
			// pt = t
		}
		// slaunch.Debug("DMI Collector: Hashing [%s] ", t.Type)

		slaunch.Debug(pt.String())
		// b := []byte(fmt.Sprintf("%s", pt)) golint complained to use String..
		b := []byte(pt.String())
		eventDesc := fmt.Sprintf("DMI Collector: Measured dmi label=[%v]", t.Type)
		if e := tpm.ExtendPCRDebug(tpmHandle, pcr, bytes.NewReader(b), eventDesc); e != nil {
			log.Printf("DMI Collector: err =%v", e)
			return e // return error if any single label fails ..
		}
		// slaunch.Debug("DMI Collector: Hashed info=[%s] for dmi type=[%v]", string(b), t.Type)
	}

	return nil
}

// old code below this line
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
//
///*
// * Collect satisfies collector interface. It calls
// * 1. smbios package to get all smbios data,
// * 2. then, filters smbios data based on type provided in policy file, and
// * 3. the filtered data is then measured into the tpmHandle (tpm device).
// */
//func (s *DmiCollector) Collect(tpmHandle io.ReadWriteCloser) error {
//	slaunch.Debug("DMI Collector: Entering ")
//	if s.Type != "dmi" {
//		return errors.New("Invalid type passed to a DmiCollector method")
//	}
//
//	// Find SMBIOS data in operating system-specific location.
//	rc, _, err := smbios.Stream()
//	if err != nil {
//		return fmt.Errorf("failed to open stream: %v", err)
//	}
//
//	// Be sure to close the stream!
//	defer rc.Close()
//
//	// Decode SMBIOS structures from the stream.
//	d := smbios.NewDecoder(rc)
//	data, err := d.Decode()
//	if err != nil {
//		return fmt.Errorf("failed to decode structures: %v", err)
//	}
//
//	//for _, s := range data {
//	//	log.Println("smbios: [", s, "]")
//	//}
//
//	var labels []string // collect all types entered by user in one slice
//	for _, fieldCluster := range s.Clusters {
//		labels = append(labels, fieldCluster.Label)
//	}
//
//	// log.Println("labels len=: [", len(labels), "]")
//	for _, k := range data { // k ==> data for each dmi type
//		// Only look at types mentioned in policy file.
//		for _, label := range labels {
//			// log.Println("k.Header.Type=[", k.Header.Type, "]  ", "type_table[label]=[", type_table[label], "]")
//			if k.Header.Type != type_table[label] {
//				continue
//			}
//
//			slaunch.Debug("DMI Collector: Hashing %s information", label)
//			b := new(bytes.Buffer)
//			for _, str := range k.Strings {
//				b.WriteString(str)
//			}
//
//			// TODO: Extract and Measure specific "Fields" of a FieldCluster on user's request.
//			// For example: for BIOS type(type=0), currently we measure entire output
//			// but in future we could measure individual fields like bios-vendor, bios-version etc.
//
//			eventDesc := []byte(fmt.Sprintf("DMI Collector: Measured dmi label=%s", label))
//			slaunch.Debug(string(eventDesc))
//			if e := tpm.ExtendPCRDebug(tpmHandle, pcr, bytes.NewReader(b.Bytes()), eventDesc); e != nil {
//				log.Printf("DMI Collector: err =%v", e)
//				return e
//			}
//			// break here maybe , so that we dont iterate over remaining labels for this structure from go-smbios
//			// log.Println("end of iteration of this label, ideally i should break here")
//		}
//	}
//
//	return nil
//}
