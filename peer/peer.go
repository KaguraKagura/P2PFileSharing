package peer

import (
	"bufio"
	"encoding/json"
	"log"
	"net"
	"os"
	"path/filepath"
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

type Peer struct {
	selfHostPort    string
	trackerHostPort string
	filesToShare    []localFile
	registered      bool
}

var self Peer
var selfLock sync.Mutex

var logger = log.New(os.Stdout, "", 0)

func Start() {
	logger.Println(welcomeMessage)

	reader := bufio.NewReader(os.Stdin)

	// to avoid duplicate ongoing registration
	registerChan := make(chan struct{}, 1)
	registerChan <- struct{}{}

	for {
		command, _ := reader.ReadString('\n')
		command = strings.TrimSpace(command)
		if command == "" {
			continue
		}

		args := strings.Split(command, " ")
		switch args[0] {
		case register:
			if len(args) < 3 {
				logger.Printf("%s. %s\n", badArguments, helpPrompt)
				continue
			}
			go self.register(registerChan, args[1], args[2], args[3:])
		case list:
			if len(args) != 1 {
				logger.Printf("%s. %s\n", badArguments, helpPrompt)
				continue
			}

		case find:
			if len(args) != 2 {
				logger.Printf("%s. %s\n", badArguments, helpPrompt)
				continue
			}

		case download:
			if len(args) != 2 {
				logger.Printf("%s. %s\n", badArguments, helpPrompt)
				continue
			}

		case help:
			logger.Printf("%s\n", helpMessage)
		default:
			logger.Printf("%s %q. %s\n", unrecognizedCommand, args[0], helpPrompt)
		}
	}
}

func (p *Peer) register(c chan struct{}, trackerHostPort, selfHostPort string, filepaths []string) {
	// to avoid duplicate ongoing registration
	<-c
	defer func() {
		c <- struct{}{}
	}()

	for _, hp := range []string{trackerHostPort, selfHostPort} {
		if _, _, err := net.SplitHostPort(hp); err != nil {
			logger.Printf("%s. %s\n", badIpPortArgument, helpPrompt)
			return
		}
	}

	var p2pFiles []localFile
	for _, path := range filepaths {
		f, err := os.Open(path)
		if err != nil {
			logger.Printf("Error: %v\n", err)
			return
		}

		checksum, err := util.HashFileSHA256(f)
		if err != nil {
			logger.Printf("Error: %v\n", err)
			_ = f.Close()
			return
		}

		stat, err := f.Stat()
		if err != nil {
			logger.Printf("Error: %v\n", err)
			_ = f.Close()
			return
		}

		_ = f.Close()

		p2pFiles = append(p2pFiles, localFile{
			name:     filepath.Base(path),
			fullPath: path,
			checksum: checksum,
			size:     stat.Size(),
		})
	}

	selfLock.Lock()
	p.trackerHostPort = trackerHostPort
	p.selfHostPort = selfHostPort
	p.filesToShare = p2pFiles
	selfLock.Unlock()

	requestId := uuid.NewString()
	req, err := json.Marshal(communication.PeerRegisterRequest{
		RequestId:    requestId,
		Operation:    communication.Register,
		HostPort:     p.selfHostPort,
		FilesToShare: localFilesToP2PFiles(p.filesToShare),
	})
	if err != nil {
		logger.Printf("Error: %v\n", err)
		return
	}

	// start to talk to the tracker
	dialer := net.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.Dial("tcp", p.trackerHostPort)
	if err != nil {
		logger.Printf("Error: %v\n", err)
		return
	}

	defer func(conn net.Conn) {
		if err := conn.Close(); err != nil {
			logger.Printf("Error: %v\n", err)
		}
	}(conn)

	if _, err := conn.Write(req); err != nil {
		logger.Printf("Error: %v\n", err)
		return
	}

	// get response from tracker
	var resp communication.PeerRegisterResponse
	d := json.NewDecoder(conn)
	if err := d.Decode(&resp); err != nil {
		logger.Printf("Error: %v\n", err)
		return
	}

	if resp.RequestId != requestId || resp.Operation != communication.Register {
		logger.Printf("Error: %v\n", badTrackerResponse)
		return
	}

	switch resp.Result {
	case communication.Success:
		// success
	case communication.Fail:
		logger.Printf("Error: %v\n", resp.ErrorMessage)
		return
	}

	selfLock.Lock()
	p.registered = true
	selfLock.Unlock()
}
