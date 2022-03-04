package peer

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

type chunkDownloadResult struct {
	error    error
	hostPort string
	index    int
	data     []byte
}

type fileDownloadJobControl struct {
	cancel chan<- struct{}
	pause  chan<- bool
}

type fileDownloadResult struct {
	error error
	f     localFile
}

type fileID struct {
	name     string
	checksum string
}

type filesInTransmission struct {
	files       map[fileID]*partiallyDownloadedFile
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

type localFile struct {
	name     string
	fullPath string
	checksum string
	size     int64
}

type partiallyDownloadedFile struct {
	fp                     *os.File
	completedChunksByIndex map[int]struct{}
	paused                 bool
}

type peer struct {
	selfHostPort    string
	trackerHostPort string
	registered      bool
	servingFiles    bool

	filesToShare        filesToShare
	filesInTransmission filesInTransmission
}

type toBeDownloadedChunk struct {
	index    int
	hostPort string
}

type fileDownloadControlOp int

const (
	pauseDownloadOp  fileDownloadControlOp = 0
	resumeDownloadOp fileDownloadControlOp = 1
	cancelDownloadOp fileDownloadControlOp = 2
)

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
				break
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
			case showDownloadsCmd:
				if len(args) != 1 {
					err = fmt.Errorf("%s. %s", badArguments, helpPrompt)
					break
				}
				result, err = self.showDownloads()
			case pauseDownloadCmd:
				if len(args) != 3 {
					err = fmt.Errorf("%s. %s", badArguments, helpPrompt)
					break
				}
				result, err = self.pauseDownload(args[1], args[2])
			case resumeDownloadCmd:
				if len(args) != 3 {
					err = fmt.Errorf("%s. %s", badArguments, helpPrompt)
					break
				}
				result, err = self.resumeDownload(args[1], args[2])
			case cancelDownloadCmd:
				if len(args) != 3 {
					err = fmt.Errorf("%s. %s", badArguments, helpPrompt)
					break
				}
				result, err = self.cancelDownload(args[1], args[2])
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
				builder.WriteString(fmt.Sprintf("%s\n", util.StructToPrettyJsonString(f)))
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

	if !p.servingFiles {
		l, err := net.Listen("tcp", selfHostPort)
		if err != nil {
			return "", err
		}
		go serveFiles(p, l)

		p.servingFiles = true
	}
	return result, nil
}

// list lists the available files in the tracker
func (p *peer) list() (string, error) {
	if !p.registered {
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
				builder.WriteString(fmt.Sprintf("%s\n", util.StructToPrettyJsonString(f)))
			}
		} else {
			builder.WriteString(noFileIsAvailableRightNow)
		}
		return builder.String(), nil
	case communication.Fail:
		return "", fmt.Errorf("%s: %s", resp.Body.Result.Code, resp.Body.Result.Detail)
	default:
		return "", fmt.Errorf("%s %q", unrecognizedPeerTrackerResponseResultCode, resp.Body.Result.Code)
	}
}

// find finds the information of a file from the tracker
func (p *peer) find(filename, checksum string) (string, error) {
	if !p.registered {
		return "", fmt.Errorf("%s", pleaseRegisterFirst)
	}

	if len(checksum) != sha256ChecksumHexStringSize {
		return "", fmt.Errorf("%s", badSha256ChecksumHexStringSize)
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
		builder.WriteString("\n")
	}

	return builder.String(), nil
}

