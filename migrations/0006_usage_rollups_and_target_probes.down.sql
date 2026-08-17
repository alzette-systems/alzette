DROP TABLE IF EXISTS worker_checkpoints;
DROP TABLE IF EXISTS target_health_observations;
DROP TABLE IF EXISTS usage_rollups_hourly_v2;
ALTER TABLE inference_targets
    DROP CONSTRAINT IF EXISTS inference_targets_probe_interval_0006_check,
    DROP COLUMN IF EXISTS probe_interval_seconds,
    DROP COLUMN IF EXISTS probe_enabled;
DELETE FROM schema_migrations WHERE version = '0006_usage_rollups_and_target_probes';
