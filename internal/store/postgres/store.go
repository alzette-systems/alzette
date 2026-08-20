package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"alzette/internal/platform"
)

type Store struct {
	db                   *sql.DB
	allowInsecureTargets bool
	now                  func() time.Time
}

func New(db *sql.DB, allowInsecureTargets bool) *Store {
	return &Store{db: db, allowInsecureTargets: allowInsecureTargets, now: time.Now}
}

func (s *Store) SetClock(clock func() time.Time) {
	if clock != nil {
		s.now = clock
	}
}

func (s *Store) Authenticate(ctx context.Context, digest [32]byte) (platform.Principal, error) {
	var principal platform.Principal
	var scopesJSON []byte
	err := s.db.QueryRowContext(ctx, `
		UPDATE api_keys AS k
		   SET last_used_at = now()
		  FROM service_accounts AS sa,
		       environments AS e,
		       projects AS p,
		       organisations AS o
		 WHERE k.key_hash = $1
		   AND k.revoked_at IS NULL
		   AND (k.expires_at IS NULL OR k.expires_at > now())
		   AND sa.id = k.service_account_id
		   AND e.id = sa.environment_id AND e.project_id = sa.project_id AND e.organisation_id = sa.organisation_id
		   AND p.id = sa.project_id AND p.organisation_id = sa.organisation_id
		   AND o.id = sa.organisation_id
		 RETURNING o.id, o.name, o.slug, p.id, p.name, p.slug,
		           e.id, e.name, e.slug, sa.id, sa.name,
		           k.id, k.key_prefix, k.scopes`, digest[:]).Scan(
		&principal.OrganisationID, &principal.OrganisationName, &principal.OrganisationSlug,
		&principal.ProjectID, &principal.ProjectName, &principal.ProjectSlug,
		&principal.EnvironmentID, &principal.EnvironmentName, &principal.EnvironmentSlug,
		&principal.ServiceAccountID, &principal.ServiceAccount,
		&principal.APIKeyID, &principal.KeyPrefix, &scopesJSON,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return platform.Principal{}, platform.ErrUnauthenticated
	}
	if err != nil {
		return platform.Principal{}, fmt.Errorf("authenticate API key: %w", err)
	}
	if err := json.Unmarshal(scopesJSON, &principal.Scopes); err != nil {
		return platform.Principal{}, fmt.Errorf("decode API key scopes: %w", err)
	}
	principal.CredentialKind = "service_account_key"
	return principal, nil
}

