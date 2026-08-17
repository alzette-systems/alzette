DROP TRIGGER IF EXISTS provider_attempts_enforce_route_binding_0003 ON provider_attempts;
DROP TRIGGER IF EXISTS inference_requests_enforce_route_binding_0003 ON inference_requests;
DROP TRIGGER IF EXISTS tenant_routes_advance_binding_generation_0003 ON tenant_routes;

DROP FUNCTION IF EXISTS enforce_attempt_route_binding_0003();
DROP FUNCTION IF EXISTS enforce_request_route_binding_0003();
DROP FUNCTION IF EXISTS advance_route_binding_generation_0003();

DROP INDEX IF EXISTS inference_requests_route_binding_time_0003_idx;

ALTER TABLE inference_requests
    DROP CONSTRAINT IF EXISTS inference_requests_route_binding_0003_check,
    DROP COLUMN IF EXISTS route_binding_generation,
    DROP COLUMN IF EXISTS bound_model_id,
    DROP COLUMN IF EXISTS bound_target_id;

ALTER TABLE tenant_routes
    DROP CONSTRAINT IF EXISTS tenant_routes_binding_generation_0003_check,
    DROP COLUMN IF EXISTS binding_generation;

DELETE FROM schema_migrations WHERE version = '0003_route_binding_observations';
