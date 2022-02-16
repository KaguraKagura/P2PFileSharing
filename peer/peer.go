package peer

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"Lab1/communication"
	"Lab1/util"

	"github.com/google/uuid"
)

type localFile struct {
	name     string
	fullPath string
	checksum string
	size     int64
}

type partiallyDownloadedFile struct {
	fp                     *os.File
	meta                   localFile
	completedChunksByIndex map[int]struct{}
}

type fileID struct {
	name     string
	checksum string
}

type toBeDownloadedChunk struct {
	index    int
	hostPort string
}

type chunkDownloadResult struct {
	error    error
	hostPort string
	index    int
	data     []byte
}

type fileDownloadResult struct {
	error error
	f     localFile
}

type fileDownloadProgressQuery struct {
	completedChunksByIndex      map[int]struct{}
	inTransmissionChunksByIndex map[int]struct{}
}

type filesInTransmission struct {
	files       map[fileID]partiallyDownloadedFile
	jobControls map[fileID]fileDownloadJobControl
	// mu needed since this struct is both written to during download,
	// and read from during serving a chunk to a peer & when job control is performed
	mu sync.Mutex
}

type filesToShare struct {
	files map[fileID]localFile
	// mu needed since a files is both written to during register,
	// and read from during serving the file to another peer
	mu sync.Mutex
}

type fileDownloadJobControl struct {
	cancel   chan<- struct{}
	pause    chan<- bool
	progress chan fileDownloadProgressQuery
}

type peer struct {
	selfHostPort    string
	trackerHostPort string
	registered      bool
	servingFiles    bool

	filesToShare        filesToShare
	filesInTransmission filesInTransmission
}

var (
	self peer

	genericLogger = log.New(os.Stdout, "", 0)
	infoLogger    = log.New(os.Stdout, "INFO: ", 0)
	errorLogger   = log.New(os.Stdout, "ERROR: ", 0)
)

func Start() {
	genericLogger.Println(welcomeMessage)

	lineFromStdinChan := make(chan string)
	resultChan := make(chan string)
	errorChan := make(chan error)

	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			lineFromStdinChan <- strings.TrimSpace(scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}

		genericLogger.Printf("%s!", goodbye)
		os.Exit(0)
	}()

	for {
		select {
		// for non-blocking functions' result
		case result := <-resultChan:
			genericLogger.Printf("%s", result)

		// for non-blocking functions' error
		case err := <-errorChan:
			errorLogger.Printf("%v", err)

		// act on an entered line from stdin
		case line := <-lineFromStdinChan:
			if line == "" {
				continue
			}

			var (
				result string
				err    error
			)
			args := strings.Fields(line)
			switch args[0] {
			case registerCmd:
				if len(args) < 3 {
					err = fmt.Errorf("%s. %s", badArguments, helpPrompt)
					break
				}
				result, err = self.register(args[1], args[2], args[3:])
			case listCmd:
				if len(args) != 1 {
					err = fmt.Errorf("%s. %s", badArguments, helpPrompt)
					break
				}
				result, err = self.list()
			case findCmd:
				if len(args) != 3 {
					err = fmt.Errorf("%s. %s", badArguments, helpPrompt)
					break
				}
				result, err = self.find(args[1], args[2])
			case downloadCmd:
				if len(args) != 3 {
					err = fmt.Errorf("%s. %s", badArguments, helpPrompt)
					break
				}
				go func() {
					result, err := self.download(args[1], args[2])
					if err != nil {
						errorChan <- err
					} else {
						resultChan <- result
					}
				}()
				// continue since download is made non-blocking thus no result or error to print right now
				continue
			// todo: case showDownloadsCmd: need to lock in that func
			// todo: case cancelDownloadCmd: lock
			// todo: case showServeCmd: lock
			case hCmd:
				fallthrough
			case helpCmd:
				result = helpMessage
			case qCmd:
				fallthrough
			case quitCmd:
				genericLogger.Printf("%s!", goodbye)
				os.Exit(0)
			default:
				err = fmt.Errorf("%s %q", unrecognizedCommand, args[0])
			}

			if err != nil {
				errorLogger.Printf("%v", err)
			} else {
				if result != "" {
					genericLogger.Printf("%s", result)
				}
			}
		}
	}
}