func (s *Store) ResolveRoute(ctx context.Context, principal platform.Principal, alias string) (platform.Route, error) {
	if !principal.AllowsModel(alias) {
		return platform.Route{}, platform.ErrForbidden
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return platform.Route{}, fmt.Errorf("begin route resolution: %w", err)
	}
	defer tx.Rollback()
	var routeID string
	err = tx.QueryRowContext(ctx, `
		SELECT r.id
		  FROM tenant_routes r
		  JOIN models m ON m.id = r.model_id
		 WHERE r.organisation_id = $1 AND r.project_id = $2 AND r.environment_id = $3
		   AND m.alias = $4 AND m.enabled = true
		 FOR SHARE OF r`, principal.OrganisationID, principal.ProjectID, principal.EnvironmentID, alias).Scan(&routeID)
	if errors.Is(err, sql.ErrNoRows) {
		return platform.Route{}, platform.ErrNotFound
	}
	if err != nil {
		return platform.Route{}, fmt.Errorf("lock route for resolution: %w", err)
	}
	var route platform.Route
	var owner sql.NullString
	var timeoutMS int64
	var lastCheck, lastSuccess sql.NullTime
	err = tx.QueryRowContext(ctx, routeSelect+`
		 WHERE r.id = $1
		   AND r.organisation_id = $2 AND r.project_id = $3 AND r.environment_id = $4
		 FOR SHARE OF t`, routeID, principal.OrganisationID, principal.ProjectID, principal.EnvironmentID).Scan(routeScan(&route, &owner, &timeoutMS, &lastCheck, &lastSuccess)...)
	if errors.Is(err, sql.ErrNoRows) {
		return platform.Route{}, platform.ErrNotFound
	}
	if err != nil {
		return platform.Route{}, fmt.Errorf("resolve route: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return platform.Route{}, fmt.Errorf("commit route resolution: %w", err)
	}
	finishRouteScan(&route, owner, timeoutMS, lastCheck, lastSuccess)
	if !route.Enabled || !route.Target.Enabled || route.Target.HealthStatus == "unavailable" {
		return platform.Route{}, platform.ErrUnavailable
	}
	if route.Target.CapacityMode == "dedicated" && route.Target.OwnerOrganisationID != principal.OrganisationID {
		return platform.Route{}, platform.ErrForbidden
	}
	return route, nil
}

func (s *Store) CreateInferenceRequest(ctx context.Context, start platform.RequestStart) error {
	var err error
	if start.Principal.CredentialKind == "human_agent_token" {
		_, err = s.db.ExecContext(ctx, `
		INSERT INTO inference_requests (
			id, organisation_id, project_id, environment_id, human_user_id,
			human_membership_id, agent_grant_id, agent_token_id, model_alias, started_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			start.ID, start.Principal.OrganisationID, start.Principal.ProjectID, start.Principal.EnvironmentID,
			start.Principal.HumanUserID, start.Principal.HumanMembershipID, start.Principal.AgentGrantID,
			start.Principal.AgentTokenID, start.ModelAlias, start.StartedAt)
	} else {
		_, err = s.db.ExecContext(ctx, `
		INSERT INTO inference_requests (
			id, organisation_id, project_id, environment_id, service_account_id,
			api_key_id, key_prefix, model_alias, started_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			start.ID, start.Principal.OrganisationID, start.Principal.ProjectID, start.Principal.EnvironmentID,
			start.Principal.ServiceAccountID, start.Principal.APIKeyID, start.Principal.KeyPrefix, start.ModelAlias, start.StartedAt)
	}
	if err != nil {
		return mapWriteError("create inference request", err)
	}
	return nil
}

func (s *Store) SetInferenceRequestRoute(ctx context.Context, requestID, routeID string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE inference_requests ir
		   SET route_id = r.id,
		       bound_target_id = r.target_id,
		       bound_model_id = r.model_id,
		       route_binding_generation = r.binding_generation
		  FROM tenant_routes r
		  JOIN models m ON m.id = r.model_id
		 WHERE ir.id = $1 AND ir.status = 'in_progress'
		   AND ir.route_id IS NULL
		   AND ir.attempt_count = 0
		   AND r.id = $2
		   AND r.organisation_id = ir.organisation_id
		   AND r.project_id = ir.project_id
		   AND r.environment_id = ir.environment_id
		   AND m.alias = ir.model_alias`, requestID, routeID)
	if err != nil {
		return mapWriteError("attach inference route", err)
	}
	return requireAffected(result)
}

func (s *Store) CompleteInferenceRequest(ctx context.Context, finish platform.RequestFinish) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE inference_requests
		   SET completed_at = $2, status = $3, http_status = $4, error_class = NULLIF($5, ''),
		       executed_model = NULLIF($6, ''), provider_request_id = NULLIF($7, ''), duration_ms = $8,
		       input_tokens = $9, output_tokens = $10, cached_tokens = $11, reasoning_tokens = $12,
		       usage_finality = $13, total_tokens = $14, cached_write_tokens = $15,
		       cached_write_tokens_5m = $16, cached_write_tokens_1h = $17,
		       text_input_tokens = $18, audio_input_tokens = $19, image_input_tokens = $20,
		       usage_normalization_version = NULLIF($21, '')
		 WHERE id = $1 AND status = 'in_progress'`,
		finish.ID, finish.CompletedAt, finish.Status, nullableInt(finish.HTTPStatus), finish.ErrorClass,
		finish.ExecutedModel, finish.ProviderRequestID, milliseconds(finish.Duration),
		nullableToken(finish.Usage.InputTokens), nullableToken(finish.Usage.OutputTokens),
		nullableToken(finish.Usage.CachedTokens), nullableToken(finish.Usage.ReasoningTokens), finish.UsageFinality,
		nullableToken(finish.Usage.TotalTokens), nullableToken(finish.Usage.CachedWriteTokens),
		nullableToken(finish.Usage.CachedWriteTokens5m), nullableToken(finish.Usage.CachedWriteTokens1h),
		nullableToken(finish.Usage.TextInputTokens), nullableToken(finish.Usage.AudioInputTokens),
		nullableToken(finish.Usage.ImageInputTokens), finish.Usage.Normalization)
	if err != nil {
		return mapWriteError("complete inference request", err)
	}
	return requireAffected(result)
}

func (s *Store) CreateProviderAttempt(ctx context.Context, start platform.AttemptStart) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE inference_requests SET attempt_count = attempt_count + 1 WHERE id = $1 AND status = 'in_progress'`, start.InferenceRequestID)
	if err != nil {
		return mapWriteError("increment attempt count", err)
	}
	if err := requireAffected(result); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO provider_attempts (id, inference_request_id, target_id, attempt_number, started_at)
		VALUES ($1,$2,$3,$4,$5)`, start.ID, start.InferenceRequestID, start.TargetID, start.AttemptNumber, start.StartedAt)
	if err != nil {
		return mapWriteError("create provider attempt", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit provider attempt: %w", err)
	}
	return nil
}

