package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"alzette/internal/credentials"
	"alzette/internal/ids"
	"alzette/internal/platform"
	"alzette/internal/provisioning"
)

type organisation struct{ id, slug, name string }
type project struct{ id, organisationID, slug, name string }
type environment struct{ id, organisationID, projectID, slug, name string }
type model struct {
	id, alias, version string
	enabled            bool
}
type serviceAccount struct{ id, organisationID, projectID, environmentID, name string }
type apiKey struct {
	id, serviceAccountID, prefix string
	digest                       [32]byte
	scopes                       []string
	revokedAt                    *time.Time
}
type route struct {
	id, organisationID, projectID, environmentID, modelID, targetID string
	bindingGeneration                                               int64
	enabled                                                         bool
	createdAt, updatedAt                                            time.Time
}

type Store struct {
	mu              sync.RWMutex
	now             func() time.Time
	organisations   map[string]organisation
	projects        map[string]project
	environments    map[string]environment
	models          map[string]model
	targets         map[string]platform.Target
	serviceAccounts map[string]serviceAccount
	keys            map[[32]byte]apiKey
	routes          map[string]route
	requests        map[string]platform.InferenceRequest
	attempts        map[string]platform.ProviderAttempt
}

func New() *Store {
	return &Store{
		now:           time.Now,
		organisations: make(map[string]organisation), projects: make(map[string]project),
		environments: make(map[string]environment), models: make(map[string]model),
		targets: make(map[string]platform.Target), serviceAccounts: make(map[string]serviceAccount),
		keys: make(map[[32]byte]apiKey), routes: make(map[string]route),
		requests: make(map[string]platform.InferenceRequest), attempts: make(map[string]platform.ProviderAttempt),
	}
}

func (s *Store) SetClock(clock func() time.Time) { s.mu.Lock(); defer s.mu.Unlock(); s.now = clock }

func newID(prefix string) (string, error) { return ids.New(prefix) }

