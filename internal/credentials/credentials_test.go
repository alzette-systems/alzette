package credentials

import (
	"bytes"
	"testing"
)

func TestGenerateFormatAndHash(t *testing.T) {
	key, err := GenerateFrom(bytes.NewReader(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateFormat(key.Token); err != nil {
		t.Fatalf("generated token is invalid: %v", err)
	}
	if key.Digest != Digest(key.Token) {
		t.Fatal("generated digest does not match token")
	}
	if key.Prefix == key.Token || len(key.Prefix) >= len(key.Token) {
		t.Fatal("stored prefix reveals the key")
	}
}

func TestValidateFormatRejectsMalformedTokens(t *testing.T) {
	for _, token := range []string{"", "secret", "alz_k_short.value", "alz_k_0000000000000000.not-base64"} {
		if err := ValidateFormat(token); err == nil {
			t.Errorf("ValidateFormat(%q) unexpectedly succeeded", token)
		}
	}
}
