package casdoorbootstrap

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunBrandsExistingCasdoorOrganisationAndApplication(t *testing.T) {
	var updatedOrganisation, updatedApplication map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/login":
			writeBootstrapResponse(t, w, nil)
		case r.Method == http.MethodGet && r.URL.Path == "/api/get-organization":
			writeBootstrapResponse(t, w, map[string]interface{}{"owner": "admin", "name": "alzette", "displayName": "Old identity", "languages": []string{"en", "fr", "de"}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/update-organization":
			decodeBootstrapRequest(t, r, &updatedOrganisation)
			writeBootstrapResponse(t, w, nil)
		case r.Method == http.MethodGet && r.URL.Path == "/api/get-application":
			writeBootstrapResponse(t, w, map[string]interface{}{
				"owner": "admin", "name": "app-built-in", "clientSecret": "must-be-preserved",
				"signinItems": []map[string]interface{}{
					{"name": "Languages", "visible": true},
					{"name": "Username", "visible": true},
					{"name": "Password", "visible": true},
					{"name": "Forgot password?", "visible": true},
					{"name": "Login button", "visible": true},
					{"name": "Signup link", "visible": true},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/update-application":
			decodeBootstrapRequest(t, r, &updatedApplication)
			writeBootstrapResponse(t, w, nil)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	_, err := Run(context.Background(), Config{
		Endpoint: server.URL, AdminPassword: "admin-password", ClientID: "alzette-client", ClientSecret: "012345678901234567890123",
		RedirectURL: "https://app.alzette.systems/app/invitations/callback", AgentRedirectURL: "http://127.0.0.1:43127/callback", AllowInsecure: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updatedOrganisation["displayName"] != "Alzette" || updatedOrganisation["logo"] != alzetteMarkURL || updatedOrganisation["websiteUrl"] != alzetteBrandURL {
		t.Fatalf("organisation branding=%#v", updatedOrganisation)
	}
	languages, ok := updatedOrganisation["languages"].([]interface{})
	if !ok || len(languages) != 1 || languages[0] != "en" {
		t.Fatalf("organisation languages=%#v", updatedOrganisation["languages"])
	}
	if updatedApplication["displayName"] != "Alzette" || updatedApplication["logo"] != alzetteMarkURL || updatedApplication["homepageUrl"] != alzetteBrandURL {
		t.Fatalf("application branding=%#v", updatedApplication)
	}
	if updatedApplication["enableSignUp"] != true || updatedApplication["clientSecret"] != "012345678901234567890123" {
		t.Fatalf("application security fields=%#v", updatedApplication)
	}
	methods, ok := updatedApplication["signinMethods"].([]interface{})
	if !ok || len(methods) != 1 || methods[0].(map[string]interface{})["name"] != "Password" {
		t.Fatalf("signin methods=%#v", updatedApplication["signinMethods"])
	}
	if !strings.Contains(updatedApplication["formCss"].(string), "Sign in to Alzette") || strings.Contains(updatedApplication["footerHtml"].(string), "Casdoor") {
		t.Fatalf("custom login branding was not installed")
	}
	items := updatedApplication["signinItems"].([]interface{})
	noteFound := false
	for _, raw := range items {
		item := raw.(map[string]interface{})
		if item["name"] == "Username" && item["placeholder"] != "Work email" {
			t.Fatalf("username item=%#v", item)
		}
		if item["name"] == "Signup link" && item["visible"] != false {
			t.Fatalf("signup item=%#v", item)
		}
		if item["name"] == "Text Alzette invitation" {
			noteFound = item["visible"] == true && strings.Contains(item["customCss"].(string), "Access starts with an invitation")
		}
	}
	if !noteFound {
		t.Fatal("invitation guidance was not installed")
	}
}

func decodeBootstrapRequest(t *testing.T, r *http.Request, target interface{}) {
	t.Helper()
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		t.Fatal(err)
	}
}

func writeBootstrapResponse(t *testing.T, w http.ResponseWriter, data interface{}) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "data": data}); err != nil {
		t.Fatal(err)
	}
}
