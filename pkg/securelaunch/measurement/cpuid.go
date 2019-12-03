package measurement

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/klauspost/cpuid"
	"github.com/u-root/u-root/pkg/mount"
	slaunch "github.com/u-root/u-root/pkg/securelaunch"
	"github.com/u-root/u-root/pkg/securelaunch/tpm"
)

const (
	defaultCPUIDFile = "cpuid.txt" //only used if user doesn't provide any
)

/* describes the "cpuid" portion of policy file */
type CPUIDCollector struct {
	Type     string `json:"type"`
	Location string `json:"location"`
}

/*
 * NewCPUIDCollector extracts the "cpuid" portion from the policy file.
 * initializes a new CPUIDCollector structure.
 * returns error if unmarshalling of CPUIDCollector fails
 */
func NewCPUIDCollector(config []byte) (Collector, error) {
	slaunch.Debug("New CPUID Collector initialized\n")
	var fc = new(CPUIDCollector)
	err := json.Unmarshal(config, &fc)
	if err != nil {
		return nil, err
	}
	return fc, nil
}

/*
 * getCPUIDInfo used a string builder to store data obtained from intel-go/cpuid package.
 * returns a byte slice of the built string.
 */
func getCPUIDInfo() []byte {
	var w strings.Builder
	fmt.Fprintf(&w, "Name: %s\n", cpuid.CPU.BrandName)
	fmt.Fprintf(&w, "PhysicalCores: %d\n", cpuid.CPU.PhysicalCores)
	fmt.Fprintf(&w, "ThreadsPerCore: %d\n", cpuid.CPU.ThreadsPerCore)
	fmt.Fprintf(&w, "LogicalCores: %d\n", cpuid.CPU.LogicalCores)
	fmt.Fprintf(&w, "Family %v\n", cpuid.CPU.Family)
	fmt.Fprintf(&w, "Model: %v\n", cpuid.CPU.Model)
	fmt.Fprintf(&w, "Features: %s\n", cpuid.CPU.Features)
	fmt.Fprintf(&w, "Cacheline bytes: %d\n", cpuid.CPU.CacheLine)

	return []byte(w.String())
}

/* stores the CPUIDInfo obtained from intel-go/cpuid package into the tpm device */
func measureCPUIDFile(tpmHandle io.ReadWriteCloser) ([]byte, error) {

	d := getCPUIDInfo() // return strings builder
	if e := tpm.ExtendPCRDebug(tpmHandle, pcr, d); e != nil {
		return nil, e
	}

	return d, nil
}

/*
 * stores the cpuid info obtained from intel-go/cpuid package into a file on disk.
 * Input
 * - data - byte slice of the cpuid data obtained from intel-go/cpuid package.
 * - cpuidTargetPath - target file path on disk where cpuid info should be copied.
 * returns error if
 * - mount or unmount of disk, where target is located, fails _OR_
 * - writing to disk fails.
 */
func persist(data []byte, cpuidTargetPath string) error {

	// cpuidTargetPath is of form sda:/boot/cpuid.txt
	filePath, mountPath, r := slaunch.GetMountedFilePath(cpuidTargetPath, true) // true = rw mount option
	if r != nil {
		return fmt.Errorf("EventLog: ERR: input %s could NOT be located, err=%v", cpuidTargetPath, r)
	}

	dst := filePath // /tmp/boot-733276578/cpuid

	target, err := slaunch.WriteToFile(data, dst, defaultCPUIDFile)
	if ret := mount.Unmount(mountPath, true, false); ret != nil {
		log.Printf("Unmount failed. PANIC")
		panic(ret)
	}

	if err != nil {
		log.Printf("persist: err=%s", err)
		return err
	}

	slaunch.Debug("CPUID Collector: Target File%s", target)
	return nil
}

/*
 * satisfies collector interface.
 * calls functions in this file to
 * 1. get the cpuid info from intel-go/cpuid package
 * 2. stores hash of the result in the tpm device.
 * 3. also keeps a copy of the result on disk at location provided in policy file.
 */
func (s *CPUIDCollector) Collect(tpmHandle io.ReadWriteCloser) error {

	d, err := measureCPUIDFile(tpmHandle)
	if err != nil {
		log.Printf("CPUID Collector: err = %v", err)
		return err
	}

	if e := persist(d, s.Location); e != nil {
		log.Printf("CPUID Collector: err= %s", e)
		return e
	}
	return nil
}
