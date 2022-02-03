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

type hostPort string

type remoteFile struct {
	name     string
	checksum string
}

type remoteFileStatus struct {
	size              int64
	locationsOfChunks []map[hostPort]struct{} // each chunk index to a set of hostPort
}

type Tracker struct {
	hostPort  string
	listener  *net.Listener
	listening bool

	remoteFileLocations map[remoteFile]remoteFileStatus
}

var t Tracker
var remoteFileStatusLock sync.Mutex

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

		args := strings.Split(command, " ")
		switch args[0] {
		case start:
			if len(args) != 2 {
				errorLogger.Printf("%s. %s\n", badArguments, helpPrompt)
				continue
			}
			t.start(args[1])
		case help:
			genericLogger.Printf("%s\n", helpMessage)
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
		conn, err := (*t.listener).Accept()
		if err != nil {
			errorLogger.Printf("%v\n", err)
			continue
		}
		go parseRequest(&conn)
	}
}

func parseRequest(conn *net.Conn) {
	defer func() {
		if err := (*conn).Close(); err != nil {
			errorLogger.Printf("%v\n", err)
		}
	}()

	var req genericRequest
	d := json.NewDecoder(*conn)
	if err := d.Decode(&req); err != nil {
		errorLogger.Printf("%v\n", err)
		return
	}

	switch req.Header.Operation {
	case communication.Register:
		var body communication.PeerRegisterRequestBody
		err := json.Unmarshal(req.Body, &body)
		if err != nil {
			errorLogger.Printf("%v\n", err)
			respondWithError(conn, communication.Register, req.Header, err)
			return
		}

		handleRegister(conn, communication.PeerRegisterRequest{
			Header: req.Header,
			Body:   body,
		})

	case communication.List:

	case communication.Find:

	default:
		errorLogger.Printf("%s %q.\n", unrecognizedPeerTrackerOperation, req.Header.Operation)
		respondWithError(conn, unrecognizedOp, req.Header, fmt.Errorf("%s", unrecognizedPeerTrackerOperation))
	}
}

func handleRegister(conn *net.Conn, req communication.PeerRegisterRequest) {
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
			if status, ok := t.remoteFileLocations[f]; !ok {
				locations := make([]map[hostPort]struct{}, 0)
				for i := 0; i < calculateNumberOfChunks(fileToShare.Size); i++ {
					locations = append(locations, map[hostPort]struct{}{
						hostPort(b.HostPort): {},
					})
				}
				t.remoteFileLocations[f] = remoteFileStatus{
					size:              fileToShare.Size,
					locationsOfChunks: locations,
				}
			} else { // file has been recorded
				if status.locationsOfChunks == nil || status.size != fileToShare.Size {
					respondWithError(conn, communication.Register, req.Header, fmt.Errorf("%s", internalTrackerError))
					return
				}
				for i := 0; i < calculateNumberOfChunks(status.size); i++ {
					hostPorts := status.locationsOfChunks[i]
					if _, ok := hostPorts[hostPort(b.HostPort)]; !ok {
						hostPorts[hostPort(b.HostPort)] = struct{}{}
					}
				}
			}
		}
		remoteFileStatusLock.Unlock()
	}

	resp, _ := json.Marshal(communication.PeerRegisterResponse{
		Header: req.Header,
		Result: communication.TrackerResponseResult{
			Result:         communication.Success,
			DetailedResult: registerSuccessful,
		},
		RegisteredFiles: b.FilesToShare,
	})

	if _, err := (*conn).Write(resp); err != nil {
		errorLogger.Printf("%v\n", err)
		return
	}
}
