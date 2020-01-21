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

func (ssHandler *SSHandler) encryptPassword(masterKey, account, username, passwordUID, newPassword string) ([]byte, error) {
	key, salt, err := KDF(masterKey, strings.Join([]string{account, username}, ""))
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

func (ssHandler *SSHandler) encryptShares(masterKey, passwordUID string, replicateIndex map[string]uint32, shares [][]byte) ([]*core.PublicShare, error) {
	var publicShares = []*core.PublicShare{}
	for origin, index := range replicateIndex {

		shareUID, err := GetShareUID(passwordUID, origin)
		if err != nil {
			return nil, err
		}

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

		publicShares = append(publicShares, ssHandler.NewPublic(origin, shareUID, encryptedShare))
		ssHandler.storeExtraInfo(shareUID, salt, nonce)
	}

	return publicShares, nil
}

func GetPasswordUID(masterKey, account, username string) (string, error) {
	uidBytes, err := bcrypt.GenerateFromPassword([]byte(masterKey+account+username), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(uidBytes), nil
}

func GetShareUID(passwordUID, origin string) (string, error) {
	uidBytes, err := bcrypt.GenerateFromPassword([]byte(strings.Join([]string{passwordUID, origin}, "")), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	shareUID := string(uidBytes)

	return shareUID, nil

}

func (ssHandler *SSHandler) openShareAndUpdate(passwordUID, masterKey string, publicShare core.PublicShare) error {
	shareUID := publicShare.UID
	sender := publicShare.Origin
	encryptedSecretBytes := publicShare.SecuredShare
	ssHandler.ssLocker.RLock()
	extraInfo, exists := ssHandler.extraInfo[shareUID]
	ssHandler.ssLocker.RUnlock()

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

	if strings.Compare(sender, secretShare.sentTo) != 0 {
		return errors.New("Malicious share received")
	}

	ssHandler.ssLocker.Lock()
	defer ssHandler.ssLocker.Unlock()
	if _, exists := ssHandler.requestedPasswordStatus[passwordUID][secretShare.replicateID]; !exists {
		ssHandler.requestedPasswordStatus[passwordUID][secretShare.replicateID] = secretShare.share
	}
	return nil
}
