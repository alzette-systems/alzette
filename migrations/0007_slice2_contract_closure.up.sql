DO $$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM tenant_service_plans tsp
          JOIN service_plans sp
            ON sp.organisation_id = tsp.organisation_id
           AND sp.id = tsp.service_plan_id
          JOIN tenant_routes r
            ON r.organisation_id = tsp.organisation_id
           AND r.project_id = tsp.project_id
           AND r.environment_id = tsp.environment_id
           AND r.id = tsp.route_id
          JOIN inference_targets t ON t.id = r.target_id
         WHERE tsp.status = 'active'
           AND sp.capacity_mode <> t.capacity_mode
    ) THEN
        RAISE EXCEPTION 'an active service plan already conflicts with its route target capacity mode'
            USING ERRCODE = '23514';
    END IF;
END;
$$;

CREATE OR REPLACE FUNCTION enforce_service_plan_route_capacity_0007() RETURNS trigger AS $$
BEGIN
    IF NEW.capacity_mode IS DISTINCT FROM OLD.capacity_mode AND EXISTS (
        SELECT 1
          FROM tenant_service_plans tsp
          JOIN tenant_routes r
            ON r.organisation_id = tsp.organisation_id
           AND r.project_id = tsp.project_id
           AND r.environment_id = tsp.environment_id
           AND r.id = tsp.route_id
          JOIN inference_targets t ON t.id = r.target_id
         WHERE tsp.organisation_id = NEW.organisation_id
           AND tsp.service_plan_id = NEW.id
           AND tsp.status = 'active'
           AND t.capacity_mode <> NEW.capacity_mode
    ) THEN
        RAISE EXCEPTION 'service plan capacity mode does not match an active bound route target'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER service_plans_enforce_route_capacity_0007
BEFORE UPDATE OF capacity_mode ON service_plans
FOR EACH ROW EXECUTE FUNCTION enforce_service_plan_route_capacity_0007();

CREATE OR REPLACE FUNCTION enforce_target_plan_capacity_0007() RETURNS trigger AS $$
BEGIN
    IF NEW.capacity_mode IS DISTINCT FROM OLD.capacity_mode AND EXISTS (
        SELECT 1
          FROM tenant_routes r
          JOIN tenant_service_plans tsp
            ON tsp.organisation_id = r.organisation_id
           AND tsp.project_id = r.project_id
           AND tsp.environment_id = r.environment_id
           AND tsp.route_id = r.id
           AND tsp.status = 'active'
          JOIN service_plans sp
            ON sp.organisation_id = tsp.organisation_id
           AND sp.id = tsp.service_plan_id
         WHERE r.target_id = NEW.id
           AND sp.capacity_mode <> NEW.capacity_mode
    ) THEN
        RAISE EXCEPTION 'target capacity mode does not match an active bound service plan'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER inference_targets_enforce_plan_capacity_0007
BEFORE UPDATE OF capacity_mode ON inference_targets
FOR EACH ROW EXECUTE FUNCTION enforce_target_plan_capacity_0007();

COMMENT ON FUNCTION enforce_service_plan_route_capacity_0007() IS
    'Prevents direct service-plan edits from drifting away from active route target capacity evidence.';
COMMENT ON FUNCTION enforce_target_plan_capacity_0007() IS
    'Protects active plan bindings even while a route is disabled, so re-enabling cannot expose a stale capacity claim.';
