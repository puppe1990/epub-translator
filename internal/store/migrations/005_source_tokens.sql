-- Tokens cl100k do XHTML original. A listagem lê esta coluna e não o corpo.

-- up
ALTER TABLE chapters ADD COLUMN source_tokens INTEGER NOT NULL DEFAULT 0;

-- down
ALTER TABLE chapters DROP COLUMN source_tokens;
