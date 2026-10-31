package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/puppe1990/amarra-cais/pkg/cais"

	"github.com/puppe1990/epub-translator/internal/store"
	"github.com/puppe1990/epub-translator/internal/translator"
)

func newSettingsHandler(t *testing.T) (*SettingsHandler, store.Store) {
	t.Helper()
	s := setupTestStore(t)
	return NewSettingsHandler(setupTestViews(t), s, testSite(), nil, cais.Config{}), s
}

func TestSettingsShow_masksKeyAndShowsDefaults(t *testing.T) {
	t.Setenv("TRANSLATE_API_KEY", "")
	handler, s := newSettingsHandler(t)
	t.Cleanup(func() { _ = s.SetSetting(translator.KeyAPIKey, "") })

	if err := s.SetSetting(translator.KeyAPIKey, "chave-secreta-1234"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/settings", nil)
	rr := httptest.NewRecorder()
	handler.Show(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	page := rr.Body.String()
	for _, want := range []string{"Configurações", "https://api.openai.com/v1", "gpt-4.1-mini", "••••1234"} {
		if !strings.Contains(page, want) {
			t.Errorf("página sem %q", want)
		}
	}
	if strings.Contains(page, "chave-secreta-1234") {
		t.Error("a chave completa não pode aparecer na página")
	}
}

func TestSettingsSave_storesEndpointAndOnlyOverwritesKeyWhenGiven(t *testing.T) {
	handler, s := newSettingsHandler(t)
	if err := s.SetSetting(translator.KeyAPIKey, "chave-antiga"); err != nil {
		t.Fatal(err)
	}

	form := url.Values{
		"translate_base_url": {"https://generativelanguage.googleapis.com/v1beta/openai"},
		"translate_model":    {"gemini-3.5-flash-lite"},
		"translate_api_key":  {""},
	}
	req := httptest.NewRequest(http.MethodPost, "/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.Save(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if base, _ := s.GetSetting(translator.KeyBaseURL); !strings.Contains(base, "generativelanguage") {
		t.Errorf("base url = %q", base)
	}
	if model, _ := s.GetSetting(translator.KeyModel); model != "gemini-3.5-flash-lite" {
		t.Errorf("model = %q", model)
	}
	if key, _ := s.GetSetting(translator.KeyAPIKey); key != "chave-antiga" {
		t.Errorf("chave vazia sobrescreveu a existente: %q", key)
	}

	form.Set("translate_api_key", "chave-nova")
	req = httptest.NewRequest(http.MethodPost, "/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	handler.Save(rr, req)
	if key, _ := s.GetSetting(translator.KeyAPIKey); key != "chave-nova" {
		t.Errorf("chave nova não persistiu: %q", key)
	}
}
