-- Bind request attribution to the API key that actually belongs to the
-- authenticated service account. The service account is already bound to the
-- request's organisation/project/environment tuple by 0001.
ALTER TABLE api_keys
    ADD CONSTRAINT api_keys_service_account_id_id_key_prefix_0002_key
    UNIQUE (service_account_id, id, key_prefix);

ALTER TABLE inference_requests
    ADD CONSTRAINT inference_requests_api_key_tuple_0002_fkey
    FOREIGN KEY (service_account_id, api_key_id, key_prefix)
    REFERENCES api_keys (service_account_id, id, key_prefix)
    ON DELETE RESTRICT;

-- A rollup row must describe the same tenant route tuple, not four unrelated
-- individually valid identifiers.
ALTER TABLE usage_rollups_hourly
    ADD CONSTRAINT usage_rollups_hourly_tenant_route_0002_fkey
    FOREIGN KEY (organisation_id, project_id, environment_id, route_id)
    REFERENCES tenant_routes (organisation_id, project_id, environment_id, id)
    ON DELETE RESTRICT;

-- A project-scoped audit event must carry its owning organisation.
ALTER TABLE audit_events
    ADD CONSTRAINT audit_events_project_requires_organisation_0002_check
    CHECK (project_id IS NULL OR organisation_id IS NOT NULL),
    ADD CONSTRAINT audit_events_organisation_project_0002_fkey
    FOREIGN KEY (organisation_id, project_id)
    REFERENCES projects (organisation_id, id)
    ON DELETE RESTRICT;

-- Terminal rows always have a completion time, and in-progress rows never do.
ALTER TABLE inference_requests
    ADD CONSTRAINT inference_requests_status_completed_at_0002_check
    CHECK (
        (status = 'in_progress' AND completed_at IS NULL) OR
        (status <> 'in_progress' AND completed_at IS NOT NULL)
    ),
    ADD CONSTRAINT inference_requests_completed_after_started_0002_check
    CHECK (completed_at IS NULL OR completed_at >= started_at);

ALTER TABLE provider_attempts
    ADD CONSTRAINT provider_attempts_status_completed_at_0002_check
    CHECK (
        (status = 'in_progress' AND completed_at IS NULL) OR
        (status <> 'in_progress' AND completed_at IS NOT NULL)
    ),
    ADD CONSTRAINT provider_attempts_completed_after_started_0002_check
    CHECK (completed_at IS NULL OR completed_at >= started_at);

-- Triggers cannot retroactively examine 0001 rows, so validate the lifecycle
-- invariants explicitly before installing them. The migration runner wraps the
-- whole file in a transaction; an unsafe existing schema remains unchanged.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM inference_requests ir
          LEFT JOIN LATERAL (
              SELECT count(*) AS actual_count,
                     min(pa.attempt_number) AS minimum_number,
                     max(pa.attempt_number) AS maximum_number,
                     count(*) FILTER (WHERE pa.status = 'in_progress') AS in_progress_count
                FROM provider_attempts pa
               WHERE pa.inference_request_id = ir.id
          ) attempts ON true
         WHERE ir.attempt_count <> attempts.actual_count
            OR (attempts.actual_count > 0 AND
                (attempts.minimum_number <> 1 OR attempts.maximum_number <> attempts.actual_count))
            OR (ir.status <> 'in_progress' AND attempts.in_progress_count > 0)
    ) THEN
        RAISE EXCEPTION 'existing inference request/provider attempt lifecycle is inconsistent'
            USING ERRCODE = '23514';
    END IF;

    IF EXISTS (
        SELECT 1
          FROM provider_attempts pa
          JOIN inference_requests ir ON ir.id = pa.inference_request_id
          LEFT JOIN tenant_routes r
            ON r.id = ir.route_id
           AND r.organisation_id = ir.organisation_id
           AND r.project_id = ir.project_id
           AND r.environment_id = ir.environment_id
          LEFT JOIN inference_targets attempt_target ON attempt_target.id = pa.target_id
         WHERE r.id IS NULL
            OR (ir.status = 'in_progress' AND pa.target_id <> r.target_id)
            OR pa.attempt_number > attempt_target.max_attempts
            OR pa.started_at < ir.started_at
    ) THEN
        RAISE EXCEPTION 'existing provider attempt does not match its routed request lifecycle'
            USING ERRCODE = '23514';
    END IF;
