// Copyright 2019 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"os/exec"

	"github.com/TrenchBoot/tpmtool/pkg/tpm"
	"github.com/TrenchBoot/u-root/pkg/measurement"
)

type launcher struct {
	Type   string
	Cmd    string
	Params []string
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
	//Attestors []Attestor
	Launcher launcher
}

func locateSLPolicy() ([]byte, error) {
	// Check of kernel param sl_policy is set,
	// 	- parse the string
	// Iterate through each local block device,
	// 	- mount the block device
	// 	- scan for securelaunch.policy under /, /efi, or /boot
	// Read in policy file
}

func parseSLPolicy(p []byte) (policy, error) {
	// Parse the policy and use to instantiate,
	// 	- Collector(s)
	// 	- Attestor
	// 	- Launcher
	// Populate instance of policy structure and return
}

func main() {
	tpm := tpm.NewTPM()

	// Request TPM locality 2, requires extending go-tpm for locality request

	rawBytes, err := locateSLPolicy()
	if err {
		//need to decide how to bail, reboot, error msg & halt, or
		//recovery shell
	}

	// The policy file must be measured and extended into PCR21 (PCR15
	// until DRTM launch is working and able to set locality

	p, err := parseSLPolicy(rawBytes)
	if err {
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
