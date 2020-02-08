// Copyright 2019 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package tpm reads and extends pcrs with measurements.
package tpm

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"unsafe"

	"github.com/google/go-tpm/tpm2"
	"github.com/google/go-tpm/tpmutil"
	slaunch "github.com/u-root/u-root/pkg/securelaunch"
	"github.com/u-root/u-root/pkg/securelaunch/eventlog"
)

/*
 * tpm2.ReadPCR and tpm2.ExtendPCR need hashAlgo passed.
 * using sha256 for now
 */
const (
	hashAlgo = tpm2.AlgSHA256
	/*
	 * Secure Launch event log entry type. The TXT specification defines the
	 * base event value as 0x400 for DRTM values.
	 */
)

/*
// IHA is a TPM2 structure
type IHA struct {
	hash [32]byte
}

// THA is a TPM2 structure
type THA struct {
	hashAlg tpm2.Algorithm // uint16
	digest  IHA
}

// LDigestValues is a TPM2 structure
type LDigestValues struct {
	count   uint32
	digests [1]THA
}

// TcgPcrEvent2 is a TPM2 default log structure (EFI only)
type TcgPcrEvent2 struct {
	pcrIndex  uint32
	eventType uint32
	digests   LDigestValues
	eventSize uint32
	event     [64]byte
}
*/

/*
func packEvent(e TcgPcrEvent2) ([]byte, error) {
	eSize := unsafe.Sizeof(e)
	log.Println("Unpacked event structure size=", eSize)

	buf := bytes.NewBuffer(make([]byte, 0, 512))
	if err := binary.Write(buf, binary.LittleEndian, &e); err != nil {
		log.Println(err)
		return nil, err
	}
	b := buf.Bytes()

	log.Printf("%v\n", b)
	bufSize := unsafe.Sizeof(b)
	log.Println("packed buffer size=", bufSize)
	return b, nil
}

func sendEventToSysfs(pcr uint32, h []byte) error {

	var a [32]byte // 256 sha is 256/8 = 32 bytes.
	copy(a[:], h)
	e := TcgPcrEvent2{
		pcrIndex:  pcr,
		eventType: slaunchType,
		digests: LDigestValues{
			count: 1,
			digests: [1]THA{
				{
					hashAlg: hashAlgo,
					digest: IHA{
						hash: a,
					},
				},
			},
		},
	}
	copy(e.event[:], "event")
	b, err := packEvent(e)
	if err != nil {
		return err
	}

	if t := eventlog.Add(b); t != nil {
		log.Println("I was here with err=", t)
		return t
	}
	return nil
}

// piecemeal writing of structure fields to buffer.
func marshalPcrEvent(e TcgPcrEvent2) ([]byte, error) {

	log.Printf("marshalPcrEvent\n")
	endianess := binary.LittleEndian
	buf := bytes.NewBuffer(make([]byte, 0, 64))

	if err := binary.Write(buf, endianess, e.pcrIndex); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, endianess, e.eventType); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, endianess, e.digests.count); err != nil {
		return nil, err
	}

	for i := uint32(0); i < e.digests.count; i++ {
		if err := binary.Write(buf, endianess, e.digests.digests[i].hashAlg); err != nil {
			return nil, err
		}

		if err := binary.Write(buf, endianess, e.digests.digests[i].digest.hash); err != nil {
			return nil, err
		}
	}

	if err := binary.Write(buf, endianess, e.eventSize); err != nil {
		return nil, err
	}

	// e.event = make([]byte, e.eventSize)
	if err := binary.Write(buf, endianess, e.event); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
*/

