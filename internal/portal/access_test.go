package portal

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"alzette/internal/federation"
	"alzette/internal/humanauth"
	"alzette/internal/platform"
	"alzette/internal/workforce"
)

type workforceStub struct {
	access       workforce.Access
	groups       map[string]workforce.Group
	created      workforce.CreateGroupInput
	createdCount int
	people       []string
	models       []string
	disabled     []string
	invited      workforce.CreateInvitationInput
	revoked      []string
	setupCount   int
	oidcCreated  bool
	accepted     workforce.FederatedIdentity
}

func (s *workforceStub) LoadAccess(context.Context, platform.PortalSession) (workforce.Access, error) {
	return s.access, nil
}
func (s *workforceStub) LoadGroup(_ context.Context, _ platform.PortalSession, id string) (workforce.Group, error) {
	group, ok := s.groups[id]
	if !ok {
		return workforce.Group{}, platform.ErrNotFound
	}
	return group, nil
}
func (s *workforceStub) CreateGroup(_ context.Context, _ platform.PortalSession, input workforce.CreateGroupInput) (workforce.Group, error) {
	if !s.access.CanManage {
		return workforce.Group{}, platform.ErrForbidden
	}
	s.created = input
	s.createdCount++
	return workforce.Group{ID: "group_new", Name: input.Name}, nil
}
func (s *workforceStub) ReplaceGroupPeople(_ context.Context, _ platform.PortalSession, id string, people []string) error {
	if !s.access.CanManage {
		return platform.ErrForbidden
	}
	if _, ok := s.groups[id]; !ok {
		return platform.ErrNotFound
	}
	s.people = people
	return nil
}
func (s *workforceStub) ReplaceGroupModels(_ context.Context, _ platform.PortalSession, id string, models []string) error {
	if !s.access.CanManage {
		return platform.ErrForbidden
	}
	if _, ok := s.groups[id]; !ok {
		return platform.ErrNotFound
	}
	s.models = models
	return nil
}
func (s *workforceStub) DisableGroup(_ context.Context, _ platform.PortalSession, id string) error {
	if !s.access.CanManage {
		return platform.ErrForbidden
	}
	if _, ok := s.groups[id]; !ok {
		return platform.ErrNotFound
	}
	s.disabled = append(s.disabled, id)
	return nil
}
func (s *workforceStub) CreateInvitation(_ context.Context, _ platform.PortalSession, input workforce.CreateInvitationInput) (workforce.InvitationDelivery, error) {
	if !s.access.CanManage {
		return workforce.InvitationDelivery{}, platform.ErrForbidden
	}
	s.invited = input
	invitation := workforce.Invitation{ID: "inv_test", Email: input.Email, DisplayName: input.DisplayName, Status: "pending", Groups: []workforce.GroupReference{{ID: "group_research", Name: "Research"}}, CreatedAt: time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC), ExpiresAt: time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC), Delivery: "manual"}
	s.access.Invitations = append([]workforce.Invitation{invitation}, s.access.Invitations...)
	return workforce.InvitationDelivery{Invitation: invitation, Token: "manual_invitation_token"}, nil
}
func (s *workforceStub) ResendInvitation(_ context.Context, _ platform.PortalSession, id string) (workforce.InvitationDelivery, error) {
	if !s.access.CanManage {
		return workforce.InvitationDelivery{}, platform.ErrForbidden
	}
	if id != "inv_test" {
		return workforce.InvitationDelivery{}, platform.ErrNotFound
	}
	return workforce.InvitationDelivery{Invitation: workforce.Invitation{ID: id, Email: "employee@example.test", Status: "pending", Groups: []workforce.GroupReference{{ID: "group_research", Name: "Research"}}, ExpiresAt: time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)}, Token: "rotated_invitation_token"}, nil
}
func (s *workforceStub) RevokeInvitation(_ context.Context, _ platform.PortalSession, id string) error {
	if !s.access.CanManage {
		return platform.ErrForbidden
	}
	if id != "inv_test" {
		return platform.ErrNotFound
	}
	s.revoked = append(s.revoked, id)
	return nil
}
func (s *workforceStub) BeginInvitationSetup(context.Context, [32]byte, time.Time) (workforce.SetupSession, error) {
	s.setupCount++
	return workforce.SetupSession{Token: "setup_token_abcdefghijklmnopqrstuvwxyz012345", ExpiresAt: time.Date(2026, 8, 17, 12, 15, 0, 0, time.UTC)}, nil
}
func (s *workforceStub) CreateOIDCTransaction(_ context.Context, _, _ [32]byte, nonce, verifier string, _, _ time.Time) error {
	s.oidcCreated = len(nonce) >= 32 && len(verifier) >= 43
	return nil
}
func (s *workforceStub) ConsumeOIDCTransaction(context.Context, [32]byte, time.Time) (workforce.OIDCTransaction, error) {
	return workforce.OIDCTransaction{ActionSessionID: "act_invitation", Nonce: "expected-nonce", Verifier: "expected-verifier"}, nil
}
func (s *workforceStub) AcceptInvitation(_ context.Context, _ string, identity workforce.FederatedIdentity, _ [32]byte, expiresAt, now time.Time) (platform.PortalSession, error) {
	s.accepted = identity
	return platform.PortalSession{ExpiresAt: expiresAt, AuthenticatedAt: now}, nil
}
func (s *workforceStub) AssignInitialOwner(context.Context, workforce.InitialOwnerSpec) (workforce.InitialOwnerResult, error) {
	return workforce.InitialOwnerResult{}, errors.New("not used")
}

