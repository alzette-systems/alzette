ALTER TABLE tenant_routes
    ADD COLUMN binding_generation BIGINT NOT NULL DEFAULT 1,
    ADD CONSTRAINT tenant_routes_binding_generation_0003_check
        CHECK (binding_generation > 0);

ALTER TABLE inference_requests
    ADD COLUMN bound_target_id TEXT REFERENCES inference_targets(id) ON DELETE RESTRICT,
    ADD COLUMN bound_model_id TEXT REFERENCES models(id) ON DELETE RESTRICT,
    ADD COLUMN route_binding_generation BIGINT;

-- Pre-0003 history has no durable route-generation marker. Start every route
-- that already exists in a new current generation, leaving generation one as
-- a conservative legacy epoch. This deliberately makes old observations
-- ineligible for current-route health even when a route previously changed
-- A -> B -> A and its historical target/model happen to match again.
UPDATE tenant_routes SET binding_generation = 2;

-- Historical attribution is a schema backfill, not a ledger mutation. Hold an
-- exclusive lock so no request can change while user triggers that would
-- reject completed-row backfills or queue deferred
-- lifecycle checks are disabled. Foreign-key/check constraints remain active.
-- Any failure rolls the disable and all writes back in the transaction.
LOCK TABLE inference_requests IN ACCESS EXCLUSIVE MODE;
ALTER TABLE inference_requests DISABLE TRIGGER USER;

-- Existing attempts are durable evidence of the target actually used. Model
-- aliases are unique, so a surviving alias also gives exact model attribution.
-- Their generation remains the isolated legacy epoch: pre-0003 history cannot
-- prove which route generation produced it and cannot influence the migrated
-- route's current observation.
WITH exact_attempt_bindings AS (
    SELECT ir.id,
           min(pa.target_id) AS target_id,
           min(m.id) AS model_id
      FROM inference_requests ir
      JOIN provider_attempts pa ON pa.inference_request_id = ir.id
      JOIN models m ON m.alias = ir.model_alias
     WHERE ir.route_id IS NOT NULL
     GROUP BY ir.id
    HAVING count(DISTINCT pa.target_id) = 1
       AND count(DISTINCT m.id) = 1
)
UPDATE inference_requests ir
   SET bound_target_id = binding.target_id,
       bound_model_id = binding.model_id,
       route_binding_generation = 1
  FROM exact_attempt_bindings binding
 WHERE ir.id = binding.id;

-- A 0002 schema prevents an in-progress route from being retargeted. It is
-- therefore safe to bind migrated active requests that have not attempted yet
-- to the route row they currently reference.
UPDATE inference_requests ir
   SET bound_target_id = r.target_id,
       bound_model_id = r.model_id,
       route_binding_generation = r.binding_generation
  FROM tenant_routes r
  JOIN models m ON m.id = r.model_id
 WHERE ir.status = 'in_progress'
   AND ir.route_id = r.id
   AND ir.organisation_id = r.organisation_id
   AND ir.project_id = r.project_id
   AND ir.environment_id = r.environment_id
   AND ir.model_alias = m.alias
   AND ir.bound_target_id IS NULL
   AND NOT EXISTS (
       SELECT 1 FROM provider_attempts pa
        WHERE pa.inference_request_id = ir.id
   );

ALTER TABLE inference_requests ENABLE TRIGGER USER;

ALTER TABLE inference_requests
    ADD CONSTRAINT inference_requests_route_binding_0003_check
        CHECK (
            (bound_target_id IS NULL AND bound_model_id IS NULL AND route_binding_generation IS NULL)
            OR
            (route_id IS NOT NULL AND bound_target_id IS NOT NULL AND bound_model_id IS NOT NULL
             AND route_binding_generation > 0)
        );

CREATE INDEX inference_requests_route_binding_time_0003_idx
    ON inference_requests (
        route_id,
        route_binding_generation,
        bound_target_id,
        bound_model_id,
        started_at DESC
    )
    WHERE bound_target_id IS NOT NULL;

