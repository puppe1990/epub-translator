package store

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/puppe1990/epub-translator/internal/models"
)

// SaveProvider faz upsert por slug; chave vazia preserva a existente (o
// formulário nunca mostra a chave inteira de volta).
func (s *SQLiteStore) SaveProvider(provider models.Provider) error {
	_, err := s.db.Exec(
		`INSERT INTO providers (slug, base_url, model, api_key) VALUES (?, ?, ?, ?)
		 ON CONFLICT(slug) DO UPDATE SET
		   base_url = excluded.base_url,
		   model = excluded.model,
		   api_key = CASE WHEN excluded.api_key = '' THEN providers.api_key ELSE excluded.api_key END,
		   updated_at = CURRENT_TIMESTAMP`,
		provider.Slug, provider.BaseURL, provider.Model, provider.APIKey)
	if err != nil {
		return fmt.Errorf("salvar provedor: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetProvider(slug string) (models.Provider, error) {
	var provider models.Provider
	err := s.db.QueryRow(
		`SELECT slug, base_url, model, api_key FROM providers WHERE slug = ?`, slug,
	).Scan(&provider.Slug, &provider.BaseURL, &provider.Model, &provider.APIKey)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Provider{}, sql.ErrNoRows
	}
	if err != nil {
		return models.Provider{}, fmt.Errorf("buscar provedor: %w", err)
	}
	return provider, nil
}

func (s *SQLiteStore) ListProviders() ([]models.Provider, error) {
	rows, err := s.db.Query(`SELECT slug, base_url, model, api_key FROM providers ORDER BY slug`)
	if err != nil {
		return nil, fmt.Errorf("listar provedores: %w", err)
	}
	defer func() { _ = rows.Close() }()

	providers := []models.Provider{}
	for rows.Next() {
		var provider models.Provider
		if err := rows.Scan(&provider.Slug, &provider.BaseURL, &provider.Model, &provider.APIKey); err != nil {
			return nil, fmt.Errorf("ler provedor: %w", err)
		}
		providers = append(providers, provider)
	}
	return providers, rows.Err()
}
