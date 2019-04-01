package main

import (
	"encoding/json"

	"github.com/TrenchBoot/tpmtool/pkg/tpm"
	"github.com/digitalocean/go-smbios"
	"github.com/ugorji/go/codec"
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
	Type     string `json:"type"`
	Clusters []fieldCluster
}

func NewDmiCollector(config []byte) (Collector, error) {
}

func (s *DmiCollector) Collect(t *tpm.TPM) error {
}
