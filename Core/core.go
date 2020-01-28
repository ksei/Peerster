package core

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
)

const (
	SIMPLE_MESSAGE     = 1
	RUMOUR_MESSAGE     = 2
	STATUS_PACKET      = 3
	PRIVATE_MESSAGE    = 4
	DATA_REQUEST       = 5
	DATA_REPLY         = 6
	SEARCH_REQUEST     = 7
	SEARCH_REPLY       = 8
	FILE_INDEXING      = 9
	TLC_MESSAGE        = 10
	TLC_ACK            = 11
	PASSWORD_INSERT    = 12
	PASSWORD_RETRIEVE  = 13
	PASSWORD_OP_RESULT = 14
	PASSWORD_DELETE    = 15
	UNKNOWN            = -1
)

//Message struct for client-gossiper communicaiton
type Message struct {
	Text        string
	Destination *string
	File        *string
	Request     *[]byte
	KeyWords    *string
	Budget      *uint64
	MasterKey   *string
	NewPassword *string
	AccountURL  *string
	UserName    *string
	DeleteUser  *string
}

//SimpleMessage structure
type SimpleMessage struct {
	OriginalName  string
	RelayPeerAddr string
	Contents      string
}

//GossipPacket for building on at a later point
type GossipPacket struct {
	Simple            *SimpleMessage
	Rumor             *RumourMessage
	Status            *StatusPacket
	Private           *PrivateMessage
	DataRequest       *DataRequest
	DataReply         *DataReply
	SearchRequest     *SearchRequest
	SearchReply       *SearchReply
	TLCMessage        *TLCMessage
	Ack               *TLCAck
	PublicSecretShare *PublicShare
	ShareRequest      *ShareRequest
}

//PrivateMessage struct for point to point messaging
type PrivateMessage struct {
	Origin      string
	ID          uint32
	Text        string
	Destination string
	HopLimit    uint32
}

//NewPrivateMessage creates a new privateMessage message
func NewPrivateMessage(id, hopLimit uint32, txt, origin, destination string) *PrivateMessage {
	privateMessage := &PrivateMessage{
		Origin:      origin,
		ID:          id,
		Text:        txt,
		Destination: destination,
		HopLimit:    hopLimit,
	}
	return privateMessage
}

//RumourMessage struct definition
type RumourMessage struct {
	Origin string `json:"origin"`
	ID     uint32
	Text   string `json:"text"`
}

//NewRumourMessage creates a new rumour message
func NewRumourMessage(id uint32, txt, origin string) *RumourMessage {
	rumour := &RumourMessage{
		Origin: origin,
		ID:     id,
		Text:   txt,
	}
	return rumour
}

//PeerStatus struct used to regulate inter-peer communication
type PeerStatus struct {
	Identifier string
	NextID     uint32
}

//StatusPacket struct
type StatusPacket struct {
	Want []PeerStatus
}

//DataRequest for chunk and metafile requests
type DataRequest struct {
	Origin      string
	Destination string
	HopLimit    uint32
	HashValue   []byte
}

//DataReply for chunk and metafile replies
type DataReply struct {
	Origin      string
	Destination string
	HopLimit    uint32
	HashValue   []byte
	Data        []byte
}

//SearchRequest for searching files
type SearchRequest struct {
	Origin   string
	Budget   uint64
	Keywords []string
}

//SearchReply for returning search results
type SearchReply struct {
	Origin      string
	Destination string
	HopLimit    uint32
	Results     []*SearchResult
}

//SearchResult containing matching file information
type SearchResult struct {
	FileName     string
	MetafileHash []byte
	ChunkMap     []uint64
	ChunkCount   uint64
}

//TxPublish for publishing fileNames
type TxPublish struct {
	Name         string
	Size         int64 // Size in bytes
	MetafileHash []byte
}

//BlockPublish for transporting transactions
type BlockPublish struct {
	PrevHash    [32]byte
	Transaction TxPublish
}

//TLCMessage used to transfer blocks
type TLCMessage struct {
	Origin      string
	ID          uint32
	Confirmed   int
	TxBlock     BlockPublish
	VectorClock *StatusPacket
	Fitness     float32
}

//TLCAck for ackonledgements
type TLCAck PrivateMessage

//InternalPacket used to transmit messages internally accompanied by the sender's address
type InternalPacket struct {
	Packet GossipPacket
	Sender string
}

