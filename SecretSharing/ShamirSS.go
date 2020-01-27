package SecretSharing

import (
	"crypto/rand"
	"errors"
	"math/big"
)

type Share struct {
	X int
	Y string
}

//Field size represents the size of the Galois Field used for share generation, and corresponds to secret size
const fieldSize string = "120108331817880498208402974220997642361982874305813858034375860017575261604401803468781848586309259637974588193404514898845697031421429112969955000482498183066089084683313836043275289589999238343502260701561985449399552710349015326719334210697851899957993043127977596076870075883832140454467364356765767338229"

//GenerateShares outputs Nshares, k = threshold out of which are enough for reconstruction of the secret
func GenerateShares(secret []byte, Nshare int, threshold int) ([]*Share, error) {
	mod, ok := new(big.Int).SetString(fieldSize, 10)
	if !ok {
		return nil, errors.New("Could not process modulus into integer")
	}
	secretint := new(big.Int).SetBytes(secret)

	coeffs := make([]*big.Int, threshold)
	coeffs[0] = secretint
	for i := 1; i < threshold; i++ {
		coeffs[i], _ = rand.Int(rand.Reader, mod)
	}

	shares := make([]*Share, Nshare)
	for i := 0; i < Nshare; i++ {
		newShare := &Share{}
		newShare.X = i + 1
		xi := big.NewInt(int64(i + 1))
		Y := big.NewInt(int64(0))
		Y.Add(Y, coeffs[0])
		xtemp := new(big.Int)
		for j := 1; j < threshold; j++ {
			xtemp.Exp(xi, big.NewInt(int64(j)), mod)
			xtemp.Mul(xtemp, coeffs[j])
			Y.Add(Y, xtemp)
			Y.Mod(Y, mod)
		}
		newShare.Y = Y.String()
		shares[i] = newShare
	}

	return shares, nil

}

//RecoverSecret reconstructs a secret given a threshold of shares
func RecoverSecret(shares []*Share, threshold int) ([]byte, error) {
	mod, ok := new(big.Int).SetString(fieldSize, 10)
	if !ok {
		return nil, errors.New("Could not process modulus into integer")
	}
	if len(shares) < threshold {
		return nil, errors.New("share: not enough shares to recover secret")
	}

	accumulator := big.NewInt(int64(0))
	nominator := new(big.Int)
	denom := new(big.Int)
	temp := new(big.Int)
	xj := new(big.Int)
	xi := new(big.Int)
	for i, si := range shares {
		Y, ok := new(big.Int).SetString(si.Y, 10)
		if !ok {
			return nil, errors.New("Could not process received value into integer")
		}
		nominator = big.NewInt(int64(1))
		denom = big.NewInt(int64(1))
		xi.SetInt64(int64(si.X))
		for j, sj := range shares {
			if i == j {
				continue
			}
			xj.SetInt64(int64(sj.X))
			nominator.Mul(nominator, xj)
			nominator.Mod(nominator, mod)
			denom.Mul(denom, temp.Sub(xj, xi))
			denom.Mod(denom, mod)
		}

		temp.ModInverse(denom, mod)
		temp.Mul(nominator, temp)
		temp.Mul(temp, Y)
		accumulator.Add(accumulator, temp)
		accumulator.Mod(accumulator, mod)
	}
	return accumulator.Bytes(), nil
}
