package tracker

import (
	"fmt"
	"strings"

	"Lab1/util"
)

const (
	h     = "h"
	help  = "help"
	start = "start"
	q     = "q"
	quit  = "quit"

	badArguments                     = "bad arguments"
	badIpPortArgument                = "bad ip:port argument"
	fileDoesNotExist                 = "file does not exist"
	fileIsFound                      = "file is found"
	handlingRequest                  = "handling request"
	lookUpFileListIsSuccessful       = "look up file list is successful"
	registerIsSuccessful             = "register is successful"
	trackerAlreadyRunningAt          = "tracker is already running at"
	trackerOnlineListeningOn         = "tracker is online and listening on"
	unrecognizedCommand              = "unrecognized command"
	unrecognizedPeerTrackerOperation = "unrecognized peer tracker operation"
)

var helpMessage = strings.Join([]string{
	fmt.Sprintf("\t%s [ip:port to listen]", start),
	fmt.Sprintf("\t%s, %s", quit, q),
}, "\n")

var helpPrompt = fmt.Sprintf("Type %q or %q to see command usages", help, h)
var welcomeMessage = fmt.Sprintf("Welcome to %s. You are running this app as a tracker.\n%s", util.AppName, helpPrompt)
