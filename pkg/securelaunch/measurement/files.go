package measurement

import (
	"encoding/json"
	"fmt"
	"github.com/u-root/u-root/pkg/mount"
	slaunch "github.com/u-root/u-root/pkg/securelaunch"
	"github.com/u-root/u-root/pkg/securelaunch/tpm"
	"io"
	"io/ioutil"
	"log"
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

/* measures file input by user in policy file and stores the hash in TPM.
 * inputVal is of format <block device identifier>:<path>
 * E.g sda:/path/to/file _OR UUID:/path/to/file
 * Performs following actions
 * 1. mount device
 * 2. Read file on device into a byte slice.
 * 3. Unmount device
 * 4. Measure byte slice and store it in TPM.
 */
func HashFile(tpmHandle io.ReadWriteCloser, inputVal string) error {
	// inputVal is of type sda:path
	mntFilePath, mountPath, e := slaunch.GetMountedFilePath(inputVal, false) // false means readonly mount
	if e != nil {
		log.Printf("HashFile: GetMountedFilePath err=%v", e)
		return fmt.Errorf("HashFile: GetMountedFilePath err=%v", e)
	}
	slaunch.Debug("ScanKernelCmdLine: Reading file=%s", mntFilePath)

	slaunch.Debug("File Collector: fileP=%s, mountP=%s\n", mntFilePath, mountPath)
	d, err := ioutil.ReadFile(mntFilePath)
	if e := mount.Unmount(mountPath, true, false); e != nil {
		log.Printf("File Collector: Unmount failed. PANIC\n")
		panic(e)
	}

	if err == io.EOF {
		return fmt.Errorf("EOF error")
	}

	if err != nil {
		return fmt.Errorf("Error reading target file: filePath=%s, mountPath=%s, inputVal=%s, err=%v",
			mntFilePath, mountPath, inputVal, err)
	}

	if e := tpm.ExtendPCRDebug(tpmHandle, pcr, d); e != nil {
		return e
	}

	return nil
}

func (s *FileCollector) Collect(tpmHandle io.ReadWriteCloser) error {

	for _, inputVal := range s.Paths {
		// inputVal is of type sda:/path/to/file
		err := HashFile(tpmHandle, inputVal)
		if err != nil {
			log.Printf("File Collector: input=%s, err = %v", inputVal, err)
			return err
		}
	}

	return nil
}