END;
$$;

CREATE OR REPLACE FUNCTION enforce_inference_request_lifecycle_0002() RETURNS trigger AS $$
DECLARE
    actual_attempts BIGINT;
    in_progress_attempts BIGINT;
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.status <> 'in_progress' OR NEW.completed_at IS NOT NULL OR
           NEW.attempt_count <> 0 OR NEW.route_id IS NOT NULL THEN
            RAISE EXCEPTION 'inference requests must be created unrouted and in progress with zero attempts'
                USING ERRCODE = '23514';
        END IF;
        RETURN NEW;
    END IF;

    IF NEW.id IS DISTINCT FROM OLD.id OR
       NEW.organisation_id IS DISTINCT FROM OLD.organisation_id OR
       NEW.project_id IS DISTINCT FROM OLD.project_id OR
       NEW.environment_id IS DISTINCT FROM OLD.environment_id OR
       NEW.service_account_id IS DISTINCT FROM OLD.service_account_id OR
       NEW.api_key_id IS DISTINCT FROM OLD.api_key_id OR
       NEW.key_prefix IS DISTINCT FROM OLD.key_prefix OR
       NEW.model_alias IS DISTINCT FROM OLD.model_alias OR
       NEW.started_at IS DISTINCT FROM OLD.started_at THEN
        RAISE EXCEPTION 'inference request identity and authenticated scope are immutable'
            USING ERRCODE = '55000';
    END IF;

    IF NEW.route_id IS DISTINCT FROM OLD.route_id THEN
        IF OLD.route_id IS NOT NULL OR NEW.route_id IS NULL OR
           OLD.status <> 'in_progress' OR NEW.status <> 'in_progress' OR
           OLD.attempt_count <> 0 OR NEW.attempt_count <> 0 THEN
            RAISE EXCEPTION 'inference request route may only be attached once before attempts begin'
                USING ERRCODE = '55000';
        END IF;
        -- Serialize route attachment with operator retargeting. The existing
        -- 0001 alias trigger performs the tenant/model authorisation check.
        PERFORM 1
          FROM tenant_routes r
          JOIN inference_targets target ON target.id = r.target_id
         WHERE r.id = NEW.route_id
         FOR UPDATE OF r, target;
    END IF;

    IF OLD.status = 'in_progress' AND NEW.status <> 'in_progress' THEN
        SELECT count(*), count(*) FILTER (WHERE status = 'in_progress')
          INTO actual_attempts, in_progress_attempts
          FROM provider_attempts
         WHERE inference_request_id = OLD.id;

        IF NEW.attempt_count <> actual_attempts THEN
            RAISE EXCEPTION 'inference request attempt count does not match provider attempts'
                USING ERRCODE = '23514';
        END IF;
        IF in_progress_attempts <> 0 THEN
            RAISE EXCEPTION 'inference request cannot complete while provider attempts are in progress'
                USING ERRCODE = '23514';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER inference_requests_enforce_lifecycle_0002
BEFORE INSERT OR UPDATE ON inference_requests
FOR EACH ROW EXECUTE FUNCTION enforce_inference_request_lifecycle_0002();

