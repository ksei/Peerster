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
	sentTo string
	share  []byte
}

//NewSecret instantiates a new SecretShare
func NewSecret(receiverOrigin string, shareToAdd []byte) *SecretShare {
	share := &SecretShare{
		sentTo: receiverOrigin,
		share:  shareToAdd,
	}
	return share
}

//toBytes converts a secretShare struct into bytes
func (ss *SecretShare) toBytes() ([]byte, error) {
	shareBytes, err := protobuf.Encode(&ss)
	if err != nil {
		return nil, err
	}

	return shareBytes, nil
}
