package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/puppe1990/amarra-cais/pkg/amarra/view"
	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/flash"
	"github.com/puppe1990/amarra-cais/pkg/cais/httpx"
	"github.com/puppe1990/amarra-cais/pkg/cais/i18n"
	"github.com/puppe1990/amarra-cais/pkg/cais/meta"

	"github.com/puppe1990/epub-translator/internal/models"
	"github.com/puppe1990/epub-translator/internal/store"
	"github.com/puppe1990/epub-translator/internal/translate"
	"github.com/puppe1990/epub-translator/internal/translator"
)

type SettingsHandler struct {
	views        *view.Renderer
	store        store.Store
	site         meta.Site
	catalog      *i18n.Catalog
	cfg          cais.Config
	modelCatalog providerModelLister
}

func NewSettingsHandler(views *view.Renderer, s store.Store, site meta.Site,
	catalog *i18n.Catalog, cfg cais.Config) *SettingsHandler {
	return &SettingsHandler{views: views, store: s, site: site, catalog: catalog, cfg: cfg, modelCatalog: translator.NewModelCatalog(nil)}
}

// providerRow é a linha de um preset de provedor na página.
type providerRow struct {
	Slug                string
	Name                string
	BaseURL             string
	Model               string
	ModelOptions        []string
	KeyMask             string
	HasKey              bool
	Active              bool
	IsCustom            bool
	ModelCatalogWarning string
}

func (h *SettingsHandler) Show(w http.ResponseWriter, r *http.Request) {
	settings, err := h.store.Settings()
	if err != nil {
		httpx.ServerError(w, err, h.cfg)
		return
	}
	saved, err := h.store.ListProviders()
	if err != nil {
		httpx.ServerError(w, err, h.cfg)
		return
	}
	bySlug := map[string]models.Provider{}
	for _, provider := range saved {
		bySlug[provider.Slug] = provider
	}

	active := settings[translator.KeyActiveProvider]
	rows := make([]providerRow, 0, len(translator.Presets))
	activeName := ""
	for _, preset := range translator.Presets {
		provider := bySlug[preset.Slug]
		row := providerRow{
			Slug:     preset.Slug,
			Name:     preset.Name,
			BaseURL:  firstNonEmpty(provider.BaseURL, preset.BaseURL),
			Model:    firstNonEmpty(provider.Model, preset.DefaultModel),
			HasKey:   provider.APIKey != "",
			Active:   preset.Slug == active,
			IsCustom: preset.Slug == "custom",
		}
		if row.HasKey {
			row.KeyMask = maskKey(provider.APIKey)
		}
		row.ModelOptions = providerModelOptions(preset.Slug, row.Model)
		if row.Active {
			activeName = preset.Name
		}
		rows = append(rows, row)
	}

	h.loadProviderModels(r.Context(), rows, bySlug)
	effective, resolveErr := translator.Resolve(h.store)
	errMsg := ""
	if resolveErr != nil {
		errMsg = resolveErr.Error()
	}
	keyOrigin := "ausente"
	if effective.APIKey != "" {
		keyOrigin = maskKey(effective.APIKey)
	}
	writeView(w, r, h.views, h.cfg, "app", "settings", amarraData(r, h.site, map[string]any{
		"Title":      "Configurações",
		"ActiveNav":  "settings",
		"Providers":  rows,
		"ActiveName": activeName,
		"BaseURL":    effective.BaseURL,
		"Model":      effective.Model,
		"KeyOrigin":  keyOrigin,
		"EnvKeys":    envKeysHint(),
		"Error":      errMsg,
	}), http.StatusOK)
}

