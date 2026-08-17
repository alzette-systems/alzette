package humanauth

import "testing"

func TestPasswordAndOpaqueTokens(t *testing.T) {
	password, err := GeneratePassword()
	if err != nil {
		t.Fatal(err)
	}
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if hash == password || !VerifyPassword(hash, password) || VerifyPassword(hash, password+"x") {
		t.Fatal("bcrypt password lifecycle failed")
	}
	first, err := GenerateSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	second, err := GenerateSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if first == second || Digest(first) == Digest(second) {
		t.Fatal("session tokens were not independently random")
	}
}
