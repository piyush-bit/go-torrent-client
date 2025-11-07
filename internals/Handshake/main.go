package handshake

import (
	"bytes"
	torrentfile "go-torrent-client/internals/torrent_file"
)

const (
	pstrlen = 19
	pstr    = "BitTorrent protocol"
)

func CreateHandshakePayload(tf *torrentfile.TorrentFile) []byte {
	payload := make([]byte, 49+pstrlen) // 68 bytes total
	payload[0] = byte(pstrlen)
	copy(payload[1:], pstr)
	// [20:28] reserved for extensions
	copy(payload[28:], tf.InfoHash[:])
	copy(payload[48:], tf.PeerId[:])
	return payload
}

func VerifyHandshakePayload(payload []byte, tf *torrentfile.TorrentFile) ([20]byte, bool) {
	if len(payload) != 68 {
		return [20]byte{}, false
	}
	if payload[0] != byte(pstrlen) {
		return [20]byte{}, false
	}
	if string(payload[1:20]) != pstr {
		return [20]byte{}, false
	}
	if !bytes.Equal(payload[28:48], tf.InfoHash[:]) {
		return [20]byte{}, false
	}
	return [20]byte(payload[48:68]), true
}
