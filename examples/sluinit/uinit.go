// Copyright 2019 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"u-root/pkg/cmdline"
	"u-root/pkg/diskboot"
	"u-root/pkg/find"
	"u-root/pkg/measurement"
	"u-root/pkg/mount"

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

func scanBlockDevice(devicepath string) ([]byte, error) {
	log.Printf("Finding securelaunch.policy file under %s", devicepath)
	f, err := find.New(func(f *find.Finder) error {
		f.Root = devicepath
		f.Pattern = "securelaunch.policy"
		return nil
	})

	if err != nil {
		// couldn't create anonymous function
		log.Printf("couldn't create anonymous function")
	}
	go f.Find()

	// check if calling len(channel) is safe.
	if len(f.Names) == 0 {
		log.Printf("No policy file found under %s, continuing", devicepath)
		return nil, fmt.Errorf("No policy file found under %s", devicepath)
	}

	// Read in policy file:
	for o := range f.Names {
		if o.Err != nil {
			log.Printf("%v: got %v, want nil", o.Name, o.Err)
		}
		d, err := ioutil.ReadFile(o.Name)
		if err != nil {
			log.Printf("Error reading policy file found under %s, continuing", o.Name)
			continue
		}
		// return when first policy file found
		return d, nil
	}
	return nil, fmt.Errorf("Unable to read any of the policy files under %s.Exiting scanBlockDevice() ", devicepath)
}

func locateSLPolicy() ([]byte, error) {

	// Override: Check of kernel param sl_policy is set, - parse the string
	log.Printf("Checking if sl_policy is set")
	if val, ok := cmdline.Flag("sl_policy"); ok {
		log.Printf("sl_policy flag is set with val=%s", val)
		// return when a policy file is found
		fmt.Println(val)
	}

	cmd := exec.Command("modprobe", "-S 4.14.35-1838.el7uek.x86_64", "ata_generic")
	if _, err := cmd.CombinedOutput(); err != nil {
		log.Printf("err not nil from modprobe: %v", err)
	}

	log.Printf("Searching for securelaunch.policy on all block devices")
	// FindDevices fn iterates over all local block devices and mounts them.
	devices := diskboot.FindDevices("/sys/class/block/*")
	if len(devices) == 0 {
		log.Printf("No block devices found. Scanning policy file elsewhere.")
	} else {
		log.Printf("Some block devices detected.")
		for _, device := range devices {
			devicepath := device.DevPath
			log.Printf("Scanning for policy file under %s", devicepath)
			// diskboot.FindDevice would return the /dev/foo path.
			raw, err := scanBlockDevice(devicepath)
			if e := mount.Unmount(devicepath, true, false); e != nil {
				log.Printf("Unmount failed for %s. PANIC", devicepath)
				panic(e)
			}

			if err != nil {
				log.Printf("Policy File not found under %s. Moving on to next device.", devicepath)
				continue
			}
			log.Printf("Policy File found under %s. Exiting locateSLPolicy()", devicepath)
			return raw, nil
		}
	}

	// scan for securelaunch.policy under /, /efi, or /boot
	var SearchRoots = []string{
		"/",
		"/efi",
		"/boot",
	}
	for _, c := range SearchRoots {
		raw, err := scanBlockDevice(c)
		if e := mount.Unmount(c, true, false); e != nil {
			log.Printf("Unmount failed for %s. PANIC", c)
			panic(e)
		}

		if err != nil {
			log.Printf("Policy File not found under %s. Moving on to next device.", c)
			continue
		}
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
