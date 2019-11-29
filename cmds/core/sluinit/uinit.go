// Copyright 2019 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"github.com/google/go-tpm/tpm2"
	slaunch "github.com/u-root/u-root/pkg/securelaunch"
	"log"
	"os"
)

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
	rawBytes, err := slaunch.LocateSLPolicy()
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
	p, err := slaunch.ParseSLPolicy(rawBytes)
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

	log.Printf("********Step 5: Write eventlog to /boot partition*********\n")
	if e := p.EventLog.Persist(); e != nil {
		log.Printf("write eventlog File To Disk failed err=%v", e)
		return
	}

	log.Printf("********Step 5: Launcher called ********\n")
	p.Launcher.Boot(tpm2)
}

func init() {

	slaunch.Cmd_exec("bash", []string{"-c", "cpuid -1 | grep -e family -e model -e stepping | grep -v extended > /tmp/cpuid.txt"})

	err := slaunch.ScanIscsiDrives()
	if err != nil {
		log.Printf("NO ISCSI DRIVES found, err=[%v]", err)
	}

	slaunch.Cmd_exec("ls", []string{"/sys/class/net"})
	slaunch.Cmd_exec("ls", []string{"/sys/class/block"})
	slaunch.Cmd_exec("tpmtool", []string{"eventlog", "dump", "--txt", "--tpm20", "/sys/kernel/security/slaunch/eventlog > /tmp/parsedEvtLog.txt"})
}
