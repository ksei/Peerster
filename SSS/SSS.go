package SSS
import(
  "math/big"
  "crypto/rand"
)

type Share struct{
  x int
  y *big.Int
}

var mod *big.Int


func GenerateShares(secret *big.Int,Nshare int,threshold_k int) []*Share {
  coeffs := make([]*big.Int, threshold_k)

  coeffs[0] = secret

	for i := 1; i < threshold_k; i++ {
		coeffs[i],_ = rand.Int(rand.Reader, mod)
  }

  shares := make([]*Share, Nshare)
  for i := 0; i < Nshare; i++ {
		shares[i].x=i+1
    xi := big.NewInt(int64(shares[i].x))
    shares[i].y=big.NewInt(int64(0))
    for j := threshold_k - 1; j > -1; j-- {
  		shares[i].y.Mul(shares[i].y, xi)
  		shares[i].y.Add(shares[i].y, coeffs[j])
    }
  }

return shares

}