func (s *Store) CompleteProviderAttempt(ctx context.Context, finish platform.AttemptFinish) error {
	if finish.UsageFinality == "" {
		finish.UsageFinality = "unknown"
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE provider_attempts
		   SET completed_at = $2, status = $3, provider_http_status = $4,
		       error_class = NULLIF($5, ''), duration_ms = $6, provider_request_id = NULLIF($7, ''),
		       input_tokens = $8, output_tokens = $9, total_tokens = $10,
		       cached_read_tokens = $11, cached_write_tokens = $12,
		       cached_write_tokens_5m = $13, cached_write_tokens_1h = $14,
		       reasoning_tokens = $15, text_input_tokens = $16,
		       audio_input_tokens = $17, image_input_tokens = $18,
		       usage_finality = $19, usage_normalization_version = NULLIF($20, '')
		 WHERE id = $1 AND status = 'in_progress'`,
		finish.ID, finish.CompletedAt, finish.Status, nullableInt(finish.ProviderHTTPStatus),
		finish.ErrorClass, milliseconds(finish.Duration), finish.ProviderRequestID,
		nullableToken(finish.Usage.InputTokens), nullableToken(finish.Usage.OutputTokens),
		nullableToken(finish.Usage.TotalTokens), nullableToken(finish.Usage.CachedTokens),
		nullableToken(finish.Usage.CachedWriteTokens), nullableToken(finish.Usage.CachedWriteTokens5m),
		nullableToken(finish.Usage.CachedWriteTokens1h), nullableToken(finish.Usage.ReasoningTokens),
		nullableToken(finish.Usage.TextInputTokens), nullableToken(finish.Usage.AudioInputTokens),
		nullableToken(finish.Usage.ImageInputTokens), finish.UsageFinality, finish.Usage.Normalization)
	if err != nil {
		return mapWriteError("complete provider attempt", err)
	}
	return requireAffected(result)
}

func (s *Store) UpdateTargetHealth(ctx context.Context, targetID, status string, checkedAt time.Time, successful bool) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE inference_targets
		   SET health_status = $2, last_health_check_at = $3,
		       last_success_at = CASE WHEN $4 THEN $3 ELSE last_success_at END,
		       updated_at = now()
		 WHERE id = $1`, targetID, status, checkedAt, successful)
	if err != nil {
		return mapWriteError("update target health", err)
	}
	return requireAffected(result)
}

