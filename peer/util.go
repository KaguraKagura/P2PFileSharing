package peer

import (
	"Lab1/communication"
	"Lab1/util"
	"fmt"
	"os"
	"path/filepath"
)

func localFilesToP2PFiles(localFiles []localFile) []communication.P2PFile {
	var p2pFiles []communication.P2PFile
	for _, f := range localFiles {
		p2pFiles = append(p2pFiles, communication.P2PFile{
			Name:     f.name,
			Checksum: f.checksum,
			Size:     f.size,
		})
	}
	return p2pFiles
}

func validatePeerPeerHeader(received, sent communication.PeerPeerHeader) error {
	if received != sent {
		return fmt.Errorf("%s", badTrackerResponse)
	}
	return nil
}

func validatePeerTrackerHeader(received, sent communication.PeerTrackerHeader) error {
	if received != sent {
		return fmt.Errorf("%s", badTrackerResponse)
	}
	return nil
}

func makeLocalFiles(filepaths []string) ([]localFile, error) {
	var p2pFiles []localFile
	for _, path := range filepaths {
		f, err := os.Open(path)
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

		p2pFiles = append(p2pFiles, localFile{
			name:     filepath.Base(path),
			fullPath: path,
			checksum: checksum,
			size:     stat.Size(),
		})
	}
	return p2pFiles, nil
}

func pickChunk(locations communication.ChunkLocations, excludedChunkIndex map[int64]struct{}) toBeDownloadedChunk {
	//todo: work
	if len(excludedChunkIndex) != 0 {
		//todo: work
	}
	return toBeDownloadedChunk{ // todo: fill fields
		index:    0,
		hostPort: "",
	}
}
