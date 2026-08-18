package agentclient

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"alzette/internal/agentauth"
)

const callbackHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Alzette sign-in complete</title></head><body><main><h1>Signed in to Alzette</h1><p>You can close this tab and return to your agent.</p></main></body></html>`

type Config struct {
	ControlURL    string
	RedirectURL   string
	AllowInsecure bool
	HTTPClient    *http.Client
	OpenBrowser   func(string) error
	Output        io.Writer
	Timeout       time.Duration
}

type Metadata struct {
	Schema         string   `json:"schema"`
	Issuer         string   `json:"issuer"`
	OAuthClientID  string   `json:"oauth_client_id"`
	ControlOrigin  string   `json:"control_origin"`
	GatewayBaseURL string   `json:"gateway_base_url"`
	OAuthRedirect  string   `json:"oauth_redirect_uri"`
	LoginModes     []string `json:"login_modes"`
}

type Discovery struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
}

type oauthTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

type ContextsResponse struct {
	Schema   string              `json:"schema"`
	Contexts []agentauth.Context `json:"contexts"`
}

type MintResponse struct {
	Schema     string `json:"schema"`
	Credential struct {
		AccessToken string    `json:"access_token"`
		TokenType   string    `json:"token_type"`
		ExpiresAt   time.Time `json:"expires_at"`
	} `json:"credential"`
	Context        agentauth.Context `json:"context"`
	GatewayBaseURL string            `json:"gateway_base_url"`
	ModelAliases   []string          `json:"model_aliases"`
}

type Session struct {
	config         Config
	metadata       Metadata
	discovery      Discovery
	accessToken    string
	refreshToken   string
	accessExpires  time.Time
	contexts       []agentauth.Context
	clientInstance string

	mu              sync.Mutex
	humanToken      string
	humanExpires    time.Time
	selectedContext agentauth.Context
}

func Login(ctx context.Context, config Config) (*Session, error) {
	if err := prepareConfig(&config); err != nil {
		return nil, err
	}
	metadata, err := readJSON[Metadata](ctx, config.HTTPClient, strings.TrimRight(config.ControlURL, "/")+"/.well-known/alzette-agent-configuration")
	if err != nil {
		return nil, fmt.Errorf("read Alzette agent configuration: %w", err)
	}
	if metadata.Schema != "alzette.agent-configuration.v1" || metadata.OAuthClientID == "" || metadata.ControlOrigin == "" || metadata.GatewayBaseURL == "" {
		return nil, errors.New("Alzette agent configuration is incomplete")
	}
	if metadata.OAuthRedirect != "" && metadata.OAuthRedirect != config.RedirectURL {
		return nil, errors.New("Alzette requires a different registered loopback redirect")
	}
	if err := validateServerURL(metadata.ControlOrigin, config.AllowInsecure); err != nil || strings.TrimRight(metadata.ControlOrigin, "/") != strings.TrimRight(config.ControlURL, "/") {
		return nil, errors.New("Alzette control origin did not match the configured origin")
	}
	if err := validateServerURL(metadata.GatewayBaseURL, config.AllowInsecure); err != nil {
		return nil, errors.New("Alzette gateway URL is unsafe")
	}
	if err := validateServerURL(metadata.Issuer, config.AllowInsecure); err != nil {
		return nil, errors.New("Alzette identity issuer is unsafe")
	}
	discovery, err := readJSON[Discovery](ctx, config.HTTPClient, strings.TrimRight(metadata.Issuer, "/")+"/.well-known/openid-configuration")
	if err != nil {
		return nil, fmt.Errorf("discover Alzette identity service: %w", err)
	}
	if discovery.Issuer != metadata.Issuer || !sameOrigin(metadata.Issuer, discovery.AuthorizationEndpoint) || !sameOrigin(metadata.Issuer, discovery.TokenEndpoint) {
		return nil, errors.New("identity discovery endpoints did not match the configured issuer")
	}
	tokens, err := browserAuthorization(ctx, config, metadata, discovery)
	if err != nil {
		return nil, err
	}
	session := &Session{
		config: config, metadata: metadata, discovery: discovery,
		accessToken: tokens.AccessToken, refreshToken: tokens.RefreshToken,
		accessExpires:  time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second),
		clientInstance: opaque("aci", 16),
	}
	if err := session.loadContexts(ctx); err != nil {
		return nil, err
	}
	return session, nil
}

func prepareConfig(config *Config) error {
	config.ControlURL = strings.TrimRight(strings.TrimSpace(config.ControlURL), "/")
	config.RedirectURL = strings.TrimSpace(config.RedirectURL)
	if config.HTTPClient == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		if config.AllowInsecure {
			dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
			transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
				host, port, splitErr := net.SplitHostPort(address)
				if splitErr == nil && (host == "localhost" || strings.HasSuffix(host, ".localhost")) {
					address = net.JoinHostPort("127.0.0.1", port)
				}
				return dialer.DialContext(ctx, network, address)
			}
		}
		config.HTTPClient = &http.Client{Transport: transport, Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect refused") }}
	}
	if config.OpenBrowser == nil {
		config.OpenBrowser = OpenBrowser
	}
	if config.Output == nil {
		config.Output = io.Discard
	}
	if config.Timeout == 0 {
		config.Timeout = 3 * time.Minute
	}
	if err := validateServerURL(config.ControlURL, config.AllowInsecure); err != nil {
		return fmt.Errorf("control URL: %w", err)
	}
	redirect, err := url.Parse(config.RedirectURL)
	if err != nil || redirect.Scheme != "http" || redirect.User != nil || redirect.RawQuery != "" || redirect.Fragment != "" || redirect.Path == "" {
		return errors.New("OAuth redirect must be an exact loopback HTTP URL")
	}
	host := redirect.Hostname()
	if host != "127.0.0.1" && host != "::1" {
		return errors.New("OAuth redirect must use a loopback IP literal")
	}
	if _, err := strconv.Atoi(redirect.Port()); err != nil {
		return errors.New("OAuth redirect must include a fixed port")
	}
	return nil
}

func browserAuthorization(ctx context.Context, config Config, metadata Metadata, discovery Discovery) (oauthTokens, error) {
	redirect, _ := url.Parse(config.RedirectURL)
	listener, err := net.Listen("tcp", redirect.Host)
	if err != nil {
		return oauthTokens{}, fmt.Errorf("listen for OAuth callback: %w", err)
	}
	defer listener.Close()
	state, nonce, verifier := opaque("", 32), opaque("", 32), opaque("", 48)
	challenge := sha256.Sum256([]byte(verifier))
	query := url.Values{
		"response_type":         {"code"},
		"client_id":             {metadata.OAuthClientID},
		"redirect_uri":          {config.RedirectURL},
		"scope":                 {"openid profile email offline_access"},
		"state":                 {state},
		"nonce":                 {nonce},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(challenge[:])},
		"code_challenge_method": {"S256"},
	}
	authorizationURL := discovery.AuthorizationEndpoint + "?" + query.Encode()
	type result struct{ code, err string }
	resultChannel := make(chan result, 1)
	completeChannel := make(chan struct{}, 1)
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if r.Method == http.MethodGet && r.URL.Path == "/complete" && r.URL.RawQuery == "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.WriteString(w, callbackHTML)
			select {
			case completeChannel <- struct{}{}:
			default:
			}
			return
		}
		if r.Method != http.MethodGet || r.URL.Path != redirect.Path || subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("state")), []byte(state)) != 1 {
			http.Error(w, "Invalid sign-in callback", http.StatusBadRequest)
			return
		}
		value := result{code: r.URL.Query().Get("code"), err: r.URL.Query().Get("error")}
		if value.code == "" || value.err != "" {
			http.Error(w, "Alzette sign-in was not completed", http.StatusBadRequest)
		} else {
			http.Redirect(w, r, "/complete", http.StatusSeeOther)
		}
		select {
		case resultChannel <- value:
		default:
		}
	})}
	go func() { _ = server.Serve(listener) }()
	fmt.Fprintln(config.Output, "Opening Alzette sign-in in your browser…")
	if err := config.OpenBrowser(authorizationURL); err != nil {
		fmt.Fprintln(config.Output, authorizationURL)
		fmt.Fprintln(config.Output, "Open the URL above to continue.")
	}
	timer := time.NewTimer(config.Timeout)
	defer timer.Stop()
	var callback result
	select {
	case <-ctx.Done():
		_ = server.Shutdown(context.Background())
		return oauthTokens{}, ctx.Err()
	case <-timer.C:
		_ = server.Shutdown(context.Background())
		return oauthTokens{}, errors.New("Alzette sign-in timed out")
	case callback = <-resultChannel:
		select {
		case <-completeChannel:
		case <-time.After(2 * time.Second):
		}
		_ = server.Shutdown(context.Background())
	}
	if callback.err != "" || callback.code == "" {
		return oauthTokens{}, errors.New("Alzette sign-in was denied")
	}
	return exchange(ctx, config.HTTPClient, discovery.TokenEndpoint, url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {metadata.OAuthClientID},
		"redirect_uri":  {config.RedirectURL},
		"code":          {callback.code},
		"code_verifier": {verifier},
	})
}

func exchange(ctx context.Context, client *http.Client, endpoint string, form url.Values) (oauthTokens, error) {
	request, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return oauthTokens{}, fmt.Errorf("exchange OAuth credential: %w", err)
	}
	defer response.Body.Close()
	var tokens oauthTokens
	if response.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&tokens) != nil || tokens.AccessToken == "" || !strings.EqualFold(tokens.TokenType, "Bearer") || tokens.ExpiresIn <= 0 {
		return oauthTokens{}, errors.New("identity service returned an invalid token response")
	}
	return tokens, nil
}

func (s *Session) Contexts() []agentauth.Context {
	return append([]agentauth.Context(nil), s.contexts...)
}

func (s *Session) SelectContext(membershipID string) (agentauth.Context, error) {
	if len(s.contexts) == 0 {
		return agentauth.Context{}, errors.New("no model access is assigned to this employee")
	}
	if membershipID == "" && len(s.contexts) == 1 {
		s.selectedContext = s.contexts[0]
		return s.selectedContext, nil
	}
	for _, candidate := range s.contexts {
		if candidate.MembershipID == membershipID {
			s.selectedContext = candidate
			return candidate, nil
		}
	}
	return agentauth.Context{}, errors.New("the requested Alzette context is unavailable")
}

func (s *Session) loadContexts(ctx context.Context) error {
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.metadata.ControlOrigin+"/api/agent/contexts", nil)
	request.Header.Set("Authorization", "Bearer "+s.accessToken)
	request.Header.Set("Accept", "application/json")
	response, err := s.config.HTTPClient.Do(request)
	if err != nil {
		return fmt.Errorf("read Alzette contexts: %w", err)
	}
	defer response.Body.Close()
	var result ContextsResponse
	if response.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result) != nil || result.Schema != "alzette.agent-contexts.v1" {
		return errors.New("Alzette model access could not be loaded")
	}
	for index := range result.Contexts {
		sort.Strings(result.Contexts[index].ModelAliases)
	}
	sort.Slice(result.Contexts, func(i, j int) bool {
		left, right := result.Contexts[i], result.Contexts[j]
		return left.Organisation+left.Project+left.Environment < right.Organisation+right.Project+right.Environment
	})
	s.contexts = result.Contexts
	return nil
}

func (s *Session) EnsureHumanCredential(ctx context.Context) (string, time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.selectedContext.MembershipID == "" {
		return "", time.Time{}, errors.New("no Alzette context is selected")
	}
	if s.humanToken != "" && time.Until(s.humanExpires) > 45*time.Second {
		return s.humanToken, s.humanExpires, nil
	}
	if time.Until(s.accessExpires) <= 45*time.Second {
		if err := s.refreshOAuth(ctx); err != nil {
			return "", time.Time{}, err
		}
	}
	body, _ := json.Marshal(agentauth.MintInput{ClientInstanceID: s.clientInstance, MembershipID: s.selectedContext.MembershipID, ModelAliases: s.selectedContext.ModelAliases})
	request, _ := http.NewRequestWithContext(ctx, http.MethodPost, s.metadata.ControlOrigin+"/api/agent/credentials", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+s.accessToken)
	request.Header.Set("Idempotency-Key", opaque("agm", 16))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, err := s.config.HTTPClient.Do(request)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("mint Alzette session credential: %w", err)
	}
	defer response.Body.Close()
	var result MintResponse
	if response.StatusCode != http.StatusCreated || json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result) != nil || result.Schema != "alzette.agent-credential.v1" || !strings.HasPrefix(result.Credential.AccessToken, "alz_u_") || result.Credential.ExpiresAt.Before(time.Now()) {
		return "", time.Time{}, errors.New("Alzette could not create a session credential")
	}
	s.humanToken, s.humanExpires = result.Credential.AccessToken, result.Credential.ExpiresAt
	return s.humanToken, s.humanExpires, nil
}

func (s *Session) refreshOAuth(ctx context.Context) error {
	if s.refreshToken == "" {
		return errors.New("Alzette login expired; sign in again")
	}
	tokens, err := exchange(ctx, s.config.HTTPClient, s.discovery.TokenEndpoint, url.Values{
		"grant_type": {"refresh_token"}, "client_id": {s.metadata.OAuthClientID}, "refresh_token": {s.refreshToken},
	})
	if err != nil {
		return errors.New("Alzette login could not be refreshed; sign in again")
	}
	s.accessToken, s.accessExpires = tokens.AccessToken, time.Now().Add(time.Duration(tokens.ExpiresIn)*time.Second)
	if tokens.RefreshToken != "" {
		s.refreshToken = tokens.RefreshToken
	}
	return nil
}

func (s *Session) Revoke(ctx context.Context) error {
	if s.selectedContext.MembershipID == "" {
		return nil
	}
	s.mu.Lock()
	if time.Until(s.accessExpires) <= 15*time.Second {
		if err := s.refreshOAuth(ctx); err != nil {
			s.mu.Unlock()
			return err
		}
	}
	accessToken := s.accessToken
	s.mu.Unlock()
	body, _ := json.Marshal(agentauth.MintInput{ClientInstanceID: s.clientInstance, MembershipID: s.selectedContext.MembershipID})
	request, _ := http.NewRequestWithContext(ctx, http.MethodPost, s.metadata.ControlOrigin+"/api/agent/credentials/revoke", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("Content-Type", "application/json")
	response, err := s.config.HTTPClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		return errors.New("Alzette session revocation failed")
	}
	s.mu.Lock()
	s.humanToken, s.humanExpires = "", time.Time{}
	s.mu.Unlock()
	return nil
}

func (s *Session) GatewayBaseURL() string { return s.metadata.GatewayBaseURL }

func OpenBrowser(target string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", target)
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		command = exec.Command("xdg-open", target)
	}
	return command.Start()
}

func readJSON[T any](ctx context.Context, client *http.Client, target string) (T, error) {
	var result T
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	request.Header.Set("Accept", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return result, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result) != nil {
		return result, errors.New("unexpected HTTP response")
	}
	return result, nil
}

func validateServerURL(raw string, allowInsecure bool) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("URL is invalid")
	}
	if parsed.Scheme != "https" && !(allowInsecure && parsed.Scheme == "http") {
		return errors.New("URL must use HTTPS")
	}
	return nil
}

func sameOrigin(origin, target string) bool {
	left, leftErr := url.Parse(origin)
	right, rightErr := url.Parse(target)
	return leftErr == nil && rightErr == nil && left.Scheme == right.Scheme && left.Host == right.Host && right.User == nil
}

func opaque(prefix string, bytesCount int) string {
	value := make([]byte, bytesCount)
	if _, err := rand.Read(value); err != nil {
		panic("system random source unavailable")
	}
	encoded := base64.RawURLEncoding.EncodeToString(value)
	if prefix == "" {
		return encoded
	}
	return prefix + "_" + encoded
}
