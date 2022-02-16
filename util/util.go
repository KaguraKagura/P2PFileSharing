package util

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
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

// UnionIntSet returns the union of the 2 sets in a new non-nil set
func UnionIntSet(a, b map[int]struct{}) map[int]struct{} {
	result := make(map[int]struct{})
	for k, v := range a {
		result[k] = v
	}
	for k, v := range b {
		result[k] = v
	}
	return result
}

func ValidateHostPort(hostPort string) error {
	_, port, err := net.SplitHostPort(hostPort)
	if err != nil {
		return err
	}

	portNumber, err := strconv.Atoi(port)
	if err != nil {
		return err
	}

	if portNumber < 0 || portNumber > 65535 {
		return fmt.Errorf("tcp port out of range")
	}
	return nil
}