//GUIPacket handles outgoing communication to the webServer
type GUIPacket struct {
	Sender           string
	Rumour           *RumourMessage
	Private          *PrivateMessage
	SearchResult     *SearchResult
	Password         *string
	PasswordOpResult *string
}

/*PublicShare represents the actual data structure to be transmitted inside a gossip packet
- replicateID: id identifying the replicate index of the share for a password (i.e. one share might be delivered to 3 different peers)
- uid: Unique Indentiefier of the SecretShare
- securedShare: a byte array representing the encrypted Share data structure to be shared inside this secretShare
*/
type PublicShare struct {
	Origin       string
	Destination  string
	HopLimit     uint32
	UID          string
	SecuredShare []byte
	Requested    bool
	Confirmation bool
}

//ShareRequest serves as a struct designated for sending requests in an expanding ring manner, in order to reconstruct a password through received shares
type ShareRequest struct {
	Origin     string
	Budget     uint64
	RequestUID string
}

//GetType used to determine contents of a given GossiperPacket
func (gp *GossipPacket) GetType(allowSimple bool) (int, error) {
	if gp.Simple != nil && gp.Rumor == nil && gp.Status == nil && gp.Private == nil && gp.DataReply == nil && gp.DataRequest == nil && gp.SearchRequest == nil && gp.SearchReply == nil && gp.TLCMessage == nil && gp.Ack == nil && gp.PublicSecretShare == nil && gp.ShareRequest == nil && allowSimple {
		return SIMPLE_MESSAGE, nil
	} else if gp.Simple == nil && gp.Rumor != nil && gp.Status == nil && gp.Private == nil && gp.DataReply == nil && gp.DataRequest == nil && gp.SearchRequest == nil && gp.SearchReply == nil && gp.TLCMessage == nil && gp.Ack == nil && gp.PublicSecretShare == nil && gp.ShareRequest == nil && !allowSimple {
		return RUMOUR_MESSAGE, nil
	} else if gp.Simple == nil && gp.Rumor == nil && gp.Status != nil && gp.Private == nil && gp.DataReply == nil && gp.DataRequest == nil && gp.SearchRequest == nil && gp.SearchReply == nil && gp.TLCMessage == nil && gp.Ack == nil && gp.PublicSecretShare == nil && gp.ShareRequest == nil && !allowSimple {
		return STATUS_PACKET, nil
	} else if gp.Simple == nil && gp.Rumor == nil && gp.Status == nil && gp.Private != nil && gp.DataReply == nil && gp.DataRequest == nil && gp.SearchRequest == nil && gp.SearchReply == nil && gp.TLCMessage == nil && gp.Ack == nil && gp.PublicSecretShare == nil && gp.ShareRequest == nil && !allowSimple {
		return PRIVATE_MESSAGE, nil
	} else if gp.Simple == nil && gp.Rumor == nil && gp.Status == nil && gp.Private == nil && gp.DataReply != nil && gp.DataRequest == nil && gp.SearchRequest == nil && gp.SearchReply == nil && gp.TLCMessage == nil && gp.Ack == nil && gp.PublicSecretShare == nil && gp.ShareRequest == nil && !allowSimple {
		return DATA_REPLY, nil
	} else if gp.Simple == nil && gp.Rumor == nil && gp.Status == nil && gp.Private == nil && gp.DataReply == nil && gp.DataRequest != nil && gp.SearchRequest == nil && gp.SearchReply == nil && gp.TLCMessage == nil && gp.Ack == nil && gp.PublicSecretShare == nil && gp.ShareRequest == nil && !allowSimple {
		return DATA_REQUEST, nil
	} else if gp.Simple == nil && gp.Rumor == nil && gp.Status == nil && gp.Private == nil && gp.DataReply == nil && gp.DataRequest == nil && gp.SearchRequest != nil && gp.SearchReply == nil && gp.TLCMessage == nil && gp.Ack == nil && gp.PublicSecretShare == nil && gp.ShareRequest == nil && !allowSimple {
		return SEARCH_REQUEST, nil
	} else if gp.Simple == nil && gp.Rumor == nil && gp.Status == nil && gp.Private == nil && gp.DataReply == nil && gp.DataRequest == nil && gp.SearchRequest == nil && gp.SearchReply != nil && gp.TLCMessage == nil && gp.Ack == nil && gp.PublicSecretShare == nil && gp.ShareRequest == nil && !allowSimple {
		return SEARCH_REPLY, nil
	} else if gp.Simple == nil && gp.Rumor == nil && gp.Status == nil && gp.Private == nil && gp.DataReply == nil && gp.DataRequest == nil && gp.SearchRequest == nil && gp.SearchReply == nil && gp.TLCMessage != nil && gp.Ack == nil && gp.PublicSecretShare == nil && gp.ShareRequest == nil && !allowSimple {
		return TLC_MESSAGE, nil
	} else if gp.Simple == nil && gp.Rumor == nil && gp.Status == nil && gp.Private == nil && gp.DataReply == nil && gp.DataRequest == nil && gp.SearchRequest == nil && gp.SearchReply == nil && gp.TLCMessage == nil && gp.Ack != nil && gp.PublicSecretShare == nil && gp.ShareRequest == nil && !allowSimple {
		return TLC_ACK, nil
	} else if gp.Simple == nil && gp.Rumor == nil && gp.Status == nil && gp.Private == nil && gp.DataReply == nil && gp.DataRequest == nil && gp.SearchRequest == nil && gp.SearchReply == nil && gp.TLCMessage == nil && gp.Ack == nil && gp.PublicSecretShare != nil && gp.ShareRequest == nil && !allowSimple {
		return PASSWORD_INSERT, nil
	} else if gp.Simple == nil && gp.Rumor == nil && gp.Status == nil && gp.Private == nil && gp.DataReply == nil && gp.DataRequest == nil && gp.SearchRequest == nil && gp.SearchReply == nil && gp.TLCMessage == nil && gp.Ack == nil && gp.PublicSecretShare == nil && gp.ShareRequest != nil && !allowSimple {
		return PASSWORD_RETRIEVE, nil
	} else {
		return 0, errors.New("Corrupt Gossip Packet received: Multiple content packet received, or faulty broadcasting mode (SimpleMode set to true)")
	}
}

