package agentauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"alzette/internal/federation"
)

type testValidator struct{}

func (testValidator) ValidateAccessToken(_ context.Context, raw string) (federation.Identity, error) {
	if raw != "oauth-access-token" {
		return federation.Identity{}, errors.New("invalid token")
	}
	return federation.Identity{Issuer: "https://identity.example.test", Subject: "employee|42", OAuthClientID: "agent-client"}, nil
}
func (testValidator) Issuer() string   { return "https://identity.example.test" }
func (testValidator) ClientID() string { return "agent-client" }

type testStore struct {
	contexts  []Context
	mintInput StoreMintInput
	revoked   bool
}

func (s *testStore) ListAgentContexts(context.Context, federation.Identity) ([]Context, error) {
	return append([]Context(nil), s.contexts...), nil
}

func (s *testStore) MintAgentCredential(_ context.Context, input StoreMintInput) (MintResult, error) {
	s.mintInput = input
	return MintResult{Context: s.contexts[0], GrantID: input.GrantID, AccessToken: input.Credential.Token, ExpiresAt: input.TokenExpiresAt, ModelAliases: input.ModelAliases}, nil
}

func (s *testStore) RevokeAgentGrant(context.Context, federation.Identity, string, [32]byte, time.Time) error {
	s.revoked = true
	return nil
}

func TestServiceMintsBoundShortCredential(t *testing.T) {
	now := time.Date(2026, 8, 18, 9, 0, 0, 0, time.UTC)
	store := &testStore{contexts: []Context{{MembershipID: "mem_1", Relationship: "employee", ModelAliases: []string{"alzette-chat", "summarise"}}}}
	service := New(store, testValidator{})
	service.SetClock(func() time.Time { return now })
	service.newID = func(prefix string) (string, error) { return prefix + "_test", nil }
	identity, err := service.Authenticate(context.Background(), "oauth-access-token")
	if err != nil {
		t.Fatal(err)
	}
	clientID := "aci_" + base64.RawURLEncoding.EncodeToString([]byte("0123456789abcdef"))
	idempotencyKey := "agm_" + base64.RawURLEncoding.EncodeToString([]byte("fedcba9876543210"))
	result, err := service.Mint(context.Background(), identity, MintInput{ClientInstanceID: clientID, MembershipID: "mem_1", ModelAliases: []string{"summarise", "alzette-chat", "summarise"}}, idempotencyKey)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(result.AccessToken, "alz_u_") || result.ExpiresAt != now.Add(10*time.Minute) || store.mintInput.GrantExpiresAt != now.Add(time.Hour) {
		t.Fatalf("result=%#v input=%#v", result, store.mintInput)
	}
	if got := strings.Join(store.mintInput.ModelAliases, ","); got != "alzette-chat,summarise" {
		t.Fatalf("normalised aliases=%q", got)
	}
	if store.mintInput.ClientInstanceDigest != sha256.Sum256([]byte(clientID)) || store.mintInput.IdempotencyKeyDigest != sha256.Sum256([]byte(idempotencyKey)) {
		t.Fatal("opaque client/idempotency values were not reduced to digests")
	}
	if result.AccessToken == string(store.mintInput.Credential.Digest[:]) {
		t.Fatal("plaintext credential was replaced with its digest")
	}
}

func TestHandlerRequiresOneOAuthBearerAndExactJSON(t *testing.T) {
	store := &testStore{contexts: []Context{{MembershipID: "mem_1", Relationship: "employee", ModelAliases: []string{"alzette-chat"}}}}
	service := New(store, testValidator{})
	handler := NewHandler(service, "https://control.example.test", "https://gateway.example.test")

	request := httptest.NewRequest(http.MethodGet, "/api/agent/contexts", nil)
	request.Header.Add("Authorization", "Bearer oauth-access-token")
	request.Header.Add("Authorization", "Bearer oauth-access-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("duplicate bearer status=%d headers=%v", response.Code, response.Header())
	}

	request = httptest.NewRequest(http.MethodGet, "/api/agent/contexts", nil)
	request.Header.Set("Authorization", "Bearer oauth-access-token")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "alzette-chat") {
		t.Fatalf("contexts status=%d body=%s", response.Code, response.Body.String())
	}

	body, _ := json.Marshal(map[string]interface{}{"client_instance_id": "aci_" + base64.RawURLEncoding.EncodeToString([]byte("0123456789abcdef")), "membership_id": "mem_1", "model_aliases": []string{"alzette-chat"}, "tenant_id": "forbidden"})
	request = httptest.NewRequest(http.MethodPost, "/api/agent/credentials", strings.NewReader(string(body)))
	request.Header.Set("Authorization", "Bearer oauth-access-token")
	request.Header.Set("Idempotency-Key", "agm_"+base64.RawURLEncoding.EncodeToString([]byte("fedcba9876543210")))
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || store.mintInput.MembershipID != "" {
		t.Fatalf("unknown field status=%d mint=%#v", response.Code, store.mintInput)
	}
}
