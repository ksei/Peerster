package SecretSharing

import (
	"errors"

	core "github.com/ksei/Peerster/Core"
	"golang.org/x/crypto/bcrypt"
)

//NewPublic instantiates a new SecretShare
func (ssHandler *SSHandler) NewPublic(shareUid, dest string, secShareToAdd []byte) *core.PublicShare {
	share := &core.PublicShare{
		Origin:       ssHandler.ctx.Name,
		Destination:  dest,
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

func (ssHandler *SSHandler) storeSalt(passwordUID string, salt []byte) error {
	ssHandler.ssLocker.Lock()
	defer ssHandler.ssLocker.Unlock()

	if _, exists := ssHandler.salts[passwordUID]; exists {
		return errors.New("Password exists")
	}

	ssHandler.salts[passwordUID] = salt
	return nil
}

func (ssHandler *SSHandler) storeNonce(passwordUID string, nonce []byte) error {
	ssHandler.ssLocker.Lock()
	defer ssHandler.ssLocker.Unlock()

	var storedNonces [][]byte
	var exists bool
	if storedNonces, exists = ssHandler.nonces[passwordUID]; exists {
		storedNonces = append(storedNonces, nonce)
	} else {
		storedNonces = [][]byte{nonce}
	}

	ssHandler.nonces[passwordUID] = storedNonces
	return nil
}

func (ssHandler *SSHandler) mapSharesToPeers(shares [][]byte) (map[string][]byte, error) {
	peerOriginList := ssHandler.ctx.GetPeerOrigins()
	totalShares := len(shares)
	totalPeers := len(peerOriginList)

	if totalPeers < MIN_SHARES {
		return nil, errors.New("Not enough peers")
	}

	shareOf := make(map[string][]byte)
	for i, origin := range peerOriginList {
		if _, exists := shareOf[origin]; exists {
			return nil, errors.New("Share exists")
		}
		shareOf[origin] = shares[i%totalShares]
	}

	return shareOf, nil
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
