package virtualfile

import (
	"fmt"
	"os"
	"path/filepath"
)

type File struct {
	length     int64
	startIndex int64
	file       *os.File
}
type VirtualFile struct {
	files []*File
}

// take files array from the decoded torrent file and create virtual files
func NewVirtualFile(filesData []map[string]any, name string) (vf *VirtualFile, totalLength int64, err error) {
	if len(filesData) == 0 {
		return nil, 0, fmt.Errorf("no files provided")
	}
	
	var files []*File

	for _, f := range filesData {
		// Extract length and path first
		length := int64(0)
		var parts []string
		
		lengthVal, hasLength := f["length"]
		pathVal, hasPath := f["path"]
		
		if !hasLength || !hasPath {
			return nil, 0, fmt.Errorf("missing length or path")
		}
		
		// Process length
		n, ok := lengthVal.(int64)
		if !ok {
			return nil, 0, fmt.Errorf("length is not int64")
		}
		length = n
		
		// Process path
		partsAny, ok := pathVal.([]any)
		if !ok {
			return nil, 0, fmt.Errorf("path is not a list")
		}
		
		for _, p := range partsAny {
			b, ok := p.([]byte)
			if !ok {
				return nil, 0, fmt.Errorf("path element is not []byte")
			}
			parts = append(parts, string(b))
		}
		
		fullPath := filepath.Join(append([]string{name}, parts...)...)
		
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			closeFiles(files) // Helper to close already opened files
			return nil, 0, err
		}

		file, err := os.OpenFile(fullPath, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			closeFiles(files)
			return nil, 0, err
		}

		fileInfo, err := file.Stat()
		if err != nil {
			closeFiles(files)
			return nil, 0, err
		}

		if fileInfo.Size() < length {
			if _, err := file.Seek(int64(length-1), 0); err != nil {
				file.Close()
				closeFiles(files)
				return nil, 0, err
			}
			if _, err := file.Write([]byte{0}); err != nil {
				file.Close()
				closeFiles(files)
				return nil, 0, err
			}
		}
		files = append(files, &File{
			length:     length,
			startIndex: totalLength,
			file:       file,
		})
		
		totalLength += length
	}

	return &VirtualFile{files: files}, totalLength, nil
}

func closeFiles(files []*File) {
	for _, f := range files {
		f.file.Close()
	}
}

func (vf *VirtualFile) Close() error {
	var firstErr error
	for _, f := range vf.files {
		if err := f.file.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}


func (vf *VirtualFile) WriteAt(p []byte, off int64) (int, error) {
    totalLen := int64(len(p))
    limit := vf.files[len(vf.files)-1].startIndex + vf.files[len(vf.files)-1].length

    if off+totalLen > limit {
        return 0, fmt.Errorf("index out of range")
    }

    written := int64(0)

    for _, f := range vf.files {
        if written >= totalLen {
            break
        }

        globalStart := off + written       // where we are writing in virtual file 	 
        globalEnd := off + totalLen        // end of write globally
        fileStart := f.startIndex
        fileEnd := f.startIndex + f.length

        // No overlap
        if globalStart >= fileEnd || globalEnd <= fileStart {
            continue
        }

        // Compute how much we can write in this file
        chunkStart := max(globalStart, fileStart)
        chunkEnd   := min(globalEnd,   fileEnd)

        writeLen := chunkEnd - chunkStart
        if writeLen <= 0 {
            continue
        }

        // offsets
        pStart := int(written + (chunkStart - globalStart))
        pEnd   := pStart + int(writeLen)

        fileOffset := chunkStart - fileStart

        n, err := f.file.WriteAt(p[pStart:pEnd], fileOffset)
        if err != nil {
            return int(written), err
        }

        written += int64(n)
    }

    return int(written), nil
}


func (vf *VirtualFile) ReadAt(p []byte, off int64) (int, error) {
    totalLen := int64(len(p))
    limit := vf.files[len(vf.files)-1].startIndex + vf.files[len(vf.files)-1].length

    if off+totalLen > limit {
        return 0, fmt.Errorf("index out of range")
    }

    read := int64(0)

    for _, f := range vf.files {
        if read >= totalLen {
            break
        }

        globalStart := off + read
        globalEnd := off + totalLen
        fileStart := f.startIndex
        fileEnd := f.startIndex + f.length

        if globalStart >= fileEnd || globalEnd <= fileStart {
            continue
        }

        chunkStart := max(globalStart, fileStart)
        chunkEnd := min(globalEnd, fileEnd)

        readLen := chunkEnd - chunkStart
        if readLen <= 0 {
            continue
        }

        pStart := int(read + (chunkStart - globalStart))
        pEnd := pStart + int(readLen)

        fileOffset := chunkStart - fileStart

        n, err := f.file.ReadAt(p[pStart:pEnd], fileOffset)
        if err != nil {
            return int(read), err
        }

        read += int64(n)
    }

    return int(read), nil
}


func (vf *VirtualFile) Sync() error {
	for _, f := range vf.files {
		if err := f.file.Sync(); err != nil {
			return err
		}
	}
	return nil
}
	

