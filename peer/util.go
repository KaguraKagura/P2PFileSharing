package peer

import "Lab1/communication"

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
