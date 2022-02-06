package peer

import (
	"fmt"
	"strings"

	"Lab1/util"
)

const (
	PARALLEL_DOWNLOAD_WORKER_COUNT = 10

	register = "register"
	list     = "list"
	find     = "find"
	download = "download"
	h        = "h"
	help     = "help"
	q        = "q"
	quit     = "quit"

	alreadyUsingTrackerAt          = "already using tracker at"
	alreadyUsingHostPortAt         = "already using ip:port at"
	availableFilesAre              = "available files are"
	badArguments                   = "bad arguments"
	badSha256ChecksumHexStringSize = "bad sha256 checksum hex string size"
	badIpPortArgument              = "bad ip:port argument"
	badTrackerResponse             = "bad tracker response"
	noAvailableFileRightNow        = "no available file right now"
	pleaseRegisterFirst            = "please register first"
	registeredFilesAre             = "registered file(s) are"
	unrecognizedCommand            = "unrecognized command"
	unrecognizedResponseResultCode = "unrecognized response result code"

	sha256ChecksumHexStringSize = 64
)

var helpMessage = strings.Join([]string{
	fmt.Sprintf("\t%s [ip:port of tracker] [ip:port to accept peer connections] [filepaths seperated by space]", register),
	fmt.Sprintf("\t%s", list),
	fmt.Sprintf("\t%s [filename] [sha256 checksum]", find),
	fmt.Sprintf("\t%s [filename] [sha256 checksum]", download),
	fmt.Sprintf("\t%s, %s", quit, q),
}, "\n")

var helpPrompt = fmt.Sprintf("Type %q or %q to see command usages", help, h)
var welcomeMessage = fmt.Sprintf("Welcome to %s. You are running this app as a peer.\n%s", util.AppName, helpPrompt)
