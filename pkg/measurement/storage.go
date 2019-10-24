package measurement

import (
	"github.com/TrenchBoot/tpmtool/pkg/tpm"
)

type StorageCollector struct {
	Type  string   `json:"type"`
	Paths []string `json:"paths"`
}

func NewStorageCollector(config []byte) (Collector, error) {
	return new(StorageCollector), nil
}

func (s *StorageCollector) Collect(t *tpm.TPM) error {
	return nil
}
