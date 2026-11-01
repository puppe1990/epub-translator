package store

import "fmt"

// Cancellation is not a failed attempt. In-flight and unstarted chapters
// park as paused — they are not an active queue until Retomar.
func (s *SQLiteStore) PauseBook(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("iniciar pausa do livro %d: %w", id, err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`UPDATE chapters SET status = 'paused', error = '',
		updated_at = CURRENT_TIMESTAMP WHERE book_id = ? AND status IN ('running', 'pending')`, id); err != nil {
		return fmt.Errorf("estacionar capítulos do livro %d: %w", id, err)
	}
	if _, err := tx.Exec(`UPDATE books SET status = CASE
		WHEN EXISTS(SELECT 1 FROM chapters WHERE book_id = ? AND status != 'done')
		THEN 'paused' ELSE 'done' END WHERE id = ?`, id, id); err != nil {
		return fmt.Errorf("pausar livro %d: %w", id, err)
	}
	return tx.Commit()
}

// ParkPausedBookQueue heals leftovers: pending chapters on a paused book
// (deploy killed a run, then boot left them looking queued).
func (s *SQLiteStore) ParkPausedBookQueue() error {
	if _, err := s.db.Exec(`UPDATE chapters SET status = 'paused', error = '',
		updated_at = CURRENT_TIMESTAMP
		WHERE status IN ('pending', 'running')
		  AND book_id IN (SELECT id FROM books WHERE status = 'paused')`); err != nil {
		return fmt.Errorf("estacionar fila de livros pausados: %w", err)
	}
	return nil
}

// ReleasePausedChapters puts parked chapters back on the active queue
// so Retomar can pick them up.
func (s *SQLiteStore) ReleasePausedChapters(bookID int64) error {
	if _, err := s.db.Exec(`UPDATE chapters SET status = 'pending',
		updated_at = CURRENT_TIMESTAMP WHERE book_id = ? AND status = 'paused'`, bookID); err != nil {
		return fmt.Errorf("devolver capítulos pausados do livro %d à fila: %w", bookID, err)
	}
	return nil
}
