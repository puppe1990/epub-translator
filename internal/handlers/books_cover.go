package handlers

import (
	"errors"
	"net/http"

	"github.com/puppe1990/amarra-cais/pkg/cais/httpx"

	"github.com/puppe1990/epub-translator/internal/epub"
	"github.com/puppe1990/epub-translator/internal/models"
	"github.com/puppe1990/epub-translator/internal/store"
)

// Cover serve a imagem de capa extraída do EPUB original (data/books). Sem
// capa — ou com o arquivo sumido — a página cai no placeholder verde.
func (h *BooksHandler) Cover(w http.ResponseWriter, r *http.Request, id int64) {
	book, err := h.store.FindBook(id)
	if errors.Is(err, store.ErrBookNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpx.ServerError(w, err, h.cfg)
		return
	}
	data, mime, err := epub.Cover(book.SourcePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(data)
}

func withCoverInfo(view bookView) bookView {
	view.HasCover = epub.HasCover(view.SourcePath)
	return view
}

func bookViewsWithCovers(books []models.Book) []bookView {
	views := bookViews(books)
	for index := range views {
		views[index] = withCoverInfo(views[index])
	}
	return views
}
