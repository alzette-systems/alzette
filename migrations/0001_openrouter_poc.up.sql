CREATE TABLE organisations (
    id              TEXT PRIMARY KEY,
    slug            TEXT NOT NULL UNIQUE,
    name            TEXT NOT NULL,
    timezone        TEXT NOT NULL DEFAULT 'UTC',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (slug ~ '^[a-z0-9][a-z0-9-]{0,62}$')
);

CREATE TABLE projects (
    id               TEXT PRIMARY KEY,
    organisation_id  TEXT NOT NULL REFERENCES organisations(id) ON DELETE RESTRICT,
    slug             TEXT NOT NULL,
    name             TEXT NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organisation_id, slug),
    UNIQUE (organisation_id, id),
    CHECK (slug ~ '^[a-z0-9][a-z0-9-]{0,62}$')
);

CREATE TABLE environments (
    id               TEXT PRIMARY KEY,
    organisation_id  TEXT NOT NULL,
    project_id       TEXT NOT NULL,
    slug             TEXT NOT NULL,
    name             TEXT NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (organisation_id, project_id)
        REFERENCES projects(organisation_id, id) ON DELETE RESTRICT,
    UNIQUE (organisation_id, project_id, slug),
    UNIQUE (organisation_id, project_id, id),
    CHECK (slug ~ '^[a-z0-9][a-z0-9-]{0,62}$')
);

