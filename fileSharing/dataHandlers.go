package filesharing

import (
	core "github.com/ksei/Peerster/Core"
)

func (fH *FileHandler) HandleDataRequest(packet core.GossipPacket) {
	dataRequestPacket := packet.DataRequest
	found, destinationIP := fH.ctx.RetrieveDestinationRoute(dataRequestPacket.Destination)
	switch found {
	case -1:
		return
	case 0:
		go fH.ProcessDataRequest(dataRequestPacket)
	default:
		if dataRequestPacket.HopLimit == 0 {
			return
		}
		dataRequestPacket.HopLimit--
		go fH.ctx.SendPacketToPeer(core.GossipPacket{DataRequest: dataRequestPacket}, destinationIP)
	}
}

func (fH *FileHandler) HandleDataReply(packet core.GossipPacket) {
	dataReplyPacket := packet.DataReply
	found, destinationIP := fH.ctx.RetrieveDestinationRoute(dataReplyPacket.Destination)
	switch found {
	case -1:
		return
	case 0:
		go fH.ProcessDataReply(dataReplyPacket)
	default:
		if dataReplyPacket.HopLimit == 0 {
			return
		}
		dataReplyPacket.HopLimit--
		go fH.ctx.SendPacketToPeer(core.GossipPacket{DataReply: dataReplyPacket}, destinationIP)
	}
}

func (fH *FileHandler) sendDataReply(dataReply *core.DataReply) {
	dataReply.Origin = fH.ctx.Name
	found, destinationIP := fH.ctx.RetrieveDestinationRoute(dataReply.Destination)
	if found == 1 {
		go fH.ctx.SendPacketToPeer(core.GossipPacket{DataReply: dataReply}, destinationIP)
	}
}

func (fH *FileHandler) sendDataRequest(dataRequest *core.DataRequest) {
	dataRequest.Origin = fH.ctx.Name
	found, destinationIP := fH.ctx.RetrieveDestinationRoute(dataRequest.Destination)
	if found == 1 {
		go fH.ctx.SendPacketToPeer(core.GossipPacket{DataRequest: dataRequest}, destinationIP)
	}
}
