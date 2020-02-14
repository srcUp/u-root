// Copyright 2019 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package securelaunch takes integrity measurements before launching the target system.
package securelaunch

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/u-root/u-root/pkg/mount"
	"github.com/u-root/u-root/pkg/storage"
)

type persistDataItem struct {
	// callback func([]byte, string) error
	desc        string // Description
	data        []byte
	location    string // of form sda:/path/to/file
	defaultFile string // if location turns out to be dir only
}

var persistData []persistDataItem

/* used to store all block devices returned from a call to storage.GetBlockStats */
var StorageBlkDevices []storage.BlockDev

// sda2-0 --> /tmp/foo
// device Name-flag --> mountPath
// var mountCache = make(map[string]string)

type mountCacheData struct {
	flags     uintptr
	mountPath string
}

// Rev 2: device Name --> []{ flags, mountPath }
type mountCacheType struct {
	m  map[string]mountCacheData
	mu sync.RWMutex
}

var mountCache = mountCacheType{m: make(map[string]mountCacheData)}

/*
 * if kernel cmd line has uroot.uinitargs=-d, debug fn is enabled.
 * kernel cmdline is checked in sluinit.
 */
var Debug = func(string, ...interface{}) {}

/*
 * WriteToFile writes a byte slice to a target file on an
 * already mounted disk and returns the target file path.
 *
 * defFileName is default dst file name, only used if user doesn't provide one.
 */
func WriteToFile(data []byte, dst, defFileName string) (string, error) {

	// make sure dst is an absolute file path
	if !filepath.IsAbs(dst) {
		return "", fmt.Errorf("dst =%s Not an absolute path ", dst)
	}

	// target is the full absolute path where []byte will be written to
	target := dst
	dstInfo, err := os.Stat(dst)
	if err == nil && dstInfo.IsDir() {
		Debug("No file name provided. Adding it now. old target=%s", target)
		target = filepath.Join(dst, defFileName)
		Debug("New target=%s", target)
	}

	Debug("WriteToFile: target=%s", target)
	err = ioutil.WriteFile(target, data, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write date to file =%s, err=%v", target, err)
	}
	Debug("WriteToFile: exit w success data written to target=%s", target)
	return target, nil
}

// TargetPath is of form sda:/boot/cpuid.txt
func persist(data []byte, targetPath string, defaultFile string) error {

	filePath, _, r := GetMountedFilePath(targetPath, 0) // 0 is flag for rw mount option
	if r != nil {
		return fmt.Errorf("persist: err: input %s could NOT be located, err=%v", targetPath, r)
	}

	dst := filePath // /tmp/boot-733276578/cpuid

	target, err := WriteToFile(data, dst, defaultFile)
	if err != nil {
		log.Printf("persist: err=%s", err)
		return err
	}

	Debug("persist: Target File%s", target)
	return nil
}

func AddToPersistQueue(desc string, data []byte, location string, defFile string) error {
	persistData = append(persistData, persistDataItem{desc, data, location, defFile})
	return nil
}

func ClearPersistQueue() error {

	for _, entry := range persistData {
		if err := persist(entry.data, entry.location, entry.defaultFile); err != nil {
			return fmt.Errorf("%s: persist failed for location %s", entry.desc, entry.location)
		}
	}
	return nil
}

func getDeviceFromUUID(uuid string) (storage.BlockDev, error) {
	if e := GetBlkInfo(); e != nil {
		return storage.BlockDev{}, fmt.Errorf("GetBlkInfo err=%s", e)
	}
	devices := storage.PartitionsByFsUUID(StorageBlkDevices, uuid) // []BlockDev
	Debug("%d device(s) matched with UUID=%s", len(devices), uuid)
	for i, d := range devices {
		Debug("No#%d ,device=%s with fsUUID=%s", i, d.Name, d.FsUUID)
		return d, nil // return first device found
	}
	return storage.BlockDev{}, fmt.Errorf("no block device exists with UUID=%s", uuid)
}

