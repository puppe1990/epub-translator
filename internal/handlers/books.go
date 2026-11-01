package handlers

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/puppe1990/amarra-cais/pkg/amarra/stream"
	"github.com/puppe1990/amarra-cais/pkg/amarra/view"
	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/flash"
	"github.com/puppe1990/amarra-cais/pkg/cais/httpx"
	"github.com/puppe1990/amarra-cais/pkg/cais/i18n"
	"github.com/puppe1990/amarra-cais/pkg/cais/meta"

	"github.com/puppe1990/epub-translator/internal/epub"
	"github.com/puppe1990/epub-translator/internal/models"
	"github.com/puppe1990/epub-translator/internal/pipeline"
	"github.com/puppe1990/epub-translator/internal/store"
)

// maxUpload limita o EPUB enviado; livros reais ficam bem abaixo disso.
const maxUpload = 64 << 20

var targetLanguages = []string{"pt-BR", "en", "es", "fr", "de", "it"}

type BooksHandler struct {
	views    *view.Renderer
	store    store.Store
	runner   *pipeline.Runner
	site     meta.Site
	catalog  *i18n.Catalog
	cfg      cais.Config
	booksDir string
}

func NewBooksHandler(views *view.Renderer, s store.Store, runner *pipeline.Runner,
	site meta.Site, catalog *i18n.Catalog, cfg cais.Config, booksDir string) *BooksHandler {
	return &BooksHandler{views: views, store: s, runner: runner, site: site, catalog: catalog, cfg: cfg, booksDir: booksDir}
}

func (h *BooksHandler) Index(w http.ResponseWriter, r *http.Request) {
	books, err := h.store.ListBooks()
	if err != nil {
		httpx.ServerError(w, err, h.cfg)
		return
	}
	h.renderHome(w, r, bookViews(books), "", http.StatusOK)
}

func (h *BooksHandler) renderHome(w http.ResponseWriter, r *http.Request, books []bookView, errMsg string, status int) {
	writeView(w, r, h.views, h.cfg, "app", "home", amarraData(r, h.site, map[string]any{
		"Title":     "Traduzir EPUB",
		"ActiveNav": "home",
		"Books":     books,
		"Languages": targetLanguages,
		"Error":     errMsg,
	}), status)
}

// Upload recebe o EPUB, valida lendo o zip, cria o livro com um capítulo por
// documento XHTML e dispara a tradução — o retry depois é por capítulo.
func (h *BooksHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxUpload); err != nil {
		h.uploadError(w, r, "envio inválido: "+err.Error())
		return
	}
	file, header, err := r.FormFile("epub")
	if err != nil {
		h.uploadError(w, r, "anexe um arquivo .epub")
		return
	}
	defer func() { _ = file.Close() }()
	if header.Size > maxUpload {
		h.uploadError(w, r, "arquivo maior que 64 MB")
		return
	}

	if err := os.MkdirAll(h.booksDir, 0o755); err != nil {
		httpx.ServerError(w, err, h.cfg)
		return
	}
	temp, err := os.CreateTemp(h.booksDir, "book-*.epub")
	if err != nil {
		httpx.ServerError(w, err, h.cfg)
		return
	}
	path := temp.Name()
	if _, err := io.Copy(temp, file); err != nil {
		_ = temp.Close()
		_ = os.Remove(path)
		httpx.ServerError(w, err, h.cfg)
		return
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(path)
		httpx.ServerError(w, err, h.cfg)
		return
	}

	parsed, err := epub.Parse(path)
	if err != nil {
		_ = os.Remove(path)
		h.uploadError(w, r, "não consegui ler o EPUB: "+err.Error())
		return
	}

	title := parsed.Title
	if title == "" {
		title = header.Filename
	}
	bookID, err := h.store.CreateBook(models.Book{
		Title:          title,
		Author:         parsed.Author,
		SourceLanguage: parsed.Language,
		TargetLanguage: allowedLanguage(r.FormValue("target_language")),
		Filename:       header.Filename,
		SourcePath:     path,
	})
	if err != nil {
		_ = os.Remove(path)
		httpx.ServerError(w, err, h.cfg)
		return
	}

	chapters := make([]models.Chapter, 0, len(parsed.Documents))
	for index, doc := range parsed.Documents {
		chapters = append(chapters, models.Chapter{
			Position:   index + 1,
			DocPath:    doc.Path,
			Title:      doc.Title,
			SourceText: doc.Body,
		})
	}
	if err := h.store.CreateChapters(bookID, chapters); err != nil {
		httpx.ServerError(w, err, h.cfg)
		return
	}

	h.runner.Start(bookID)
	flash.Set(w, "notice", fmt.Sprintf("EPUB enviado com %d capítulos — tradução em andamento.", len(chapters)), h.cfg.CookieSecure())
	http.Redirect(w, r, fmt.Sprintf("/books/%d", bookID), http.StatusSeeOther)
}

func (h *BooksHandler) uploadError(w http.ResponseWriter, r *http.Request, message string) {
	books, err := h.store.ListBooks()
	if err != nil {
		httpx.ServerError(w, err, h.cfg)
		return
	}
	h.renderHome(w, r, bookViews(books), message, http.StatusUnprocessableEntity)
}

func allowedLanguage(requested string) string {
	for _, language := range targetLanguages {
		if strings.EqualFold(requested, language) {
			return language
		}
	}
	return "pt-BR"
}

