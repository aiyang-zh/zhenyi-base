// GM/T wire helpers for SM2 ciphertext ASN.1 used by GM-TLS (ported from tjfoc/gmsm/sm2, Apache-2.0).

package gmtls

import (
	"encoding/asn1"
	"errors"
	"math/big"
)

type sm2CipherASN1 struct {
	XCoordinate *big.Int
	YCoordinate *big.Int
	HASH        []byte
	CipherText  []byte
}

// cipherMarshalSM2Plain converts non-ASN.1 SM2 ciphertext (0x04 || X || Y || C3 || C2) to ASN.1.
func cipherMarshalSM2Plain(data []byte) ([]byte, error) {
	if len(data) < 1+32+32+32 {
		return nil, errors.New("sm2: ciphertext too short for GMTLS marshal")
	}
	if data[0] != 0x04 {
		return nil, errors.New("sm2: expected uncompressed point prefix 0x04")
	}
	data = data[1:]
	x := new(big.Int).SetBytes(data[:32])
	y := new(big.Int).SetBytes(data[32:64])
	hash := data[64:96]
	cipherText := data[96:]
	return asn1.Marshal(sm2CipherASN1{x, y, hash, cipherText})
}

// cipherUnmarshalSM2ToPlain converts ASN.1 ciphertext to 0x04 || X || Y || C3 || C2 for Decrypt.
func cipherUnmarshalSM2ToPlain(data []byte) ([]byte, error) {
	var cipher sm2CipherASN1
	_, err := asn1.Unmarshal(data, &cipher)
	if err != nil {
		return nil, err
	}
	x := cipher.XCoordinate.Bytes()
	y := cipher.YCoordinate.Bytes()
	hash := cipher.HASH
	cipherText := cipher.CipherText
	pad32 := func(b []byte) []byte {
		if n := len(b); n < 32 {
			p := make([]byte, 32-n)
			b = append(p, b...)
		}
		return b
	}
	x = pad32(x)
	y = pad32(y)
	c := make([]byte, 0, 1+32+32+len(hash)+len(cipherText))
	c = append(c, 0x04)
	c = append(c, x...)
	c = append(c, y...)
	c = append(c, hash...)
	c = append(c, cipherText...)
	return c, nil
}
