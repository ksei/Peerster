package webserver

import (
	"encoding/hex"
	"errors"

	core "github.com/ksei/Peerster/Core"
)

type sockPacket struct {
	Type        string `json:"type"`
	IPAddress   string `json:"ipAddr"`
	Origin      string `json:"origin"`
	Message     string `json:"message"`
	Destination string `jsong:"destination"`
	Me          string `json:"me"`
	Filename    string `json:"filename"`
	Metahash    string `json:"metahash"`
	Keywords    string `json:"keywords"`
}

//Creates peerPackets for sending to the client
func createPeerPacket(peer string) *sockPacket {
	packet := &sockPacket{Type: "PeerUpdate", IPAddress: peer}
	return packet
}

//Creates Message Packets for sending to the client
func processGUIPacket(incomingPacket core.GUIPacket) (*sockPacket, error) {
	contentType := incomingPacket.GetType()
	var packet = &sockPacket{}
	switch contentType {
	case core.RUMOUR_MESSAGE:
		packet.Type = "Message"
		packet.IPAddress = incomingPacket.Sender
		packet.Origin = incomingPacket.Rumour.Origin
		packet.Message = incomingPacket.Rumour.Text
		return packet, nil
	case core.PRIVATE_MESSAGE:
		packet.Type = "PrivateMessage"
		packet.Origin = incomingPacket.Private.Origin
		packet.Message = incomingPacket.Private.Text
		return packet, nil
	case core.SEARCH_REPLY:
		packet.Type = "SearchMatch"
		packet.Filename = incomingPacket.SearchResult.FileName
		packet.Metahash = hex.EncodeToString(incomingPacket.SearchResult.MetafileHash)
		return packet, nil
	}
	return nil, errors.New("Corrupt Packet received")
}
