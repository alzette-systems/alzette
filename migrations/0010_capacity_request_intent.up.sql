-- Preserve the workload facts used to size and quote both initial dedicated
-- deployments and later capacity changes. The customer never supplies target
-- or hardware identifiers; this is bounded commercial intent only.
ALTER TABLE deployment_requests
    ADD COLUMN workload_use_case              TEXT NOT NULL DEFAULT '',
    ADD COLUMN expected_context_tokens        BIGINT,
    ADD COLUMN expected_concurrency           INTEGER,
    ADD COLUMN expected_requests_per_minute   INTEGER,
    ADD COLUMN latency_priority               TEXT,
    ADD COLUMN expected_monthly_requests      BIGINT,
    ADD COLUMN idempotency_key_hash           BYTEA,
    ADD COLUMN idempotency_request_hash       BYTEA,
    ADD CONSTRAINT deployment_requests_workload_use_case_0010_check
        CHECK (length(workload_use_case) <= 2000),
    ADD CONSTRAINT deployment_requests_expected_context_0010_check
        CHECK (expected_context_tokens IS NULL OR expected_context_tokens BETWEEN 1 AND 10000000),
    ADD CONSTRAINT deployment_requests_expected_concurrency_0010_check
        CHECK (expected_concurrency IS NULL OR expected_concurrency BETWEEN 1 AND 10000),
    ADD CONSTRAINT deployment_requests_expected_rpm_0010_check
        CHECK (expected_requests_per_minute IS NULL OR expected_requests_per_minute BETWEEN 1 AND 10000000),
    ADD CONSTRAINT deployment_requests_latency_priority_0010_check
        CHECK (latency_priority IS NULL OR latency_priority IN ('balanced', 'latency', 'throughput')),
    ADD CONSTRAINT deployment_requests_expected_monthly_0010_check
        CHECK (expected_monthly_requests IS NULL OR expected_monthly_requests BETWEEN 1 AND 1000000000000),
    ADD CONSTRAINT deployment_requests_idempotency_pair_0010_check
        CHECK ((idempotency_key_hash IS NULL) = (idempotency_request_hash IS NULL)),
    ADD CONSTRAINT deployment_requests_idempotency_key_length_0010_check
        CHECK (idempotency_key_hash IS NULL OR octet_length(idempotency_key_hash) = 32),
    ADD CONSTRAINT deployment_requests_idempotency_request_length_0010_check
        CHECK (idempotency_request_hash IS NULL OR octet_length(idempotency_request_hash) = 32);

CREATE UNIQUE INDEX deployment_requests_idempotency_0010_uidx
    ON deployment_requests(organisation_id, idempotency_key_hash)
    WHERE idempotency_key_hash IS NOT NULL;

CREATE OR REPLACE FUNCTION protect_deployment_request_intent_0010() RETURNS trigger AS $$
BEGIN
    IF NEW.organisation_id IS DISTINCT FROM OLD.organisation_id
       OR NEW.project_id IS DISTINCT FROM OLD.project_id
       OR NEW.environment_id IS DISTINCT FROM OLD.environment_id
       OR NEW.request_kind IS DISTINCT FROM OLD.request_kind
       OR NEW.deployment_profile_id IS DISTINCT FROM OLD.deployment_profile_id
       OR NEW.deployment_id IS DISTINCT FROM OLD.deployment_id
       OR NEW.current_capacity_units IS DISTINCT FROM OLD.current_capacity_units
       OR NEW.requested_capacity_units IS DISTINCT FROM OLD.requested_capacity_units
       OR NEW.requested_by_user_id IS DISTINCT FROM OLD.requested_by_user_id
       OR NEW.workload_use_case IS DISTINCT FROM OLD.workload_use_case
       OR NEW.expected_context_tokens IS DISTINCT FROM OLD.expected_context_tokens
       OR NEW.expected_concurrency IS DISTINCT FROM OLD.expected_concurrency
       OR NEW.expected_requests_per_minute IS DISTINCT FROM OLD.expected_requests_per_minute
       OR NEW.latency_priority IS DISTINCT FROM OLD.latency_priority
       OR NEW.expected_monthly_requests IS DISTINCT FROM OLD.expected_monthly_requests
       OR NEW.idempotency_key_hash IS DISTINCT FROM OLD.idempotency_key_hash
       OR NEW.idempotency_request_hash IS DISTINCT FROM OLD.idempotency_request_hash THEN
        RAISE EXCEPTION 'deployment request intent is immutable' USING ERRCODE='55000';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER deployment_requests_protect_intent_0010
BEFORE UPDATE ON deployment_requests
FOR EACH ROW EXECUTE FUNCTION protect_deployment_request_intent_0010();

COMMENT ON COLUMN deployment_requests.workload_use_case IS
    'Customer sizing intent only; prompts, outputs, secrets, target hosts, and hardware identifiers are prohibited.';
COMMENT ON COLUMN deployment_requests.idempotency_key_hash IS
    'SHA-256 digest of the customer mutation key; the raw key is never stored.';
