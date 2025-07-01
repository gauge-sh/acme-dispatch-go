package store

import (
	"errors"
	"testing"

	"github.com/acme-dispatch/dispatch/core"
)

func seedUser(t *testing.T, s *Store, email string, role core.Role) *core.User {
	t.Helper()
	u := core.User{Email: email, Name: "Test User", Role: role, IsActive: true}
	if err := s.CreateUser(&u, "a-long-password"); err != nil {
		t.Fatal(err)
	}
	return &u
}

func TestUserAuthentication(t *testing.T) {
	s := testStore(t)
	u := seedUser(t, s, "pat@example.com", core.RoleDispatcher)

	got, err := s.Authenticate("PAT@example.com", "a-long-password")
	if err != nil || got == nil || got.ID != u.ID {
		t.Fatalf("login failed: %v %v", got, err)
	}
	if got, _ := s.Authenticate("pat@example.com", "wrong"); got != nil {
		t.Fatal("wrong password accepted")
	}
	if got, _ := s.Authenticate("nobody@example.com", "a-long-password"); got != nil {
		t.Fatal("unknown email accepted")
	}

	u.IsActive = false
	admin := seedUser(t, s, "admin@example.com", core.RoleAdmin)
	if err := s.UpdateUser(admin.ID, u); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Authenticate("pat@example.com", "a-long-password"); got != nil {
		t.Fatal("inactive user logged in")
	}
}

func TestDuplicateEmail(t *testing.T) {
	s := testStore(t)
	seedUser(t, s, "pat@example.com", core.RoleDispatcher)
	dupe := core.User{Email: "Pat@Example.com", Name: "Other", Role: core.RoleDispatcher, IsActive: true}
	err := s.CreateUser(&dupe, "password123")
	var derr *core.Error
	if !errors.As(err, &derr) || derr.Fields["email"] == "" {
		t.Fatalf("case-insensitive uniqueness: %v", err)
	}
}

func TestSelfServiceRules(t *testing.T) {
	s := testStore(t)
	admin := seedUser(t, s, "admin@example.com", core.RoleAdmin)
	other := seedUser(t, s, "second@example.com", core.RoleAdmin)

	me := *admin
	me.Role = core.RoleDispatcher
	err := s.UpdateUser(admin.ID, &me)
	var derr *core.Error
	if !errors.As(err, &derr) || derr.Message != "you cannot change your own role" {
		t.Fatalf("own role: %v", err)
	}

	me = *admin
	me.IsActive = false
	err = s.UpdateUser(admin.ID, &me)
	if !errors.As(err, &derr) || derr.Message != "you cannot deactivate yourself" {
		t.Fatalf("self deactivate: %v", err)
	}

	// Another admin can demote them while a second active admin remains.
	me = *admin
	me.Role = core.RoleDispatcher
	if err := s.UpdateUser(other.ID, &me); err != nil {
		t.Fatalf("peer demotion: %v", err)
	}
}

func TestLastActiveAdminIsProtected(t *testing.T) {
	s := testStore(t)
	admin := seedUser(t, s, "admin@example.com", core.RoleAdmin)
	dispatcher := seedUser(t, s, "pat@example.com", core.RoleDispatcher)

	err := s.DeleteUser(admin.ID)
	var derr *core.Error
	if !errors.As(err, &derr) || derr.Message != "cannot remove the last active admin" {
		t.Fatalf("delete last admin: %v", err)
	}

	demoted := *admin
	demoted.Role = core.RoleDispatcher
	err = s.UpdateUser(dispatcher.ID, &demoted)
	if !errors.As(err, &derr) || derr.Message != "cannot remove the last active admin" {
		t.Fatalf("demote last admin: %v", err)
	}

	// With a second active admin the first can go.
	seedUser(t, s, "second@example.com", core.RoleAdmin)
	if err := s.DeleteUser(admin.ID); err != nil {
		t.Fatalf("delete with backup admin: %v", err)
	}
}

func TestTokenLifecycle(t *testing.T) {
	s := testStore(t)
	u := seedUser(t, s, "pat@example.com", core.RoleDispatcher)

	plaintext, token, err := s.IssueToken(u.ID, "ci token")
	if err != nil {
		t.Fatal(err)
	}
	if len(plaintext) != 64 {
		t.Fatalf("token length: %d", len(plaintext))
	}
	if token.TokenHash == plaintext {
		t.Fatal("plaintext stored")
	}

	got, err := s.UserForToken(plaintext)
	if err != nil || got == nil || got.ID != u.ID {
		t.Fatalf("token lookup: %v %v", got, err)
	}
	tokens, _ := s.TokensForUser(u.ID)
	if len(tokens) != 1 || tokens[0].LastUsedAt.IsZero() {
		t.Fatalf("last_used_at not written: %+v", tokens)
	}

	if got, _ := s.UserForToken("not-a-token"); got != nil {
		t.Fatal("bogus token resolved")
	}

	// Deactivating the owner kills the token.
	admin := seedUser(t, s, "admin@example.com", core.RoleAdmin)
	u.IsActive = false
	if err := s.UpdateUser(admin.ID, u); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.UserForToken(plaintext); got != nil {
		t.Fatal("token for inactive user resolved")
	}

	if err := s.DeleteToken(token.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteToken(token.ID); err == nil {
		t.Fatal("double delete")
	}
}