// register tells the tracker what files to share and also starts a goroutine to serve local files to peers
func (p *peer) register(trackerHostPort, selfHostPort string, filepaths []string) (string, error) {
	for _, hp := range []string{trackerHostPort, selfHostPort} {
		if err := util.ValidateHostPort(hp); err != nil {
			return "", err
		}
	}

	if p.trackerHostPort != "" && p.trackerHostPort != trackerHostPort {
		return "", fmt.Errorf("%s %s", alreadyUsingTrackerAt, p.trackerHostPort)
	}

	if p.selfHostPort != "" && p.selfHostPort != selfHostPort {
		return "", fmt.Errorf("%s %s", alreadyUsingSelfHostPortAt, p.selfHostPort)
	}

	localFiles, err := makeLocalFiles(filepaths)
	if err != nil {
		return "", err
	}

	// prepare to talk to the tracker
	requestId := uuid.NewString()
	req, _ := json.Marshal(communication.RegisterFileRequest{
		Header: communication.PeerTrackerHeader{
			RequestId: requestId,
			Operation: communication.RegisterFile,
		},
		Body: communication.RegisterFileRequestBody{
			HostPort:     selfHostPort,
			FilesToShare: localFilesToP2PFiles(localFiles),
		},
	})

	// start to talk to the tracker
	dialer := net.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.Dial("tcp", trackerHostPort)
	if err != nil {
		return "", err
	}
	defer func(conn net.Conn) {
		_ = conn.Close()
	}(conn)

	if _, err := conn.Write(req); err != nil {
		return "", err
	}

	// get response from tracker
	var resp communication.RegisterFileResponse
	d := json.NewDecoder(conn)
	if err := d.Decode(&resp); err != nil {
		return "", err
	}

	if err := validatePeerTrackerHeader(resp.Header, communication.PeerTrackerHeader{
		RequestId: requestId,
		Operation: communication.RegisterFile,
	}); err != nil {
		return "", err
	}

	var (
		result string
		e      error
	)
	switch resp.Body.Result.Code {
	case communication.Success:
		p.selfHostPort = selfHostPort
		p.trackerHostPort = trackerHostPort
		p.registered = true

		p.filesToShare.mu.Lock()

		if p.filesToShare.files == nil {
			p.filesToShare.files = make(map[fileID]localFile)
		}
		for _, f := range resp.Body.RegisteredFiles {
			file := fileID{
				name:     f.Name,
				checksum: f.Checksum,
			}
			p.filesToShare.files[file] = localFiles[file]
		}

		p.filesToShare.mu.Unlock()

		var builder strings.Builder
		builder.WriteString(fmt.Sprintf("%s: %s\n", resp.Body.Result.Code, resp.Body.Result.Detail))
		registeredFiles := resp.Body.RegisteredFiles
		if registeredFiles != nil {
			builder.WriteString(fmt.Sprintf("%s:\n", registeredFilesAre))
			for _, f := range registeredFiles {
				builder.WriteString(fmt.Sprintf("%s\n", util.StructToPrettyString(f)))
			}
		}
		result = builder.String()
	case communication.Fail:
		e = fmt.Errorf("%s: %s", resp.Body.Result.Code, resp.Body.Result.Detail)
	default:
		e = fmt.Errorf("%s %q", unrecognizedPeerTrackerResponseResultCode, resp.Body.Result.Code)
	}

	if e != nil {
		return "", e
	}

	if p.servingFiles == false {
		l, err := net.Listen("tcp", selfHostPort)
		if err != nil {
			return "", err
		}
		go serveFiles(p, &l)

		p.servingFiles = true
	}
	return result, nil
}