CREATE OR REPLACE FUNCTION protect_active_inference_route_0002() RETURNS trigger AS $$
BEGIN
    IF (NEW.id,
        NEW.organisation_id,
        NEW.project_id,
        NEW.environment_id,
        NEW.model_id,
        NEW.target_id) IS DISTINCT FROM
       (OLD.id,
        OLD.organisation_id,
        OLD.project_id,
        OLD.environment_id,
        OLD.model_id,
        OLD.target_id) AND (
            EXISTS (
                SELECT 1
                  FROM inference_requests ir
                 WHERE ir.route_id = OLD.id
                   AND ir.status = 'in_progress'
            ) OR EXISTS (
                SELECT 1
                  FROM inference_requests ir
                  JOIN models m ON m.id = OLD.model_id
                 WHERE ir.route_id IS NULL
                   AND ir.status = 'in_progress'
                   AND ir.organisation_id = OLD.organisation_id
                   AND ir.project_id = OLD.project_id
                   AND ir.environment_id = OLD.environment_id
                   AND ir.model_alias = m.alias
            )
       ) THEN
        RAISE EXCEPTION 'tenant route cannot change scope, model, or target while a request is in progress'
            USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tenant_routes_protect_active_requests_0002
BEFORE UPDATE ON tenant_routes
FOR EACH ROW EXECUTE FUNCTION protect_active_inference_route_0002();

CREATE OR REPLACE FUNCTION protect_inference_target_execution_0002() RETURNS trigger AS $$
BEGIN
    IF (NEW.execution_class,
        NEW.capacity_mode,
        NEW.capacity_evidence_ref,
        NEW.owner_organisation_id,
        NEW.base_url,
        NEW.provider_model,
        NEW.secret_ref,
        NEW.timeout_ms,
        NEW.max_attempts) IS DISTINCT FROM
       (OLD.execution_class,
        OLD.capacity_mode,
        OLD.capacity_evidence_ref,
        OLD.owner_organisation_id,
        OLD.base_url,
        OLD.provider_model,
        OLD.secret_ref,
        OLD.timeout_ms,
        OLD.max_attempts) AND (
            EXISTS (
                SELECT 1
                  FROM provider_attempts pa
                 WHERE pa.target_id = OLD.id
            ) OR EXISTS (
                SELECT 1
                  FROM tenant_routes r
                  JOIN models m ON m.id = r.model_id
                  JOIN inference_requests ir
                    ON ir.organisation_id = r.organisation_id
                   AND ir.project_id = r.project_id
                   AND ir.environment_id = r.environment_id
                   AND ir.model_alias = m.alias
                 WHERE r.target_id = OLD.id
                   AND ir.status = 'in_progress'
                   AND (ir.route_id = r.id OR ir.route_id IS NULL)
            )
       ) THEN
        RAISE EXCEPTION 'inference target execution configuration is immutable while active or after an attempt'
            USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER inference_targets_protect_execution_0002
BEFORE UPDATE ON inference_targets
FOR EACH ROW EXECUTE FUNCTION protect_inference_target_execution_0002();

CREATE OR REPLACE FUNCTION enforce_provider_attempt_lifecycle_0002() RETURNS trigger AS $$
DECLARE
    parent_status TEXT;
    parent_route_id TEXT;
    parent_attempt_count INTEGER;
    parent_started_at TIMESTAMPTZ;
    route_target_id TEXT;
    route_max_attempts INTEGER;
    existing_attempts BIGINT;
