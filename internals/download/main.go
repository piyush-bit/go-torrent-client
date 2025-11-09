package download

import "crypto/sha1"

type Download struct {
	file []byte
}

type Piece struct {
	Index        int32
	data         []byte
	bufferStatus []int8 // 0:empty , 1:requested , 2:received
}

func NewPiece(index int32, pieceLength int, bufferSize int) *Piece {
	return &Piece{
		Index:        index,
		data:         make([]byte, pieceLength),
		bufferStatus: make([]int8, (pieceLength-1)/bufferSize+1),
	}
}

func (p *Piece) WriteData(index int32, data []byte) {
	copy(p.data[index:], data)
}

func (p *Piece) Verify(hash [20]byte) bool {
	return sha1.Sum(p.data) == hash
}

func (p *Piece) Clear(){
	for i := 0; i < len(p.bufferStatus); i++ {
		p.bufferStatus[i] = 0
	}
}

func (p *Piece) ClearRequested() {
	for i := 0; i < len(p.bufferStatus); i++ {
		if p.bufferStatus[i] == 1 {
			p.bufferStatus[i] = 0
		}
	}
}

func (p *Piece) SetRequested(index int) {
	p.bufferStatus[index] = 1
}

func (p *Piece) UnsetRequested(index int) {
	if p.bufferStatus[index] == 1 {
		p.bufferStatus[index] = 0
	}
}

func (p *Piece) SetReceived(index int) {
	p.bufferStatus[index] = 2
}

func (p *Piece) UnsetReceived(index int) {
	p.bufferStatus[index] = 0
}

func (p *Piece) Status() (empty, requested, received int) {
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
	for i := 0; i < len(p.bufferStatus); i++ {
		if p.bufferStatus[i] == 0 {
			return i
		}
	}
	return -1
}

func NewDownload(length int64) *Download {
	return &Download{
		file: make([]byte, length),
	}
}

func (d *Download) WritePiece(index int32, data []byte) error {
	copy(d.file[index:], data)
	return nil
}
