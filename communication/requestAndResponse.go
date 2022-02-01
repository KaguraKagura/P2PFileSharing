package communication

import (
	"Lab1/p2pFile"
)

const (
	Success = "success"
	Fail    = "fail"
)

type PeerRegisterRequest struct {
	Operation    string
	HostPort     string
	FilesToShare []p2pFile.P2PFile
}

type PeerRegisterResponse struct {
	Result       string
	ErrorMessage string
}
