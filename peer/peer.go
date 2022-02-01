package peer

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"Lab1/communication"
	"Lab1/p2pFile"
)

type Peer struct {
	selfHostPort    string
	trackerHostPort string
	filesToShare    []p2pFile.P2PFile
	registered      bool
}

var self Peer
var selfLock sync.Mutex

func Start() {
	fmt.Println(welcomeMessage)

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
				fmt.Printf("%s. %s\n", badArguments, helpPrompt)
				continue
			}
			go self.Register(registerChan, args[1], args[2], args[3:])
		case list:
			if len(args) != 1 {
				fmt.Printf("%s. %s\n", badArguments, helpPrompt)
				continue
			}

		case find:
			if len(args) != 2 {
				fmt.Printf("%s. %s\n", badArguments, helpPrompt)
				continue
			}

		case download:
			if len(args) != 2 {
				fmt.Printf("%s. %s\n", badArguments, helpPrompt)
				continue
			}

		case help:
			fmt.Printf("%s\n", helpMessage)
		default:
			fmt.Printf("%s %q. %s\n", unrecognizedCommand, args[0], helpPrompt)
		}

	}
}

func (p *Peer) Register(c chan struct{}, trackerHostPort, selfHostPort string, filepaths []string) {
	// to avoid duplicate ongoing registration
	<-c
	defer func() {
		c <- struct{}{}
	}()

	for _, hp := range []string{trackerHostPort, selfHostPort} {
		if _, _, err := net.SplitHostPort(hp); err != nil {
			fmt.Printf("%s. %s\n", badIpPortArgument, helpPrompt)
			return
		}
	}

	var p2pFiles []p2pFile.P2PFile
	for _, path := range filepaths {
		f, err := os.Open(path)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		checksum, err := p2pFile.HashFileSHA256(f)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			_ = f.Close()
			return
		}

		stat, err := f.Stat()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			_ = f.Close()
			return
		}

		_ = f.Close()

		p2pFiles = append(p2pFiles, p2pFile.P2PFile{
			Name:     filepath.Base(path),
			FullPath: path,
			Checksum: checksum,
			Size:     stat.Size(),
		})
	}

	selfLock.Lock()
	p.trackerHostPort = trackerHostPort
	p.selfHostPort = selfHostPort
	p.filesToShare = p2pFiles
	selfLock.Unlock()

	req, err := json.Marshal(communication.PeerRegisterRequest{
		Operation:    register,
		HostPort:     p.selfHostPort,
		FilesToShare: p.filesToShare,
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// start to talk to the tracker
	dialer := net.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.Dial("tcp", p.trackerHostPort)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	defer func(conn net.Conn) {
		if err := conn.Close(); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	}(conn)

	if _, err := conn.Write(req); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// get response from tracker
	var resp communication.PeerRegisterResponse
	d := json.NewDecoder(conn)
	if err := d.Decode(&resp); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	switch resp.Result {
	case communication.Success:
		// success
	case communication.Fail:
		fmt.Printf("Error: %v\n", resp.ErrorMessage)
		return
	}

	selfLock.Lock()
	p.registered = true
	selfLock.Unlock()
}
