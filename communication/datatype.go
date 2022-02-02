package communication

type PeerTrackerOperation string
type PeerPeerOperation string
type TrackerResponse string

const (
	CHUNK_SIZE = 10

	Register PeerTrackerOperation = "register"
	List     PeerTrackerOperation = "list"
	Find     PeerTrackerOperation = "find"
	Download PeerPeerOperation    = "download"

	Success TrackerResponse = "success"
	Fail    TrackerResponse = "fail"
)

type P2PFile struct {
	Name     string
	Checksum string
	Size     int64
}

type PeerRegisterRequest struct {
	RequestId    string
	Operation    PeerTrackerOperation
	HostPort     string
	FilesToShare []P2PFile
}

type PeerRegisterResponse struct {
	RequestId    string
	Operation    PeerTrackerOperation
	Result       TrackerResponse
	ErrorMessage string
}
