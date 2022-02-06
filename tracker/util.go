package tracker

import (
	"encoding/json"

	"Lab1/communication"
)

type genericRequest struct {
	Header communication.PeerTrackerHeader
	Body   json.RawMessage
}

// the []byte return value has been encoded into a raw json message
func makeFailedOperationResponse(header communication.PeerTrackerHeader, e error) []byte {
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
	case communication.List:
		resp, _ = json.Marshal(communication.ListFileResponse{
			Header: header,
			Body: communication.ListFileResponseBody{
				Result: result,
				Files:  nil,
			},
		})
	case communication.Find:
		resp, _ = json.Marshal(communication.FindFileResponse{
			Header: header,
			Body: communication.FindFileResponseBody{
				Result:         result,
				ChunkLocations: nil,
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
