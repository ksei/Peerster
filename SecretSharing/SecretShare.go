/*
Created and Developed by: Ksandros Apostoli
Part of the course project for Decentralized System Engineering
*/
package SecretSharing

import (
	"github.com/dedis/protobuf"
)

/*SecretShare represents a tuple of:
- sentTo: Origin of the peer who is receiving the share
- share: a byte array which contains the actual share
! Note that this data structure must always be encrypted before being sent
*/
type SecretShare struct {
	SentTo      string
	ReplicateID uint32
	Share       *Share
}

//NewSecret instantiates a new SecretShare
func NewSecret(receiverOrigin string, repID uint32, shareToAdd *Share) *SecretShare {
	share := &SecretShare{
		SentTo:      receiverOrigin,
		ReplicateID: repID,
		Share:       shareToAdd,
	}
	return share
}

//toBytes converts a secretShare struct into bytes
func (ss *SecretShare) toBytes() ([]byte, error) {
	shareBytes, err := protobuf.Encode(ss)
	if err != nil {
		return nil, err
	}

	return shareBytes, nil
}

//fromBytes parses a secretShare from given bytes
func fromBytes(inputBytes []byte) (*SecretShare, error) {
	outputSecretShare := SecretShare{}
	err := protobuf.Decode(inputBytes, &outputSecretShare)
	if err != nil {
		return nil, err
	}
	return &outputSecretShare, nil
}
