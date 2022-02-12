package peer

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
	meta                    localFile
	downloadedChunksByIndex map[int64]struct{}
	// mu needed since a downloadedChunksByIndex is both written to during download,
	// and read from during serving the chunk to another peer
	mu sync.Mutex
}

type fileID struct {
	name     string
	checksum string
}

type toBeDownloadedChunk struct {
	index    int64
	hostPort string
}

type chunkDownloadResult struct {
	error    error
	hostPort string
	index    int64
	data     []byte
}

type fileDownloadResult struct {
	error error
}

type fileDownloadProgressQuery struct {
	completedChunksByIndex      map[int64]struct{}
	inTransmissionChunksByIndex map[int64]struct{}
}

type filesInDownload struct {
	files map[fileID]fileDownloadJob
	// mu needed since files is both written to during download,
	// and read from during querying download progress
	mu sync.Mutex
}

type filesToShare struct {
	files map[fileID]localFile
	// mu needed since a files is both written to during register,
	// and read from during serving the file to another peer
	mu sync.Mutex
}

type fileDownloadJob struct {
	cancel   chan<- struct{}
	progress chan fileDownloadProgressQuery
}

type fileServeJob struct {
	cancel chan<- string
	// todo: things to help file serving
}

type peer struct {
	selfHostPort    string
	trackerHostPort string
	registered      bool
	servingFiles    bool

	//todo: partiallyDownloadedFiles
	filesToShare    filesToShare
	filesInDownload filesInDownload
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
			//todo: sanitize hostport (xx < port < xx) in bothe peer and tracker
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
				// continue since download is non-blocking thus no result or error to print right now
				continue
			// todo: case showDownloadsCmd: lock
			// todo: case cancelDownloadCmd: lock filesInDownload
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
				genericLogger.Printf("%s", result)
			}
		}
	}
}