func newAccessTestApp(t *testing.T, workforceStore *workforceStub) *App {
	return newAccessTestAppWithOIDC(t, workforceStore, nil)
}

func newAccessTestAppWithOIDC(t *testing.T, workforceStore *workforceStub, provider federation.Provider) *App {
	t.Helper()
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	portalStore := newPortalStub(now)
	directory := t.TempDir()
	writePortalAssets(t, directory)
	app, err := New(Config{
		Store: portalStore, PortalStore: portalStore, StaticDirectory: directory, SessionTTL: time.Hour,
		Clock: func() time.Time { return now }, Workforce: workforce.New(workforceStore), OIDC: provider,
		PublicGatewayURL: "http://127.0.0.1:8080", AllowInsecurePublicGateway: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return app
}

type oidcStub struct {
	identity  federation.Identity
	exchanged bool
}

func (s *oidcStub) AuthorizationURL(state, nonce, verifier string) string {
	return "https://identity.example.test/authorize?state=" + url.QueryEscape(state) + "&nonce=" + url.QueryEscape(nonce) + "&challenge=" + url.QueryEscape(verifier)
}
func (s *oidcStub) Exchange(_ context.Context, code, verifier, nonce string) (federation.Identity, error) {
	if code != "test-code" || verifier != "expected-verifier" || nonce != "expected-nonce" {
		return federation.Identity{}, errors.New("unexpected exchange")
	}
	s.exchanged = true
	return s.identity, nil
}
func (s *oidcStub) Issuer() string { return "https://identity.example.test" }

func ownerAccessFixture() workforce.Access {
	models := []workforce.ModelAccess{{RouteID: "route_chat", Alias: "alzette-chat", Project: "Inference Pilot", Environment: "PoC"}}
	people := []workforce.Person{
		{ID: "person_owner", DisplayName: `Owner <script>alert(1)</script>`, Email: "owner@example.test", Relationship: workforce.RelationshipOwner, Enabled: true, EffectiveModels: models},
		{ID: "person_employee", DisplayName: "Erin Employee", Email: "erin@example.test", Relationship: workforce.RelationshipEmployee, Enabled: true},
	}
	groups := []workforce.Group{{ID: "group_research", Name: "Research", Description: "Research team", Enabled: true, Project: "Inference Pilot", Environment: "PoC", Models: models}}
	return workforce.Access{Configured: true, Relationship: workforce.RelationshipOwner, CanManage: true, CurrentPersonID: "person_owner", People: people, Groups: groups, AvailableModels: models}
}

func TestAccessWorkspaceIsServerRenderedEscapedAndKeepsApplicationAccess(t *testing.T) {
	store := &workforceStub{access: ownerAccessFixture(), groups: map[string]workforce.Group{}}
	app := newAccessTestApp(t, store)

	response := httptest.NewRecorder()
	app.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/app/access", ""))
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(response.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("status=%d type=%q", response.Code, response.Header().Get("Content-Type"))
	}
	for _, required := range []string{"People and model access", "Company owner", "Erin Employee", "alzette-chat", "/app/access/groups", "/app/access?view=applications", `<details class="server-mobile-nav">`} {
		if !strings.Contains(body, required) {
			t.Fatalf("access page missing %q", required)
		}
	}
	for _, forbidden := range []string{"<script", "/portal.js", "Owner <script>alert(1)</script>", "org_admin", "project_admin", "role picker"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("access page contains %q", forbidden)
		}
	}
	if !strings.Contains(body, "Owner &lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Fatal("person display name was not HTML escaped")
	}
	if csp := response.Header().Get("Content-Security-Policy"); csp == "" || strings.Contains(csp, "unsafe-") {
		t.Fatalf("unsafe CSP %q", csp)
	}
	legacy := httptest.NewRecorder()
	app.ServeHTTP(legacy, authenticatedRequest(http.MethodGet, "/app/access?view=applications", ""))
	if legacy.Code != http.StatusOK || !strings.Contains(legacy.Body.String(), `<script src="/portal.js" defer></script>`) || !strings.Contains(legacy.Body.String(), `class="access-section access-applications"`) {
		t.Fatal("application access did not preserve the existing key-management surface")
	}
}

func TestAccessWorkspaceTruthfullyHandlesUnreconciledAndEmployeeStates(t *testing.T) {
	unreconciled := &workforceStub{access: workforce.Access{}, groups: map[string]workforce.Group{}}
	app := newAccessTestApp(t, unreconciled)
	response := httptest.NewRecorder()
	app.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/app/access", ""))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "has not been explicitly established") || !strings.Contains(response.Body.String(), "ownership assign") {
		t.Fatalf("unreconciled page status=%d", response.Code)
	}

	employee := ownerAccessFixture()
	employee.Relationship = workforce.RelationshipEmployee
	employee.CanManage = false
	employee.CurrentPersonID = "person_employee"
	employee.People = employee.People[1:]
	employee.Groups = nil
	app = newAccessTestApp(t, &workforceStub{access: employee, groups: map[string]workforce.Group{}})
	response = httptest.NewRecorder()
	app.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/app/access/groups", ""))
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "Create group") || !strings.Contains(response.Body.String(), "owner has not assigned") {
		t.Fatalf("employee groups page status=%d body=%q", response.Code, response.Body.String())
	}
}

