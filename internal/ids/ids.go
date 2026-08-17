package ids

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
)

func New(prefix string) (string, error) {
	return From(prefix, rand.Reader)
}

func From(prefix string, source io.Reader) (string, error) {
	value := make([]byte, 16)
	if _, err := io.ReadFull(source, value); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(value)), nil
}