// todo: multiple register and effect on shared file
func (p *peer) register(trackerHostPort, selfHostPort string, filepaths []string) (string, error) {
	for _, hp := range []string{trackerHostPort, selfHostPort} {
		if _, _, err := net.SplitHostPort(hp); err != nil {
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

		p.selfHostPort = selfHostPort
		p.trackerHostPort = trackerHostPort
		p.registered = true

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
		if files != nil {
			sort.Slice(files, func(i, j int) bool {
				return files[i].Name < files[j].Name
			})
			builder.WriteString(fmt.Sprintf("%s:\n", availableFilesAre))
			for _, f := range files {
				builder.WriteString(fmt.Sprintf("%s\n", util.StructToPrettyString(f)))
			}
		} else {
			builder.WriteString(fmt.Sprintf("%s", noAvailableFileRightNow))
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

	resultChan := make(chan fileDownloadResult)
	// progressQueryChan is only used inside the current function to query download progress
	progressQueryChan := make(chan fileDownloadProgressQuery)
	go downloadFile(ctx, p, file, resultChan, progressQueryChan)

	// update p.filesInDownload
	p.filesInDownload.mu.Lock()

	if p.filesInDownload.files == nil {
		p.filesInDownload.files = make(map[fileID]fileDownloadJob)
	}
	cancelDownloadChan := make(chan struct{})
	// downloadProgressQueryChan is used by outside functions to query download progress
	downloadProgressQueryChan := make(chan fileDownloadProgressQuery)
	p.filesInDownload.files[file] = fileDownloadJob{
		cancel:   cancelDownloadChan,
		progress: downloadProgressQueryChan,
	}

	p.filesInDownload.mu.Unlock()

	// wait for download
	for {
		select {
		case r := <-resultChan:
			p.filesInDownload.mu.Lock()

			delete(p.filesInDownload.files, file)

			p.filesInDownload.mu.Unlock()

			if r.error != nil {
				return "", fmt.Errorf("%s %q: %v\n", failToDownload, filename, r.error)
			} else {
				return fmt.Sprintf("%s %q", downloadCompletedFor, filename), nil
			}
		case query := <-downloadProgressQueryChan:
			// forward the query from outside to func downloadFile()
			progressQueryChan <- query
			response := <-progressQueryChan
			downloadProgressQueryChan <- response
		case <-cancelDownloadChan:
			p.filesInDownload.mu.Lock()

			delete(p.filesInDownload.files, file)

			p.filesInDownload.mu.Unlock()
			return "", fmt.Errorf("%s", downloadCanceledByUser)
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
		return nil, fmt.Errorf("%s: %s\n", resp.Body.Result.Code, resp.Body.Result.Detail)
	default:
		return nil, fmt.Errorf("%s %q\n", unrecognizedPeerTrackerResponseResultCode, resp.Body.Result.Code)
	}
}

func downloadFile(ctx context.Context, p *peer, fileToDownload fileID,
	result chan<- fileDownloadResult, progressQuery chan fileDownloadProgressQuery) {
	resp, err := findFile(p, fileToDownload)
	if err != nil {
		result <- fileDownloadResult{
			error: err,
		}
		return
	}

	// preallocate a file with size equal to the whole file
	file, err := os.Create(fileToDownload.name)
	if err != nil {
		result <- fileDownloadResult{
			error: err,
		}
		return
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(file)
	if err = file.Truncate(resp.Body.FileSize); err != nil {
		result <- fileDownloadResult{
			error: err,
		}
		return
	}

	p.filesToShare.mu.Lock()

	p.filesToShare.mu.Unlock()

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
	inTransmissionChunksByIndex := make(map[int64]struct{})
	for i := 0; i < workerCount; i++ {
		picked := pickChunk(chunkLocations, inTransmissionChunksByIndex)
		toDownloadChunkChan <- picked
		inTransmissionChunksByIndex[picked.index] = struct{}{}
	}

	// ticker to periodically update chunkLocations
	chunkLocationsUpdateInterval := 10 * time.Second
	chunkLocationsUpdateTicker := time.NewTicker(chunkLocationsUpdateInterval)
	defer chunkLocationsUpdateTicker.Stop()

	// wait for download completion
	completedChunksByIndex := make(map[int64]struct{})
	for {
		select {
		case <-ctx.Done():
			return
		case r := <-chunkDownloadResultChan:
			// remove chunk from in transmission set
			delete(inTransmissionChunksByIndex, r.index)

			if r.error != nil {
				// discard failed chunk
				infoLogger.Printf("%s (file: %q, chunk index: %v, host: %q) %s: %v",
					discardBadChunk, fileToDownload.name, r.index, r.hostPort, dueToError, r.error)

				// give a new chunk to a chunk download goroutine
				picked := pickChunk(chunkLocations, util.UnionInt64Set(inTransmissionChunksByIndex, completedChunksByIndex))
				toDownloadChunkChan <- picked
				inTransmissionChunksByIndex[picked.index] = struct{}{}
				continue
			}

			// write the chunk and if fails, discard the chunk
			// todo partiallyDownloaded mu.Lock()?
			if err := writeChunk(file, r.index, r.data); err != nil {
				result <- fileDownloadResult{
					error: fmt.Errorf("%s (file: %q, chunk index: %v) %s: %v",
						writeChunkFails, fileToDownload.name, r.index, dueToError, err),
				}
				return
			}

			// todo: new channel and go
			// todo: update partiallyDownloadedFiles mu.Lock()?
			// tell the tracker about the new chunk
			// try at most totalTry times if error occurs
			totalTry := 3
			var registerChunkErr error
			for i := 0; i < totalTry; i++ {
				registerChunkErr = registerChunk(p, fileToDownload, r.index)
				if registerChunkErr != nil {
					continue
				}
				break
			}
			if registerChunkErr != nil {
				result <- fileDownloadResult{
					error: err,
				}
				return
			}

			// add chunk to completed set
			completedChunksByIndex[r.index] = struct{}{}

			// if whole file download completed
			if len(completedChunksByIndex) == totalChunkCount {
				checksumToVerify, err := util.Sha256FileChecksum(file)
				if err != nil {
					result <- fileDownloadResult{
						error: err,
					}
					return
				}
				if checksumToVerify != fileToDownload.checksum {
					result <- fileDownloadResult{
						error: fmt.Errorf("%v\n", downloadedFileChecksumMismatch),
					}
					return
				}
				result <- fileDownloadResult{
					error: nil,
				}
				return
			}

			// else still chunks left to download
			picked := pickChunk(chunkLocations, util.UnionInt64Set(inTransmissionChunksByIndex, completedChunksByIndex))
			toDownloadChunkChan <- picked
			inTransmissionChunksByIndex[picked.index] = struct{}{}
		case <-chunkLocationsUpdateTicker.C:
			resp, err := findFile(p, fileToDownload)
			if err != nil {
				infoLogger.Printf("%s: %v", ignoreFailedUpdateToChunkLocations, err)
				continue
			}
			chunkLocations = resp.Body.ChunkLocations
		case <-progressQuery:
			progressQuery <- fileDownloadProgressQuery{
				completedChunksByIndex:      completedChunksByIndex,
				inTransmissionChunksByIndex: inTransmissionChunksByIndex,
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
		return nil, fmt.Errorf("%s: %s\n", resp.Body.Result.Code, resp.Body.Result.Detail)
	default:
		return nil, fmt.Errorf("%s %q\n", unrecognizedPeerPeerResponseResultCode, resp.Body.Result.Code)
	}
}

// registerChunk registers a file chunk with the tracker
func registerChunk(p *peer, file fileID, chunkIndex int64) error {
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
		return fmt.Errorf("%s: %s\n", resp.Body.Result.Code, resp.Body.Result.Detail)
	default:
		return fmt.Errorf("%s %q\n", unrecognizedPeerTrackerResponseResultCode, resp.Body.Result.Code)
	}

	return nil
}

func serveFiles(p *peer, l *net.Listener) {
	infoLogger.Printf("%s", startToServeFilesToPeers)

	for {
		conn, err := (*l).Accept()
		if err != nil {
			errorLogger.Printf("%v\n", err)
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
				switch req.Header.Operation {
				case communication.DownloadChunk:
					//todo: infoLogger.Printf("serving chunk xxx to xxx")
					//todo: p.filesToShare.mu.Lock() because register is changing this
					//todo: p.partiallyDownloadedFiles.mu.Lock() because download is changing this
				default:
					err = fmt.Errorf("%s %q.\n", unrecognizedPeerPeerOperation, req.Header.Operation)
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