/*
// func GetDevPathFromUUID(uuid string) (string, error) {
func GetDeviceFromUUID(uuid string) (storage.BlockDev, error) {
	if e := GetBlkInfo(); e != nil {
		return storage.BlockDev{}, fmt.Errorf("GetBlkInfo err=%s", e)
		// return "", fmt.Errorf("GetBlkInfo err=%s", e)
		// return "", "", fmt.Errorf("GetBlkInfo err=%s", e)
		// return "", nil, fmt.Errorf("GetBlkInfo err=%s", e)
	}
	devices := storage.PartitionsByFsUUID(StorageBlkDevices, uuid) // []BlockDev
	//if len(devices) == 0 {
	// return nil, fmt.Errorf("no block device exists with UUID=%s", uuid)
	// return "", "", fmt.Errorf("no block device exists with UUID=%s", uuid)
	// return "", nil, fmt.Errorf("no block device exists with UUID=%s", s[0])
	// }
	Debug("%d device(s) matched with UUID=%s", len(devices), uuid)
	for i, d := range devices {
		Debug("No#%d ,device=%s with fsUUID=%s", i, d.Name, d.FsUUID)
		// device = d
		// devPath := filepath.Join("/dev", d.Name)
		// return devPath, nil // return first device found
		return d, nil // return first device found
	}
	return storage.BlockDev{}, fmt.Errorf("no block device exists with UUID=%s", uuid)
	// return "", fmt.Errorf("no block device exists with UUID=%s", uuid)
}
*/

func getDeviceFromName(name string) (storage.BlockDev, error) {
	if e := GetBlkInfo(); e != nil {
		return storage.BlockDev{}, fmt.Errorf("GetBlkInfo err=%s", e)
	}
	devices := storage.PartitionsByName(StorageBlkDevices, name) // []BlockDev
	Debug("%d device(s) matched with Name=%s", len(devices), name)
	for i, d := range devices {
		Debug("No#%d ,device=%s with fsUUID=%s", i, d.Name, d.FsUUID)
		return d, nil // return first device found
	}
	return storage.BlockDev{}, fmt.Errorf("no block device exists with name=%s", name)
}

func GetStorageDevice(input string) (storage.BlockDev, error) {
	device, e := getDeviceFromUUID(input)
	if e != nil {
		// return storage.BlockDev{}, fmt.Errorf("fn getDeviceFromUUID: err = %v", e)
		d2, e2 := getDeviceFromName(input)
		if e2 != nil {
			return storage.BlockDev{}, fmt.Errorf("getDeviceFromUUID: err=%v, getDeviceFromName: err=%v", e, e2)
		}
		device = d2
	}
	return device, nil
}

/*
// func getDevPathFromName(name string) (string, error) {
func getDeviceFromName(name string) (storage.BlockDev, error) {
	if e := GetBlkInfo(); e != nil {
		return storage.BlockDev{}, fmt.Errorf("GetBlkInfo err=%s", e)
		// return "", fmt.Errorf("GetBlkInfo err=%s", e)
		// return "", "", fmt.Errorf("GetBlkInfo err=%s", e)
		// return "", nil, fmt.Errorf("GetBlkInfo err=%s", e)
	}
	devices := storage.PartitionsByName(StorageBlkDevices, name) // []BlockDev
	//if len(devices) == 0 {
	// return nil, fmt.Errorf("no block device exists with UUID=%s", uuid)
	// return "", "", fmt.Errorf("no block device exists with UUID=%s", uuid)
	// return "", nil, fmt.Errorf("no block device exists with UUID=%s", s[0])
	// }
	Debug("%d device(s) matched with Name=%s", len(devices), name)
	for i, d := range devices {
		Debug("No#%d ,device=%s with fsUUID=%s", i, d.Name, d.FsUUID)
		// device = d
		// devPath := filepath.Join("/dev", d.Name)
		// return devPath, nil // return first device found
		return d, nil // return first device found
	}
	// return "", fmt.Errorf("no block device exists with name=%s", name)
	return storage.BlockDev{}, fmt.Errorf("no block device exists with name=%s", name)
}
*/
func deleteEntryMountCache(key string) {
	mountCache.mu.Lock()
	delete(mountCache.m, key)
	mountCache.mu.Unlock()

	Debug("mountCache: Deleted key %s", key)
}

func setMountCache(key string, val mountCacheData) {

	mountCache.mu.Lock()
	mountCache.m[key] = val
	mountCache.mu.Unlock()

	Debug("mountCache: Updated key %s, value %v", key, val)
}

