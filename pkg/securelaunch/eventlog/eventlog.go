// Copyright 2019 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package eventlog parses kernel event logs and saves the parsed data on a file on disk.
package eventlog

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/9elements/tpmtool/pkg/tpm"
	slaunch "github.com/u-root/u-root/pkg/securelaunch"
)

/* describes the "eventlog" section of policy file */
type EventLog struct {
	Type     string `json:"type"`
	Location string `json:"location"`
}

const (
	eventLogFile        = "/sys/kernel/security/slaunch/eventlog"
	defaultEventLogFile = "eventlog.txt" //only used if user doesn't provide any
)

// Add writes event logs to sysfs file.
func Add(b []byte) error {
	fd, err := os.OpenFile(eventLogFile, os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	defer fd.Close()
	// TODO: writing to sysfs always throws an EOF error but still works
	fd.Write(b)
	return nil
}

/*
 * parseEvtlog uses tpmtool package to parse the event logs generated by a
 * kernel with CONFIG_SECURE_LAUNCH enabled and returns the parsed data in a byte slice.
 *
 * these event logs are originally in binary format and need to be parsed into human readable
 * format. error is returned if parsing code fails in tpmtool.
 */
func parseEvtLog(evtLogFile string) ([]byte, error) {
	tpm.DefaultTCPABinaryLog = evtLogFile
	firmware := tpm.Txt
	TPMSpecVersion := tpm.TPM20
	tcpaLog, err := tpm.ParseLog(firmware, TPMSpecVersion)
	if err != nil {
		return nil, err
	}

	var w strings.Builder
	for _, pcr := range tcpaLog.PcrList {
		fmt.Fprintf(&w, "%s\n", pcr)
		fmt.Fprintf(&w, "\n")
	}
	return []byte(w.String()), nil
}

/*
 * Persist uses tpmtool to parse event logs generated by the kernel into
 * human readable format, mounts the target disk and store the result in the file on
 * target disk.
 *
 * The location of the file on disk is specified in policy file by Location tag.
 * returns
 * - error if mounting the disk fails __OR_ writing to location on disk fails.
 */
func (e *EventLog) Persist() error {
	if e.Type != "file" {
		return fmt.Errorf("unsupported eventlog type exiting")
	}

	slaunch.Debug("Identified EventLog Type = file")

	// e.Location is of the form sda:path/to/file.txt
	eventlogPath := e.Location
	if eventlogPath == "" {
		return fmt.Errorf("empty eventlog path provided exiting")
	}

	filePath, r := slaunch.GetMountedFilePath(eventlogPath, 0) // 0 is flag value for rw mount option
	if r != nil {
		return fmt.Errorf("failed to mount target disk for target=%s, err=%v", eventlogPath, r)
	}

	dst := filePath // /tmp/boot-733276578/evtlog

	// parse eventlog
	data, err := parseEvtLog(eventLogFile)
	if err != nil {
		log.Printf("tpmtool could NOT parse Eventlogfile=%s, err=%s", eventLogFile, err)
		return fmt.Errorf("parseEvtLog err=%v", err)
	}

	// write parsed data onto disk
	target, err := slaunch.WriteToFile(data, dst, defaultEventLogFile)
	if err != nil {
		log.Printf("EventLog: Write err=%v, dst=%s, exiting", err, dst)
		return fmt.Errorf("failed to write parsed eventLog to disk, err=%v, dst=%s, exiting", err, dst)
	}

	slaunch.Debug("EventLog: success, data written to %s", target)
	return nil
}
