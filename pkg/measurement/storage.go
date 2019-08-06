package measurement

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/TrenchBoot/tpmtool/pkg/tpm"
	"github.com/u-root/u-root/pkg/diskboot"
	"github.com/u-root/u-root/pkg/mount"
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
func ReadDisk(mountPath string) (byteCount int, buffer *bytes.Buffer, e error) {

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

	byteCount = buffer.Len()
	if err != io.EOF {
		log.Printf("Error Reading ", mountPath, ": ", err)
		return byteCount, nil, err
	} else {
		err = nil
	}
	return byteCount, buffer, nil
}

/* - mount block device
 * - Read block device in chunks into a buffer
 * - Unmount block device
 * - Measure buffer and store in TPM.
 */
func MeasureStorageDevice(t *tpm.TPM, blkDevicePath string) error {
	log.Printf("Storage Collector: Measuring block device %s\n", blkDevicePath)
	dev, err := diskboot.FindDevice(blkDevicePath) // FindDevice fn mounts devicePath=/dev/sda.
	if err != nil {
		return err
	}

	buflen, buf, err := ReadDisk(dev.MountPath)
	if err != nil {
		return err
	}

	if e := mount.Unmount(dev.MountPath, true, false); e != nil {
		log.Printf("Storage Collector: Unmount failed. PANIC\n")
		panic(e)
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
