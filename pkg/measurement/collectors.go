package measurement

import (
	"encoding/json"
	"fmt"

	"github.com/TrenchBoot/tpmtool/pkg/tpm"
)

type Collector interface {
	Collect(t *tpm.TPM) error
}

var supportedCollectors = map[string]func([]byte) (Collector, error){
	"storage": NewStorageCollector,
}

func GetCollector(config []byte) (Collector, error) {
	var header struct {
		Type string `json:"type"`
	}
	err := json.Unmarshal(config, &header)
	if err != nil {
		return nil, err
	}

	if init, ok := supportedCollectors[header.Type]; ok {
		return init(config)
	}

	return nil, fmt.Errorf("unsupported collector %s", header.Type)
}
