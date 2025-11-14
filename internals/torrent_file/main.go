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
	DownloadComplete chan bool
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
	tf.DownloadComplete = make(chan bool)
	return nil
}

func (tf *TorrentFile) ParseTorrentField(data []byte) error {
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

	tf.BitfieldLength = int32(len(tf.Info.Pieces) / 20)
	tf.Bitfield = bitfield.NewBitfield(tf.BitfieldLength)
	return nil
}

func (tf *TorrentFile) Initialize() error {
	tf.NeddedPieces = make(chan *download.Piece, WORKING_PIECES)
	tf.DownloadedPieces = make(chan *download.Piece, WORKING_PIECES)
	tf.notifyDownload = make(chan bool, 2)
	tf.notifyDownload <- false
	
	if _, err := os.Stat(tf.Info.Name); err == nil {
		tf.Bitfield.SetAll()
		tf.Download = download.NewDownloadFromExistingFile(tf.Info.Name)
		err := VerifyFileIntegrity(tf.Download.GetFile(),tf,&tf.Bitfield)
		if err != nil {
			return err
		}
	} else {
		tf.Download = download.NewDownload(uint32(tf.Info.Length), tf.Info.Name)
	}
	tf.DownloadComplete = make(chan bool)
	return nil
}


func ParseTorrentFile(path string) (*TorrentFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	tf := &TorrentFile{}
	err = tf.ParseTorrentField(data)
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
	i := tf.BitfieldLength -1	
	for {
		increment := <-tf.notifyDownload
		if i<0{
			continue
		}
		if !increment {
			count := 0
			for count < WORKING_PIECES	 && i >= 0 {
				for tf.Bitfield.Test(i) {
					i--
				}
				if i<0{
					break
				}
				pieceLength := tf.Info.PieceLength
				if i == tf.BitfieldLength-1 {
					pieceLength = tf.Info.Length % tf.Info.PieceLength
					if pieceLength == 0 {
						pieceLength = tf.Info.PieceLength
					}
				}
				piece := download.NewPiece(int32(i), int(pieceLength), DOWNLOAD_BUFFER_SIZE)
				tf.NeddedPieces <- piece
				fmt.Println("Piece added to nedded pieces :", piece.Index)
				count++
				i--
			}
		} else {
			for tf.Bitfield.Test(i) {
				i--
			}
			if i<0{
				break
			}
			pieceLength := tf.Info.PieceLength
			if i == tf.BitfieldLength-1 {
				pieceLength = tf.Info.Length % tf.Info.PieceLength
				if pieceLength == 0 {
					pieceLength = tf.Info.PieceLength
				}
			}
			piece := download.NewPiece(int32(i), int(pieceLength), DOWNLOAD_BUFFER_SIZE)
			tf.NeddedPieces <- piece
			fmt.Println("Piece added to nedded pieces :", piece.Index)
			i--
		}
	}
}