// download downloads a file from a peer
func (p *peer) download(filename, checksum string) (string, error) {
	if !p.registered {
		return "", fmt.Errorf("%s", pleaseRegisterFirst)
	}

	if len(checksum) != sha256ChecksumHexStringSize {
		return "", fmt.Errorf("%s", badSha256ChecksumHexStringSize)
	}

	infoLogger.Printf("%s %q %s", beginDownloading, filename, inTheBackground)

	file := fileID{
		name:     filename,
		checksum: checksum,
	}

	// update p.filesInTransmission
	p.filesInTransmission.mu.Lock()

	if p.filesInTransmission.files == nil {
		p.filesInTransmission.files = make(map[fileID]*partiallyDownloadedFile)
	}
	if p.filesInTransmission.jobControls == nil {
		p.filesInTransmission.jobControls = make(map[fileID]fileDownloadJobControl)
	}

	p.filesInTransmission.files[file] = &partiallyDownloadedFile{
		fp:                     nil,
		completedChunksByIndex: make(map[int]struct{}),
		paused:                 false,
	}

	// next 2 channels are accessible to outside functions to query download progress
	cancelDownloadChan := make(chan struct{})
	pauseDownloadChan := make(chan bool)
	p.filesInTransmission.jobControls[file] = fileDownloadJobControl{
		cancel: cancelDownloadChan,
		pause:  pauseDownloadChan,
	}

	p.filesInTransmission.mu.Unlock()

	// start download
	// next 2 channels are only accessible to the current function and its callee
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resultChan := make(chan fileDownloadResult)
	pauseChan := make(chan bool)
	go downloadFile(ctx, p, file, resultChan, pauseChan)

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
		case <-cancelDownloadChan:
			cancel()
			<-resultChan
			p.filesInTransmission.mu.Lock()

			delete(p.filesInTransmission.files, file)
			delete(p.filesInTransmission.jobControls, file)

			p.filesInTransmission.mu.Unlock()
			return fmt.Sprintf("%s %q", downloadCanceledFor, filename), nil
		case pause := <-pauseDownloadChan:
			pauseChan <- pause
		}
	}
}

// showDownloads shows what files are being downloaded right now
func (p *peer) showDownloads() (string, error) {
	p.filesInTransmission.mu.Lock()
	defer p.filesInTransmission.mu.Unlock()

	if len(p.filesInTransmission.files) == 0 {
		return noFileIsBeingDownloaded, nil
	}

	type downloadProgress struct {
		FileName              string
		Checksum              string
		CompletedChunkIndexes string
		Paused                bool
	}

	progresses := make([]downloadProgress, 0)
	for id, file := range p.filesInTransmission.files {
		indexes := make([]int, 0)
		for index := range file.completedChunksByIndex {
			indexes = append(indexes, index)
		}
		sort.Ints(indexes)
		progresses = append(progresses, downloadProgress{
			FileName:              id.name,
			Checksum:              id.checksum,
			CompletedChunkIndexes: strings.Join(strings.Fields(fmt.Sprint(indexes)), ", "),
			Paused:                file.paused,
		})

	}

	var builder strings.Builder
	for _, p := range progresses {
		builder.WriteString(fmt.Sprintf("%s\n", util.StructToPrettyJsonString(p)))
	}
	return builder.String(), nil
}

//pauseDownload pauses the downloading of a file
func (p *peer) pauseDownload(filename, checksum string) (string, error) {
	return controlDownload(p, filename, checksum, pauseDownloadOp)
}

//resumeDownload resumes the downloading of a file
func (p *peer) resumeDownload(filename, checksum string) (string, error) {
	return controlDownload(p, filename, checksum, resumeDownloadOp)
}

//cancelDownload cancels the downloading of a file
func (p *peer) cancelDownload(filename, checksum string) (string, error) {
	return controlDownload(p, filename, checksum, cancelDownloadOp)
}

// controlDownload controls the downloading of a file according to fileDownloadControlOp
func controlDownload(p *peer, filename, checksum string, op fileDownloadControlOp) (string, error) {
	if len(checksum) != sha256ChecksumHexStringSize {
		return "", fmt.Errorf("%s", badSha256ChecksumHexStringSize)
	}

	p.filesInTransmission.mu.Lock()
	defer p.filesInTransmission.mu.Unlock()

	control, ok := p.filesInTransmission.jobControls[fileID{
		name:     filename,
		checksum: checksum,
	}]
	if !ok {
		return "", fmt.Errorf("%s", noSuchFileIsBeingDownloaded)
	}

	switch op {
	case pauseDownloadOp:
		control.pause <- true
		return fmt.Sprintf("%s %q", pausingDownloadFor, filename), nil
	case resumeDownloadOp:
		control.pause <- false
		return fmt.Sprintf("%s %q", resumingDownloadFor, filename), nil
	case cancelDownloadOp:
		control.cancel <- struct{}{}
		return fmt.Sprintf("%s %q", cancelingDownloadFor, filename), nil
	default:
		return "", fmt.Errorf("%s", unsupportedFileDownloadControlOp)
	}
}

