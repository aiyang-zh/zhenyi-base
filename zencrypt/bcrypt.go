package zencrypt

import (
	"encoding/base64"

	"golang.org/x/crypto/bcrypt"
)

type Bcrypt struct{}

func NewBcrypt() *Bcrypt {
	return &Bcrypt{}
}

func (b *Bcrypt) Encrypt(password string) (string, error) {
	hashPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(hashPassword), nil
}
func (b *Bcrypt) CompareHashAndPassword(password, hashPassword string) bool {
	hashPasswords, err := base64.RawStdEncoding.DecodeString(hashPassword)
	if err != nil {
		return false
	}
	err = bcrypt.CompareHashAndPassword(hashPasswords, []byte(password))
	return err == nil
}
