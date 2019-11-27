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

// FindDevicesRW searches for devices with bootable configs and RW mount.
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

		dev, _ := mountDeviceRW(blk, fstypes)
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

func mountDeviceRW(devPath string, fstypes []string) (*Device, error) {
	mountPath, err := ioutil.TempDir("/tmp", "boot-")
	if err != nil {
		return nil, fmt.Errorf("failed to create tmp mount directory: %v", err)
	}

	// unix.MS_RDONLY = 1
	// tested mount command by giving it -o rw,user and flags=0 is being passed to unix.Mount,
	// so directly passing 0.
	// cmds/core/mount/mount.go can be used to test what gets passed to unix.Mount
	// mount -t ext4 /dev/sda1 /tmp/simran -o rw,user
	for _, fstype := range fstypes {
		if err := mount.Mount(devPath, mountPath, fstype, "", 0); err != nil {
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

func mountDevice(devPath string, fstypes []string) (*Device, error) {
	mountPath, err := ioutil.TempDir("/tmp", "boot-")
	if err != nil {
		return nil, fmt.Errorf("failed to create tmp mount directory: %v", err)
	}
	for _, fstype := range fstypes {
		if err := mount.Mount(devPath, mountPath, fstype, "", unix.MS_RDONLY); err != nil {
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
