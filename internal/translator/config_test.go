package translator

import (
	"strings"
	"testing"

	"github.com/puppe1990/epub-translator/internal/models"
	"github.com/puppe1990/epub-translator/internal/store"
)

func newTestStore(t *testing.T) *store.SQLiteStore {
	t.Helper()
	s, err := store.NewSQLiteStore(":memory:", "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestResolve_settingsWinOverEnvAndDefaults(t *testing.T) {
	s := newTestStore(t)
	t.Setenv("TRANSLATE_API_KEY", "chave-ambiente")
	t.Setenv("TRANSLATE_MODEL", "modelo-ambiente")

	config, err := Resolve(s)
	if err != nil {
		t.Fatal(err)
	}
	if config.APIKey != "chave-ambiente" || config.Model != "modelo-ambiente" {
		t.Errorf("fallback do ambiente falhou: %+v", config)
	}
	if config.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("BaseURL padrão = %q", config.BaseURL)
	}

	if err := s.SetSetting(KeyAPIKey, "chave-app"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSetting(KeyModel, "modelo-app"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSetting(KeyBaseURL, "https://generativelanguage.googleapis.com/v1beta/openai"); err != nil {
		t.Fatal(err)
	}
	config, err = Resolve(s)
	if err != nil {
		t.Fatal(err)
	}
	if config.APIKey != "chave-app" || config.Model != "modelo-app" ||
		!strings.Contains(config.BaseURL, "generativelanguage") {
		t.Errorf("settings não venceram o ambiente: %+v", config)
	}
}

func TestResolve_requiresKey(t *testing.T) {
	s := newTestStore(t)
	t.Setenv("TRANSLATE_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	_, err := Resolve(s)
	if err == nil || !strings.Contains(err.Error(), "/settings") {
		t.Errorf("err = %v, want erro pedindo a chave", err)
	}
}

func TestResolve_activeProviderWins(t *testing.T) {
	s := newTestStore(t)
	t.Setenv("TRANSLATE_API_KEY", "chave-ambiente")

	if err := s.SaveProvider(models.Provider{Slug: "nvidia", APIKey: "nvapi-teste"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSetting(KeyActiveProvider, "nvidia"); err != nil {
		t.Fatal(err)
	}

	config, err := Resolve(s)
	if err != nil {
		t.Fatal(err)
	}
	if config.APIKey != "nvapi-teste" {
		t.Errorf("chave = %q, want do perfil", config.APIKey)
	}
	if config.BaseURL != "https://integrate.api.nvidia.com/v1" {
		t.Errorf("BaseURL = %q, want preset da NVIDIA", config.BaseURL)
	}
	if config.Model != "minimaxai/minimax-m3.1-flash" {
		t.Errorf("Model = %q, want modelo padrão do preset", config.Model)
	}
}

func TestResolve_activeProviderWithoutKeyFallsBackToEnv(t *testing.T) {
	s := newTestStore(t)
	t.Setenv("TRANSLATE_API_KEY", "chave-ambiente")

	if err := s.SetSetting(KeyActiveProvider, "groq"); err != nil {
		t.Fatal(err)
	}
	config, err := Resolve(s)
	if err != nil {
		t.Fatal(err)
	}
	if config.APIKey != "chave-ambiente" {
		t.Errorf("chave = %q, want fallback do ambiente", config.APIKey)
	}
}

func TestResolve_customProviderUsesSavedBaseAndModel(t *testing.T) {
	s := newTestStore(t)
	if err := s.SaveProvider(models.Provider{
		Slug: "custom", BaseURL: "https://meu-proxy/v1", Model: "meu-modelo", APIKey: "k",
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSetting(KeyActiveProvider, "custom"); err != nil {
		t.Fatal(err)
	}
	config, _ := Resolve(s)
	if config.BaseURL != "https://meu-proxy/v1" || config.Model != "meu-modelo" {
		t.Errorf("custom = %+v", config)
	}
}

func TestPresetBySlug(t *testing.T) {
	for _, slug := range []string{"openai", "gemini", "groq", "nvidia", "openrouter", "custom"} {
		if _, ok := PresetBySlug(slug); !ok {
			t.Errorf("preset %q ausente", slug)
		}
	}
	if _, ok := PresetBySlug("inexistente"); ok {
		t.Error("PresetBySlug deveria recusar slug desconhecido")
	}
	if nvidia, _ := PresetBySlug("nvidia"); !strings.Contains(nvidia.BaseURL, "nvidia.com") {
		t.Errorf("base da NVIDIA = %q", nvidia.BaseURL)
	}
}
