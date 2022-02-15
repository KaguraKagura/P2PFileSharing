package peer

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"Lab1/communication"
	"Lab1/util"
)

func localFilesToP2PFiles(localFiles map[fileID]localFile) []communication.P2PFile {
	var p2pFiles []communication.P2PFile
	for _, file := range localFiles {
		p2pFiles = append(p2pFiles, communication.P2PFile{
			Name:     file.name,
			Checksum: file.checksum,
			Size:     file.size,
		})
	}
	return p2pFiles
}

func validatePeerPeerHeader(received, sent communication.PeerPeerHeader) error {
	if received != sent {
		return fmt.Errorf("%s", badPeerResponse)
	}
	return nil
}

func validatePeerTrackerHeader(received, sent communication.PeerTrackerHeader) error {
	if received != sent {
		return fmt.Errorf("%s", badTrackerResponse)
	}
	return nil
}

// the []byte return value has been encoded into a raw json message
func makeFailedOperationResponse(header communication.PeerPeerHeader, e error) []byte {
	result := communication.OperationResult{
		Code:   communication.Fail,
		Detail: e.Error(),
	}

	var resp []byte
	switch header.Operation {
	case communication.DownloadChunk:
		resp, _ = json.Marshal(communication.DownloadChunkResponse{
			Header: header,
			Body: communication.DownloadChunkResponseBody{
				Result:        result,
				ChunkIndex:    0,
				ChunkData:     nil,
				ChunkChecksum: "",
			},
		})
	default:
		resp, _ = json.Marshal(communication.GenericPeerPeerResponse{
			Header: header,
			Body: communication.GenericPeerPeerResponseBody{
				Result: result,
			},
		})
	}

	return resp
}

// makeLocalFiles returns a map from fileID to localFile
func makeLocalFiles(filepaths []string) (map[fileID]localFile, error) {
	files := make(map[fileID]localFile)
	for _, fullPath := range filepaths {
		f, err := os.Open(fullPath)
		if err != nil {
			return nil, err
		}

		checksum, err := util.Sha256FileChecksum(f)
		if err != nil {
			_ = f.Close()
			return nil, err
		}

		stat, err := f.Stat()
		if err != nil {
			_ = f.Close()
			return nil, err
		}

		_ = f.Close()

		name := filepath.Base(fullPath)

		files[fileID{
			name:     name,
			checksum: checksum,
		}] = localFile{
			name:     name,
			fullPath: fullPath,
			checksum: checksum,
			size:     stat.Size(),
		}
	}
	return files, nil
}

// pickChunk
func pickChunk(locations communication.ChunkLocations, excludedChunkByIndex map[int64]struct{}) toBeDownloadedChunk {

	if len(excludedChunkByIndex) == 0 {
		// todo: special case?
	}

	// todo: pick a chunk

	return toBeDownloadedChunk{
		index:    0,
		hostPort: "",
	}
}

func writeChunk(f *os.File, chunkIndex int64, data []byte) error {
	if _, err := f.Seek(chunkIndex*communication.ChunkSize, io.SeekStart); err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		return err
	}
	return nil
}
