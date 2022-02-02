package tracker

import (
	"fmt"
	"strings"

	"Lab1/util"
)

const (
	help  = "help"
	start = "start"

	badArguments                     = "Bad arguments"
	badIpPortArgument                = "Bad ip:port argument"
	unrecognizedCommand              = "Unrecognized command"
	unrecognizedPeerTrackerOperation = "Unrecognized peer tracker operation"
	trackerAlreadyRunningAt          = "Tracker is already running at"
)

var helpMessage = strings.Join([]string{
	fmt.Sprintf("\t%s [ip:port to listen]", start),
	fmt.Sprintf("\t%s", help),
}, "\n")

var helpPrompt = fmt.Sprintf("Type %q to see command usages", help)
var welcomeMessage = fmt.Sprintf("Welcome to %s. You are running this app as a tracker.\n%s", util.AppName, helpPrompt)
