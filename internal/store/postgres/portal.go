package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"alzette/internal/credentials"
	"alzette/internal/humanauth"
	"alzette/internal/ids"
	"alzette/internal/platform"
	"alzette/internal/provisioning"
)

var (
	portalUsernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,62}$`)
	portalNamePattern     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 ._:-]{0,126}[A-Za-z0-9]$|^[A-Za-z0-9]$`)
	dummyPasswordHash     = func() string {
		hash, _ := humanauth.HashPassword("not-a-real-password-credential")
		return hash
	}()
)

func (s *Store) ProvisionHuman(ctx context.Context, spec platform.HumanUserSpec) (platform.HumanUserResult, error) {
	spec.Username = strings.TrimSpace(strings.ToLower(spec.Username))
	spec.DisplayName = strings.TrimSpace(spec.DisplayName)
	if !portalUsernamePattern.MatchString(spec.Username) || spec.DisplayName == "" || len(spec.DisplayName) > 255 {
		return platform.HumanUserResult{}, platform.ErrInvalid
	}
	switch spec.Role {
	case platform.PortalRoleOrgAdmin, platform.PortalRoleProjectAdmin, platform.PortalRoleDeveloper, platform.PortalRoleViewer:
	default:
		return platform.HumanUserResult{}, platform.ErrInvalid
	}
	if len(spec.PasswordHash) < 20 {
		return platform.HumanUserResult{}, platform.ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return platform.HumanUserResult{}, err
	}
	defer tx.Rollback()
	var orgID, projectID, environmentID string
	if err := tx.QueryRowContext(ctx, `
		SELECT o.id,p.id,e.id
		  FROM organisations o
		  JOIN projects p ON p.organisation_id=o.id
		  JOIN environments e ON e.organisation_id=o.id AND e.project_id=p.id
		 WHERE o.slug=$1 AND p.slug=$2 AND e.slug=$3`, spec.OrganisationSlug, spec.ProjectSlug, spec.EnvironmentSlug).Scan(&orgID, &projectID, &environmentID); errors.Is(err, sql.ErrNoRows) {
		return platform.HumanUserResult{}, platform.ErrNotFound
	} else if err != nil {
		return platform.HumanUserResult{}, err
	}
	result := platform.HumanUserResult{Username: spec.Username}
	var existingEnabled bool
	err = tx.QueryRowContext(ctx, `SELECT id,enabled FROM human_users WHERE username=$1 FOR UPDATE`, spec.Username).Scan(&result.UserID, &existingEnabled)
	if errors.Is(err, sql.ErrNoRows) {
		result.UserID, err = ids.New("usr")
		if err != nil {
			return platform.HumanUserResult{}, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO human_users(id,username,display_name,password_hash) VALUES($1,$2,$3,$4)`, result.UserID, spec.Username, spec.DisplayName, spec.PasswordHash); err != nil {
			return platform.HumanUserResult{}, mapWriteError("provision human user", err)
		}
		result.Created = true
	} else if err != nil {
		return platform.HumanUserResult{}, err
	} else if !existingEnabled {
		return platform.HumanUserResult{}, platform.ErrConflict
	}
	membershipID, err := ids.New("mem")
	if err != nil {
		return platform.HumanUserResult{}, err
	}
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO human_memberships(id,user_id,organisation_id,project_id,environment_id,role)
		VALUES($1,$2,$3,$4,$5,$6)
		ON CONFLICT(user_id,organisation_id,project_id,environment_id)
		DO UPDATE SET role=EXCLUDED.role,enabled=true,updated_at=now()
		RETURNING id`, membershipID, result.UserID, orgID, projectID, environmentID, spec.Role).Scan(&result.MembershipID); err != nil {
		return platform.HumanUserResult{}, mapWriteError("provision human membership", err)
	}
	personID, err := ids.New("per")
	if err != nil {
		return platform.HumanUserResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO organisation_people(id,organisation_id,user_id)
		VALUES($1,$2,$3)
		ON CONFLICT(organisation_id,user_id)
		DO UPDATE SET enabled=true,updated_at=now()`, personID, orgID, result.UserID); err != nil {
		return platform.HumanUserResult{}, mapWriteError("provision company person", err)
	}
	if err := insertActorAudit(ctx, tx, "operator", "cli", orgID, projectID, "human_user.provisioned", "succeeded", map[string]string{"user_id": result.UserID, "membership_id": result.MembershipID, "role": spec.Role}); err != nil {
		return platform.HumanUserResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return platform.HumanUserResult{}, err
	}
	return result, nil
}

func (s *Store) RotateHumanPassword(ctx context.Context, username, passwordHash string) error {
	username = strings.TrimSpace(strings.ToLower(username))
	if !portalUsernamePattern.MatchString(username) || len(passwordHash) < 20 {
		return platform.ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var userID string
	if err := tx.QueryRowContext(ctx, `UPDATE human_users SET password_hash=$2,password_changed_at=now(),updated_at=now() WHERE username=$1 RETURNING id`, username, passwordHash).Scan(&userID); errors.Is(err, sql.ErrNoRows) {
		return platform.ErrNotFound
	} else if err != nil {
		return err
	}
	if err := insertActorAudit(ctx, tx, "operator", "cli", "", "", "human_user.password_rotated", "succeeded", map[string]string{"user_id": userID}); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) DisableHuman(ctx context.Context, username string) error {
	username = strings.TrimSpace(strings.ToLower(username))
	if !portalUsernamePattern.MatchString(username) {
		return platform.ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var userID string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM human_users WHERE username=$1 FOR UPDATE`, username).Scan(&userID); errors.Is(err, sql.ErrNoRows) {
		return platform.ErrNotFound
	} else if err != nil {
		return err
	}
	var currentOwner bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (
		SELECT 1 FROM organisation_people person
		JOIN organisation_ownerships ownership ON ownership.organisation_id=person.organisation_id AND ownership.person_id=person.id AND ownership.ended_at IS NULL
		WHERE person.user_id=$1
	)`, userID).Scan(&currentOwner); err != nil {
		return err
	}
	if currentOwner {
		return platform.ErrConflict
	}
	if _, err := tx.ExecContext(ctx, `UPDATE human_users SET enabled=false,updated_at=now() WHERE id=$1`, userID); err != nil {
		return mapWriteError("disable human user", err)
	}
	if err := insertActorAudit(ctx, tx, "operator", "cli", "", "", "human_user.disabled", "succeeded", map[string]string{"user_id": userID}); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) CreatePortalSession(ctx context.Context, username, password string, tokenHash [32]byte, expiresAt, now time.Time) (platform.PortalSession, error) {
	username = strings.TrimSpace(strings.ToLower(username))
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return platform.PortalSession{}, err
	}
	defer tx.Rollback()
	var userID string
	var hash sql.NullString
	var enabled bool
	err = tx.QueryRowContext(ctx, `SELECT id,password_hash,enabled FROM human_users WHERE username=$1`, username).Scan(&userID, &hash, &enabled)
	if errors.Is(err, sql.ErrNoRows) {
		_ = humanauth.VerifyPassword(dummyPasswordHash, password)
		_ = insertActorAudit(ctx, tx, "system", "portal_auth", "", "", "portal.login", "failed", map[string]string{"reason": "invalid_credentials"})
		_ = tx.Commit()
		return platform.PortalSession{}, platform.ErrUnauthenticated
	}
	if err != nil {
		return platform.PortalSession{}, err
	}
	if !enabled || !hash.Valid || !humanauth.VerifyPassword(hash.String, password) {
		_ = insertActorAudit(ctx, tx, "system", "portal_auth", "", "", "portal.login", "failed", map[string]string{"reason": "invalid_credentials"})
		_ = tx.Commit()
		return platform.PortalSession{}, platform.ErrUnauthenticated
	}
	memberships, err := listMemberships(ctx, tx, userID)
	if err != nil {
		return platform.PortalSession{}, err
	}
	if len(memberships) == 0 {
		_ = insertActorAudit(ctx, tx, "human_user", userID, "", "", "portal.login", "failed", map[string]string{"reason": "no_membership"})
		_ = tx.Commit()
		return platform.PortalSession{}, platform.ErrUnauthenticated
	}
	sessionID, err := ids.New("ses")
	if err != nil {
		return platform.PortalSession{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO portal_sessions(id,user_id,current_membership_id,token_hash,created_at,last_seen_at,authenticated_at,expires_at) VALUES($1,$2,$3,$4,$5,$5,$5,$6)`, sessionID, userID, memberships[0].ID, tokenHash[:], now, expiresAt); err != nil {
		return platform.PortalSession{}, mapWriteError("create portal session", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE human_users SET last_sign_in_at=$2,updated_at=now() WHERE id=$1`, userID, now); err != nil {
		return platform.PortalSession{}, err
	}
	if err := insertActorAudit(ctx, tx, "human_user", userID, memberships[0].OrganisationID, memberships[0].ProjectID, "portal.login", "succeeded", map[string]string{"session_id": sessionID, "membership_id": memberships[0].ID}); err != nil {
		return platform.PortalSession{}, err
	}
	var user platform.PortalUser
	if err := tx.QueryRowContext(ctx, `SELECT id,username,display_name FROM human_users WHERE id=$1`, userID).Scan(&user.ID, &user.Username, &user.DisplayName); err != nil {
		return platform.PortalSession{}, err
	}
	if err := tx.Commit(); err != nil {
		return platform.PortalSession{}, err
	}
	return platform.PortalSession{ID: sessionID, User: user, Current: memberships[0], Memberships: memberships, AuthenticatedAt: now, ExpiresAt: expiresAt}, nil
}

func (s *Store) AuthenticatePortalSession(ctx context.Context, tokenHash [32]byte, now time.Time) (platform.PortalSession, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return platform.PortalSession{}, err
	}
	defer tx.Rollback()
	session, err := loadSession(ctx, tx, tokenHash, now, true)
	if err != nil {
		return platform.PortalSession{}, err
	}
	if err := tx.Commit(); err != nil {
		return platform.PortalSession{}, err
	}
	return session, nil
}

// ReauthenticatePortalSession verifies the current human user's password and
// advances only the existing session's recent-authentication timestamp. It
// does not create a new session or rotate either browser cookie.
func (s *Store) ReauthenticatePortalSession(ctx context.Context, tokenHash [32]byte, password string, now time.Time) (platform.PortalSession, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return platform.PortalSession{}, err
	}
	defer tx.Rollback()
	session, err := loadSession(ctx, tx, tokenHash, now, false)
	if err != nil {
		return platform.PortalSession{}, err
	}
	var passwordHash sql.NullString
	var enabled bool
	if err := tx.QueryRowContext(ctx, `SELECT password_hash,enabled FROM human_users WHERE id=$1`, session.User.ID).Scan(&passwordHash, &enabled); err != nil {
		return platform.PortalSession{}, err
	}
	if !enabled || !passwordHash.Valid || !humanauth.VerifyPassword(passwordHash.String, password) {
		if err := insertActorAudit(ctx, tx, "human_user", session.User.ID, session.Current.OrganisationID, session.Current.ProjectID, "portal.reauthentication", "failed", map[string]string{"reason": "invalid_credentials", "session_id": session.ID}); err != nil {
			return platform.PortalSession{}, err
		}
		if err := tx.Commit(); err != nil {
			return platform.PortalSession{}, err
		}
		return platform.PortalSession{}, platform.ErrUnauthenticated
	}
	if _, err := tx.ExecContext(ctx, `UPDATE portal_sessions SET authenticated_at=$2,last_seen_at=$2 WHERE id=$1 AND revoked_at IS NULL`, session.ID, now); err != nil {
		return platform.PortalSession{}, err
	}
	if err := insertActorAudit(ctx, tx, "human_user", session.User.ID, session.Current.OrganisationID, session.Current.ProjectID, "portal.reauthentication", "succeeded", map[string]string{"session_id": session.ID}); err != nil {
		return platform.PortalSession{}, err
	}
	if err := tx.Commit(); err != nil {
		return platform.PortalSession{}, err
	}
	session.AuthenticatedAt = now
	return session, nil
}

func (s *Store) RevokePortalSession(ctx context.Context, tokenHash [32]byte, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	session, err := loadSession(ctx, tx, tokenHash, now, false)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE portal_sessions SET revoked_at=GREATEST($2,created_at) WHERE id=$1 AND revoked_at IS NULL`, session.ID, now); err != nil {
		return err
	}
	if err := insertActorAudit(ctx, tx, "human_user", session.User.ID, session.Current.OrganisationID, session.Current.ProjectID, "portal.logout", "succeeded", map[string]string{"session_id": session.ID}); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) SwitchPortalContext(ctx context.Context, tokenHash [32]byte, membershipID string, now time.Time) (platform.PortalSession, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return platform.PortalSession{}, err
	}
	defer tx.Rollback()
	session, err := loadSession(ctx, tx, tokenHash, now, false)
	if err != nil {
		return platform.PortalSession{}, err
	}
	var selected *platform.PortalMembership
	for index := range session.Memberships {
		if session.Memberships[index].ID == membershipID {
			selected = &session.Memberships[index]
			break
		}
	}
	if selected == nil {
		return platform.PortalSession{}, platform.ErrForbidden
	}
	if _, err := tx.ExecContext(ctx, `UPDATE portal_sessions SET current_membership_id=$2,last_seen_at=$3 WHERE id=$1`, session.ID, selected.ID, now); err != nil {
		return platform.PortalSession{}, err
	}
	if err := insertActorAudit(ctx, tx, "human_user", session.User.ID, selected.OrganisationID, selected.ProjectID, "portal.context_switched", "succeeded", map[string]string{"membership_id": selected.ID}); err != nil {
		return platform.PortalSession{}, err
	}
	session.Current = *selected
	if err := tx.Commit(); err != nil {
		return platform.PortalSession{}, err
	}
	return session, nil
}