func TestApplicationAccessUsesExplicitOwnerNotLegacyRole(t *testing.T) {
	employee := ownerAccessFixture()
	employee.Relationship = workforce.RelationshipEmployee
	employee.CanManage = false
	employee.CurrentPersonID = "person_employee"
	employee.People = employee.People[1:]
	app := newAccessTestApp(t, &workforceStub{access: employee, groups: map[string]workforce.Group{}})

	accessResponse := httptest.NewRecorder()
	app.ServeHTTP(accessResponse, authenticatedRequest(http.MethodGet, "/api/portal/access", ""))
	if accessResponse.Code != http.StatusOK || !strings.Contains(accessResponse.Body.String(), `"can_manage":false`) {
		t.Fatalf("employee access status=%d body=%s", accessResponse.Code, accessResponse.Body.String())
	}
	mutation := httptest.NewRecorder()
	app.ServeHTTP(mutation, authenticatedRequest(http.MethodPost, "/api/portal/service-accounts", `{"name":"employee must not create"}`))
	if mutation.Code != http.StatusForbidden || !strings.Contains(mutation.Body.String(), `"code":"owner_required"`) {
		t.Fatalf("legacy project-admin employee mutation status=%d body=%s", mutation.Code, mutation.Body.String())
	}

	me := httptest.NewRecorder()
	app.ServeHTTP(me, authenticatedRequest(http.MethodGet, "/api/portal/me", ""))
	if me.Code != http.StatusOK || strings.Contains(me.Body.String(), `"access:manage"`) {
		t.Fatalf("employee me contract claimed application management status=%d body=%s", me.Code, me.Body.String())
	}
}

