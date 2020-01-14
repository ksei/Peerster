package SecretSharing

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"io"
	"strings"

	core "github.com/ksei/Peerster/Core"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/hkdf"
)

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
	ssHandler.storeSalt(passwordUID, salt)
	ssHandler.storeNonce(passwordUID, nonce)

	return encryptedPassword, nil

}

func (ssHandler *SSHandler) encryptShares(masterKey, account, username string, sharesToPeers map[string][]byte) ([]*core.PublicShare, error) {
	var publicShares = []*core.PublicShare{}
	for origin, share := range sharesToPeers {
		key, salt, err := KDF(masterKey, strings.Join([]string{account, username, origin}, ""))
		if err != nil {
			return nil, err
		}

		uidBytes, err := bcrypt.GenerateFromPassword([]byte(masterKey+account+username+origin), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		shareUID := string(uidBytes)

		secretShare := NewSecret(origin, share)
		shareBytes, err := secretShare.toBytes()
		if err != nil {
			return nil, err
		}
		encryptedShare, nonce, err := Enc(key, shareBytes)
		if err != nil {
			return nil, err
		}

		publicShares = append(publicShares, ssHandler.NewPublic(origin, shareUID, encryptedShare))
		ssHandler.storeSalt(shareUID, salt)
		ssHandler.storeNonce(shareUID, nonce)
	}
	return publicShares, nil
}
