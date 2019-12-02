// Copyright 2019 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"github.com/u-root/u-root/pkg/cmdline"
	slaunch "github.com/u-root/u-root/pkg/securelaunch"
	"github.com/u-root/u-root/pkg/securelaunch/policy"
	"github.com/u-root/u-root/pkg/securelaunch/tpm"
	"log"
	"os"
)

/*
 * 1. gets the TPM handle
 * 2. Locates Secure Launch Policy File entered by user.
 * 3. Parses Secure Launch Policy File found in 2.
 * 4. Calls collectors to collect measurements(hashes) a.k.a evidence.
 */
func main() {

	slaunch.Debug("********Step 1: init completed. starting main ********")
	tpmDev, err := tpm.GetHandle()
	if err != nil {
		log.Printf("tpm.getHandle failed. err=%v", err)
		os.Exit(1)
	}
	defer tpmDev.Close()

	slaunch.Debug("********Step 2: locate SL Policy ********")
	rawBytes, err := policy.Locate()
	if err != nil {
		log.Printf("locate SL Policy failed: err=%v\n", err)
		os.Exit(1)
	}
	slaunch.Debug("policy file located\n")

	slaunch.Debug("********Step 3: parse SL Policy ********")
	//TODO: The policy file must be measured and extended into PCR21 (PCR15
	// until DRTM launch is working and able to set locality
	p, err := policy.Parse(rawBytes)
	if err != nil {
		log.Printf("parse Policy failed")
		os.Exit(1)
	}
	if p == nil {
		log.Printf("SL Policy parsed into a null set")
		os.Exit(1)
	}
	slaunch.Debug("policy file parsed\n")

	slaunch.Debug("********Step 4: Collecting Evidence ********")
	slaunch.Debug("policy file parsed=%v\n", p)

	for _, c := range p.Collectors {
		slaunch.Debug("Input Collector: %v\n", c)
		c.Collect(tpmDev)
	}
	slaunch.Debug("Collectors completed\n")

}

/*
 * check if uroot-debug=true in kernel cmdline.
 * if set, slaunch.Debug is set to log.Printf.
 * In debug mode, this function prints output of ls /sys/class/net
 * and ls /sys/class/block
 */
func init() {
	val, ok := cmdline.Flag("uroot-debug")
	if ok && val == "true" {
		slaunch.Debug = log.Printf
		slaunch.Debug("uroot-debug is set to true in kernel cmdline. Logging Enabled.")
		slaunch.CmdExec("ls", []string{"/sys/class/net"})
		slaunch.CmdExec("ls", []string{"/sys/class/block"})
	}
}
