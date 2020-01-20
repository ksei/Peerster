package SecretSharing

import (
	"crypto/rand"
	"github.com/dedis/protobuf"
	"math/big"
	"errors"
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

	bytesShares := make([][]byte, Nshare)
	for s := range shares {
		bs, _ := protobuf.Encode(s)
		bytesShares = append(bytesShares, bs)
	}
	return bytesShares

}

func RecoverSecret(byteShares [][]byte, threshold int) ([]byte, error) {

	if len(byteShares) < threshold {
		return nil, errors.New("share: not enough shares to recover secret")
	}

	shares := make([]*Share, len(byteShares))
	for _,bs := range byteShares {
		var s *Share
		protobuf.Decode(bs,s)
		shares = append(shares, s)
	}


	accumulator :=big.NewInt(int64(0))
	var num *big.Int
	var denom *big.Int
	var temp *big.Int
	var xi *big.Int

	for i, si:= range shares{
		num.Set(si.y)
		denom=big.NewInt(int64(1))
		xi.SetInt64(int64(si.x))
		for j,sj := range shares {
			if i == j {
				continue
			}
			temp.SetInt64(int64(sj.x))
			num.Mul(num, temp)
			denom.Mul(denom, temp.Sub(xi, temp))
			num.Mod(num, mod)
			denom.Mod(denom, mod)
		}

		temp.ModInverse(denom, mod)
		temp.Mul(num, temp)
		accumulator.Add(accumulator,temp)
		accumulator.Mod(accumulator, mod)

	}

	return accumulator.Bytes(),nil
}
