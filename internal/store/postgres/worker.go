package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"alzette/internal/platform"
)

type rollupKey struct {
	organisationID, projectID, environmentID, routeID, serviceAccountID, modelAlias string
	bucketStart                                                                     time.Time
}

type rollupAccumulator struct {
	logical, succeeded, failed, blocked, cancelled, inProgress int64
	providerAttempts, retried                                  int64
	input, output, cached, reasoning                           int64
	inputKnown, outputKnown, cachedKnown, reasoningKnown       int64
	durations                                                  []int64
	intervals                                                  []rollupInterval
	partial                                                    bool
}

type rollupInterval struct{ start, end time.Time }

func (s *Store) RefreshUsageRollups(ctx context.Context, from, to, asOf time.Time) (resultCount int64, resultErr error) {
	from = from.UTC().Truncate(time.Hour)
	to = to.UTC()
	asOf = asOf.UTC()
	if !to.After(from) || asOf.Before(from) {
		return 0, platform.ErrInvalid
	}
	connection, err := s.db.Conn(ctx)
	if err != nil {
		return 0, err
	}
	defer connection.Close()
	var lockAcquired bool
	if err := connection.QueryRowContext(ctx, `SELECT pg_try_advisory_lock(719922430168292743)`).Scan(&lockAcquired); err != nil {
		return 0, err
	}
	if !lockAcquired {
		return 0, platform.ErrConflict
	}
	defer func() {
		unlockContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = connection.ExecContext(unlockContext, `SELECT pg_advisory_unlock(719922430168292743)`)
	}()
	if err := markRollupStarted(ctx, connection, from, to, asOf); err != nil {
		return 0, err
	}
	defer func() {
		if resultErr != nil {
			failureContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			markRollupFailed(failureContext, connection, asOf, "refresh_failed")
		}
	}()
	tx, err := connection.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `
		SELECT organisation_id,project_id,environment_id,COALESCE(route_id,''),service_account_id,model_alias,
		       started_at,completed_at,status,duration_ms,input_tokens,output_tokens,cached_tokens,reasoning_tokens,
		       usage_finality,attempt_count
		  FROM inference_requests
		 WHERE started_at >= $1 AND started_at < $2
		 ORDER BY started_at`, from, to)
	if err != nil {
		return 0, err
	}
	groups := make(map[rollupKey]*rollupAccumulator)
	var total int64
	for rows.Next() {
		var key rollupKey
		var started time.Time
		var completed sql.NullTime
		var status, finality string
		var duration, input, output, cached, reasoning sql.NullInt64
		var attempts int64
		if err := rows.Scan(&key.organisationID, &key.projectID, &key.environmentID, &key.routeID, &key.serviceAccountID, &key.modelAlias, &started, &completed, &status, &duration, &input, &output, &cached, &reasoning, &finality, &attempts); err != nil {
			rows.Close()
			return 0, err
		}
		key.bucketStart = started.UTC().Truncate(time.Hour)
		group := groups[key]
		if group == nil {
			group = &rollupAccumulator{}
			groups[key] = group
		}
		group.logical++
		total++
		switch status {
		case "succeeded":
			group.succeeded++
		case "failed":
			group.failed++
		case "blocked":
			group.blocked++
		case "cancelled":
			group.cancelled++
		default:
			group.inProgress++
			group.partial = true
		}
		group.providerAttempts += attempts
		if attempts > 1 {
			group.retried++
		}
		if duration.Valid {
			group.durations = append(group.durations, duration.Int64)
		}
		if status == "succeeded" {
			accumulateKnown(&group.input, &group.inputKnown, input)
			accumulateKnown(&group.output, &group.outputKnown, output)
			accumulateKnown(&group.cached, &group.cachedKnown, cached)
			accumulateKnown(&group.reasoning, &group.reasoningKnown, reasoning)
			if finality != "final" {
				group.partial = true
			}
		}
		end := asOf
		if completed.Valid {
			end = completed.Time.UTC()
		}
		if end.After(asOf) {
			end = asOf
		}
		bucketEnd := key.bucketStart.Add(time.Hour)
		if end.After(bucketEnd) {
			end = bucketEnd
		}
		if end.After(started) {
			group.intervals = append(group.intervals, rollupInterval{start: started.UTC(), end: end})
		}
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	deleteTo := to.Truncate(time.Hour)
	if !deleteTo.Equal(to) {
		deleteTo = deleteTo.Add(time.Hour)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM usage_rollups_hourly_v2 WHERE bucket_start >= $1 AND bucket_start < $2`, from, deleteTo); err != nil {
		return 0, err
	}
	for key, group := range groups {
		finality := "final"
		if group.partial || key.bucketStart.Add(time.Hour).After(asOf) {
			finality = "partial"
		}
		var routeID interface{}
		if key.routeID != "" {
			routeID = key.routeID
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO usage_rollups_hourly_v2(
				organisation_id,project_id,environment_id,route_id,service_account_id,model_alias,bucket_start,
				logical_requests,successful_requests,failed_requests,blocked_requests,cancelled_requests,in_progress_requests,
				provider_attempts,retried_requests,
				input_tokens,input_known_requests,output_tokens,output_known_requests,cached_tokens,cached_known_requests,
				reasoning_tokens,reasoning_known_requests,p50_latency_ms,p95_latency_ms,peak_concurrency,
				source_row_count,source,finality,refreshed_at
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$8,'inference_requests',$27,$28)`,
			key.organisationID, key.projectID, key.environmentID, routeID, key.serviceAccountID, key.modelAlias, key.bucketStart,
			group.logical, group.succeeded, group.failed, group.blocked, group.cancelled, group.inProgress,
			group.providerAttempts, group.retried,
			nullableRollupTotal(group.input, group.inputKnown), group.inputKnown,
			nullableRollupTotal(group.output, group.outputKnown), group.outputKnown,
			nullableRollupTotal(group.cached, group.cachedKnown), group.cachedKnown,
			nullableRollupTotal(group.reasoning, group.reasoningKnown), group.reasoningKnown,
			percentile(group.durations, 0.50), percentile(group.durations, 0.95), peakConcurrency(group.intervals),
			finality, asOf)
		if err != nil {
			return 0, fmt.Errorf("insert usage rollup: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO worker_checkpoints(organisation_id,project_id,environment_id,worker_name,last_started_at,last_completed_at,status,range_from,range_to,source_rows,safe_error_class)
		SELECT e.organisation_id,e.project_id,e.id,'usage_rollup',$3,$3,'succeeded',$1,$2,
		       (SELECT count(*) FROM inference_requests ir WHERE ir.organisation_id=e.organisation_id AND ir.project_id=e.project_id AND ir.environment_id=e.id AND ir.started_at >= $1 AND ir.started_at < $2),NULL
		  FROM environments e
		ON CONFLICT(organisation_id,project_id,environment_id,worker_name)
		DO UPDATE SET last_started_at=EXCLUDED.last_started_at,last_completed_at=EXCLUDED.last_completed_at,
		              status='succeeded',range_from=EXCLUDED.range_from,range_to=EXCLUDED.range_to,
		              source_rows=EXCLUDED.source_rows,safe_error_class=NULL`, from, to, asOf); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return total, nil
}

type databaseExecer interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
}

func markRollupStarted(ctx context.Context, db databaseExecer, from, to, now time.Time) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO worker_checkpoints(organisation_id,project_id,environment_id,worker_name,last_started_at,status,range_from,range_to,source_rows)
		SELECT organisation_id,project_id,id,'usage_rollup',$3,'running',$1,$2,0 FROM environments
		ON CONFLICT(organisation_id,project_id,environment_id,worker_name)
		DO UPDATE SET last_started_at=EXCLUDED.last_started_at,status='running',range_from=EXCLUDED.range_from,range_to=EXCLUDED.range_to,source_rows=0,safe_error_class=NULL`, from, to, now)
	return err
}

func markRollupFailed(ctx context.Context, db databaseExecer, runStarted time.Time, class string) {
	_, _ = db.ExecContext(ctx, `UPDATE worker_checkpoints SET status='failed',safe_error_class=$2 WHERE worker_name='usage_rollup' AND status='running' AND last_started_at=$1`, runStarted, class)
}

func (s *Store) ListProbeTargets(ctx context.Context, now time.Time) ([]platform.ProbeTarget, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id,t.base_url,t.secret_ref,t.provider_model,t.timeout_ms,t.probe_interval_seconds
		  FROM inference_targets t
		 WHERE t.probe_enabled AND t.enabled
		   AND NOT EXISTS (
		       SELECT 1 FROM target_health_observations o
		        WHERE o.target_id=t.id AND o.fresh_until>$1
		   )
		 ORDER BY t.id`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []platform.ProbeTarget
	for rows.Next() {
		var item platform.ProbeTarget
		var timeoutMS, intervalSeconds int64
		if err := rows.Scan(&item.ID, &item.BaseURL, &item.SecretRef, &item.ProviderModel, &timeoutMS, &intervalSeconds); err != nil {
			return nil, err
		}
		item.Timeout = time.Duration(timeoutMS) * time.Millisecond
		item.ProbeInterval = time.Duration(intervalSeconds) * time.Second
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) RecordProbeObservation(ctx context.Context, observation platform.ProbeObservation) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var httpStatus, latency interface{}
	if observation.HTTPStatus != 0 {
		httpStatus = observation.HTTPStatus
	}
	if observation.Latency > 0 {
		latency = observation.Latency.Milliseconds()
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO target_health_observations(id,target_id,observed_at,status,source,credential_available,http_status,error_class,latency_ms,fresh_until) VALUES($1,$2,$3,$4,'opt_in_compatible_probe',$5,$6,NULLIF($7,''),$8,$9)`, observation.ID, observation.TargetID, observation.ObservedAt, observation.Status, observation.CredentialAvailable, httpStatus, observation.ErrorClass, latency, observation.FreshUntil); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE inference_targets SET health_status=$2,last_health_check_at=$3,last_success_at=CASE WHEN $2='operational' THEN $3 ELSE last_success_at END,updated_at=now() WHERE id=$1`, observation.TargetID, observation.Status, observation.ObservedAt); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) WorkerHealthy(ctx context.Context, now time.Time, maximumAge time.Duration) error {
	var environments, healthy int64
	err := s.db.QueryRowContext(ctx, `
		SELECT (SELECT count(*) FROM environments),
		       (SELECT count(*) FROM worker_checkpoints WHERE worker_name='usage_rollup' AND status='succeeded' AND last_completed_at>$1)`, now.Add(-maximumAge)).Scan(&environments, &healthy)
	if err != nil {
		return err
	}
	if environments != healthy {
		return platform.ErrUnavailable
	}
	return nil
}

func accumulateKnown(total, known *int64, value sql.NullInt64) {
	if value.Valid {
		*total += value.Int64
		*known++
	}
}

func nullableRollupTotal(total, known int64) interface{} {
	if known == 0 {
		return nil
	}
	return total
}

func percentile(values []int64, fraction float64) interface{} {
	if len(values) == 0 {
		return nil
	}
	copy := append([]int64(nil), values...)
	sort.Slice(copy, func(i, j int) bool { return copy[i] < copy[j] })
	index := int(float64(len(copy)-1) * fraction)
	if fraction >= .95 && index < len(copy)-1 {
		index++
	}
	return copy[index]
}

func peakConcurrency(intervals []rollupInterval) interface{} {
	if len(intervals) == 0 {
		return nil
	}
	type event struct {
		at    time.Time
		delta int
	}
	events := make([]event, 0, len(intervals)*2)
	for _, interval := range intervals {
		events = append(events, event{at: interval.start, delta: 1}, event{at: interval.end, delta: -1})
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].at.Equal(events[j].at) {
			return events[i].delta < events[j].delta
		}
		return events[i].at.Before(events[j].at)
	})
	current, peak := 0, 0
	for _, item := range events {
		current += item.delta
		if current > peak {
			peak = current
		}
	}
	return int64(peak)
}

var _ platform.RollupStore = (*Store)(nil)

var _ = errors.Is
