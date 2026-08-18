package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"alzette/internal/ids"
	"alzette/internal/platform"
	"alzette/internal/workforce"
)

func (s *Store) LoadAccess(ctx context.Context, session platform.PortalSession) (workforce.Access, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return workforce.Access{}, err
	}
	defer tx.Rollback()
	result := workforce.Access{}
	var ownerPersonID string
	err = tx.QueryRowContext(ctx, `SELECT person_id FROM organisation_ownerships WHERE organisation_id=$1 AND ended_at IS NULL`, session.Current.OrganisationID).Scan(&ownerPersonID)
	if errors.Is(err, sql.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return workforce.Access{}, err
	}
	result.Configured = true
	var currentEnabled bool
	err = tx.QueryRowContext(ctx, `SELECT id,enabled FROM organisation_people WHERE organisation_id=$1 AND user_id=$2`, session.Current.OrganisationID, session.User.ID).Scan(&result.CurrentPersonID, &currentEnabled)
	if errors.Is(err, sql.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return workforce.Access{}, err
	}
	if !currentEnabled {
		return result, nil
	}
	result.Relationship = workforce.RelationshipEmployee
	if result.CurrentPersonID == ownerPersonID {
		result.Relationship = workforce.RelationshipOwner
		result.CanManage = true
	}
	result.AvailableModels, err = loadAvailableModels(ctx, tx, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID)
	if err != nil {
		return workforce.Access{}, err
	}
	result.People, err = loadPeople(ctx, tx, session, result.CurrentPersonID, ownerPersonID, result.CanManage)
	if err != nil {
		return workforce.Access{}, err
	}
	result.Groups, err = loadGroups(ctx, tx, session, result.CurrentPersonID, result.CanManage)
	if err != nil {
		return workforce.Access{}, err
	}
	if result.CanManage {
		result.Invitations, err = loadInvitations(ctx, tx, session.Current.OrganisationID, s.now().UTC())
		if err != nil {
			return workforce.Access{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return workforce.Access{}, err
	}
	return result, nil
}

func loadPeople(ctx context.Context, tx *sql.Tx, session platform.PortalSession, currentPersonID, ownerPersonID string, canManage bool) ([]workforce.Person, error) {
	query := `
		SELECT op.id,u.display_name,COALESCE(u.email,''),op.enabled
		  FROM organisation_people op
		  JOIN human_users u ON u.id=op.user_id
		 WHERE op.organisation_id=$1`
	args := []any{session.Current.OrganisationID}
	if !canManage {
		query += ` AND op.id=$2`
		args = append(args, currentPersonID)
	}
	query += ` ORDER BY CASE WHEN op.id=$2 THEN 0 ELSE 1 END,lower(u.display_name),op.id`
	if canManage {
		args = append(args, ownerPersonID)
	}
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	people := make([]workforce.Person, 0)
	for rows.Next() {
		var person workforce.Person
		if err := rows.Scan(&person.ID, &person.DisplayName, &person.Email, &person.Enabled); err != nil {
			return nil, err
		}
		person.Relationship = workforce.RelationshipEmployee
		if person.ID == ownerPersonID {
			person.Relationship = workforce.RelationshipOwner
		}
		people = append(people, person)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for index := range people {
		people[index].Groups, err = loadPersonGroups(ctx, tx, session.Current.OrganisationID, people[index].ID)
		if err != nil {
			return nil, err
		}
		if people[index].Relationship == workforce.RelationshipOwner {
			people[index].EffectiveModels, err = loadCompanyModels(ctx, tx, session.Current.OrganisationID)
		} else {
			people[index].EffectiveModels, err = loadPersonModels(ctx, tx, session.Current.OrganisationID, people[index].ID)
		}
		if err != nil {
			return nil, err
		}
	}
	return people, nil
}

func loadPersonGroups(ctx context.Context, tx *sql.Tx, organisationID, personID string) ([]workforce.GroupReference, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT g.id,g.name
		  FROM access_group_people gp
		  JOIN access_groups g ON g.id=gp.group_id AND g.organisation_id=gp.organisation_id
		 WHERE gp.organisation_id=$1 AND gp.person_id=$2 AND g.enabled
		 ORDER BY lower(g.name),g.id`, organisationID, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]workforce.GroupReference, 0)
	for rows.Next() {
		var item workforce.GroupReference
		if err := rows.Scan(&item.ID, &item.Name); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func loadCompanyModels(ctx context.Context, tx *sql.Tx, organisationID string) ([]workforce.ModelAccess, error) {
	return queryModels(ctx, tx, `
		SELECT r.id,m.alias,p.name,e.name
		  FROM tenant_routes r
		  JOIN models m ON m.id=r.model_id AND m.enabled
		  JOIN projects p ON p.id=r.project_id AND p.organisation_id=r.organisation_id
		  JOIN environments e ON e.id=r.environment_id AND e.project_id=r.project_id AND e.organisation_id=r.organisation_id
		 WHERE r.organisation_id=$1 AND r.enabled
		 ORDER BY lower(m.alias),p.name,e.name,r.id`, organisationID)
}

func loadPersonModels(ctx context.Context, tx *sql.Tx, organisationID, personID string) ([]workforce.ModelAccess, error) {
	return queryModels(ctx, tx, `
		SELECT DISTINCT r.id,m.alias,p.name,e.name
		  FROM access_group_people gp
		  JOIN access_groups g ON g.id=gp.group_id AND g.organisation_id=gp.organisation_id AND g.enabled
		  JOIN access_group_models gm ON gm.group_id=g.id AND gm.organisation_id=g.organisation_id
		  JOIN tenant_routes r ON r.id=gm.route_id AND r.organisation_id=gm.organisation_id AND r.enabled
		  JOIN models m ON m.id=r.model_id AND m.enabled
		  JOIN projects p ON p.id=r.project_id AND p.organisation_id=r.organisation_id
		  JOIN environments e ON e.id=r.environment_id AND e.project_id=r.project_id AND e.organisation_id=r.organisation_id
		 WHERE gp.organisation_id=$1 AND gp.person_id=$2
		 ORDER BY m.alias,p.name,e.name,r.id`, organisationID, personID)
}

func loadAvailableModels(ctx context.Context, tx *sql.Tx, organisationID, projectID, environmentID string) ([]workforce.ModelAccess, error) {
	return queryModels(ctx, tx, `
		SELECT r.id,m.alias,p.name,e.name
		  FROM tenant_routes r
		  JOIN models m ON m.id=r.model_id AND m.enabled
		  JOIN projects p ON p.id=r.project_id AND p.organisation_id=r.organisation_id
		  JOIN environments e ON e.id=r.environment_id AND e.project_id=r.project_id AND e.organisation_id=r.organisation_id
		 WHERE r.organisation_id=$1 AND r.project_id=$2 AND r.environment_id=$3 AND r.enabled
		 ORDER BY lower(m.alias),r.id`, organisationID, projectID, environmentID)
}

func queryModels(ctx context.Context, tx *sql.Tx, query string, args ...any) ([]workforce.ModelAccess, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]workforce.ModelAccess, 0)
	for rows.Next() {
		var item workforce.ModelAccess
		if err := rows.Scan(&item.RouteID, &item.Alias, &item.Project, &item.Environment); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func loadGroups(ctx context.Context, tx *sql.Tx, session platform.PortalSession, currentPersonID string, canManage bool) ([]workforce.Group, error) {
	query := `
		SELECT DISTINCT g.id,g.name,g.description,g.enabled,p.name,e.name
		  FROM access_groups g
		  JOIN projects p ON p.id=g.project_id AND p.organisation_id=g.organisation_id
		  JOIN environments e ON e.id=g.environment_id AND e.project_id=g.project_id AND e.organisation_id=g.organisation_id`
	args := []any{session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID}
	if !canManage {
		query += ` JOIN access_group_people gp ON gp.group_id=g.id AND gp.organisation_id=g.organisation_id AND gp.person_id=$4`
		args = append(args, currentPersonID)
	}
	query += ` WHERE g.organisation_id=$1 AND g.project_id=$2 AND g.environment_id=$3 ORDER BY g.enabled DESC,g.name,g.id`
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]workforce.Group, 0)
	for rows.Next() {
		var group workforce.Group
		if err := rows.Scan(&group.ID, &group.Name, &group.Description, &group.Enabled, &group.Project, &group.Environment); err != nil {
			return nil, err
		}
		result = append(result, group)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for index := range result {
		result[index].People, err = loadGroupPeople(ctx, tx, session.Current.OrganisationID, result[index].ID)
		if err != nil {
			return nil, err
		}
		result[index].Models, err = loadGroupModels(ctx, tx, session.Current.OrganisationID, result[index].ID)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func loadGroupPeople(ctx context.Context, tx *sql.Tx, organisationID, groupID string) ([]workforce.PersonReference, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT op.id,u.display_name,COALESCE(u.email,''),
		       CASE WHEN ownership.person_id IS NULL THEN 'employee' ELSE 'owner' END
		  FROM access_group_people gp
		  JOIN organisation_people op ON op.id=gp.person_id AND op.organisation_id=gp.organisation_id
		  JOIN human_users u ON u.id=op.user_id
		  LEFT JOIN organisation_ownerships ownership ON ownership.organisation_id=op.organisation_id AND ownership.person_id=op.id AND ownership.ended_at IS NULL
		 WHERE gp.organisation_id=$1 AND gp.group_id=$2
		 ORDER BY lower(u.display_name),op.id`, organisationID, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]workforce.PersonReference, 0)
	for rows.Next() {
		var item workforce.PersonReference
		if err := rows.Scan(&item.ID, &item.DisplayName, &item.Email, &item.Relationship); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func loadGroupModels(ctx context.Context, tx *sql.Tx, organisationID, groupID string) ([]workforce.ModelAccess, error) {
	return queryModels(ctx, tx, `
		SELECT r.id,m.alias,p.name,e.name
		  FROM access_group_models gm
		  JOIN tenant_routes r ON r.id=gm.route_id AND r.organisation_id=gm.organisation_id
		  JOIN models m ON m.id=r.model_id
		  JOIN projects p ON p.id=r.project_id AND p.organisation_id=r.organisation_id
		  JOIN environments e ON e.id=r.environment_id AND e.project_id=r.project_id AND e.organisation_id=r.organisation_id
		 WHERE gm.organisation_id=$1 AND gm.group_id=$2
		 ORDER BY lower(m.alias),r.id`, organisationID, groupID)
}

func (s *Store) LoadGroup(ctx context.Context, session platform.PortalSession, groupID string) (workforce.Group, error) {
	access, err := s.LoadAccess(ctx, session)
	if err != nil {
		return workforce.Group{}, err
	}
	for _, group := range access.Groups {
		if group.ID == groupID {
			return group, nil
		}
	}
	return workforce.Group{}, platform.ErrNotFound
}

func (s *Store) CreateGroup(ctx context.Context, session platform.PortalSession, input workforce.CreateGroupInput) (workforce.Group, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return workforce.Group{}, err
	}
	defer tx.Rollback()
	if _, err := requireCompanyOwner(ctx, tx, session); err != nil {
		return workforce.Group{}, err
	}
	if err := validateRoutes(ctx, tx, session, input.RouteIDs); err != nil {
		return workforce.Group{}, err
	}
	groupID, err := ids.New("grp")
	if err != nil {
		return workforce.Group{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO access_groups(id,organisation_id,project_id,environment_id,name,description,created_by) VALUES($1,$2,$3,$4,$5,$6,$7)`, groupID, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID, input.Name, input.Description, session.User.ID); err != nil {
		return workforce.Group{}, mapWriteError("create access group", err)
	}
	for _, routeID := range input.RouteIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO access_group_models(organisation_id,project_id,environment_id,group_id,route_id,created_by) VALUES($1,$2,$3,$4,$5,$6)`, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID, groupID, routeID, session.User.ID); err != nil {
			return workforce.Group{}, mapWriteError("assign access group model", err)
		}
	}
	if err := insertActorAudit(ctx, tx, "human_user", session.User.ID, session.Current.OrganisationID, session.Current.ProjectID, "access_group.created", "succeeded", map[string]string{"group_id": groupID}); err != nil {
		return workforce.Group{}, err
	}
	if err := tx.Commit(); err != nil {
		return workforce.Group{}, err
	}
	return s.LoadGroup(ctx, session, groupID)
}

func (s *Store) ReplaceGroupPeople(ctx context.Context, session platform.PortalSession, groupID string, personIDs []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := requireCompanyOwner(ctx, tx, session); err != nil {
		return err
	}
	if err := lockGroup(ctx, tx, session, groupID); err != nil {
		return err
	}
	if err := validateEmployeePeople(ctx, tx, session.Current.OrganisationID, personIDs); err != nil {
		return err
	}
	oldIDs, err := groupPersonIDs(ctx, tx, groupID)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM access_group_people WHERE group_id=$1`, groupID); err != nil {
		return err
	}
	for _, personID := range personIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO access_group_people(organisation_id,project_id,environment_id,group_id,person_id,created_by) VALUES($1,$2,$3,$4,$5,$6)`, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID, groupID, personID, session.User.ID); err != nil {
			return mapWriteError("assign access group person", err)
		}
	}
	if err := advancePersonPolicy(ctx, tx, append(oldIDs, personIDs...)); err != nil {
		return err
	}
	if err := insertActorAudit(ctx, tx, "human_user", session.User.ID, session.Current.OrganisationID, session.Current.ProjectID, "access_group.people_replaced", "succeeded", map[string]string{"group_id": groupID}); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ReplaceGroupModels(ctx context.Context, session platform.PortalSession, groupID string, routeIDs []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := requireCompanyOwner(ctx, tx, session); err != nil {
		return err
	}
	if err := lockGroup(ctx, tx, session, groupID); err != nil {
		return err
	}
	if err := validateRoutes(ctx, tx, session, routeIDs); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM access_group_models WHERE group_id=$1`, groupID); err != nil {
		return err
	}
	for _, routeID := range routeIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO access_group_models(organisation_id,project_id,environment_id,group_id,route_id,created_by) VALUES($1,$2,$3,$4,$5,$6)`, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID, groupID, routeID, session.User.ID); err != nil {
			return mapWriteError("assign access group model", err)
		}
	}
	personIDs, err := groupPersonIDs(ctx, tx, groupID)
	if err != nil {
		return err
	}
	if err := advancePersonPolicy(ctx, tx, personIDs); err != nil {
		return err
	}
	if err := insertActorAudit(ctx, tx, "human_user", session.User.ID, session.Current.OrganisationID, session.Current.ProjectID, "access_group.models_replaced", "succeeded", map[string]string{"group_id": groupID}); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) DisableGroup(ctx context.Context, session platform.PortalSession, groupID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := requireCompanyOwner(ctx, tx, session); err != nil {
		return err
	}
	if err := lockGroup(ctx, tx, session, groupID); err != nil {
		return err
	}
	personIDs, err := groupPersonIDs(ctx, tx, groupID)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE access_groups SET enabled=false,updated_at=now() WHERE id=$1 AND enabled`, groupID)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return platform.ErrConflict
	}
	if err := advancePersonPolicy(ctx, tx, personIDs); err != nil {
		return err
	}
	if err := insertActorAudit(ctx, tx, "human_user", session.User.ID, session.Current.OrganisationID, session.Current.ProjectID, "access_group.disabled", "succeeded", map[string]string{"group_id": groupID}); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) CreateInvitation(ctx context.Context, session platform.PortalSession, input workforce.CreateInvitationInput) (workforce.InvitationDelivery, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return workforce.InvitationDelivery{}, err
	}
	defer tx.Rollback()
	if _, err := requireCompanyOwner(ctx, tx, session); err != nil {
		return workforce.InvitationDelivery{}, err
	}
	groups, err := validateInvitationGroups(ctx, tx, session, input.GroupIDs)
	if err != nil {
		return workforce.InvitationDelivery{}, err
	}
	var existingPerson string
	err = tx.QueryRowContext(ctx, `
		SELECT person.id
		  FROM organisation_people person
		  JOIN human_users u ON u.id=person.user_id
		 WHERE person.organisation_id=$1 AND u.email_normalized=$2`, session.Current.OrganisationID, input.Email).Scan(&existingPerson)
	if err == nil {
		return workforce.InvitationDelivery{}, platform.ErrConflict
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return workforce.InvitationDelivery{}, err
	}
	now := s.now().UTC()
	if _, err := tx.ExecContext(ctx, `UPDATE human_invitations SET status='expired',updated_at=$2 WHERE organisation_id=$1 AND status='pending' AND expires_at<=$2`, session.Current.OrganisationID, now); err != nil {
		return workforce.InvitationDelivery{}, err
	}
	var pendingID string
	err = tx.QueryRowContext(ctx, `SELECT id FROM human_invitations WHERE organisation_id=$1 AND email_normalized=$2 AND status='pending' FOR UPDATE`, session.Current.OrganisationID, input.Email).Scan(&pendingID)
	if err == nil {
		return workforce.InvitationDelivery{}, platform.ErrConflict
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return workforce.InvitationDelivery{}, err
	}
	id, err := ids.New("inv")
	if err != nil {
		return workforce.InvitationDelivery{}, err
	}
	token, digest, err := invitationCredential()
	if err != nil {
		return workforce.InvitationDelivery{}, err
	}
	expiresAt := now.Add(72 * time.Hour)
	if _, err := tx.ExecContext(ctx, `INSERT INTO human_invitations(id,organisation_id,email_normalized,intended_display_name,token_digest,expires_at,created_by,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$8)`, id, session.Current.OrganisationID, input.Email, input.DisplayName, digest[:], expiresAt, session.User.ID, now); err != nil {
		return workforce.InvitationDelivery{}, mapWriteError("create employee invitation", err)
	}
	for _, group := range groups {
		if _, err := tx.ExecContext(ctx, `INSERT INTO human_invitation_groups(organisation_id,project_id,environment_id,invitation_id,group_id,created_at) VALUES($1,$2,$3,$4,$5,$6)`, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID, id, group.ID, now); err != nil {
			return workforce.InvitationDelivery{}, mapWriteError("snapshot invitation group", err)
		}
	}
	if err := insertActorAudit(ctx, tx, "human_user", session.User.ID, session.Current.OrganisationID, session.Current.ProjectID, "employee_invitation.created", "succeeded", map[string]string{"invitation_id": id}); err != nil {
		return workforce.InvitationDelivery{}, err
	}
	if err := tx.Commit(); err != nil {
		return workforce.InvitationDelivery{}, err
	}
	return workforce.InvitationDelivery{Invitation: workforce.Invitation{ID: id, Email: input.Email, DisplayName: input.DisplayName, Status: "pending", Groups: groups, CreatedAt: now, ExpiresAt: expiresAt, Delivery: "manual"}, Token: token}, nil
}

func (s *Store) ResendInvitation(ctx context.Context, session platform.PortalSession, invitationID string) (workforce.InvitationDelivery, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return workforce.InvitationDelivery{}, err
	}
	defer tx.Rollback()
	if _, err := requireCompanyOwner(ctx, tx, session); err != nil {
		return workforce.InvitationDelivery{}, err
	}
	now := s.now().UTC()
	invitation, err := lockInvitation(ctx, tx, session.Current.OrganisationID, invitationID, now)
	if err != nil {
		return workforce.InvitationDelivery{}, err
	}
	token, digest, err := invitationCredential()
	if err != nil {
		return workforce.InvitationDelivery{}, err
	}
	invitation.ExpiresAt = now.Add(72 * time.Hour)
	if _, err := tx.ExecContext(ctx, `UPDATE human_invitations SET token_digest=$1,token_generation=token_generation+1,expires_at=$2,delivery_status='manual',updated_at=$3 WHERE id=$4`, digest[:], invitation.ExpiresAt, now, invitation.ID); err != nil {
		return workforce.InvitationDelivery{}, err
	}
	if err := insertActorAudit(ctx, tx, "human_user", session.User.ID, session.Current.OrganisationID, session.Current.ProjectID, "employee_invitation.resent", "succeeded", map[string]string{"invitation_id": invitation.ID}); err != nil {
		return workforce.InvitationDelivery{}, err
	}
	if err := tx.Commit(); err != nil {
		return workforce.InvitationDelivery{}, err
	}
	return workforce.InvitationDelivery{Invitation: invitation, Token: token}, nil
}

func (s *Store) RevokeInvitation(ctx context.Context, session platform.PortalSession, invitationID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := requireCompanyOwner(ctx, tx, session); err != nil {
		return err
	}
	now := s.now().UTC()
	invitation, err := lockInvitation(ctx, tx, session.Current.OrganisationID, invitationID, now)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE human_invitations SET status='revoked',revoked_at=$1,updated_at=$1 WHERE id=$2`, now, invitation.ID); err != nil {
		return err
	}
	if err := insertActorAudit(ctx, tx, "human_user", session.User.ID, session.Current.OrganisationID, session.Current.ProjectID, "employee_invitation.revoked", "succeeded", map[string]string{"invitation_id": invitation.ID}); err != nil {
		return err
	}
	return tx.Commit()
}

