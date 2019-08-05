package measurement

import (
	"encoding/json"
	"fmt"
	"github.com/TrenchBoot/tpmtool/pkg/tpm"
	"github.com/u-root/u-root/pkg/diskboot"
	"github.com/u-root/u-root/pkg/mount"
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
	log.Printf("New Files Collector initialized\n")
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
/* - mount device
 * - Read file on device into a byte slice.
 * - Unmount device
 * - Measure byte slice and store in TPM.
 */
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
	log.Printf("File Collector: reading file=%s\n", filePath)
	d, err := ioutil.ReadFile(filePath)
	if e := mount.Unmount(dev.MountPath, true, false); e != nil {
		log.Printf("File Collector: Unmount failed. PANIC\n")
		panic(e)
	}

	if err != io.EOF {
		return fmt.Errorf("Error reading target file: mountPath=%s, devicePath=%s, passed=%s",
			filePath, devicePath, inputVal)
	}

	return (*t).Measure(pcrIndex, d)
}

func (s *FileCollector) Collect(t *tpm.TPM) error {

	for _, inputVal := range s.Paths {
		// inputVal is of type sda:/path/to/file
		err := MeasureInputFile(t, inputVal)
		if err != nil {
			log.Printf("File Collector: input=%s, err = %v", inputVal, err)
			return err
		}
	}

	return nil
}
