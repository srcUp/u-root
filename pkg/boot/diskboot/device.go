// Copyright 2017-2018 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package diskboot

import (
	"fmt"
	"io/ioutil"
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

/*
 * FindDevicesRW is identical to FindDevices, except the "RW"one
 * calls mountDeviceRW (i.e. pass read write option to mount.Mount)
 * In comparison, FindDevices calls mountDevice which mounts
 * the device as read only.
 */
func FindDevicesRW(devicesGlob string) (devices []*Device) {
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

		dev, _ := mountDevice(blk, fstypes, 0)
		if dev != nil && len(dev.Configs) > 0 {
			devices = append(devices, dev)
		}
	}

	return devices
}

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

		dev, _ := mountDevice(blk, fstypes, unix.MS_RDONLY)
		if dev != nil && len(dev.Configs) > 0 {
			devices = append(devices, dev)
		}
	}

	return devices
}

/*
 * FindDeviceRW is identical to FindDevice, except the "RW" one
 * calls mountDeviceRW (i.e. pass read write option to mount.Mount)
 * In comparison, FindDevice calls mountDevice which mounts the
 * device as read only.
 */
func FindDeviceRW(devPath string) (*Device, error) {
	fstypes, err := storage.GetSupportedFilesystems()
	if err != nil {
		return nil, nil
	}

	return mountDevice(devPath, fstypes, 0)
}

// FindDevice attempts to construct a boot device at the given path
func FindDevice(devPath string) (*Device, error) {
	fstypes, err := storage.GetSupportedFilesystems()
	if err != nil {
		return nil, nil
	}

	return mountDevice(devPath, fstypes, unix.MS_RDONLY)
}

func mountDevice(devPath string, fstypes []string, mountFlags uintptr) (*Device, error) {
	mountPath, err := ioutil.TempDir("/tmp", "boot-")
	if err != nil {
		return nil, fmt.Errorf("failed to create tmp mount directory: %v", err)
	}
	for _, fstype := range fstypes {
		if err := mount.Mount(devPath, mountPath, fstype, "", mountFlags); err != nil {
			continue
		}

		configs := FindConfigs(mountPath)
		if len(configs) == 0 {
			continue
		}

		return &Device{devPath, mountPath, fstype, configs}, nil
	}
	return nil, fmt.Errorf("failed to find a valid boot device with configs")
}
