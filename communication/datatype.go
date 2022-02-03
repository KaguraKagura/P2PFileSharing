package communication

type PeerTrackerOperation string
type PeerPeerOperation string

const (
	CHUNK_SIZE = 10

	Register PeerTrackerOperation = "register"
	List     PeerTrackerOperation = "list"
	Find     PeerTrackerOperation = "find"
	Download PeerPeerOperation    = "download"

	Success = "success"
	Fail    = "fail"
)

type P2PFile struct {
	Name     string
	Checksum string
	Size     int64
}

type PeerRegisterRequest struct {
	Header PeerTrackerHeader
	Body   PeerRegisterRequestBody
}

type PeerRegisterResponse struct {
	Header          PeerTrackerHeader
	Result          TrackerResponseResult
	RegisteredFiles []P2PFile
}

type GenericResponse struct {
	Header PeerTrackerHeader
	Result TrackerResponseResult
}

type PeerTrackerHeader struct {
	RequestId string
	Operation PeerTrackerOperation
}

type PeerRegisterRequestBody struct {
	HostPort     string
	FilesToShare []P2PFile
}

type TrackerResponseResult struct {
	Result         string
	DetailedResult string
}
