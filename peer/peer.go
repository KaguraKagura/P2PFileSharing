package peer

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"Lab1/file"
)

type Peer struct {
	Ip           net.IP
	Port         uint
	FilesToShare []file.File
	trackerIp    net.IP
	trackerPort  uint
	registered   bool
}

var self Peer

func Start() {
	fmt.Println(welcomeMessage)

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
			if len(args) == 1 {
				fmt.Printf("%s. %s\n", badArguments, helpPrompt)
				continue
			}
			go func() {
				//todo: remove
				time.Sleep(5 * time.Second)
				fmt.Println("inside " + register)

			}()

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

func (p Peer) Register() (error error) {
	trackerAddr := fmt.Sprintf("%s:%s", p.trackerIp, p.trackerPort)
	conn, err := net.Dial("tcp", trackerAddr)
	if err != nil {
		return err
	}

	defer func(conn net.Conn) {
		if err := conn.Close(); err != nil {
			error = err
		}
	}(conn)

	data, err := json.Marshal(p)
	if err != nil {
		return err
	}

	if _, err := conn.Write(data); err != nil {
		return err
	}

	return nil
}
