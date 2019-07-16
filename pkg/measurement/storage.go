package measurement

import (
	"github.com/TrenchBoot/tpmtool/pkg/tpm"
	"log"
)

type StorageCollector struct {
	Type  string   `json:"type"`
	Paths []string `json:"paths"`
}

func NewStorageCollector(config []byte) (Collector, error) {
	a := new(StorageCollector)
	var b error
	return a, b
}

func (s *StorageCollector) Collect(t *tpm.TPM) error {

	for _, blkDevicePath := range s.Paths {
		log.Printf("Measuring content in block device Path=%s\n", blkDevicePath)
		err := MeasureInputFile(t, blkDevicePath+":/")
		if err != nil {
			log.Printf("MeasureInputVal err = %v", err)
		}
	}

	return nil
}
