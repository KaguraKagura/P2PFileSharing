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

type remoteFile struct {
	name     string
	checksum string
}

type toBeDownloadedChunk struct {
	index    int64
	hostPort string
}

type downloadedChunk struct {
	index int64
	data  []byte
}

type failedChunk struct {
	chunk toBeDownloadedChunk
	error error
}

type fileDownloadResult struct {
	successful bool
	error      error
}

type fileDownloadJob struct {
	cancel chan<- struct{}
	// todo: things to help view download progress
}

type peer struct {
	selfHostPort    string
	trackerHostPort string
	// filesToShare maps localFile.fullPath to localFile
	filesToShare map[string]localFile
	registered   bool
}

var (
	self peer

	filesInDownload map[remoteFile]fileDownloadJob

	genericLogger = log.New(os.Stdout, "", 0)
	infoLogger    = log.New(os.Stdout, "INFO: ", 0)
	errorLogger   = log.New(os.Stdout, "ERROR: ", 0)
)

func Start() {
	genericLogger.Println(welcomeMessage)

	reader := bufio.NewReader(os.Stdin)

	for {
		command, _ := reader.ReadString('\n')
		command = strings.TrimSpace(command)
		if command == "" {
			continue
		}

		args := strings.Fields(command)
		switch args[0] {
		case registerCmd:
			if len(args) < 3 {
				errorLogger.Printf("%s. %s\n", badArguments, helpPrompt)
				continue
			}
			self.register(args[1], args[2], args[3:])
		case listCmd:
			if len(args) != 1 {
				errorLogger.Printf("%s. %s\n", badArguments, helpPrompt)
				continue
			}
			self.list()
		case findCmd:
			if len(args) != 3 {
				errorLogger.Printf("%s. %s\n", badArguments, helpPrompt)
				continue
			}
			self.find(args[1], args[2])
		case downloadCmd:
			if len(args) != 3 {
				errorLogger.Printf("%s. %s\n", badArguments, helpPrompt)
				continue
			}
			self.download(args[1], args[2])
		// todo: case showDownloadsCmd:
		// todo: case cancelDownloadCmd:
		case hCmd:
			fallthrough
		case helpCmd:
			genericLogger.Printf("%s\n", helpMessage)
		case qCmd:
			fallthrough
		case quitCmd:
			os.Exit(0)
		default:
			errorLogger.Printf("%s %q. %s\n", unrecognizedCommand, args[0], helpPrompt)
		}
	}
}

// todo: multiple register and effect on shared file
func (p *peer) register(trackerHostPort, selfHostPort string, filepaths []string) {
	for _, hp := range []string{trackerHostPort, selfHostPort} {
		if _, _, err := net.SplitHostPort(hp); err != nil {
			errorLogger.Printf("%s. %s\n", badIpPortArgument, helpPrompt)
			return
		}
	}

	if p.trackerHostPort != "" && p.trackerHostPort != trackerHostPort {
		infoLogger.Printf("%s %s.\n", alreadyUsingTrackerAt, p.trackerHostPort)
		return
	}

	if p.selfHostPort != "" && p.selfHostPort != selfHostPort {
		infoLogger.Printf("%s %s.\n", alreadyUsingHostPortAt, p.selfHostPort)
		return
	}

	localFiles, err := makeLocalFiles(filepaths)
	if err != nil {
		errorLogger.Printf("%v\n", err)
		return
	}

	p.trackerHostPort = trackerHostPort
	p.selfHostPort = selfHostPort
	p.filesToShare = localFiles

	// prepare to talk to the tracker
	requestId := uuid.NewString()
	req, _ := json.Marshal(communication.RegisterFileRequest{
		Header: communication.PeerTrackerHeader{
			RequestId: requestId,
			Operation: communication.RegisterFile,
		},
		Body: communication.RegisterFileRequestBody{
			HostPort:     p.selfHostPort,
			FilesToShare: localFilesMapToP2PFiles(p.filesToShare),
		},
	})

	// start to talk to the tracker
	dialer := net.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.Dial("tcp", p.trackerHostPort)
	if err != nil {
		errorLogger.Printf("%v\n", err)
		return
	}
	defer func(conn net.Conn) {
		_ = conn.Close()
	}(conn)

	if _, err := conn.Write(req); err != nil {
		errorLogger.Printf("%v\n", err)
		return
	}

	// get response from tracker
	var resp communication.RegisterFileResponse
	d := json.NewDecoder(conn)
	if err := d.Decode(&resp); err != nil {
		errorLogger.Printf("%v\n", err)
		return
	}

	if err := validatePeerTrackerHeader(resp.Header, communication.PeerTrackerHeader{
		RequestId: requestId,
		Operation: communication.RegisterFile,
	}); err != nil {
		errorLogger.Printf("%v\n", err)
		return
	}

	switch resp.Body.Result.Code {
	case communication.Success:
		infoLogger.Printf("%s: %s\n", resp.Body.Result.Code, resp.Body.Result.Detail)
		registeredFiles := resp.Body.RegisteredFiles
		if registeredFiles != nil {
			genericLogger.Printf("%s:\n", registeredFilesAre)
			for _, f := range registeredFiles {
				util.PrettyLogStruct(genericLogger, f)
			}
		}
	case communication.Fail:
		errorLogger.Printf("%s: %s\n", resp.Body.Result.Code, resp.Body.Result.Detail)
		return
	default:
		errorLogger.Printf("%s %q\n", unrecognizedPeerTrackerResponseResultCode, resp.Body.Result.Code)
		return
	}

	p.registered = true

	// todo: go serve files
	go func() {

	}()
}

