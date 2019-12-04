package filesharing

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	core "github.com/ksei/Peerster/Core"
)

//FileHandler is a structure used for handling file requests/replies within a gossiper instance
type FileHandler struct {
	ctx                            *core.Context
	fileLocker                     sync.RWMutex
	indexedFiles                   map[string]*File
	ongoingFileRequests            map[string][]string
	requestLocker                  sync.RWMutex
	bytesRequested                 map[string]chan *[]byte
	DownloadProgress               map[string]int
	terminateOngoingSearchRequests chan bool
	searchLocker                   sync.RWMutex
	searchMatches                  map[string](map[string][]string)
	searchMatchFound               chan bool
	requestCache                   map[string]string
	ongoingSearch                  bool
}

//NewFileHandler creates new fileHandler instance
func NewFileHandler(cntx *core.Context) *FileHandler {
	fh := &FileHandler{
		ctx:                            cntx,
		indexedFiles:                   make(map[string]*File),
		ongoingFileRequests:            make(map[string][]string),
		bytesRequested:                 make(map[string]chan *[]byte),
		DownloadProgress:               make(map[string]int),
		terminateOngoingSearchRequests: make(chan bool, 10),
		searchMatches:                  make(map[string]map[string][]string),
		searchMatchFound:               make(chan bool, 10),
		requestCache:                   make(map[string]string),
		ongoingSearch:                  false,
	}
	return fh
}

//IndexFile creates internal instance of a given file
func (fH *FileHandler) IndexFile(fileName string) (int64, []byte) {
	file, err := NewIndexedFile(fileName)
	if err != nil {
		fmt.Println("Could not index file: ", err)
		return -1, nil
	}
	fH.addToFiles(file)
	return file.GetSize(), file.GetMetaHash()
}

func (fH *FileHandler) addToFiles(file *File) {
	fH.fileLocker.Lock()
	defer fH.fileLocker.Unlock()
	metahash := file.GetMetaHash()
	// fmt.Println("metahash:" + hex.EncodeToString(metahash))
	fH.indexedFiles[hex.EncodeToString(metahash)] = file
}

//ProcessDataRequest handles incoming file requests from the gossiper
func (fH *FileHandler) ProcessDataRequest(dataRequest *core.DataRequest) {
	hashValue := hex.EncodeToString(dataRequest.HashValue)
	fH.fileLocker.RLock()
	defer fH.fileLocker.RUnlock()
	file, ok := fH.indexedFiles[hashValue]

	if ok && file.status >= 0 {
		dataReply := &core.DataReply{
			Destination: dataRequest.Origin,
			HopLimit:    fH.ctx.GetHopLimit(),
			HashValue:   file.meta.metahash,
			Data:        file.meta.metaFile,
		}
		fH.ongoingFileRequests[dataRequest.Origin] = append(fH.ongoingFileRequests[dataRequest.Origin], hex.EncodeToString(file.meta.getMetaHash()))
		fH.sendDataReply(dataReply)
		return
	}

	ongoingFilesWithPeer, ok := fH.ongoingFileRequests[dataRequest.Origin]

	if ok {
		for _, file := range ongoingFilesWithPeer {
			f := fH.indexedFiles[file]
			chunk, ok := f.meta.GetChunkByHash(dataRequest.HashValue)
			if ok {
				dataReply := &core.DataReply{
					Destination: dataRequest.Origin,
					HopLimit:    fH.ctx.GetHopLimit(),
					HashValue:   dataRequest.HashValue,
					Data:        chunk,
				}
				fH.sendDataReply(dataReply)
				return
			}
		}
	}

	dataReply := &core.DataReply{
		Destination: dataRequest.Origin,
		HopLimit:    fH.ctx.GetHopLimit(),
		HashValue:   dataRequest.HashValue,
		Data:        nil,
	}
	fH.sendDataReply(dataReply)

}

//ProcessDataReply processes data replies from gossiper, mapping them to the corresponding destinations
func (fH *FileHandler) ProcessDataReply(dataReply *core.DataReply) {
	if !validateReceivedDataReplyHash(*dataReply) {
		return
	}
	fH.requestLocker.RLock()
	defer fH.requestLocker.RUnlock()
	awaitingChannel, ok := fH.bytesRequested[hex.EncodeToString(dataReply.HashValue)]
	if ok {
		awaitingChannel <- &dataReply.Data
	}
}

