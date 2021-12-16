package gossiper

import (
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	core "github.com/ksei/Peerster/Core"
	mng "github.com/ksei/Peerster/Mongering"
	"github.com/ksei/Peerster/SecretSharing"
	tlc "github.com/ksei/Peerster/TLC"
	fh "github.com/ksei/Peerster/fileSharing"
	mh "github.com/ksei/Peerster/messageHandling"
	"go.dedis.ch/protobuf"
)

const localAddress = "127.0.0.1"

//Gossiper basic instance
type Gossiper struct {
	ctx                   *core.Context
	clientIncomingChannel chan core.Message
	peerIncomingChannel   chan core.InternalPacket
	fileHandler           *fh.FileHandler
	mongerer              *mng.Mongerer
	messageHandler        *mh.MessageHandler
	tlcHandler            *tlc.TLCHandler
	shamirHandler         *SecretSharing.SSHandler
}

//NewGossiper method
func NewGossiper(address, name, UIp string, useSimpleMode, hw3ex2, hw3ex3 bool, antiEntropy, routing, totalPeers, stubbornTimeout, hopLimit int) (*Gossiper, *core.Context) {
	gossiper := &Gossiper{
		clientIncomingChannel: make(chan core.Message, 50),
		peerIncomingChannel:   make(chan core.InternalPacket, 50),
	}
	gossiper.ctx = core.CreateContext(address, name, UIp, useSimpleMode, hw3ex2, hw3ex3, uint32(hopLimit))
	gossiper.fileHandler = fh.NewFileHandler(gossiper.ctx)
	gossiper.mongerer = mng.NewMongerer(gossiper.ctx, antiEntropy)
	gossiper.messageHandler = mh.NewMessageHandler(gossiper.mongerer)
	gossiper.tlcHandler = tlc.NewTLCHandler(gossiper.mongerer, totalPeers, stubbornTimeout)
	gossiper.shamirHandler = SecretSharing.NewSSHandler(gossiper.ctx)
	go gossiper.ListenToClients()
	go gossiper.ListenToPeers()
	go gossiper.startRouting(routing)
	go gossiper.waitForIncomingClientMessage()
	go gossiper.waitForIncomingPeerMessage()
	return gossiper, gossiper.ctx
}

//ListenToClients method
func (g *Gossiper) ListenToClients() {
	udpAddress, err := net.ResolveUDPAddr("udp4", localAddress+":"+g.ctx.UIport)
	clientServer, err := net.ListenUDP("udp4", udpAddress)
	if err != nil {
		log.Fatal("Error: ", err)
	}

	for {
		buf := make([]byte, 1024)
		n, _, err := clientServer.ReadFromUDP(buf)
		if err != nil {
			log.Println("Error while reading: ", err)
		}
		go g.handleResponse(buf, n, "CLIENT")
		buf = nil
	}
}

func (g *Gossiper) waitForIncomingClientMessage() {
	for cMessage := range g.clientIncomingChannel {
		contentType := cMessage.GetType(g.ctx.SimpleMode)
		switch contentType {
		case core.SIMPLE_MESSAGE:
			simpleMessage := &core.SimpleMessage{OriginalName: g.ctx.Name, Contents: cMessage.Text}
			go g.ctx.ForwardToPeers(*simpleMessage)
		case core.FILE_INDEXING:
			fileSize, metahash := g.fileHandler.IndexFile(*cMessage.File)
			if fileSize != -1 && g.ctx.RunningHw3Ex2() {
				packet := core.GossipPacket{TLCMessage: g.tlcHandler.NewTLCFromTxPublish(*cMessage.File, fileSize, metahash)}
				go g.tlcHandler.HandleTLCMessage(packet, g.ctx.Address.String())
			}
		case core.DATA_REQUEST:
			go g.fileHandler.InitiateFileRequest(cMessage.Destination, *cMessage.File, []byte(*cMessage.Request))
		case core.SEARCH_REQUEST:
			go g.fileHandler.LaunchSearch(cMessage.KeyWords, cMessage.Budget)
		case core.PASSWORD_RETRIEVE:
			go g.shamirHandler.HandlePasswordRetrieval(*cMessage.MasterKey, *cMessage.AccountURL, *cMessage.UserName)
		case core.PASSWORD_INSERT:
			go g.shamirHandler.HandlePasswordInsert(*cMessage.MasterKey, *cMessage.AccountURL, *cMessage.UserName, *cMessage.NewPassword)
		case core.PASSWORD_DELETE:
			go g.shamirHandler.HandlePasswordDelete(*cMessage.MasterKey, *cMessage.AccountURL, *cMessage.DeleteUser)
		case core.PRIVATE_MESSAGE:
			fmt.Println("CLIENT MESSAGE", cMessage.Text, "dest", *(cMessage.Destination))
			privateMessage := core.NewPrivateMessage(0, g.ctx.GetHopLimit(), cMessage.Text, g.ctx.Name, *cMessage.Destination)
			go g.messageHandler.HandlePrivateMessage(core.GossipPacket{Private: privateMessage})
		case core.RUMOUR_MESSAGE:
			fmt.Println("CLIENT MESSAGE", cMessage.Text)
			rumour := core.NewRumourMessage(g.ctx.VectorClock.GetNextIDFrom(g.ctx.Name), cMessage.Text, g.ctx.Name)
			go g.messageHandler.HandleRumourMessage(core.GossipPacket{Rumor: rumour}, g.ctx.Address.String())
		}
	}
}