// getMountCacheData looks up mountCache using devName as key
// and clears an entry in cache if result is found with different
// flags, otherwise returns the cached entry or nil.
func getMountCacheData(key string, flags uintptr) (string, error) {
	// key := fmt.Sprintf("%s-%d", devName, flags)
	// Debug("Lookup mountCache with key = %s", key)
	Debug("mountCache: Lookup with key %s", key)
	cachedData, ok := mountCache.m[key]
	if ok {
		cachedMountPath := cachedData.mountPath
		cachedFlags := cachedData.flags
		Debug("mountCache: Lookup succeeded: cachedMountPath %s, cachedFlags %d found for key %s", cachedMountPath, cachedFlags, key)
		if cachedFlags == flags {
			return cachedMountPath, nil
		}
		Debug("mountCache: flags mismatch in cache and input !!")
		Debug("mountCache: need to mount the same device with different flags")
		Debug("mountCache: lets unmount this device first. Unmounting %s", cachedMountPath)
		if e := mount.Unmount(cachedMountPath, true, false); e != nil {
			log.Printf("Unmount failed for %s. PANIC")
			panic(e)
		}
		Debug("mountCache: unmount successfull. lets clean up it's entry in map")
		deleteEntryMountCache(key)
		// setMountCache(key, mountCacheData{})
		Debug("mountCache: declare this as a failed lookup since we need to mount again")
		return "", fmt.Errorf("device was already mounted. Mount again.")
	}

	return "", fmt.Errorf("Lookup mountCache failed: No key exists that matches %s", key)
}

func MountDevice(device storage.BlockDev, flags uintptr) (string, error) {

	devName := device.Name

	Debug("MountDevice: Checking cache first for %s", devName)
	cachedMountPath, err := getMountCacheData(devName, flags)
	if err == nil {
		log.Printf("getMountCacheData succeeded for %s", devName)
		return cachedMountPath, nil
	}
	Debug("MountDevice: cache lookup failed for %s", devName)

	Debug("MountDevice: Attempting to mount %s with flags %d", devName, flags)
	mountPath, err := ioutil.TempDir("/tmp", "slaunch-")
	if err != nil {
		return "", fmt.Errorf("failed to create tmp mount directory: %v", err)
	}

	if _, err := device.Mount(mountPath, flags); err != nil {
		return "", fmt.Errorf("failed to mount %s, flags %d, err=%v", devName, flags, err)
	}

	Debug("MountDevice: Mounted %s with flags %d", devName, flags)
	setMountCache(devName, mountCacheData{flags: flags, mountPath: mountPath}) // update cache
	return mountPath, nil
}

/*
func MountDevice(device storage.BlockDev, flags uintptr) (string, error) {

	devName := device.Name
	key := devName
	// key := fmt.Sprintf("%s-%d", devName, flags)
	// Debug("Lookup mountCache with key = %s", key)
	Debug("mountCache: Lookup with key = %s", key)
	cachedData, ok := mountCache[key]
	if ok {
		cachedMountPath := cachedData[0]
		cachedFlags := cachedData[1]
		Debug("mountCache: Lookup succeeded: cachedMountPath %s, flags %s found for key %s", cachedMountPath, cachedFlags, key)
		if cachedFlags == flags {
			return cachedMountPath, nil
		}
		Debug("mountCache: need to mount the same device with different flags")
		Debug("mountCache: lets unmount this device first. Unmounting %s", cachedMountPath)
		if e := mount.Unmount(cachedMountPath, true, false); e != nil {
			log.Printf("Unmount failed for %s. PANIC")
			panic(e)
		}
		Debug("mountCache: unmount successfull. lets clean up it's entry in map")
		mountCache[key] = nil
		return nil, fmt.Errorf("device was already mounted. Mount again.")
	} else {
		log.Printf("Lookup mountCache failed: No key exists that matches %s ", key)
	}

	// devicePath := filepath.Join("/dev", devName)
	// Debug("Attempting to mount %s", devicePath)
	Debug("Attempting to mount %s", devName)
	mountPath, err := ioutil.TempDir("/tmp", "slaunch-")
	if err != nil {
		return "", fmt.Errorf("failed to create tmp mount directory: %v", err)
	}

	if _, err := device.Mount(mountPath, flags); err != nil {
		return "", fmt.Errorf("failed to mount %s, err=%v", devName, err)
		// log.Printf("failed to mount %s, continuing to next block device", devicePath)
		// return "", fmt.Errorf("failed to mount %s, continuing to next block device", devicePath)
	}
	// slaunch.Debug("mp.Path=%s, mountPath=%s", mp.Path, mountPath) verified they are equal

	Debug("Mounted %s", devName)
	// Debug("Mounted %s", devicePath)
	mountCache[key] = []string{mountPath, flags} // update cache
	Debug("Added enty to mountCache with key = %s, value = %s", key, mountPath)
	//mountCache[devName] = mountPath // update cache
	return mountPath, nil

}
*/