func (s *Store) Provision(_ context.Context, input platform.ProvisionSpec) (platform.ProvisionResult, error) {
	spec, err := provisioning.Validate(input, true)
	if err != nil {
		return platform.ProvisionResult{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()

	org, ok := findOrganisation(s.organisations, spec.OrganisationSlug)
	if !ok {
		org.id, err = newID("org")
		if err != nil {
			return platform.ProvisionResult{}, err
		}
		org.slug, org.name = spec.OrganisationSlug, spec.OrganisationName
		s.organisations[org.id] = org
	} else {
		org.name = spec.OrganisationName
		s.organisations[org.id] = org
	}
	proj, ok := findProject(s.projects, org.id, spec.ProjectSlug)
	if !ok {
		proj.id, err = newID("prj")
		if err != nil {
			return platform.ProvisionResult{}, err
		}
		proj.organisationID, proj.slug, proj.name = org.id, spec.ProjectSlug, spec.ProjectName
		s.projects[proj.id] = proj
	} else {
		proj.name = spec.ProjectName
		s.projects[proj.id] = proj
	}
	env, ok := findEnvironment(s.environments, org.id, proj.id, spec.EnvironmentSlug)
	if !ok {
		env.id, err = newID("env")
		if err != nil {
			return platform.ProvisionResult{}, err
		}
		env.organisationID, env.projectID, env.slug, env.name = org.id, proj.id, spec.EnvironmentSlug, spec.EnvironmentName
		s.environments[env.id] = env
	} else {
		env.name = spec.EnvironmentName
		s.environments[env.id] = env
	}
	mdl, ok := findModel(s.models, spec.ModelAlias)
	if !ok {
		mdl.id, err = newID("mdl")
		if err != nil {
			return platform.ProvisionResult{}, err
		}
		mdl.alias, mdl.version, mdl.enabled = spec.ModelAlias, spec.ModelVersion, true
		s.models[mdl.id] = mdl
	} else {
		if mdl.version != spec.ModelVersion {
			return platform.ProvisionResult{}, platform.ErrConflict
		}
		mdl.enabled = true
		s.models[mdl.id] = mdl
	}

	target, ok := findTarget(s.targets, spec.TargetName)
	if !ok {
		target.ID, err = newID("tgt")
		if err != nil {
			return platform.ProvisionResult{}, err
		}
	} else {
		wantedOwner := ""
		if spec.CapacityMode == "dedicated" {
			wantedOwner = org.id
		}
		if target.ExecutionClass != spec.ExecutionClass || target.CapacityMode != spec.CapacityMode || target.CapacityEvidenceRef != spec.CapacityEvidenceRef || target.OwnerOrganisationID != wantedOwner || target.BaseURL != spec.TargetBaseURL || target.ProviderModel != spec.ProviderModel || target.SecretRef != spec.SecretRef || target.Timeout != spec.TargetTimeout || target.MaxAttempts != spec.MaxAttempts {
			return platform.ProvisionResult{}, platform.ErrConflict
		}
	}
	target.Name, target.ExecutionClass, target.CapacityMode = spec.TargetName, spec.ExecutionClass, spec.CapacityMode
	target.CapacityEvidenceRef = spec.CapacityEvidenceRef
	if spec.CapacityMode == "dedicated" {
		target.OwnerOrganisationID = org.id
	} else {
		target.OwnerOrganisationID = ""
	}
	target.BaseURL, target.ProviderModel, target.SecretRef = spec.TargetBaseURL, spec.ProviderModel, spec.SecretRef
	target.Timeout, target.MaxAttempts, target.Enabled = spec.TargetTimeout, spec.MaxAttempts, true
	if target.HealthStatus == "" {
		target.HealthStatus = "unknown"
	}
	s.targets[target.ID] = target

	rt, ok := findRoute(s.routes, org.id, proj.id, env.id, mdl.id)
	if !ok {
		rt.id, err = newID("rte")
		if err != nil {
			return platform.ProvisionResult{}, err
		}
		rt.organisationID, rt.projectID, rt.environmentID, rt.modelID = org.id, proj.id, env.id, mdl.id
		rt.bindingGeneration = 1
		rt.createdAt = now
	} else if rt.targetID != target.ID {
		rt.bindingGeneration++
	}
	if target.CapacityMode == "dedicated" && target.OwnerOrganisationID != org.id {
		return platform.ProvisionResult{}, platform.ErrConflict
	}
	rt.targetID, rt.enabled, rt.updatedAt = target.ID, true, now
	s.routes[rt.id] = rt

	account, ok := findServiceAccount(s.serviceAccounts, org.id, proj.id, env.id, spec.ServiceAccount)
	if !ok {
		account.id, err = newID("sa")
		if err != nil {
			return platform.ProvisionResult{}, err
		}
		account.organisationID, account.projectID, account.environmentID, account.name = org.id, proj.id, env.id, spec.ServiceAccount
		s.serviceAccounts[account.id] = account
	}

	result := platform.ProvisionResult{OrganisationID: org.id, ProjectID: proj.id, EnvironmentID: env.id, RouteID: rt.id, TargetID: target.ID, ServiceAccountID: account.id, Scopes: append([]string(nil), spec.Scopes...)}
	if existing, found := s.activeKeyForAccount(account.id); found {
		result.KeyPrefix = existing.prefix
		result.Scopes = append([]string(nil), existing.scopes...)
		return result, nil
	}
	generated, err := credentials.Generate()
	if err != nil {
		return platform.ProvisionResult{}, err
	}
	keyID, err := newID("key")
	if err != nil {
		return platform.ProvisionResult{}, err
	}
	s.keys[generated.Digest] = apiKey{id: keyID, serviceAccountID: account.id, prefix: generated.Prefix, digest: generated.Digest, scopes: append([]string(nil), spec.Scopes...)}
	result.KeyPrefix, result.APIKey, result.KeyCreated = generated.Prefix, generated.Token, true
	return result, nil
}

func (s *Store) RotateKey(_ context.Context, input platform.RotateKeySpec) (platform.KeyResult, error) {
	spec, err := provisioning.ValidateRotate(input)
	if err != nil {
		return platform.KeyResult{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	org, ok := findOrganisation(s.organisations, spec.OrganisationSlug)
	if !ok {
		return platform.KeyResult{}, platform.ErrNotFound
	}
	proj, ok := findProject(s.projects, org.id, spec.ProjectSlug)
	if !ok {
		return platform.KeyResult{}, platform.ErrNotFound
	}
	env, ok := findEnvironment(s.environments, org.id, proj.id, spec.EnvironmentSlug)
	if !ok {
		return platform.KeyResult{}, platform.ErrNotFound
	}
	account, ok := findServiceAccount(s.serviceAccounts, org.id, proj.id, env.id, spec.ServiceAccount)
	if !ok {
		return platform.KeyResult{}, platform.ErrNotFound
	}
	now := s.now().UTC()
	for digest, key := range s.keys {
		if key.serviceAccountID == account.id && key.revokedAt == nil {
			key.revokedAt = &now
			s.keys[digest] = key
		}
	}
	generated, err := credentials.Generate()
	if err != nil {
		return platform.KeyResult{}, err
	}
	keyID, err := newID("key")
	if err != nil {
		return platform.KeyResult{}, err
	}
	s.keys[generated.Digest] = apiKey{id: keyID, serviceAccountID: account.id, prefix: generated.Prefix, digest: generated.Digest, scopes: append([]string(nil), spec.Scopes...)}
	return platform.KeyResult{KeyPrefix: generated.Prefix, APIKey: generated.Token, Scopes: append([]string(nil), spec.Scopes...)}, nil
}

func (s *Store) RevokeKey(_ context.Context, prefix string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	for digest, key := range s.keys {
		if key.prefix == prefix {
			if key.revokedAt == nil {
				key.revokedAt = &now
				s.keys[digest] = key
			}
			return nil
		}
	}
	return platform.ErrNotFound
}

func (s *Store) Authenticate(_ context.Context, digest [32]byte) (platform.Principal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key, ok := s.keys[digest]
	if !ok || key.revokedAt != nil {
		return platform.Principal{}, platform.ErrUnauthenticated
	}
	account, ok := s.serviceAccounts[key.serviceAccountID]
	if !ok {
		return platform.Principal{}, platform.ErrUnauthenticated
	}
	org, ok1 := s.organisations[account.organisationID]
	proj, ok2 := s.projects[account.projectID]
	env, ok3 := s.environments[account.environmentID]
	if !ok1 || !ok2 || !ok3 {
		return platform.Principal{}, platform.ErrUnauthenticated
	}
	return platform.Principal{OrganisationID: org.id, OrganisationName: org.name, OrganisationSlug: org.slug, ProjectID: proj.id, ProjectName: proj.name, ProjectSlug: proj.slug, EnvironmentID: env.id, EnvironmentName: env.name, EnvironmentSlug: env.slug, ServiceAccountID: account.id, ServiceAccount: account.name, APIKeyID: key.id, KeyPrefix: key.prefix, Scopes: append([]string(nil), key.scopes...)}, nil
}

func (s *Store) ResolveRoute(_ context.Context, principal platform.Principal, alias string) (platform.Route, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	mdl, ok := findModel(s.models, alias)
	if !ok || !mdl.enabled {
		return platform.Route{}, platform.ErrNotFound
	}
	rt, ok := findRoute(s.routes, principal.OrganisationID, principal.ProjectID, principal.EnvironmentID, mdl.id)
	if !ok {
		return platform.Route{}, platform.ErrNotFound
	}
	if !rt.enabled {
		return platform.Route{}, platform.ErrUnavailable
	}
	target, ok := s.targets[rt.targetID]
	if !ok || !target.Enabled || target.HealthStatus == "unavailable" {
		return platform.Route{}, platform.ErrUnavailable
	}
	if target.CapacityMode == "dedicated" && target.OwnerOrganisationID != principal.OrganisationID {
		return platform.Route{}, platform.ErrForbidden
	}
	return materialiseRoute(rt, mdl, target), nil
}

func (s *Store) CreateInferenceRequest(_ context.Context, start platform.RequestStart) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.requests[start.ID]; exists {
		return platform.ErrConflict
	}
	s.requests[start.ID] = platform.InferenceRequest{ID: start.ID, OrganisationID: start.Principal.OrganisationID, ProjectID: start.Principal.ProjectID, EnvironmentID: start.Principal.EnvironmentID, ServiceAccountID: start.Principal.ServiceAccountID, APIKeyID: start.Principal.APIKeyID, KeyPrefix: start.Principal.KeyPrefix, ModelAlias: start.ModelAlias, StartedAt: start.StartedAt, Status: "in_progress", UsageFinality: "unknown"}
	return nil
}

func (s *Store) SetInferenceRequestRoute(_ context.Context, requestID, routeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.requests[requestID]
	if !ok {
		return platform.ErrNotFound
	}
	rt, ok := s.routes[routeID]
	mdl, modelOK := s.models[rt.modelID]
	if !ok || !modelOK || mdl.alias != record.ModelAlias || rt.organisationID != record.OrganisationID || rt.projectID != record.ProjectID || rt.environmentID != record.EnvironmentID {
		return platform.ErrForbidden
	}
	if record.Status != "in_progress" || record.RouteID != "" || record.AttemptCount != 0 {
		return platform.ErrConflict
	}
	record.RouteID = routeID
	record.BoundTargetID = rt.targetID
	record.BoundModelID = rt.modelID
	record.RouteBindingGeneration = rt.bindingGeneration
	s.requests[requestID] = record
	return nil
}

func (s *Store) CompleteInferenceRequest(_ context.Context, finish platform.RequestFinish) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.requests[finish.ID]
	if !ok {
		return platform.ErrNotFound
	}
	if record.Status != "in_progress" {
		return platform.ErrConflict
	}
	record.CompletedAt, record.Status, record.HTTPStatus, record.ErrorClass = timePointer(finish.CompletedAt), finish.Status, finish.HTTPStatus, finish.ErrorClass
	record.ExecutedModel, record.ProviderRequestID, record.Duration = finish.ExecutedModel, finish.ProviderRequestID, finish.Duration
	record.Usage, record.UsageFinality = finish.Usage, finish.UsageFinality
	s.requests[finish.ID] = record
	return nil
}

func (s *Store) CreateProviderAttempt(_ context.Context, start platform.AttemptStart) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.attempts[start.ID]; ok {
		return platform.ErrConflict
	}
	record, ok := s.requests[start.InferenceRequestID]
	if !ok {
		return platform.ErrNotFound
	}
	if record.Status != "in_progress" {
		return platform.ErrConflict
	}
	if record.RouteID == "" || record.BoundTargetID == "" || record.BoundTargetID != start.TargetID || start.AttemptNumber != record.AttemptCount+1 {
		return platform.ErrConflict
	}
	s.attempts[start.ID] = platform.ProviderAttempt{ID: start.ID, InferenceRequestID: start.InferenceRequestID, TargetID: start.TargetID, AttemptNumber: start.AttemptNumber, StartedAt: start.StartedAt, Status: "in_progress"}
	record.AttemptCount++
	s.requests[record.ID] = record
	return nil
}

func (s *Store) CompleteProviderAttempt(_ context.Context, finish platform.AttemptFinish) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	attempt, ok := s.attempts[finish.ID]
	if !ok {
		return platform.ErrNotFound
	}
	if attempt.Status != "in_progress" {
		return platform.ErrConflict
	}
	attempt.CompletedAt, attempt.Status, attempt.ProviderHTTPStatus, attempt.ErrorClass = timePointer(finish.CompletedAt), finish.Status, finish.ProviderHTTPStatus, finish.ErrorClass
	attempt.Duration, attempt.ProviderRequestID = finish.Duration, finish.ProviderRequestID
	s.attempts[finish.ID] = attempt
	return nil
}

