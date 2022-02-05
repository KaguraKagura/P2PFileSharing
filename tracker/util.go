package tracker

import (
	"encoding/json"

	"Lab1/communication"
)

type genericRequest struct {
	Header communication.Header
	Body   json.RawMessage
}

func calculateNumberOfChunks(fileSize int64) int {
	// todo
	return -1
}

// the []byte return value has been encoded into a raw json message
func makeFailedOperationResponse(header communication.Header, e error) []byte {
	result := communication.OperationResult{
		Code:   communication.Fail,
		Detail: e.Error(),
	}

	var resp []byte
	switch header.Operation {
	case communication.Register:
		resp, _ = json.Marshal(communication.RegisterResponse{
			Header: header,
			Body: communication.RegisterResponseBody{
				Result:          result,
				RegisteredFiles: nil,
			},
		})
	// todo other cases

	default:
		resp, _ = json.Marshal(communication.GenericResponse{
			Header: header,
			Body: communication.GenericResponseBody{
				Result: result,
			},
		})
	}

	return resp
}
