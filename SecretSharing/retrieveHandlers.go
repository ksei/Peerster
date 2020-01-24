package SecretSharing

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	core "github.com/ksei/Peerster/Core"
)

func (ssHandler *SSHandler) isDuplicate(passwordUID string) bool {
	ssHandler.ssLocker.RLock()
	defer ssHandler.ssLocker.RUnlock()

	_, exists := ssHandler.requestedPasswordStatus[passwordUID]
	return exists
}

func (ssHandler *SSHandler) initiateShareCollection(passwordUID string) {
	shareRequest := &core.ShareRequest{
		Origin:     ssHandler.ctx.Name,
		Budget:     128,
		RequestUID: passwordUID,
	}

	ssHandler.registerPasswordRequest(passwordUID)
	go ssHandler.expandRing(shareRequest)
}

func (ssHandler *SSHandler) expandRing(shareRequest *core.ShareRequest) {
	budget := uint64(8)
	ssHandler.forwardSearchRequest(ssHandler.ctx.Address.String(), shareRequest, budget)
	for {
		select {
		case passwordUID := <-ssHandler.thresholdReached:
			if strings.Compare(*passwordUID, shareRequest.RequestUID) == 0 {
				fmt.Println("All Shares Retrieved")
				return
			}
		case <-time.After(1 * time.Second):
			budget = 2 * budget
			if budget > 128 {
				fmt.Println("Aborting Search: Maximum budget exhausted...")
				return
			}
			go ssHandler.forwardSearchRequest(ssHandler.ctx.Address.String(), shareRequest, budget)
		}
	}
}

func (ssHandler *SSHandler) forwardSearchRequest(sender string, shareRequest *core.ShareRequest, totalBudget uint64) {
	peerList := ssHandler.ctx.GetPeers()
	totalPeers := len(peerList)
	if int(totalBudget) < totalPeers {
		shareRequest.Budget = 1
		for _, peer := range core.RandomPeers(int(totalBudget), ssHandler.ctx, sender) {
			go ssHandler.ctx.SendPacketToPeer(core.GossipPacket{ShareRequest: shareRequest}, peer)
		}
	} else {
		remainingBudget := totalBudget % uint64(totalPeers)
		rand.Seed(time.Now().UnixNano())
		randomPeerIndices := rand.Perm(totalPeers)[:remainingBudget]
		for i, peer := range peerList {
			shareRequest.Budget = totalBudget / uint64(totalPeers)
			for _, randomPeer := range randomPeerIndices {
				if i == randomPeer {
					shareRequest.Budget++
					break
				}
			}
			go ssHandler.ctx.SendPacketToPeer(core.GossipPacket{ShareRequest: shareRequest}, peer)
		}
	}
}

//HandleSearchRequest sent from peers
func (ssHandler *SSHandler) HandleSearchRequest(packet core.GossipPacket, sender string) {
	shareRequest := packet.ShareRequest
	// if ssHandler.isDuplicate(shareRequest.RequestUID) {
	// 	return
	// }
	// go fH.cacheRequest(*searchRequest)
	publicShare, found := ssHandler.searchHostedShares(*shareRequest)
	if found {
		gossipPacket := &core.GossipPacket{
			PublicSecretShare: publicShare,
		}
		err := ssHandler.ctx.SendPacketToPeerViaRouting(*gossipPacket, publicShare.Destination)
		if err != nil {
			fmt.Println(err)
		}
	}
	go ssHandler.forwardSearchRequest(sender, shareRequest, shareRequest.Budget-1)
}

func (ssHandler *SSHandler) searchHostedShares(request core.ShareRequest) (*core.PublicShare, bool) {
	shareUID, err := GetShareUID(request.RequestUID, ssHandler.ctx.Name)
	if err != nil {
		return nil, false
	}

	secretShare, found := ssHandler.hostedShares[shareUID]
	if found {
		return ssHandler.NewPublic(shareUID, request.Origin, secretShare), true
	}

	return nil, false
}
