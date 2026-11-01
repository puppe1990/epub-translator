package handlers

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"github.com/puppe1990/amarra-cais/pkg/cais/httpx"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/puppe1990/epub-translator/internal/models"
	"github.com/puppe1990/epub-translator/internal/store"
)

// chapterPreviewPolicy strips script/style/handlers from EPUB bodies before
// they land in the modal. The upload is the user's own book, but the page
// still must not execute markup from the zip.
var chapterPreviewPolicy = bluemonday.UGCPolicy()

var chapterPreviewTemplate = template.Must(template.New("chapter-preview").Parse(`
<article class="chapter-preview-spread" aria-label="Preview do capítulo {{ .Position }}">
  <header class="preview-spread-heading">
    <p class="eyebrow">CAPÍTULO {{ printf "%02d" .Position }}</p>
    <h2>{{ .Title }}</h2>
  </header>
  <div class="preview-pages">
    <section class="preview-page preview-verso" aria-labelledby="preview-original-label">
      <p id="preview-original-label" class="preview-leaf">Original</p>
      <p class="preview-lang">{{ .SourceLanguage }}</p>
      <div class="preview-prose">{{ .SourceHTML }}</div>
    </section>
    <section class="preview-page preview-recto" aria-labelledby="preview-translated-label">
      <p id="preview-translated-label" class="preview-leaf">Traduzido</p>
      <p class="preview-lang">{{ .TargetLanguage }}</p>
      {{ if .HasTranslation }}
      <div class="preview-prose">{{ .TranslatedHTML }}</div>
      {{ else }}
      <p class="preview-empty">Ainda sem tradução.</p>
      {{ end }}
    </section>
  </div>
</article>
`))

type chapterPreviewView struct {
	Position       int
	Title          string
	SourceLanguage string
	TargetLanguage string
	SourceHTML     template.HTML
	TranslatedHTML template.HTML
	HasTranslation bool
}

func previewHTML(raw string) template.HTML {
	return template.HTML(serializePreviewFragment(chapterPreviewPolicy.Sanitize(raw)))
}

// serializePreviewFragment re-parses each pane alone so XHTML empty anchors
// (`<a id="page_3"/>`) get an end tag. In HTML5 those are not void; an open
// <a> is a formatting element and the parser clones it as a sibling of the
// two preview columns, which steals a grid cell and hides the translation.
func serializePreviewFragment(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	nodes, err := html.ParseFragment(strings.NewReader(raw), &html.Node{
		Type:     html.ElementNode,
		Data:     "div",
		DataAtom: atom.Div,
	})
	if err != nil {
		return raw
	}
	var builder strings.Builder
	for _, node := range nodes {
		if err := html.Render(&builder, node); err != nil {
			return raw
		}
	}
	return builder.String()
}

func renderChapterPreview(book models.Book, chapter models.Chapter) (string, error) {
	var builder strings.Builder
	view := chapterPreviewView{
		Position:       chapter.Position,
		Title:          chapter.Title,
		SourceLanguage: book.SourceLanguage,
		TargetLanguage: book.TargetLanguage,
		SourceHTML:     previewHTML(chapter.SourceText),
		TranslatedHTML: previewHTML(chapter.TranslatedText),
		HasTranslation: strings.TrimSpace(chapter.TranslatedText) != "",
	}
	if err := chapterPreviewTemplate.Execute(&builder, view); err != nil {
		return "", fmt.Errorf("renderizar preview do capítulo %d: %w", chapter.ID, err)
	}
	return builder.String(), nil
}

func bookChapterIDs(r *http.Request) (bookID, chapterID int64, ok bool) {
	bookID, errBook := strconv.ParseInt(r.PathValue("id"), 10, 64)
	chapterID, errChapter := strconv.ParseInt(r.PathValue("chapterID"), 10, 64)
	if errBook != nil || errChapter != nil || bookID <= 0 || chapterID <= 0 {
		return 0, 0, false
	}
	return bookID, chapterID, true
}

// Preview mostra o original e a tradução lado a lado. A listagem não carrega
// os corpos; esta rota usa FindChapter. O header X-Chapter-Preview: dialog
// devolve só o spread para a <dialog> da página do livro.
func (h *BooksHandler) Preview(w http.ResponseWriter, r *http.Request) {
	bookID, chapterID, ok := bookChapterIDs(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	book, err := h.store.FindBook(bookID)
	if errors.Is(err, store.ErrBookNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpx.ServerError(w, err, h.cfg)
		return
	}
	chapter, err := h.store.FindChapter(chapterID)
	if err != nil || chapter.BookID != bookID {
		http.NotFound(w, r)
		return
	}
	spread, err := renderChapterPreview(book, chapter)
	if err != nil {
		httpx.ServerError(w, err, h.cfg)
		return
	}
	if r.Header.Get("X-Chapter-Preview") == "dialog" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(spread))
		return
	}
	writeView(w, r, h.views, h.cfg, "app", "chapter-preview", amarraData(r, h.site, map[string]any{
		"Title":      fmt.Sprintf("%s · Preview", chapter.Title),
		"ActiveNav":  "home",
		"Book":       newBookView(book),
		"SpreadHTML": template.HTML(spread),
	}), http.StatusOK)
}
