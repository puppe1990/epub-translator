-- Perfis de provedor de tradução: cada um guarda a própria chave, então trocar
-- de provedor é um clique em /settings (antes havia uma config única).

-- up
CREATE TABLE IF NOT EXISTS providers (
    slug TEXT PRIMARY KEY,
    base_url TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    api_key TEXT NOT NULL DEFAULT '',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Migra a configuração legada (settings translate_*) para o perfil "custom",
-- para quem já tinha chave salva continuar funcionando após o upgrade.
INSERT OR IGNORE INTO providers (slug, base_url, model, api_key)
SELECT 'custom',
       COALESCE((SELECT value FROM settings WHERE key = 'translate_base_url'), ''),
       COALESCE((SELECT value FROM settings WHERE key = 'translate_model'), ''),
       (SELECT value FROM settings WHERE key = 'translate_api_key')
WHERE EXISTS (SELECT 1 FROM settings WHERE key = 'translate_api_key' AND value <> '')
  AND NOT EXISTS (SELECT 1 FROM providers WHERE slug = 'custom');

INSERT OR IGNORE INTO settings (key, value)
SELECT 'active_provider', 'custom'
WHERE EXISTS (SELECT 1 FROM providers WHERE slug = 'custom' AND api_key <> '')
  AND NOT EXISTS (SELECT 1 FROM settings WHERE key = 'active_provider');

-- down
DROP TABLE IF EXISTS providers;
