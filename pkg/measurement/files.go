package measurement

import (
	"github.com/TrenchBoot/tpmtool/pkg/tpm"
)

type FileCollector struct {
	Type  string   `json:"type"`
	Paths []string `json:"paths"`
}

func NewFileCollector(config []byte) (Collector, error) {
	a := new(FileCollector)
	var b error
	return a, b
}

func (s *FileCollector) Collect(t *tpm.TPM) error {
	var a error
	return a
}