func (s *Store) UpdateTargetHealth(_ context.Context, targetID, status string, checkedAt time.Time, successful bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	target, ok := s.targets[targetID]
	if !ok {
		return platform.ErrNotFound
	}
	target.HealthStatus, target.LastHealthCheckAt = status, timePointer(checkedAt)
	if successful {
		target.LastSuccessAt = timePointer(checkedAt)
	}
	s.targets[targetID] = target
	return nil
}

func (s *Store) ListRoutes(_ context.Context, principal platform.Principal) ([]platform.Route, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]platform.Route, 0)
	for _, rt := range s.routes {
		if rt.organisationID != principal.OrganisationID || rt.projectID != principal.ProjectID || rt.environmentID != principal.EnvironmentID {
			continue
		}
		mdl, modelOK := s.models[rt.modelID]
		target, targetOK := s.targets[rt.targetID]
		if !modelOK || !targetOK {
			continue
		}
		result = append(result, materialiseRoute(rt, mdl, target))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ModelAlias < result[j].ModelAlias })
	return result, nil
}

func (s *Store) ListInferenceRequests(_ context.Context, principal platform.Principal, filter platform.UsageFilter) (platform.RequestPage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]platform.InferenceRequest, 0)
	for _, record := range s.requests {
		if record.OrganisationID != principal.OrganisationID || record.ProjectID != principal.ProjectID || record.EnvironmentID != principal.EnvironmentID {
			continue
		}
		if !filter.From.IsZero() && record.StartedAt.Before(filter.From) {
			continue
		}
		if !filter.To.IsZero() && !record.StartedAt.Before(filter.To) {
			continue
		}
		if filter.ModelAlias != "" && record.ModelAlias != filter.ModelAlias {
			continue
		}
		items = append(items, record)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].StartedAt.After(items[j].StartedAt) })
	page := platform.RequestPage{Requests: items}
	if filter.Limit > 0 && len(page.Requests) > filter.Limit {
		page.Requests = page.Requests[:filter.Limit]
		page.Truncated = true
	}
	return page, nil
}

