package main


import (
	"fmt"
	peer "go-torrent-client/internals/peer"
	torrentfile "go-torrent-client/internals/torrent_file"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	tf, err := torrentfile.ParseTorrentFile("./tor.torrent")
	if err != nil {
		fmt.Println(err)
		return
	}
	err = tf.Initialize()
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
	type peerStatus struct {
		status bool
		err    error
	}
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	peerTracker := make([]peerStatus, len(peers))
	for {
		for i, peer := range peers {
			if peerTracker[i].status {
				continue
			}
			peerTracker[i] = peerStatus{status: true, err: nil}
			go func() {
				err := peer.SpawnPeer(tf)
				if err != nil {
					peerTracker[i] = peerStatus{status: false, err: err}
				}
			}()
		}
		select {
		case <-tf.DownloadComplete:
			tf.Download.Save()
			fmt.Println("Download complete")
			return
		case <-sigs:
			fmt.Println("Download interrupted")
			tf.Download.Save()
			fmt.Println(tf.Bitfield)
			return
		case <-time.After(5 * time.Minute):
			fmt.Println("Download timed out")

		}
	}

}
