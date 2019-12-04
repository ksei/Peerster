package filesharing

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	"os"
	"strings"
	"sync"
)

const fileDirectory = "./_SharedFiles/"
const downloadDirectory = "./_Downloads/"
const chunkSize int64 = 8192

//Metadata stores information on file indexing
type Metadata struct {
	fileName    string
	fileSize    int64
	metaFile    []byte
	metahash    []byte
	chunkMap    map[string][]byte
	totalChunks int
	locker      sync.RWMutex
}

func createMetadata(fName string) (*Metadata, error) {
	if len(fName) == 0 {
		return nil, errors.New("File name could not be resolved: empty file name recieved")
	}
	metadata := &Metadata{
		fileName:    fName,
		chunkMap:    make(map[string][]byte),
		totalChunks: 0,
	}

	fileInfo, err := os.Stat(fileDirectory + fName)
	if err != nil {
		return nil, errors.New("File name could not be resolved: file not found in directory")
	}
	metadata.fileSize = fileInfo.Size()

	return metadata, nil
}

func (metadata *Metadata) evaluateForFile() error {
	f, err := os.Open(fileDirectory + metadata.fileName)
	if err != nil {
		return err
	}

	defer func() {
		if err = f.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	chunkCount := 0
	remainingBytes := metadata.fileSize
	reader := bufio.NewReader(f)
	for remainingBytes > 0 {
		bufferLength := chunkSize
		if bufferLength > remainingBytes {
			bufferLength = remainingBytes
		}
		chunk := make([]byte, bufferLength)
		_, err := reader.Read(chunk)
		if err != nil {
			return err
		}

		hashedChunk := sha256.Sum256(chunk)
		tmpHashedChunk := hashedChunk[:]
		// fmt.Println("chunk:" + hex.EncodeToString(tmpHashedChunk))
		metadata.chunkMap[hex.EncodeToString(tmpHashedChunk)] = chunk
		metadata.metaFile = append(metadata.metaFile, tmpHashedChunk...)
		remainingBytes -= bufferLength
		chunkCount++
	}

	tmpMetaHash := sha256.Sum256(metadata.metaFile)
	metadata.metahash = tmpMetaHash[:]
	// fmt.Println("metafile:" + hex.EncodeToString(metadata.metaFile))
	metadata.totalChunks = chunkCount
	return nil
}

func (metadata *Metadata) getMetaHash() []byte {
	metadata.locker.RLock()
	defer metadata.locker.RUnlock()
	return metadata.metahash
}

func (metadata *Metadata) GetChunkByHash(chunkHash []byte) ([]byte, bool) {
	metadata.locker.RLock()
	defer metadata.locker.RUnlock()
	resp, ok := metadata.chunkMap[hex.EncodeToString(chunkHash)]
	return resp, ok
}

func (metadata *Metadata) validateHash(hashVal []byte) int {
	metadata.locker.RLock()
	defer metadata.locker.RUnlock()
	if bytes.Equal(metadata.metahash, hashVal) {
		return 0
	}
	if _, ok := metadata.chunkMap[hex.EncodeToString(hashVal)]; ok {
		return 1
	}
	return 0
}

func (metadata *Metadata) addChunk(chunk []byte) {
	metadata.locker.Lock()
	defer metadata.locker.Unlock()
	metadata.totalChunks++
	hashedChunk := sha256.Sum256(chunk)
	tmpHashedChunk := hashedChunk[:]
	metadata.chunkMap[hex.EncodeToString(tmpHashedChunk)] = chunk
}

func (metadata *Metadata) reconstructFileBytes() []byte {
	fileBytes := []byte{}
	hashSize := 32
	metadata.locker.RLock()
	defer metadata.locker.RUnlock()
	for i := 0; i < len(metadata.metaFile); i += hashSize {
		fileBytes = append(fileBytes, metadata.chunkMap[hex.EncodeToString(metadata.metaFile[i:i+hashSize])]...)
	}
	metadata.fileSize = int64(len(fileBytes))
	return fileBytes
}

func (metadata *Metadata) writeFileBytesToDisk() error {
	f, err := os.Create(downloadDirectory + metadata.fileName)
	if err != nil {
		return err
	}
	defer func() {
		if err = f.Close(); err != nil {
			log.Fatal(err)
		}
	}()
	// fmt.Println("fileBytes:" + hex.EncodeToString(metadata.reconstructFileBytes()))
	_, err = f.Write(metadata.reconstructFileBytes())
	if err != nil {
		return err
	}
	return nil
}

func (metadata *Metadata) computeSize() {
	totalChunks := len(metadata.metaFile)
	size := int64((totalChunks/32 - 1) * int(chunkSize))
	size += int64(len(metadata.chunkMap[hex.EncodeToString(metadata.metaFile[(totalChunks/32-1)*32:])]))
	metadata.fileSize = size
	// fmt.Println(size)
}

func (metadata *Metadata) getChunkMapByIndex() []uint64 {
	metadata.locker.RLock()
	defer metadata.locker.RUnlock()
	indices := []uint64{}
	metafileString := hex.EncodeToString(metadata.metaFile)
	for chunkHash := range metadata.chunkMap {
		indices = append(indices, uint64(strings.Index(metafileString, chunkHash)/32)+1)
	}
	return indices
}

func (metadata *Metadata) getTotalChunks() int {
	metadata.locker.RLock()
	defer metadata.locker.RUnlock()
	return metadata.totalChunks
}

func (metadata *Metadata) getSize() int64 {
	return metadata.fileSize
}
