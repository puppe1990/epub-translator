package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/puppe1990/epub-translator/internal/models"
)

func TestProviderModelOptions_nvidiaListsTranslationCandidates(t *testing.T) {
	want := []string{
		"nvidia/riva-translate-4b-instruct-v2",
		"mistralai/mistral-large-4-1t-a52b",
		"deepseek-ai/deepseek-v4.1-flash",
		"google/gemma-4-31b-it",
		"openai/gpt-oss-20b",
		"nvidia/nemotron-3.5-lightning-30b-a3b",
		"z-ai/glm-5.3-flash",
	}
	got := providerModelOptions("nvidia", "mistralai/mistral-large-4-1t-a52b")
	if !slices.Equal(got, want) {
		t.Fatalf("catálogo NVIDIA = %v, want %v", got, want)
	}
	for _, excluded := range []string{
		"minimaxai/minimax-m3.1-flash",
		"nvidia/nemotron-3.5-super-vl-120b-a12b",
		"nvidia/nemotron-3-super-120b-a12b",
		"nvidia/nemotron-3.5-parse",
	} {
		if slices.Contains(got, excluded) {
			t.Errorf("lista de tradução não deve oferecer %s", excluded)
		}
	}
}

func TestProviderModelOptions_nvidiaKeepsSavedModelOutsideCatalog(t *testing.T) {
	saved := "minimaxai/minimax-m3.1-flash"
	got := providerModelOptions("nvidia", saved)
	if got[0] != saved || !slices.Contains(got, "google/gemma-4-31b-it") {
		t.Fatalf("modelo salvo precisa continuar selecionável sem substituir a lista: %v", got)
	}
}

func TestProviderModelOptions_geminiListsTranslationCandidates(t *testing.T) {
	want := []string{
		"gemini-3.5-flash-lite",
		"gemini-3.5-flash",
		"gemini-3.5-live-translate-preview",
		"gemini-3.8-flash",
		"gemini-3.1-flash-lite",
		"gemini-2.5-flash",
		"gemini-2.5-pro",
	}
	got := providerModelOptions("gemini", "gemini-3.5-flash")
	if !slices.Equal(got, want) {
		t.Fatalf("catálogo Gemini = %v, want %v", got, want)
	}
	for _, excluded := range []string{
		"gemini-2.5-flash-image",
		"gemini-2.5-flash-preview-tts",
		"gemini-embedding-001",
		"models/gemini-3.5-flash-lite",
	} {
		if slices.Contains(got, excluded) {
			t.Errorf("lista de tradução não deve oferecer %s", excluded)
		}
	}
}

func TestProviderModelOptions_openrouterListsCurrentFreeModels(t *testing.T) {
	got := providerModelOptions("openrouter", "google/gemini-2.5-flash")
	if got[0] != "google/gemini-2.5-flash" {
		t.Fatalf("default OpenRouter precisa continuar primeiro: %v", got)
	}
	for _, want := range []string{
		"recursive/flint-v1.0:free",
		"xingchen-agi/xing4.0-29b-a4b:free",
		"nvidia/nemotron-3.5-super-vl:free",
		"dots-studio/dots-3-note-preview:free",
		"liquid/lfm-2.5-2.6b:free",
		"nvidia/nemotron-3.5-lightning:free",
		"thinkingmachines/inkling-small:free",
		"thinkingmachines/inkling:free",
		"cohere/north-mini-code:free",
		"nvidia/nemotron-3-ultra-550b-a55b:free",
		"nvidia/nemotron-3-nano-omni-30b-a3b-reasoning:free",
		"google/gemma-4-26b-a4b-it:free",
		"google/gemma-4-31b-it:free",
	} {
		if !slices.Contains(got, want) {
			t.Errorf("seletor OpenRouter sem %s", want)
		}
	}
	if slices.Contains(got, "nvidia/nemotron-3.5-content-safety:free") {
		t.Error("content-safety é guardrail, não modelo de tradução")
	}
}

