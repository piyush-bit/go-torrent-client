package download

import (
	"crypto/sha1"
	"sync"
)

type Status struct {
	Empty     int
	Requested int
	Received  int
}

type Piece struct {
	Index        int32
	Data         []byte
	bufferStatus []int8 // 0:empty , 1:requested , 2:received
	mu           sync.Mutex
	HasChanged   chan bool
	status       *Status
}

func NewPiece(index int32, pieceLength int, bufferSize int) *Piece {
	p := &Piece{
		Index:        index,
		Data:         make([]byte, pieceLength),
		bufferStatus: make([]int8, (pieceLength-1)/bufferSize+1),
		mu:           sync.Mutex{},
		HasChanged:   make(chan bool, 1),
		status:       &Status{Empty: (pieceLength-1)/bufferSize + 1, Requested: 0, Received: 0},
	}
	p.HasChanged <- true
	return p
}

func (p *Piece) changeStatus(empty, requested, received int) {
	p.status = &Status{Empty: empty, Requested: requested, Received: received}
	select {
	case p.HasChanged <- true:
	default:
		return
	}
}

func (p *Piece) calculateStatus() {
	var empty, requested, received int
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
	p.changeStatus(empty, requested, received)
}

func (p *Piece) WriteData(index int32, data []byte) {
	// fmt.Println("Piece ", p.Index, " written at index ", index)
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
	p.changeStatus(len(p.bufferStatus), 0, 0)
}

func (p *Piece) ClearRequested() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := 0; i < len(p.bufferStatus); i++ {
		if p.bufferStatus[i] == 1 {
			p.bufferStatus[i] = 0
		}
	}
	p.calculateStatus()
}

func (p *Piece) SetRequested(index int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.bufferStatus[index] = 1
	p.calculateStatus()
}

func (p *Piece) UnsetRequested(index int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.bufferStatus[index] == 1 {
		p.bufferStatus[index] = 0
	}
	p.calculateStatus()
}

func (p *Piece) SetReceived(index int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.bufferStatus[index] = 2
	p.calculateStatus()
}

func (p *Piece) UnsetReceived(index int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.bufferStatus[index] = 0
	p.calculateStatus()
}

func (p *Piece) Status() (empty, requested, received int) {
	return p.status.Empty, p.status.Requested, p.status.Received
}

func (p *Piece) IsComplete() bool {
	_, _, received := p.Status()
	return received == len(p.bufferStatus)
}

func (p *Piece) GetEmptyIndex() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	empty, _, _ := p.Status()
	if empty == 0 {
		return -1
	}
	for i := 0; i < len(p.bufferStatus); i++ {
		if p.bufferStatus[i] == 0 {
			return i
		}
	}
	return -1
}

func (p *Piece) GetTotalBufferLen() int {
	return len(p.bufferStatus)
}
