package communication

const (
	CHUNK_SIZE = 10

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

type FileListRequest struct {
	Header Header
	Body   FileListRequestBody
}

type FileListResponse struct {
	Header Header
	Body   FileListResponseBody
}

type FileListRequestBody struct {
}

type FileListResponseBody struct {
	Result OperationResult
	Files  []P2PFile
}
