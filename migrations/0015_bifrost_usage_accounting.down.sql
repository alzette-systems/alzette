ALTER TABLE provider_attempts
    DROP CONSTRAINT IF EXISTS provider_attempts_usage_nonnegative_0015_check,
    DROP CONSTRAINT IF EXISTS provider_attempts_usage_0015_check,
    DROP COLUMN IF EXISTS usage_normalization_version,
    DROP COLUMN IF EXISTS usage_finality,
    DROP COLUMN IF EXISTS image_input_tokens,
    DROP COLUMN IF EXISTS audio_input_tokens,
    DROP COLUMN IF EXISTS text_input_tokens,
    DROP COLUMN IF EXISTS reasoning_tokens,
    DROP COLUMN IF EXISTS cached_write_tokens_1h,
    DROP COLUMN IF EXISTS cached_write_tokens_5m,
    DROP COLUMN IF EXISTS cached_write_tokens,
    DROP COLUMN IF EXISTS cached_read_tokens,
    DROP COLUMN IF EXISTS total_tokens,
    DROP COLUMN IF EXISTS output_tokens,
    DROP COLUMN IF EXISTS input_tokens;

ALTER TABLE inference_requests
    DROP CONSTRAINT IF EXISTS inference_requests_bifrost_usage_nonnegative_0015_check,
    DROP CONSTRAINT IF EXISTS inference_requests_bifrost_usage_0015_check,
    DROP COLUMN IF EXISTS usage_normalization_version,
    DROP COLUMN IF EXISTS image_input_tokens,
    DROP COLUMN IF EXISTS audio_input_tokens,
    DROP COLUMN IF EXISTS text_input_tokens,
    DROP COLUMN IF EXISTS cached_write_tokens_1h,
    DROP COLUMN IF EXISTS cached_write_tokens_5m,
    DROP COLUMN IF EXISTS cached_write_tokens,
    DROP COLUMN IF EXISTS total_tokens;

DELETE FROM schema_migrations WHERE version = '0015_bifrost_usage_accounting';
