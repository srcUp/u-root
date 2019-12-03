package launcher

import (
	"fmt"
	"io"
	"log"

	"github.com/u-root/u-root/pkg/boot"
	"github.com/u-root/u-root/pkg/kexec"
	slaunch "github.com/u-root/u-root/pkg/securelaunch"
	"github.com/u-root/u-root/pkg/securelaunch/measurement"
	"github.com/u-root/u-root/pkg/uio"
)

/* describes the "launcher" section of policy file */
type Launcher struct {
	Type   string            `json:"type"`
	Params map[string]string `json:"params"`
}

/*
 * This function boots the target kernel based on information provided
 * in the "launcher" section of policy file.
 * Boot fn,
 * - extracts the kernel, initrd and cmdline from the "launcher" section of policy file.
 * - measures the kernel and initrd file into the tpmDev (tpm device).
 * - mounts the disks where the kernel and initrd file are located.
 * - uses kexec to boot into the target kernel.
 * returns error
 * - if measurement of kernel and initrd fails
 * - if mount fails
 * - if kexec fails
 */
func (l *Launcher) Boot(tpmDev io.ReadWriteCloser) error {

	if l.Type != "kexec" {
		log.Printf("launcher: Unsupported launcher type. Exiting.")
		return fmt.Errorf("launcher: Unsupported launcher type. Exiting")
	}

	slaunch.Debug("Identified Launcher Type = Kexec\n")

	// TODO: if kernel and initrd are on different devices.
	kernel := l.Params["kernel"]
	initrd := l.Params["initrd"]
	cmdline := l.Params["cmdline"]

	slaunch.Debug("********Step 6: Measuring kernel, initrd ********\n")
	if e := measurement.HashFile(tpmDev, kernel); e != nil {
		log.Printf("launcher: ERR: measure kernel input=%s, err=%v", kernel, e)
		return e
	}

	if e := measurement.HashFile(tpmDev, initrd); e != nil {
		log.Printf("launcher: ERR: measure initrd input=%s, err=%v", initrd, e)
		return e
	}

	k, _, e := slaunch.GetMountedFilePath(kernel, false) // false=read only mount option
	if e != nil {
		log.Printf("launcher: ERR: kernel input %s couldnt be located, err=%v", kernel, e)
		return e
	}

	i, _, e := slaunch.GetMountedFilePath(initrd, false) // false=read only mount option
	if e != nil {
		log.Printf("launcher: ERR: initrd input %s couldnt be located, err=%v", initrd, e)
		return e
	}

	slaunch.Debug("********Step 7: kexec called  ********")
	image := &boot.LinuxImage{
		Kernel:  uio.NewLazyFile(k),
		Initrd:  uio.NewLazyFile(i),
		Cmdline: cmdline,
	}
	if err := image.Load(false); err != nil {
		log.Printf("kexec -l failed. err: %v", err)
		return err
	}

	err := kexec.Reboot()
	if err != nil {
		log.Printf("kexec reboot failed. err=%v", err)
	}
	return nil
}
