package peer

import (
	"fmt"
	"strings"

	"Lab1/util"
)

const (
	register = "register"
	list     = "list"
	find     = "find"
	download = "download"
	help     = "help"

	alreadyUsingTrackerAt      = "already using tracker at"
	alreadyUsingHostPortAt     = "already using ip:port at"
	badArguments               = "bad arguments"
	badIpPortArgument          = "bad ip:port argument"
	badTrackerResponse         = "bad tracker response"
	failToRegister             = "fail to register"
	fileSuccessfullyRegistered = "files successfully registered"
	unrecognizedCommand        = "unrecognized command"
)

var helpMessage = strings.Join([]string{
	fmt.Sprintf("\t%s [ip:port of tracker] [ip:port of yourself] [filepaths seperated by space]", register),
	fmt.Sprintf("\t%s", list),
	fmt.Sprintf("\t%s [filename]", find),
	fmt.Sprintf("\t%s [filename]", download),
	fmt.Sprintf("\t%s", help),
}, "\n")

var helpPrompt = fmt.Sprintf("Type %q to see command usages", help)
var welcomeMessage = fmt.Sprintf("Welcome to %s. You are running this app as a peer.\n%s", util.AppName, helpPrompt)
