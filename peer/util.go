package peer

import (
	"fmt"
	"os"
	"path/filepath"

	"Lab1/communication"
	"Lab1/util"
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

func validateResponseHeader(received, sent communication.Header) error {
	if received != sent {
		return fmt.Errorf("%s", badTrackerResponse)
	}
	return nil
}

func validateFilepaths(filepaths []string) ([]localFile, error) {
	var p2pFiles []localFile
	for _, path := range filepaths {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}

		checksum, err := util.HashFileSHA256(f)
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

func pickChunk(locations communication.ChunkLocations, excludeIndex map[int64]struct{}) chunkToDownload {
	//todo
	if len(excludeIndex) != 0 {
		//todo:
	}
	return chunkToDownload{ // todo
		index:    0,
		hostPort: "",
	}
}