// findFile finds the information of a file from the tracker
func findFile(p *peer, file fileID) (*communication.FindFileResponse, error) {
	checksum := file.checksum
	if len(checksum) != sha256ChecksumHexStringSize {
		return nil, fmt.Errorf("%s", badSha256ChecksumHexStringSize)
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

// downloadFile downloads a file from peers and validates the file checksum
func downloadFile(ctx context.Context, p *peer, file fileID,
	result chan<- fileDownloadResult, pauseDownload <-chan bool) {
	resp, err := findFile(p, file)
	if err != nil {
		result <- fileDownloadResult{
			error: err,
			f:     localFile{},
		}
		return
	}

	// preallocate a file with size equal to the whole file
	fp, err := os.Create(file.name)
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

	p.filesInTransmission.mu.Lock()

	p.filesInTransmission.files[file].fp = fp

	p.filesInTransmission.mu.Unlock()

	// start to download
	var chunkLocations = resp.Body.ChunkLocations

	// "chunk download" goroutines
	toDownloadChunkChan := make(chan toBeDownloadedChunk)
	chunkDownloadResultChan := make(chan chunkDownloadResult)

	totalChunkCount := len(chunkLocations)
	workerCount := parallelDownloadWorkerCount
	if totalChunkCount < parallelDownloadWorkerCount {
		workerCount = totalChunkCount
	}

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		i := i
		go func(ctx context.Context, toDownload <-chan toBeDownloadedChunk, result chan<- chunkDownloadResult) {
			infoLogger.Printf("download worker %d starts for %q", i, file.name)
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				case want := <-toDownload:
					r := chunkDownloadResult{
						hostPort: want.hostPort,
						index:    want.index,
					}
					if data, err := downloadChunk(file, want); err != nil {
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
		picked := pickChunk(chunkLocations, inTransmissionChunksByIndex)
		toDownloadChunkChan <- *picked
		inTransmissionChunksByIndex[picked.index] = struct{}{}
	}

	// ticker to periodically update chunkLocations
	chunkLocationsUpdateInterval := 10 * time.Second
	chunkLocationsUpdateTicker := time.NewTicker(chunkLocationsUpdateInterval)
	defer chunkLocationsUpdateTicker.Stop()

	// wait for download completion
	paused := false
	completedChunksByIndex := make(map[int]struct{})
	timesFailed := 0
	timesFailedLimit := 5

	for {
		select {
		// download needs to be canceled
		case <-ctx.Done():
			c := make(chan struct{})
			// this func is to unblock all the chunk download goroutines so that they can be canceled by ctx
			go func() {
				for {
					select {
					case <-c:
						return
					case <-chunkDownloadResultChan:
						// receive from the channel and discard
					}
				}
			}()

			// wait for every chunk download goroutine to return
			wg.Wait()
			// when every chunk download goroutine is done, tell the "unblock" goroutine above to return
			c <- struct{}{}

			//close result to show we canceled the download
			close(result)
			return
		// download needs to be paused
		case paused = <-pauseDownload:
			p.filesInTransmission.mu.Lock()

			p.filesInTransmission.files[file].paused = paused

			p.filesInTransmission.mu.Unlock()

			if paused {
				infoLogger.Printf("%s %q", downloadingPausedFor, file.name)
			} else {
				infoLogger.Printf("%s %q", downloadingResumedFor, file.name)
			}
		default:
			if paused {
				continue
			}

			select {
			// a chunk download goroutine produces a new result
			case r := <-chunkDownloadResultChan:
				// remove chunk from in transmission set
				delete(inTransmissionChunksByIndex, r.index)

				if r.error != nil {
					timesFailed++
					if timesFailed > timesFailedLimit {
						result <- fileDownloadResult{
							error: fmt.Errorf("%s %q", tooManyFailedChunksDuringDownloading, file.name),
							f:     localFile{},
						}
						return
					}

					infoLogger.Printf("discard %q chunk index %d from %q: %v", file.name, r.index, r.hostPort, r.error)

					// discard failed chunk implicitly
					// give a new chunk to a chunk download goroutine
					picked := pickChunk(chunkLocations, util.UnionIntSet(inTransmissionChunksByIndex, completedChunksByIndex))
					toDownloadChunkChan <- *picked
					inTransmissionChunksByIndex[picked.index] = struct{}{}
					continue
				}

				// write the chunk
				// need to lock because writeChunk shares the same fp of type *os.File
				// with serving of this partially downloaded file to a peer
				p.filesInTransmission.mu.Lock()

				if err := writeChunk(fp, r.index, r.data); err != nil {
					result <- fileDownloadResult{
						error: fmt.Errorf("%s %q chunk index %d: %w",
							writeChunkFailsFor, file.name, r.index, err),
						f: localFile{},
					}
					return
				}

				p.filesInTransmission.mu.Unlock()

				// tell the tracker about the new chunk
				if err := registerChunk(p, file, r.index); err != nil {
					result <- fileDownloadResult{
						error: err,
						f:     localFile{},
					}
					return
				}

				// add chunk to completed set
				completedChunksByIndex[r.index] = struct{}{}

				// add chunk to p.filesInTransmission
				p.filesInTransmission.mu.Lock()

				p.filesInTransmission.files[file].completedChunksByIndex[r.index] = struct{}{}

				p.filesInTransmission.mu.Unlock()

				infoLogger.Printf("downloaded %q chunk index %d from %s", file.name, r.index, r.hostPort)

				// if whole file download completed
				if len(completedChunksByIndex) == totalChunkCount {
					checksumToVerify, err := sha256FileChecksum(fp)
					if err != nil {
						result <- fileDownloadResult{
							error: err,
							f:     localFile{},
						}
						return
					}
					if checksumToVerify != file.checksum {
						result <- fileDownloadResult{
							error: fmt.Errorf("%v", downloadedFileChecksumMismatch),
							f:     localFile{},
						}
						return
					}
					result <- fileDownloadResult{
						error: nil,
						f: localFile{
							name:     file.name,
							fullPath: file.name,
							checksum: file.checksum,
							size:     fileSize,
						},
					}
					return
				}

				// else still chunks left to download
				picked := pickChunk(chunkLocations, util.UnionIntSet(inTransmissionChunksByIndex, completedChunksByIndex))
				if picked == nil {
					// no op if no chunk is picked
					// this occurs when all the chunks of a file are either completed or in transmission,
					// and so, no new chunk needs to be picked or given to a chunk download goroutine
					continue
				}
				toDownloadChunkChan <- *picked
				inTransmissionChunksByIndex[picked.index] = struct{}{}
			// update chunkLocations periodically
			case <-chunkLocationsUpdateTicker.C:
				resp, err := findFile(p, file)
				if err != nil {
					infoLogger.Printf("%s: %v", ignoreFailedUpdateToChunkLocations, err)
					continue
				}
				chunkLocations = resp.Body.ChunkLocations
			}
		}
	}
}

// downloadChunk downloads a chunk from a peer and validates the chunk checksum
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

// serveFiles serves local files to peers
func serveFiles(p *peer, l net.Listener) {
	infoLogger.Printf("%s", readyToAcceptPeerConnections)

	for {
		conn, err := l.Accept()
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

// getChunk gets a chunk from local files
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

		if n, err := fp.ReadAt(data, int64(offset)); err != nil {
			if err == io.EOF {
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
			if n, err := f.fp.ReadAt(data, int64(offset)); err != nil {
				if err == io.EOF {
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