//InitiateFileRequest starts an outgoing file request to a given destination using a known metahash and filename
func (fH *FileHandler) InitiateFileRequest(dest *string, fileName string, metahash []byte) {
	var destination string
	if dest == nil {
		var ok bool
		fH.searchLocker.RLock()
		destinations, ok := fH.searchMatches[fileName][hex.EncodeToString(metahash)]
		fH.searchLocker.RUnlock()
		if !ok {
			fmt.Println("Match not found in search results...")
			return
		}
		destination = destinations[0]
	} else {
		destination = *dest
	}
	dataRequest := &core.DataRequest{
		Destination: destination,
		HopLimit:    fH.ctx.GetHopLimit(),
		HashValue:   metahash,
	}

	metahashString := hex.EncodeToString(metahash)
	fH.fileLocker.Lock()
	fH.indexedFiles[metahashString] = NewIncomingFile(fileName, metahash)
	fmt.Println("DOWNLOADING metafile of", fH.indexedFiles[metahashString].Name, "from", destination)
	fH.fileLocker.Unlock()
	fH.requestLocker.Lock()
	defer fH.requestLocker.Unlock()
	fH.bytesRequested[metahashString] = make(chan *[]byte)
	go fH.waitForMetafile(dataRequest, fH.bytesRequested[metahashString], destination, metahashString)
}

func (fH *FileHandler) waitForMetafile(dataRequest *core.DataRequest, channel chan *[]byte, destination, metahash string) {
	fH.sendDataRequest(dataRequest)
	for {
		select {
		case receivedMetafile := <-channel:
			fH.requestLocker.Lock()
			delete(fH.bytesRequested, metahash)
			fH.requestLocker.Unlock()
			if len(*receivedMetafile) > 0 {
				fH.fileLocker.RLock()
				fH.indexedFiles[metahash].AddMetafile(*receivedMetafile)
				fH.fileLocker.RUnlock()
				go fH.initiateDownload(destination, metahash, *receivedMetafile)
			} else {
				fmt.Println("File not found at peer")
			}
			return
		case <-time.After(5 * time.Second):
			fH.sendDataRequest(dataRequest)
		}
	}
}

func (fH *FileHandler) initiateDownload(destination, metahash string, metafile []byte) {
	var chunks [][]byte
	hashSize := 32
	fH.fileLocker.Lock()
	fH.DownloadProgress[metahash] = len(metafile) / hashSize
	fH.fileLocker.Unlock()
	for i := 0; i < len(metafile); i += hashSize {
		chunks = append(chunks, metafile[i:i+hashSize])
	}
	go fH.DownloadChunks(chunks, destination, metahash)
}

//DownloadChunks starts sending requests and opens go routines for waiting replies of individual chunks
func (fH *FileHandler) DownloadChunks(chunks [][]byte, destination, metahash string) {
	for i, chunk := range chunks {
		dataRequest := &core.DataRequest{
			Destination: destination,
			HopLimit:    fH.ctx.GetHopLimit(),
			HashValue:   chunk,
		}
		chunkHashString := hex.EncodeToString(chunk)
		awaitingChannel := make(chan *[]byte)
		fH.requestLocker.Lock()
		fH.bytesRequested[chunkHashString] = awaitingChannel
		fH.requestLocker.Unlock()
		go fH.waitForChunk(dataRequest, chunkHashString, metahash, awaitingChannel)
		fH.fileLocker.RLock()
		fmt.Println("DOWNLOADING", fH.indexedFiles[metahash].Name, "chunk", i+1, "from", dataRequest.Destination)
		if fH.indexedFiles[metahash].status != DOWNLOADING {
			break
		}
		fH.fileLocker.RUnlock()
	}
}

func (fH *FileHandler) waitForChunk(dataRequest *core.DataRequest, chunkHash, metahash string, channel chan *[]byte) {
	fH.sendDataRequest(dataRequest)
	for {
		select {
		case receivedChunk := <-channel:
			fH.requestLocker.Lock()
			delete(fH.bytesRequested, chunkHash)
			fH.requestLocker.Unlock()
			if len(*receivedChunk) == 0 {
				fH.fileLocker.Lock()
				fH.indexedFiles[metahash].status = INCOMPLETE
				fH.fileLocker.Unlock()
				return
			}
			fH.fileLocker.Lock()
			fH.indexedFiles[metahash].AddChunk(*receivedChunk)
			fH.DownloadProgress[metahash]--
			if fH.DownloadProgress[metahash] == 0 {
				fH.indexedFiles[metahash].status = INDEXED
				fH.indexedFiles[metahash].saveFile()
			}
			fH.fileLocker.Unlock()
			return
		case <-time.After(5 * time.Second):
			fH.sendDataRequest(dataRequest)
		}
	}
}

func validateReceivedDataReplyHash(dataReply core.DataReply) bool {
	if len(dataReply.Data) == 0 {
		return true
	}
	computedHash := sha256.Sum256(dataReply.Data)
	return bytes.Equal(computedHash[:], dataReply.HashValue)
}

// func (fH *FileHandler) WriteFileAt(metahash string) {
// 	fH.indexedFiles[metahash].saveFile()
// }
