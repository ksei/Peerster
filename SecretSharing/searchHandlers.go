package SecretSharing

import (
  "math/rand"
  "time"
	core "github.com/ksei/Peerster/Core"
)


//HandleShareSearch handles a new incoming share research; processes it and redistibute it
func (ss *SSHandler) HandleShareSearch(packet core.GossipPacket, sender string) {
	search := packet.ShareSearch

  share,exist:= ss.hostedShares[search.RequestUID]

	if exist {
		searchReply := ss.NewPublic(search.RequestUID, search.Origin, share)
    gossipPacket := &core.GossipPacket{
			PublicSecretShare: searchReply,
		}
		go ss.ctx.SendPacketToPeerViaRouting(*gossipPacket, searchReply.Destination)
	}
	go ss.forwardSearchRequest(sender, search, search.Budget-1)
}



func (ss *SSHandler) forwardSearchRequest(sender string, searchRequest *core.ShareRequest, totalBudget uint64) {
	peerList := ss.ctx.GetPeers()
	totalPeers := len(peerList)
	if int(totalBudget) < totalPeers {
		searchRequest.Budget = 1
		for _, peer := range core.RandomPeers(int(totalBudget), ss.ctx, sender) {
			go ss.ctx.SendPacketToPeer(core.GossipPacket{ShareSearch: searchRequest}, peer)
		}
	} else {
		remainingBudget := totalBudget % uint64(totalPeers)
		rand.Seed(time.Now().UnixNano())
		randomPeerIndices := rand.Perm(totalPeers)[:remainingBudget]
		for i, peer := range peerList {
			searchRequest.Budget = totalBudget / uint64(totalPeers)
			for _, randomPeer := range randomPeerIndices {
				if i == randomPeer {
					searchRequest.Budget++
					break
				}
			}
			go ss.ctx.SendPacketToPeer(core.GossipPacket{ShareSearch: searchRequest}, peer)
		}
	}
}