func loadSession(ctx context.Context, tx *sql.Tx, tokenHash [32]byte, now time.Time, touch bool) (platform.PortalSession, error) {
	var result platform.PortalSession
	var currentID string
	err := tx.QueryRowContext(ctx, `
		SELECT ps.id,u.id,u.username,u.display_name,ps.current_membership_id,ps.authenticated_at,ps.expires_at
		  FROM portal_sessions ps
		  JOIN human_users u ON u.id=ps.user_id AND u.enabled
		  JOIN human_memberships hm ON hm.id=ps.current_membership_id AND hm.user_id=u.id AND hm.enabled
		 WHERE ps.token_hash=$1 AND ps.revoked_at IS NULL AND ps.expires_at>$2
		 FOR UPDATE OF ps`, tokenHash[:], now).Scan(&result.ID, &result.User.ID, &result.User.Username, &result.User.DisplayName, &currentID, &result.AuthenticatedAt, &result.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return platform.PortalSession{}, platform.ErrUnauthenticated
	}
	if err != nil {
		return platform.PortalSession{}, err
	}
	result.Memberships, err = listMemberships(ctx, tx, result.User.ID)
	if err != nil {
		return platform.PortalSession{}, err
	}
	for _, membership := range result.Memberships {
		if membership.ID == currentID {
			result.Current = membership
			break
		}
	}
	if result.Current.ID == "" {
		return platform.PortalSession{}, platform.ErrUnauthenticated
	}
	if touch {
		if _, err := tx.ExecContext(ctx, `UPDATE portal_sessions SET last_seen_at=$2 WHERE id=$1`, result.ID, now); err != nil {
			return platform.PortalSession{}, err
		}
	}
	return result, nil
}

