package SecretSharing

import (
	"encoding/hex"
	"errors"
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
				sharemap, retrievingThreshold := ssHandler.getReconstructionParams(*passwordUID)
				shareslice := []*Share{}
				for _, v := range sharemap {
					shareslice = append(shareslice, v)
				}
				//Reconstruct secret
				secret, err := RecoverSecret(shareslice, retrievingThreshold)
				//Clean shares from map and remove tempKey if no more searches going on
				if err != nil {
					ssHandler.concludeRetrieval(*passwordUID)
					ssHandler.communicateError(err)
				}
				//decrypting secret
				clearPasswordBytes, err := ssHandler.decryptPassword(*passwordUID, secret)
				ssHandler.concludeRetrieval(*passwordUID)
				if err != nil {
					ssHandler.communicateError(err)
				}
				clearPassword := string(clearPasswordBytes)
				ssHandler.ctx.GUImessageChannel <- &core.GUIPacket{Password: &clearPassword}
				return
			}
		case <-time.After(1 * time.Second):
			budget = 2 * budget
			if budget > 256 {
				ssHandler.communicateError(errors.New("Aborting Search: Maximum budget exhausted"))
				return
			}
			go ssHandler.forwardSearchRequest(ssHandler.ctx.Address.String(), shareRequest, budget)
		}
	}
}

func (ssHandler *SSHandler) forwardSearchRequest(sender string, shareRequest *core.ShareRequest, totalBudget uint64) {
	peerList := ssHandler.ctx.GetPeers()
	if strings.Compare(sender, ssHandler.ctx.Address.String()) != 0 {
		for i, peer := range peerList {
			if strings.Compare(sender, peer) == 0 {
				peerList = append(peerList[:i], peerList[i+1:]...)
				break
			}
		}
	}
	totalPeers := len(peerList)
	if totalPeers < 1 {
		return
	} else if int(totalBudget) < totalPeers {
		shareRequest.Budget = 1
		for _, peer := range core.RandomPeers(int(totalBudget), peerList) {
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
		publicShare.Requested = true
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
	shareUID := GetShareUID(request.RequestUID, ssHandler.ctx.Name)
	fmt.Println("Searching for :", hex.EncodeToString([]byte(shareUID)))
	secretShare, found := ssHandler.hostedShares[shareUID]
	if found {
		return ssHandler.NewPublic(shareUID, request.Origin, secretShare), true
	}

	return nil, false
}
