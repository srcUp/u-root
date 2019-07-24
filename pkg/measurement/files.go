package measurement

import (
	"encoding/json"
	"fmt"
	"github.com/TrenchBoot/tpmtool/pkg/tpm"
	"github.com/u-root/u-root/pkg/diskboot"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"
)

type FileCollector struct {
	Type  string   `json:"type"`
	Paths []string `json:"paths"`
}

func NewFileCollector(config []byte) (Collector, error) {
	var fc = new(FileCollector)
	err := json.Unmarshal(config, &fc)
	if err != nil {
		return nil, err
	}
	return fc, nil
}

// measures file input by user in policy file and store in TPM.
// inputVal is of format <block device identifier>:<path>
// Example sda:/path/to/file
func MeasureInputFile(t tpm.TPM, inputVal string) error {
	s := strings.Split(inputVal, ":")
	if len(s) != 2 {
		return fmt.Errorf("%v: incorrect format. Usage: <block device identifier>:<path>", inputVal)
	}

	devicePath := filepath.Join("/dev", s[0])   // assumes deviceId is sda, devicePath=/dev/sda
	dev, err := diskboot.FindDevice(devicePath) // FindDevice fn mounts devicePath=/dev/sda.
	if err != nil {
		return err
	}

	mountPath := dev.MountPath + s[1] // mountPath=/tmp/path/to/target/file if /dev/sda mounted on /tmp
	d, err := ioutil.ReadFile(mountPath)
	if err != nil {
		// - TODO: should we check for end of file ?
		return fmt.Errorf("Error reading target file found at mountPath=%s, devicePath=%s, passed=%s",
			mountPath, devicePath, inputVal)
	}
	t.Measure(pcrIndex, d)
	return nil
}

func (s *FileCollector) Collect(t tpm.TPM) error {

	for _, inputVal := range s.Paths {
		err := MeasureInputFile(t, inputVal)
		if err != nil {
			log.Printf("hashInputFile err = %v", err)
		}
	}

	return nil
}
