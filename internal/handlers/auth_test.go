package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/session"

	"github.com/puppe1990/epub-translator/internal/store"
)

func newAuthHandler(t *testing.T) (*AuthHandler, store.Store) {
	t.Helper()
	s := setupTestStore(t)
	handler := NewAuthHandler(setupTestViews(t), s, testSite(), cais.Config{})
	return handler, s
}

func seedUser(t *testing.T, s store.Store, email, password string) int64 {
	t.Helper()
	hash, err := session.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	userID, err := s.CreateUser(email, hash)
	if err != nil {
		t.Fatal(err)
	}
	return userID
}

func cookieValue(rr *httptest.ResponseRecorder, name string) string {
	for _, cookie := range rr.Result().Cookies() {
		if cookie.Name == name && cookie.Value != "" {
			return cookie.Value
		}
	}
	return ""
}

func TestLogin_getRendersForm(t *testing.T) {
	handler, _ := newAuthHandler(t)

	rr := httptest.NewRecorder()
	handler.Login(rr, httptest.NewRequest(http.MethodGet, "/login", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	page := rr.Body.String()
	for _, want := range []string{`action="/login"`, `name="email"`, `name="password"`, "Entrar"} {
		if !strings.Contains(page, want) {
			t.Errorf("página de login sem %q", want)
		}
	}
}

func TestLoginPost_rejectsInvalidCredentials(t *testing.T) {
	handler, s := newAuthHandler(t)
	seedUser(t, s, "dono@example.com", "senha-certa-123")

	form := strings.NewReader("email=dono@example.com&password=errada")
	req := httptest.NewRequest(http.MethodPost, "/login", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.LoginPost(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Email ou senha inválidos") {
		t.Error("erro sem mensagem clara")
	}
	if token := cookieValue(rr, session.DefaultCookieName); token != "" {
		t.Errorf("sessão criada com credencial inválida: %q", token)
	}
}

func TestLoginPost_signsInAndCreatesServerSession(t *testing.T) {
	handler, s := newAuthHandler(t)
	userID := seedUser(t, s, "dono@example.com", "senha-certa-123")

	form := strings.NewReader("email=dono@example.com&password=senha-certa-123")
	req := httptest.NewRequest(http.MethodPost, "/login", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.LoginPost(rr, req)

	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/" {
		t.Fatalf("status = %d, location = %q", rr.Code, rr.Header().Get("Location"))
	}
	token := cookieValue(rr, session.DefaultCookieName)
	if token == "" {
		t.Fatal("sem cookie de sessão após login")
	}
	signedID, ok := s.Sessions().Get(token)
	if !ok || signedID != userID {
		t.Errorf("sessão = %d/%v, want user %d", signedID, ok, userID)
	}
}

func TestLogin_getRedirectsWhenSignedIn(t *testing.T) {
	handler, _ := newAuthHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	req = session.WithUserID(req, 7)
	rr := httptest.NewRecorder()
	handler.Login(rr, req)

	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/" {
		t.Fatalf("status = %d, location = %q", rr.Code, rr.Header().Get("Location"))
	}
}

func TestLogoutPost_revokesSession(t *testing.T) {
	handler, s := newAuthHandler(t)
	userID := seedUser(t, s, "dono@example.com", "senha-certa-123")
	token, err := s.Sessions().Create(userID)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.AddCookie(&http.Cookie{Name: session.DefaultCookieName, Value: token})
	rr := httptest.NewRecorder()
	handler.LogoutPost(rr, req)

	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/login" {
		t.Fatalf("status = %d, location = %q", rr.Code, rr.Header().Get("Location"))
	}
	if _, ok := s.Sessions().Get(token); ok {
		t.Error("sessão ainda válida depois do logout")
	}
}

func changePasswordRequest(userID int64, current, next, confirmation string) *http.Request {
	form := strings.NewReader("current_password=" + current + "&new_password=" + next + "&password_confirmation=" + confirmation)
	req := httptest.NewRequest(http.MethodPost, "/settings/password", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return session.WithUserID(req, userID)
}

func TestChangePassword_rejectsWrongCurrentPassword(t *testing.T) {
	handler, s := newAuthHandler(t)
	userID := seedUser(t, s, "dono@example.com", "senha-atual-123")

	rr := httptest.NewRecorder()
	handler.ChangePassword(rr, changePasswordRequest(userID, "errada", "nova-senha-456", "nova-senha-456"))

	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/settings" {
		t.Fatalf("status = %d, location = %q", rr.Code, rr.Header().Get("Location"))
	}
	user, err := s.FindUser(userID)
	if err != nil {
		t.Fatal(err)
	}
	if !session.VerifyPassword(user.PasswordHash, "senha-atual-123") {
		t.Error("senha mudou com senha atual errada")
	}
}

func TestChangePassword_rejectsMismatchAndShort(t *testing.T) {
	handler, s := newAuthHandler(t)
	userID := seedUser(t, s, "dono@example.com", "senha-atual-123")

	rr := httptest.NewRecorder()
	handler.ChangePassword(rr, changePasswordRequest(userID, "senha-atual-123", "curta", "curta"))
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("curta: status = %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	handler.ChangePassword(rr, changePasswordRequest(userID, "senha-atual-123", "nova-senha-456", "outra-senha-789"))
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("mismatch: status = %d", rr.Code)
	}

	user, err := s.FindUser(userID)
	if err != nil {
		t.Fatal(err)
	}
	if !session.VerifyPassword(user.PasswordHash, "senha-atual-123") {
		t.Error("senha mudou em validação inválida")
	}
}

func TestChangePassword_updatesHash(t *testing.T) {
	handler, s := newAuthHandler(t)
	userID := seedUser(t, s, "dono@example.com", "senha-atual-123")

	rr := httptest.NewRecorder()
	handler.ChangePassword(rr, changePasswordRequest(userID, "senha-atual-123", "nova-senha-456", "nova-senha-456"))

	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/settings" {
		t.Fatalf("status = %d, location = %q", rr.Code, rr.Header().Get("Location"))
	}
	user, err := s.FindUser(userID)
	if err != nil {
		t.Fatal(err)
	}
	if !session.VerifyPassword(user.PasswordHash, "nova-senha-456") {
		t.Error("senha não foi atualizada")
	}
}