func TestMergeCuratedFirst_pinsOpenRouterFreeModelsAheadOfLiveCatalog(t *testing.T) {
	curated := providerModelOptions("openrouter", "")
	live := []string{"z-ai/glm-5", "recursive/flint-v1.0:free", "openai/gpt-5.4-mini"}
	got := mergeCuratedFirst(curated, live)
	flint := slices.Index(got, "recursive/flint-v1.0:free")
	glm := slices.Index(got, "z-ai/glm-5")
	if flint < 0 || glm < 0 || flint > glm {
		t.Fatalf("Flint precisa aparecer antes do resto do catálogo: %v", got)
	}
	if slices.Index(got, "openai/gpt-5.4-mini") != slices.Index(curated, "openai/gpt-5.4-mini") {
		t.Fatalf("modelo curated não pode duplicar: %v", got)
	}
}

func TestProviderModelOptions_geminiKeepsSavedModelOutsideCatalog(t *testing.T) {
	saved := "gemini-3.6-flash"
	got := providerModelOptions("gemini", saved)
	if got[0] != saved || !slices.Contains(got, "gemini-3.5-flash-lite") {
		t.Fatalf("modelo salvo precisa continuar selecionável sem substituir a lista: %v", got)
	}
}

type modelCatalogFixture struct{ fail bool }

func (f modelCatalogFixture) List(_ context.Context, slug, _ string) ([]string, error) {
	if f.fail {
		return nil, errors.New("catálogo indisponível")
	}
	return []string{slug + "/modelo-do-catalogo"}, nil
}

func TestSettingsShow_GroqAndOpenRouterHaveModelSelections(t *testing.T) {
	for _, slug := range []string{"groq", "openrouter"} {
		t.Run(slug, func(t *testing.T) {
			handler, s := newSettingsHandler(t)
			selected := "modelo-salvo-fora-do-catalogo"
			if err := s.SaveProvider(models.Provider{Slug: slug, Model: selected}); err != nil {
				t.Fatal(err)
			}
			rr := httptest.NewRecorder()
			handler.Show(rr, httptest.NewRequest(http.MethodGet, "/settings", nil))
			page := rr.Body.String()
			start := strings.Index(page, `action="/settings/providers/`+slug+`"`)
			form := strings.SplitN(page[start:], "</form>", 2)[0]
			for _, want := range []string{"<select", `id="model-` + slug + `"`, `value="` + selected + `" selected`, "<option"} {
				if !strings.Contains(form, want) {
					t.Errorf("%s sem %q", slug, want)
				}
			}
			if strings.Count(form, "<option") < 3 {
				t.Error("seletor precisa oferecer alternativas ao modelo salvo")
			}
			if slug == "openrouter" && !strings.Contains(form, `value="recursive/flint-v1.0:free"`) {
				t.Error("OpenRouter precisa listar os modelos :free atuais")
			}
			if strings.Contains(form, "Modelo NVIDIA NIM") {
				t.Error("rótulo de modelo pertence a outro provedor")
			}
		})
	}
}

func TestSettingsShow_UsesLiveCatalogAndSurvivesCatalogFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		handler, s := newSettingsHandler(t)
		handler.modelCatalog = modelCatalogFixture{fail: fail}
		for _, slug := range []string{"groq", "openrouter"} {
			if err := s.SaveProvider(models.Provider{Slug: slug, Model: "modelo-salvo", APIKey: "chave-nunca-exposta"}); err != nil {
				t.Fatal(err)
			}
		}
		rr := httptest.NewRecorder()
		handler.Show(rr, httptest.NewRequest(http.MethodGet, "/settings", nil))
		page := rr.Body.String()
		if rr.Code != http.StatusOK || strings.Contains(page, "chave-nunca-exposta") {
			t.Fatalf("catálogo não deve quebrar a página ou expor a chave; status=%d", rr.Code)
		}
		for _, slug := range []string{"groq", "openrouter"} {
			if !fail && !strings.Contains(page, slug+"/modelo-do-catalogo") {
				t.Errorf("catálogo de %s não chegou ao seletor", slug)
			}
		}
		if !strings.Contains(page, `value="modelo-salvo" selected`) {
			t.Error("modelo salvo não pode ser substituído pela primeira opção")
		}
		if fail && !strings.Contains(page, "Não foi possível atualizar o catálogo") {
			t.Error("falha do catálogo precisa explicar por que a lista local foi mantida")
		}
	}
}
