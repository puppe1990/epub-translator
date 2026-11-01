package translator

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/puppe1990/epub-translator/internal/models"
	"github.com/puppe1990/epub-translator/internal/translate"
)

func TestPreparedTranslationKeepsTheRecordedModelWhenSettingsChange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct{ Model string }
		_ = json.NewDecoder(r.Body).Decode(&request)
		if request.Model != "model-original" || r.Header.Get("Authorization") != "Bearer original-test-key" {
			t.Error("requisição usou configuração diferente da registrada")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"<p>Olá mundo.</p>"}}]}`))
	}))
	defer server.Close()
	s := newTestStore(t)
	_ = s.SaveProvider(models.Provider{Slug: "custom", BaseURL: server.URL, Model: "model-original", APIKey: "original-test-key"})
	_ = s.SetSetting(KeyActiveProvider, "custom")
	prepared, details, err := (Configured{Store: s}).Prepare()
	if err != nil || details.Provider != "custom" || details.Model != "model-original" {
		t.Fatalf("configuração preparada incorreta: %+v, %v", details, err)
	}
	_ = s.SaveProvider(models.Provider{Slug: "custom", BaseURL: server.URL, Model: "model-novo", APIKey: "different-test-key"})
	if _, err := prepared.TranslateBody(context.Background(), "pt-BR", "<p>Hello world.</p>"); err != nil {
		t.Fatal(err)
	}
}

func TestPrepare_rivaGetsInputCharBudget(t *testing.T) {
	s := newTestStore(t)
	_ = s.SaveProvider(models.Provider{
		Slug: "nvidia", BaseURL: "https://integrate.api.nvidia.com/v1",
		Model: "nvidia/riva-translate-4b-instruct-v2", APIKey: "k",
	})
	_ = s.SetSetting(KeyActiveProvider, "nvidia")
	prepared, _, err := (Configured{Store: s}).Prepare()
	if err != nil {
		t.Fatal(err)
	}
	service, ok := prepared.(translate.Service)
	if !ok {
		t.Fatalf("Prepare devolveu %T, want translate.Service", prepared)
	}
	want := translate.InputCharBudget("nvidia/riva-translate-4b-instruct-v2")
	if service.MaxInputChars != want || want <= 0 {
		t.Errorf("MaxInputChars = %d, want %d", service.MaxInputChars, want)
	}
}

func TestPrepare_geminiGetsInputCharBudget(t *testing.T) {
	s := newTestStore(t)
	_ = s.SaveProvider(models.Provider{
		Slug: "gemini", BaseURL: "https://generativelanguage.googleapis.com/v1beta/openai",
		Model: "gemini-3.5-flash-lite", APIKey: "k",
	})
	_ = s.SetSetting(KeyActiveProvider, "gemini")
	prepared, _, err := (Configured{Store: s}).Prepare()
	if err != nil {
		t.Fatal(err)
	}
	service, ok := prepared.(translate.Service)
	if !ok {
		t.Fatalf("Prepare devolveu %T, want translate.Service", prepared)
	}
	want := translate.InputCharBudget("gemini-3.5-flash-lite")
	if service.MaxInputChars != want || want <= 0 {
		t.Errorf("MaxInputChars = %d, want %d (Gemini 64k output)", service.MaxInputChars, want)
	}
}
