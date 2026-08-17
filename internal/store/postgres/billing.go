package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"alzette/internal/billing"
	"alzette/internal/ids"
	"alzette/internal/platform"
)

func (s *Store) GetBillingSummary(ctx context.Context, session platform.PortalSession) (billing.Summary, error) {
	var result billing.Summary
	var accountState, taxStatus sql.NullString
	if err := s.db.QueryRowContext(ctx, `SELECT o.name,ba.lifecycle_state,ba.tax_status
		FROM organisations o
		LEFT JOIN billing_accounts ba ON ba.organisation_id=o.id AND ba.lifecycle_state='active'
		WHERE o.id=$1`, session.Current.OrganisationID).Scan(&result.AccountName, &accountState, &taxStatus); errors.Is(err, sql.ErrNoRows) {
		return billing.Summary{}, platform.ErrNotFound
	} else if err != nil {
		return billing.Summary{}, err
	}
	result.State = accountState.String
	result.TaxStatus = "not_collected"
	if taxStatus.Valid {
		result.TaxStatus = taxStatus.String
	}
	result.CommercialState = "none"
	err := s.db.QueryRowContext(ctx, `SELECT bs.status FROM billing_subscriptions bs
		JOIN customer_endpoints ce ON ce.organisation_id=bs.organisation_id AND ce.id=bs.endpoint_id
		WHERE ce.organisation_id=$1 AND ce.project_id=$2 AND ce.environment_id=$3
		ORDER BY bs.latest_event_at DESC LIMIT 1`, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID).Scan(&result.CommercialState)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return billing.Summary{}, err
	}
	result.CheckoutState = "none"
	err = s.db.QueryRowContext(ctx, `SELECT bcs.state FROM billing_checkout_sessions bcs
		JOIN payment_requirements pr ON pr.organisation_id=bcs.organisation_id AND pr.id=bcs.payment_requirement_id
		WHERE pr.organisation_id=$1 AND pr.project_id=$2 AND pr.environment_id=$3
		ORDER BY bcs.updated_at DESC LIMIT 1`, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID).Scan(&result.CheckoutState)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return billing.Summary{}, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT bi.safe_number,bi.status,bi.amount_due_minor,bi.amount_paid_minor,bi.currency,bi.due_at,bi.paid_at,bi.created_at
		FROM billing_invoices bi
		JOIN customer_endpoints ce ON ce.organisation_id=bi.organisation_id AND ce.id=bi.endpoint_id
		WHERE ce.organisation_id=$1 AND ce.project_id=$2 AND ce.environment_id=$3
		ORDER BY bi.created_at DESC LIMIT 50`, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID)
	if err != nil {
		return billing.Summary{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var invoice billing.Invoice
		var reference sql.NullString
		var due, paid sql.NullTime
		if err := rows.Scan(&reference, &invoice.Status, &invoice.AmountDueMinor, &invoice.AmountPaidMinor, &invoice.Currency, &due, &paid, &invoice.IssuedAt); err != nil {
			return billing.Summary{}, err
		}
		invoice.Reference = reference.String
		invoice.DueAt = nullTimePointer(due)
		invoice.PaidAt = nullTimePointer(paid)
		if paid.Valid && (result.ConfirmedAt == nil || paid.Time.After(*result.ConfirmedAt)) {
			confirmed := paid.Time
			result.ConfirmedAt = &confirmed
		}
		result.Invoices = append(result.Invoices, invoice)
	}
	if err := rows.Err(); err != nil {
		return billing.Summary{}, err
	}
	if result.State != "" {
		result.Detail = "Billing state is scoped to this organisation and the selected project/environment. Payment confirmation remains server-authoritative."
	}
	return result, nil
}

func (s *Store) PrepareCheckout(ctx context.Context, session platform.PortalSession, requirementID string, digest [32]byte, now time.Time) (billing.CheckoutPlan, *billing.CheckoutResponse, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return billing.CheckoutPlan{}, nil, err
	}
	defer tx.Rollback()
	var plan billing.CheckoutPlan
	var requirementState, collectionMode string
	var accountRef sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT pr.organisation_id,o.name,pr.endpoint_id,pr.id,pr.state,pr.collection_mode,
		COALESCE(ba.provider_customer_ref,''),COALESCE(m.provider_price_ref,'')
		FROM payment_requirements pr
		JOIN customer_endpoints ce ON ce.organisation_id=pr.organisation_id AND ce.project_id=pr.project_id
		 AND ce.environment_id=pr.environment_id AND ce.id=pr.endpoint_id
		JOIN organisations o ON o.id=pr.organisation_id
		LEFT JOIN billing_accounts ba ON ba.organisation_id=pr.organisation_id AND ba.provider='stripe' AND ba.lifecycle_state='active'
		LEFT JOIN billing_price_mappings m ON m.offer_id=ce.offer_id AND m.provider='stripe' AND m.active
		WHERE pr.organisation_id=$1 AND pr.project_id=$2 AND pr.environment_id=$3 AND pr.id=$4
		FOR UPDATE OF pr,ce`, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID, requirementID).Scan(
		&plan.OrganisationID, &plan.OrganisationName, &plan.EndpointID, &plan.PaymentRequirementID,
		&requirementState, &collectionMode, &accountRef, &plan.ProviderPriceRef)
	if errors.Is(err, sql.ErrNoRows) {
		return billing.CheckoutPlan{}, nil, platform.ErrNotFound
	}
	if err != nil {
		return billing.CheckoutPlan{}, nil, err
	}
	if requirementState == "paid" {
		response := &billing.CheckoutResponse{Schema: "alzette.portal.checkout.v1", PaymentRequirementID: requirementID, State: "paid", PaymentConfirmed: true, RuntimeReady: false}
		return plan, response, nil
	}
	if requirementState != "action_required" && requirementState != "pending" && requirementState != "failed" {
		return billing.CheckoutPlan{}, nil, platform.ErrConflict
	}
	if collectionMode != "checkout_subscription" && collectionMode != "checkout_payment" {
		return billing.CheckoutPlan{}, nil, platform.ErrConflict
	}
	if plan.ProviderPriceRef == "" {
		return billing.CheckoutPlan{}, nil, billing.ErrNotConfigured
	}
	plan.Provider = "stripe"
	plan.ProviderCustomerRef = accountRef.String
	plan.Mode = "subscription"
	if collectionMode == "checkout_payment" {
		plan.Mode = "payment"
	}
	var existingID, existingRequirement string
	err = tx.QueryRowContext(ctx, `SELECT id,payment_requirement_id FROM billing_checkout_sessions WHERE operation_key_hash=$1 FOR UPDATE`, digest[:]).Scan(&existingID, &existingRequirement)
	if err == nil {
		if existingRequirement != requirementID {
			return billing.CheckoutPlan{}, nil, platform.ErrConflict
		}
		plan.OperationID, plan.OperationKey = existingID, "alzette-checkout-"+existingID
		return plan, nil, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return billing.CheckoutPlan{}, nil, err
	}
	operationID, err := ids.New("bcs")
	if err != nil {
		return billing.CheckoutPlan{}, nil, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO billing_checkout_sessions(id,organisation_id,payment_requirement_id,provider,operation_key_hash,state) VALUES($1,$2,$3,'stripe',$4,'creating')`, operationID, plan.OrganisationID, requirementID, digest[:])
	if err != nil {
		return billing.CheckoutPlan{}, nil, mapWriteError("prepare hosted checkout", err)
	}
	plan.OperationID, plan.OperationKey = operationID, "alzette-checkout-"+operationID
	if err := insertActorAudit(ctx, tx, "human_user", session.User.ID, session.Current.OrganisationID, session.Current.ProjectID, "billing.checkout_requested", "succeeded", map[string]string{"payment_requirement_id": requirementID, "endpoint_id": plan.EndpointID, "mode": plan.Mode}); err != nil {
		return billing.CheckoutPlan{}, nil, err
	}
	if err := tx.Commit(); err != nil {
		return billing.CheckoutPlan{}, nil, err
	}
	return plan, nil, nil
}

func (s *Store) SetBillingCustomer(ctx context.Context, session platform.PortalSession, customerRef, provider string, _ time.Time) error {
	if provider != "stripe" {
		return platform.ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	accountID, err := ids.New("bac")
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO billing_accounts(id,organisation_id,provider,provider_customer_ref,legal_name) SELECT $1,o.id,$2,$3,o.name FROM organisations o WHERE o.id=$4 ON CONFLICT(organisation_id) DO UPDATE SET updated_at=now() WHERE billing_accounts.provider=EXCLUDED.provider AND billing_accounts.provider_customer_ref=EXCLUDED.provider_customer_ref`, accountID, provider, customerRef, session.Current.OrganisationID)
	if err != nil {
		return mapWriteError("record billing customer", err)
	}
	if err := requireAffected(result); err != nil {
		return platform.ErrConflict
	}
	if err := insertActorAudit(ctx, tx, "human_user", session.User.ID, session.Current.OrganisationID, session.Current.ProjectID, "billing.customer_linked", "succeeded", map[string]string{"provider": provider}); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) CompleteCheckout(ctx context.Context, session platform.PortalSession, plan billing.CheckoutPlan, hosted billing.HostedResult, _ time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE billing_checkout_sessions SET provider_session_ref=$2,state='open',expires_at=$3,updated_at=now() WHERE id=$1 AND organisation_id=$4 AND payment_requirement_id=$5 AND state IN ('creating','open') AND (provider_session_ref IS NULL OR provider_session_ref=$2)`, plan.OperationID, hosted.ProviderSessionRef, hosted.ExpiresAt, session.Current.OrganisationID, plan.PaymentRequirementID)
	if err != nil {
		return mapWriteError("complete hosted checkout", err)
	}
	if err := requireAffected(result); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) PrepareBillingPortal(ctx context.Context, session platform.PortalSession, digest [32]byte, _ time.Time) (billing.PortalPlan, error) {
	var result billing.PortalPlan
	if err := s.db.QueryRowContext(ctx, `SELECT organisation_id,provider_customer_ref FROM billing_accounts WHERE organisation_id=$1 AND provider='stripe' AND lifecycle_state='active'`, session.Current.OrganisationID).Scan(&result.OrganisationID, &result.ProviderCustomerRef); errors.Is(err, sql.ErrNoRows) {
		return billing.PortalPlan{}, platform.ErrNotFound
	} else if err != nil {
		return billing.PortalPlan{}, err
	}
	result.OperationKey = fmt.Sprintf("alzette-portal-%x", digest[:12])
	return result, nil
}

func (s *Store) ReceiveBillingEvent(ctx context.Context, event billing.Event) (bool, bool, error) {
	if event.Provider != "stripe" || event.ID == "" || event.Type == "" || event.ObjectRef == "" || event.ProviderCreatedAt.IsZero() || event.SignatureVerifiedAt.IsZero() {
		return false, false, platform.ErrInvalid
	}
	result, err := s.db.ExecContext(ctx, `INSERT INTO billing_webhook_receipts(provider,provider_event_id,event_type,provider_object_ref,payment_requirement_id,provider_customer_ref,provider_subscription_ref,provider_invoice_ref,object_status,payment_status,amount_minor,currency,period_start,period_end,cancel_at_period_end,provider_created_at,payload_digest,signature_verified_at) VALUES($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),NULLIF($7,''),NULLIF($8,''),NULLIF($9,''),NULLIF($10,''),$11,NULLIF($12,''),$13,$14,$15,$16,$17,$18) ON CONFLICT(provider,provider_event_id) DO NOTHING`, event.Provider, event.ID, event.Type, event.ObjectRef, event.PaymentRequirementID, event.CustomerRef, event.SubscriptionRef, event.InvoiceRef, event.ObjectStatus, event.PaymentStatus, event.AmountMinor, event.Currency, event.PeriodStart, event.PeriodEnd, event.CancelAtPeriodEnd, event.ProviderCreatedAt, event.PayloadDigest[:], event.SignatureVerifiedAt)
	if err != nil {
		return false, false, mapWriteError("record billing webhook receipt", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, false, err
	}
	if affected == 1 {
		return false, false, nil
	}
	var digest []byte
	var eventType, objectRef, state string
	if err := s.db.QueryRowContext(ctx, `SELECT payload_digest,event_type,provider_object_ref,processing_state FROM billing_webhook_receipts WHERE provider=$1 AND provider_event_id=$2`, event.Provider, event.ID).Scan(&digest, &eventType, &objectRef, &state); err != nil {
		return true, false, err
	}
	if eventType != event.Type || objectRef != event.ObjectRef || !equalBytes(digest, event.PayloadDigest[:]) {
		return true, false, platform.ErrInvalid
	}
	return true, state == "processed", nil
}

func equalBytes(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	var different byte
	for index := range left {
		different |= left[index] ^ right[index]
	}
	return different == 0
}

type storedBillingEvent struct {
	provider, id, eventType, objectRef, paymentID, customerRef, subscriptionRef, invoiceRef string
	objectStatus, paymentStatus, currency                                                   string
	amount                                                                                  sql.NullInt64
	periodStart, periodEnd                                                                  sql.NullTime
	cancelAtPeriodEnd                                                                       sql.NullBool
	createdAt                                                                               time.Time
	state                                                                                   string
}

func (s *Store) ApplyBillingEvent(ctx context.Context, provider, eventID string, now time.Time) error {
	if err := s.applyBillingEventOnce(ctx, provider, eventID, now); err != nil {
		return err
	}
	// A linking Checkout/subscription event can make an earlier invoice event
	// resolvable. Reconcile a bounded batch; receipts remain durable if more work
	// is required and Stripe delivery never controls gateway availability.
	rows, err := s.db.QueryContext(ctx, `SELECT provider_event_id FROM billing_webhook_receipts WHERE provider=$1 AND processing_state='deferred' ORDER BY provider_created_at LIMIT 32`, provider)
	if err != nil {
		return err
	}
	deferred := make([]string, 0, 32)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		if id != eventID {
			deferred = append(deferred, id)
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, id := range deferred {
		if err := s.applyBillingEventOnce(ctx, provider, id, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) applyBillingEventOnce(ctx context.Context, provider, eventID string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var event storedBillingEvent
	var paymentID, customerRef, subscriptionRef, invoiceRef, objectStatus, paymentStatus, currency sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT provider,provider_event_id,event_type,provider_object_ref,payment_requirement_id,provider_customer_ref,provider_subscription_ref,provider_invoice_ref,object_status,payment_status,amount_minor,currency,period_start,period_end,cancel_at_period_end,provider_created_at,processing_state FROM billing_webhook_receipts WHERE provider=$1 AND provider_event_id=$2 FOR UPDATE`, provider, eventID).Scan(&event.provider, &event.id, &event.eventType, &event.objectRef, &paymentID, &customerRef, &subscriptionRef, &invoiceRef, &objectStatus, &paymentStatus, &event.amount, &currency, &event.periodStart, &event.periodEnd, &event.cancelAtPeriodEnd, &event.createdAt, &event.state)
	if errors.Is(err, sql.ErrNoRows) {
		return platform.ErrNotFound
	}
	if err != nil {
		return err
	}
	if event.state == "processed" {
		return nil
	}
	event.paymentID, event.customerRef, event.subscriptionRef, event.invoiceRef = paymentID.String, customerRef.String, subscriptionRef.String, invoiceRef.String
	event.objectStatus, event.paymentStatus, event.currency = objectStatus.String, paymentStatus.String, currency.String
	if event.paymentID == "" {
		event.paymentID, err = resolveBillingPaymentReference(ctx, tx, event)
		if err != nil {
			return err
		}
	}
	if event.paymentID == "" {
		_, err := tx.ExecContext(ctx, `UPDATE billing_webhook_receipts SET processing_state='deferred',processing_attempts=processing_attempts+1,safe_failure_class='unmatched_reference',updated_at=now() WHERE provider=$1 AND provider_event_id=$2`, provider, eventID)
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	var orgID, projectID, environmentID, endpointID, requirementState, requirementCurrency, collectionMode string
	var requirementAmount int64
	var latestAt sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT organisation_id,project_id,environment_id,endpoint_id,state,currency,amount_minor,collection_mode,latest_event_at FROM payment_requirements WHERE id=$1 FOR UPDATE`, event.paymentID).Scan(&orgID, &projectID, &environmentID, &endpointID, &requirementState, &requirementCurrency, &requirementAmount, &collectionMode, &latestAt)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.ExecContext(ctx, `UPDATE billing_webhook_receipts SET processing_state='rejected',processing_attempts=processing_attempts+1,safe_failure_class='unknown_requirement',updated_at=now() WHERE provider=$1 AND provider_event_id=$2`, provider, eventID)
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	if err != nil {
		return err
	}
	if event.customerRef != "" {
		var accountOrg string
		if err := tx.QueryRowContext(ctx, `SELECT organisation_id FROM billing_accounts WHERE provider=$1 AND provider_customer_ref=$2`, provider, event.customerRef).Scan(&accountOrg); err != nil || accountOrg != orgID {
			return rejectBillingEvent(ctx, tx, event, "customer_mismatch")
		}
	}
	if latestAt.Valid && event.createdAt.Before(latestAt.Time) {
		return finishBillingReceipt(ctx, tx, event, "out_of_order_ignored", now)
	}
	if event.amount.Valid && event.currency != "" && (event.amount.Int64 != requirementAmount || event.currency != requirementCurrency) && (event.eventType == "invoice.paid" || event.eventType == "checkout.session.completed") {
		return rejectBillingEvent(ctx, tx, event, "commercial_mismatch")
	}
	if err := applyBillingEventEffects(ctx, tx, event, orgID, projectID, environmentID, endpointID, requirementState, requirementCurrency, requirementAmount, collectionMode, now); err != nil {
		return err
	}
	return finishBillingReceipt(ctx, tx, event, "", now)
}

func resolveBillingPaymentReference(ctx context.Context, tx *sql.Tx, event storedBillingEvent) (string, error) {
	var result string
	queries := []struct {
		query string
		value string
	}{
		{`SELECT payment_requirement_id FROM billing_checkout_sessions WHERE provider=$1 AND provider_session_ref=$2`, event.objectRef},
		{`SELECT payment_requirement_id FROM billing_subscriptions WHERE provider=$1 AND provider_subscription_ref=$2`, event.subscriptionRef},
		{`SELECT payment_requirement_id FROM billing_invoices WHERE provider=$1 AND provider_invoice_ref=$2`, event.invoiceRef},
	}
	for _, candidate := range queries {
		if candidate.value == "" {
			continue
		}
		err := tx.QueryRowContext(ctx, candidate.query, event.provider, candidate.value).Scan(&result)
		if err == nil {
			return result, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
	}
	return "", nil
}

func applyBillingEventEffects(ctx context.Context, tx *sql.Tx, event storedBillingEvent, orgID, projectID, environmentID, endpointID, requirementState, currency string, amount int64, collectionMode string, now time.Time) error {
	switch event.eventType {
	case "checkout.session.completed":
		_, _ = tx.ExecContext(ctx, `UPDATE billing_checkout_sessions SET state='complete',completed_at=COALESCE(completed_at,$2),updated_at=now() WHERE provider=$1 AND provider_session_ref=$3`, event.provider, now, event.objectRef)
		if requirementState != "paid" {
			if _, err := tx.ExecContext(ctx, `UPDATE payment_requirements SET state='processing',latest_event_at=$2,latest_event_id=$3,updated_at=now() WHERE id=$1`, event.paymentID, event.createdAt, event.id); err != nil {
				return err
			}
			_, _ = tx.ExecContext(ctx, `UPDATE customer_endpoints SET commercial_state='processing',latest_payment_event_at=$2,latest_payment_event_id=$3,updated_at=now() WHERE id=$1`, endpointID, event.createdAt, event.id)
		}
	case "checkout.session.expired":
		_, _ = tx.ExecContext(ctx, `UPDATE billing_checkout_sessions SET state='expired',updated_at=now() WHERE provider=$1 AND provider_session_ref=$2 AND state<>'complete'`, event.provider, event.objectRef)
		if requirementState != "paid" {
			_, _ = tx.ExecContext(ctx, `UPDATE payment_requirements SET state='action_required',latest_event_at=$2,latest_event_id=$3,updated_at=now() WHERE id=$1`, event.paymentID, event.createdAt, event.id)
		}
	case "invoice.paid":
		if err := upsertBillingInvoice(ctx, tx, event, orgID, endpointID, amount, currency, "paid", now); err != nil {
			return err
		}
		if err := markPaymentPaid(ctx, tx, event, orgID, projectID, environmentID, endpointID, amount, currency, now); err != nil {
			return err
		}
	case "invoice.finalized":
		if err := upsertBillingInvoice(ctx, tx, event, orgID, endpointID, amount, currency, "open", now); err != nil {
			return err
		}
	case "invoice.payment_failed":
		if requirementState != "paid" {
			_, _ = tx.ExecContext(ctx, `UPDATE payment_requirements SET state='past_due',safe_failure_class='payment_failed',latest_event_at=$2,latest_event_id=$3,updated_at=now() WHERE id=$1`, event.paymentID, event.createdAt, event.id)
		}
		_, _ = tx.ExecContext(ctx, `UPDATE customer_endpoints SET commercial_state='past_due',latest_payment_event_at=$2,latest_payment_event_id=$3,updated_at=now() WHERE id=$1`, endpointID, event.createdAt, event.id)
	case "customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted":
		if err := upsertBillingSubscription(ctx, tx, event, orgID, endpointID, amount, currency, collectionMode); err != nil {
			return err
		}
	case "charge.refunded":
		_, _ = tx.ExecContext(ctx, `UPDATE payment_requirements SET state='refunded',latest_event_at=$2,latest_event_id=$3,updated_at=now() WHERE id=$1`, event.paymentID, event.createdAt, event.id)
		_, _ = tx.ExecContext(ctx, `UPDATE customer_endpoints SET commercial_state='refunded',latest_payment_event_at=$2,latest_payment_event_id=$3,updated_at=now() WHERE id=$1`, endpointID, event.createdAt, event.id)
	case "charge.dispute.created":
		_, _ = tx.ExecContext(ctx, `UPDATE payment_requirements SET state='disputed',latest_event_at=$2,latest_event_id=$3,updated_at=now() WHERE id=$1`, event.paymentID, event.createdAt, event.id)
		_, _ = tx.ExecContext(ctx, `UPDATE customer_endpoints SET commercial_state='disputed',latest_payment_event_at=$2,latest_payment_event_id=$3,updated_at=now() WHERE id=$1`, endpointID, event.createdAt, event.id)
	case "charge.dispute.closed":
		// A closed dispute is not proof of payment outcome. Retain the last paid
		// state if present; otherwise require operator/provider reconciliation.
		if requirementState != "paid" {
			_, _ = tx.ExecContext(ctx, `UPDATE payment_requirements SET state='processing',latest_event_at=$2,latest_event_id=$3,updated_at=now() WHERE id=$1`, event.paymentID, event.createdAt, event.id)
		}
	}
	return nil
}

func markPaymentPaid(ctx context.Context, tx *sql.Tx, event storedBillingEvent, orgID, projectID, environmentID, endpointID string, amount int64, currency string, now time.Time) error {
	if _, err := tx.ExecContext(ctx, `UPDATE payment_requirements SET state='paid',paid_at=COALESCE(paid_at,$2),paid_amount_minor=$3,paid_currency=$4,safe_failure_class=NULL,latest_event_at=$5,latest_event_id=$6,updated_at=now() WHERE id=$1`, event.paymentID, event.createdAt, amount, currency, event.createdAt, event.id); err != nil {
		return err
	}
	var serviceMode string
	if err := tx.QueryRowContext(ctx, `SELECT service_mode FROM customer_endpoints WHERE id=$1`, endpointID).Scan(&serviceMode); err != nil {
		return err
	}
	if serviceMode == "shared_subscription" {
		offer, err := resolveOfferByEndpoint(ctx, tx, endpointID)
		if err != nil {
			return err
		}
		scope := platform.PortalMembership{OrganisationID: orgID, ProjectID: projectID, EnvironmentID: environmentID}
		if err := activateSharedEndpoint(ctx, tx, scope, endpointID, offer, now); err != nil {
			return err
		}
	}
	_, err := tx.ExecContext(ctx, `UPDATE customer_endpoints SET commercial_state='paid',latest_payment_event_at=$2,latest_payment_event_id=$3,allowance_period_start=CASE WHEN allowance_period='month' THEN COALESCE($4,allowance_period_start) ELSE allowance_period_start END,allowance_period_end=CASE WHEN allowance_period='month' THEN COALESCE($5,allowance_period_end) ELSE allowance_period_end END,updated_at=now() WHERE id=$1`, endpointID, event.createdAt, event.id, nullTimeValue(event.periodStart), nullTimeValue(event.periodEnd))
	return err
}

func resolveOfferByEndpoint(ctx context.Context, tx *sql.Tx, endpointID string) (resolvedOffer, error) {
	var result resolvedOffer
	err := tx.QueryRowContext(ctx, `SELECT o.id,o.offer_kind,o.deployment_profile_id,o.routable_model_id,m.alias,cm.slug,cm.name,v.version,p.code,p.service_mode,p.execution_class,p.min_capacity_units,p.max_capacity_units,o.target_id,o.request_allowance,o.token_allowance,o.allowance_period,o.source_label,o.evidence_ref,o.profile_price_id,pr.currency,pr.billing_period,pr.recurring_unit_amount_minor,pr.setup_amount_minor,pr.finality FROM customer_endpoints ce JOIN endpoint_offers o ON o.id=ce.offer_id JOIN deployment_profiles p ON p.id=o.deployment_profile_id JOIN catalogue_model_versions v ON v.id=p.catalogue_model_version_id JOIN catalogue_models cm ON cm.id=v.catalogue_model_id JOIN models m ON m.id=o.routable_model_id LEFT JOIN deployment_profile_prices pr ON pr.id=o.profile_price_id WHERE ce.id=$1 FOR UPDATE OF ce,o`, endpointID).Scan(&result.id, &result.kind, &result.profileID, &result.modelID, &result.modelAlias, &result.modelSlug, &result.modelName, &result.releaseVersion, &result.profileCode, &result.profileServiceMode, &result.executionClass, &result.minUnits, &result.maxUnits, &result.targetID, &result.requestAllowance, &result.tokenAllowance, &result.allowancePeriod, &result.sourceLabel, &result.evidenceRef, &result.priceID, &result.currency, &result.billingPeriod, &result.recurringAmount, &result.setupAmount, &result.priceFinality)
	return result, err
}

func upsertBillingSubscription(ctx context.Context, tx *sql.Tx, event storedBillingEvent, orgID, endpointID string, amount int64, currency, collectionMode string) error {
	if event.subscriptionRef == "" {
		return nil
	}
	status := event.objectStatus
	if event.eventType == "customer.subscription.deleted" {
		status = "cancelled"
	}
	switch status {
	case "incomplete", "trialing", "active", "past_due", "unpaid", "paused", "cancelled":
	default:
		status = "incomplete"
	}
	id, err := ids.New("sub")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO billing_subscriptions(id,organisation_id,endpoint_id,payment_requirement_id,provider,provider_subscription_ref,provider_customer_ref,status,current_period_start,current_period_end,cancel_at_period_end,fixed_amount_minor,currency,billing_period,latest_event_at,latest_event_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,COALESCE($11,false),$12,$13,'month',$14,$15) ON CONFLICT(provider,provider_subscription_ref) DO UPDATE SET status=EXCLUDED.status,current_period_start=EXCLUDED.current_period_start,current_period_end=EXCLUDED.current_period_end,cancel_at_period_end=EXCLUDED.cancel_at_period_end,latest_event_at=EXCLUDED.latest_event_at,latest_event_id=EXCLUDED.latest_event_id,updated_at=now() WHERE billing_subscriptions.organisation_id=EXCLUDED.organisation_id AND billing_subscriptions.payment_requirement_id=EXCLUDED.payment_requirement_id AND billing_subscriptions.latest_event_at<=EXCLUDED.latest_event_at`, id, orgID, endpointID, event.paymentID, event.provider, event.subscriptionRef, event.customerRef, status, nullTimeValue(event.periodStart), nullTimeValue(event.periodEnd), nullBoolValue(event.cancelAtPeriodEnd), amount, currency, event.createdAt, event.id)
	return err
}