BEGIN
    IF TG_OP = 'UPDATE' THEN
        IF NEW.id IS DISTINCT FROM OLD.id OR
           NEW.inference_request_id IS DISTINCT FROM OLD.inference_request_id OR
           NEW.target_id IS DISTINCT FROM OLD.target_id OR
           NEW.attempt_number IS DISTINCT FROM OLD.attempt_number OR
           NEW.started_at IS DISTINCT FROM OLD.started_at THEN
            RAISE EXCEPTION 'provider attempt identity and routing are immutable'
                USING ERRCODE = '55000';
        END IF;

        IF OLD.status = 'in_progress' AND NEW.status <> 'in_progress' THEN
            SELECT status
              INTO parent_status
              FROM inference_requests
             WHERE id = OLD.inference_request_id
             FOR UPDATE;
            IF parent_status IS DISTINCT FROM 'in_progress' THEN
                RAISE EXCEPTION 'provider attempt cannot complete after its request'
                    USING ERRCODE = '23514';
            END IF;
        END IF;
        RETURN NEW;
    END IF;

    IF NEW.status <> 'in_progress' OR NEW.completed_at IS NOT NULL THEN
        RAISE EXCEPTION 'provider attempts must be created in progress'
            USING ERRCODE = '23514';
    END IF;

    SELECT ir.status, ir.route_id, ir.attempt_count, ir.started_at,
           r.target_id, target.max_attempts
      INTO parent_status, parent_route_id, parent_attempt_count, parent_started_at,
           route_target_id, route_max_attempts
      FROM inference_requests ir
      LEFT JOIN tenant_routes r
        ON r.id = ir.route_id
       AND r.organisation_id = ir.organisation_id
       AND r.project_id = ir.project_id
       AND r.environment_id = ir.environment_id
      LEFT JOIN inference_targets target ON target.id = r.target_id
     WHERE ir.id = NEW.inference_request_id
     FOR UPDATE OF ir;

    IF NOT FOUND OR parent_status <> 'in_progress' THEN
        RAISE EXCEPTION 'provider attempts require an in-progress inference request'
            USING ERRCODE = '23514';
    END IF;
    IF parent_route_id IS NULL OR route_target_id IS NULL THEN
        RAISE EXCEPTION 'provider attempts require a routed inference request'
            USING ERRCODE = '23514';
    END IF;
    IF NEW.target_id IS DISTINCT FROM route_target_id THEN
        RAISE EXCEPTION 'provider attempt target does not match the request route'
            USING ERRCODE = '23514';
    END IF;
    IF NEW.started_at < parent_started_at THEN
        RAISE EXCEPTION 'provider attempt cannot start before its inference request'
            USING ERRCODE = '23514';
    END IF;

    SELECT count(*)
      INTO existing_attempts
      FROM provider_attempts
     WHERE inference_request_id = NEW.inference_request_id;

    IF NEW.attempt_number <> existing_attempts + 1 OR
       parent_attempt_count <> existing_attempts + 1 THEN
        RAISE EXCEPTION 'provider attempt number and request attempt count are inconsistent'
            USING ERRCODE = '23514';
    END IF;
    IF NEW.attempt_number > route_max_attempts THEN
        RAISE EXCEPTION 'provider attempt exceeds the routed target retry limit'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER provider_attempts_enforce_lifecycle_0002
BEFORE INSERT OR UPDATE ON provider_attempts
FOR EACH ROW EXECUTE FUNCTION enforce_provider_attempt_lifecycle_0002();

CREATE OR REPLACE FUNCTION check_inference_request_attempt_count_0002() RETURNS trigger AS $$
DECLARE
    request_id TEXT;
    recorded_count INTEGER;
    actual_count BIGINT;
BEGIN
    request_id := CASE WHEN TG_OP = 'DELETE' THEN OLD.inference_request_id ELSE NEW.inference_request_id END;
    SELECT attempt_count
      INTO recorded_count
      FROM inference_requests
     WHERE id = request_id;
    IF NOT FOUND THEN
        RETURN NULL;
    END IF;
    SELECT count(*) INTO actual_count
      FROM provider_attempts
     WHERE inference_request_id = request_id;
    IF recorded_count <> actual_count THEN
        RAISE EXCEPTION 'inference request attempt count does not match provider attempts'
            USING ERRCODE = '23514';
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION check_provider_attempt_count_from_request_0002() RETURNS trigger AS $$
DECLARE
    actual_count BIGINT;
BEGIN
    SELECT count(*) INTO actual_count
      FROM provider_attempts
     WHERE inference_request_id = NEW.id;
    IF NEW.attempt_count <> actual_count THEN
        RAISE EXCEPTION 'inference request attempt count does not match provider attempts'
            USING ERRCODE = '23514';
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE CONSTRAINT TRIGGER provider_attempts_attempt_count_consistent_0002
AFTER INSERT OR UPDATE OR DELETE ON provider_attempts
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION check_inference_request_attempt_count_0002();

CREATE CONSTRAINT TRIGGER inference_requests_attempt_count_consistent_0002
AFTER INSERT OR UPDATE ON inference_requests
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION check_provider_attempt_count_from_request_0002();
