package app

import (
	"github.com/puppe1990/amarra-cais/pkg/amarra/live"
	"github.com/puppe1990/amarra-cais/pkg/cais"

	"github.com/puppe1990/epub-translator/internal/handlers"
)

func registerRoutes(r *cais.Router, deps Deps, cfg cais.Config) {
	books := handlers.NewBooksHandler(deps.Views, deps.Store, deps.Runner, deps.Site, deps.Catalog, cfg, deps.BooksDir)
	settings := handlers.NewSettingsHandler(deps.Views, deps.Store, deps.Site, deps.Catalog, cfg)

	r.Get("/", books.Index)
	r.Post("/books", books.Upload)
	r.Get("/books/{id}", cais.IntParam("id", books.Show))
	r.Post("/books/{id}/retry", cais.IntParam("id", books.Retry))
	r.Post("/books/{id}/chapters/{chapterID}/retry", books.RetryChapter)
	r.Get("/books/{id}/download", cais.IntParam("id", books.Download))
	r.Get("/books/{id}/events", cais.IntParam("id", books.Events))
	r.Get("/settings", settings.Show)
	r.Post("/settings/test", settings.Test)
	r.Post("/settings/active-provider", settings.SelectProvider)
	r.Post("/settings/providers/{slug}", settings.SaveProvider)
	r.Post("/settings/providers/{slug}/activate", settings.ActivateProvider)
}

func registerLiveViews(hub *live.Hub, deps Deps) {
	_ = hub
	_ = deps
	// cais:live-views
}
