package measurement

import (
	"encoding/json"
	"fmt"

	"github.com/TrenchBoot/tpmtool/pkg/tpm"
)

type Collector interface {
	Collect(t tpm.TPM) error
}

const (
	pcrIndex uint32 = 23
)

var supportedCollectors = map[string]func([]byte) (Collector, error){
	"storage": NewStorageCollector,
	"dmi":     NewDmiCollector,
	"files":   NewFileCollector,
}

func GetCollector(config []byte) (Collector, error) {
	var header struct {
		Type string `json:"type"`
	}
	err := json.Unmarshal(config, &header)
	if err != nil {
		fmt.Printf("Unmarshal errir in measurement pkg\n")
		return nil, err
	}

	if init, ok := supportedCollectors[header.Type]; ok {
		fmt.Printf("init=%v\n", init)
		return init(config)
	}

	return nil, fmt.Errorf("unsupported collector %s", header.Type)
}