func loadInvitations(ctx context.Context, tx *sql.Tx, organisationID string, now time.Time) ([]workforce.Invitation, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id,email_normalized,intended_display_name,status,created_at,expires_at,delivery_status FROM human_invitations WHERE organisation_id=$1 ORDER BY created_at DESC,id`, organisationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]workforce.Invitation, 0)
	for rows.Next() {
		var invitation workforce.Invitation
		if err := rows.Scan(&invitation.ID, &invitation.Email, &invitation.DisplayName, &invitation.Status, &invitation.CreatedAt, &invitation.ExpiresAt, &invitation.Delivery); err != nil {
			return nil, err
		}
		if invitation.Status == "pending" && !invitation.ExpiresAt.After(now) {
			invitation.Status = "expired"
		}
		result = append(result, invitation)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for index := range result {
		result[index].Groups, err = loadInvitationGroups(ctx, tx, organisationID, result[index].ID)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func loadInvitationGroups(ctx context.Context, tx *sql.Tx, organisationID, invitationID string) ([]workforce.GroupReference, error) {
	rows, err := tx.QueryContext(ctx, `SELECT g.id,g.name FROM human_invitation_groups ig JOIN access_groups g ON g.id=ig.group_id AND g.organisation_id=ig.organisation_id WHERE ig.organisation_id=$1 AND ig.invitation_id=$2 ORDER BY lower(g.name),g.id`, organisationID, invitationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]workforce.GroupReference, 0)
	for rows.Next() {
		var group workforce.GroupReference
		if err := rows.Scan(&group.ID, &group.Name); err != nil {
			return nil, err
		}
		result = append(result, group)
	}
	return result, rows.Err()
}

func validateInvitationGroups(ctx context.Context, tx *sql.Tx, session platform.PortalSession, groupIDs []string) ([]workforce.GroupReference, error) {
	query := `SELECT id,name FROM access_groups WHERE organisation_id=$1 AND project_id=$2 AND environment_id=$3 AND enabled AND id IN (` + placeholders(4, len(groupIDs)) + `) ORDER BY lower(name),id`
	args := []any{session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID}
	for _, id := range groupIDs {
		args = append(args, id)
	}
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	groups := make([]workforce.GroupReference, 0, len(groupIDs))
	for rows.Next() {
		var group workforce.GroupReference
		if err := rows.Scan(&group.ID, &group.Name); err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(groups) != len(groupIDs) {
		return nil, platform.ErrInvalid
	}
	return groups, nil
}

func lockInvitation(ctx context.Context, tx *sql.Tx, organisationID, invitationID string, now time.Time) (workforce.Invitation, error) {
	var invitation workforce.Invitation
	err := tx.QueryRowContext(ctx, `SELECT id,email_normalized,intended_display_name,status,created_at,expires_at,delivery_status FROM human_invitations WHERE id=$1 AND organisation_id=$2 FOR UPDATE`, invitationID, organisationID).Scan(&invitation.ID, &invitation.Email, &invitation.DisplayName, &invitation.Status, &invitation.CreatedAt, &invitation.ExpiresAt, &invitation.Delivery)
	if errors.Is(err, sql.ErrNoRows) {
		return workforce.Invitation{}, platform.ErrNotFound
	}
	if err != nil {
		return workforce.Invitation{}, err
	}
	if invitation.Status != "pending" || !invitation.ExpiresAt.After(now) {
		if invitation.Status == "pending" {
			_, _ = tx.ExecContext(ctx, `UPDATE human_invitations SET status='expired',updated_at=$1 WHERE id=$2`, now, invitation.ID)
		}
		return workforce.Invitation{}, platform.ErrConflict
	}
	invitation.Groups, err = loadInvitationGroups(ctx, tx, organisationID, invitation.ID)
	return invitation, err
}

func invitationCredential() (string, [32]byte, error) {
	var digest [32]byte
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", digest, err
	}
	token := base64.RawURLEncoding.EncodeToString(value)
	digest = sha256.Sum256([]byte(token))
	return token, digest, nil
}

func (s *Store) BeginInvitationSetup(ctx context.Context, invitationDigest [32]byte, now time.Time) (workforce.SetupSession, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return workforce.SetupSession{}, err
	}
	defer tx.Rollback()
	var invitationID string
	err = tx.QueryRowContext(ctx, `SELECT id FROM human_invitations WHERE token_digest=$1 AND status='pending' AND expires_at>$2 FOR SHARE`, invitationDigest[:], now).Scan(&invitationID)
	if errors.Is(err, sql.ErrNoRows) {
		return workforce.SetupSession{}, platform.ErrNotFound
	}
	if err != nil {
		return workforce.SetupSession{}, err
	}
	id, err := ids.New("act")
	if err != nil {
		return workforce.SetupSession{}, err
	}
	token, digest, err := invitationCredential()
	if err != nil {
		return workforce.SetupSession{}, err
	}
	expiresAt := now.Add(15 * time.Minute)
	if _, err := tx.ExecContext(ctx, `INSERT INTO human_action_sessions(id,action_type,invitation_id,token_digest,created_at,expires_at) VALUES($1,'accept_invitation',$2,$3,$4,$5)`, id, invitationID, digest[:], now, expiresAt); err != nil {
		return workforce.SetupSession{}, mapWriteError("create invitation setup session", err)
	}
	if err := tx.Commit(); err != nil {
		return workforce.SetupSession{}, err
	}
	return workforce.SetupSession{Token: token, ExpiresAt: expiresAt}, nil
}

func (s *Store) CreateOIDCTransaction(ctx context.Context, setupDigest, stateDigest [32]byte, nonce, verifier string, createdAt, expiresAt time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var actionSessionID string
	err = tx.QueryRowContext(ctx, `
		SELECT action.id
		  FROM human_action_sessions action
		  JOIN human_invitations invitation ON invitation.id=action.invitation_id
		 WHERE action.token_digest=$1 AND action.action_type='accept_invitation'
		   AND action.consumed_at IS NULL AND action.revoked_at IS NULL AND action.expires_at>$2
		   AND invitation.status='pending' AND invitation.expires_at>$2
		 FOR SHARE OF action,invitation`, setupDigest[:], createdAt).Scan(&actionSessionID)
	if errors.Is(err, sql.ErrNoRows) {
		return platform.ErrUnauthenticated
	}
	if err != nil {
		return err
	}
	id, err := ids.New("oid")
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO oidc_login_transactions(id,action_session_id,state_digest,nonce,code_verifier,created_at,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, actionSessionID, stateDigest[:], nonce, verifier, createdAt, expiresAt); err != nil {
		return mapWriteError("create OIDC login transaction", err)
	}
	return tx.Commit()
}

func (s *Store) ConsumeOIDCTransaction(ctx context.Context, stateDigest [32]byte, now time.Time) (workforce.OIDCTransaction, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return workforce.OIDCTransaction{}, err
	}
	defer tx.Rollback()
	var result workforce.OIDCTransaction
	var transactionID string
	err = tx.QueryRowContext(ctx, `
		SELECT transaction.id,action.id,transaction.nonce,transaction.code_verifier
		  FROM oidc_login_transactions transaction
		  JOIN human_action_sessions action ON action.id=transaction.action_session_id
		  JOIN human_invitations invitation ON invitation.id=action.invitation_id
		 WHERE transaction.state_digest=$1
		   AND transaction.consumed_at IS NULL AND transaction.expires_at>$2
		   AND action.consumed_at IS NULL AND action.revoked_at IS NULL AND action.expires_at>$2
		   AND invitation.status='pending' AND invitation.expires_at>$2
		 FOR UPDATE OF transaction`, stateDigest[:], now).Scan(&transactionID, &result.ActionSessionID, &result.Nonce, &result.Verifier)
	if errors.Is(err, sql.ErrNoRows) {
		return workforce.OIDCTransaction{}, platform.ErrUnauthenticated
	}
	if err != nil {
		return workforce.OIDCTransaction{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE oidc_login_transactions SET consumed_at=$1 WHERE id=$2`, now, transactionID); err != nil {
		return workforce.OIDCTransaction{}, err
	}
	if err := tx.Commit(); err != nil {
		return workforce.OIDCTransaction{}, err
	}
	return result, nil
}

