package app

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/puppe1990/amarra-cais/pkg/amarra/view"
	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/meta"
	"github.com/puppe1990/amarra-cais/pkg/cais/middleware"
	"github.com/puppe1990/amarra-cais/pkg/cais/session"

	appi18n "github.com/puppe1990/epub-translator/internal/i18n"
	"github.com/puppe1990/epub-translator/internal/store"
)

func testViews(t *testing.T) *view.Renderer {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			t.Fatal("go.mod not found")
		}
		wd = parent
	}
	renderer, err := view.Load(os.DirFS(filepath.Join(wd, "web", "templates")), appi18n.DefaultCatalog())
	if err != nil {
		t.Fatal(err)
	}
	return renderer
}

func newTestRouter(t *testing.T) (http.Handler, store.Store) {
	t.Helper()
	s, err := store.NewSQLiteStore(":memory:", "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	router := cais.NewRouter()
	// Produção monta esta cadeia no app.New; aqui basta a sessão para o
	// RequireAuth enxergar o cookie.
	router.Use(middleware.LoadSession(s.Sessions()))
	registerRoutes(router, Deps{
		Views:   testViews(t),
		Store:   s,
		Site:    meta.Site{AppName: "epub-translator", AppURL: "https://translate.example.com"},
		Catalog: appi18n.DefaultCatalog(),
	}, cais.Config{})
	return router, s
}

func TestRoutes_anonymousVisitorsGoToLogin(t *testing.T) {
	router, _ := newTestRouter(t)

	protected := []struct{ method, path string }{
		{http.MethodGet, "/"},
		{http.MethodGet, "/settings"},
		{http.MethodGet, "/books/1"},
		{http.MethodGet, "/books/1/cover"},
		{http.MethodGet, "/books/1/download"},
		{http.MethodPost, "/books"},
		{http.MethodPost, "/settings/password"},
	}
	for _, route := range protected {
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, httptest.NewRequest(route.method, route.path, nil))
		if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/login" {
			t.Errorf("%s %s: status = %d, location = %q", route.method, route.path, rr.Code, rr.Header().Get("Location"))
		}
	}

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/login", nil))
	if rr.Code != http.StatusOK {
		t.Errorf("GET /login: status = %d, want 200 (página pública)", rr.Code)
	}
}

func TestRoutes_signedInUserReachesLibrary(t *testing.T) {
	router, s := newTestRouter(t)

	hash, err := session.HashPassword("senha-certa-123")
	if err != nil {
		t.Fatal(err)
	}
	userID, err := s.CreateUser("dono@example.com", hash)
	if err != nil {
		t.Fatal(err)
	}
	token, err := s.Sessions().Create(userID)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: session.DefaultCookieName, Value: token})
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %.200s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "Sua biblioteca") {
		t.Error("biblioteca não renderizou para usuário logado")
	}
}
