package measurement

import (
	"encoding/json"
	"github.com/TrenchBoot/tpmtool/pkg/tpm"
	"log"
)

type StorageCollector struct {
	Type  string   `json:"type"`
	Paths []string `json:"paths"`
}

func NewStorageCollector(config []byte) (Collector, error) {
	var sc = new(StorageCollector)
	err := json.Unmarshal(config, &sc)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

func (s *StorageCollector) Collect(t tpm.TPM) error {

	for _, blkDevicePath := range s.Paths {
		log.Printf("Measuring content in block device Path=%s\n", blkDevicePath)
		err := MeasureInputFile(t, blkDevicePath+":/")
		if err != nil {
			log.Printf("MeasureInputVal err = %v", err)
		}
	}

	return nil
}