func TestAccessGroupFormsRequireCSRFAndUsePostRedirectGet(t *testing.T) {
	access := ownerAccessFixture()
	group := access.Groups[0]
	store := &workforceStub{access: access, groups: map[string]workforce.Group{group.ID: group}}
	app := newAccessTestApp(t, store)

	bad := accessFormRequest("/app/access/groups", url.Values{"_csrf": {"wrong"}, "name": {"Research"}})
	badResponse := httptest.NewRecorder()
	app.ServeHTTP(badResponse, bad)
	if badResponse.Code != http.StatusForbidden || store.createdCount != 0 {
		t.Fatalf("bad CSRF status=%d creates=%d", badResponse.Code, store.createdCount)
	}

	valid := accessFormRequest("/app/access/groups", url.Values{"_csrf": {testCSRFToken}, "name": {" Client operations "}, "description": {" Employees "}, "route_id": {"route_chat"}})
	validResponse := httptest.NewRecorder()
	app.ServeHTTP(validResponse, valid)
	if validResponse.Code != http.StatusSeeOther || validResponse.Header().Get("Location") != "/app/access/groups/group_new?saved=group" || store.createdCount != 1 {
		t.Fatalf("create status=%d location=%q count=%d", validResponse.Code, validResponse.Header().Get("Location"), store.createdCount)
	}
	if store.created.Name != "Client operations" || store.created.Description != "Employees" || len(store.created.RouteIDs) != 1 || store.created.RouteIDs[0] != "route_chat" {
		t.Fatalf("created=%#v", store.created)
	}

	invalid := accessFormRequest("/app/access/groups", url.Values{"_csrf": {testCSRFToken}, "name": {"<invalid>"}, "description": {"Keep this explanation"}, "route_id": {"route_chat"}})
	invalidResponse := httptest.NewRecorder()
	app.ServeHTTP(invalidResponse, invalid)
	invalidBody := invalidResponse.Body.String()
	if invalidResponse.Code != http.StatusUnprocessableEntity || !strings.Contains(invalidBody, `value="&lt;invalid&gt;"`) || !strings.Contains(invalidBody, `Keep this explanation`) || !strings.Contains(invalidBody, `value="route_chat" checked`) || !strings.Contains(invalidBody, `aria-invalid="true"`) {
		t.Fatalf("invalid form did not preserve safe input status=%d", invalidResponse.Code)
	}

	store.access.People = store.access.People[:1]
	detail := httptest.NewRecorder()
	app.ServeHTTP(detail, authenticatedRequest(http.MethodGet, "/app/access/groups/"+group.ID, ""))
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), "No employees are available. Invitations are not enabled yet.") {
		t.Fatalf("group detail status=%d", detail.Code)
	}

	confirmation := httptest.NewRecorder()
	app.ServeHTTP(confirmation, authenticatedRequest(http.MethodGet, "/app/access/groups/"+group.ID+"/disable", ""))
	if confirmation.Code != http.StatusOK || !strings.Contains(confirmation.Body.String(), "Confirm access removal") || !strings.Contains(confirmation.Body.String(), "Disable group now") || len(store.disabled) != 0 {
		t.Fatalf("disable confirmation status=%d disabled=%v", confirmation.Code, store.disabled)
	}
	disable := accessFormRequest("/app/access/groups/"+group.ID+"/disable", url.Values{"_csrf": {testCSRFToken}})
	disableResponse := httptest.NewRecorder()
	app.ServeHTTP(disableResponse, disable)
	if disableResponse.Code != http.StatusSeeOther || disableResponse.Header().Get("Location") != "/app/access/groups" || len(store.disabled) != 1 || store.disabled[0] != group.ID {
		t.Fatalf("disable status=%d location=%q disabled=%v", disableResponse.Code, disableResponse.Header().Get("Location"), store.disabled)
	}
}

func TestOwnerCreatesAndRevokesExactEmployeeInvitation(t *testing.T) {
	access := ownerAccessFixture()
	store := &workforceStub{access: access, groups: map[string]workforce.Group{"group_research": access.Groups[0]}}
	app := newAccessTestApp(t, store)

	form := accessFormRequest("/app/access/invitations", url.Values{"_csrf": {testCSRFToken}, "email": {" Employee@Example.TEST "}, "display_name": {" Erin Invited "}, "group_id": {"group_research"}})
	response := httptest.NewRecorder()
	app.ServeHTTP(response, form)
	body := response.Body.String()
	if response.Code != http.StatusCreated || store.invited.Email != "employee@example.test" || store.invited.DisplayName != "Erin Invited" || len(store.invited.GroupIDs) != 1 {
		t.Fatalf("create status=%d invited=%#v", response.Code, store.invited)
	}
	for _, expected := range []string{"Invitation created", "/accept-invite?token=manual_invitation_token", "Research", "shown once"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("invitation result missing %q", expected)
		}
	}
	if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("unsafe invitation headers: %#v", response.Header())
	}

	revoke := accessFormRequest("/app/access/invitations/inv_test/revoke", url.Values{"_csrf": {testCSRFToken}})
	revoked := httptest.NewRecorder()
	app.ServeHTTP(revoked, revoke)
	if revoked.Code != http.StatusSeeOther || revoked.Header().Get("Location") != "/app/access/people?saved=invitation-revoked" || len(store.revoked) != 1 {
		t.Fatalf("revoke status=%d location=%q revoked=%v", revoked.Code, revoked.Header().Get("Location"), store.revoked)
	}
}

