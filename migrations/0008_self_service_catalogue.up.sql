-- Add the commercial/catalogue and evaluation-account domain without changing
-- the existing gateway, route, target, service-plan, or request-ledger truth.

ALTER TABLE organisations
    ADD COLUMN account_kind TEXT NOT NULL DEFAULT 'customer',
    ADD COLUMN lifecycle_status TEXT NOT NULL DEFAULT 'active',
    ADD COLUMN created_via TEXT NOT NULL DEFAULT 'operator',
    ADD COLUMN business_approved_at TIMESTAMPTZ,
    ADD COLUMN business_approval_evidence_ref TEXT,
    ADD CONSTRAINT organisations_account_kind_0008_check
        CHECK (account_kind IN ('evaluation', 'customer')),
    ADD CONSTRAINT organisations_lifecycle_status_0008_check
        CHECK (
            (account_kind = 'evaluation'
             AND lifecycle_status IN ('evaluation', 'qualification_pending', 'suspended', 'closed'))
            OR
            (account_kind = 'customer'
             AND lifecycle_status IN ('approved', 'active', 'suspended', 'closed'))
        ),
    ADD CONSTRAINT organisations_created_via_0008_check
        CHECK (created_via IN ('operator', 'self_service')),
    ADD CONSTRAINT organisations_business_approval_pair_0008_check
        CHECK ((business_approved_at IS NULL) = (business_approval_evidence_ref IS NULL)),
    ADD CONSTRAINT organisations_self_service_approval_0008_check
        CHECK (
            created_via <> 'self_service'
            OR account_kind = 'evaluation'
            OR (business_approved_at IS NOT NULL AND business_approval_evidence_ref IS NOT NULL)
        ),
    ADD CONSTRAINT organisations_business_approval_evidence_0008_check
        CHECK (
            business_approval_evidence_ref IS NULL
            OR business_approval_evidence_ref ~ '^[A-Za-z0-9][A-Za-z0-9 ._:/-]{0,254}$'
        );

ALTER TABLE human_users
    ADD COLUMN email TEXT,
    ADD COLUMN email_normalized TEXT,
    ADD COLUMN email_verified_at TIMESTAMPTZ,
    ADD COLUMN identity_origin TEXT NOT NULL DEFAULT 'operator_legacy',
    ADD CONSTRAINT human_users_identity_origin_0008_check
        CHECK (identity_origin IN ('operator_legacy', 'self_service', 'invitation', 'federated')),
    ADD CONSTRAINT human_users_email_pair_0008_check
        CHECK ((email IS NULL) = (email_normalized IS NULL)),
    ADD CONSTRAINT human_users_email_normalized_0008_check
        CHECK (
            email_normalized IS NULL
            OR (
                length(email_normalized) BETWEEN 3 AND 320
                AND email_normalized = lower(email_normalized)
                AND position('@' IN email_normalized) > 1
                AND email_normalized !~ '[[:space:][:cntrl:]]'
            )
        ),
    ADD CONSTRAINT human_users_verified_email_0008_check
        CHECK (email_verified_at IS NULL OR email_normalized IS NOT NULL),
    ADD CONSTRAINT human_users_self_service_email_0008_check
        CHECK (
            identity_origin <> 'self_service'
            OR (email_normalized IS NOT NULL AND email_verified_at IS NOT NULL)
        );

CREATE UNIQUE INDEX human_users_email_normalized_0008_idx
    ON human_users(email_normalized)
    WHERE email_normalized IS NOT NULL;

CREATE TABLE catalogue_models (
    id                TEXT PRIMARY KEY,
    slug              TEXT NOT NULL UNIQUE,
    name              TEXT NOT NULL,
    family            TEXT NOT NULL,
    description       TEXT NOT NULL DEFAULT '',
    modalities        JSONB NOT NULL DEFAULT '[]'::jsonb,
    capabilities      JSONB NOT NULL DEFAULT '[]'::jsonb,
    lifecycle_status  TEXT NOT NULL DEFAULT 'draft',
    published_at      TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (slug ~ '^[a-z0-9][a-z0-9-]{0,62}$'),
    CHECK (length(name) BETWEEN 1 AND 255),
    CHECK (length(family) BETWEEN 1 AND 255),
    CHECK (length(description) <= 4000),
    CHECK (jsonb_typeof(modalities) = 'array'),
    CHECK (jsonb_typeof(capabilities) = 'array'),
    CHECK (lifecycle_status IN ('draft', 'published', 'deprecated', 'retired')),
    CHECK (
        (lifecycle_status = 'draft' AND published_at IS NULL)
        OR (lifecycle_status <> 'draft' AND published_at IS NOT NULL)
    )
);

CREATE TABLE catalogue_model_versions (
    id                     TEXT PRIMARY KEY,
    catalogue_model_id     TEXT NOT NULL REFERENCES catalogue_models(id) ON DELETE RESTRICT,
    version                TEXT NOT NULL,
    artifact_digest        TEXT,
    routable_model_id      TEXT REFERENCES models(id) ON DELETE RESTRICT,
    context_window_tokens  BIGINT,
    licence_name           TEXT NOT NULL,
    licence_ref            TEXT,
    licence_status         TEXT NOT NULL DEFAULT 'pending',
    support_status         TEXT NOT NULL DEFAULT 'reviewing',
    lifecycle_status       TEXT NOT NULL DEFAULT 'draft',
    source_label           TEXT NOT NULL,
    evidence_ref           TEXT,
    published_at           TIMESTAMPTZ,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (catalogue_model_id, version),
    UNIQUE (catalogue_model_id, id),
    CHECK (version ~ '^[A-Za-z0-9][A-Za-z0-9._:+-]{0,126}[A-Za-z0-9]$' OR version ~ '^[A-Za-z0-9]$'),
    CHECK (artifact_digest IS NULL OR artifact_digest ~ '^(sha256:)?[0-9a-f]{64}$'),
    CHECK (context_window_tokens IS NULL OR context_window_tokens > 0),
    CHECK (length(licence_name) BETWEEN 1 AND 255),
    CHECK (licence_ref IS NULL OR length(licence_ref) <= 1000),
    CHECK (licence_status IN ('pending', 'approved', 'restricted', 'rejected')),
    CHECK (support_status IN ('reviewing', 'supported', 'limited', 'unsupported')),
    CHECK (lifecycle_status IN ('draft', 'available', 'deprecated', 'retired')),
    CHECK (source_label ~ '^[A-Za-z0-9][A-Za-z0-9 ._:/-]{0,254}$'),
    CHECK (evidence_ref IS NULL OR (length(evidence_ref) <= 1000 AND evidence_ref ~ '^[A-Za-z0-9][A-Za-z0-9 ._:/-]*$')),
    CHECK (
        (lifecycle_status = 'draft' AND published_at IS NULL)
        OR
        (lifecycle_status <> 'draft'
         AND published_at IS NOT NULL
         AND licence_status IN ('approved', 'restricted'))
    )
);

