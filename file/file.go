package file

import "crypto/sha256"




type File struct {
	Name string
	Checksum [sha256.Size]byte
	Length uint64


}

