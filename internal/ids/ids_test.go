package ids

import (
	"bytes"
	"testing"
)

func TestFrom(t *testing.T) {
	got, err := From("req", bytes.NewReader(make([]byte, 16)))
	if err != nil {
		t.Fatal(err)
	}
	if got != "req_00000000000000000000000000000000" {
		t.Fatalf("id = %q", got)
	}
}