// piecemeal writing of structure fields to buffer.
func marshalPcrEvent(pcr uint32, h []byte, eventDesc []byte) ([]byte, error) {

	//log.Printf("marshalPcrEvent\n")
	//log.Println("pcr=", pcr)
	const baseTypeTXT = 0x400
	const slaunchType = uint32(baseTypeTXT + 0x102)
	//log.Println("slaunchType=", slaunchType)
	count := uint32(1) // if don't do this: binary.Write: invalid type int
	//log.Println("count=", count)
	//log.Println("hashAlgo=", hashAlgo)
	//log.Println("eventDesc=", string(eventDesc))
	eventDescLen := uint32(len(eventDesc))
	//log.Println("eventDescLen=", eventDescLen)
	slaunch.Debug("marshalPcrEvent: pcr=[%v], slaunchType=[%v], count=[%v], hashAlgo=[%v], eventDesc=[%s], eventDescLen=[%v]",
		pcr, slaunchType, count, hashAlgo, eventDesc, eventDescLen)

	endianess := binary.LittleEndian
	var buf bytes.Buffer

	if err := binary.Write(&buf, endianess, pcr); err != nil {
		return nil, err
	}

	if err := binary.Write(&buf, endianess, slaunchType); err != nil {
		return nil, err
	}

	if err := binary.Write(&buf, endianess, count); err != nil {
		return nil, err
	}

	for i := uint32(0); i < count; i++ {
		if err := binary.Write(&buf, endianess, hashAlgo); err != nil {
			return nil, err
		}

		if err := binary.Write(&buf, endianess, h); err != nil {
			return nil, err
		}
	}

	if err := binary.Write(&buf, endianess, eventDescLen); err != nil {
		return nil, err
	}

	if err := binary.Write(&buf, endianess, eventDesc); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func sendEventToSysfs(pcr uint32, h []byte, eventDesc []byte) {

	b, err := marshalPcrEvent(pcr, h, eventDesc)
	if err != nil {
		log.Println(err)
	}

	// log.Printf("%v\n", b)
	bufSize := unsafe.Sizeof(b)
	log.Println("packed buffer size=", bufSize)

	if t := eventlog.Add(b); t != nil {
		// log.Println("I was here with err=", t)
		return
	}
}

/*
 * max eventDesc is 64 bytes..
 * #define SL_MAX_EVENT_DATA   64 in Ross's tpmevtwrite.c
 * https://github.com/rossphilipson/travail/blob/master/trenchboot/debug/tpmwrevt.c#L77
 */
/*
func sendEventToSysfs(pcr uint32, h []byte, eventDesc []byte) {

	log.Println("pcr=", pcr)
	log.Println("slaunchType=", slaunchType)
	log.Println("count=", 1)
	log.Println("hashAlgo=", hashAlgo)

	var a [32]byte // 256 sha is 256/8 = 32 bytes.
	copy(a[:], h)
	e := TcgPcrEvent2{
		pcrIndex:  pcr,
		eventType: slaunchType,
		digests: LDigestValues{
			count: 1,
			digests: [1]THA{
				{
					hashAlg: hashAlgo,
					digest: IHA{
						hash: a,
					},
				},
			},
		},
	}
	// eventDesc can be nil.
	eventstrlen := len(eventDesc)
	copy(e.event[:], eventDesc)
	log.Println("event=", string(e.event[:63]))
	e.eventSize = uint32(eventstrlen)
	// log.Println("eventsize=", e.eventSize)

	eSize := unsafe.Sizeof(e)
	log.Println("Unpacked event structure size=", eSize)

	// unpacked (64) --> packed(24) bytes
	// but this didn't populate /boot/evtlog file.
	// b := []byte(fmt.Sprintf("%v", e)) // 64 --> 24 size compression

	b, err := marshalPcrEvent(e)
	if err != nil {
		log.Println(err)
	}

	// populates /boot/evtlog file but only one log added.
	//
	//	buf := bytes.NewBuffer(make([]byte, 0, 512))
	//	if err := binary.Write(buf, binary.LittleEndian, e); err != nil {
	//		log.Println(err)
	//	}
	//	b := buf.Bytes()
	//

	log.Printf("%v\n", b)
	bufSize := unsafe.Sizeof(b)
	log.Println("packed buffer size=", bufSize)

	if t := eventlog.Add(b); t != nil {
		log.Println("I was here with err=", t)
		return
	}

	//	fd, _ := os.OpenFile("/sys/kernel/security/slaunch/eventlog", os.O_WRONLY, 0644)
	//	fd.Write(b)
	//	fd.Close()
}
*/

/*
 * hashReader calculates the sha256 sum of an io reader.
 */
func hashReader(f io.Reader) []byte {

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		log.Fatal(err)
	}

	return h.Sum(nil)
}

