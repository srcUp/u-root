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

func locateSLPolicy() ([]byte, error) {
	log.Printf("Checking if sl_policy is set")
	// Check of kernel param sl_policy is set,
	// - parse the string
	if val, ok := cmdline.Flag("sl_policy"); ok {
		log.Printf("sl_policy flag is set with val=%s", val)
		// val contains value of sl_policy set. what is this ?
		// return when a policy file is found
		fmt.Println(val)
	}

	log.Printf("Searching for all block devices")
	// FindDevices fn iterates over all local block devices and mounts them.
	devices := diskboot.FindDevices("/sys/class/block/*")
	if len(devices) == 0 {
		return nil, errors.New("No devices found")
	}

	// 	- scan for securelaunch.policy under /, /efi, or /boot
	var SearchRoots = []string{
		"/",
		"/efi",
		"/boot",
	}

	for _, c := range SearchRoots {
		log.Printf("Finding securelaunch.policy file under %s", c)
		f, err := find.New(func(f *find.Finder) error {
			f.Root = c
			f.Pattern = "securelaunch.policy"
			return nil
		})

		if err != nil {
			// couldn't create anonymous function
			log.Printf("couldn't create anonymous function")
		}
		go f.Find()

		if len(f.Names) == 0 {
			log.Printf("No policy file found under %s, continuing", c)
			continue
		}

		// Read in policy file:
		for o := range f.Names {
			if o.Err != nil {
				log.Printf("%v: got %v, want nil", o.Name, o.Err)
			}
			d, err := ioutil.ReadFile(o.Name)
			if err != nil {
				log.Printf("Error reading policy file found under %s, continuing", c)
				continue
			}
			// return when first policy file found
			return d, nil
		}
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
