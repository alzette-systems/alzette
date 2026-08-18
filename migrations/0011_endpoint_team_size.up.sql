-- Optional customer team-size intent for endpoint acquisition. Existing rows
-- remain NULL: concurrency is not a proxy for people and is never backfilled.
ALTER TABLE endpoint_configurations
    ADD COLUMN expected_user_count INTEGER,
    ADD CONSTRAINT endpoint_configurations_expected_user_count_0011_check
        CHECK (expected_user_count IS NULL OR expected_user_count BETWEEN 1 AND 10000);

ALTER TABLE deployment_requests
    ADD COLUMN expected_user_count INTEGER,
    ADD CONSTRAINT deployment_requests_expected_user_count_0011_check
        CHECK (expected_user_count IS NULL OR
               (request_kind = 'new_endpoint' AND expected_user_count BETWEEN 1 AND 10000));

-- Preserve the 0009 trigger name because the existing trigger already invokes
-- it; extend only the set of protected submitted configuration facts.
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
        NEW.expected_user_count IS DISTINCT FROM OLD.expected_user_count OR
        NEW.requested_by_user_id IS DISTINCT FROM OLD.requested_by_user_id OR
        NEW.idempotency_key_hash IS DISTINCT FROM OLD.idempotency_key_hash) THEN
        RAISE EXCEPTION 'submitted endpoint configuration facts are immutable' USING ERRCODE='55000';
    END IF;
    RETURN COALESCE(NEW,OLD);
END;
$$ LANGUAGE plpgsql;

-- Likewise extend the immutable deployment-request intent installed by 0010.
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
       OR NEW.expected_user_count IS DISTINCT FROM OLD.expected_user_count
       OR NEW.idempotency_key_hash IS DISTINCT FROM OLD.idempotency_key_hash
       OR NEW.idempotency_request_hash IS DISTINCT FROM OLD.idempotency_request_hash THEN
        RAISE EXCEPTION 'deployment request intent is immutable' USING ERRCODE='55000';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

COMMENT ON COLUMN endpoint_configurations.expected_user_count IS
    'Optional customer-declared people count for endpoint acquisition; never inferred from concurrency.';
COMMENT ON COLUMN deployment_requests.expected_user_count IS
    'Immutable submitted endpoint-acquisition people count; NULL means not recorded.';
