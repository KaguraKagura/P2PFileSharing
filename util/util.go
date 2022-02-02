package util

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

const (
	AppName = "p2pFileSharing"
)

// HashFileSHA256 computes sha256 of the file. It assumes f is valid
func HashFileSHA256(f *os.File) (string, error) {
	buffer := make([]byte, 10*1024*1024)

	hash := sha256.New()
	for {
		n, err := f.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}

		if _, err := hash.Write(buffer[:n]); err != nil {
			return "", err
		}
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
