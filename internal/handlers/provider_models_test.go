package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/puppe1990/epub-translator/internal/models"
)

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
