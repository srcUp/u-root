package tpm

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"github.com/google/go-tpm/tpm2"
	"github.com/google/go-tpm/tpmutil"
	slaunch "github.com/u-root/u-root/pkg/securelaunch"
	"io"
)

/*
 * tpm2.ReadPCR and tpm2.ExtendPCR need hashAlgo passed.
 * using sha256 for now
 */
const (
	hashAlgo = tpm2.AlgSHA256
)

/*
 * hashSum calculates the sha256 sum of a byte slice.
 */
func hashSum(in []byte) []byte {
	s := sha256.Sum256(in)
	return s[:]
}

/*
 * returns a tpm handle from go-tpm/tpm2
 * that can be used for storing hashes.
 */
func GetHandle() (io.ReadWriteCloser, error) {
	tpm2, err := tpm2.OpenTPM("/dev/tpm0")
	if err != nil {
		return nil, fmt.Errorf("Couldn't talk to TPM Device: err=%v\n", err)
	}

	return tpm2, nil
}

/*
 * ReadPCR reads pcr#x, where x is provided by 'pcr' arg.
 * result in a byte slice. 'tpmHandle' is the tpm device that owns the 'pcr'.
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
 * where x is provided by 'pcr' arg. pcr is owned by tpm2Handle which
 * is a tpm device.
 * err is returned if write to pcr fails.
 */
func ExtendPCR(tpmHandle io.ReadWriteCloser, pcr int, hash []byte) error {
	if e := tpm2.PCRExtend(tpmHandle, tpmutil.Handle(pcr), hashAlgo, hash, ""); e != nil {
		return e
	}
	return nil
}

/*
 * ExtendPCRDebug is identical to ExtendPCR except
 * provides the added functionality i.e in debug mode, it prints
 * 1. old pcr value before the hash is written to pcr
 * 2. new pcr values after hash is written to pcr
 * 3. compares old and new pcr values and prints error if they are not
 */
func ExtendPCRDebug(tpmHandle io.ReadWriteCloser, pcr int, data []byte) error {
	oldPCRValue, err := ReadPCR(tpmHandle, pcr)
	if err != nil {
		return fmt.Errorf("ReadPCR failed, err=%v", err)
	}
	slaunch.Debug("ExtendPCRDebug: oldPCRValue = [%x]", oldPCRValue)

	hash := hashSum(data)
	slaunch.Debug("Adding hash=[%x] to PCR #%d", hash, pcr)
	if e := ExtendPCR(tpmHandle, pcr, hash); e != nil {
		return fmt.Errorf("Can't extend PCR %d, err=%v", pcr, e)
	}

	newPCRValue, err := ReadPCR(tpmHandle, pcr)
	if err != nil {
		return fmt.Errorf("ReadPCR failed, err=%v", err)
	}
	slaunch.Debug("ExtendPCRDebug: newPCRValue = [%x]", newPCRValue)

	finalPCR := hashSum(append(oldPCRValue, hash...))
	if !bytes.Equal(finalPCR, newPCRValue) {
		return fmt.Errorf("PCRs not equal, got %x, want %x", finalPCR, newPCRValue)
	}

	return nil
}
