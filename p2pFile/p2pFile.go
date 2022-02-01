package p2pFile

type P2PFile struct {
	Name     string
	FullPath string `json:"-"`
	Checksum string
	Size     int64
}
