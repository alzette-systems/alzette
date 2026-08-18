-- Short-lived human inference authority. OAuth tokens remain outside Alzette;
-- only random Alzette token digests and safe authorization lineage persist.

CREATE TABLE human_agent_grants (
    id                       TEXT PRIMARY KEY,
    user_id                  TEXT NOT NULL REFERENCES human_users(id) ON DELETE RESTRICT,
    federated_identity_id    TEXT NOT NULL REFERENCES human_federated_identities(id) ON DELETE RESTRICT,
    person_id                TEXT NOT NULL,
    membership_id            TEXT NOT NULL,
    organisation_id          TEXT NOT NULL,
    project_id               TEXT NOT NULL,
    environment_id           TEXT NOT NULL,
    oauth_client_id          TEXT NOT NULL,
    client_instance_digest   BYTEA NOT NULL,
    permitted_model_aliases  JSONB NOT NULL,
    created_at               TIMESTAMPTZ NOT NULL,
    authenticated_at         TIMESTAMPTZ NOT NULL,
    absolute_expires_at      TIMESTAMPTZ NOT NULL,
    last_used_at             TIMESTAMPTZ,
    revoked_at               TIMESTAMPTZ,
    revocation_reason        TEXT,
    FOREIGN KEY (organisation_id, person_id)
        REFERENCES organisation_people(organisation_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (user_id, membership_id)
        REFERENCES human_memberships(user_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (organisation_id, project_id, environment_id)
        REFERENCES environments(organisation_id, project_id, id) ON DELETE RESTRICT,
    UNIQUE (federated_identity_id, oauth_client_id, client_instance_digest, membership_id),
    CHECK (octet_length(client_instance_digest) = 32),
    CHECK (jsonb_typeof(permitted_model_aliases) = 'array' AND jsonb_array_length(permitted_model_aliases) > 0),
    CHECK (absolute_expires_at > created_at),
    CHECK (revoked_at IS NULL OR revoked_at >= created_at),
    CHECK ((revoked_at IS NULL AND revocation_reason IS NULL) OR
           (revoked_at IS NOT NULL AND length(revocation_reason) BETWEEN 1 AND 255))
);

CREATE TABLE human_agent_access_tokens (
    id             TEXT PRIMARY KEY,
    grant_id       TEXT NOT NULL REFERENCES human_agent_grants(id) ON DELETE RESTRICT,
    token_prefix   TEXT NOT NULL UNIQUE,
    token_hash     BYTEA NOT NULL UNIQUE,
    generation     BIGINT NOT NULL,
    issued_at      TIMESTAMPTZ NOT NULL,
    expires_at     TIMESTAMPTZ NOT NULL,
    last_used_at   TIMESTAMPTZ,
    revoked_at     TIMESTAMPTZ,
    replaced_by_id TEXT REFERENCES human_agent_access_tokens(id) ON DELETE RESTRICT,
    UNIQUE (grant_id, generation),
    CHECK (token_prefix ~ '^alz_u_[0-9a-f]{16}$'),
    CHECK (octet_length(token_hash) = 32),
    CHECK (generation > 0),
    CHECK (expires_at > issued_at),
    CHECK (expires_at <= issued_at + interval '10 minutes'),
    CHECK (revoked_at IS NULL OR revoked_at >= issued_at)
);

CREATE UNIQUE INDEX human_agent_access_tokens_one_active_0014_idx
    ON human_agent_access_tokens(grant_id)
    WHERE revoked_at IS NULL;

CREATE TABLE human_agent_credential_mints (
    id                     TEXT PRIMARY KEY,
    federated_identity_id  TEXT NOT NULL REFERENCES human_federated_identities(id) ON DELETE RESTRICT,
    oauth_client_id        TEXT NOT NULL,
    idempotency_key_digest BYTEA NOT NULL,
    canonical_request_hash BYTEA NOT NULL,
    grant_id               TEXT NOT NULL REFERENCES human_agent_grants(id) ON DELETE RESTRICT,
    token_id               TEXT NOT NULL REFERENCES human_agent_access_tokens(id) ON DELETE RESTRICT,
    state                  TEXT NOT NULL,
    created_at             TIMESTAMPTZ NOT NULL,
    completed_at           TIMESTAMPTZ,
    replayed_at            TIMESTAMPTZ,
    UNIQUE (federated_identity_id, oauth_client_id, idempotency_key_digest),
    CHECK (octet_length(idempotency_key_digest) = 32),
    CHECK (octet_length(canonical_request_hash) = 32),
    CHECK (state IN ('in_progress', 'completed', 'response_unrecoverable')),
    CHECK ((state = 'in_progress' AND completed_at IS NULL) OR
           (state <> 'in_progress' AND completed_at IS NOT NULL))
);

ALTER TABLE inference_requests
    ALTER COLUMN service_account_id DROP NOT NULL,
    ALTER COLUMN api_key_id DROP NOT NULL,
    ALTER COLUMN key_prefix DROP NOT NULL,
    ADD COLUMN human_user_id TEXT REFERENCES human_users(id) ON DELETE RESTRICT,
    ADD COLUMN human_membership_id TEXT,
    ADD COLUMN agent_grant_id TEXT REFERENCES human_agent_grants(id) ON DELETE RESTRICT,
    ADD COLUMN agent_token_id TEXT REFERENCES human_agent_access_tokens(id) ON DELETE RESTRICT,
    ADD CONSTRAINT inference_requests_human_membership_0014_fk
        FOREIGN KEY (human_user_id, human_membership_id)
        REFERENCES human_memberships(user_id, id) ON DELETE RESTRICT,
    ADD CONSTRAINT inference_requests_actor_0014_check CHECK (
        (service_account_id IS NOT NULL AND api_key_id IS NOT NULL AND key_prefix IS NOT NULL
         AND human_user_id IS NULL AND human_membership_id IS NULL AND agent_grant_id IS NULL AND agent_token_id IS NULL)
        OR
        (service_account_id IS NULL AND api_key_id IS NULL AND key_prefix IS NULL
         AND human_user_id IS NOT NULL AND human_membership_id IS NOT NULL AND agent_grant_id IS NOT NULL AND agent_token_id IS NOT NULL)
    );

CREATE INDEX human_agent_grants_person_0014_idx
    ON human_agent_grants(organisation_id, person_id, revoked_at);
CREATE INDEX human_agent_tokens_hash_0014_idx
    ON human_agent_access_tokens(token_hash) WHERE revoked_at IS NULL;

CREATE OR REPLACE FUNCTION revoke_human_agent_authority_0014() RETURNS trigger AS $$
BEGIN
    IF OLD.enabled AND NOT NEW.enabled THEN
        UPDATE human_agent_grants
           SET revoked_at = COALESCE(revoked_at, now()),
               revocation_reason = COALESCE(revocation_reason, 'identity_or_person_disabled')
         WHERE (TG_TABLE_NAME = 'human_users' AND user_id = OLD.id)
            OR (TG_TABLE_NAME = 'organisation_people' AND person_id = OLD.id)
            OR (TG_TABLE_NAME = 'human_federated_identities' AND federated_identity_id = OLD.id);
        UPDATE human_agent_access_tokens t
           SET revoked_at = COALESCE(t.revoked_at, now())
          FROM human_agent_grants g
         WHERE t.grant_id = g.id
           AND ((TG_TABLE_NAME = 'human_users' AND g.user_id = OLD.id)
             OR (TG_TABLE_NAME = 'organisation_people' AND g.person_id = OLD.id)
             OR (TG_TABLE_NAME = 'human_federated_identities' AND g.federated_identity_id = OLD.id));
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER human_users_revoke_agent_authority_0014
AFTER UPDATE OF enabled ON human_users
FOR EACH ROW EXECUTE FUNCTION revoke_human_agent_authority_0014();
CREATE TRIGGER organisation_people_revoke_agent_authority_0014
AFTER UPDATE OF enabled ON organisation_people
FOR EACH ROW EXECUTE FUNCTION revoke_human_agent_authority_0014();
CREATE TRIGGER human_federated_identities_revoke_agent_authority_0014
AFTER UPDATE OF enabled ON human_federated_identities
FOR EACH ROW EXECUTE FUNCTION revoke_human_agent_authority_0014();
