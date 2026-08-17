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

const tokenMarker = "alz_k_"

type Key struct {
	Token  string
	Prefix string
	Digest [32]byte
}

func Generate() (Key, error) {
	return GenerateFrom(rand.Reader)
}

func GenerateFrom(source io.Reader) (Key, error) {
	public := make([]byte, 8)
	secret := make([]byte, 24)
	if _, err := io.ReadFull(source, public); err != nil {
		return Key{}, err
	}
	if _, err := io.ReadFull(source, secret); err != nil {
		return Key{}, err
	}
	prefix := tokenMarker + hex.EncodeToString(public)
	token := prefix + "." + base64.RawURLEncoding.EncodeToString(secret)
	return Key{Token: token, Prefix: prefix, Digest: Digest(token)}, nil
}

func Digest(token string) [32]byte {
	return sha256.Sum256([]byte(token))
}

func ValidateFormat(token string) error {
	if len(token) != len(tokenMarker)+16+1+32 || !strings.HasPrefix(token, tokenMarker) {
		return errors.New("invalid API key format")
	}
	separator := len(tokenMarker) + 16
	if token[separator] != '.' {
		return errors.New("invalid API key format")
	}
	if _, err := hex.DecodeString(token[len(tokenMarker):separator]); err != nil {
		return errors.New("invalid API key format")
	}
	secret, err := base64.RawURLEncoding.DecodeString(token[separator+1:])
	if err != nil || len(secret) != 24 {
		return errors.New("invalid API key format")
	}
	return nil
}
