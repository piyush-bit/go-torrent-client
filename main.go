package main

import (
	"fmt"
	torrentfile "go-torrent-client/internals/torrent_file"
)

func main() {
	tf, err := torrentfile.ParseTorrentFile("./tor.torrent")
	if err != nil {
		fmt.Println(err)
		return
	}
	tf.Info.Pieces = nil
	fmt.Printf("%+v\n", tf)
}
