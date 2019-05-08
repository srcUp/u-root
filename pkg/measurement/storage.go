package measurement

import (
	"github.com/TrenchBoot/tpmtool/pkg/tpm"
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
	var a error
	return a
}
