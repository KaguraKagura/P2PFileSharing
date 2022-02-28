package peer

import (
	"fmt"
	"strings"

	"Lab1/util"
)

const (
	parallelDownloadWorkerCount = 10
	sha256ChecksumHexStringSize = 64

	registerCmd       = "register"
	listCmd           = "list"
	findCmd           = "find"
	downloadCmd       = "download"
	showDownloadsCmd  = "showDownloads"
	pauseDownloadCmd  = "pauseDownload"
	resumeDownloadCmd = "resumeDownload"
	cancelDownloadCmd = "cancelDownload"
	hCmd              = "h"
	helpCmd           = "help"
	qCmd              = "q"
	quitCmd           = "quit"

	alreadyUsingTrackerAt                     = "already using tracker ip:port at"
	alreadyUsingSelfHostPortAt                = "already using self ip:port at"
	availableFilesAre                         = "available files are"
	badArguments                              = "bad arguments"
	badSha256ChecksumHexStringSize            = "bad sha256 checksum hex string size"
	beginDownloading                          = "begin downloading"
	badPeerResponse                           = "bad peer response"
	badTrackerResponse                        = "bad tracker response"
	cancelingDownloadFor                      = "canceling download for"
	chunkDoesNotExist                         = "chunk does not exist"
	chunkIsFound                              = "chunk is found"
	downloadCanceledFor                       = "download canceled for"
	downloadCompletedFor                      = "download completed for"
	downloadedChunkIndexMismatch              = "downloaded chunk index mismatch"
	downloadedChunkChecksumMismatch           = "downloaded chunk checksum mismatch"
	downloadedFileChecksumMismatch            = "downloaded file checksum mismatch"
	downloadingPausedFor                      = "downloading paused for"
	downloadingResumedFor                     = "downloading resumed for"
	failToDownload                            = "fail to download"
	fileDoesNotExist                          = "file does not exist"
	goodbye                                   = "goodbye"
	ignoreFailedUpdateToChunkLocations        = "ignore failed update to chunk locations"
	inTheBackground                           = "in the background"
	noFileIsAvailableRightNow                 = "no file is available right now"
	noFileIsBeingDownloaded                   = "no file is being downloaded"
	noSuchFileIsBeingDownloaded               = "no such file is being downloaded"
	pausingDownloadFor                        = "pausing download for"
	pleaseRegisterFirst                       = "please " + registerCmd + " first"
	registeredFilesAre                        = "registered file(s) are"
	readyToAcceptPeerConnections              = "ready to accept peer connections"
	resumingDownloadFor                       = "resuming download for"
	tooManyFailedChunksDuringDownloading      = "too many failed chunks during downloading"
	unrecognizedCommand                       = "unrecognized command"
	unrecognizedPeerPeerOperation             = "unrecognized peer peer operation"
	unrecognizedPeerPeerResponseResultCode    = "unrecognized peer peer response result code"
	unrecognizedPeerTrackerResponseResultCode = "unrecognized peer tracker response result code"
	unsupportedFileDownloadControlOp          = "unsupported file download control op"
	writeChunkFailsFor                        = "write chunk fails for"
)

var helpMessage = strings.Join([]string{
	fmt.Sprintf("\t%s [ip:port of tracker] [ip:port to accept peer connections] [Optional: filepaths seperated by space]", registerCmd),
	fmt.Sprintf("\t%s", listCmd),
	fmt.Sprintf("\t%s [filename] [sha256 checksum]", findCmd),
	fmt.Sprintf("\t%s [filename] [sha256 checksum]", downloadCmd),
	fmt.Sprintf("\t%s", showDownloadsCmd),
	fmt.Sprintf("\t%s [filename] [sha256 checksum]", pauseDownloadCmd),
	fmt.Sprintf("\t%s [filename] [sha256 checksum]", resumeDownloadCmd),
	fmt.Sprintf("\t%s [filename] [sha256 checksum]", cancelDownloadCmd),
	fmt.Sprintf("\t%s, %s", quitCmd, qCmd),
	fmt.Sprintf("\t%s, %s", helpCmd, hCmd),
}, "\n")

var helpPrompt = fmt.Sprintf("Type %q or %q to see command usages", helpCmd, hCmd)
var welcomeMessage = fmt.Sprintf("Welcome to %s. You are running this app as a peer.\n%s", util.AppName, helpPrompt)
