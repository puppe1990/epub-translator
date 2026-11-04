package app

import (
	"net/http"

	"github.com/puppe1990/amarra-cais/pkg/amarra/live"
	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/middleware"

	"github.com/puppe1990/epub-translator/internal/handlers"
)

func registerRoutes(r *cais.Router, deps Deps, cfg cais.Config) {
	books := handlers.NewBooksHandler(deps.Views, deps.Store, deps.Runner, deps.Site, deps.Catalog, cfg, deps.BooksDir)
	settings := handlers.NewSettingsHandler(deps.Views, deps.Store, deps.Site, deps.Catalog, cfg)
	auth := handlers.NewAuthHandler(deps.Views, deps.Store, deps.Site, cfg)

	// O estúdio é privado: tudo abaixo exige sessão; só /login (e /static
	// e /health, registrados fora daqui) fica público.
	require := func(handler http.HandlerFunc) http.HandlerFunc {
		return middleware.RequireAuthFunc("/login", handler)
	}
	loginLimit := middleware.NewRateLimiter(10, cfg)

	r.Get("/login", auth.Login)
	r.Post("/login", loginLimit.Middleware(http.HandlerFunc(auth.LoginPost)).ServeHTTP)
	r.Post("/logout", auth.LogoutPost)

	r.Get("/", require(books.Index))
	r.Post("/books", require(books.Upload))
	r.Get("/books/{id}", require(cais.IntParam("id", books.Show)))
	r.Get("/books/{id}/cover", require(cais.IntParam("id", books.Cover)))
	r.Post("/books/{id}/retry", require(cais.IntParam("id", books.Retry)))
	r.Post("/books/{id}/pause", require(cais.IntParam("id", books.Pause)))
	r.Post("/books/{id}/chapters/{chapterID}/retry", require(books.RetryChapter))
	r.Get("/books/{id}/chapters/{chapterID}/preview", require(books.Preview))
	r.Get("/books/{id}/download", require(cais.IntParam("id", books.Download)))
	r.Get("/books/{id}/events", require(cais.IntParam("id", books.Events)))
	r.Get("/settings", require(settings.Show))
	r.Post("/settings/password", require(auth.ChangePassword))
	r.Post("/settings/test", require(settings.Test))
	r.Post("/settings/active-provider", require(settings.SelectProvider))
	r.Post("/settings/providers/{slug}", require(settings.SaveProvider))
	r.Post("/settings/providers/{slug}/activate", require(settings.ActivateProvider))
}

func registerLiveViews(hub *live.Hub, deps Deps) {
	_ = hub
	_ = deps
	// cais:live-views
}
