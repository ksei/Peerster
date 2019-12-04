package core

import (
	"sync"
)

//VectorClock structure
type VectorClock struct {
	Locker sync.RWMutex
	Stack  map[string]map[uint32]interface{}
}

//NewVectorClock constructor for vector clock
func NewVectorClock() *VectorClock {
	vClock := &VectorClock{
		Stack: make(map[string]map[uint32]interface{}),
	}
	return vClock
}

//GetStoredMessage retrieves a stored message from the Message Stack
func (vClock *VectorClock) GetStoredMessage(origin string, ID uint32) (interface{}, bool) {
	vClock.Locker.RLock()
	defer vClock.Locker.RUnlock()
	v, ok := vClock.Stack[origin][ID]
	return v, ok
}

//StoreMessage stores a new message to the message stack
func (vClock *VectorClock) StoreMessage(value Stackable) {
	nextID := vClock.GetMaxIdFrom(value.GetOrigin())
	vClock.Locker.Lock()
	defer vClock.Locker.Unlock()
	if nextID == 0 {
		toAdd := map[uint32]interface{}{value.GetID(): value.GetValue()}
		vClock.Stack[value.GetOrigin()] = toAdd
	} else {
		vClock.Stack[value.GetOrigin()][value.GetID()] = value.GetValue()
	}
}

//IsInSyncWith : Check if vector clock is synchronized with another
func (vClock *VectorClock) IsInSyncWith(vClockToCompare StatusPacket) bool {
	for _, peerStatus := range vClockToCompare.Want {
		nextID := vClock.GetNextIDFrom(peerStatus.Identifier)
		if nextID != peerStatus.NextID {
			return false
		}
	}
	return true
}

//CompareV2 - Given a peer-vector-clock, return a list of packets needed and a list of packets to send to peer
func (vClock *VectorClock) CompareV2(vClockToCompare StatusPacket) ([]PeerStatus, []PeerStatus) {
	var toSend []PeerStatus
	var need []PeerStatus
	for _, peerStatus := range vClockToCompare.Want {
		nextID := vClock.GetNextIDFrom(peerStatus.Identifier)
		if nextID == 1 {
			need = append(need, PeerStatus{Identifier: peerStatus.Identifier, NextID: 1})
		} else if nextID < peerStatus.NextID {
			need = append(need, PeerStatus{Identifier: peerStatus.Identifier, NextID: nextID})
		} else if nextID > peerStatus.NextID {
			toSend = append(toSend, PeerStatus{Identifier: peerStatus.Identifier, NextID: peerStatus.NextID})
		}
	}
	return toSend, need
}

//GetMaxIdFrom returns the latest id from a given origin
func (vClock *VectorClock) GetMaxIdFrom(origin string) uint32 {
	vClock.Locker.RLock()
	defer vClock.Locker.RUnlock()
	allMessagesFromPeer, ok := vClock.Stack[origin]
	if !ok {
		return 0
	}
	var maxID uint32
	for maxID = range allMessagesFromPeer {
		break
	}
	for n := range allMessagesFromPeer {
		if n > maxID {
			maxID = n
		}
	}
	return maxID
}

//GetNextIDFrom - If user exists get lnextatest message ID from vector clock, otherwise return 0
func (vClock *VectorClock) GetNextIDFrom(origin string) uint32 {
	vClock.Locker.RLock()
	defer vClock.Locker.RUnlock()
	allMessagesFromPeer, ok := vClock.Stack[origin]
	if !ok {
		return 1
	}

	return getNextID(allMessagesFromPeer)
}

func getNextID(messages map[uint32]interface{}) uint32 {
	var i uint32
	i = 1
	ok := true
	for {
		if _, ok = messages[i]; !ok {
			return i
		}
		i++
	}
}

//GetCurrentStatus - Produce a want[] packet based on current vector clock status
func (vClock *VectorClock) GetCurrentStatus() []PeerStatus {
	var packet []PeerStatus

	vClock.Locker.RLock()
	defer vClock.Locker.RUnlock()
	for k, v := range vClock.Stack {
		toAdd := PeerStatus{Identifier: k, NextID: getNextID(v)}
		packet = append(packet, toAdd)
	}
	return packet
}
