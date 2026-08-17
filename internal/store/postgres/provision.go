package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"alzette/internal/credentials"
	"alzette/internal/ids"
	"alzette/internal/platform"
	"alzette/internal/provisioning"
)

func (s *Store) Provision(ctx context.Context, input platform.ProvisionSpec) (platform.ProvisionResult, error) {
	spec, err := provisioning.Validate(input, s.allowInsecureTargets)
	if err != nil {
		return platform.ProvisionResult{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return platform.ProvisionResult{}, err
	}
	defer tx.Rollback()
	plansAvailable, probesAvailable, err := provisioningSchemaCapabilities(ctx, tx)
	if err != nil {
		return platform.ProvisionResult{}, err
	}

	orgID, err := upsertOrganisation(ctx, tx, spec)
	if err != nil {
		return platform.ProvisionResult{}, err
	}
	projectID, err := upsertProject(ctx, tx, orgID, spec)
	if err != nil {
		return platform.ProvisionResult{}, err
	}
	environmentID, err := upsertEnvironment(ctx, tx, orgID, projectID, spec)
	if err != nil {
		return platform.ProvisionResult{}, err
	}
	modelID, err := upsertModel(ctx, tx, spec)
	if err != nil {
		return platform.ProvisionResult{}, err
	}
	// ResolveRoute locks an existing route before its target. Take the same
	// route-before-target order here so provisioning cannot form a deadlock
	// cycle with concurrent inference route resolution.
	routeIDBefore, err := lockExistingRoute(ctx, tx, orgID, projectID, environmentID, modelID)
	if err != nil {
		return platform.ProvisionResult{}, err
	}
	priorPlanID := ""
	if plansAvailable {
		priorPlanID, err = prepareServicePlanTransition(ctx, tx, orgID, projectID, environmentID, routeIDBefore, spec)
		if err != nil {
			return platform.ProvisionResult{}, err
		}
	}
	targetID, err := upsertTarget(ctx, tx, orgID, spec, probesAvailable)
	if err != nil {
		return platform.ProvisionResult{}, err
	}
	routeID, err := upsertRoute(ctx, tx, orgID, projectID, environmentID, modelID, targetID)
	if err != nil {
		return platform.ProvisionResult{}, err
	}
	if plansAvailable {
		if err := upsertServicePlan(ctx, tx, orgID, projectID, environmentID, routeID, spec); err != nil {
			return platform.ProvisionResult{}, err
		}
	}
	accountID, err := upsertServiceAccount(ctx, tx, orgID, projectID, environmentID, spec.ServiceAccount)
	if err != nil {
		return platform.ProvisionResult{}, err
	}

	result := platform.ProvisionResult{OrganisationID: orgID, ProjectID: projectID, EnvironmentID: environmentID, RouteID: routeID, TargetID: targetID, ServiceAccountID: accountID, Scopes: append([]string(nil), spec.Scopes...), ServicePlanCode: spec.ServicePlanCode}
	var existingScopes []byte
	err = tx.QueryRowContext(ctx, `
		SELECT key_prefix, scopes FROM api_keys
		 WHERE service_account_id = $1 AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > now())
		 ORDER BY created_at DESC LIMIT 1`, accountID).Scan(&result.KeyPrefix, &existingScopes)
	if err == nil {
		if err := json.Unmarshal(existingScopes, &result.Scopes); err != nil {
			return platform.ProvisionResult{}, err
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return platform.ProvisionResult{}, err
	} else {
		generated, err := credentials.Generate()
		if err != nil {
			return platform.ProvisionResult{}, err
		}
		keyID, err := ids.New("key")
		if err != nil {
			return platform.ProvisionResult{}, err
		}
		scopes, _ := json.Marshal(spec.Scopes)
		if err := insertOperatorAPIKey(ctx, tx, keyID, accountID, generated, scopes); err != nil {
			return platform.ProvisionResult{}, mapWriteError("issue API key", err)
		}
		result.KeyPrefix, result.APIKey, result.KeyCreated = generated.Prefix, generated.Token, true
	}
	auditMetadata := map[string]string{"route_id": routeID, "target_id": targetID, "service_account_id": accountID, "key_prefix": result.KeyPrefix}
	auditMetadata["probe_enabled"] = fmt.Sprintf("%t", spec.ProbeEnabled)
	auditMetadata["probe_interval_seconds"] = fmt.Sprintf("%d", int(spec.ProbeInterval/time.Second))
	if priorPlanID != "" {
		auditMetadata["prior_service_plan_id"] = priorPlanID
	}
	if err := insertAudit(ctx, tx, orgID, projectID, "operator.provision", auditMetadata); err != nil {
		return platform.ProvisionResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return platform.ProvisionResult{}, err
	}
	return result, nil
}

func provisioningSchemaCapabilities(ctx context.Context, tx *sql.Tx) (plans, probes bool, err error) {
	err = tx.QueryRowContext(ctx, `
		SELECT to_regclass(current_schema() || '.tenant_service_plans') IS NOT NULL,
		       EXISTS (
		           SELECT 1 FROM information_schema.columns
		            WHERE table_schema=current_schema()
		              AND table_name='inference_targets'
		              AND column_name='probe_enabled'
		       )`).Scan(&plans, &probes)
	return plans, probes, err
}

func (s *Store) RotateKey(ctx context.Context, input platform.RotateKeySpec) (platform.KeyResult, error) {
	spec, err := provisioning.ValidateRotate(input)
	if err != nil {
		return platform.KeyResult{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return platform.KeyResult{}, err
	}
	defer tx.Rollback()
	var orgID, projectID, accountID string
	err = tx.QueryRowContext(ctx, `
		SELECT o.id, p.id, sa.id
		  FROM service_accounts sa
		  JOIN environments e ON e.id = sa.environment_id
		  JOIN projects p ON p.id = sa.project_id
		  JOIN organisations o ON o.id = sa.organisation_id
		 WHERE o.slug=$1 AND p.slug=$2 AND e.slug=$3 AND sa.name=$4
		 FOR UPDATE OF sa`, spec.OrganisationSlug, spec.ProjectSlug, spec.EnvironmentSlug, spec.ServiceAccount).Scan(&orgID, &projectID, &accountID)
	if errors.Is(err, sql.ErrNoRows) {
		return platform.KeyResult{}, platform.ErrNotFound
	}
	if err != nil {
		return platform.KeyResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE api_keys SET revoked_at=now() WHERE service_account_id=$1 AND revoked_at IS NULL`, accountID); err != nil {
		return platform.KeyResult{}, err
	}
	generated, err := credentials.Generate()
	if err != nil {
		return platform.KeyResult{}, err
	}
	keyID, err := ids.New("key")
	if err != nil {
		return platform.KeyResult{}, err
	}
	scopes, _ := json.Marshal(spec.Scopes)
	if err := insertOperatorAPIKey(ctx, tx, keyID, accountID, generated, scopes); err != nil {
		return platform.KeyResult{}, mapWriteError("rotate API key", err)
	}
	if err := insertAudit(ctx, tx, orgID, projectID, "api_key.rotated", map[string]string{"service_account_id": accountID, "key_prefix": generated.Prefix}); err != nil {
		return platform.KeyResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return platform.KeyResult{}, err
	}
	return platform.KeyResult{KeyPrefix: generated.Prefix, APIKey: generated.Token, Scopes: append([]string(nil), spec.Scopes...)}, nil
}

func insertOperatorAPIKey(ctx context.Context, tx *sql.Tx, keyID, accountID string, generated credentials.Key, scopes []byte) error {
	var namesAvailable bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (
		SELECT 1 FROM information_schema.columns
		 WHERE table_schema=current_schema() AND table_name='api_keys' AND column_name='name'
	)`).Scan(&namesAvailable); err != nil {
		return err
	}
	if !namesAvailable {
		_, err := tx.ExecContext(ctx, `INSERT INTO api_keys (id, service_account_id, key_prefix, key_hash, scopes) VALUES ($1,$2,$3,$4,$5)`, keyID, accountID, generated.Prefix, generated.Digest[:], scopes)
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO api_keys (id, service_account_id, key_prefix, key_hash, scopes, name) VALUES ($1,$2,$3,$4,$5,$6)`, keyID, accountID, generated.Prefix, generated.Digest[:], scopes, "operator-"+generated.Prefix)
	return err
}

func (s *Store) RevokeKey(ctx context.Context, prefix string) error {
	if prefix == "" {
		return platform.ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var orgID, projectID, accountID string
	err = tx.QueryRowContext(ctx, `
		UPDATE api_keys k SET revoked_at=now()
		  FROM service_accounts sa
		 WHERE k.key_prefix=$1 AND k.revoked_at IS NULL AND sa.id=k.service_account_id
		 RETURNING sa.organisation_id, sa.project_id, sa.id`, prefix).Scan(&orgID, &projectID, &accountID)
	if errors.Is(err, sql.ErrNoRows) {
		return platform.ErrNotFound
	}
	if err != nil {
		return err
	}
	if err := insertAudit(ctx, tx, orgID, projectID, "api_key.revoked", map[string]string{"service_account_id": accountID, "key_prefix": prefix}); err != nil {
		return err
	}
	return tx.Commit()
}

func upsertOrganisation(ctx context.Context, tx *sql.Tx, spec platform.ProvisionSpec) (string, error) {
	id, err := ids.New("org")
	if err != nil {
		return "", err
	}
	var result string
	err = tx.QueryRowContext(ctx, `INSERT INTO organisations (id,slug,name) VALUES ($1,$2,$3) ON CONFLICT (slug) DO UPDATE SET name=EXCLUDED.name,updated_at=now() RETURNING id`, id, spec.OrganisationSlug, spec.OrganisationName).Scan(&result)
	return result, err
}
func upsertProject(ctx context.Context, tx *sql.Tx, orgID string, spec platform.ProvisionSpec) (string, error) {
	id, err := ids.New("prj")
	if err != nil {
		return "", err
	}
	var result string
	err = tx.QueryRowContext(ctx, `INSERT INTO projects (id,organisation_id,slug,name) VALUES ($1,$2,$3,$4) ON CONFLICT (organisation_id,slug) DO UPDATE SET name=EXCLUDED.name,updated_at=now() RETURNING id`, id, orgID, spec.ProjectSlug, spec.ProjectName).Scan(&result)
	return result, err
}
func upsertEnvironment(ctx context.Context, tx *sql.Tx, orgID, projectID string, spec platform.ProvisionSpec) (string, error) {
	id, err := ids.New("env")
	if err != nil {
		return "", err
	}
	var result string
	err = tx.QueryRowContext(ctx, `INSERT INTO environments (id,organisation_id,project_id,slug,name) VALUES ($1,$2,$3,$4,$5) ON CONFLICT (organisation_id,project_id,slug) DO UPDATE SET name=EXCLUDED.name,updated_at=now() RETURNING id`, id, orgID, projectID, spec.EnvironmentSlug, spec.EnvironmentName).Scan(&result)
	return result, err
}
func upsertModel(ctx context.Context, tx *sql.Tx, spec platform.ProvisionSpec) (string, error) {
	var existingID, existingVersion string
	err := tx.QueryRowContext(ctx, `SELECT id,version FROM models WHERE alias=$1 FOR UPDATE`, spec.ModelAlias).Scan(&existingID, &existingVersion)
	if err == nil {
		if existingVersion != spec.ModelVersion {
			return "", fmt.Errorf("model alias already has a different version: %w", platform.ErrConflict)
		}
		return existingID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	id, err := ids.New("mdl")
	if err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO models (id,alias,version) VALUES ($1,$2,$3)`, id, spec.ModelAlias, spec.ModelVersion)
	return id, err
}
func upsertTarget(ctx context.Context, tx *sql.Tx, orgID string, spec platform.ProvisionSpec, probesAvailable bool) (string, error) {
	var id, executionClass, capacityMode, capacityEvidenceRef, baseURL, providerModel, secretRef string
	var owner sql.NullString
	var timeoutMS, maxAttempts int
	err := tx.QueryRowContext(ctx, `SELECT id,execution_class,capacity_mode,COALESCE(capacity_evidence_ref,''),owner_organisation_id,base_url,provider_model,secret_ref,timeout_ms,max_attempts FROM inference_targets WHERE name=$1 FOR UPDATE`, spec.TargetName).Scan(&id, &executionClass, &capacityMode, &capacityEvidenceRef, &owner, &baseURL, &providerModel, &secretRef, &timeoutMS, &maxAttempts)
	wantedOwner := ""
	if spec.CapacityMode == "dedicated" {
		wantedOwner = orgID
	}
	if err == nil {
		actualOwner := ""
		if owner.Valid {
			actualOwner = owner.String
		}
		if executionClass != spec.ExecutionClass || capacityMode != spec.CapacityMode || capacityEvidenceRef != spec.CapacityEvidenceRef || actualOwner != wantedOwner || baseURL != spec.TargetBaseURL || providerModel != spec.ProviderModel || secretRef != spec.SecretRef || timeoutMS != int(spec.TargetTimeout.Milliseconds()) || maxAttempts != spec.MaxAttempts {
			return "", fmt.Errorf("target name already has different immutable configuration: %w", platform.ErrConflict)
		}
		if probesAvailable {
			if _, err := tx.ExecContext(ctx, `UPDATE inference_targets SET probe_enabled=$2,probe_interval_seconds=$3,updated_at=now() WHERE id=$1`, id, spec.ProbeEnabled, int(spec.ProbeInterval/time.Second)); err != nil {
				return "", err
			}
		}
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	id, err = ids.New("tgt")
	if err != nil {
		return "", err
	}
	var ownerValue interface{}
	if wantedOwner != "" {
		ownerValue = wantedOwner
	}
	var evidenceValue interface{}
	if spec.CapacityEvidenceRef != "" {
		evidenceValue = spec.CapacityEvidenceRef
	}
	if probesAvailable {
		_, err = tx.ExecContext(ctx, `INSERT INTO inference_targets (id,name,execution_class,capacity_mode,capacity_evidence_ref,owner_organisation_id,base_url,provider_model,secret_ref,timeout_ms,max_attempts,probe_enabled,probe_interval_seconds) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, id, spec.TargetName, spec.ExecutionClass, spec.CapacityMode, evidenceValue, ownerValue, spec.TargetBaseURL, spec.ProviderModel, spec.SecretRef, spec.TargetTimeout.Milliseconds(), spec.MaxAttempts, spec.ProbeEnabled, int(spec.ProbeInterval/time.Second))
	} else {
		_, err = tx.ExecContext(ctx, `INSERT INTO inference_targets (id,name,execution_class,capacity_mode,capacity_evidence_ref,owner_organisation_id,base_url,provider_model,secret_ref,timeout_ms,max_attempts) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, id, spec.TargetName, spec.ExecutionClass, spec.CapacityMode, evidenceValue, ownerValue, spec.TargetBaseURL, spec.ProviderModel, spec.SecretRef, spec.TargetTimeout.Milliseconds(), spec.MaxAttempts)
	}
	return id, err
}

func prepareServicePlanTransition(ctx context.Context, tx *sql.Tx, orgID, projectID, environmentID, routeID string, spec platform.ProvisionSpec) (string, error) {
	if routeID == "" {
		return "", nil
	}
	var targetMode string
	if err := tx.QueryRowContext(ctx, `SELECT t.capacity_mode FROM tenant_routes r JOIN inference_targets t ON t.id=r.target_id WHERE r.id=$1 FOR UPDATE OF r`, routeID).Scan(&targetMode); err != nil {
		return "", err
	}
	var planID, planCode string
	err := tx.QueryRowContext(ctx, `
		SELECT tsp.service_plan_id,sp.code
		  FROM tenant_service_plans tsp
		  JOIN service_plans sp ON sp.organisation_id=tsp.organisation_id AND sp.id=tsp.service_plan_id
		 WHERE tsp.organisation_id=$1 AND tsp.project_id=$2 AND tsp.environment_id=$3
		   AND tsp.route_id=$4 AND tsp.status='active'
		 FOR UPDATE OF tsp`, orgID, projectID, environmentID, routeID).Scan(&planID, &planCode)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if targetMode == spec.CapacityMode && (spec.ServicePlanCode == "" || planCode == spec.ServicePlanCode) {
		return "", nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE tenant_service_plans SET status='inactive',updated_at=now() WHERE organisation_id=$1 AND project_id=$2 AND environment_id=$3 AND route_id=$4 AND service_plan_id=$5 AND status='active'`, orgID, projectID, environmentID, routeID, planID); err != nil {
		return "", err
	}
	return planID, nil
}

func upsertServicePlan(ctx context.Context, tx *sql.Tx, orgID, projectID, environmentID, routeID string, spec platform.ProvisionSpec) error {
	if spec.ServicePlanCode == "" {
		return nil
	}
	planID, err := ids.New("plan")
	if err != nil {
		return err
	}
	var requestUnit, tokenUnit interface{}
	if spec.SharedRequestAllowance != nil {
		requestUnit = "logical_requests"
	}
	if spec.SharedTokenAllowance != nil {
		tokenUnit = "provider_reported_tokens"
	}
	var actualPlanID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO service_plans(
			id,organisation_id,code,name,capacity_mode,
			shared_request_allowance,shared_request_allowance_unit,shared_request_allowance_period,
			shared_token_allowance,shared_token_allowance_unit,shared_token_allowance_period,
			dedicated_resource_class,dedicated_accelerator_count,source_label,finality
		) VALUES($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''),$9,$10,NULLIF($11,''),NULLIF($12,''),$13,$14,$15)
		ON CONFLICT(organisation_id,code) DO UPDATE SET
			name=EXCLUDED.name,
			shared_request_allowance=EXCLUDED.shared_request_allowance,
			shared_request_allowance_unit=EXCLUDED.shared_request_allowance_unit,
			shared_request_allowance_period=EXCLUDED.shared_request_allowance_period,
			shared_token_allowance=EXCLUDED.shared_token_allowance,
			shared_token_allowance_unit=EXCLUDED.shared_token_allowance_unit,
			shared_token_allowance_period=EXCLUDED.shared_token_allowance_period,
			dedicated_resource_class=EXCLUDED.dedicated_resource_class,
			dedicated_accelerator_count=EXCLUDED.dedicated_accelerator_count,
			source_label=EXCLUDED.source_label,finality=EXCLUDED.finality,updated_at=now()
		WHERE service_plans.capacity_mode=EXCLUDED.capacity_mode
		RETURNING id`, planID, orgID, spec.ServicePlanCode, spec.ServicePlanName, spec.CapacityMode,
		spec.SharedRequestAllowance, requestUnit, spec.SharedRequestAllowancePeriod,
		spec.SharedTokenAllowance, tokenUnit, spec.SharedTokenAllowancePeriod,
		spec.DedicatedResourceClass, spec.DedicatedAcceleratorCount, spec.ServicePlanSource, spec.ServicePlanFinality).Scan(&actualPlanID)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("service plan capacity mode conflicts with the route target: %w", platform.ErrConflict)
	}
	if err != nil {
		return mapWriteError("upsert service plan", err)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO tenant_service_plans(organisation_id,project_id,environment_id,route_id,service_plan_id,source_label,finality)
		VALUES($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT(organisation_id,project_id,environment_id,route_id,service_plan_id)
		DO UPDATE SET status='active',source_label=EXCLUDED.source_label,finality=EXCLUDED.finality,
		              effective_at=CASE WHEN tenant_service_plans.status='active' THEN tenant_service_plans.effective_at ELSE now() END,
		              updated_at=now()`, orgID, projectID, environmentID, routeID, actualPlanID, spec.ServicePlanSource, spec.ServicePlanFinality)
	return err
}
func lockExistingRoute(ctx context.Context, tx *sql.Tx, orgID, projectID, envID, modelID string) (string, error) {
	var routeID string
	err := tx.QueryRowContext(ctx, `SELECT id FROM tenant_routes
		WHERE organisation_id=$1 AND project_id=$2 AND environment_id=$3 AND model_id=$4
		FOR UPDATE`, orgID, projectID, envID, modelID).Scan(&routeID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return routeID, err
}
func upsertRoute(ctx context.Context, tx *sql.Tx, orgID, projectID, envID, modelID, targetID string) (string, error) {
	id, err := ids.New("rte")
	if err != nil {
		return "", err
	}
	var result string
	err = tx.QueryRowContext(ctx, `INSERT INTO tenant_routes (id,organisation_id,project_id,environment_id,model_id,target_id) VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (organisation_id,project_id,environment_id,model_id) DO UPDATE SET target_id=EXCLUDED.target_id,enabled=true,updated_at=now() RETURNING id`, id, orgID, projectID, envID, modelID, targetID).Scan(&result)
	return result, err
}
func upsertServiceAccount(ctx context.Context, tx *sql.Tx, orgID, projectID, envID, name string) (string, error) {
	id, err := ids.New("sa")
	if err != nil {
		return "", err
	}
	var result string
	err = tx.QueryRowContext(ctx, `INSERT INTO service_accounts (id,organisation_id,project_id,environment_id,name) VALUES ($1,$2,$3,$4,$5) ON CONFLICT (organisation_id,project_id,environment_id,name) DO UPDATE SET updated_at=now() RETURNING id`, id, orgID, projectID, envID, name).Scan(&result)
	return result, err
}
func insertAudit(ctx context.Context, tx *sql.Tx, orgID, projectID, action string, metadata map[string]string) error {
	id, err := ids.New("aud")
	if err != nil {
		return err
	}
	correlationID, err := ids.New("op")
	if err != nil {
		return err
	}
	safeMetadata, _ := json.Marshal(metadata)
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_events (id,actor_type,actor_id,organisation_id,project_id,action,result,correlation_id,safe_metadata) VALUES ($1,'operator','cli',$2,$3,$4,'succeeded',$5,$6)`, id, orgID, projectID, action, correlationID, safeMetadata)
	return err
}

var _ platform.Provisioner = (*Store)(nil)
