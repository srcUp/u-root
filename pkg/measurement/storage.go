package main

import (
	"encoding/json"

	"github.com/TrenchBoot/tpmtool/pkg/tpm"
)

type StorageCollector struct {
	Type  string   `json:"type"`
	Paths []string `json:"paths"`
}

func NewStorageCollector(config []byte) (Collector, error) {
}

func (s *StorageCollector) Collect(t *tpm.TPM) error {
}
