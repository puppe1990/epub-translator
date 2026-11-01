package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/puppe1990/amarra-cais/pkg/cais/flash"
	"github.com/puppe1990/amarra-cais/pkg/cais/httpx"

	"github.com/puppe1990/epub-translator/internal/store"
)

func (h *BooksHandler) Pause(w http.ResponseWriter, r *http.Request, id int64) {
	if _, err := h.store.FindBook(id); err != nil {
		if errors.Is(err, store.ErrBookNotFound) {
			http.NotFound(w, r)
		} else {
			httpx.ServerError(w, err, h.cfg)
		}
		return
	}
	stopped, err := h.runner.Pause(id)
	if err == nil && !stopped {
		err = h.store.PauseBook(id)
	}
	if err != nil {
		httpx.ServerError(w, err, h.cfg)
		return
	}
	book, err := h.store.FindBook(id)
	if err != nil {
		httpx.ServerError(w, err, h.cfg)
		return
	}
	message := "Tradução pausada. Seus capítulos concluídos foram preservados."
	if book.Status == "done" {
		message = "A tradução já foi concluída. Seu EPUB está pronto para baixar."
	}
	flash.Set(w, "notice", message, h.cfg.CookieSecure())
	http.Redirect(w, r, fmt.Sprintf("/books/%d", id), http.StatusSeeOther)
}
