// Copyright 2019 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/go-tpm/tpm2"
	// "github.com/u-root/iscsinl"
	"github.com/u-root/u-root/pkg/cmdline"
	"github.com/u-root/u-root/pkg/diskboot"
	"github.com/u-root/u-root/pkg/find"
	"github.com/u-root/u-root/pkg/measurement"
	"github.com/u-root/u-root/pkg/mount"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	// "time"
)

type launcher struct {
	Type string `json:"type"`
	// Cmd    string   `json:"cmd"`
	Params map[string]string `json:"params"`
}

type eventlog struct {
	Type     string `json:"type"`
	Location string `json:"location"`
}

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

// /tmp/parsedEvtLog.txt --> written to --> eventlog_path
func (e *eventlog) Persist() error {

	if e.Type != "file" {
		return fmt.Errorf("EventLog: Unsupported eventlog type. Exiting.")
	}

	log.Printf("Identified EventLog Type = file")

	// e.Location is of the form sda:path/to/file.txt
	eventlog_path := e.Location
	if eventlog_path == "" {
		return fmt.Errorf("EventLog: Empty eventlog path. Exiting.")
	}

	filePath, mountPath, r := GetMountedFilePath(eventlog_path, true) // true = rw mount option
	if r != nil {
		return fmt.Errorf("EventLog: ERR: eventlog input %s couldnt be located, err=%v", eventlog_path, r)
	}

	dst := filePath // /tmp/boot-733276578/evtlog
	src1 := "/tmp/parsedEvtLog.txt"
	src2 := "/tmp/cpuid.txt"
	default1_fname := "eventlog.txt"
	default2_fname := "cpuid.txt"

	target1, err1 := writeSrcFiletoDst(src1, dst, default1_fname)
	target2, err2 := writeSrcFiletoDst(src2, filepath.Dir(dst)+"/", default2_fname)
	if ret := mount.Unmount(mountPath, true, false); ret != nil {
		log.Printf("Unmount failed. PANIC")
		panic(ret)
	}

	if err1 == nil && err2 == nil {
		log.Printf("writeSrcFiletoDst successful: src1=%s, src2=%s, dst1=%s, dst2=%s", src1, src2, target1, target2)
		return nil
	}

	if err1 != nil {
		// return fmt.Errorf("writeSrcFiletoDst: src1 returned errors err1=%v, src1=%s, dst1=%s, exiting", err1, src1, dst)
		log.Printf("writeSrcFiletoDst: src1 returned errors err1=%v, src1=%s, dst1=%s, exiting", err1, src1, dst)
	}

	if err2 != nil {
		// return fmt.Errorf("writeSrcFiletoDst src2 returned errors err2=%v, src2=%s, dst2=%s", err2, src2, filepath.Dir(dst)+default2_fname)
		log.Printf("writeSrcFiletoDst src2 returned errors err2=%v, src2=%s, dst2=%s", err2, src2, filepath.Dir(dst)+default2_fname)
	}

	return nil
}

func (l *launcher) Boot(t io.ReadWriter) {

	if l.Type != "kexec" {
		log.Printf("Launcher: Unsupported launcher type. Exiting.")
		return
	}
	log.Printf("Identified Launcher Type = Kexec")
	// TODO: if kernel and initrd are on different devices.
	kernel := l.Params["kernel"]
	initrd := l.Params["initrd"]
	cmdline := l.Params["cmdline"]

	log.Printf("********Step 6: Measuring kernel, initrd ********")
	if e := measurement.MeasureInputFile(t, kernel); e != nil {
		log.Printf("Launcher: ERR: measure kernel input=%s, err=%v", kernel, e)
		return
	}

	if e := measurement.MeasureInputFile(t, initrd); e != nil {
		log.Printf("Launcher: ERR: measure initrd input=%s, err=%v", initrd, e)
		return
	}

	// I don't unmount the mount path ?
	k, _, e := GetMountedFilePath(kernel, false) // false=read only mount option
	if e != nil {
		log.Printf("Launcher: ERR: kernel input %s couldnt be located, err=%v", kernel, e)
		return
	}

	// I don't unmount the mount path ?
	i, _, e := GetMountedFilePath(initrd, false) // false=read only mount option
	if e != nil {
		log.Printf("Launcher: ERR: initrd input %s couldnt be located, err=%v", initrd, e)
		return
	}

	log.Printf("********Step 7: kexec called  ********")
	//i := "initramfs-4.14.35-builtin-no-embedded+.img"
	//k := "vmlinuz-4.14.35-builtin-no-embedded-signed+"
	//cmdline := "console=ttyS0,115200n8 BOOT_IMAGE=/vmlinuz-4.14.35-builtin-no-embedded-signed+ root=/dev/mapper/ol-root ro crashkernel=auto netroot=iscsi:@10.196.210.62::3260::iqn.1986-03.com.sun:ovs112-boot rd.iscsi.initiator=iqn.1988-12.com.oracle:ovs112 netroot=iscsi:@10.196.210.64::3260::iqn.1986-03.com.sun:ovs112-boot rd.lvm.lv=ol/root rd.lvm.lv=ol/swap  numa=off transparent_hugepage=never LANG=en_US.UTF-8"

	// log.Printf("running command : kexec -s --initrd %s --command-line %s kernel=[%s]", i, cmdline, k)
	boot := exec.Command("kexec", "-l", "-s", "--initrd", i, "--command-line", cmdline, k)

	// boot := exec.Command("kexec", "-s", "-i", i, "-l", k, "-c", cmdline) /* this is u-root's kexec */

	boot.Stdin = os.Stdin
	boot.Stderr = os.Stderr
	boot.Stdout = os.Stdout
	if err := boot.Run(); err != nil {
		//need to decide how to bail, reboot, error msg & halt, or
		//recovery shell
		log.Printf("command finished with error: %v", err)
		os.Exit(1)
	}
	//sudo sync; sudo umount -a; sudo kexec -e
	boot = exec.Command("kexec", "-e")
	if err := boot.Run(); err != nil {
		//need to decide how to bail, reboot, error msg & halt, or
		//recovery shell
		log.Printf("command finished with error: %v", err)
		os.Exit(1)
	}
}

