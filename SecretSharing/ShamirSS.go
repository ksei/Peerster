package SecretSharing

import (
	"crypto/rand"
	"errors"
	"math/big"

	"github.com/dedis/protobuf"
)

type share struct {
	x int
	y *big.Int
}

//Field size represents the size of the Galois Field used for share generation, and corresponds to secret size
const fieldSize string = "18446744073709551557"

//GenerateShares outputs Nshares, k = threshold out of which are enough for reconstruction of the secret
func GenerateShares(secret []byte, Nshare int, threshold int) [][]byte {
	mod, _ := new(big.Int).SetString(fieldSize, 10)
	secretint := new(big.Int).SetBytes(secret)

	coeffs := make([]*big.Int, threshold)
	coeffs[0] = secretint

	for i := 1; i < threshold; i++ {
		coeffs[i], _ = rand.Int(rand.Reader, mod)
	}

	shares := make([]*share, Nshare)
	for i := 0; i < Nshare; i++ {
		newShare := &share{}
		newShare.x = i + 1
		xi := big.NewInt(int64(i + 1))
		newShare.y = big.NewInt(int64(0))
		newShare.y.Add(newShare.y, coeffs[0])
		xtemp := new(big.Int)
		for j := 1; j < threshold; j++ {
			xtemp.Exp(xi, big.NewInt(int64(j)), mod)
			xtemp.Mul(xtemp, coeffs[j])
			newShare.y.Add(newShare.y, xtemp)
			newShare.y.Mod(newShare.y, mod)
		}
		shares[i] = newShare
	}

	bytesShares := make([][]byte, Nshare)
	for s := range shares {
		bs, _ := protobuf.Encode(s)
		bytesShares = append(bytesShares, bs)
	}

	return bytesShares

}

//RecoverSecret reconstructs a secret given a threshold of shares
func RecoverSecret(byteShares [][]byte, threshold int) ([]byte, error) {
	mod, _ := new(big.Int).SetString(fieldSize, 10)

	if len(byteShares) < threshold {
		return nil, errors.New("share: not enough shares to recover secret")
	}

	shares := make([]*share, len(byteShares))
	for _, bs := range byteShares {
		var s *share
		protobuf.Decode(bs, s)
		shares = append(shares, s)
	}

	accumulator := big.NewInt(int64(0))
	nominator := new(big.Int)
	denom := new(big.Int)
	temp := new(big.Int)
	xj := new(big.Int)
	xi := new(big.Int)
	for i, si := range shares {
		nominator = big.NewInt(int64(1))
		denom = big.NewInt(int64(1))
		xi.SetInt64(int64(si.x))
		for j, sj := range shares {
			if i == j {
				continue
			}
			xj.SetInt64(int64(sj.x))
			nominator.Mul(nominator, xj)
			nominator.Mod(nominator, mod)
			denom.Mul(denom, temp.Sub(xj, xi))
			denom.Mod(denom, mod)
		}

		temp.ModInverse(denom, mod)
		temp.Mul(nominator, temp)
		temp.Mul(temp, si.y)
		accumulator.Add(accumulator, temp)
		accumulator.Mod(accumulator, mod)
	}
	return accumulator.Bytes(), nil
}
