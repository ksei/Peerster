package TLC

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	core "github.com/ksei/Peerster/Core"
	mongering "github.com/ksei/Peerster/Mongering"
)

type TLCHandler struct {
	ctx                   *core.Context
	mongerer              *mongering.Mongerer
	tlcLocker             sync.RWMutex
	confirmations         map[uint32][]string
	TotalPeers            int
	stubbornTimeout       int
	myTime                uint32
	messageBuffer         map[string][]*core.TLCMessage
	clientBuffer          []*core.TLCMessage
	peerConfirmations     map[string][]uint32
	awaitingConfirmations map[uint32]bool
	peerRounds            map[string]uint32
	readyForNextRound     bool
}

func NewTLCHandler(mng *mongering.Mongerer, totalPeers, stubborn int) *TLCHandler {
	tlc := &TLCHandler{
		ctx:                   mng.GetContext(),
		mongerer:              mng,
		confirmations:         make(map[uint32][]string),
		TotalPeers:            totalPeers,
		stubbornTimeout:       stubborn,
		myTime:                0,
		messageBuffer:         make(map[string][]*core.TLCMessage),
		clientBuffer:          []*core.TLCMessage{},
		peerConfirmations:     make(map[string][]uint32),
		awaitingConfirmations: make(map[uint32]bool),
		peerRounds:            make(map[string]uint32),
		readyForNextRound:     true,
	}
	return tlc
}

func (tlc *TLCHandler) HandleTLCMessage(packet core.GossipPacket, sender string) {
	tlcMessage := packet.TLCMessage
	if !tlc.messageExists(*tlcMessage) {
		if tlc.ctx.VectorClock.GetMaxIdFrom(tlcMessage.Origin) < tlcMessage.ID {
			tlc.ctx.UpdateDSDV(tlcMessage.Origin, sender, false)
		}
		if strings.Compare(sender, tlc.ctx.Address.String()) != 0 {
			if tlc.ctx.RunningHw3Ex3() && !tlc.satisfiesVectorClock(*tlcMessage) {
				go tlc.bufferMessage(*tlcMessage)
			} else {
				tlc.acceptTLCMessage(*tlcMessage)
				go tlc.updateBufferStatus(tlcMessage.Origin)
			}
			go tlc.mongerer.StartMongering(tlcMessage, core.RandomPeer(tlc.ctx, sender))
		} else if tlc.ctx.RunningHw3Ex3() && !tlc.readyForNextRound {
			tlc.tlcLocker.Lock()
			tlc.clientBuffer = append(tlc.clientBuffer, tlcMessage)
			tlc.tlcLocker.Unlock()
		} else {
			go tlc.advanceToNextRound(*tlcMessage)
		}
	} else if strings.Compare(sender, tlc.ctx.Address.String()) != 0 && tlcMessage.Confirmed == -1 {
		go tlc.mongerer.Acknowledge(sender)
	}
}

func (tlc *TLCHandler) HandleTLCAck(packet core.GossipPacket) {
	tlcAck := packet.Ack
	found, destinationIP := tlc.ctx.RetrieveDestinationRoute(tlcAck.Destination)
	switch found {
	case -1:
		return
	case 0:
		tlc.tlcLocker.Lock()
		defer tlc.tlcLocker.Unlock()

		if awaiting, ok := tlc.awaitingConfirmations[tlcAck.ID]; !ok || !awaiting {
			return
		}
		if _, ok := tlc.confirmations[tlcAck.ID]; !ok {
			tlc.confirmations[tlcAck.ID] = []string{}
		}
		tlc.confirmations[tlcAck.ID] = append(tlc.confirmations[tlcAck.ID], tlcAck.Origin)
		if len(tlc.confirmations[tlcAck.ID]) > tlc.TotalPeers/2 {
			fmt.Println("RE-BROADCAST ID", tlcAck.ID, "WITNESSES", strings.Join(tlc.confirmations[tlcAck.ID], ","))
			delete(tlc.awaitingConfirmations, tlcAck.ID)
			go tlc.publishConfirmed(tlcAck.ID)
		}
	default:
		if tlcAck.HopLimit == 0 {
			return
		}
		tlcAck.HopLimit--
		go tlc.ctx.SendPacketToPeer(core.GossipPacket{Ack: tlcAck}, destinationIP)
	}
}

