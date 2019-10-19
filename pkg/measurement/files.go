package measurement

import (
	"encoding/json"
	"fmt"
	"github.com/google/go-tpm/tpm2"
	"github.com/google/go-tpm/tpmutil"
	// "github.com/TrenchBoot/tpmtool/pkg/tpm"
	"bytes"
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

func MeasureCPUIDFile(rwc io.ReadWriter, inputVal string) error {
	filePath := inputVal
	d, err := ioutil.ReadFile(filePath)
	if err == io.EOF {
		return fmt.Errorf("EOF error")
	}

	if err != nil {
		return fmt.Errorf("Error reading target file: filePath=%s, tmpfs(No Mount Operation Needed), inputVal=%s, err=%v",
			filePath, inputVal, err)
	}

	oldPCRValue, err := tpm2.ReadPCR(rwc, pcr, tpm2.AlgSHA256)
	if err != nil {
		log.Fatal("Can't read PCR %d from the TPM: %s", pcr, err)
	}
	log.Printf("File Collector: oldPCRValue = [%x]", oldPCRValue)

	hash := hashSum(d)
	log.Printf("File Collector: Measured %s, Adding hash=[%x] to PCR #%d", filePath, hash, pcr)
	if e := tpm2.PCRExtend(rwc, tpmutil.Handle(pcr), tpm2.AlgSHA256, hash, ""); e != nil {
		return e
	}

	newPCRValue, err := tpm2.ReadPCR(rwc, pcr, tpm2.AlgSHA256)
	if err != nil {
		log.Fatal("Can't read PCR %d from the TPM: %s", pcr, err)
	}

	log.Printf("File Collector: newPCRValue = [%x]", newPCRValue)

	finalPCR := hashSum(append(oldPCRValue, hash...))
	if !bytes.Equal(finalPCR, newPCRValue) {
		log.Fatal("PCRs not equal, got %x, want %x", finalPCR, newPCRValue)
	}
	return nil
}

// measures file input by user in policy file and store in TPM.
// inputVal is of format <block device identifier>:<path>
// Example sda:/path/to/file
/* - mount device
 * - Read file on device into a byte slice.
 * - Unmount device
 * - Measure byte slice and store in TPM.
 */
func MeasureInputFile(rwc io.ReadWriter, inputVal string) error {
	s := strings.Split(inputVal, ":")
	if len(s) == 1 {
		return MeasureCPUIDFile(rwc, inputVal) // special case
	}

	if len(s) != 2 {
		return fmt.Errorf("%s: Usage: <block device identifier>:<path>", inputVal)
	}

	devicePath := filepath.Join("/dev", s[0])   // assumes deviceId is sda, devicePath=/dev/sda
	dev, err := diskboot.FindDevice(devicePath) // FindDevice fn mounts devicePath=/dev/sda.
	if err != nil {
		return fmt.Errorf("diskboot.FindDevice error=%v", err)
	}

	filePath := dev.MountPath + s[1] // mountPath=/tmp/path/to/target/file if /dev/sda mounted on /tmp

	log.Printf("File Collector: fileP=%s, mountP=%s\n", filePath, dev.MountPath)
	d, err := ioutil.ReadFile(filePath)
	if e := mount.Unmount(dev.MountPath, true, false); e != nil {
		log.Printf("File Collector: Unmount failed. PANIC\n")
		panic(e)
	}

	if err == io.EOF {
		return fmt.Errorf("EOF error")
	}

	if err != nil {
		return fmt.Errorf("Error reading target file: filePath=%s, mountPath=%s, inputVal=%s, err=%v",
			filePath, dev.MountPath, inputVal, err)
	}

	oldPCRValue, err := tpm2.ReadPCR(rwc, pcr, tpm2.AlgSHA256)
	if err != nil {
		log.Fatal("Can't read PCR %d from the TPM: %s", pcr, err)
	}
	log.Printf("File Collector: oldPCRValue = [%x]", oldPCRValue)

	hash := hashSum(d)
	log.Printf("File Collector: Measured %s, path = %s, Adding hash=[%x] to PCR #%d", devicePath, filePath, hash, pcr)
	if e := tpm2.PCRExtend(rwc, tpmutil.Handle(pcr), tpm2.AlgSHA256, hash, ""); e != nil {
		return e
	}

	newPCRValue, err := tpm2.ReadPCR(rwc, pcr, tpm2.AlgSHA256)
	if err != nil {
		log.Fatal("Can't read PCR %d from the TPM: %s", pcr, err)
	}

	log.Printf("File Collector: newPCRValue = [%x]", newPCRValue)

	finalPCR := hashSum(append(oldPCRValue, hash...))
	if !bytes.Equal(finalPCR, newPCRValue) {
		log.Fatal("PCRs not equal, got %x, want %x", finalPCR, newPCRValue)
	}

	return nil
}

func (s *FileCollector) Collect(rwc io.ReadWriter) error {

	for _, inputVal := range s.Paths {
		// inputVal is of type sda:/path/to/file
		err := MeasureInputFile(rwc, inputVal)
		if err != nil {
			log.Printf("File Collector: input=%s, err = %v", inputVal, err)
			return err
		}
	}

	return nil
}
