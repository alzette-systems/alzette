package workforce

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"alzette/internal/platform"
)

type recordingStore struct {
	created CreateGroupInput
	invited CreateInvitationInput
	people  []string
	models  []string
	owner   InitialOwnerSpec
}

func (s *recordingStore) LoadAccess(context.Context, platform.PortalSession) (Access, error) {
	return Access{}, nil
}
func (s *recordingStore) LoadGroup(context.Context, platform.PortalSession, string) (Group, error) {
	return Group{}, nil
}
func (s *recordingStore) CreateGroup(_ context.Context, _ platform.PortalSession, input CreateGroupInput) (Group, error) {
	s.created = input
	return Group{Name: input.Name}, nil
}
func (s *recordingStore) ReplaceGroupPeople(_ context.Context, _ platform.PortalSession, _ string, ids []string) error {
	s.people = ids
	return nil
}
func (s *recordingStore) ReplaceGroupModels(_ context.Context, _ platform.PortalSession, _ string, ids []string) error {
	s.models = ids
	return nil
}
func (s *recordingStore) DisableGroup(context.Context, platform.PortalSession, string) error {
	return nil
}
func (s *recordingStore) CreateInvitation(_ context.Context, _ platform.PortalSession, input CreateInvitationInput) (InvitationDelivery, error) {
	s.invited = input
	return InvitationDelivery{}, nil
}
func (s *recordingStore) ResendInvitation(context.Context, platform.PortalSession, string) (InvitationDelivery, error) {
	return InvitationDelivery{}, nil
}
func (s *recordingStore) RevokeInvitation(context.Context, platform.PortalSession, string) error {
	return nil
}
func (s *recordingStore) AssignInitialOwner(_ context.Context, spec InitialOwnerSpec) (InitialOwnerResult, error) {
	s.owner = spec
	return InitialOwnerResult{}, nil
}

func TestServiceNormalisesGroupInputs(t *testing.T) {
	store := &recordingStore{}
	service := New(store)
	if _, err := service.CreateGroup(context.Background(), platform.PortalSession{}, CreateGroupInput{
		Name: "  Client operations  ", Description: "  Production users  ", RouteIDs: []string{"route_b", "route_a", "route_b"},
	}); err != nil {
		t.Fatal(err)
	}
	if store.created.Name != "Client operations" || store.created.Description != "Production users" || !reflect.DeepEqual(store.created.RouteIDs, []string{"route_a", "route_b"}) {
		t.Fatalf("normalised input=%#v", store.created)
	}
	if err := service.ReplaceGroupPeople(context.Background(), platform.PortalSession{}, "group_1", []string{"person_b", "person_a", "person_b"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(store.people, []string{"person_a", "person_b"}) {
		t.Fatalf("normalised people=%v", store.people)
	}
}

func TestServiceNormalisesInvitationInputs(t *testing.T) {
	store := &recordingStore{}
	service := New(store)
	if _, err := service.CreateInvitation(context.Background(), platform.PortalSession{}, CreateInvitationInput{Email: " Employee@Example.TEST ", DisplayName: " Erin Employee ", GroupIDs: []string{"group_b", "group_a", "group_b"}}); err != nil {
		t.Fatal(err)
	}
	want := CreateInvitationInput{Email: "employee@example.test", DisplayName: "Erin Employee", GroupIDs: []string{"group_a", "group_b"}}
	if !reflect.DeepEqual(store.invited, want) {
		t.Fatalf("invitation=%#v want=%#v", store.invited, want)
	}
	for _, input := range []CreateInvitationInput{{Email: "not-an-email", GroupIDs: []string{"group_a"}}, {Email: "employee@example.test"}, {Email: "employee@example.test", GroupIDs: []string{"bad/group"}}} {
		if _, err := service.CreateInvitation(context.Background(), platform.PortalSession{}, input); !errors.Is(err, platform.ErrInvalid) {
			t.Fatalf("invalid invitation %#v error=%v", input, err)
		}
	}
}

func TestServiceRejectsInvalidGroupInputsBeforeStore(t *testing.T) {
	store := &recordingStore{}
	service := New(store)
	tests := []struct {
		name string
		run  func() error
	}{
		{"blank name", func() error {
			_, err := service.CreateGroup(context.Background(), platform.PortalSession{}, CreateGroupInput{Name: " "})
			return err
		}},
		{"markup name", func() error {
			_, err := service.CreateGroup(context.Background(), platform.PortalSession{}, CreateGroupInput{Name: "<script>"})
			return err
		}},
		{"bad route id", func() error {
			_, err := service.CreateGroup(context.Background(), platform.PortalSession{}, CreateGroupInput{Name: "Team", RouteIDs: []string{"not an id"}})
			return err
		}},
		{"bad group id", func() error {
			return service.ReplaceGroupModels(context.Background(), platform.PortalSession{}, "../group", nil)
		}},
		{"bad person id", func() error {
			return service.ReplaceGroupPeople(context.Background(), platform.PortalSession{}, "group_1", []string{"person/1"})
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.run(); !errors.Is(err, platform.ErrInvalid) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestServiceNormalisesAndValidatesInitialOwner(t *testing.T) {
	store := &recordingStore{}
	service := New(store)
	if _, err := service.AssignInitialOwner(context.Background(), InitialOwnerSpec{OrganisationSlug: "  ACME  ", Username: " Owner.User ", EvidenceRef: " ticket/123 "}); err != nil {
		t.Fatal(err)
	}
	want := InitialOwnerSpec{OrganisationSlug: "acme", Username: "owner.user", EvidenceRef: "ticket/123"}
	if store.owner != want {
		t.Fatalf("owner spec=%#v want=%#v", store.owner, want)
	}
	if _, err := service.AssignInitialOwner(context.Background(), InitialOwnerSpec{OrganisationSlug: "bad slug", Username: "owner", EvidenceRef: "ticket"}); !errors.Is(err, platform.ErrInvalid) {
		t.Fatalf("invalid slug error=%v", err)
	}
}
