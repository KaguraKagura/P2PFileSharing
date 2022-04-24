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

Every `json` message consists of a **header** and a **body**.

- The **header** consists of a unique request ID and an operation.
  - Peer to tracker operations include:
    - `RegisterChunk` that informs the tracker of the newly downloaded chunk.
    - `RegisterFile` that informs the tracker of the files to be shared.
    - `List` that gets all the files being shared.
    - `Find` that, for a particular file, gets which peers have which chunks
  - Peer to peer operations include
    - `DownloadChunk` that asks a peer for a file’s particular chunk by an index
- The **body** is either a request or a response to an operation.
  - A request body includes request parameters.
    - E.g. `RegisterFile` request includes files’ metadata and the host port for other peers to connect to.
  - A response body consists of a result and additional parameters.
    - E.g. `RegisterFile` response includes a result indicating if the request is successful or failed and a detailed reason. The additional parameter is the files that are successfully registered to the tracker.

## Program Description

The program is a command line application with detailed help prompts. It can be run in either peer mode or tracker mode:

- `$ ./p2pFileSharing peer`
- `$ ./p2pFileSharing tracker`

Then, the application enters an interactive environment.

**Peer mode** supports:

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

**Tracker mode** supports:

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

## Which part works and which part doesn’t

The programs works for all the features including the rarest chunk selection feature.

## Sample Output

### General

In this demo, I set the chunk size to be `1` byte and use the file `a.txt`, which content is the alphabet from a to z. I also let the download chunk function to sleep `15` seconds for simulation.

- First run the program in tracker mode and start the tracker **T**.

  ```
  $ ./p2pFileSharing tracker
  Welcome to p2pFileSharing. You are running this app as a tracker.
  Type "help" or "h" to see command usages
  start localhost:9999
  INFO: tracker is online and listening on localhost:9999
  ```

- Run another program instance **P1** in peer mode and register a file. It also starts to accept potential download requests from peers.

  ```
  $ ./p2pFileSharing peer
  Welcome to p2pFileSharing. You are running this app as a peer.
  Type "help" or "h" to see command usages
  register localhost:9999 localhost:11111 a.txt
  success: register is successful
  registered file(s) are:
  {
          "Name": "a.txt",
          "Checksum": "71c480df93d6ae2f1efad1447c66c9525e316218cf51fc8d9ed832f2daf18b73",
          "Size": 26
  }
  INFO: ready to accept peer connections
  ```

- On **T**, it shows a register request.

  ```
  INFO: handling request:
  {
          "Header": {
                  "RequestId": "f1e8fd6c-5002-4171-919a-7abd0cf750a2",
                  "Operation": "register_file"
          },
          "Body": {
                  "HostPort": "localhost:11111",
                  "FilesToShare": [
                          {
                                  "Name": "a.txt",
                                  "Checksum": "71c480df93d6ae2f1efad1447c66c9525e316218cf51fc8d9ed832f2daf18b73",
                                  "Size": 26
                          }
                  ]
          }
  }
  ```

- From a different directory than **P1**, run another program instance **P2** in peer mode and register with no files.

  ```
  $ ./p2pFileSharing peer
  Welcome to p2pFileSharing. You are running this app as a peer.
  Type "help" or "h" to see command usages
  register localhost:9999 localhost:22222
  INFO: ready to accept peer connections
  success: register is successful
  ```


- Type `list` in **P2** and the file registered by **P1** shows up. 

  ```
  list
  success: look up file list is successful
  available files are:
  {
          "Name": "a.txt",
          "Checksum": "71c480df93d6ae2f1efad1447c66c9525e316218cf51fc8d9ed832f2daf18b73",
          "Size": 26
  }
  ```

