package portal

import (
	"context"
	"crypto/subtle"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"alzette/internal/api"
	"alzette/internal/billing"
	"alzette/internal/catalogue"
	"alzette/internal/endpoints"
	"alzette/internal/humanauth"
	"alzette/internal/ids"
	"alzette/internal/platform"
)

const (
	rawMarker       = `<meta name="alzette-api-mode" content="fallback">`
	liveMarker      = `<meta name="alzette-api-mode" content="live">`
	maximumFormBody = 16 << 10
	maximumJSONBody = 32 << 10
	maximumRows     = 10000
)

var staticAssetNames = []string{"login.html", "login.css", "portal.html", "portal.css", "portal.js", "alzette-mark.svg"}

type Config struct {
	Store                      platform.Store
	PortalStore                platform.PortalStore
	StaticDirectory            string
	CookieSecure               bool
	SessionTTL                 time.Duration
	Clock                      func() time.Time
	GenerateSessionToken       func() (string, error)
	GenerateCSRFToken          func() (string, error)
	NewID                      func(string) (string, error)
	PublicGatewayURL           string
	AllowInsecurePublicGateway bool
	Catalogue                  *catalogue.Service
	Endpoints                  *endpoints.Service
	Billing                    *billing.Service
}

type asset struct {
	name, contentType string
	contents          []byte
	modifiedAt        time.Time
}

type App struct {
	store                platform.Store
	portalStore          platform.PortalStore
	assets               map[string]asset
	cookieSecure         bool
	sessionTTL           time.Duration
	clock                func() time.Time
	generateSessionToken func() (string, error)
	generateCSRFToken    func() (string, error)
	newID                func(string) (string, error)
	publicGatewayURL     string
	chatCompletionsURL   string
	csp                  string
	catalogue            *catalogue.Service
	endpoints            *endpoints.Service
	billing              *billing.Service
	overview             *overviewRenderer
}

type reauthenticationStore interface {
	ReauthenticatePortalSession(context.Context, [32]byte, string, time.Time) (platform.PortalSession, error)
}

func New(config Config) (*App, error) {
	if config.Store == nil || config.PortalStore == nil {
		return nil, errors.New("portal machine and session stores are required")
	}
	if config.SessionTTL == 0 {
		config.SessionTTL = 12 * time.Hour
	}
	if config.SessionTTL < 15*time.Minute || config.SessionTTL > 7*24*time.Hour {
		return nil, errors.New("portal session TTL is outside supported bounds")
	}
	if config.Clock == nil {
		config.Clock = time.Now
	}
	if config.GenerateSessionToken == nil {
		config.GenerateSessionToken = humanauth.GenerateSessionToken
	}
	if config.GenerateCSRFToken == nil {
		config.GenerateCSRFToken = humanauth.GenerateCSRFToken
	}
	if config.NewID == nil {
		config.NewID = ids.New
	}
	publicBase, chatURL, err := validatePublicGatewayURL(config.PublicGatewayURL, config.AllowInsecurePublicGateway)
	if err != nil {
		return nil, err
	}
	assets := make(map[string]asset, len(staticAssetNames))
	for _, name := range staticAssetNames {
		value, err := readAsset(config.StaticDirectory, name)
		if err != nil {
			return nil, fmt.Errorf("read portal asset %s: %w", name, err)
		}
		if name == "login.html" || name == "portal.html" {
			if strings.Count(string(value.contents), rawMarker) != 1 {
				return nil, fmt.Errorf("portal asset %s must contain exactly one runtime marker", name)
			}
			value.contents = []byte(strings.Replace(string(value.contents), rawMarker, liveMarker, 1))
			value.contentType = "text/html; charset=utf-8"
		}
		assets[name] = value
	}
	overview, err := newOverviewRenderer()
	if err != nil {
		return nil, fmt.Errorf("parse portal overview template: %w", err)
	}
	return &App{
		store: config.Store, portalStore: config.PortalStore, assets: assets,
		cookieSecure: config.CookieSecure, sessionTTL: config.SessionTTL, clock: config.Clock,
		generateSessionToken: config.GenerateSessionToken, generateCSRFToken: config.GenerateCSRFToken, newID: config.NewID,
		publicGatewayURL: publicBase, chatCompletionsURL: chatURL,
		catalogue: config.Catalogue, endpoints: config.Endpoints, billing: config.Billing,
		overview: overview,
		csp:      "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'",
	}, nil
}

func validatePublicGatewayURL(value string, allowInsecure bool) (string, string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || strings.Contains(parsed.EscapedPath(), "%") {
		return "", "", errors.New("ALZETTE_PUBLIC_GATEWAY_URL is invalid")
	}
	if parsed.Scheme != "https" && !(allowInsecure && parsed.Scheme == "http") {
		return "", "", errors.New("ALZETTE_PUBLIC_GATEWAY_URL must use HTTPS unless the explicit LAN/dev override is enabled")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", "", errors.New("ALZETTE_PUBLIC_GATEWAY_URL must not contain a path")
	}
	parsed.Path = ""
	base := strings.TrimRight(parsed.String(), "/")
	return base, base + "/v1/chat/completions", nil
}

func readAsset(directory, name string) (asset, error) {
	filename := filepath.Join(directory, name)
	pathInfo, err := os.Lstat(filename)
	if err != nil {
		return asset{}, err
	}
	if pathInfo.Mode()&os.ModeSymlink != 0 || !pathInfo.Mode().IsRegular() {
		return asset{}, errors.New("portal assets must be regular, non-symlink files")
	}
	file, err := os.Open(filename)
	if err != nil {
		return asset{}, err
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return asset{}, err
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(pathInfo, openedInfo) {
		return asset{}, errors.New("portal asset changed while it was opened")
	}
	contents, err := io.ReadAll(file)
	if err != nil {
		return asset{}, err
	}
	contentType := mime.TypeByExtension(filepath.Ext(name))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return asset{name: name, contentType: contentType, contents: contents, modifiedAt: openedInfo.ModTime()}, nil
}

func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.setHeaders(w)
	requestID, err := a.newID("por")
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "internal_error", "api_error", "request could not be initialised", "")
		return
	}
	w.Header().Set("X-Alzette-Request-ID", requestID)
	w.Header().Set("X-Request-ID", requestID)
	if unsafeRequestPath(r.URL.Path) {
		http.NotFound(w, r)
		return
	}
	switch {
	case r.URL.Path == "/":
		http.Redirect(w, r, "/app/overview", http.StatusSeeOther)
	case (r.URL.Path == "/login" || r.URL.Path == "/login.html") && r.Method == http.MethodGet:
		a.serveAsset(w, r, "login.html")
	case r.URL.Path == "/login" && r.Method == http.MethodPost:
		a.login(w, r)
	case r.URL.Path == "/logout" || r.URL.Path == "/api/portal/logout":
		a.logout(w, r)
	case r.URL.Path == "/login.css" || r.URL.Path == "/portal.css" || r.URL.Path == "/portal.js" || r.URL.Path == "/alzette-mark.svg":
		a.serveAsset(w, r, strings.TrimPrefix(r.URL.Path, "/"))
	case r.URL.Path == "/app/portal.css" || r.URL.Path == "/app/portal.js" || r.URL.Path == "/app/alzette-mark.svg":
		a.serveAsset(w, r, strings.TrimPrefix(r.URL.Path, "/app/"))
	case r.URL.Path == "/app/overview":
		a.serveOverview(w, r)
	case r.URL.Path == "/app" || r.URL.Path == "/app/" || strings.HasPrefix(r.URL.Path, "/app/"):
		a.servePortal(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/portal/"):
		a.serveAPI(w, r)
	default:
		http.NotFound(w, r)
	}
}

func unsafeRequestPath(value string) bool {
	if value == "" || strings.ContainsAny(value, "\\\x00") {
		return true
	}
	cleaned := path.Clean(value)
	return value != cleaned && value != cleaned+"/"
}

func (a *App) setHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	appendVary(w.Header(), "Cookie")
	w.Header().Set("Content-Security-Policy", a.csp)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Frame-Options", "DENY")
}

func appendVary(header http.Header, value string) {
	for _, line := range header.Values("Vary") {
		for _, existing := range strings.Split(line, ",") {
			if strings.EqualFold(strings.TrimSpace(existing), value) {
				return
			}
		}
	}
	header.Add("Vary", value)
}

func (a *App) serveAsset(w http.ResponseWriter, r *http.Request, name string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		api.MethodNotAllowed(w, "GET, HEAD", "")
		return
	}
	value, ok := a.assets[name]
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", value.contentType)
	http.ServeContent(w, r, value.name, value.modifiedAt, strings.NewReader(string(value.contents)))
}