func UnmountAll() {

	Debug("UnmountAll: %d devices need to be unmounted", len(mountCache.m))
	for key, mountCacheData := range mountCache.m {
		cachedMountPath := mountCacheData.mountPath
		Debug("UnmountAll: Unmounting %s", cachedMountPath)
		if e := mount.Unmount(cachedMountPath, true, false); e != nil {
			log.Printf("Unmount failed for %s. PANIC", cachedMountPath)
			panic(e)
		}
		Debug("UnmountAll: Unmounted %s", cachedMountPath)
		deleteEntryMountCache(key)
		Debug("UnmountAll: Deleted key %s from cache", key)
	}
}

/*
 * GetMountedFilePath returns a file path corresponding to a <device_identifier>:<path> user input format.
 * <device_identifier> can only be FS UUID.
 *
 * NOTE: Caller's responsbility to unmount this..use return var mountPath to unmount in caller.
 */
// func GetMountedFilePath(inputVal string, flags uintptr) (string, *mount.MountPoint, error) {
func GetMountedFilePath(inputVal string, flags uintptr) (string, string, error) {
	s := strings.Split(inputVal, ":")
	if len(s) != 2 {
		return "", "", fmt.Errorf("%s: Usage: <block device identifier>:<path>", inputVal)
		// return "", nil, fmt.Errorf("%s: Usage: <block device identifier>:<path>", inputVal)
	}

	// s[0] can be sda or UUID. if UUID, then we need to find its name
	device, err := GetStorageDevice(s[0])
	if err != nil {
		return "", "", fmt.Errorf("fn GetStorageDevice: err = %v", err)
	}

	//	device, e := GetDeviceFromUUID(s[0])
	//	if e != nil {
	//		d2, e2 := getDeviceFromName(s[0])
	//		if e2 != nil {
	//			return "", "", fmt.Errorf("fn GetDeviceFromUUID: err = %v, %v", e, e2)
	//		}
	//		device = d2
	//	}

	devName := device.Name
	mountPath, err := MountDevice(device, flags)
	if err != nil {
		return "", "", fmt.Errorf("failed to mount %s , flags=%v, err=%v", devName, flags, err)
	}

	// var dev storage.BlockDev // inject error so that mount fails...
	// if _, err := dev.Mount(mountPath, flags); err != nil { // inject error so that mount fails...

	// if _, err := device.Mount(mountPath, flags); err != nil {
	//	return "", "", fmt.Errorf("failed to mount %s , flags=%v, err=%v", devName, flags, err)
	// }
	//	Debug("Attempting to mount %s", devPath)
	//	mountPath, err := ioutil.TempDir("/tmp", "slaunch-")
	//	if err != nil {
	//		return "", "", fmt.Errorf("failed to create tmp mount directory: %v", err)
	//	}
	//
	//	// var dev storage.BlockDev // inject error so that mount fails...
	//	// if _, err := dev.Mount(mountPath, flags); err != nil {
	//	if _, err := device.Mount(mountPath, flags); err != nil {
	//		return "", "", fmt.Errorf("failed to mount %s , flags=%v, err=%v", devPath, flags, err)
	//	}
	//
	//	Debug("Mounted %s", devPath)

	fPath := filepath.Join(mountPath, s[1]) // mountPath=/tmp/path/to/target/file if /dev/sda mounted on /tmp
	return fPath, mountPath, nil
}

/*
 * GetMountedFilePath returns a file path corresponding to a <device_identifier>:<path> user input format.
 * <device_identifier> may be a Linux block device identifier like sda or a FS UUID.
 *
 * NOTE: Caller's responsbility to unmount this..use return var mountPath to unmount in caller.
 */
