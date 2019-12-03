// Copyright 2019 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package securelaunch

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/u-root/u-root/pkg/cmdline"
	"github.com/u-root/u-root/pkg/diskboot"
	"github.com/u-root/u-root/pkg/find"
	"github.com/u-root/u-root/pkg/mount"
	"github.com/u-root/u-root/pkg/storage"
)

/* used to store all block devices returned from a call to storage.GetBlockStats */
var storageBlkDevices []storage.BlockDev

/*
 * if kernel cmd line has uroot.uinitargs=-d, debug fn is enabled.
 * kernel cmdline is checked in sluinit.
 */
var Debug = func(string, ...interface{}) {}

/*
 * modified ver of https://stackoverflow.com/questions/10510691/how-to-check-whether-a-file-or-directory-exists
 * exists returns whether the given file or directory exists
 * also returns if the path is a directory
 */
func exists(path string) (bool, bool, error) {
	fileInfo, err := os.Stat(path)
	if err == nil {
		return true, fileInfo.IsDir(), nil
	}
	if os.IsNotExist(err) {
		return false, false, nil
	}
	// file or dir def exists..
	return true, fileInfo.IsDir(), err
}

/*
 * data is a []byte slice
 * dst is an already mounted file path.
 * dst is an Absolute file path.
 * defFileName is default dst file name, only used if user doesn't provide one.
 * returns
 * 1. target file path where file was written
 * 2. error
 */
func WriteToFile(data []byte, dst, defFileName string) (string, error) {

	// make sure dst is an absolute file path
	if !filepath.IsAbs(dst) {
		return "", fmt.Errorf("Error: dst =%s Not an absolute path ", dst)
	}

	// target is the full absolute path where []byte will be written to
	target := dst

	/* dst = /foo/bar/eventlog, check if /foo/bar/eventlog is a dir or file that exists..
	     * if it exists as a file, this if loop is untouched.
		 * if it exists as a dir, append defFileName to it
	     * NOTE both dst = /foo/bar/ and dst=/foo/bar are considered dirs i.e. trailing "/" has no meaning..
	*/
	dstFound, is_dir, err := exists(dst)
	if is_dir {
		if !dstFound {
			return "", fmt.Errorf("destination dir doesn't")
		} else {
			// User provided a dir, Use defFileName as target
			Debug("No file name provided. Adding it now. old target=%s", target)
			target = filepath.Join(dst, defFileName)
			Debug("New target=%s", target)
		}
	}

	Debug("target=%s", target)
	err = ioutil.WriteFile(target, data, 0644)
	if err != nil {
		return "", fmt.Errorf("Could't write file to %s, err=%v", target, err)
	}
	Debug("WriteToFile exit w success data written to target=%s", target)
	return target, nil
}

/*
 * NOTE: Caller's responsbility to unmount this..use return var mountPath to unmount in caller.
 *
 * obtains absolute file path based on input entered by user in policy file.
 * inputVal is of format <block device identifier>:<path>
 * inputVal e.g sda:/boot/foo.go , 4qccd342-12zr-4e99-9ze7-1234cb1234c4:/bar/zyx.go
 * rw_option = true --> mount is of type read-write, else read-only.
 *
 * This function
 * parses user input.
 * mounts device
 * Get absolute file path on mounted device.
 * does NOT Unmount device, use returned mountPath to unmount...
 * returns filePath, mountPath, error in that order
 */
func GetMountedFilePath(inputVal string, rw_option bool) (string, string, error) {
	s := strings.Split(inputVal, ":")
	if len(s) != 2 {
		return "", "", fmt.Errorf("%s: Usage: <block device identifier>:<path>", inputVal)
	}

	// s[0] can be sda or UUID. if UUID, then we need to find its name
	var deviceId string = s[0]
	if !strings.HasPrefix(deviceId, "sd") {
		if e := getBlkInfo(); e != nil {
			return "", "", fmt.Errorf("GetMountedFilePath: getBlkInfo err=%s", e)
		}
		devices := storage.PartitionsByFsUUID(storageBlkDevices, s[0]) // []BlockDev
		for _, device := range devices {
			Debug("device =%s with fsuuid=%s", device.Name, s[0])
			deviceId = device.Name
		}
	}

	devicePath := filepath.Join("/dev", deviceId) // assumes deviceId is sda, devicePath=/dev/sda
	Debug("Attempting to mount %s", devicePath)
	var dev *diskboot.Device
	var err error
	if rw_option {
		dev, err = diskboot.FindDeviceRW(devicePath) // FindDevice fn mounts , w rw option, devicePath=/dev/sda.
	} else {
		dev, err = diskboot.FindDevice(devicePath) // FindDevice fn mounts devicePath=/dev/sda.
	}
	if err != nil {
		return "", "", fmt.Errorf("Mount %v failed, err=%v", devicePath, err)
	}

	Debug("Mounted %s", devicePath)
	fPath := filepath.Join(dev.MountPath, s[1]) // mountPath=/tmp/path/to/target/file if /dev/sda mounted on /tmp
	return fPath, dev.MountPath, nil
}

