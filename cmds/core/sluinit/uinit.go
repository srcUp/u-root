// Copyright 2019 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/u-root/u-root/pkg/cmdline"
	slaunch "github.com/u-root/u-root/pkg/securelaunch"
	"github.com/u-root/u-root/pkg/securelaunch/policy"
	"github.com/u-root/u-root/pkg/securelaunch/tpm"
)

var (
	slDebug = flag.Bool("d", false, "enable debug logs")
)

func checkDebugFlag() {
	/*
	 * check if uroot.uinitargs=-d is set in kernel cmdline.
	 * if set, slaunch.Debug is set to log.Printf.
	 */
	flag.Parse()

	if flag.NArg() > 1 {
		log.Fatal("Incorrect number of arguments")
	}

	if *slDebug {
		slaunch.Debug = log.Printf
		slaunch.Debug("debug flag is set. Logging Enabled.")
	}
}

/*
 * main parses platform policy file, and based on the inputs,
 * performs measurements and then launches a target kernel.
 *
 * steps followed by sluinit:
 * 1. if debug flag is set, enable logging.
 * 2. gets the TPM handle
 * 3. Gets secure launch policy file entered by user.
 * 4. calls collectors to collect measurements(hashes) a.k.a evidence.
 */
func main() {
	checkDebugFlag()

	slaunch.Debug("********Step 1: init completed. starting main ********")
	tpmDev, err := tpm.GetHandle()
	if err != nil {
		log.Printf("tpm.getHandle failed. err=%v", err)
		os.Exit(1)
	}
	defer tpmDev.Close()

	slaunch.Debug("********Step 2: locate and parse SL Policy ********")
	p, err := policy.Get()
	if err != nil {
		log.Printf("failed to get policy err=%v", err)
		os.Exit(1)
	}
	slaunch.Debug("policy file successfully parsed")

	slaunch.Debug("********Step 3: Collecting Evidence ********")
	for _, c := range p.Collectors {
		slaunch.Debug("Input Collector: %v", c)
		if e := c.Collect(tpmDev); e != nil {
			log.Printf("Collector %v failed, err = %v", c, e)
		}
	}
	slaunch.Debug("Collectors completed")

	slaunch.Debug("********Step *: Write raw eventlog to /boot partition*********")
	if e := p.EventLog.Temp(); e != nil {
		log.Printf("EventLog.Temp() failed err=%v", e)
		os.Exit(1)
	}

	slaunch.Debug("********Step 4: Measuring target kernel, initrd ********")
	if e := p.Launcher.MeasureKernel(tpmDev); e != nil {
		log.Printf("Launcher.MeasureKernel failed err=%v", e)
		os.Exit(1)
	}

	slaunch.Debug("********Step 5: Write eventlog to /boot partition*********")
	if e := p.EventLog.Persist(); e != nil {
		log.Printf("EventLog.Persist() failed err=%v", e)
		os.Exit(1)
	}

	slaunch.Debug("********Step 6: Launcher called to Boot ********")
	err = p.Launcher.Boot(tpmDev)
	log.Printf("Boot failed. err=%s", err)
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

func init() {
	err := scanIscsiDrives()
	if err != nil {
		log.Printf("NO ISCSI DRIVES found, err=[%v]", err)
	}
}