//ListenToPeers method
func (g *Gossiper) ListenToPeers() {
	for {
		buf := make([]byte, 12288)
		n, udpAddr, err := g.ctx.GetConnection().ReadFromUDP(buf)
		if err != nil {
			log.Fatal("Error: ", err)
		}
		g.evaluateIncomingAddress(udpAddr)
		go g.handleResponse(buf, n, udpAddr.String())
		buf = nil
	}
}
func (g *Gossiper) evaluateIncomingAddress(address *net.UDPAddr) {
	for _, knwonPeer := range g.ctx.GetPeers() {
		if strings.Compare(knwonPeer, address.String()) == 0 {
			return
		}
	}
	g.ctx.AddPeer(address.String())
}

func (g *Gossiper) handleResponse(bytes []byte, n int, sender string) error {
	isClient := strings.Compare(sender, "CLIENT") == 0
	if isClient {
		incomingClientMessage := core.Message{}
		err := protobuf.Decode(bytes[:n], &incomingClientMessage)
		if err != nil {
			return err
		}
		g.clientIncomingChannel <- incomingClientMessage
	} else {
		incomingPacket := core.GossipPacket{}
		err := protobuf.Decode(bytes[:n], &incomingPacket)
		if err != nil {
			fmt.Println(err)
			return err
		}
		g.peerIncomingChannel <- core.InternalPacket{Packet: incomingPacket, Sender: sender}
	}
	return nil
}

func (g *Gossiper) waitForIncomingPeerMessage() {
	for receivedPacket := range g.peerIncomingChannel {
		sender := receivedPacket.Sender
		packet := receivedPacket.Packet
		contentType, err := packet.GetType(g.ctx.SimpleMode)
		if err != nil {
			log.Fatal("Error receiving peer packet: ", err)
		}
		switch contentType {
		case core.SIMPLE_MESSAGE:
			go g.messageHandler.HandleSimpleMessage(packet)
		case core.STATUS_PACKET:
			go g.mongerer.HandleStatusPacket(packet, sender)
		case core.PRIVATE_MESSAGE:
			go g.messageHandler.HandlePrivateMessage(packet)
		case core.DATA_REQUEST:
			go g.fileHandler.HandleDataRequest(packet)
		case core.DATA_REPLY:
			go g.fileHandler.HandleDataReply(packet)
		case core.SEARCH_REQUEST:
			go g.fileHandler.HandleSearchRequest(packet, sender)
		case core.SEARCH_REPLY:
			go g.fileHandler.HandleSearchReply(packet)
		case core.TLC_MESSAGE:
			go g.tlcHandler.HandleTLCMessage(packet, sender)
		case core.TLC_ACK:
			go g.tlcHandler.HandleTLCAck(packet)
		case core.PASSWORD_INSERT:
			go g.shamirHandler.HandlePublicShare(packet)
		case core.PASSWORD_RETRIEVE:
			go g.shamirHandler.HandleSearchRequest(packet, sender)
		default:
			go g.messageHandler.HandleRumourMessage(packet, sender)
		}
	}
}

func (g *Gossiper) startRouting(intervalPeriodseconds int) {
	if g.ctx.SimpleMode {
		return
	}
	for {
		if len(g.ctx.GetPeers()) == 0 {
			continue
		}
		rumour := core.NewRumourMessage(g.ctx.VectorClock.GetNextIDFrom(g.ctx.Name), "", g.ctx.Name)
		go g.messageHandler.HandleRumourMessage(core.GossipPacket{Rumor: rumour}, g.ctx.Address.String())
		if intervalPeriodseconds == 0 {
			break
		}
		time.Sleep(time.Duration(intervalPeriodseconds) * time.Second)
	}
}
