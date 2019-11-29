// Copyright 2019 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package securelaunch

import (
	"bytes"
	"errors"
	"fmt"
	// "github.com/u-root/iscsinl"
	"github.com/u-root/u-root/pkg/cmdline"
	"github.com/u-root/u-root/pkg/diskboot"
	"github.com/u-root/u-root/pkg/find"
	"github.com/u-root/u-root/pkg/mount"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	// "time"
)

// Caller's responsbility to unmount this..use mountPath returned to unmount.
// obtains file path based on input entered by user in policy file.
// inputVal is of format <block device identifier>:<path>
// rw_option = true --> mount is read write type
// Example sda:/path/to/file
/* - mount device
 * - Get file Path on mounted device.
 * - does NOT Unmount device, use returned devicePath to unmount...
 * returns filePath, mountPath, error in that order
 */
func GetMountedFilePath(inputVal string, rw_option bool) (string, string, error) {
	s := strings.Split(inputVal, ":")
	if len(s) != 2 {
		return "", "", fmt.Errorf("%s: Usage: <block device identifier>:<path>", inputVal)
	}

	devicePath := filepath.Join("/dev", s[0]) // assumes deviceId is sda, devicePath=/dev/sda
	var dev *diskboot.Device
	var err error
	if rw_option {
		dev, err = diskboot.FindDeviceRW(devicePath) // FindDevice fn mounts , w rw option, devicePath=/dev/sda.
	} else {
		dev, err = diskboot.FindDevice(devicePath) // FindDevice fn mounts devicePath=/dev/sda.
	}
	if err != nil {
		log.Printf("Mount %v failed, err=%v", devicePath, err)
		return "", "", err
	}

	filePath := dev.MountPath + s[1] // mountPath=/tmp/path/to/target/file if /dev/sda mounted on /tmp
	return filePath, dev.MountPath, nil
}

// https://stackoverflow.com/questions/10510691/how-to-check-whether-a-file-or-directory-exists
// exists returns whether the given file or directory exists
// also returns if the path is a directory
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

// src and dst are already mounted file paths.
// src := Absolute source file path
// dst := Absolute destination file path
// defFileName := default file name, only used if user doesn't provide one.
// returns the target file path where file was written..
func writeSrcFiletoDst(src, dst, defFileName string) (string, error) {
	srcFileFound, _, err := exists(src)
	if !srcFileFound {
		return "", fmt.Errorf("File to be written doesnt exist:")
	}

	d, err := ioutil.ReadFile(src)
	if err != nil {
		return "", fmt.Errorf("Error reading src file %s, err=%v", src, err)
	}

	// make sure dst is an absolute file path
	if !filepath.IsAbs(dst) {
		return "", fmt.Errorf("Error: dst =%s Not an absolute path ", dst)
	}

	// target is the full absolute path where thesrc will be written to
	target := dst

	/*dst = /foo/bar/eventlog, check if /foo/bar/eventlog is a dir or file that exists..
		 *if it exists as a file, this if loop is untouched.
	     *if it exists as a dir, append defaul file name to it
		 *NOT both dst = /foo/bar/ and dst=/foo/bar are considered dirs i.e. trailing "/" has no meaning..
	*/
	dstFound, is_dir, err := exists(dst)
	if is_dir {
		if !dstFound {
			return "", fmt.Errorf("destination dir doesn't")
		} else {
			log.Printf("No file name provided. Adding it here.old target=%s", target)
			// no file name was provided. Use default.
			// check if defFileName doesn't have a trailing "/"
			if strings.HasPrefix(defFileName, "/") || strings.HasSuffix(dst, "/") {
				target = dst + defFileName
			} else {
				log.Printf("I need to construct filepath. ERROR: Pass dir w suffix=/ or fname w prefix=/")
				return "", fmt.Errorf("Pass dir w suffix=/ or fname w prefix=/")
			}
			log.Printf("New target=%s", target)
		}
	}

	log.Printf("target=%s", target)
	err = ioutil.WriteFile(target, d, 0644)
	if err != nil {
		return "", fmt.Errorf("Could't write file to %s, err=%v", target, err)
	}
	log.Printf("writeSrcFiletoDst exiting, success src=%s written to targetFilePath=%s", src, target)
	return target, nil
}

/* 	scanKernelCmdLine() ([]byte, error)
	format sl_policy=<block device identifier>:<path>
	e.g 1 sda:/boot/securelaunch.policy
 	TODO: e.g 2 77d8da74-a690-481a-86d5-9beab5a8e842:/boot/securelaunch.policy
	TODO: <block device identifier>:<mount options>,<path> */
func scanKernelCmdLine() ([]byte, error) {

	log.Printf("scanKernelCmdLine: scanning kernel cmd line for *sl_policy* flag")
	val, ok := cmdline.Flag("sl_policy")
	if !ok {
		log.Printf("scanKernelCmdLine: sl_policy cmdline flag is not set")
		return nil, errors.New("Flag Not Set")
	}

	// val is of type sda:path
	mntFilePath, mountPath, e := GetMountedFilePath(val, false) // false means readonly mount
	if e != nil {
		log.Printf("scanKernelCmdLine: GetMountedFilePath err=%v", e)
	}
	log.Printf("scanKernelCmdLine: Reading file=%s", mntFilePath)

	d, err := ioutil.ReadFile(mntFilePath)
	if e := mount.Unmount(mountPath, true, false); e != nil {
		log.Printf("Unmount failed. PANIC")
		panic(e)
	}

	if err != nil {
		// - TODO: should we check for end of file ?
		return nil, fmt.Errorf("Error reading policy file:mountPath=%s, passed=%s\n", mntFilePath, val)
	}
	return d, nil
}

