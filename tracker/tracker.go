package tracker

import (
	"bufio"
	"encoding/json"
	"log"
	"net"
	"os"
	"strings"
	"sync"

	"Lab1/communication"
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
var logger = log.New(os.Stdout, "", 0)
var remoteFileStatusLock sync.Mutex

func Start() {
	logger.Println(welcomeMessage)

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
				logger.Printf("%s. %s\n", badArguments, helpPrompt)
				continue
			}
			t.start(args[1])
		case help:
			logger.Printf("%s\n", helpMessage)
		default:
			logger.Printf("%s %q. %s\n", unrecognizedCommand, args[0], helpPrompt)
		}
	}
}

func (t *Tracker) start(hostPort string) {
	if t.listening == true {
		logger.Printf("%s %s\n", trackerAlreadyRunningAt, t.hostPort)
		return
	}

	if _, _, err := net.SplitHostPort(hostPort); err != nil {
		logger.Printf("%s. %s\n", badIpPortArgument, helpPrompt)
		return
	}

	t.hostPort = hostPort

	l, err := net.Listen("tcp", hostPort)
	if err != nil {
		logger.Printf("Error: %v\n", err)
		return
	}
	t.listener = &l
	t.listening = true
	logger.Printf("Tracker is online and listening on %s\n", hostPort)

	go serve()
}

func serve() {
	for {
		conn, err := (*t.listener).Accept()
		if err != nil {
			logger.Printf("Error: %v\n", err)
			continue
		}
		go handleRequest(&conn)
	}
}

func handleRequest(conn *net.Conn) {
	defer func() {
		if err := (*conn).Close(); err != nil {
			logger.Printf("Error when closing tcp connection: %v\n", err)
		}
	}()

	req := struct {
		RequestId string
		Operation communication.PeerTrackerOperation
		Args      json.RawMessage
	}{}

	d := json.NewDecoder(*conn)
	if err := d.Decode(&req); err != nil {
		logger.Printf("Error: %v\n", err)
		return
	}

	switch req.Operation {
	case communication.Register:
		args := struct {
			HostPort     string
			FilesToShare []communication.P2PFile
		}{}
		err := json.Unmarshal(req.Args, &args)
		if err != nil {
			logger.Printf("Error: %v\n", err)
			return
		}

		if args.FilesToShare != nil {
			remoteFileStatusLock.Lock()
			for _, fileToShare := range args.FilesToShare {
				f := remoteFile{
					name:     fileToShare.Name,
					checksum: fileToShare.Checksum,
				}
				if status, ok := t.remoteFileLocations[f]; ok {
					if status.locationsOfChunks == nil || status.size != fileToShare.Size {
						// todo: server error
						return
					}
					for i := 0; i < calculateNumberOfChunks(status.size); i++ {
						hostPorts := status.locationsOfChunks[i]
						if _, ok := hostPorts[hostPort(args.HostPort)]; !ok {
							hostPorts[hostPort(args.HostPort)] = struct{}{}
						}
					}
				} else {
					locations := make([]map[hostPort]struct{}, 0)
					for i := 0; i < calculateNumberOfChunks(fileToShare.Size); i++ {
						locations = append(locations, map[hostPort]struct{}{
							hostPort(args.HostPort): {},
						})
					}
					t.remoteFileLocations[f] = remoteFileStatus{
						size:              fileToShare.Size,
						locationsOfChunks: locations,
					}
				}
			}
			remoteFileStatusLock.Unlock()
		}

	case communication.List:

	case communication.Find:

	default:
		logger.Printf("%s %q.\n", unrecognizedPeerTrackerOperation, req.Operation)
	}
}