CREATE OR REPLACE FUNCTION advance_route_binding_generation_0003() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.binding_generation <> 1 THEN
            RAISE EXCEPTION 'new tenant routes must start at binding generation one'
                USING ERRCODE = '23514';
        END IF;
        RETURN NEW;
    END IF;

    IF (NEW.organisation_id,
        NEW.project_id,
        NEW.environment_id,
        NEW.model_id,
        NEW.target_id) IS DISTINCT FROM
       (OLD.organisation_id,
        OLD.project_id,
        OLD.environment_id,
        OLD.model_id,
        OLD.target_id) THEN
        IF NEW.binding_generation IS DISTINCT FROM OLD.binding_generation THEN
            RAISE EXCEPTION 'route binding generation is database controlled'
                USING ERRCODE = '55000';
        END IF;
        IF OLD.binding_generation = 9223372036854775807 THEN
            RAISE EXCEPTION 'route binding generation exhausted'
                USING ERRCODE = '54000';
        END IF;
        NEW.binding_generation := OLD.binding_generation + 1;
    ELSIF NEW.binding_generation IS DISTINCT FROM OLD.binding_generation THEN
        RAISE EXCEPTION 'route binding generation is database controlled'
            USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tenant_routes_advance_binding_generation_0003
BEFORE INSERT OR UPDATE ON tenant_routes
FOR EACH ROW EXECUTE FUNCTION advance_route_binding_generation_0003();

CREATE OR REPLACE FUNCTION enforce_request_route_binding_0003() RETURNS trigger AS $$
DECLARE
    actual_target_id TEXT;
    actual_model_id TEXT;
    actual_generation BIGINT;
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.bound_target_id IS NOT NULL OR
           NEW.bound_model_id IS NOT NULL OR
           NEW.route_binding_generation IS NOT NULL THEN
            RAISE EXCEPTION 'inference requests must be created without a route binding'
                USING ERRCODE = '23514';
        END IF;
        RETURN NEW;
    END IF;

    IF OLD.route_id IS NULL AND NEW.route_id IS NOT NULL THEN
        -- This conflicting lock makes attachment serialize with a retarget. It
        -- also ensures validation observes the current committed generation.
        SELECT r.target_id, r.model_id, r.binding_generation
          INTO actual_target_id, actual_model_id, actual_generation
          FROM tenant_routes r
          JOIN models m ON m.id = r.model_id
         WHERE r.id = NEW.route_id
           AND r.organisation_id = NEW.organisation_id
           AND r.project_id = NEW.project_id
           AND r.environment_id = NEW.environment_id
           AND m.alias = NEW.model_alias
         FOR UPDATE OF r;

        IF NOT FOUND OR
           NEW.bound_target_id IS DISTINCT FROM actual_target_id OR
           NEW.bound_model_id IS DISTINCT FROM actual_model_id OR
           NEW.route_binding_generation IS DISTINCT FROM actual_generation THEN
            RAISE EXCEPTION 'inference request binding does not match the current authorised route generation'
                USING ERRCODE = '23514';
        END IF;
    ELSIF (NEW.bound_target_id,
           NEW.bound_model_id,
           NEW.route_binding_generation) IS DISTINCT FROM
          (OLD.bound_target_id,
           OLD.bound_model_id,
           OLD.route_binding_generation) THEN
        RAISE EXCEPTION 'inference request route binding is immutable'
            USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER inference_requests_enforce_route_binding_0003
BEFORE INSERT OR UPDATE ON inference_requests
FOR EACH ROW EXECUTE FUNCTION enforce_request_route_binding_0003();

CREATE OR REPLACE FUNCTION enforce_attempt_route_binding_0003() RETURNS trigger AS $$
DECLARE
    request_target_id TEXT;
    request_generation BIGINT;
BEGIN
    IF TG_OP <> 'INSERT' THEN
        RETURN NEW;
    END IF;

    SELECT bound_target_id, route_binding_generation
      INTO request_target_id, request_generation
      FROM inference_requests
     WHERE id = NEW.inference_request_id
     FOR UPDATE;

    IF NOT FOUND OR request_target_id IS NULL OR request_generation IS NULL OR
       NEW.target_id IS DISTINCT FROM request_target_id THEN
        RAISE EXCEPTION 'provider attempt target does not match the request route binding'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER provider_attempts_enforce_route_binding_0003
BEFORE INSERT OR UPDATE ON provider_attempts
FOR EACH ROW EXECUTE FUNCTION enforce_attempt_route_binding_0003();
