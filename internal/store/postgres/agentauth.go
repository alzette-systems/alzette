package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"alzette/internal/agentauth"
	"alzette/internal/federation"
	"alzette/internal/platform"
)

func (s *Store) ListAgentContexts(ctx context.Context, identity federation.Identity) ([]agentauth.Context, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT hm.id, o.id, o.name, p.id, p.name, e.id, e.name, op.id,
		       EXISTS (SELECT 1 FROM organisation_ownerships own WHERE own.organisation_id=o.id AND own.person_id=op.id AND own.ended_at IS NULL)
		  FROM human_federated_identities fi
		  JOIN human_users u ON u.id=fi.user_id AND u.enabled
		  JOIN organisation_people op ON op.user_id=u.id AND op.enabled
		  JOIN human_memberships hm ON hm.user_id=u.id AND hm.organisation_id=op.organisation_id AND hm.enabled
		  JOIN organisations o ON o.id=hm.organisation_id
		  JOIN projects p ON p.id=hm.project_id AND p.organisation_id=hm.organisation_id
		  JOIN environments e ON e.id=hm.environment_id AND e.project_id=hm.project_id AND e.organisation_id=hm.organisation_id
		 WHERE fi.issuer=$1 AND fi.subject=$2 AND fi.enabled
		 ORDER BY o.name,p.name,e.name,hm.id`, identity.Issuer, identity.Subject)
	if err != nil {
		return nil, fmt.Errorf("list agent contexts: %w", err)
	}
	defer rows.Close()
	var contexts []agentauth.Context
	for rows.Next() {
		var value agentauth.Context
		var organisationID, projectID, environmentID, personID string
		var owner bool
		if err := rows.Scan(&value.MembershipID, &organisationID, &value.Organisation, &projectID, &value.Project, &environmentID, &value.Environment, &personID, &owner); err != nil {
			return nil, err
		}
		if owner {
			value.Relationship = "owner"
		} else {
			value.Relationship = "employee"
		}
		value.Models, err = loadEffectiveModels(ctx, s.db, organisationID, projectID, environmentID, personID, owner)
		if err != nil {
			return nil, err
		}
		value.ModelAliases = aliasesForAgentModels(value.Models)
		if len(value.ModelAliases) != 0 {
			contexts = append(contexts, value)
		}
	}
	return contexts, rows.Err()
}

type queryer interface {
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
}

func loadEffectiveModels(ctx context.Context, q queryer, organisationID, projectID, environmentID, personID string, owner bool) ([]agentauth.Model, error) {
	query := `SELECT DISTINCT ON (m.alias) m.alias,COALESCE(cat.name,m.alias),COALESCE(cat.capabilities,'[]'::jsonb),cat.context_window_tokens
		FROM tenant_routes r
		JOIN models m ON m.id=r.model_id AND m.enabled
		JOIN inference_targets t ON t.id=r.target_id AND t.enabled
		LEFT JOIN LATERAL (
			SELECT cm.name,cm.capabilities,v.context_window_tokens
			FROM catalogue_model_versions v
			JOIN catalogue_models cm ON cm.id=v.catalogue_model_id AND cm.lifecycle_status IN ('published','deprecated')
			WHERE v.routable_model_id=m.id AND v.lifecycle_status IN ('available','deprecated')
			ORDER BY v.published_at DESC NULLS LAST,v.created_at DESC
			LIMIT 1
		) cat ON true
		WHERE r.organisation_id=$1 AND r.project_id=$2 AND r.environment_id=$3 AND r.enabled`
	args := []interface{}{organisationID, projectID, environmentID}
	if !owner {
		query += ` AND EXISTS (
			SELECT 1 FROM access_group_people gp
			JOIN access_groups g ON g.id=gp.group_id AND g.organisation_id=gp.organisation_id AND g.enabled
			JOIN access_group_models gm ON gm.group_id=g.id AND gm.organisation_id=g.organisation_id AND gm.route_id=r.id
			WHERE gp.organisation_id=$1 AND gp.project_id=$2 AND gp.environment_id=$3 AND gp.person_id=$4)`
		args = append(args, personID)
	}
	query += ` ORDER BY m.alias`
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("resolve effective models: %w", err)
	}
	defer rows.Close()
	var models []agentauth.Model
	for rows.Next() {
		var model agentauth.Model
		var capabilities []byte
		var contextWindow sql.NullInt64
		if err := rows.Scan(&model.Alias, &model.DisplayName, &capabilities, &contextWindow); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(capabilities, &model.Capabilities); err != nil {
			return nil, fmt.Errorf("decode effective model capabilities: %w", err)
		}
		if contextWindow.Valid {
			value := contextWindow.Int64
			model.ContextWindowTokens = &value
		}
		models = append(models, model)
	}
	return models, rows.Err()
}

func aliasesForAgentModels(models []agentauth.Model) []string {
	aliases := make([]string, 0, len(models))
	for _, model := range models {
		aliases = append(aliases, model.Alias)
	}
	return aliases
}

func loadEffectiveAliases(ctx context.Context, q queryer, organisationID, projectID, environmentID, personID string, owner bool) ([]string, error) {
	models, err := loadEffectiveModels(ctx, q, organisationID, projectID, environmentID, personID, owner)
	if err != nil {
		return nil, err
	}
	return aliasesForAgentModels(models), nil
}

type agentContextRecord struct {
	Context                                                                agentauth.Context
	IdentityID, UserID, PersonID, OrganisationID, ProjectID, EnvironmentID string
	Owner                                                                  bool
}

func loadAgentContextTx(ctx context.Context, tx *sql.Tx, identity federation.Identity, membershipID string, lock bool) (agentContextRecord, error) {
	lockSQL := ""
	if lock {
		lockSQL = " FOR UPDATE OF fi,op,hm"
	}
	var record agentContextRecord
	err := tx.QueryRowContext(ctx, `
		SELECT fi.id,u.id,op.id,hm.id,o.id,o.name,p.id,p.name,e.id,e.name,
		       EXISTS (SELECT 1 FROM organisation_ownerships own WHERE own.organisation_id=o.id AND own.person_id=op.id AND own.ended_at IS NULL)
		  FROM human_federated_identities fi
		  JOIN human_users u ON u.id=fi.user_id AND u.enabled
		  JOIN organisation_people op ON op.user_id=u.id AND op.enabled
		  JOIN human_memberships hm ON hm.user_id=u.id AND hm.organisation_id=op.organisation_id AND hm.enabled
		  JOIN organisations o ON o.id=hm.organisation_id
		  JOIN projects p ON p.id=hm.project_id AND p.organisation_id=hm.organisation_id
		  JOIN environments e ON e.id=hm.environment_id AND e.project_id=hm.project_id AND e.organisation_id=hm.organisation_id
		 WHERE fi.issuer=$1 AND fi.subject=$2 AND fi.enabled AND hm.id=$3`+lockSQL,
		identity.Issuer, identity.Subject, membershipID).Scan(
		&record.IdentityID, &record.UserID, &record.PersonID, &record.Context.MembershipID,
		&record.OrganisationID, &record.Context.Organisation, &record.ProjectID, &record.Context.Project,
		&record.EnvironmentID, &record.Context.Environment, &record.Owner)
	if errors.Is(err, sql.ErrNoRows) {
		return record, platform.ErrForbidden
	}
	if err != nil {
		return record, fmt.Errorf("resolve agent context: %w", err)
	}
	if record.Owner {
		record.Context.Relationship = "owner"
	} else {
		record.Context.Relationship = "employee"
	}
	record.Context.Models, err = loadEffectiveModels(ctx, tx, record.OrganisationID, record.ProjectID, record.EnvironmentID, record.PersonID, record.Owner)
	record.Context.ModelAliases = aliasesForAgentModels(record.Context.Models)
	return record, err
}

func (s *Store) MintAgentCredential(ctx context.Context, input agentauth.StoreMintInput) (agentauth.MintResult, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return agentauth.MintResult{}, err
	}
	defer tx.Rollback()
	record, err := loadAgentContextTx(ctx, tx, input.Identity, input.MembershipID, true)
	if err != nil {
		return agentauth.MintResult{}, err
	}
	var previousHash []byte
	var previousToken string
	err = tx.QueryRowContext(ctx, `SELECT canonical_request_hash,token_id FROM human_agent_credential_mints
		WHERE federated_identity_id=$1 AND oauth_client_id=$2 AND idempotency_key_digest=$3 FOR UPDATE`,
		record.IdentityID, input.Identity.OAuthClientID, input.IdempotencyKeyDigest[:]).Scan(&previousHash, &previousToken)
	if err == nil {
		if string(previousHash) != string(input.CanonicalRequestHash[:]) {
			return agentauth.MintResult{}, agentauth.ErrIdempotencyConflict
		}
		if _, err := tx.ExecContext(ctx, `UPDATE human_agent_access_tokens SET revoked_at=COALESCE(revoked_at,$2) WHERE id=$1`, previousToken, input.Now); err != nil {
			return agentauth.MintResult{}, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE human_agent_credential_mints SET state='response_unrecoverable',replayed_at=$2 WHERE token_id=$1`, previousToken, input.Now); err != nil {
			return agentauth.MintResult{}, err
		}
		if err := tx.Commit(); err != nil {
			return agentauth.MintResult{}, err
		}
		return agentauth.MintResult{}, agentauth.ErrResponseUnrecoverable
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return agentauth.MintResult{}, err
	}
	if !aliasSubset(input.ModelAliases, record.Context.ModelAliases) {
		return agentauth.MintResult{}, platform.ErrForbidden
	}
	aliasesJSON, _ := json.Marshal(input.ModelAliases)
	grantID := input.GrantID
	var existingGrant string
	var revokedAt sql.NullTime
	var revocationReason sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT id,revoked_at,revocation_reason FROM human_agent_grants WHERE federated_identity_id=$1 AND oauth_client_id=$2 AND client_instance_digest=$3 AND membership_id=$4 FOR UPDATE`,
		record.IdentityID, input.Identity.OAuthClientID, input.ClientInstanceDigest[:], input.MembershipID).Scan(&existingGrant, &revokedAt, &revocationReason)
	if err == nil {
		// A Connect launch is an explicit, freshly OAuth-authenticated act. It may
		// renew the same client binding after that client deliberately disconnected
		// or after its bounded grant expired. Policy/offboarding revocations remain
		// terminal: loadAgentContextTx and the reason check both fail closed.
		if revokedAt.Valid && (!revocationReason.Valid || revocationReason.String != "client_logout") {
			return agentauth.MintResult{}, platform.ErrForbidden
		}
		grantID = existingGrant
		result, err := tx.ExecContext(ctx, `UPDATE human_agent_grants SET permitted_model_aliases=$2,authenticated_at=$3,absolute_expires_at=$4,last_used_at=$3,revoked_at=NULL,revocation_reason=NULL
			WHERE id=$1 AND (revoked_at IS NULL OR revocation_reason='client_logout')`, grantID, aliasesJSON, input.Now, input.GrantExpiresAt)
		if err != nil {
			return agentauth.MintResult{}, err
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return agentauth.MintResult{}, platform.ErrForbidden
		}
	} else if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.ExecContext(ctx, `INSERT INTO human_agent_grants(id,user_id,federated_identity_id,person_id,membership_id,organisation_id,project_id,environment_id,oauth_client_id,client_instance_digest,permitted_model_aliases,created_at,authenticated_at,absolute_expires_at,last_used_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$12,$13,$12)`, grantID, record.UserID, record.IdentityID, record.PersonID, input.MembershipID, record.OrganisationID, record.ProjectID, record.EnvironmentID, input.Identity.OAuthClientID, input.ClientInstanceDigest[:], aliasesJSON, input.Now, input.GrantExpiresAt)
		if err != nil {
			return agentauth.MintResult{}, mapWriteError("create human agent grant", err)
		}
	} else {
		return agentauth.MintResult{}, err
	}
	var generation int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(max(generation),0)+1 FROM human_agent_access_tokens WHERE grant_id=$1`, grantID).Scan(&generation); err != nil {
		return agentauth.MintResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE human_agent_access_tokens SET revoked_at=COALESCE(revoked_at,$2) WHERE grant_id=$1 AND revoked_at IS NULL`, grantID, input.Now); err != nil {
		return agentauth.MintResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO human_agent_access_tokens(id,grant_id,token_prefix,token_hash,generation,issued_at,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, input.TokenID, grantID, input.Credential.Prefix, input.Credential.Digest[:], generation, input.Now, input.TokenExpiresAt); err != nil {
		return agentauth.MintResult{}, mapWriteError("create human agent token", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE human_agent_access_tokens SET replaced_by_id=$2 WHERE grant_id=$1 AND id<>$2 AND revoked_at=$3 AND replaced_by_id IS NULL`, grantID, input.TokenID, input.Now); err != nil {
		return agentauth.MintResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO human_agent_credential_mints(id,federated_identity_id,oauth_client_id,idempotency_key_digest,canonical_request_hash,grant_id,token_id,state,created_at,completed_at) VALUES($1,$2,$3,$4,$5,$6,$7,'completed',$8,$8)`, input.MintID, record.IdentityID, input.Identity.OAuthClientID, input.IdempotencyKeyDigest[:], input.CanonicalRequestHash[:], grantID, input.TokenID, input.Now); err != nil {
		return agentauth.MintResult{}, mapWriteError("record human agent mint", err)
	}
	if err := tx.Commit(); err != nil {
		return agentauth.MintResult{}, mapWriteError("commit human agent mint", err)
	}
	return agentauth.MintResult{Context: record.Context, GrantID: grantID, AccessToken: input.Credential.Token, ExpiresAt: input.TokenExpiresAt, ModelAliases: append([]string(nil), input.ModelAliases...)}, nil
}

func aliasSubset(wanted, available []string) bool {
	set := make(map[string]struct{}, len(available))
	for _, value := range available {
		set[value] = struct{}{}
	}
	for _, value := range wanted {
		if _, ok := set[value]; !ok {
			return false
		}
	}
	return len(wanted) > 0
}

func (s *Store) RevokeAgentGrant(ctx context.Context, identity federation.Identity, membershipID string, clientDigest [32]byte, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var grantID string
	err = tx.QueryRowContext(ctx, `SELECT g.id FROM human_agent_grants g JOIN human_federated_identities fi ON fi.id=g.federated_identity_id
		WHERE fi.issuer=$1 AND fi.subject=$2 AND fi.enabled AND g.oauth_client_id=$3 AND g.membership_id=$4 AND g.client_instance_digest=$5 AND g.revoked_at IS NULL FOR UPDATE OF g`, identity.Issuer, identity.Subject, identity.OAuthClientID, membershipID, clientDigest[:]).Scan(&grantID)
	if errors.Is(err, sql.ErrNoRows) {
		return platform.ErrNotFound
	}
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE human_agent_grants SET revoked_at=$2,revocation_reason='client_logout' WHERE id=$1`, grantID, now); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE human_agent_access_tokens SET revoked_at=COALESCE(revoked_at,$2) WHERE grant_id=$1`, grantID, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) AuthenticateHuman(ctx context.Context, digest [32]byte) (platform.Principal, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return platform.Principal{}, err
	}
	defer tx.Rollback()
	var p platform.Principal
	var personID string
	var aliasesJSON []byte
	var owner bool
	err = tx.QueryRowContext(ctx, `SELECT o.id,o.name,o.slug,p.id,p.name,p.slug,e.id,e.name,e.slug,u.id,hm.id,g.id,t.id,t.token_prefix,g.permitted_model_aliases,op.id,
		EXISTS(SELECT 1 FROM organisation_ownerships own WHERE own.organisation_id=o.id AND own.person_id=op.id AND own.ended_at IS NULL)
		FROM human_agent_access_tokens t JOIN human_agent_grants g ON g.id=t.grant_id
		JOIN human_federated_identities fi ON fi.id=g.federated_identity_id AND fi.enabled
		JOIN human_users u ON u.id=g.user_id AND u.id=fi.user_id AND u.enabled
		JOIN organisation_people op ON op.id=g.person_id AND op.organisation_id=g.organisation_id AND op.user_id=u.id AND op.enabled
		JOIN human_memberships hm ON hm.id=g.membership_id AND hm.user_id=u.id AND hm.organisation_id=g.organisation_id AND hm.project_id=g.project_id AND hm.environment_id=g.environment_id AND hm.enabled
		JOIN organisations o ON o.id=g.organisation_id JOIN projects p ON p.id=g.project_id AND p.organisation_id=o.id
		JOIN environments e ON e.id=g.environment_id AND e.project_id=p.id AND e.organisation_id=o.id
		WHERE t.token_hash=$1 AND t.revoked_at IS NULL AND t.expires_at>now() AND g.revoked_at IS NULL AND g.absolute_expires_at>now()
		FOR UPDATE OF t,g`, digest[:]).Scan(&p.OrganisationID, &p.OrganisationName, &p.OrganisationSlug, &p.ProjectID, &p.ProjectName, &p.ProjectSlug, &p.EnvironmentID, &p.EnvironmentName, &p.EnvironmentSlug, &p.HumanUserID, &p.HumanMembershipID, &p.AgentGrantID, &p.AgentTokenID, &p.KeyPrefix, &aliasesJSON, &personID, &owner)
	if errors.Is(err, sql.ErrNoRows) {
		return platform.Principal{}, platform.ErrUnauthenticated
	}
	if err != nil {
		return platform.Principal{}, fmt.Errorf("authenticate human agent token: %w", err)
	}
	var granted []string
	if json.Unmarshal(aliasesJSON, &granted) != nil {
		return platform.Principal{}, errors.New("decode human token aliases")
	}
	current, err := loadEffectiveAliases(ctx, tx, p.OrganisationID, p.ProjectID, p.EnvironmentID, personID, owner)
	if err != nil {
		return platform.Principal{}, err
	}
	currentSet := map[string]struct{}{}
	for _, a := range current {
		currentSet[a] = struct{}{}
	}
	for _, a := range granted {
		if _, ok := currentSet[a]; ok {
			p.AllowedModelAliases = append(p.AllowedModelAliases, a)
		}
	}
	if len(p.AllowedModelAliases) == 0 {
		return platform.Principal{}, platform.ErrUnauthenticated
	}
	sort.Strings(p.AllowedModelAliases)
	p.CredentialKind = "human_agent_token"
	p.Scopes = []string{platform.ScopeInferenceWrite}
	if _, err = tx.ExecContext(ctx, `UPDATE human_agent_access_tokens SET last_used_at=now() WHERE id=$1`, p.AgentTokenID); err != nil {
		return platform.Principal{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE human_agent_grants SET last_used_at=now() WHERE id=$1`, p.AgentGrantID); err != nil {
		return platform.Principal{}, err
	}
	if err = tx.Commit(); err != nil {
		return platform.Principal{}, err
	}
	return p, nil
}

var _ agentauth.Store = (*Store)(nil)
