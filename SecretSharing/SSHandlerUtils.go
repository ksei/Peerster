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
func (ssHandler *SSHandler) NewPublic(shareUID, dest string, secShareToAdd []byte) *core.PublicShare {
	share := &core.PublicShare{
		Origin:       ssHandler.ctx.Name,
		Destination:  dest,
		HopLimit:     ssHandler.ctx.GetHopLimit(),
		UID:          shareUID,
		SecuredShare: secShareToAdd,
		Requested:    false,
		Confirmation: false,
	}
	return share
}

func (ssHandler *SSHandler) passwordExists(masterKey, account, username string) (string, bool) {
	ssHandler.ssLocker.RLock()
	defer ssHandler.ssLocker.RUnlock()
	found := false
	ret := ""
	for _, storedPassword := range ssHandler.storedPasswords {
		if bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(masterKey+account+username)) == nil {
			found = true
			ret = storedPassword
		}
	}
	return ret, found
}

func (ssHandler *SSHandler) getSplittingParams() (int, int, error) {
	totalPeers := len(ssHandler.ctx.GetPeerOrigins())
	if totalPeers < MIN_SHARES {
		return 0, 0, errors.New("Not enough peers, please try again at a later time")
	}
	peerReplicates := int(math.Sqrt(float64(totalPeers) / 4))
	totalShares := totalPeers / peerReplicates
	threshold := 2 * totalShares / 3

	return totalShares, threshold, nil
}

func (ssHandler *SSHandler) mapSharesToPeers(totalShares int) (map[string]uint32, error) {
	peerOriginList := ssHandler.ctx.GetPeerOrigins()

	replicateIDMap := make(map[string]uint32)
	for i, origin := range peerOriginList {
		if _, exists := replicateIDMap[origin]; exists {
			return nil, errors.New("Share exists")
		}
		replicateIDMap[origin] = uint32(i % totalShares)
	}
	ssHandler.ssLocker.Lock()
	defer ssHandler.ssLocker.Unlock()
	return replicateIDMap, nil
}

func (ssHandler *SSHandler) awaitingShare(publicShare core.PublicShare) (string, bool) {
	ssHandler.ssLocker.RLock()
	defer ssHandler.ssLocker.RUnlock()
	sender := publicShare.Origin
	shareUID := publicShare.UID
	for passwordUID := range ssHandler.awaitingPasswords {
		UIDtoCompare := GetShareUID(passwordUID, sender)
		if strings.Compare(UIDtoCompare, shareUID) == 0 {
			return passwordUID, true
		}
	}
	return "", false
}

func (ssHandler *SSHandler) storeThreshold(passwordUID string, thresh int) error {
	ssHandler.ssLocker.Lock()
	defer ssHandler.ssLocker.Unlock()

	if _, exists := ssHandler.thresholds[passwordUID]; exists {
		return errors.New("password threshold already stored")
	}

	ssHandler.thresholds[passwordUID] = thresh
	return nil
}

func (ssHandler *SSHandler) retrieveThreshold(passwordUID string) (int, bool) {
	ssHandler.ssLocker.RLock()
	defer ssHandler.ssLocker.RUnlock()
	thresh, err := ssHandler.thresholds[passwordUID]
	return thresh, err
}

func (ssHandler *SSHandler) thresholdAchievedAndStillWaiting(passwordUID string) bool {
	ssHandler.ssLocker.RLock()
	defer ssHandler.ssLocker.RUnlock()
	_, waiting := ssHandler.awaitingPasswords[passwordUID]
	return len(ssHandler.requestedPasswordStatus[passwordUID]) >= ssHandler.thresholds[passwordUID] && waiting
}

func (ssHandler *SSHandler) stopWaiting(passwordUID string) {
	ssHandler.ssLocker.Lock()
	defer ssHandler.ssLocker.Unlock()
	delete(ssHandler.awaitingPasswords, passwordUID)
	ssHandler.thresholdReached <- &passwordUID
}

func (ssHandler *SSHandler) getReconstructionParams(passwordUID string) (map[uint32]*Share, int) {
	ssHandler.ssLocker.RLock()
	defer ssHandler.ssLocker.RUnlock()
	shareMap := ssHandler.requestedPasswordStatus[passwordUID]
	thresh := ssHandler.thresholds[passwordUID]
	return shareMap, thresh
}

func (ssHandler *SSHandler) storeShare(publicShare core.PublicShare) {
	ssHandler.ssLocker.Lock()
	defer ssHandler.ssLocker.Unlock()

	ssHandler.hostedShares[publicShare.UID] = publicShare.SecuredShare
}

func (ssHandler *SSHandler) storeTemporaryKey(masterKey string) {
	ssHandler.ssLocker.Lock()
	defer ssHandler.ssLocker.Unlock()

	ssHandler.tempKeyStorage = masterKey
}

func (ssHandler *SSHandler) registerPasswordRequest(passwordUID string) {
	ssHandler.ssLocker.Lock()
	defer ssHandler.ssLocker.Unlock()
	ssHandler.awaitingPasswords[passwordUID] = true
	ssHandler.requestedPasswordStatus[passwordUID] = make(map[uint32]*Share)
}

func (ssHandler *SSHandler) hostShare(publicShare core.PublicShare) {
	ssHandler.ssLocker.Lock()
	defer ssHandler.ssLocker.RUnlock()

	if _, exists := ssHandler.hostedShares[publicShare.UID]; exists {
		fmt.Println("Attempted to overwrite existing share")
		return
	}

	ssHandler.hostedShares[publicShare.UID] = publicShare.SecuredShare
}

func (ssHandler *SSHandler) concludeRetrieval(passwordUID string) {
	ssHandler.ssLocker.Lock()
	defer ssHandler.ssLocker.Unlock()

	delete(ssHandler.requestedPasswordStatus, passwordUID)
	if len(ssHandler.requestedPasswordStatus) == 0 {
		ssHandler.tempKeyStorage = ""
	}

}

func (ssHandler *SSHandler) communicateError(err error) {
	res := err.Error()
	ssHandler.ctx.GUImessageChannel <- &core.GUIPacket{PasswordOpResult: &res}
	fmt.Println(err)
}

func (ssHandler *SSHandler) updateRoutingTable() {
	ssHandler.ssLocker.RLock()
	defer ssHandler.ssLocker.RUnlock()
	for inactivePeer := range ssHandler.confirmationMap {
		ssHandler.ctx.RemoveInactiveDestination(inactivePeer)
	}
}

func (ssHandler *SSHandler) clearResidues(passwordUID string) {
	ssHandler.ssLocker.Lock()
	defer ssHandler.ssLocker.Unlock()

	ssHandler.confirmationMap = make(map[string]string)
	delete(ssHandler.thresholds, passwordUID)
	delete(ssHandler.extraInfo, passwordUID)
	for i, password := range ssHandler.storedPasswords {
		if strings.Compare(password, passwordUID) == 0 {
			ssHandler.storedPasswords = append(ssHandler.storedPasswords[:i], ssHandler.storedPasswords[i+1:]...)
			return
		}
	}
}

func (ssHandler *SSHandler) handleError(passwordUID string, err error) {
	ssHandler.communicateError(err)
	go ssHandler.clearResidues(passwordUID)
}

func (ssHandler *SSHandler) archivePassword(passwordUID string) {
	ssHandler.ssLocker.Lock()
	defer ssHandler.ssLocker.Unlock()

	ssHandler.archivedPasswords = append(ssHandler.archivedPasswords, passwordUID)

}
