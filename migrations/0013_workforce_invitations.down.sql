DROP TABLE IF EXISTS oidc_login_transactions;
DROP TABLE IF EXISTS human_action_sessions;
DROP TABLE IF EXISTS human_federated_identities;
ALTER TABLE human_users DROP CONSTRAINT IF EXISTS human_users_auth_method_0013_check;
ALTER TABLE human_users ALTER COLUMN password_hash SET NOT NULL;
DROP TABLE IF EXISTS human_invitation_groups;
DROP TABLE IF EXISTS human_invitations;
