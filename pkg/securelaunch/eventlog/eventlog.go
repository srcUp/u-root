package eventlog

import (
	"fmt"
	"github.com/9elements/tpmtool/pkg/tpm"
	"github.com/u-root/u-root/pkg/mount"
	slaunch "github.com/u-root/u-root/pkg/securelaunch"
	"log"
	"strings"
)

type EventLog struct {
	Type     string `json:"type"`
	Location string `json:"location"`
}

const (
	eventLogFile        = "/sys/kernel/security/slaunch/eventlog"
	defaultEventLogFile = "eventlog.txt" //only used if user doesn't provide any
)

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

// tpmtool parsed data --> written to --> eventlogPath
func (e *EventLog) Persist() error {

	if e.Type != "file" {
		return fmt.Errorf("EventLog: Unsupported eventlog type. Exiting.")
	}

	slaunch.Debug("Identified EventLog Type = file")

	// e.Location is of the form sda:path/to/file.txt
	eventlogPath := e.Location
	if eventlogPath == "" {
		return fmt.Errorf("EventLog: Empty eventlog path. Exiting.")
	}

	filePath, mountPath, r := slaunch.GetMountedFilePath(eventlogPath, true) // true = rw mount option
	if r != nil {
		return fmt.Errorf("EventLog: ERR: input %s could NOT be located, err=%v", eventlogPath, r)
	}

	dst := filePath // /tmp/boot-733276578/evtlog

	// parse eventlog
	data, err := parseEvtLog(eventLogFile)
	if err != nil {
		log.Printf("tpmtool could NOT parse Eventlogfile=%s, err=%s", eventLogFile, err)
		return fmt.Errorf("EventLog(): Persist() err=%v", err)
	}

	// write parsed data onto disk
	target, err := slaunch.WriteToFile(data, dst, defaultEventLogFile)
	if ret := mount.Unmount(mountPath, true, false); ret != nil {
		log.Printf("Unmount failed. PANIC")
		panic(ret)
	}

	if err != nil {
		log.Printf("EventLog: Write err=%v, dst=%s, exiting", err, dst)
		return fmt.Errorf("EventLog: Write err=%v, dst=%s, exiting", err, dst)
	}

	slaunch.Debug("EventLog: success, data written to %s", target)
	return nil
}
