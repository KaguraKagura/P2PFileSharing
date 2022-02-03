package tracker

import (
	"fmt"
	"strings"

	"Lab1/communication"
	"Lab1/util"
)

const (
	help  = "help"
	start = "start"

	badArguments                     = "bad arguments"
	badIpPortArgument                = "bad ip:port argument"
	handlingRequest                  = "handling request"
	internalTrackerError             = "internal tracker error"
	registerSuccessful               = "register is successful"
	trackerAlreadyRunningAt          = "tracker is already running at"
	trackerOnlineListeningOn         = "tracker is online and listening on"
	unrecognizedCommand              = "unrecognized command"
	unrecognizedPeerTrackerOperation = "unrecognized peer tracker operation"

	unrecognizedOp communication.PeerTrackerOperation = "unrecognizedOp"
)

var helpMessage = strings.Join([]string{
	fmt.Sprintf("\t%s [ip:port to listen]", start),
	fmt.Sprintf("\t%s", help),
}, "\n")

var helpPrompt = fmt.Sprintf("Type %q to see command usages", help)
var welcomeMessage = fmt.Sprintf("Welcome to %s. You are running this app as a tracker.\n%s", util.AppName, helpPrompt)
