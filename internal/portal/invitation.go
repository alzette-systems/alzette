package portal

import (
	"crypto/subtle"
	"embed"
	"html/template"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"alzette/internal/api"
	"alzette/internal/humanauth"
	"alzette/internal/workforce"
)

const (
	setupCookieSecureName = "__Host-alzette_setup"
	setupCookieDevName    = "alzette_setup"
	oidcStateCookieName   = "alzette_oidc_state"
)

//go:embed templates/invitation.html
var invitationTemplateFS embed.FS

var invitationTemplate = template.Must(template.New("invitation.html").Option("missingkey=error").ParseFS(invitationTemplateFS, "templates/invitation.html"))

type invitationPageView struct {
	Available bool
	Error     string
}

func (a *App) invitationEntry(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		values, hasToken := r.URL.Query()["token"]
		if hasToken {
			if len(values) != 1 || len(values[0]) < 32 || len(values[0]) > 512 || len(r.URL.Query()) != 1 || a.workforce == nil {
				a.renderInvitationPage(w, http.StatusBadRequest, "This invitation link is invalid or no longer available.")
				return
			}
			setup, err := a.workforce.BeginInvitationSetup(r.Context(), values[0], a.clock().UTC())
			if err != nil {
				a.renderInvitationPage(w, http.StatusBadRequest, "This invitation link is invalid or no longer available.")
				return
			}
			a.setActionCookie(w, a.setupCookieName(), setup.Token, setup.ExpiresAt, http.SameSiteStrictMode)
			http.Redirect(w, r, "/accept-invite", http.StatusSeeOther)
			return
		}
		a.renderInvitationPage(w, http.StatusOK, "")
	case http.MethodPost:
		if a.workforce == nil || a.oidc == nil {
			a.renderInvitationPage(w, http.StatusServiceUnavailable, "Employee federated sign-in is not configured on this deployment.")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maximumFormBody)
		if err := r.ParseForm(); err != nil || r.PostForm.Get("intent") != "continue" || len(r.PostForm) != 1 {
			a.renderInvitationPage(w, http.StatusBadRequest, "The invitation continuation request was invalid. Reload the invitation link and try again.")
			return
		}
		setup, ok := singleCookie(r, a.setupCookieName())
		if !ok {
			a.renderInvitationPage(w, http.StatusBadRequest, "The invitation setup session expired. Open the original invitation link again.")
			return
		}
		state, nonce, verifier := oauth2.GenerateVerifier(), oauth2.GenerateVerifier(), oauth2.GenerateVerifier()
		now := a.clock().UTC()
		if err := a.workforce.CreateOIDCTransaction(r.Context(), setup, state, nonce, verifier, now); err != nil {
			a.renderInvitationPage(w, http.StatusBadRequest, "The invitation setup session expired. Open the original invitation link again.")
			return
		}
		a.setActionCookie(w, oidcStateCookieName, state, now.Add(10*time.Minute), http.SameSiteLaxMode)
		http.Redirect(w, r, a.oidc.AuthorizationURL(state, nonce, verifier), http.StatusSeeOther)
	default:
		api.MethodNotAllowed(w, "GET, POST", "")
	}
}

func (a *App) oidcCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || a.workforce == nil || a.oidc == nil {
		http.NotFound(w, r)
		return
	}
	states, codes := r.URL.Query()["state"], r.URL.Query()["code"]
	cookieState, ok := singleCookie(r, oidcStateCookieName)
	if len(states) != 1 || len(codes) != 1 || len(r.URL.Query()) != 2 || !ok || len(states[0]) != len(cookieState) || subtle.ConstantTimeCompare([]byte(states[0]), []byte(cookieState)) != 1 {
		a.renderInvitationPage(w, http.StatusBadRequest, "Company sign-in could not be verified. Open the invitation link and try again.")
		return
	}
	now := a.clock().UTC()
	transaction, err := a.workforce.ConsumeOIDCTransaction(r.Context(), states[0], now)
	if err != nil {
		a.renderInvitationPage(w, http.StatusBadRequest, "Company sign-in could not be verified. Open the invitation link and try again.")
		return
	}
	identity, err := a.oidc.Exchange(r.Context(), codes[0], transaction.Verifier, transaction.Nonce)
	if err != nil || !identity.EmailVerified {
		a.renderInvitationPage(w, http.StatusUnauthorized, "Company sign-in did not return the exact verified identity required by this invitation.")
		return
	}
	sessionToken, err := a.generateSessionToken()
	if err != nil {
		a.renderInvitationPage(w, http.StatusServiceUnavailable, "Employee access is temporarily unavailable. Open the invitation link and try again later.")
		return
	}
	csrfToken, err := a.generateCSRFToken()
	if err != nil {
		a.renderInvitationPage(w, http.StatusServiceUnavailable, "Employee access is temporarily unavailable. Open the invitation link and try again later.")
		return
	}
	expiresAt := now.Add(a.sessionTTL)
	_, err = a.workforce.AcceptInvitation(r.Context(), transaction.ActionSessionID, workforce.FederatedIdentity{Issuer: identity.Issuer, Subject: identity.Subject, Email: identity.Email, DisplayName: identity.DisplayName}, humanauth.Digest(sessionToken), expiresAt, now)
	if err != nil {
		a.renderInvitationPage(w, http.StatusForbidden, "This identity cannot accept the invitation. Confirm the exact invited email or ask the owner to resend it.")
		return
	}
	a.setCookie(w, humanauth.SessionCookieName, sessionToken, true, expiresAt)
	a.setCookie(w, humanauth.CSRFCookieName, csrfToken, false, expiresAt)
	a.clearActionCookie(w, a.setupCookieName(), http.SameSiteStrictMode)
	a.clearActionCookie(w, oidcStateCookieName, http.SameSiteLaxMode)
	http.Redirect(w, r, "/app/access/people", http.StatusSeeOther)
}

func (a *App) renderInvitationPage(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", a.csp)
	w.WriteHeader(status)
	_ = invitationTemplate.ExecuteTemplate(w, "invitation.html", invitationPageView{Available: a.oidc != nil, Error: message})
}

func (a *App) setupCookieName() string {
	if a.cookieSecure {
		return setupCookieSecureName
	}
	return setupCookieDevName
}

func (a *App) setActionCookie(w http.ResponseWriter, name, value string, expires time.Time, sameSite http.SameSite) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", HttpOnly: true, Secure: a.cookieSecure, SameSite: sameSite, Expires: expires, MaxAge: int(expires.Sub(a.clock().UTC()).Seconds())})
}

func (a *App) clearActionCookie(w http.ResponseWriter, name string, sameSite http.SameSite) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", HttpOnly: true, Secure: a.cookieSecure, SameSite: sameSite, MaxAge: -1, Expires: time.Unix(1, 0)})
}

func singleCookie(r *http.Request, name string) (string, bool) {
	count, value := 0, ""
	for _, cookie := range r.Cookies() {
		if cookie.Name == name {
			count++
			value = strings.TrimSpace(cookie.Value)
		}
	}
	return value, count == 1 && value != "" && len(value) <= 512
}
