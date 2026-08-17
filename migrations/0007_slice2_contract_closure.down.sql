DROP TRIGGER IF EXISTS inference_targets_enforce_plan_capacity_0007 ON inference_targets;
DROP FUNCTION IF EXISTS enforce_target_plan_capacity_0007();
DROP TRIGGER IF EXISTS service_plans_enforce_route_capacity_0007 ON service_plans;
DROP FUNCTION IF EXISTS enforce_service_plan_route_capacity_0007();
DELETE FROM schema_migrations WHERE version = '0007_slice2_contract_closure';