- On **T**, it shows a register request and a list request.

  ```
  INFO: handling request:
  {
          "Header": {
                  "RequestId": "ce8e8afc-32a0-4ae2-b828-fe1e32fec5b0",
                  "Operation": "register_file"
          },
          "Body": {
                  "HostPort": "localhost:22222",
                  "FilesToShare": null
          }
  }
  INFO: handling request:
  {
          "Header": {
                  "RequestId": "80dc0ba7-385f-4f9c-941e-05e20071c7b7",
                  "Operation": "list"
          },
          "Body": {}
  }
  ```

- Download the file in **P2**, pause, and view download status.

  ```
  download a.txt 71c480df93d6ae2f1efad1447c66c9525e316218cf51fc8d9ed832f2daf18b73
  INFO: begin downloading "a.txt" in the background
  INFO: download worker 9 starts for "a.txt"
  INFO: download worker 2 starts for "a.txt"
  INFO: download worker 3 starts for "a.txt"
  INFO: download worker 6 starts for "a.txt"
  INFO: download worker 7 starts for "a.txt"
  INFO: download worker 0 starts for "a.txt"
  INFO: download worker 5 starts for "a.txt"
  INFO: download worker 1 starts for "a.txt"
  INFO: download worker 8 starts for "a.txt"
  INFO: download worker 4 starts for "a.txt"
  INFO: downloaded "a.txt" chunk index 6 from localhost:11111
  INFO: downloaded "a.txt" chunk index 7 from localhost:11111
  INFO: downloaded "a.txt" chunk index 9 from localhost:11111
  INFO: downloaded "a.txt" chunk index 5 from localhost:11111
  INFO: downloaded "a.txt" chunk index 4 from localhost:11111
  INFO: downloaded "a.txt" chunk index 3 from localhost:11111
  INFO: downloaded "a.txt" chunk index 2 from localhost:11111
  INFO: downloaded "a.txt" chunk index 8 from localhost:11111
  INFO: downloaded "a.txt" chunk index 0 from localhost:11111
  INFO: downloaded "a.txt" chunk index 1 from localhost:11111
  ```
  ```
  pauseDownload a.txt 71c480df93d6ae2f1efad1447c66c9525e316218cf51fc8d9ed832f2daf18b73
  pausing download for "a.txt"
  INFO: downloading paused for "a.txt"
  ```
  ```
  showDownloads
  {
          "FileName": "a.txt",
          "Checksum": "71c480df93d6ae2f1efad1447c66c9525e316218cf51fc8d9ed832f2daf18b73",
          "CompletedChunkIndexes": "[0, 1, 2, 3, 4, 5, 6, 7, 8, 9]",
          "Paused": true
  }
  ```

- From a different directory than **P1** and **P2**, run another program instance **P3** in peer mode, register with no files and download the same file.

  ```
  $ ./p2pFileSharing peer
  Welcome to p2pFileSharing. You are running this app as a peer.
  Type "help" or "h" to see command usages
  register localhost:9999 localhost:33333
  success: register is successful
  INFO: ready to accept peer connections
  ```

  ```
  download a.txt 71c480df93d6ae2f1efad1447c66c9525e316218cf51fc8d9ed832f2daf18b73
  INFO: begin downloading "a.txt" in the background
  INFO: download worker 9 starts for "a.txt"
  INFO: download worker 2 starts for "a.txt"
  INFO: download worker 1 starts for "a.txt"
  INFO: download worker 7 starts for "a.txt"
  INFO: download worker 0 starts for "a.txt"
  INFO: download worker 6 starts for "a.txt"
  INFO: download worker 8 starts for "a.txt"
  INFO: download worker 5 starts for "a.txt"
  INFO: download worker 4 starts for "a.txt"
  INFO: download worker 3 starts for "a.txt"
  INFO: downloaded "a.txt" chunk index 10 from localhost:11111
  INFO: downloaded "a.txt" chunk index 18 from localhost:11111
  INFO: downloaded "a.txt" chunk index 11 from localhost:11111
  INFO: downloaded "a.txt" chunk index 16 from localhost:11111
  INFO: downloaded "a.txt" chunk index 14 from localhost:11111
  INFO: downloaded "a.txt" chunk index 19 from localhost:11111
  INFO: downloaded "a.txt" chunk index 17 from localhost:11111
  INFO: downloaded "a.txt" chunk index 15 from localhost:11111
  INFO: downloaded "a.txt" chunk index 12 from localhost:11111
  INFO: downloaded "a.txt" chunk index 13 from localhost:11111
  ```

  ```
  pauseDownload a.txt 71c480df93d6ae2f1efad1447c66c9525e316218cf51fc8d9ed832f2daf18b73
  pausing download for "a.txt"
  INFO: downloading paused for "a.txt"
  ```

  ```
  showDownloads
  {
          "FileName": "a.txt",
          "Checksum": "71c480df93d6ae2f1efad1447c66c9525e316218cf51fc8d9ed832f2daf18b73",
          "CompletedChunkIndexes": "[10, 11, 12, 13, 14, 15, 16, 17, 18, 19]",
          "Paused": true
  }
  ```

  Rarest chunk feature is shown above. Because **P2** downloaded chunks 0 to 9, **P3** sees that both **P1** and **P2** have those chunks, so **P3** choose to skip those and instead download from index 10 to 19, which only **P1** has them right now.

