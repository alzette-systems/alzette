package humanauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const (
	SessionCookieName = "alzette_session"
	CSRFCookieName    = "alzette_csrf"
	generatedBytes    = 24
)

func HashPassword(password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}
	digest, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(digest), nil
}

func VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func ValidatePassword(password string) error {
	if len(password) < 16 || len(password) > 128 || strings.TrimSpace(password) != password {
		return errors.New("password must be 16 to 128 characters without surrounding whitespace")
	}
	return nil
}

func GeneratePassword() (string, error)     { return randomToken(generatedBytes) }
func GenerateSessionToken() (string, error) { return randomToken(32) }
func GenerateCSRFToken() (string, error)    { return randomToken(32) }

func Digest(token string) [32]byte { return sha256.Sum256([]byte(token)) }

func randomToken(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
