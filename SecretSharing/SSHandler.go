package SecretSharing

import (
	"fmt"
	"sync"
	"encoding/hex"

	core "github.com/ksei/Peerster/Core"
)

const (
	MIN_ITER   = 2000
	MAX_ITER   = 3000
	MIN_SHARES = 6
)

//SSHandler struct to handle secret sharing within e gossiper network
type SSHandler struct {
	ssLocker                sync.RWMutex
	ctx                     *core.Context
	storedPasswords         []string
	tempKeyStorage          string
	extraInfo               map[string]*extraInfo
	thresholds              map[string]int
	hostedShares            map[string][]byte
	requestedPasswordStatus map[string]map[uint32]*Share
	thresholdReached        chan *string
}

//NewSSHandler initialized a new SSHandler
func NewSSHandler(ctx *core.Context) *SSHandler {
	h := &SSHandler{
		ctx:                     ctx,
		storedPasswords:         make([]string, 0),
		extraInfo:               make(map[string]*extraInfo),
		thresholds:              make(map[string]int),
		hostedShares:            make(map[string][]byte),
		requestedPasswordStatus: make(map[string]map[uint32]*Share),
		thresholdReached:        make(chan *string, 10),
	}

	return h
}

//HandlePasswordInsert handles password insertion by user
func (ssHandler *SSHandler) HandlePasswordInsert(masterKey, account, username, newPassword string) {

	//1. Assign Password UID and check if already inserted (for now we can start without supporting password upates)
	if _, exists := ssHandler.passwordExists(masterKey, account, username); exists {
		fmt.Println("Password has been already registered. Updates are not supported at the moment.")
		return
	}
	passwordUID, err := ssHandler.registerPassword(masterKey, account, username)
	if err != nil {
		fmt.Println(err)
		return
	}
	//2. Encrypt password using key derived by master key + account + username
	encryptedPass, err := ssHandler.encryptPassword(masterKey, passwordUID, newPassword)
	if err != nil {
		fmt.Println(err)
		return
	}
	//3. Generate shares
	totalShares, retrievingThreshold, err := ssHandler.getSplittingParams()
	if err != nil {
		fmt.Println(err)
		return
	}
	ssHandler.storeThreshold(passwordUID, retrievingThreshold)
	shares, err := GenerateShares(encryptedPass, totalShares, retrievingThreshold)
	if err != nil {
		fmt.Println(err)
		return
	}
	//4. Assign each share to an origin from dsdv
	peerReplicateIndex, err := ssHandler.mapSharesToPeers(totalShares)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Password Successfully Deconstructed")
	//5. Create secret shares encrypting each share using key derived from master key + account + username + peer-to-be-sent-to
	//6. Create public shares using the secret share and a uid generated by the hash of password UID, peer that it is sent to and index of the share for that peer
	//encryptShares returns a map with origins as keys and public shares as values
	publicShares, err := ssHandler.encryptShares(masterKey, passwordUID, peerReplicateIndex, shares)
	if err != nil {
		fmt.Println(err)
		return
	}
	//7. Send each public share to its destination
	err = ssHandler.distributePublicShares(publicShares)
	if err != nil {
		fmt.Println(err)
		return
	}
}

//HandlePasswordRetrieval starts recollection on shamir shares and finally recosntruction
func (ssHandler *SSHandler) HandlePasswordRetrieval(masterKey, account, username string) {
	//1. Assign Password UID and check if it exists and it is not currently being retrieved
	passwordUID, exists := ssHandler.passwordExists(masterKey, account, username)
	if !exists {
		fmt.Println("Incorrect credentials provided, please try again")
		return
	}
	if ssHandler.isDuplicate(passwordUID) {
		fmt.Println("Your password is currently being retrieved, please wait")
		return
	}

	//2. If yes, proceed by creating a search expanding ring using the uid
	ssHandler.storeTemporaryKey(masterKey)
	go ssHandler.initiateShareCollection(passwordUID)
	//3. Wait until the threshold of unique received shares is received
	//4. Decrypt each share generating key by kdf with the same parameters as above
	//5. Reconstruct secret
	//6. Decrypt reconstructed secret using as key the kdf with same parameters as above
	//7. Return password
}

//HandlePublicShare handles a new incoming public share; processes it if it is intendeed for the peer itself or forwards it if it is directed to another peer
func (ssHandler *SSHandler) HandlePublicShare(packet core.GossipPacket) {
	publicShare := packet.PublicSecretShare
	found, destinationIP := ssHandler.ctx.RetrieveDestinationRoute(publicShare.Destination)
	switch found {
	case -1:
		return
	case 0:
		ssHandler.processShare(*publicShare)
	default:
		if publicShare.HopLimit == 0 {
			return
		}
		publicShare.HopLimit--
		go ssHandler.ctx.SendPacketToPeer(core.GossipPacket{PublicSecretShare: publicShare}, destinationIP)
	}
}

func (ssHandler *SSHandler) processShare(publicShare core.PublicShare) error {
	//First check if the received public share is requested or sent to be stored
	if !publicShare.Requested {
		fmt.Println("Share stored: ", hex.EncodeToString( []byte(publicShare.UID)))
		ssHandler.storeShare(publicShare)
		//If not requested then check if this node is still awaiting for a password matching to the received share
	} else if passwordUID, awaiting := ssHandler.awaitingShare(publicShare); awaiting {
		//If such a password exists, open, verify and update status share with openShareAndUpdate
		err := ssHandler.openShareAndUpdate(passwordUID, ssHandler.tempKeyStorage, publicShare)
		if err != nil {
			fmt.Println(err)
			return err
		}
		//Check now if received shares for passwordUID meet the threshold.
		_, _, thresholdAchieved := ssHandler.thresholdAchieved(passwordUID)
		if thresholdAchieved {
			ssHandler.thresholdReached <- &passwordUID
		}
	}
	return nil
}

func (ssHandler *SSHandler) registerPassword(masterKey, account, username string) (string, error) {
	passwordUID, err := GetPasswordUID(masterKey, account, username)

	if err != nil {
		return "", err
	}
	ssHandler.ssLocker.Lock()
	defer ssHandler.ssLocker.Unlock()
	ssHandler.storedPasswords = append(ssHandler.storedPasswords, passwordUID)
	return passwordUID, nil
}
