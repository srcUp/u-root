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
	"time"
)

type launcher struct {
	Type string `json:"type"`
	// Cmd    string   `json:"cmd"`
	Params map[string]string `json:"params"`
}

// obtains file path based on input entered by user in policy file.
// inputVal is of format <block device identifier>:<path>
// Example sda:/path/to/file
/* - mount device
 * - Get file Path on mounted device.
 * - Unmount device
 */
func GetMountedFilePath(inputVal string) (string, error) {
	s := strings.Split(inputVal, ":")
	if len(s) != 2 {
		return "", fmt.Errorf("%s: Usage: <block device identifier>:<path>\n", inputVal)
	}

	devicePath := filepath.Join("/dev", s[0])   // assumes deviceId is sda, devicePath=/dev/sda
	dev, err := diskboot.FindDevice(devicePath) // FindDevice fn mounts devicePath=/dev/sda.
	if err != nil {
		fmt.Printf("Mount %v failed, err=%v\n", devicePath, err)
		return "", err
	}

	filePath := dev.MountPath + s[1] // mountPath=/tmp/path/to/target/file if /dev/sda mounted on /tmp
	return filePath, nil
}

func (l *launcher) Boot(t io.ReadWriter) {

	if l.Type != "kexec" {
		log.Printf("Launcher: Unsupported launcher type. Exiting.\n")
		return
	}
	log.Printf("Identified Launcher Type = Kexec\n")
	// TODO: if kernel and initrd are on different devices.
	kernel := l.Params["kernel"]
	initrd := l.Params["initrd"]
	cmdline := l.Params["cmdline"]

	log.Printf("********Step 6: Measuring kernel, initrd ********\n")
	if e := measurement.MeasureInputFile(t, kernel); e != nil {
		log.Printf("Launcher: ERR: measure kernel input=%s, err=%v\n", kernel, e)
		return
	}

	if e := measurement.MeasureInputFile(t, initrd); e != nil {
		log.Printf("Launcher: ERR: measure initrd input=%s, err=%v\n", initrd, e)
		return
	}

	k, e := GetMountedFilePath(kernel)
	if e != nil {
		log.Printf("Launcher: ERR: kernel input %s couldnt be located, err=%v\n", kernel, e)
		return
	}

	i, e := GetMountedFilePath(initrd)
	if e != nil {
		log.Printf("Launcher: ERR: initrd input %s couldnt be located, err=%v\n", initrd, e)
		return
	}

	log.Printf("********Step 7: kexec called  ********\n")
	//i := "initramfs-4.14.35-builtin-no-embedded+.img"
	//k := "vmlinuz-4.14.35-builtin-no-embedded-signed+"
	//cmdline := "console=ttyS0,115200n8 BOOT_IMAGE=/vmlinuz-4.14.35-builtin-no-embedded-signed+ root=/dev/mapper/ol-root ro crashkernel=auto netroot=iscsi:@10.196.210.62::3260::iqn.1986-03.com.sun:ovs112-boot rd.iscsi.initiator=iqn.1988-12.com.oracle:ovs112 netroot=iscsi:@10.196.210.64::3260::iqn.1986-03.com.sun:ovs112-boot rd.lvm.lv=ol/root rd.lvm.lv=ol/swap  numa=off transparent_hugepage=never LANG=en_US.UTF-8"

	// log.Printf("running command : kexec -s --initrd %s --command-line %s kernel=[%s]\n", i, cmdline, k)
	boot := exec.Command("kexec", "-l", "-s", "--initrd", i, "--command-line", cmdline, k)

	// boot := exec.Command("kexec", "-s", "-i", i, "-l", k, "-c", cmdline) /* this is u-root's kexec */

	boot.Stdin = os.Stdin
	boot.Stderr = os.Stderr
	boot.Stdout = os.Stdout
	if err := boot.Run(); err != nil {
		//need to decide how to bail, reboot, error msg & halt, or
		//recovery shell
		log.Printf("command finished with error: %v\n", err)
		os.Exit(1)
	}
	//sudo sync; sudo umount -a; sudo kexec -e
	boot = exec.Command("kexec", "-e")
	if err := boot.Run(); err != nil {
		//need to decide how to bail, reboot, error msg & halt, or
		//recovery shell
		log.Printf("command finished with error: %v\n", err)
		os.Exit(1)
	}
}

