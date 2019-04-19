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
	Launcher      launcher
}

func locateSLPolicy() ([]byte, error) {
	// Check of kernel param sl_policy is set,
	// 	- parse the string
	// Iterate through each local block device,
	// 	- mount the block device
	// 	- scan for securelaunch.policy under /, /efi, or /boot
	// Read in policy file
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
		if collector, err := measurement.GetCollector(c); err != nil {
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
