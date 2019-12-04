package mongering

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	core "github.com/ksei/Peerster/Core"
)

type Mongerer struct {
	ctx                         *core.Context
	acknowledgementChannel      chan *core.InternalPacket
	ackLocker                   sync.RWMutex
	awaitingAcknowledgementFrom map[string]bool
}

func NewMongerer(cntx *core.Context, antiEntropy int) *Mongerer {
	mongerer := &Mongerer{
		acknowledgementChannel:      make(chan *core.InternalPacket, 50),
		awaitingAcknowledgementFrom: make(map[string]bool),
	}
	mongerer.ctx = cntx
	go mongerer.StartAntiEntropy(antiEntropy)
	return mongerer
}

func (mongerer *Mongerer) StartMongering(content core.Stackable, peer string) {
	// fmt.Println("MONGERING with", peer)
	gossipPacket := core.CreateGossipPacket(content)
	mongerer.ackLocker.Lock()
	mongerer.awaitingAcknowledgementFrom[peer] = true
	mongerer.ackLocker.Unlock()
	go mongerer.ctx.SendPacketToPeer(gossipPacket, peer)
	for {
		select {
		case acknowledged := <-mongerer.acknowledgementChannel:
			if strings.Compare(peer, acknowledged.Sender) != 0 {
				continue
			}
			mongerer.ackLocker.Lock()
			mongerer.awaitingAcknowledgementFrom[peer] = false
			mongerer.ackLocker.Unlock()
			// printStatus(acknowledged.Sender, acknowledged.Packet.Status.Want)
			if !mongerer.ctx.VectorClock.IsInSyncWith(*acknowledged.Packet.Status) {
				go mongerer.syncStatuses(*acknowledged.Packet.Status, acknowledged.Sender)
			} else {
				// fmt.Println("IN SYNC WITH", acknowledged.Sender)
				go mongerer.flipCoin(content, peer)
			}
			return
		case <-time.After(10 * time.Second):
			mongerer.ackLocker.Lock()
			mongerer.awaitingAcknowledgementFrom[peer] = false
			mongerer.ackLocker.Unlock()
			go mongerer.flipCoin(content, peer)
			return
		}
	}
}

func (mongerer *Mongerer) HandleStatusPacket(packet core.GossipPacket, sender string) {
	mongerer.ackLocker.RLock()
	waiting, ok := mongerer.awaitingAcknowledgementFrom[sender]
	waiting = ok && waiting
	mongerer.ackLocker.RUnlock()
	if waiting {
		mongerer.acknowledgementChannel <- &core.InternalPacket{Packet: packet, Sender: sender}
	} else if !mongerer.ctx.VectorClock.IsInSyncWith(*packet.Status) {
		go mongerer.syncStatuses(*packet.Status, sender)
	}
}

func (mongerer *Mongerer) Acknowledge(peer string) {
	statusPacket := &core.StatusPacket{Want: mongerer.ctx.VectorClock.GetCurrentStatus()}
	gossipPacket := &core.GossipPacket{Status: statusPacket}
	go mongerer.ctx.SendPacketToPeer(*gossipPacket, peer)
}

func (mongerer *Mongerer) messageExists(rumour core.RumourMessage) bool {
	_, ok := mongerer.ctx.VectorClock.GetStoredMessage(rumour.Origin, rumour.ID)
	return ok
}

func (mongerer *Mongerer) syncStatuses(statusPacket core.StatusPacket, sender string) {
	have, need := mongerer.ctx.VectorClock.CompareV2(statusPacket)

	if len(have) > 0 {
		mongerer.ctx.VectorClock.Locker.RLock()
		var content core.Stackable
		switch mongerer.ctx.VectorClock.Stack[have[0].Identifier][have[0].NextID].(type) {
		case string:
			content = core.NewRumourMessage(have[0].NextID, mongerer.ctx.VectorClock.Stack[have[0].Identifier][have[0].NextID].(string), have[0].Identifier)
		case *core.TLCMessage:
			content = mongerer.ctx.VectorClock.Stack[have[0].Identifier][have[0].NextID].(*core.TLCMessage)
		}
		mongerer.ctx.VectorClock.Locker.RUnlock()
		go mongerer.StartMongering(content, sender)
	} else {
		statusPacket := &core.StatusPacket{Want: need}
		gossipPacket := &core.GossipPacket{Status: statusPacket}
		go mongerer.ctx.SendPacketToPeer(*gossipPacket, sender)
	}
}

func (mongerer *Mongerer) flipCoin(content core.Stackable, previousPeer string) {
	continueMongering := rand.Intn(2) == 1
	if continueMongering {
		randomPeer := core.RandomPeer(mongerer.ctx, previousPeer)
		// fmt.Println("FLIPPED COIN sending rumor to", randomPeer)
		go mongerer.StartMongering(content, randomPeer)
	}
}

func (mongerer *Mongerer) StartAntiEntropy(waitPeriodSeconds int) {
	if mongerer.ctx.SimpleMode || waitPeriodSeconds == 0 || len(mongerer.ctx.Peers) == 0 {
		return
	}
	for {
		time.Sleep(time.Duration(waitPeriodSeconds) * time.Second)
		if len(mongerer.ctx.GetPeers()) == 0 {
			continue
		}
		go mongerer.Acknowledge(core.RandomPeer(mongerer.ctx, ""))
	}
}

func (mongerer *Mongerer) GetContext() *core.Context {
	return mongerer.ctx
}

func printStatus(sender string, status []core.PeerStatus) {
	fmt.Print("STATUS from ", sender, " ")
	for _, peerStatus := range status {
		fmt.Print("peer ", peerStatus.Identifier, " nextID ", peerStatus.NextID, " ")
	}
	fmt.Print("\n")
}