func (a *App) servePortal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		api.MethodNotAllowed(w, "GET, HEAD", "")
		return
	}
	if _, _, err := a.session(r); err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	a.serveAsset(w, r, "portal.html")
}

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maximumFormBody)
	var parseErr error
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		parseErr = r.ParseMultipartForm(maximumFormBody)
		if r.MultipartForm != nil {
			defer r.MultipartForm.RemoveAll()
		}
	} else {
		parseErr = r.ParseForm()
	}
	if parseErr != nil {
		a.loginFailure(w, r)
		return
	}
	username, password := r.PostForm.Get("username"), r.PostForm.Get("password")
	if len(username) > 64 || len(password) > 128 {
		a.loginFailure(w, r)
		return
	}
	sessionToken, err := a.generateSessionToken()
	if err != nil {
		api.WriteError(w, http.StatusServiceUnavailable, "authentication_unavailable", "api_error", "sign-in is temporarily unavailable", "")
		return
	}
	csrfToken, err := a.generateCSRFToken()
	if err != nil {
		api.WriteError(w, http.StatusServiceUnavailable, "authentication_unavailable", "api_error", "sign-in is temporarily unavailable", "")
		return
	}
	now := a.clock().UTC()
	_, err = a.portalStore.CreatePortalSession(r.Context(), username, password, humanauth.Digest(sessionToken), now.Add(a.sessionTTL), now)
	if err != nil {
		if errors.Is(err, platform.ErrUnauthenticated) || errors.Is(err, platform.ErrForbidden) {
			a.loginFailure(w, r)
			return
		}
		api.WriteError(w, http.StatusServiceUnavailable, "authentication_unavailable", "api_error", "sign-in is temporarily unavailable", "")
		return
	}
	a.setCookie(w, humanauth.SessionCookieName, sessionToken, true, now.Add(a.sessionTTL))
	a.setCookie(w, humanauth.CSRFCookieName, csrfToken, false, now.Add(a.sessionTTL))
	http.Redirect(w, r, "/app/overview", http.StatusSeeOther)
}

func (a *App) loginFailure(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_credentials", "message": "Sign-in failed"})
}

