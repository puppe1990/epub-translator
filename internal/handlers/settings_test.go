package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/puppe1990/amarra-cais/pkg/cais"

	"github.com/puppe1990/epub-translator/internal/models"
	"github.com/puppe1990/epub-translator/internal/store"
	"github.com/puppe1990/epub-translator/internal/translator"
)

func newSettingsHandler(t *testing.T) (*SettingsHandler, store.Store) {
	t.Helper()
	s := setupTestStore(t)
	handler := NewSettingsHandler(setupTestViews(t), s, testSite(), nil, cais.Config{})
	handler.modelCatalog = nil
	return handler, s
}

func TestSettingsShow_listsProvidersAndMasksKeys(t *testing.T) {
	t.Setenv("TRANSLATE_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	handler, s := newSettingsHandler(t)

	if err := s.SaveProvider(models.Provider{Slug: "nvidia", APIKey: "nvapi-secreta-1234"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSetting(translator.KeyActiveProvider, "nvidia"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/settings", nil)
	rr := httptest.NewRecorder()
	handler.Show(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	page := rr.Body.String()
	for _, want := range []string{
		"NVIDIA NIM", "OpenAI", "Gemini", "Groq", "OpenRouter", "Personalizado",
		"••••1234", "Ativo:", "Um provedor por vez",
		"minimaxai/minimax-m3.1-flash",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("página sem %q", want)
		}
	}
	if strings.Contains(page, "nvapi-secreta-1234") {
		t.Error("a chave completa não pode aparecer na página")
	}
}

func TestSettingsShow_nvidiaModelSelectPreservesSavedModel(t *testing.T) {
	for _, model := range []string{"minimaxai/minimax-m3.1-flash", "nvidia/nemotron-3.5-lightning-30b-a3b", "meu-modelo-nim"} {
		t.Run(model, func(t *testing.T) {
			handler, s := newSettingsHandler(t)
			if err := s.SaveProvider(models.Provider{Slug: "nvidia", Model: model, APIKey: "nvapi-secreta"}); err != nil {
				t.Fatal(err)
			}
			rr := httptest.NewRecorder()
			handler.Show(rr, httptest.NewRequest(http.MethodGet, "/settings", nil))
			page := rr.Body.String()
			start := strings.Index(page, `action="/settings/providers/nvidia"`)
			if start < 0 {
				t.Fatal("formulário NVIDIA ausente")
			}
			form := strings.SplitN(page[start:], "</form>", 2)[0]
			for _, want := range []string{"<select", `name="model"`, `id="model-nvidia"`, `for="model-nvidia"`, `value="` + model + `" selected`, "google/gemma-4-31b-it", "deepseek-ai/deepseek-v4.1-flash", "z-ai/glm-5.3-flash"} {
				if !strings.Contains(form, want) {
					t.Errorf("seletor NVIDIA sem %q", want)
				}
			}
			if strings.Contains(form, "nvapi-secreta") {
				t.Error("chave exposta no formulário")
			}
		})
	}
}

func TestSettingsSaveProvider_selectedNvidiaModelKeepsKeyAndResolves(t *testing.T) {
	handler, s := newSettingsHandler(t)
	if err := s.SaveProvider(models.Provider{Slug: "nvidia", APIKey: "nvapi-existente"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSetting(translator.KeyActiveProvider, "nvidia"); err != nil {
		t.Fatal(err)
	}
	model := "nvidia/nemotron-3.5-lightning-30b-a3b"
	form := url.Values{"model": {model}, "api_key": {""}}
	req := httptest.NewRequest(http.MethodPost, "/settings/providers/nvidia", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("slug", "nvidia")
	rr := httptest.NewRecorder()
	handler.SaveProvider(rr, req)
	config, err := translator.Resolve(s)
	if err != nil {
		t.Fatal(err)
	}
	if rr.Code != http.StatusSeeOther || config.Model != model || config.APIKey != "nvapi-existente" {
		t.Errorf("status=%d, modelo=%q; modelo selecionado ou chave não preservados", rr.Code, config.Model)
	}
}

func TestSettingsSaveProvider_keepsPresetBaseAndStoresModel(t *testing.T) {
	handler, s := newSettingsHandler(t)

	form := url.Values{
		"model":    {"modelo-custom-do-preset"},
		"api_key":  {"nvapi-nova"},
		"base_url": {"https://endpoint-malicioso/v1"},
	}
	req := httptest.NewRequest(http.MethodPost, "/settings/providers/nvidia", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("slug", "nvidia")
	rr := httptest.NewRecorder()
	handler.SaveProvider(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	provider, err := s.GetProvider("nvidia")
	if err != nil {
		t.Fatal(err)
	}
	if provider.APIKey != "nvapi-nova" || provider.Model != "modelo-custom-do-preset" {
		t.Errorf("provider = %+v", provider)
	}
	if provider.BaseURL != "https://integrate.api.nvidia.com/v1" {
		t.Errorf("base dos presets é fixa; gravou %q", provider.BaseURL)
	}

	unknown := httptest.NewRequest(http.MethodPost, "/settings/providers/inexistente", nil)
	unknown.SetPathValue("slug", "inexistente")
	unknownRecorder := httptest.NewRecorder()
	handler.SaveProvider(unknownRecorder, unknown)
	if unknownRecorder.Code != http.StatusNotFound {
		t.Errorf("slug desconhecido: status = %d, want 404", unknownRecorder.Code)
	}
}

func TestSettingsSaveProvider_customUsesFormBase(t *testing.T) {
	handler, s := newSettingsHandler(t)

	form := url.Values{
		"base_url": {"https://meu-proxy/v1"},
		"model":    {"meu-modelo"},
		"api_key":  {"k"},
	}
	req := httptest.NewRequest(http.MethodPost, "/settings/providers/custom", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("slug", "custom")
	rr := httptest.NewRecorder()
	handler.SaveProvider(rr, req)

	provider, _ := s.GetProvider("custom")
	if provider.BaseURL != "https://meu-proxy/v1" || provider.Model != "meu-modelo" {
		t.Errorf("custom = %+v", provider)
	}
}

func TestSettingsActivate_requiresKeyWithoutEnvFallback(t *testing.T) {
	t.Setenv("TRANSLATE_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	handler, s := newSettingsHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/settings/providers/groq/activate", nil)
	req.SetPathValue("slug", "groq")
	rr := httptest.NewRecorder()
	handler.ActivateProvider(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rr.Code)
	}
	if active, _ := s.GetSetting(translator.KeyActiveProvider); active != "" {
		t.Errorf("ativo sem chave deveria ser recusado, ficou %q", active)
	}
}

func TestSettingsActivate_withSavedKeySetsActive(t *testing.T) {
	handler, s := newSettingsHandler(t)
	if err := s.SaveProvider(models.Provider{Slug: "nvidia", APIKey: "nvapi-x"}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/settings/providers/nvidia/activate", nil)
	req.SetPathValue("slug", "nvidia")
	rr := httptest.NewRecorder()
	handler.ActivateProvider(rr, req)

	if active, _ := s.GetSetting(translator.KeyActiveProvider); active != "nvidia" {
		t.Errorf("ativo = %q, want nvidia", active)
	}
}

func TestSettingsActivate_doesNotActivateProviderUsingAnotherProvidersEnvKey(t *testing.T) {
	t.Setenv("TRANSLATE_API_KEY", "chave-ambiente")
	handler, s := newSettingsHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/settings/providers/groq/activate", nil)
	req.SetPathValue("slug", "groq")
	rr := httptest.NewRecorder()
	handler.ActivateProvider(rr, req)

	if active, _ := s.GetSetting(translator.KeyActiveProvider); active != "" {
		t.Errorf("ativo = %q; provedor sem chave própria não pode ser ativado", active)
	}
}