func (h *BooksHandler) Show(w http.ResponseWriter, r *http.Request, id int64) {
	book, err := h.store.FindBook(id)
	if errors.Is(err, store.ErrBookNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpx.ServerError(w, err, h.cfg)
		return
	}
	chapters, err := h.store.ListChapters(id)
	if err != nil {
		httpx.ServerError(w, err, h.cfg)
		return
	}
	filter := parseChapterListFilter(r.URL.Query())
	progress, list, err := RenderBookFragments(book, chapters, filter)
	if err != nil {
		httpx.ServerError(w, err, h.cfg)
		return
	}
	actions, err := RenderBookActions(book, meta.ForRequest(h.site, r).CSRFToken)
	if err != nil {
		httpx.ServerError(w, err, h.cfg)
		return
	}

	writeView(w, r, h.views, h.cfg, "app", "book", amarraData(r, h.site, map[string]any{
		"Title":           book.Title,
		"ActiveNav":       "home",
		"Book":            newBookView(book),
		"FilterQuery":     filter.query(),
		"ProgressHTML":    template.HTML(progress),
		"ChapterListHTML": template.HTML(list),
		"ActionsHTML":     template.HTML(actions),
	}), http.StatusOK)
}

// Retry dispara a fila do livro; RetryChapter devolve o capítulo falho ou
// já traduzido para a fila antes de disparar — a chamada continua sendo por capítulo.
func (h *BooksHandler) Retry(w http.ResponseWriter, r *http.Request, id int64) {
	if _, err := h.store.FindBook(id); err != nil {
		http.NotFound(w, r)
		return
	}
	if h.runner.Start(id) {
		flash.Set(w, "notice", "Traduzindo pendentes e retentando falhas…", h.cfg.CookieSecure())
	} else {
		flash.Set(w, "notice", "Este livro já está traduzindo.", h.cfg.CookieSecure())
	}
	http.Redirect(w, r, fmt.Sprintf("/books/%d", id), http.StatusSeeOther)
}

func (h *BooksHandler) RetryChapter(w http.ResponseWriter, r *http.Request) {
	bookID, chapterID, ok := bookChapterIDs(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	chapter, err := h.store.FindChapter(chapterID)
	if err != nil || chapter.BookID != bookID {
		http.NotFound(w, r)
		return
	}
	if err := h.store.ResetChapter(chapterID); err != nil {
		httpx.ServerError(w, err, h.cfg)
		return
	}
	h.runner.StartPreferred(bookID, chapterID)
	flash.Set(w, "notice", fmt.Sprintf("Capítulo %d marcado para retry.", chapter.Position), h.cfg.CookieSecure())
	http.Redirect(w, r, fmt.Sprintf("/books/%d", bookID), http.StatusSeeOther)
}

// Download reconstrói o EPUB com os capítulos já traduzidos; o que ainda não
// foi traduzido sai no idioma original.
func (h *BooksHandler) Download(w http.ResponseWriter, r *http.Request, id int64) {
	book, err := h.store.FindBook(id)
	if errors.Is(err, store.ErrBookNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpx.ServerError(w, err, h.cfg)
		return
	}
	translations, err := h.store.ChapterTranslations(id)
	if err != nil {
		httpx.ServerError(w, err, h.cfg)
		return
	}

	var buffer bytes.Buffer
	if err := epub.Rebuild(book.SourcePath, translations, book.TargetLanguage, &buffer); err != nil {
		httpx.ServerError(w, err, h.cfg)
		return
	}
	w.Header().Set("Content-Type", "application/epub+zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, downloadFilename(book)))
	if _, err := w.Write(buffer.Bytes()); err != nil {
		return
	}
}

var slugRE = regexp.MustCompile(`[^a-z0-9]+`)

func downloadFilename(book models.Book) string {
	slug := strings.Trim(slugRE.ReplaceAllString(strings.ToLower(book.Title), "-"), "-")
	if slug == "" {
		slug = fmt.Sprintf("book-%d", book.ID)
	}
	return fmt.Sprintf("%s-%s.epub", slug, strings.ToLower(book.TargetLanguage))
}

// Events morfa #book-progress e #chapter-list enquanto a tradução roda — a
// página não precisa recarregar nem fazer polling próprio.
func (h *BooksHandler) Events(w http.ResponseWriter, r *http.Request, id int64) {
	stream.RelaySSE(w)
	var lastProgress, lastActions string
	var lastChapters []models.Chapter
	ticker := time.NewTicker(1500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			book, err := h.store.FindBook(id)
			if err != nil {
				return
			}
			chapters, err := h.store.ListChapters(id)
			if err != nil {
				continue
			}
			progress, list, err := RenderBookFragments(book, chapters, parseChapterListFilter(r.URL.Query()))
			if err != nil {
				continue
			}
			if progress != lastProgress {
				_ = stream.WriteOp(w, stream.Op{Kind: "morph", Target: "book-progress", HTML: progress})
				lastProgress = progress
			}
			if !slices.Equal(chapters, lastChapters) {
				_ = stream.WriteOp(w, stream.Op{Kind: "morph", Target: "chapter-list", HTML: list})
				lastChapters = chapters
			}
			// Only the active clock changes each tick; avoid retransmitting the
			// entire chapter table merely to advance its elapsed time.
			for _, chapter := range chapters {
				if chapter.Status == "running" && chapter.StartedAt != "" {
					_ = stream.WriteOp(w, stream.Op{Kind: "morph", Target: fmt.Sprintf("chapter-time-%d", chapter.ID), HTML: chapterDurationLabel(chapter, time.Now())})
				}
			}
			actions, err := RenderBookActions(book, meta.ForRequest(h.site, r).CSRFToken)
			if err == nil && actions != lastActions {
				_ = stream.WriteOp(w, stream.Op{Kind: "morph", Target: "book-actions", HTML: actions})
				lastActions = actions
			}
			_ = stream.Flush(w)
		}
	}
}
