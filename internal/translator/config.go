// Package translator combina as configurações salvas no app (página
// /settings) com variáveis de ambiente para montar o tradutor usado pela
// pipeline. A resolução acontece a cada chamada, então trocar a chave na UI
// vale imediatamente para o próximo capítulo.
package translator

import (
	"context"
	"fmt"
	"os"

	"github.com/puppe1990/epub-translator/internal/store"
	"github.com/puppe1990/epub-translator/internal/translate"
)

// Chaves guardadas na tabela settings (página /settings).
const (
	KeyBaseURL = "translate_base_url"
	KeyAPIKey  = "translate_api_key"
	KeyModel   = "translate_model"
)

// Config é o endpoint OpenAI-compatível efetivo.
type Config struct {
	BaseURL string
	APIKey  string
	Model   string
}

// Resolve aplica a precedência: settings do app > ambiente > padrão.
func Resolve(s store.Store) (Config, error) {
	settings, err := s.Settings()
	if err != nil {
		return Config{}, err
	}
	config := Config{
		BaseURL: firstNonEmpty(settings[KeyBaseURL], os.Getenv("TRANSLATE_BASE_URL"), "https://api.openai.com/v1"),
		APIKey:  firstNonEmpty(settings[KeyAPIKey], os.Getenv("TRANSLATE_API_KEY"), os.Getenv("OPENAI_API_KEY")),
		Model:   firstNonEmpty(settings[KeyModel], os.Getenv("TRANSLATE_MODEL"), "gpt-4.1-mini"),
	}
	if config.APIKey == "" {
		return config, fmt.Errorf("nenhuma chave de API configurada — defina em /settings ou em TRANSLATE_API_KEY")
	}
	return config, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// Configured implementa pipeline.Translator resolvendo a configuração a cada
// capítulo — é o que o runner recebe em produção.
type Configured struct {
	Store store.Store
}

func (c Configured) TranslateBody(ctx context.Context, targetLanguage, body string) (string, error) {
	config, err := Resolve(c.Store)
	if err != nil {
		return "", err
	}
	service := translate.Service{
		Client: translate.Client{
			BaseURL: config.BaseURL,
			APIKey:  config.APIKey,
			Model:   config.Model,
		},
		Attempts: 3,
	}
	return service.TranslateBody(ctx, targetLanguage, body)
}
