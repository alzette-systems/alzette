package agentclient

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"alzette/internal/agentauth"
)

func TestBrowserLoginContextMintProxyAndRevoke(t *testing.T) {
	callbackListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	callbackAddress := callbackListener.Addr().String()
	callbackListener.Close()
	redirectURL := "http://" + callbackAddress + "/callback"

	var server *httptest.Server
	var mu sync.Mutex
	var challenge, gatewayAuthorization string
	var gatewayBody map[string]json.RawMessage
	revoked := false
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/alzette-agent-configuration":
			writeJSON(w, map[string]interface{}{
				"schema": "alzette.agent-configuration.v1", "issuer": server.URL,
				"oauth_client_id": "alzette-agent-test", "control_origin": server.URL,
				"gateway_base_url": server.URL + "/v1", "oauth_redirect_uri": redirectURL,
				"login_modes": []string{"authorization_code_pkce_s256"},
			})
		case "/.well-known/openid-configuration":
			writeJSON(w, map[string]string{"issuer": server.URL, "authorization_endpoint": server.URL + "/authorize", "token_endpoint": server.URL + "/token"})
		case "/authorize":
			if r.URL.Query().Get("client_id") != "alzette-agent-test" || r.URL.Query().Get("redirect_uri") != redirectURL || r.URL.Query().Get("code_challenge_method") != "S256" || r.URL.Query().Get("nonce") == "" {
				http.Error(w, "bad authorization", http.StatusBadRequest)
				return
			}
			mu.Lock()
			challenge = r.URL.Query().Get("code_challenge")
			mu.Unlock()
			callback, _ := url.Parse(redirectURL)
			query := callback.Query()
			query.Set("code", "single-use-code")
			query.Set("state", r.URL.Query().Get("state"))
			callback.RawQuery = query.Encode()
			http.Redirect(w, r, callback.String(), http.StatusFound)
		case "/token":
			if err := r.ParseForm(); err != nil || r.Form.Get("grant_type") != "authorization_code" || r.Form.Get("code") != "single-use-code" || r.Form.Get("client_id") != "alzette-agent-test" {
				http.Error(w, "bad token exchange", http.StatusBadRequest)
				return
			}
			verifier := r.Form.Get("code_verifier")
			digest := sha256Bytes(verifier)
			mu.Lock()
			valid := challenge == base64.RawURLEncoding.EncodeToString(digest)
			mu.Unlock()
			if !valid {
				http.Error(w, "bad PKCE", http.StatusBadRequest)
				return
			}
			writeJSON(w, map[string]interface{}{"access_token": "oauth-access-token", "refresh_token": "oauth-refresh-token", "token_type": "Bearer", "expires_in": 3600})
		case "/api/agent/contexts":
			if r.Header.Get("Authorization") != "Bearer oauth-access-token" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			writeJSON(w, map[string]interface{}{"schema": "alzette.agent-contexts.v1", "contexts": []map[string]interface{}{{"membership_id": "mem_test", "organisation": "Example", "project": "Shared", "environment": "Production", "relationship": "employee", "model_aliases": []string{"alzette-chat"}}}})
		case "/api/agent/credentials":
			if r.Header.Get("Authorization") != "Bearer oauth-access-token" || !strings.HasPrefix(r.Header.Get("Idempotency-Key"), "agm_") {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			writeJSONStatus(w, http.StatusCreated, map[string]interface{}{"schema": "alzette.agent-credential.v1", "credential": map[string]interface{}{"access_token": "alz_u_test-human", "token_type": "Bearer", "expires_at": time.Now().Add(10 * time.Minute)}, "gateway_base_url": server.URL + "/v1", "model_aliases": []string{"alzette-chat"}})
		case "/api/agent/credentials/revoke":
			if r.Header.Get("Authorization") != "Bearer oauth-access-token" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			revoked = true
			w.WriteHeader(http.StatusNoContent)
		case "/v1/chat/completions":
			gatewayAuthorization = r.Header.Get("Authorization")
			_ = json.NewDecoder(r.Body).Decode(&gatewayBody)
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"id":"req_test","choices":[{"message":{"role":"assistant","content":"hello"}}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	var output bytes.Buffer
	browserClient := server.Client()
	session, err := Login(context.Background(), Config{
		ControlURL: server.URL, RedirectURL: redirectURL, AllowInsecure: true,
		HTTPClient: server.Client(), Output: &output,
		OpenBrowser: func(target string) error {
			go func() {
				response, getErr := browserClient.Get(target)
				if getErr == nil {
					response.Body.Close()
				}
			}()
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "state=") || strings.Contains(output.String(), "oauth-access-token") {
		t.Fatal("successful browser login exposed OAuth transaction material")
	}
	contexts := session.Contexts()
	if len(contexts) != 1 || contexts[0].ModelAliases[0] != "alzette-chat" {
		t.Fatalf("contexts=%#v", contexts)
	}
	if _, err := session.SelectContext(contexts[0].MembershipID); err != nil {
		t.Fatal(err)
	}
	proxy, err := StartProxy(session)
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close(context.Background())
	request, _ := http.NewRequest(http.MethodPost, proxy.BaseURL()+"/chat/completions", strings.NewReader(`{"model":"alzette-chat","messages":[],"temperature":0.7,"top_k":20,"repeat_penalty":1.12,"client_specific_unknown":true}`))
	request.Header.Set("Authorization", "Bearer "+proxy.Key())
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK || gatewayAuthorization != "Bearer alz_u_test-human" {
		t.Fatalf("proxy status=%d gateway authorization=%q", response.StatusCode, gatewayAuthorization)
	}
	if _, exists := gatewayBody["top_k"]; exists {
		t.Fatal("proxy forwarded Jan top_k outside the supported subset")
	}
	if _, exists := gatewayBody["repeat_penalty"]; exists {
		t.Fatal("proxy forwarded Jan repeat_penalty outside the supported subset")
	}
	if _, exists := gatewayBody["temperature"]; !exists {
		t.Fatal("proxy removed a supported sampling field")
	}
	if _, exists := gatewayBody["client_specific_unknown"]; !exists {
		t.Fatal("proxy silently removed an arbitrary field instead of preserving strict gateway validation")
	}
	if err := session.Revoke(context.Background()); err != nil || !revoked {
		t.Fatalf("revoke err=%v revoked=%v", err, revoked)
	}
}

func TestProxyRejectsWrongCapabilityAndRoutes(t *testing.T) {
	session := &Session{metadata: Metadata{GatewayBaseURL: "http://127.0.0.1:1/v1"}, config: Config{HTTPClient: http.DefaultClient}}
	proxy, err := StartProxy(session)
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close(context.Background())
	for _, test := range []struct {
		method, path, key string
		status            int
	}{
		{http.MethodPost, "/v1/chat/completions", "wrong", http.StatusUnauthorized},
		{http.MethodGet, "/v1/chat/completions", proxy.Key(), http.StatusNotFound},
		{http.MethodPost, "/v1/models", proxy.Key(), http.StatusNotFound},
	} {
		request, _ := http.NewRequest(test.method, proxy.BaseURL()+strings.TrimPrefix(test.path, "/v1"), strings.NewReader("{}"))
		request.Header.Set("Authorization", "Bearer "+test.key)
		response, requestErr := http.DefaultClient.Do(request)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		response.Body.Close()
		if response.StatusCode != test.status {
			t.Fatalf("%s %s status=%d", test.method, test.path, response.StatusCode)
		}
	}
}

func TestProxyListsOnlySelectedContextModels(t *testing.T) {
	session := &Session{metadata: Metadata{GatewayBaseURL: "http://127.0.0.1:1/v1"}, config: Config{HTTPClient: http.DefaultClient}, selectedContext: agentauth.Context{ModelAliases: []string{"alzette-chat", "summarise"}}}
	proxy, err := StartProxy(session)
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close(context.Background())
	request, _ := http.NewRequest(http.MethodGet, proxy.BaseURL()+"/models", nil)
	request.Header.Set("Authorization", "Bearer "+proxy.Key())
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var result struct {
		Object string `json:"object"`
		Data   []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(response.Body).Decode(&result) != nil || result.Object != "list" || len(result.Data) != 2 || result.Data[0].ID != "alzette-chat" || result.Data[1].ID != "summarise" {
		t.Fatalf("status=%d result=%#v", response.StatusCode, result)
	}
}

func sha256Bytes(value string) []byte {
	digest := sha256.Sum256([]byte(value))
	return digest[:]
}

func writeJSON(w http.ResponseWriter, value interface{}) {
	writeJSONStatus(w, http.StatusOK, value)
}

func writeJSONStatus(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