type policy struct {
	DefaultAction string
	Collectors    []measurement.Collector
	Launcher      launcher
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

	s := strings.Split(val, ":")
	if len(s) != 2 {
		return nil, fmt.Errorf("%v: incorrect format. Usage: sl_policy=<block device identifier>:<path>", val)
	}

	log.Printf("sl_policy flag is set with val=%s", val)
	devicePath := filepath.Join("/dev", s[0]) // assumes deviceId is sda, devicePath=/dev/sda
	log.Printf("devicePath=%v", devicePath)
	dev, err := diskboot.FindDevice(devicePath) // FindDevice fn mounts devicePath=/dev/sda.
	if err != nil {
		return nil, err
	}

	mountPath := dev.MountPath + s[1] // mountPath=/tmp/slaunch.policy if /dev/sda mounted on /tmp
	log.Printf("Reading file=%s", mountPath)
	d, err := ioutil.ReadFile(mountPath)
	if err != nil {
		// - TODO: should we check for end of file ?
		return nil, fmt.Errorf("Error reading policy file found at mountPath=%s, devicePath=%s, passed=%s\n", mountPath, devicePath, val)
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

	log.Printf("Searching and mounting block devices with bootable configs\n")
	blkDevices := diskboot.FindDevices("/sys/class/block/*") // FindDevices find and *mounts* the devices.
	if len(blkDevices) == 0 {
		return nil, errors.New("No block devices found")
	}

	for _, device := range blkDevices {
		devicePath, mountPath := device.DevPath, device.MountPath
		log.Printf("scanning for policy file under devicePath=%s, mountPath=%s\n", devicePath, mountPath)
		raw, found := scanBlockDevice(mountPath)
		if e := mount.Unmount(mountPath, true, false); e != nil {
			log.Printf("Unmount failed. PANIC\n")
			panic(e)
		}

		if !found {
			log.Printf("no policy file found under this device\n")
			continue
		}

		log.Printf("policy file found.\n")
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
	}

	if err := json.Unmarshal(pf, &parse); err != nil {
		log.Printf("parseSLPolicy: Unmarshall error for entire policy file!! err=%v\n", err)
		return nil, err
	}

	p.DefaultAction = parse.DefaultAction

	for _, c := range parse.Collectors {
		collector, err := measurement.GetCollector(c)
		if err != nil {
			log.Printf("getCollector failed for c=%s, collector=%v\n", c, collector)
			return nil, err
		}
		p.Collectors = append(p.Collectors, collector)
	}

	// log.Printf("len(parse.Launcher)=%d, parse.Launcher=%s\n", len(parse.Launcher), parse.Launcher)
	if len(parse.Launcher) > 0 {
		if err := json.Unmarshal(parse.Launcher, &p.Launcher); err != nil {
			log.Printf("parseSLPolicy: Launcher Unmarshall error=%v!!\n", err)
			return nil, err
		}
	}
	return p, nil
}

func main() {

	log.Printf("********Step 1: init completed. starting main ********\n")
	tpm2, err := tpm2.OpenTPM("/dev/tpm0")
	if err != nil {
		log.Printf("Couldn't talk to TPM Device: err=%v\n", err)
		os.Exit(1)
	}

	defer tpm2.Close()
	// log.Printf("TPM version %s", tpm.Version())
	// TODO Request TPM locality 2, requires extending go-tpm for locality request

	log.Printf("********Step 2: locateSLPolicy ********\n")
	rawBytes, err := locateSLPolicy()
	if err != nil {
		// TODO unmount all devices.
		log.Printf("locateSLPolicy failed: err=%v\n", err)
		//need to decide how to bail, reboot, error msg & halt, or
		//recovery shell
		os.Exit(1)
	}

	log.Printf("policy file located\n")
	log.Printf("********Step 3: parseSLPolicy ********\n")
	// The policy file must be measured and extended into PCR21 (PCR15
	// until DRTM launch is working and able to set locality
	p, err := parseSLPolicy(rawBytes)
	if err != nil {
		//need to decide how to bail, reboot, error msg & halt, or
		//recovery shell
		log.Printf("parseSLPolicy failed \n")
		return
	}

	if p == nil {
		log.Printf("SL Policy parsed into a null set\n")
		os.Exit(1)
	}

	log.Printf("policy file parsed\n")

	log.Printf("********Step 4: Collecting Evidence ********\n")
	// log.Printf("policy file parsed=%v\n", p)
	for _, c := range p.Collectors {
		log.Printf("Input Collector: %v\n", c)
		c.Collect(tpm2)
	}
	log.Printf("Collectors completed\n")

	log.Printf("********Step 5: Launcher called ********\n")
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
		log.Printf("Output: \n%v", cmd00.Stdout)
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
		log.Printf("Output: \n%v", cmd0.Stdout)
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
		log.Printf("Output: \n%v", cmd1.Stdout)
	}

	cmd2 := exec.Command("ls", "/sys/class/block")
	var out2 bytes.Buffer
	cmd2.Stdout = &out2
	log.Printf("Executing %v", cmd2.Args)
	if err2 := cmd2.Run(); err2 != nil {
		fmt.Println(err2)
	} else {
		log.Printf("Output: \n%v", cmd2.Stdout)
	}

	cmd3 := exec.Command("tpmtool", "-log", "/sys/kernel/security/slaunch/eventlog", "-tpm20")
	var out3 bytes.Buffer
	cmd3.Stdout = &out3
	log.Printf("Executing %v", cmd3.Args)
	if err3 := cmd3.Run(); err != nil {
		fmt.Println(err3)
	} else {
		log.Printf("Output: \n%v", cmd3.Stdout)
	}

	s := "sleeping, press CTRL C if u like"
	for i := 0; i < 5; i++ {
		time.Sleep(5 * time.Second)
		fmt.Println(s)
	}

}
