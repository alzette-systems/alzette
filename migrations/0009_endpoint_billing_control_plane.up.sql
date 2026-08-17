-- Add the endpoint-acquisition and provider-neutral billing control plane.
-- Migration 0008 remains the catalogue/deployment foundation. Provider URLs,
-- credentials, card data, and raw webhook payloads have no column here.

ALTER TABLE portal_sessions
    ADD COLUMN authenticated_at TIMESTAMPTZ;
UPDATE portal_sessions SET authenticated_at=created_at WHERE authenticated_at IS NULL;
ALTER TABLE portal_sessions
    ALTER COLUMN authenticated_at SET NOT NULL,
    ADD CONSTRAINT portal_sessions_authenticated_lifecycle_0009_check
        CHECK (authenticated_at >= created_at);

-- A one-time API-key reveal cannot be replayed after an interrupted response.
-- Reserve each human-readable name permanently within its service account so
-- an ambiguous client retry can never mint an unnoticed duplicate credential.
-- Pre-0009 operator rotations used the default value from migration 0005, so
-- preserve the oldest display name and deterministically disambiguate only
-- historical duplicates before installing the invariant.
WITH ranked_key_names AS (
    SELECT id,
           row_number() OVER (
               PARTITION BY service_account_id, name
               ORDER BY created_at, id
           ) AS duplicate_ordinal
      FROM api_keys
)
UPDATE api_keys k
   SET name = left(k.name, 94) || '-legacy-' || substr(md5(k.id), 1, 16)
  FROM ranked_key_names ranked
 WHERE ranked.id = k.id
   AND ranked.duplicate_ordinal > 1;

ALTER TABLE api_keys
    ADD CONSTRAINT api_keys_service_account_name_0009_unique
        UNIQUE (service_account_id, name);

CREATE TABLE endpoint_offers (
    id                         TEXT PRIMARY KEY,
    code                       TEXT NOT NULL UNIQUE,
    name                       TEXT NOT NULL,
    deployment_profile_id      TEXT NOT NULL REFERENCES deployment_profiles(id) ON DELETE RESTRICT,
    routable_model_id          TEXT NOT NULL REFERENCES models(id) ON DELETE RESTRICT,
    target_id                  TEXT REFERENCES inference_targets(id) ON DELETE RESTRICT,
    profile_price_id           TEXT REFERENCES deployment_profile_prices(id) ON DELETE RESTRICT,
    offer_kind                 TEXT NOT NULL,
    status                     TEXT NOT NULL DEFAULT 'draft',
    eligible_evaluation        BOOLEAN NOT NULL DEFAULT false,
    eligible_customer          BOOLEAN NOT NULL DEFAULT true,
    request_allowance          BIGINT,
    token_allowance            BIGINT,
    allowance_period           TEXT,
    source_label               TEXT NOT NULL,
    evidence_ref               TEXT,
    published_at               TIMESTAMPTZ,
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (id, deployment_profile_id, routable_model_id),
    CHECK (code ~ '^[a-z0-9][a-z0-9-]{0,62}$'),
    CHECK (length(name) BETWEEN 1 AND 255),
    CHECK (offer_kind IN ('shared_evaluation', 'shared_subscription', 'dedicated_quote')),
    CHECK (status IN ('draft', 'published', 'unavailable', 'retired')),
    CHECK (eligible_evaluation OR eligible_customer),
    CHECK (request_allowance IS NULL OR request_allowance > 0),
    CHECK (token_allowance IS NULL OR token_allowance > 0),
    CHECK (allowance_period IS NULL OR allowance_period IN ('lifetime', 'month', 'contract_term')),
    CHECK ((request_allowance IS NULL AND token_allowance IS NULL) = (allowance_period IS NULL)),
    CHECK (source_label ~ '^[A-Za-z0-9][A-Za-z0-9 ._:/-]{0,254}$'),
    CHECK (evidence_ref IS NULL OR (length(evidence_ref) <= 1000 AND evidence_ref ~ '^[A-Za-z0-9][A-Za-z0-9 ._:/-]*$')),
    CHECK (
        (offer_kind = 'shared_evaluation'
         AND target_id IS NOT NULL
         AND profile_price_id IS NULL
         AND request_allowance IS NOT NULL)
        OR
        (offer_kind = 'shared_subscription'
         AND target_id IS NOT NULL
         AND profile_price_id IS NOT NULL
         AND request_allowance IS NOT NULL
         AND allowance_period = 'month')
        OR
        (offer_kind = 'dedicated_quote'
         AND target_id IS NULL
         AND profile_price_id IS NULL
         AND request_allowance IS NULL
         AND token_allowance IS NULL)
    ),
    CHECK (
        (status = 'draft' AND published_at IS NULL)
        OR (status <> 'draft' AND published_at IS NOT NULL)
    ),
    CHECK (status <> 'published' OR evidence_ref IS NOT NULL)
);

CREATE TABLE billing_price_mappings (
    offer_id             TEXT NOT NULL REFERENCES endpoint_offers(id) ON DELETE RESTRICT,
    provider             TEXT NOT NULL,
    provider_price_ref   TEXT NOT NULL,
    active               BOOLEAN NOT NULL DEFAULT true,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (offer_id, provider),
    UNIQUE (provider, provider_price_ref),
    CHECK (provider IN ('stripe')),
    CHECK (provider_price_ref ~ '^price_[A-Za-z0-9_]{6,120}$')
);

