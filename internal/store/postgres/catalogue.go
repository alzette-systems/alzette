package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"alzette/internal/catalogue"
	"alzette/internal/ids"
	"alzette/internal/platform"
)

func (s *Store) ListCatalogue(ctx context.Context, session platform.PortalSession) ([]catalogue.Model, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT cm.slug,m.alias,cm.name,cm.family,cm.description,cm.modalities,cm.capabilities,cm.lifecycle_status,
		       v.id,v.version,v.context_window_tokens,v.licence_name,v.licence_status,v.support_status,v.lifecycle_status,v.source_label,v.published_at
		  FROM catalogue_models cm
		  JOIN LATERAL (
		      SELECT x.* FROM catalogue_model_versions x
		       WHERE x.catalogue_model_id=cm.id AND x.lifecycle_status IN ('available','deprecated')
		       ORDER BY x.published_at DESC,x.created_at DESC LIMIT 1
		  ) v ON true
		  JOIN models m ON m.id=v.routable_model_id AND m.enabled
		 WHERE cm.lifecycle_status IN ('published','deprecated')
		 ORDER BY cm.name,cm.slug`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]catalogue.Model, 0)
	for rows.Next() {
		item, versionID, err := scanCatalogueModel(rows)
		if err != nil {
			return nil, err
		}
		item.Offers, err = s.listCatalogueOffers(ctx, session, versionID)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) GetCatalogueModel(ctx context.Context, session platform.PortalSession, slug string) (catalogue.Model, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT cm.slug,m.alias,cm.name,cm.family,cm.description,cm.modalities,cm.capabilities,cm.lifecycle_status,
		       v.id,v.version,v.context_window_tokens,v.licence_name,v.licence_status,v.support_status,v.lifecycle_status,v.source_label,v.published_at
		  FROM catalogue_models cm
		  JOIN LATERAL (
		      SELECT x.* FROM catalogue_model_versions x
		       WHERE x.catalogue_model_id=cm.id AND x.lifecycle_status IN ('available','deprecated')
		       ORDER BY x.published_at DESC,x.created_at DESC LIMIT 1
		  ) v ON true
		  JOIN models m ON m.id=v.routable_model_id AND m.enabled
		 WHERE cm.slug=$1 AND cm.lifecycle_status IN ('published','deprecated')`, slug)
	result, versionID, err := scanCatalogueModel(row)
	if errors.Is(err, sql.ErrNoRows) {
		return catalogue.Model{}, platform.ErrNotFound
	}
	if err != nil {
		return catalogue.Model{}, err
	}
	result.Offers, err = s.listCatalogueOffers(ctx, session, versionID)
	return result, err
}

type catalogueScanner interface{ Scan(...interface{}) error }

func scanCatalogueModel(row catalogueScanner) (catalogue.Model, string, error) {
	var result catalogue.Model
	var versionID string
	var modalities, capabilities []byte
	var contextTokens sql.NullInt64
	var published sql.NullTime
	err := row.Scan(&result.Slug, &result.EndpointAlias, &result.Name, &result.Family, &result.Description, &modalities, &capabilities, &result.Lifecycle,
		&versionID, &result.Release.Version, &contextTokens, &result.Release.LicenceName, &result.Release.LicenceStatus,
		&result.Release.SupportStatus, &result.Release.LifecycleStatus, &result.Release.Source, &published)
	if err != nil {
		return catalogue.Model{}, "", err
	}
	if err := json.Unmarshal(modalities, &result.Modalities); err != nil {
		return catalogue.Model{}, "", err
	}
	if err := json.Unmarshal(capabilities, &result.Capabilities); err != nil {
		return catalogue.Model{}, "", err
	}
	result.Release.ContextWindowTokens = nullInt64Pointer(contextTokens)
	result.Release.PublishedAt = nullTimePointer(published)
	return result, versionID, nil
}

