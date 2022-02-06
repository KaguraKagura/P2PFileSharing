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

//type chunk struct {
//	index    int64
//	hostPort string
//	data     []byte
//}

type toBeDownloadedChunk struct {
	index    int64
	hostPort string
}

type downloadedChunk struct {
	index int64
	data  []byte
}

type Peer struct {
	selfHostPort    string
	trackerHostPort string
	filesToShare    []localFile
	registered      bool
}

var self Peer
var genericLogger = log.New(os.Stdout, "", 0)
var infoLogger = log.New(os.Stdout, "INFO: ", 0)
var errorLogger = log.New(os.Stdout, "ERROR: ", 0)

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
		case register:
			if len(args) < 3 {
				errorLogger.Printf("%s. %s\n", badArguments, helpPrompt)
				continue
			}
			self.register(args[1], args[2], args[3:])
		case list:
			if len(args) != 1 {
				errorLogger.Printf("%s. %s\n", badArguments, helpPrompt)
				continue
			}
			self.list()
		case find:
			if len(args) != 3 {
				errorLogger.Printf("%s. %s\n", badArguments, helpPrompt)
				continue
			}
			self.find(args[1], args[2])
		case download:
			if len(args) != 3 {
				errorLogger.Printf("%s. %s\n", badArguments, helpPrompt)
				continue
			}
			go self.download(args[1], args[2])
		case h:
			fallthrough
		case help:
			genericLogger.Printf("%s\n", helpMessage)
		case q:
			fallthrough
		case quit:
			os.Exit(0)
		default:
			errorLogger.Printf("%s %q. %s\n", unrecognizedCommand, args[0], helpPrompt)
		}
	}
}

