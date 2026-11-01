package store

import (
	"fmt"

	"github.com/puppe1990/epub-translator/internal/tokens"
)

// backfillSourceTokens preenche capítulos antigos (DEFAULT 0) uma vez na
// abertura do banco. MaxOpenConns(1) exige fechar o SELECT antes do UPDATE.
func (s *SQLiteStore) backfillSourceTokens() error {
	rows, err := s.db.Query(`SELECT id, source_text FROM chapters WHERE source_tokens = 0 AND source_text <> ''`)
	if err != nil {
		return fmt.Errorf("listar capítulos sem tokens: %w", err)
	}

	type pending struct {
		id   int64
		text string
	}
	var items []pending
	for rows.Next() {
		var item pending
		if err := rows.Scan(&item.id, &item.text); err != nil {
			_ = rows.Close()
			return fmt.Errorf("ler capítulo sem tokens: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("fechar capítulos sem tokens: %w", err)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("varrer capítulos sem tokens: %w", err)
	}

	for _, item := range items {
		if _, err := s.db.Exec(`UPDATE chapters SET source_tokens = ? WHERE id = ?`, tokens.Count(item.text), item.id); err != nil {
			return fmt.Errorf("gravar tokens do capítulo %d: %w", item.id, err)
		}
	}
	return nil
}
