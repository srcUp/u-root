package tpm

import (
	"fmt"
	"github.com/google/go-tpm/tpm2"
	"io"
)

func GetHandle() (io.ReadWriteCloser, error) {
	tpm2, err := tpm2.OpenTPM("/dev/tpm0")
	if err != nil {
		return nil, fmt.Errorf("Couldn't talk to TPM Device: err=%v\n", err)
	}

	return tpm2, nil
}