//GetType of Client Message
func (m *Message) GetType(simpleMode bool) int {
	if simpleMode {
		return SIMPLE_MESSAGE
	} else if m.File != nil && m.Destination == nil && len(*m.Request) == 0 {
		return FILE_INDEXING
	} else if m.File != nil && m.Request != nil {
		return DATA_REQUEST
	} else if m.KeyWords != nil {
		return SEARCH_REQUEST
	} else if m.MasterKey != nil && m.NewPassword != nil {
		return PASSWORD_INSERT
	} else if m.MasterKey != nil && m.NewPassword == nil && m.UserName != nil {
		return PASSWORD_RETRIEVE
	} else if m.MasterKey != nil && m.NewPassword == nil && m.DeleteUser != nil {
		return PASSWORD_DELETE
	} else if m.Destination != nil {
		return PRIVATE_MESSAGE
	} else {
		return RUMOUR_MESSAGE
	}
}

//CreateGossipPacket shortcut for getting gossip packet out of RumourMessage
func CreateGossipPacket(content Stackable) GossipPacket {
	switch content.(type) {
	case *TLCMessage:
		return GossipPacket{TLCMessage: content.(*TLCMessage)}
	default:
		return GossipPacket{Rumor: content.(*RumourMessage)}

	}
}

//GetType for GuiPackets
func (gp *GUIPacket) GetType() int {
	if gp.Rumour != nil {
		return RUMOUR_MESSAGE
	}
	if gp.Private != nil {
		return PRIVATE_MESSAGE
	}
	if gp.SearchResult != nil {
		return SEARCH_REPLY
	}
	if gp.Password != nil {
		return PASSWORD_RETRIEVE
	}
	if gp.PasswordOpResult != nil {
		return PASSWORD_OP_RESULT
	}
	return UNKNOWN
}

func (b *BlockPublish) Hash() (out [32]byte) {
	h := sha256.New()
	h.Write(b.PrevHash[:])
	th := b.Transaction.Hash()
	h.Write(th[:])
	copy(out[:], h.Sum(nil))
	return
}

func (t *TxPublish) Hash() (out [32]byte) {
	h := sha256.New()
	binary.Write(h, binary.LittleEndian, uint32(len(t.Name)))
	h.Write([]byte(t.Name))
	h.Write(t.MetafileHash)
	copy(out[:], h.Sum(nil))
	return
}
