package peer

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	handshake "go-torrent-client/internals/Handshake"
	"go-torrent-client/internals/bencoding"
	torrentfile "go-torrent-client/internals/torrent_file"
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
	Peer           peer
	Conn           net.Conn
	PeerId         [20]byte
	InfoHash       [20]byte
	AmChoked       bool
	PeerChoked     bool
	AmInterested   bool
	PeerInterested bool
	Bitfield       []byte
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
	peerId := GeneratePeerId()
	tf.PeerId = peerId
	url, err := tf.BuildAnnounceURL(peerId, 3100)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	resp, err := http.Get(url)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	data, _, err := bencoding.ParseBencode(body)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	peers, err := ParsePeers(data.(map[string]any)["peers"].([]byte))
	if err != nil {
		fmt.Println(err)
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
	conn, err := net.DialTimeout("tcp", p.String(), 10*time.Second)
	if err != nil {
		fmt.Println(err)
		return nil , err
	}
	payload := handshake.CreateHandshakePayload(tf)
	conn.Write(payload)

	buffer := make([]byte, 68)
	_, err = conn.Read(buffer)
	if err != nil {
		fmt.Println(err)
		return nil ,err
	}
	peerId, ok := handshake.VerifyHandshakePayload(buffer, tf)
	if !ok {
		fmt.Println("handshake failed")
		return nil , fmt.Errorf("handshake failed")
	}

	fmt.Println("peer id: ", peerId)
	return &peerConnection{
		Peer: *p,
		Conn: conn,
		PeerId: peerId,
		InfoHash: tf.InfoHash,
		AmChoked: true,
		PeerChoked: true,
		AmInterested: false,
		PeerInterested: false,
		Bitfield: nil,
	}, nil
}

func (p *peerConnection) Close() error {
	return p.Conn.Close()
}

