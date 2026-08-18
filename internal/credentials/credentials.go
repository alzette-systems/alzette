package credentials

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"strings"
)

const (
	tokenMarker      = "alz_k_"
	humanTokenMarker = "alz_u_"
)

type Key struct {
	Token  string
	Prefix string
	Digest [32]byte
}

func Generate() (Key, error) {
	return GenerateFrom(rand.Reader)
}

func GenerateHuman() (Key, error) {
	return generateFrom(rand.Reader, humanTokenMarker)
}

func GenerateFrom(source io.Reader) (Key, error) {
	return generateFrom(source, tokenMarker)
}

func generateFrom(source io.Reader, marker string) (Key, error) {
	public := make([]byte, 8)
	secret := make([]byte, 24)
	if _, err := io.ReadFull(source, public); err != nil {
		return Key{}, err
	}
	if _, err := io.ReadFull(source, secret); err != nil {
		return Key{}, err
	}
	prefix := marker + hex.EncodeToString(public)
	token := prefix + "." + base64.RawURLEncoding.EncodeToString(secret)
	return Key{Token: token, Prefix: prefix, Digest: Digest(token)}, nil
}

func Digest(token string) [32]byte {
	return sha256.Sum256([]byte(token))
}

func ValidateFormat(token string) error {
	return validateFormat(token, tokenMarker)
}

func ValidateHumanFormat(token string) error {
	return validateFormat(token, humanTokenMarker)
}

func validateFormat(token, marker string) error {
	if len(token) != len(marker)+16+1+32 || !strings.HasPrefix(token, marker) {
		return errors.New("invalid API key format")
	}
	separator := len(marker) + 16
	if token[separator] != '.' {
		return errors.New("invalid API key format")
	}
	if _, err := hex.DecodeString(token[len(marker):separator]); err != nil {
		return errors.New("invalid API key format")
	}
	secret, err := base64.RawURLEncoding.DecodeString(token[separator+1:])
	if err != nil || len(secret) != 24 {
		return errors.New("invalid API key format")
	}
	return nil
}
