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

	"github.com/puppe1990/epub-translator/internal/store"
	"github.com/puppe1990/epub-translator/internal/translate"
	"github.com/puppe1990/epub-translator/internal/translator"
)

type SettingsHandler struct {
	views   *view.Renderer
	store   store.Store
	site    meta.Site
	catalog *i18n.Catalog
	cfg     cais.Config
}

func NewSettingsHandler(views *view.Renderer, s store.Store, site meta.Site,
	catalog *i18n.Catalog, cfg cais.Config) *SettingsHandler {
	return &SettingsHandler{views: views, store: s, site: site, catalog: catalog, cfg: cfg}
}

func (h *SettingsHandler) Show(w http.ResponseWriter, r *http.Request) {
	settings, err := h.store.Settings()
	if err != nil {
		httpx.ServerError(w, err, h.cfg)
		return
	}
	effective, resolveErr := translator.Resolve(h.store)
	h.render(w, r, settings, effective, resolveErr, http.StatusOK)
}

func (h *SettingsHandler) render(w http.ResponseWriter, r *http.Request, settings map[string]string,
	effective translator.Config, resolveErr error, status int) {
	keyOrigin := "nenhuma"
	if effective.APIKey != "" {
		keyOrigin = maskKey(effective.APIKey)
		if settings[translator.KeyAPIKey] == "" {
			keyOrigin += " (do ambiente)"
		}
	}
	errMsg := ""
	if resolveErr != nil {
		errMsg = resolveErr.Error()
	}
	writeView(w, r, h.views, h.cfg, "app", "settings", amarraData(r, h.site, map[string]any{
		"Title":     "Configurações",
		"ActiveNav": "settings",
		"BaseURL":   effective.BaseURL,
		"Model":     effective.Model,
		"KeyOrigin": keyOrigin,
		"EnvKeys":   envKeysHint(),
		"Error":     errMsg,
	}), status)
}

// Save grava o endpoint do provedor; a chave só é sobrescrita quando o campo
// vem preenchido (o formulário nunca mostra a chave inteira de volta).
func (h *SettingsHandler) Save(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		httpx.ServerError(w, err, h.cfg)
		return
	}
	base := strings.TrimSpace(r.FormValue("translate_base_url"))
	model := strings.TrimSpace(r.FormValue("translate_model"))
	key := strings.TrimSpace(r.FormValue("translate_api_key"))

	for name, value := range map[string]string{
		translator.KeyBaseURL: base,
		translator.KeyModel:   model,
	} {
		if err := h.store.SetSetting(name, value); err != nil {
			httpx.ServerError(w, err, h.cfg)
			return
		}
	}
	if key != "" {
		if err := h.store.SetSetting(translator.KeyAPIKey, key); err != nil {
			httpx.ServerError(w, err, h.cfg)
			return
		}
	}
	flash.Set(w, "notice", "Configuração salva.", h.cfg.CookieSecure())
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

// Test faz uma tradução mínima para validar chave/endpoint/modelo na hora.
func (h *SettingsHandler) Test(w http.ResponseWriter, r *http.Request) {
	config, err := translator.Resolve(h.store)
	if err != nil {
		flash.Set(w, "alert", err.Error(), h.cfg.CookieSecure())
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
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

// envKeysHint lista as variáveis de ambiente aceitas — mostrado na página
// para quem preferir configurar fora do app.
func envKeysHint() []string {
	var found []string
	for _, name := range []string{"TRANSLATE_API_KEY", "OPENAI_API_KEY", "TRANSLATE_BASE_URL", "TRANSLATE_MODEL"} {
		if os.Getenv(name) != "" {
			found = append(found, name)
		}
	}
	return found
}
