package measurement

import (
	"encoding/json"
	"fmt"
	"io"
)

const (
	pcr = int(22)
)

type Collector interface {
	Collect(t io.ReadWriteCloser) error
}

var supportedCollectors = map[string]func([]byte) (Collector, error){
	"storage": NewStorageCollector,
	"dmi":     NewDmiCollector,
	"files":   NewFileCollector,
	"cpuid":   NewCPUIDCollector,
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