/*  scanBlockDevice(mountPath string) ([]byte, bool)
	recursively scans an already mounted block device inside directories
	"/", "/efi" and "/boot" for policy file

	e.g: if you mount /dev/sda1 on /tmp/sda1,
 	then mountPath would be /tmp/sda1
 	and searchPath would be /tmp/sda1/, /tmp/sda1/efi, and /tmp/sda1/boot
		respectively for each iteration of loop over SearchRoots slice. */
func scanBlockDevice(mountPath string) ([]byte, bool) {

	log.Printf("scanBlockDevice\n")
	// scan for securelaunch.policy under /, /efi, or /boot
	var SearchRoots = []string{"/", "/efi", "/boot"}
	for _, c := range SearchRoots {

		searchPath := mountPath + c
		f, _ := find.New(func(f *find.Finder) error {
			f.Root = searchPath
			f.Pattern = "securelaunch.policy"
			return nil
		})

		// spawn a goroutine
		go f.Find()

		// Read in policy file:
		for o := range f.Names {
			if o.Err != nil {
				log.Printf("%v: got %v, want nil", o.Name, o.Err)
			}
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

// To run the daemon in debug mode please pass the parameter  '-d <debug level>'
// DEBUG         4 - Print all messages
// INFO          3 - Print messages needed to follow the uIP code (default)
// WARN          2 - Print warning messages
// ERROR         1 - Only print critical errors
// netroot=iscsi:@10.196.210.62::3260::iqn.1986-03.com.sun:ovs112-boot rd.iscsi.initiator=iqn.1988-12.com.oracle:ovs112
// netroot=iscsi:@10.196.210.64::3260::iqn.1986-03.com.sun:ovs112-boot
//NOTE:  if you have two netroot params in kernel command line , second one will be used.
func ScanIscsiDrives() error {

	log.Printf("Scanning kernel cmd line for *netroot* flag")
	val, ok := cmdline.Flag("netroot")
	if !ok {
		log.Printf("sl_policy netroot flag is not set")
		return errors.New("Flag Not Set")
	}

	// val = iscsi:@10.196.210.62::3260::iqn.1986-03.com.sun:ovs112-boot
	log.Printf("netroot flag is set with val=%s", val)
	s := strings.Split(val, "::")
	if len(s) != 3 {
		return fmt.Errorf("%v: incorrect format ::,  Usage: netroot=iscsi:@10.X.Y.Z::1224::iqn.foo:hostname-bar, [Expecting len(%s) = 3] ", val, s)
	}

	// s[0] = iscsi:@10.196.210.62 or iscsi:@10.196.210.62,2
	// s[1] = 3260
	// s[2] = iqn.1986-03.com.sun:ovs112-boot
	port := s[1]
	target := s[2]
	tmp := strings.Split(s[0], ":@")
	if len(tmp) > 3 || len(tmp) < 2 {
		return fmt.Errorf("%v: incorrect format :@, Usage: netroot=iscsi:@10.X.Y.Z::1224::iqn.foo:hostname-bar, [ Expecting 2 <= len(%s) <= 3", val, tmp)
	}

	if tmp[0] != "iscsi" {
		return fmt.Errorf("%v: incorrect format iscsi:, Usage: netroot=iscsi:@10.X.Y.Z::1224::iqn.foo:hostname-bar, [ %s != 'iscsi'] ", val, tmp[0])
	}

	var ip, group string
	tmp2 := strings.Split(tmp[1], ",")
	if len(tmp2) == 1 {
		ip = tmp[1]
	} else if len(tmp2) == 2 {
		ip = tmp[1]
		group = tmp[2]
	}

	log.Printf("Scanning kernel cmd line for *rd.iscsi.initiator* flag")
	initiatorName, ok := cmdline.Flag("rd.iscsi.initiator")
	if !ok {
		log.Printf("sl_policy rd.iscsi.initiator flag is not set")
		return errors.New("Flag Not Set")
	}

	var portalGroup string
	if group == "" {
		portalGroup = "1"
	}

	cmdArgs := []string{"-d=ERROR", "-a", ip, "-g", portalGroup, "-p", port, "-t", target, "-i", initiatorName}
	Cmd_exec("iscsistart", cmdArgs)
	return nil
}

func Cmd_exec(cmdname string, cmdArgs []string) {
	cmd := exec.Command(cmdname, cmdArgs...)
	var out bytes.Buffer
	cmd.Stdout = &out
	log.Printf("Executing %v", cmd.Args)
	if err := cmd.Run(); err != nil {
		fmt.Println(err)
	} else {
		log.Printf("Output: \n%v", cmd.Stdout)
	}
}
