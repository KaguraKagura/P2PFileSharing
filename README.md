# Lab1

## Compile & Run

Install Go 1.17 or higher at https://go.dev/doc/install

`$ cd Lab1`

`$ go build -o p2pFileSharing`

Run the program by `$ ./p2pFileSharing`

## System Description

### General

The system is a peer to peer (P2P) file sharing system consisting of a tracker and multiple peers. The tracker records what files are being shared by the peers and on which peers the files are stored. A peer asks the tracker about the available files to download and which peers have the file, then downloads directly from the peers.

### Supported Features

The system supports

- Multiple connections: the tracker can handle requests from multiple peers at the same time. A peer can handle multiple requests from multiple peers at the same time.
- Parallel downloading: a peer can establish multiple TCP connections to different peers to download different chunks of the file.
- Chunk registration: a peer can register a newly downloaded chunk to the tracker so that others can download from this peer.
- Integrity check: SHA256 checksum is used to verify the integrity of each chunk and of the whole file.
- Rarest chunk selection: a peer first downloads the chunk that fewest peers have

### Communication Protocol

Peer to tracker communications and peer to peer communications are implemented with `json` data over TCP connections. 

Every `json` message consists of a header and a body.

- The header consists of a unique request ID and an operation.
  - Peer to tracker operations include:
    - `RegisterChunk` that informs the tracker of the newly downloaded chunk.
    - `RegisterFile` that informs the tracker of the files to be shared.
    - `List` that gets all the files being shared.
    - `Find` that, for a particular file, gets which peers have which chunks
  - Peer to peer operations include
    - `DownloadChunk` that asks a peer for a file’s particular chunk by an index
- The body is either a request or a response to an operation.
  - A request body includes request parameters.
    - E.g. `RegisterFile` request includes files’ metadata and the host port for other peers to connect to.
  - A response body consists of a result and additional parameters.
    - E.g. `RegisterFile` response includes a result indicating if the request is successful or failed and a detailed reason. The additional parameter is the files that are successfully registered to the tracker.

## Program Description

To run: `$ ./p2pFileSharing`

The program is a command line application with detailed help prompts. It can be run in either peer mode or tracker mode. Then, the application enters an interactive environment.

Peer mode supports:

- `register` that performs `RegisterFile` protocol operation
- `list` that performs `List` protocol operation
- `find` that performs `Find` protocol operation
- `download` that downloads a file, during which `Find` and `DownloadChunk` protocol operations are used
- `showDownloads` that displays current file downloads and their progress
- `pauseDownload` that pauses a file download
- `resumeDownload` that resumes a paused file download
- `cancelDownload` that cancels a file download
- `quit` that quits the program
- `help` that displays help

Tracker mode supports:

- `start` that starts the tracker server
- `quit` that quits the program
- `help` that displays help

## Program Structure

The program is written in Go. It consists of 5 packages.

- `package main` includes the main function that starts the program
- `package peer` includes the logic when the program runs in peer mode
- `package tracker` includes the logic when the program runs in tracker mode
- `package communication` includes the communication protocol specifications
- `package util` includes helper functions

## Sample Output