func listMemberships(ctx context.Context, tx *sql.Tx, userID string) ([]platform.PortalMembership, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT hm.id,o.id,o.name,o.slug,p.id,p.name,p.slug,e.id,e.name,e.slug,hm.role
		  FROM human_memberships hm
		  JOIN organisations o ON o.id=hm.organisation_id
		  JOIN projects p ON p.id=hm.project_id AND p.organisation_id=o.id
		  JOIN environments e ON e.id=hm.environment_id AND e.project_id=p.id AND e.organisation_id=o.id
		 WHERE hm.user_id=$1 AND hm.enabled
		 ORDER BY CASE hm.role WHEN 'org_admin' THEN 0 WHEN 'project_admin' THEN 1 WHEN 'developer' THEN 2 ELSE 3 END,o.name,p.name,e.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []platform.PortalMembership
	for rows.Next() {
		var item platform.PortalMembership
		if err := rows.Scan(&item.ID, &item.OrganisationID, &item.OrganisationName, &item.OrganisationSlug, &item.ProjectID, &item.ProjectName, &item.ProjectSlug, &item.EnvironmentID, &item.EnvironmentName, &item.EnvironmentSlug, &item.Role); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) ListPortalAccess(ctx context.Context, session platform.PortalSession) ([]platform.PortalServiceAccount, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT sa.id,sa.name,sa.created_at,k.name,k.key_prefix,k.scopes,k.created_at,k.expires_at,k.last_used_at,k.revoked_at
		  FROM service_accounts sa
		  LEFT JOIN api_keys k ON k.service_account_id=sa.id
		 WHERE sa.organisation_id=$1 AND sa.project_id=$2 AND sa.environment_id=$3
		 ORDER BY sa.name,k.created_at DESC`, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ordered := make([]platform.PortalServiceAccount, 0)
	indexes := make(map[string]int)
	now := s.now().UTC()
	for rows.Next() {
		var accountID, accountName string
		var accountCreated time.Time
		var keyName, prefix sql.NullString
		var scopes []byte
		var keyCreated sql.NullTime
		var expires, lastUsed, revoked sql.NullTime
		if err := rows.Scan(&accountID, &accountName, &accountCreated, &keyName, &prefix, &scopes, &keyCreated, &expires, &lastUsed, &revoked); err != nil {
			return nil, err
		}
		index, ok := indexes[accountID]
		if !ok {
			index = len(ordered)
			indexes[accountID] = index
			ordered = append(ordered, platform.PortalServiceAccount{ID: accountID, Name: accountName, CreatedAt: accountCreated})
		}
		if !prefix.Valid {
			continue
		}
		var decoded []string
		if err := json.Unmarshal(scopes, &decoded); err != nil {
			return nil, err
		}
		status := "active"
		if revoked.Valid {
			status = "revoked"
		} else if expires.Valid && !expires.Time.After(now) {
			status = "expired"
		}
		ordered[index].Keys = append(ordered[index].Keys, platform.PortalKeyRecord{Name: keyName.String, Prefix: prefix.String, Status: status, Scopes: decoded, CreatedAt: keyCreated.Time, ExpiresAt: nullTimePointer(expires), LastUsedAt: nullTimePointer(lastUsed), RevokedAt: nullTimePointer(revoked)})
	}
	return ordered, rows.Err()
}

func (s *Store) CreatePortalServiceAccount(ctx context.Context, session platform.PortalSession, name string) (platform.PortalServiceAccount, error) {
	name = strings.TrimSpace(name)
	if !portalNamePattern.MatchString(name) {
		return platform.PortalServiceAccount{}, platform.ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return platform.PortalServiceAccount{}, err
	}
	defer tx.Rollback()
	if _, err := requireCompanyOwner(ctx, tx, session); err != nil {
		return platform.PortalServiceAccount{}, err
	}
	id, err := ids.New("sa")
	if err != nil {
		return platform.PortalServiceAccount{}, err
	}
	var createdAt time.Time
	if err := tx.QueryRowContext(ctx, `INSERT INTO service_accounts(id,organisation_id,project_id,environment_id,name) VALUES($1,$2,$3,$4,$5) RETURNING created_at`, id, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID, name).Scan(&createdAt); err != nil {
		return platform.PortalServiceAccount{}, mapWriteError("create service account", err)
	}
	if err := insertActorAudit(ctx, tx, "human_user", session.User.ID, session.Current.OrganisationID, session.Current.ProjectID, "service_account.created", "succeeded", map[string]string{"service_account_id": id}); err != nil {
		return platform.PortalServiceAccount{}, err
	}
	if err := tx.Commit(); err != nil {
		return platform.PortalServiceAccount{}, err
	}
	return platform.PortalServiceAccount{ID: id, Name: name, CreatedAt: createdAt, Keys: []platform.PortalKeyRecord{}}, nil
}

func (s *Store) IssuePortalKey(ctx context.Context, session platform.PortalSession, spec platform.PortalKeyIssueSpec) (platform.PortalKeyResult, error) {
	spec.Name = strings.TrimSpace(spec.Name)
	if !portalNamePattern.MatchString(spec.Name) {
		return platform.PortalKeyResult{}, platform.ErrInvalid
	}
	scopes, err := provisioning.ValidateScopes(spec.Scopes)
	if err != nil {
		return platform.PortalKeyResult{}, err
	}
	now := s.now().UTC()
	if spec.ExpiresAt == nil || spec.ExpiresAt.Before(now.Add(time.Hour)) || spec.ExpiresAt.After(now.Add(365*24*time.Hour)) {
		return platform.PortalKeyResult{}, platform.ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return platform.PortalKeyResult{}, err
	}
	defer tx.Rollback()
	if _, err := requireCompanyOwner(ctx, tx, session); err != nil {
		return platform.PortalKeyResult{}, err
	}
	var accountID string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM service_accounts WHERE id=$1 AND organisation_id=$2 AND project_id=$3 AND environment_id=$4 FOR UPDATE`, spec.ServiceAccountID, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID).Scan(&accountID); errors.Is(err, sql.ErrNoRows) {
		return platform.PortalKeyResult{}, platform.ErrNotFound
	} else if err != nil {
		return platform.PortalKeyResult{}, err
	}
	var existingName int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM api_keys WHERE service_account_id=$1 AND name=$2 LIMIT 1`, accountID, spec.Name).Scan(&existingName); err == nil {
		return platform.PortalKeyResult{}, platform.ErrConflict
	} else if !errors.Is(err, sql.ErrNoRows) {
		return platform.PortalKeyResult{}, err
	}
	var rotatedFrom interface{}
	if spec.RotatedFromPrefix != "" {
		var oldID string
		if err := tx.QueryRowContext(ctx, `SELECT id FROM api_keys WHERE key_prefix=$1 AND service_account_id=$2 AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at>$3) FOR UPDATE`, spec.RotatedFromPrefix, accountID, now).Scan(&oldID); errors.Is(err, sql.ErrNoRows) {
			return platform.PortalKeyResult{}, platform.ErrNotFound
		} else if err != nil {
			return platform.PortalKeyResult{}, err
		}
		rotatedFrom = oldID
	}
	generated, err := credentials.Generate()
	if err != nil {
		return platform.PortalKeyResult{}, err
	}
	keyID, err := ids.New("key")
	if err != nil {
		return platform.PortalKeyResult{}, err
	}
	scopesJSON, _ := json.Marshal(scopes)
	if _, err := tx.ExecContext(ctx, `INSERT INTO api_keys(id,service_account_id,key_prefix,key_hash,scopes,name,expires_at,rotated_from_key_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, keyID, accountID, generated.Prefix, generated.Digest[:], scopesJSON, spec.Name, spec.ExpiresAt, rotatedFrom); err != nil {
		return platform.PortalKeyResult{}, mapWriteError("issue portal API key", err)
	}
	action := "api_key.issued"
	if spec.RotatedFromPrefix != "" {
		action = "api_key.rotated_with_overlap"
	}
	if err := insertActorAudit(ctx, tx, "human_user", session.User.ID, session.Current.OrganisationID, session.Current.ProjectID, action, "succeeded", map[string]string{
		"service_account_id": accountID,
		"key_prefix":         generated.Prefix,
		"key_name":           spec.Name,
		"scopes":             strings.Join(scopes, ","),
		"expires_at":         spec.ExpiresAt.UTC().Format(time.RFC3339),
	}); err != nil {
		return platform.PortalKeyResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return platform.PortalKeyResult{}, err
	}
	return platform.PortalKeyResult{Name: spec.Name, Prefix: generated.Prefix, APIKey: generated.Token, Scopes: scopes, ExpiresAt: spec.ExpiresAt}, nil
}

