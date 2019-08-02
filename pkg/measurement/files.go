package measurement

import (
	"encoding/json"
	"fmt"
	"github.com/TrenchBoot/tpmtool/pkg/tpm"
	"github.com/u-root/u-root/pkg/diskboot"
	"io"
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
	fmt.Printf("New Files Collector\n")
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
func MeasureInputFile(t *tpm.TPM, inputVal string) error {
	s := strings.Split(inputVal, ":")
	if len(s) != 2 {
		return fmt.Errorf("%s: Usage: <block device identifier>:<path>", inputVal)
	}

	devicePath := filepath.Join("/dev", s[0])   // assumes deviceId is sda, devicePath=/dev/sda
	dev, err := diskboot.FindDevice(devicePath) // FindDevice fn mounts devicePath=/dev/sda.
	if err != nil {
		return err
	}

	filePath := dev.MountPath + s[1] // mountPath=/tmp/path/to/target/file if /dev/sda mounted on /tmp
	fmt.Printf("File Collector reading file=%s\n", filePath)
	d, err := ioutil.ReadFile(filePath)
	if err != io.EOF {
		return fmt.Errorf("Error reading target file: mountPath=%s, devicePath=%s, passed=%s",
			filePath, devicePath, inputVal)
	}
	(*t).Measure(pcrIndex, d)
	return nil
}

func (s *FileCollector) Collect(t *tpm.TPM) error {

	for _, inputVal := range s.Paths {
		err := MeasureInputFile(t, inputVal)
		if err != nil {
			log.Printf("hashInputFile err = %v", err)
		}
	}

	return nil
}
