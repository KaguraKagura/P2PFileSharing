package peer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"Lab1/communication"
	"Lab1/util"
)

func localFilesMapToP2PFiles(localFiles map[string]localFile) []communication.P2PFile {
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

func makeLocalFiles(filepaths []string) (map[string]localFile, error) {
	var files map[string]localFile
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

		files[fullPath] = localFile{
			name:     filepath.Base(fullPath),
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
