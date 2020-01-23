package SecretSharing

import (
	"fmt"
	"sync"

	core "github.com/ksei/Peerster/Core"
)

const (
	MIN_ITER   = 2000
	MAX_ITER   = 3000
	MIN_SHARES = 6
)

type SSHandler struct {
	ssLocker                sync.RWMutex
	ctx                     core.Context
	storedPasswords         []string
	tempKeyStorage          string
	extraInfo               map[string]*extraInfo
	thresholds              map[string]int
	hostedShares            map[string][]byte
	requestedPasswordStatus map[string]map[uint32][]byte
}

func (ssHandler *SSHandler) handlePasswordInsert(masterKey, account, username, newPassword string) {

	//1. Assign Password UID and check if already inserted (for now we can start without supporting password upates)
	if ssHandler.passwordExists(masterKey, account, username) {
		fmt.Println("Password has been already registered. Updates are not supported at the moment.")
		return
	}
	passwordUID, err := ssHandler.registerPassword(masterKey, account, username)
	if err != nil {
		fmt.Println(err)
		return
	}
	//2. Encrypt password using key derived by master key + account + username
	encryptedPass, err := ssHandler.encryptPassword(masterKey, account, username, passwordUID, newPassword)
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
	shares := GenerateShares(encryptedPass, totalShares, retrievingThreshold)
	//4. Assign each share to an origin from dsdv
	peerReplicateIndex, err := ssHandler.mapSharesToPeers(totalShares)
	if err != nil {
		fmt.Println(err)
		return
	}
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

func (ssHandler *SSHandler) handlePasswordRetrieval(masterKey, account, username string) {
	//1. Assign Password UID and check if it exists and it is not currently being retrieved
	if !ssHandler.passwordExists(masterKey, account, username) {
		fmt.Println("Incorrect credentials provided, please try again")
		return
	}
	passwordUID, err := GetPasswordUID(masterKey, account, username)
	if err != nil {
		fmt.Println("An error occured while processing your data")

		return
	}
	//2. If yes, proceed by creating a search expanding ring using the uid
	ssHandler.storeTemporaryKey(masterKey)

	//3. Wait until the threshold of unique received shares is received
	//we can do better than that
	for !ssHandler.thresholdAchieved(passwordUID){
		//wait
	}

	//4. Decrypt each share generating key by kdf with the same parameters as above
	shareslice := make([][]byte, len(ssHandler.requestedPasswordStatus[passwordUID]))
	for _, v := range ssHandler.requestedPasswordStatus[passwordUID] {
		shareslice = append(shareslice, /*DecryptShare( v )*/)
	}

	//5. Reconstruct secret
	secret, err := RecoverSecret(shareslice, ssHandler.thresholds[passwordUID])

	//6. Decrypt reconstructed secret using as key the kdf with same parameters as above
	clear_pass,err:=ssHandler.decryptPassword(masterKey, account, username, passwordUID, secret)
	if err != nil {
		fmt.Println("An error occured while decrypting your password")
		return
	}
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
		go ssHandler.processShare(*publicShare)
	default:
		if publicShare.HopLimit == 0 {
			return
		}
		publicShare.HopLimit--
		go ssHandler.ctx.SendPacketToPeer(core.GossipPacket{PublicSecretShare: publicShare}, destinationIP)
	}
}

func (ssHandler *SSHandler) processShare(publicShare core.PublicShare) error {
	//First check if the received public share belongs to a password being awaited
	if passwordUID, awaiting := ssHandler.awaitingShare(publicShare); awaiting {
		//If such a password exists, open, verify and update status share with openShareAndUpdate
		err := ssHandler.openShareAndUpdate(passwordUID, ssHandler.tempKeyStorage, publicShare)
		if err != nil {
			return err
		}
	} else { //If public share does not belong to shares we are waiting for, it means that share is to be hosted for another peer
		ssHandler.storeShare(publicShare)

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
