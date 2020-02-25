// Copyright 2019 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package eventlog parses kernel event logs and saves the parsed data on a file on disk.
package eventlog

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/9elements/tpmtool/pkg/tpm"
	// "github.com/u-root/u-root/pkg/mount"
	slaunch "github.com/u-root/u-root/pkg/securelaunch"
)

// EventLog stores location for dumping event logs on disk.
type EventLog struct {
	Type     string `json:"type"`
	Location string `json:"location"`
}

const (
	eventLogFile        = "/sys/kernel/security/slaunch/eventlog"
	defaultEventLogFile = "eventlog.txt" //only used if user doesn't provide any
)

// add to eventlog via sysfs
func Add(b []byte) error {
	// fd, err := os.OpenFile(eventLogFile, os.O_APPEND|os.O_WRONLY, 0644)
	fd, err := os.OpenFile(eventLogFile, os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	defer fd.Close()
	fi, err := fd.Stat()
	if err != nil {
		return err
	}
	log.Printf("The file %s is %d bytes long", eventLogFile, fi.Size())

	_, err = fd.Write(b)
	if err != nil && err != io.EOF {
		return err
	}
	slaunch.Debug("err = %v", err)
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

// Parse uses tpmtool to parse event logs generated by the kernel into
// human readable format, and queues the data to persist queue .
//
// The location of the file on disk is specified in policy file by Location tag.
// returns
// - error if parsing eventlog fails or user enters incorrect format for input.
func (e *EventLog) Parse() error {

	if e.Type != "file" {
		return fmt.Errorf("unsupported eventlog type exiting")
	}

	slaunch.Debug("Identified EventLog Type = file")

	// e.Location is of the form sda:path/to/file.txt
	eventlogPath := e.Location
	if eventlogPath == "" {
		return fmt.Errorf("empty eventlog path provided exiting")
	}

	// parse eventlog
	data, err := parseEvtLog(eventLogFile)
	if err != nil {
		log.Printf("tpmtool could NOT parse Eventlogfile=%s, err=%s", eventLogFile, err)
		return fmt.Errorf("parseEvtLog err=%v", err)
	}

	return slaunch.AddToPersistQueue("EventLog:", data, eventlogPath, defaultEventLogFile)
}

///*
// * Persist uses tpmtool to parse event logs generated by the kernel into
// * human readable format, mounts the target disk and store the result in the file on
// * target disk.
// *
// * The location of the file on disk is specified in policy file by Location tag.
// * returns
// * - error if mounting the disk fails __OR_ writing to location on disk fails.
// */
//func (e *EventLog) Persist() error {
//
//	if e.Type != "file" {
//		return fmt.Errorf("unsupported eventlog type exiting")
//	}
//
//	slaunch.Debug("Identified EventLog Type = file")
//
//	// e.Location is of the form sda:path/to/file.txt
//	eventlogPath := e.Location
//	if eventlogPath == "" {
//		return fmt.Errorf("empty eventlog path provided exiting")
//	}
//
//	// filePath, mp, r := slaunch.GetMountedFilePath(eventlogPath, 0) // 0 is flag value for rw mount option
//	// filePath, mountPath, r := slaunch.GetMountedFilePath(eventlogPath, 0) // 0 is flag value for rw mount option
//	filePath, _, r := slaunch.GetMountedFilePath(eventlogPath, 0) // 0 is flag value for rw mount option
//	if r != nil {
//		return fmt.Errorf("failed to mount target disk for target=%s, err=%v", eventlogPath, r)
//	}
//
//	dst := filePath // /tmp/boot-733276578/evtlog
//
//	// parse eventlog
//	data, err := parseEvtLog(eventLogFile)
//	if err != nil {
//		log.Printf("tpmtool could NOT parse Eventlogfile=%s, err=%s", eventLogFile, err)
//		//		if ret := mount.Unmount(mountPath, true, false); ret != nil {
//		//			log.Printf("Unmount failed. PANIC")
//		//			panic(ret)
//		//		}
//		//if e := mp.Unmount(mount.MNT_DETACH); e != nil {
//		//	log.Printf("Failed to unmount %v: %v", mp, e)
//		//	panic(e)
//		//}
//		return fmt.Errorf("parseEvtLog err=%v", err)
//	}
//
//	// write parsed data onto disk
//	target, err := slaunch.WriteToFile(data, dst, defaultEventLogFile)
//	//	if ret := mount.Unmount(mountPath, true, false); ret != nil {
//	//		log.Printf("Unmount failed. PANIC")
//	//		panic(ret)
//	//	}
//
//	//if e := mp.Unmount(mount.MNT_DETACH); e != nil {
//	//	log.Printf("Failed to unmount %v: %v", mp, e)
//	//	panic(e)
//	//}
//
//	if err != nil {
//		log.Printf("EventLog: Write err=%v, dst=%s, exiting", err, dst)
//		return fmt.Errorf("failed to write parsed eventLog to disk, err=%v, dst=%s, exiting", err, dst)
//	}
//
//	slaunch.Debug("EventLog: success, data written to %s", target)
//	return nil
//}

func (e *EventLog) Temp() error {
	if e.Type != "file" {
		return fmt.Errorf("unsupported eventlog type exiting")
	}

	slaunch.Debug("Identified EventLog Type = file")

	// read /sysfs file
	data, err := ioutil.ReadFile(eventLogFile)
	if err != nil {
		log.Printf("err=%v", err)
		return err
	}

	return slaunch.AddToPersistQueue("Raw EventLogs", data, "sda2:/Daniel", "Daniel")
}

//func (e *EventLog) Temp() error {
//
//	if e.Type != "file" {
//		return fmt.Errorf("unsupported eventlog type exiting")
//	}
//
//	slaunch.Debug("Identified EventLog Type = file")
//
//	// e.Location is of the form sda:path/to/file.txt
//	// eventlogPath := "sda1:/Daniel" // qemu
//	eventlogPath := "sda2:/Daniel"
//	// eventlogPath := "65cfe18c-4f9b-402b-9ea6-4c68c856546e:/Daniel" // ovs112
//	// eventlogPath := "4fbbe726-80cb-4e66-9ce7-1199cb4299c4:/Daniel" // qemu loop device
//	if eventlogPath == "" {
//		return fmt.Errorf("empty eventlog path provided exiting")
//	}
//
//	// filePath, mountPath, r := slaunch.GetMountedFilePath(eventlogPath, 0) // 0 is flag value for rw mount option
//	filePath, _, r := slaunch.GetMountedFilePath(eventlogPath, 0) // 0 is flag value for rw mount option
//	if r != nil {
//		return fmt.Errorf("GetMountedFilePath: target=%s, err=%v", eventlogPath, r)
//	}
//
//	dst := filePath // /tmp/boot-733276578/evtlog
//	w, err := os.Create(dst)
//	if err != nil {
//		return err
//	}
//
//	f, err := os.Open(eventLogFile)
//	if err != nil {
//		return err
//	}
//
//	_, err = io.Copy(w, f)
//	w.Close()
//	f.Close()
//
//	/*
//		// read /sysfs file
//		data, err := ioutil.ReadFile(eventLogFile)
//		if err != nil {
//			log.Printf("err=%v", err)
//			return err
//		}
//
//		// write parsed data onto disk
//		target, err := slaunch.WriteToFile(data, dst, "simran.txt")
//	*/
//
//	//	if ret := mount.Unmount(mountPath, true, false); ret != nil {
//	//		log.Printf("Unmount failed. PANIC")
//	//		panic(ret)
//	//	}
//	//if e := mp.Unmount(mount.MNT_DETACH); e != nil {
//	//	log.Printf("Failed to unmount %v: %v", mp, e)
//	//	panic(e)
//	//}
//
//	if err != nil {
//		log.Printf("EventLog: Write err=%v, dst=%s, exiting", err, dst)
//		return fmt.Errorf("failed to write raw eventLog to disk, err=%v, dst=%s, exiting", err, dst)
//	}
//
//	slaunch.Debug("EventLog: success, data written to %s", dst)
//	// slaunch.Debug("EventLog: success, data written to %s", target)
//	return nil
//}
