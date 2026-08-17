package postgres

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"alzette/internal/endpoints"
	"alzette/internal/ids"
	"alzette/internal/platform"
)

const deploymentRequestSelect = `SELECT dr.id,COALESCE(c.id,''),COALESCE(ce.id,''),dr.request_kind,dr.status,p.code,
	cm.slug,cm.name,v.version,COALESCE(ce.endpoint_alias,''),
	dr.current_capacity_units,dr.requested_capacity_units,dr.workload_use_case,dr.expected_context_tokens,
	dr.expected_concurrency,dr.expected_requests_per_minute,dr.latency_priority,dr.expected_monthly_requests,
	dr.quote_id,pr.id,dr.submitted_at,dr.approved_at,dr.completed_at
	FROM deployment_requests dr
	JOIN deployment_profiles p ON p.id=dr.deployment_profile_id
	JOIN catalogue_model_versions v ON v.id=p.catalogue_model_version_id
	JOIN catalogue_models cm ON cm.id=v.catalogue_model_id
	LEFT JOIN endpoint_configurations c ON c.organisation_id=dr.organisation_id AND c.project_id=dr.project_id
	 AND c.environment_id=dr.environment_id AND c.deployment_request_id=dr.id
	LEFT JOIN LATERAL (
	 SELECT x.* FROM customer_endpoints x
	 WHERE x.organisation_id=dr.organisation_id AND x.project_id=dr.project_id AND x.environment_id=dr.environment_id
	   AND (x.configuration_id=c.id OR (dr.deployment_id IS NOT NULL AND x.deployment_id=dr.deployment_id))
	 ORDER BY x.created_at LIMIT 1
	) ce ON true
	LEFT JOIN LATERAL (
	 SELECT x.id FROM payment_requirements x
	 WHERE x.organisation_id=dr.organisation_id AND x.project_id=dr.project_id AND x.environment_id=dr.environment_id
	   AND (x.deployment_request_id=dr.id OR x.quote_id=dr.quote_id)
	 ORDER BY x.created_at DESC LIMIT 1
	) pr ON true`

func (s *Store) GetDeploymentRequest(ctx context.Context, session platform.PortalSession, id string) (endpoints.DeploymentRequest, error) {
	row := s.db.QueryRowContext(ctx, deploymentRequestSelect+` WHERE dr.organisation_id=$1 AND dr.project_id=$2 AND dr.environment_id=$3 AND dr.id=$4`, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID, id)
	return scanDeploymentRequest(row)
}

func scanDeploymentRequest(row rowScanner) (endpoints.DeploymentRequest, error) {
	var result endpoints.DeploymentRequest
	var current, contextTokens, monthlyRequests sql.NullInt64
	var concurrency, requestsPerMinute sql.NullInt64
	var latencyPriority sql.NullString
	var quoteID, paymentID sql.NullString
	var submitted, approved, completed sql.NullTime
	if err := row.Scan(&result.ID, &result.ConfigurationID, &result.EndpointID, &result.Kind, &result.Status,
		&result.ProfileCode, &result.ModelSlug, &result.ModelName, &result.ModelVersion, &result.EndpointAlias,
		&current, &result.RequestedCapacityUnits, &result.Workload.UseCase,
		&contextTokens, &concurrency, &requestsPerMinute, &latencyPriority, &monthlyRequests,
		&quoteID, &paymentID, &submitted, &approved, &completed); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return endpoints.DeploymentRequest{}, platform.ErrNotFound
		}
		return endpoints.DeploymentRequest{}, err
	}
	if current.Valid {
		value := int(current.Int64)
		result.CurrentCapacityUnits = &value
	}
	result.Workload.ExpectedContextTokens = nullInt64Pointer(contextTokens)
	result.Workload.ExpectedMonthlyRequests = nullInt64Pointer(monthlyRequests)
	if concurrency.Valid {
		value := int(concurrency.Int64)
		result.Workload.ExpectedConcurrency = &value
	}
	if requestsPerMinute.Valid {
		value := int(requestsPerMinute.Int64)
		result.Workload.ExpectedRequestsPerMinute = &value
	}
	result.Workload.LatencyPriority = nullStringPointer(latencyPriority)
	result.QuoteID = nullStringPointer(quoteID)
	result.PaymentRequirementID = nullStringPointer(paymentID)
	result.SubmittedAt = nullTimePointer(submitted)
	result.ApprovedAt = nullTimePointer(approved)
	result.CompletedAt = nullTimePointer(completed)
	return result, nil
}

