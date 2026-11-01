package store

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/puppe1990/epub-translator/internal/models"
)

// ErrBookNotFound marca livro inexistente para os handlers responderem 404.
var ErrBookNotFound = errors.New("livro não encontrado")

// bookSelect carrega o livro já com os contadores de capítulos — a página
// inicial e a barra de progresso leem só isto.
const bookSelect = `
SELECT b.id, b.title, b.author, b.source_language, b.target_language, b.filename,
       b.source_path, b.status, b.created_at,
       COUNT(c.id),
       COALESCE(SUM(c.status = 'done'), 0),
       COALESCE(SUM(c.status = 'failed'), 0)
FROM books b LEFT JOIN chapters c ON c.book_id = b.id`

func (s *SQLiteStore) CreateBook(book models.Book) (int64, error) {
	result, err := s.db.Exec(
		`INSERT INTO books (title, author, source_language, target_language, filename, source_path, status)
		 VALUES (?, ?, ?, ?, ?, ?, 'pending')`,
		book.Title, book.Author, book.SourceLanguage, book.TargetLanguage, book.Filename, book.SourcePath,
	)
	if err != nil {
		return 0, fmt.Errorf("criar livro: %w", err)
	}
	return result.LastInsertId()
}

func (s *SQLiteStore) CreateChapters(bookID int64, chapters []models.Chapter) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("inserir capítulos: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, chapter := range chapters {
		if _, err := tx.Exec(
			`INSERT INTO chapters (book_id, position, doc_path, title, source_text)
			 VALUES (?, ?, ?, ?, ?)`,
			bookID, chapter.Position, chapter.DocPath, chapter.Title, chapter.SourceText,
		); err != nil {
			return fmt.Errorf("inserir capítulo %s: %w", chapter.DocPath, err)
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) ListBooks() ([]models.Book, error) {
	rows, err := s.db.Query(bookSelect + " GROUP BY b.id ORDER BY b.id DESC")
	if err != nil {
		return nil, fmt.Errorf("listar livros: %w", err)
	}
	defer func() { _ = rows.Close() }()

	books := []models.Book{}
	for rows.Next() {
		book, err := scanBook(rows)
		if err != nil {
			return nil, err
		}
		books = append(books, book)
	}
	return books, rows.Err()
}

func (s *SQLiteStore) FindBook(id int64) (models.Book, error) {
	rows, err := s.db.Query(bookSelect+" WHERE b.id = ? GROUP BY b.id", id)
	if err != nil {
		return models.Book{}, fmt.Errorf("buscar livro: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return models.Book{}, fmt.Errorf("livro %d: %w", id, ErrBookNotFound)
	}
	return scanBook(rows)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanBook(row rowScanner) (models.Book, error) {
	var book models.Book
	err := row.Scan(&book.ID, &book.Title, &book.Author, &book.SourceLanguage, &book.TargetLanguage,
		&book.Filename, &book.SourcePath, &book.Status, &book.CreatedAt,
		&book.TotalChapters, &book.DoneChapters, &book.FailedChapters)
	if err != nil {
		return models.Book{}, fmt.Errorf("ler livro: %w", err)
	}
	return book, nil
}

func (s *SQLiteStore) ListChapters(bookID int64) ([]models.Chapter, error) {
	rows, err := s.db.Query(
		`SELECT id, book_id, position, doc_path, title, status, attempts, error,
                 provider, model, started_at, finished_at, duration_ms,
                 length(source_text) AS source_chars
		 FROM chapters WHERE book_id = ? ORDER BY position`, bookID)
	if err != nil {
		return nil, fmt.Errorf("listar capítulos: %w", err)
	}
	defer func() { _ = rows.Close() }()

	chapters := []models.Chapter{}
	for rows.Next() {
		var chapter models.Chapter
		if err := rows.Scan(&chapter.ID, &chapter.BookID, &chapter.Position, &chapter.DocPath,
			&chapter.Title, &chapter.Status, &chapter.Attempts, &chapter.Error,
			&chapter.Provider, &chapter.Model, &chapter.StartedAt, &chapter.FinishedAt, &chapter.DurationMS,
			&chapter.SourceChars); err != nil {
			return nil, fmt.Errorf("ler capítulo: %w", err)
		}
		chapters = append(chapters, chapter)
	}
	return chapters, rows.Err()
}

func (s *SQLiteStore) FindChapter(id int64) (models.Chapter, error) {
	var chapter models.Chapter
	err := s.db.QueryRow(
		`SELECT id, book_id, position, doc_path, title, source_text, translated_text, status, attempts, error,
                 provider, model, started_at, finished_at, duration_ms
		 FROM chapters WHERE id = ?`, id,
	).Scan(&chapter.ID, &chapter.BookID, &chapter.Position, &chapter.DocPath, &chapter.Title,
		&chapter.SourceText, &chapter.TranslatedText, &chapter.Status, &chapter.Attempts, &chapter.Error,
		&chapter.Provider, &chapter.Model, &chapter.StartedAt, &chapter.FinishedAt, &chapter.DurationMS)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Chapter{}, fmt.Errorf("capítulo %d não existe", id)
	}
	if err != nil {
		return models.Chapter{}, fmt.Errorf("buscar capítulo: %w", err)
	}
	return chapter, nil
}

// ChapterTranslations devolve doc_path → corpo traduzido das partes
// concluídas — é o mapa que o rebuild do EPUB consome no download.
func (s *SQLiteStore) ChapterTranslations(bookID int64) (map[string]string, error) {
	rows, err := s.db.Query(
		`SELECT doc_path, translated_text FROM chapters
		 WHERE book_id = ? AND status = 'done' AND translated_text <> ''`, bookID)
	if err != nil {
		return nil, fmt.Errorf("listar traduções: %w", err)
	}
	defer func() { _ = rows.Close() }()

	translations := map[string]string{}
	for rows.Next() {
		var docPath, body string
		if err := rows.Scan(&docPath, &body); err != nil {
			return nil, fmt.Errorf("ler tradução: %w", err)
		}
		translations[docPath] = body
	}
	return translations, rows.Err()
}

// PendingChapters devolve o que ainda precisa de chamada: pendentes e falhos,
// em ordem — é a fila do runner e o alvo do botão de retry.
func (s *SQLiteStore) PendingChapters(bookID int64) ([]models.Chapter, error) {
	rows, err := s.db.Query(
		`SELECT id, book_id, position, doc_path, title, source_text, status, attempts, error,
                 provider, model, started_at, finished_at, duration_ms
		 FROM chapters WHERE book_id = ? AND status IN ('pending', 'failed') ORDER BY position`, bookID)
	if err != nil {
		return nil, fmt.Errorf("filtrar capítulos: %w", err)
	}
	defer func() { _ = rows.Close() }()

	chapters := []models.Chapter{}
	for rows.Next() {
		var chapter models.Chapter
		if err := rows.Scan(&chapter.ID, &chapter.BookID, &chapter.Position, &chapter.DocPath,
			&chapter.Title, &chapter.SourceText, &chapter.Status, &chapter.Attempts, &chapter.Error,
			&chapter.Provider, &chapter.Model, &chapter.StartedAt, &chapter.FinishedAt, &chapter.DurationMS); err != nil {
			return nil, fmt.Errorf("ler capítulo: %w", err)
		}
		chapters = append(chapters, chapter)
	}
	return chapters, rows.Err()
}

func (s *SQLiteStore) SetChapterRunning(id int64) error {
	return s.updateChapter(`UPDATE chapters SET status = 'running', error = '',
		updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
}

func (s *SQLiteStore) SetChapterDone(id int64, translated string) error {
	return s.updateChapter(`UPDATE chapters SET status = 'done', translated_text = ?, error = '',
		updated_at = CURRENT_TIMESTAMP WHERE id = ?`, translated, id)
}

func (s *SQLiteStore) SetChapterFailed(id int64, message string) error {
	return s.updateChapter(`UPDATE chapters SET status = 'failed', error = ?, attempts = attempts + 1,
		updated_at = CURRENT_TIMESTAMP WHERE id = ?`, message, id)
}

// ResetChapter devolve um capítulo falho para a fila (botão "Retentar").
func (s *SQLiteStore) ResetChapter(id int64) error {
	return s.updateChapter(`UPDATE chapters SET status = 'pending', error = '',
		updated_at = CURRENT_TIMESTAMP WHERE id = ? AND status = 'failed'`, id)
}

func (s *SQLiteStore) updateChapter(query string, args ...any) error {
	if _, err := s.db.Exec(query, args...); err != nil {
		return fmt.Errorf("atualizar capítulo: %w", err)
	}
	return nil
}

// ResetRunningChapters devolve capítulos travados em 'running' (restart no
// meio) para a fila — o retry manual e o boot usam isto.
func (s *SQLiteStore) ResetRunningChapters() error {
	if _, err := s.db.Exec(`UPDATE chapters SET status = 'pending' WHERE status = 'running'`); err != nil {
		return fmt.Errorf("resetar capítulos: %w", err)
	}
	if _, err := s.db.Exec(`UPDATE books SET status = 'pending' WHERE status = 'translating'`); err != nil {
		return fmt.Errorf("resetar livros: %w", err)
	}
	return nil
}

func (s *SQLiteStore) SetBookStatus(id int64, status string) error {
	if _, err := s.db.Exec(`UPDATE books SET status = ? WHERE id = ?`, status, id); err != nil {
		return fmt.Errorf("atualizar status do livro: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetSetting(key string) (string, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("ler configuração: %w", err)
	}
	return value, nil
}

func (s *SQLiteStore) SetSetting(key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO settings (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	if err != nil {
		return fmt.Errorf("salvar configuração: %w", err)
	}
	return nil
}

func (s *SQLiteStore) Settings() (map[string]string, error) {
	rows, err := s.db.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return nil, fmt.Errorf("listar configurações: %w", err)
	}
	defer func() { _ = rows.Close() }()

	settings := map[string]string{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("ler configuração: %w", err)
		}
		settings[key] = value
	}
	return settings, rows.Err()
}
