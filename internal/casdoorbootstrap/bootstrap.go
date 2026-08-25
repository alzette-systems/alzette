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

const (
	alzetteBrandURL = "https://alzette.systems"
	alzetteMarkURL  = alzetteBrandURL + "/alzette-mark.svg"
	alzetteFormCSS  = `.loginBackground {
  background: #faf9f6 !important;
}
.login-content {
  width: 420px !important;
  max-width: calc(100vw - 32px) !important;
}
.login-panel {
  width: 100% !important;
  overflow: hidden;
  border: 1px solid #d9ddd8 !important;
  border-radius: 6px !important;
  background: #ffffff !important;
  box-shadow: none !important;
}
.login-form {
  width: 100% !important;
  padding: 40px !important;
}
.login-logo-box {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  margin-bottom: 30px !important;
}
.login-logo-box::after {
  content: "Sign in to Alzette";
  color: #10151a;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
  font-size: 22px;
  font-weight: 650;
  line-height: 1.15;
  letter-spacing: -0.02em;
}
.panel-logo {
  width: 64px !important;
  height: 64px !important;
  margin: 0 0 22px !important;
  object-fit: contain;
}
.signin-methods,
.login-auto-signin,
.login-forget-password,
.anticon-global {
  display: none !important;
}
.login-signup-link {
  display: flex !important;
  justify-content: center !important;
  margin: 16px 0 0 !important;
  font-size: 14px;
}
.login-signup-link a {
  color: #087c4e !important;
  font-weight: 620;
}
.ant-form-item {
  margin-bottom: 16px !important;
}
.ant-input-affix-wrapper {
  min-height: 48px;
  border-color: #d9ddd8 !important;
  border-radius: 4px !important;
  box-shadow: none !important;
}
.ant-input-affix-wrapper:hover {
  border-color: #6b747c !important;
}
.ant-input-affix-wrapper-focused {
  border-color: #0d9e63 !important;
  box-shadow: 0 0 0 3px rgba(13, 158, 99, 0.18) !important;
}
.login-button {
  min-height: 48px;
  border-color: #10151a !important;
  border-radius: 4px !important;
  background: #10151a !important;
  box-shadow: none !important;
  font-weight: 620;
}
.login-button:hover,
.login-button:focus-visible {
  border-color: #087c4e !important;
  background: #087c4e !important;
}
#footer {
  padding: 24px !important;
  background: #faf9f6 !important;
  color: #4f5b64 !important;
}`
	alzetteFooterHTML = `<span style="font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;color:#4f5b64;font-size:13px">Alzette Systems · Luxembourg</span>`
)

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
	organisationExists := organisation != nil
	if !organisationExists {
		organisation = map[string]interface{}{"owner": "admin", "name": "alzette", "displayName": "Alzette Workforce", "passwordType": "bcrypt", "passwordOptions": []string{"AtLeast8", "Uppercase", "Lowercase", "Number"}, "defaultApplication": "app-built-in", "defaultTokenFormat": "JWT", "isProfilePublic": false, "languages": []string{"en", "fr", "de"}}
	}
	organisation["displayName"] = "Alzette"
	organisation["logo"] = alzetteMarkURL
	organisation["logoDark"] = alzetteMarkURL
	organisation["favicon"] = alzetteMarkURL
	organisation["websiteUrl"] = alzetteBrandURL
	organisation["languages"] = []string{"en"}
	organisation["themeData"] = map[string]interface{}{"isEnabled": true, "themeType": "default", "colorPrimary": "#0d9e63", "borderRadius": 4, "isCompact": false}
	if !organisationExists {
		if _, err = post(ctx, client, endpoint+"/api/add-organization", organisation); err != nil {
			return Result{}, fmt.Errorf("create Casdoor organisation: %w", err)
		}
	} else if _, err = post(ctx, client, endpoint+"/api/update-organization?id=admin/alzette", organisation); err != nil {
		return Result{}, fmt.Errorf("configure Casdoor organisation: %w", err)
	}
	application, err := getObject(ctx, client, endpoint+"/api/get-application?id=admin/app-built-in")
	if err != nil || application == nil {
		if err == nil {
			err = errors.New("built-in application is absent")
		}
		return Result{}, err
	}
	application["displayName"] = "Alzette"
	application["title"] = "Alzette — Sign in"
	application["description"] = "Sign in to use the model access assigned by your company."
	application["logo"] = alzetteMarkURL
	application["favicon"] = alzetteMarkURL
	application["homepageUrl"] = alzetteBrandURL
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
	application["enableAutoSignin"] = false
	application["enableCodeSignin"] = false
	application["enableWebAuthn"] = false
	application["disableSignin"] = false
	application["signinMethods"] = []map[string]interface{}{{"name": "Password", "displayName": "Password", "rule": "All"}}
	application["themeData"] = map[string]interface{}{"isEnabled": true, "themeType": "default", "colorPrimary": "#0d9e63", "borderRadius": 4, "isCompact": false}
	application["footerHtml"] = alzetteFooterHTML
	application["formCss"] = alzetteFormCSS
	application["formCssMobile"] = alzetteFormCSS
	configureSigninItems(application)
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

func configureSigninItems(application map[string]interface{}) {
	items, ok := application["signinItems"].([]interface{})
	if !ok {
		return
	}
	for _, raw := range items {
		item, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		switch item["name"] {
		case "Back button", "Languages", "Signin methods", "Verification code", "Agreement", "Forgot password?", "Providers":
			item["visible"] = false
		case "Username":
			item["visible"] = true
			item["placeholder"] = "Work email"
		case "Password":
			item["visible"] = true
			item["placeholder"] = "Password"
		case "Login button":
			item["visible"] = true
			item["label"] = "Sign in"
		case "Signup link":
			item["visible"] = true
			item["label"] = "Create your account"
		}
	}
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
