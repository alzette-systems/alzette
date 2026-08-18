-- Owner-created employee invitations. Plaintext invitation tokens are never stored.

CREATE TABLE human_invitations (
    id                    TEXT PRIMARY KEY,
    organisation_id       TEXT NOT NULL REFERENCES organisations(id) ON DELETE RESTRICT,
    email_normalized      TEXT NOT NULL,
    intended_display_name TEXT NOT NULL DEFAULT '',
    status                TEXT NOT NULL DEFAULT 'pending',
    token_digest          BYTEA NOT NULL UNIQUE,
    token_generation      BIGINT NOT NULL DEFAULT 1,
    expires_at            TIMESTAMPTZ NOT NULL,
    created_by            TEXT NOT NULL REFERENCES human_users(id) ON DELETE RESTRICT,
    accepted_user_id      TEXT REFERENCES human_users(id) ON DELETE RESTRICT,
    accepted_at           TIMESTAMPTZ,
    revoked_at            TIMESTAMPTZ,
    delivery_mode         TEXT NOT NULL DEFAULT 'manual',
    delivery_status       TEXT NOT NULL DEFAULT 'manual',
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organisation_id, id),
    CHECK (length(email_normalized) BETWEEN 3 AND 320),
    CHECK (email_normalized = lower(email_normalized)),
    CHECK (position('@' IN email_normalized) > 1),
    CHECK (email_normalized !~ '[[:space:][:cntrl:]]'),
    CHECK (length(intended_display_name) <= 255),
    CHECK (status IN ('pending', 'accepted', 'revoked', 'expired')),
    CHECK (octet_length(token_digest) = 32),
    CHECK (token_generation > 0),
    CHECK (expires_at > created_at),
    CHECK (delivery_mode IN ('manual', 'email')),
    CHECK (delivery_status IN ('manual', 'pending', 'sent', 'failed')),
    CHECK (
        (status = 'accepted' AND accepted_user_id IS NOT NULL AND accepted_at IS NOT NULL AND revoked_at IS NULL)
        OR (status = 'revoked' AND accepted_user_id IS NULL AND accepted_at IS NULL AND revoked_at IS NOT NULL)
        OR (status IN ('pending', 'expired') AND accepted_user_id IS NULL AND accepted_at IS NULL AND revoked_at IS NULL)
    )
);

CREATE UNIQUE INDEX human_invitations_one_pending_0013_idx
    ON human_invitations(organisation_id, email_normalized)
    WHERE status = 'pending';

CREATE INDEX human_invitations_owner_ledger_0013_idx
    ON human_invitations(organisation_id, status, created_at DESC);

CREATE TABLE human_invitation_groups (
    organisation_id TEXT NOT NULL,
    project_id      TEXT NOT NULL,
    environment_id  TEXT NOT NULL,
    invitation_id   TEXT NOT NULL,
    group_id        TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (invitation_id, group_id),
    FOREIGN KEY (organisation_id, invitation_id)
        REFERENCES human_invitations(organisation_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (organisation_id, project_id, environment_id, group_id)
        REFERENCES access_groups(organisation_id, project_id, environment_id, id) ON DELETE RESTRICT
);

ALTER TABLE human_users
    ALTER COLUMN password_hash DROP NOT NULL;

ALTER TABLE human_users
    ADD CONSTRAINT human_users_auth_method_0013_check
        CHECK (password_hash IS NOT NULL OR identity_origin IN ('invitation', 'federated'));

CREATE TABLE human_federated_identities (
    id                    TEXT PRIMARY KEY,
    user_id               TEXT NOT NULL REFERENCES human_users(id) ON DELETE RESTRICT,
    issuer                TEXT NOT NULL,
    subject               TEXT NOT NULL,
    provider_kind         TEXT NOT NULL DEFAULT 'oidc',
    enabled               BOOLEAN NOT NULL DEFAULT true,
    email_snapshot        TEXT NOT NULL,
    email_verified_at     TIMESTAMPTZ NOT NULL,
    linked_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_authenticated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    disabled_at           TIMESTAMPTZ,
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    link_source           TEXT NOT NULL,
    UNIQUE (issuer, subject),
    CHECK (length(issuer) BETWEEN 8 AND 2048),
    CHECK (length(subject) BETWEEN 1 AND 255),
    CHECK (provider_kind = 'oidc'),
    CHECK (length(email_snapshot) BETWEEN 3 AND 320),
    CHECK (email_snapshot = lower(email_snapshot)),
    CHECK ((enabled AND disabled_at IS NULL) OR (NOT enabled AND disabled_at IS NOT NULL)),
    CHECK (link_source IN ('invitation', 'existing_user_link', 'operator_recovery'))
);

CREATE TABLE human_action_sessions (
    id            TEXT PRIMARY KEY,
    action_type   TEXT NOT NULL,
    invitation_id TEXT NOT NULL REFERENCES human_invitations(id) ON DELETE RESTRICT,
    token_digest  BYTEA NOT NULL UNIQUE,
    created_at    TIMESTAMPTZ NOT NULL,
    expires_at    TIMESTAMPTZ NOT NULL,
    consumed_at   TIMESTAMPTZ,
    revoked_at    TIMESTAMPTZ,
    CHECK (action_type = 'accept_invitation'),
    CHECK (octet_length(token_digest) = 32),
    CHECK (expires_at > created_at),
    CHECK (consumed_at IS NULL OR consumed_at >= created_at),
    CHECK (revoked_at IS NULL OR revoked_at >= created_at),
    CHECK (consumed_at IS NULL OR revoked_at IS NULL)
);

CREATE INDEX human_action_sessions_invitation_0013_idx
    ON human_action_sessions(invitation_id, expires_at);

CREATE TABLE oidc_login_transactions (
    id                TEXT PRIMARY KEY,
    action_session_id TEXT NOT NULL REFERENCES human_action_sessions(id) ON DELETE RESTRICT,
    state_digest      BYTEA NOT NULL UNIQUE,
    nonce             TEXT NOT NULL,
    code_verifier     TEXT NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL,
    expires_at        TIMESTAMPTZ NOT NULL,
    consumed_at       TIMESTAMPTZ,
    CHECK (octet_length(state_digest) = 32),
    CHECK (length(nonce) BETWEEN 32 AND 255),
    CHECK (length(code_verifier) BETWEEN 43 AND 128),
    CHECK (expires_at > created_at),
    CHECK (consumed_at IS NULL OR consumed_at >= created_at)
);
