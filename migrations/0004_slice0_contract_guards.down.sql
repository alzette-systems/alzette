ALTER TABLE inference_requests
    DROP CONSTRAINT IF EXISTS inference_requests_usage_finality_0004_check;

ALTER TABLE inference_targets
    DROP CONSTRAINT IF EXISTS inference_targets_slice0_mode_0004_check;

DELETE FROM schema_migrations WHERE version = '0004_slice0_contract_guards';
