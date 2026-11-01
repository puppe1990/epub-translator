package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/puppe1990/epub-translator/internal/models"
	"github.com/puppe1990/epub-translator/internal/translator"
)

func TestSettingsShow_singleProviderSelectionMatchesEffectiveProvider(t *testing.T) {
	handler, s := newSettingsHandler(t)
	for _, slug := range []string{"groq", "openrouter"} {
		if err := s.SaveProvider(models.Provider{Slug: slug, APIKey: "chave-" + slug}); err != nil {
			t.Fatal(err)
		}
	}
	for _, slug := range []string{"groq", "openrouter"} {
		req := httptest.NewRequest(http.MethodPost, "/settings/providers/"+slug+"/activate", nil)
		req.SetPathValue("slug", slug)
		handler.ActivateProvider(httptest.NewRecorder(), req)
		rr := httptest.NewRecorder()
		handler.Show(rr, httptest.NewRequest(http.MethodGet, "/settings", nil))
		inputs := regexp.MustCompile(`<input[^>]*type="radio"[^>]*>`).FindAllString(rr.Body.String(), -1)
		checked := 0
		for _, input := range inputs {
			if !strings.Contains(input, `name="provider"`) {
				t.Error("todos os provedores devem pertencer ao mesmo grupo")
			}
			if strings.Contains(input, "checked") {
				checked++
				if !strings.Contains(input, `value="`+slug+`"`) {
					t.Errorf("seleção visual diferente de %s", slug)
				}
			}
			if strings.Contains(input, `value="gemini"`) && !strings.Contains(input, "disabled") {
				t.Error("provedor sem chave precisa ficar indisponível para ativação")
			}
		}
		if len(inputs) != 6 || checked != 1 {
			t.Errorf("radios=%d, selecionados=%d; esperava 6 opções e só 1 selecionada", len(inputs), checked)
		}
		config, err := translator.Resolve(s)
		if err != nil || config.APIKey != "chave-"+slug {
			t.Errorf("configuração efetiva não corresponde ao provedor %s: %v", slug, err)
		}
	}
}

func TestSelectProviderAcceptsExactlyOneConfiguredProvider(t *testing.T) {
	for _, values := range [][]string{{"groq"}, {}, {"groq", "openrouter"}, {"inexistente"}, {"gemini"}} {
		t.Run(strings.Join(values, "+"), func(t *testing.T) {
			handler, s := newSettingsHandler(t)
			for _, slug := range []string{"nvidia", "groq", "openrouter"} {
				if err := s.SaveProvider(models.Provider{Slug: slug, APIKey: "chave-" + slug}); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.SetSetting(translator.KeyActiveProvider, "nvidia"); err != nil {
				t.Fatal(err)
			}
			form := url.Values{"provider": values}
			req := httptest.NewRequest(http.MethodPost, "/settings/active-provider", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rr := httptest.NewRecorder()
			handler.SelectProvider(rr, req)
			want := "nvidia"
			if len(values) == 1 && values[0] == "groq" {
				want = "groq"
			}
			active, _ := s.GetSetting(translator.KeyActiveProvider)
			if active != want || rr.Code != http.StatusSeeOther {
				t.Errorf("seleção %v: ativo=%q, status=%d; esperado %q e redirect", values, active, rr.Code, want)
			}
		})
	}
}
