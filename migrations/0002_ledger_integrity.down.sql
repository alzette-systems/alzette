DROP TRIGGER IF EXISTS inference_requests_attempt_count_consistent_0002 ON inference_requests;
DROP TRIGGER IF EXISTS provider_attempts_attempt_count_consistent_0002 ON provider_attempts;
DROP TRIGGER IF EXISTS provider_attempts_enforce_lifecycle_0002 ON provider_attempts;
DROP TRIGGER IF EXISTS inference_targets_protect_execution_0002 ON inference_targets;
DROP TRIGGER IF EXISTS tenant_routes_protect_active_requests_0002 ON tenant_routes;
DROP TRIGGER IF EXISTS inference_requests_enforce_lifecycle_0002 ON inference_requests;

DROP FUNCTION IF EXISTS check_provider_attempt_count_from_request_0002();
DROP FUNCTION IF EXISTS check_inference_request_attempt_count_0002();
DROP FUNCTION IF EXISTS enforce_provider_attempt_lifecycle_0002();
DROP FUNCTION IF EXISTS protect_inference_target_execution_0002();
DROP FUNCTION IF EXISTS protect_active_inference_route_0002();
DROP FUNCTION IF EXISTS enforce_inference_request_lifecycle_0002();

ALTER TABLE provider_attempts
    DROP CONSTRAINT IF EXISTS provider_attempts_completed_after_started_0002_check,
    DROP CONSTRAINT IF EXISTS provider_attempts_status_completed_at_0002_check;

ALTER TABLE inference_requests
    DROP CONSTRAINT IF EXISTS inference_requests_completed_after_started_0002_check,
    DROP CONSTRAINT IF EXISTS inference_requests_status_completed_at_0002_check,
    DROP CONSTRAINT IF EXISTS inference_requests_api_key_tuple_0002_fkey;

ALTER TABLE audit_events
    DROP CONSTRAINT IF EXISTS audit_events_organisation_project_0002_fkey,
    DROP CONSTRAINT IF EXISTS audit_events_project_requires_organisation_0002_check;

ALTER TABLE usage_rollups_hourly
    DROP CONSTRAINT IF EXISTS usage_rollups_hourly_tenant_route_0002_fkey;

ALTER TABLE api_keys
    DROP CONSTRAINT IF EXISTS api_keys_service_account_id_id_key_prefix_0002_key;

DELETE FROM schema_migrations WHERE version = '0002_ledger_integrity';
