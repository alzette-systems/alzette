package portal

import (
	"bytes"
	"embed"
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"alzette/internal/api"
	"alzette/internal/platform"
	"alzette/internal/workforce"
)

//go:embed templates/access.html
var accessTemplateFS embed.FS

type accessRenderer struct {
	template *template.Template
}

type accessPageView struct {
	CSRFToken          string
	Scope              overviewScopeView
	UserLabel          string
	Active             string
	Access             workforce.Access
	Employees          []workforce.Person
	Group              *workforce.Group
	DraftGroup         workforce.CreateGroupInput
	DraftInvitation    workforce.CreateInvitationInput
	InvitationDelivery *workforce.InvitationDelivery
	InvitationURL      string
	Error              string
	Notice             string
}

func newAccessRenderer() (*accessRenderer, error) {
	functions := template.FuncMap{
		"hasPerson": func(group workforce.Group, id string) bool {
			for _, person := range group.People {
				if person.ID == id {
					return true
				}
			}
			return false
		},
		"hasModel": func(group workforce.Group, id string) bool {
			for _, model := range group.Models {
				if model.RouteID == id {
					return true
				}
			}
			return false
		},
		"hasID": func(values []string, id string) bool {
			for _, value := range values {
				if value == id {
					return true
				}
			}
			return false
		},
	}
	parsed, err := template.New("access.html").Funcs(functions).Option("missingkey=error").ParseFS(accessTemplateFS, "templates/access.html")
	if err != nil {
		return nil, err
	}
	return &accessRenderer{template: parsed}, nil
}

