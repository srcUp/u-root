// Copyright 2019 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/u-root/u-root/pkg/cmdline"
	"github.com/u-root/u-root/pkg/diskboot"
	"github.com/u-root/u-root/pkg/find"
	"github.com/u-root/u-root/pkg/measurement"
	"github.com/u-root/u-root/pkg/mount"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/TrenchBoot/tpmtool/pkg/tpm"
)

type launcher struct {
	Type   string   `json:"type"`
	Cmd    string   `json:"cmd"`
	Params []string `json:"params"`
}

func (l *launcher) Boot() {
	boot := exec.Command(l.Cmd, l.Params...)
	boot.Stdin = os.Stdin
	boot.Stderr = os.Stderr
	boot.Stdout = os.Stdout

	err := boot.Run()
	if err != nil {
		//need to decide how to bail, reboot, error msg & halt, or
		//recovery shell
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
	log.Printf("Checking if sl_policy cmdline param is set")
	val, ok := cmdline.Flag("sl_policy")
	if !ok {
		return nil, errors.New("Flag Not Set")
	}

	s := strings.Split(val, ":")
	if len(s) != 2 {
		return nil, fmt.Errorf("%v: incorrect format. Usage: sl_policy=<block device identifier>:<path>", val)
	}

	log.Printf("sl_policy flag is set with val=%s", val)
	devicePath := filepath.Join("/dev", s[0])   // assumes deviceId is sda, devicePath=/dev/sda
	dev, err := diskboot.FindDevice(devicePath) // FindDevice fn mounts devicePath=/dev/sda.
	if err != nil {
		return nil, err
	}

	mountPath := dev.MountPath + s[1] // mountPath=/tmp/slaunch.policy if /dev/sda mounted on /tmp
	d, err := ioutil.ReadFile(mountPath)
	if err != nil {
		// - TODO: should we check for end of file ?
		return nil, fmt.Errorf("Error reading policy file found at mountPath=%s, devicePath=%s, passed=%s", mountPath, devicePath, val)
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
	if err == nil || err.Error() != "FlagNotSet" {
		return d, nil
	}

	blkDevices := diskboot.FindDevices("/sys/class/block/*") // FindDevices find and *mounts* the devices.
	if len(blkDevices) == 0 {
		return nil, errors.New("No block devices found")
	}

	log.Printf("%v block devices detected", len(blkDevices))
	for _, device := range blkDevices {
		devicePath, mountPath := device.DevPath, device.MountPath
		log.Printf("scanning for policy file under devicePath=%s, mountPath=%s\n", devicePath, mountPath)
		raw, found := scanBlockDevice(mountPath)
		if e := mount.Unmount(mountPath, true, false); e != nil {
			log.Printf("Unmount failed. PANIC")
			panic(e)
		}

		if !found {
			log.Printf("no policy file found under this device")
			continue
		}

		log.Printf("policy File Found.\n")
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
		Launcher      json.RawMessage   `json:"launcher`
	}

	if err := json.Unmarshal(pf, &parse); err != nil {
		return nil, err
	}

	p.DefaultAction = parse.DefaultAction

	for _, c := range parse.Collectors {
		collector, err := measurement.GetCollector(c)
		if err != nil {
			return nil, err
		}
		p.Collectors = append(p.Collectors, collector)
	}

	if len(parse.Launcher) > 0 {
		if err := json.Unmarshal(parse.Launcher, &p.Launcher); err != nil {
			return nil, err
		}
	}

	return p, nil
}

func main() {
	log.Printf("init completed. starting main ......\n")
	tpm, err := tpm.NewTPM()

	// Request TPM locality 2, requires extending go-tpm for locality request

	rawBytes, err := locateSLPolicy()
	if err != nil {
		log.Printf("locateSLPolicy failed: err=%v", err)
		//need to decide how to bail, reboot, error msg & halt, or
		//recovery shell
	}
	return
	// The policy file must be measured and extended into PCR21 (PCR15
	// until DRTM launch is working and able to set locality

	p, err := parseSLPolicy(rawBytes)
	if err != nil {
		//need to decide how to bail, reboot, error msg & halt, or
		//recovery shell
	}

	for _, c := range p.Collectors {
		c.Collect(&tpm)
	}

	p.Launcher.Boot()
}

// populate required modules here
func init() {
	// depmod is executed once while modprobe three times
	cmds := map[string][]string{
		"/usr/sbin/depmod":   {"-a"},
		"/usr/sbin/modprobe": {"ahci", "sd_mod", "ext4"},
	}

	for cmd_path, options := range cmds {
		for _, option := range options {
			cmd := exec.Command(cmd_path, option)
			log.Printf("running command : %v %v: and waiting for it to finish...", cmd_path, option)
			if err := cmd.Run(); err != nil {
				log.Printf("command finished with error: %v", err)
				os.Exit(1)
			}
			log.Printf("..... Done\n")
		}
	}
}
