package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/puppe1990/amarra-cais/pkg/cais/devlog"
	"github.com/puppe1990/amarra-cais/pkg/cais/session"
	caissqlite "github.com/puppe1990/amarra-cais/pkg/cais/sqlite"
	"github.com/puppe1990/amarra-cais/pkg/cais/sqllog"
	_ "modernc.org/sqlite"

	"github.com/puppe1990/epub-translator/internal/models"
)

type Store interface {
	CreateBook(book models.Book) (int64, error)
	CreateChapters(bookID int64, chapters []models.Chapter) error
	ListBooks() ([]models.Book, error)
	FindBook(id int64) (models.Book, error)
	ListChapters(bookID int64) ([]models.Chapter, error)
	FindChapter(id int64) (models.Chapter, error)
	PendingChapters(bookID int64) ([]models.Chapter, error)
	ChapterTranslations(bookID int64) (map[string]string, error)
	SetChapterRunning(id int64) error
	StartChapterExecution(id int64, details models.TranslationDetails, started time.Time) error
	FinishChapterExecution(id int64, finished time.Time, durationMS int64) error
	SetChapterDone(id int64, translated string) error
	SetChapterFailed(id int64, message string) error
	ResetChapter(id int64) error
	ResetRunningChapters() error
	SetBookStatus(id int64, status string) error
	PauseBook(id int64) error
	GetSetting(key string) (string, error)
	SetSetting(key, value string) error
	Settings() (map[string]string, error)
	SaveProvider(provider models.Provider) error
	GetProvider(slug string) (models.Provider, error)
	ListProviders() ([]models.Provider, error)
	Sessions() session.Store
	Ping() error
	DB() *sql.DB
	Close() error
}

type SQLiteStore struct {
	db *sqllog.DB
}

func NewSQLiteStore(dsn string, env string) (*SQLiteStore, error) {
	if dsn != ":memory:" {
		dir := filepath.Dir(dsn)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}

	db, err := sql.Open("sqlite", caissqlite.DSN(dsn))
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	// WAL + busy_timeout + MaxOpenConns(1): sem isto o :memory: dos testes vira
	// um banco por conexão e o SQLITE_BUSY aparece sob concorrência.
	if err := caissqlite.Configure(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("configure sqlite: %w", err)
	}

	if err := applyMigrations(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := session.EnsureSQLiteSchema(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sessions schema: %w", err)
	}

	cfg := sqllog.ConfigForEnv(env)
	if cfg.Enabled {
		cfg.Writer = devlog.MirrorDefault(os.Stdout)
	}
	store := &SQLiteStore{db: sqllog.Wrap(db, cfg)}
	if err := store.backfillSourceTokens(); err != nil {
		_ = store.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) Sessions() session.Store {
	return session.NewSQLiteStore(s.db.Raw())
}

func (s *SQLiteStore) Ping() error {
	return s.db.Raw().Ping()
}

func (s *SQLiteStore) DB() *sql.DB {
	return s.db.Raw()
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