func (p *peer) list() (string, error) {
	if p.registered == false {
		return "", fmt.Errorf("%s", pleaseRegisterFirst)
	}

	requestId := uuid.NewString()
	req, _ := json.Marshal(communication.ListFileRequest{
		Header: communication.PeerTrackerHeader{
			RequestId: requestId,
			Operation: communication.List,
		},
		Body: communication.ListFileRequestBody{},
	})

	// start to talk to the tracker
	dialer := net.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.Dial("tcp", p.trackerHostPort)
	if err != nil {
		return "", err
	}
	defer func(conn net.Conn) {
		_ = conn.Close()
	}(conn)

	if _, err := conn.Write(req); err != nil {
		return "", err
	}

	// get response from tracker
	var resp communication.ListFileResponse
	d := json.NewDecoder(conn)
	if err := d.Decode(&resp); err != nil {
		return "", err
	}

	if err := validatePeerTrackerHeader(resp.Header, communication.PeerTrackerHeader{
		RequestId: requestId,
		Operation: communication.List,
	}); err != nil {
		return "", err
	}

	switch resp.Body.Result.Code {
	case communication.Success:
		var builder strings.Builder
		builder.WriteString(fmt.Sprintf("%s: %s\n", resp.Body.Result.Code, resp.Body.Result.Detail))
		files := resp.Body.Files
		if len(files) != 0 {
			sort.Slice(files, func(i, j int) bool {
				return files[i].Name < files[j].Name
			})
			builder.WriteString(fmt.Sprintf("%s:\n", availableFilesAre))
			for _, f := range files {
				builder.WriteString(fmt.Sprintf("%s\n", util.StructToPrettyString(f)))
			}
		} else {
			builder.WriteString(fmt.Sprintf("%s", noFileIsAvailableRightNow))
		}
		return builder.String(), nil
	case communication.Fail:
		return "", fmt.Errorf("%s: %s", resp.Body.Result.Code, resp.Body.Result.Detail)
	default:
		return "", fmt.Errorf("%s %q", unrecognizedPeerTrackerResponseResultCode, resp.Body.Result.Code)
	}
}

func (p *peer) find(filename, checksum string) (string, error) {
	if p.registered == false {
		return "", fmt.Errorf("%s", pleaseRegisterFirst)
	}

	if len(checksum) != util.Sha256ChecksumHexStringSize {
		return "", fmt.Errorf("%s", util.BadSha256ChecksumHexStringSize)
	}

	resp, err := findFile(p, fileID{
		name:     filename,
		checksum: checksum,
	})
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("%s: %s\n", resp.Body.Result.Code, resp.Body.Result.Detail))
	chunkLocations := resp.Body.ChunkLocations
	for chunkIndex, chunkLocation := range chunkLocations {
		builder.WriteString(fmt.Sprintf("Chunk %d is at: ", chunkIndex))
		for l := range chunkLocation {
			builder.WriteString(fmt.Sprintf("%s ", l))
		}
		builder.WriteString(fmt.Sprintf("\n"))
	}

	return builder.String(), nil
}

func (p *peer) download(filename, checksum string) (string, error) {
	if p.registered == false {
		return "", fmt.Errorf("%s", pleaseRegisterFirst)
	}

	if len(checksum) != util.Sha256ChecksumHexStringSize {
		return "", fmt.Errorf("%s", util.BadSha256ChecksumHexStringSize)
	}

	infoLogger.Printf("%s %q %s", beginDownloading, filename, inTheBackground)

	file := fileID{
		name:     filename,
		checksum: checksum,
	}

	// start download
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// next 3 channels are only accessible to the current function and its callee
	resultChan := make(chan fileDownloadResult)
	pauseChan := make(chan bool)
	progressQueryChan := make(chan fileDownloadProgressQuery)
	go downloadFile(ctx, p, file, resultChan, progressQueryChan, pauseChan)

	// update p.filesInTransmission
	p.filesInTransmission.mu.Lock()

	if p.filesInTransmission.files == nil {
		p.filesInTransmission.files = make(map[fileID]partiallyDownloadedFile)
	}
	if p.filesInTransmission.jobControls == nil {
		p.filesInTransmission.jobControls = make(map[fileID]fileDownloadJobControl)
	}

	// next 3 channels are accessible to outside functions to query download progress
	cancelDownloadChan := make(chan struct{})
	pauseDownloadChan := make(chan bool)
	downloadProgressQueryChan := make(chan fileDownloadProgressQuery)
	p.filesInTransmission.jobControls[file] = fileDownloadJobControl{
		cancel:   cancelDownloadChan,
		pause:    pauseDownloadChan,
		progress: downloadProgressQueryChan,
	}

	p.filesInTransmission.mu.Unlock()

	// wait for download
	for {
		select {
		case r := <-resultChan:
			// remove the completed file from filesInTransmission
			p.filesInTransmission.mu.Lock()

			delete(p.filesInTransmission.files, file)
			delete(p.filesInTransmission.jobControls, file)

			p.filesInTransmission.mu.Unlock()

			if r.error != nil {
				return "", fmt.Errorf("%s %q: %w", failToDownload, filename, r.error)
			}

			// move the completed file to filesToShare
			p.filesToShare.mu.Lock()

			p.filesToShare.files[fileID{
				name:     r.f.name,
				checksum: r.f.checksum,
			}] = r.f

			p.filesToShare.mu.Unlock()
			return fmt.Sprintf("%s %q", downloadCompletedFor, filename), nil
		case query := <-downloadProgressQueryChan:
			// forward the query from outside to func downloadFile()
			progressQueryChan <- query
			response := <-progressQueryChan
			downloadProgressQueryChan <- response
		case <-cancelDownloadChan:
			p.filesInTransmission.mu.Lock()

			delete(p.filesInTransmission.files, file)
			delete(p.filesInTransmission.jobControls, file)

			p.filesInTransmission.mu.Unlock()
			return "", fmt.Errorf("%s", downloadCanceledByUser)
		case pause := <-pauseDownloadChan:
			pauseChan <- pause
		}
	}
}

