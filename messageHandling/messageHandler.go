package messageHandling

import (
	"fmt"
	"strings"

	core "github.com/ksei/Peerster/Core"
	mongering "github.com/ksei/Peerster/Mongering"
)

type MessageHandler struct {
	ctx      *core.Context
	mongerer *mongering.Mongerer
}

func NewMessageHandler(mng *mongering.Mongerer) *MessageHandler {
	mh := &MessageHandler{
		ctx:      mng.GetContext(),
		mongerer: mng,
	}
	return mh
}

func (mh *MessageHandler) HandleSimpleMessage(packet core.GossipPacket) {
	message := packet.Simple
	fmt.Println("SIMPLE MESSAGE origin", message.OriginalName, "from", message.RelayPeerAddr, "contents", message.Contents)
	mh.printPeers()
	go mh.ctx.ForwardToPeers(*packet.Simple)
}

func (mh *MessageHandler) HandleRumourMessage(packet core.GossipPacket, sender string) {
	rumour := packet.Rumor
	isRouteRumour := len(rumour.Text) == 0
	if !mh.messageExists(*rumour) {
		if mh.ctx.VectorClock.GetMaxIdFrom(rumour.Origin) < rumour.ID {
			mh.ctx.UpdateDSDV(rumour.Origin, sender, isRouteRumour)
		}
		mh.ctx.VectorClock.StoreMessage(rumour)
		mh.ctx.GUImessageChannel <- &core.GUIPacket{Rumour: rumour, Sender: sender}
		go mh.mongerer.StartMongering(packet.Rumor, core.RandomPeer(mh.ctx, sender))
		if strings.Compare(sender, mh.ctx.Address.String()) != 0 {
			// fmt.Println("RUMOR origin", rumour.Origin, "from", sender, "ID", rumour.ID, "contents", rumour.Text)
		}
	}
	if strings.Compare(sender, mh.ctx.Address.String()) != 0 {
		go mh.mongerer.Acknowledge(sender)
	}
}

func (mh *MessageHandler) HandlePrivateMessage(packet core.GossipPacket) {
	private := packet.Private
	found, destinationIP := mh.ctx.RetrieveDestinationRoute(private.Destination)
	switch found {
	case -1:
		return
	case 0:
		mh.ctx.GUImessageChannel <- &core.GUIPacket{Private: private}
		// fmt.Println("PRIVATE origin", private.Origin, "hop-limit", private.HopLimit, "contents", private.Text)
	default:
		if private.HopLimit == 0 {
			return
		}
		private.HopLimit = private.HopLimit - 1
		go mh.ctx.SendPacketToPeer(core.GossipPacket{Private: private}, destinationIP)
	}
}

func (mh *MessageHandler) messageExists(rumour core.RumourMessage) bool {
	_, ok := mh.ctx.VectorClock.GetStoredMessage(rumour.Origin, rumour.ID)
	return ok
}

func (mh *MessageHandler) printPeers() {
	fmt.Print("PEERS ")
	peers := mh.ctx.GetPeers()
	for i := 0; i < len(peers); i++ {
		fmt.Printf(peers[i])
		if i != len(peers)-1 {
			fmt.Print(",")
		}
	}
	fmt.Println("")
}
