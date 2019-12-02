package measurement

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/u-root/u-root/pkg/securelaunch/tpm"
	"io"
	"log"
	"os"
)

/* disk reads are done in chunks of this size */
const (
	chunksize int = 1024
)

/* describes the "storage" portion of policy file */
type StorageCollector struct {
	Type  string   `json:"type"`
	Paths []string `json:"paths"`
}

/*
 * NewStorageCollector extracts the "storage" portion from the policy file.
 * initializes a new StorageCollector structure.
 * returns error if unmarshalling of StorageCollector fails
 */
func NewStorageCollector(config []byte) (Collector, error) {
	log.Printf("New Storage Collector initialized\n")
	var sc = new(StorageCollector)
	err := json.Unmarshal(config, &sc)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

/*
 * ReadDisk function uses os package to open a blk device (e.g /dev/sda)
 * and uses a bufio buffer to read the block device in chunks ('chunksize').
 * returns
 * buffer - the disk is read into this.
 * bytecount - number of bytes read.
 * error if opening the device fails or reading the device fails.
 * code found here: https://gist.github.com/minikomi/2900454
 */
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
 * - Reads block device by making a call to ReadDisk()
 * - tpmHandle - tpm device where measurements are stored.
 * - blkDevicePath - string e.g /dev/sda
 * returns
 * - error if Reading the block device fails, _OR_ if disk is empty.
 */
func MeasureStorageDevice(tpmHandle io.ReadWriteCloser, blkDevicePath string) error {

	log.Printf("Storage Collector: Measuring block device %s\n", blkDevicePath)
	buflen, buf, err := ReadDisk(blkDevicePath)
	if err != nil {
		return fmt.Errorf("Storage Collector: buflen=%v, err=%v\n", buflen, err)
	}
	if buflen == 0 {
		return fmt.Errorf("Empty Disk %s Nothing to measure.\n", blkDevicePath)
	}

	if e := tpm.ExtendPCRDebug(tpmHandle, pcr, buf.Bytes()); e != nil {
		return e
	}

	return nil
}

/* satisfies Collector Interface */
func (s *StorageCollector) Collect(tpmHandle io.ReadWriteCloser) error {

	for _, inputVal := range s.Paths {
		err := MeasureStorageDevice(tpmHandle, inputVal) // inputVal is blkDevicePath e.g /dev/sda
		if err != nil {
			log.Printf("Storage Collector: input = %s, err = %v", inputVal, err)
			return err
		}
	}

	return nil
}
