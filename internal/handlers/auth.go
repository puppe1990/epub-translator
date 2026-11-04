package handlers

import (
	"net/http"
	"strings"

	"github.com/puppe1990/amarra-cais/pkg/amarra/view"
	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/flash"
	"github.com/puppe1990/amarra-cais/pkg/cais/httpx"
	"github.com/puppe1990/amarra-cais/pkg/cais/meta"
	"github.com/puppe1990/amarra-cais/pkg/cais/session"

	"github.com/puppe1990/epub-translator/internal/store"
)

const passwordMinLength = 8

// AuthHandler cobre entrada, saída e troca de senha. Cadastro é fechado: o
// estúdio é pessoal e as traduções gastam a chave do dono — novas contas
// nascem pelo seed de ADMIN_EMAIL ou direto no banco.
type AuthHandler struct {
	views *view.Renderer
	store store.Store
	site  meta.Site
	cfg   cais.Config
}

func NewAuthHandler(views *view.Renderer, s store.Store, site meta.Site, cfg cais.Config) *AuthHandler {
	return &AuthHandler{views: views, store: s, site: site, cfg: cfg}
}

// Login renderiza a página de entrada; quem já tem sessão vai direto à estante.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if _, ok := session.UserID(r); ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	h.renderLogin(w, r, "", http.StatusOK)
}

func (h *AuthHandler) renderLogin(w http.ResponseWriter, r *http.Request, errorMessage string, status int) {
	writeView(w, r, h.views, h.cfg, "app", "login", amarraData(r, h.site, map[string]any{
		"Title": "Entrar",
		"Error": errorMessage,
	}), status)
}

func (h *AuthHandler) LoginPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		httpx.ServerError(w, err, h.cfg)
		return
	}
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")
	user, err := h.store.FindUserByEmail(email)
	if err != nil || !session.VerifyPassword(user.PasswordHash, password) {
		h.renderLogin(w, r, "Email ou senha inválidos.", http.StatusUnprocessableEntity)
		return
	}
	if err := session.SignIn(w, h.store.Sessions(), r, user.ID, session.CookieOptionsFromConfig(h.cfg)); err != nil {
		httpx.ServerError(w, err, h.cfg)
		return
	}
	flash.Set(w, "notice", "Bem-vindo de volta ao estúdio.", h.cfg.CookieSecure())
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// LogoutPost revoga a sessão no servidor antes de limpar o cookie.
func (h *AuthHandler) LogoutPost(w http.ResponseWriter, r *http.Request) {
	if err := session.SignOut(w, h.store.Sessions(), r, session.CookieOptionsFromConfig(h.cfg)); err != nil {
		httpx.ServerError(w, err, h.cfg)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// ChangePassword troca a senha do usuário logado. Erros voltam como flash na
// página de configurações, o mesmo padrão dos formulários de provedor.
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := session.UserID(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		httpx.ServerError(w, err, h.cfg)
		return
	}
	current := r.FormValue("current_password")
	next := r.FormValue("new_password")
	confirmation := r.FormValue("password_confirmation")

	if len(next) < passwordMinLength {
		h.passwordAlert(w, r, "A nova senha precisa de pelo menos 8 caracteres.")
		return
	}
	if next != confirmation {
		h.passwordAlert(w, r, "As senhas novas não coincidem.")
		return
	}
	user, err := h.store.FindUser(userID)
	if err != nil {
		httpx.ServerError(w, err, h.cfg)
		return
	}
	if !session.VerifyPassword(user.PasswordHash, current) {
		h.passwordAlert(w, r, "A senha atual não confere.")
		return
	}
	hash, err := session.HashPassword(next)
	if err != nil {
		httpx.ServerError(w, err, h.cfg)
		return
	}
	if err := h.store.UpdatePasswordHash(userID, hash); err != nil {
		httpx.ServerError(w, err, h.cfg)
		return
	}
	flash.Set(w, "notice", "Senha atualizada.", h.cfg.CookieSecure())
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (h *AuthHandler) passwordAlert(w http.ResponseWriter, r *http.Request, message string) {
	flash.Set(w, "alert", message, h.cfg.CookieSecure())
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}
