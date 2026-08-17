DROP TRIGGER IF EXISTS deployment_requests_protect_intent_0010 ON deployment_requests;
DROP FUNCTION IF EXISTS protect_deployment_request_intent_0010();
DROP INDEX IF EXISTS deployment_requests_idempotency_0010_uidx;

ALTER TABLE deployment_requests
    DROP CONSTRAINT IF EXISTS deployment_requests_idempotency_request_length_0010_check,
    DROP CONSTRAINT IF EXISTS deployment_requests_idempotency_key_length_0010_check,
    DROP CONSTRAINT IF EXISTS deployment_requests_idempotency_pair_0010_check,
    DROP CONSTRAINT IF EXISTS deployment_requests_expected_monthly_0010_check,
    DROP CONSTRAINT IF EXISTS deployment_requests_latency_priority_0010_check,
    DROP CONSTRAINT IF EXISTS deployment_requests_expected_rpm_0010_check,
    DROP CONSTRAINT IF EXISTS deployment_requests_expected_concurrency_0010_check,
    DROP CONSTRAINT IF EXISTS deployment_requests_expected_context_0010_check,
    DROP CONSTRAINT IF EXISTS deployment_requests_workload_use_case_0010_check,
    DROP COLUMN IF EXISTS idempotency_request_hash,
    DROP COLUMN IF EXISTS idempotency_key_hash,
    DROP COLUMN IF EXISTS expected_monthly_requests,
    DROP COLUMN IF EXISTS latency_priority,
    DROP COLUMN IF EXISTS expected_requests_per_minute,
    DROP COLUMN IF EXISTS expected_concurrency,
    DROP COLUMN IF EXISTS expected_context_tokens,
    DROP COLUMN IF EXISTS workload_use_case;

DELETE FROM schema_migrations WHERE version='0010_capacity_request_intent';
