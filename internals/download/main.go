package download

import (
	"crypto/sha1"
	"fmt"
	"os"
	"sync"
)

type Download struct {
	file *os.File
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

func (d *Download) Close() error {
	return d.file.Close()
}

func (d *Download) WritePiece(index int32, data []byte) error {
	_, err := d.file.WriteAt(data, int64(index))
	return err
}

type Piece struct {
	Index        int32
	Data         []byte
	bufferStatus []int8 // 0:empty , 1:requested , 2:received
	mu           sync.Mutex
}

func NewPiece(index int32, pieceLength int, bufferSize int) *Piece {
	return &Piece{
		Index:        index,
		Data:         make([]byte, pieceLength),
		bufferStatus: make([]int8, (pieceLength-1)/bufferSize+1),
		mu:           sync.Mutex{},
	}
}

func (p *Piece) WriteData(index int32, data []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	copy(p.Data[index:], data)
}

func (p *Piece) Verify(hash [20]byte) bool {
	return sha1.Sum(p.Data) == hash
}

func (p *Piece) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := 0; i < len(p.bufferStatus); i++ {
		p.bufferStatus[i] = 0
	}
}

func (p *Piece) ClearRequested() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := 0; i < len(p.bufferStatus); i++ {
		if p.bufferStatus[i] == 1 {
			p.bufferStatus[i] = 0
		}
	}
}

func (p *Piece) SetRequested(index int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.bufferStatus[index] = 1
}

func (p *Piece) UnsetRequested(index int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.bufferStatus[index] == 1 {
		p.bufferStatus[index] = 0
	}
}

func (p *Piece) SetReceived(index int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.bufferStatus[index] = 2
}

func (p *Piece) UnsetReceived(index int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.bufferStatus[index] = 0
}

func (p *Piece) Status() (empty, requested, received int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := 0; i < len(p.bufferStatus); i++ {
		switch p.bufferStatus[i] {
		case 0:
			empty++
		case 1:
			requested++
		case 2:
			received++
		}
	}
	return empty, requested, received
}

func (p *Piece) IsComplete() bool {
	_, _, received := p.Status()
	return received == len(p.bufferStatus)
}

func (p *Piece) GetEmptyIndex() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := 0; i < len(p.bufferStatus); i++ {
		if p.bufferStatus[i] == 0 {
			return i
		}
	}
	return -1
}
