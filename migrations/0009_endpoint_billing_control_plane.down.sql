DROP TRIGGER IF EXISTS billing_webhook_receipts_protect_0009 ON billing_webhook_receipts;
DROP FUNCTION IF EXISTS protect_billing_webhook_receipt_0009();
DROP TRIGGER IF EXISTS billing_checkout_sessions_protect_0009 ON billing_checkout_sessions;
DROP FUNCTION IF EXISTS protect_billing_checkout_session_0009();
DROP TRIGGER IF EXISTS payment_requirements_protect_facts_0009 ON payment_requirements;
DROP FUNCTION IF EXISTS protect_payment_requirement_facts_0009();
DROP TRIGGER IF EXISTS customer_endpoints_protect_facts_0009 ON customer_endpoints;
DROP FUNCTION IF EXISTS protect_customer_endpoint_facts_0009();
DROP TRIGGER IF EXISTS customer_endpoints_enforce_0009 ON customer_endpoints;
DROP FUNCTION IF EXISTS enforce_customer_endpoint_0009();
DROP TRIGGER IF EXISTS endpoint_configurations_protect_submitted_0009 ON endpoint_configurations;
DROP FUNCTION IF EXISTS protect_submitted_endpoint_configuration_0009();
DROP TRIGGER IF EXISTS endpoint_configurations_enforce_0009 ON endpoint_configurations;
DROP FUNCTION IF EXISTS enforce_endpoint_configuration_0009();
DROP TRIGGER IF EXISTS endpoint_offers_protect_references_0009 ON endpoint_offers;
DROP FUNCTION IF EXISTS protect_referenced_endpoint_offer_0009();
DROP TRIGGER IF EXISTS billing_price_mappings_protect_0009 ON billing_price_mappings;
DROP FUNCTION IF EXISTS protect_billing_price_mapping_0009();
DROP TRIGGER IF EXISTS endpoint_offers_enforce_0009 ON endpoint_offers;
DROP FUNCTION IF EXISTS enforce_endpoint_offer_0009();

ALTER TABLE portal_sessions
    DROP CONSTRAINT IF EXISTS portal_sessions_authenticated_lifecycle_0009_check,
    DROP COLUMN IF EXISTS authenticated_at;

ALTER TABLE api_keys
    DROP CONSTRAINT IF EXISTS api_keys_service_account_name_0009_unique;

DROP TABLE IF EXISTS billing_webhook_receipts;
DROP TABLE IF EXISTS billing_invoices;
DROP TABLE IF EXISTS billing_subscriptions;
DROP TABLE IF EXISTS billing_checkout_sessions;
DROP TABLE IF EXISTS payment_requirements;
DROP TABLE IF EXISTS billing_accounts;
DROP TABLE IF EXISTS deployment_quote_payment_terms;
DROP TABLE IF EXISTS customer_endpoints;
DROP TABLE IF EXISTS endpoint_configurations;
DROP TABLE IF EXISTS billing_price_mappings;
DROP TABLE IF EXISTS endpoint_offers;

DELETE FROM schema_migrations WHERE version='0009_endpoint_billing_control_plane';
