package communication

const (
	//ChunkSize = 1024 * 1024
	ChunkSize = 1

	RegisterChunk PeerTrackerOperation = "register_chunk"
	RegisterFile  PeerTrackerOperation = "register_file"
	List          PeerTrackerOperation = "list"
	Find          PeerTrackerOperation = "find"

	DownloadChunk PeerPeerOperation = "download_chunk"

	Success = "success"
	Fail    = "fail"
)

// ChunkLocations for each (chunk) index is a set of host:port containing the chunk
type ChunkLocations []map[string]struct{}
type OperationResultCode string
type PeerPeerOperation string
type PeerTrackerOperation string

// general

type P2PFile struct {
	Name     string
	Checksum string
	Size     int64
}

type P2PChunk struct {
	FileName     string
	FileChecksum string
	ChunkIndex   int
}

type GenericPeerPeerResponse struct {
	Header PeerPeerHeader
	Body   GenericPeerPeerResponseBody
}

type GenericPeerPeerResponseBody struct {
	Result OperationResult
}

type GenericPeerTrackerResponse struct {
	Header PeerTrackerHeader
	Body   GenericPeerTrackerResponseBody
}

type GenericPeerTrackerResponseBody struct {
	Result OperationResult
}

type OperationResult struct {
	Code   OperationResultCode
	Detail string
}

type PeerPeerHeader struct {
	RequestId string
	Operation PeerPeerOperation
}

type PeerTrackerHeader struct {
	RequestId string
	Operation PeerTrackerOperation
}

// PeerTrackerOperation

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

type RegisterChunkRequest struct {
	Header PeerTrackerHeader
	Body   RegisterChunkRequestBody
}

type RegisterChunkResponse struct {
	Header PeerTrackerHeader
	Body   RegisterChunkResponseBody
}

type RegisterChunkRequestBody struct {
	HostPort string
	Chunk    P2PChunk
}

type RegisterChunkResponseBody struct {
	Result          OperationResult
	RegisteredChunk P2PChunk
}

type RegisterFileRequest struct {
	Header PeerTrackerHeader
	Body   RegisterFileRequestBody
}

type RegisterFileResponse struct {
	Header PeerTrackerHeader
	Body   RegisterFileResponseBody
}

type RegisterFileRequestBody struct {
	HostPort     string
	FilesToShare []P2PFile
}

type RegisterFileResponseBody struct {
	Result          OperationResult
	RegisteredFiles []P2PFile
}

// PeerPeerOperations

type DownloadChunkRequest struct {
	Header PeerPeerHeader
	Body   DownloadChunkRequestBody
}

type DownloadChunkResponse struct {
	Header PeerPeerHeader
	Body   DownloadChunkResponseBody
}

type DownloadChunkRequestBody struct {
	FileName     string
	FileChecksum string
	ChunkIndex   int
}

type DownloadChunkResponseBody struct {
	Result        OperationResult
	ChunkIndex    int
	ChunkData     []byte
	ChunkChecksum string
}
