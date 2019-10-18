// Copyright 2017-2018 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package diskboot

import (
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"

	"github.com/u-root/u-root/pkg/mount"
	"github.com/u-root/u-root/pkg/storage"
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

// FindDevices searches for devices with bootable configs
func FindDevices(devicesGlob string) (devices []*Device) {
	fstypes, err := storage.GetSupportedFilesystems()
	if err != nil {
		return nil
	}

	sysList, err := filepath.Glob(devicesGlob)
	if err != nil {
		return nil
	}
	// The Linux /sys file system is a bit, er, awkward. You can't find
	// the device special in there; just everything else.
	for _, sys := range sysList {
		blk := filepath.Join("/dev", filepath.Base(sys))

		log.Printf("Attempting to mount %s", blk)
		dev, _ := mountDevice(blk, fstypes)
		if dev != nil && len(dev.Configs) > 0 {
			devices = append(devices, dev)
		}
	}

	return devices
}

// FindDevice attempts to construct a boot device at the given path
func FindDevice(devPath string) (*Device, error) {
	fstypes, err := storage.GetSupportedFilesystems()
	if err != nil {
		return nil, nil
	}

	return mountDevice(devPath, fstypes)
}

func mountDevice(devPath string, fstypes []string) (*Device, error) {
	log.Printf("mounting dev=%s\n", devPath)

	mountPath, err := ioutil.TempDir("/tmp", "boot-")
	if err != nil {
		return nil, fmt.Errorf("Failed to create tmp mount directory: %v", err)
	}
	log.Printf("mountPath=%v\n", mountPath)
	for _, fstype := range fstypes {
		log.Printf("fstype=%v\n", fstype)
		if err := mount.Mount(devPath, mountPath, fstype, "", unix.MS_RDONLY); err != nil {
			log.Printf("mount Failed..continuing to next fstype\n")
			continue
		}

		configs := FindConfigs(mountPath)
		if len(configs) == 0 {
			log.Printf("FindConfigs returned 0 configs. BUT I AM OKAY WITH IT\n")
			// continue
		}

		return &Device{devPath, mountPath, fstype, configs}, nil
	}
	return nil, fmt.Errorf("Failed to find a valid boot device with configs")
}
