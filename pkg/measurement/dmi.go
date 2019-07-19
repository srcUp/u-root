package measurement

import (
	"github.com/TrenchBoot/tpmtool/pkg/tpm"
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

func (s *DmiCollector) Collect(t *tpm.TPM) error {
	return nil
}
