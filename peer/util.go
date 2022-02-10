package peer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"Lab1/communication"
	"Lab1/util"
)

func localFilesToP2PFiles(localFiles map[remoteFile]localFile) []communication.P2PFile {
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
	// todo: failed response for other operations

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

// parseFilepaths returns a map from remoteFile to localFile
func parseFilepaths(filepaths []string) (map[remoteFile]localFile, error) {
	var files map[remoteFile]localFile
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

		files[remoteFile{
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

// pickChunk picks a chunk when excludedChunkByIndexChan receives things, then send the picked chunk to pickedChunkChan
func pickChunk(ctx context.Context, locations communication.ChunkLocations, locationsLock *sync.Mutex,
	excludedChunkByIndexChan <-chan map[int64]struct{}, pickedChunkChan chan<- toBeDownloadedChunk) {
	//chunkInTransmissionByIndex := make(map[int64]struct{})

	for {
		select {
		case <-ctx.Done():
			return
		case excludedIndex := <-excludedChunkByIndexChan:
			if len(excludedIndex) == 0 {
				//todo: special case?
			}

			locationsLock.Lock()

			// todo: pick a chunk

			locationsLock.Unlock()

			// todo: pickedChunkChan<- chunk
			// todo: chunkInTransmissionByIndex[index] = struct{}
		}
	}
}

func writeChunk(f *os.File, c downloadedChunk) error {
	if _, err := f.Seek(communication.ChunkSize*c.index, 0); err != nil {
		return err
	}
	if _, err := f.Write(c.data); err != nil {
		return err
	}
	return nil
}
