package measurement

import (
	"github.com/TrenchBoot/tpmtool/pkg/tpm"
)

type FileCollector struct {
	Type  string   `json:"type"`
	Paths []string `json:"paths"`
}

func NewFileCollector(config []byte) (Collector, error) {
	return new(FileCollector), nil
}

func (s *FileCollector) Collect(t *tpm.TPM) error {
	return nil
}
