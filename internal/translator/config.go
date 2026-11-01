// Package translator combina os perfis de provedor salvos no app (página
// /settings) com variáveis de ambiente para montar o tradutor usado pela
// pipeline. A resolução acontece a cada chamada, então trocar a chave ou o
// provedor ativo vale imediatamente para o próximo capítulo.
package translator

import (
	"context"
	"fmt"
	"os"

	"github.com/puppe1990/epub-translator/internal/store"
	"github.com/puppe1990/epub-translator/internal/translate"
)

// Chaves guardadas na tabela settings.
const (
	KeyBaseURL        = "translate_base_url"
	KeyAPIKey         = "translate_api_key"
	KeyModel          = "translate_model"
	KeyActiveProvider = "active_provider"
)

// Config é o endpoint OpenAI-compatível efetivo.
type Config struct {
	BaseURL string
	APIKey  string
	Model   string
}

// Resolve aplica a precedência: perfil ativo em /settings > configuração
// legada (settings translate_*) > ambiente > padrão.
func Resolve(s store.Store) (Config, error) {
	settings, err := s.Settings()
	if err != nil {
		return Config{}, err
	}

	if slug := settings[KeyActiveProvider]; slug != "" {
		return providerConfig(s, slug)
	}

	config := Config{
		BaseURL: firstNonEmpty(settings[KeyBaseURL], os.Getenv("TRANSLATE_BASE_URL"), "https://api.openai.com/v1"),
		APIKey:  firstNonEmpty(settings[KeyAPIKey], os.Getenv("TRANSLATE_API_KEY"), os.Getenv("OPENAI_API_KEY")),
		Model:   firstNonEmpty(settings[KeyModel], os.Getenv("TRANSLATE_MODEL"), "gpt-4.1-mini"),
	}
	if config.APIKey == "" {
		return config, fmt.Errorf("nenhuma chave de API configurada — salve e ative um provedor em /settings ou defina TRANSLATE_API_KEY")
	}
	return config, nil
}

// An explicit provider must never silently translate using another endpoint.
func providerConfig(s store.Store, slug string) (Config, error) {
	preset, ok := PresetBySlug(slug)
	if !ok {
		return Config{}, fmt.Errorf("provedor ativo %q desconhecido — escolha um provedor em /settings", slug)
	}
	provider, err := s.GetProvider(slug)
	if err != nil || provider.APIKey == "" {
		return Config{}, fmt.Errorf("%s está selecionado, mas não tem chave salva — configure sua chave em /settings", preset.Name)
	}
	baseURL := firstNonEmpty(provider.BaseURL, preset.BaseURL)
	model := firstNonEmpty(provider.Model, preset.DefaultModel)
	if baseURL == "" || model == "" {
		return Config{}, fmt.Errorf("%s está incompleto — preencha Base URL e modelo em /settings", preset.Name)
	}
	return Config{
		BaseURL: baseURL,
		APIKey:  provider.APIKey,
		Model:   model,
	}, nil
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
