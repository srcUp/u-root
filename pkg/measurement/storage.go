package measurement

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/TrenchBoot/tpmtool/pkg/tpm"
	"github.com/u-root/u-root/pkg/diskboot"
	"io"
	"log"
	"os"
)

const (
	chunksize int = 1024
)

type StorageCollector struct {
	Type  string   `json:"type"`
	Paths []string `json:"paths"`
}

func NewStorageCollector(config []byte) (Collector, error) {
	var sc = new(StorageCollector)
	err := json.Unmarshal(config, &sc)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

// code found here: https://gist.github.com/minikomi/2900454
func ReadDisk(mountPath string) (byteCount int, buffer *bytes.Buffer) {

	data, err := os.Open(mountPath)
	if err != nil {
		log.Fatal(err)
	}
	defer data.Close()

	reader := bufio.NewReader(data)
	buffer = bytes.NewBuffer(make([]byte, 0))
	part := make([]byte, chunksize)
	var count int

	for {
		if count, err = reader.Read(part); err != nil {
			break
		}
		buffer.Write(part[:count])
	}
	if err != io.EOF {
		log.Fatal("Error Reading ", mountPath, ": ", err)
	} else {
		err = nil
	}

	byteCount = buffer.Len()
	return
}

func (s *StorageCollector) Collect(t *tpm.TPM) error {

	for _, blkDevicePath := range s.Paths {
		log.Printf("Measuring content in block device Path=%s\n", blkDevicePath)

		dev, err := diskboot.FindDevice(blkDevicePath) // FindDevice fn mounts devicePath=/dev/sda.
		if err != nil {
			return err
		}

		buflen, buf := ReadDisk(dev.MountPath)
		if buflen == 0 {
			return fmt.Errorf("Empty Disk %s Nothing to measure.\n", blkDevicePath)
		}
		(*t).Measure(pcrIndex, buf.Bytes())
	}

	return nil
}
