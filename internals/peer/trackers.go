package peer

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"go-torrent-client/internals/bencoding"
	torrentfile "go-torrent-client/internals/torrent_file"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const UDP_TRACKER_TIMEOUT = 2 * time.Second

// generate random uint32 transaction id
func randUint32() uint32 {
	var b [4]byte
	rand.Read(b[:])
	return binary.BigEndian.Uint32(b[:])
}

func UdpTrackerRequest(tf *torrentfile.TorrentFile) ([]peer, int, error) {
	tracker := tf.Announce
	addr, err := net.ResolveUDPAddr("udp", strings.TrimPrefix(tracker, "udp://"))
	if err != nil {
		return nil, 0, fmt.Errorf("resolve: %w", err)
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, 0, fmt.Errorf("dial udp: %w", err)
	}
	defer conn.Close()


	var buf bytes.Buffer
	protocolID := uint64(0x41727101980)
	action := uint32(0)
	transactionID := randUint32()

	binary.Write(&buf, binary.BigEndian, protocolID)
	binary.Write(&buf, binary.BigEndian, action)
	binary.Write(&buf, binary.BigEndian, transactionID)

	_, err = conn.Write(buf.Bytes())
	if err != nil {
		return nil, 0, fmt.Errorf("send connect: %w", err)
	}

	// wait for response (UDP timeout)
	conn.SetReadDeadline(time.Now().Add(UDP_TRACKER_TIMEOUT))

	resp := make([]byte, 2048)
	n, err := conn.Read(resp)
	if err != nil {
		return nil, 0, fmt.Errorf("connect resp: %w", err)
	}

	if n < 16 {
		return nil, 0, fmt.Errorf("connect response too short")
	}

	respAction := binary.BigEndian.Uint32(resp[0:4])
	respTransID := binary.BigEndian.Uint32(resp[4:8])

	if respAction != 0 || respTransID != transactionID {
		return nil, 0, fmt.Errorf("invalid connect response")
	}

	connectionID := binary.BigEndian.Uint64(resp[8:16])

	fmt.Println("Connection ID: ", connectionID)

	
	var ann bytes.Buffer

	action = 1
	transactionID = randUint32()
	if tf.PeerId == [20]byte{} {
		tf.PeerId = GeneratePeerId()
	}
	peerID := tf.PeerId
	downloaded := uint64(0)
	uploaded := uint64(0)
	left := uint64(0) // if non-zero, tracker thinks you want all peers

	event := uint32(0)
	ip := uint32(0)
	key := randUint32()
	numWant := int32(-1) // default
	// port is given in function arg

	binary.Write(&ann, binary.BigEndian, connectionID)
	binary.Write(&ann, binary.BigEndian, action)
	binary.Write(&ann, binary.BigEndian, transactionID)
	ann.Write(tf.InfoHash[:])
	ann.Write(peerID[:])
	binary.Write(&ann, binary.BigEndian, downloaded)
	binary.Write(&ann, binary.BigEndian, left)
	binary.Write(&ann, binary.BigEndian, uploaded)
	binary.Write(&ann, binary.BigEndian, event)
	binary.Write(&ann, binary.BigEndian, ip)
	binary.Write(&ann, binary.BigEndian, key)
	binary.Write(&ann, binary.BigEndian, numWant)
	binary.Write(&ann, binary.BigEndian, uint16(tf.Port))

	_, err = conn.Write(ann.Bytes())
	if err != nil {
		return nil, 0, fmt.Errorf("send announce: %w", err)
	}

	conn.SetReadDeadline(time.Now().Add(UDP_TRACKER_TIMEOUT))
	n, err = conn.Read(resp)
	if err != nil {
		return nil, 0, fmt.Errorf("announce resp: %w", err)
	}

	if n < 20 {
		return nil, 0, fmt.Errorf("announce response too short")
	}

	respAction = binary.BigEndian.Uint32(resp[0:4])
	respTransID = binary.BigEndian.Uint32(resp[4:8])
	interval := binary.BigEndian.Uint32(resp[8:12])

	if respAction == 3 {
		return nil, 0, fmt.Errorf("tracker error: %s", string(resp[8:n]))
	}

	if respAction != 1 || respTransID != transactionID {
		return nil, 0, fmt.Errorf("invalid announce response")
	}

	peers := []peer{}
	offset := 20

	for offset+6 <= n {
		ip := net.IPv4(resp[offset], resp[offset+1], resp[offset+2], resp[offset+3])
		port := binary.BigEndian.Uint16(resp[offset+4 : offset+6])
		offset += 6

		peers = append(peers, peer{
			ip:   ip.String(),
			port: int(port),
		})
	}

	return peers, int(interval), nil
}

func HttpTrackerRequest(tf *torrentfile.TorrentFile) ([]peer, int, error) {
	peerId := tf.PeerId
	if peerId == [20]byte{} {
		peerId = GeneratePeerId()
		tf.PeerId = peerId
	}
	url, err := tf.BuildAnnounceURL(peerId, tf.Port)
	if err != nil {
		return nil, 0, err
	}

	resp, err := http.Get(url)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}
	data, _, err := bencoding.ParseBencode(body)
	if err != nil {
		return nil, 0, err
	}
	peers, err := ParsePeers(data.(map[string]any)["peers"].([]byte))
	if err != nil {
		return nil, 0, err
	}
	return peers, int(data.(map[string]any)["interval"].(int64)), nil
}