CREATE TABLE models (
    id               TEXT PRIMARY KEY,
    alias            TEXT NOT NULL UNIQUE,
    version          TEXT NOT NULL,
    enabled          BOOLEAN NOT NULL DEFAULT true,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (alias ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$')
);

CREATE TABLE inference_targets (
    id                     TEXT PRIMARY KEY,
    name                   TEXT NOT NULL UNIQUE,
    execution_class        TEXT NOT NULL,
    capacity_mode          TEXT NOT NULL,
    capacity_evidence_ref  TEXT,
    owner_organisation_id  TEXT REFERENCES organisations(id) ON DELETE RESTRICT,
    base_url               TEXT NOT NULL,
    provider_model         TEXT NOT NULL,
    secret_ref             TEXT NOT NULL,
    timeout_ms             INTEGER NOT NULL DEFAULT 30000,
    max_attempts           INTEGER NOT NULL DEFAULT 2,
    enabled                BOOLEAN NOT NULL DEFAULT true,
    health_status          TEXT NOT NULL DEFAULT 'unknown',
    last_health_check_at   TIMESTAMPTZ,
    last_success_at        TIMESTAMPTZ,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (execution_class IN ('external_pilot', 'private_compatible', 'meluxina')),
    CHECK (capacity_mode IN ('shared', 'dedicated')),
    CHECK ((capacity_mode = 'shared' AND owner_organisation_id IS NULL) OR
           (capacity_mode = 'dedicated' AND owner_organisation_id IS NOT NULL)),
    CHECK ((capacity_mode = 'shared' AND capacity_evidence_ref IS NULL) OR
           (capacity_mode = 'dedicated' AND capacity_evidence_ref IS NOT NULL)),
    CHECK (secret_ref ~ '^[A-Z][A-Z0-9_]{0,127}$'),
    CHECK (timeout_ms BETWEEN 100 AND 60000),
    CHECK (max_attempts BETWEEN 1 AND 4),
    CHECK (health_status IN ('unknown', 'operational', 'degraded', 'unavailable'))
);

CREATE TABLE service_accounts (
    id                  TEXT PRIMARY KEY,
    organisation_id     TEXT NOT NULL,
    project_id          TEXT NOT NULL,
    environment_id      TEXT NOT NULL,
    name                TEXT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (organisation_id, project_id, environment_id)
        REFERENCES environments(organisation_id, project_id, id) ON DELETE RESTRICT,
    UNIQUE (organisation_id, project_id, environment_id, name),
    UNIQUE (organisation_id, project_id, environment_id, id)
);

CREATE TABLE api_keys (
    id                  TEXT PRIMARY KEY,
    service_account_id  TEXT NOT NULL REFERENCES service_accounts(id) ON DELETE RESTRICT,
    key_prefix          TEXT NOT NULL UNIQUE,
    key_hash            BYTEA NOT NULL UNIQUE,
    scopes              JSONB NOT NULL,
    expires_at          TIMESTAMPTZ,
    revoked_at          TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at        TIMESTAMPTZ,
    CHECK (octet_length(key_hash) = 32),
    CHECK (jsonb_typeof(scopes) = 'array' AND jsonb_array_length(scopes) > 0)
);

CREATE TABLE tenant_routes (
    id               TEXT PRIMARY KEY,
    organisation_id  TEXT NOT NULL,
    project_id       TEXT NOT NULL,
    environment_id   TEXT NOT NULL,
    model_id         TEXT NOT NULL REFERENCES models(id) ON DELETE RESTRICT,
    target_id        TEXT NOT NULL REFERENCES inference_targets(id) ON DELETE RESTRICT,
    enabled          BOOLEAN NOT NULL DEFAULT true,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (organisation_id, project_id, environment_id)
        REFERENCES environments(organisation_id, project_id, id) ON DELETE RESTRICT,
    UNIQUE (organisation_id, project_id, environment_id, model_id),
    UNIQUE (organisation_id, project_id, environment_id, id)
);

CREATE OR REPLACE FUNCTION enforce_target_tenancy() RETURNS trigger AS $$
DECLARE
    target_mode TEXT;
    target_owner TEXT;
BEGIN
    SELECT capacity_mode, owner_organisation_id
      INTO target_mode, target_owner
      FROM inference_targets
     WHERE id = NEW.target_id;

    IF target_mode = 'dedicated' AND target_owner <> NEW.organisation_id THEN
        RAISE EXCEPTION 'dedicated target is owned by another organisation'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tenant_routes_enforce_target_tenancy
BEFORE INSERT OR UPDATE OF organisation_id, target_id ON tenant_routes
FOR EACH ROW EXECUTE FUNCTION enforce_target_tenancy();

CREATE OR REPLACE FUNCTION protect_target_tenancy_update() RETURNS trigger AS $$
BEGIN
    IF NEW.capacity_mode = 'dedicated' AND EXISTS (
        SELECT 1 FROM tenant_routes
         WHERE target_id = NEW.id
           AND organisation_id <> NEW.owner_organisation_id
    ) THEN
        RAISE EXCEPTION 'dedicated target has a route owned by another organisation'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER inference_targets_protect_tenancy_update
BEFORE UPDATE OF capacity_mode, owner_organisation_id ON inference_targets
FOR EACH ROW EXECUTE FUNCTION protect_target_tenancy_update();

CREATE TABLE inference_requests (
    id                   TEXT PRIMARY KEY,
    organisation_id      TEXT NOT NULL,
    project_id           TEXT NOT NULL,
    environment_id       TEXT NOT NULL,
    route_id             TEXT,
    service_account_id   TEXT NOT NULL,
    api_key_id           TEXT NOT NULL REFERENCES api_keys(id) ON DELETE RESTRICT,
    key_prefix           TEXT NOT NULL,
    model_alias          TEXT NOT NULL,
    executed_model       TEXT,
    provider_request_id  TEXT,
    started_at           TIMESTAMPTZ NOT NULL,
    completed_at         TIMESTAMPTZ,
    status               TEXT NOT NULL DEFAULT 'in_progress',
    http_status          INTEGER,
    error_class          TEXT,
    duration_ms          BIGINT,
    input_tokens         BIGINT,
    output_tokens        BIGINT,
    cached_tokens        BIGINT,
    reasoning_tokens     BIGINT,
    usage_finality       TEXT NOT NULL DEFAULT 'unknown',
    attempt_count        INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (organisation_id, project_id, environment_id, service_account_id)
        REFERENCES service_accounts(organisation_id, project_id, environment_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (organisation_id, project_id, environment_id, route_id)
        REFERENCES tenant_routes(organisation_id, project_id, environment_id, id) ON DELETE RESTRICT,
    CHECK (status IN ('in_progress', 'succeeded', 'failed', 'blocked', 'cancelled')),
    CHECK (http_status IS NULL OR http_status BETWEEN 100 AND 599),
    CHECK (duration_ms IS NULL OR duration_ms >= 0),
    CHECK (input_tokens IS NULL OR input_tokens >= 0),
    CHECK (output_tokens IS NULL OR output_tokens >= 0),
    CHECK (cached_tokens IS NULL OR cached_tokens >= 0),
    CHECK (reasoning_tokens IS NULL OR reasoning_tokens >= 0),
    CHECK (usage_finality IN ('unknown', 'partial', 'final')),
    CHECK (attempt_count >= 0)
);

CREATE OR REPLACE FUNCTION enforce_request_route_alias() RETURNS trigger AS $$
BEGIN
    IF NEW.route_id IS NOT NULL AND NOT EXISTS (
        SELECT 1
          FROM tenant_routes r
          JOIN models m ON m.id = r.model_id
         WHERE r.id = NEW.route_id
           AND r.organisation_id = NEW.organisation_id
           AND r.project_id = NEW.project_id
           AND r.environment_id = NEW.environment_id
           AND m.alias = NEW.model_alias
    ) THEN
        RAISE EXCEPTION 'inference request route does not match its authenticated scope and alias'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER inference_requests_enforce_route_alias
BEFORE INSERT OR UPDATE OF route_id, model_alias ON inference_requests
FOR EACH ROW EXECUTE FUNCTION enforce_request_route_alias();

CREATE OR REPLACE FUNCTION protect_completed_inference_request() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' OR OLD.status <> 'in_progress' THEN
        RAISE EXCEPTION 'completed inference requests are immutable'
            USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER inference_requests_protect_completed
BEFORE UPDATE OR DELETE ON inference_requests
FOR EACH ROW EXECUTE FUNCTION protect_completed_inference_request();

CREATE TABLE provider_attempts (
    id                    TEXT PRIMARY KEY,
    inference_request_id  TEXT NOT NULL REFERENCES inference_requests(id) ON DELETE RESTRICT,
    target_id             TEXT NOT NULL REFERENCES inference_targets(id) ON DELETE RESTRICT,
    attempt_number        INTEGER NOT NULL,
    started_at            TIMESTAMPTZ NOT NULL,
    completed_at          TIMESTAMPTZ,
    status                TEXT NOT NULL DEFAULT 'in_progress',
    provider_http_status  INTEGER,
    error_class           TEXT,
    duration_ms           BIGINT,
    provider_request_id   TEXT,
    UNIQUE (inference_request_id, attempt_number),
    CHECK (attempt_number BETWEEN 1 AND 4),
    CHECK (status IN ('in_progress', 'succeeded', 'failed', 'cancelled')),
    CHECK (provider_http_status IS NULL OR provider_http_status BETWEEN 100 AND 599),
    CHECK (duration_ms IS NULL OR duration_ms >= 0)
);

CREATE OR REPLACE FUNCTION protect_completed_provider_attempt() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' OR OLD.status <> 'in_progress' THEN
        RAISE EXCEPTION 'completed provider attempts are immutable'
            USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER provider_attempts_protect_completed
BEFORE UPDATE OR DELETE ON provider_attempts
FOR EACH ROW EXECUTE FUNCTION protect_completed_provider_attempt();

CREATE TABLE usage_rollups_hourly (
    organisation_id   TEXT NOT NULL REFERENCES organisations(id) ON DELETE RESTRICT,
    project_id        TEXT NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    environment_id    TEXT NOT NULL REFERENCES environments(id) ON DELETE RESTRICT,
    route_id          TEXT NOT NULL REFERENCES tenant_routes(id) ON DELETE RESTRICT,
    bucket_start      TIMESTAMPTZ NOT NULL,
    logical_requests  BIGINT NOT NULL,
    successful_requests BIGINT NOT NULL,
    failed_requests   BIGINT NOT NULL,
    blocked_requests  BIGINT NOT NULL,
    input_tokens      BIGINT,
    output_tokens     BIGINT,
    cached_tokens     BIGINT,
    reasoning_tokens  BIGINT,
    finality          TEXT NOT NULL,
    refreshed_at      TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (organisation_id, project_id, environment_id, route_id, bucket_start),
    CHECK (date_trunc('hour', bucket_start) = bucket_start),
    CHECK (logical_requests >= 0 AND successful_requests >= 0 AND failed_requests >= 0 AND blocked_requests >= 0),
    CHECK (finality IN ('partial', 'final'))
);

CREATE TABLE audit_events (
    id                TEXT PRIMARY KEY,
    occurred_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    actor_type        TEXT NOT NULL,
    actor_id          TEXT NOT NULL,
    organisation_id   TEXT REFERENCES organisations(id) ON DELETE RESTRICT,
    project_id        TEXT REFERENCES projects(id) ON DELETE RESTRICT,
    action            TEXT NOT NULL,
    result            TEXT NOT NULL,
    correlation_id    TEXT NOT NULL,
    safe_metadata     JSONB NOT NULL DEFAULT '{}'::jsonb,
    CHECK (actor_type IN ('operator', 'service_account', 'system')),
    CHECK (result IN ('succeeded', 'failed')),
    CHECK (jsonb_typeof(safe_metadata) = 'object')
);

CREATE OR REPLACE FUNCTION prevent_audit_event_mutation() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit events are append-only'
        USING ERRCODE = '55000';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_events_append_only
BEFORE UPDATE OR DELETE ON audit_events
FOR EACH ROW EXECUTE FUNCTION prevent_audit_event_mutation();

CREATE INDEX api_keys_active_hash_idx
    ON api_keys (key_hash) WHERE revoked_at IS NULL;
CREATE INDEX inference_requests_scope_time_idx
    ON inference_requests (organisation_id, project_id, environment_id, started_at DESC);
CREATE INDEX inference_requests_route_time_idx
    ON inference_requests (route_id, started_at DESC);
CREATE INDEX provider_attempts_request_idx
    ON provider_attempts (inference_request_id, attempt_number);
CREATE INDEX audit_events_scope_time_idx
    ON audit_events (organisation_id, occurred_at DESC);

COMMENT ON TABLE inference_requests IS
    'Metadata-only customer ledger. Prompt and response content must never be added here.';
COMMENT ON TABLE provider_attempts IS
    'Operator-only outbound attempt ledger; never use attempt count as customer request usage.';
COMMENT ON COLUMN inference_targets.base_url IS
    'Operator-only target data. Never return through a customer API.';
COMMENT ON COLUMN inference_targets.secret_ref IS
    'Environment or secret-manager reference only; never a provider credential.';
