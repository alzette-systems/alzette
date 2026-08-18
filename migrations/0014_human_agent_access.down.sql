DROP TRIGGER IF EXISTS human_federated_identities_revoke_agent_authority_0014 ON human_federated_identities;
DROP TRIGGER IF EXISTS organisation_people_revoke_agent_authority_0014 ON organisation_people;
DROP TRIGGER IF EXISTS human_users_revoke_agent_authority_0014 ON human_users;
DROP FUNCTION IF EXISTS revoke_human_agent_authority_0014();

ALTER TABLE inference_requests
    DROP CONSTRAINT IF EXISTS inference_requests_actor_0014_check,
    DROP CONSTRAINT IF EXISTS inference_requests_human_membership_0014_fk,
    DROP COLUMN IF EXISTS agent_token_id,
    DROP COLUMN IF EXISTS agent_grant_id,
    DROP COLUMN IF EXISTS human_membership_id,
    DROP COLUMN IF EXISTS human_user_id,
    ALTER COLUMN key_prefix SET NOT NULL,
    ALTER COLUMN api_key_id SET NOT NULL,
    ALTER COLUMN service_account_id SET NOT NULL;

DROP TABLE IF EXISTS human_agent_credential_mints;
DROP TABLE IF EXISTS human_agent_access_tokens;
DROP TABLE IF EXISTS human_agent_grants;