func (s *Store) GetInferenceRequest(_ context.Context, principal platform.Principal, requestID string) (platform.InferenceRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.requests[requestID]
	if !ok || record.OrganisationID != principal.OrganisationID || record.ProjectID != principal.ProjectID || record.EnvironmentID != principal.EnvironmentID {
		return platform.InferenceRequest{}, platform.ErrNotFound
	}
	return record, nil
}

func (s *Store) AttemptsForRequest(requestID string) []platform.ProviderAttempt {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]platform.ProviderAttempt, 0)
	for _, attempt := range s.attempts {
		if attempt.InferenceRequestID == requestID {
			result = append(result, attempt)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].AttemptNumber < result[j].AttemptNumber })
	return result
}

func (s *Store) activeKeyForAccount(accountID string) (apiKey, bool) {
	for _, key := range s.keys {
		if key.serviceAccountID == accountID && key.revokedAt == nil {
			return key, true
		}
	}
	return apiKey{}, false
}
func findOrganisation(values map[string]organisation, slug string) (organisation, bool) {
	for _, value := range values {
		if value.slug == slug {
			return value, true
		}
	}
	return organisation{}, false
}
func findProject(values map[string]project, orgID, slug string) (project, bool) {
	for _, value := range values {
		if value.organisationID == orgID && value.slug == slug {
			return value, true
		}
	}
	return project{}, false
}
func findEnvironment(values map[string]environment, orgID, projectID, slug string) (environment, bool) {
	for _, value := range values {
		if value.organisationID == orgID && value.projectID == projectID && value.slug == slug {
			return value, true
		}
	}
	return environment{}, false
}
func findModel(values map[string]model, alias string) (model, bool) {
	for _, value := range values {
		if value.alias == alias {
			return value, true
		}
	}
	return model{}, false
}
func findTarget(values map[string]platform.Target, name string) (platform.Target, bool) {
	for _, value := range values {
		if value.Name == name {
			return value, true
		}
	}
	return platform.Target{}, false
}
func findRoute(values map[string]route, orgID, projectID, envID, modelID string) (route, bool) {
	for _, value := range values {
		if value.organisationID == orgID && value.projectID == projectID && value.environmentID == envID && value.modelID == modelID {
			return value, true
		}
	}
	return route{}, false
}
func findServiceAccount(values map[string]serviceAccount, orgID, projectID, envID, name string) (serviceAccount, bool) {
	for _, value := range values {
		if value.organisationID == orgID && value.projectID == projectID && value.environmentID == envID && value.name == name {
			return value, true
		}
	}
	return serviceAccount{}, false
}
func materialiseRoute(rt route, mdl model, target platform.Target) platform.Route {
	return platform.Route{ID: rt.id, OrganisationID: rt.organisationID, ProjectID: rt.projectID, EnvironmentID: rt.environmentID, ModelID: mdl.id, ModelAlias: mdl.alias, ModelVersion: mdl.version, ModelEnabled: mdl.enabled, BindingGeneration: rt.bindingGeneration, Enabled: rt.enabled, Target: target, CreatedAt: rt.createdAt, UpdatedAt: rt.updatedAt}
}
func timePointer(value time.Time) *time.Time { copy := value; return &copy }

var _ platform.Store = (*Store)(nil)
var _ platform.Provisioner = (*Store)(nil)