type policy struct {
	DefaultAction string
	Collectors    []measurement.Collector
	Launcher      launcher
	EventLog      eventlog
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

	log.Printf("Scanning kernel cmd line for *sl_policy* flag")
	val, ok := cmdline.Flag("sl_policy")
	if !ok {
		log.Printf("sl_policy cmdline flag is not set")
		return nil, errors.New("Flag Not Set")
	}

	// val is of type sda:path
	mntFilePath, mountPath, e := GetMountedFilePath(val, false) // false means readonly mount
	if e != nil {
		return nil, fmt.Errorf("scanKernelCmdLine: GetMountedFilePath err=%v", e)
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

	log.Printf("scanBlockDevice")
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
			log.Printf("policy file found on mountPath=%s, directory =%s", mountPath, c)
			return d, true // return when first policy file found
		}
		// Policy File not found. Moving on to next search root...
	}
	return nil, false
}

/*locateSLPolicy() ([]byte, error)
Check of kernel param sl_policy is set,
	- parse the string
Iterate through each local block device,
	- mount the block device
	- scan for securelaunch.policy under /, /efi, or /boot
Read in policy file */
func locateSLPolicy() ([]byte, error) {

	d, err := scanKernelCmdLine()
	if err == nil || err.Error() != "Flag Not Set" {
		return d, err
	}

	log.Printf("Searching and mounting block devices with bootable configs")
	blkDevices := diskboot.FindDevices("/sys/class/block/*") // FindDevices find and *mounts* the devices.
	if len(blkDevices) == 0 {
		return nil, errors.New("No block devices found")
	}

	for _, device := range blkDevices {
		devicePath, mountPath := device.DevPath, device.MountPath
		log.Printf("scanning for policy file under devicePath=%s, mountPath=%s", devicePath, mountPath)
		raw, found := scanBlockDevice(mountPath)
		if e := mount.Unmount(mountPath, true, false); e != nil {
			log.Printf("Unmount failed. PANIC")
			panic(e)
		}

		if !found {
			log.Printf("no policy file found under this device")
			continue
		}

		log.Printf("policy file found.")
		return raw, nil
	}

	return nil, errors.New("policy file not found anywhere.")
}

func parseSLPolicy(pf []byte) (*policy, error) {
	p := &policy{}
	var parse struct {
		DefaultAction string            `json:"default_action"`
		Collectors    []json.RawMessage `json:"collectors"`
		Attestor      json.RawMessage   `json:"attestor"`
		Launcher      json.RawMessage   `json:"launcher"`
		EventLog      json.RawMessage   `json:"eventlog"`
	}

	if err := json.Unmarshal(pf, &parse); err != nil {
		log.Printf("parseSLPolicy: Unmarshall error for entire policy file!! err=%v", err)
		return nil, err
	}

	p.DefaultAction = parse.DefaultAction

	for _, c := range parse.Collectors {
		collector, err := measurement.GetCollector(c)
		if err != nil {
			log.Printf("getCollector failed for c=%s, collector=%v", c, collector)
			return nil, err
		}
		p.Collectors = append(p.Collectors, collector)
	}

	// log.Printf("len(parse.Launcher)=%d, parse.Launcher=%s", len(parse.Launcher), parse.Launcher)
	if len(parse.Launcher) > 0 {
		if err := json.Unmarshal(parse.Launcher, &p.Launcher); err != nil {
			log.Printf("parseSLPolicy: Launcher Unmarshall error=%v!!", err)
			return nil, err
		}
	}

	if len(parse.EventLog) > 0 {
		if err := json.Unmarshal(parse.EventLog, &p.EventLog); err != nil {
			log.Printf("parseSLPolicy: EventLog Unmarshall error=%v!!", err)
			return nil, err
		}
	}

	return p, nil
}

