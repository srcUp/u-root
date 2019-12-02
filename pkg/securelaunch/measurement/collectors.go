package measurement

import (
	"encoding/json"
	"fmt"
	"io"
)

/*
 * pcr number where all measurements taken by securelaunch pkg
 * will be stored.
 */
const (
	pcr = int(22)
)

/*
 * all collectors (storage, dmi, cpuid, files) should satisfy this
 * collectors get information and store the hash of that information in pcr
 * owned by the tpm device.
 */
type Collector interface {
	Collect(t io.ReadWriteCloser) error
}

var supportedCollectors = map[string]func([]byte) (Collector, error){
	"storage": NewStorageCollector,
	"dmi":     NewDmiCollector,
	"files":   NewFileCollector,
	"cpuid":   NewCPUIDCollector,
}

/*
 * Each collector object in the "collectors" section of policy file
 * is passed as an arg to this function. This function calls the appropriate init handlers
 * for the particular collector passed to it.
 * Input
 * - config []byte -individual collector JSON object
 * Returns
 * - new Collector Interface, which is returned from the NewXXXCollector functions.
 * - error if unmarshalling fails or unsupported collector is passed.
 */
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
