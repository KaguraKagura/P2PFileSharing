package tracker

import (
	"encoding/json"
	"net"

	"Lab1/communication"
)

type genericRequest struct {
	Header communication.PeerTrackerHeader
	Body   json.RawMessage
}

func calculateNumberOfChunks(fileSize int64) int {
	return -1
}

func respondWithError(conn *net.Conn, op communication.PeerTrackerOperation, header communication.PeerTrackerHeader, e error) {
	var resp interface{}
	switch op {
	case communication.Register:
		resp, _ = json.Marshal(communication.PeerRegisterResponse{
			Header: communication.PeerTrackerHeader{
				RequestId: header.RequestId,
				Operation: header.Operation,
			},
			Result: communication.TrackerResponseResult{
				Result:         communication.Fail,
				DetailedResult: e.Error(),
			},
			RegisteredFiles: nil,
		})
	// todo other cases

	case unrecognizedOp:
		fallthrough
	default:
		resp, _ = json.Marshal(communication.GenericResponse{
			Header: communication.PeerTrackerHeader{
				RequestId: header.RequestId,
				Operation: header.Operation,
			},
			Result: communication.TrackerResponseResult{
				Result:         communication.Fail,
				DetailedResult: e.Error(),
			},
		})
	}

	_, err := (*conn).Write(resp.([]byte))
	if err != nil {
		errorLogger.Printf("%v\n", err)
		return
	}
}
