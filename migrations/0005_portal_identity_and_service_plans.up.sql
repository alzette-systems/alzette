ALTER TABLE api_keys
    ADD COLUMN name TEXT NOT NULL DEFAULT 'default',
    ADD COLUMN rotated_from_key_id TEXT,
    ADD CONSTRAINT api_keys_service_account_id_id_0005_unique UNIQUE (service_account_id, id),
    ADD CONSTRAINT api_keys_rotated_from_account_0005_fkey
        FOREIGN KEY (service_account_id, rotated_from_key_id)
        REFERENCES api_keys(service_account_id, id) ON DELETE RESTRICT,
    ADD CONSTRAINT api_keys_name_0005_check
        CHECK (name ~ '^[A-Za-z0-9][A-Za-z0-9 ._:-]{0,126}[A-Za-z0-9]$' OR name ~ '^[A-Za-z0-9]$'),
    ADD CONSTRAINT api_keys_expiry_lifecycle_0005_check
        CHECK (expires_at IS NULL OR expires_at > created_at),
    ADD CONSTRAINT api_keys_revocation_lifecycle_0005_check
        CHECK (revoked_at IS NULL OR revoked_at >= created_at);

ALTER TABLE service_accounts
    ADD CONSTRAINT service_accounts_name_0005_check
    CHECK (name ~ '^[A-Za-z0-9][A-Za-z0-9 ._:-]{0,126}[A-Za-z0-9]$' OR name ~ '^[A-Za-z0-9]$')
    NOT VALID;

CREATE TABLE service_plans (
    id                            TEXT PRIMARY KEY,
    organisation_id               TEXT NOT NULL REFERENCES organisations(id) ON DELETE RESTRICT,
    code                          TEXT NOT NULL,
    name                          TEXT NOT NULL,
    capacity_mode                 TEXT NOT NULL,
    shared_request_allowance      BIGINT,
    shared_request_allowance_unit TEXT,
    shared_request_allowance_period TEXT,
    shared_token_allowance        BIGINT,
    shared_token_allowance_unit   TEXT,
    shared_token_allowance_period TEXT,
    dedicated_resource_class      TEXT,
    dedicated_accelerator_count   INTEGER,
    source_label                  TEXT NOT NULL,
    finality                      TEXT NOT NULL,
    created_at                    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organisation_id, code),
    UNIQUE (organisation_id, id),
    CHECK (code ~ '^[a-z0-9][a-z0-9-]{0,62}$'),
    CHECK (length(name) BETWEEN 1 AND 255),
    CHECK (capacity_mode IN ('shared', 'dedicated')),
    CHECK (shared_request_allowance IS NULL OR shared_request_allowance >= 0),
    CHECK (shared_token_allowance IS NULL OR shared_token_allowance >= 0),
    CHECK ((shared_request_allowance IS NULL
            AND shared_request_allowance_unit IS NULL
            AND shared_request_allowance_period IS NULL)
           OR (shared_request_allowance IS NOT NULL
               AND shared_request_allowance_unit = 'logical_requests'
               AND shared_request_allowance_period IN ('hour', 'day', 'month', 'contract_term'))),
    CHECK ((shared_token_allowance IS NULL
            AND shared_token_allowance_unit IS NULL
            AND shared_token_allowance_period IS NULL)
           OR (shared_token_allowance IS NOT NULL
               AND shared_token_allowance_unit = 'provider_reported_tokens'
               AND shared_token_allowance_period IN ('hour', 'day', 'month', 'contract_term'))),
    CHECK (dedicated_accelerator_count IS NULL OR dedicated_accelerator_count > 0),
    CHECK (source_label ~ '^[A-Za-z0-9][A-Za-z0-9 ._:/-]{0,254}$'),
    CHECK (finality IN ('declared', 'unknown')),
    CHECK (
        (capacity_mode = 'shared'
         AND dedicated_resource_class IS NULL
         AND dedicated_accelerator_count IS NULL)
        OR
        (capacity_mode = 'dedicated'
         AND shared_request_allowance IS NULL
         AND shared_token_allowance IS NULL)
    )
);