func (p *peer) list() {
	if p.registered == false {
		errorLogger.Printf("%s\n", pleaseRegisterFirst)
		return
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
		errorLogger.Printf("%v\n", err)
		return
	}
	defer func(conn net.Conn) {
		_ = conn.Close()
	}(conn)

	if _, err := conn.Write(req); err != nil {
		errorLogger.Printf("%v\n", err)
		return
	}

	// get response from tracker
	var resp communication.ListFileResponse
	d := json.NewDecoder(conn)
	if err := d.Decode(&resp); err != nil {
		errorLogger.Printf("%v\n", err)
		return
	}

	if err := validatePeerTrackerHeader(resp.Header, communication.PeerTrackerHeader{
		RequestId: requestId,
		Operation: communication.List,
	}); err != nil {
		errorLogger.Printf("%v\n", err)
		return
	}

	switch resp.Body.Result.Code {
	case communication.Success:
		infoLogger.Printf("%s: %s\n", resp.Body.Result.Code, resp.Body.Result.Detail)
		files := resp.Body.Files
		if files != nil {
			sort.Slice(files, func(i, j int) bool {
				return files[i].Name < files[j].Name
			})
			genericLogger.Printf("%s:\n", availableFilesAre)
			for _, f := range files {
				util.PrettyLogStruct(genericLogger, f)
			}
		} else {
			genericLogger.Printf("%s\n", noAvailableFileRightNow)
		}
	case communication.Fail:
		errorLogger.Printf("%s: %s\n", resp.Body.Result.Code, resp.Body.Result.Detail)
		return
	default:
		errorLogger.Printf("%s %q\n", unrecognizedPeerTrackerResponseResultCode, resp.Body.Result.Code)
		return
	}
}

func (p *peer) find(filename, checksum string) {
	if p.registered == false {
		errorLogger.Printf("%s\n", pleaseRegisterFirst)
		return
	}

	if len(checksum) != util.Sha256ChecksumHexStringSize {
		errorLogger.Printf("%s", util.BadSha256ChecksumHexStringSize)
		return
	}

	resp, err := findFile(p, remoteFile{
		name:     filename,
		checksum: checksum,
	})
	if err != nil {
		errorLogger.Printf("%v\n", err)
		return
	}

	infoLogger.Printf("%s: %s\n", resp.Body.Result.Code, resp.Body.Result.Detail)
	chunkLocations := resp.Body.ChunkLocations
	if chunkLocations == nil {
		errorLogger.Printf("%s\n", badTrackerResponse)
		return
	}
	for chunkIndex, chunkLocation := range chunkLocations {
		locations := make([]string, 0)
		for l := range chunkLocation {
			locations = append(locations, l)
		}
		genericLogger.Printf("Chunk %d is at: %s", chunkIndex, strings.Join(locations, " "))
	}
}