func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.MethodNotAllowed(w, http.MethodPost, "")
		return
	}
	_, digest, err := a.session(r)
	if err != nil {
		a.clearCookies(w)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !validCSRF(r) && !validCSRFForm(w, r) {
		api.WriteError(w, http.StatusForbidden, "csrf_failed", "permission_error", "request could not be verified", "")
		return
	}
	if err := a.portalStore.RevokePortalSession(r.Context(), digest, a.clock().UTC()); err != nil && !errors.Is(err, platform.ErrUnauthenticated) {
		api.WriteError(w, http.StatusServiceUnavailable, "logout_unavailable", "api_error", "sign-out is temporarily unavailable", "")
		return
	}
	a.clearCookies(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (a *App) serveAPI(w http.ResponseWriter, r *http.Request) {
	session, digest, err := a.session(r)
	if err != nil {
		api.WriteError(w, http.StatusUnauthorized, "session_required", "authentication_error", "sign-in required", "")
		return
	}
	if r.Method != http.MethodGet && !validCSRF(r) {
		api.WriteError(w, http.StatusForbidden, "csrf_failed", "permission_error", "request could not be verified", "")
		return
	}
	if r.URL.Path == "/api/portal/usage/export" && !validCSRF(r) {
		api.WriteError(w, http.StatusForbidden, "csrf_failed", "permission_error", "request could not be verified", "")
		return
	}
	switch {
	case r.URL.Path == "/api/portal/me" && r.Method == http.MethodGet:
		a.me(w, r, session)
	case r.URL.Path == "/api/portal/dashboard" && r.Method == http.MethodGet:
		a.dashboard(w, r, session)
	case r.URL.Path == "/api/portal/access" && r.Method == http.MethodGet:
		a.access(w, r, session)
	case r.URL.Path == "/api/portal/reauthenticate" && r.Method == http.MethodPost:
		a.reauthenticate(w, r, session, digest)
	case r.URL.Path == "/api/portal/service-accounts" && r.Method == http.MethodPost:
		a.createServiceAccount(w, r, session)
	case r.URL.Path == "/api/portal/keys/issue" && r.Method == http.MethodPost:
		a.issueKey(w, r, session, false)
	case r.URL.Path == "/api/portal/keys/rotate" && r.Method == http.MethodPost:
		a.issueKey(w, r, session, true)
	case r.URL.Path == "/api/portal/keys/revoke" && r.Method == http.MethodPost:
		a.revokeKey(w, r, session)
	case r.URL.Path == "/api/portal/context" && r.Method == http.MethodPost:
		a.switchContext(w, r, session, digest)
	case r.URL.Path == "/api/portal/usage/export" && r.Method == http.MethodGet:
		a.export(w, r, session)
	case a.serveEndpointAPI(w, r, session):
		return
	default:
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			api.MethodNotAllowed(w, "GET, POST", "")
			return
		}
		api.WriteError(w, http.StatusNotFound, "not_found", "invalid_request_error", "resource not found", "")
	}
}

func (a *App) reauthenticate(w http.ResponseWriter, r *http.Request, _ platform.PortalSession, digest [32]byte) {
	var input struct {
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Password == "" || len(input.Password) > 128 {
		api.WriteError(w, http.StatusUnauthorized, "invalid_credentials", "authentication_error", "password confirmation failed", "")
		return
	}
	store, ok := a.portalStore.(reauthenticationStore)
	if !ok {
		api.WriteError(w, http.StatusServiceUnavailable, "reauthentication_unavailable", "api_error", "password confirmation is temporarily unavailable", "")
		return
	}
	session, err := store.ReauthenticatePortalSession(r.Context(), digest, input.Password, a.clock().UTC())
	if errors.Is(err, platform.ErrUnauthenticated) || errors.Is(err, platform.ErrForbidden) {
		api.WriteError(w, http.StatusUnauthorized, "invalid_credentials", "authentication_error", "password confirmation failed", "")
		return
	}
	if err != nil {
		api.WriteError(w, http.StatusServiceUnavailable, "reauthentication_unavailable", "api_error", "password confirmation is temporarily unavailable", "")
		return
	}
	api.WriteJSON(w, http.StatusOK, map[string]interface{}{"schema": "alzette.portal.reauthentication.v1", "authenticated_at": session.AuthenticatedAt})
}

func (a *App) serveEndpointAPI(w http.ResponseWriter, r *http.Request, session platform.PortalSession) bool {
	if a.catalogue != nil && r.Method == http.MethodGet {
		if r.URL.Path == "/api/portal/catalogue/models" {
			values, err := a.catalogue.List(r.Context(), session)
			if err != nil {
				a.writeEndpointError(w, err, "catalogue_unavailable")
				return true
			}
			api.WriteJSON(w, http.StatusOK, map[string]interface{}{"schema": "alzette.portal.catalogue.v1", "models": values})
			return true
		}
		if slug, ok := singlePathValue(r.URL.Path, "/api/portal/catalogue/models/"); ok {
			value, err := a.catalogue.Get(r.Context(), session, slug)
			if err != nil {
				a.writeEndpointError(w, err, "catalogue_unavailable")
				return true
			}
			api.WriteJSON(w, http.StatusOK, map[string]interface{}{"schema": "alzette.portal.catalogue_model.v1", "model": value})
			return true
		}
	}
	if a.endpoints != nil {
		switch {
		case r.URL.Path == "/api/portal/endpoints" && r.Method == http.MethodGet:
			values, err := a.endpoints.List(r.Context(), session)
			if err != nil {
				a.writeEndpointError(w, err, "endpoints_unavailable")
				return true
			}
			api.WriteJSON(w, http.StatusOK, map[string]interface{}{"schema": "alzette.portal.endpoints.v1", "endpoints": values})
			return true
		case r.URL.Path == "/api/portal/endpoint-configurations" && r.Method == http.MethodPost:
			var input endpoints.CreateInput
			if !decodeJSON(w, r, &input) {
				return true
			}
			value, err := a.endpoints.Create(r.Context(), session, input, requestIdempotencyKey(r))
			if err != nil {
				a.writeEndpointError(w, err, "configuration_unavailable")
				return true
			}
			api.WriteJSON(w, http.StatusCreated, map[string]interface{}{"schema": "alzette.portal.endpoint_configuration.v1", "configuration": value})
			return true
		case strings.HasPrefix(r.URL.Path, "/api/portal/endpoint-configurations/"):
			return a.serveConfigurationMutation(w, r, session)
		case strings.HasPrefix(r.URL.Path, "/api/portal/deployment-requests/") && r.Method == http.MethodGet:
			id, ok := singlePathValue(r.URL.Path, "/api/portal/deployment-requests/")
			if !ok {
				return false
			}
			value, err := a.endpoints.Request(r.Context(), session, id)
			if err != nil {
				a.writeEndpointError(w, err, "deployment_request_unavailable")
				return true
			}
			api.WriteJSON(w, http.StatusOK, map[string]interface{}{"schema": "alzette.portal.deployment_request.v1", "deployment_request": value})
			return true
		case strings.HasPrefix(r.URL.Path, "/api/portal/deployment-quotes/"):
			return a.serveQuoteAPI(w, r, session)
		case strings.HasPrefix(r.URL.Path, "/api/portal/endpoints/"):
			return a.serveCustomerEndpointAPI(w, r, session)
		}
	}
	if a.billing != nil {
		if r.URL.Path == "/api/portal/billing" && r.Method == http.MethodGet {
			value, err := a.billing.Summary(r.Context(), session)
			if err != nil {
				a.writeEndpointError(w, err, "billing_unavailable")
				return true
			}
			api.WriteJSON(w, http.StatusOK, value)
			return true
		}
		if requirementID, ok := actionPathValue(r.URL.Path, "/api/portal/payment-requirements/", "/checkout-session"); ok && r.Method == http.MethodPost {
			var input struct{}
			if !decodeJSON(w, r, &input) {
				return true
			}
			value, err := a.billing.CreateCheckout(r.Context(), session, requirementID, requestIdempotencyKey(r))
			if err != nil {
				a.writeEndpointError(w, err, "payment_unavailable")
				return true
			}
			api.WriteJSON(w, http.StatusCreated, value)
			return true
		}
		if r.URL.Path == "/api/portal/billing/portal-session" && r.Method == http.MethodPost {
			var input struct{}
			if !decodeJSON(w, r, &input) {
				return true
			}
			value, err := a.billing.CreatePortal(r.Context(), session, requestIdempotencyKey(r))
			if err != nil {
				a.writeEndpointError(w, err, "billing_portal_unavailable")
				return true
			}
			api.WriteJSON(w, http.StatusCreated, value)
			return true
		}
	}
	return false
}

func (a *App) serveConfigurationMutation(w http.ResponseWriter, r *http.Request, session platform.PortalSession) bool {
	if id, ok := actionPathValue(r.URL.Path, "/api/portal/endpoint-configurations/", "/submit"); ok {
		if r.Method != http.MethodPost {
			return false
		}
		var input struct{}
		if !decodeJSON(w, r, &input) {
			return true
		}
		value, err := a.endpoints.Submit(r.Context(), session, id, requestIdempotencyKey(r))
		if err != nil {
			a.writeEndpointError(w, err, "configuration_unavailable")
			return true
		}
		api.WriteJSON(w, http.StatusCreated, map[string]interface{}{"schema": "alzette.portal.endpoint.v1", "endpoint": value})
		return true
	}
	id, ok := singlePathValue(r.URL.Path, "/api/portal/endpoint-configurations/")
	if !ok {
		return false
	}
	if r.Method == http.MethodGet {
		value, err := a.endpoints.Configuration(r.Context(), session, id)
		if err != nil {
			a.writeEndpointError(w, err, "configuration_unavailable")
			return true
		}
		api.WriteJSON(w, http.StatusOK, map[string]interface{}{"schema": "alzette.portal.endpoint_configuration.v1", "configuration": value})
		return true
	}
	if r.Method != http.MethodPatch {
		return false
	}
	var input endpoints.PatchInput
	if !decodeJSON(w, r, &input) {
		return true
	}
	value, err := a.endpoints.Update(r.Context(), session, id, input, requestIdempotencyKey(r))
	if err != nil {
		a.writeEndpointError(w, err, "configuration_unavailable")
		return true
	}
	api.WriteJSON(w, http.StatusOK, map[string]interface{}{"schema": "alzette.portal.endpoint_configuration.v1", "configuration": value})
	return true
}

func (a *App) serveQuoteAPI(w http.ResponseWriter, r *http.Request, session platform.PortalSession) bool {
	if id, ok := actionPathValue(r.URL.Path, "/api/portal/deployment-quotes/", "/accept"); ok {
		if r.Method != http.MethodPost {
			return false
		}
		var input struct{}
		if !decodeJSON(w, r, &input) {
			return true
		}
		value, err := a.endpoints.AcceptQuote(r.Context(), session, id, requestIdempotencyKey(r))
		if err != nil {
			a.writeEndpointError(w, err, "quote_unavailable")
			return true
		}
		api.WriteJSON(w, http.StatusOK, map[string]interface{}{"schema": "alzette.portal.quote_acceptance.v1", "result": value})
		return true
	}
	id, ok := singlePathValue(r.URL.Path, "/api/portal/deployment-quotes/")
	if !ok || r.Method != http.MethodGet {
		return false
	}
	value, err := a.endpoints.Quote(r.Context(), session, id)
	if err != nil {
		a.writeEndpointError(w, err, "quote_unavailable")
		return true
	}
	api.WriteJSON(w, http.StatusOK, map[string]interface{}{"schema": "alzette.portal.deployment_quote.v1", "quote": value})
	return true
}

func (a *App) serveCustomerEndpointAPI(w http.ResponseWriter, r *http.Request, session platform.PortalSession) bool {
	if id, ok := actionPathValue(r.URL.Path, "/api/portal/endpoints/", "/capacity-requests"); ok {
		if r.Method != http.MethodPost {
			return false
		}
		var input struct {
			CapacityUnits int                `json:"capacity_units"`
			Workload      endpoints.Workload `json:"workload"`
		}
		if !decodeJSON(w, r, &input) {
			return true
		}
		value, err := a.endpoints.Capacity(r.Context(), session, id, input.CapacityUnits, input.Workload, requestIdempotencyKey(r))
		if err != nil {
			a.writeEndpointError(w, err, "capacity_unavailable")
			return true
		}
		api.WriteJSON(w, http.StatusCreated, map[string]interface{}{"schema": "alzette.portal.deployment_request.v1", "deployment_request": value})
		return true
	}
	id, ok := singlePathValue(r.URL.Path, "/api/portal/endpoints/")
	if !ok || r.Method != http.MethodGet {
		return false
	}
	value, err := a.endpoints.Get(r.Context(), session, id)
	if err != nil {
		a.writeEndpointError(w, err, "endpoint_unavailable")
		return true
	}
	api.WriteJSON(w, http.StatusOK, map[string]interface{}{"schema": "alzette.portal.endpoint.v1", "endpoint": value})
	return true
}

func singlePathValue(value, prefix string) (string, bool) {
	if !strings.HasPrefix(value, prefix) {
		return "", false
	}
	item := strings.TrimPrefix(value, prefix)
	return item, item != "" && !strings.Contains(item, "/")
}

func actionPathValue(value, prefix, suffix string) (string, bool) {
	if !strings.HasPrefix(value, prefix) || !strings.HasSuffix(value, suffix) {
		return "", false
	}
	item := strings.TrimSuffix(strings.TrimPrefix(value, prefix), suffix)
	return item, item != "" && !strings.Contains(item, "/")
}

func requestIdempotencyKey(r *http.Request) string {
	values := r.Header.Values("Idempotency-Key")
	if len(values) != 1 || strings.TrimSpace(values[0]) == "" || strings.ContainsAny(values[0], "\r\n") {
		return ""
	}
	return values[0]
}

func (a *App) writeEndpointError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, billing.ErrNotConfigured):
		api.WriteError(w, http.StatusServiceUnavailable, "payment_not_configured", "api_error", "hosted billing is not configured", "")
	case errors.Is(err, endpoints.ErrRecentAuthenticationRequired), errors.Is(err, billing.ErrRecentAuthenticationRequired):
		api.WriteError(w, http.StatusUnauthorized, "recent_authentication_required", "authentication_error", "recent sign-in is required", "")
	case errors.Is(err, platform.ErrUnauthenticated):
		api.WriteError(w, http.StatusUnauthorized, "session_required", "authentication_error", "sign-in required", "")
	case errors.Is(err, platform.ErrForbidden):
		api.WriteError(w, http.StatusForbidden, "permission_denied", "permission_error", "permission denied", "")
	case errors.Is(err, platform.ErrInvalid):
		api.WriteError(w, http.StatusBadRequest, "invalid_request", "invalid_request_error", "request is invalid", "")
	case errors.Is(err, platform.ErrNotFound):
		api.WriteError(w, http.StatusNotFound, "not_found", "invalid_request_error", "resource not found", "")
	case errors.Is(err, platform.ErrConflict):
		api.WriteError(w, http.StatusConflict, "state_conflict", "invalid_request_error", "resource conflicts with its current state", "")
	case errors.Is(err, platform.ErrUnavailable):
		api.WriteError(w, http.StatusServiceUnavailable, fallback, "api_error", "resource is temporarily unavailable", "")
	default:
		api.WriteError(w, http.StatusServiceUnavailable, fallback, "api_error", "request is temporarily unavailable", "")
	}
}

