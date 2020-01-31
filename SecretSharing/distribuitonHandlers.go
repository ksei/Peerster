/*
Created and Developed by: Ksandros Apostoli
Part of the course project for Decentralized System Engineering
*/
package SecretSharing

import (
	"fmt"
	"strings"
	"time"

	core "github.com/ksei/Peerster/Core"
)

func (ssHandler *SSHandler) distributePublicShares(publicShares []*core.PublicShare) error {
	go ssHandler.waitForConfirmaitons()
	for _, pubShare := range publicShares {
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

func (ssHandler *SSHandler) waitForConfirmaitons() {
	timeout := 5
	for {
		select {
		case peer := <-ssHandler.confirmationReceived:
			if ssHandler.updateDistributionStatus(*peer) {
				fmt.Println("All Confirmations Received")
				ssHandler.distributionSuccessful <- true
				return
			}
		case <-time.After(1 * time.Second):
			timeout = timeout - 1
			if timeout <= 0 {
				ssHandler.distributionSuccessful <- false
				return
			}
		}
	}
}

func (ssHandler *SSHandler) sendConfirmation(publicShare core.PublicShare) error {
	confirmation := &core.PublicShare{
		Origin:       ssHandler.ctx.Name,
		Destination:  publicShare.Origin,
		HopLimit:     64,
		UID:          publicShare.UID,
		Confirmation: true,
		Requested:    false,
	}
	gossipPacket := &core.GossipPacket{
		PublicSecretShare: confirmation,
	}
	err := ssHandler.ctx.SendPacketToPeerViaRouting(*gossipPacket, publicShare.Origin)
	if err != nil {
		return err
	}
	return nil
}

func (ssHandler *SSHandler) verifyConfirmation(confirmation core.PublicShare) {
	ssHandler.ssLocker.RLock()
	defer ssHandler.ssLocker.RUnlock()
	registeredShareUID, exists := ssHandler.confirmationMap[confirmation.Origin]
	if exists && strings.Compare(registeredShareUID, confirmation.UID) == 0 {
		ssHandler.confirmationReceived <- &confirmation.Origin
	}
}

func (ssHandler *SSHandler) updateDistributionStatus(confirmingPeer string) bool {
	ssHandler.ssLocker.Lock()
	defer ssHandler.ssLocker.Unlock()
	if _, exists := ssHandler.confirmationMap[confirmingPeer]; exists {
		delete(ssHandler.confirmationMap, confirmingPeer)
	}
	return len(ssHandler.confirmationMap) == 0
}

func (ssHandler *SSHandler) updateConfirmationMap(peer, shareUID string) {
	ssHandler.ssLocker.Lock()
	defer ssHandler.ssLocker.Unlock()
	ssHandler.confirmationMap[peer] = shareUID
}