func main() {

	log.Printf("********Step 1: init completed. starting main ********")
	tpm2, err := tpm2.OpenTPM("/dev/tpm0")
	if err != nil {
		log.Printf("Couldn't talk to TPM Device: err=%v", err)
		os.Exit(1)
	}

	defer tpm2.Close()
	// log.Printf("TPM version %s", tpm.Version())
	// TODO Request TPM locality 2, requires extending go-tpm for locality request

	log.Printf("********Step 2: locateSLPolicy ********")
	rawBytes, err := locateSLPolicy()
	if err != nil {
		// TODO unmount all devices.
		log.Printf("locateSLPolicy failed: err=%v", err)
		//need to decide how to bail, reboot, error msg & halt, or
		//recovery shell
		os.Exit(1)
	}

	log.Printf("policy file located")
	log.Printf("********Step 3: parseSLPolicy ********")
	// The policy file must be measured and extended into PCR21 (PCR15
	// until DRTM launch is working and able to set locality
	p, err := parseSLPolicy(rawBytes)
	if err != nil {
		//need to decide how to bail, reboot, error msg & halt, or
		//recovery shell
		log.Printf("parseSLPolicy failed ")
		return
	}

	if p == nil {
		log.Printf("SL Policy parsed into a null set")
		os.Exit(1)
	}

	log.Printf("policy file parsed")

	log.Printf("********Step 4: Collecting Evidence ********")
	// log.Printf("policy file parsed=%v", p)
	for _, c := range p.Collectors {
		log.Printf("Input Collector: %v", c)
		c.Collect(tpm2)
	}
	log.Printf("Collectors completed")

	log.Printf("********Step 5: Write parsed eventlog to disk *********")
	if e := p.EventLog.Persist(); e != nil {
		log.Printf("write eventlog File To Disk failed err=%v", e)
		return
	}

	log.Printf("********Step 5: Launcher called ********")
	p.Launcher.Boot(tpm2)
}

// To run the daemon in debug mode please pass the parameter  '-d <debug level>'
// DEBUG         4 - Print all messages
// INFO          3 - Print messages needed to follow the uIP code (default)
// WARN          2 - Print warning messages
// ERROR         1 - Only print critical errors
// netroot=iscsi:@10.196.210.62::3260::iqn.1986-03.com.sun:ovs112-boot rd.iscsi.initiator=iqn.1988-12.com.oracle:ovs112
// netroot=iscsi:@10.196.210.64::3260::iqn.1986-03.com.sun:ovs112-boot
//NOTE:  if you have two netroot params in kernel command line , second one will be used.
func scanIscsiDrives() error {

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

	cmd00 := exec.Command("iscsistart", "-d=ERROR", "-a", ip, "-g", portalGroup, "-p", port, "-t", target, "-i", initiatorName)
	var out00 bytes.Buffer
	cmd00.Stdout = &out00
	log.Printf("Executing %v", cmd00.Args)
	if err00 := cmd00.Run(); err00 != nil {
		fmt.Println(err00)
	} else {
		log.Printf("Output: %v", cmd00.Stdout)
	}
	return nil
}

