package translator

import (
	"strings"
	"testing"

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
