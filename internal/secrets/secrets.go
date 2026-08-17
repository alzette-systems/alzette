package secrets

import (
	"io"
	"os"
	"strings"
)

const maximumSecretBytes = 64 << 10

// Lookup resolves an operator-controlled secret reference. A configured
// <REFERENCE>_FILE is authoritative and is read before the environment value;
// an unreadable or empty configured file fails closed instead of falling back.
func Lookup(reference string) (string, bool) {
	if path, configured := os.LookupEnv(reference + "_FILE"); configured {
		if path == "" {
			return "", false
		}
		file, err := os.Open(path)
		if err != nil {
			return "", false
		}
		defer file.Close()
		contents, err := io.ReadAll(io.LimitReader(file, maximumSecretBytes+1))
		if err != nil || len(contents) > maximumSecretBytes {
			return "", false
		}
		return valid(strings.TrimRight(string(contents), "\r\n"))
	}
	value, ok := os.LookupEnv(reference)
	if !ok {
		return "", false
	}
	return valid(value)
}

func valid(value string) (string, bool) {
	if value == "" || strings.ContainsAny(value, "\r\n") {
		return "", false
	}
	return value, true
}