func findFile(p *peer, file fileID) (*communication.FindFileResponse, error) {
	checksum := file.checksum
	if len(checksum) != util.Sha256ChecksumHexStringSize {
		return nil, fmt.Errorf("%s", util.BadSha256ChecksumHexStringSize)
	}

	requestId := uuid.NewString()
	req, _ := json.Marshal(communication.FindFileRequest{
		Header: communication.PeerTrackerHeader{
			RequestId: requestId,
			Operation: communication.Find,
		},
		Body: communication.FindFileRequestBody{
			FileName: file.name,
			Checksum: checksum,
		},
	})

	// start to talk to the tracker
	dialer := net.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.Dial("tcp", p.trackerHostPort)
	if err != nil {
		return nil, err
	}
	defer func(conn net.Conn) {
		_ = conn.Close()
	}(conn)

	if _, err := conn.Write(req); err != nil {
		return nil, err
	}

	// get response from tracker
	var resp communication.FindFileResponse
	d := json.NewDecoder(conn)
	if err := d.Decode(&resp); err != nil {
		return nil, err
	}

	if err := validatePeerTrackerHeader(resp.Header, communication.PeerTrackerHeader{
		RequestId: requestId,
		Operation: communication.Find,
	}); err != nil {
		return nil, err
	}

	switch resp.Body.Result.Code {
	case communication.Success:
		return &resp, nil
	case communication.Fail:
		return nil, fmt.Errorf("%s: %s", resp.Body.Result.Code, resp.Body.Result.Detail)
	default:
		return nil, fmt.Errorf("%s %q", unrecognizedPeerTrackerResponseResultCode, resp.Body.Result.Code)
	}
}