- Resume download for **P2**.

  ```
  resumeDownload a.txt 71c480df93d6ae2f1efad1447c66c9525e316218cf51fc8d9ed832f2daf18b73
  resuming download for "a.txt"
  INFO: downloading resumed for "a.txt"
  INFO: downloaded "a.txt" chunk index 10 from localhost:11111
  INFO: downloaded "a.txt" chunk index 11 from localhost:11111
  INFO: downloaded "a.txt" chunk index 12 from localhost:11111
  INFO: downloaded "a.txt" chunk index 13 from localhost:11111
  INFO: downloaded "a.txt" chunk index 14 from localhost:11111
  INFO: downloaded "a.txt" chunk index 15 from localhost:11111
  INFO: downloaded "a.txt" chunk index 16 from localhost:11111
  INFO: downloaded "a.txt" chunk index 17 from localhost:11111
  INFO: downloaded "a.txt" chunk index 18 from localhost:11111
  INFO: downloaded "a.txt" chunk index 19 from localhost:11111
  INFO: downloaded "a.txt" chunk index 21 from localhost:11111
  INFO: downloaded "a.txt" chunk index 20 from localhost:11111
  INFO: downloaded "a.txt" chunk index 22 from localhost:11111
  INFO: downloaded "a.txt" chunk index 24 from localhost:11111
  INFO: downloaded "a.txt" chunk index 23 from localhost:11111
  INFO: downloaded "a.txt" chunk index 25 from localhost:11111
  download completed for "a.txt"
  ```

- Resume download for **P3**.

  ```
  resumeDownload a.txt 71c480df93d6ae2f1efad1447c66c9525e316218cf51fc8d9ed832f2daf18b73
  resuming download for "a.txt"
  INFO: downloading resumed for "a.txt"
  INFO: downloaded "a.txt" chunk index 20 from localhost:11111
  INFO: downloaded "a.txt" chunk index 21 from localhost:11111
  INFO: downloaded "a.txt" chunk index 22 from localhost:11111
  INFO: downloaded "a.txt" chunk index 23 from localhost:11111
  INFO: downloaded "a.txt" chunk index 25 from localhost:11111
  INFO: downloaded "a.txt" chunk index 0 from localhost:22222
  INFO: downloaded "a.txt" chunk index 24 from localhost:11111
  INFO: downloaded "a.txt" chunk index 2 from localhost:22222
  INFO: downloaded "a.txt" chunk index 1 from localhost:11111
  INFO: downloaded "a.txt" chunk index 3 from localhost:11111
  INFO: downloaded "a.txt" chunk index 4 from localhost:11111
  INFO: downloaded "a.txt" chunk index 8 from localhost:22222
  INFO: downloaded "a.txt" chunk index 9 from localhost:22222
  INFO: downloaded "a.txt" chunk index 5 from localhost:11111
  INFO: downloaded "a.txt" chunk index 7 from localhost:11111
  INFO: downloaded "a.txt" chunk index 6 from localhost:11111
  download completed for "a.txt"
  ```