CREATE TABLE deployment_profiles (
    id                      TEXT PRIMARY KEY,
    catalogue_model_version_id TEXT NOT NULL REFERENCES catalogue_model_versions(id) ON DELETE RESTRICT,
    code                    TEXT NOT NULL,
    name                    TEXT NOT NULL,
    service_mode            TEXT NOT NULL,
    execution_class         TEXT NOT NULL,
    runtime_class           TEXT NOT NULL,
    accelerator_class       TEXT,
    accelerators_per_unit   INTEGER,
    accelerator_memory_gib  NUMERIC(10,2),
    min_capacity_units      INTEGER NOT NULL DEFAULT 1,
    max_capacity_units      INTEGER NOT NULL DEFAULT 1,
    capacity_finality       TEXT NOT NULL DEFAULT 'unknown',
    status                  TEXT NOT NULL DEFAULT 'draft',
    source_label            TEXT NOT NULL,
    evidence_ref            TEXT,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (catalogue_model_version_id, code),
    CHECK (code ~ '^[a-z0-9][a-z0-9-]{0,62}$'),
    CHECK (length(name) BETWEEN 1 AND 255),
    CHECK (service_mode IN ('shared_evaluation', 'dedicated_private', 'customer_site')),
    CHECK (execution_class IN ('external_pilot', 'private_compatible', 'meluxina', 'customer_site')),
    CHECK (runtime_class ~ '^[A-Za-z0-9][A-Za-z0-9 ._:/+-]{0,254}$'),
    CHECK (accelerator_class IS NULL OR accelerator_class ~ '^[A-Za-z0-9][A-Za-z0-9 ._:/+-]{0,254}$'),
    CHECK (accelerators_per_unit IS NULL OR accelerators_per_unit > 0),
    CHECK (accelerator_memory_gib IS NULL OR accelerator_memory_gib > 0),
    CHECK (min_capacity_units > 0 AND max_capacity_units >= min_capacity_units),
    CHECK (capacity_finality IN ('unknown', 'estimated', 'measured', 'contractual')),
    CHECK (status IN ('draft', 'quotable', 'unavailable', 'retired')),
    CHECK (source_label ~ '^[A-Za-z0-9][A-Za-z0-9 ._:/-]{0,254}$'),
    CHECK (evidence_ref IS NULL OR (length(evidence_ref) <= 1000 AND evidence_ref ~ '^[A-Za-z0-9][A-Za-z0-9 ._:/-]*$')),
    CHECK (
        (service_mode = 'shared_evaluation'
         AND execution_class IN ('external_pilot', 'private_compatible', 'meluxina'))
        OR
        (service_mode = 'dedicated_private'
         AND execution_class IN ('private_compatible', 'meluxina')
         AND accelerator_class IS NOT NULL
         AND accelerators_per_unit IS NOT NULL)
        OR
        (service_mode = 'customer_site'
         AND execution_class = 'customer_site'
         AND accelerator_class IS NOT NULL
         AND accelerators_per_unit IS NOT NULL)
    ),
    CHECK (
        status <> 'quotable'
        OR (capacity_finality <> 'unknown' AND evidence_ref IS NOT NULL)
    )
);

CREATE TABLE deployment_profile_metrics (
    id                    TEXT PRIMARY KEY,
    deployment_profile_id TEXT NOT NULL REFERENCES deployment_profiles(id) ON DELETE RESTRICT,
    metric_code           TEXT NOT NULL,
    unit                  TEXT NOT NULL,
    minimum_value         NUMERIC,
    target_value          NUMERIC,
    maximum_value         NUMERIC,
    per_capacity_unit     BOOLEAN NOT NULL DEFAULT true,
    scales_with_units     BOOLEAN NOT NULL DEFAULT false,
    finality              TEXT NOT NULL,
    source_label          TEXT NOT NULL,
    evidence_ref          TEXT,
    measured_at           TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (deployment_profile_id, metric_code),
    CHECK (metric_code ~ '^[a-z][a-z0-9_]{1,62}$'),
    CHECK (unit ~ '^[A-Za-z0-9][A-Za-z0-9 ._/%-]{0,126}$'),
    CHECK (minimum_value IS NOT NULL OR target_value IS NOT NULL OR maximum_value IS NOT NULL),
    CHECK (minimum_value IS NULL OR minimum_value >= 0),
    CHECK (target_value IS NULL OR target_value >= 0),
    CHECK (maximum_value IS NULL OR maximum_value >= 0),
    CHECK (minimum_value IS NULL OR target_value IS NULL OR minimum_value <= target_value),
    CHECK (target_value IS NULL OR maximum_value IS NULL OR target_value <= maximum_value),
    CHECK (minimum_value IS NULL OR maximum_value IS NULL OR minimum_value <= maximum_value),
    CHECK (NOT scales_with_units OR per_capacity_unit),
    CHECK (finality IN ('estimated', 'measured', 'contractual')),
    CHECK (source_label ~ '^[A-Za-z0-9][A-Za-z0-9 ._:/-]{0,254}$'),
    CHECK (evidence_ref IS NULL OR (length(evidence_ref) <= 1000 AND evidence_ref ~ '^[A-Za-z0-9][A-Za-z0-9 ._:/-]*$')),
    CHECK (finality = 'estimated' OR evidence_ref IS NOT NULL),
    CHECK (finality <> 'measured' OR measured_at IS NOT NULL)
);

