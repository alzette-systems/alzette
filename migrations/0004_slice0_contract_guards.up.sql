-- Prevent direct SQL from widening the execution/capacity vocabulary beyond
-- the Slice 0 provisioning contract. NOT VALID keeps upgrades safe for any
-- legacy rows while enforcing the rule for every new or changed row.
ALTER TABLE inference_targets
    ADD CONSTRAINT inference_targets_slice0_mode_0004_check
    CHECK (
        execution_class IN ('external_pilot', 'private_compatible')
        AND (execution_class <> 'external_pilot' OR capacity_mode = 'shared')
    ) NOT VALID;

-- Usage finality is evidence, not a defaulted total. Unknown means no token
-- dimension is known, partial means at least one dimension is known while an
-- input/output total is missing, and final requires both logical totals.
-- Non-success rows cannot carry provider usage.
ALTER TABLE inference_requests
    ADD CONSTRAINT inference_requests_usage_finality_0004_check
    CHECK (
        (
            status <> 'succeeded'
            AND usage_finality = 'unknown'
            AND input_tokens IS NULL
            AND output_tokens IS NULL
            AND cached_tokens IS NULL
            AND reasoning_tokens IS NULL
        )
        OR
        (
            status = 'succeeded'
            AND (
                (
                    usage_finality = 'unknown'
                    AND input_tokens IS NULL
                    AND output_tokens IS NULL
                    AND cached_tokens IS NULL
                    AND reasoning_tokens IS NULL
                )
                OR
                (
                    usage_finality = 'partial'
                    AND (input_tokens IS NOT NULL
                         OR output_tokens IS NOT NULL
                         OR cached_tokens IS NOT NULL
                         OR reasoning_tokens IS NOT NULL)
                    AND (input_tokens IS NULL OR output_tokens IS NULL)
                )
                OR
                (
                    usage_finality = 'final'
                    AND input_tokens IS NOT NULL
                    AND output_tokens IS NOT NULL
                )
            )
        )
    ) NOT VALID;