const deploymentQuoteSelect = `SELECT q.id,q.quote_version,q.quote_kind,p.code,q.capacity_units,q.service_mode_snapshot,
	q.execution_class_snapshot,q.accelerator_class_snapshot,q.accelerator_count_snapshot,q.capacity_snapshot,
	q.currency,q.billing_period,q.recurring_unit_amount_minor,q.recurring_total_amount_minor,q.setup_total_amount_minor,
	q.tax_treatment,q.price_finality,q.status,terms.collection_mode,terms.payment_due_days,q.source_label,
	q.offered_at,q.expires_at,q.accepted_at
	FROM deployment_quotes q
	JOIN deployment_profiles p ON p.id=q.deployment_profile_id
	JOIN deployment_quote_payment_terms terms ON terms.organisation_id=q.organisation_id AND terms.project_id=q.project_id
	 AND terms.environment_id=q.environment_id AND terms.quote_id=q.id`

func (s *Store) GetDeploymentQuote(ctx context.Context, session platform.PortalSession, id string) (endpoints.Quote, error) {
	row := s.db.QueryRowContext(ctx, deploymentQuoteSelect+` WHERE q.organisation_id=$1 AND q.project_id=$2 AND q.environment_id=$3 AND q.id=$4`, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID, id)
	return scanDeploymentQuote(row)
}

func scanDeploymentQuote(row rowScanner) (endpoints.Quote, error) {
	var result endpoints.Quote
	var acceleratorClass sql.NullString
	var acceleratorCount, dueDays sql.NullInt64
	var capacityJSON []byte
	var accepted sql.NullTime
	if err := row.Scan(&result.ID, &result.Version, &result.Kind, &result.ProfileCode, &result.CapacityUnits,
		&result.ServiceMode, &result.ExecutionClass, &acceleratorClass, &acceleratorCount, &capacityJSON,
		&result.Currency, &result.BillingPeriod, &result.RecurringUnitAmountMinor, &result.RecurringTotalAmountMinor,
		&result.SetupTotalAmountMinor, &result.TaxTreatment, &result.PriceFinality, &result.Status,
		&result.CollectionMode, &dueDays, &result.Source, &result.OfferedAt, &result.ExpiresAt, &accepted); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return endpoints.Quote{}, platform.ErrNotFound
		}
		return endpoints.Quote{}, err
	}
	result.AcceleratorClass = nullStringPointer(acceleratorClass)
	if acceleratorCount.Valid {
		value := int(acceleratorCount.Int64)
		result.AcceleratorCount = &value
	}
	if dueDays.Valid {
		value := int(dueDays.Int64)
		result.PaymentDueDays = &value
	}
	result.AcceptedAt = nullTimePointer(accepted)
	var capacity map[string]interface{}
	if err := json.Unmarshal(capacityJSON, &capacity); err != nil {
		return endpoints.Quote{}, err
	}
	result.Capacity = capacity
	return result, nil
}