CREATE TABLE deployment_profile_prices (
    id                          TEXT PRIMARY KEY,
    deployment_profile_id       TEXT NOT NULL REFERENCES deployment_profiles(id) ON DELETE RESTRICT,
    currency                    TEXT NOT NULL,
    billing_period              TEXT NOT NULL,
    recurring_unit_amount_minor BIGINT NOT NULL,
    setup_amount_minor          BIGINT NOT NULL DEFAULT 0,
    visibility                  TEXT NOT NULL DEFAULT 'authenticated',
    finality                    TEXT NOT NULL DEFAULT 'indicative',
    source_label                TEXT NOT NULL,
    evidence_ref                TEXT,
    effective_from              TIMESTAMPTZ NOT NULL,
    effective_to                TIMESTAMPTZ,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (deployment_profile_id, currency, billing_period, effective_from),
    CHECK (currency ~ '^[A-Z]{3}$'),
    CHECK (billing_period IN ('hour', 'month', 'contract_term')),
    CHECK (recurring_unit_amount_minor >= 0),
    CHECK (setup_amount_minor >= 0),
    CHECK (visibility IN ('public', 'authenticated', 'operator')),
    CHECK (finality IN ('indicative', 'contractual')),
    CHECK (source_label ~ '^[A-Za-z0-9][A-Za-z0-9 ._:/-]{0,254}$'),
    CHECK (evidence_ref IS NULL OR (length(evidence_ref) <= 1000 AND evidence_ref ~ '^[A-Za-z0-9][A-Za-z0-9 ._:/-]*$')),
    CHECK (finality <> 'contractual' OR evidence_ref IS NOT NULL),
    CHECK (effective_to IS NULL OR effective_to > effective_from)
);

CREATE TABLE evaluation_offer_templates (
    id                              TEXT PRIMARY KEY,
    code                            TEXT NOT NULL UNIQUE,
    name                            TEXT NOT NULL,
    deployment_profile_id           TEXT NOT NULL REFERENCES deployment_profiles(id) ON DELETE RESTRICT,
    routable_model_id               TEXT NOT NULL REFERENCES models(id) ON DELETE RESTRICT,
    target_id                       TEXT NOT NULL REFERENCES inference_targets(id) ON DELETE RESTRICT,
    status                          TEXT NOT NULL DEFAULT 'disabled',
    is_default                      BOOLEAN NOT NULL DEFAULT false,
    request_allowance               BIGINT NOT NULL,
    token_allowance                 BIGINT,
    rate_limit_requests_per_minute  INTEGER NOT NULL,
    concurrency_limit               INTEGER NOT NULL,
    expires_after_days              INTEGER NOT NULL,
    privacy_notice_version          TEXT NOT NULL,
    acceptable_use_version          TEXT NOT NULL,
    source_label                    TEXT NOT NULL,
    evidence_ref                    TEXT,
    created_at                      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (code ~ '^[a-z0-9][a-z0-9-]{0,62}$'),
    CHECK (length(name) BETWEEN 1 AND 255),
    CHECK (status IN ('disabled', 'enabled', 'retired')),
    CHECK (NOT is_default OR status = 'enabled'),
    CHECK (request_allowance > 0),
    CHECK (token_allowance IS NULL OR token_allowance > 0),
    CHECK (rate_limit_requests_per_minute > 0),
    CHECK (concurrency_limit > 0),
    CHECK (expires_after_days BETWEEN 1 AND 365),
    CHECK (length(privacy_notice_version) BETWEEN 1 AND 255),
    CHECK (length(acceptable_use_version) BETWEEN 1 AND 255),
    CHECK (source_label ~ '^[A-Za-z0-9][A-Za-z0-9 ._:/-]{0,254}$'),
    CHECK (evidence_ref IS NULL OR (length(evidence_ref) <= 1000 AND evidence_ref ~ '^[A-Za-z0-9][A-Za-z0-9 ._:/-]*$')),
    CHECK (status <> 'enabled' OR evidence_ref IS NOT NULL)
);

CREATE UNIQUE INDEX evaluation_offer_templates_one_default_0008_idx
    ON evaluation_offer_templates(is_default)
    WHERE is_default AND status = 'enabled';

CREATE TABLE self_service_registrations (
    id                              TEXT PRIMARY KEY,
    email                           TEXT NOT NULL,
    email_normalized                TEXT NOT NULL,
    proposed_display_name           TEXT NOT NULL,
    proposed_organisation_name      TEXT NOT NULL,
    privacy_notice_version          TEXT NOT NULL,
    acceptable_use_version          TEXT NOT NULL,
    accepted_at                     TIMESTAMPTZ NOT NULL,
    status                          TEXT NOT NULL DEFAULT 'pending_verification',
    verification_token_hash         BYTEA NOT NULL UNIQUE,
    token_generation                INTEGER NOT NULL DEFAULT 1,
    expires_at                      TIMESTAMPTZ NOT NULL,
    verified_at                     TIMESTAMPTZ,
    completed_at                    TIMESTAMPTZ,
    completed_user_id               TEXT REFERENCES human_users(id) ON DELETE RESTRICT,
    completed_organisation_id       TEXT REFERENCES organisations(id) ON DELETE RESTRICT,
    completed_offer_id              TEXT REFERENCES evaluation_offer_templates(id) ON DELETE RESTRICT,
    safe_failure_class              TEXT,
    created_at                      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (length(email) BETWEEN 3 AND 320),
    CHECK (email_normalized = lower(email_normalized)),
    CHECK (email_normalized !~ '[[:space:][:cntrl:]]'),
    CHECK (position('@' IN email_normalized) > 1),
    CHECK (length(proposed_display_name) BETWEEN 1 AND 255),
    CHECK (length(proposed_organisation_name) BETWEEN 1 AND 255),
    CHECK (length(privacy_notice_version) BETWEEN 1 AND 255),
    CHECK (length(acceptable_use_version) BETWEEN 1 AND 255),
    CHECK (status IN ('pending_verification', 'verified', 'completed', 'expired', 'blocked', 'superseded')),
    CHECK (octet_length(verification_token_hash) = 32),
    CHECK (token_generation > 0),
    CHECK (expires_at > created_at),
    CHECK (safe_failure_class IS NULL OR safe_failure_class ~ '^[a-z][a-z0-9_]{1,62}$'),
    CHECK (
        (status = 'pending_verification'
         AND verified_at IS NULL
         AND completed_at IS NULL
         AND completed_user_id IS NULL
         AND completed_organisation_id IS NULL
         AND completed_offer_id IS NULL)
        OR
        (status = 'verified'
         AND verified_at IS NOT NULL
         AND completed_at IS NULL
         AND completed_user_id IS NULL
         AND completed_organisation_id IS NULL
         AND completed_offer_id IS NULL)
        OR
        (status = 'completed'
         AND verified_at IS NOT NULL
         AND completed_at IS NOT NULL
         AND completed_user_id IS NOT NULL
         AND completed_organisation_id IS NOT NULL
         AND completed_offer_id IS NOT NULL)
        OR
        (status IN ('expired', 'blocked', 'superseded')
         AND completed_at IS NULL
         AND completed_user_id IS NULL
         AND completed_organisation_id IS NULL
         AND completed_offer_id IS NULL)
    )
);

