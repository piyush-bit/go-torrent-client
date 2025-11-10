package torrentfile

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"go-torrent-client/internals/bencoding"
	bitfield "go-torrent-client/internals/bitfield"
	download "go-torrent-client/internals/download"
	"net/url"
	"os"
	"strconv"
	"time"
)

const (
	DOWNLOAD_BUFFER_SIZE = 16 * 1024
	REQUEST_TIMEOUT      = 60 * time.Second
	MAX_REQUESTS         = 5
	WORKING_PIECES       = 20
)

type TorrentFile struct {
	Info             TorrentInfo
	Announce         string
	InfoHash         [20]byte
	PeerId           [20]byte
	Bitfield         bitfield.Bitfield
	BitfieldLength   int32
	NeddedPieces     chan *download.Piece
	DownloadedPieces chan *download.Piece
	notifyDownload   chan bool
	Download         *download.Download
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
	tf.NeddedPieces = make(chan *download.Piece, WORKING_PIECES)
	tf.DownloadedPieces = make(chan *download.Piece, WORKING_PIECES)
	tf.BitfieldLength = int32(tf.Info.Length / tf.Info.PieceLength)
	tf.Bitfield = bitfield.NewBitfield(tf.BitfieldLength)
	tf.notifyDownload = make(chan bool, 2)
	tf.notifyDownload <- false
	tf.Download = download.NewDownload(uint32(tf.Info.Length), tf.Info.Name)
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
		"info_hash":  {string(tf.InfoHash[:])},
		"peer_id":    {string(peerId[:])},
		"port":       {strconv.Itoa(port)},
		"uploaded":   {"0"},
		"downloaded": {"0"},
		"left":       {strconv.FormatInt(tf.Info.Length, 10)},
		"compact":    {"1"},
		"event":      {"started"},
	}
	u.RawQuery = params.Encode()
	return u.String(), nil
}

func (tf *TorrentFile) UpdateNeddedPieces() {
	i := tf.BitfieldLength - 1
	for i >= 0 {
		increment := <-tf.notifyDownload
		if !increment {
			init := i
			for i > init-WORKING_PIECES && i >= 0 {
				piece := download.NewPiece(int32(i), int(tf.Info.PieceLength), DOWNLOAD_BUFFER_SIZE)
				tf.NeddedPieces <- piece
				fmt.Println("Piece added to nedded pieces :", piece.Index)
				i--
			}
		} else {
			piece := download.NewPiece(int32(i), int(tf.Info.PieceLength), DOWNLOAD_BUFFER_SIZE)
			tf.NeddedPieces <- piece
			fmt.Println("Piece added to nedded pieces :", piece.Index)
			i--
		}
	}
}

func (tf *TorrentFile) UpdateDownloadedPieces() {
	for {
		dowloadedPiece := <-tf.DownloadedPieces
		if tf.Bitfield.Test(dowloadedPiece.Index) {
			fmt.Println("Piece already downloaded :", dowloadedPiece.Index)
			return
		}
		var hash [20]byte
		copy(hash[:], tf.Info.Pieces[dowloadedPiece.Index*20:(dowloadedPiece.Index+1)*20])
		//verify sha-1
		if dowloadedPiece.Verify(hash) {
			tf.Download.WritePiece(dowloadedPiece.Index, dowloadedPiece.Data)
			tf.Bitfield.Set(dowloadedPiece.Index)
			fmt.Println("Piece downloaded :", dowloadedPiece.Index)
			tf.notifyDownload <- true
		} else {
			fmt.Println("Piece verification failed :", dowloadedPiece.Index)
			dowloadedPiece.Clear()
			tf.NeddedPieces <- dowloadedPiece
		}
	}
}