func (tlc *TLCHandler) publishConfirmed(id uint32) {
	content, ok := tlc.ctx.VectorClock.GetStoredMessage(tlc.ctx.Name, id)
	tlcMessage := content.(*core.TLCMessage)
	if ok {
		tlcMessage.Origin = tlc.ctx.Name
		tlcMessage.Confirmed = int(id)
		tlcMessage.ID = tlc.ctx.VectorClock.GetNextIDFrom(tlc.ctx.Name)
		go tlc.mongerer.StartMongering(tlcMessage, core.RandomPeer(tlc.ctx, tlc.ctx.Name))
	}
}

func (tlc *TLCHandler) messageExists(tlcPacket core.TLCMessage) bool {
	_, ok := tlc.ctx.VectorClock.GetStoredMessage(tlcPacket.Origin, tlcPacket.ID)
	return ok
}

func (tlc *TLCHandler) AcknowledgeTLC(tlcMessage core.TLCMessage) {
	ack := &core.TLCAck{
		Origin:      tlc.ctx.Name,
		ID:          tlcMessage.ID,
		Text:        "",
		Destination: tlcMessage.Origin,
		HopLimit:    tlc.ctx.GetHopLimit(),
	}
	packet := core.GossipPacket{Ack: ack}
	fmt.Println("SENDING ACK origin", tlcMessage.Origin, "ID", tlcMessage.ID)
	go tlc.ctx.SendPacketToPeerViaRouting(packet, tlcMessage.Origin)
}

func printStatus(sender string, status []core.PeerStatus) {
	fmt.Print("STATUS from ", sender, " ")
	for _, peerStatus := range status {
		fmt.Print("peer ", peerStatus.Identifier, " nextID ", peerStatus.NextID, " ")
	}
	fmt.Print("\n")
}

//NewTLCFromTxPublish creates a new TLC message from file info
func (tlc *TLCHandler) NewTLCFromTxPublish(name string, size int64, metahash []byte) *core.TLCMessage {
	txPublish := core.TxPublish{
		Name:         name,
		Size:         size,
		MetafileHash: metahash,
	}
	blockPublish := core.BlockPublish{
		Transaction: txPublish,
	}
	return &core.TLCMessage{
		Origin:      tlc.ctx.Name,
		TxBlock:     blockPublish,
		Confirmed:   -1,
		ID:          tlc.ctx.VectorClock.GetNextIDFrom(tlc.ctx.Name),
		VectorClock: &core.StatusPacket{Want: tlc.ctx.VectorClock.GetCurrentStatus()},
	}
}

func (tlc *TLCHandler) isConifrmed(id uint32) bool {
	tlc.tlcLocker.RLock()
	defer tlc.tlcLocker.RUnlock()
	confirmations, ok := tlc.confirmations[id]
	if ok {
		return len(confirmations) > tlc.TotalPeers/2
	}
	return false
}

func (tlc *TLCHandler) stubbornRetries(tlcMessage core.TLCMessage) {
	for {
		fmt.Println("Sending stubborn")
		go tlc.mongerer.StartMongering(&tlcMessage, core.RandomPeer(tlc.ctx, tlc.ctx.Name))
		time.Sleep(time.Duration(tlc.stubbornTimeout) * time.Second)
		if tlc.isConifrmed(tlcMessage.ID) {
			return
		}
	}
}

func (tlc *TLCHandler) acceptTLCMessage(tlcMessage core.TLCMessage) {
	tlc.ctx.VectorClock.StoreMessage(&tlcMessage)
	switch tlcMessage.Confirmed {
	case -1:
		fmt.Println("UNCONFIRMED GOSSIP origin", tlcMessage.Origin, "ID", tlcMessage.ID, "file name", tlcMessage.TxBlock.Transaction.Name, "size", tlcMessage.TxBlock.Transaction.Size, "metahash", hex.EncodeToString(tlcMessage.TxBlock.Transaction.MetafileHash))
		tlc.incrementPeerRound(tlcMessage.Origin)
		if !tlc.ctx.RunningHw3Ex3() || tlc.getPeerRound(tlcMessage.Origin) >= tlc.myTime {
			go tlc.AcknowledgeTLC(tlcMessage)
		}
	default:
		fmt.Println("CONFIRMED GOSSIP origin", tlcMessage.Origin, "ID", tlcMessage.Confirmed, "file name", tlcMessage.TxBlock.Transaction.Name, "size", tlcMessage.TxBlock.Transaction.Size, "metahash", hex.EncodeToString(tlcMessage.TxBlock.Transaction.MetafileHash))
		tlc.storeConfirmation(tlcMessage)
	}
}

