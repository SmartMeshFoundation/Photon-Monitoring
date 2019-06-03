package utils

import (
	"encoding/json"
	"math/big"
)

// BigIntToString :
func BigIntToString(b *big.Int) string {
	if b == nil {
		return "0"
	}
	return b.String()
}

// StringToBigInt :
func StringToBigInt(s string) *big.Int {
	bi, b := new(big.Int).SetString(s, 10)
	if !b {
		bi = new(big.Int)
	}
	return bi
}

// ToJSONStringFormat :
func ToJSONStringFormat(v interface{}) string {
	buf, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		panic(err)
	}
	return string(buf)
}