// SaveProvider grava modelo/chave de um preset. A base URL dos presets é fixa
// em código (só o "custom" aceita base livre) para ninguém apontar uma chave
// OpenAI para um endpoint arbitrário sem querer.
func (h *SettingsHandler) SaveProvider(w http.ResponseWriter, r *http.Request) {
	preset, ok := translator.PresetBySlug(r.PathValue("slug"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		httpx.ServerError(w, err, h.cfg)
		return
	}
	base := preset.BaseURL
	if preset.Slug == "custom" {
		base = strings.TrimSpace(r.FormValue("base_url"))
	}
	provider := models.Provider{
		Slug:    preset.Slug,
		BaseURL: base,
		Model:   strings.TrimSpace(r.FormValue("model")),
		APIKey:  strings.TrimSpace(r.FormValue("api_key")),
	}
	if err := h.store.SaveProvider(provider); err != nil {
		httpx.ServerError(w, err, h.cfg)
		return
	}
	flash.Set(w, "notice", fmt.Sprintf("Provedor %s salvo.", preset.Name), h.cfg.CookieSecure())
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

// SelectProvider accepts the single radio group without duplicating activation.
func (h *SettingsHandler) SelectProvider(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		httpx.ServerError(w, err, h.cfg)
		return
	}
	selected := r.PostForm["provider"]
	if len(selected) != 1 {
		flash.Set(w, "alert", "Escolha um único provedor para ativar. O provedor atual foi mantido.", h.cfg.CookieSecure())
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	slug := strings.TrimSpace(selected[0])
	if _, ok := translator.PresetBySlug(slug); !ok {
		flash.Set(w, "alert", fmt.Sprintf("Provedor %q inválido. Escolha uma das opções da lista.", slug), h.cfg.CookieSecure())
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	r.SetPathValue("slug", slug)
	h.ActivateProvider(w, r)
}

// ActivateProvider replaces the one active_provider setting, keeping saved keys.
func (h *SettingsHandler) ActivateProvider(w http.ResponseWriter, r *http.Request) {
	preset, ok := translator.PresetBySlug(r.PathValue("slug"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	provider, err := h.store.GetProvider(preset.Slug)
	if err != nil || provider.APIKey == "" {
		flash.Set(w, "alert", fmt.Sprintf("Salve a chave do %s antes de ativar. O provedor atual foi mantido.", preset.Name), h.cfg.CookieSecure())
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	if err := h.store.SetSetting(translator.KeyActiveProvider, preset.Slug); err != nil {
		httpx.ServerError(w, err, h.cfg)
		return
	}
	flash.Set(w, "notice", fmt.Sprintf("%s é o único provedor ativo. A mudança vale no próximo capítulo.", preset.Name), h.cfg.CookieSecure())
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

// Test faz uma tradução mínima com o provedor ativo para validar chave,
// endpoint e modelo na hora.
func (h *SettingsHandler) Test(w http.ResponseWriter, r *http.Request) {
	config, err := translator.Resolve(h.store)
	if err != nil {
		flash.Set(w, "alert", err.Error(), h.cfg.CookieSecure())
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	// Botão de teste: modelos "thinking"/provedores lentos podem levar >60s por
	// uma única chamada; o teto aqui é só para a requisição não pendurar.
	ctx, cancel := context.WithTimeout(r.Context(), 150*time.Second)
	defer cancel()

	client := translate.Client{BaseURL: config.BaseURL, APIKey: config.APIKey, Model: config.Model}
	out, err := client.Translate(ctx, "pt-BR", "Traduza para português do Brasil:\n\n<p>Hello, this is a test.</p>")
	if err != nil {
		flash.Set(w, "alert", "Falhou: "+err.Error(), h.cfg.CookieSecure())
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	flash.Set(w, "notice", "Chave OK — resposta: "+out, h.cfg.CookieSecure())
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func maskKey(key string) string {
	if len(key) <= 4 {
		return "••••"
	}
	return fmt.Sprintf("••••%s", key[len(key)-4:])
}

// envKeysHint lista as variáveis de ambiente com chave/endpoint definidos —
// mostrado na página para quem preferir configurar fora do app.
func envKeysHint() []string {
	var found []string
	for _, name := range []string{"TRANSLATE_API_KEY", "OPENAI_API_KEY", "TRANSLATE_BASE_URL", "TRANSLATE_MODEL"} {
		if os.Getenv(name) != "" {
			found = append(found, name)
		}
	}
	return found
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
