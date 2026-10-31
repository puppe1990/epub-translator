-- Tradutor de EPUB: livros enviados, capítulos (um documento XHTML por linha,
-- para retry por capítulo) e as configurações do provedor de tradução.

-- up
CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS books (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL DEFAULT '',
    author TEXT NOT NULL DEFAULT '',
    source_language TEXT NOT NULL DEFAULT '',
    target_language TEXT NOT NULL DEFAULT 'pt-BR',
    filename TEXT NOT NULL DEFAULT '',
    source_path TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS chapters (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    book_id INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    doc_path TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    source_text TEXT NOT NULL DEFAULT '',
    translated_text TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    error TEXT NOT NULL DEFAULT '',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(book_id, doc_path)
);

CREATE INDEX IF NOT EXISTS idx_chapters_book_position ON chapters(book_id, position);

-- down
DROP TABLE IF EXISTS chapters;
DROP TABLE IF EXISTS books;
DROP TABLE IF EXISTS settings;