func (p *peer) download(filename, checksum string) {
	if p.registered == false {
		errorLogger.Printf("%s\n", pleaseRegisterFirst)
		return
	}

	if len(checksum) != util.Sha256ChecksumHexStringSize {
		errorLogger.Printf("%s", util.BadSha256ChecksumHexStringSize)
		return
	}

	infoLogger.Printf("%s %q %s\n", beginDownloading, filename, inTheBackground)

	file := remoteFile{
		name:     filename,
		checksum: checksum,
	}

	//todo: lock filesInDownload here and in func cancelDownload?
	cancelDownloadJobChan := make(chan struct{})
	filesInDownload[file] = fileDownloadJob{
		cancel: cancelDownloadJobChan,
	}

	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		downloadResult := make(chan fileDownloadResult)
		go downloadFile(ctx, p, file, downloadResult)

		for {
			select {
			case <-cancelDownloadJobChan:
				//todo: remove entry in filesInDownload (Lock?) here
				return
			case result := <-downloadResult:
				if result.successful == true {
					infoLogger.Printf("%s %q\n", downloadCompletedFor, filename)
				} else {
					errorLogger.Printf("%s %q: %v\n", failToDownload, filename, result.error)
				}
				//todo: remove entry in filesInDownload (Lock?) here
				return
			}
		}
	}()
}