func (tf *TorrentFile) UpdateDownloadedPieces() {
	count := int32(0)
	lastPiece := int32(-1)
	for {
		// recived := make(chan bool)
		// ticker := time.NewTicker(time.Second)
		// go func() {
		// 	for {
		// 		select {
		// 		case <-recived:
		// 			ticker.Stop()
		// 			return
		// 		case <-ticker.C:
		// 			fmt.Println("Waiting for downloaded piece...")
		// 		}
		// 	}
		// }()
		fmt.Printf("Waiting for downloaded piece... current len : %d\n", len(tf.DownloadedPieces))
		dowloadedPiece := <-tf.DownloadedPieces
		fmt.Printf("Downloaded piece recived : %d\n", dowloadedPiece.Index)
		// recived <- true

		// Defensive: channel sender should never send nil, but guard anyway.
		if dowloadedPiece == nil {
			fmt.Println("Nil piece received on DownloadedPieces, skipping")
			continue
		}

		fmt.Printf("Piece %d recived for verification\n", dowloadedPiece.Index)

		if tf.Bitfield.Test(dowloadedPiece.Index) {
			fmt.Println("Piece already downloaded:", dowloadedPiece.Index)
			continue
		}

		// Unlikely to happen, but just in case.
		if dowloadedPiece.Index < 0 || dowloadedPiece.Index >= tf.BitfieldLength {
			fmt.Println("Invalid downloaded piece index, re-queuing:", dowloadedPiece.Index)
			continue
		}

		var hash [20]byte
		copy(hash[:], tf.Info.Pieces[dowloadedPiece.Index*20:(dowloadedPiece.Index+1)*20])

		if dowloadedPiece.Verify(hash) {
			// Persist data.
			if err := tf.Download.WritePiece(dowloadedPiece.Index*int32(tf.Info.PieceLength), dowloadedPiece.Data); err != nil {
				fmt.Println("Error writing piece to disk, re-queuing:", dowloadedPiece.Index, err)
				dowloadedPiece.Clear()
				tf.NeddedPieces <- dowloadedPiece
				continue
			}

			// Mark in bitfield.
			tf.Bitfield.Set(dowloadedPiece.Index)

			if err := tf.Download.Sync(); err != nil {
				fmt.Println("Error syncing piece to disk, re-queuing:", dowloadedPiece.Index, err)
				dowloadedPiece.Clear()
				tf.NeddedPieces <- dowloadedPiece
				continue
			}

			if lastPiece != -1 {
				data, err := tf.Download.GetPiece(lastPiece*int32(tf.Info.PieceLength), int32(tf.Info.PieceLength))
				if err != nil {
					fmt.Println("Error reading piece from disk", lastPiece, err)
					continue
				}
				
				hash := sha1.Sum(data)
				tobeHash := tf.Info.Pieces[lastPiece*20 : (lastPiece+1)*20]
				if !bytes.Equal(hash[:], tobeHash) {
					fmt.Println("Verification after write failed", lastPiece)
					continue
				}
			}

			fmt.Printf("Piece %d verified and saved\n", dowloadedPiece.Index)
			tf.notifyDownload <- true
			count++
			if tf.Bitfield.IsAllSet(tf.BitfieldLength) {
				fmt.Println("All pieces downloaded, signaling completion")
				tf.DownloadComplete <- true
				return
			}
		} else {
			fmt.Println("Piece verification failed:", dowloadedPiece.Index)
			dowloadedPiece.Clear()
			tf.NeddedPieces <- dowloadedPiece
		}
	}
}

func (tf *TorrentFile) Save() error {
	return tf.Download.Save()
}

func VerifyFileIntegrity(f *os.File, t *TorrentFile, b *bitfield.Bitfield) error {
	for i := int32(0); i < t.BitfieldLength; i++ {
		if !b.Test(i) {
			continue
		}

		// Compute the correct piece length. Last piece may be shorter.
		pieceLen := t.Info.PieceLength
		if i == t.BitfieldLength-1 {
			rem := t.Info.Length % t.Info.PieceLength
			if rem != 0 {
				pieceLen = rem
			}
		}

		// Read exactly pieceLen bytes for this piece.
		buff := make([]byte, pieceLen)
		offset := int64(i) * t.Info.PieceLength
		if _, err := f.Seek(offset, 0); err != nil {
			return fmt.Errorf("failed to seek for piece %d: %w", i, err)
		}
		n, err := f.Read(buff)
		if err != nil {
			return fmt.Errorf("failed to read piece %d: %w", i, err)
		}
		if int64(n) != pieceLen {
			return fmt.Errorf("short read for piece %d: expected %d, got %d", i, pieceLen, n)
		}

		// Expected hash from torrent metadata.
		var expected [20]byte
		copy(expected[:], t.Info.Pieces[i*20:(i+1)*20])

		// Actual hash from file slice (only bytes read).
		actual := sha1.Sum(buff[:n])

		// On mismatch, clear bit; caller can recompute score/valid count.
		if !bytes.Equal(expected[:], actual[:]) {
			b.Clear(i)
			fmt.Printf("Piece %d failed integrity check : %x\n", i, expected)
		}
	}
	return nil
}