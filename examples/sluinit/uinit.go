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
	//Attestor      []attestation.Attestor
	Launcher launcher
}

/* recursively scans an already mounted block device inside directories
	"/", "/efi" and "/boot" for policy file

	e.g: if you mount /dev/sda1 on /tmp/sda1,
 	then mountPath would be /tmp/sda1
 	and searchPath would be /tmp/sda1/, /tmp/sda1/efi, and /tmp/sda1/boot
		respectively for each iteration of loop over SearchRoots slice.
*/
func scanBlockDevice(mountPath string) ([]byte, error) {
	log.Printf("Finding securelaunch.policy file under %s", mountPath)
	// scan for securelaunch.policy under /, /efi, or /boot
	var SearchRoots = []string{"/", "/efi", "/boot"}
	for _, c := range SearchRoots {

		searchPath := mountPath + c
		f, err := find.New(func(f *find.Finder) error {
			f.Root = searchPath
			f.Pattern = "securelaunch.policy"
			return nil
		})

		if err != nil {
			log.Printf("Error creating anonymous function for find, continue")
			continue
		}
		//spawn a goroutine
		go f.Find()

		// check if calling len(channel) is safe.
		if len(f.Names) == 0 {
			log.Printf("No policy file found under %s, continuing", searchPath)
			continue
		}

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
			// return when first policy file found
			return d, nil
		}
		log.Printf("Policy File not found under %s. Moving on to next search root.", searchPath)
	}
	return nil, fmt.Errorf("No policy file found _OR_ if found, error reading them. Exiting scanBlockDevice() for %s", mountPath)
}

func locateSLPolicy() ([]byte, error) {
	// Override: Check of kernel param sl_policy is set, - parse the string
	// <block device identifier>:<path>
	// e.g 1 sda:/boot/securelaunch.policy
	// e.g 2 77d8da74-a690-481a-86d5-9beab5a8e842:/boot/securelaunch.policy
	// TODO: <block device identifier>:<mount options>,<path>
	log.Printf("Checking if sl_policy is set")
	if val, ok := cmdline.Flag("sl_policy"); ok {
		log.Printf("sl_policy flag is set with val=%s", val)
		fmt.Println(val)
		s := strings.Split(val, ":")
		if len(s) != 2 {
			log.Printf("incorrect format of sl_policy cmd line parameter")
			log.Printf("I will be nice. Instead of quiting on you. Will search block devices for you in case you put the file there")
		} else {
			deviceId := s[0]
			devicePath := s[1]
			log.Printf("Policy file found at %s on device %s", devicePath, deviceId)
			// mount the first one and read the file.
			d, err := ioutil.ReadFile(devicePath)
			if err != nil {
				log.Printf("Error reading policy file found at %s under %s", devicePath, deviceId)
			} else {
				// return when first policy file found
				return d, nil
			}
		}
	}

	cmd := exec.Command("modprobe", "-S", "4.14.35-1838.el7uek.x86_64", "ata_generic")
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("err not nil from modprobe: %s %v", string(out), err)
	}

	log.Printf("Searching for securelaunch.policy on all block devices")
	// FindDevices fn iterates over all local block devices and mounts them.
	blkDevices := diskboot.FindDevices("/sys/class/block/*")
	if len(blkDevices) == 0 {
		log.Printf("No block devices found. Scanning policy file elsewhere.")
		return nil, errors.New("No block devices found. where is policy file ?")
	}

	log.Printf("Some block devices detected.")
	for _, device := range blkDevices {
		devicePath := device.DevPath
		mountPath := device.MountPath
		log.Printf("Scanning for policy file under devicePath=%s, mountPath=%s", devicePath, mountPath)
		raw, err := scanBlockDevice(mountPath)
		if e := mount.Unmount(mountPath, true, false); e != nil {
			log.Printf("Unmount failed for devicePath=%s mountPath=%s. PANIC", devicePath, mountPath)
			panic(e)
		}
		if err != nil {
			log.Printf("Policy File not found under devicePath=%s, mountPath=%s. Moving on to next device.", devicePath, mountPath)
			continue
		}

		log.Printf("Policy File found under devicePath=%s, mountPath=%s. Exiting locateSLPolicy()", devicePath, mountPath)
		return raw, nil
	}

	return nil, errors.New("Policy File not found")
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

	/* TODO
	if len(parse.Attestor) > 0 {
		if p.Attestor, err = attestation.GetAttestor(parse.Attestor); err != nil {
			return nil, err
		}
	}
	*/

	if len(parse.Launcher) > 0 {
		if err := json.Unmarshal(parse.Launcher, &p.Launcher); err != nil {
			return nil, err
		}
	}

	return p, nil
}

func main() {
	log.Printf("testing printf in live environment\n")
	tpm, err := tpm.NewTPM()

	// Request TPM locality 2, requires extending go-tpm for locality request

	rawBytes, err := locateSLPolicy()
	if err != nil {
		log.Printf("locateSLPolicy failed with err=%v", err)
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

	/* TODO
	res, err := p.Attestor.Attest(&tpm)
	if err {
		//need to decide how to bail, reboot, error msg & halt, or
		//recovery shell
	}
	*/

	p.Launcher.Boot()
}