func (a *App) session(r *http.Request) (platform.PortalSession, [32]byte, error) {
	var token string
	count := 0
	for _, cookie := range r.Cookies() {
		if cookie.Name == humanauth.SessionCookieName {
			count++
			token = cookie.Value
		}
	}
	if count != 1 || token == "" || len(token) > 128 {
		return platform.PortalSession{}, [32]byte{}, platform.ErrUnauthenticated
	}
	digest := humanauth.Digest(token)
	session, err := a.portalStore.AuthenticatePortalSession(r.Context(), digest, a.clock().UTC())
	return session, digest, err
}

func validCSRF(r *http.Request) bool {
	return validCSRFValue(r, r.Header.Get("X-CSRF-Token"))
}

func validCSRFForm(w http.ResponseWriter, r *http.Request) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/x-www-form-urlencoded" {
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maximumFormBody)
	if err := r.ParseForm(); err != nil {
		return false
	}
	values := r.PostForm["_csrf"]
	return len(values) == 1 && validCSRFValue(r, values[0])
}

func validCSRFValue(r *http.Request, provided string) bool {
	if provided == "" || len(provided) > 128 {
		return false
	}
	count, cookieValue := 0, ""
	for _, cookie := range r.Cookies() {
		if cookie.Name == humanauth.CSRFCookieName {
			count++
			cookieValue = cookie.Value
		}
	}
	return count == 1 && len(cookieValue) == len(provided) && subtle.ConstantTimeCompare([]byte(cookieValue), []byte(provided)) == 1
}

func (a *App) setCookie(w http.ResponseWriter, name, value string, httpOnly bool, expires time.Time) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", HttpOnly: httpOnly, Secure: a.cookieSecure, SameSite: http.SameSiteLaxMode, Expires: expires, MaxAge: int(a.sessionTTL.Seconds())})
}

func (a *App) clearCookies(w http.ResponseWriter) {
	for _, item := range []struct {
		name     string
		httpOnly bool
	}{{humanauth.SessionCookieName, true}, {humanauth.CSRFCookieName, false}} {
		http.SetCookie(w, &http.Cookie{Name: item.name, Value: "", Path: "/", HttpOnly: item.httpOnly, Secure: a.cookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(1, 0)})
	}
}

type gatewayView struct {
	BaseURL            string `json:"base_url"`
	ChatCompletionsURL string `json:"chat_completions_url"`
}

func (a *App) me(w http.ResponseWriter, r *http.Request, session platform.PortalSession) {
	csrf := ""
	if cookie, err := r.Cookie(humanauth.CSRFCookieName); err == nil {
		csrf = cookie.Value
	}
	permissions := []string{"usage:read", "routes:read", "access:read"}
	if session.Current.CanManageAccess() {
		permissions = append(permissions, "access:manage")
	}
	response := map[string]interface{}{
		"schema": "alzette.portal.me.v1", "user": session.User, "context": contextView(session.Current),
		"memberships": membershipViews(session.Memberships), "permissions": permissions, "csrf_token": csrf,
		"session":          map[string]interface{}{"expires_at": session.ExpiresAt},
		"gateway":          gatewayView{BaseURL: a.publicGatewayURL, ChatCompletionsURL: a.chatCompletionsURL},
		"gateway_base_url": a.publicGatewayURL,
		"allowed_scopes":   []string{platform.ScopeInferenceWrite, platform.ScopeRoutesRead, platform.ScopeUsageRead},
	}
	if a.billing != nil {
		response["billing"] = a.billing.Capability()
	}
	api.WriteJSON(w, http.StatusOK, response)
}

func contextView(value platform.PortalMembership) map[string]interface{} {
	return map[string]interface{}{
		"id": value.ID, "membership_id": value.ID, "role": value.Role,
		"organization": map[string]string{"id": value.OrganisationID, "name": value.OrganisationName, "slug": value.OrganisationSlug},
		"organisation": map[string]string{"id": value.OrganisationID, "name": value.OrganisationName, "slug": value.OrganisationSlug},
		"project":      map[string]string{"id": value.ProjectID, "name": value.ProjectName, "slug": value.ProjectSlug},
		"environment":  map[string]string{"id": value.EnvironmentID, "name": value.EnvironmentName, "slug": value.EnvironmentSlug},
	}
}

func membershipViews(values []platform.PortalMembership) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(values))
	for _, value := range values {
		result = append(result, contextView(value))
	}
	return result
}

func (a *App) access(w http.ResponseWriter, r *http.Request, session platform.PortalSession) {
	accounts, err := a.portalStore.ListPortalAccess(r.Context(), session)
	if err != nil {
		api.WriteError(w, http.StatusServiceUnavailable, "access_unavailable", "api_error", "access metadata is temporarily unavailable", "")
		return
	}
	api.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"schema": "alzette.portal.access.v1", "context": session.Current, "can_manage": session.Current.CanManageAccess(),
		"service_accounts": accounts,
		"role":             map[bool]string{true: "admin", false: session.Current.Role}[session.Current.CanManageAccess()],
		"permissions":      map[string]bool{"can_manage_access": session.Current.CanManageAccess()},
		"allowed_scopes":   []string{platform.ScopeInferenceWrite, platform.ScopeRoutesRead, platform.ScopeUsageRead},
		"key_policy":       map[string]interface{}{"name_required": true, "expiry_required": true, "minimum_ttl_seconds": 3600, "maximum_ttl_seconds": 31536000, "default_expiry_days": 90, "allowed_expiry_days": []int{30, 90, 365}, "allowed_scopes": []string{platform.ScopeInferenceWrite, platform.ScopeRoutesRead, platform.ScopeUsageRead}, "rotation_overlap": "old key remains active until explicit revoke"},
	})
}

func (a *App) createServiceAccount(w http.ResponseWriter, r *http.Request, session platform.PortalSession) {
	var input struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	account, err := a.portalStore.CreatePortalServiceAccount(r.Context(), session, input.Name)
	if err != nil {
		a.writeMutationError(w, err, "service account")
		return
	}
	api.WriteJSON(w, http.StatusCreated, map[string]interface{}{"schema": "alzette.portal.service_account.v1", "service_account": account})
}

