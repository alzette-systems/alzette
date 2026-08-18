package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"alzette/internal/endpoints"
	"alzette/internal/ids"
	"alzette/internal/platform"
)

type resolvedOffer struct {
	id, code, kind, profileID, modelID, modelAlias, modelSlug, modelName, releaseVersion string
	profileCode, profileServiceMode, executionClass, sourceLabel, evidenceRef            string
	minUnits, maxUnits                                                                   int
	targetID                                                                             sql.NullString
	requestAllowance, tokenAllowance                                                     sql.NullInt64
	allowancePeriod                                                                      sql.NullString
	priceID, currency, billingPeriod, priceFinality                                      sql.NullString
	recurringAmount, setupAmount                                                         sql.NullInt64
}

func (s *Store) GetEndpointConfiguration(ctx context.Context, session platform.PortalSession, id string) (endpoints.Configuration, error) {
	return getConfiguration(ctx, s.db, session, id)
}

func (s *Store) CreateEndpointConfiguration(ctx context.Context, session platform.PortalSession, input endpoints.CreateInput, digest [32]byte) (endpoints.Configuration, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return endpoints.Configuration{}, err
	}
	defer tx.Rollback()
	if existing, err := getConfigurationByIdempotency(ctx, tx, session, digest); err == nil {
		matchesSelection := existing.OfferCode == input.OfferCode && existing.ProfileCode == input.ProfileCode && existing.EndpointAlias == input.EndpointAlias && existing.CapacityUnits == input.CapacityUnits
		if input.ServiceMode != "" {
			matchesSelection = offerKindMatchesServiceMode(existing.OfferKind, input.ServiceMode)
		}
		if existing.ModelSlug != input.ModelSlug || !matchesSelection || !workloadEqual(existing.Workload, input.Workload) {
			return endpoints.Configuration{}, platform.ErrConflict
		}
		return existing, nil
	} else if !errors.Is(err, platform.ErrNotFound) {
		return endpoints.Configuration{}, err
	}
	var offer resolvedOffer
	if input.ServiceMode != "" {
		offer, err = resolveManagedEndpointOffer(ctx, tx, session.Current.OrganisationID, input.ModelSlug, input.ServiceMode)
		input.OfferCode = offer.code
		input.ProfileCode = offer.profileCode
		input.EndpointAlias = offer.modelAlias
		input.CapacityUnits = offer.minUnits
	} else {
		offer, err = resolveEndpointOffer(ctx, tx, session.Current.OrganisationID, input.ModelSlug, input.OfferCode, input.ProfileCode)
	}
	if err != nil {
		return endpoints.Configuration{}, err
	}
	if input.CapacityUnits < offer.minUnits || input.CapacityUnits > offer.maxUnits || input.EndpointAlias != offer.modelAlias {
		return endpoints.Configuration{}, platform.ErrInvalid
	}
	id, err := ids.New("cfg")
	if err != nil {
		return endpoints.Configuration{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO endpoint_configurations(
		id,organisation_id,project_id,environment_id,offer_id,deployment_profile_id,routable_model_id,
		endpoint_alias,capacity_units,workload_use_case,expected_context_tokens,expected_concurrency,
		expected_requests_per_minute,latency_priority,expected_monthly_requests,expected_user_count,
		requested_by_user_id,idempotency_key_hash)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`,
		id, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID,
		offer.id, offer.profileID, offer.modelID, input.EndpointAlias, input.CapacityUnits,
		strings.TrimSpace(input.Workload.UseCase), input.Workload.ExpectedContextTokens, input.Workload.ExpectedConcurrency,
		input.Workload.ExpectedRequestsPerMinute, input.Workload.LatencyPriority, input.Workload.ExpectedMonthlyRequests,
		input.Workload.ExpectedUserCount, session.User.ID, digest[:])
	if err != nil {
		return endpoints.Configuration{}, mapWriteError("create endpoint configuration", err)
	}
	if err := insertActorAudit(ctx, tx, "human_user", session.User.ID, session.Current.OrganisationID, session.Current.ProjectID, "endpoint_configuration.created", "succeeded", map[string]string{"configuration_id": id, "offer_code": offer.kind, "profile_code": offer.profileCode}); err != nil {
		return endpoints.Configuration{}, err
	}
	result, err := getConfiguration(ctx, tx, session, id)
	if err != nil {
		return endpoints.Configuration{}, err
	}
	if err := tx.Commit(); err != nil {
		return endpoints.Configuration{}, err
	}
	return result, nil
}

func (s *Store) UpdateEndpointConfiguration(ctx context.Context, session platform.PortalSession, id string, input endpoints.PatchInput) (endpoints.Configuration, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return endpoints.Configuration{}, err
	}
	defer tx.Rollback()
	current, err := getConfiguration(ctx, tx, session, id)
	if err != nil {
		return endpoints.Configuration{}, err
	}
	if current.Status != "draft" {
		return endpoints.Configuration{}, platform.ErrConflict
	}
	units := current.CapacityUnits
	workload := current.Workload
	if input.CapacityUnits != nil {
		units = *input.CapacityUnits
	}
	if input.Workload != nil {
		patch := *input.Workload
		if patch.ExpectedUserCount != nil {
			workload = mergeRevisedWorkloadPatch(workload, patch)
		} else {
			// Legacy clients replace the legacy workload snapshot. They cannot
			// see the new field, so an omitted team size must not erase it.
			expectedUserCount := workload.ExpectedUserCount
			workload = patch
			workload.ExpectedUserCount = expectedUserCount
		}
		workload.UseCase = strings.TrimSpace(workload.UseCase)
	}
	var minUnits, maxUnits int
	if err := tx.QueryRowContext(ctx, `SELECT p.min_capacity_units,p.max_capacity_units FROM endpoint_configurations c JOIN deployment_profiles p ON p.id=c.deployment_profile_id WHERE c.organisation_id=$1 AND c.project_id=$2 AND c.environment_id=$3 AND c.id=$4 FOR UPDATE OF c`, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID, id).Scan(&minUnits, &maxUnits); err != nil {
		return endpoints.Configuration{}, err
	}
	if units < minUnits || units > maxUnits {
		return endpoints.Configuration{}, platform.ErrInvalid
	}
	if _, err := tx.ExecContext(ctx, `UPDATE endpoint_configurations SET capacity_units=$5,workload_use_case=$6,expected_context_tokens=$7,expected_concurrency=$8,expected_requests_per_minute=$9,latency_priority=$10,expected_monthly_requests=$11,expected_user_count=$12,updated_at=now() WHERE organisation_id=$1 AND project_id=$2 AND environment_id=$3 AND id=$4 AND status='draft'`, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID, id, units, workload.UseCase, workload.ExpectedContextTokens, workload.ExpectedConcurrency, workload.ExpectedRequestsPerMinute, workload.LatencyPriority, workload.ExpectedMonthlyRequests, workload.ExpectedUserCount); err != nil {
		return endpoints.Configuration{}, mapWriteError("update endpoint configuration", err)
	}
	if err := insertActorAudit(ctx, tx, "human_user", session.User.ID, session.Current.OrganisationID, session.Current.ProjectID, "endpoint_configuration.updated", "succeeded", map[string]string{"configuration_id": id}); err != nil {
		return endpoints.Configuration{}, err
	}
	result, err := getConfiguration(ctx, tx, session, id)
	if err != nil {
		return endpoints.Configuration{}, err
	}
	if err := tx.Commit(); err != nil {
		return endpoints.Configuration{}, err
	}
	return result, nil
}

func (s *Store) SubmitEndpointConfiguration(ctx context.Context, session platform.PortalSession, id string, digest [32]byte, now time.Time) (endpoints.Endpoint, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return endpoints.Endpoint{}, err
	}
	defer tx.Rollback()
	var lockedID string
	err = tx.QueryRowContext(ctx, `SELECT id FROM endpoint_configurations WHERE organisation_id=$1 AND project_id=$2 AND environment_id=$3 AND id=$4 FOR UPDATE`, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID, id).Scan(&lockedID)
	if errors.Is(err, sql.ErrNoRows) {
		return endpoints.Endpoint{}, platform.ErrNotFound
	}
	if err != nil {
		return endpoints.Endpoint{}, err
	}
	configuration, err := getConfiguration(ctx, tx, session, id)
	if err != nil {
		return endpoints.Endpoint{}, err
	}
	if configuration.Status != "draft" {
		result, err := getEndpointByConfiguration(ctx, tx, session, id)
		if err == nil {
			return result, nil
		}
		return endpoints.Endpoint{}, platform.ErrConflict
	}
	offer, err := resolveEndpointOffer(ctx, tx, session.Current.OrganisationID, configuration.ModelSlug, configuration.OfferCode, configuration.ProfileCode)
	if err != nil {
		return endpoints.Endpoint{}, err
	}
	requestDigest := deploymentRequestDigest("new_endpoint", id, configuration.CapacityUnits, configuration.Workload)
	requestID, err := ids.New("drq")
	if err != nil {
		return endpoints.Endpoint{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO deployment_requests(
		id,organisation_id,project_id,environment_id,request_kind,deployment_profile_id,
		requested_capacity_units,status,requested_by_user_id,submitted_at,workload_use_case,
		expected_context_tokens,expected_concurrency,expected_requests_per_minute,latency_priority,
		expected_monthly_requests,expected_user_count,idempotency_key_hash,idempotency_request_hash)
		VALUES($1,$2,$3,$4,'new_endpoint',$5,$6,'submitted',$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
		requestID, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID,
		offer.profileID, configuration.CapacityUnits, session.User.ID, now, configuration.Workload.UseCase,
		configuration.Workload.ExpectedContextTokens, configuration.Workload.ExpectedConcurrency,
		configuration.Workload.ExpectedRequestsPerMinute, configuration.Workload.LatencyPriority,
		configuration.Workload.ExpectedMonthlyRequests, configuration.Workload.ExpectedUserCount,
		digest[:], requestDigest[:]); err != nil {
		return endpoints.Endpoint{}, mapWriteError("submit deployment request", err)
	}
	endpointID, err := ids.New("end")
	if err != nil {
		return endpoints.Endpoint{}, err
	}
	commercialState, runtimeState := "quote_pending", "awaiting_allocation"
	if offer.kind == "shared_evaluation" {
		commercialState, runtimeState = "not_required", "awaiting_submission"
	} else if offer.kind == "shared_subscription" {
		commercialState, runtimeState = "payment_action_required", "awaiting_payment"
	}
	serviceMode := offer.kind
	if offer.kind == "dedicated_quote" {
		serviceMode = offer.profileServiceMode
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO customer_endpoints(id,organisation_id,project_id,environment_id,configuration_id,offer_id,deployment_profile_id,routable_model_id,endpoint_alias,service_mode,commercial_state,runtime_state,capacity_units,request_allowance,token_allowance,allowance_period) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`, endpointID, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID, id, offer.id, offer.profileID, offer.modelID, offer.modelAlias, serviceMode, commercialState, runtimeState, configuration.CapacityUnits, nullInt64Value(offer.requestAllowance), nullInt64Value(offer.tokenAllowance), nullStringValue(offer.allowancePeriod)); err != nil {
		return endpoints.Endpoint{}, mapWriteError("create customer endpoint", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE endpoint_configurations SET status='submitted',deployment_request_id=$2,submitted_at=$3,updated_at=now() WHERE id=$1`, id, requestID, now); err != nil {
		return endpoints.Endpoint{}, err
	}
	if offer.kind == "shared_evaluation" {
		if err := activateSharedEndpoint(ctx, tx, session.Current, endpointID, offer, now); err != nil {
			return endpoints.Endpoint{}, err
		}
	} else if offer.kind == "shared_subscription" {
		if err := createSharedPaymentRequirement(ctx, tx, session.Current, endpointID, requestID, offer); err != nil {
			return endpoints.Endpoint{}, err
		}
	}
	if err := insertActorAudit(ctx, tx, "human_user", session.User.ID, session.Current.OrganisationID, session.Current.ProjectID, "endpoint_configuration.submitted", "succeeded", map[string]string{"configuration_id": id, "deployment_request_id": requestID, "endpoint_id": endpointID, "offer_kind": offer.kind}); err != nil {
		return endpoints.Endpoint{}, err
	}
	result, err := getCustomerEndpoint(ctx, tx, session, endpointID)
	if err != nil {
		return endpoints.Endpoint{}, err
	}
	if err := tx.Commit(); err != nil {
		return endpoints.Endpoint{}, err
	}
	return result, nil
}

func activateSharedEndpoint(ctx context.Context, tx *sql.Tx, scope platform.PortalMembership, endpointID string, offer resolvedOffer, now time.Time) error {
	if !offer.targetID.Valid {
		return platform.ErrUnavailable
	}
	var targetEnabled bool
	if err := tx.QueryRowContext(ctx, `SELECT enabled FROM inference_targets WHERE id=$1 AND capacity_mode='shared' AND owner_organisation_id IS NULL FOR UPDATE`, offer.targetID.String).Scan(&targetEnabled); errors.Is(err, sql.ErrNoRows) || !targetEnabled {
		return platform.ErrUnavailable
	} else if err != nil {
		return err
	}
	var routeID, existingTarget string
	err := tx.QueryRowContext(ctx, `SELECT id,target_id FROM tenant_routes WHERE organisation_id=$1 AND project_id=$2 AND environment_id=$3 AND model_id=$4 FOR UPDATE`, scope.OrganisationID, scope.ProjectID, scope.EnvironmentID, offer.modelID).Scan(&routeID, &existingTarget)
	if errors.Is(err, sql.ErrNoRows) {
		routeID, err = ids.New("rte")
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO tenant_routes(id,organisation_id,project_id,environment_id,model_id,target_id) VALUES($1,$2,$3,$4,$5,$6)`, routeID, scope.OrganisationID, scope.ProjectID, scope.EnvironmentID, offer.modelID, offer.targetID.String); err != nil {
			return mapWriteError("activate shared endpoint route", err)
		}
	} else if err != nil {
		return err
	} else if existingTarget != offer.targetID.String {
		return platform.ErrConflict
	}
	var existingPlanID string
	err = tx.QueryRowContext(ctx, `SELECT sp.id FROM tenant_service_plans tsp JOIN service_plans sp ON sp.organisation_id=tsp.organisation_id AND sp.id=tsp.service_plan_id WHERE tsp.organisation_id=$1 AND tsp.project_id=$2 AND tsp.environment_id=$3 AND tsp.route_id=$4 AND tsp.status='active' FOR UPDATE OF tsp`, scope.OrganisationID, scope.ProjectID, scope.EnvironmentID, routeID).Scan(&existingPlanID)
	if errors.Is(err, sql.ErrNoRows) {
		planID, err := ids.New("pln")
		if err != nil {
			return err
		}
		planCode := endpointPlanCode(endpointID)
		period := "contract_term"
		if offer.allowancePeriod.String == "month" {
			period = "month"
		}
		var requestUnit, requestPeriod, tokenUnit, tokenPeriod interface{}
		if offer.requestAllowance.Valid {
			requestUnit, requestPeriod = "logical_requests", period
		}
		if offer.tokenAllowance.Valid {
			tokenUnit, tokenPeriod = "provider_reported_tokens", period
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO service_plans(id,organisation_id,code,name,capacity_mode,shared_request_allowance,shared_request_allowance_unit,shared_request_allowance_period,shared_token_allowance,shared_token_allowance_unit,shared_token_allowance_period,source_label,finality) VALUES($1,$2,$3,$4,'shared',$5,$6,$7,$8,$9,$10,$11,'declared')`, planID, scope.OrganisationID, planCode, offer.modelAlias+" endpoint offer", nullInt64Value(offer.requestAllowance), requestUnit, requestPeriod, nullInt64Value(offer.tokenAllowance), tokenUnit, tokenPeriod, offer.sourceLabel); err != nil {
			return mapWriteError("create shared endpoint plan", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO tenant_service_plans(organisation_id,project_id,environment_id,route_id,service_plan_id,status,source_label,finality,effective_at) VALUES($1,$2,$3,$4,$5,'active',$6,'declared',$7)`, scope.OrganisationID, scope.ProjectID, scope.EnvironmentID, routeID, planID, offer.sourceLabel, now); err != nil {
			return mapWriteError("bind shared endpoint plan", err)
		}
	} else if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE customer_endpoints SET route_id=$2,runtime_state='route_bound',updated_at=now() WHERE id=$1`, endpointID, routeID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE endpoint_configurations SET status='activated',updated_at=now() WHERE id=(SELECT configuration_id FROM customer_endpoints WHERE id=$1)`, endpointID); err != nil {
		return err
	}
	return nil
}

