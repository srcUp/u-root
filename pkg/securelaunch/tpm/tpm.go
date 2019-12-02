package tpm

import (
	"fmt"
	"github.com/google/go-tpm/tpm2"
	"io"
)

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