func (s *Store) RevokePortalKey(ctx context.Context, session platform.PortalSession, prefix string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := requireCompanyOwner(ctx, tx, session); err != nil {
		return err
	}
	var accountID string
	err = tx.QueryRowContext(ctx, `
		UPDATE api_keys k SET revoked_at=now()
		  FROM service_accounts sa
		 WHERE k.key_prefix=$1 AND k.revoked_at IS NULL AND sa.id=k.service_account_id
		   AND sa.organisation_id=$2 AND sa.project_id=$3 AND sa.environment_id=$4
		 RETURNING sa.id`, prefix, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID).Scan(&accountID)
	if errors.Is(err, sql.ErrNoRows) {
		return platform.ErrNotFound
	}
	if err != nil {
		return err
	}
	if err := insertActorAudit(ctx, tx, "human_user", session.User.ID, session.Current.OrganisationID, session.Current.ProjectID, "api_key.revoked", "succeeded", map[string]string{"service_account_id": accountID, "key_prefix": prefix}); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) GetPortalServicePlan(ctx context.Context, session platform.PortalSession, modelAlias string) (platform.PortalServicePlan, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id,m.alias
		  FROM tenant_routes r JOIN models m ON m.id=r.model_id
		 WHERE r.organisation_id=$1 AND r.project_id=$2 AND r.environment_id=$3
		   AND ($4='' OR m.alias=$4)
		 ORDER BY m.alias`, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID, modelAlias)
	if err != nil {
		return platform.PortalServicePlan{}, err
	}
	defer rows.Close()
	type selectedRoute struct{ id, alias string }
	var routes []selectedRoute
	for rows.Next() {
		var item selectedRoute
		if err := rows.Scan(&item.id, &item.alias); err != nil {
			return platform.PortalServicePlan{}, err
		}
		routes = append(routes, item)
	}
	if err := rows.Err(); err != nil {
		return platform.PortalServicePlan{}, err
	}
	if len(routes) == 0 {
		return platform.PortalServicePlan{Available: false, Source: "operator_registry", Finality: "unknown"}, nil
	}
	if len(routes) > 1 {
		return platform.PortalServicePlan{Available: false, Ambiguous: true, Source: "operator_registry", Finality: "unknown"}, nil
	}
	result := platform.PortalServicePlan{ModelAlias: routes[0].alias, Source: "operator_registry", Finality: "unknown"}
	var requestAllowance, tokenAllowance, acceleratorCount sql.NullInt64
	var requestUnit, requestPeriod, tokenUnit, tokenPeriod, resourceClass sql.NullString
	var effectiveAt time.Time
	err = s.db.QueryRowContext(ctx, `
		SELECT sp.code,sp.name,sp.capacity_mode,
		       sp.shared_request_allowance,sp.shared_request_allowance_unit,sp.shared_request_allowance_period,
		       sp.shared_token_allowance,sp.shared_token_allowance_unit,sp.shared_token_allowance_period,
		       sp.dedicated_resource_class,sp.dedicated_accelerator_count,
		       tsp.status,tsp.source_label,tsp.finality,tsp.effective_at
		  FROM tenant_service_plans tsp
		  JOIN service_plans sp ON sp.organisation_id=tsp.organisation_id AND sp.id=tsp.service_plan_id
		 WHERE tsp.organisation_id=$1 AND tsp.project_id=$2 AND tsp.environment_id=$3 AND tsp.route_id=$4 AND tsp.status='active'`, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID, routes[0].id).Scan(
		&result.Code, &result.Name, &result.CapacityMode,
		&requestAllowance, &requestUnit, &requestPeriod, &tokenAllowance, &tokenUnit, &tokenPeriod,
		&resourceClass, &acceleratorCount, &result.Status, &result.Source, &result.Finality, &effectiveAt)
	if errors.Is(err, sql.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return platform.PortalServicePlan{}, err
	}
	result.Available = true
	result.SharedRequestAllowance = nullInt64Pointer(requestAllowance)
	result.SharedRequestAllowanceUnit = nullStringPointer(requestUnit)
	result.SharedRequestAllowancePeriod = nullStringPointer(requestPeriod)
	result.SharedTokenAllowance = nullInt64Pointer(tokenAllowance)
	result.SharedTokenAllowanceUnit = nullStringPointer(tokenUnit)
	result.SharedTokenAllowancePeriod = nullStringPointer(tokenPeriod)
	result.DedicatedResourceClass = nullStringPointer(resourceClass)
	result.DedicatedAcceleratorCount = nullInt64Pointer(acceleratorCount)
	result.EffectiveAt = &effectiveAt
	return result, nil
}

func (s *Store) ListPortalExport(ctx context.Context, session platform.PortalSession, filter platform.UsageFilter, format string) ([]platform.PortalExportRow, error) {
	if format != "csv" && format != "json" {
		return nil, platform.ErrInvalid
	}
	limit := filter.Limit
	if limit <= 0 || limit > 10000 {
		limit = 10000
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT ir.id,ir.started_at,ir.completed_at,sa.name,ir.model_alias,m.version,COALESCE(ir.executed_model,''),
		       CASE WHEN pa.has_attempt THEN t.execution_class END,
		       CASE WHEN pa.has_attempt THEN t.capacity_mode END,
		       ir.status,COALESCE(ir.http_status,0),COALESCE(ir.error_class,''),ir.duration_ms,
		       ir.input_tokens,ir.output_tokens,ir.cached_tokens,ir.reasoning_tokens,ir.usage_finality
		  FROM inference_requests ir
		  JOIN service_accounts sa ON sa.id=ir.service_account_id
		  LEFT JOIN models m ON m.id=ir.bound_model_id
		  LEFT JOIN inference_targets t ON t.id=ir.bound_target_id
		  LEFT JOIN LATERAL (
		      SELECT true AS has_attempt FROM provider_attempts p
		       WHERE p.inference_request_id=ir.id LIMIT 1
		  ) pa ON true
		 WHERE ir.organisation_id=$1 AND ir.project_id=$2 AND ir.environment_id=$3
		   AND ir.started_at >= $4 AND ir.started_at < $5 AND ($6='' OR ir.model_alias=$6)
		 ORDER BY ir.started_at DESC LIMIT $7`, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID, filter.From, filter.To, filter.ModelAlias, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []platform.PortalExportRow
	for rows.Next() {
		var item platform.PortalExportRow
		var completed sql.NullTime
		var modelVersion, executionClass, capacityMode sql.NullString
		var duration, input, output, cached, reasoning sql.NullInt64
		if err := rows.Scan(&item.RequestID, &item.StartedAt, &completed, &item.ServiceAccount, &item.ModelAlias, &modelVersion, &item.ExecutedModel, &executionClass, &capacityMode, &item.Status, &item.HTTPStatus, &item.ErrorClass, &duration, &input, &output, &cached, &reasoning, &item.UsageFinality); err != nil {
			return nil, err
		}
		item.ModelVersion = nullStringPointer(modelVersion)
		item.ExecutionClass = nullStringPointer(executionClass)
		item.CapacityMode = nullStringPointer(capacityMode)
		item.CompletedAt = nullTimePointer(completed)
		item.DurationMS = nullInt64Pointer(duration)
		item.InputTokens = nullInt64Pointer(input)
		item.OutputTokens = nullInt64Pointer(output)
		item.CachedTokens = nullInt64Pointer(cached)
		item.ReasoningTokens = nullInt64Pointer(reasoning)
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(result) > limit {
		return nil, platform.ErrConflict
	}
	if err := s.auditPortalRead(ctx, session, "usage.exported", map[string]string{
		"format":        format,
		"from":          filter.From.UTC().Format(time.RFC3339),
		"to":            filter.To.UTC().Format(time.RFC3339),
		"timezone":      "UTC",
		"scope":         "authenticated_project_environment",
		"membership_id": session.Current.ID,
		"source":        "inference_requests",
	}); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Store) ListPortalRollups(ctx context.Context, session platform.PortalSession, filter platform.UsageFilter) ([]platform.PortalUsageRollup, error) {
	from := filter.From.UTC().Truncate(time.Hour)
	to := filter.To.UTC().Truncate(time.Hour)
	if !to.Equal(filter.To.UTC()) {
		to = to.Add(time.Hour)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.bucket_start,sa.name,u.model_alias,
		       u.logical_requests,u.successful_requests,u.failed_requests,u.blocked_requests,u.cancelled_requests,u.in_progress_requests,
		       u.input_tokens,u.input_known_requests,u.output_tokens,u.output_known_requests,
		       u.cached_tokens,u.cached_known_requests,u.reasoning_tokens,u.reasoning_known_requests,
		       u.peak_concurrency,u.p50_latency_ms,u.p95_latency_ms,u.source,u.finality,u.refreshed_at
		  FROM usage_rollups_hourly_v2 u
		  JOIN service_accounts sa ON sa.id=u.service_account_id
		 WHERE u.organisation_id=$1 AND u.project_id=$2 AND u.environment_id=$3
		   AND u.bucket_start >= $4 AND u.bucket_start < $5
		   AND ($6='' OR u.model_alias=$6)
		 ORDER BY u.bucket_start,sa.name,u.model_alias`, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID, from, to, filter.ModelAlias)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []platform.PortalUsageRollup
	for rows.Next() {
		var item platform.PortalUsageRollup
		var input, output, cached, reasoning, peak, p50, p95 sql.NullInt64
		var inputKnown, outputKnown, cachedKnown, reasoningKnown int64
		if err := rows.Scan(&item.BucketStart, &item.ServiceAccount, &item.ModelAlias,
			&item.LogicalRequests, &item.SuccessfulRequests, &item.FailedRequests, &item.BlockedRequests, &item.CancelledRequests, &item.InProgressRequests,
			&input, &inputKnown, &output, &outputKnown, &cached, &cachedKnown, &reasoning, &reasoningKnown,
			&peak, &p50, &p95, &item.Source, &item.Finality, &item.RefreshedAt); err != nil {
			return nil, err
		}
		item.InputTokens = completeRollupToken(input, inputKnown, item.SuccessfulRequests)
		item.InputKnownRequests = inputKnown
		item.OutputTokens = completeRollupToken(output, outputKnown, item.SuccessfulRequests)
		item.OutputKnownRequests = outputKnown
		item.CachedTokens = completeRollupToken(cached, cachedKnown, item.SuccessfulRequests)
		item.CachedKnownRequests = cachedKnown
		item.ReasoningTokens = completeRollupToken(reasoning, reasoningKnown, item.SuccessfulRequests)
		item.ReasoningKnownRequests = reasoningKnown
		item.TokenEligibleRequests = item.SuccessfulRequests
		item.PeakConcurrency = nullInt64Pointer(peak)
		item.P50LatencyMS = nullInt64Pointer(p50)
		item.P95LatencyMS = nullInt64Pointer(p95)
		throughput := float64(item.LogicalRequests) / 3600
		item.ThroughputRPS = &throughput
		result = append(result, item)
	}
	return result, rows.Err()
}

func completeRollupToken(value sql.NullInt64, known, eligible int64) *int64 {
	if eligible == 0 || known != eligible || !value.Valid {
		return nil
	}
	copy := value.Int64
	return &copy
}

func (s *Store) ListPortalObservations(ctx context.Context, session platform.PortalSession, modelAlias string, now time.Time) ([]platform.PortalObservation, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.alias,m.version,t.execution_class,t.capacity_mode,
		       CASE WHEN NOT r.enabled OR NOT m.enabled OR NOT t.enabled THEN 'unavailable' ELSE 'enabled' END,
		       t.probe_enabled,o.status,o.observed_at,o.fresh_until,o.latency_ms,
		       scoped.status,scoped.completed_at,success.last_success_at
		  FROM tenant_routes r
		  JOIN models m ON m.id=r.model_id
		  JOIN inference_targets t ON t.id=r.target_id
		  LEFT JOIN LATERAL (
		      SELECT status,observed_at,fresh_until,latency_ms,credential_available
		        FROM target_health_observations WHERE target_id=t.id ORDER BY observed_at DESC LIMIT 1
		  ) o ON true
		  LEFT JOIN LATERAL (
		      SELECT CASE
		               WHEN ir.status='succeeded' THEN 'operational'
		               WHEN ir.status='failed' THEN 'degraded'
		               ELSE 'unknown'
		             END AS status,ir.completed_at
		        FROM inference_requests ir
		       WHERE ir.organisation_id=r.organisation_id AND ir.project_id=r.project_id AND ir.environment_id=r.environment_id
		         AND ir.route_id=r.id AND ir.bound_target_id=r.target_id AND ir.bound_model_id=r.model_id
		         AND ir.route_binding_generation=r.binding_generation AND ir.completed_at IS NOT NULL
		         AND (ir.status='succeeded' OR (ir.status='failed' AND ir.error_class IN (
		             'target_configuration','upstream_rate_limited','upstream_timeout','upstream_transport',
		             'upstream_unavailable','upstream_error','invalid_upstream_response','upstream_response_too_large'
		         )))
		       ORDER BY ir.completed_at DESC LIMIT 1
		  ) scoped ON true
		  LEFT JOIN LATERAL (
		      SELECT max(ir.completed_at) AS last_success_at
		        FROM inference_requests ir
		       WHERE ir.organisation_id=r.organisation_id AND ir.project_id=r.project_id AND ir.environment_id=r.environment_id
		         AND ir.route_id=r.id AND ir.bound_target_id=r.target_id AND ir.bound_model_id=r.model_id
		         AND ir.route_binding_generation=r.binding_generation AND ir.status='succeeded'
		  ) success ON true
		 WHERE r.organisation_id=$1 AND r.project_id=$2 AND r.environment_id=$3
		   AND ($4='' OR m.alias=$4)
		 ORDER BY m.alias`, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID, modelAlias)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []platform.PortalObservation
	for rows.Next() {
		var item platform.PortalObservation
		var status sql.NullString
		var observed, fresh sql.NullTime
		var latency sql.NullInt64
		var scopedStatus sql.NullString
		var scopedAt, lastSuccess sql.NullTime
		if err := rows.Scan(&item.ModelAlias, &item.ModelVersion, &item.ExecutionClass, &item.CapacityMode, &item.RegistryStatus, &item.ProbeEnabled, &status, &observed, &fresh, &latency, &scopedStatus, &scopedAt, &lastSuccess); err != nil {
			return nil, err
		}
		item.LatestInferenceStatus = "unknown"
		item.State = "unknown"
		item.EndpointPath = "POST /v1/chat/completions"
		if scopedStatus.Valid {
			item.LatestInferenceStatus = scopedStatus.String
			item.LatestInferenceAt = nullTimePointer(scopedAt)
			item.LastObservationAt = nullTimePointer(scopedAt)
		}
		item.LastSuccessAt = nullTimePointer(lastSuccess)
		item.ProbeStatus = "unknown"
		item.Freshness = "unavailable"
		item.Source = "target_registry"
		item.StatusDetail = "Registry policy allows the route, but no fresh opted-in compatible probe is available; current callability is unknown. Current-binding inference observations are informational only."
		if item.RegistryStatus == "unavailable" {
			item.State = "unavailable"
			item.StatusDetail = "Registry policy marks this route unavailable. This is not another tenant's inference evidence."
		}
		if status.Valid {
			item.ProbeStatus = status.String
			item.ObservedAt = nullTimePointer(observed)
			item.FreshUntil = nullTimePointer(fresh)
			item.LatencyMS = nullInt64Pointer(latency)
			item.Source = "opt_in_compatible_probe"
			if fresh.Time.After(now) {
				item.Freshness = "fresh"
				if item.RegistryStatus != "unavailable" && item.ProbeEnabled {
					item.State = status.String
					item.StatusDetail = "Current callability is derived from a fresh, explicitly opted-in compatible probe. Current-binding tenant inference observations remain separately informational."
				}
			} else {
				item.Freshness = "stale"
				if item.RegistryStatus != "unavailable" {
					item.State = "unknown"
					item.StatusDetail = "The latest opted-in compatible probe is stale, so current callability is unknown. Current-binding inference observations are informational only."
				}
			}
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) GetRollupCheckpoint(ctx context.Context, session platform.PortalSession) (platform.RollupCheckpoint, error) {
	var result platform.RollupCheckpoint
	var started, completed, rangeFrom, rangeTo sql.NullTime
	var sourceRows sql.NullInt64
	var safeClass sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT status,last_started_at,last_completed_at,range_from,range_to,source_rows,safe_error_class FROM worker_checkpoints WHERE worker_name='usage_rollup' AND organisation_id=$1 AND project_id=$2 AND environment_id=$3`, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID).Scan(&result.Status, &started, &completed, &rangeFrom, &rangeTo, &sourceRows, &safeClass)
	if errors.Is(err, sql.ErrNoRows) {
		return platform.RollupCheckpoint{Status: "unavailable"}, nil
	}
	if err != nil {
		return platform.RollupCheckpoint{}, err
	}
	result.LastStartedAt = nullTimePointer(started)
	result.LastCompletedAt = nullTimePointer(completed)
	result.RangeFrom = nullTimePointer(rangeFrom)
	result.RangeTo = nullTimePointer(rangeTo)
	result.SourceRows = nullInt64Pointer(sourceRows)
	result.SafeErrorClass = nullStringPointer(safeClass)
	return result, nil
}

func (s *Store) auditPortalRead(ctx context.Context, session platform.PortalSession, action string, metadata map[string]string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := insertActorAudit(ctx, tx, "human_user", session.User.ID, session.Current.OrganisationID, session.Current.ProjectID, action, "succeeded", metadata); err != nil {
		return err
	}
	return tx.Commit()
}

func insertActorAudit(ctx context.Context, tx *sql.Tx, actorType, actorID, orgID, projectID, action, result string, metadata map[string]string) error {
	id, err := ids.New("aud")
	if err != nil {
		return err
	}
	correlationID, err := ids.New("act")
	if err != nil {
		return err
	}
	safeJSON, _ := json.Marshal(metadata)
	var orgValue, projectValue interface{}
	if orgID != "" {
		orgValue = orgID
	}
	if projectID != "" {
		projectValue = projectID
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_events(id,actor_type,actor_id,organisation_id,project_id,action,result,correlation_id,safe_metadata) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id, actorType, actorID, orgValue, projectValue, action, result, correlationID, safeJSON)
	return err
}

func nullTimePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	copy := value.Time
	return &copy
}

func nullInt64Pointer(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	copy := value.Int64
	return &copy
}

func nullStringPointer(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	copy := value.String
	return &copy
}

func digestString(value string) [32]byte { return sha256.Sum256([]byte(value)) }

var (
	_ platform.PortalStore      = (*Store)(nil)
	_ platform.HumanProvisioner = (*Store)(nil)
	_                           = sort.Strings
	_                           = fmt.Sprintf
	_                           = digestString
)
