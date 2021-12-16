package webserver

import (
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	core "github.com/ksei/Peerster/Core"

	"github.com/gorilla/websocket"
	"go.dedis.ch/protobuf"
)

const localAddress string = "127.0.0.1"

//WebServer for main execution
type WebServer struct {
	ctx                 *core.Context
	clients             map[*websocket.Conn]bool
	broadcast           chan sockPacket // broadcast channel
	upgrader            websocket.Upgrader
	locker              sync.RWMutex
	PeerViewState       map[string]bool
	UIPort              string
	incomingClientPeers chan *sockPacket
}

//NewServer Instantiates new server
func NewServer(cntx *core.Context, UIP *string) *WebServer {
	webServer := &WebServer{
		ctx:                 cntx,
		clients:             make(map[*websocket.Conn]bool),
		broadcast:           make(chan sockPacket), // broadcast channel
		upgrader:            websocket.Upgrader{},
		PeerViewState:       make(map[string]bool),
		incomingClientPeers: make(chan *sockPacket),
		UIPort:              *UIP,
	}

	return webServer
}

//WEB SERVER SPECIFIC FUNCTIONALITY --------------------------------------------------------

//Launch starts webServer
func (webServer *WebServer) Launch(gossiperAddress string) {
	webServer.updatePeerView(append(webServer.ctx.GetPeers(), gossiperAddress+"--me"))
	// Create a simple file server
	fs := http.FileServer(http.Dir("./public"))
	http.Handle("/", fs)
	http.HandleFunc("/ws", webServer.handleConnections)
	go webServer.handleIncomingPeerUpdate()
	go webServer.handleSocketPackets()
	go webServer.handleGossiperPackets()

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}

	fmt.Println("Using port:", listener.Addr().(*net.TCPAddr).Port)
	err = exec.Command("xdg-open", "http://localhost:"+strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)).Start()
	if err != nil {
		fmt.Println(err)
	}
	panic(http.Serve(listener, nil))
}

//Handles Connections with connected clients
func (webServer *WebServer) handleConnections(w http.ResponseWriter, r *http.Request) {
	webServer.upgrader.CheckOrigin = func(r *http.Request) bool { return true }

	// Upgrade initial GET request to a websocket
	ws, err := webServer.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	// Make sure we close the connection when the function returns
	defer ws.Close()
	webServer.clients[ws] = true
	webServer.initiatePeerView(ws)
	for {
		var msg sockPacket
		// Read in a new message as JSON and map it to a Message object
		err := ws.ReadJSON(&msg)
		if err != nil {
			log.Printf("error: %v", err)
			delete(webServer.clients, ws)
			break
		}
		// Send the newly received message to the broadcast channel
		webServer.broadcast <- msg
	}
}

//GOSSIP-PACKET HANDLING ---------------------------------------------------------

//Handles GossipPackets coming from the gossiper
func (webServer *WebServer) handleGossiperPackets() {
	for guiPacket := range webServer.ctx.GUImessageChannel {
		// Send it out to every client that is currently connected
		for client := range webServer.clients {
			sockPck, err := processGUIPacket(*guiPacket)
			if err != nil {
				if err != nil {
					log.Printf("error: %v", err)
					client.Close()
					delete(webServer.clients, client)
				}
			}
			err = client.WriteJSON(sockPck)
			if err != nil {
				log.Printf("error: %v", err)
				client.Close()
				delete(webServer.clients, client)
			}
		}
		webServer.updatePeerView(webServer.ctx.GetPeers())
	}
}

//Method for forwarding a core.Message packet to the gossiper
func (webServer *WebServer) sendMessageToGossiper(message core.Message) {
	toSend := localAddress + ":" + webServer.UIPort
	updAddr, err1 := net.ResolveUDPAddr("udp", toSend)
	if err1 != nil {
		fmt.Println(err1)
	}
	conn, err := net.DialUDP("udp", nil, updAddr)
	if err != nil {
		fmt.Println(err)
	}
	defer conn.Close()
	packetBytes, err := protobuf.Encode(&message)
	conn.Write(packetBytes)
}

//WEB-CLIENT INCOMING MESSAGE HANDLING ---------------------------------