func (s *Store) ListRoutes(ctx context.Context, principal platform.Principal) ([]platform.Route, error) {
	rows, err := s.db.QueryContext(ctx, routeSelect+`
		 WHERE r.organisation_id = $1 AND r.project_id = $2 AND r.environment_id = $3
		 ORDER BY m.alias`, principal.OrganisationID, principal.ProjectID, principal.EnvironmentID)
	if err != nil {
		return nil, fmt.Errorf("list routes: %w", err)
	}
	defer rows.Close()
	var routes []platform.Route
	for rows.Next() {
		var route platform.Route
		var owner sql.NullString
		var timeoutMS int64
		var lastCheck, lastSuccess sql.NullTime
		if err := rows.Scan(routeScan(&route, &owner, &timeoutMS, &lastCheck, &lastSuccess)...); err != nil {
			return nil, err
		}
		finishRouteScan(&route, owner, timeoutMS, lastCheck, lastSuccess)
		routes = append(routes, route)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return routes, nil
}

func (s *Store) ListInferenceRequests(ctx context.Context, principal platform.Principal, filter platform.UsageFilter) (platform.RequestPage, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 10000
	}
	rows, err := s.db.QueryContext(ctx, requestSelect+`
		 WHERE organisation_id = $1 AND project_id = $2 AND environment_id = $3
		   AND started_at >= $4 AND started_at < $5
		   AND ($6 = '' OR model_alias = $6)
		 ORDER BY started_at DESC
		 LIMIT $7`, principal.OrganisationID, principal.ProjectID, principal.EnvironmentID, filter.From, filter.To, filter.ModelAlias, limit+1)
	if err != nil {
		return platform.RequestPage{}, fmt.Errorf("list inference requests: %w", err)
	}
	defer rows.Close()
	page := platform.RequestPage{}
	for rows.Next() {
		record, err := scanRequest(rows)
		if err != nil {
			return platform.RequestPage{}, err
		}
		page.Requests = append(page.Requests, record)
	}
	if err := rows.Err(); err != nil {
		return platform.RequestPage{}, err
	}
	if len(page.Requests) > limit {
		page.Requests = page.Requests[:limit]
		page.Truncated = true
	}
	return page, nil
}

func (s *Store) GetInferenceRequest(ctx context.Context, principal platform.Principal, requestID string) (platform.InferenceRequest, error) {
	row := s.db.QueryRowContext(ctx, requestSelect+`
		 WHERE id = $1 AND organisation_id = $2 AND project_id = $3 AND environment_id = $4`,
		requestID, principal.OrganisationID, principal.ProjectID, principal.EnvironmentID)
	record, err := scanRequest(row)
	if errors.Is(err, sql.ErrNoRows) {
		return platform.InferenceRequest{}, platform.ErrNotFound
	}
	if err != nil {
		return platform.InferenceRequest{}, fmt.Errorf("get inference request: %w", err)
	}
	return record, nil
}

const routeSelect = `
	SELECT r.id, r.organisation_id, r.project_id, r.environment_id,
	       m.id, m.alias, m.version, m.enabled, r.binding_generation, r.enabled, r.created_at, r.updated_at,
	       t.id, t.name, t.execution_class, t.capacity_mode, COALESCE(t.capacity_evidence_ref, ''), t.owner_organisation_id,
	       t.base_url, t.provider_model, t.secret_ref, t.timeout_ms, t.max_attempts,
	       t.enabled, t.health_status, t.last_health_check_at, t.last_success_at
	  FROM tenant_routes r
	  JOIN models m ON m.id = r.model_id
	  JOIN inference_targets t ON t.id = r.target_id`

func routeScan(route *platform.Route, owner *sql.NullString, timeoutMS *int64, lastCheck, lastSuccess *sql.NullTime) []interface{} {
	return []interface{}{
		&route.ID, &route.OrganisationID, &route.ProjectID, &route.EnvironmentID,
		&route.ModelID, &route.ModelAlias, &route.ModelVersion, &route.ModelEnabled, &route.BindingGeneration, &route.Enabled, &route.CreatedAt, &route.UpdatedAt,
		&route.Target.ID, &route.Target.Name, &route.Target.ExecutionClass, &route.Target.CapacityMode, &route.Target.CapacityEvidenceRef, owner,
		&route.Target.BaseURL, &route.Target.ProviderModel, &route.Target.SecretRef, timeoutMS, &route.Target.MaxAttempts,
		&route.Target.Enabled, &route.Target.HealthStatus, lastCheck, lastSuccess,
	}
}

