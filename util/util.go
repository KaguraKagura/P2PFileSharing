package util

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
)

const (
	AppName = "p2pFileSharing"

	BadSha256ChecksumHexStringSize = "bad sha256 checksum hex string size"
	Sha256ChecksumHexStringSize    = 64
)

// Sha256FileChecksum computes sha256 of the file. It assumes f is valid
func Sha256FileChecksum(f *os.File) (string, error) {
	buffer := make([]byte, 100*1024*1024)

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

func StructToPrettyString(v interface{}) string {
	prettyString, _ := json.MarshalIndent(v, "", "\t")
	return string(prettyString)
}

// UnionInt64Set returns the union of the 2 sets in a new non-nil set
func UnionInt64Set(a, b map[int64]struct{}) map[int64]struct{} {
	result := make(map[int64]struct{})
	for k, v := range a {
		result[k] = v
	}
	for k, v := range b {
		result[k] = v
	}
	return result
}