func TestInvitationGETIsScannerSafeAndOIDCCallbackAcceptsExactIdentity(t *testing.T) {
	access := ownerAccessFixture()
	store := &workforceStub{access: access, groups: map[string]workforce.Group{}}
	provider := &oidcStub{identity: federation.Identity{Issuer: "https://identity.example.test", Subject: "employee-subject", Email: "employee@example.test", EmailVerified: true, DisplayName: "Employee Person"}}
	app := newAccessTestAppWithOIDC(t, store, provider)

	entry := httptest.NewRecorder()
	app.ServeHTTP(entry, httptest.NewRequest(http.MethodGet, "/accept-invite?token=abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG", nil))
	if entry.Code != http.StatusSeeOther || entry.Header().Get("Location") != "/accept-invite" || store.setupCount != 1 || store.accepted.Email != "" {
		t.Fatalf("scanner GET status=%d location=%q setups=%d accepted=%#v", entry.Code, entry.Header().Get("Location"), store.setupCount, store.accepted)
	}
	setupCookie := entry.Result().Cookies()[0]
	clean := httptest.NewRecorder()
	app.ServeHTTP(clean, httptest.NewRequest(http.MethodGet, "/accept-invite", nil))
	if csp := clean.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "form-action 'self' https://identity.example.test") || strings.Contains(csp, "unsafe-") {
		t.Fatalf("invitation CSP cannot safely follow the exact identity-provider redirect: %q", csp)
	}
	for _, forbidden := range []string{"Owner &lt;script", "Research", "employee@example.test", "alzette-chat"} {
		if strings.Contains(clean.Body.String(), forbidden) {
			t.Fatalf("clean invitation page disclosed %q", forbidden)
		}
	}

	continueRequest := httptest.NewRequest(http.MethodPost, "/accept-invite", strings.NewReader("intent=continue"))
	continueRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	continueRequest.AddCookie(setupCookie)
	continuation := httptest.NewRecorder()
	app.ServeHTTP(continuation, continueRequest)
	if continuation.Code != http.StatusSeeOther || !strings.HasPrefix(continuation.Header().Get("Location"), "https://identity.example.test/authorize?") || !store.oidcCreated {
		t.Fatalf("OIDC start status=%d location=%q created=%t", continuation.Code, continuation.Header().Get("Location"), store.oidcCreated)
	}
	var stateCookie *http.Cookie
	for _, cookie := range continuation.Result().Cookies() {
		if cookie.Name == oidcStateCookieName {
			stateCookie = cookie
		}
	}
	if stateCookie == nil {
		t.Fatal("OIDC state cookie missing")
	}
	callback := httptest.NewRequest(http.MethodGet, "/login/oidc/callback?state="+url.QueryEscape(stateCookie.Value)+"&code=test-code", nil)
	callback.AddCookie(stateCookie)
	accepted := httptest.NewRecorder()
	app.ServeHTTP(accepted, callback)
	if accepted.Code != http.StatusSeeOther || accepted.Header().Get("Location") != "/app/access/people" || !provider.exchanged || store.accepted.Email != "employee@example.test" || store.accepted.Subject != "employee-subject" {
		t.Fatalf("callback status=%d location=%q exchanged=%t identity=%#v body=%s", accepted.Code, accepted.Header().Get("Location"), provider.exchanged, store.accepted, accepted.Body.String())
	}
	cookies := accepted.Result().Cookies()
	foundSession, foundCSRF := false, false
	for _, cookie := range cookies {
		foundSession = foundSession || cookie.Name == humanauth.SessionCookieName && cookie.Value != ""
		foundCSRF = foundCSRF || cookie.Name == humanauth.CSRFCookieName && cookie.Value != ""
	}
	if !foundSession || !foundCSRF {
		t.Fatalf("accepted invitation cookies=%#v", cookies)
	}
}

func accessFormRequest(target string, values url.Values) *http.Request {
	request := httptest.NewRequest(http.MethodPost, target, strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(&http.Cookie{Name: "alzette_session", Value: testSessionToken})
	request.AddCookie(&http.Cookie{Name: "alzette_csrf", Value: testCSRFToken})
	return request
}
