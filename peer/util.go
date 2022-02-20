package peer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"Lab1/communication"
)

// localFilesToP2PFiles converts map[fileID]localFile to []communication.P2PFile
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

// makeFailedOperationResponse makes a failed response
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

// makeLocalFiles converts local filepaths to a map[fileID]localFile
func makeLocalFiles(filepaths []string) (map[fileID]localFile, error) {
	files := make(map[fileID]localFile)
	for _, fullPath := range filepaths {
		f, err := os.Open(fullPath)
		if err != nil {
			return nil, err
		}

		checksum, err := sha256FileChecksum(f)
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

// pickChunk picks a chunk which index is not in excludedChunksByIndex
// can return nil if no chunk is picked
func pickChunk(locations communication.ChunkLocations, excludedChunksByIndex map[int]struct{}) *toBeDownloadedChunk {
	// pick the chunk that fewest peers have it
	numberOfPeersWithChunk := math.MaxInt64
	chunkIndex := -1
	for index, location := range locations {
		if _, ok := excludedChunksByIndex[index]; !ok {
			n := len(location)
			if n < numberOfPeersWithChunk {
				numberOfPeersWithChunk = n
				chunkIndex = index
			}
		}
	}

	// no chunk is picked
	if chunkIndex == -1 {
		return nil
	}

	// randomly pick a peer from all the peers having the chunk
	hostPorts := make([]string, 0, numberOfPeersWithChunk)
	for hostPort := range locations[chunkIndex] {
		hostPorts = append(hostPorts, hostPort)
	}
	rand.Seed(time.Now().UnixNano())
	pickedHostPort := hostPorts[rand.Intn(numberOfPeersWithChunk)]

	return &toBeDownloadedChunk{
		index:    chunkIndex,
		hostPort: pickedHostPort,
	}
}

// sha256FileChecksum computes sha256 of the file. It assumes f is not nil.
func sha256FileChecksum(f *os.File) (string, error) {
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

// validatePeerPeerHeader compares 2 communication.PeerPeerHeader
func validatePeerPeerHeader(received, sent communication.PeerPeerHeader) error {
	if received != sent {
		return fmt.Errorf("%s", badPeerResponse)
	}
	return nil
}

// validatePeerTrackerHeader compares 2 communication.PeerTrackerHeader
func validatePeerTrackerHeader(received, sent communication.PeerTrackerHeader) error {
	if received != sent {
		return fmt.Errorf("%s", badTrackerResponse)
	}
	return nil
}

// writeChunk writes a chunk to the file
func writeChunk(f *os.File, chunkIndex int, data []byte) error {
	offset := chunkIndex * communication.ChunkSize
	if _, err := f.WriteAt(data, int64(offset)); err != nil {
		return err
	}
	return nil
}
