package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLookupPrefersFileAndTrimsOnlyLineEnding(t *testing.T) {
	path := filepath.Join(t.TempDir(), "provider-key")
	if err := os.WriteFile(path, []byte(" file-secret \r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_PROVIDER_SECRET", "environment-secret")
	t.Setenv("TEST_PROVIDER_SECRET_FILE", path)
	value, ok := Lookup("TEST_PROVIDER_SECRET")
	if !ok || value != " file-secret " {
		t.Fatalf("file-backed lookup mismatch; available=%t length=%d", ok, len(value))
	}
}

func TestLookupFallsBackToEnvironmentWhenFileIsNotConfigured(t *testing.T) {
	t.Setenv("TEST_PROVIDER_ENV_ONLY", "environment-secret")
	value, ok := Lookup("TEST_PROVIDER_ENV_ONLY")
	if !ok || value != "environment-secret" {
		t.Fatalf("environment lookup mismatch; available=%t length=%d", ok, len(value))
	}
}

func TestLookupConfiguredFileFailsClosed(t *testing.T) {
	t.Setenv("TEST_PROVIDER_BAD_FILE", "must-not-be-used")
	t.Setenv("TEST_PROVIDER_BAD_FILE_FILE", filepath.Join(t.TempDir(), "missing"))
	if value, ok := Lookup("TEST_PROVIDER_BAD_FILE"); ok || value != "" {
		t.Fatalf("configured missing file did not fail closed; available=%t length=%d", ok, len(value))
	}

	empty := filepath.Join(t.TempDir(), "empty")
	if err := os.WriteFile(empty, nil, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_PROVIDER_EMPTY_FILE_FILE", empty)
	if _, ok := Lookup("TEST_PROVIDER_EMPTY_FILE"); ok {
		t.Fatal("empty secret file was accepted")
	}
}

func TestLookupRejectsEmbeddedNewline(t *testing.T) {
	t.Setenv("TEST_PROVIDER_HEADER", "secret\ninjection")
	if _, ok := Lookup("TEST_PROVIDER_HEADER"); ok {
		t.Fatal("header-unsafe secret was accepted")
	}
}
