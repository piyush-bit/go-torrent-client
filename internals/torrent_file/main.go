package torrentfile

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	bitfield "go-torrent-client/internals/bitfield"
	"go-torrent-client/internals/bencoding"
	"net/url"
	"os"
	"strconv"
)

type TorrentFile struct {
	Info     TorrentInfo
	Announce string
	InfoHash [20]byte
	PeerId   [20]byte
	Bitfield bitfield.Bitfield
}

// using single file for now , will add multiple files later
//
//	TODO : add multiple files
type TorrentInfo struct {
	Name        string
	PieceLength int64
	Pieces      []byte
	Length      int64
}

func (tf *TorrentFile) ParseTorrentFile(data []byte) error {
	parsedData, _, err := bencoding.ParseBencode(data)
	if err != nil {
		return err
	}
	m, ok := parsedData.(map[string]any)
	if !ok {
		return fmt.Errorf("invalid torrent file : root not dictionary")
	}
	announce, ok := m["announce"].([]byte)
	if !ok {
		return fmt.Errorf("invalid torrent file : announce not byte array")
	}
	tf.Announce = string(announce)
	info, ok := m["info"].(map[string]any)
	if !ok {
		return fmt.Errorf("invalid torrent file : info not dictionary")
	}

	infoKey := "4:info"
	index := bytes.Index(data, []byte(infoKey))
	if index == -1 {
		return fmt.Errorf("invalid torrent file : info not found")
	}
	infoData := data[index+len(infoKey):]
	_, length, err := bencoding.ParseBencode(infoData)
	if err != nil {
		return err
	}

	bencodedInfo := infoData[:length]
	tf.InfoHash = sha1.Sum(bencodedInfo)

	if name, ok := info["name"].([]byte); ok {
		tf.Info.Name = string(name)
	} else {
		return fmt.Errorf("invalid torrent file : name not 	byte array")
	}
	if pieceLength, ok := info["piece length"].(int64); ok {
		tf.Info.PieceLength = pieceLength
	} else {
		return fmt.Errorf("invalid torrent file : piece length not int64")
	}
	if pieces, ok := info["pieces"].([]byte); ok {
		tf.Info.Pieces = pieces
	} else {
		return fmt.Errorf("invalid torrent file : pieces not byte array")
	}
	if length, ok := info["length"].(int64); ok {
		tf.Info.Length = length
	} else {
		return fmt.Errorf("invalid torrent file : length not int")
	}

	tf.Bitfield = bitfield.NewBitfield(int32(tf.Info.Length / tf.Info.PieceLength))
	return nil
}

func ParseTorrentFile(path string) (*TorrentFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	tf := &TorrentFile{}
	err = tf.ParseTorrentFile(data)
	if err != nil {
		return nil, err
	}
	return tf, nil
}

func (tf *TorrentFile) BuildAnnounceURL(peerId [20]byte, port int) (string, error) {
	u, err := url.Parse(tf.Announce)
	if err != nil {
		return "", err
	}
	params := url.Values{
		"info_hash": {string(tf.InfoHash[:])},
		"peer_id":   {string(peerId[:])},
		"port":      {strconv.Itoa(port)},
		"uploaded":  {"0"},
		"downloaded": {"0"},
		"left": {strconv.FormatInt(tf.Info.Length, 10)},
		"compact":   {"1"},
		"event":     {"started"},
	}
	u.RawQuery = params.Encode()
	return u.String(), nil
}
