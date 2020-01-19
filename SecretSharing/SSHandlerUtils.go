package SecretSharing

import (
	"errors"
	"fmt"
	"math"
	"strings"

	core "github.com/ksei/Peerster/Core"
	"golang.org/x/crypto/bcrypt"
)

//NewPublic instantiates a new SecretShare
func (ssHandler *SSHandler) NewPublic(shareUid, dest string, replicateID uint32, secShareToAdd []byte) *core.PublicShare {
	share := &core.PublicShare{
		Origin:       ssHandler.ctx.Name,
		Destination:  dest,
		ReplicateID:  replicateID,
		HopLimit:     ssHandler.ctx.GetHopLimit(),
		UID:          shareUid,
		SecuredShare: secShareToAdd,
	}
	return share
}

func (ssHandler *SSHandler) passwordExists(masterKey, account, username string) bool {
	ssHandler.ssLocker.RLock()
	defer ssHandler.ssLocker.RUnlock()
	found := false
	for _, storedPassword := range ssHandler.storedPasswords {
		if bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(masterKey+account+username)) != nil {
			found = true
		}
	}
	return found
}

func (ssHandler *SSHandler) getSplittingParams() (int, int, error) {
	totalPeers := len(ssHandler.ctx.GetPeerOrigins())
	if totalPeers < 6 {
		return 0, 0, errors.New("Not enough peers, please try again at a later time")
	}
	peerReplicates := int(math.Sqrt(float64(totalPeers) / 4))
	totalShares := totalPeers / peerReplicates
	threshold := totalShares / 3

	return totalShares, threshold, nil
}

func (ssHandler *SSHandler) mapSharesToPeers(totalShares int) (map[string]uint32, error) {
	peerOriginList := ssHandler.ctx.GetPeerOrigins()
	totalPeers := len(peerOriginList)

	if totalPeers < MIN_SHARES {
		return nil, errors.New("Not enough peers")
	}

	replicateIDMap := make(map[string]uint32)
	for i, origin := range peerOriginList {
		if _, exists := replicateIDMap[origin]; exists {
			return nil, errors.New("Share exists")
		}
		replicateIDMap[origin] = uint32(i % totalShares)
	}

	return replicateIDMap, nil
}

func (ssHandler *SSHandler) distributePublicShares(publicSHares []*core.PublicShare) error {
	for _, pubShare := range publicSHares {
		gossipPacket := &core.GossipPacket{
			PublicSecretShare: pubShare,
		}
		err := ssHandler.ctx.SendPacketToPeerViaRouting(*gossipPacket, pubShare.Destination)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ssHandler *SSHandler) awaitingShare(publicShare core.PublicShare) (string, bool) {
	ssHandler.ssLocker.RLock()
	defer ssHandler.ssLocker.RUnlock()
	sender := publicShare.Origin
	shareUID := publicShare.UID
	for passwordUID := range ssHandler.requestedPasswordStatus {
		UIDtoCompare, err := GetShareUID(passwordUID, sender)
		if err != nil {
			fmt.Println(err)
			return "", false
		}
		if strings.Compare(UIDtoCompare, shareUID) == 0 {
			return passwordUID, true
		}
	}
	return "", false
}

// func (ssHandler *SSHandler) updateAwaitingStatus(passwordUID string, publicShare core.PublicShare) {
// 	ssHandler.ssLocker.Lock()
// 	defer ssHandler.ssLocker.Unlock()

// 	shareUID := publicShare.UID
// 	secretShare := publicShare.SecuredShare
// 	index := ssHandler.replicateIndex[shareUID]

// 	if _, exists := ssHandler.receivedShares[passwordUID]; !exists {
// 		ssHandler.receivedShares[passwordUID] = make(map[string][]byte)
// 	}
// 	//Check size of distinct shares received for that passwordUID, and if above the threshhold begin reconstruction.
// }