- Interleaved in **P2**’s output (I separate them out below) shows it served 4 chunks to **P3**, consistent with the output above for **P3**

  ```
  INFO: served chunk 0 of file a.txt to 127.0.0.1:49800
  INFO: served chunk 2 of file a.txt to 127.0.0.1:49804
  INFO: served chunk 8 of file a.txt to 127.0.0.1:49894
  INFO: served chunk 9 of file a.txt to 127.0.0.1:49890
  ```

- **P1**’s output shows it served many chunks. Every chunk except chunk 0, 2, 8, 9 appear twice because 0, 2, 8, 9 happen to be served by **P2**. 

  ```
  INFO: served chunk 6 of file a.txt to 127.0.0.1:41614
  INFO: served chunk 7 of file a.txt to 127.0.0.1:41616
  INFO: served chunk 9 of file a.txt to 127.0.0.1:41620
  INFO: served chunk 5 of file a.txt to 127.0.0.1:41634
  INFO: served chunk 4 of file a.txt to 127.0.0.1:41622
  INFO: served chunk 3 of file a.txt to 127.0.0.1:41632
  INFO: served chunk 2 of file a.txt to 127.0.0.1:41630
  INFO: served chunk 8 of file a.txt to 127.0.0.1:41624
  INFO: served chunk 0 of file a.txt to 127.0.0.1:41628
  INFO: served chunk 1 of file a.txt to 127.0.0.1:41626
  INFO: served chunk 10 of file a.txt to 127.0.0.1:41658
  INFO: served chunk 11 of file a.txt to 127.0.0.1:41660
  INFO: served chunk 12 of file a.txt to 127.0.0.1:41662
  INFO: served chunk 13 of file a.txt to 127.0.0.1:41664
  INFO: served chunk 14 of file a.txt to 127.0.0.1:41666
  INFO: served chunk 15 of file a.txt to 127.0.0.1:41668
  INFO: served chunk 16 of file a.txt to 127.0.0.1:41670
  INFO: served chunk 17 of file a.txt to 127.0.0.1:41672
  INFO: served chunk 18 of file a.txt to 127.0.0.1:41674
  INFO: served chunk 19 of file a.txt to 127.0.0.1:41676
  INFO: served chunk 10 of file a.txt to 127.0.0.1:41688
  INFO: served chunk 18 of file a.txt to 127.0.0.1:41684
  INFO: served chunk 11 of file a.txt to 127.0.0.1:41692
  INFO: served chunk 16 of file a.txt to 127.0.0.1:41698
  INFO: served chunk 14 of file a.txt to 127.0.0.1:41700
  INFO: served chunk 19 of file a.txt to 127.0.0.1:41694
  INFO: served chunk 17 of file a.txt to 127.0.0.1:41690
  INFO: served chunk 15 of file a.txt to 127.0.0.1:41696
  INFO: served chunk 12 of file a.txt to 127.0.0.1:41686
  INFO: served chunk 13 of file a.txt to 127.0.0.1:41702
  INFO: served chunk 20 of file a.txt to 127.0.0.1:41726
  INFO: served chunk 21 of file a.txt to 127.0.0.1:41728
  INFO: served chunk 22 of file a.txt to 127.0.0.1:41730
  INFO: served chunk 23 of file a.txt to 127.0.0.1:41732
  INFO: served chunk 25 of file a.txt to 127.0.0.1:41736
  INFO: served chunk 24 of file a.txt to 127.0.0.1:41734
  INFO: served chunk 1 of file a.txt to 127.0.0.1:41740
  INFO: served chunk 3 of file a.txt to 127.0.0.1:41744
  INFO: served chunk 21 of file a.txt to 127.0.0.1:41806
  INFO: served chunk 20 of file a.txt to 127.0.0.1:41802
  INFO: served chunk 22 of file a.txt to 127.0.0.1:41800
  INFO: served chunk 24 of file a.txt to 127.0.0.1:41810
  INFO: served chunk 23 of file a.txt to 127.0.0.1:41808
  INFO: served chunk 25 of file a.txt to 127.0.0.1:41812
  INFO: served chunk 4 of file a.txt to 127.0.0.1:41826
  INFO: served chunk 5 of file a.txt to 127.0.0.1:41830
  INFO: served chunk 7 of file a.txt to 127.0.0.1:41836
  INFO: served chunk 6 of file a.txt to 127.0.0.1:41834
  ```

