package store

import (
	"time"

	"github.com/puppe1990/epub-translator/internal/models"
)

func (s *SQLiteStore) StartChapterExecution(id int64, details models.TranslationDetails, started time.Time) error {
	return s.updateChapter(`UPDATE chapters SET status = 'running', error = '',
		provider = ?, model = ?, started_at = ?, finished_at = '', duration_ms = 0,
		updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		details.Provider, details.Model, started.UTC().Format(time.RFC3339Nano), id)
}

func (s *SQLiteStore) FinishChapterExecution(id int64, finished time.Time, durationMS int64) error {
	return s.updateChapter(`UPDATE chapters SET finished_at = ?, duration_ms = ?,
		updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		finished.UTC().Format(time.RFC3339Nano), max(0, durationMS), id)
}
