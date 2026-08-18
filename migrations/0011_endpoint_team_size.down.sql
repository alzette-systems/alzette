-- Restore the trigger functions to their pre-0011 definitions before removing
-- the columns they reference. Existing submitted evidence is not rewritten.
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

ALTER TABLE deployment_requests
    DROP CONSTRAINT IF EXISTS deployment_requests_expected_user_count_0011_check,
    DROP COLUMN IF EXISTS expected_user_count;

ALTER TABLE endpoint_configurations
    DROP CONSTRAINT IF EXISTS endpoint_configurations_expected_user_count_0011_check,
    DROP COLUMN IF EXISTS expected_user_count;

DELETE FROM schema_migrations WHERE version='0011_endpoint_team_size';
