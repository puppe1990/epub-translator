-- Usuários do estúdio: login próprio (o app deixou de ser público).
-- is_admin fica pronto para gates administrativos futuros (middleware.RequireAdmin).

-- up
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    is_admin INTEGER NOT NULL DEFAULT 0 CHECK (is_admin IN (0, 1)),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- down
DROP TABLE IF EXISTS users;
