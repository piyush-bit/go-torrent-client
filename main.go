package main

import (
	"fmt"
	peer "go-torrent-client/internals/peer"
	torrentfile "go-torrent-client/internals/torrent_file"
)

func main() {
	tf, err := torrentfile.ParseTorrentFile("./tor.torrent")
	if err != nil {
		fmt.Println(err)
		return
	}

	peers, err := peer.RetrivePeers(tf)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("Retrieved peers")
	go tf.UpdateNeddedPieces()
	go tf.UpdateDownloadedPieces()
	for _, peer := range peers {
		go peer.SpawnPeer(tf)
	}

	select {}

}