//Handles Packets coming from the client interface
func (webServer *WebServer) handleSocketPackets() {
	for {
		// Grab the next message from the broadcast channel
		msg := <-webServer.broadcast
		switch msg.Type {
		case "PeerUpdate":
			webServer.incomingClientPeers <- &msg
		case "FileSharing":
			go webServer.handleIncomingFileRequest(msg)
		case "SearchRequest":
			go webServer.handleNewSearchRequest(msg)
		case "PasswordRequest":
			go webServer.handlePasswordRequest(msg)
		case "StorePasswordRequest":
			go webServer.handleStorePasswordRequest(msg)
		case "PasswordDelete":
			go webServer.handlePasswordDelete(msg)
		default:
			go webServer.handleIncomingMessage(msg)
		}
	}
}

//Handles incoming messages from the web client either Rumor or Private
func (webServer *WebServer) handleIncomingMessage(msg sockPacket) {
	message := core.Message{Text: msg.Message}
	if strings.Compare(msg.Type, "PrivateMessage") == 0 {
		message.Destination = &msg.Destination
	}
	webServer.sendMessageToGossiper(message)
}

func (webServer *WebServer) handleNewSearchRequest(req sockPacket) {
	message := core.Message{KeyWords: &req.Keywords}
	webServer.sendMessageToGossiper(message)
}

func (webServer *WebServer) handlePasswordRequest(req sockPacket) {
	message := core.Message{AccountURL: &req.Account, UserName: &req.Username, MasterKey: &req.MasterKey}
	webServer.sendMessageToGossiper(message)
}

func (webServer *WebServer) handleStorePasswordRequest(req sockPacket) {
	message := core.Message{AccountURL: &req.Account, UserName: &req.Username, MasterKey: &req.MasterKey, NewPassword: &req.Password}
	webServer.sendMessageToGossiper(message)
}

func (webServer *WebServer) handlePasswordDelete(req sockPacket) {
	message := core.Message{AccountURL: &req.Account, DeleteUser: &req.Username, MasterKey: &req.MasterKey}
	webServer.sendMessageToGossiper(message)
}

//Handles File Requests initiated from the web client
func (webServer *WebServer) handleIncomingFileRequest(msg sockPacket) {
	var requestBytes []byte
	var err error
	requestBytes = nil
	if len(msg.Metahash) > 0 {
		requestBytes, err = hex.DecodeString(msg.Metahash)
		if err != nil {
			fmt.Println(err)
		}
	}

	destination := &msg.Destination
	if len(msg.Destination) == 0 {
		destination = nil
	}

	message := core.Message{File: &msg.Filename, Request: &requestBytes, Destination: destination}
	webServer.sendMessageToGossiper(message)
}

func (webServer *WebServer) handleIncomingPeerUpdate() {
	for peerPacket := range webServer.incomingClientPeers {
		peerToAdd := peerPacket.IPAddress
		webServer.ctx.AddPeer(peerToAdd)
		webServer.updatePeerView(webServer.ctx.GetPeers())
	}
}

//Updates the peer-model we have for maintaining a synchronized state between gossiper's and webServer's Peers
func (webServer *WebServer) updatePeerView(peerList []string) error {
	webServer.locker.Lock()
	defer webServer.locker.Unlock()
	for _, peer := range peerList {
		_, ok := webServer.PeerViewState[peer]
		if !ok {
			err := webServer.sendPeer(peer)
			if err != nil {
				return err
			}
			webServer.PeerViewState[peer] = true
		}
	}
	return nil
}

//Initial Update of the Peer View-Model
func (webServer *WebServer) initiatePeerView(ws *websocket.Conn) error {
	webServer.locker.Lock()
	defer webServer.locker.Unlock()
	for peer := range webServer.PeerViewState {
		err := webServer.sendPeerToClient(peer, ws)
		if err != nil {
			return err
		}
		webServer.PeerViewState[peer] = true
	}
	return nil
}

func (webServer *WebServer) sendPeerToClient(peer string, ws *websocket.Conn) error {
	packet := *createPeerPacket(peer)
	packet.Me = webServer.ctx.Name
	err := ws.WriteJSON(packet)
	if err != nil {
		return err
	}
	return nil
}

//Sends a given Peer to each clinet instance
func (webServer *WebServer) sendPeer(peer string) error {
	for client := range webServer.clients {
		packet := *createPeerPacket(peer)
		packet.Me = webServer.ctx.Name
		err := client.WriteJSON(packet)
		if err != nil {
			return err
		}
	}
	return nil
}
