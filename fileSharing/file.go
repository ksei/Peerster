package filesharing

import "fmt"

const (
	CREATED     = -2
	INCOMPLETE  = -1
	DOWNLOADING = 0
	INDEXED     = 1
)

//File is the base struct for file exchange
type File struct {
	Name   string
	meta   *Metadata
	status int
}

//NewIndexedFile used to obtain new file from local source
func NewIndexedFile(fileName string) (*File, error) {
	file := &File{Name: fileName}
	meta, err := createMetadata(fileName)
	if err != nil {
		return nil, err
	}
	file.meta = meta
	if err = file.meta.evaluateForFile(); err != nil {
		return nil, err
	}
	file.status = INDEXED
	return file, nil
}

//NewIncomingFile used to obtain an empty file struct
func NewIncomingFile(fileName string, metaHash []byte) *File {
	partialMeta := &Metadata{
		fileName:    fileName,
		metahash:    metaHash,
		metaFile:    nil,
		chunkMap:    make(map[string][]byte),
		totalChunks: 0,
	}
	file := &File{
		Name:   fileName,
		meta:   partialMeta,
		status: CREATED}

	return file
}

//GetMetaHash returns the hash of the metafile generated from the given file
func (f *File) GetMetaHash() []byte {
	return f.meta.getMetaHash()
}

//GetChunkMapByIndex returns a list of indexes for existing chunks
func (f *File) GetChunkMapByIndex() []uint64 {
	return f.meta.getChunkMapByIndex()
}

//AddMetafile to a file after creation
func (f *File) AddMetafile(metafile []byte) {
	f.meta.metaFile = metafile
	f.status = DOWNLOADING
}

//AddChunk adds individual chunks to the file's meta
func (f *File) AddChunk(chunk []byte) {
	f.meta.addChunk(chunk)
}

func (f *File) saveFile() {
	f.meta.computeSize()
	fmt.Println("RECONSTRUCTED file", f.Name)
	f.meta.writeFileBytesToDisk()
}

//GetTotalChunks total amount of chunks available
func (f *File) GetTotalChunks() int {
	return f.meta.getTotalChunks()
}

func (f *File) GetSize() int64 {
	return f.meta.getSize()
}
