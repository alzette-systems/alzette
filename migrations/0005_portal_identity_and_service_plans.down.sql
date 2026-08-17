DROP TRIGGER IF EXISTS human_memberships_revoke_sessions_0005 ON human_memberships;
DROP TRIGGER IF EXISTS human_users_revoke_sessions_0005 ON human_users;
DROP TRIGGER IF EXISTS inference_targets_enforce_plan_capacity_0005 ON inference_targets;
DROP TRIGGER IF EXISTS tenant_routes_enforce_plan_capacity_0005 ON tenant_routes;
DROP TRIGGER IF EXISTS tenant_service_plans_enforce_capacity_0005 ON tenant_service_plans;
DROP FUNCTION IF EXISTS revoke_membership_sessions_0005();
DROP FUNCTION IF EXISTS revoke_human_sessions_0005();
DROP FUNCTION IF EXISTS enforce_target_plan_capacity_0005();
DROP FUNCTION IF EXISTS enforce_route_plan_capacity_0005();
DROP FUNCTION IF EXISTS enforce_plan_capacity_0005();

DROP TABLE IF EXISTS portal_sessions;
DROP TABLE IF EXISTS human_memberships;
DROP TABLE IF EXISTS human_users;
DROP TABLE IF EXISTS tenant_service_plans;
DROP TABLE IF EXISTS service_plans;

ALTER TABLE audit_events DROP CONSTRAINT IF EXISTS audit_events_actor_type_check;
ALTER TABLE audit_events
    ADD CONSTRAINT audit_events_actor_type_check
    CHECK (actor_type IN ('operator', 'service_account', 'system', 'human_user'));

COMMENT ON CONSTRAINT audit_events_actor_type_check ON audit_events IS
    'human_user remains admitted after rollback so append-only portal audit evidence is preserved.';

ALTER TABLE api_keys
    DROP CONSTRAINT IF EXISTS api_keys_name_0005_check,
    DROP CONSTRAINT IF EXISTS api_keys_expiry_lifecycle_0005_check,
    DROP CONSTRAINT IF EXISTS api_keys_revocation_lifecycle_0005_check,
    DROP CONSTRAINT IF EXISTS api_keys_rotated_from_account_0005_fkey,
    DROP CONSTRAINT IF EXISTS api_keys_service_account_id_id_0005_unique,
    DROP COLUMN IF EXISTS rotated_from_key_id,
    DROP COLUMN IF EXISTS name;

ALTER TABLE service_accounts
    DROP CONSTRAINT IF EXISTS service_accounts_name_0005_check;

DELETE FROM schema_migrations WHERE version = '0005_portal_identity_and_service_plans';
