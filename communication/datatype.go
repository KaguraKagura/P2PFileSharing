package communication

const (
	ChunkSize = 1024 * 1024

	Register PeerTrackerOperation = "register"
	List     PeerTrackerOperation = "list"
	Find     PeerTrackerOperation = "find"

	DownloadChunk PeerPeerOperation = "download_chunk"

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

type PeerTrackerHeader struct {
	RequestId string
	Operation PeerTrackerOperation
}

type PeerPeerHeader struct {
	RequestId string
	Operation PeerPeerOperation
}

type OperationResult struct {
	Code   OperationResultCode
	Detail string
}

type GenericResponse struct {
	Header PeerTrackerHeader
	Body   GenericResponseBody
}

type GenericResponseBody struct {
	Result OperationResult
}

type RegisterRequest struct {
	Header PeerTrackerHeader
	Body   RegisterRequestBody
}

type RegisterResponse struct {
	Header PeerTrackerHeader
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
	Header PeerTrackerHeader
	Body   ListFileRequestBody
}

type ListFileResponse struct {
	Header PeerTrackerHeader
	Body   ListFileResponseBody
}

type ListFileRequestBody struct {
}

type ListFileResponseBody struct {
	Result OperationResult
	Files  []P2PFile
}

type FindFileRequest struct {
	Header PeerTrackerHeader
	Body   FindFileRequestBody
}

type FindFileResponse struct {
	Header PeerTrackerHeader
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

type DownloadChunkRequest struct {
	Header PeerPeerHeader
	Body   DownloadChunkRequestBody
}

type DownloadChunkResponse struct {
	Header PeerPeerHeader
	Body   DownloadChunkResponseBody
}

type DownloadChunkRequestBody struct {
	FileName   string
	Checksum   string
	ChunkIndex int64
}

type DownloadChunkResponseBody struct {
	Result        OperationResult
	ChunkIndex    int64
	ChunkData     []byte
	ChunkChecksum string
}
