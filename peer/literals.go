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

	alreadyUsingTrackerAt                     = "already using tracker ip:port at"
	alreadyUsingSelfHostPortAt                = "already using self ip:port at"
	availableFilesAre                         = "available files are"
	badArguments                              = "bad arguments"
	beginDownloading                          = "begin downloading"
	badPeerResponse                           = "bad peer response"
	badTrackerResponse                        = "bad tracker response"
	discardBadChunk                           = "discard bad chunk"
	downloadCanceledByUser                    = "download canceled by user"
	downloadCompletedFor                      = "download completed for"
	downloadedChunkIndexMismatch              = "downloaded chunk index mismatch"
	downloadedChunkChecksumMismatch           = "downloaded chunk checksum mismatch"
	downloadedFileChecksumMismatch            = "downloaded file checksum mismatch"
	downloadingPausedFor                      = "downloading paused for"
	downloadingResumedFor                     = "downloading resumed for"
	dueToError                                = "due to error"
	failToDownload                            = "fail to download"
	goodbye                                   = "goodbye"
	ignoreFailedUpdateToChunkLocations        = "ignore failed update to chunk locations"
	inTheBackground                           = "in the background"
	noAvailableFileRightNow                   = "no available file right now"
	pleaseRegisterFirst                       = "please " + registerCmd + " first"
	registeredFilesAre                        = "registered file(s) are"
	startToServeFilesToPeers                  = "start to serve files to peers"
	unrecognizedCommand                       = "unrecognized command"
	unrecognizedPeerPeerOperation             = "unrecognized peer peer operation"
	unrecognizedPeerPeerResponseResultCode    = "unrecognized peer peer response result code"
	unrecognizedPeerTrackerResponseResultCode = "unrecognized peer tracker response result code"
	writeChunkFails                           = "write chunk fails"
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