func (p *Peer) register(trackerHostPort, selfHostPort string, filepaths []string) {
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
	req, _ := json.Marshal(communication.RegisterRequest{
		Header: communication.PeerTrackerHeader{
			RequestId: requestId,
			Operation: communication.Register,
		},
		Body: communication.RegisterRequestBody{
			HostPort:     p.selfHostPort,
			FilesToShare: localFilesToP2PFiles(p.filesToShare),
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
		if err := conn.Close(); err != nil {
			errorLogger.Printf("%v\n", err)
		}
	}(conn)

	if _, err := conn.Write(req); err != nil {
		errorLogger.Printf("%v\n", err)
		return
	}

	// get response from tracker
	var resp communication.RegisterResponse
	d := json.NewDecoder(conn)
	if err := d.Decode(&resp); err != nil {
		errorLogger.Printf("%v\n", err)
		return
	}

	if err := validatePeerTrackerHeader(resp.Header, communication.PeerTrackerHeader{
		RequestId: requestId,
		Operation: communication.Register,
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
}

func (p *Peer) list() {
	if p.registered == false {
		errorLogger.Printf("%s\n", pleaseRegisterFirst)
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
		if err := conn.Close(); err != nil {
			errorLogger.Printf("%v\n", err)
		}
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

func (p *Peer) find(filename, checksum string) {
	if p.registered == false {
		errorLogger.Printf("%s\n", pleaseRegisterFirst)
	}

	if len(checksum) != util.Sha256ChecksumHexStringSize {
		errorLogger.Printf("%s", util.BadSha256ChecksumHexStringSize)
	}

	resp, err := findFile(p, remoteFile{
		name:     filename,
		checksum: checksum,
	})
	if err != nil {
		errorLogger.Printf("%v\n", err)
		return
	}

	switch resp.Body.Result.Code {
	case communication.Success:
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
	case communication.Fail:
		errorLogger.Printf("%s: %s\n", resp.Body.Result.Code, resp.Body.Result.Detail)
		return
	default:
		errorLogger.Printf("%s %q\n", unrecognizedPeerTrackerResponseResultCode, resp.Body.Result.Code)
		return
	}
}

func (p *Peer) download(filename, checksum string) {
	if p.registered == false {
		errorLogger.Printf("%s\n", pleaseRegisterFirst)
	}

	if len(checksum) != util.Sha256ChecksumHexStringSize {
		errorLogger.Printf("%s", util.BadSha256ChecksumHexStringSize)
	}

	fileToDownload := remoteFile{
		name:     filename,
		checksum: checksum,
	}
	resp, err := findFileSuccess(p, fileToDownload)
	if err != nil {
		errorLogger.Printf("%v\n", err)
		return
	}

	// preallocate a file
	file, err := os.Create(filename)
	if err != nil {
		errorLogger.Printf("%v\n", err)
		return
	}
	if err = file.Truncate(resp.Body.FileSize); err != nil {
		errorLogger.Printf("%v\n", err)
		return
	}

	// context to cancel all the goroutines created from this function when file download completes or error occurs
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// chunkLocations will be updated periodically in place
	chunkLocations := resp.Body.ChunkLocations
	// chunkInTransmissionByIndex is a set of index of chunks in transmission
	chunkInTransmissionByIndex := map[int64]struct{}{}

	// create concurrent download workers
	totalChunkCount := len(chunkLocations)
	workerCount := ParallelDownloadWorkerCount
	if totalChunkCount < ParallelDownloadWorkerCount {
		workerCount = totalChunkCount
	}
	toBeDownloadedChunkChan := make(chan toBeDownloadedChunk)
	downloadedChunkChan := make(chan downloadedChunk)
	downloadErrorChan := make(chan error)
	for i := 0; i < workerCount; i++ {
		go func(ctx context.Context, toBeDownloaded <-chan toBeDownloadedChunk, downloaded chan<- downloadedChunk, e chan<- error) {
			for {
				select {
				case <-ctx.Done():
					return
				case want := <-toBeDownloaded:
					get, err := downloadChunk(fileToDownload, want)
					if err != nil {
						e <- err
						continue
					}
					downloaded <- *get
				}
			}
		}(ctx, toBeDownloadedChunkChan, downloadedChunkChan, downloadErrorChan)
		// assign work to the worker just created
		c := pickChunk(chunkLocations, chunkInTransmissionByIndex)
		toBeDownloadedChunkChan <- c
		chunkInTransmissionByIndex[c.index] = struct{}{}
	}

	// update chunkLocations in place periodically in the background
	// due to the update, accessing chunkLocations needs chunkLocationsLock from here on
	updateInterval := 10 * time.Second
	chunkLocationsLock := sync.Mutex{}
	findFileErrorChan := make(chan error)
	go func(ctx context.Context, interval time.Duration, locations *communication.ChunkLocations, lock *sync.Mutex, e chan<- error) {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				resp, err := findFileSuccess(p, fileToDownload)
				if err != nil {
					e <- err
					return
				}
				// update locations in place
				lock.Lock()
				for i, location := range resp.Body.ChunkLocations {
					(*locations)[i] = location
				}
				lock.Unlock()
				e <- nil
			}
		}
	}(ctx, updateInterval, &chunkLocations, &chunkLocationsLock, findFileErrorChan)

	// monitor whole file download progress
	chunksCompletedByIndex := make(map[int64]struct{})
	for {
		select {
		case c := <-downloadedChunkChan:
			delete(chunkInTransmissionByIndex, c.index)
			chunksCompletedByIndex[c.index] = struct{}{}
			//todo: write chunk

			// whole file download completed
			if len(chunksCompletedByIndex) == totalChunkCount {
				checksumToVerify, err := util.Sha256FileChecksum(file)
				if err != nil {
					errorLogger.Printf("%v\n", err)
				}
				if checksumToVerify != checksum {
					errorLogger.Printf("%v\n", downloadedFileChecksumMismatch)
				}
				return
			}

			// still chunks left to be downloaded
			chunkLocationsLock.Lock()
			newChunk := pickChunk(chunkLocations, util.UnionInt64Set(chunkInTransmissionByIndex, chunksCompletedByIndex))
			chunkLocationsLock.Unlock()
			toBeDownloadedChunkChan <- newChunk
			chunkInTransmissionByIndex[newChunk.index] = struct{}{}
		case err := <-downloadErrorChan:
			// todo retry
			errorLogger.Printf("%v\n", err)
		case err := <-findFileErrorChan:
			// todo retry
			errorLogger.Printf("%v\n", err)
		}
	}
}

func findFile(p *Peer, file remoteFile) (*communication.FindFileResponse, error) {
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

	return &resp, nil
}

// findFileSuccess returns a FindFileResponse response with result code "Success" or else error
func findFileSuccess(p *Peer, file remoteFile) (*communication.FindFileResponse, error) {
	resp, err := findFile(p, file)
	if err != nil {
		return nil, err
	}

	switch resp.Body.Result.Code {
	case communication.Success:
		return resp, nil
	case communication.Fail:
		return nil, fmt.Errorf("%s: %s\n", resp.Body.Result.Code, resp.Body.Result.Detail)
	default:
		return nil, fmt.Errorf("%s %q\n", unrecognizedPeerTrackerResponseResultCode, resp.Body.Result.Code)
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
			FileName:   file.name,
			Checksum:   file.checksum,
			ChunkIndex: c.index,
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
