package tracker

import (
	"fmt"
	"strings"

	"Lab1/util"
)

const (
	startCmd = "start"
	hCmd     = "h"
	helpCmd  = "help"
	qCmd     = "q"
	quitCmd  = "quit"

	badArguments                     = "bad arguments"
	badIpPortArgument                = "bad ip:port argument"
	chunkRegisterIsSuccessful        = "chunk register is successful"
	fileDoesNotExist                 = "file does not exist"
	fileIsFound                      = "file is found"
	fileRegisterIsSuccessful         = "file register is successful"
	handlingRequest                  = "handling request"
	lookUpFileListIsSuccessful       = "look up file list is successful"
	trackerAlreadyRunningAt          = "tracker is already running at"
	trackerOnlineListeningOn         = "tracker is online and listening on"
	unrecognizedCommand              = "unrecognized command"
	unrecognizedPeerTrackerOperation = "unrecognized peer tracker operation"
)

var helpMessage = strings.Join([]string{
	fmt.Sprintf("\t%s [ip:port to listen]", startCmd),
	fmt.Sprintf("\t%s, %s", quitCmd, qCmd),
}, "\n")

var helpPrompt = fmt.Sprintf("Type %q or %q to see command usages", helpCmd, hCmd)
var welcomeMessage = fmt.Sprintf("Welcome to %s. You are running this app as a tracker.\n%s", util.AppName, helpPrompt)
