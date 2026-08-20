-- Preserve Bifrost's provider-reported accounting dimensions without storing
-- raw provider payloads. cached_tokens remains the backwards-compatible cache
-- read value; cache creation is deliberately separate so it cannot be added to
-- Bifrost's already-normalized input total twice.
ALTER TABLE inference_requests
    ADD COLUMN total_tokens BIGINT,
    ADD COLUMN cached_write_tokens BIGINT,
    ADD COLUMN cached_write_tokens_5m BIGINT,
    ADD COLUMN cached_write_tokens_1h BIGINT,
    ADD COLUMN text_input_tokens BIGINT,
    ADD COLUMN audio_input_tokens BIGINT,
    ADD COLUMN image_input_tokens BIGINT,
    ADD COLUMN usage_normalization_version TEXT;

ALTER TABLE provider_attempts
    ADD COLUMN input_tokens BIGINT,
    ADD COLUMN output_tokens BIGINT,
    ADD COLUMN total_tokens BIGINT,
    ADD COLUMN cached_read_tokens BIGINT,
    ADD COLUMN cached_write_tokens BIGINT,
    ADD COLUMN cached_write_tokens_5m BIGINT,
    ADD COLUMN cached_write_tokens_1h BIGINT,
    ADD COLUMN reasoning_tokens BIGINT,
    ADD COLUMN text_input_tokens BIGINT,
    ADD COLUMN audio_input_tokens BIGINT,
    ADD COLUMN image_input_tokens BIGINT,
    ADD COLUMN usage_finality TEXT NOT NULL DEFAULT 'unknown',
    ADD COLUMN usage_normalization_version TEXT;

ALTER TABLE inference_requests
    ADD CONSTRAINT inference_requests_bifrost_usage_0015_check CHECK (
        (status <> 'succeeded' AND total_tokens IS NULL
            AND cached_write_tokens IS NULL AND cached_write_tokens_5m IS NULL
            AND cached_write_tokens_1h IS NULL AND text_input_tokens IS NULL
            AND audio_input_tokens IS NULL AND image_input_tokens IS NULL
            AND usage_normalization_version IS NULL)
        OR status = 'succeeded'
    ),
    ADD CONSTRAINT inference_requests_bifrost_usage_nonnegative_0015_check CHECK (
        (total_tokens IS NULL OR total_tokens >= 0)
        AND (cached_write_tokens IS NULL OR cached_write_tokens >= 0)
        AND (cached_write_tokens_5m IS NULL OR cached_write_tokens_5m >= 0)
        AND (cached_write_tokens_1h IS NULL OR cached_write_tokens_1h >= 0)
        AND (text_input_tokens IS NULL OR text_input_tokens >= 0)
        AND (audio_input_tokens IS NULL OR audio_input_tokens >= 0)
        AND (image_input_tokens IS NULL OR image_input_tokens >= 0)
    );

ALTER TABLE provider_attempts
    ADD CONSTRAINT provider_attempts_usage_0015_check CHECK (
        usage_finality IN ('unknown', 'partial', 'final')
        AND (
            (usage_finality = 'unknown' AND input_tokens IS NULL AND output_tokens IS NULL
                AND total_tokens IS NULL AND cached_read_tokens IS NULL
                AND cached_write_tokens IS NULL AND cached_write_tokens_5m IS NULL
                AND cached_write_tokens_1h IS NULL AND reasoning_tokens IS NULL
                AND text_input_tokens IS NULL AND audio_input_tokens IS NULL
                AND image_input_tokens IS NULL AND usage_normalization_version IS NULL)
            OR
            (usage_finality IN ('partial', 'final')
                AND (input_tokens IS NOT NULL OR output_tokens IS NOT NULL
                    OR total_tokens IS NOT NULL OR cached_read_tokens IS NOT NULL
                    OR cached_write_tokens IS NOT NULL OR reasoning_tokens IS NOT NULL)
                AND usage_normalization_version IS NOT NULL)
        )
    ),
    ADD CONSTRAINT provider_attempts_usage_nonnegative_0015_check CHECK (
        (input_tokens IS NULL OR input_tokens >= 0)
        AND (output_tokens IS NULL OR output_tokens >= 0)
        AND (total_tokens IS NULL OR total_tokens >= 0)
        AND (cached_read_tokens IS NULL OR cached_read_tokens >= 0)
        AND (cached_write_tokens IS NULL OR cached_write_tokens >= 0)
        AND (cached_write_tokens_5m IS NULL OR cached_write_tokens_5m >= 0)
        AND (cached_write_tokens_1h IS NULL OR cached_write_tokens_1h >= 0)
        AND (reasoning_tokens IS NULL OR reasoning_tokens >= 0)
        AND (text_input_tokens IS NULL OR text_input_tokens >= 0)
        AND (audio_input_tokens IS NULL OR audio_input_tokens >= 0)
        AND (image_input_tokens IS NULL OR image_input_tokens >= 0)
    );

COMMENT ON COLUMN inference_requests.input_tokens IS
    'Bifrost-normalized provider input total; already includes cache reads/writes where reported.';
COMMENT ON COLUMN inference_requests.cached_tokens IS
    'Provider-reported cache-read tokens. Never add this value to input_tokens.';
COMMENT ON COLUMN inference_requests.cached_write_tokens IS
    'Provider-reported cache-creation tokens. Never add this value to input_tokens.';
COMMENT ON COLUMN provider_attempts.usage_finality IS
    'Provider-attempt evidence, including billed usage reported for failed or cancelled attempts.';
