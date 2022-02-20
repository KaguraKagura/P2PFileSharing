package util

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
)

const (
	AppName = "p2pFileSharing"
)

// StructToPrettyJsonString turns a struct into a pretty json string
func StructToPrettyJsonString(v interface{}) string {
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

// ValidateHostPort checks if the string is in valid "host:port" format
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
