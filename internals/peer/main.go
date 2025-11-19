package peer

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	handshake "go-torrent-client/internals/Handshake"
	bitfield "go-torrent-client/internals/bitfield"
	download "go-torrent-client/internals/download"
	message "go-torrent-client/internals/message"
	torrentfile "go-torrent-client/internals/torrent_file"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

type peer struct {
	ip   string
	port int
}

type peerConnection struct {
	Tf             *torrentfile.TorrentFile
	Peer           peer
	Conn           net.Conn
	PeerId         [20]byte
	InfoHash       [20]byte
	AmChoked       bool
	PeerChoked     bool
	AmInterested   bool
	PeerInterested bool
	Bitfield       bitfield.Bitfield
	Outgoing       chan *message.Message
	workingPiece   *download.Piece
	chokedSignal   chan bool
	ScheduledRetry []*time.Timer
	workingPieceMu sync.Mutex
}

func (p *peer) String() string {
	return p.ip + ":" + strconv.Itoa(p.port)
}

func ParsePeers(data []byte) ([]peer, error) {
	n := len(data)
	ans := []peer{}
	for i := 0; i < n; i += 6 {
		ip := net.IP(data[i : i+4])
		port := binary.BigEndian.Uint16(data[i+4 : i+6])
		ans = append(ans, peer{ip.String(), int(port)})
	}
	return ans, nil
}

func RetrivePeers(tf *torrentfile.TorrentFile) ([]peer, int, error) {
	announce := tf.Announce 
	if strings.HasPrefix(announce,"http") {
		return HttpTrackerRequest(tf)
	} else if strings.HasPrefix(announce,"udp") {
		return udpTrackerRequest(tf)
	}
	return nil, 0, fmt.Errorf("invalid announce url")
}

func GeneratePeerId() [20]byte {
	peerId := [20]byte{}
	randombyte := make([]byte, 18)
	_, _ = rand.Read(randombyte)
	copy(peerId[:], fmt.Sprintf("-%s%d-%s", "GO", int32(0001), randombyte))
	return peerId
}

func (p *peer) Connect(tf *torrentfile.TorrentFile) (*peerConnection, error) {
	conn, err := net.DialTimeout("tcp", p.String(), 2*time.Second)
	if err != nil {
		return nil, err
	}
	payload := handshake.CreateHandshakePayload(tf)
	conn.Write(payload)

	buffer := make([]byte, 68)
	_, err = conn.Read(buffer)
	if err != nil {
		return nil, err
	}
	peerId, ok := handshake.VerifyHandshakePayload(buffer, tf)
	if !ok {
		return nil, fmt.Errorf("handshake failed")
	}

	return &peerConnection{
		Tf:             tf,
		Peer:           *p,
		Conn:           conn,
		PeerId:         peerId,
		InfoHash:       tf.InfoHash,
		AmChoked:       true,
		PeerChoked:     true,
		AmInterested:   false,
		PeerInterested: false,
		Bitfield:       bitfield.NewBitfield(int32(len(tf.Info.Pieces) / 20)),
		Outgoing:       make(chan *message.Message, 20),
		chokedSignal:   make(chan bool),
		ScheduledRetry: []*time.Timer{},
	}, nil
}

func (p *peer) SpawnPeer(tf *torrentfile.TorrentFile) error {
	peerConnection, err := p.Connect(tf)
	if err != nil {
		fmt.Println(err)
		return err
	}
	fmt.Println("Connected to peer")
	defer peerConnection.Close()
	errChan := make(chan error)
	go func() {
		err := peerConnection.ReadLoop()
		fmt.Println("Read loop done : ", err)
		errChan <- err
	}()
	go func() {
		err := peerConnection.WriteLoop()
		fmt.Println("Write loop done : ", err)
		errChan <- err
	}()

	peerConnection.Outgoing <- message.Bitfield(peerConnection.Tf.Bitfield)
	peerConnection.Outgoing <- message.Interested()
	return <-errChan
}

func (p *peerConnection) Close() error {
	return p.Conn.Close()
}

