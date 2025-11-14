package main

import (
	"fmt"
	"go-torrent-client/internals/bitfield"
	torrentfile "go-torrent-client/internals/torrent_file"
	"os"
)

func main() {
	tf, err := torrentfile.ParseTorrentFile("./tor.torrent")
	if err != nil {
		fmt.Println(err)
		return
	}

	b := bitfield.NewBitfield(tf.BitfieldLength)
	for i := int32(0); i < tf.BitfieldLength; i++ {
		b.Set(i)
	}
	f, err := os.Open(tf.Info.Name)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()
	torrentfile.VerifyFileIntegrity(f, tf, &b)

	count := 0
	for i := int32(0); i < tf.BitfieldLength; i++ {
		if b.Test(i) {
			count++
		}
	}
	fmt.Printf("All pieces verified : %d/%d\n", count, tf.BitfieldLength)
}