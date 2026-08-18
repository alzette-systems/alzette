package federation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type Config struct {
	Issuer        string
	ClientID      string
	ClientSecret  string
	RedirectURL   string
	AllowInsecure bool
}

type Identity struct {
	Issuer        string
	Subject       string
	Email         string
	EmailVerified bool
	DisplayName   string
	OAuthClientID string
}

type Provider interface {
	AuthorizationURL(state, nonce, verifier string) string
	Exchange(context.Context, string, string, string) (Identity, error)
	Issuer() string
}

type AccessTokenProvider interface {
	ValidateAccessToken(context.Context, string) (Identity, error)
	Issuer() string
	ClientID() string
}

type Client struct {
	issuer                string
	oauth                 oauth2.Config
	verifier              *oidc.IDTokenVerifier
	httpClient            *http.Client
	introspectionEndpoint string
}

func New(ctx context.Context, config Config) (*Client, error) {
	config.Issuer = strings.TrimSuffix(strings.TrimSpace(config.Issuer), "/")
	config.ClientID = strings.TrimSpace(config.ClientID)
	config.RedirectURL = strings.TrimSpace(config.RedirectURL)
	issuerURL, err := url.Parse(config.Issuer)
	if err != nil || issuerURL.Host == "" || issuerURL.RawQuery != "" || issuerURL.Fragment != "" || (issuerURL.Scheme != "https" && !(config.AllowInsecure && issuerURL.Scheme == "http")) {
		return nil, errors.New("OIDC issuer must be an exact HTTPS origin")
	}
	redirectURL, err := url.Parse(config.RedirectURL)
	if err != nil || redirectURL.Host == "" || redirectURL.RawQuery != "" || redirectURL.Fragment != "" || (redirectURL.Scheme != "https" && !(config.AllowInsecure && redirectURL.Scheme == "http")) {
		return nil, errors.New("OIDC redirect URL must be an exact HTTPS URL")
	}
	if config.ClientID == "" || len(config.ClientID) > 255 || len(config.ClientSecret) > 4096 {
		return nil, errors.New("OIDC client configuration is incomplete")
	}
	provider, err := oidc.NewProvider(ctx, config.Issuer)
	if err != nil {
		return nil, fmt.Errorf("discover OIDC provider: %w", err)
	}
	baseHTTPClient, _ := ctx.Value(oauth2.HTTPClient).(*http.Client)
	if baseHTTPClient == nil {
		baseHTTPClient = http.DefaultClient
	}
	httpClient := &http.Client{Transport: baseHTTPClient.Transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("OIDC redirects are disabled") }}
	var discovery struct {
		IntrospectionEndpoint string `json:"introspection_endpoint"`
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, config.Issuer+"/.well-known/openid-configuration", nil)
	if err != nil {
		return nil, err
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("read OIDC discovery metadata: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || json.NewDecoder(response.Body).Decode(&discovery) != nil {
		return nil, errors.New("OIDC discovery metadata is invalid")
	}
	introspectionURL, err := url.Parse(discovery.IntrospectionEndpoint)
	if err != nil || introspectionURL.Scheme != issuerURL.Scheme || introspectionURL.Host != issuerURL.Host || introspectionURL.User != nil || introspectionURL.RawQuery != "" || introspectionURL.Fragment != "" {
		return nil, errors.New("OIDC introspection endpoint must use the configured issuer origin")
	}
	return &Client{
		issuer:     config.Issuer,
		oauth:      oauth2.Config{ClientID: config.ClientID, ClientSecret: config.ClientSecret, RedirectURL: config.RedirectURL, Endpoint: provider.Endpoint(), Scopes: []string{oidc.ScopeOpenID, "profile", "email"}},
		verifier:   provider.Verifier(&oidc.Config{ClientID: config.ClientID, SupportedSigningAlgs: []string{"RS256"}}),
		httpClient: httpClient, introspectionEndpoint: discovery.IntrospectionEndpoint,
	}, nil
}

func (c *Client) Issuer() string   { return c.issuer }
func (c *Client) ClientID() string { return c.oauth.ClientID }

func (c *Client) AuthorizationURL(state, nonce, verifier string) string {
	return c.oauth.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier), oauth2.SetAuthURLParam("nonce", nonce))
}

func (c *Client) ValidateAccessToken(ctx context.Context, raw string) (Identity, error) {
	if strings.TrimSpace(raw) != raw || len(raw) < 64 || len(raw) > 16384 || strings.Count(raw, ".") != 2 {
		return Identity{}, errors.New("OIDC access token format is invalid")
	}
	token, err := c.verifier.Verify(ctx, raw)
	if err != nil {
		return Identity{}, fmt.Errorf("verify OIDC access token: %w", err)
	}
	var claims struct {
		TokenType string `json:"tokenType"`
	}
	if err := token.Claims(&claims); err != nil || claims.TokenType != "access-token" {
		return Identity{}, errors.New("OIDC token is not an access token")
	}
	form := url.Values{"token": {raw}, "token_type_hint": {"access_token"}}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.introspectionEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return Identity{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	request.SetBasicAuth(c.oauth.ClientID, c.oauth.ClientSecret)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return Identity{}, fmt.Errorf("introspect OIDC access token: %w", err)
	}
	defer response.Body.Close()
	var result struct {
		Active    bool   `json:"active"`
		ClientID  string `json:"client_id"`
		Subject   string `json:"sub"`
		TokenType string `json:"token_type"`
		ExpiresAt int64  `json:"exp"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(response.Body).Decode(&result) != nil || !result.Active || result.ClientID != c.oauth.ClientID || !strings.EqualFold(result.TokenType, "Bearer") || result.ExpiresAt <= time.Now().Unix() {
		return Identity{}, errors.New("OIDC access token is inactive")
	}
	if result.Subject != "" && result.Subject != token.Subject {
		return Identity{}, errors.New("OIDC access-token subject did not match introspection")
	}
	if token.Subject == "" {
		return Identity{}, errors.New("OIDC access token has no subject")
	}
	return Identity{Issuer: c.issuer, Subject: token.Subject, OAuthClientID: c.oauth.ClientID}, nil
}

func (c *Client) Exchange(ctx context.Context, code, verifier, expectedNonce string) (Identity, error) {
	if strings.TrimSpace(code) == "" || verifier == "" || expectedNonce == "" {
		return Identity{}, errors.New("OIDC callback is incomplete")
	}
	token, err := c.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return Identity{}, fmt.Errorf("exchange OIDC code: %w", err)
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return Identity{}, errors.New("OIDC token response did not include an ID token")
	}
	idToken, err := c.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return Identity{}, fmt.Errorf("verify OIDC ID token: %w", err)
	}
	var claims struct {
		Nonce         string `json:"nonce"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return Identity{}, fmt.Errorf("decode OIDC identity claims: %w", err)
	}
	if claims.Nonce != expectedNonce {
		return Identity{}, errors.New("OIDC nonce did not match")
	}
	email := strings.ToLower(strings.TrimSpace(claims.Email))
	if idToken.Subject == "" || email == "" || !claims.EmailVerified {
		return Identity{}, errors.New("OIDC identity is missing a verified email")
	}
	return Identity{Issuer: c.issuer, Subject: idToken.Subject, Email: email, EmailVerified: true, DisplayName: strings.TrimSpace(claims.Name)}, nil
}

var _ Provider = (*Client)(nil)
var _ AccessTokenProvider = (*Client)(nil)
