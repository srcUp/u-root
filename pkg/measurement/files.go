package main

import (
	"encoding/json"

	"github.com/TrenchBoot/tpmtool/pkg/tpm"
)

type FileCollector struct {
	Type  string   `json:"type"`
	Paths []string `json:"paths"`
}

func NewFileCollector(config []byte) (Collector, error) {
}

func (s *FileCollector) Collect(t *tpm.TPM) error {
}
