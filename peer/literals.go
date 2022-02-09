package peer

import (
	"fmt"
	"strings"

	"Lab1/util"
)

const (
	parallelDownloadWorkerCount = 10

	registerCmd = "register"
	listCmd     = "list"
	findCmd     = "find"
	downloadCmd = "download"
	hCmd        = "h"
	helpCmd     = "help"
	qCmd        = "q"
	quitCmd     = "quit"

	alreadyUsingTrackerAt                     = "already using tracker at"
	alreadyUsingHostPortAt                    = "already using ip:port at"
	availableFilesAre                         = "available files are"
	badArguments                              = "bad arguments"
	badIpPortArgument                         = "bad ip:port argument"
	beginDownloading                          = "begin downloading"
	badPeerResponse                           = "bad peer response"
	badTrackerResponse                        = "bad tracker response"
	downloadedChunkIndexMismatch              = "downloaded chunk index mismatch"
	downloadedChunkChecksumMismatch           = "downloaded chunk checksum mismatch"
	failToDownload                            = "fail to download"
	downloadCompletedFor                      = "download completed for"
	downloadedFileChecksumMismatch            = "downloaded file checksum mismatch"
	dueToError                                = "due to error"
	ignoreFailedUpdateToChunkLocations        = "ignore failed update to chunk locations"
	inTheBackground                           = "in the background"
	noAvailableFileRightNow                   = "no available file right now"
	pleaseRegisterFirst                       = "please " + registerCmd + " first"
	registeredFilesAre                        = "registered file(s) are"
	retryDownloadChunk                        = "retry download chunk"
	unrecognizedCommand                       = "unrecognized command"
	unrecognizedPeerPeerResponseResultCode    = "unrecognized peer peer response result code"
	unrecognizedPeerTrackerResponseResultCode = "unrecognized peer tracker response result code"
)

var helpMessage = strings.Join([]string{
	fmt.Sprintf("\t%s [ip:port of tracker] [ip:port to accept peer connections] [Optional: filepaths seperated by space]", registerCmd),
	fmt.Sprintf("\t%s", listCmd),
	fmt.Sprintf("\t%s [filename] [sha256 checksum]", findCmd),
	fmt.Sprintf("\t%s [filename] [sha256 checksum]", downloadCmd),
	fmt.Sprintf("\t%s, %s", quitCmd, qCmd),
}, "\n")

var helpPrompt = fmt.Sprintf("Type %q or %q to see command usages", helpCmd, hCmd)
var welcomeMessage = fmt.Sprintf("Welcome to %s. You are running this app as a peer.\n%s", util.AppName, helpPrompt)