func (s *Store) AcceptInvitation(ctx context.Context, actionSessionID string, identity workforce.FederatedIdentity, sessionDigest [32]byte, sessionExpiresAt, now time.Time) (platform.PortalSession, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return platform.PortalSession{}, err
	}
	defer tx.Rollback()
	var actionID, invitationID, organisationID, invitedEmail, intendedName string
	err = tx.QueryRowContext(ctx, `
		SELECT action.id,invitation.id,invitation.organisation_id,invitation.email_normalized,invitation.intended_display_name
		  FROM human_action_sessions action
		  JOIN human_invitations invitation ON invitation.id=action.invitation_id
		 WHERE action.id=$1 AND action.action_type='accept_invitation'
		   AND action.consumed_at IS NULL AND action.revoked_at IS NULL AND action.expires_at>$2
		   AND invitation.status='pending' AND invitation.expires_at>$2
		 FOR UPDATE OF action,invitation`, actionSessionID, now).Scan(&actionID, &invitationID, &organisationID, &invitedEmail, &intendedName)
	if errors.Is(err, sql.ErrNoRows) {
		return platform.PortalSession{}, platform.ErrUnauthenticated
	}
	if err != nil {
		return platform.PortalSession{}, err
	}
	if identity.Email != invitedEmail {
		return platform.PortalSession{}, platform.ErrForbidden
	}
	var userID string
	err = tx.QueryRowContext(ctx, `SELECT user_id FROM human_federated_identities WHERE issuer=$1 AND subject=$2 AND enabled FOR UPDATE`, identity.Issuer, identity.Subject).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx, `SELECT id FROM human_users WHERE email_normalized=$1 AND enabled FOR UPDATE`, identity.Email).Scan(&userID)
		if errors.Is(err, sql.ErrNoRows) {
			userID, err = ids.New("usr")
			if err != nil {
				return platform.PortalSession{}, err
			}
			username := "employee." + strings.TrimPrefix(userID, "usr_")[:16]
			displayName := identity.DisplayName
			if displayName == "" {
				displayName = intendedName
			}
			if displayName == "" {
				displayName = identity.Email
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO human_users(id,username,display_name,password_hash,email,email_normalized,email_verified_at,identity_origin,created_at,updated_at) VALUES($1,$2,$3,NULL,$4,$4,$5,'invitation',$5,$5)`, userID, username, displayName, identity.Email, now); err != nil {
				return platform.PortalSession{}, mapWriteError("create invited human user", err)
			}
		} else if err != nil {
			return platform.PortalSession{}, err
		}
		identityID, err := ids.New("fid")
		if err != nil {
			return platform.PortalSession{}, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO human_federated_identities(id,user_id,issuer,subject,email_snapshot,email_verified_at,linked_at,last_authenticated_at,updated_at,link_source) VALUES($1,$2,$3,$4,$5,$6,$6,$6,$6,'invitation')`, identityID, userID, identity.Issuer, identity.Subject, identity.Email, now); err != nil {
			return platform.PortalSession{}, mapWriteError("link invited federated identity", err)
		}
	} else if err != nil {
		return platform.PortalSession{}, err
	} else {
		var linkedEmail string
		if err := tx.QueryRowContext(ctx, `SELECT email_normalized FROM human_users WHERE id=$1 AND enabled FOR UPDATE`, userID).Scan(&linkedEmail); err != nil {
			return platform.PortalSession{}, err
		}
		if linkedEmail != identity.Email {
			return platform.PortalSession{}, platform.ErrForbidden
		}
		if _, err := tx.ExecContext(ctx, `UPDATE human_federated_identities SET last_authenticated_at=$1,updated_at=$1 WHERE issuer=$2 AND subject=$3`, now, identity.Issuer, identity.Subject); err != nil {
			return platform.PortalSession{}, err
		}
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT snapshot.project_id,snapshot.environment_id,snapshot.group_id
		  FROM human_invitation_groups snapshot
		  JOIN access_groups g ON g.id=snapshot.group_id AND g.organisation_id=snapshot.organisation_id AND g.project_id=snapshot.project_id AND g.environment_id=snapshot.environment_id AND g.enabled
		 WHERE snapshot.organisation_id=$1 AND snapshot.invitation_id=$2
		 ORDER BY snapshot.project_id,snapshot.environment_id,snapshot.group_id`, organisationID, invitationID)
	if err != nil {
		return platform.PortalSession{}, err
	}
	type groupScope struct{ projectID, environmentID, groupID string }
	groups := make([]groupScope, 0)
	for rows.Next() {
		var group groupScope
		if err := rows.Scan(&group.projectID, &group.environmentID, &group.groupID); err != nil {
			rows.Close()
			return platform.PortalSession{}, err
		}
		groups = append(groups, group)
	}
	if err := rows.Close(); err != nil {
		return platform.PortalSession{}, err
	}
	if len(groups) == 0 {
		return platform.PortalSession{}, platform.ErrConflict
	}
	projectID, environmentID := groups[0].projectID, groups[0].environmentID
	for _, group := range groups {
		if group.projectID != projectID || group.environmentID != environmentID {
			return platform.PortalSession{}, platform.ErrConflict
		}
	}
	membershipID, err := ids.New("mem")
	if err != nil {
		return platform.PortalSession{}, err
	}
	if err := tx.QueryRowContext(ctx, `INSERT INTO human_memberships(id,user_id,organisation_id,project_id,environment_id,role,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$7) ON CONFLICT(user_id,organisation_id,project_id,environment_id) DO UPDATE SET enabled=true,updated_at=EXCLUDED.updated_at RETURNING id`, membershipID, userID, organisationID, projectID, environmentID, platform.PortalRoleViewer, now).Scan(&membershipID); err != nil {
		return platform.PortalSession{}, mapWriteError("create invited portal membership", err)
	}
	personID, err := ids.New("per")
	if err != nil {
		return platform.PortalSession{}, err
	}
	if err := tx.QueryRowContext(ctx, `INSERT INTO organisation_people(id,organisation_id,user_id,created_at,updated_at) VALUES($1,$2,$3,$4,$4) ON CONFLICT(organisation_id,user_id) DO UPDATE SET enabled=true,updated_at=EXCLUDED.updated_at RETURNING id`, personID, organisationID, userID, now).Scan(&personID); err != nil {
		return platform.PortalSession{}, mapWriteError("create invited company person", err)
	}
	for _, group := range groups {
		if _, err := tx.ExecContext(ctx, `INSERT INTO access_group_people(organisation_id,project_id,environment_id,group_id,person_id,created_by,created_at) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(group_id,person_id) DO NOTHING`, organisationID, group.projectID, group.environmentID, group.groupID, personID, userID, now); err != nil {
			return platform.PortalSession{}, mapWriteError("assign invited employee group", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE human_invitations SET status='accepted',accepted_user_id=$1,accepted_at=$2,updated_at=$2 WHERE id=$3`, userID, now, invitationID); err != nil {
		return platform.PortalSession{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE human_action_sessions SET consumed_at=CASE WHEN id=$1 THEN $2 ELSE consumed_at END,revoked_at=CASE WHEN id<>$1 AND consumed_at IS NULL THEN $2 ELSE revoked_at END WHERE invitation_id=$3`, actionID, now, invitationID); err != nil {
		return platform.PortalSession{}, err
	}
	sessionID, err := ids.New("ses")
	if err != nil {
		return platform.PortalSession{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO portal_sessions(id,user_id,current_membership_id,token_hash,created_at,last_seen_at,authenticated_at,expires_at) VALUES($1,$2,$3,$4,$5,$5,$5,$6)`, sessionID, userID, membershipID, sessionDigest[:], now, sessionExpiresAt); err != nil {
		return platform.PortalSession{}, mapWriteError("create invited portal session", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE human_users SET last_sign_in_at=$2,updated_at=$2 WHERE id=$1`, userID, now); err != nil {
		return platform.PortalSession{}, err
	}
	if err := insertActorAudit(ctx, tx, "human_user", userID, organisationID, projectID, "employee_invitation.accepted", "succeeded", map[string]string{"invitation_id": invitationID, "person_id": personID}); err != nil {
		return platform.PortalSession{}, err
	}
	memberships, err := listMemberships(ctx, tx, userID)
	if err != nil {
		return platform.PortalSession{}, err
	}
	var user platform.PortalUser
	if err := tx.QueryRowContext(ctx, `SELECT id,username,display_name FROM human_users WHERE id=$1`, userID).Scan(&user.ID, &user.Username, &user.DisplayName); err != nil {
		return platform.PortalSession{}, err
	}
	if err := tx.Commit(); err != nil {
		return platform.PortalSession{}, err
	}
	current := memberships[0]
	for _, membership := range memberships {
		if membership.ID == membershipID {
			current = membership
			break
		}
	}
	return platform.PortalSession{ID: sessionID, User: user, Current: current, Memberships: memberships, AuthenticatedAt: now, ExpiresAt: sessionExpiresAt}, nil
}

func requireCompanyOwner(ctx context.Context, tx *sql.Tx, session platform.PortalSession) (string, error) {
	var personID string
	err := tx.QueryRowContext(ctx, `
		SELECT person.id
		  FROM organisation_people person
		  JOIN organisation_ownerships ownership ON ownership.organisation_id=person.organisation_id AND ownership.person_id=person.id AND ownership.ended_at IS NULL
		 WHERE person.organisation_id=$1 AND person.user_id=$2 AND person.enabled
		 FOR UPDATE OF person`, session.Current.OrganisationID, session.User.ID).Scan(&personID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", platform.ErrForbidden
	}
	return personID, err
}

func lockGroup(ctx context.Context, tx *sql.Tx, session platform.PortalSession, groupID string) error {
	var id string
	err := tx.QueryRowContext(ctx, `SELECT id FROM access_groups WHERE id=$1 AND organisation_id=$2 AND project_id=$3 AND environment_id=$4 AND enabled FOR UPDATE`, groupID, session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return platform.ErrNotFound
	}
	return err
}

func validateRoutes(ctx context.Context, tx *sql.Tx, session platform.PortalSession, routeIDs []string) error {
	if len(routeIDs) == 0 {
		return nil
	}
	query := `SELECT count(*) FROM tenant_routes WHERE organisation_id=$1 AND project_id=$2 AND environment_id=$3 AND enabled AND id IN (` + placeholders(4, len(routeIDs)) + `)`
	args := []any{session.Current.OrganisationID, session.Current.ProjectID, session.Current.EnvironmentID}
	for _, routeID := range routeIDs {
		args = append(args, routeID)
	}
	var count int
	if err := tx.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return err
	}
	if count != len(routeIDs) {
		return platform.ErrInvalid
	}
	return nil
}

func validateEmployeePeople(ctx context.Context, tx *sql.Tx, organisationID string, personIDs []string) error {
	if len(personIDs) == 0 {
		return nil
	}
	query := `
		SELECT count(*)
		  FROM organisation_people person
		 WHERE person.organisation_id=$1 AND person.enabled AND person.id IN (` + placeholders(2, len(personIDs)) + `)
		   AND NOT EXISTS (
		       SELECT 1 FROM organisation_ownerships ownership
		        WHERE ownership.organisation_id=person.organisation_id
		          AND ownership.person_id=person.id AND ownership.ended_at IS NULL
		   )`
	args := []any{organisationID}
	for _, personID := range personIDs {
		args = append(args, personID)
	}
	var count int
	if err := tx.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return err
	}
	if count != len(personIDs) {
		return platform.ErrInvalid
	}
	return nil
}

func groupPersonIDs(ctx context.Context, tx *sql.Tx, groupID string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT person_id FROM access_group_people WHERE group_id=$1`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result = append(result, id)
	}
	return result, rows.Err()
}

func advancePersonPolicy(ctx context.Context, tx *sql.Tx, personIDs []string) error {
	if len(personIDs) == 0 {
		return nil
	}
	unique := make(map[string]struct{}, len(personIDs))
	values := make([]string, 0, len(personIDs))
	for _, id := range personIDs {
		if _, ok := unique[id]; ok {
			continue
		}
		unique[id] = struct{}{}
		values = append(values, id)
	}
	query := `UPDATE organisation_people SET authorisation_generation=authorisation_generation+1,updated_at=now() WHERE id IN (` + placeholders(1, len(values)) + `)`
	args := make([]any, 0, len(values))
	for _, value := range values {
		args = append(args, value)
	}
	_, err := tx.ExecContext(ctx, query, args...)
	return err
}

func placeholders(start, count int) string {
	values := make([]string, count)
	for index := range values {
		values[index] = fmt.Sprintf("$%d", start+index)
	}
	return strings.Join(values, ",")
}

func (s *Store) AssignInitialOwner(ctx context.Context, spec workforce.InitialOwnerSpec) (workforce.InitialOwnerResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return workforce.InitialOwnerResult{}, err
	}
	defer tx.Rollback()
	result := workforce.InitialOwnerResult{Username: spec.Username}
	if err := tx.QueryRowContext(ctx, `
		SELECT organisation.id,user_account.id
		  FROM organisations organisation
		  JOIN human_users user_account ON user_account.username=$2 AND user_account.enabled
		 WHERE organisation.slug=$1
		   AND EXISTS (
		       SELECT 1 FROM human_memberships membership
		        WHERE membership.organisation_id=organisation.id
		          AND membership.user_id=user_account.id AND membership.enabled
		   )
		 FOR UPDATE OF organisation,user_account`, spec.OrganisationSlug, spec.Username).Scan(&result.OrganisationID, &result.UserID); errors.Is(err, sql.ErrNoRows) {
		return workforce.InitialOwnerResult{}, platform.ErrNotFound
	} else if err != nil {
		return workforce.InitialOwnerResult{}, err
	}
	var currentPersonID, currentOwnershipID string
	err = tx.QueryRowContext(ctx, `SELECT person_id,id FROM organisation_ownerships WHERE organisation_id=$1 AND ended_at IS NULL FOR UPDATE`, result.OrganisationID).Scan(&currentPersonID, &currentOwnershipID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return workforce.InitialOwnerResult{}, err
	}
	personID, idErr := ids.New("per")
	if idErr != nil {
		return workforce.InitialOwnerResult{}, idErr
	}
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO organisation_people(id,organisation_id,user_id)
		VALUES($1,$2,$3)
		ON CONFLICT(organisation_id,user_id)
		DO UPDATE SET enabled=true,updated_at=now()
		RETURNING id`, personID, result.OrganisationID, result.UserID).Scan(&result.PersonID); err != nil {
		return workforce.InitialOwnerResult{}, mapWriteError("reconcile company person", err)
	}
	if currentOwnershipID != "" {
		if currentPersonID != result.PersonID {
			return workforce.InitialOwnerResult{}, platform.ErrConflict
		}
		result.OwnershipID = currentOwnershipID
		if err := tx.Commit(); err != nil {
			return workforce.InitialOwnerResult{}, err
		}
		return result, nil
	}
	result.OwnershipID, err = ids.New("own")
	if err != nil {
		return workforce.InitialOwnerResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO organisation_ownerships(id,organisation_id,person_id,change_kind,actor_type,actor_id,evidence_ref) VALUES($1,$2,$3,'initial','operator','cli',$4)`, result.OwnershipID, result.OrganisationID, result.PersonID, spec.EvidenceRef); err != nil {
		return workforce.InitialOwnerResult{}, mapWriteError("assign initial company owner", err)
	}
	if err := insertActorAudit(ctx, tx, "operator", "cli", result.OrganisationID, "", "organisation.owner_assigned", "succeeded", map[string]string{"person_id": result.PersonID, "user_id": result.UserID, "evidence_ref": spec.EvidenceRef}); err != nil {
		return workforce.InitialOwnerResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return workforce.InitialOwnerResult{}, err
	}
	result.Created = true
	return result, nil
}

var _ workforce.Store = (*Store)(nil)