func (s *Store) listCatalogueOffers(ctx context.Context, session platform.PortalSession, versionID string) ([]catalogue.Offer, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT o.id,o.code,o.name,o.offer_kind,o.source_label,o.published_at,
		       p.id,p.code,p.name,p.execution_class,p.runtime_class,p.accelerator_class,p.accelerators_per_unit,
		       p.accelerator_memory_gib,p.min_capacity_units,p.max_capacity_units,p.capacity_finality,p.source_label,p.evidence_ref,
		       pr.currency,pr.billing_period,pr.recurring_unit_amount_minor,pr.setup_amount_minor,pr.finality,pr.source_label,
		       o.request_allowance,o.token_allowance,o.allowance_period,
		       CASE WHEN o.offer_kind='dedicated_quote' THEN true ELSE COALESCE(t.enabled,false) END
		  FROM endpoint_offers o
		  JOIN deployment_profiles p ON p.id=o.deployment_profile_id AND p.catalogue_model_version_id=$2
		  LEFT JOIN deployment_profile_prices pr ON pr.id=o.profile_price_id
		  LEFT JOIN inference_targets t ON t.id=o.target_id
		  JOIN organisations org ON org.id=$1
		 WHERE o.status='published'
		   AND ((org.account_kind='evaluation' AND o.eligible_evaluation)
		        OR (org.account_kind='customer' AND o.eligible_customer))
		 ORDER BY CASE o.offer_kind WHEN 'shared_evaluation' THEN 0 WHEN 'shared_subscription' THEN 1 ELSE 2 END,o.code`, session.Current.OrganisationID, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]catalogue.Offer, 0)
	for rows.Next() {
		var item catalogue.Offer
		var offerID, profileID string
		var acceleratorClass, currency, billingPeriod, priceFinality, priceSource sql.NullString
		var accelerators sql.NullInt64
		var acceleratorMemory sql.NullFloat64
		var evidence sql.NullString
		var recurring, setup sql.NullInt64
		var requestAllowance, tokenAllowance sql.NullInt64
		var allowancePeriod sql.NullString
		var targetEnabled bool
		if err := rows.Scan(&offerID, &item.Code, &item.Name, &item.Kind, &item.Source, &item.PublishedAt,
			&profileID, &item.Profile.Code, &item.Profile.Name, &item.Profile.ExecutionClass, &item.Profile.RuntimeClass,
			&acceleratorClass, &accelerators, &acceleratorMemory, &item.Profile.MinimumCapacityUnits, &item.Profile.MaximumCapacityUnits,
			&item.Profile.CapacityFinality, &item.Profile.Source, &evidence,
			&currency, &billingPeriod, &recurring, &setup, &priceFinality, &priceSource,
			&requestAllowance, &tokenAllowance, &allowancePeriod, &targetEnabled); err != nil {
			return nil, err
		}
		item.Eligible = true
		item.Availability = "available_to_configure"
		if !targetEnabled {
			item.Availability = "runtime_unavailable"
		}
		item.Profile.AcceleratorClass = nullStringPointer(acceleratorClass)
		if accelerators.Valid {
			value := int(accelerators.Int64)
			item.Profile.AcceleratorsPerUnit = &value
		}
		item.Profile.AcceleratorMemoryGiB = nullFloat64Pointer(acceleratorMemory)
		item.Profile.EvidenceProvided = evidence.Valid
		item.Profile.Metrics, err = s.listProfileMetrics(ctx, profileID)
		if err != nil {
			return nil, err
		}
		if currency.Valid {
			item.Price = &catalogue.Price{Currency: currency.String, BillingPeriod: billingPeriod.String, RecurringAmountMinor: recurring.Int64, SetupAmountMinor: setup.Int64, Finality: priceFinality.String, Source: priceSource.String}
		}
		if requestAllowance.Valid || tokenAllowance.Valid {
			item.Allowance = &catalogue.Allowance{LogicalRequests: nullInt64Pointer(requestAllowance), ReportedTokens: nullInt64Pointer(tokenAllowance), Period: allowancePeriod.String, HardLimit: true}
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) listProfileMetrics(ctx context.Context, profileID string) ([]catalogue.Metric, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT metric_code,unit,minimum_value,target_value,maximum_value,per_capacity_unit,scales_with_units,finality,source_label,measured_at,evidence_ref IS NOT NULL FROM deployment_profile_metrics WHERE deployment_profile_id=$1 ORDER BY metric_code`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]catalogue.Metric, 0)
	for rows.Next() {
		var item catalogue.Metric
		var minimum, target, maximum sql.NullFloat64
		var measured sql.NullTime
		if err := rows.Scan(&item.Code, &item.Unit, &minimum, &target, &maximum, &item.PerCapacityUnit, &item.ScalesWithUnits, &item.Finality, &item.Source, &measured, &item.EvidenceProvided); err != nil {
			return nil, err
		}
		item.Minimum, item.Target, item.Maximum = nullFloat64Pointer(minimum), nullFloat64Pointer(target), nullFloat64Pointer(maximum)
		item.MeasuredAt = nullTimePointer(measured)
		result = append(result, item)
	}
	return result, rows.Err()
}

