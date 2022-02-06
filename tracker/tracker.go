package tracker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"

	"Lab1/communication"
	"Lab1/util"
)

type remoteFile struct {
	name     string
	checksum string
}

type remoteFileStatus struct {
	size int64
	// chunkLocations: for each (chunk) index is a set of host:port containing the chunk
	chunkLocations []map[string]struct{}
}

type Tracker struct {
	hostPort  string
	listener  *net.Listener
	listening bool

	remoteFileLocations map[remoteFile]remoteFileStatus
}

var (
	tracker              Tracker
	remoteFileStatusLock sync.Mutex

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

		args := strings.Split(command, " ")
		switch args[0] {
		case start:
			if len(args) != 2 {
				errorLogger.Printf("%s. %s\n", badArguments, helpPrompt)
				continue
			}
			tracker.start(args[1])
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

func (t *Tracker) start(hostPort string) {
	if t.listening == true {
		infoLogger.Printf("%s %s\n", trackerAlreadyRunningAt, t.hostPort)
		return
	}

	if _, _, err := net.SplitHostPort(hostPort); err != nil {
		errorLogger.Printf("%s. %s\n", badIpPortArgument, helpPrompt)
		return
	}

	t.hostPort = hostPort

	l, err := net.Listen("tcp", hostPort)
	if err != nil {
		errorLogger.Printf("%v\n", err)
		return
	}
	t.listener = &l
	t.listening = true
	t.remoteFileLocations = make(map[remoteFile]remoteFileStatus)
	infoLogger.Printf("%s %s\n", trackerOnlineListeningOn, hostPort)

	go serve()
}

func serve() {
	for {
		conn, err := (*tracker.listener).Accept()
		if err != nil {
			errorLogger.Printf("%v\n", err)
			continue
		}

		go func() {
			defer func() {
				if err := conn.Close(); err != nil {
					errorLogger.Printf("%v\n", err)
				}
			}()

			var req genericRequest
			d := json.NewDecoder(conn)
			if err := d.Decode(&req); err != nil {
				errorLogger.Printf("%v\n", err)
				return
			}

			var (
				resp []byte
				err  error
			)
			switch req.Header.Operation {
			case communication.Register:
				var body communication.RegisterRequestBody
				if err = json.Unmarshal(req.Body, &body); err != nil {
					break
				}
				if resp, err = tracker.handleRegister(communication.RegisterRequest{
					Header: req.Header,
					Body:   body,
				}); err != nil {
					break
				}
			case communication.List:
				var body communication.ListFileRequestBody
				if err = json.Unmarshal(req.Body, &body); err != nil {
					break
				}
				if resp, err = tracker.handleList(communication.ListFileRequest{
					Header: req.Header,
					Body:   body,
				}); err != nil {
					break
				}
			case communication.Find:
				var body communication.FindFileRequestBody
				if err = json.Unmarshal(req.Body, &body); err != nil {
					break
				}
				if resp, err = tracker.handleFind(communication.FindFileRequest{
					Header: req.Header,
					Body:   body,
				}); err != nil {
					break
				}
			default:
				err = fmt.Errorf("%s %q.\n", unrecognizedPeerTrackerOperation, req.Header.Operation)
			}

			if err != nil {
				errorLogger.Printf("%v\n", err)
				resp = makeFailedOperationResponse(req.Header, err)
			}

			if _, err := conn.Write(resp); err != nil {
				errorLogger.Printf("%v\n", err)
			}
		}()
	}
}

// handleRegister returns a valid response and nil if the request is successfully served else returns nil and error
// the []byte return value has been encoded into a raw json message
func (t *Tracker) handleRegister(req communication.RegisterRequest) ([]byte, error) {
	infoLogger.Printf("%s:\n", handlingRequest)
	util.PrettyLogStruct(genericLogger, req)

	b := req.Body
	if b.FilesToShare != nil {
		remoteFileStatusLock.Lock()
		for _, fileToShare := range b.FilesToShare {
			f := remoteFile{
				name:     fileToShare.Name,
				checksum: fileToShare.Checksum,
			}
			// if file has not been recorded
			if status, ok := tracker.remoteFileLocations[f]; !ok {
				locations := make([]map[string]struct{}, 0)
				for i := 0; i < util.CalculateNumberOfChunks(fileToShare.Size); i++ {
					locations = append(locations, map[string]struct{}{
						b.HostPort: {},
					})
				}
				tracker.remoteFileLocations[f] = remoteFileStatus{
					size:           fileToShare.Size,
					chunkLocations: locations,
				}
			} else { // file has been recorded
				for i := 0; i < util.CalculateNumberOfChunks(status.size); i++ {
					hostPortSet := status.chunkLocations[i]
					if _, ok := hostPortSet[b.HostPort]; !ok {
						hostPortSet[b.HostPort] = struct{}{}
					}
				}
			}
		}
		remoteFileStatusLock.Unlock()
	}

	resp, _ := json.Marshal(communication.RegisterResponse{
		Header: req.Header,
		Body: communication.RegisterResponseBody{
			Result: communication.OperationResult{
				Code:   communication.Success,
				Detail: registerIsSuccessful,
			},
			RegisteredFiles: b.FilesToShare,
		},
	})
	return resp, nil
}

func (t *Tracker) handleList(req communication.ListFileRequest) ([]byte, error) {
	infoLogger.Printf("%s:\n", handlingRequest)
	util.PrettyLogStruct(genericLogger, req)

	var p2pFiles []communication.P2PFile
	remoteFileStatusLock.Lock()
	for fileID, stat := range t.remoteFileLocations {
		p2pFiles = append(p2pFiles, communication.P2PFile{
			Name:     fileID.name,
			Checksum: fileID.checksum,
			Size:     stat.size,
		})
	}
	remoteFileStatusLock.Unlock()

	resp, _ := json.Marshal(communication.ListFileResponse{
		Header: req.Header,
		Body: communication.ListFileResponseBody{
			Result: communication.OperationResult{
				Code:   communication.Success,
				Detail: lookUpFileListIsSuccessful,
			},
			Files: p2pFiles,
		},
	})

	return resp, nil
}

func (t *Tracker) handleFind(req communication.FindFileRequest) ([]byte, error) {
	infoLogger.Printf("%s:\n", handlingRequest)
	util.PrettyLogStruct(genericLogger, req)

	remoteFileStatusLock.Lock()
	status, ok := t.remoteFileLocations[remoteFile{
		name:     req.Body.FileName,
		checksum: req.Body.Checksum,
	}]
	remoteFileStatusLock.Unlock()
	if !ok {
		return nil, fmt.Errorf("%s", fileDoesNotExist)
	}

	resp, _ := json.Marshal(communication.FindFileResponse{
		Header: req.Header,
		Body: communication.FindFileResponseBody{
			Result: communication.OperationResult{
				Code:   communication.Success,
				Detail: fileIsFound,
			},
			FileSize:       status.size,
			ChunkLocations: status.chunkLocations,
		},
	})

	return resp, nil
}