func createSharedPaymentRequirement(ctx context.Context, tx *sql.Tx, scope platform.PortalMembership, endpointID, requestID string, offer resolvedOffer) error {
	if !offer.priceID.Valid || !offer.currency.Valid || !offer.recurringAmount.Valid || !offer.billingPeriod.Valid {
		return platform.ErrUnavailable
	}
	paymentID, err := ids.New("pay")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO payment_requirements(id,organisation_id,project_id,environment_id,endpoint_id,deployment_request_id,purpose,amount_minor,currency,billing_period,tax_treatment,collection_mode,state,price_finality,source_label,evidence_ref) VALUES($1,$2,$3,$4,$5,$6,'shared_activation',$7,$8,'month','not_determined','checkout_subscription','action_required',$9,$10,$11)`, paymentID, scope.OrganisationID, scope.ProjectID, scope.EnvironmentID, endpointID, requestID, offer.recurringAmount.Int64, offer.currency.String, offer.priceFinality.String, offer.sourceLabel, offer.evidenceRef)
	return err
}

func (s *Store) ListCustomerEndpoints(ctx context.Context, session platform.PortalSession) ([]endpoints.Endpoint, error) {
	rows, err := s.db.QueryContext(ctx, endpointSelect+` WHERE ce.organisation_id=$1 AND ce.project_id=$2 AND ce.environment_id=$3 ORDER BY ce.created_at DESC`, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]endpoints.Endpoint, 0)
	for rows.Next() {
		item, err := scanEndpoint(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) GetCustomerEndpoint(ctx context.Context, session platform.PortalSession, id string) (endpoints.Endpoint, error) {
	return getCustomerEndpoint(ctx, s.db, session, id)
}

type endpointQueryer interface {
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

const endpointSelect = `SELECT ce.id,ce.configuration_id,c.deployment_request_id,ce.endpoint_alias,cm.slug,cm.name,v.version,p.code,ce.service_mode,p.execution_class,CASE WHEN ce.service_mode='dedicated_private' THEN COALESCE(md.validation_evidence_ref IS NOT NULL AND md.state IN ('ready','degraded'),false) ELSE o.evidence_ref IS NOT NULL END,ce.capacity_units,ce.request_allowance,ce.allowance_requests_used,ce.token_allowance,ce.allowance_period,ce.allowance_period_start,ce.allowance_period_end,c.status,ce.commercial_state,ce.runtime_state,ce.route_id IS NOT NULL,COALESCE(r.enabled AND t.enabled,false),ce.created_at,ce.updated_at,pr.id,pr.purpose,pr.state,pr.amount_minor,pr.currency,pr.billing_period,pr.tax_treatment,pr.collection_mode,pr.price_finality,pr.paid_at FROM customer_endpoints ce JOIN endpoint_configurations c ON c.id=ce.configuration_id JOIN endpoint_offers o ON o.id=ce.offer_id JOIN deployment_profiles p ON p.id=ce.deployment_profile_id JOIN catalogue_model_versions v ON v.id=p.catalogue_model_version_id JOIN catalogue_models cm ON cm.id=v.catalogue_model_id LEFT JOIN model_deployments md ON md.id=ce.deployment_id LEFT JOIN tenant_routes r ON r.id=ce.route_id LEFT JOIN inference_targets t ON t.id=r.target_id LEFT JOIN LATERAL (SELECT x.* FROM payment_requirements x WHERE x.endpoint_id=ce.id ORDER BY x.created_at DESC LIMIT 1) pr ON true`

func getCustomerEndpoint(ctx context.Context, query endpointQueryer, session platform.PortalSession, id string) (endpoints.Endpoint, error) {
	row := query.QueryRowContext(ctx, endpointSelect+` WHERE ce.organisation_id=$1 AND ce.project_id=$2 AND ce.environment_id=$3 AND ce.id=$4`, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID, id)
	result, err := scanEndpoint(row)
	if errors.Is(err, sql.ErrNoRows) {
		return endpoints.Endpoint{}, platform.ErrNotFound
	}
	return result, err
}

func getEndpointByConfiguration(ctx context.Context, query endpointQueryer, session platform.PortalSession, configurationID string) (endpoints.Endpoint, error) {
	row := query.QueryRowContext(ctx, endpointSelect+` WHERE ce.organisation_id=$1 AND ce.project_id=$2 AND ce.environment_id=$3 AND ce.configuration_id=$4`, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID, configurationID)
	result, err := scanEndpoint(row)
	if errors.Is(err, sql.ErrNoRows) {
		return endpoints.Endpoint{}, platform.ErrNotFound
	}
	return result, err
}

type rowScanner interface{ Scan(...interface{}) error }

func scanEndpoint(row rowScanner) (endpoints.Endpoint, error) {
	var result endpoints.Endpoint
	var requestID sql.NullString
	var requestLimit, tokenLimit sql.NullInt64
	var allowanceUsed int64
	var allowancePeriod sql.NullString
	var periodStart, periodEnd sql.NullTime
	var configurationState, commercialState, runtimeState string
	var routeBound, routeCallable bool
	var paymentID, paymentPurpose, paymentState, paymentCurrency, paymentPeriod, paymentTax, paymentMode, paymentFinality sql.NullString
	var paymentAmount sql.NullInt64
	var paymentPaidAt sql.NullTime
	err := row.Scan(&result.ID, &result.ConfigurationID, &requestID, &result.Alias, &result.ModelSlug, &result.ModelName, &result.ModelVersion, &result.ProfileCode, &result.ServiceMode, &result.ExecutionClass, &result.ExecutionEvidenced, &result.CapacityUnits, &requestLimit, &allowanceUsed, &tokenLimit, &allowancePeriod, &periodStart, &periodEnd, &configurationState, &commercialState, &runtimeState, &routeBound, &routeCallable, &result.CreatedAt, &result.UpdatedAt, &paymentID, &paymentPurpose, &paymentState, &paymentAmount, &paymentCurrency, &paymentPeriod, &paymentTax, &paymentMode, &paymentFinality, &paymentPaidAt)
	if err != nil {
		return endpoints.Endpoint{}, err
	}
	result.DeploymentRequestID = nullStringPointer(requestID)
	result.RouteBound = routeBound
	result.Callable = routeBound && routeCallable && runtimeState != "retired" && runtimeState != "failed"
	result.Configuration = endpointRail("configuration", configurationState)
	result.Commercial = endpointRail("commercial", commercialState)
	result.Payment = endpointRail("payment", paymentState.String)
	result.Runtime = endpointRail("runtime", runtimeState)
	if requestLimit.Valid || tokenLimit.Valid {
		result.Allowance = &endpoints.Allowance{LogicalRequestLimit: nullInt64Pointer(requestLimit), LogicalRequestsUsed: allowanceUsed, ReportedTokenLimit: nullInt64Pointer(tokenLimit), Period: allowancePeriod.String, PeriodStart: nullTimePointer(periodStart), PeriodEnd: nullTimePointer(periodEnd), HardLimit: true}
	} else {
		result.Allowance = nil
	}
	if paymentID.Valid {
		result.PaymentRequirement = &endpoints.PaymentRequirement{ID: paymentID.String, Purpose: paymentPurpose.String, State: paymentState.String, AmountMinor: paymentAmount.Int64, Currency: paymentCurrency.String, BillingPeriod: paymentPeriod.String, TaxTreatment: paymentTax.String, CollectionMode: paymentMode.String, PriceFinality: paymentFinality.String, PaidAt: nullTimePointer(paymentPaidAt)}
	}
	return result, nil
}

func endpointRail(kind, state string) endpoints.Rail {
	detail := state
	switch kind + ":" + state {
	case "configuration:draft":
		detail = "Configuration is a draft and has not been submitted."
	case "configuration:submitted":
		detail = "Configuration is submitted; commercial and runtime states are separate."
	case "configuration:activated":
		detail = "Configuration has produced a server-authorised endpoint record."
	case "commercial:not_required":
		detail = "No payment is required for this evaluation offer."
	case "commercial:quote_pending":
		detail = "An operator-reviewed versioned quote has not been offered."
	case "commercial:quote_offered":
		detail = "A versioned quote is available; it is not payment or runtime readiness."
	case "commercial:quote_accepted":
		detail = "Quote terms are accepted; payment, allocation, and readiness remain separate."
	case "commercial:payment_action_required":
		detail = "Hosted payment action is required; no runtime route has been activated."
	case "commercial:paid":
		detail = "Payment evidence is recorded; this does not imply runtime readiness."
	case "runtime:awaiting_payment":
		detail = "No route is callable while payment confirmation is pending."
	case "runtime:awaiting_allocation":
		detail = "Infrastructure allocation has not started."
	case "runtime:route_bound":
		detail = "An authorised route is bound; live readiness requires separately labelled observation evidence."
	case "runtime:ready":
		detail = "Operator validation and route binding are recorded."
	}
	if state == "" {
		state, detail = "not_required", "No separate payment requirement exists."
	}
	return endpoints.Rail{State: state, Detail: detail}
}

func resolveEndpointOffer(ctx context.Context, tx *sql.Tx, organisationID, modelSlug, offerCode, profileCode string) (resolvedOffer, error) {
	row := tx.QueryRowContext(ctx, resolvedOfferSelect+` WHERE cm.slug=$2 AND o.code=$3 AND p.code=$4 AND o.status='published' AND ((org.account_kind='evaluation' AND o.eligible_evaluation) OR (org.account_kind='customer' AND o.eligible_customer)) FOR SHARE OF o,p`, organisationID, modelSlug, offerCode, profileCode)
	result, err := scanResolvedOffer(row)
	if errors.Is(err, sql.ErrNoRows) {
		return resolvedOffer{}, platform.ErrNotFound
	}
	return result, err
}

func resolveManagedEndpointOffer(ctx context.Context, tx *sql.Tx, organisationID, modelSlug, serviceMode string) (resolvedOffer, error) {
	row := tx.QueryRowContext(ctx, resolvedOfferSelect+` LEFT JOIN inference_targets t ON t.id=o.target_id WHERE cm.slug=$2 AND o.status='published' AND p.status='quotable' AND ((org.account_kind='evaluation' AND o.eligible_evaluation) OR (org.account_kind='customer' AND o.eligible_customer)) AND (($3='shared' AND o.offer_kind IN ('shared_evaluation','shared_subscription') AND COALESCE(t.enabled,false)) OR ($3='dedicated' AND o.offer_kind='dedicated_quote')) ORDER BY CASE o.offer_kind WHEN 'shared_evaluation' THEN 0 WHEN 'shared_subscription' THEN 1 ELSE 2 END,o.code LIMIT 1 FOR SHARE OF o,p`, organisationID, modelSlug, serviceMode)
	result, err := scanResolvedOffer(row)
	if errors.Is(err, sql.ErrNoRows) {
		return resolvedOffer{}, platform.ErrNotFound
	}
	return result, err
}

const resolvedOfferSelect = `SELECT o.id,o.code,o.offer_kind,o.deployment_profile_id,o.routable_model_id,m.alias,cm.slug,cm.name,v.version,p.code,p.service_mode,p.execution_class,p.min_capacity_units,p.max_capacity_units,o.target_id,o.request_allowance,o.token_allowance,o.allowance_period,o.source_label,o.evidence_ref,o.profile_price_id,pr.currency,pr.billing_period,pr.recurring_unit_amount_minor,pr.setup_amount_minor,pr.finality FROM endpoint_offers o JOIN deployment_profiles p ON p.id=o.deployment_profile_id JOIN catalogue_model_versions v ON v.id=p.catalogue_model_version_id JOIN catalogue_models cm ON cm.id=v.catalogue_model_id JOIN models m ON m.id=o.routable_model_id JOIN organisations org ON org.id=$1 LEFT JOIN deployment_profile_prices pr ON pr.id=o.profile_price_id`

func scanResolvedOffer(row rowScanner) (resolvedOffer, error) {
	var result resolvedOffer
	err := row.Scan(&result.id, &result.code, &result.kind, &result.profileID, &result.modelID, &result.modelAlias, &result.modelSlug, &result.modelName, &result.releaseVersion, &result.profileCode, &result.profileServiceMode, &result.executionClass, &result.minUnits, &result.maxUnits, &result.targetID, &result.requestAllowance, &result.tokenAllowance, &result.allowancePeriod, &result.sourceLabel, &result.evidenceRef, &result.priceID, &result.currency, &result.billingPeriod, &result.recurringAmount, &result.setupAmount, &result.priceFinality)
	return result, err
}

func offerKindMatchesServiceMode(kind, mode string) bool {
	return (mode == "shared" && (kind == "shared_evaluation" || kind == "shared_subscription")) || (mode == "dedicated" && kind == "dedicated_quote")
}

func getConfiguration(ctx context.Context, query endpointQueryer, session platform.PortalSession, id string) (endpoints.Configuration, error) {
	row := query.QueryRowContext(ctx, configurationSelect+` WHERE c.organisation_id=$1 AND c.project_id=$2 AND c.environment_id=$3 AND c.id=$4`, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID, id)
	result, err := scanConfiguration(row)
	if errors.Is(err, sql.ErrNoRows) {
		return endpoints.Configuration{}, platform.ErrNotFound
	}
	return result, err
}

func getConfigurationByIdempotency(ctx context.Context, query endpointQueryer, session platform.PortalSession, digest [32]byte) (endpoints.Configuration, error) {
	row := query.QueryRowContext(ctx, configurationSelect+` WHERE c.organisation_id=$1 AND c.project_id=$2 AND c.environment_id=$3 AND c.idempotency_key_hash=$4`, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID, digest[:])
	result, err := scanConfiguration(row)
	if errors.Is(err, sql.ErrNoRows) {
		return endpoints.Configuration{}, platform.ErrNotFound
	}
	return result, err
}

const configurationSelect = `SELECT c.id,cm.slug,cm.name,v.version,o.code,o.offer_kind,p.code,c.endpoint_alias,c.capacity_units,c.workload_use_case,c.expected_context_tokens,c.expected_concurrency,c.expected_requests_per_minute,c.latency_priority,c.expected_monthly_requests,c.expected_user_count,c.status,c.deployment_request_id,c.created_at,c.submitted_at FROM endpoint_configurations c JOIN endpoint_offers o ON o.id=c.offer_id JOIN deployment_profiles p ON p.id=c.deployment_profile_id JOIN catalogue_model_versions v ON v.id=p.catalogue_model_version_id JOIN catalogue_models cm ON cm.id=v.catalogue_model_id`

func scanConfiguration(row rowScanner) (endpoints.Configuration, error) {
	var result endpoints.Configuration
	var contextTokens, monthly, userCount sql.NullInt64
	var concurrency, rpm sql.NullInt64
	var latency, requestID sql.NullString
	var submitted sql.NullTime
	err := row.Scan(&result.ID, &result.ModelSlug, &result.ModelName, &result.ReleaseVersion, &result.OfferCode, &result.OfferKind, &result.ProfileCode, &result.EndpointAlias, &result.CapacityUnits, &result.Workload.UseCase, &contextTokens, &concurrency, &rpm, &latency, &monthly, &userCount, &result.Status, &requestID, &result.CreatedAt, &submitted)
	if err != nil {
		return endpoints.Configuration{}, err
	}
	result.Workload.ExpectedContextTokens = nullInt64Pointer(contextTokens)
	result.Workload.ExpectedMonthlyRequests = nullInt64Pointer(monthly)
	if userCount.Valid {
		value := int(userCount.Int64)
		result.Workload.ExpectedUserCount = &value
	}
	if concurrency.Valid {
		value := int(concurrency.Int64)
		result.Workload.ExpectedConcurrency = &value
	}
	if rpm.Valid {
		value := int(rpm.Int64)
		result.Workload.ExpectedRequestsPerMinute = &value
	}
	result.Workload.LatencyPriority = nullStringPointer(latency)
	result.RequestID = nullStringPointer(requestID)
	result.SubmittedAt = nullTimePointer(submitted)
	return result, nil
}

func workloadEqual(a, b endpoints.Workload) bool {
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	return string(left) == string(right)
}

func mergeRevisedWorkloadPatch(current, patch endpoints.Workload) endpoints.Workload {
	if useCase := strings.TrimSpace(patch.UseCase); useCase != "" {
		current.UseCase = useCase
	}
	if patch.ExpectedContextTokens != nil {
		current.ExpectedContextTokens = patch.ExpectedContextTokens
	}
	if patch.ExpectedConcurrency != nil {
		current.ExpectedConcurrency = patch.ExpectedConcurrency
	}
	if patch.ExpectedRequestsPerMinute != nil {
		current.ExpectedRequestsPerMinute = patch.ExpectedRequestsPerMinute
	}
	if patch.LatencyPriority != nil {
		current.LatencyPriority = patch.LatencyPriority
	}
	if patch.ExpectedMonthlyRequests != nil {
		current.ExpectedMonthlyRequests = patch.ExpectedMonthlyRequests
	}
	current.ExpectedUserCount = patch.ExpectedUserCount
	return current
}

func deploymentRequestDigest(kind, targetID string, units int, workload endpoints.Workload) [32]byte {
	payload, _ := json.Marshal(struct {
		Kind     string             `json:"kind"`
		TargetID string             `json:"target_id"`
		Units    int                `json:"units"`
		Workload endpoints.Workload `json:"workload"`
	}{Kind: kind, TargetID: targetID, Units: units, Workload: workload})
	return sha256.Sum256(payload)
}

func nullInt64Value(value sql.NullInt64) interface{} {
	if !value.Valid {
		return nil
	}
	return value.Int64
}
func nullStringValue(value sql.NullString) interface{} {
	if !value.Valid {
		return nil
	}
	return value.String
}
func endpointPlanCode(endpointID string) string {
	value := strings.ReplaceAll(endpointID, "_", "-")
	if len(value) > 58 {
		value = value[:58]
	}
	return "ep-" + value
}

// Remaining request/quote/operator methods are in endpoints_workflow.go to
// keep the acquisition and fulfilment transactions reviewable.

var _ endpoints.Store = (*Store)(nil)
