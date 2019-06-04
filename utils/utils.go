package utils

import (
	"encoding/json"
	"math/big"
)

// BigIntToString type convert
func BigIntToString(b *big.Int) string {
	if b == nil {
		return "0"
	}
	return b.String()
}

// StringToBigInt type convert
func StringToBigInt(s string) *big.Int {
	bi, b := new(big.Int).SetString(s, 10)
	if !b {
		bi = new(big.Int)
	}
	return bi
}

// ToJSONStringFormat for log
func ToJSONStringFormat(v interface{}) string {
	buf, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		panic(err)
	}
	return string(buf)
}