CREATE UNIQUE INDEX self_service_registrations_one_active_email_0008_idx
    ON self_service_registrations(email_normalized)
    WHERE status IN ('pending_verification', 'verified');

CREATE UNIQUE INDEX self_service_registrations_completed_user_0008_idx
    ON self_service_registrations(completed_user_id)
    WHERE completed_user_id IS NOT NULL;

CREATE UNIQUE INDEX self_service_registrations_completed_org_0008_idx
    ON self_service_registrations(completed_organisation_id)
    WHERE completed_organisation_id IS NOT NULL;

CREATE TABLE business_qualification_requests (
    id                    TEXT PRIMARY KEY,
    organisation_id       TEXT NOT NULL REFERENCES organisations(id) ON DELETE RESTRICT,
    submitted_by_user_id  TEXT NOT NULL REFERENCES human_users(id) ON DELETE RESTRICT,
    legal_name            TEXT NOT NULL,
    applicant_title       TEXT NOT NULL,
    website               TEXT,
    workload_summary      TEXT NOT NULL,
    status                TEXT NOT NULL DEFAULT 'draft',
    submitted_at          TIMESTAMPTZ,
    reviewed_at           TIMESTAMPTZ,
    reviewed_by           TEXT,
    review_reason         TEXT,
    review_evidence_ref   TEXT,
    withdrawn_at          TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organisation_id, id),
    CHECK (length(legal_name) BETWEEN 1 AND 255),
    CHECK (length(applicant_title) BETWEEN 1 AND 255),
    CHECK (website IS NULL OR length(website) <= 1000),
    CHECK (length(workload_summary) BETWEEN 1 AND 4000),
    CHECK (status IN ('draft', 'submitted', 'approved', 'rejected', 'withdrawn', 'expired')),
    CHECK (reviewed_by IS NULL OR reviewed_by ~ '^[A-Za-z0-9][A-Za-z0-9 ._:@/-]{0,254}$'),
    CHECK (review_reason IS NULL OR length(review_reason) <= 2000),
    CHECK (review_evidence_ref IS NULL OR (length(review_evidence_ref) <= 1000 AND review_evidence_ref ~ '^[A-Za-z0-9][A-Za-z0-9 ._:/-]*$')),
    CHECK (status = 'draft' OR submitted_at IS NOT NULL),
    CHECK (
        status NOT IN ('approved', 'rejected')
        OR (reviewed_at IS NOT NULL AND reviewed_by IS NOT NULL AND review_reason IS NOT NULL)
    ),
    CHECK (status <> 'approved' OR review_evidence_ref IS NOT NULL),
    CHECK ((status = 'withdrawn') = (withdrawn_at IS NOT NULL))
);

CREATE UNIQUE INDEX business_qualification_requests_one_active_0008_idx
    ON business_qualification_requests(organisation_id)
    WHERE status IN ('draft', 'submitted');

CREATE TABLE deployment_quotes (
    id                             TEXT PRIMARY KEY,
    organisation_id                TEXT NOT NULL,
    project_id                     TEXT NOT NULL,
    environment_id                 TEXT NOT NULL,
    quote_version                  INTEGER NOT NULL DEFAULT 1,
    quote_kind                     TEXT NOT NULL,
    deployment_profile_id          TEXT NOT NULL REFERENCES deployment_profiles(id) ON DELETE RESTRICT,
    profile_price_id               TEXT REFERENCES deployment_profile_prices(id) ON DELETE RESTRICT,
    capacity_units                 INTEGER NOT NULL,
    service_mode_snapshot          TEXT NOT NULL,
    execution_class_snapshot       TEXT NOT NULL,
    accelerator_class_snapshot     TEXT,
    accelerator_count_snapshot     INTEGER,
    capacity_snapshot              JSONB NOT NULL,
    currency                       TEXT NOT NULL,
    billing_period                 TEXT NOT NULL,
    recurring_unit_amount_minor    BIGINT NOT NULL,
    recurring_total_amount_minor   BIGINT NOT NULL,
    setup_total_amount_minor       BIGINT NOT NULL DEFAULT 0,
    tax_treatment                  TEXT NOT NULL DEFAULT 'not_determined',
    price_finality                 TEXT NOT NULL,
    status                         TEXT NOT NULL DEFAULT 'offered',
    source_label                   TEXT NOT NULL,
    evidence_ref                   TEXT,
    offered_at                     TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at                     TIMESTAMPTZ NOT NULL,
    accepted_at                    TIMESTAMPTZ,
    accepted_by_user_id            TEXT REFERENCES human_users(id) ON DELETE RESTRICT,
    created_at                     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                     TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (organisation_id, project_id, environment_id)
        REFERENCES environments(organisation_id, project_id, id) ON DELETE RESTRICT,
    UNIQUE (organisation_id, project_id, environment_id, id),
    CHECK (quote_version > 0),
    CHECK (quote_kind IN ('new_endpoint', 'scale_up', 'scale_down')),
    CHECK (capacity_units > 0),
    CHECK (service_mode_snapshot IN ('shared_evaluation', 'dedicated_private', 'customer_site')),
    CHECK (execution_class_snapshot IN ('external_pilot', 'private_compatible', 'meluxina', 'customer_site')),
    CHECK (accelerator_class_snapshot IS NULL OR accelerator_class_snapshot ~ '^[A-Za-z0-9][A-Za-z0-9 ._:/+-]{0,254}$'),
    CHECK (accelerator_count_snapshot IS NULL OR accelerator_count_snapshot > 0),
    CHECK (jsonb_typeof(capacity_snapshot) = 'object'),
    CHECK (currency ~ '^[A-Z]{3}$'),
    CHECK (billing_period IN ('hour', 'month', 'contract_term')),
    CHECK (recurring_unit_amount_minor >= 0),
    CHECK (recurring_total_amount_minor >= 0),
    CHECK (setup_total_amount_minor >= 0),
    CHECK (tax_treatment IN ('not_determined', 'exclusive', 'inclusive', 'not_applicable')),
    CHECK (price_finality IN ('indicative', 'contractual')),
    CHECK (status IN ('offered', 'accepted', 'rejected', 'expired', 'superseded', 'cancelled')),
    CHECK (source_label ~ '^[A-Za-z0-9][A-Za-z0-9 ._:/-]{0,254}$'),
    CHECK (evidence_ref IS NULL OR (length(evidence_ref) <= 1000 AND evidence_ref ~ '^[A-Za-z0-9][A-Za-z0-9 ._:/-]*$')),
    CHECK (expires_at > offered_at),
    CHECK (
        (status = 'accepted'
         AND accepted_at IS NOT NULL
         AND accepted_by_user_id IS NOT NULL
         AND price_finality = 'contractual'
         AND evidence_ref IS NOT NULL)
        OR
        (status <> 'accepted' AND accepted_at IS NULL AND accepted_by_user_id IS NULL)
    )
);

