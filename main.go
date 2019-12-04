package main

import (
	// core "github.com/ksei/Peerster/Core"
	"flag"
	"strings"

	gsp "github.com/ksei/Peerster/gossiper"
	webS "github.com/ksei/Peerster/webServer"
	// fh "github.com/ksei/Peerster/fileSharing"
)

func main() {
	UIPort := flag.String("UIPort", "8080", "Port for the client. Default: 8080.")
	gossipAddress := flag.String("gossipAddr", "127.0.0.1:5000", "IP:Port for the gossiper. Default: 127.0.0.1:5000")
	gossipName := flag.String("name", "Lenovo", "Name of the gossiper")
	peerList := flag.String("peers", "127.0.0.1:5001", "Comma separated list of known peers to the gossiper")
	antiEntr := flag.Int("antiEntropy", 10, "Frequency for performing AntiEntropy")
	simpleMsg := flag.Bool("simple", false, "Run Gossiper in simple broadcast mode")
	rtimer := flag.Int("rtimer", 0, "Frequency for pulsing route messages")
	totalPeers := flag.Int("N", 2, "Total number of peers in network")
	stubbornTimeout := flag.Int("stubbornTimeout", 5, "timeout before resending stubborn messages")
	hopLimit := flag.Int("hopLimit", 10, "Maximum number of hops specified for private messaging")
	hw3ex2 := flag.Bool("hw3ex2", false, "Support hw3ex2 functionality")
	hw3ex3 := flag.Bool("hw3ex3", false, "Support hw3ex3 functionality")

	flag.Parse()

	_, ctx := gsp.NewGossiper(*gossipAddress, *gossipName, *UIPort, *simpleMsg, *hw3ex2, *hw3ex3, *antiEntr, *rtimer, *totalPeers, *stubbornTimeout, *hopLimit)
	peers := strings.Split(*peerList, ",")
	for i := 0; i < len(peers); i++ {
		ctx.AddPeer(peers[i])
	}

	webServer := webS.NewServer(ctx, UIPort)
	webServer.Launch(*gossipAddress)
}
