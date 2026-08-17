CREATE TABLE usage_rollups_hourly_v2 (
    organisation_id          TEXT NOT NULL,
    project_id               TEXT NOT NULL,
    environment_id           TEXT NOT NULL,
    route_id                 TEXT,
    route_key                TEXT GENERATED ALWAYS AS (COALESCE(route_id, '')) STORED,
    service_account_id       TEXT NOT NULL,
    model_alias              TEXT NOT NULL,
    bucket_start             TIMESTAMPTZ NOT NULL,
    logical_requests         BIGINT NOT NULL,
    successful_requests      BIGINT NOT NULL,
    failed_requests          BIGINT NOT NULL,
    blocked_requests         BIGINT NOT NULL,
    cancelled_requests       BIGINT NOT NULL,
    in_progress_requests     BIGINT NOT NULL,
    provider_attempts        BIGINT NOT NULL,
    retried_requests         BIGINT NOT NULL,
    input_tokens             BIGINT,
    input_known_requests     BIGINT NOT NULL,
    output_tokens            BIGINT,
    output_known_requests    BIGINT NOT NULL,
    cached_tokens            BIGINT,
    cached_known_requests    BIGINT NOT NULL,
    reasoning_tokens         BIGINT,
    reasoning_known_requests BIGINT NOT NULL,
    p50_latency_ms           BIGINT,
    p95_latency_ms           BIGINT,
    peak_concurrency         BIGINT,
    source_row_count         BIGINT NOT NULL,
    source                   TEXT NOT NULL,
    finality                 TEXT NOT NULL,
    refreshed_at             TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (
        organisation_id, project_id, environment_id, route_key,
        service_account_id, model_alias, bucket_start
    ),
    FOREIGN KEY (organisation_id, project_id, environment_id)
        REFERENCES environments(organisation_id, project_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (organisation_id, project_id, environment_id, route_id)
        REFERENCES tenant_routes(organisation_id, project_id, environment_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (organisation_id, project_id, environment_id, service_account_id)
        REFERENCES service_accounts(organisation_id, project_id, environment_id, id) ON DELETE RESTRICT,
    CHECK (date_trunc('hour', bucket_start) = bucket_start),
    CHECK (logical_requests >= 0),
    CHECK (successful_requests >= 0 AND failed_requests >= 0 AND blocked_requests >= 0
           AND cancelled_requests >= 0 AND in_progress_requests >= 0),
    CHECK (logical_requests = successful_requests + failed_requests + blocked_requests
           + cancelled_requests + in_progress_requests),
    CHECK (provider_attempts >= 0 AND retried_requests >= 0),
    CHECK (retried_requests <= logical_requests),
    CHECK (provider_attempts >= 2 * retried_requests),
    CHECK (input_tokens IS NULL OR input_tokens >= 0),
    CHECK (output_tokens IS NULL OR output_tokens >= 0),
    CHECK (cached_tokens IS NULL OR cached_tokens >= 0),
    CHECK (reasoning_tokens IS NULL OR reasoning_tokens >= 0),
    CHECK (input_known_requests >= 0 AND output_known_requests >= 0
           AND cached_known_requests >= 0 AND reasoning_known_requests >= 0),
    CHECK (input_known_requests <= successful_requests
           AND output_known_requests <= successful_requests
           AND cached_known_requests <= successful_requests
           AND reasoning_known_requests <= successful_requests),
    CHECK (p50_latency_ms IS NULL OR p50_latency_ms >= 0),
    CHECK (p95_latency_ms IS NULL OR p95_latency_ms >= 0),
    CHECK (p50_latency_ms IS NULL OR p95_latency_ms IS NULL OR p95_latency_ms >= p50_latency_ms),
    CHECK (peak_concurrency IS NULL OR peak_concurrency >= 0),
    CHECK (source_row_count = logical_requests),
    CHECK (source = 'inference_requests'),
    CHECK (finality IN ('partial', 'final'))
);

CREATE INDEX usage_rollups_hourly_v2_scope_time_0006_idx
    ON usage_rollups_hourly_v2(organisation_id, project_id, environment_id, bucket_start);

ALTER TABLE inference_targets
    ADD COLUMN probe_enabled BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN probe_interval_seconds INTEGER NOT NULL DEFAULT 300,
    ADD CONSTRAINT inference_targets_probe_interval_0006_check
        CHECK (probe_interval_seconds BETWEEN 30 AND 86400);

CREATE TABLE target_health_observations (
    id                    TEXT PRIMARY KEY,
    target_id             TEXT NOT NULL REFERENCES inference_targets(id) ON DELETE RESTRICT,
    observed_at           TIMESTAMPTZ NOT NULL,
    status                TEXT NOT NULL,
    source                TEXT NOT NULL,
    credential_available  BOOLEAN NOT NULL,
    http_status           INTEGER,
    error_class           TEXT,
    latency_ms            BIGINT,
    fresh_until           TIMESTAMPTZ NOT NULL,
    CHECK (status IN ('unknown', 'operational', 'degraded', 'unavailable')),
    CHECK (source = 'opt_in_compatible_probe'),
    CHECK (http_status IS NULL OR http_status BETWEEN 100 AND 599),
    CHECK (latency_ms IS NULL OR latency_ms >= 0),
    CHECK (fresh_until > observed_at),
    CHECK (error_class IS NULL OR error_class IN (
        'credential_unavailable', 'probe_timeout', 'probe_transport',
        'probe_rejected', 'probe_unavailable', 'invalid_probe_response'
    ))
);

CREATE INDEX target_health_observations_target_time_0006_idx
    ON target_health_observations(target_id, observed_at DESC);

CREATE TABLE worker_checkpoints (
    organisation_id  TEXT NOT NULL,
    project_id       TEXT NOT NULL,
    environment_id   TEXT NOT NULL,
    worker_name       TEXT NOT NULL,
    last_started_at   TIMESTAMPTZ NOT NULL,
    last_completed_at TIMESTAMPTZ,
    status            TEXT NOT NULL,
    range_from        TIMESTAMPTZ,
    range_to          TIMESTAMPTZ,
    source_rows       BIGINT NOT NULL,
    safe_error_class  TEXT,
    PRIMARY KEY (organisation_id, project_id, environment_id, worker_name),
    FOREIGN KEY (organisation_id, project_id, environment_id)
        REFERENCES environments(organisation_id, project_id, id) ON DELETE RESTRICT,
    CHECK (worker_name IN ('usage_rollup')),
    CHECK (status IN ('running', 'succeeded', 'failed')),
    CHECK (
        (status = 'succeeded' AND last_completed_at IS NOT NULL AND last_completed_at >= last_started_at)
        OR
        (status IN ('running', 'failed') AND (last_completed_at IS NULL OR last_completed_at <= last_started_at))
    ),
    CHECK (source_rows >= 0),
    CHECK (safe_error_class IS NULL OR safe_error_class IN ('database_unavailable', 'refresh_failed'))
);

COMMENT ON TABLE usage_rollups_hourly_v2 IS
    'Reconciled from logical inference_requests; provider attempts are retry/infra evidence only.';
COMMENT ON TABLE target_health_observations IS
    'Opt-in metadata-only probes. No prompt, output, target URL, or credential may be stored.';
COMMENT ON TABLE worker_checkpoints IS
    'Safe liveness/finality evidence for zero-row rollup refreshes; never represents target health.';
