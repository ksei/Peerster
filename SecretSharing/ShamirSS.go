package SecretSharing

import (
	"crypto/rand"
	"math/big"
	"github.com/dedis/protobuf"
)

type Share struct {
	x int
	y *big.Int
}

var mod *big.Int

func GenerateShares(secret []byte, Nshare int, threshold_k int) [][]byte {

	var secretint *big.Int
	secretint.SetBytes(secret)

	coeffs := make([]*big.Int, threshold_k)

	coeffs[0] = secretint

	for i := 1; i < threshold_k; i++ {
		coeffs[i], _ = rand.Int(rand.Reader, mod)
	}

	shares := make([]*Share, Nshare)
	for i := 0; i < Nshare; i++ {
		shares[i].x = i + 1
		xi := big.NewInt(int64(shares[i].x))
		shares[i].y = big.NewInt(int64(0))
		for j := threshold_k - 1; j > -1; j-- {
			shares[i].y.Mul(shares[i].y, xi)
			shares[i].y.Add(shares[i].y, coeffs[j])
      shares[i].y.Mod(shares[i].y, mod)
		}
	}

	bytesShares:=make([][]byte, Nshare)
	for s:= range shares{
		bs,_:=protobuf.Encode(s)
		bytesShares=append(bytesShares,  bs)
	}
	return bytesShares

}