/*
 * ScanKernelCmdLine() ([]byte, error)
 * format sl_policy=<block device identifier>:<path>
 * e.g sda:/boot/securelaunch.policy
 * e.g 4qccd342-12zr-4e99-9ze7-1234cb1234c4:/foo/securelaunch.policy
 */
func ScanKernelCmdLine() ([]byte, error) {

	Debug("ScanKernelCmdLine: scanning kernel cmd line for *sl_policy* flag")
	val, ok := cmdline.Flag("sl_policy")
	if !ok {
		log.Printf("ScanKernelCmdLine: sl_policy cmdline flag is not set")
		return nil, errors.New("Flag Not Set")
	}

	// val is of type sda:path
	mntFilePath, mountPath, e := GetMountedFilePath(val, false) // false means readonly mount
	if e != nil {
		log.Printf("ScanKernelCmdLine: GetMountedFilePath err=%v", e)
		return nil, fmt.Errorf("ScanKernelCmdLine: GetMountedFilePath err=%v", e)
	}
	Debug("ScanKernelCmdLine: Reading file=%s", mntFilePath)

	d, err := ioutil.ReadFile(mntFilePath)
	if e := mount.Unmount(mountPath, true, false); e != nil {
		log.Printf("Unmount failed. PANIC")
		panic(e)
	}

	if err != nil {
		// - TODO: should we check for end of file ?
		log.Printf("Error reading policy file:mountPath=%s, passed=%s\n", mntFilePath, val)
		return nil, fmt.Errorf("Error reading policy file:mountPath=%s, passed=%s\n", mntFilePath, val)
	}
	return d, nil
}

/*
 *  scanBlockDevice(mountPath string) ([]byte, bool)
 *	recursively scans an already mounted block device inside directories
 *	"/", "/efi" and "/boot" for policy file
 *
 *	e.g: if you mount /dev/sda1 on /tmp/sda1,
 *	then mountPath would be /tmp/sda1
 *	and searchPath would be /tmp/sda1/, /tmp/sda1/efi, and /tmp/sda1/boot
 *		respectively for each iteration of loop over SearchRoots slice.
 */
func ScanBlockDevice(mountPath string) ([]byte, bool) {

	log.Printf("scanBlockDevice\n")
	// scan for securelaunch.policy under /, /efi, or /boot
	var SearchRoots = []string{"/", "/efi", "/boot"}
	for _, c := range SearchRoots {

		searchPath := filepath.Join(mountPath, c)
		files := find.Find(context.Background(),
			find.WithRoot(searchPath),
			find.WithFilenameMatch("securelaunch.policy"),
		)

		for o := range files {
			// Read in policy file:
			d, err := ioutil.ReadFile(o.Name)
			if err != nil {
				log.Printf("Error reading policy file %s, continuing", o.Name)
				continue
			}
			log.Printf("policy file found on mountPath=%s, directory =%s\n", mountPath, c)
			return d, true // return when first policy file found
		}
		// Policy File not found. Moving on to next search root...
	}
	return nil, false
}

/*
 * getBlkInfo calls storage package to get information on all block devices.
 * The information is stored in a global variable "storageBlkDevices"
 * If the global variable is already non-zero, we skip the call to storage package.
 * In debug mode, it also prints names and UUIDs for all devices.
 */
func getBlkInfo() error {
	if len(storageBlkDevices) == 0 {
		var err error
		storageBlkDevices, err = storage.GetBlockStats()
		if err != nil {
			log.Printf("getBlkInfo: storage.GetBlockStats err=%v. Exiting", err)
			return err
		}
	}

	for k, d := range storageBlkDevices {
		Debug("block device #%d, Name=%s, FsUUID=%s", k, d.Name, d.FsUUID)
	}
	return nil
}
