package main

import (
	"fmt"
	"go-torrent-client/internals/message"
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

	peerConnection, err := peers[0].Connect(tf)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Connected to peer")
	defer peerConnection.Close()
	errChan := make(chan error)
	go tf.UpdateNeddedPieces()
	go tf.UpdateDownloadedPieces()
	go func() {
		err := peerConnection.ReadLoop()
		errChan <- err
		fmt.Println("Read loop done : ", err)
	}()
	go func() {
		err := peerConnection.WriteLoop()
		errChan <- err
		fmt.Println("Write loop done : ", err)
	}()

	peerConnection.Outgoing <- message.Bitfield(peerConnection.Tf.Bitfield)
	peerConnection.Outgoing <- message.Interested()
	<-errChan

}