func (s *Store) AcceptDeploymentQuote(ctx context.Context, session platform.PortalSession, quoteID string, _ [32]byte, now time.Time) (endpoints.AcceptResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return endpoints.AcceptResult{}, err
	}
	defer tx.Rollback()
	var requestID, endpointID, quoteStatus, collectionMode string
	var expiresAt time.Time
	var priceFinality, currency, billingPeriod, taxTreatment, sourceLabel, evidenceRef string
	var recurringTotal, setupTotal int64
	var dueDays sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT dr.id,ce.id,q.status,q.expires_at,q.price_finality,q.currency,q.billing_period,
		q.recurring_total_amount_minor,q.setup_total_amount_minor,q.tax_treatment,q.source_label,q.evidence_ref,
		terms.collection_mode,terms.payment_due_days
		FROM deployment_quotes q
		JOIN deployment_requests dr ON dr.organisation_id=q.organisation_id AND dr.project_id=q.project_id
		 AND dr.environment_id=q.environment_id AND dr.quote_id=q.id
		JOIN endpoint_configurations c ON c.deployment_request_id=dr.id
		JOIN customer_endpoints ce ON ce.configuration_id=c.id
		JOIN deployment_quote_payment_terms terms ON terms.organisation_id=q.organisation_id AND terms.project_id=q.project_id
		 AND terms.environment_id=q.environment_id AND terms.quote_id=q.id
		WHERE q.organisation_id=$1 AND q.project_id=$2 AND q.environment_id=$3 AND q.id=$4
		FOR UPDATE OF q,dr,ce`, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID, quoteID).Scan(
		&requestID, &endpointID, &quoteStatus, &expiresAt, &priceFinality, &currency, &billingPeriod,
		&recurringTotal, &setupTotal, &taxTreatment, &sourceLabel, &evidenceRef, &collectionMode, &dueDays)
	if errors.Is(err, sql.ErrNoRows) {
		return endpoints.AcceptResult{}, platform.ErrNotFound
	}
	if err != nil {
		return endpoints.AcceptResult{}, err
	}
	if quoteStatus == "accepted" {
		return acceptedQuoteResult(ctx, tx, session, quoteID, endpointID)
	}
	if quoteStatus != "offered" || !expiresAt.After(now) || priceFinality != "contractual" || evidenceRef == "" {
		return endpoints.AcceptResult{}, platform.ErrConflict
	}
	if _, err := tx.ExecContext(ctx, `UPDATE deployment_quotes SET status='accepted',accepted_at=$2,accepted_by_user_id=$3,updated_at=now() WHERE id=$1`, quoteID, now, session.User.ID); err != nil {
		return endpoints.AcceptResult{}, mapWriteError("accept deployment quote", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE deployment_requests SET status='accepted',updated_at=now() WHERE id=$1`, requestID); err != nil {
		return endpoints.AcceptResult{}, err
	}
	commercial, runtimeState := "quote_accepted", "awaiting_allocation"
	var payment *endpoints.PaymentRequirement
	if collectionMode != "not_required" {
		paymentID, err := ids.New("pay")
		if err != nil {
			return endpoints.AcceptResult{}, err
		}
		purpose, period, mode := "dedicated_recurring", billingPeriod, collectionMode
		amount := recurringTotal
		if collectionMode == "checkout_payment" {
			purpose, period, mode = "dedicated_setup", "one_time", "checkout_payment"
			amount += setupTotal
		}
		if period == "hour" {
			return endpoints.AcceptResult{}, platform.ErrConflict
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO payment_requirements(id,organisation_id,project_id,environment_id,endpoint_id,deployment_request_id,quote_id,purpose,amount_minor,currency,billing_period,tax_treatment,collection_mode,state,price_finality,source_label,evidence_ref) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,'action_required',$14,$15,$16)`, paymentID, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID, endpointID, requestID, quoteID, purpose, amount, currency, period, taxTreatment, mode, priceFinality, sourceLabel, evidenceRef)
		if err != nil {
			return endpoints.AcceptResult{}, mapWriteError("create quote payment requirement", err)
		}
		payment = &endpoints.PaymentRequirement{ID: paymentID, Purpose: purpose, State: "action_required", AmountMinor: amount, Currency: currency, BillingPeriod: period, TaxTreatment: taxTreatment, CollectionMode: mode, PriceFinality: priceFinality}
		commercial, runtimeState = "payment_action_required", "awaiting_payment"
	}
	if _, err := tx.ExecContext(ctx, `UPDATE customer_endpoints SET commercial_state=$2,runtime_state=$3,updated_at=now() WHERE id=$1`, endpointID, commercial, runtimeState); err != nil {
		return endpoints.AcceptResult{}, err
	}
	if err := insertActorAudit(ctx, tx, "human_user", session.User.ID, session.Current.OrganisationID, session.Current.ProjectID, "deployment_quote.accepted", "succeeded", map[string]string{"quote_id": quoteID, "deployment_request_id": requestID, "collection_mode": collectionMode}); err != nil {
		return endpoints.AcceptResult{}, err
	}
	quote, err := getDeploymentQuoteTx(ctx, tx, session, quoteID)
	if err != nil {
		return endpoints.AcceptResult{}, err
	}
	endpoint, err := getCustomerEndpoint(ctx, tx, session, endpointID)
	if err != nil {
		return endpoints.AcceptResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return endpoints.AcceptResult{}, err
	}
	return endpoints.AcceptResult{Quote: quote, Endpoint: endpoint, PaymentRequirement: payment}, nil
}

func acceptedQuoteResult(ctx context.Context, tx *sql.Tx, session platform.PortalSession, quoteID, endpointID string) (endpoints.AcceptResult, error) {
	quote, err := getDeploymentQuoteTx(ctx, tx, session, quoteID)
	if err != nil {
		return endpoints.AcceptResult{}, err
	}
	endpoint, err := getCustomerEndpoint(ctx, tx, session, endpointID)
	if err != nil {
		return endpoints.AcceptResult{}, err
	}
	return endpoints.AcceptResult{Quote: quote, Endpoint: endpoint, PaymentRequirement: endpoint.PaymentRequirement}, nil
}

func getDeploymentQuoteTx(ctx context.Context, tx *sql.Tx, session platform.PortalSession, id string) (endpoints.Quote, error) {
	return scanDeploymentQuote(tx.QueryRowContext(ctx, deploymentQuoteSelect+` WHERE q.organisation_id=$1 AND q.project_id=$2 AND q.environment_id=$3 AND q.id=$4`, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID, id))
}

func (s *Store) CreateCapacityRequest(ctx context.Context, session platform.PortalSession, endpointID string, units int, workload endpoints.Workload, digest [32]byte, now time.Time) (endpoints.DeploymentRequest, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return endpoints.DeploymentRequest{}, err
	}
	defer tx.Rollback()
	requestDigest := deploymentRequestDigest("capacity_change", endpointID, units, workload)
	var replayID, replayProjectID, replayEnvironmentID string
	var replayDigest []byte
	err = tx.QueryRowContext(ctx, `SELECT id,project_id,environment_id,idempotency_request_hash
		FROM deployment_requests WHERE organisation_id=$1 AND idempotency_key_hash=$2 FOR UPDATE`,
		session.Current.OrganisationID, digest[:]).Scan(&replayID, &replayProjectID, &replayEnvironmentID, &replayDigest)
	if err == nil {
		if replayProjectID != session.Current.ProjectID || replayEnvironmentID != session.Current.EnvironmentID || !bytes.Equal(replayDigest, requestDigest[:]) {
			return endpoints.DeploymentRequest{}, platform.ErrConflict
		}
		return scanDeploymentRequest(tx.QueryRowContext(ctx, deploymentRequestSelect+` WHERE dr.organisation_id=$1 AND dr.project_id=$2 AND dr.environment_id=$3 AND dr.id=$4`, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID, replayID))
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return endpoints.DeploymentRequest{}, err
	}
	var deploymentID, profileID string
	var current, minimum, maximum int
	err = tx.QueryRowContext(ctx, `SELECT ce.deployment_id,ce.deployment_profile_id,ce.capacity_units,p.min_capacity_units,p.max_capacity_units
		FROM customer_endpoints ce JOIN deployment_profiles p ON p.id=ce.deployment_profile_id
		WHERE ce.organisation_id=$1 AND ce.project_id=$2 AND ce.environment_id=$3 AND ce.id=$4
		 AND ce.service_mode='dedicated_private' AND ce.deployment_id IS NOT NULL AND ce.runtime_state IN ('ready','degraded')
		FOR UPDATE OF ce`, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID, endpointID).Scan(&deploymentID, &profileID, &current, &minimum, &maximum)
	if errors.Is(err, sql.ErrNoRows) {
		return endpoints.DeploymentRequest{}, platform.ErrNotFound
	}
	if err != nil {
		return endpoints.DeploymentRequest{}, err
	}
	if units == current || units < minimum || units > maximum {
		return endpoints.DeploymentRequest{}, platform.ErrInvalid
	}
	var existingID string
	var existingKey []byte
	err = tx.QueryRowContext(ctx, `SELECT id,idempotency_key_hash FROM deployment_requests
		WHERE deployment_id=$1 AND status IN ('submitted','quoted','accepted','approved','allocating','deploying','validating')
		FOR UPDATE`, deploymentID).Scan(&existingID, &existingKey)
	if err == nil {
		if !bytes.Equal(existingKey, digest[:]) {
			return endpoints.DeploymentRequest{}, platform.ErrConflict
		}
		return scanDeploymentRequest(tx.QueryRowContext(ctx, deploymentRequestSelect+` WHERE dr.id=$1`, existingID))
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return endpoints.DeploymentRequest{}, err
	}
	requestID, err := ids.New("drq")
	if err != nil {
		return endpoints.DeploymentRequest{}, err
	}
	kind := "scale_up"
	if units < current {
		kind = "scale_down"
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO deployment_requests(
		id,organisation_id,project_id,environment_id,request_kind,deployment_profile_id,deployment_id,
		current_capacity_units,requested_capacity_units,status,requested_by_user_id,submitted_at,
		workload_use_case,expected_context_tokens,expected_concurrency,expected_requests_per_minute,
		latency_priority,expected_monthly_requests,idempotency_key_hash,idempotency_request_hash)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'submitted',$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)`,
		requestID, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID,
		kind, profileID, deploymentID, current, units, session.User.ID, now, workload.UseCase,
		workload.ExpectedContextTokens, workload.ExpectedConcurrency, workload.ExpectedRequestsPerMinute,
		workload.LatencyPriority, workload.ExpectedMonthlyRequests, digest[:], requestDigest[:])
	if err != nil {
		return endpoints.DeploymentRequest{}, mapWriteError("create capacity request", err)
	}
	if err := insertActorAudit(ctx, tx, "human_user", session.User.ID, session.Current.OrganisationID, session.Current.ProjectID, "deployment_capacity.requested", "succeeded", map[string]string{"endpoint_id": endpointID, "deployment_request_id": requestID, "direction": kind, "requested_capacity_units": fmt.Sprintf("%d", units)}); err != nil {
		return endpoints.DeploymentRequest{}, err
	}
	result, err := scanDeploymentRequest(tx.QueryRowContext(ctx, deploymentRequestSelect+` WHERE dr.id=$1`, requestID))
	if err != nil {
		return endpoints.DeploymentRequest{}, err
	}
	if err := tx.Commit(); err != nil {
		return endpoints.DeploymentRequest{}, err
	}
	return result, nil
}