func upsertBillingInvoice(ctx context.Context, tx *sql.Tx, event storedBillingEvent, orgID, endpointID string, amount int64, currency, status string, now time.Time) error {
	if event.invoiceRef == "" {
		return nil
	}
	id, err := ids.New("inv")
	if err != nil {
		return err
	}
	paidAmount := int64(0)
	var paidAt interface{}
	if status == "paid" {
		paidAmount, paidAt = amount, now
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO billing_invoices(id,organisation_id,endpoint_id,payment_requirement_id,provider,provider_invoice_ref,provider_subscription_ref,status,amount_due_minor,amount_paid_minor,currency,paid_at,latest_event_at,latest_event_id) VALUES($1,$2,$3,$4,$5,$6,NULLIF($7,''),$8,$9,$10,$11,$12,$13,$14) ON CONFLICT(provider,provider_invoice_ref) DO UPDATE SET status=EXCLUDED.status,amount_due_minor=EXCLUDED.amount_due_minor,amount_paid_minor=EXCLUDED.amount_paid_minor,currency=EXCLUDED.currency,paid_at=COALESCE(billing_invoices.paid_at,EXCLUDED.paid_at),latest_event_at=EXCLUDED.latest_event_at,latest_event_id=EXCLUDED.latest_event_id,updated_at=now() WHERE billing_invoices.organisation_id=EXCLUDED.organisation_id AND billing_invoices.payment_requirement_id=EXCLUDED.payment_requirement_id AND billing_invoices.latest_event_at<=EXCLUDED.latest_event_at`, id, orgID, endpointID, event.paymentID, event.provider, event.invoiceRef, event.subscriptionRef, status, amount, paidAmount, currency, paidAt, event.createdAt, event.id)
	return err
}

func rejectBillingEvent(ctx context.Context, tx *sql.Tx, event storedBillingEvent, failure string) error {
	_, err := tx.ExecContext(ctx, `UPDATE billing_webhook_receipts SET processing_state='rejected',processing_attempts=processing_attempts+1,safe_failure_class=$3,updated_at=now() WHERE provider=$1 AND provider_event_id=$2`, event.provider, event.id, failure)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func finishBillingReceipt(ctx context.Context, tx *sql.Tx, event storedBillingEvent, safeClass string, now time.Time) error {
	_, err := tx.ExecContext(ctx, `UPDATE billing_webhook_receipts SET payment_requirement_id=COALESCE(payment_requirement_id,$3),processing_state='processed',processing_attempts=processing_attempts+1,safe_failure_class=NULLIF($4,''),processed_at=$5,updated_at=now() WHERE provider=$1 AND provider_event_id=$2`, event.provider, event.id, event.paymentID, safeClass, now)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func nullTimeValue(value sql.NullTime) interface{} {
	if !value.Valid {
		return nil
	}
	return value.Time
}

func nullBoolValue(value sql.NullBool) interface{} {
	if !value.Valid {
		return nil
	}
	return value.Bool
}

var _ billing.Store = (*Store)(nil)
var _ billing.WebhookStore = (*Store)(nil)