func finishRouteScan(route *platform.Route, owner sql.NullString, timeoutMS int64, lastCheck, lastSuccess sql.NullTime) {
	if owner.Valid {
		route.Target.OwnerOrganisationID = owner.String
	}
	route.Target.Timeout = time.Duration(timeoutMS) * time.Millisecond
	if lastCheck.Valid {
		route.Target.LastHealthCheckAt = &lastCheck.Time
	}
	if lastSuccess.Valid {
		route.Target.LastSuccessAt = &lastSuccess.Time
	}
}

const requestSelect = `
	SELECT id, organisation_id, project_id, environment_id, COALESCE(route_id, ''),
	       COALESCE(bound_target_id, ''), COALESCE(bound_model_id, ''), COALESCE(route_binding_generation, 0),
	       service_account_id, api_key_id, key_prefix, model_alias,
	       COALESCE(executed_model, ''), COALESCE(provider_request_id, ''), started_at,
	       completed_at, status, COALESCE(http_status, 0), COALESCE(error_class, ''),
	       COALESCE(duration_ms, 0), input_tokens, output_tokens, cached_tokens, reasoning_tokens,
	       usage_finality, attempt_count, total_tokens, cached_write_tokens,
	       cached_write_tokens_5m, cached_write_tokens_1h, text_input_tokens,
	       audio_input_tokens, image_input_tokens, COALESCE(usage_normalization_version, '')
	  FROM inference_requests`

type scanner interface{ Scan(...interface{}) error }

func scanRequest(row scanner) (platform.InferenceRequest, error) {
	var record platform.InferenceRequest
	var completed sql.NullTime
	var durationMS int64
	var input, output, cached, reasoning, total, cachedWrite, cachedWrite5m, cachedWrite1h sql.NullInt64
	var textInput, audioInput, imageInput sql.NullInt64
	err := row.Scan(&record.ID, &record.OrganisationID, &record.ProjectID, &record.EnvironmentID, &record.RouteID,
		&record.BoundTargetID, &record.BoundModelID, &record.RouteBindingGeneration,
		&record.ServiceAccountID, &record.APIKeyID, &record.KeyPrefix, &record.ModelAlias,
		&record.ExecutedModel, &record.ProviderRequestID, &record.StartedAt, &completed, &record.Status,
		&record.HTTPStatus, &record.ErrorClass, &durationMS, &input, &output, &cached, &reasoning,
		&record.UsageFinality, &record.AttemptCount, &total, &cachedWrite, &cachedWrite5m,
		&cachedWrite1h, &textInput, &audioInput, &imageInput, &record.Usage.Normalization)
	if err != nil {
		return platform.InferenceRequest{}, err
	}
	if completed.Valid {
		record.CompletedAt = &completed.Time
	}
	record.Duration = time.Duration(durationMS) * time.Millisecond
	record.Usage.InputTokens = intPointer(input)
	record.Usage.OutputTokens = intPointer(output)
	record.Usage.CachedTokens = intPointer(cached)
	record.Usage.ReasoningTokens = intPointer(reasoning)
	record.Usage.TotalTokens = intPointer(total)
	record.Usage.CachedWriteTokens = intPointer(cachedWrite)
	record.Usage.CachedWriteTokens5m = intPointer(cachedWrite5m)
	record.Usage.CachedWriteTokens1h = intPointer(cachedWrite1h)
	record.Usage.TextInputTokens = intPointer(textInput)
	record.Usage.AudioInputTokens = intPointer(audioInput)
	record.Usage.ImageInputTokens = intPointer(imageInput)
	return record, nil
}

func intPointer(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
}
func nullableToken(value *int64) interface{} {
	if value == nil {
		return nil
	}
	return *value
}
func nullableInt(value int) interface{} {
	if value == 0 {
		return nil
	}
	return value
}
func milliseconds(value time.Duration) int64 {
	if value < 0 {
		return 0
	}
	return value.Milliseconds()
}
func requireAffected(result sql.Result) error {
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return platform.ErrNotFound
	}
	return nil
}
func mapWriteError(operation string, err error) error { return fmt.Errorf("%s: %w", operation, err) }

var _ platform.Store = (*Store)(nil)
