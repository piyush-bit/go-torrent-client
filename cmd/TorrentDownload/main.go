package main

import (
	"fmt"
	peer "go-torrent-client/internals/peer"
	torrentfile "go-torrent-client/internals/torrent_file"
	"os"
	"os/signal"
	"runtime/pprof"
	"syscall"
	"time"
)

func main() {
	f, _ := os.Create("cpu.prof")
	pprof.StartCPUProfile(f)
	defer pprof.StopCPUProfile()
	tf, err := torrentfile.ParseTorrentFile("./tor2.torrent")
	if err != nil {
		fmt.Println("Error parsing torrent file :", err)
		return
	}
	if tf.Bitfield.IsAllSet(tf.BitfieldLength) {
		fmt.Println("Already Completed")
		return
	}
	peers, _, err := peer.RetrivePeers(tf)
	if err != nil {
		fmt.Println("Error retriving peers :", err)
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