CREATE TABLE endpoint_configurations (
    id                            TEXT PRIMARY KEY,
    organisation_id               TEXT NOT NULL,
    project_id                    TEXT NOT NULL,
    environment_id                TEXT NOT NULL,
    offer_id                      TEXT NOT NULL,
    deployment_profile_id         TEXT NOT NULL,
    routable_model_id             TEXT NOT NULL,
    endpoint_alias                TEXT NOT NULL,
    capacity_units                INTEGER NOT NULL,
    workload_use_case             TEXT NOT NULL DEFAULT '',
    expected_context_tokens       BIGINT,
    expected_concurrency          INTEGER,
    expected_requests_per_minute  INTEGER,
    latency_priority              TEXT,
    expected_monthly_requests     BIGINT,
    status                        TEXT NOT NULL DEFAULT 'draft',
    requested_by_user_id          TEXT NOT NULL REFERENCES human_users(id) ON DELETE RESTRICT,
    idempotency_key_hash          BYTEA NOT NULL,
    deployment_request_id         TEXT,
    submitted_at                  TIMESTAMPTZ,
    cancelled_at                  TIMESTAMPTZ,
    created_at                    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                    TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (organisation_id, project_id, environment_id)
        REFERENCES environments(organisation_id, project_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (offer_id, deployment_profile_id, routable_model_id)
        REFERENCES endpoint_offers(id, deployment_profile_id, routable_model_id) ON DELETE RESTRICT,
    FOREIGN KEY (organisation_id, project_id, environment_id, deployment_request_id)
        REFERENCES deployment_requests(organisation_id, project_id, environment_id, id) ON DELETE RESTRICT,
    UNIQUE (organisation_id, project_id, environment_id, id),
    UNIQUE (organisation_id, idempotency_key_hash),
    CHECK (octet_length(idempotency_key_hash) = 32),
    CHECK (endpoint_alias ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$'),
    CHECK (capacity_units > 0),
    CHECK (length(workload_use_case) <= 2000),
    CHECK (expected_context_tokens IS NULL OR expected_context_tokens > 0),
    CHECK (expected_concurrency IS NULL OR expected_concurrency > 0),
    CHECK (expected_requests_per_minute IS NULL OR expected_requests_per_minute > 0),
    CHECK (latency_priority IS NULL OR latency_priority IN ('balanced', 'latency', 'throughput')),
    CHECK (expected_monthly_requests IS NULL OR expected_monthly_requests > 0),
    CHECK (status IN ('draft', 'submitted', 'activated', 'cancelled')),
    CHECK ((status = 'draft' AND submitted_at IS NULL AND cancelled_at IS NULL)
           OR (status IN ('submitted', 'activated') AND submitted_at IS NOT NULL AND cancelled_at IS NULL)
           OR (status = 'cancelled' AND cancelled_at IS NOT NULL)),
    CHECK (status = 'draft' OR deployment_request_id IS NOT NULL)
);

CREATE TABLE customer_endpoints (
    id                         TEXT PRIMARY KEY,
    organisation_id            TEXT NOT NULL,
    project_id                 TEXT NOT NULL,
    environment_id             TEXT NOT NULL,
    configuration_id           TEXT NOT NULL,
    offer_id                   TEXT NOT NULL,
    deployment_profile_id      TEXT NOT NULL,
    routable_model_id          TEXT NOT NULL,
    deployment_id              TEXT,
    route_id                   TEXT,
    endpoint_alias             TEXT NOT NULL,
    service_mode               TEXT NOT NULL,
    commercial_state           TEXT NOT NULL,
    runtime_state              TEXT NOT NULL,
    capacity_units             INTEGER NOT NULL,
    request_allowance          BIGINT,
    token_allowance            BIGINT,
    allowance_period           TEXT,
    allowance_period_start     TIMESTAMPTZ,
    allowance_period_end       TIMESTAMPTZ,
    allowance_requests_used    BIGINT NOT NULL DEFAULT 0,
    latest_payment_event_at    TIMESTAMPTZ,
    latest_payment_event_id    TEXT,
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (organisation_id, project_id, environment_id, configuration_id)
        REFERENCES endpoint_configurations(organisation_id, project_id, environment_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (offer_id, deployment_profile_id, routable_model_id)
        REFERENCES endpoint_offers(id, deployment_profile_id, routable_model_id) ON DELETE RESTRICT,
    FOREIGN KEY (organisation_id, project_id, environment_id, deployment_id)
        REFERENCES model_deployments(organisation_id, project_id, environment_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (organisation_id, project_id, environment_id, route_id)
        REFERENCES tenant_routes(organisation_id, project_id, environment_id, id) ON DELETE RESTRICT,
    UNIQUE (organisation_id, project_id, environment_id, id),
    UNIQUE (organisation_id, id),
    UNIQUE (configuration_id),
    UNIQUE (route_id),
    CHECK (endpoint_alias ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$'),
    CHECK (service_mode IN ('shared_evaluation', 'shared_subscription', 'dedicated_private')),
    CHECK (commercial_state IN ('not_required', 'quote_pending', 'quote_offered', 'quote_accepted', 'payment_action_required', 'processing', 'paid', 'past_due', 'failed', 'cancelled', 'refunded', 'disputed')),
    CHECK (runtime_state IN ('awaiting_submission', 'awaiting_payment', 'awaiting_allocation', 'allocating', 'deploying', 'validating', 'route_bound', 'ready', 'degraded', 'failed', 'retired')),
    CHECK (capacity_units > 0),
    CHECK (request_allowance IS NULL OR request_allowance > 0),
    CHECK (token_allowance IS NULL OR token_allowance > 0),
    CHECK (allowance_requests_used >= 0),
    CHECK (allowance_period IS NULL OR allowance_period IN ('lifetime', 'month', 'contract_term')),
    CHECK ((request_allowance IS NULL AND token_allowance IS NULL) = (allowance_period IS NULL)),
    CHECK ((allowance_period_start IS NULL) = (allowance_period_end IS NULL)),
    CHECK (allowance_period_end IS NULL OR allowance_period_end > allowance_period_start),
    CHECK ((latest_payment_event_at IS NULL) = (latest_payment_event_id IS NULL)),
    CHECK (route_id IS NOT NULL OR runtime_state NOT IN ('route_bound', 'ready', 'degraded')),
    CHECK (service_mode <> 'dedicated_private' OR request_allowance IS NULL),
    CHECK (service_mode = 'dedicated_private' OR deployment_id IS NULL)
);

CREATE TABLE deployment_quote_payment_terms (
    organisation_id      TEXT NOT NULL,
    project_id           TEXT NOT NULL,
    environment_id       TEXT NOT NULL,
    quote_id             TEXT NOT NULL,
    collection_mode      TEXT NOT NULL,
    payment_due_days     INTEGER,
    source_label         TEXT NOT NULL,
    evidence_ref         TEXT NOT NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (organisation_id, project_id, environment_id, quote_id),
    FOREIGN KEY (organisation_id, project_id, environment_id, quote_id)
        REFERENCES deployment_quotes(organisation_id, project_id, environment_id, id) ON DELETE RESTRICT,
    CHECK (collection_mode IN ('checkout_payment', 'invoice', 'invoice_terms', 'not_required')),
    CHECK ((collection_mode = 'invoice_terms') = (payment_due_days IS NOT NULL)),
    CHECK (payment_due_days IS NULL OR payment_due_days BETWEEN 1 AND 120),
    CHECK (source_label ~ '^[A-Za-z0-9][A-Za-z0-9 ._:/-]{0,254}$'),
    CHECK (length(evidence_ref) <= 1000 AND evidence_ref ~ '^[A-Za-z0-9][A-Za-z0-9 ._:/-]*$')
);

CREATE TABLE billing_accounts (
    id                         TEXT PRIMARY KEY,
    organisation_id            TEXT NOT NULL UNIQUE REFERENCES organisations(id) ON DELETE RESTRICT,
    provider                   TEXT NOT NULL,
    provider_customer_ref      TEXT NOT NULL,
    lifecycle_state            TEXT NOT NULL DEFAULT 'active',
    legal_name                 TEXT NOT NULL,
    tax_status                 TEXT NOT NULL DEFAULT 'not_collected',
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, provider_customer_ref),
    UNIQUE (organisation_id, id),
    CHECK (provider IN ('stripe')),
    CHECK (provider_customer_ref ~ '^cus_[A-Za-z0-9_]{6,120}$'),
    CHECK (lifecycle_state IN ('active', 'disabled')),
    CHECK (length(legal_name) BETWEEN 1 AND 255),
    CHECK (tax_status IN ('not_collected', 'pending', 'verified', 'invalid'))
);

CREATE TABLE payment_requirements (
    id                         TEXT PRIMARY KEY,
    organisation_id            TEXT NOT NULL,
    project_id                 TEXT NOT NULL,
    environment_id             TEXT NOT NULL,
    endpoint_id                TEXT NOT NULL,
    deployment_request_id      TEXT,
    quote_id                   TEXT,
    purpose                    TEXT NOT NULL,
    amount_minor               BIGINT NOT NULL,
    currency                   TEXT NOT NULL,
    billing_period             TEXT NOT NULL,
    tax_treatment              TEXT NOT NULL,
    collection_mode            TEXT NOT NULL,
    state                      TEXT NOT NULL DEFAULT 'pending',
    price_finality             TEXT NOT NULL,
    source_label               TEXT NOT NULL,
    evidence_ref               TEXT NOT NULL,
    paid_at                    TIMESTAMPTZ,
    paid_amount_minor          BIGINT,
    paid_currency              TEXT,
    safe_failure_class         TEXT,
    latest_event_at            TIMESTAMPTZ,
    latest_event_id            TEXT,
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (organisation_id, project_id, environment_id, endpoint_id)
        REFERENCES customer_endpoints(organisation_id, project_id, environment_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (organisation_id, project_id, environment_id, deployment_request_id)
        REFERENCES deployment_requests(organisation_id, project_id, environment_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (organisation_id, project_id, environment_id, quote_id)
        REFERENCES deployment_quotes(organisation_id, project_id, environment_id, id) ON DELETE RESTRICT,
    UNIQUE (organisation_id, project_id, environment_id, id),
    UNIQUE (organisation_id, id),
    UNIQUE (endpoint_id, purpose, quote_id),
    CHECK (purpose IN ('shared_activation', 'dedicated_setup', 'dedicated_recurring', 'capacity_change')),
    CHECK (amount_minor >= 0),
    CHECK (currency ~ '^[A-Z]{3}$'),
    CHECK (billing_period IN ('one_time', 'month', 'contract_term')),
    CHECK (tax_treatment IN ('not_determined', 'exclusive', 'inclusive', 'not_applicable')),
    CHECK (collection_mode IN ('checkout_subscription', 'checkout_payment', 'invoice', 'invoice_terms')),
    CHECK (state IN ('pending', 'action_required', 'processing', 'paid', 'past_due', 'failed', 'cancelled', 'refunded', 'disputed')),
    CHECK (price_finality IN ('indicative', 'contractual')),
    CHECK (source_label ~ '^[A-Za-z0-9][A-Za-z0-9 ._:/-]{0,254}$'),
    CHECK (length(evidence_ref) <= 1000 AND evidence_ref ~ '^[A-Za-z0-9][A-Za-z0-9 ._:/-]*$'),
    CHECK (safe_failure_class IS NULL OR safe_failure_class ~ '^[a-z][a-z0-9_]{1,62}$'),
    CHECK ((paid_at IS NULL AND paid_amount_minor IS NULL AND paid_currency IS NULL)
           OR (paid_at IS NOT NULL AND paid_amount_minor IS NOT NULL AND paid_amount_minor >= 0 AND paid_currency ~ '^[A-Z]{3}$')),
    CHECK (state <> 'paid' OR paid_at IS NOT NULL),
    CHECK ((latest_event_at IS NULL) = (latest_event_id IS NULL))
);

CREATE UNIQUE INDEX payment_requirements_shared_purpose_0009_uidx
    ON payment_requirements(endpoint_id, purpose)
    WHERE quote_id IS NULL;

CREATE TABLE billing_checkout_sessions (
    id                         TEXT PRIMARY KEY,
    organisation_id            TEXT NOT NULL,
    payment_requirement_id     TEXT NOT NULL,
    provider                   TEXT NOT NULL,
    provider_session_ref       TEXT,
    operation_key_hash         BYTEA NOT NULL,
    state                      TEXT NOT NULL DEFAULT 'creating',
    expires_at                 TIMESTAMPTZ,
    completed_at               TIMESTAMPTZ,
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (organisation_id, payment_requirement_id)
        REFERENCES payment_requirements(organisation_id, id) ON DELETE RESTRICT,
    UNIQUE (provider, provider_session_ref),
    UNIQUE (operation_key_hash),
    UNIQUE (payment_requirement_id, id),
    CHECK (provider IN ('stripe')),
    CHECK (provider_session_ref IS NULL OR provider_session_ref ~ '^cs_[A-Za-z0-9_]{6,180}$'),
    CHECK (octet_length(operation_key_hash) = 32),
    CHECK (state IN ('creating', 'open', 'complete', 'expired', 'failed')),
    CHECK (state = 'creating' OR provider_session_ref IS NOT NULL),
    CHECK (state <> 'complete' OR completed_at IS NOT NULL)
);

CREATE UNIQUE INDEX billing_checkout_sessions_one_open_0009_uidx
    ON billing_checkout_sessions(payment_requirement_id)
    WHERE state IN ('creating', 'open');

CREATE TABLE billing_subscriptions (
    id                         TEXT PRIMARY KEY,
    organisation_id            TEXT NOT NULL,
    endpoint_id                TEXT NOT NULL,
    payment_requirement_id     TEXT NOT NULL,
    provider                   TEXT NOT NULL,
    provider_subscription_ref  TEXT NOT NULL,
    provider_customer_ref      TEXT NOT NULL,
    status                     TEXT NOT NULL,
    current_period_start       TIMESTAMPTZ,
    current_period_end         TIMESTAMPTZ,
    cancel_at_period_end       BOOLEAN NOT NULL DEFAULT false,
    fixed_amount_minor         BIGINT NOT NULL,
    currency                   TEXT NOT NULL,
    billing_period             TEXT NOT NULL,
    latest_event_at            TIMESTAMPTZ NOT NULL,
    latest_event_id            TEXT NOT NULL,
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (organisation_id, endpoint_id)
        REFERENCES customer_endpoints(organisation_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (organisation_id, payment_requirement_id)
        REFERENCES payment_requirements(organisation_id, id) ON DELETE RESTRICT,
    UNIQUE (provider, provider_subscription_ref),
    CHECK (provider IN ('stripe')),
    CHECK (provider_subscription_ref ~ '^sub_[A-Za-z0-9_]{6,120}$'),
    CHECK (provider_customer_ref ~ '^cus_[A-Za-z0-9_]{6,120}$'),
    CHECK (status IN ('incomplete', 'trialing', 'active', 'past_due', 'cancelled', 'unpaid', 'paused')),
    CHECK ((current_period_start IS NULL) = (current_period_end IS NULL)),
    CHECK (current_period_end IS NULL OR current_period_end > current_period_start),
    CHECK (fixed_amount_minor >= 0),
    CHECK (currency ~ '^[A-Z]{3}$'),
    CHECK (billing_period = 'month')
);

CREATE TABLE billing_invoices (
    id                         TEXT PRIMARY KEY,
    organisation_id            TEXT NOT NULL,
    endpoint_id                TEXT NOT NULL,
    payment_requirement_id     TEXT NOT NULL,
    provider                   TEXT NOT NULL,
    provider_invoice_ref       TEXT NOT NULL,
    provider_subscription_ref  TEXT,
    safe_number                TEXT,
    status                     TEXT NOT NULL,
    amount_due_minor           BIGINT NOT NULL,
    amount_paid_minor          BIGINT NOT NULL,
    currency                   TEXT NOT NULL,
    due_at                     TIMESTAMPTZ,
    paid_at                    TIMESTAMPTZ,
    latest_event_at            TIMESTAMPTZ NOT NULL,
    latest_event_id            TEXT NOT NULL,
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (organisation_id, endpoint_id)
        REFERENCES customer_endpoints(organisation_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (organisation_id, payment_requirement_id)
        REFERENCES payment_requirements(organisation_id, id) ON DELETE RESTRICT,
    UNIQUE (provider, provider_invoice_ref),
    CHECK (provider IN ('stripe')),
    CHECK (provider_invoice_ref ~ '^in_[A-Za-z0-9_]{6,120}$'),
    CHECK (provider_subscription_ref IS NULL OR provider_subscription_ref ~ '^sub_[A-Za-z0-9_]{6,120}$'),
    CHECK (safe_number IS NULL OR length(safe_number) <= 128),
    CHECK (status IN ('draft', 'open', 'paid', 'void', 'uncollectible', 'past_due')),
    CHECK (amount_due_minor >= 0 AND amount_paid_minor >= 0),
    CHECK (currency ~ '^[A-Z]{3}$'),
    CHECK (status <> 'paid' OR paid_at IS NOT NULL)
);

CREATE TABLE billing_webhook_receipts (
    provider                       TEXT NOT NULL,
    provider_event_id              TEXT NOT NULL,
    event_type                     TEXT NOT NULL,
    provider_object_ref            TEXT NOT NULL,
    payment_requirement_id         TEXT,
    provider_customer_ref          TEXT,
    provider_subscription_ref      TEXT,
    provider_invoice_ref           TEXT,
    object_status                  TEXT,
    payment_status                 TEXT,
    amount_minor                   BIGINT,
    currency                       TEXT,
    period_start                   TIMESTAMPTZ,
    period_end                     TIMESTAMPTZ,
    cancel_at_period_end           BOOLEAN,
    provider_created_at            TIMESTAMPTZ NOT NULL,
    payload_digest                 BYTEA NOT NULL,
    signature_verified_at          TIMESTAMPTZ NOT NULL,
    processing_state               TEXT NOT NULL DEFAULT 'received',
    processing_attempts            INTEGER NOT NULL DEFAULT 0,
    safe_failure_class             TEXT,
    processed_at                   TIMESTAMPTZ,
    created_at                     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (provider, provider_event_id),
    CHECK (provider IN ('stripe')),
    CHECK (provider_event_id ~ '^evt_[A-Za-z0-9_]{6,180}$'),
    CHECK (length(event_type) BETWEEN 3 AND 128),
    CHECK (length(provider_object_ref) BETWEEN 3 AND 255),
    CHECK (payment_requirement_id IS NULL OR payment_requirement_id ~ '^pay_[A-Za-z0-9_:-]{4,180}$'),
    CHECK (provider_customer_ref IS NULL OR provider_customer_ref ~ '^cus_[A-Za-z0-9_]{6,120}$'),
    CHECK (provider_subscription_ref IS NULL OR provider_subscription_ref ~ '^sub_[A-Za-z0-9_]{6,120}$'),
    CHECK (provider_invoice_ref IS NULL OR provider_invoice_ref ~ '^in_[A-Za-z0-9_]{6,120}$'),
    CHECK (amount_minor IS NULL OR amount_minor >= 0),
    CHECK (currency IS NULL OR currency ~ '^[A-Z]{3}$'),
    CHECK ((period_start IS NULL) = (period_end IS NULL)),
    CHECK (period_end IS NULL OR period_end > period_start),
    CHECK (octet_length(payload_digest) = 32),
    CHECK (processing_state IN ('received', 'processed', 'deferred', 'rejected')),
    CHECK (processing_attempts >= 0),
    CHECK (safe_failure_class IS NULL OR safe_failure_class ~ '^[a-z][a-z0-9_]{1,62}$'),
    CHECK ((processing_state = 'processed') = (processed_at IS NOT NULL))
);

CREATE INDEX endpoint_configurations_scope_time_0009_idx
    ON endpoint_configurations(organisation_id, project_id, environment_id, created_at DESC);
CREATE INDEX customer_endpoints_scope_time_0009_idx
    ON customer_endpoints(organisation_id, project_id, environment_id, created_at DESC);
CREATE INDEX payment_requirements_scope_state_0009_idx
    ON payment_requirements(organisation_id, project_id, environment_id, state, created_at DESC);
CREATE INDEX billing_webhook_receipts_processing_0009_idx
    ON billing_webhook_receipts(processing_state, created_at)
    WHERE processing_state IN ('received', 'deferred');
CREATE INDEX billing_webhook_receipts_object_0009_idx
    ON billing_webhook_receipts(provider, event_type, provider_object_ref, provider_created_at DESC);

CREATE OR REPLACE FUNCTION enforce_endpoint_offer_0009() RETURNS trigger AS $$
DECLARE
    profile_mode TEXT;
    profile_status TEXT;
    mapped_model TEXT;
    target_mode TEXT;
    target_owner TEXT;
    price_profile TEXT;
    price_finality TEXT;
BEGIN
    SELECT p.service_mode, p.status, v.routable_model_id
      INTO profile_mode, profile_status, mapped_model
      FROM deployment_profiles p
      JOIN catalogue_model_versions v ON v.id=p.catalogue_model_version_id
     WHERE p.id=NEW.deployment_profile_id;
    IF profile_status IS DISTINCT FROM 'quotable' OR mapped_model IS DISTINCT FROM NEW.routable_model_id THEN
        RAISE EXCEPTION 'endpoint offer requires a quotable profile with its mapped model' USING ERRCODE='23514';
    END IF;
    IF NEW.offer_kind IN ('shared_evaluation','shared_subscription') THEN
        IF profile_mode IS DISTINCT FROM 'shared_evaluation' THEN
            RAISE EXCEPTION 'shared endpoint offer requires a shared profile' USING ERRCODE='23514';
        END IF;
        SELECT capacity_mode, owner_organisation_id INTO target_mode, target_owner
          FROM inference_targets WHERE id=NEW.target_id;
        IF target_mode IS DISTINCT FROM 'shared' OR target_owner IS NOT NULL THEN
            RAISE EXCEPTION 'shared endpoint offer requires an unowned shared target' USING ERRCODE='23514';
        END IF;
    ELSIF profile_mode IS DISTINCT FROM 'dedicated_private' THEN
        RAISE EXCEPTION 'dedicated endpoint offer requires a dedicated profile' USING ERRCODE='23514';
    END IF;
    IF NEW.profile_price_id IS NOT NULL THEN
        SELECT deployment_profile_id, finality INTO price_profile, price_finality
          FROM deployment_profile_prices WHERE id=NEW.profile_price_id;
        IF price_profile IS DISTINCT FROM NEW.deployment_profile_id OR price_finality IS DISTINCT FROM 'contractual' THEN
            RAISE EXCEPTION 'paid shared offer requires a contractual price for its profile' USING ERRCODE='23514';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER endpoint_offers_enforce_0009
BEFORE INSERT OR UPDATE ON endpoint_offers
FOR EACH ROW EXECUTE FUNCTION enforce_endpoint_offer_0009();

CREATE OR REPLACE FUNCTION protect_referenced_endpoint_offer_0009() RETURNS trigger AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM endpoint_configurations c WHERE c.offer_id=OLD.id)
       AND (NEW.deployment_profile_id IS DISTINCT FROM OLD.deployment_profile_id
            OR NEW.routable_model_id IS DISTINCT FROM OLD.routable_model_id
            OR NEW.target_id IS DISTINCT FROM OLD.target_id
            OR NEW.profile_price_id IS DISTINCT FROM OLD.profile_price_id
            OR NEW.offer_kind IS DISTINCT FROM OLD.offer_kind
            OR NEW.request_allowance IS DISTINCT FROM OLD.request_allowance
            OR NEW.token_allowance IS DISTINCT FROM OLD.token_allowance
            OR NEW.allowance_period IS DISTINCT FROM OLD.allowance_period) THEN
        RAISE EXCEPTION 'referenced endpoint offer facts are immutable' USING ERRCODE='55000';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER endpoint_offers_protect_references_0009
BEFORE UPDATE ON endpoint_offers
FOR EACH ROW EXECUTE FUNCTION protect_referenced_endpoint_offer_0009();

CREATE OR REPLACE FUNCTION protect_billing_price_mapping_0009() RETURNS trigger AS $$
BEGIN
    IF TG_OP='DELETE' AND EXISTS (SELECT 1 FROM endpoint_configurations c WHERE c.offer_id=OLD.offer_id) THEN
        RAISE EXCEPTION 'referenced billing price mapping is retained' USING ERRCODE='55000';
    END IF;
    IF TG_OP='UPDATE' AND EXISTS (SELECT 1 FROM endpoint_configurations c WHERE c.offer_id=OLD.offer_id)
       AND (NEW.provider IS DISTINCT FROM OLD.provider OR NEW.provider_price_ref IS DISTINCT FROM OLD.provider_price_ref) THEN
        RAISE EXCEPTION 'referenced billing price mapping is immutable' USING ERRCODE='55000';
    END IF;
    RETURN COALESCE(NEW,OLD);
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER billing_price_mappings_protect_0009
BEFORE UPDATE OR DELETE ON billing_price_mappings
FOR EACH ROW EXECUTE FUNCTION protect_billing_price_mapping_0009();

CREATE OR REPLACE FUNCTION enforce_endpoint_configuration_0009() RETURNS trigger AS $$
DECLARE
    profile_min INTEGER;
    profile_max INTEGER;
    model_alias TEXT;
BEGIN
    SELECT p.min_capacity_units,p.max_capacity_units,m.alias
      INTO profile_min,profile_max,model_alias
      FROM deployment_profiles p
      JOIN endpoint_offers o ON o.deployment_profile_id=p.id AND o.id=NEW.offer_id
      JOIN models m ON m.id=o.routable_model_id
     WHERE o.status='published'
       AND ((o.eligible_evaluation AND EXISTS (SELECT 1 FROM organisations x WHERE x.id=NEW.organisation_id AND x.account_kind='evaluation'))
            OR (o.eligible_customer AND EXISTS (SELECT 1 FROM organisations x WHERE x.id=NEW.organisation_id AND x.account_kind='customer')));
    IF profile_min IS NULL OR NEW.capacity_units < profile_min OR NEW.capacity_units > profile_max THEN
        RAISE EXCEPTION 'endpoint configuration profile is not eligible or units are invalid' USING ERRCODE='23514';
    END IF;
    IF NEW.endpoint_alias IS DISTINCT FROM model_alias THEN
        RAISE EXCEPTION 'endpoint alias must be the server-approved model alias' USING ERRCODE='23514';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM human_memberships hm
         WHERE hm.user_id=NEW.requested_by_user_id AND hm.organisation_id=NEW.organisation_id
           AND hm.project_id=NEW.project_id AND hm.environment_id=NEW.environment_id AND hm.enabled
    ) THEN
        RAISE EXCEPTION 'endpoint configuration actor lacks the exact tenant scope' USING ERRCODE='23514';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER endpoint_configurations_enforce_0009
BEFORE INSERT OR UPDATE ON endpoint_configurations
FOR EACH ROW EXECUTE FUNCTION enforce_endpoint_configuration_0009();

CREATE OR REPLACE FUNCTION protect_submitted_endpoint_configuration_0009() RETURNS trigger AS $$
BEGIN
    IF TG_OP='DELETE' AND OLD.status <> 'draft' THEN
        RAISE EXCEPTION 'submitted endpoint configuration is immutable' USING ERRCODE='55000';
    END IF;
    IF TG_OP='UPDATE' AND OLD.status <> 'draft' AND (
        NEW.offer_id IS DISTINCT FROM OLD.offer_id OR
        NEW.deployment_profile_id IS DISTINCT FROM OLD.deployment_profile_id OR
        NEW.routable_model_id IS DISTINCT FROM OLD.routable_model_id OR
        NEW.endpoint_alias IS DISTINCT FROM OLD.endpoint_alias OR
        NEW.capacity_units IS DISTINCT FROM OLD.capacity_units OR
        NEW.workload_use_case IS DISTINCT FROM OLD.workload_use_case OR
        NEW.expected_context_tokens IS DISTINCT FROM OLD.expected_context_tokens OR
        NEW.expected_concurrency IS DISTINCT FROM OLD.expected_concurrency OR
        NEW.expected_requests_per_minute IS DISTINCT FROM OLD.expected_requests_per_minute OR
        NEW.latency_priority IS DISTINCT FROM OLD.latency_priority OR
        NEW.expected_monthly_requests IS DISTINCT FROM OLD.expected_monthly_requests OR
        NEW.requested_by_user_id IS DISTINCT FROM OLD.requested_by_user_id OR
        NEW.idempotency_key_hash IS DISTINCT FROM OLD.idempotency_key_hash) THEN
        RAISE EXCEPTION 'submitted endpoint configuration facts are immutable' USING ERRCODE='55000';
    END IF;
    RETURN COALESCE(NEW,OLD);
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER endpoint_configurations_protect_submitted_0009
BEFORE UPDATE OR DELETE ON endpoint_configurations
FOR EACH ROW EXECUTE FUNCTION protect_submitted_endpoint_configuration_0009();

CREATE OR REPLACE FUNCTION enforce_customer_endpoint_0009() RETURNS trigger AS $$
DECLARE
    config_offer TEXT;
    config_profile TEXT;
    config_model TEXT;
    config_alias TEXT;
    offer_kind_value TEXT;
    offer_target TEXT;
    route_target TEXT;
    route_model TEXT;
    deployment_profile TEXT;
BEGIN
    SELECT offer_id,deployment_profile_id,routable_model_id,endpoint_alias
      INTO config_offer,config_profile,config_model,config_alias
      FROM endpoint_configurations
     WHERE organisation_id=NEW.organisation_id AND project_id=NEW.project_id
       AND environment_id=NEW.environment_id AND id=NEW.configuration_id;
    IF config_offer IS DISTINCT FROM NEW.offer_id OR config_profile IS DISTINCT FROM NEW.deployment_profile_id
       OR config_model IS DISTINCT FROM NEW.routable_model_id OR config_alias IS DISTINCT FROM NEW.endpoint_alias THEN
        RAISE EXCEPTION 'endpoint facts do not match their configuration' USING ERRCODE='23514';
    END IF;
    SELECT offer_kind,target_id INTO offer_kind_value,offer_target FROM endpoint_offers WHERE id=NEW.offer_id;
    IF NEW.service_mode IS DISTINCT FROM (CASE WHEN offer_kind_value='dedicated_quote' THEN 'dedicated_private' ELSE offer_kind_value END) THEN
        RAISE EXCEPTION 'endpoint service mode does not match its offer' USING ERRCODE='23514';
    END IF;
    IF NEW.route_id IS NOT NULL THEN
        SELECT target_id,model_id INTO route_target,route_model FROM tenant_routes
         WHERE organisation_id=NEW.organisation_id AND project_id=NEW.project_id
           AND environment_id=NEW.environment_id AND id=NEW.route_id;
        IF route_model IS DISTINCT FROM NEW.routable_model_id THEN
            RAISE EXCEPTION 'endpoint route model does not match' USING ERRCODE='23514';
        END IF;
        IF NEW.service_mode <> 'dedicated_private' AND route_target IS DISTINCT FROM offer_target THEN
            RAISE EXCEPTION 'shared endpoint route target does not match its allow-listed offer' USING ERRCODE='23514';
        END IF;
    END IF;
    IF NEW.deployment_id IS NOT NULL THEN
        SELECT deployment_profile_id INTO deployment_profile FROM model_deployments
         WHERE organisation_id=NEW.organisation_id AND project_id=NEW.project_id
           AND environment_id=NEW.environment_id AND id=NEW.deployment_id;
        IF deployment_profile IS DISTINCT FROM NEW.deployment_profile_id THEN
            RAISE EXCEPTION 'endpoint deployment profile does not match' USING ERRCODE='23514';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER customer_endpoints_enforce_0009
BEFORE INSERT OR UPDATE ON customer_endpoints
FOR EACH ROW EXECUTE FUNCTION enforce_customer_endpoint_0009();

CREATE OR REPLACE FUNCTION protect_customer_endpoint_facts_0009() RETURNS trigger AS $$
BEGIN
    IF TG_OP='DELETE' THEN
        RAISE EXCEPTION 'customer endpoint evidence is retained' USING ERRCODE='55000';
    END IF;
    IF NEW.organisation_id IS DISTINCT FROM OLD.organisation_id OR NEW.project_id IS DISTINCT FROM OLD.project_id
       OR NEW.environment_id IS DISTINCT FROM OLD.environment_id OR NEW.configuration_id IS DISTINCT FROM OLD.configuration_id
       OR NEW.offer_id IS DISTINCT FROM OLD.offer_id OR NEW.deployment_profile_id IS DISTINCT FROM OLD.deployment_profile_id
       OR NEW.routable_model_id IS DISTINCT FROM OLD.routable_model_id OR NEW.endpoint_alias IS DISTINCT FROM OLD.endpoint_alias
       OR NEW.service_mode IS DISTINCT FROM OLD.service_mode OR NEW.capacity_units IS DISTINCT FROM OLD.capacity_units
       OR NEW.request_allowance IS DISTINCT FROM OLD.request_allowance OR NEW.token_allowance IS DISTINCT FROM OLD.token_allowance
       OR NEW.allowance_period IS DISTINCT FROM OLD.allowance_period THEN
        RAISE EXCEPTION 'customer endpoint identity and commercial facts are immutable' USING ERRCODE='55000';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER customer_endpoints_protect_facts_0009
BEFORE UPDATE OR DELETE ON customer_endpoints
FOR EACH ROW EXECUTE FUNCTION protect_customer_endpoint_facts_0009();

CREATE OR REPLACE FUNCTION protect_payment_requirement_facts_0009() RETURNS trigger AS $$
BEGIN
    IF TG_OP='DELETE' THEN
        RAISE EXCEPTION 'payment requirement evidence is retained' USING ERRCODE='55000';
    END IF;
    IF NEW.organisation_id IS DISTINCT FROM OLD.organisation_id OR NEW.project_id IS DISTINCT FROM OLD.project_id
       OR NEW.environment_id IS DISTINCT FROM OLD.environment_id OR NEW.endpoint_id IS DISTINCT FROM OLD.endpoint_id
       OR NEW.deployment_request_id IS DISTINCT FROM OLD.deployment_request_id OR NEW.quote_id IS DISTINCT FROM OLD.quote_id
       OR NEW.purpose IS DISTINCT FROM OLD.purpose OR NEW.amount_minor IS DISTINCT FROM OLD.amount_minor
       OR NEW.currency IS DISTINCT FROM OLD.currency OR NEW.billing_period IS DISTINCT FROM OLD.billing_period
       OR NEW.tax_treatment IS DISTINCT FROM OLD.tax_treatment OR NEW.collection_mode IS DISTINCT FROM OLD.collection_mode
       OR NEW.price_finality IS DISTINCT FROM OLD.price_finality OR NEW.source_label IS DISTINCT FROM OLD.source_label
       OR NEW.evidence_ref IS DISTINCT FROM OLD.evidence_ref THEN
        RAISE EXCEPTION 'payment requirement commercial facts are immutable' USING ERRCODE='55000';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER payment_requirements_protect_facts_0009
BEFORE UPDATE OR DELETE ON payment_requirements
FOR EACH ROW EXECUTE FUNCTION protect_payment_requirement_facts_0009();

CREATE OR REPLACE FUNCTION protect_billing_checkout_session_0009() RETURNS trigger AS $$
BEGIN
    IF TG_OP='DELETE' THEN
        RAISE EXCEPTION 'checkout session evidence is retained' USING ERRCODE='55000';
    END IF;
    IF NEW.organisation_id IS DISTINCT FROM OLD.organisation_id OR NEW.payment_requirement_id IS DISTINCT FROM OLD.payment_requirement_id
       OR NEW.provider IS DISTINCT FROM OLD.provider OR NEW.operation_key_hash IS DISTINCT FROM OLD.operation_key_hash
       OR (OLD.provider_session_ref IS NOT NULL AND NEW.provider_session_ref IS DISTINCT FROM OLD.provider_session_ref) THEN
        RAISE EXCEPTION 'checkout session identity is immutable' USING ERRCODE='55000';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER billing_checkout_sessions_protect_0009
BEFORE UPDATE OR DELETE ON billing_checkout_sessions
FOR EACH ROW EXECUTE FUNCTION protect_billing_checkout_session_0009();

CREATE OR REPLACE FUNCTION protect_billing_webhook_receipt_0009() RETURNS trigger AS $$
BEGIN
    IF TG_OP='DELETE' THEN
        RAISE EXCEPTION 'billing webhook receipts are append-only evidence' USING ERRCODE='55000';
    END IF;
    IF NEW.provider IS DISTINCT FROM OLD.provider OR NEW.provider_event_id IS DISTINCT FROM OLD.provider_event_id
       OR NEW.event_type IS DISTINCT FROM OLD.event_type OR NEW.provider_object_ref IS DISTINCT FROM OLD.provider_object_ref
       OR NEW.payment_requirement_id IS DISTINCT FROM OLD.payment_requirement_id
       OR NEW.provider_customer_ref IS DISTINCT FROM OLD.provider_customer_ref
       OR NEW.provider_subscription_ref IS DISTINCT FROM OLD.provider_subscription_ref
       OR NEW.provider_invoice_ref IS DISTINCT FROM OLD.provider_invoice_ref
       OR NEW.object_status IS DISTINCT FROM OLD.object_status OR NEW.payment_status IS DISTINCT FROM OLD.payment_status
       OR NEW.amount_minor IS DISTINCT FROM OLD.amount_minor OR NEW.currency IS DISTINCT FROM OLD.currency
       OR NEW.period_start IS DISTINCT FROM OLD.period_start OR NEW.period_end IS DISTINCT FROM OLD.period_end
       OR NEW.cancel_at_period_end IS DISTINCT FROM OLD.cancel_at_period_end
       OR NEW.provider_created_at IS DISTINCT FROM OLD.provider_created_at
       OR NEW.payload_digest IS DISTINCT FROM OLD.payload_digest
       OR NEW.signature_verified_at IS DISTINCT FROM OLD.signature_verified_at THEN
        RAISE EXCEPTION 'verified billing webhook receipt facts are immutable' USING ERRCODE='55000';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER billing_webhook_receipts_protect_0009
BEFORE UPDATE OR DELETE ON billing_webhook_receipts
FOR EACH ROW EXECUTE FUNCTION protect_billing_webhook_receipt_0009();

COMMENT ON TABLE endpoint_offers IS
    'Server-owned catalogue eligibility and commercial mode; shared targets and prices are never accepted from a customer request.';
COMMENT ON TABLE customer_endpoints IS
    'Stable customer endpoint identity with commercial and runtime state kept in separate columns; paid never implies ready.';
COMMENT ON TABLE payment_requirements IS
    'Immutable server-owned payment snapshot. Alzette/DUCHENE is merchant of record; organisations are ordinary provider customers.';
COMMENT ON TABLE billing_webhook_receipts IS
    'Digest-only verified event ledger. Raw Stripe webhook payloads and secrets are not persisted.';