func (p *peerConnection) ReadMessage() (*message.Message, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(p.Conn, header); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(header)
	if length == 0 {
		return &message.Message{ID: message.MsgKeepAlive}, nil
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(p.Conn, body); err != nil {
		return nil, err
	}

	msg := &message.Message{
		ID: message.MessageID(body[0]),
	}
	if len(body) > 1 {
		msg.Data = body[1:]
	}
	return msg, nil
}

func (p *peerConnection) WriteMessage(msg *message.Message) error {
	buffer := msg.Serialize()
	_, err := p.Conn.Write(buffer)
	if err != nil {
		return err
	}
	return nil
}

func (p *peerConnection) DecodeMessage(msg *message.Message) error {

	switch msg.ID {
	case message.MsgChoke:
		p.AmChoked = true
		p.chokedSignal <- true
	case message.MsgUnchoke:
		p.AmChoked = false
		go p.FindWork()
	case message.MsgInterested:
		p.PeerInterested = true
		if !p.PeerChoked {
			p.Outgoing <- message.Unchoke()
		}
	case message.MsgNotInterested:
		p.PeerInterested = false
	case message.MsgHave:
		p.Bitfield.Set(msg.GetPieceIndex())
	case message.MsgBitfield:
		if len(msg.Data) != len(p.Bitfield) {
			return fmt.Errorf("invalid bitfield")
		}
		copy(p.Bitfield, msg.Data)
	case message.MsgRequest:
		// TODO : handle request
	case message.MsgPiece:
		// MsgPiece: <len=9+X><id=7><index><begin><block>
		p.workingPieceMu.Lock()
		defer p.workingPieceMu.Unlock()
		if len(msg.Data) < 8 {
			return fmt.Errorf("piece message too short: %d", len(msg.Data))
		}
		index := binary.BigEndian.Uint32(msg.Data[0:4])
		offset := binary.BigEndian.Uint32(msg.Data[4:8])
		data := msg.Data[8:] // rest is block payload

		// fmt.Println("Piece received header: index=", index, " begin=", offset, " dataLen=", len(data))

		if p.workingPiece == nil {
			return fmt.Errorf("received piece with no workingPiece set")
		}
		if index != uint32(p.workingPiece.Index) {
			return fmt.Errorf("invalid piece index: got %d expected %d", index, p.workingPiece.Index)
		}

		// Validate offset and bounds against current piece size (torrent piece length)
		pieceSize := int(p.Tf.Info.PieceLength)
		if int(offset) < 0 || int(offset) >= pieceSize {
			return fmt.Errorf("invalid piece offset: %d (pieceSize=%d)", offset, pieceSize)
		}
		if len(data) == 0 {
			return fmt.Errorf("empty piece block received")
		}
		if int(offset)+len(data) > pieceSize {
			return fmt.Errorf("piece block exceeds piece size: offset=%d len=%d pieceSize=%d", offset, len(data), pieceSize)
		}

		// Write block and mark corresponding chunk as received
		p.workingPiece.WriteData(int32(offset), data)
		p.workingPiece.SetReceived(int(offset / torrentfile.DOWNLOAD_BUFFER_SIZE))

	case message.MsgCancel:
		// TODO : handle cancel
	}
	return nil
}

func (p *peerConnection) ReadLoop() error {
	for {
		// p.Conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		msg, err := p.ReadMessage()
		if err != nil {
			// if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			// 	return nil
			// }
			return err
		}
		// fmt.Println("Received message : ", msg.ID)
		err = p.DecodeMessage(msg)
		if err != nil {
			fmt.Println("Error decoding message : ", err)
		}
	}
}

func (p *peerConnection) WriteLoop() error {
	for {
		msg, ok := <-p.Outgoing
		if !ok {
			break
		}
		err := p.WriteMessage(msg)
		if err != nil {
			return err
		}

		// fmt.Println("Sent message : ", msg.ID)
	}
	return nil
}

func (p *peerConnection) FindWork() error {
	for {
		select {
		case <-p.chokedSignal:
			// On choke: if we were working on a piece, put it back and reset.
			p.workingPieceMu.Lock()

			if p.workingPiece != nil {
				p.workingPiece.ClearRequested()
				p.Tf.NeddedPieces <- p.workingPiece
				p.workingPiece = nil
			}
			for _, retry := range p.ScheduledRetry {
				retry.Stop()
			}
			p.ScheduledRetry = []*time.Timer{}
			p.workingPieceMu.Unlock()

		default:
			// Ensure we have a workingPiece; block until there is valid work.
			p.workingPieceMu.Lock()
			if p.workingPiece == nil {
				piece := <-p.Tf.NeddedPieces
				fmt.Printf("Piece %d picked by peer %s\n", piece.Index, p.Peer.String())
				if !p.Bitfield.Test(piece.Index) {
					p.Tf.NeddedPieces <- piece
					p.workingPieceMu.Unlock()
					// fmt.Printf("Bitfield of :%s\n%v\n", p.Peer.String(), p.Bitfield)
					// return nil
					time.Sleep(time.Second)
					continue
				}

				p.workingPiece = piece
				// fmt.Printf("Piece %d picked by peer %s\n", piece.Index, p.Peer.String())
			}

			empty, requested, _ := p.workingPiece.Status()
			if requested < torrentfile.MAX_REQUESTS {
				maxNew := min(empty, torrentfile.MAX_REQUESTS-requested)
				for range maxNew {
					index := p.workingPiece.GetEmptyIndex()
					if index == -1 {
						break
					}
					bufferSize := torrentfile.DOWNLOAD_BUFFER_SIZE
					if index == p.workingPiece.GetTotalBufferLen()-1 {
						bufferSize = len(p.workingPiece.Data) % torrentfile.DOWNLOAD_BUFFER_SIZE
						if bufferSize == 0 {
							bufferSize = torrentfile.DOWNLOAD_BUFFER_SIZE
						}
					}

					p.Outgoing <- message.Request(
						p.workingPiece.Index,
						int32(index*torrentfile.DOWNLOAD_BUFFER_SIZE),
						int32(bufferSize),
					)
					p.workingPiece.SetRequested(index)

					idx := index
					currPiece := p.workingPiece
					retry := time.AfterFunc(torrentfile.REQUEST_TIMEOUT, func() {
						// Guard against nil in retry callback
						p.workingPieceMu.Lock()
						if p.workingPiece != nil && p.workingPiece.Index == currPiece.Index {
							p.workingPiece.UnsetRequested(idx)
						}
						p.workingPieceMu.Unlock()
					})
					p.ScheduledRetry = append(p.ScheduledRetry, retry)
				}
			}

			// Guard IsComplete with nil check to prevent panic.
			if p.workingPiece != nil && p.workingPiece.IsComplete() {
				p.Tf.DownloadedPieces <- p.workingPiece
				fmt.Printf("Piece %d downloaded by peer %s, len(%d)\n", p.workingPiece.Index, p.Peer.String(), len(p.Tf.DownloadedPieces))
				p.workingPiece = nil
			}
			p.workingPieceMu.Unlock()
		}
	}
}