func (s *Store) IssueDeploymentQuote(ctx context.Context, spec endpoints.QuoteSpec) (endpoints.Quote, error) {
	if err := validateQuoteSpec(spec); err != nil {
		return endpoints.Quote{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return endpoints.Quote{}, err
	}
	defer tx.Rollback()
	var orgID, projectID, environmentID, requestKind, requestStatus, profileID string
	var units, acceleratorsPerUnit int
	var profileCode, serviceMode, executionClass, acceleratorClass, capacityFinality, profileSource, profileEvidence string
	err = tx.QueryRowContext(ctx, `SELECT dr.organisation_id,dr.project_id,dr.environment_id,dr.request_kind,dr.status,dr.deployment_profile_id,
		dr.requested_capacity_units,p.code,p.service_mode,p.execution_class,COALESCE(p.accelerator_class,''),COALESCE(p.accelerators_per_unit,0),p.capacity_finality,p.source_label,COALESCE(p.evidence_ref,'')
		FROM deployment_requests dr JOIN deployment_profiles p ON p.id=dr.deployment_profile_id
		WHERE dr.id=$1 FOR UPDATE OF dr,p`, spec.RequestID).Scan(&orgID, &projectID, &environmentID, &requestKind, &requestStatus, &profileID, &units, &profileCode, &serviceMode, &executionClass, &acceleratorClass, &acceleratorsPerUnit, &capacityFinality, &profileSource, &profileEvidence)
	if errors.Is(err, sql.ErrNoRows) {
		return endpoints.Quote{}, platform.ErrNotFound
	}
	if err != nil {
		return endpoints.Quote{}, err
	}
	if requestStatus != "submitted" && requestStatus != "quoted" {
		return endpoints.Quote{}, platform.ErrConflict
	}
	if serviceMode == "dedicated_private" && (acceleratorClass == "" || acceleratorsPerUnit < 1 || capacityFinality == "unknown" || profileEvidence == "") {
		return endpoints.Quote{}, platform.ErrConflict
	}
	if _, err := tx.ExecContext(ctx, `UPDATE deployment_quotes SET status='superseded',updated_at=now() WHERE organisation_id=$1 AND project_id=$2 AND environment_id=$3 AND id=(SELECT quote_id FROM deployment_requests WHERE id=$4) AND status='offered'`, orgID, projectID, environmentID, spec.RequestID); err != nil {
		return endpoints.Quote{}, err
	}
	var version int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(q.quote_version),0)+1 FROM deployment_quotes q JOIN deployment_requests r ON r.quote_id=q.id OR (r.organisation_id=q.organisation_id AND r.project_id=q.project_id AND r.environment_id=q.environment_id AND r.deployment_profile_id=q.deployment_profile_id AND r.requested_capacity_units=q.capacity_units AND r.request_kind=q.quote_kind) WHERE r.id=$1`, spec.RequestID).Scan(&version); err != nil {
		return endpoints.Quote{}, err
	}
	quoteID, err := ids.New("quo")
	if err != nil {
		return endpoints.Quote{}, err
	}
	capacity := map[string]interface{}{"capacity_units": units, "finality": capacityFinality, "source": profileSource}
	var acceleratorClassValue, acceleratorCountValue interface{}
	if acceleratorClass != "" {
		acceleratorClassValue = acceleratorClass
		acceleratorCountValue = acceleratorsPerUnit * units
		capacity["accelerator_class"] = acceleratorClass
		capacity["accelerator_count"] = acceleratorsPerUnit * units
	}
	capacityJSON, _ := json.Marshal(capacity)
	recurringTotal := spec.RecurringUnitAmountMinor * int64(units)
	_, err = tx.ExecContext(ctx, `INSERT INTO deployment_quotes(id,organisation_id,project_id,environment_id,quote_version,quote_kind,deployment_profile_id,capacity_units,service_mode_snapshot,execution_class_snapshot,accelerator_class_snapshot,accelerator_count_snapshot,capacity_snapshot,currency,billing_period,recurring_unit_amount_minor,recurring_total_amount_minor,setup_total_amount_minor,tax_treatment,price_finality,status,source_label,evidence_ref,offered_at,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,'offered',$21,$22,now(),$23)`, quoteID, orgID, projectID, environmentID, version, requestKind, profileID, units, serviceMode, executionClass, acceleratorClassValue, acceleratorCountValue, capacityJSON, spec.Currency, spec.BillingPeriod, spec.RecurringUnitAmountMinor, recurringTotal, spec.SetupTotalAmountMinor, spec.TaxTreatment, spec.PriceFinality, spec.SourceLabel, spec.EvidenceRef, spec.ExpiresAt.UTC())
	if err != nil {
		return endpoints.Quote{}, mapWriteError("issue deployment quote", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO deployment_quote_payment_terms(organisation_id,project_id,environment_id,quote_id,collection_mode,payment_due_days,source_label,evidence_ref) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, orgID, projectID, environmentID, quoteID, spec.CollectionMode, spec.PaymentDueDays, spec.SourceLabel, spec.EvidenceRef); err != nil {
		return endpoints.Quote{}, mapWriteError("store deployment quote terms", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE deployment_requests SET quote_id=$2,status='quoted',reviewed_by='operator_cli',updated_at=now() WHERE id=$1`, spec.RequestID, quoteID); err != nil {
		return endpoints.Quote{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE customer_endpoints ce SET commercial_state='quote_offered',updated_at=now() FROM endpoint_configurations c WHERE c.deployment_request_id=$1 AND ce.configuration_id=c.id`, spec.RequestID); err != nil {
		return endpoints.Quote{}, err
	}
	if err := insertActorAudit(ctx, tx, "operator", "cli", orgID, projectID, "deployment_quote.issued", "succeeded", map[string]string{"deployment_request_id": spec.RequestID, "quote_id": quoteID, "profile_code": profileCode, "collection_mode": spec.CollectionMode}); err != nil {
		return endpoints.Quote{}, err
	}
	result, err := scanDeploymentQuote(tx.QueryRowContext(ctx, deploymentQuoteSelect+` WHERE q.id=$1`, quoteID))
	if err != nil {
		return endpoints.Quote{}, err
	}
	if err := tx.Commit(); err != nil {
		return endpoints.Quote{}, err
	}
	return result, nil
}

func validateQuoteSpec(spec endpoints.QuoteSpec) error {
	if strings.TrimSpace(spec.RequestID) == "" || len(spec.RequestID) > 192 || spec.Currency == "" || spec.RecurringUnitAmountMinor < 0 || spec.SetupTotalAmountMinor < 0 || spec.PriceFinality != "contractual" || spec.SourceLabel == "" || spec.EvidenceRef == "" || spec.ExpiresAt.IsZero() {
		return platform.ErrInvalid
	}
	if spec.BillingPeriod != "month" && spec.BillingPeriod != "contract_term" {
		return platform.ErrInvalid
	}
	if spec.TaxTreatment != "not_determined" && spec.TaxTreatment != "exclusive" && spec.TaxTreatment != "inclusive" && spec.TaxTreatment != "not_applicable" {
		return platform.ErrInvalid
	}
	switch spec.CollectionMode {
	case "checkout_payment", "invoice", "not_required":
		if spec.PaymentDueDays != nil {
			return platform.ErrInvalid
		}
	case "invoice_terms":
		if spec.PaymentDueDays == nil || *spec.PaymentDueDays < 1 || *spec.PaymentDueDays > 120 {
			return platform.ErrInvalid
		}
	default:
		return platform.ErrInvalid
	}
	return nil
}

func (s *Store) TransitionDeploymentRequest(ctx context.Context, spec endpoints.TransitionSpec) (endpoints.DeploymentRequest, error) {
	if spec.RequestID == "" || spec.State == "" || (spec.State == "ready" && (spec.TargetName == "" || spec.EvidenceRef == "")) {
		return endpoints.DeploymentRequest{}, platform.ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return endpoints.DeploymentRequest{}, err
	}
	defer tx.Rollback()
	var orgID, projectID, environmentID, status, kind, profileID, quoteID, endpointID, modelID string
	var deploymentID sql.NullString
	var capacityUnits int
	err = tx.QueryRowContext(ctx, `SELECT dr.organisation_id,dr.project_id,dr.environment_id,dr.status,dr.request_kind,
		dr.deployment_profile_id,dr.quote_id,dr.deployment_id,dr.requested_capacity_units,ce.id,ce.routable_model_id
		FROM deployment_requests dr
		LEFT JOIN endpoint_configurations c ON c.deployment_request_id=dr.id
		JOIN customer_endpoints ce ON ce.configuration_id=c.id OR (dr.deployment_id IS NOT NULL AND ce.deployment_id=dr.deployment_id)
		WHERE dr.id=$1 FOR UPDATE OF dr,ce`, spec.RequestID).Scan(&orgID, &projectID, &environmentID, &status, &kind, &profileID, &quoteID, &deploymentID, &capacityUnits, &endpointID, &modelID)
	if errors.Is(err, sql.ErrNoRows) {
		return endpoints.DeploymentRequest{}, platform.ErrNotFound
	}
	if err != nil {
		return endpoints.DeploymentRequest{}, err
	}
	if status == spec.State {
		return scanDeploymentRequest(tx.QueryRowContext(ctx, deploymentRequestSelect+` WHERE dr.id=$1`, spec.RequestID))
	}
	allowed := map[string]string{"accepted": "approved", "approved": "allocating", "allocating": "deploying", "deploying": "validating", "validating": "ready"}
	if spec.State != "failed" && allowed[status] != spec.State {
		return endpoints.DeploymentRequest{}, platform.ErrConflict
	}
	if spec.State == "approved" {
		var unpaid int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM payment_requirements WHERE deployment_request_id=$1 AND state<>'paid'`, spec.RequestID).Scan(&unpaid); err != nil {
			return endpoints.DeploymentRequest{}, err
		}
		if unpaid != 0 {
			return endpoints.DeploymentRequest{}, platform.ErrConflict
		}
		if kind == "new_endpoint" {
			newDeploymentID, err := ids.New("dep")
			if err != nil {
				return endpoints.DeploymentRequest{}, err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO model_deployments(id,organisation_id,project_id,environment_id,deployment_profile_id,quote_id,state) VALUES($1,$2,$3,$4,$5,$6,'requested')`, newDeploymentID, orgID, projectID, environmentID, profileID, quoteID); err != nil {
				return endpoints.DeploymentRequest{}, mapWriteError("create model deployment", err)
			}
			deploymentID = sql.NullString{String: newDeploymentID, Valid: true}
			if _, err := tx.ExecContext(ctx, `UPDATE customer_endpoints SET deployment_id=$2,commercial_state=CASE WHEN commercial_state='quote_accepted' THEN 'quote_accepted' ELSE commercial_state END,runtime_state='awaiting_allocation',updated_at=now() WHERE id=$1`, endpointID, newDeploymentID); err != nil {
				return endpoints.DeploymentRequest{}, err
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE deployment_requests SET status='approved',approved_at=now(),reviewed_by='operator_cli',updated_at=now() WHERE id=$1`, spec.RequestID); err != nil {
			return endpoints.DeploymentRequest{}, err
		}
	} else if spec.State == "ready" {
		if !deploymentID.Valid {
			var currentDeployment string
			if err := tx.QueryRowContext(ctx, `SELECT deployment_id FROM customer_endpoints WHERE id=$1`, endpointID).Scan(&currentDeployment); err != nil {
				return endpoints.DeploymentRequest{}, err
			}
			deploymentID = sql.NullString{String: currentDeployment, Valid: true}
		}
		if err := completeDedicatedDeployment(ctx, tx, orgID, projectID, environmentID, endpointID, deploymentID.String, modelID, profileID, capacityUnits, spec); err != nil {
			return endpoints.DeploymentRequest{}, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE deployment_requests SET status='ready',completed_at=now(),updated_at=now() WHERE id=$1`, spec.RequestID); err != nil {
			return endpoints.DeploymentRequest{}, err
		}
	} else if spec.State == "failed" {
		if _, err := tx.ExecContext(ctx, `UPDATE deployment_requests SET status='failed',safe_failure_class='operator_failed',completed_at=now(),updated_at=now() WHERE id=$1`, spec.RequestID); err != nil {
			return endpoints.DeploymentRequest{}, err
		}
		if deploymentID.Valid {
			_, _ = tx.ExecContext(ctx, `UPDATE model_deployments SET state='failed',safe_error_class='operator_failed',updated_at=now() WHERE id=$1`, deploymentID.String)
		}
		_, _ = tx.ExecContext(ctx, `UPDATE customer_endpoints SET runtime_state='failed',updated_at=now() WHERE id=$1`, endpointID)
	} else {
		if !deploymentID.Valid {
			if err := tx.QueryRowContext(ctx, `SELECT deployment_id FROM customer_endpoints WHERE id=$1`, endpointID).Scan(&deploymentID); err != nil {
				return endpoints.DeploymentRequest{}, err
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE deployment_requests SET status=$2,updated_at=now() WHERE id=$1`, spec.RequestID, spec.State); err != nil {
			return endpoints.DeploymentRequest{}, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE model_deployments SET state=$2,updated_at=now() WHERE id=$1`, deploymentID.String, spec.State); err != nil {
			return endpoints.DeploymentRequest{}, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE customer_endpoints SET runtime_state=$2,updated_at=now() WHERE id=$1`, endpointID, spec.State); err != nil {
			return endpoints.DeploymentRequest{}, err
		}
	}
	if err := insertActorAudit(ctx, tx, "operator", "cli", orgID, projectID, "deployment_request.transitioned", "succeeded", map[string]string{"deployment_request_id": spec.RequestID, "state": spec.State, "evidence_provided": fmt.Sprint(spec.EvidenceRef != "")}); err != nil {
		return endpoints.DeploymentRequest{}, err
	}
	result, err := scanDeploymentRequest(tx.QueryRowContext(ctx, deploymentRequestSelect+` WHERE dr.id=$1`, spec.RequestID))
	if err != nil {
		return endpoints.DeploymentRequest{}, err
	}
	if err := tx.Commit(); err != nil {
		return endpoints.DeploymentRequest{}, err
	}
	return result, nil
}