func nullFloat64Pointer(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	copy := value.Float64
	return &copy
}

func (s *Store) SeedCatalogue(ctx context.Context, input catalogue.SeedSpec) (catalogue.SeedResult, error) {
	spec, err := catalogue.ValidateSeed(input)
	if err != nil {
		return catalogue.SeedResult{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return catalogue.SeedResult{}, err
	}
	defer tx.Rollback()
	var modelID, modelVersion, targetID string
	if err := tx.QueryRowContext(ctx, `SELECT id,version FROM models WHERE alias=$1 AND enabled FOR UPDATE`, spec.ModelAlias).Scan(&modelID, &modelVersion); errors.Is(err, sql.ErrNoRows) {
		return catalogue.SeedResult{}, platform.ErrNotFound
	} else if err != nil {
		return catalogue.SeedResult{}, err
	}
	if spec.ReleaseVersion != modelVersion {
		return catalogue.SeedResult{}, platform.ErrConflict
	}
	if err := tx.QueryRowContext(ctx, `SELECT id FROM inference_targets WHERE name=$1 AND capacity_mode='shared' AND owner_organisation_id IS NULL FOR UPDATE`, spec.TargetName).Scan(&targetID); errors.Is(err, sql.ErrNoRows) {
		return catalogue.SeedResult{}, platform.ErrNotFound
	} else if err != nil {
		return catalogue.SeedResult{}, err
	}
	modelCatalogueID, err := upsertCatalogueModel(ctx, tx, spec)
	if err != nil {
		return catalogue.SeedResult{}, err
	}
	versionID, err := upsertCatalogueVersion(ctx, tx, modelCatalogueID, modelID, spec)
	if err != nil {
		return catalogue.SeedResult{}, err
	}
	profileID, err := upsertSharedProfile(ctx, tx, versionID, spec)
	if err != nil {
		return catalogue.SeedResult{}, err
	}
	if err := upsertEvaluationOffer(ctx, tx, profileID, modelID, targetID, spec); err != nil {
		return catalogue.SeedResult{}, err
	}
	result := catalogue.SeedResult{ModelSlug: spec.ModelSlug, ReleaseVersion: spec.ReleaseVersion, SharedProfileCode: spec.SharedProfileCode, EvaluationOfferCode: spec.EvaluationOfferCode, CreatedOrUpdated: true}
	if spec.PaidOfferCode != "" {
		if err := upsertPaidOffer(ctx, tx, profileID, modelID, targetID, spec); err != nil {
			return catalogue.SeedResult{}, err
		}
		result.PaidOfferCode = spec.PaidOfferCode
	}
	if spec.DedicatedProfileCode != "" {
		if err := upsertDedicatedProfile(ctx, tx, versionID, modelID, spec); err != nil {
			return catalogue.SeedResult{}, err
		}
		result.DedicatedProfileCode = spec.DedicatedProfileCode
	}
	if err := insertActorAudit(ctx, tx, "operator", "cli", "", "", "catalogue.seeded", "succeeded", map[string]string{"model_slug": spec.ModelSlug, "evaluation_offer_code": spec.EvaluationOfferCode, "target_name": spec.TargetName}); err != nil {
		return catalogue.SeedResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return catalogue.SeedResult{}, err
	}
	return result, nil
}

func upsertCatalogueModel(ctx context.Context, tx *sql.Tx, spec catalogue.SeedSpec) (string, error) {
	id, err := ids.New("cat")
	if err != nil {
		return "", err
	}
	modalities, _ := json.Marshal([]string{"text"})
	capabilities, _ := json.Marshal([]string{"chat_completions", "streaming", "function_tools"})
	var result string
	err = tx.QueryRowContext(ctx, `INSERT INTO catalogue_models(id,slug,name,family,description,modalities,capabilities,lifecycle_status,published_at) VALUES($1,$2,$3,$4,$5,$6,$7,'published',now()) ON CONFLICT(slug) DO UPDATE SET name=EXCLUDED.name,family=EXCLUDED.family,description=EXCLUDED.description,modalities=EXCLUDED.modalities,capabilities=EXCLUDED.capabilities,updated_at=now() RETURNING id`, id, spec.ModelSlug, spec.ModelName, spec.ModelFamily, spec.Description, modalities, capabilities).Scan(&result)
	return result, err
}

func upsertCatalogueVersion(ctx context.Context, tx *sql.Tx, catalogueID, modelID string, spec catalogue.SeedSpec) (string, error) {
	id, err := ids.New("cmv")
	if err != nil {
		return "", err
	}
	var result string
	err = tx.QueryRowContext(ctx, `INSERT INTO catalogue_model_versions(id,catalogue_model_id,version,routable_model_id,context_window_tokens,licence_name,licence_status,support_status,lifecycle_status,source_label,evidence_ref,published_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'available',$9,$10,now()) ON CONFLICT(catalogue_model_id,version) DO UPDATE SET updated_at=now() RETURNING id`, id, catalogueID, spec.ReleaseVersion, modelID, spec.ContextWindowTokens, spec.LicenceName, spec.LicenceStatus, spec.SupportStatus, spec.SourceLabel, spec.EvidenceRef).Scan(&result)
	if err != nil {
		return "", err
	}
	var mapped string
	if err := tx.QueryRowContext(ctx, `SELECT routable_model_id FROM catalogue_model_versions WHERE id=$1`, result).Scan(&mapped); err != nil || mapped != modelID {
		return "", platform.ErrConflict
	}
	return result, nil
}

func upsertSharedProfile(ctx context.Context, tx *sql.Tx, versionID string, spec catalogue.SeedSpec) (string, error) {
	id, err := ids.New("dpf")
	if err != nil {
		return "", err
	}
	var result string
	err = tx.QueryRowContext(ctx, `INSERT INTO deployment_profiles(id,catalogue_model_version_id,code,name,service_mode,execution_class,runtime_class,min_capacity_units,max_capacity_units,capacity_finality,status,source_label,evidence_ref) VALUES($1,$2,$3,$4,'shared_evaluation','external_pilot',$5,1,1,'estimated','quotable',$6,$7) ON CONFLICT(catalogue_model_version_id,code) DO UPDATE SET name=EXCLUDED.name,status='quotable',source_label=EXCLUDED.source_label,evidence_ref=EXCLUDED.evidence_ref,updated_at=now() RETURNING id`, id, versionID, spec.SharedProfileCode, spec.SharedProfileName, spec.RuntimeClass, spec.SourceLabel, spec.EvidenceRef).Scan(&result)
	return result, err
}

func upsertEvaluationOffer(ctx context.Context, tx *sql.Tx, profileID, modelID, targetID string, spec catalogue.SeedSpec) error {
	templateID, err := ids.New("eot")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO evaluation_offer_templates(id,code,name,deployment_profile_id,routable_model_id,target_id,status,is_default,request_allowance,token_allowance,rate_limit_requests_per_minute,concurrency_limit,expires_after_days,privacy_notice_version,acceptable_use_version,source_label,evidence_ref) VALUES($1,$2,$3,$4,$5,$6,'enabled',false,$7,$8,60,4,30,'operator-poc-v1','operator-poc-v1',$9,$10) ON CONFLICT(code) DO UPDATE SET name=EXCLUDED.name,status='enabled',request_allowance=EXCLUDED.request_allowance,token_allowance=EXCLUDED.token_allowance,source_label=EXCLUDED.source_label,evidence_ref=EXCLUDED.evidence_ref,updated_at=now()`, templateID, spec.EvaluationOfferCode, spec.EvaluationOfferName, profileID, modelID, targetID, spec.EvaluationRequestLimit, spec.EvaluationTokenLimit, spec.SourceLabel, spec.EvidenceRef)
	if err != nil {
		return err
	}
	offerID, err := ids.New("off")
	if err != nil {
		return err
	}
	var actualID, actualProfile, actualModel, actualTarget, actualKind string
	err = tx.QueryRowContext(ctx, `INSERT INTO endpoint_offers(id,code,name,deployment_profile_id,routable_model_id,target_id,offer_kind,status,eligible_evaluation,eligible_customer,request_allowance,token_allowance,allowance_period,source_label,evidence_ref,published_at) VALUES($1,$2,$3,$4,$5,$6,'shared_evaluation','published',$7,$8,$9,$10,'lifetime',$11,$12,now()) ON CONFLICT(code) DO UPDATE SET name=EXCLUDED.name,status='published',eligible_evaluation=EXCLUDED.eligible_evaluation,eligible_customer=EXCLUDED.eligible_customer,source_label=EXCLUDED.source_label,evidence_ref=EXCLUDED.evidence_ref,published_at=COALESCE(endpoint_offers.published_at,now()),updated_at=now() RETURNING id,deployment_profile_id,routable_model_id,target_id,offer_kind`, offerID, spec.EvaluationOfferCode, spec.EvaluationOfferName, profileID, modelID, targetID, spec.EligibleEvaluation, spec.EligibleCustomer, spec.EvaluationRequestLimit, spec.EvaluationTokenLimit, spec.SourceLabel, spec.EvidenceRef).Scan(&actualID, &actualProfile, &actualModel, &actualTarget, &actualKind)
	if err != nil {
		return err
	}
	if actualProfile != profileID || actualModel != modelID || actualTarget != targetID || actualKind != "shared_evaluation" {
		return platform.ErrConflict
	}
	return nil
}

func upsertPaidOffer(ctx context.Context, tx *sql.Tx, profileID, modelID, targetID string, spec catalogue.SeedSpec) error {
	currency := strings.ToUpper(spec.PaidCurrency)
	var priceID string
	var existingAmount int64
	err := tx.QueryRowContext(ctx, `SELECT id,recurring_unit_amount_minor FROM deployment_profile_prices WHERE deployment_profile_id=$1 AND currency=$2 AND billing_period='month' AND effective_to IS NULL ORDER BY effective_from DESC LIMIT 1 FOR UPDATE`, profileID, currency).Scan(&priceID, &existingAmount)
	if err == nil && existingAmount != spec.PaidAmountMinor {
		return platform.ErrConflict
	}
	if errors.Is(err, sql.ErrNoRows) {
		priceID, err = ids.New("prc")
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO deployment_profile_prices(id,deployment_profile_id,currency,billing_period,recurring_unit_amount_minor,setup_amount_minor,visibility,finality,source_label,evidence_ref,effective_from) VALUES($1,$2,$3,'month',$4,0,'authenticated','contractual',$5,$6,date_trunc('second',now()))`, priceID, profileID, currency, spec.PaidAmountMinor, spec.SourceLabel, spec.EvidenceRef); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	offerID, err := ids.New("off")
	if err != nil {
		return err
	}
	var actualProfile, actualModel, actualTarget, actualPrice, actualKind string
	if err := tx.QueryRowContext(ctx, `INSERT INTO endpoint_offers(id,code,name,deployment_profile_id,routable_model_id,target_id,profile_price_id,offer_kind,status,eligible_evaluation,eligible_customer,request_allowance,allowance_period,source_label,evidence_ref,published_at) VALUES($1,$2,$3,$4,$5,$6,$7,'shared_subscription','published',false,true,$8,'month',$9,$10,now()) ON CONFLICT(code) DO UPDATE SET name=EXCLUDED.name,status='published',source_label=EXCLUDED.source_label,evidence_ref=EXCLUDED.evidence_ref,published_at=COALESCE(endpoint_offers.published_at,now()),updated_at=now() RETURNING id,deployment_profile_id,routable_model_id,target_id,profile_price_id,offer_kind`, offerID, spec.PaidOfferCode, spec.PaidOfferName, profileID, modelID, targetID, priceID, spec.PaidRequestLimit, spec.SourceLabel, spec.EvidenceRef).Scan(&offerID, &actualProfile, &actualModel, &actualTarget, &actualPrice, &actualKind); err != nil {
		return err
	}
	if actualProfile != profileID || actualModel != modelID || actualTarget != targetID || actualPrice != priceID || actualKind != "shared_subscription" {
		return platform.ErrConflict
	}
	var mappedRef string
	err = tx.QueryRowContext(ctx, `INSERT INTO billing_price_mappings(offer_id,provider,provider_price_ref) VALUES($1,'stripe',$2) ON CONFLICT(offer_id,provider) DO UPDATE SET active=true WHERE billing_price_mappings.provider_price_ref=EXCLUDED.provider_price_ref RETURNING provider_price_ref`, offerID, spec.StripePriceRef).Scan(&mappedRef)
	if errors.Is(err, sql.ErrNoRows) {
		return platform.ErrConflict
	}
	return err
}

