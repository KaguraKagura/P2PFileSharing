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

type genericRequest struct {
	Header communication.PeerTrackerHeader
	Body   json.RawMessage
}

type p2pFileStatus struct {
	size           int64
	chunkLocations communication.ChunkLocations
}

type p2pFileLocations struct {
	locations map[remoteFile]p2pFileStatus
	mu        sync.Mutex
}

type remoteFile struct {
	name     string
	checksum string
}

type tracker struct {
	hostPort  string
	listening bool

	p2pFileLocations p2pFileLocations
}

var (
	t tracker

	genericLogger = log.New(os.Stdout, "", 0)
	infoLogger    = log.New(os.Stdout, "INFO: ", 0)
	errorLogger   = log.New(os.Stdout, "ERROR: ", 0)
)

func Start() {
	genericLogger.Println(welcomeMessage)

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" {
			continue
		}

		var (
			result string
			err    error
		)
		args := strings.Fields(line)
		switch args[0] {
		case startCmd:
			if len(args) != 2 {
				err = fmt.Errorf("%s. %s", badArguments, helpPrompt)
				break
			}
			err = t.start(args[1])
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
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	genericLogger.Printf("%s!", goodbye)
	os.Exit(0)
}

// start starts the tracker server
func (t *tracker) start(hostPort string) error {
	if t.listening == true {
		return fmt.Errorf("%s %s", trackerAlreadyRunningAt, t.hostPort)
	}

	if err := util.ValidateHostPort(hostPort); err != nil {
		return err
	}
	t.hostPort = hostPort

	l, err := net.Listen("tcp", hostPort)
	if err != nil {
		return err
	}
	t.listening = true

	t.p2pFileLocations.locations = make(map[remoteFile]p2pFileStatus)

	infoLogger.Printf("%s %s", trackerOnlineListeningOn, hostPort)

	// start to listen and serve
	go func() {
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

				var req genericRequest
				d := json.NewDecoder(conn)
				err = d.Decode(&req)
				if err == nil {
					switch req.Header.Operation {
					case communication.RegisterChunk:
						var body communication.RegisterChunkRequestBody
						if err = json.Unmarshal(req.Body, &body); err != nil {
							break
						}
						if resp, err = t.handleRegisterChunk(communication.RegisterChunkRequest{
							Header: req.Header,
							Body:   body,
						}); err != nil {
							break
						}
					case communication.RegisterFile:
						var body communication.RegisterFileRequestBody
						if err = json.Unmarshal(req.Body, &body); err != nil {
							break
						}
						if resp, err = t.handleRegisterFile(communication.RegisterFileRequest{
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
						if resp, err = t.handleList(communication.ListFileRequest{
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
						if resp, err = t.handleFind(communication.FindFileRequest{
							Header: req.Header,
							Body:   body,
						}); err != nil {
							break
						}
					default:
						err = fmt.Errorf("%s %q", unrecognizedPeerTrackerOperation, req.Header.Operation)
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
	}()

	return nil
}

// handleRegisterChunk returns a valid response and nil if the request is successfully served else returns nil and error
// the []byte return value has been encoded into a raw json message
func (t *tracker) handleRegisterChunk(req communication.RegisterChunkRequest) ([]byte, error) {
	infoLogger.Printf("%s:", handlingRequest)
	genericLogger.Printf("%s", util.StructToPrettyJsonString(req))

	file := remoteFile{
		name:     req.Body.Chunk.FileName,
		checksum: req.Body.Chunk.FileChecksum,
	}
	chunkIndex := req.Body.Chunk.ChunkIndex

	t.p2pFileLocations.mu.Lock()
	defer t.p2pFileLocations.mu.Unlock()

	// if file has not been recorded, i.e. never registered
	if status, ok := t.p2pFileLocations.locations[file]; !ok {
		return nil, fmt.Errorf("%s", fileDoesNotExist)
	} else { // file has been recorded
		status.chunkLocations[chunkIndex][req.Body.HostPort] = struct{}{}
	}

	resp, _ := json.Marshal(communication.RegisterChunkResponse{
		Header: req.Header,
		Body: communication.RegisterChunkResponseBody{
			Result: communication.OperationResult{
				Code:   communication.Success,
				Detail: chunkRegisterIsSuccessful,
			},
			RegisteredChunk: communication.P2PChunk{
				FileName:     file.name,
				FileChecksum: file.checksum,
				ChunkIndex:   chunkIndex,
			},
		},
	})
	return resp, nil
}

// handleRegisterFile returns a valid response and nil if the request is successfully served else returns nil and error
// the []byte return value has been encoded into a raw json message
func (t *tracker) handleRegisterFile(req communication.RegisterFileRequest) ([]byte, error) {
	infoLogger.Printf("%s:", handlingRequest)
	genericLogger.Printf("%s", util.StructToPrettyJsonString(req))

	b := req.Body

	if b.FilesToShare != nil {
		t.p2pFileLocations.mu.Lock()
		defer t.p2pFileLocations.mu.Unlock()

		for _, fileToShare := range b.FilesToShare {
			file := remoteFile{
				name:     fileToShare.Name,
				checksum: fileToShare.Checksum,
			}
			// if file has not been recorded
			if status, ok := t.p2pFileLocations.locations[file]; !ok {
				chunkCount := communication.CalculateNumberOfChunks(fileToShare.Size)
				locations := make(communication.ChunkLocations, 0, chunkCount)
				for i := 0; i < chunkCount; i++ {
					locations = append(locations, map[string]struct{}{
						b.HostPort: {},
					})
				}
				t.p2pFileLocations.locations[file] = p2pFileStatus{
					size:           fileToShare.Size,
					chunkLocations: locations,
				}
			} else { // file has been recorded
				for i := 0; i < communication.CalculateNumberOfChunks(status.size); i++ {
					hostPortSet := status.chunkLocations[i]
					if _, ok := hostPortSet[b.HostPort]; !ok {
						hostPortSet[b.HostPort] = struct{}{}
					}
				}
			}
		}
	}

	resp, _ := json.Marshal(communication.RegisterFileResponse{
		Header: req.Header,
		Body: communication.RegisterFileResponseBody{
			Result: communication.OperationResult{
				Code:   communication.Success,
				Detail: registerIsSuccessful,
			},
			RegisteredFiles: b.FilesToShare,
		},
	})
	return resp, nil
}

// handleList returns a valid response and nil if the request is successfully served else returns nil and error
// the []byte return value has been encoded into a raw json message
func (t *tracker) handleList(req communication.ListFileRequest) ([]byte, error) {
	infoLogger.Printf("%s:", handlingRequest)
	genericLogger.Printf("%s", util.StructToPrettyJsonString(req))

	t.p2pFileLocations.mu.Lock()
	defer t.p2pFileLocations.mu.Unlock()

	var p2pFiles []communication.P2PFile
	for fileID, stat := range t.p2pFileLocations.locations {
		p2pFiles = append(p2pFiles, communication.P2PFile{
			Name:     fileID.name,
			Checksum: fileID.checksum,
			Size:     stat.size,
		})
	}

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

// handleFind returns a valid response and nil if the request is successfully served else returns nil and error
// the []byte return value has been encoded into a raw json message
func (t *tracker) handleFind(req communication.FindFileRequest) ([]byte, error) {
	infoLogger.Printf("%s:", handlingRequest)
	genericLogger.Printf("%s", util.StructToPrettyJsonString(req))

	t.p2pFileLocations.mu.Lock()
	defer t.p2pFileLocations.mu.Unlock()

	status, ok := t.p2pFileLocations.locations[remoteFile{
		name:     req.Body.FileName,
		checksum: req.Body.Checksum,
	}]

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
