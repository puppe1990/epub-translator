package store

import "fmt"

// Cancellation is not a failed attempt. Only this book's in-flight chapter
// returns to pending; completed translations and earlier failures stay intact.
func (s *SQLiteStore) PauseBook(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("iniciar pausa do livro %d: %w", id, err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`UPDATE chapters SET status = 'pending', error = '',
		updated_at = CURRENT_TIMESTAMP WHERE book_id = ? AND status = 'running'`, id); err != nil {
		return fmt.Errorf("devolver capítulo do livro %d à fila: %w", id, err)
	}
	if _, err := tx.Exec(`UPDATE books SET status = CASE
		WHEN EXISTS(SELECT 1 FROM chapters WHERE book_id = ? AND status != 'done')
		THEN 'paused' ELSE 'done' END WHERE id = ?`, id, id); err != nil {
		return fmt.Errorf("pausar livro %d: %w", id, err)
	}
	return tx.Commit()
}