func (a *App) issueKey(w http.ResponseWriter, r *http.Request, session platform.PortalSession, rotation bool) {
	var input struct {
		ServiceAccountID  string     `json:"service_account_id"`
		Name              string     `json:"name"`
		Scopes            []string   `json:"scopes"`
		ExpiresAt         *time.Time `json:"expires_at"`
		RotatedFromPrefix string     `json:"rotated_from_prefix"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if rotation && input.RotatedFromPrefix == "" {
		api.WriteError(w, http.StatusBadRequest, "invalid_request", "invalid_request_error", "rotated_from_prefix is required for rotation", "")
		return
	}
	if !rotation && input.RotatedFromPrefix != "" {
		api.WriteError(w, http.StatusBadRequest, "invalid_request", "invalid_request_error", "rotated_from_prefix is accepted only by rotation", "")
		return
	}
	result, err := a.portalStore.IssuePortalKey(r.Context(), session, platform.PortalKeyIssueSpec{ServiceAccountID: input.ServiceAccountID, Name: input.Name, Scopes: input.Scopes, ExpiresAt: input.ExpiresAt, RotatedFromPrefix: input.RotatedFromPrefix})
	if err != nil {
		a.writeMutationError(w, err, "API key")
		return
	}
	api.WriteJSON(w, http.StatusCreated, map[string]interface{}{"schema": "alzette.portal.api_key_once.v1", "key": result, "plaintext_available_once": true})
}

func (a *App) revokeKey(w http.ResponseWriter, r *http.Request, session platform.PortalSession) {
	var input struct {
		Prefix string `json:"prefix"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := a.portalStore.RevokePortalKey(r.Context(), session, input.Prefix); err != nil {
		a.writeMutationError(w, err, "API key")
		return
	}
	api.WriteJSON(w, http.StatusOK, map[string]interface{}{"schema": "alzette.portal.api_key_revocation.v1", "prefix": input.Prefix, "revoked": true})
}

func (a *App) switchContext(w http.ResponseWriter, r *http.Request, _ platform.PortalSession, digest [32]byte) {
	var input struct {
		MembershipID string `json:"membership_id"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	session, err := a.portalStore.SwitchPortalContext(r.Context(), digest, input.MembershipID, a.clock().UTC())
	if err != nil {
		a.writeMutationError(w, err, "context")
		return
	}
	api.WriteJSON(w, http.StatusOK, map[string]interface{}{"schema": "alzette.portal.context.v1", "context": session.Current})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination interface{}) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maximumJSONBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		api.WriteError(w, http.StatusBadRequest, "invalid_json", "invalid_request_error", "request body is invalid", "")
		return false
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		api.WriteError(w, http.StatusBadRequest, "invalid_json", "invalid_request_error", "request body is invalid", "")
		return false
	}
	return true
}

func (a *App) writeMutationError(w http.ResponseWriter, err error, resource string) {
	switch {
	case errors.Is(err, platform.ErrForbidden):
		api.WriteError(w, http.StatusForbidden, "insufficient_role", "permission_error", "permission denied", "")
	case errors.Is(err, platform.ErrInvalid):
		api.WriteError(w, http.StatusBadRequest, "invalid_request", "invalid_request_error", resource+" request is invalid", "")
	case errors.Is(err, platform.ErrNotFound):
		api.WriteError(w, http.StatusNotFound, "not_found", "invalid_request_error", resource+" was not found", "")
	case errors.Is(err, platform.ErrConflict):
		api.WriteError(w, http.StatusConflict, "conflict", "invalid_request_error", resource+" conflicts with current state", "")
	default:
		api.WriteError(w, http.StatusServiceUnavailable, "mutation_unavailable", "api_error", "request is temporarily unavailable", "")
	}
}

type portalDashboard struct {
	Schema      string                       `json:"schema"`
	Context     platform.PortalMembership    `json:"context"`
	Gateway     gatewayView                  `json:"gateway"`
	Period      portalPeriod                 `json:"period"`
	Source      portalSource                 `json:"source"`
	ServicePlan platform.PortalServicePlan   `json:"service_plan"`
	Routes      []platform.PortalObservation `json:"routes"`
	Usage       portalUsage                  `json:"usage"`
	Breakdowns  portalBreakdowns             `json:"breakdowns"`
	Trend       []portalTrendPoint           `json:"trend"`
	Recent      []portalRequest              `json:"recent_requests"`
	Export      portalExportAvailability     `json:"export"`
	Rollups     portalRollupSeries           `json:"rollups"`
}

type portalPeriod struct {
	From     time.Time `json:"from"`
	To       time.Time `json:"to"`
	Timezone string    `json:"timezone"`
}
type portalSource struct {
	Kind      string                    `json:"kind"`
	Label     string                    `json:"label"`
	AsOf      time.Time                 `json:"as_of"`
	Freshness string                    `json:"freshness"`
	Finality  string                    `json:"finality"`
	Detail    string                    `json:"detail"`
	Rollup    platform.RollupCheckpoint `json:"rollup"`
}
type nullableTokens struct {
	Input     *int64 `json:"input"`
	Output    *int64 `json:"output"`
	Cached    *int64 `json:"cached"`
	Reasoning *int64 `json:"reasoning"`
	Total     *int64 `json:"total"`
}
type portalTokenMetric struct {
	Value            *int64 `json:"value"`
	KnownRequests    int64  `json:"known_requests"`
	EligibleRequests int64  `json:"eligible_requests"`
	Finality         string `json:"finality"`
}
type portalTokenMetrics struct {
	Input     portalTokenMetric `json:"input"`
	Output    portalTokenMetric `json:"output"`
	Cached    portalTokenMetric `json:"cached"`
	Reasoning portalTokenMetric `json:"reasoning"`
	Total     portalTokenMetric `json:"total"`
}
type portalUsage struct {
	LogicalRequests    int64              `json:"logical_requests"`
	SuccessfulRequests int64              `json:"successful_requests"`
	FailedRequests     int64              `json:"failed_requests"`
	BlockedRequests    int64              `json:"blocked_requests"`
	CancelledRequests  int64              `json:"cancelled_requests"`
	InProgressRequests int64              `json:"in_progress_requests"`
	Tokens             nullableTokens     `json:"tokens"`
	TokenMetrics       portalTokenMetrics `json:"token_metrics"`
	ThroughputRPS      *float64           `json:"throughput_rps"`
	PeakConcurrency    *int64             `json:"peak_concurrency"`
	P50LatencyMS       *int64             `json:"p50_latency_ms"`
	P95LatencyMS       *int64             `json:"p95_latency_ms"`
	SuccessRate        *float64           `json:"success_rate"`
	ErrorRate          *float64           `json:"error_rate"`
	Throughput         *float64           `json:"throughput"`
	Allowance          portalAllocation   `json:"allowance"`
}
type portalBreakdown struct {
	Name          string            `json:"name"`
	Alias         string            `json:"alias,omitempty"`
	ExecutedModel *string           `json:"executed_model"`
	Requests      int64             `json:"requests"`
	Tokens        *int64            `json:"tokens"`
	TokenMetric   portalTokenMetric `json:"token_metric"`
	Share         *float64          `json:"share"`
}
type portalBreakdowns struct {
	ServiceAccounts []portalBreakdown `json:"service_accounts"`
	Models          []portalBreakdown `json:"models"`
	Projects        []portalBreakdown `json:"projects"`
}
type portalRequest struct {
	RequestID      string     `json:"request_id"`
	StartedAt      time.Time  `json:"started_at"`
	OccurredAt     time.Time  `json:"occurred_at"`
	CompletedAt    *time.Time `json:"completed_at"`
	ServiceAccount string     `json:"service_account"`
	Project        string     `json:"project"`
	ModelAlias     string     `json:"model_alias"`
	ExecutedModel  string     `json:"executed_model,omitempty"`
	Status         string     `json:"status"`
	ErrorClass     *string    `json:"error_class"`
	DurationMS     *int64     `json:"duration_ms"`
	LatencyMS      *int64     `json:"latency_ms"`
	Tokens         *int64     `json:"tokens"`
	UsageFinality  string     `json:"usage_finality"`
}
type portalAllocation struct {
	Shared    interface{} `json:"shared"`
	Dedicated interface{} `json:"dedicated"`
	Source    string      `json:"source"`
	Finality  string      `json:"finality"`
	Detail    string      `json:"detail"`
}
type portalTrendPoint struct {
	BucketStart        time.Time         `json:"bucket_start"`
	LogicalRequests    int64             `json:"logical_requests"`
	SuccessfulRequests int64             `json:"successful_requests"`
	FailedRequests     int64             `json:"failed_requests"`
	BlockedRequests    int64             `json:"blocked_requests"`
	CancelledRequests  int64             `json:"cancelled_requests"`
	InProgressRequests int64             `json:"in_progress_requests"`
	Tokens             portalTokenMetric `json:"tokens"`
	P95LatencyMS       *int64            `json:"p95_latency_ms"`
	Finality           string            `json:"finality"`
	Source             string            `json:"source"`
}
type portalExportAvailability struct {
	Available bool     `json:"available"`
	Formats   []string `json:"formats"`
	Scope     string   `json:"scope"`
}
type portalRollupSeries struct {
	CoverageFrom          time.Time                    `json:"coverage_from"`
	CoverageTo            time.Time                    `json:"coverage_to"`
	RangeFullyRepresented bool                         `json:"range_fully_represented"`
	Freshness             string                       `json:"freshness"`
	Rows                  []platform.PortalUsageRollup `json:"rows"`
}

func (a *App) dashboard(w http.ResponseWriter, r *http.Request, session platform.PortalSession) {
	dashboard, loadErr := a.loadDashboard(r, session)
	if loadErr != nil {
		api.WriteError(w, loadErr.status, loadErr.code, loadErr.errorType, loadErr.message, "")
		return
	}
	api.WriteJSON(w, http.StatusOK, dashboard)
}

type dashboardLoadError struct {
	status    int
	code      string
	errorType string
	message   string
}

func (e *dashboardLoadError) Error() string { return e.code }

func (a *App) loadDashboard(r *http.Request, session platform.PortalSession) (portalDashboard, *dashboardLoadError) {
	return a.loadDashboardWithOptions(r, session, dashboardLoadOptions{})
}

type dashboardLoadOptions struct {
	tolerateRollupFailure bool
}

func (a *App) loadDashboardWithOptions(r *http.Request, session platform.PortalSession, options dashboardLoadOptions) (portalDashboard, *dashboardLoadError) {
	filter, err := parseUsageFilter(r, a.clock().UTC(), false)
	if err != nil {
		return portalDashboard{}, &dashboardLoadError{http.StatusBadRequest, "invalid_period", "invalid_request_error", "usage period is invalid"}
	}
	filter.Limit = maximumRows
	page, err := a.store.ListInferenceRequests(r.Context(), session.Current.Principal(), filter)
	if err != nil || page.Truncated {
		return portalDashboard{}, &dashboardLoadError{http.StatusUnprocessableEntity, "usage_unavailable", "api_error", "select a narrower usage period"}
	}
	accounts, err := a.portalStore.ListPortalAccess(r.Context(), session)
	if err != nil {
		return portalDashboard{}, &dashboardLoadError{http.StatusServiceUnavailable, "usage_unavailable", "api_error", "usage is temporarily unavailable"}
	}
	accountNames := make(map[string]string)
	for _, account := range accounts {
		accountNames[account.ID] = account.Name
	}
	plan, err := a.portalStore.GetPortalServicePlan(r.Context(), session, filter.ModelAlias)
	if err != nil {
		return portalDashboard{}, &dashboardLoadError{http.StatusServiceUnavailable, "plan_unavailable", "api_error", "service plan is temporarily unavailable"}
	}
	now := a.clock().UTC()
	observations, err := a.portalStore.ListPortalObservations(r.Context(), session, filter.ModelAlias, now)
	if err != nil {
		return portalDashboard{}, &dashboardLoadError{http.StatusServiceUnavailable, "routes_unavailable", "api_error", "route observations are temporarily unavailable"}
	}
	checkpoint, err := a.portalStore.GetRollupCheckpoint(r.Context(), session)
	if err != nil {
		checkpoint = platform.RollupCheckpoint{Status: "unavailable"}
	}
	rollupRows, err := a.portalStore.ListPortalRollups(r.Context(), session, filter)
	rollupUnavailable := err != nil
	if rollupUnavailable {
		if !options.tolerateRollupFailure {
			return portalDashboard{}, &dashboardLoadError{http.StatusServiceUnavailable, "rollups_unavailable", "api_error", "usage rollups are temporarily unavailable"}
		}
		rollupRows = []platform.PortalUsageRollup{}
	}
	scopeLabel := session.Current.ProjectName + " / " + session.Current.EnvironmentName
	usage, breakdowns, recent, partial := buildPortalUsage(page.Requests, accountNames, scopeLabel, filter, now)
	usage.Allowance = allocationFromPlan(plan)
	coverageFrom := filter.From.UTC().Truncate(time.Hour)
	coverageTo := filter.To.UTC().Truncate(time.Hour)
	if !coverageTo.Equal(filter.To.UTC()) {
		coverageTo = coverageTo.Add(time.Hour)
	}
	rangeFullyRepresented := filter.From.Equal(coverageFrom) && filter.To.Equal(coverageTo) &&
		checkpoint.Status == "succeeded" && checkpoint.LastCompletedAt != nil && checkpoint.RangeFrom != nil && checkpoint.RangeTo != nil &&
		!checkpoint.RangeFrom.After(coverageFrom) && !checkpoint.RangeTo.Before(coverageTo) && checkpoint.LastCompletedAt.After(now.Add(-5*time.Minute))
	detail := "Logical request ledger queried for the authenticated project/environment. Route inference evidence is current-binding and tenant-scoped; compatible probe evidence is target-shared and separately labelled."
	finality := "final"
	if len(page.Requests) == 0 && !filter.To.After(now) {
		detail = "The authenticated project/environment logical ledger was queried at the reported as-of time and contained zero requests in this period. Token totals are exact zero/not-applicable; rollup worker freshness is reported separately."
	} else if partial || filter.To.After(now) {
		finality = "partial"
		detail = "Partial logical-ledger snapshot: in-progress, incomplete usage, or a future endpoint is present. Unknown token totals remain null."
	}
	rollupFreshness := checkpointFreshness(checkpoint, now)
	if rollupUnavailable {
		rollupFreshness = "unavailable"
		rangeFullyRepresented = false
	}
	return portalDashboard{
		Schema: "alzette.portal.dashboard.v1", Context: session.Current,
		Gateway:     gatewayView{BaseURL: a.publicGatewayURL, ChatCompletionsURL: a.chatCompletionsURL},
		Period:      portalPeriod{From: filter.From, To: filter.To, Timezone: "UTC"},
		Source:      portalSource{Kind: "inference_requests", Label: "Authenticated logical request ledger", AsOf: now, Freshness: "fresh", Finality: finality, Detail: detail, Rollup: checkpoint},
		ServicePlan: plan, Routes: observations, Usage: usage, Breakdowns: breakdowns, Trend: buildDirectTrend(page.Requests), Recent: recent,
		Export:  portalExportAvailability{Available: finality == "final", Formats: []string{"csv", "json"}, Scope: "authenticated_project_environment"},
		Rollups: portalRollupSeries{CoverageFrom: coverageFrom, CoverageTo: coverageTo, RangeFullyRepresented: rangeFullyRepresented, Freshness: rollupFreshness, Rows: rollupRows},
	}, nil
}

func allocationFromPlan(plan platform.PortalServicePlan) portalAllocation {
	result := portalAllocation{Source: plan.Source, Finality: plan.Finality, Detail: "No operator-entered route-bound service-plan value is available."}
	if !plan.Available || plan.Ambiguous {
		return result
	}
	result.Detail = "Operator-entered service-plan evidence bound to the selected model route. Nullable values are not inferred."
	if plan.CapacityMode == "shared" {
		result.Shared = map[string]interface{}{
			"logical_requests":         map[string]interface{}{"value": plan.SharedRequestAllowance, "unit": plan.SharedRequestAllowanceUnit, "period": plan.SharedRequestAllowancePeriod},
			"provider_reported_tokens": map[string]interface{}{"value": plan.SharedTokenAllowance, "unit": plan.SharedTokenAllowanceUnit, "period": plan.SharedTokenAllowancePeriod},
		}
	} else if plan.CapacityMode == "dedicated" {
		result.Dedicated = map[string]interface{}{"resource_class": plan.DedicatedResourceClass, "accelerator_count": plan.DedicatedAcceleratorCount}
	}
	return result
}

func checkpointFreshness(checkpoint platform.RollupCheckpoint, now time.Time) string {
	if checkpoint.LastCompletedAt == nil {
		return "unavailable"
	}
	if checkpoint.LastCompletedAt.Before(now.Add(-5 * time.Minute)) {
		return "stale"
	}
	return "fresh"
}

func buildPortalUsage(requests []platform.InferenceRequest, accountNames map[string]string, projectEnvironmentLabel string, filter platform.UsageFilter, now time.Time) (portalUsage, portalBreakdowns, []portalRequest, bool) {
	result := portalUsage{LogicalRequests: int64(len(requests))}
	seconds := filter.To.Sub(filter.From).Seconds()
	throughput := float64(len(requests)) / seconds
	result.ThroughputRPS = &throughput
	result.Throughput = &throughput
	peak := int64(peakRequests(requests, now))
	result.PeakConcurrency = &peak
	partial := false
	usageTokenPartial := false
	input, output, cached, reasoning := int64(0), int64(0), int64(0), int64(0)
	inputKnown, outputKnown, cachedKnown, reasoningKnown := int64(0), int64(0), int64(0), int64(0)
	totalKnown := int64(0)
	var durations []int64
	accountGroups := make(map[string][]platform.InferenceRequest)
	modelGroups := make(map[string][]platform.InferenceRequest)
	recent := make([]portalRequest, 0, minInt(20, len(requests)))
	for index, request := range requests {
		switch request.Status {
		case "succeeded":
			result.SuccessfulRequests++
		case "failed":
			result.FailedRequests++
		case "blocked":
			result.BlockedRequests++
		case "cancelled":
			result.CancelledRequests++
		default:
			result.InProgressRequests++
			partial = true
		}
		if request.Status == "succeeded" {
			if request.Usage.InputTokens != nil {
				input += *request.Usage.InputTokens
				inputKnown++
			}
			if request.Usage.OutputTokens != nil {
				output += *request.Usage.OutputTokens
				outputKnown++
			}
			if request.Usage.CachedTokens != nil {
				cached += *request.Usage.CachedTokens
				cachedKnown++
			}
			if request.Usage.ReasoningTokens != nil {
				reasoning += *request.Usage.ReasoningTokens
				reasoningKnown++
			}
			if request.Usage.InputTokens != nil && request.Usage.OutputTokens != nil {
				totalKnown++
			}
			if request.UsageFinality != "final" {
				partial = true
				usageTokenPartial = true
			}
		}
		if request.CompletedAt != nil {
			durations = append(durations, request.Duration.Milliseconds())
		}
		accountGroups[request.ServiceAccountID] = append(accountGroups[request.ServiceAccountID], request)
		modelGroups[request.ModelAlias] = append(modelGroups[request.ModelAlias], request)
		if index < 20 {
			var errorClass *string
			if request.ErrorClass != "" {
				value := request.ErrorClass
				errorClass = &value
			}
			var duration *int64
			if request.CompletedAt != nil {
				value := request.Duration.Milliseconds()
				duration = &value
			}
			recent = append(recent, portalRequest{RequestID: request.ID, StartedAt: request.StartedAt, OccurredAt: request.StartedAt, CompletedAt: request.CompletedAt, ServiceAccount: accountNames[request.ServiceAccountID], Project: projectEnvironmentLabel, ModelAlias: request.ModelAlias, ExecutedModel: request.ExecutedModel, Status: request.Status, ErrorClass: errorClass, DurationMS: duration, LatencyMS: duration, Tokens: requestTokenTotal(request), UsageFinality: request.UsageFinality})
		}
	}
	result.Tokens.Input = completeMetric(input, inputKnown, result.SuccessfulRequests)
	result.Tokens.Output = completeMetric(output, outputKnown, result.SuccessfulRequests)
	result.Tokens.Cached = completeMetric(cached, cachedKnown, result.SuccessfulRequests)
	result.Tokens.Reasoning = completeMetric(reasoning, reasoningKnown, result.SuccessfulRequests)
	if result.Tokens.Input != nil && result.Tokens.Output != nil {
		value := *result.Tokens.Input + *result.Tokens.Output
		result.Tokens.Total = &value
	}
	result.TokenMetrics.Input = tokenMetric(input, inputKnown, result.SuccessfulRequests, usageTokenPartial)
	result.TokenMetrics.Output = tokenMetric(output, outputKnown, result.SuccessfulRequests, usageTokenPartial)
	result.TokenMetrics.Cached = tokenMetric(cached, cachedKnown, result.SuccessfulRequests, usageTokenPartial)
	result.TokenMetrics.Reasoning = tokenMetric(reasoning, reasoningKnown, result.SuccessfulRequests, usageTokenPartial)
	result.TokenMetrics.Total = tokenMetric(input+output, totalKnown, result.SuccessfulRequests, usageTokenPartial)
	if result.LogicalRequests == 0 {
		zero := int64(0)
		result.Tokens = nullableTokens{Input: &zero, Output: &zero, Cached: &zero, Reasoning: &zero, Total: &zero}
		result.TokenMetrics = portalTokenMetrics{
			Input: emptyTokenMetric(), Output: emptyTokenMetric(), Cached: emptyTokenMetric(),
			Reasoning: emptyTokenMetric(), Total: emptyTokenMetric(),
		}
	}
	result.P50LatencyMS = durationPercentile(durations, .50)
	result.P95LatencyMS = durationPercentile(durations, .95)
	result.SuccessRate = ratio(result.SuccessfulRequests, result.LogicalRequests)
	result.ErrorRate = ratio(result.FailedRequests+result.CancelledRequests, result.LogicalRequests)
	breakdowns := portalBreakdowns{ServiceAccounts: makeBreakdowns(accountGroups, accountNames), Models: makeModelBreakdowns(modelGroups)}
	breakdowns.Projects = []portalBreakdown{{Name: projectEnvironmentLabel, Requests: result.LogicalRequests, Tokens: result.Tokens.Total, TokenMetric: result.TokenMetrics.Total, Share: ratio(result.LogicalRequests, result.LogicalRequests)}}
	return result, breakdowns, recent, partial
}

func emptyTokenMetric() portalTokenMetric {
	zero := int64(0)
	return portalTokenMetric{Value: &zero, Finality: "not_applicable"}
}

func ratio(value, total int64) *float64 {
	if total == 0 {
		return nil
	}
	result := float64(value) / float64(total) * 100
	return &result
}

func makeBreakdowns(groups map[string][]platform.InferenceRequest, names map[string]string) []portalBreakdown {
	var allRequests int64
	for _, requests := range groups {
		allRequests += int64(len(requests))
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]portalBreakdown, 0, len(keys))
	for _, key := range keys {
		name := key
		if names != nil {
			name = names[key]
		}
		var total, known, eligible int64
		groupPartial := false
		for _, request := range groups[key] {
			if request.Status != "succeeded" {
				continue
			}
			eligible++
			if request.UsageFinality != "final" {
				groupPartial = true
			}
			if value := requestTokenTotal(request); value != nil {
				total += *value
				known++
			}
		}
		metric := tokenMetric(total, known, eligible, groupPartial)
		result = append(result, portalBreakdown{Name: name, Requests: int64(len(groups[key])), Tokens: metric.Value, TokenMetric: metric, Share: ratio(int64(len(groups[key])), allRequests)})
	}
	return result
}

func makeModelBreakdowns(aliasGroups map[string][]platform.InferenceRequest) []portalBreakdown {
	type modelKey struct{ alias, executed string }
	groups := make(map[modelKey][]platform.InferenceRequest)
	var allRequests int64
	for alias, requests := range aliasGroups {
		for _, request := range requests {
			key := modelKey{alias: alias, executed: request.ExecutedModel}
			groups[key] = append(groups[key], request)
			allRequests++
		}
	}
	keys := make([]modelKey, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].alias == keys[j].alias {
			return keys[i].executed < keys[j].executed
		}
		return keys[i].alias < keys[j].alias
	})
	result := make([]portalBreakdown, 0, len(keys))
	for _, key := range keys {
		requests := groups[key]
		var total, known, eligible int64
		partial := false
		for _, request := range requests {
			if request.Status != "succeeded" {
				continue
			}
			eligible++
			if request.UsageFinality != "final" {
				partial = true
			}
			if value := requestTokenTotal(request); value != nil {
				total += *value
				known++
			}
		}
		metric := tokenMetric(total, known, eligible, partial)
		var executed *string
		if key.executed != "" {
			value := key.executed
			executed = &value
		}
		result = append(result, portalBreakdown{Name: key.alias, Alias: key.alias, ExecutedModel: executed, Requests: int64(len(requests)), Tokens: metric.Value, TokenMetric: metric, Share: ratio(int64(len(requests)), allRequests)})
	}
	return result
}

func requestTokenTotal(request platform.InferenceRequest) *int64 {
	if request.Status != "succeeded" || request.Usage.InputTokens == nil || request.Usage.OutputTokens == nil {
		return nil
	}
	value := *request.Usage.InputTokens + *request.Usage.OutputTokens
	return &value
}
func completeMetric(total, known, eligible int64) *int64 {
	if eligible == 0 || known != eligible {
		return nil
	}
	value := total
	return &value
}
func tokenMetric(total, known, eligible int64, forcedPartial bool) portalTokenMetric {
	finality := "unknown"
	var value *int64
	if eligible > 0 {
		finality = "partial"
		if known == eligible {
			copy := total
			value = &copy
			finality = "final"
		}
	}
	if forcedPartial && eligible > 0 {
		finality = "partial"
	}
	return portalTokenMetric{Value: value, KnownRequests: known, EligibleRequests: eligible, Finality: finality}
}

func durationPercentile(values []int64, fraction float64) *int64 {
	if len(values) == 0 {
		return nil
	}
	copy := append([]int64(nil), values...)
	sort.Slice(copy, func(i, j int) bool { return copy[i] < copy[j] })
	index := int(float64(len(copy)-1) * fraction)
	if fraction >= .95 && index < len(copy)-1 {
		index++
	}
	value := copy[index]
	return &value
}

func buildDirectTrend(requests []platform.InferenceRequest) []portalTrendPoint {
	byBucket := make(map[time.Time][]platform.InferenceRequest)
	for _, request := range requests {
		bucket := request.StartedAt.UTC().Truncate(time.Hour)
		byBucket[bucket] = append(byBucket[bucket], request)
	}
	buckets := make([]time.Time, 0, len(byBucket))
	for bucket := range byBucket {
		buckets = append(buckets, bucket)
	}
	sort.Slice(buckets, func(i, j int) bool { return buckets[i].Before(buckets[j]) })
	result := make([]portalTrendPoint, 0, len(buckets))
	for _, bucket := range buckets {
		point := portalTrendPoint{BucketStart: bucket, Finality: "final", Source: "inference_requests"}
		var total, known, eligible int64
		var durations []int64
		forcedPartial := false
		for _, request := range byBucket[bucket] {
			point.LogicalRequests++
			switch request.Status {
			case "succeeded":
				point.SuccessfulRequests++
				eligible++
				if value := requestTokenTotal(request); value != nil {
					total += *value
					known++
				}
				if request.UsageFinality != "final" {
					forcedPartial = true
				}
			case "failed":
				point.FailedRequests++
			case "blocked":
				point.BlockedRequests++
			case "cancelled":
				point.CancelledRequests++
			default:
				point.InProgressRequests++
				forcedPartial = true
			}
			if request.CompletedAt != nil {
				durations = append(durations, request.Duration.Milliseconds())
			}
		}
		point.Tokens = tokenMetric(total, known, eligible, forcedPartial)
		point.P95LatencyMS = durationPercentile(durations, .95)
		if forcedPartial {
			point.Finality = "partial"
		}
		result = append(result, point)
	}
	return result
}
func peakRequests(requests []platform.InferenceRequest, now time.Time) int {
	type event struct {
		at    time.Time
		delta int
	}
	events := make([]event, 0, len(requests)*2)
	for _, request := range requests {
		end := now
		if request.CompletedAt != nil {
			end = *request.CompletedAt
		}
		if end.After(request.StartedAt) {
			events = append(events, event{request.StartedAt, 1}, event{end, -1})
		}
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].at.Equal(events[j].at) {
			return events[i].delta < events[j].delta
		}
		return events[i].at.Before(events[j].at)
	})
	current, peak := 0, 0
	for _, e := range events {
		current += e.delta
		if current > peak {
			peak = current
		}
	}
	return peak
}

func parseUsageFilter(r *http.Request, now time.Time, export bool) (platform.UsageFilter, error) {
	allowed := map[string]bool{"from": true, "to": true, "model": true}
	if export {
		allowed["format"] = true
	}
	for key, values := range r.URL.Query() {
		if !allowed[key] || len(values) != 1 {
			return platform.UsageFilter{}, platform.ErrInvalid
		}
	}
	to := now
	from := to.Add(-24 * time.Hour)
	var err error
	if value := r.URL.Query().Get("from"); value != "" {
		from, err = time.Parse(time.RFC3339, value)
		if err != nil {
			return platform.UsageFilter{}, err
		}
	}
	if value := r.URL.Query().Get("to"); value != "" {
		to, err = time.Parse(time.RFC3339, value)
		if err != nil {
			return platform.UsageFilter{}, err
		}
	}
	if !to.After(from) || to.Sub(from) > 31*24*time.Hour {
		return platform.UsageFilter{}, platform.ErrInvalid
	}
	model := r.URL.Query().Get("model")
	if len(model) > 128 {
		return platform.UsageFilter{}, platform.ErrInvalid
	}
	return platform.UsageFilter{From: from.UTC(), To: to.UTC(), ModelAlias: model}, nil
}

type usageExportEnvelope struct {
	Schema      string                     `json:"schema"`
	GeneratedAt time.Time                  `json:"generated_at"`
	Scope       platform.PortalMembership  `json:"scope"`
	Period      portalPeriod               `json:"period"`
	Units       []string                   `json:"units"`
	Source      portalSource               `json:"source"`
	Context     usageExportContext         `json:"context"`
	Rows        []platform.PortalExportRow `json:"rows"`
}

type usageExportContext struct {
	Semantics   string                       `json:"semantics"`
	Routes      []platform.PortalObservation `json:"routes"`
	ServicePlan platform.PortalServicePlan   `json:"service_plan"`
	Allocation  portalAllocation             `json:"allocation"`
}

func (a *App) export(w http.ResponseWriter, r *http.Request, session platform.PortalSession) {
	format := r.URL.Query().Get("format")
	if format != "csv" && format != "json" {
		api.WriteError(w, http.StatusBadRequest, "invalid_format", "invalid_request_error", "format must be csv or json", "")
		return
	}
	filter, err := parseUsageFilter(r, a.clock().UTC(), true)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "invalid_period", "invalid_request_error", "usage period is invalid", "")
		return
	}
	filter.Limit = maximumRows
	rows, err := a.portalStore.ListPortalExport(r.Context(), session, filter, format)
	if err != nil {
		a.writeMutationError(w, err, "usage export")
		return
	}
	if rows == nil {
		rows = []platform.PortalExportRow{}
	}
	now := a.clock().UTC()
	plan, err := a.portalStore.GetPortalServicePlan(r.Context(), session, filter.ModelAlias)
	if err != nil {
		api.WriteError(w, http.StatusServiceUnavailable, "plan_unavailable", "api_error", "service plan is temporarily unavailable", "")
		return
	}
	routes, err := a.portalStore.ListPortalObservations(r.Context(), session, filter.ModelAlias, now)
	if err != nil {
		api.WriteError(w, http.StatusServiceUnavailable, "routes_unavailable", "api_error", "route context is temporarily unavailable", "")
		return
	}
	finality := "final"
	for _, row := range rows {
		if row.Status == "in_progress" || (row.Status == "succeeded" && row.UsageFinality != "final") {
			finality = "partial"
			break
		}
	}
	if filter.To.After(now) {
		finality = "partial"
	}
	units := []string{"logical_requests", "milliseconds", "provider_reported_tokens"}
	source := portalSource{Kind: "inference_requests", Label: "Authenticated logical request ledger", AsOf: now, Freshness: "fresh", Finality: finality, Detail: "Authenticated project/environment logical request ledger export. Per-row route fields use the immutable bound model/target where an attempt exists; current plan/allocation context is separately labelled and is not inferred historically."}
	context := usageExportContext{Semantics: "current_route_and_plan_context_not_historical", Routes: routes, ServicePlan: plan, Allocation: allocationFromPlan(plan)}
	w.Header().Set("X-Alzette-Export-Schema", "alzette.portal.usage_export.v1")
	w.Header().Set("X-Alzette-Export-Timezone", "UTC")
	if format == "json" {
		api.WriteJSON(w, http.StatusOK, usageExportEnvelope{Schema: "alzette.portal.usage_export.v1", GeneratedAt: now, Scope: session.Current, Period: portalPeriod{filter.From, filter.To, "UTC"}, Units: units, Source: source, Context: context, Rows: rows})
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="alzette-usage.csv"`)
	w.WriteHeader(http.StatusOK)
	writer := csv.NewWriter(w)
	writeCSVRow(writer, []string{"schema", "alzette.portal.usage_export.v1"})
	writeCSVRow(writer, []string{"scope", session.Current.OrganisationName + " / " + session.Current.ProjectName + " / " + session.Current.EnvironmentName})
	writeCSVRow(writer, []string{"period", filter.From.Format(time.RFC3339), filter.To.Format(time.RFC3339), "UTC"})
	writeCSVRow(writer, []string{"generated_at", now.Format(time.RFC3339)})
	writeCSVRow(writer, []string{"units", strings.Join(units, "|")})
	writeCSVRow(writer, []string{"source", "inference_requests", finality, now.Format(time.RFC3339)})
	writeCSVRow(writer, []string{"context_semantics", context.Semantics})
	for _, route := range routes {
		writeCSVRow(writer, []string{"current_route", route.ModelAlias, route.ModelVersion, route.ExecutionClass, route.CapacityMode, route.RegistryStatus, route.State})
	}
	writeCSVRow(writer, []string{"current_service_plan", plan.ModelAlias, plan.Code, plan.Name, plan.CapacityMode, plan.Status, plan.Source, plan.Finality, formatTime(plan.EffectiveAt)})
	writeCSVRow(writer, []string{"shared_request_allowance", formatInt64Pointer(plan.SharedRequestAllowance), formatStringPointer(plan.SharedRequestAllowanceUnit), formatStringPointer(plan.SharedRequestAllowancePeriod)})
	writeCSVRow(writer, []string{"shared_token_allowance", formatInt64Pointer(plan.SharedTokenAllowance), formatStringPointer(plan.SharedTokenAllowanceUnit), formatStringPointer(plan.SharedTokenAllowancePeriod)})
	writeCSVRow(writer, []string{"dedicated_allocation", formatStringPointer(plan.DedicatedResourceClass), formatInt64Pointer(plan.DedicatedAcceleratorCount)})
	writeCSVRow(writer, []string{"request_id", "started_at", "completed_at", "service_account", "model_alias", "model_version", "executed_model", "execution_class", "capacity_mode", "status", "http_status", "error_class", "duration_ms", "input_tokens", "output_tokens", "cached_tokens", "reasoning_tokens", "usage_finality"})
	for _, row := range rows {
		writeCSVRow(writer, []string{row.RequestID, row.StartedAt.Format(time.RFC3339), formatTime(row.CompletedAt), row.ServiceAccount, row.ModelAlias, formatStringPointer(row.ModelVersion), row.ExecutedModel, formatStringPointer(row.ExecutionClass), formatStringPointer(row.CapacityMode), row.Status, formatInt(row.HTTPStatus), row.ErrorClass, formatIntPointer(row.DurationMS), formatIntPointer(row.InputTokens), formatIntPointer(row.OutputTokens), formatIntPointer(row.CachedTokens), formatIntPointer(row.ReasoningTokens), row.UsageFinality})
	}
	writer.Flush()
}

func writeCSVRow(writer *csv.Writer, cells []string) {
	safe := make([]string, len(cells))
	for index, cell := range cells {
		safe[index] = spreadsheetSafe(cell)
	}
	_ = writer.Write(safe)
}

func spreadsheetSafe(value string) string {
	candidate := strings.TrimLeft(value, " \t\r\n")
	if candidate != "" && strings.ContainsRune("=+-@", rune(candidate[0])) {
		return "'" + value
	}
	if value != "" && (value[0] == '\t' || value[0] == '\r' || value[0] == '\n') {
		return "'" + value
	}
	return value
}

func formatTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
func formatInt(value int) string {
	if value == 0 {
		return ""
	}
	return strconv.Itoa(value)
}
func formatIntPointer(value *int64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatInt(*value, 10)
}
func formatInt64Pointer(value *int64) string { return formatIntPointer(value) }
func formatStringPointer(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
