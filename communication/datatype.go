package communication

const (
	ChunkSize = 1024 * 1024

	RegisterChunk PeerTrackerOperation = "register_chunk"
	RegisterFile  PeerTrackerOperation = "register_file"
	List          PeerTrackerOperation = "list"
	Find          PeerTrackerOperation = "find"

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

type P2PChunk struct {
	FileName     string
	FileChecksum string
	ChunkIndex   int64
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
	FileName     string
	FileChecksum string
	ChunkIndex   int64
}

type DownloadChunkResponseBody struct {
	Result        OperationResult
	ChunkIndex    int64
	ChunkData     []byte
	ChunkChecksum string
}
