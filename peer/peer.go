package peer

import (
	"bufio"
	"encoding/json"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
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
	filesToHost     map[string][]localFile
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

		args := strings.Split(command, " ")
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

		case find:
			if len(args) != 2 {
				errorLogger.Printf("%s. %s\n", badArguments, helpPrompt)
				continue
			}

		case download:
			if len(args) != 2 {
				errorLogger.Printf("%s. %s\n", badArguments, helpPrompt)
				continue
			}

		case help:
			genericLogger.Printf("%s\n", helpMessage)
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

	var p2pFiles []localFile
	for _, path := range filepaths {
		f, err := os.Open(path)
		if err != nil {
			errorLogger.Printf("%v\n", err)
			return
		}

		checksum, err := util.HashFileSHA256(f)
		if err != nil {
			errorLogger.Printf("%v\n", err)
			_ = f.Close()
			return
		}

		stat, err := f.Stat()
		if err != nil {
			errorLogger.Printf("%v\n", err)
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

	p.trackerHostPort = trackerHostPort
	p.selfHostPort = selfHostPort
	p.filesToShare = p2pFiles

	// prepare to talk to the tracker
	requestId := uuid.NewString()
	req, _ := json.Marshal(communication.PeerRegisterRequest{
		Header: communication.PeerTrackerHeader{
			RequestId: requestId,
			Operation: communication.Register,
		},
		Body: communication.PeerRegisterRequestBody{
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
	var resp communication.PeerRegisterResponse
	d := json.NewDecoder(conn)
	if err := d.Decode(&resp); err != nil {
		errorLogger.Printf("%v\n", err)
		return
	}

	if resp.Header.RequestId != requestId || resp.Header.Operation != communication.Register {
		errorLogger.Printf("%s\n", badTrackerResponse)
		return
	}

	switch resp.Result.Result {
	case communication.Success:
		infoLogger.Printf("%s (%s):\n", fileSuccessfullyRegistered, resp.Result.DetailedResult)
		for _, f := range resp.RegisteredFiles {
			util.PrettyLogStruct(genericLogger, f)
		}
	case communication.Fail:
		errorLogger.Printf("%s: %s\n", failToRegister, resp.Result.DetailedResult)
		return
	}
}