CREATE INDEX deployment_quotes_scope_status_0008_idx
    ON deployment_quotes(organisation_id, project_id, environment_id, status, offered_at DESC);

CREATE TABLE model_deployments (
    id                    TEXT PRIMARY KEY,
    organisation_id       TEXT NOT NULL,
    project_id            TEXT NOT NULL,
    environment_id        TEXT NOT NULL,
    deployment_profile_id TEXT NOT NULL REFERENCES deployment_profiles(id) ON DELETE RESTRICT,
    quote_id              TEXT NOT NULL,
    target_id             TEXT REFERENCES inference_targets(id) ON DELETE RESTRICT,
    route_id              TEXT,
    state                 TEXT NOT NULL DEFAULT 'requested',
    validation_evidence_ref TEXT,
    last_verified_at      TIMESTAMPTZ,
    ready_at              TIMESTAMPTZ,
    safe_error_class      TEXT,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (organisation_id, project_id, environment_id)
        REFERENCES environments(organisation_id, project_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (organisation_id, project_id, environment_id, quote_id)
        REFERENCES deployment_quotes(organisation_id, project_id, environment_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (organisation_id, project_id, environment_id, route_id)
        REFERENCES tenant_routes(organisation_id, project_id, environment_id, id) ON DELETE RESTRICT,
    UNIQUE (organisation_id, project_id, environment_id, id),
    CHECK (state IN ('requested', 'allocating', 'deploying', 'validating', 'ready', 'degraded', 'failed', 'retired')),
    CHECK (validation_evidence_ref IS NULL OR (length(validation_evidence_ref) <= 1000 AND validation_evidence_ref ~ '^[A-Za-z0-9][A-Za-z0-9 ._:/-]*$')),
    CHECK (safe_error_class IS NULL OR safe_error_class ~ '^[a-z][a-z0-9_]{1,62}$'),
    CHECK (
        state NOT IN ('ready', 'degraded')
        OR (
            target_id IS NOT NULL
            AND route_id IS NOT NULL
            AND validation_evidence_ref IS NOT NULL
            AND last_verified_at IS NOT NULL
            AND ready_at IS NOT NULL
        )
    )
);

CREATE UNIQUE INDEX model_deployments_one_current_route_0008_idx
    ON model_deployments(route_id)
    WHERE route_id IS NOT NULL AND state IN ('ready', 'degraded');

CREATE TABLE deployment_requests (
    id                       TEXT PRIMARY KEY,
    organisation_id          TEXT NOT NULL,
    project_id               TEXT NOT NULL,
    environment_id           TEXT NOT NULL,
    request_kind             TEXT NOT NULL,
    deployment_profile_id    TEXT NOT NULL REFERENCES deployment_profiles(id) ON DELETE RESTRICT,
    deployment_id            TEXT,
    quote_id                 TEXT,
    current_capacity_units   INTEGER,
    requested_capacity_units INTEGER NOT NULL,
    status                   TEXT NOT NULL DEFAULT 'draft',
    requested_by_user_id     TEXT NOT NULL REFERENCES human_users(id) ON DELETE RESTRICT,
    reviewed_by              TEXT,
    safe_failure_class       TEXT,
    submitted_at             TIMESTAMPTZ,
    approved_at              TIMESTAMPTZ,
    completed_at             TIMESTAMPTZ,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (organisation_id, project_id, environment_id)
        REFERENCES environments(organisation_id, project_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (organisation_id, project_id, environment_id, deployment_id)
        REFERENCES model_deployments(organisation_id, project_id, environment_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (organisation_id, project_id, environment_id, quote_id)
        REFERENCES deployment_quotes(organisation_id, project_id, environment_id, id) ON DELETE RESTRICT,
    UNIQUE (organisation_id, project_id, environment_id, id),
    CHECK (request_kind IN ('new_endpoint', 'scale_up', 'scale_down')),
    CHECK (requested_capacity_units > 0),
    CHECK (current_capacity_units IS NULL OR current_capacity_units > 0),
    CHECK (status IN ('draft', 'submitted', 'quoted', 'accepted', 'approved', 'allocating', 'deploying', 'validating', 'ready', 'rejected', 'cancelled', 'failed')),
    CHECK (reviewed_by IS NULL OR reviewed_by ~ '^[A-Za-z0-9][A-Za-z0-9 ._:@/-]{0,254}$'),
    CHECK (safe_failure_class IS NULL OR safe_failure_class ~ '^[a-z][a-z0-9_]{1,62}$'),
    CHECK (status = 'draft' OR submitted_at IS NOT NULL),
    CHECK (status <> 'approved' OR (approved_at IS NOT NULL AND reviewed_by IS NOT NULL)),
    CHECK (status <> 'ready' OR completed_at IS NOT NULL),
    CHECK (
        (request_kind = 'new_endpoint'
         AND deployment_id IS NULL
         AND current_capacity_units IS NULL)
        OR
        (request_kind = 'scale_up'
         AND deployment_id IS NOT NULL
         AND current_capacity_units IS NOT NULL
         AND requested_capacity_units > current_capacity_units)
        OR
        (request_kind = 'scale_down'
         AND deployment_id IS NOT NULL
         AND current_capacity_units IS NOT NULL
         AND requested_capacity_units < current_capacity_units)
    ),
    CHECK (
        status NOT IN ('accepted', 'approved', 'allocating', 'deploying', 'validating', 'ready')
        OR quote_id IS NOT NULL
    )
);

CREATE INDEX deployment_requests_scope_status_0008_idx
    ON deployment_requests(organisation_id, project_id, environment_id, status, created_at DESC);

CREATE UNIQUE INDEX deployment_requests_one_active_change_0008_idx
    ON deployment_requests(deployment_id)
    WHERE deployment_id IS NOT NULL
      AND status IN ('submitted', 'quoted', 'accepted', 'approved', 'allocating', 'deploying', 'validating');

CREATE TABLE deployment_capacity_revisions (
    id                    TEXT PRIMARY KEY,
    organisation_id       TEXT NOT NULL,
    project_id            TEXT NOT NULL,
    environment_id        TEXT NOT NULL,
    deployment_id         TEXT NOT NULL,
    quote_id              TEXT NOT NULL,
    capacity_units        INTEGER NOT NULL,
    state                 TEXT NOT NULL DEFAULT 'pending',
    resource_evidence_ref TEXT,
    effective_at          TIMESTAMPTZ,
    ended_at              TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (organisation_id, project_id, environment_id, deployment_id)
        REFERENCES model_deployments(organisation_id, project_id, environment_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (organisation_id, project_id, environment_id, quote_id)
        REFERENCES deployment_quotes(organisation_id, project_id, environment_id, id) ON DELETE RESTRICT,
    UNIQUE (organisation_id, project_id, environment_id, id),
    CHECK (capacity_units > 0),
    CHECK (state IN ('pending', 'active', 'superseded', 'cancelled', 'failed')),
    CHECK (resource_evidence_ref IS NULL OR (length(resource_evidence_ref) <= 1000 AND resource_evidence_ref ~ '^[A-Za-z0-9][A-Za-z0-9 ._:/-]*$')),
    CHECK (
        (state = 'active'
         AND resource_evidence_ref IS NOT NULL
         AND effective_at IS NOT NULL
         AND ended_at IS NULL)
        OR
        (state = 'superseded'
         AND resource_evidence_ref IS NOT NULL
         AND effective_at IS NOT NULL
         AND ended_at IS NOT NULL
         AND ended_at >= effective_at)
        OR
        (state IN ('pending', 'cancelled', 'failed') AND ended_at IS NULL)
    )
);

CREATE UNIQUE INDEX deployment_capacity_revisions_one_active_0008_idx
    ON deployment_capacity_revisions(deployment_id)
    WHERE state = 'active';

CREATE OR REPLACE FUNCTION enforce_evaluation_offer_0008() RETURNS trigger AS $$
DECLARE
    profile_mode TEXT;
    profile_status TEXT;
    mapped_model_id TEXT;
    target_mode TEXT;
    target_owner TEXT;
    target_enabled BOOLEAN;
BEGIN
    SELECT p.service_mode, p.status, v.routable_model_id
      INTO profile_mode, profile_status, mapped_model_id
      FROM deployment_profiles p
      JOIN catalogue_model_versions v ON v.id = p.catalogue_model_version_id
     WHERE p.id = NEW.deployment_profile_id;

    IF profile_mode IS DISTINCT FROM 'shared_evaluation' OR profile_status IS DISTINCT FROM 'quotable' THEN
        RAISE EXCEPTION 'evaluation offer requires a quotable shared-evaluation profile';
    END IF;
    IF mapped_model_id IS NULL OR mapped_model_id IS DISTINCT FROM NEW.routable_model_id THEN
        RAISE EXCEPTION 'evaluation offer model does not match its catalogue version';
    END IF;

    SELECT capacity_mode, owner_organisation_id, enabled
      INTO target_mode, target_owner, target_enabled
      FROM inference_targets
     WHERE id = NEW.target_id;
    IF target_mode IS DISTINCT FROM 'shared' OR target_owner IS NOT NULL THEN
        RAISE EXCEPTION 'evaluation offer target must be shared';
    END IF;
    IF NEW.status = 'enabled' AND NOT target_enabled THEN
        RAISE EXCEPTION 'enabled evaluation offer requires an enabled target';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER evaluation_offer_templates_enforce_0008
BEFORE INSERT OR UPDATE ON evaluation_offer_templates
FOR EACH ROW EXECUTE FUNCTION enforce_evaluation_offer_0008();

CREATE OR REPLACE FUNCTION protect_evaluation_offer_target_0008() RETURNS trigger AS $$
BEGIN
    IF (NEW.capacity_mode IS DISTINCT FROM OLD.capacity_mode
        OR NEW.owner_organisation_id IS DISTINCT FROM OLD.owner_organisation_id)
       AND EXISTS (
           SELECT 1
             FROM evaluation_offer_templates o
            WHERE o.target_id = OLD.id AND o.status = 'enabled'
       )
       AND (NEW.capacity_mode <> 'shared' OR NEW.owner_organisation_id IS NOT NULL) THEN
        RAISE EXCEPTION 'enabled evaluation offer target must remain shared';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER inference_targets_protect_evaluation_offer_0008
BEFORE UPDATE OF capacity_mode, owner_organisation_id ON inference_targets
FOR EACH ROW EXECUTE FUNCTION protect_evaluation_offer_target_0008();

CREATE OR REPLACE FUNCTION enforce_deployment_quote_0008() RETURNS trigger AS $$
DECLARE
    profile_mode TEXT;
    profile_execution TEXT;
    profile_accelerator TEXT;
    profile_accelerators_per_unit INTEGER;
    profile_min_units INTEGER;
    profile_max_units INTEGER;
    profile_status TEXT;
    profile_capacity_finality TEXT;
    listed_profile_id TEXT;
    listed_currency TEXT;
    listed_period TEXT;
BEGIN
    SELECT service_mode, execution_class, accelerator_class,
           accelerators_per_unit, min_capacity_units, max_capacity_units,
           status, capacity_finality
      INTO profile_mode, profile_execution, profile_accelerator,
           profile_accelerators_per_unit, profile_min_units, profile_max_units,
           profile_status, profile_capacity_finality
      FROM deployment_profiles
     WHERE id = NEW.deployment_profile_id;

    IF profile_status IS DISTINCT FROM 'quotable' THEN
        RAISE EXCEPTION 'deployment quote requires a quotable profile';
    END IF;
    IF NEW.capacity_units < profile_min_units OR NEW.capacity_units > profile_max_units THEN
        RAISE EXCEPTION 'quoted capacity units are outside the profile range';
    END IF;
    IF NEW.service_mode_snapshot IS DISTINCT FROM profile_mode
       OR NEW.execution_class_snapshot IS DISTINCT FROM profile_execution THEN
        RAISE EXCEPTION 'quote service mode or execution class does not match profile';
    END IF;
    IF profile_mode IN ('dedicated_private', 'customer_site') THEN
        IF NEW.accelerator_class_snapshot IS DISTINCT FROM profile_accelerator
           OR NEW.accelerator_count_snapshot IS DISTINCT FROM profile_accelerators_per_unit * NEW.capacity_units THEN
            RAISE EXCEPTION 'quote accelerator snapshot does not match profile units';
        END IF;
        IF profile_capacity_finality = 'unknown'
           OR NOT EXISTS (
               SELECT 1 FROM deployment_profile_metrics m
                WHERE m.deployment_profile_id = NEW.deployment_profile_id
           ) THEN
            RAISE EXCEPTION 'dedicated quote requires evidenced capacity metrics';
        END IF;
    ELSIF NEW.accelerator_count_snapshot IS NOT NULL THEN
        RAISE EXCEPTION 'shared evaluation quote cannot claim dedicated accelerator count';
    END IF;

    IF NEW.profile_price_id IS NOT NULL THEN
        SELECT deployment_profile_id, currency, billing_period
          INTO listed_profile_id, listed_currency, listed_period
          FROM deployment_profile_prices
         WHERE id = NEW.profile_price_id;
        IF listed_profile_id IS DISTINCT FROM NEW.deployment_profile_id
           OR listed_currency IS DISTINCT FROM NEW.currency
           OR listed_period IS DISTINCT FROM NEW.billing_period THEN
            RAISE EXCEPTION 'quote price reference does not match profile/currency/period';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER deployment_quotes_enforce_0008
BEFORE INSERT OR UPDATE ON deployment_quotes
FOR EACH ROW EXECUTE FUNCTION enforce_deployment_quote_0008();

CREATE OR REPLACE FUNCTION protect_accepted_deployment_quote_0008() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        IF OLD.status = 'accepted' THEN
            RAISE EXCEPTION 'accepted deployment quote is immutable';
        END IF;
        RETURN OLD;
    END IF;
    IF OLD.status = 'accepted' THEN
        RAISE EXCEPTION 'accepted deployment quote is immutable';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER deployment_quotes_protect_accepted_update_0008
BEFORE UPDATE ON deployment_quotes
FOR EACH ROW EXECUTE FUNCTION protect_accepted_deployment_quote_0008();

CREATE TRIGGER deployment_quotes_protect_accepted_delete_0008
BEFORE DELETE ON deployment_quotes
FOR EACH ROW EXECUTE FUNCTION protect_accepted_deployment_quote_0008();

CREATE OR REPLACE FUNCTION enforce_model_deployment_binding_0008() RETURNS trigger AS $$
DECLARE
    profile_mode TEXT;
    profile_execution TEXT;
    mapped_model_id TEXT;
    quote_status TEXT;
    quote_profile_id TEXT;
    target_mode TEXT;
    target_execution TEXT;
    target_owner TEXT;
    route_target_id TEXT;
    route_model_id TEXT;
BEGIN
    SELECT p.service_mode, p.execution_class, v.routable_model_id
      INTO profile_mode, profile_execution, mapped_model_id
      FROM deployment_profiles p
      JOIN catalogue_model_versions v ON v.id = p.catalogue_model_version_id
     WHERE p.id = NEW.deployment_profile_id;

    SELECT status, deployment_profile_id
      INTO quote_status, quote_profile_id
      FROM deployment_quotes
     WHERE organisation_id = NEW.organisation_id
       AND project_id = NEW.project_id
       AND environment_id = NEW.environment_id
       AND id = NEW.quote_id;
    IF quote_status IS DISTINCT FROM 'accepted'
       OR quote_profile_id IS DISTINCT FROM NEW.deployment_profile_id THEN
        RAISE EXCEPTION 'deployment requires an accepted matching quote';
    END IF;

    IF NEW.target_id IS NOT NULL THEN
        SELECT capacity_mode, execution_class, owner_organisation_id
          INTO target_mode, target_execution, target_owner
          FROM inference_targets
         WHERE id = NEW.target_id;
        IF target_execution IS DISTINCT FROM profile_execution THEN
            RAISE EXCEPTION 'deployment target execution class does not match profile';
        END IF;
        IF profile_mode = 'shared_evaluation' THEN
            IF target_mode IS DISTINCT FROM 'shared' OR target_owner IS NOT NULL THEN
                RAISE EXCEPTION 'shared deployment requires a shared target';
            END IF;
        ELSE
            IF target_mode IS DISTINCT FROM 'dedicated'
               OR target_owner IS DISTINCT FROM NEW.organisation_id THEN
                RAISE EXCEPTION 'private deployment requires an owned dedicated target';
            END IF;
        END IF;
    END IF;

    IF NEW.route_id IS NOT NULL THEN
        IF NEW.target_id IS NULL THEN
            RAISE EXCEPTION 'deployment route requires an assigned target';
        END IF;
        SELECT target_id, model_id
          INTO route_target_id, route_model_id
          FROM tenant_routes
         WHERE organisation_id = NEW.organisation_id
           AND project_id = NEW.project_id
           AND environment_id = NEW.environment_id
           AND id = NEW.route_id;
        IF route_target_id IS DISTINCT FROM NEW.target_id THEN
            RAISE EXCEPTION 'deployment route is not bound to its target';
        END IF;
        IF mapped_model_id IS NOT NULL AND route_model_id IS DISTINCT FROM mapped_model_id THEN
            RAISE EXCEPTION 'deployment route model does not match catalogue version';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER model_deployments_enforce_binding_0008
BEFORE INSERT OR UPDATE ON model_deployments
FOR EACH ROW EXECUTE FUNCTION enforce_model_deployment_binding_0008();

CREATE OR REPLACE FUNCTION enforce_deployment_request_0008() RETURNS trigger AS $$
DECLARE
    quote_status TEXT;
    quote_kind_value TEXT;
    quote_profile_id TEXT;
    quote_units INTEGER;
    deployment_profile TEXT;
    deployment_state TEXT;
BEGIN
    IF NEW.quote_id IS NOT NULL THEN
        SELECT q.status, q.quote_kind, q.deployment_profile_id, q.capacity_units
          INTO quote_status, quote_kind_value, quote_profile_id, quote_units
          FROM deployment_quotes q
         WHERE organisation_id = NEW.organisation_id
           AND project_id = NEW.project_id
           AND environment_id = NEW.environment_id
           AND id = NEW.quote_id;
        IF quote_kind_value IS DISTINCT FROM NEW.request_kind
           OR quote_profile_id IS DISTINCT FROM NEW.deployment_profile_id
           OR quote_units IS DISTINCT FROM NEW.requested_capacity_units THEN
            RAISE EXCEPTION 'deployment request does not match its quote';
        END IF;
        IF NEW.status IN ('accepted', 'approved', 'allocating', 'deploying', 'validating', 'ready')
           AND quote_status IS DISTINCT FROM 'accepted' THEN
            RAISE EXCEPTION 'deployment request requires an accepted quote';
        END IF;
    END IF;

    IF NEW.deployment_id IS NOT NULL THEN
        SELECT deployment_profile_id, state
          INTO deployment_profile, deployment_state
          FROM model_deployments
         WHERE organisation_id = NEW.organisation_id
           AND project_id = NEW.project_id
           AND environment_id = NEW.environment_id
           AND id = NEW.deployment_id;
        IF deployment_profile IS DISTINCT FROM NEW.deployment_profile_id THEN
            RAISE EXCEPTION 'capacity change profile does not match deployment';
        END IF;
        IF NEW.status = 'ready' AND deployment_state NOT IN ('ready', 'degraded') THEN
            RAISE EXCEPTION 'capacity request cannot be ready before deployment';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER deployment_requests_enforce_0008
BEFORE INSERT OR UPDATE ON deployment_requests
FOR EACH ROW EXECUTE FUNCTION enforce_deployment_request_0008();

CREATE OR REPLACE FUNCTION enforce_capacity_revision_0008() RETURNS trigger AS $$
DECLARE
    deployment_profile TEXT;
    deployment_state TEXT;
    quote_profile TEXT;
    quote_status TEXT;
    quote_units INTEGER;
BEGIN
    SELECT deployment_profile_id, state
      INTO deployment_profile, deployment_state
      FROM model_deployments
     WHERE organisation_id = NEW.organisation_id
       AND project_id = NEW.project_id
       AND environment_id = NEW.environment_id
       AND id = NEW.deployment_id;
    SELECT deployment_profile_id, status, capacity_units
      INTO quote_profile, quote_status, quote_units
      FROM deployment_quotes
     WHERE organisation_id = NEW.organisation_id
       AND project_id = NEW.project_id
       AND environment_id = NEW.environment_id
       AND id = NEW.quote_id;

    IF quote_status IS DISTINCT FROM 'accepted'
       OR quote_profile IS DISTINCT FROM deployment_profile
       OR quote_units IS DISTINCT FROM NEW.capacity_units THEN
        RAISE EXCEPTION 'capacity revision requires an accepted matching quote';
    END IF;
    IF NEW.state = 'active' AND deployment_state NOT IN ('ready', 'degraded') THEN
        RAISE EXCEPTION 'capacity revision cannot activate before deployment is ready';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER deployment_capacity_revisions_enforce_0008
BEFORE INSERT OR UPDATE ON deployment_capacity_revisions
FOR EACH ROW EXECUTE FUNCTION enforce_capacity_revision_0008();

CREATE OR REPLACE FUNCTION protect_referenced_deployment_profile_0008() RETURNS trigger AS $$
BEGIN
    IF (
        NEW.catalogue_model_version_id IS DISTINCT FROM OLD.catalogue_model_version_id
        OR NEW.service_mode IS DISTINCT FROM OLD.service_mode
        OR NEW.execution_class IS DISTINCT FROM OLD.execution_class
        OR NEW.runtime_class IS DISTINCT FROM OLD.runtime_class
        OR NEW.accelerator_class IS DISTINCT FROM OLD.accelerator_class
        OR NEW.accelerators_per_unit IS DISTINCT FROM OLD.accelerators_per_unit
        OR NEW.accelerator_memory_gib IS DISTINCT FROM OLD.accelerator_memory_gib
        OR NEW.min_capacity_units IS DISTINCT FROM OLD.min_capacity_units
        OR NEW.max_capacity_units IS DISTINCT FROM OLD.max_capacity_units
    ) AND (
        EXISTS (SELECT 1 FROM evaluation_offer_templates o WHERE o.deployment_profile_id = OLD.id)
        OR EXISTS (SELECT 1 FROM deployment_quotes q WHERE q.deployment_profile_id = OLD.id)
        OR EXISTS (SELECT 1 FROM model_deployments d WHERE d.deployment_profile_id = OLD.id)
    ) THEN
        RAISE EXCEPTION 'referenced deployment profile structure is immutable';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER deployment_profiles_protect_references_0008
BEFORE UPDATE ON deployment_profiles
FOR EACH ROW EXECUTE FUNCTION protect_referenced_deployment_profile_0008();

COMMENT ON TABLE evaluation_offer_templates IS
    'Disabled-by-default server-owned templates for isolated hard-capped shared evaluation; rows do not enable signup by themselves.';
COMMENT ON TABLE deployment_profiles IS
    'Versioned model/runtime/hardware capacity units; customers choose profiles and units, never raw target addresses.';
COMMENT ON TABLE deployment_quotes IS
    'Customer-specific price/capacity snapshots; accepted rows are immutable commercial evidence, not deployment readiness.';
COMMENT ON TABLE model_deployments IS
    'Actual deployment lifecycle separated from catalogue, quote, request, target, route, and runtime health evidence.';
COMMENT ON TABLE deployment_capacity_revisions IS
    'Historical endpoint capacity units; adding hardware creates a new revision while the customer route remains stable.';
