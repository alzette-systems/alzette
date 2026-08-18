package casdoorbootstrap

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

type Config struct {
	Endpoint, AdminPassword, ClientID, ClientSecret, RedirectURL, AgentRedirectURL string
	DemoUsername, DemoPassword, DemoEmail                                          string
	AllowInsecure                                                                  bool
}

type Result struct {
	Application  string `json:"application"`
	Organisation string `json:"organisation"`
	DemoUser     string `json:"demo_user"`
}

type response struct {
	Status string          `json:"status"`
	Msg    string          `json:"msg"`
	Data   json.RawMessage `json:"data"`
}

func Run(ctx context.Context, config Config) (Result, error) {
	endpoint, err := validate(config)
	if err != nil {
		return Result{}, err
	}
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return errors.New("Casdoor bootstrap redirects are disabled")
	}}
	login := map[string]interface{}{"type": "login", "signinMethod": "Password", "organization": "built-in", "application": "app-built-in", "username": "admin", "password": config.AdminPassword}
	if _, err = post(ctx, client, endpoint+"/api/login", login); err != nil {
		return Result{}, fmt.Errorf("Casdoor bootstrap login: %w", err)
	}
	organisation, err := getObject(ctx, client, endpoint+"/api/get-organization?id=admin/alzette")
	if err != nil {
		return Result{}, err
	}
	if organisation == nil {
		organisation = map[string]interface{}{"owner": "admin", "name": "alzette", "displayName": "Alzette Workforce", "passwordType": "bcrypt", "passwordOptions": []string{"AtLeast8", "Uppercase", "Lowercase", "Number"}, "defaultApplication": "app-built-in", "defaultTokenFormat": "JWT", "isProfilePublic": false, "languages": []string{"en", "fr", "de"}}
		if _, err = post(ctx, client, endpoint+"/api/add-organization", organisation); err != nil {
			return Result{}, fmt.Errorf("create Casdoor organisation: %w", err)
		}
	}
	application, err := getObject(ctx, client, endpoint+"/api/get-application?id=admin/app-built-in")
	if err != nil || application == nil {
		if err == nil {
			err = errors.New("built-in application is absent")
		}
		return Result{}, err
	}
	application["displayName"] = "Alzette"
	application["organization"] = "alzette"
	application["clientId"] = config.ClientID
	application["clientSecret"] = config.ClientSecret
	redirects := []string{config.RedirectURL}
	if config.AgentRedirectURL != "" && config.AgentRedirectURL != config.RedirectURL {
		redirects = append(redirects, config.AgentRedirectURL)
	}
	application["redirectUris"] = redirects
	application["grantTypes"] = []string{"authorization_code", "refresh_token"}
	application["tokenFormat"] = "JWT"
	application["tokenSigningMethod"] = "RS256"
	application["expireInHours"] = 1
	application["refreshExpireInHours"] = 24
	application["enablePassword"] = true
	application["enableSignUp"] = true
	application["disableSignin"] = false
	application["signupItems"] = []map[string]interface{}{
		{"name": "Username", "visible": true, "required": true, "rule": "None"},
		{"name": "Display name", "visible": true, "required": true, "rule": "None"},
		{"name": "Password", "visible": true, "required": true, "rule": "None"},
		{"name": "Confirm password", "visible": true, "required": true, "rule": "None"},
		{"name": "Email", "visible": true, "required": true, "rule": "No verification"},
	}
	if _, err = post(ctx, client, endpoint+"/api/update-application?id=admin/app-built-in", application); err != nil {
		return Result{}, fmt.Errorf("configure Casdoor application: %w", err)
	}
	if config.DemoUsername != "" {
		user, err := getObject(ctx, client, endpoint+"/api/get-user?id=alzette/"+url.QueryEscape(config.DemoUsername))
		if err != nil {
			return Result{}, err
		}
		if user == nil {
			user = map[string]interface{}{"owner": "alzette", "name": config.DemoUsername, "displayName": "Demo Employee", "type": "normal-user", "password": config.DemoPassword, "passwordType": "plain", "email": strings.ToLower(config.DemoEmail), "emailVerified": true, "isAdmin": false, "isForbidden": false, "isDeleted": false, "signupApplication": "app-built-in"}
			if _, err = post(ctx, client, endpoint+"/api/add-user", user); err != nil {
				return Result{}, fmt.Errorf("create Casdoor demo employee: %w", err)
			}
		}
	}
	return Result{Application: "app-built-in", Organisation: "alzette", DemoUser: config.DemoUsername}, nil
}

func validate(config Config) (string, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(config.Endpoint), "/")
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Scheme != "https" && !(config.AllowInsecure && parsed.Scheme == "http")) {
		return "", errors.New("Casdoor bootstrap endpoint is invalid")
	}
	redirect, err := url.Parse(config.RedirectURL)
	if err != nil || redirect.Host == "" || redirect.Fragment != "" || (redirect.Scheme != "https" && !(config.AllowInsecure && redirect.Scheme == "http")) {
		return "", errors.New("Casdoor redirect URL is invalid")
	}
	if config.AgentRedirectURL != "" {
		agentRedirect, parseErr := url.Parse(config.AgentRedirectURL)
		if parseErr != nil || agentRedirect.Scheme != "http" || agentRedirect.User != nil || agentRedirect.RawQuery != "" || agentRedirect.Fragment != "" || (agentRedirect.Hostname() != "127.0.0.1" && agentRedirect.Hostname() != "::1") || agentRedirect.Port() == "" {
			return "", errors.New("Casdoor agent redirect URL must use an exact loopback IP")
		}
	}
	if config.AdminPassword == "" || len(config.ClientID) < 8 || len(config.ClientSecret) < 24 || config.DemoUsername != "" && (len(config.DemoPassword) < 8 || !strings.Contains(config.DemoEmail, "@")) {
		return "", errors.New("Casdoor bootstrap credentials are incomplete")
	}
	return endpoint, nil
}

func getObject(ctx context.Context, client *http.Client, endpoint string) (map[string]interface{}, error) {
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	request.Header.Set("Accept", "application/json")
	result, err := do(client, request)
	if err != nil {
		return nil, err
	}
	if string(result.Data) == "null" || len(result.Data) == 0 {
		return nil, nil
	}
	var object map[string]interface{}
	if json.Unmarshal(result.Data, &object) != nil {
		return nil, errors.New("Casdoor returned an invalid object")
	}
	return object, nil
}

func post(ctx context.Context, client *http.Client, endpoint string, value interface{}) (response, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return response{}, err
	}
	request, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	return do(client, request)
}

func do(client *http.Client, request *http.Request) (response, error) {
	httpResponse, err := client.Do(request)
	if err != nil {
		return response{}, err
	}
	defer httpResponse.Body.Close()
	var result response
	if httpResponse.StatusCode != http.StatusOK || json.NewDecoder(httpResponse.Body).Decode(&result) != nil || result.Status != "ok" {
		return response{}, fmt.Errorf("Casdoor request failed with status %d", httpResponse.StatusCode)
	}
	return result, nil
}
