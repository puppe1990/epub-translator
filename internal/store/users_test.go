package store

import (
	"errors"
	"testing"

	"github.com/puppe1990/amarra-cais/pkg/cais/session"
)

func newUsersTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := NewSQLiteStore(":memory:", "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestUsers_createFindAndChangePassword(t *testing.T) {
	s := newUsersTestStore(t)

	hash, err := session.HashPassword("senha-secreta-123")
	if err != nil {
		t.Fatal(err)
	}
	userID, err := s.CreateUser("Matheus@Example.com ", hash)
	if err != nil {
		t.Fatal(err)
	}

	user, err := s.FindUserByEmail("matheus@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if user.ID != userID || user.Email != "matheus@example.com" {
		t.Errorf("user = %+v", user)
	}
	if !session.VerifyPassword(user.PasswordHash, "senha-secreta-123") {
		t.Error("senha gravada não confere")
	}

	if _, err := s.CreateUser("matheus@example.com", hash); !errors.Is(err, ErrEmailTaken) {
		t.Errorf("duplicado: err = %v, want ErrEmailTaken", err)
	}

	byID, err := s.FindUser(userID)
	if err != nil || byID.Email != user.Email {
		t.Errorf("FindUser = %+v, err = %v", byID, err)
	}

	newHash, err := session.HashPassword("outra-senha-456")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpdatePasswordHash(userID, newHash); err != nil {
		t.Fatal(err)
	}
	updated, err := s.FindUser(userID)
	if err != nil {
		t.Fatal(err)
	}
	if !session.VerifyPassword(updated.PasswordHash, "outra-senha-456") {
		t.Error("senha não foi atualizada")
	}
	if session.VerifyPassword(updated.PasswordHash, "senha-secreta-123") {
		t.Error("senha antiga ainda vale")
	}
}

func TestSeedAdminFromEnv_createsAdminOnce(t *testing.T) {
	t.Setenv("ADMIN_EMAIL", "admin@example.com")
	t.Setenv("ADMIN_PASSWORD", "senha-admin-123")
	s := newUsersTestStore(t)

	var isAdmin int
	if err := s.DB().QueryRow(
		"SELECT is_admin FROM users WHERE email = ?", "admin@example.com",
	).Scan(&isAdmin); err != nil {
		t.Fatal(err)
	}
	if isAdmin != 1 {
		t.Errorf("is_admin = %d, want 1", isAdmin)
	}
	user, err := s.FindUserByEmail("admin@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if !session.VerifyPassword(user.PasswordHash, "senha-admin-123") {
		t.Error("senha do admin não confere")
	}

	// Idempotente: mudar a env e rodar de novo NÃO troca a senha sozinho.
	t.Setenv("ADMIN_PASSWORD", "outra-senha-999")
	if err := seedAdminFromEnv(s.DB()); err != nil {
		t.Fatal(err)
	}
	again, err := s.FindUserByEmail("admin@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if !session.VerifyPassword(again.PasswordHash, "senha-admin-123") {
		t.Error("seed sobrescreveu a senha existente")
	}
	if session.VerifyPassword(again.PasswordHash, "outra-senha-999") {
		t.Error("seed aplicou a nova env sobre usuário existente")
	}
}

func TestSeedAdminFromEnv_skipsWithoutEnv(t *testing.T) {
	t.Setenv("ADMIN_EMAIL", "")
	t.Setenv("ADMIN_PASSWORD", "")
	s := newUsersTestStore(t)

	var count int
	if err := s.DB().QueryRow("SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("usuários criados sem env: %d", count)
	}
}