// populate required modules here
func init() {

	/*
		// log.Printf("Executing depmod -a\n")
		cmd1 := exec.Command("/usr/sbin/depmod", "-a")
		log.Printf("Executing %v", cmd1.Args)
		if err := cmd1.Run(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		//modules_list := []string{"ahci", "sd_mod", "ext4", "iscsi_tcp", "be2iscsi", "e1000", "e1000e", "bnxt_en", "ib_isert", "ib_iser", "qla4xxx", "cxgb4i", "cxgb3i", "bnx2i", "libcxgbi", "iscsi_target_mod", "ib_srpt"}

		/* depmod is executed once while modprobe three times
		ahci, sd_mod for block device detection, ext4 for fstypes() function */
	// cmds := map[string][]string{
	//	"/usr/sbin/modprobe -a": modules_list,
	// }
	/*
		cmd2 := exec.Command("/usr/sbin/modprobe", "-a", "ahci", "sd_mod", "ext4", "iscsi_tcp", "be2iscsi", "e1000", "e1000e", "bnxt_en")
		log.Printf("Executing %v", cmd2.Args)
		//log.Printf("running command : %v %v: and waiting for it to finish...\n", cmd_path, option)
		if err := cmd2.Run(); err != nil {
			log.Printf("command finished with error: %v\n", err)
			// os.Exit(1)
		}
	*/
	/*
		for cmd_path, options := range cmds {
			for _, option := range options {
				cmd3 = exec.Command(cmd_path, option)
				log.Printf("running command : %v %v: and waiting for it to finish...\n", cmd_path, option)
				if err := cmd.Run(); err != nil {
					log.Printf("command finished with error: %v\n", err)
					os.Exit(1)
				}
				log.Printf("..... Done\n")
			}
		}
	*/

	// DOES NOT WORK: dhclient -v -ipv4 -ipv6=false
	// command finished with error: exec: "dhclient -v -ipv4 -ipv6=false": executable file not found in $PATH
	// cmd3 := exec.Command("dhclient -v -ipv4 -ipv6=false")

	/*
		// log.Printf("Executing dhclient -v -ipv4 -ipv6=false -timeout 5 -retry 5\n")
		for i := 2; i > 0; i-- {
			cmd3 := exec.Command("dhclient", "-v", "-ipv4", "-ipv6=false")
			log.Printf("Executing %v", cmd3.Args)
			// stdoutStderr, err := cmd3.CombinedOutput()
			if err := cmd3.Run(); err != nil {
				log.Printf("command finished with error: %v\n", err)
				//fmt.Printf("Output: %s\n", stdoutStderr)
				// fmt.Println(err)
				// os.Exit(1)
			}
			// fmt.Printf("Command ran\n: %s\n", stdoutStderr)
		}
	*/

	/*
		// iscsi should be obtained from platform policy file.
		portal_ip1 := "10.211.11.101:3260"
		target_iqn1 := "iqn.2003-01.org.linux-iscsi.ca-virt1-1.x8664:sn.fe34c16ae741"
		log.Printf("Attempt #1 to mount ISCSI drive over TCP/IP\n")
		log.Printf("portal=%s\n", portal_ip1)
		log.Printf("target_iqn=%s\n", target_iqn1)
		device1, err1 := iscsinl.MountIscsi(portal_ip1, target_iqn1)
		// device, err := iscsinl.MountIscsi("10.196.210.62:3260", "iqn.1986-03.com.sun:ovs112-boot")
		if err1 != nil {
			fmt.Println(err1)
			// os.Exit(1)
		} else {
			log.Printf("Mounted at dev %v", device1)
		}*/

	/*
		// iscsi should be obtained from platform policy file.
		portal_ip2 := "10.196.210.62:3260"
		target_iqn2 := "iqn.1986-03.com.sun:ovs112-boot"
		log.Printf("Attempt to mount ISCSI drive over TCP/IP\n")
		log.Printf("portal=%s\n", portal_ip2)
		log.Printf("target_iqn=%s\n", target_iqn2)
		device2, err2 := iscsinl.MountIscsi(portal_ip2, target_iqn2)
		// device, err := iscsinl.MountIscsi("10.196.210.62:3260", "iqn.1986-03.com.sun:ovs112-boot")
		if err2 != nil {
			fmt.Println(err2)
			// os.Exit(1)
		} else {
			log.Printf("Mounted at dev %v", device2)
		}*/

	/*
		// iscsistart -a 10.196.210.62 -g 1 -t iqn.1986-03.com.sun:ovs112-boot -i iqn.1988-12.com.oracle:ovs112
		cmd00 := exec.Command("iscsistart", "-a", "10.211.11.101", "-g", "1", "-t", "iqn.2003-01.org.linux-iscsi.ca-virt1-1.x8664:sn.fe34c16ae741", "-i", "iqn.1988-12.com.oracle:debcc0ab1ba2")
		// cmd00 := exec.Command("iscsistart", "-a", "10.196.210.62", "-g", "1", "-t", "iqn.1986-03.com.sun:ovs112-boot", "-i", "iqn.1988-12.com.oracle:ovs112")
		var out00 bytes.Buffer
		cmd00.Stdout = &out00
		log.Printf("Executing %v", cmd00.Args)
		if err00 := cmd00.Run(); err00 != nil {
			fmt.Println(err00)
		} else {
			log.Printf("Output: \n%v", cmd00.Stdout)
		}
	*/
	cmd0 := exec.Command("bash", "-c", "cpuid > /tmp/cpuid.txt")
	var out0 bytes.Buffer
	cmd0.Stdout = &out0
	log.Printf("Executing %v", cmd0.Args)
	if err0 := cmd0.Run(); err0 != nil {
		fmt.Println(err0)
	} else {
		log.Printf("Output: %v", cmd0.Stdout)
	}

	/*
		cmd01 := exec.Command("bash", "-c", "chipsec_main > /tmp/chipsec_main.txt")
		var out01 bytes.Buffer
		cmd01.Stdout = &out01
		log.Printf("Executing %v", cmd01.Args)
		if err01 := cmd01.Run(); err01 != nil {
			fmt.Println(err01)
		} else {
			log.Printf("Output: \n%v", cmd01.Stdout)
		}

		// check if any device is a lvm
		cmd0 := exec.Command("lvdisplay")
		var out0 bytes.Buffer
		cmd0.Stdout = &out0
		log.Printf("Executing %v", cmd0.Args)
		if err0 := cmd0.Run(); err0 != nil {
			fmt.Println(err0)
		} else {
			log.Printf("Output: \n%v", cmd0.Stdout)
		}

		cmd01 := exec.Command("vgscan")
		var out01 bytes.Buffer
		cmd01.Stdout = &out01
		log.Printf("Executing %v", cmd01.Args)
		if err01 := cmd01.Run(); err01 != nil {
			fmt.Println(err01)
		} else {
			log.Printf("Output: \n%v", cmd01.Stdout)
		}

		// vgchange creates block devices under /sys/class/block
		cmd02 := exec.Command("vgchange", "-ay", "VG_DB")
		var out02 bytes.Buffer
		cmd02.Stdout = &out02
		log.Printf("Executing %v", cmd02.Args)
		if err02 := cmd02.Run(); err02 != nil {
			fmt.Println(err02)
		} else {
			log.Printf("Output: \n%v", cmd02.Stdout)
		}

		// vgchange creates block devices under /sys/class/block
		cmd020 := exec.Command("vgchange", "-ay", "ol")
		var out020 bytes.Buffer
		cmd020.Stdout = &out020
		log.Printf("Executing %v", cmd020.Args)
		if err020 := cmd020.Run(); err020 != nil {
			fmt.Println(err020)
		} else {
			log.Printf("Output: \n%v", cmd020.Stdout)
		}

		cmd03 := exec.Command("lvs")
		var out03 bytes.Buffer
		cmd03.Stdout = &out03
		log.Printf("Executing %v", cmd03.Args)
		if err03 := cmd03.Run(); err03 != nil {
			fmt.Println(err03)
		} else {
			log.Printf("Output: \n%v", cmd03.Stdout)
		}
	*/
	err := scanIscsiDrives()
	if err != nil {
		log.Printf("NO ISCSI DRIVES found, err=[%v]", err)
	}

	cmd1 := exec.Command("ls", "/sys/class/net")
	var out1 bytes.Buffer
	cmd1.Stdout = &out1
	log.Printf("Executing %v", cmd1.Args)
	if err1 := cmd1.Run(); err1 != nil {
		fmt.Println(err1)
	} else {
		log.Printf("Output: %v", cmd1.Stdout)
	}

	cmd2 := exec.Command("ls", "/sys/class/block")
	var out2 bytes.Buffer
	cmd2.Stdout = &out2
	log.Printf("Executing %v", cmd2.Args)
	if err2 := cmd2.Run(); err2 != nil {
		fmt.Println(err2)
	} else {
		log.Printf("Output: %v", cmd2.Stdout)
	}

	cmd3 := exec.Command("tpmtool", "eventlog", "dump", "--txt", "--tpm12", "/eventlog_ross > /tmp/parsedEvtLog.txt")
	// cmd3 := exec.Command("tpmtool", "eventlog", "dump", "--txt", "--tpm20", "/sys/kernel/security/slaunch/eventlog > /tmp/parsedEvtLog.txt")
	var out3 bytes.Buffer
	cmd3.Stdout = &out3
	log.Printf("Executing %v", cmd3.Args)
	if err3 := cmd3.Run(); err != nil {
		fmt.Println(err3)
	} else {
		log.Printf("Output: %v", cmd3.Stdout)
	}
	/*
	   s := "sleeping, press CTRL C if u like"
	   for i := 0; i < 5; i++ {
	       time.Sleep(5 * time.Second)
	       fmt.Println(s)
	   } */
}