func (tlc *TLCHandler) updateBufferStatus(peer string) {
	tlc.tlcLocker.RLock()
	defer tlc.tlcLocker.RUnlock()
	nextBuffered, exists := tlc.messageBuffer[peer]
	if exists && len(nextBuffered) > 0 && tlc.satisfiesVectorClock(*tlc.messageBuffer[peer][0]) {
		go tlc.acceptTLCMessage(*tlc.messageBuffer[peer][0])
		tlc.messageBuffer[peer] = tlc.messageBuffer[peer][1:]
	}
}

func (tlc *TLCHandler) satisfiesVectorClock(tlcMessage core.TLCMessage) bool {
	tlc.tlcLocker.Lock()
	defer tlc.tlcLocker.Unlock()
	for _, peerStatus := range tlcMessage.VectorClock.Want {
		if tlc.ctx.VectorClock.GetNextIDFrom(peerStatus.Identifier) < peerStatus.NextID {
			return false
		}
	}
	return true
}

func (tlc *TLCHandler) bufferMessage(tlcMessage core.TLCMessage) {
	tlc.tlcLocker.Lock()
	defer tlc.tlcLocker.Unlock()
	if _, ok := tlc.messageBuffer[tlcMessage.Origin]; !ok {
		tlc.messageBuffer[tlcMessage.Origin] = []*core.TLCMessage{&tlcMessage}
	} else {
		tlc.messageBuffer[tlcMessage.Origin] = append(tlc.messageBuffer[tlcMessage.Origin], &tlcMessage)
	}
}

func (tlc *TLCHandler) storeConfirmation(tlcMessage core.TLCMessage) {
	tlc.tlcLocker.Lock()
	defer tlc.tlcLocker.Unlock()
	if _, ok := tlc.peerConfirmations[tlcMessage.Origin]; !ok {
		tlc.peerConfirmations[tlcMessage.Origin] = []uint32{uint32(tlcMessage.Confirmed)}
	} else {
		tlc.peerConfirmations[tlcMessage.Origin] = append(tlc.peerConfirmations[tlcMessage.Origin], uint32(tlcMessage.Confirmed))
	}

	totalConfirmations := 0
	for _, peerConfirmation := range tlc.peerConfirmations {
		if len(peerConfirmation) >= int(tlc.myTime) {
			totalConfirmations++
		}
	}
	if totalConfirmations > tlc.TotalPeers/2 {
		tlc.readyForNextRound = true
		if len(tlc.clientBuffer) > 0 {
			go tlc.advanceToNextRound(*tlc.clientBuffer[0])
			tlc.clientBuffer = tlc.clientBuffer[1:]
		}
	}
}

func (tlc *TLCHandler) advanceToNextRound(tlcMessage core.TLCMessage) {
	tlcMessage.ID = tlc.ctx.VectorClock.GetNextIDFrom(tlc.ctx.Name)
	tlc.ctx.VectorClock.StoreMessage(&tlcMessage)
	tlc.tlcLocker.Lock()
	tlc.confirmations[tlcMessage.ID] = []string{tlc.ctx.Name}
	if tlc.ctx.RunningHw3Ex3() {
		tlc.printAdvancement()
		tlc.myTime++
		tlc.readyForNextRound = false
	}
	tlc.awaitingConfirmations[tlcMessage.ID] = true
	tlc.tlcLocker.Unlock()
	go tlc.stubbornRetries(tlcMessage)
}

func (tlc *TLCHandler) getPeerRound(peer string) uint32 {
	tlc.tlcLocker.RLock()
	defer tlc.tlcLocker.RUnlock()
	round, exists := tlc.peerRounds[peer]
	if exists {
		return round
	}
	return 0
}

func (tlc *TLCHandler) incrementPeerRound(peer string) {
	tlc.tlcLocker.RLock()
	defer tlc.tlcLocker.RUnlock()

	if _, exists := tlc.peerRounds[peer]; !exists {
		tlc.peerRounds[peer] = 1
		return
	}
	tlc.peerRounds[peer]++
}

func (tlc *TLCHandler) printAdvancement() {
	if tlc.myTime == 0 {
		return
	}
	confirmationStr := ""
	peerC := tlc.peerConfirmations
	i := 1
	for peer, confirmations := range peerC {
		if len(confirmations) >= int(tlc.myTime) {
			if i != 1 {
				confirmationStr = confirmationStr + ","
			}
			confirmationStr = confirmationStr + "origin" + strconv.Itoa(i) + " " + peer + " ID" + strconv.Itoa(i) + " " + strconv.FormatUint(uint64(confirmations[tlc.myTime-1]), 10) + " "
			i++
		}
	}
	fmt.Println("ADVANCING TO round", tlc.myTime+1, "BASED ON CONFIRMED MESSAGES", confirmationStr)
}
