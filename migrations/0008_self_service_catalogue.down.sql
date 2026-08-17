DROP TRIGGER IF EXISTS deployment_profiles_protect_references_0008 ON deployment_profiles;
DROP FUNCTION IF EXISTS protect_referenced_deployment_profile_0008();

DROP TRIGGER IF EXISTS deployment_capacity_revisions_enforce_0008 ON deployment_capacity_revisions;
DROP FUNCTION IF EXISTS enforce_capacity_revision_0008();

DROP TRIGGER IF EXISTS deployment_requests_enforce_0008 ON deployment_requests;
DROP FUNCTION IF EXISTS enforce_deployment_request_0008();

DROP TRIGGER IF EXISTS model_deployments_enforce_binding_0008 ON model_deployments;
DROP FUNCTION IF EXISTS enforce_model_deployment_binding_0008();

DROP TRIGGER IF EXISTS deployment_quotes_protect_accepted_delete_0008 ON deployment_quotes;
DROP TRIGGER IF EXISTS deployment_quotes_protect_accepted_update_0008 ON deployment_quotes;
DROP FUNCTION IF EXISTS protect_accepted_deployment_quote_0008();

DROP TRIGGER IF EXISTS deployment_quotes_enforce_0008 ON deployment_quotes;
DROP FUNCTION IF EXISTS enforce_deployment_quote_0008();

DROP TRIGGER IF EXISTS inference_targets_protect_evaluation_offer_0008 ON inference_targets;
DROP FUNCTION IF EXISTS protect_evaluation_offer_target_0008();

DROP TRIGGER IF EXISTS evaluation_offer_templates_enforce_0008 ON evaluation_offer_templates;
DROP FUNCTION IF EXISTS enforce_evaluation_offer_0008();

DROP TABLE IF EXISTS deployment_capacity_revisions;
DROP TABLE IF EXISTS deployment_requests;
DROP TABLE IF EXISTS model_deployments;
DROP TABLE IF EXISTS deployment_quotes;
DROP TABLE IF EXISTS business_qualification_requests;
DROP TABLE IF EXISTS self_service_registrations;
DROP TABLE IF EXISTS evaluation_offer_templates;
DROP TABLE IF EXISTS deployment_profile_prices;
DROP TABLE IF EXISTS deployment_profile_metrics;
DROP TABLE IF EXISTS deployment_profiles;
DROP TABLE IF EXISTS catalogue_model_versions;
DROP TABLE IF EXISTS catalogue_models;

DROP INDEX IF EXISTS human_users_email_normalized_0008_idx;

ALTER TABLE human_users
    DROP CONSTRAINT IF EXISTS human_users_self_service_email_0008_check,
    DROP CONSTRAINT IF EXISTS human_users_verified_email_0008_check,
    DROP CONSTRAINT IF EXISTS human_users_email_normalized_0008_check,
    DROP CONSTRAINT IF EXISTS human_users_email_pair_0008_check,
    DROP CONSTRAINT IF EXISTS human_users_identity_origin_0008_check,
    DROP COLUMN IF EXISTS identity_origin,
    DROP COLUMN IF EXISTS email_verified_at,
    DROP COLUMN IF EXISTS email_normalized,
    DROP COLUMN IF EXISTS email;

ALTER TABLE organisations
    DROP CONSTRAINT IF EXISTS organisations_business_approval_evidence_0008_check,
    DROP CONSTRAINT IF EXISTS organisations_self_service_approval_0008_check,
    DROP CONSTRAINT IF EXISTS organisations_business_approval_pair_0008_check,
    DROP CONSTRAINT IF EXISTS organisations_created_via_0008_check,
    DROP CONSTRAINT IF EXISTS organisations_lifecycle_status_0008_check,
    DROP CONSTRAINT IF EXISTS organisations_account_kind_0008_check,
    DROP COLUMN IF EXISTS business_approval_evidence_ref,
    DROP COLUMN IF EXISTS business_approved_at,
    DROP COLUMN IF EXISTS created_via,
    DROP COLUMN IF EXISTS lifecycle_status,
    DROP COLUMN IF EXISTS account_kind;

DELETE FROM schema_migrations WHERE version = '0008_self_service_catalogue';
