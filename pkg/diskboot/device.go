// Copyright 2017-2018 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package diskboot

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/u-root/u-root/pkg/mount"
	"golang.org/x/sys/unix"
)

// Device contains the path to a block filesystem along with its type
type Device struct {
	DevPath   string
	MountPath string
	Fstype    string
	Configs   []*Config
}

// fstypes returns all block file system supported by the linuxboot kernel
func fstypes() (fstypes []string, err error) {
	var bytes []byte
	if bytes, err = ioutil.ReadFile("/proc/filesystems"); err != nil {
		return nil, fmt.Errorf("Failed to read /proc/filesystems: %v", err)
	}

	fmt.Printf("read bytes from /proc/filesystems= %v\n", bytes)
	for _, line := range strings.Split(string(bytes), "\n") {
		fmt.Printf("line= %v, fields=%v\n", line, len(strings.Fields(line)))
		if fields := strings.Fields(line); len(fields) == 1 {
			fstypes = append(fstypes, fields[0])
		}
	}
	return fstypes, nil
}

// FindDevices searches for devices with bootable configs
func FindDevices(devicesGlob string) (devices []*Device) {
	fstypes, err := fstypes()
	if err != nil {
		fmt.Printf("I got err in calling fstypes %v\n", err)
		return nil
	}

	// What if no 0 fstypes are found ?
	fmt.Printf("I got %v fstypes= %v\n", len(fstypes), fstypes)

	sysList, err := filepath.Glob(devicesGlob)
	if err != nil {
		fmt.Printf("I got error in sysList\n")
		return nil
	}

	// The Linux /sys file system is a bit, er, awkward. You can't find
	// the device special in there; just everything else.
	for _, sys := range sysList {
		fmt.Println()
		fmt.Printf("sys=%v\n", sys)
		blk := filepath.Join("/dev", filepath.Base(sys))

		fmt.Printf("blk=%v\n", blk)
		dev, _ := mountDevice(blk, fstypes)
		if dev != nil && len(dev.Configs) > 0 {
			devices = append(devices, dev)
		}
	}

	fmt.Printf("returning\n")
	return devices
}

// FindDevice attempts to construct a boot device at the given path
func FindDevice(devPath string) (*Device, error) {
	fstypes, err := fstypes()
	if err != nil {
		return nil, nil
	}

	return mountDevice(devPath, fstypes)
}

func mountDevice(devPath string, fstypes []string) (*Device, error) {
	mountPath, err := ioutil.TempDir("/tmp", "boot-")
	if err != nil {
		fmt.Printf("failed to create tmp mount directory\n")
		return nil, fmt.Errorf("Failed to create tmp mount directory: %v", err)
	}
	for _, fstype := range fstypes {
		if err := mount.Mount(devPath, mountPath, fstype, "", unix.MS_RDONLY); err != nil {
			fmt.Printf("failed to mount %v, %v, %v\n", devPath, mountPath, fstype)
			continue
		}

		fmt.Printf("Succeeded in mount %v, %v, %v\n", devPath, mountPath, fstype)
		fmt.Printf("Caling FindConfigs\n")
		configs := FindConfigs(mountPath)
		if len(configs) == 0 {
			fmt.Printf("no configs found\n")
			continue
		}

		return &Device{devPath, mountPath, fstype, configs}, nil
	}
	// fmt.Printf("len(fstypes)=%v\n", len(fstypes))
	return nil, fmt.Errorf("Failed to find a valid boot device with configs")
}
