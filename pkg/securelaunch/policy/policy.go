package policy

import (
	"encoding/json"
	"errors"
	"github.com/u-root/u-root/pkg/diskboot"
	"github.com/u-root/u-root/pkg/mount"
	slaunch "github.com/u-root/u-root/pkg/securelaunch"
	"github.com/u-root/u-root/pkg/securelaunch/measurement"
	"log"
)

/*
 * policy structure passed around various sub-packages (
 * measurement, eventlog and launcher) in securelaunch.
 * Used to store parsed policy file. The policy file
 * is a JSON file.
 */
type policy struct {
	DefaultAction string
	Collectors    []measurement.Collector
}

/*
 * Locate() ([]byte, error)
 * Check if kernel param sl_policy is set,
 *	- parse the string
 * Iterate through each local block device,
 *	- mount the block device
 *	- scan for securelaunch.policy under /, /efi, or /boot
 * Read in policy file
 */
func Locate() ([]byte, error) {

	d, _ := slaunch.ScanKernelCmdLine()
	if d != nil {
		return d, nil
	}

	slaunch.Debug("Searching and mounting block devices with bootable configs\n")
	blkDevices := diskboot.FindDevices("/sys/class/block/*") // FindDevices find and *mounts* the devices.
	if len(blkDevices) == 0 {
		return nil, errors.New("No block devices found")
	}

	for _, device := range blkDevices {
		devicePath, mountPath := device.DevPath, device.MountPath
		slaunch.Debug("scanning for policy file under devicePath=%s, mountPath=%s\n", devicePath, mountPath)
		raw, found := slaunch.ScanBlockDevice(mountPath)
		if e := mount.Unmount(mountPath, true, false); e != nil {
			log.Printf("Unmount failed. PANIC\n")
			panic(e)
		}

		if !found {
			log.Printf("no policy file found under this device\n")
			continue
		}

		slaunch.Debug("policy file found at devicePath=%s\n", devicePath)
		return raw, nil
	}

	return nil, errors.New("policy file not found anywhere.")
}

/*
 * Parse(pf []byte) (*policy, error)
 * Sets up policy structure by unmarshalling policy file.
 * returns pointer to policy structure.
 */
func Parse(pf []byte) (*policy, error) {
	p := &policy{}
	var parse struct {
		DefaultAction string            `json:"default_action"`
		Collectors    []json.RawMessage `json:"collectors"`
		Attestor      json.RawMessage   `json:"attestor"`
		Launcher      json.RawMessage   `json:"launcher"`
		EventLog      json.RawMessage   `json:"eventlog"`
	}

	if err := json.Unmarshal(pf, &parse); err != nil {
		log.Printf("parse SL Policy: Unmarshall error for entire policy file!! err=%v\n", err)
		return nil, err
	}

	p.DefaultAction = parse.DefaultAction

	for _, c := range parse.Collectors {
		collector, err := measurement.GetCollector(c)
		if err != nil {
			log.Printf("GetCollector err:c=%s, collector=%v\n", c, collector)
			return nil, err
		}
		p.Collectors = append(p.Collectors, collector)
	}

	return p, nil
}
