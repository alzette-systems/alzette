package federation

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

func TestOIDCClientUsesPKCEAndValidatesExactIdentityAndNonce(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	verifier := oauth2.GenerateVerifier()
	nonce := oauth2.GenerateVerifier()
	var server *httptest.Server
	var accessToken string
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			writeTestJSON(w, map[string]interface{}{"issuer": server.URL, "authorization_endpoint": server.URL + "/authorize", "token_endpoint": server.URL + "/token", "jwks_uri": server.URL + "/jwks", "introspection_endpoint": server.URL + "/introspect", "id_token_signing_alg_values_supported": []string{"RS256"}})
		case "/jwks":
			writeTestJSON(w, map[string]interface{}{"keys": []interface{}{map[string]string{"kty": "RSA", "kid": "test-key", "use": "sig", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes())}}})
		case "/token":
			if err := r.ParseForm(); err != nil || r.PostForm.Get("code") != "one-use-code" || r.PostForm.Get("code_verifier") != verifier || r.PostForm.Get("grant_type") != "authorization_code" {
				http.Error(w, "bad exchange", http.StatusBadRequest)
				return
			}
			now := time.Now().UTC()
			claims := map[string]interface{}{"iss": server.URL, "sub": "employee|42", "aud": "alzette-portal", "exp": now.Add(5 * time.Minute).Unix(), "iat": now.Unix(), "nonce": nonce, "email": "Employee@Example.TEST", "email_verified": true, "name": "Employee Person"}
			writeTestJSON(w, map[string]interface{}{"access_token": "not-persisted", "token_type": "Bearer", "expires_in": 300, "id_token": signTestJWT(t, key, claims)})
		case "/introspect":
			clientID, clientSecret, ok := r.BasicAuth()
			if !ok || clientID != "alzette-portal" || clientSecret != "test-secret" || r.ParseForm() != nil || r.PostForm.Get("token") != accessToken || r.PostForm.Get("token_type_hint") != "access_token" {
				http.Error(w, "bad introspection", http.StatusUnauthorized)
				return
			}
			writeTestJSON(w, map[string]interface{}{"active": true, "client_id": "alzette-portal", "sub": "employee|42", "token_type": "Bearer", "exp": time.Now().Add(5 * time.Minute).Unix()})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	ctx := oidc.ClientContext(context.Background(), server.Client())
	ctx = context.WithValue(ctx, oauth2.HTTPClient, server.Client())
	client, err := New(ctx, Config{Issuer: server.URL, ClientID: "alzette-portal", ClientSecret: "test-secret", RedirectURL: "https://portal.example.test/login/oidc/callback", SignupURL: server.URL + "/signup/oauth/authorize", AllowInsecure: true})
	if err != nil {
		t.Fatal(err)
	}
	authorizationURL, err := url.Parse(client.AuthorizationURL("state-value", nonce, verifier))
	if err != nil {
		t.Fatal(err)
	}
	query := authorizationURL.Query()
	challenge := sha256.Sum256([]byte(verifier))
	if query.Get("state") != "state-value" || query.Get("nonce") != nonce || query.Get("code_challenge_method") != "S256" || query.Get("code_challenge") != base64.RawURLEncoding.EncodeToString(challenge[:]) {
		t.Fatalf("authorization query=%v", query)
	}
	signupURL, err := url.Parse(client.SignupURL("signup-state", nonce, verifier))
	if err != nil || signupURL.Path != "/signup/oauth/authorize" || signupURL.Query().Get("state") != "signup-state" || signupURL.Query().Get("code_challenge") != base64.RawURLEncoding.EncodeToString(challenge[:]) {
		t.Fatalf("signup URL=%v error=%v", signupURL, err)
	}
	identity, err := client.Exchange(ctx, "one-use-code", verifier, nonce)
	if err != nil {
		t.Fatal(err)
	}
	if identity.Issuer != server.URL || identity.Subject != "employee|42" || identity.Email != "employee@example.test" || !identity.EmailVerified || identity.DisplayName != "Employee Person" {
		t.Fatalf("identity=%#v", identity)
	}
	if _, err := client.Exchange(ctx, "one-use-code", verifier, "wrong-nonce"); err == nil || !strings.Contains(err.Error(), "nonce") {
		t.Fatalf("wrong nonce error=%v", err)
	}
	idToken := signTestJWT(t, key, map[string]interface{}{"iss": server.URL, "sub": "employee|42", "aud": "alzette-portal", "exp": time.Now().Add(5 * time.Minute).Unix(), "iat": time.Now().Unix()})
	if _, err := client.ValidateAccessToken(ctx, idToken); err == nil || !strings.Contains(err.Error(), "not an access token") {
		t.Fatalf("ID token accepted as access token: %v", err)
	}
	accessToken = signTestJWT(t, key, map[string]interface{}{"iss": server.URL, "sub": "employee|42", "aud": "alzette-portal", "exp": time.Now().Add(5 * time.Minute).Unix(), "iat": time.Now().Unix(), "tokenType": "access-token"})
	accessIdentity, err := client.ValidateAccessToken(ctx, accessToken)
	if err != nil {
		t.Fatal(err)
	}
	if accessIdentity.Issuer != server.URL || accessIdentity.Subject != "employee|42" || accessIdentity.OAuthClientID != "alzette-portal" {
		t.Fatalf("access identity=%#v", accessIdentity)
	}
	if _, err := New(ctx, Config{Issuer: server.URL, ClientID: "alzette-portal", ClientSecret: "test-secret", RedirectURL: "https://portal.example.test/login/oidc/callback", SignupURL: "https://untrusted.example/signup"}); err == nil || !strings.Contains(err.Error(), "signup URL") {
		t.Fatalf("cross-origin signup URL error=%v", err)
	}
}

func signTestJWT(t *testing.T, key *rsa.PrivateKey, claims map[string]interface{}) string {
	t.Helper()
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "test-key", "typ": "JWT"})
	payload, _ := json.Marshal(claims)
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func writeTestJSON(w http.ResponseWriter, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		panic(fmt.Sprintf("encode test JSON: %v", err))
	}
}