func findFile(p *peer, file remoteFile) (*communication.FindFileResponse, error) {
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

func downloadFile(ctx context.Context, p *peer, fileToDownload remoteFile, result chan<- fileDownloadResult) {
	resp, err := findFile(p, fileToDownload)
	if err != nil {
		result <- fileDownloadResult{
			successful: false,
			error:      err,
		}
		return
	}

	// preallocate a file with size equal to the whole file
	file, err := os.Create(fileToDownload.name)
	if err != nil {
		result <- fileDownloadResult{
			successful: false,
			error:      err,
		}
		return
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(file)
	if err = file.Truncate(resp.Body.FileSize); err != nil {
		result <- fileDownloadResult{
			successful: false,
			error:      err,
		}
		return
	}

	// chunkLocations will be updated periodically in place in the background
	// due to the update, accessing chunkLocations needs chunkLocationsLock
	chunkLocations := resp.Body.ChunkLocations
	chunkLocationsLock := new(sync.Mutex)

	// "update chunkLocations" goroutine
	updateInterval := 10 * time.Second
	updateChunkLocationsErrorChan := make(chan error)
	go func(ctx context.Context, interval time.Duration, locations *communication.ChunkLocations, locationsLock *sync.Mutex, e chan<- error) {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				resp, err := findFile(p, fileToDownload)
				if err != nil {
					e <- err
					continue
				}
				// update locations in place
				locationsLock.Lock()
				for i, location := range resp.Body.ChunkLocations {
					(*locations)[i] = location
				}
				locationsLock.Unlock()
			}
		}
	}(ctx, updateInterval, &chunkLocations, chunkLocationsLock, updateChunkLocationsErrorChan)

	// "chunk download" goroutines
	toBeDownloadedChunkChan := make(chan toBeDownloadedChunk)
	downloadedChunkChan := make(chan downloadedChunk)
	failedChunkChan := make(chan failedChunk)

	totalChunkCount := len(chunkLocations)
	workerCount := parallelDownloadWorkerCount
	if totalChunkCount < parallelDownloadWorkerCount {
		workerCount = totalChunkCount
	}

	for i := 0; i < workerCount; i++ {
		go func(ctx context.Context, toBeDownloaded <-chan toBeDownloadedChunk, downloaded chan<- downloadedChunk, failed chan<- failedChunk) {
			for {
				select {
				case <-ctx.Done():
					return
				case want := <-toBeDownloaded:
					get, err := downloadChunk(fileToDownload, want)
					if err != nil {
						failed <- failedChunk{
							chunk: want,
							error: err,
						}
						continue
					}
					downloaded <- *get
				}
			}
		}(ctx, toBeDownloadedChunkChan, downloadedChunkChan, failedChunkChan)
	}

	// "pick chunk" goroutine to assign work to chunk download goroutines
	completedChunksByIndexChan := make(chan map[int64]struct{})
	go pickChunk(ctx, chunkLocations, chunkLocationsLock, completedChunksByIndexChan, toBeDownloadedChunkChan)

	// assign initial work to every chunk download goroutine
	for i := 0; i < workerCount; i++ {
		completedChunksByIndexChan <- nil
	}

	// monitor whole file download progress
	chunksCompletedByIndex := make(map[int64]struct{})
	for {
		select {
		case <-ctx.Done():
			return
		case c := <-downloadedChunkChan:
			// write the chunk and if fails, effectively discard and retry
			if err := writeChunk(file, c); err != nil {
				infoLogger.Printf("%s (file: %q, chunk index: %q) %s: %v\n",
					retryDownloadChunk, fileToDownload.name, c.index, dueToError, err)
				continue
			}

			chunksCompletedByIndex[c.index] = struct{}{}

			// todo: new channel and go
			// tell the tracker about the new chunk
			// try at most totalTry times if error occurs
			totalTry := 3
			var registerChunkErr error
			for i := 0; i < totalTry; i++ {
				registerChunkErr = registerChunk(p, fileToDownload, c.index)
				if registerChunkErr != nil {
					continue
				}
				break
			}
			if registerChunkErr != nil {
				result <- fileDownloadResult{
					successful: false,
					error:      err,
				}
				return
			}

			// whole file download completed
			if len(chunksCompletedByIndex) == totalChunkCount {
				checksumToVerify, err := util.Sha256FileChecksum(file)
				if err != nil {
					result <- fileDownloadResult{
						successful: false,
						error:      err,
					}
					return
				}
				if checksumToVerify != fileToDownload.checksum {
					result <- fileDownloadResult{
						successful: false,
						error:      fmt.Errorf("%v\n", downloadedFileChecksumMismatch),
					}
					return
				}
				// close result to show download completes without error
				result <- fileDownloadResult{
					successful: true,
					error:      nil,
				}
				return
			}
			// still chunks left to download
			// send to completedChunksByIndexChan to tell "pick chunk" goroutine to pick a new chunk for "chunk download" goroutines
			completedChunksByIndexChan <- chunksCompletedByIndex
		case c := <-failedChunkChan:
			// retry failed chunk
			completedChunksByIndexChan <- chunksCompletedByIndex
			infoLogger.Printf("%s (file: %q, chunk index: %q, host: %q) %s: %v\n",
				retryDownloadChunk, fileToDownload.name, c.chunk.index, c.chunk.hostPort, dueToError, c.error)
		case err := <-updateChunkLocationsErrorChan:
			// ignore the failed update to chunkLocations
			infoLogger.Printf("%s %s: %v\n", ignoreFailedUpdateToChunkLocations, dueToError, err)
		}
	}
}

// downloadChunk downloads a chunk from a peer and validates the checksum
func downloadChunk(file remoteFile, c toBeDownloadedChunk) (*downloadedChunk, error) {
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
			return nil, fmt.Errorf("%s: expect %q get %q",
				downloadedChunkIndexMismatch, c.index, resp.Body.ChunkIndex)
		}

		digest := sha256.Sum256(data)
		checksum := hex.EncodeToString(digest[:])
		if checksum != resp.Body.ChunkChecksum {
			return nil, fmt.Errorf("%s: expect %q get %q",
				downloadedChunkChecksumMismatch, resp.Body.ChunkChecksum, checksum)
		}

		return &downloadedChunk{
			index: index,
			data:  data,
		}, nil
	case communication.Fail:
		return nil, fmt.Errorf("%s: %s\n", resp.Body.Result.Code, resp.Body.Result.Detail)
	default:
		return nil, fmt.Errorf("%s %q\n", unrecognizedPeerPeerResponseResultCode, resp.Body.Result.Code)
	}
}

// registerChunk registers a file chunk with the tracker
func registerChunk(p *peer, file remoteFile, chunkIndex int64) error {
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