func completeDedicatedDeployment(ctx context.Context, tx *sql.Tx, orgID, projectID, environmentID, endpointID, deploymentID, modelID, profileID string, units int, spec endpoints.TransitionSpec) error {
	var targetID, executionClass, resourceClass string
	var acceleratorsPerUnit int
	if err := tx.QueryRowContext(ctx, `SELECT t.id,t.execution_class,p.accelerator_class,p.accelerators_per_unit
		FROM inference_targets t JOIN deployment_profiles p ON p.id=$2
		WHERE t.name=$1 AND t.capacity_mode='dedicated' AND t.owner_organisation_id=$3 AND t.enabled
		 AND t.execution_class=p.execution_class FOR UPDATE OF t,p`, spec.TargetName, profileID, orgID).Scan(&targetID, &executionClass, &resourceClass, &acceleratorsPerUnit); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return platform.ErrNotFound
		}
		return err
	}
	routeID, err := lockExistingRoute(ctx, tx, orgID, projectID, environmentID, modelID)
	if err != nil {
		return err
	}
	planSpec := platform.ProvisionSpec{CapacityMode: "dedicated", ServicePlanCode: endpointPlanCode(endpointID), ServicePlanName: "Dedicated endpoint allocation", DedicatedResourceClass: resourceClass, ServicePlanSource: "operator_deployment", ServicePlanFinality: "declared"}
	count := int64(acceleratorsPerUnit * units)
	planSpec.DedicatedAcceleratorCount = &count
	if routeID != "" {
		if _, err := prepareServicePlanTransition(ctx, tx, orgID, projectID, environmentID, routeID, planSpec); err != nil {
			return err
		}
	}
	routeID, err = upsertRoute(ctx, tx, orgID, projectID, environmentID, modelID, targetID)
	if err != nil {
		return mapWriteError("bind dedicated route", err)
	}
	if err := upsertServicePlan(ctx, tx, orgID, projectID, environmentID, routeID, planSpec); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE model_deployments SET target_id=$2,route_id=$3,state='ready',validation_evidence_ref=$4,last_verified_at=now(),ready_at=COALESCE(ready_at,now()),safe_error_class=NULL,updated_at=now() WHERE id=$1`, deploymentID, targetID, routeID, spec.EvidenceRef); err != nil {
		return mapWriteError("complete model deployment", err)
	}
	revisionID, err := ids.New("dcr")
	if err != nil {
		return err
	}
	var quoteID string
	if err := tx.QueryRowContext(ctx, `SELECT quote_id FROM model_deployments WHERE id=$1`, deploymentID).Scan(&quoteID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE deployment_capacity_revisions SET state='superseded',ended_at=now() WHERE deployment_id=$1 AND state='active'`, deploymentID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO deployment_capacity_revisions(id,organisation_id,project_id,environment_id,deployment_id,quote_id,capacity_units,state,resource_evidence_ref,effective_at) VALUES($1,$2,$3,$4,$5,$6,$7,'active',$8,now())`, revisionID, orgID, projectID, environmentID, deploymentID, quoteID, units, spec.EvidenceRef); err != nil {
		return mapWriteError("record deployment capacity", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE customer_endpoints SET route_id=$2,deployment_id=$3,runtime_state='ready',updated_at=now() WHERE id=$1`, endpointID, routeID, deploymentID); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE endpoint_configurations SET status='activated',updated_at=now() WHERE id=(SELECT configuration_id FROM customer_endpoints WHERE id=$1)`, endpointID)
	return err
}

var _ endpoints.Operator = (*Store)(nil)