CREATE TABLE tenant_service_plans (
    organisation_id  TEXT NOT NULL,
    project_id       TEXT NOT NULL,
    environment_id   TEXT NOT NULL,
    route_id         TEXT NOT NULL,
    service_plan_id  TEXT NOT NULL,
    status           TEXT NOT NULL DEFAULT 'active',
    source_label     TEXT NOT NULL,
    finality         TEXT NOT NULL,
    effective_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (organisation_id, project_id, environment_id, route_id, service_plan_id),
    FOREIGN KEY (organisation_id, project_id, environment_id, route_id)
        REFERENCES tenant_routes(organisation_id, project_id, environment_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (organisation_id, service_plan_id)
        REFERENCES service_plans(organisation_id, id) ON DELETE RESTRICT,
    CHECK (status IN ('active', 'inactive')),
    CHECK (source_label ~ '^[A-Za-z0-9][A-Za-z0-9 ._:/-]{0,254}$'),
    CHECK (finality IN ('declared', 'unknown'))
);

CREATE UNIQUE INDEX tenant_service_plans_one_active_route_0005_idx
    ON tenant_service_plans(organisation_id, project_id, environment_id, route_id)
    WHERE status = 'active';

CREATE TABLE human_users (
    id                   TEXT PRIMARY KEY,
    username             TEXT NOT NULL,
    display_name         TEXT NOT NULL,
    password_hash        TEXT NOT NULL,
    enabled              BOOLEAN NOT NULL DEFAULT true,
    password_changed_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_sign_in_at      TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (username),
    CHECK (username ~ '^[a-z0-9][a-z0-9._-]{2,62}$'),
    CHECK (length(display_name) BETWEEN 1 AND 255),
    CHECK (password_hash ~ '^[$]2[aby][$][0-9]{2}[$]')
);

CREATE TABLE human_memberships (
    id                TEXT PRIMARY KEY,
    user_id           TEXT NOT NULL REFERENCES human_users(id) ON DELETE RESTRICT,
    organisation_id   TEXT NOT NULL,
    project_id        TEXT NOT NULL,
    environment_id    TEXT NOT NULL,
    role              TEXT NOT NULL,
    enabled           BOOLEAN NOT NULL DEFAULT true,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (organisation_id, project_id, environment_id)
        REFERENCES environments(organisation_id, project_id, id) ON DELETE RESTRICT,
    UNIQUE (user_id, organisation_id, project_id, environment_id),
    UNIQUE (user_id, id),
    CHECK (role IN ('org_admin', 'project_admin', 'developer', 'viewer'))
);

CREATE TABLE portal_sessions (
    id                     TEXT PRIMARY KEY,
    user_id                TEXT NOT NULL REFERENCES human_users(id) ON DELETE RESTRICT,
    current_membership_id  TEXT NOT NULL,
    token_hash             BYTEA NOT NULL UNIQUE,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at             TIMESTAMPTZ NOT NULL,
    revoked_at             TIMESTAMPTZ,
    FOREIGN KEY (user_id, current_membership_id)
        REFERENCES human_memberships(user_id, id) ON DELETE RESTRICT,
    CHECK (octet_length(token_hash) = 32),
    CHECK (expires_at > created_at),
    CHECK (revoked_at IS NULL OR revoked_at >= created_at)
);

CREATE INDEX portal_sessions_active_hash_0005_idx
    ON portal_sessions(token_hash)
    WHERE revoked_at IS NULL;
CREATE INDEX human_memberships_user_0005_idx
    ON human_memberships(user_id, enabled);

CREATE OR REPLACE FUNCTION revoke_human_sessions_0005() RETURNS trigger AS $$
BEGIN
    IF (OLD.enabled AND NOT NEW.enabled)
       OR NEW.password_hash IS DISTINCT FROM OLD.password_hash THEN
        UPDATE portal_sessions
           SET revoked_at = COALESCE(revoked_at, GREATEST(now(), created_at))
         WHERE user_id = OLD.id AND revoked_at IS NULL;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER human_users_revoke_sessions_0005
AFTER UPDATE OF enabled, password_hash ON human_users
FOR EACH ROW EXECUTE FUNCTION revoke_human_sessions_0005();

CREATE OR REPLACE FUNCTION revoke_membership_sessions_0005() RETURNS trigger AS $$
BEGIN
    IF OLD.enabled AND NOT NEW.enabled THEN
        UPDATE portal_sessions
           SET revoked_at = COALESCE(revoked_at, GREATEST(now(), created_at))
         WHERE current_membership_id = OLD.id AND revoked_at IS NULL;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER human_memberships_revoke_sessions_0005
AFTER UPDATE OF enabled ON human_memberships
FOR EACH ROW EXECUTE FUNCTION revoke_membership_sessions_0005();

CREATE OR REPLACE FUNCTION enforce_plan_capacity_0005() RETURNS trigger AS $$
DECLARE
    plan_mode TEXT;
BEGIN
    IF NEW.status <> 'active' THEN
        RETURN NEW;
    END IF;
    SELECT capacity_mode INTO plan_mode
      FROM service_plans
     WHERE organisation_id = NEW.organisation_id AND id = NEW.service_plan_id;
    IF EXISTS (
        SELECT 1
          FROM tenant_routes r
          JOIN inference_targets t ON t.id = r.target_id
         WHERE r.organisation_id = NEW.organisation_id
           AND r.project_id = NEW.project_id
           AND r.environment_id = NEW.environment_id
           AND r.id = NEW.route_id
           AND t.capacity_mode <> plan_mode
    ) THEN
        RAISE EXCEPTION 'service plan capacity mode does not match its bound route target'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tenant_service_plans_enforce_capacity_0005
BEFORE INSERT OR UPDATE OF service_plan_id, status, organisation_id, project_id, environment_id, route_id
ON tenant_service_plans
FOR EACH ROW EXECUTE FUNCTION enforce_plan_capacity_0005();

CREATE OR REPLACE FUNCTION enforce_route_plan_capacity_0005() RETURNS trigger AS $$
DECLARE
    target_mode TEXT;
    plan_mode TEXT;
BEGIN
    SELECT capacity_mode INTO target_mode FROM inference_targets WHERE id = NEW.target_id;
    SELECT sp.capacity_mode INTO plan_mode
      FROM tenant_service_plans tsp
      JOIN service_plans sp ON sp.id = tsp.service_plan_id
     WHERE tsp.organisation_id = NEW.organisation_id
       AND tsp.project_id = NEW.project_id
       AND tsp.environment_id = NEW.environment_id
       AND tsp.route_id = NEW.id
       AND tsp.status = 'active';
    IF FOUND AND target_mode <> plan_mode THEN
        RAISE EXCEPTION 'route target capacity mode does not match the active service plan'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tenant_routes_enforce_plan_capacity_0005
BEFORE INSERT OR UPDATE OF target_id, organisation_id, project_id, environment_id
ON tenant_routes
FOR EACH ROW EXECUTE FUNCTION enforce_route_plan_capacity_0005();

CREATE OR REPLACE FUNCTION enforce_target_plan_capacity_0005() RETURNS trigger AS $$
BEGIN
    IF NEW.capacity_mode IS DISTINCT FROM OLD.capacity_mode AND EXISTS (
        SELECT 1
          FROM tenant_routes r
          JOIN tenant_service_plans tsp
            ON tsp.organisation_id = r.organisation_id
           AND tsp.project_id = r.project_id
           AND tsp.environment_id = r.environment_id
           AND tsp.route_id = r.id
           AND tsp.status = 'active'
          JOIN service_plans sp ON sp.id = tsp.service_plan_id
         WHERE r.target_id = NEW.id
           AND r.enabled
           AND sp.capacity_mode <> NEW.capacity_mode
    ) THEN
        RAISE EXCEPTION 'target capacity mode does not match an active bound service plan'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER inference_targets_enforce_plan_capacity_0005
BEFORE UPDATE OF capacity_mode ON inference_targets
FOR EACH ROW EXECUTE FUNCTION enforce_target_plan_capacity_0005();

ALTER TABLE audit_events DROP CONSTRAINT audit_events_actor_type_check;
ALTER TABLE audit_events
    ADD CONSTRAINT audit_events_actor_type_check
    CHECK (actor_type IN ('operator', 'service_account', 'system', 'human_user'));

COMMENT ON TABLE human_users IS
    'Portal identities only. Passwords are bcrypt hashes; users are never data-plane service accounts.';
COMMENT ON TABLE portal_sessions IS
    'Portal session tokens are stored only as SHA-256 digests.';
COMMENT ON TABLE service_plans IS
    'Operator-entered plan evidence. Nullable allowances and allocations must never be invented.';