func upsertDedicatedProfile(ctx context.Context, tx *sql.Tx, versionID, modelID string, spec catalogue.SeedSpec) error {
	profileID, err := ids.New("dpf")
	if err != nil {
		return err
	}
	if err := tx.QueryRowContext(ctx, `INSERT INTO deployment_profiles(id,catalogue_model_version_id,code,name,service_mode,execution_class,runtime_class,accelerator_class,accelerators_per_unit,accelerator_memory_gib,min_capacity_units,max_capacity_units,capacity_finality,status,source_label,evidence_ref) VALUES($1,$2,$3,$4,'dedicated_private',$5,$6,$7,$8,$9,$10,$11,$12,'quotable',$13,$14) ON CONFLICT(catalogue_model_version_id,code) DO UPDATE SET name=EXCLUDED.name,status='quotable',source_label=EXCLUDED.source_label,evidence_ref=EXCLUDED.evidence_ref,updated_at=now() RETURNING id`, profileID, versionID, spec.DedicatedProfileCode, spec.DedicatedProfileName, spec.DedicatedExecutionClass, spec.DedicatedRuntimeClass, spec.AcceleratorClass, spec.AcceleratorsPerUnit, spec.AcceleratorMemoryGiB, spec.MinimumCapacityUnits, spec.MaximumCapacityUnits, spec.DedicatedCapacityFinality, spec.SourceLabel, spec.DedicatedEvidenceRef).Scan(&profileID); err != nil {
		return err
	}
	metricID, err := ids.New("met")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO deployment_profile_metrics(id,deployment_profile_id,metric_code,unit,minimum_value,target_value,maximum_value,per_capacity_unit,scales_with_units,finality,source_label,evidence_ref) VALUES($1,$2,'accelerator_count','accelerators',$3,$3,$3,true,true,'contractual',$4,$5) ON CONFLICT(deployment_profile_id,metric_code) DO UPDATE SET source_label=EXCLUDED.source_label,evidence_ref=EXCLUDED.evidence_ref`, metricID, profileID, spec.AcceleratorsPerUnit, spec.SourceLabel, spec.DedicatedEvidenceRef)
	if err != nil {
		return err
	}
	offerID, err := ids.New("off")
	if err != nil {
		return err
	}
	var actualProfile, actualModel, actualKind string
	err = tx.QueryRowContext(ctx, `INSERT INTO endpoint_offers(id,code,name,deployment_profile_id,routable_model_id,offer_kind,status,eligible_evaluation,eligible_customer,source_label,evidence_ref,published_at) VALUES($1,$2,$3,$4,$5,'dedicated_quote','published',false,true,$6,$7,now()) ON CONFLICT(code) DO UPDATE SET name=EXCLUDED.name,status='published',source_label=EXCLUDED.source_label,evidence_ref=EXCLUDED.evidence_ref,published_at=COALESCE(endpoint_offers.published_at,now()),updated_at=now() RETURNING deployment_profile_id,routable_model_id,offer_kind`, offerID, spec.DedicatedProfileCode, spec.DedicatedProfileName, profileID, modelID, spec.SourceLabel, spec.DedicatedEvidenceRef).Scan(&actualProfile, &actualModel, &actualKind)
	if err != nil {
		return err
	}
	if actualProfile != profileID || actualModel != modelID || actualKind != "dedicated_quote" {
		return platform.ErrConflict
	}
	return nil
}

var _ catalogue.Store = (*Store)(nil)
var _ catalogue.Provisioner = (*Store)(nil)
