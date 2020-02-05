// Copyright 2019 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package launcher boots the target kernel.
package launcher

import (
	"fmt"
	"io"
	"log"
	//"os"
	//"os/exec"

	"github.com/u-root/u-root/pkg/boot"
	"github.com/u-root/u-root/pkg/boot/kexec"
	// cmd "github.com/u-root/u-root/pkg/cmdline"
	"github.com/u-root/u-root/pkg/mount"
	slaunch "github.com/u-root/u-root/pkg/securelaunch"
	"github.com/u-root/u-root/pkg/securelaunch/measurement"
	"github.com/u-root/u-root/pkg/uio"
)

/* describes the "launcher" section of policy file */
type Launcher struct {
	Type   string            `json:"type"`
	Params map[string]string `json:"params"`
}

// we dont want a separate function for measuring hash files
// because we want to prevent TOCTOU error..time of check to time of use error.
// we want to check the file close to the kexec load operation,
// so keeping it in the same function as Load helps us prevent TOCTOU error..
func (l *Launcher) MeasureKernel(tpmDev io.ReadWriteCloser) error {

	kernel := l.Params["kernel"]
	initrd := l.Params["initrd"]

	if e := measurement.HashFile(tpmDev, kernel); e != nil {
		log.Printf("ERR: measure kernel input=%s, err=%v", kernel, e)
		return e
	}

	if e := measurement.HashFile(tpmDev, initrd); e != nil {
		log.Printf("ERR: measure initrd input=%s, err=%v", initrd, e)
		return e
	}
	return nil
}

/*
 * // Load loads the target kernel and initrd based on information provided
 * Boot boots the target kernel based on information provided
 * in the "launcher" section of policy file.
 *
 * Summary of steps:
 * - extracts the kernel, initrd and cmdline from the "launcher" section of policy file.
 * - measures the kernel and initrd file into the tpmDev (tpm device).
 * - mounts the disks where the kernel and initrd file are located.
 * returns error
 * - if measurement of kernel and initrd fails
 * - if mount fails
 * - if kexec fails
 */
// func (l *Launcher) Load(tpmDev io.ReadWriteCloser) error {
func (l *Launcher) Boot(tpmDev io.ReadWriteCloser) error {

	if l.Type != "kexec" {
		log.Printf("launcher: Unsupported launcher type. Exiting.")
		return fmt.Errorf("launcher: Unsupported launcher type. Exiting")
	}

	slaunch.Debug("Identified Launcher Type = Kexec")

	// TODO: if kernel and initrd are on different devices.
	kernel := l.Params["kernel"]
	initrd := l.Params["initrd"]
	cmdline := l.Params["cmdline"]

	// slaunch.Debug("********Step 6: Measuring kernel, initrd ********") all steps moved to sluinit.
	//	if e := measurement.HashFile(tpmDev, kernel); e != nil {
	//		log.Printf("ERR: measure kernel input=%s, err=%v", kernel, e)
	//		return e
	//	}
	//
	//	if e := measurement.HashFile(tpmDev, initrd); e != nil {
	//		log.Printf("ERR: measure initrd input=%s, err=%v", initrd, e)
	//		return e
	//	}

	k, kMountPath, e := slaunch.GetMountedFilePath(kernel, mount.MS_RDONLY)
	if e != nil {
		log.Printf("launcher: ERR: kernel input %s couldnt be located, err=%v", kernel, e)
		return e
	}

	i, iMountPath, e := slaunch.GetMountedFilePath(initrd, mount.MS_RDONLY)
	if e != nil {
		log.Printf("launcher: ERR: initrd input %s couldnt be located, err=%v", initrd, e)
		return e
	}

	// slaunch.Debug("********Step 7: kexec called  ********") No need for this anymore
	// since sluinit has all the steps..
	/*
		var err error
		// OL-kexec=1 use OL's kexec
		// OL-kexec=2 use u-root's inhouse but with verbose set to true.
		val, ok := cmd.Flag("OL-kexec")
		if ok {
			if val == "1" {
				// log.Printf("running command : kexec -s --initrd %s --command-line %s kernel=[%s]\n", i, cmdline, k)
				boot := exec.Command("kexec", "-l", "-s", "--initrd", i, "--command-line", cmdline, k)
				// boot := exec.Command("kexec", "-s", "-i", i, "-l", k, "-c", cmdline) // this is u-root's kexec //
				boot.Stdin = os.Stdin
				boot.Stderr = os.Stderr
				boot.Stdout = os.Stdout
				if err = boot.Run(); err != nil {
					//need to decide how to bail, reboot, error msg & halt, or
					//recovery shell
					log.Printf("command finished with error: %v\n", err)
					goto ERR
				}
				//sudo sync; sudo umount -a; sudo kexec -e
				boot = exec.Command("kexec", "-e")
				if err = boot.Run(); err != nil {
					//need to decide how to bail, reboot, error msg & halt, or
					//recovery shell
					log.Printf("command finished with error: %v\n", err)
					goto ERR
				}
			} else if val == "2" {
				image := &boot.LinuxImage{
					Kernel:  uio.NewLazyFile(k),
					Initrd:  uio.NewLazyFile(i),
					Cmdline: cmdline,
				}
				if err = image.Load(true); err != nil {
					log.Printf("kexec -l failed. err: %v", err)
					goto ERR
				}

				err = kexec.Reboot()
				if err != nil {
					log.Printf("kexec reboot failed. err=%v", err)
					goto ERR
				}
			}
		} else {
			image := &boot.LinuxImage{
				Kernel:  uio.NewLazyFile(k),
				Initrd:  uio.NewLazyFile(i),
				Cmdline: cmdline,
			}
			if err = image.Load(false); err != nil {
				log.Printf("kexec -l failed. err: %v", err)
				goto ERR
			}

			err = kexec.Reboot()
			if err != nil {
				log.Printf("kexec reboot failed. err=%v", err)
				goto ERR
			}
		}*/

	image := &boot.LinuxImage{
		Kernel:  uio.NewLazyFile(k),
		Initrd:  uio.NewLazyFile(i),
		Cmdline: cmdline,
	}
	err := image.Load(false)
	if err != nil {
		log.Printf("kexec -l failed. err: %v", err)
		goto ERR
		// return err
	}

	err = kexec.Reboot()
	if err != nil {
		log.Printf("kexec reboot failed. err=%v", err)
		goto ERR
	}
	return nil
ERR:
	// On error, unmount the files on disk holding kernel and initrd files
	if e := mount.Unmount(kMountPath, true, false); e != nil {
		log.Printf("Unmount failed. PANIC")
		panic(e)
	}
	if e := mount.Unmount(iMountPath, true, false); e != nil {
		log.Printf("Unmount failed. PANIC")
		panic(e)
	}
	return err
}

// Boot uses kexec to boot into the target kernel.
//func (l *Launcher) Boot(tpmDev io.ReadWriteCloser) error {
//
//	if l.Type != "kexec" {
//		log.Printf("launcher: Unsupported launcher type. Exiting.")
//		return fmt.Errorf("launcher: Unsupported launcher type. Exiting")
//	}
//
//	err := kexec.Reboot()
//	if err != nil {
//		log.Printf("kexec reboot failed. err=%v", err)
//		return err
//	}
//	return nil
//}
