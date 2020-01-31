/*
Created and Developed by: Ksandros Apostoli
Part of the course project for Decentralized System Engineering
*/
package SecretSharing

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
	"strings"

	core "github.com/ksei/Peerster/Core"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/hkdf"
)

type extraInfo struct {
	Salt  []byte
	Nonce []byte
}

func (ssHandler *SSHandler) storeExtraInfo(uid string, salt, nonce []byte) error {
	info := &extraInfo{
		Salt:  salt,
		Nonce: nonce,
	}

	ssHandler.ssLocker.Lock()
	defer ssHandler.ssLocker.Unlock()

	if _, exists := ssHandler.extraInfo[uid]; exists {
		return errors.New("Password exists")
	}

	ssHandler.extraInfo[uid] = info
	return nil
}

//KDF - KeyDistribution Function generates a unique and random encryption key provided a master key and passwordUID extra info
func KDF(masterKey, extraInfo string) ([]byte, []byte, error) {
	hash := sha256.New

	secret := []byte(masterKey)
	// Non-secret salt, of length equal to hash.
	salt := make([]byte, hash().Size())
	if _, err := rand.Read(salt); err != nil {
		return nil, nil, err
	}

	// Non-secret context info
	info := []byte(extraInfo)

	// Generate 128-bit derived key.
	hkdf := hkdf.New(hash, secret, salt, info)
	key := make([]byte, 16)
	if _, err := io.ReadFull(hkdf, key); err != nil {
		return nil, nil, err
	}

	return key, salt, nil
}

//RecoverKeyKDF recomputes the ky by the KDF based on a master key and passwordUID
func RecoverKeyKDF(masterKey string, salt, info []byte) ([]byte, error) {
	hash := sha256.New
	secret := []byte(masterKey)

	// Generate 128-bit derived key.
	hkdf := hkdf.New(hash, secret, salt, info)
	key := make([]byte, 16)
	if _, err := io.ReadFull(hkdf, key); err != nil {
		return nil, err
	}

	return key, nil
}

//Enc is an interface method for encrypting a plaintext in byte format using the crypto.aes cipher with a provided key
func Enc(key, plaintext []byte) ([]byte, []byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}

	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}

	ciphertext := aesgcm.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

//Dec is an interface method for decrypting a ciphertext in byte format using the crypto.aes cipher with a provided key
func Dec(key, ciphertext, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}

func (ssHandler *SSHandler) encryptPassword(masterKey, passwordUID, newPassword string) ([]byte, error) {
	key, salt, err := KDF(masterKey, passwordUID)
	if err != nil {
		return nil, err
	}

	encryptedPassword, nonce, err := Enc(key, []byte(newPassword))
	if err != nil {
		return nil, err
	}
	ssHandler.storeExtraInfo(passwordUID, salt, nonce)
	return encryptedPassword, nil

}

//to debug and check
func (ssHandler *SSHandler) decryptPassword(passwordUID string, encryptedPassword []byte) ([]byte, error) {
	ssHandler.ssLocker.RLock()
	extra, foundExtra := ssHandler.extraInfo[passwordUID]
	masterKey := ssHandler.tempKeyStorage
	ssHandler.ssLocker.RUnlock()
	if !foundExtra || strings.Compare(masterKey, "") == 0 {
		return nil, errors.New("Error while decrypting password")
	}
	key, err := RecoverKeyKDF(masterKey, extra.Salt, []byte(passwordUID))

	if err != nil {
		return nil, err
	}

	clearPassword, err := Dec(key, encryptedPassword, extra.Nonce)
	if err != nil {
		return nil, err
	}

	return clearPassword, nil
}

func (ssHandler *SSHandler) encryptShares(masterKey, passwordUID string, replicateIndex map[string]uint32, shares []*Share) ([]*core.PublicShare, error) {
	var publicShares = []*core.PublicShare{}
	for origin, index := range replicateIndex {

		shareUID := GetShareUID(passwordUID, origin)

		key, salt, err := KDF(masterKey, shareUID)
		if err != nil {
			return nil, err
		}

		secretShare := NewSecret(origin, index, shares[index])
		shareBytes, err := secretShare.toBytes()
		if err != nil {
			return nil, err
		}
		encryptedShare, nonce, err := Enc(key, shareBytes)
		if err != nil {
			return nil, err
		}

		publicShares = append(publicShares, ssHandler.NewPublic(shareUID, origin, encryptedShare))
		ssHandler.storeExtraInfo(shareUID, salt, nonce)
		ssHandler.updateConfirmationMap(origin, shareUID)
	}

	return publicShares, nil
}

//GetPasswordUID computer a passwordUID uniquely based on the masterKey, account name and username
func GetPasswordUID(masterKey, account, username string) (string, error) {
	uidBytes, err := bcrypt.GenerateFromPassword([]byte(masterKey+account+username), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(uidBytes), nil
}

//GetShareUID provides a unique ID for each computed share hashing the passwordUID together with the designated destination of the share
func GetShareUID(passwordUID, origin string) string {
	uidBytes := sha256.Sum256([]byte(strings.Join([]string{passwordUID, origin}, "")))
	shareUID := string(uidBytes[:])
	return shareUID

}

func (ssHandler *SSHandler) openShareAndUpdate(passwordUID, masterKey string, publicShare core.PublicShare) error {
	ssHandler.ssLocker.Lock()
	defer ssHandler.ssLocker.Unlock()
	shareUID := publicShare.UID
	sender := publicShare.Origin
	encryptedSecretBytes := publicShare.SecuredShare
	extraInfo, exists := ssHandler.extraInfo[shareUID]

	if !exists {
		return errors.New("Could not find share information")
	}

	key, err := RecoverKeyKDF(masterKey, extraInfo.Salt, []byte(shareUID))
	if err != nil {
		return errors.New("Could not open share")
	}
	secretBytes, err := Dec(key, encryptedSecretBytes, extraInfo.Nonce)
	if err != nil {
		return errors.New("Could not open share" + err.Error())
	}

	secretShare, err := fromBytes(secretBytes)
	if err != nil {
		return errors.New("Error while decoding share" + err.Error())
	}

	if strings.Compare(sender, secretShare.SentTo) != 0 {
		return errors.New("Malicious share received")
	}

	if _, exists := ssHandler.requestedPasswordStatus[passwordUID]; !exists {
		return nil
	}

	if _, exists := ssHandler.requestedPasswordStatus[passwordUID][secretShare.ReplicateID]; !exists {
		ssHandler.requestedPasswordStatus[passwordUID][secretShare.ReplicateID] = secretShare.Share
	}
	return nil
}