func (r *accessRenderer) render(view accessPageView) ([]byte, error) {
	var output bytes.Buffer
	if err := r.template.ExecuteTemplate(&output, "access.html", view); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func (a *App) serveAccessWorkspace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodPost {
		api.MethodNotAllowed(w, "GET, HEAD, POST", "")
		return
	}
	session, _, err := a.session(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if r.Method == http.MethodPost {
		a.mutateAccessWorkspace(w, r, session)
		return
	}
	view, status := a.accessView(r, session)
	a.renderAccessPage(w, r, view, status)
}

func (a *App) accessView(r *http.Request, session platform.PortalSession) (accessPageView, int) {
	view := accessPageView{
		CSRFToken: csrfCookieValue(r),
		Scope: overviewScopeView{
			Organisation:       fallbackText(session.Current.OrganisationName, "Organisation unavailable"),
			Project:            fallbackText(session.Current.ProjectName, "Project unavailable"),
			Environment:        fallbackText(session.Current.EnvironmentName, "Environment unavailable"),
			ProjectEnvironment: joinScope(session.Current.ProjectName, session.Current.EnvironmentName),
		},
		Active: "people",
	}
	if a.workforce == nil {
		view.UserLabel = "Legacy portal member"
		return view, http.StatusOK
	}
	access, err := a.workforce.Access(r.Context(), session)
	if err != nil {
		view.Error = "Company access could not be loaded. No people or group authority is being claimed."
		view.UserLabel = "Company access unavailable"
		return view, http.StatusServiceUnavailable
	}
	view.Access = access
	for _, person := range access.People {
		if person.Relationship != workforce.RelationshipOwner {
			view.Employees = append(view.Employees, person)
		}
	}
	switch access.Relationship {
	case workforce.RelationshipOwner:
		view.UserLabel = "Company owner"
	case workforce.RelationshipEmployee:
		view.UserLabel = "Employee"
	default:
		view.UserLabel = "Legacy portal member"
	}
	path := strings.TrimSuffix(r.URL.Path, "/")
	switch {
	case path == "/app/access" && r.URL.Query().Get("view") == "applications":
		view.Active = "applications"
	case path == "/app/access/invitations/new":
		view.Active = "new-invitation"
	case path == "/app/access/groups/new":
		view.Active = "new-group"
	case path == "/app/access/groups":
		view.Active = "groups"
	case strings.HasSuffix(path, "/disable"):
		view.Active = "disable-group"
		id, ok := actionPathValue(path, "/app/access/groups/", "/disable")
		if !ok {
			view.Error = "The requested access group was not found."
			return view, http.StatusNotFound
		}
		group, err := a.workforce.Group(r.Context(), session, id)
		if err != nil {
			view.Error = "The requested access group was not found."
			return view, http.StatusNotFound
		}
		view.Group = &group
	case strings.HasPrefix(path, "/app/access/groups/"):
		view.Active = "group"
		id := strings.TrimPrefix(path, "/app/access/groups/")
		if strings.Contains(id, "/") || id == "" {
			view.Error = "The requested access group was not found."
			return view, http.StatusNotFound
		}
		group, err := a.workforce.Group(r.Context(), session, id)
		if err != nil {
			if errors.Is(err, platform.ErrNotFound) || errors.Is(err, platform.ErrForbidden) || errors.Is(err, platform.ErrInvalid) {
				view.Error = "The requested access group was not found."
				return view, http.StatusNotFound
			}
			view.Error = "The access group could not be loaded."
			return view, http.StatusServiceUnavailable
		}
		view.Group = &group
	default:
		view.Active = "people"
	}
	switch r.URL.Query().Get("saved") {
	case "group":
		view.Notice = "Group saved. Effective employee model access now reflects the current assignments."
	case "people":
		view.Notice = "Employee assignments saved. Removed access is invalidated for the next authorization check."
	case "models":
		view.Notice = "Model endpoint assignments saved. Newly added access requires fresh authorization."
	case "invitation-revoked":
		view.Notice = "Invitation revoked. Its previous acceptance link can no longer establish employee access."
	}
	return view, http.StatusOK
}

func (a *App) mutateAccessWorkspace(w http.ResponseWriter, r *http.Request, session platform.PortalSession) {
	if a.workforce == nil {
		http.NotFound(w, r)
		return
	}
	if !validCSRFForm(w, r) {
		a.renderAccessMutationError(w, r, session, http.StatusForbidden, "The form could not be verified. Reload the page and try again.")
		return
	}
	path := strings.TrimSuffix(r.URL.Path, "/")
	switch {
	case path == "/app/access/invitations":
		draft := workforce.CreateInvitationInput{Email: r.PostForm.Get("email"), DisplayName: r.PostForm.Get("display_name"), GroupIDs: r.PostForm["group_id"]}
		delivery, err := a.workforce.CreateInvitation(r.Context(), session, draft)
		if err != nil {
			status, message := invitationMutationError(err, "The invitation could not be created. Check the exact email and select at least one enabled group.")
			view, _ := a.accessView(r, session)
			view.Active = "new-invitation"
			view.Error = message
			view.DraftInvitation = draft
			a.renderAccessPage(w, r, view, status)
			return
		}
		a.renderInvitationDelivery(w, r, session, delivery, http.StatusCreated)
	case strings.HasSuffix(path, "/resend"):
		id, ok := actionPathValue(path, "/app/access/invitations/", "/resend")
		if !ok {
			http.NotFound(w, r)
			return
		}
		delivery, err := a.workforce.ResendInvitation(r.Context(), session, id)
		if err != nil {
			a.renderAccessMutationResult(w, r, session, err, "The invitation could not be resent.", "people")
			return
		}
		a.renderInvitationDelivery(w, r, session, delivery, http.StatusOK)
	case strings.HasSuffix(path, "/revoke"):
		id, ok := actionPathValue(path, "/app/access/invitations/", "/revoke")
		if !ok {
			http.NotFound(w, r)
			return
		}
		if err := a.workforce.RevokeInvitation(r.Context(), session, id); err != nil {
			a.renderAccessMutationResult(w, r, session, err, "The invitation could not be revoked.", "people")
			return
		}
		http.Redirect(w, r, "/app/access/people?saved=invitation-revoked", http.StatusSeeOther)
	case path == "/app/access/groups":
		draft := workforce.CreateGroupInput{
			Name: r.PostForm.Get("name"), Description: r.PostForm.Get("description"), RouteIDs: r.PostForm["route_id"],
		}
		group, err := a.workforce.CreateGroup(r.Context(), session, draft)
		if err != nil {
			status, message := accessMutationError(err, "The group could not be created. Check the name and model endpoint choices.")
			view, _ := a.accessView(r, session)
			view.Active = "new-group"
			view.Error = message
			view.DraftGroup = draft
			a.renderAccessPage(w, r, view, status)
			return
		}
		http.Redirect(w, r, "/app/access/groups/"+url.PathEscape(group.ID)+"?saved=group", http.StatusSeeOther)
	case strings.HasSuffix(path, "/people"):
		id, ok := actionPathValue(path, "/app/access/groups/", "/people")
		if !ok {
			http.NotFound(w, r)
			return
		}
		err := a.workforce.ReplaceGroupPeople(r.Context(), session, id, r.PostForm["person_id"])
		if err != nil {
			a.renderAccessMutationResult(w, r, session, err, "Employee assignments could not be saved.", "group")
			return
		}
		http.Redirect(w, r, "/app/access/groups/"+url.PathEscape(id)+"?saved=people", http.StatusSeeOther)
	case strings.HasSuffix(path, "/models"):
		id, ok := actionPathValue(path, "/app/access/groups/", "/models")
		if !ok {
			http.NotFound(w, r)
			return
		}
		err := a.workforce.ReplaceGroupModels(r.Context(), session, id, r.PostForm["route_id"])
		if err != nil {
			a.renderAccessMutationResult(w, r, session, err, "Model endpoint assignments could not be saved.", "group")
			return
		}
		http.Redirect(w, r, "/app/access/groups/"+url.PathEscape(id)+"?saved=models", http.StatusSeeOther)
	case strings.HasSuffix(path, "/disable"):
		id, ok := actionPathValue(path, "/app/access/groups/", "/disable")
		if !ok {
			http.NotFound(w, r)
			return
		}
		if err := a.workforce.DisableGroup(r.Context(), session, id); err != nil {
			a.renderAccessMutationResult(w, r, session, err, "The group could not be disabled.", "group")
			return
		}
		http.Redirect(w, r, "/app/access/groups", http.StatusSeeOther)
	default:
		http.NotFound(w, r)
	}
}

func invitationMutationError(err error, fallback string) (int, string) {
	status, message := accessMutationError(err, fallback)
	if errors.Is(err, platform.ErrConflict) {
		message = "That person is already an employee or already has an active invitation. Revoke or resend the pending invitation instead."
	}
	return status, message
}

func (a *App) renderInvitationDelivery(w http.ResponseWriter, r *http.Request, session platform.PortalSession, delivery workforce.InvitationDelivery, status int) {
	view, _ := a.accessView(r, session)
	view.Active = "invitation-created"
	view.InvitationDelivery = &delivery
	view.InvitationURL = "/accept-invite?token=" + url.QueryEscape(delivery.Token)
	a.renderAccessPage(w, r, view, status)
}

func (a *App) renderAccessMutationResult(w http.ResponseWriter, r *http.Request, session platform.PortalSession, err error, fallback, active string) {
	status, message := accessMutationError(err, fallback)
	a.renderAccessMutationErrorWithActive(w, r, session, status, message, active)
}

func accessMutationError(err error, fallback string) (int, string) {
	status := http.StatusServiceUnavailable
	message := fallback
	switch {
	case errors.Is(err, platform.ErrInvalid):
		status = http.StatusUnprocessableEntity
	case errors.Is(err, platform.ErrConflict):
		status = http.StatusConflict
		message = "The group changed before this form was saved. Reload it and review the current assignments."
	case errors.Is(err, platform.ErrForbidden):
		status = http.StatusForbidden
		message = "Only the current company owner can change access groups."
	case errors.Is(err, platform.ErrNotFound):
		status = http.StatusNotFound
		message = "The requested access group was not found."
	}
	return status, message
}

func (a *App) renderAccessMutationError(w http.ResponseWriter, r *http.Request, session platform.PortalSession, status int, message string) {
	a.renderAccessMutationErrorWithActive(w, r, session, status, message, "groups")
}

func (a *App) renderAccessMutationErrorWithActive(w http.ResponseWriter, r *http.Request, session platform.PortalSession, status int, message, active string) {
	view, _ := a.accessView(r, session)
	view.Active = active
	view.Error = message
	a.renderAccessPage(w, r, view, status)
}

func (a *App) renderAccessPage(w http.ResponseWriter, r *http.Request, view accessPageView, status int) {
	contents, err := a.accessRenderer.render(view)
	if err != nil {
		http.Error(w, "Portal page could not be rendered", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if view.InvitationURL != "" {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(contents)))
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(contents)
	}
}
