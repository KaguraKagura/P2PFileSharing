package peer

import (
	"bufio"
	"context"
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

type chunkToDownload struct {
	index    int64
	hostPort string
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

	localFiles, err := validateFilepaths(filepaths)
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
		Header: communication.Header{
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

	if err := validateResponseHeader(resp.Header, communication.Header{
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
		errorLogger.Printf("%s %q\n", unrecognizedResponseResultCode, resp.Body.Result.Code)
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
		Header: communication.Header{
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

	if err := validateResponseHeader(resp.Header, communication.Header{
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
		errorLogger.Printf("%s %q\n", unrecognizedResponseResultCode, resp.Body.Result.Code)
		return
	}

}

func (p *Peer) find(filename, checksum string) {
	if p.registered == false {
		errorLogger.Printf("%s\n", pleaseRegisterFirst)
	}

	resp, err := findFile(p, filename, checksum)
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
		errorLogger.Printf("%s %q\n", unrecognizedResponseResultCode, resp.Body.Result.Code)
		return
	}
}

func (p *Peer) download(filename, checksum string) {
	if p.registered == false {
		errorLogger.Printf("%s\n", pleaseRegisterFirst)
	}

	resp, err := findFileSuccess(p, filename, checksum)
	if err != nil {
		errorLogger.Printf("%v\n", err)
		return
	}

	chunkLocationsLock := sync.Mutex{}
	chunkLocations := resp.Body.ChunkLocations
	go func(chunkLocations communication.ChunkLocations, lock *sync.Mutex) {
		lock.Lock()
		defer lock.Unlock()
		// todo: update chunkLocations
		// todo: experiment  chunkLocations = new slice

	}(chunkLocations, &chunkLocationsLock)

	var workerCount = PARALLEL_DOWNLOAD_WORKER_COUNT
	if len(resp.Body.ChunkLocations) < PARALLEL_DOWNLOAD_WORKER_COUNT {
		workerCount = len(chunkLocations)
	}

	//todo: need a file writer
	chunkToDownloadChan := make(chan chunkToDownload, 1) //todo chan size
	completedChunkIndexChan := make(chan int64)          //todo chan size

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for i := 0; i < workerCount; i++ {
		go func(ctx context.Context, need <-chan chunkToDownload, completed chan<- int64) {
			for {
				select {
				case <-ctx.Done():
					return
				case chunk := <-need:
					// todo: func downloadChunk() validate chunk checksum
					completed <- chunk.index
				}
			}
		}(ctx, chunkToDownloadChan, completedChunkIndexChan)
	}

	chunksDownloaded := make(map[int64]struct{}, 0)
	for len(chunksDownloaded) != len(chunkLocations) {
		index := <-completedChunkIndexChan
		chunksDownloaded[index] = struct{}{}
		chunkLocationsLock.Lock()
		chunkToDownloadChan <- pickChunk(chunkLocations, chunksDownloaded)
		chunkLocationsLock.Unlock()
		//todo: more actions?
	}
}

func findFile(p *Peer, filename, checksum string) (*communication.FindFileResponse, error) {
	if len(checksum) != sha256ChecksumHexStringSize {
		return nil, fmt.Errorf("%s", badSha256ChecksumHexStringSize)
	}

	requestId := uuid.NewString()
	req, _ := json.Marshal(communication.FindFileRequest{
		Header: communication.Header{
			RequestId: requestId,
			Operation: communication.Find,
		},
		Body: communication.FindFileRequestBody{
			FileName: filename,
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
		if err := conn.Close(); err != nil {
			errorLogger.Printf("%v\n", err)
		}
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

	if err := validateResponseHeader(resp.Header, communication.Header{
		RequestId: requestId,
		Operation: communication.Find,
	}); err != nil {
		errorLogger.Printf("%v\n", err)
	}

	return &resp, nil
}

// findFileSuccess returns a FindFileResponse response with result code "Success" or else error
func findFileSuccess(p *Peer, filename, checksum string) (*communication.FindFileResponse, error) {
	resp, err := findFile(p, filename, checksum)
	if err != nil {
		return nil, err
	}

	switch resp.Body.Result.Code {
	case communication.Success:
		return resp, nil
	case communication.Fail:
		return nil, fmt.Errorf("%s: %s\n", resp.Body.Result.Code, resp.Body.Result.Detail)
	default:
		return nil, fmt.Errorf("%s %q\n", unrecognizedResponseResultCode, resp.Body.Result.Code)
	}
}
