package download

import (
	"fmt"
	virtualfile "go-torrent-client/internals/virtual_file"
	"os"
)

type Download struct {
	file TorrentFileStorage
}

type TorrentFileStorage interface {
	Close() error
	WriteAt(p []byte, off int64) (n int, err error)
	ReadAt(p []byte, off int64) (n int, err error)
	Sync() error
}

func NewDownload(length uint32, location string) *Download {
	f, err := os.Create(location)
	if err != nil {
		panic(fmt.Errorf("failed to create file: %w", err))
	}
	if _, err := f.Seek(int64(length-1), 0); err != nil {
		f.Close()
		panic(fmt.Errorf("failed to seek: %w", err))
	}
	if _, err := f.Write([]byte{0}); err != nil {
		f.Close()
		panic(fmt.Errorf("failed to write last byte: %w", err))
	}
	return &Download{file: f}
}

func NewMultiFileDownload(files []map[string]any, location string) (int64, *Download ,error) {
	vf, totalLength, err := virtualfile.NewVirtualFile(files, location)
	if err != nil {
		return 0, nil, err
	}
	return totalLength ,&Download{file: vf}, nil
}

func NewDownloadFromExistingFile(location string) (*Download, error) {
	f, err := os.OpenFile(location, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	return &Download{file: f}, nil
}

func (d *Download) Close() error {
	return d.file.Close()
}

func (d *Download) WritePiece(index int32, data []byte) error {
	_, err := d.file.WriteAt(data, int64(index))
	return err
}

func (d *Download) GetPiece(index int32, length int32) ([]byte, error) {
	data := make([]byte, length)
	_, err := d.file.ReadAt(data, int64(index))
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (d *Download) GetFile() TorrentFileStorage {
	return d.file
}

func (d *Download) Sync() error {
	return d.file.Sync()
}

func (d *Download) Save() error {
	return d.file.Close()
}
