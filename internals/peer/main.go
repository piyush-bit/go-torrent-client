package peer

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	handshake "go-torrent-client/internals/Handshake"
	"go-torrent-client/internals/bencoding"
	torrentfile "go-torrent-client/internals/torrent_file"
	bitfield "go-torrent-client/internals/bitfield"
	message "go-torrent-client/internals/message"
	"io"
	"net"
	"net/http"
	"strconv"
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

func RetrivePeers(tf *torrentfile.TorrentFile) ([]peer, error) {
	peerId := tf.PeerId
	if peerId == [20]byte{} {
		peerId = GeneratePeerId()
		tf.PeerId = peerId
	}
	url, err := tf.BuildAnnounceURL(peerId, 3100)
	if err != nil {
		return nil, err
	}

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	data, _, err := bencoding.ParseBencode(body)
	if err != nil {
		return nil, err
	}
	peers, err := ParsePeers(data.(map[string]any)["peers"].([]byte))
	if err != nil {
		return nil, err
	}
	return peers, nil
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
		Bitfield:       bitfield.NewBitfield(int32(tf.Info.Length / tf.Info.PieceLength)),
		Outgoing:       make(chan *message.Message, 20),
	}, nil
}

func (p *peerConnection) Close() error {
	return p.Conn.Close()
}

func (p *peerConnection) ReadMessage() (*message.Message, error) {
	buffer := make([]byte, 4)
	_, err := p.Conn.Read(buffer)
	if err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(buffer)
	buffer = make([]byte, length)
	_, err = p.Conn.Read(buffer)
	if err != nil {
		return nil, err
	}
	var payload []byte
	if length > 1 {
		payload = buffer[1:]
	}
	if length == 0 {
		return &message.Message{ID: message.MsgKeepAlive}, nil
	}
	return &message.Message{
		ID:   message.MessageID(buffer[0]),
		Data: payload,
	}, nil
}

func (p *peerConnection) WriteMessage(msg *message.Message) error {
	buffer := msg.Serialize()
	fmt.Println("writing message : ", buffer)
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
		case message.MsgUnchoke:
			p.AmChoked = false
			p.Outgoing <- message.Request(p.Bitfield.FirstSetBit(0), 0, int32(16*1024))
		case message.MsgInterested:
			p.PeerInterested = true
			if !p.PeerChoked{
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
			// TODO : handle piece
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
		fmt.Println("Received message : ", msg.ID)
		p.DecodeMessage(msg)
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

		fmt.Println("Sent message : ", msg.ID)
	}
	return nil
}