/*
func GetMountedFilePath(inputVal string, flags uintptr) (string, string, error) {
	s := strings.Split(inputVal, ":")
	if len(s) != 2 {
		return "", "", fmt.Errorf("%s: Usage: <block device identifier>:<path>", inputVal)
	}

	// s[0] can be sda or UUID. if UUID, then we need to find its name
	device, e := GetDeviceFromUUID(s[0])
	if e != nil {
		// log.Printf("err = %v", e)
		return "", "", fmt.Errorf("fn GetDeviceFromUUID: err = %v", e)
		// d2, e2 := getDevPathFromName(s[0])
		// d2, e2 := getDeviceFromName(s[0])
		// if e2 != nil {
		// log.Printf("err = %v", e2)
		// 	return "", "", fmt.Errorf("fn GetDeviceFromUUID: err = %v, %v", e, e2)
		// }
		// device = d2
	}

//		if !strings.HasPrefix(s[0], "sd") {
//			if e := GetBlkInfo(); e != nil {
//				return "", "", fmt.Errorf("GetBlkInfo err=%s", e)
//			}
//			devices := storage.PartitionsByFsUUID(StorageBlkDevices, s[0]) // []BlockDev
//			if len(devices) == 0 {
//				return "", "", fmt.Errorf("no block device exists with UUID=%s", s[0])
//			}
//			for _, d := range devices {
//				Debug("device =%s with fsuuid=%s", d.Name, s[0])
//				device = d
//			}
//		} else {
//			if e := GetBlkInfo(); e != nil {
//				return "", "", fmt.Errorf("GetBlkInfo err=%s", e)
//			}
//			devices := storage.PartitionsByName(StorageBlkDevices, s[0]) // []BlockDev
//			if len(devices) == 0 {
//				return "", "", fmt.Errorf("no block device exists with name=%s", s[0])
//			}
//			for _, d := range devices {
//				Debug("device =%s with name=%s", d.Name, s[0])
//				device = d
//			}
//		}

	devPath := filepath.Join("/dev", device.Name)
	Debug("Attempting to mount %s", devPath)
	mountPath, err := ioutil.TempDir("/tmp", "slaunch-")
	if err != nil {
		return "", "", fmt.Errorf("failed to create tmp mount directory: %v", err)
		// return "", "", fmt.Errorf("failed to mount %v , flags=%v, err=%v", devicePath, flags, err)
		// return "", nil, fmt.Errorf("failed to mount %v , flags=%v, err=%v", devicePath, flags, err)
		// return "", "", fmt.Errorf("failed to mount %v , flags=%v, err=%v", devicePath, flags, err)
	}

	if _, err := device.Mount(mountPath, flags); err != nil {
		return "", "", fmt.Errorf("failed to mount %s , flags=%v, err=%v", devPath, flags, err)
	}

	// Debug("mountPath=%s, mp.Path=%s", mountPath, mp.Path) this is always equal
	Debug("Mounted %s", devPath)
	fPath := filepath.Join(mountPath, s[1]) // mountPath=/tmp/path/to/target/file if /dev/sda mounted on /tmp
	// fPath := filepath.Join("/dev/", mp.Path, s[1]) // mountPath=/tmp/path/to/target/file if /dev/sda mounted on /tmp
	return fPath, mountPath, nil
	// return fPath, mp.Path, nil
}
*/

/*
 * GetBlkInfo calls storage package to get information on all block devices.
 * The information is stored in a global variable 'StorageBlkDevices'
 * If the global variable is already non-zero, we skip the call to storage package.
 *
 * In debug mode, it also prints names and UUIDs for all devices.
 */
func GetBlkInfo() error {
	if len(StorageBlkDevices) == 0 {
		var err error
		Debug("getBlkInfo: expensive function call to get block stats from storage pkg")
		StorageBlkDevices, err = storage.GetBlockStats()
		if err != nil {
			return fmt.Errorf("getBlkInfo: storage.GetBlockStats err=%v. Exiting", err)
		}
		// no block devices exist on the system.
		if len(StorageBlkDevices) == 0 {
			return fmt.Errorf("getBlkInfo: no block devices found")
		}
		// print the debug info only when expensive call to storage is made
		for k, d := range StorageBlkDevices {
			Debug("block device #%d, Name=%s, FSType=%s, FsUUID=%s", k, d.Name, d.FSType, d.FsUUID)
		}
		return nil
	}
	Debug("getBlkInfo: noop")
	return nil
}
