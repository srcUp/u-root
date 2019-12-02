package measurement

import (
	"encoding/json"
	"fmt"
	"github.com/intel-go/cpuid"
	"github.com/u-root/u-root/pkg/mount"
	slaunch "github.com/u-root/u-root/pkg/securelaunch"
	"github.com/u-root/u-root/pkg/securelaunch/tpm"
	"io"
	"log"
	"strings"
)

const (
	defaultCPUIDFile = "cpuid.txt" //only used if user doesn't provide any
)

type CPUIDCollector struct {
	Type     string `json:"type"`
	Location string `json:"location"`
}

func NewCPUIDCollector(config []byte) (Collector, error) {
	log.Printf("New CPUID Collector initialized\n")
	var fc = new(CPUIDCollector)
	err := json.Unmarshal(config, &fc)
	if err != nil {
		return nil, err
	}
	return fc, nil
}

func getCPUIDInfo() []byte {
	var w strings.Builder
	fmt.Fprintf(&w, "VendorString:           %s\n", cpuid.VendorIdentificatorString)
	fmt.Fprintf(&w, "ProcessorBrandString:   %s\n", cpuid.ProcessorBrandString)
	fmt.Fprintf(&w, "SteppingId:     %d\n", cpuid.SteppingId)
	fmt.Fprintf(&w, "ProcessorType:  %d\n", cpuid.ProcessorType)
	fmt.Fprintf(&w, "DisplayFamily:  %d\n", cpuid.DisplayFamily)
	fmt.Fprintf(&w, "DisplayModel:   %d\n", cpuid.DisplayModel)
	fmt.Fprintf(&w, "CacheLineSize:  %d\n", cpuid.CacheLineSize)
	fmt.Fprintf(&w, "MaxLogocalCPUId:%d\n", cpuid.MaxLogocalCPUId)
	fmt.Fprintf(&w, "InitialAPICId:  %d\n", cpuid.InitialAPICId)
	fmt.Fprintf(&w, "Smallest monitor-line size in bytes:  %d\n", cpuid.MonLineSizeMin)
	fmt.Fprintf(&w, "Largest monitor-line size in bytes:   %d\n", cpuid.MonLineSizeMax)
	fmt.Fprintf(&w, "Monitor Interrupt break-event is supported:  %v\n", cpuid.MonitorIBE)
	fmt.Fprintf(&w, "MONITOR/MWAIT extensions are supported:      %v\n", cpuid.MonitorEMX)
	fmt.Fprintf(&w, "AVX state:     %v\n", cpuid.EnabledAVX)
	fmt.Fprintf(&w, "AVX-512 state: %v\n", cpuid.EnabledAVX512)
	fmt.Fprintf(&w, "Interrupt thresholds in digital thermal sensor: %v\n", cpuid.ThermalSensorInterruptThresholds)

	fmt.Fprintf(&w, "Features: ")
	for i := uint64(0); i < 64; i++ {
		if cpuid.HasFeature(1 << i) {
			fmt.Fprintf(&w, "%s ", cpuid.FeatureNames[1<<i])
		}
	}
	fmt.Fprintf(&w, "\n")

	fmt.Fprintf(&w, "ExtendedFeatures: ")
	for i := uint64(0); i < 64; i++ {
		if cpuid.HasExtendedFeature(1 << i) {
			fmt.Fprintf(&w, "%s ", cpuid.ExtendedFeatureNames[1<<i])
		}
	}
	fmt.Fprintf(&w, "\n")

	fmt.Fprintf(&w, "ExtraFeatures: ")
	for i := uint64(0); i < 64; i++ {
		if cpuid.HasExtraFeature(1 << i) {
			fmt.Fprintf(&w, "%s ", cpuid.ExtraFeatureNames[1<<i])
		}
	}
	fmt.Fprintf(&w, "\n")

	fmt.Fprintf(&w, "ThermalAndPowerFeatures: ")
	for i := uint32(0); i < 64; i++ {
		if cpuid.HasThermalAndPowerFeature(1 << i) {
			if name, found := cpuid.ThermalAndPowerFeatureNames[1<<i]; found {
				fmt.Fprintf(&w, "%s ", name)
			}
		}
	}
	fmt.Fprintf(&w, "\n")

	for _, cacheDescription := range cpuid.CacheDescriptors {
		fmt.Fprintf(&w, "CacheDescriptor: %v\n", cacheDescription)
	}
	return []byte(w.String())
}

func measureCPUIDFile(tpmHandle io.ReadWriteCloser) ([]byte, error) {

	d := getCPUIDInfo() // return strings builder
	if e := tpm.ExtendPCRDebug(tpmHandle, pcr, d); e != nil {
		return nil, e
	}

	return d, nil
}

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
