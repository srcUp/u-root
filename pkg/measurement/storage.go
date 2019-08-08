package measurement

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/TrenchBoot/tpmtool/pkg/tpm"
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
	log.Printf("New Storage Collector initialized\n")
	var sc = new(StorageCollector)
	err := json.Unmarshal(config, &sc)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

// code found here: https://gist.github.com/minikomi/2900454
func ReadDisk(blkDevPath string) (byteCount int, buffer *bytes.Buffer, e error) {

	data, err := os.Open(blkDevPath)
	if err != nil {
		return 0, nil, err
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

	byteCount = buffer.Len()
	if err == io.EOF {
		log.Printf("End of disk. Read %v bytes\n", byteCount)
		return byteCount, buffer, nil
	}

	if err != nil {
		return byteCount, nil, fmt.Errorf("Error Reading ", blkDevPath, ": ", err)
	}
	return byteCount, buffer, nil
}

/* -
 * - Reads block device in one go: TODO: make this efficient
 * - Store data read above in TPM.
 */
func MeasureStorageDevice(t *tpm.TPM, blkDevicePath string) error {

	log.Printf("Storage Collector: Measuring block device %s\n", blkDevicePath)
	buflen, buf, err := ReadDisk(blkDevicePath)
	if err != nil {
		return fmt.Errorf("Storage Collector: buflen=%v, err=%v\n", buflen, err)
	}
	if buflen == 0 {
		return fmt.Errorf("Empty Disk %s Nothing to measure.\n", blkDevicePath)
	}
	return (*t).Measure(pcrIndex, buf.Bytes())
}

func (s *StorageCollector) Collect(t *tpm.TPM) error {

	for _, inputVal := range s.Paths {
		err := MeasureStorageDevice(t, inputVal) // inputVal is blkDevicePath e.g /dev/sda
		if err != nil {
			log.Printf("Storage Collector: input = %s, err = %v", inputVal, err)
			return err
		}
	}

	return nil
}