- **T** shows a lot of register chunk requests. One of them is below.

  ```
  INFO: handling request:
  {
          "Header": {
                  "RequestId": "914aa4ce-09a1-4556-80e8-c57c0fea3c2e",
                  "Operation": "register_chunk"
          },
          "Body": {
                  "HostPort": "localhost:22222",
                  "Chunk": {
                          "FileName": "a.txt",
                          "FileChecksum": "71c480df93d6ae2f1efad1447c66c9525e316218cf51fc8d9ed832f2daf18b73",
                          "ChunkIndex": 20
                  }
          }
  }
  ```

- **T** also shows many find requests, which come from the peers periodically to get the latest chunk locations. One of them is below.

  ```
  INFO: handling request:
  {
          "Header": {
                  "RequestId": "9709c2ab-3060-4a25-95da-1847f04f159b",
                  "Operation": "find"
          },
          "Body": {
                  "FileName": "a.txt",
                  "Checksum": "71c480df93d6ae2f1efad1447c66c9525e316218cf51fc8d9ed832f2daf18b73"
          }
  }
  ```

- In the end, from any peer, find a.txt shows all 3 peers have every chunk of the file.

  ``` 
  find a.txt 71c480df93d6ae2f1efad1447c66c9525e316218cf51fc8d9ed832f2daf18b73
  success: file is found
  Chunk 0 is at: localhost:11111 localhost:22222 localhost:33333
  Chunk 1 is at: localhost:11111 localhost:22222 localhost:33333
  Chunk 2 is at: localhost:22222 localhost:33333 localhost:11111
  Chunk 3 is at: localhost:11111 localhost:22222 localhost:33333
  Chunk 4 is at: localhost:11111 localhost:22222 localhost:33333
  Chunk 5 is at: localhost:11111 localhost:22222 localhost:33333
  Chunk 6 is at: localhost:11111 localhost:22222 localhost:33333
  Chunk 7 is at: localhost:11111 localhost:22222 localhost:33333
  Chunk 8 is at: localhost:11111 localhost:22222 localhost:33333
  Chunk 9 is at: localhost:11111 localhost:22222 localhost:33333
  Chunk 10 is at: localhost:11111 localhost:22222 localhost:33333
  Chunk 11 is at: localhost:11111 localhost:22222 localhost:33333
  Chunk 12 is at: localhost:11111 localhost:22222 localhost:33333
  Chunk 13 is at: localhost:22222 localhost:33333 localhost:11111
  Chunk 14 is at: localhost:11111 localhost:22222 localhost:33333
  Chunk 15 is at: localhost:22222 localhost:33333 localhost:11111
  Chunk 16 is at: localhost:33333 localhost:11111 localhost:22222
  Chunk 17 is at: localhost:11111 localhost:22222 localhost:33333
  Chunk 18 is at: localhost:11111 localhost:22222 localhost:33333
  Chunk 19 is at: localhost:33333 localhost:11111 localhost:22222
  Chunk 20 is at: localhost:11111 localhost:22222 localhost:33333
  Chunk 21 is at: localhost:33333 localhost:11111 localhost:22222
  Chunk 22 is at: localhost:22222 localhost:33333 localhost:11111
  Chunk 23 is at: localhost:11111 localhost:22222 localhost:33333
  Chunk 24 is at: localhost:33333 localhost:11111 localhost:22222
  Chunk 25 is at: localhost:11111 localhost:22222 localhost:33333

### Integrity Check

In this demo,I set the chunk size to be `10` byte and use the file `a.txt`, which content is the alphabet from a to z. I let the peer that serves a chunk to modify the content and make the chunk inconsistent with the checksum. 

- First run the program in tracker mode and start the tracker **T**.

  ```
  $ ./p2pFileSharing tracker
  Welcome to p2pFileSharing. You are running this app as a tracker.
  Type "help" or "h" to see command usages
  start localhost:9999
  INFO: tracker is online and listening on localhost:9999
  ```

- Run another program instance **P1** in peer mode and register a file.

  ```
  $ ./p2pFileSharing peer
  Welcome to p2pFileSharing. You are running this app as a peer.
  Type "help" or "h" to see command usages
  register localhost:9999 localhost:11111 a.txt
  INFO: ready to accept peer connections
  success: register is successful
  registered file(s) are:
  {
          "Name": "a.txt",
          "Checksum": "71c480df93d6ae2f1efad1447c66c9525e316218cf51fc8d9ed832f2daf18b73",
          "Size": 26
  }
  ```

- From a different directory than **P1**, run another program instance **P2** in peer mode and register with no files.

  ```
  $ ./p2pFileSharing peer
  Welcome to p2pFileSharing. You are running this app as a peer.
  Type "help" or "h" to see command usages
  register localhost:9999 localhost:22222
  success: register is successful
  INFO: ready to accept peer connections
  ```

- Download in **P2** shows that when it sees a bad chunk, it discards it and retries. After some number of failures (I set to 5), the whole download fails.

  ```
  download a.txt 71c480df93d6ae2f1efad1447c66c9525e316218cf51fc8d9ed832f2daf18b73
  INFO: begin downloading "a.txt" in the background
  INFO: download worker 2 starts for "a.txt"
  INFO: download worker 1 starts for "a.txt"
  INFO: download worker 0 starts for "a.txt"
  INFO: discard "a.txt" chunk index 0 from "localhost:11111": downloaded chunk checksum mismatch: expect "72399361da6a7754fec986dca5b7cbaf1c810a28ded4abaf56b2106d06cb78b0" get "7a5e6e8212e9ffa61bb5765581e20bcd2f03206c5fad284a45e81c61c426f023"
  INFO: discard "a.txt" chunk index 1 from "localhost:11111": downloaded chunk checksum mismatch: expect "e683456c3fca63fe2cc7655a7f574e8b22a1ec23d98a55495cfbe8e7c6adfa15" get "e46106942f676f986255f034b93c49b71bf126f43896f138bd03b1427d67e7eb"
  INFO: discard "a.txt" chunk index 2 from "localhost:11111": downloaded chunk checksum mismatch: expect "5347f5b986fa92683f21a1e5287025ca2706f1339040d8ee922c9671b9d033dd" get "683dd40229a0d4d1b0a539d61459189d887caaccaedd901dac1ddaddce4e0993"
  INFO: discard "a.txt" chunk index 0 from "localhost:11111": downloaded chunk checksum mismatch: expect "72399361da6a7754fec986dca5b7cbaf1c810a28ded4abaf56b2106d06cb78b0" get "7a5e6e8212e9ffa61bb5765581e20bcd2f03206c5fad284a45e81c61c426f023"
  INFO: discard "a.txt" chunk index 1 from "localhost:11111": downloaded chunk checksum mismatch: expect "e683456c3fca63fe2cc7655a7f574e8b22a1ec23d98a55495cfbe8e7c6adfa15" get "e46106942f676f986255f034b93c49b71bf126f43896f138bd03b1427d67e7eb"
  ERROR: fail to download "a.txt": too many failed chunks during downloading "a.txt"
  ```
