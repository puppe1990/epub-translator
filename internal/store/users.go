package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/puppe1990/amarra-cais/pkg/cais/session"

	"github.com/puppe1990/epub-translator/internal/models"
)

var ErrEmailTaken = errors.New("email already registered")

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (s *SQLiteStore) FindUser(id int64) (models.User, error) {
	var user models.User
	err := s.db.QueryRow(
		"SELECT id, email, password_hash FROM users WHERE id = ?",
		id,
	).Scan(&user.ID, &user.Email, &user.PasswordHash)
	if err != nil {
		return models.User{}, fmt.Errorf("find user: %w", err)
	}
	return user, nil
}

func (s *SQLiteStore) FindUserByEmail(email string) (models.User, error) {
	var user models.User
	err := s.db.QueryRow(
		"SELECT id, email, password_hash FROM users WHERE email = ?",
		normalizeEmail(email),
	).Scan(&user.ID, &user.Email, &user.PasswordHash)
	if err != nil {
		return models.User{}, fmt.Errorf("find user: %w", err)
	}
	return user, nil
}

func (s *SQLiteStore) CreateUser(email, passwordHash string) (int64, error) {
	result, err := s.db.Exec(
		"INSERT INTO users (email, password_hash) VALUES (?, ?)",
		normalizeEmail(email), passwordHash,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return 0, ErrEmailTaken
		}
		return 0, fmt.Errorf("create user: %w", err)
	}
	return result.LastInsertId()
}

func (s *SQLiteStore) UpdatePasswordHash(userID int64, passwordHash string) error {
	if _, err := s.db.Exec("UPDATE users SET password_hash = ? WHERE id = ?", passwordHash, userID); err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	return nil
}

// seedAdminFromEnv cria o usuário admin no boot quando ADMIN_EMAIL e
// ADMIN_PASSWORD estão definidos e o email ainda não existe. Nunca sobrescreve
// um usuário existente — trocar a senha é papel da página de configurações.
func seedAdminFromEnv(db *sql.DB) error {
	email := normalizeEmail(os.Getenv("ADMIN_EMAIL"))
	password := os.Getenv("ADMIN_PASSWORD")
	if email == "" || password == "" {
		return nil
	}
	hash, err := session.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}
	if _, err := db.Exec(
		"INSERT OR IGNORE INTO users (email, password_hash, is_admin) VALUES (?, ?, 1)",
		email, hash,
	); err != nil {
		return fmt.Errorf("seed admin user: %w", err)
	}
	return nil
}
