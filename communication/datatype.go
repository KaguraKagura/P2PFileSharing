package communication

const (
	CHUNK_SIZE = 1024 * 1024

	Register PeerTrackerOperation = "register"
	List     PeerTrackerOperation = "list"
	Find     PeerTrackerOperation = "find"
	Download PeerPeerOperation    = "download"

	Success = "success"
	Fail    = "fail"
)

type PeerTrackerOperation string
type PeerPeerOperation string
type OperationResultCode string
type ChunkLocations []map[string]struct{}

type P2PFile struct {
	Name     string
	Checksum string
	Size     int64
}

type Header struct {
	RequestId string
	Operation PeerTrackerOperation
}

type OperationResult struct {
	Code   OperationResultCode
	Detail string
}

type GenericResponse struct {
	Header Header
	Body   GenericResponseBody
}

type GenericResponseBody struct {
	Result OperationResult
}

type RegisterRequest struct {
	Header Header
	Body   RegisterRequestBody
}

type RegisterResponse struct {
	Header Header
	Body   RegisterResponseBody
}

type RegisterRequestBody struct {
	HostPort     string
	FilesToShare []P2PFile
}

type RegisterResponseBody struct {
	Result          OperationResult
	RegisteredFiles []P2PFile
}

type ListFileRequest struct {
	Header Header
	Body   ListFileRequestBody
}

type ListFileResponse struct {
	Header Header
	Body   ListFileResponseBody
}

type ListFileRequestBody struct {
}

type ListFileResponseBody struct {
	Result OperationResult
	Files  []P2PFile
}

type FindFileRequest struct {
	Header Header
	Body   FindFileRequestBody
}

type FindFileResponse struct {
	Header Header
	Body   FindFileResponseBody
}

type FindFileRequestBody struct {
	FileName string
	Checksum string
}

type FindFileResponseBody struct {
	Result   OperationResult
	FileSize int64
	// ChunkLocations: for each (chunk) index is a set of host:port containing the chunk
	ChunkLocations ChunkLocations
}