func downloadFile(ctx context.Context, p *peer, fileToDownload fileID,
	result chan<- fileDownloadResult, progressQuery chan fileDownloadProgressQuery, pauseDownload <-chan bool) {
	resp, err := findFile(p, fileToDownload)
	if err != nil {
		result <- fileDownloadResult{
			error: err,
			f:     localFile{},
		}
		return
	}

	// preallocate a file with size equal to the whole file
	fp, err := os.Create(fileToDownload.name)
	if err != nil {
		result <- fileDownloadResult{
			error: err,
			f:     localFile{},
		}
		return
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(fp)
	fileSize := resp.Body.FileSize
	if err = fp.Truncate(fileSize); err != nil {
		result <- fileDownloadResult{
			error: err,
			f:     localFile{},
		}
		return
	}

	var chunkLocations = resp.Body.ChunkLocations

	// "chunk download" goroutines
	toDownloadChunkChan := make(chan toBeDownloadedChunk)
	chunkDownloadResultChan := make(chan chunkDownloadResult)

	totalChunkCount := len(chunkLocations)
	workerCount := parallelDownloadWorkerCount
	if totalChunkCount < parallelDownloadWorkerCount {
		workerCount = totalChunkCount
	}

	for i := 0; i < workerCount; i++ {
		go func(ctx context.Context, toDownload <-chan toBeDownloadedChunk, result chan<- chunkDownloadResult) {
			for {
				select {
				case <-ctx.Done():
					return
				case want := <-toDownload:
					r := chunkDownloadResult{
						hostPort: want.hostPort,
						index:    want.index,
					}
					if data, err := downloadChunk(fileToDownload, want); err != nil {
						r.error = err
					} else {
						r.data = data
					}
					result <- r
				}
			}
		}(ctx, toDownloadChunkChan, chunkDownloadResultChan)
	}

	// assign initial work to every "chunk download" goroutine
	inTransmissionChunksByIndex := make(map[int]struct{})
	for i := 0; i < workerCount; i++ {
		picked, err := pickChunk(chunkLocations, inTransmissionChunksByIndex)
		if err != nil {
			result <- fileDownloadResult{
				error: err,
				f:     localFile{},
			}
			return
		}

		toDownloadChunkChan <- picked
		inTransmissionChunksByIndex[picked.index] = struct{}{}
	}

	// ticker to periodically update chunkLocations
	chunkLocationsUpdateInterval := 10 * time.Second
	chunkLocationsUpdateTicker := time.NewTicker(chunkLocationsUpdateInterval)
	defer chunkLocationsUpdateTicker.Stop()

	// wait for download completion
	paused := false
	completedChunksByIndex := make(map[int]struct{})

	for {
		select {
		case <-ctx.Done():
			return
		case paused = <-pauseDownload:
			if paused == true {
				infoLogger.Printf("%s %q", downloadingPausedFor, fileToDownload.name)
			} else {
				infoLogger.Printf("%s %q", downloadingResumedFor, fileToDownload.name)
			}
		case <-progressQuery:
			progressQuery <- fileDownloadProgressQuery{
				completedChunksByIndex:      completedChunksByIndex,
				inTransmissionChunksByIndex: inTransmissionChunksByIndex,
			}
		default:
			if paused == true {
				continue
			}

			select {
			case r := <-chunkDownloadResultChan:
				// remove chunk from in transmission set
				delete(inTransmissionChunksByIndex, r.index)

				if r.error != nil {
					// discard failed chunk
					infoLogger.Printf("%s (file: %q, chunk index: %v, host: %q) %s: %v",
						discardBadChunk, fileToDownload.name, r.index, r.hostPort, dueToError, r.error)

					// give a new chunk to a chunk download goroutine
					picked, err := pickChunk(chunkLocations, util.UnionIntSet(inTransmissionChunksByIndex, completedChunksByIndex))
					if err != nil {
						result <- fileDownloadResult{
							error: err,
							f:     localFile{},
						}
						return
					}

					toDownloadChunkChan <- picked
					inTransmissionChunksByIndex[picked.index] = struct{}{}
					continue
				}

				// write the chunk
				// need to lock because writeChunk shares the same fp of type *os.File
				// with serving of this partially downloaded file to a peer
				p.filesInTransmission.mu.Lock()

				if err := writeChunk(fp, r.index, r.data); err != nil {
					result <- fileDownloadResult{
						error: fmt.Errorf("%s (file: %q, chunk index: %v) %s: %w",
							writeChunkFails, fileToDownload.name, r.index, dueToError, err),
						f: localFile{},
					}
					return
				}

				p.filesInTransmission.mu.Unlock()

				// tell the tracker about the new chunk
				if err := registerChunk(p, fileToDownload, r.index); err != nil {
					result <- fileDownloadResult{
						error: err,
						f:     localFile{},
					}
					return
				}

				// update p.filesInTransmission by adding the new chunk
				f := localFile{
					name:     fileToDownload.name,
					fullPath: fileToDownload.name,
					checksum: fileToDownload.checksum,
					size:     fileSize,
				}
				p.filesInTransmission.mu.Lock()

				if fileInTransmission, ok := p.filesInTransmission.files[fileToDownload]; !ok {
					p.filesInTransmission.files[fileToDownload] = partiallyDownloadedFile{
						fp:                     fp,
						meta:                   f,
						completedChunksByIndex: make(map[int]struct{}),
					}
				} else {
					fileInTransmission.completedChunksByIndex[r.index] = struct{}{}
				}

				p.filesInTransmission.mu.Unlock()

				// add chunk to completed set
				completedChunksByIndex[r.index] = struct{}{}

				// if whole file download completed
				if len(completedChunksByIndex) == totalChunkCount {
					// no need of p.filesInTransmission.mu.Lock() to use fp since write only occurs above,
					// which is in the same goroutine as here
					checksumToVerify, err := util.Sha256FileChecksum(fp)
					if err != nil {
						result <- fileDownloadResult{
							error: err,
							f:     localFile{},
						}
						return
					}
					if checksumToVerify != fileToDownload.checksum {
						result <- fileDownloadResult{
							error: fmt.Errorf("%v", downloadedFileChecksumMismatch),
							f:     localFile{},
						}
						return
					}
					result <- fileDownloadResult{
						error: nil,
						f:     f,
					}
					return
				}

				// else still chunks left to download
				picked, err := pickChunk(chunkLocations, util.UnionIntSet(inTransmissionChunksByIndex, completedChunksByIndex))
				if err != nil {
					result <- fileDownloadResult{
						error: err,
						f:     localFile{},
					}
					return
				}
				toDownloadChunkChan <- picked
				inTransmissionChunksByIndex[picked.index] = struct{}{}
			case <-chunkLocationsUpdateTicker.C:
				resp, err := findFile(p, fileToDownload)
				if err != nil {
					infoLogger.Printf("%s: %v", ignoreFailedUpdateToChunkLocations, err)
					continue
				}
				chunkLocations = resp.Body.ChunkLocations
			}
		}
	}
}

// downloadChunk downloads a chunk from a peer and validates the checksum
func downloadChunk(file fileID, c toBeDownloadedChunk) ([]byte, error) {
	requestId := uuid.NewString()
	req, _ := json.Marshal(communication.DownloadChunkRequest{
		Header: communication.PeerPeerHeader{
			RequestId: requestId,
			Operation: communication.DownloadChunk,
		},
		Body: communication.DownloadChunkRequestBody{
			FileName:     file.name,
			FileChecksum: file.checksum,
			ChunkIndex:   c.index,
		},
	})

	// start to talk to the peer
	dialer := net.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.Dial("tcp", c.hostPort)
	if err != nil {
		return nil, err
	}
	defer func(conn net.Conn) {
		_ = conn.Close()
	}(conn)

	if _, err := conn.Write(req); err != nil {
		return nil, err
	}

	// get response from peer
	var resp communication.DownloadChunkResponse
	d := json.NewDecoder(conn)
	if err := d.Decode(&resp); err != nil {
		return nil, err
	}

	if err := validatePeerPeerHeader(resp.Header, communication.PeerPeerHeader{
		RequestId: requestId,
		Operation: communication.DownloadChunk,
	}); err != nil {
		return nil, err
	}

	switch resp.Body.Result.Code {
	case communication.Success:
		index := resp.Body.ChunkIndex
		data := resp.Body.ChunkData
		if index != c.index {
			return nil, fmt.Errorf("%s: expect %v get %v", downloadedChunkIndexMismatch, c.index, resp.Body.ChunkIndex)
		}

		digest := sha256.Sum256(data)
		checksum := hex.EncodeToString(digest[:])
		if checksum != resp.Body.ChunkChecksum {
			return nil, fmt.Errorf("%s: expect %q get %q", downloadedChunkChecksumMismatch, resp.Body.ChunkChecksum, checksum)
		}
		return data, nil
	case communication.Fail:
		return nil, fmt.Errorf("%s: %s", resp.Body.Result.Code, resp.Body.Result.Detail)
	default:
		return nil, fmt.Errorf("%s %q", unrecognizedPeerPeerResponseResultCode, resp.Body.Result.Code)
	}
}

// registerChunk registers a file chunk with the tracker
func registerChunk(p *peer, file fileID, chunkIndex int) error {
	requestId := uuid.NewString()
	chunk := communication.P2PChunk{
		FileName:     file.name,
		FileChecksum: file.checksum,
		ChunkIndex:   chunkIndex,
	}

	req, _ := json.Marshal(communication.RegisterChunkRequest{
		Header: communication.PeerTrackerHeader{
			RequestId: requestId,
			Operation: communication.RegisterChunk,
		},
		Body: communication.RegisterChunkRequestBody{
			HostPort: p.selfHostPort,
			Chunk:    chunk,
		},
	})

	// start to talk to the peer
	dialer := net.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.Dial("tcp", p.trackerHostPort)
	if err != nil {
		return err
	}
	defer func(conn net.Conn) {
		_ = conn.Close()
	}(conn)

	if _, err := conn.Write(req); err != nil {
		return err
	}

	// get response from peer
	var resp communication.RegisterChunkResponse
	d := json.NewDecoder(conn)
	if err := d.Decode(&resp); err != nil {
		return err
	}

	if err := validatePeerTrackerHeader(resp.Header, communication.PeerTrackerHeader{
		RequestId: requestId,
		Operation: communication.RegisterChunk,
	}); err != nil {
		return err
	}

	switch resp.Body.Result.Code {
	case communication.Success:
	case communication.Fail:
		return fmt.Errorf("%s: %s", resp.Body.Result.Code, resp.Body.Result.Detail)
	default:
		return fmt.Errorf("%s %q", unrecognizedPeerTrackerResponseResultCode, resp.Body.Result.Code)
	}

	return nil
}

func serveFiles(p *peer, l *net.Listener) {
	infoLogger.Printf("%s", startToServeFilesToPeers)

	for {
		conn, err := (*l).Accept()
		if err != nil {
			errorLogger.Printf("%v", err)
			continue
		}

		go func() {
			defer func() {
				_ = conn.Close()
			}()

			var (
				resp []byte
				err  error
			)

			var req communication.DownloadChunkRequest
			d := json.NewDecoder(conn)
			err = d.Decode(&req)
			if err == nil {
				chunkIndex := req.Body.ChunkIndex
				switch req.Header.Operation {
				case communication.DownloadChunk:
					id := fileID{
						name:     req.Body.FileName,
						checksum: req.Body.FileChecksum,
					}

					var data []byte
					data, err = getChunk(p, id, chunkIndex, communication.ChunkSize)
					if err != nil {
						break
					}

					digest := sha256.Sum256(data)
					resp, _ = json.Marshal(communication.DownloadChunkResponse{
						Header: req.Header,
						Body: communication.DownloadChunkResponseBody{
							Result: communication.OperationResult{
								Code:   communication.Success,
								Detail: chunkIsFound,
							},
							ChunkIndex:    chunkIndex,
							ChunkData:     data,
							ChunkChecksum: hex.EncodeToString(digest[:]),
						},
					})

					infoLogger.Printf("served chunk %d of file %s to %s", chunkIndex, id.name, conn.RemoteAddr().String())
				default:
					err = fmt.Errorf("%s %q", unrecognizedPeerPeerOperation, req.Header.Operation)
				}
			}

			if err != nil {
				errorLogger.Printf("%v", err)
				resp = makeFailedOperationResponse(req.Header, err)
			}

			if _, err := conn.Write(resp); err != nil {
				errorLogger.Printf("%v", err)
			}
		}()
	}
}

func getChunk(p *peer, id fileID, chunkIndex int, chunkSize int) ([]byte, error) {
	data := make([]byte, chunkSize)

	offset := chunkIndex * chunkSize

	// first look at peer.filesToShare
	p.filesToShare.mu.Lock()

	if f, ok := p.filesToShare.files[id]; ok {
		defer p.filesToShare.mu.Unlock()

		fp, err := os.Open(f.fullPath)
		if err != nil {
			return nil, err
		}

		defer func(fp *os.File) {
			_ = fp.Close()
		}(fp)

		if _, err := fp.Seek(int64(offset), io.SeekStart); err != nil {
			return nil, err
		}

		if n, err := fp.Read(data); err != nil {
			if errors.Is(err, io.EOF) {
				return data[:n], nil
			}
			return nil, err
		}
		return data, nil
	}

	p.filesToShare.mu.Unlock()

	// did not find in peer.filesToShare, then look at peer.filesInTransmission
	// no need to close the file from peer.filesToShare because download takes care of it
	p.filesInTransmission.mu.Lock()
	defer p.filesInTransmission.mu.Unlock()

	if f, ok := p.filesInTransmission.files[id]; ok {
		if _, ok := f.completedChunksByIndex[chunkIndex]; ok {
			if _, err := f.fp.Seek(int64(offset), io.SeekStart); err != nil {
				return nil, err
			}
			if n, err := f.fp.Read(data); err != nil {
				if errors.Is(err, io.EOF) {
					return data[:n], nil
				}
				return nil, err
			}
			return data, nil
		}
		return nil, fmt.Errorf("%s", chunkDoesNotExist)
	}
	return nil, fmt.Errorf("%s", fileDoesNotExist)
}
