package measurement

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	// "github.com/google/go-tpm/tpm2"
)

type Collector interface {
	Collect(t io.ReadWriter) error
}

const (
	//pcr             = int(16)
	pcr             = int(22)
	pcrIndex uint32 = 23
)

var supportedCollectors = map[string]func([]byte) (Collector, error){
	"storage": NewStorageCollector,
	"dmi":     NewDmiCollector,
	"files":   NewFileCollector,
}

func hashSum(in []byte) []byte {
	s := sha256.Sum256(in)
	return s[:]
}

func GetCollector(config []byte) (Collector, error) {
	var header struct {
		Type string `json:"type"`
	}
	err := json.Unmarshal(config, &header)
	if err != nil {
		fmt.Printf("Measurement: Unmarshal error\n")
		return nil, err
	}

	if init, ok := supportedCollectors[header.Type]; ok {
		return init(config)
	}

	return nil, fmt.Errorf("unsupported collector %s", header.Type)
}