/*
 * GetHandle returns a tpm device handle from go-tpm/tpm2
 * returns a tpm handle from go-tpm/tpm2
 * that can be used for storing hashes.
 */
func GetHandle() (io.ReadWriteCloser, error) {
	tpm2, err := tpm2.OpenTPM("/dev/tpm0")
	if err != nil {
		return nil, fmt.Errorf("couldn't talk to TPM Device: err=%v", err)
	}

	return tpm2, nil
}

/*
 * ReadPCR reads pcr#x, where x is provided by 'pcr' arg and returns
 * the result in a byte slice.
 * 'tpmHandle' is the tpm device that owns the 'pcr'.
 * err is returned if read fails.
 */
func ReadPCR(tpmHandle io.ReadWriteCloser, pcr int) ([]byte, error) {
	val, err := tpm2.ReadPCR(tpmHandle, pcr, hashAlgo)
	if err != nil {
		return nil, fmt.Errorf("Can't read PCR %d, err= %v", pcr, err)
	}
	return val, nil
}

/*
 * ExtendPCR writes the measurements passed as 'hash' arg to pcr#x,
 * where x is provided by 'pcr' arg.
 *
 * pcr is owned by 'tpm2Handle', a tpm device handle.
 * err is returned if write to pcr fails.
 */
func ExtendPCR(tpmHandle io.ReadWriteCloser, pcr int, hash []byte) error {
	return tpm2.PCRExtend(tpmHandle, tpmutil.Handle(pcr), hashAlgo, hash, "")
}

/*
 * ExtendPCRDebug extends a PCR with the contents of a byte slice.
 *
 * In debug mode, it prints
 * 1. old pcr value before the hash is written to pcr
 * 2. new pcr values after hash is written to pcr
 * 3. compares old and new pcr values and prints error if they are not
 */
func ExtendPCRDebug(tpmHandle io.ReadWriteCloser, pcr int, data io.Reader, eventDesc string) error {
	oldPCRValue, err := ReadPCR(tpmHandle, pcr)
	if err != nil {
		return fmt.Errorf("ReadPCR failed, err=%v", err)
	}
	slaunch.Debug("ExtendPCRDebug: oldPCRValue = [%x]", oldPCRValue)

	hash := hashReader(data)

	// sendEventToSysfs(uint32(pcr), hash)
	slaunch.Debug("Adding hash=[%x] to PCR #%d", hash, pcr)
	if e := ExtendPCR(tpmHandle, pcr, hash); e != nil {
		return fmt.Errorf("Can't extend PCR %d, err=%v", pcr, e)
	}
	slaunch.Debug(eventDesc)

	// send event if PCR was successfully extended above.
	sendEventToSysfs(uint32(pcr), hash, []byte(eventDesc))

	newPCRValue, err := ReadPCR(tpmHandle, pcr)
	if err != nil {
		return fmt.Errorf("ReadPCR failed, err=%v", err)
	}
	slaunch.Debug("ExtendPCRDebug: newPCRValue = [%x]", newPCRValue)

	finalPCR := hashReader(bytes.NewReader(append(oldPCRValue, hash...)))
	if !bytes.Equal(finalPCR, newPCRValue) {
		return fmt.Errorf("PCRs not equal, got %x, want %x", finalPCR, newPCRValue)
	}

	return nil
}
