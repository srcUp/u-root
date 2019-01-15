// Copyright 2019 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package multiboot

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const DebugPrefix = "MULTIBOOT_DEBUG_INFO:"

// Description stores representation of multiboot
// information passed to a final kernel used for
// for debugging and testing.
type Description struct {
	Status string `json:"status"`

	Flags      uint32 `json:"flags"`
	MemLower   uint32 `json:"mem_lower"`
	MemUpper   uint32 `json:"mem_upper"`
	MmapAddr   uint32 `json:"mmap_addr"`
	MmapLength uint32 `json:"mmap_length"`

	CmdLine    string `json:"cmdline"`
	Bootloader string `json:"bootloader"`

	Mmap    []MemoryMap  `json:"mmap"`
	Modules []ModuleDesc `json:"modules"`
}

// Description returns string representation of
// multiboot information.
func (m Multiboot) Description() (string, error) {
	var modules []ModuleDesc
	for i, mod := range m.loadedModules {
		name := strings.Fields(m.modules[i])[0]
		b, err := readFile(name)
		if err != nil {
			return "", nil
		}
		hash := md5.Sum(b)
		modules = append(modules, ModuleDesc{
			Start:   mod.Start,
			End:     mod.End,
			CmdLine: m.modules[i],
			MD5:     fmt.Sprintf("%x", hash),
		})

	}

	b, err := json.Marshal(Description{
		Status:     "ok",
		Flags:      uint32(m.info.Flags),
		MemLower:   m.info.MemLower,
		MemUpper:   m.info.MemUpper,
		MmapAddr:   m.info.MmapAddr,
		MmapLength: m.info.MmapLength,

		CmdLine:    m.cmdLine,
		Bootloader: m.bootloader,

		Mmap:    m.memoryMap(),
		Modules: modules,
	})
	if err != nil {
		return "", err
	}

	b = bytes.Replace(b, []byte{'\n'}, []byte{' '}, -1)
	return string(b), nil
}

// ModuleDesc is a debug representation of
// loaded module.
type ModuleDesc struct {
	Start   uint32 `json:"start"`
	End     uint32 `json:"end"`
	CmdLine string `json:"cmdline"`
	MD5     string `json:"md5"`
}

type mmap struct {
	Size     uint32 `json:"size"`
	BaseAddr string `json:"base_addr"`
	Length   string `json:"length"`
	Type     uint32 `json:"type"`
}

// MarshalJSON implements json.Marshaler
func (m MemoryMap) MarshalJSON() ([]byte, error) {
	return json.Marshal(mmap{
		Size:     m.Size,
		BaseAddr: fmt.Sprintf("%#x", m.BaseAddr),
		Length:   fmt.Sprintf("%#x", m.Length),
		Type:     m.Type,
	})
}

// UnmarshalJSON implements json.Unmarshaler
func (m *MemoryMap) UnmarshalJSON(b []byte) error {
	var desc mmap
	err := json.Unmarshal(b, &desc)
	if err != nil {
		return err
	}

	m.Size = desc.Size
	m.Type = desc.Type
	v, err := strconv.ParseUint(desc.BaseAddr, 0, 64)
	if err != nil {
		return err
	}
	m.BaseAddr = v

	v, err = strconv.ParseUint(desc.Length, 0, 64)
	if err != nil {
		return err
	}
	m.Length = v
	return nil
}
