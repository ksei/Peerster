package core

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/dedis/protobuf"
)

//Context carrying contextual information on the state of the gossiper
type Context struct {
	Address           *net.UDPAddr
	conn              *net.UDPConn
	UIport            string
	Name              string
	connLocker        sync.RWMutex
	peerLocker        sync.RWMutex
	Peers             []string
	GUImessageChannel chan *GUIPacket
	VectorClock       VectorClock
	SimpleMode        bool
	dsdvLocker        sync.RWMutex
	DSDVector         map[string]string
	hopLimit          uint32
	hw3Flags          [2]bool
}

//CreateContext creates a new Context
func CreateContext(Address, name, UIp string, simple, hw3ex2, hw3ex3 bool, hopLim uint32) *Context {
	udpAddr, err := net.ResolveUDPAddr("udp4", Address)
	udpConn, err := net.ListenUDP("udp4", udpAddr)
	if err != nil {
		log.Fatal(err)
	}

	ctx := &Context{
		Address:           udpAddr,
		conn:              udpConn,
		Name:              name,
		UIport:            UIp,
		GUImessageChannel: make(chan *GUIPacket, 50),
		SimpleMode:        simple,
		DSDVector:         make(map[string]string),
		hopLimit:          hopLim,
	}
	ctx.hw3Flags[0] = hw3ex2 || hw3ex3
	ctx.hw3Flags[1] = hw3ex3
	ctx.VectorClock = *NewVectorClock()
	return ctx
}

//AddPeer to gossiper
func (ctx *Context) AddPeer(pAddr string) {
	ctx.peerLocker.Lock()
	ctx.peerLocker.Unlock()

	if len(pAddr) > 0 {
		ctx.Peers = append(ctx.Peers, pAddr)
	}
}

//GetPeers safley locking
func (ctx *Context) GetPeers() []string {
	ctx.peerLocker.RLock()
	defer ctx.peerLocker.RUnlock()

	return ctx.Peers
}

//GetConnection returns the connection of our context
func (ctx *Context) GetConnection() *net.UDPConn {
	ctx.connLocker.RLock()
	defer ctx.connLocker.RUnlock()
	return ctx.conn
}

//SendPacketToPeer sends a gossipPacket to a specified peer
func (ctx *Context) SendPacketToPeer(gossipPacket GossipPacket, peer string) error {
	ctx.connLocker.RLock()
	defer ctx.connLocker.RUnlock()
	peerAddress, err := net.ResolveUDPAddr("udp", peer)
	if err != nil {
		return err
	}

	packetBytes, err := protobuf.Encode(&gossipPacket)
	if err != nil {
		return err
	}

	ctx.conn.WriteToUDP(packetBytes, peerAddress)
	return nil
}

func (ctx *Context) SendPacketToPeerViaRouting(gossipePacket GossipPacket, peer string) {
	found, destination := ctx.RetrieveDestinationRoute(peer)
	if found == 1 {
		ctx.SendPacketToPeer(gossipePacket, destination)
		return
	}
	fmt.Println("Unable to retrieve route for given origin...")
}

//ForwardToPeers forwards a message to all known peers
func (ctx *Context) ForwardToPeers(message SimpleMessage) error {
	for _, knwonPeer := range ctx.GetPeers() {
		if strings.Compare(knwonPeer, message.RelayPeerAddr) == 0 {
			continue
		}
		message.RelayPeerAddr = ctx.Address.String()
		gossipPacket := GossipPacket{Simple: &message}

		peerAddress, err1 := net.ResolveUDPAddr("udp", knwonPeer)
		if err1 != nil {
			return err1
		}

		packetBytes, err := protobuf.Encode(&gossipPacket)
		if err != nil {
			fmt.Println(err)
		}
		ctx.conn.WriteToUDP(packetBytes, peerAddress)
	}
	return nil
}

//RandomPeer to be generated
func RandomPeer(ctx *Context, sender string) string {
	peerList := ctx.GetPeers()
	totalPeers := len(peerList) //Preventing infinite loop in case of only one peer
	randPeer := peerList[rand.Intn(totalPeers)]
	for keepSearching := true; keepSearching; keepSearching = (strings.Compare(randPeer, sender) == 0 && totalPeers != 1) {
		randPeer = peerList[rand.Intn(totalPeers)]
	}
	return randPeer
}

//RandomPeers gets n random peers different than the given sender
func RandomPeers(n int, ctx *Context, sender string) []string {
	peerList := ctx.GetPeers()
	totalPeers := len(peerList)
	rand.Seed(time.Now().UnixNano())
	p := rand.Perm(totalPeers)
	randomPeers := []string{}
	for _, r := range p[:n] {
		if peerList[r] == sender {
			peerList = append(peerList[:r], peerList[r+1:]...)
			r--
			continue
		}
		randomPeers = append(randomPeers, peerList[r])
	}
	return randomPeers
}

//UpdateDSDV updates the routing table based on route messages
func (ctx *Context) UpdateDSDV(origin, latestIP string, isRouteMessage bool) {
	if strings.Compare(latestIP, ctx.Address.String()) == 0 {
		return
	}
	ctx.dsdvLocker.Lock()
	ctx.DSDVector[origin] = latestIP
	ctx.dsdvLocker.Unlock()
	if !isRouteMessage {
		// fmt.Println("DSDV", origin, latestIP)
	}
}

//RetrieveDestinationRoute finds the next hop to follow given a destination
func (ctx *Context) RetrieveDestinationRoute(destination string) (int, string) {
	if strings.Compare(destination, ctx.Name) == 0 {
		return 0, ""
	}
	ctx.dsdvLocker.RLock()
	defer ctx.dsdvLocker.RUnlock()
	destinationIP, ok := ctx.DSDVector[destination]
	if !ok {
		return -1, ""
	}
	return 1, destinationIP
}

//GetHopLimit retrieves the common hopLimit from the context
func (ctx *Context) GetHopLimit() uint32 {
	return ctx.hopLimit
}

//RunningHw3Ex2 gets Hw3Ex2 Flag
func (ctx *Context) RunningHw3Ex2() bool {
	return ctx.hw3Flags[0]
}

//RunningHw3Ex3 gets Hw3Ex3 Flag
func (ctx *Context) RunningHw3Ex3() bool {
	return ctx.hw3Flags[1]
}
