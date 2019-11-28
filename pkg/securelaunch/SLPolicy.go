package securelaunch

import (
	"encoding/json"
	"errors"
	"github.com/u-root/u-root/pkg/diskboot"
	"github.com/u-root/u-root/pkg/mount"
	"github.com/u-root/u-root/pkg/securelaunch/measurement"
	"io"
	"log"
	"os"
	"os/exec"
	// "time"v
	// "github.com/u-root/iscsinl"
)

type launcher struct {
	Type string `json:"type"`
	// Cmd    string   `json:"cmd"`
	Params map[string]string `json:"params"`
}

type policy struct {
	DefaultAction string
	Collectors    []measurement.Collector
	Launcher      launcher
}

func (l *launcher) Boot(t io.ReadWriter) {

	if l.Type != "kexec" {
		log.Printf("Launcher: Unsupported launcher type. Exiting.\n")
		return
	}
	log.Printf("Identified Launcher Type = Kexec\n")
	// TODO: if kernel and initrd are on different devices.
	kernel := l.Params["kernel"]
	initrd := l.Params["initrd"]
	cmdline := l.Params["cmdline"]

	log.Printf("********Step 6: Measuring kernel, initrd ********\n")
	if e := measurement.MeasureInputFile(t, kernel); e != nil {
		log.Printf("Launcher: ERR: measure kernel input=%s, err=%v\n", kernel, e)
		return
	}

	if e := measurement.MeasureInputFile(t, initrd); e != nil {
		log.Printf("Launcher: ERR: measure initrd input=%s, err=%v\n", initrd, e)
		return
	}

	k, e := GetMountedFilePath(kernel)
	if e != nil {
		log.Printf("Launcher: ERR: kernel input %s couldnt be located, err=%v\n", kernel, e)
		return
	}

	i, e := GetMountedFilePath(initrd)
	if e != nil {
		log.Printf("Launcher: ERR: initrd input %s couldnt be located, err=%v\n", initrd, e)
		return
	}

	log.Printf("********Step 7: kexec called  ********\n")
	//i := "initramfs-4.14.35-builtin-no-embedded+.img"
	//k := "vmlinuz-4.14.35-builtin-no-embedded-signed+"
	//cmdline := "console=ttyS0,115200n8 BOOT_IMAGE=/vmlinuz-4.14.35-builtin-no-embedded-signed+ root=/dev/mapper/ol-root ro crashkernel=auto netroot=iscsi:@10.196.210.62::3260::iqn.1986-03.com.sun:ovs112-boot rd.iscsi.initiator=iqn.1988-12.com.oracle:ovs112 netroot=iscsi:@10.196.210.64::3260::iqn.1986-03.com.sun:ovs112-boot rd.lvm.lv=ol/root rd.lvm.lv=ol/swap  numa=off transparent_hugepage=never LANG=en_US.UTF-8"

	// log.Printf("running command : kexec -s --initrd %s --command-line %s kernel=[%s]\n", i, cmdline, k)
	boot := exec.Command("kexec", "-l", "-s", "--initrd", i, "--command-line", cmdline, k)

	// boot := exec.Command("kexec", "-s", "-i", i, "-l", k, "-c", cmdline) /* this is u-root's kexec */

	boot.Stdin = os.Stdin
	boot.Stderr = os.Stderr
	boot.Stdout = os.Stdout
	if err := boot.Run(); err != nil {
		//need to decide how to bail, reboot, error msg & halt, or recovery shell
		log.Printf("command finished with error: %v\n", err)
		os.Exit(1)
	}
	//sudo sync; sudo umount -a; sudo kexec -e
	boot = exec.Command("kexec", "-e")
	if err := boot.Run(); err != nil {
		//need to decide how to bail, reboot, error msg & halt, or recovery shell
		log.Printf("command finished with error: %v\n", err)
		os.Exit(1)
	}
}

/*locateSLPolicy() ([]byte, error)
Check of kernel param sl_policy is set,
	- parse the string
Iterate through each local block device,
	- mount the block device
	- scan for securelaunch.policy under /, /efi, or /boot
Read in policy file */
func LocateSLPolicy() ([]byte, error) {

	d, err := scanKernelCmdLine()
	if err == nil || err.Error() != "Flag Not Set" {
		return d, err
	}

	log.Printf("Searching and mounting block devices with bootable configs\n")
	blkDevices := diskboot.FindDevices("/sys/class/block/*") // FindDevices find and *mounts* the devices.
	if len(blkDevices) == 0 {
		return nil, errors.New("No block devices found")
	}

	for _, device := range blkDevices {
		devicePath, mountPath := device.DevPath, device.MountPath
		log.Printf("scanning for policy file under devicePath=%s, mountPath=%s\n", devicePath, mountPath)
		raw, found := scanBlockDevice(mountPath)
		if e := mount.Unmount(mountPath, true, false); e != nil {
			log.Printf("Unmount failed. PANIC\n")
			panic(e)
		}

		if !found {
			log.Printf("no policy file found under this device\n")
			continue
		}

		log.Printf("policy file found at devicePath=%s\n", devicePath)
		return raw, nil
	}

	return nil, errors.New("policy file not found anywhere.")
}

func ParseSLPolicy(pf []byte) (*policy, error) {
	p := &policy{}
	var parse struct {
		DefaultAction string            `json:"default_action"`
		Collectors    []json.RawMessage `json:"collectors"`
		Attestor      json.RawMessage   `json:"attestor"`
		Launcher      json.RawMessage   `json:"launcher"`
	}

	if err := json.Unmarshal(pf, &parse); err != nil {
		log.Printf("parseSLPolicy: Unmarshall error for entire policy file!! err=%v\n", err)
		return nil, err
	}

	p.DefaultAction = parse.DefaultAction

	for _, c := range parse.Collectors {
		collector, err := measurement.GetCollector(c)
		if err != nil {
			log.Printf("getCollector failed for c=%s, collector=%v\n", c, collector)
			return nil, err
		}
		p.Collectors = append(p.Collectors, collector)
	}

	// log.Printf("len(parse.Launcher)=%d, parse.Launcher=%s\n", len(parse.Launcher), parse.Launcher)
	if len(parse.Launcher) > 0 {
		if err := json.Unmarshal(parse.Launcher, &p.Launcher); err != nil {
			log.Printf("parseSLPolicy: Launcher Unmarshall error=%v!!\n", err)
			return nil, err
		}
	}
	return p, nil
}
