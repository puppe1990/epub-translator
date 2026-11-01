package handlers

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/net/html"

	"github.com/puppe1990/epub-translator/internal/models"
)

func TestBooksShow_hasPreviewLinkForEachChapterWithoutBodies(t *testing.T) {
	handler, s, _, _ := newBooksHandler(t)
	bookID, chapters := seedStoredBook(t, s, "")
	_ = s.SetChapterDone(chapters[0].ID, "<p>Olá um.</p>")

	req := httptest.NewRequest(http.MethodGet, "/books/1", nil)
	rr := httptest.NewRecorder()
	handler.Show(rr, req, bookID)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	page := rr.Body.String()
	for _, chapter := range chapters {
		href := "/books/1/chapters/" + strconv.FormatInt(chapter.ID, 10) + "/preview"
		if !strings.Contains(page, href) {
			t.Errorf("página sem Preview %q", href)
		}
	}
	if !strings.Contains(page, "Preview") {
		t.Error("página sem o botão Preview")
	}
	if strings.Contains(page, "Hello one.") {
		t.Error("a listagem não deve embutir o corpo do capítulo")
	}
}

func TestBooksPreview_rendersOriginalAndTranslatedSideBySide(t *testing.T) {
	handler, s, _, _ := newBooksHandler(t)
	bookID, chapters := seedStoredBook(t, s, "")
	_ = s.SetChapterDone(chapters[0].ID, "<p>Olá um.</p>")

	req := httptest.NewRequest(http.MethodGet, "/books/1/chapters/1/preview", nil)
	req.SetPathValue("id", strconv.FormatInt(bookID, 10))
	req.SetPathValue("chapterID", strconv.FormatInt(chapters[0].ID, 10))
	rr := httptest.NewRecorder()
	handler.Preview(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	page := rr.Body.String()
	for _, want := range []string{
		"Original",
		"Traduzido",
		"Hello one.",
		"Olá um.",
		"Um",
		`class="chapter-preview-spread"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("preview sem %q", want)
		}
	}
}

func TestBooksPreview_showsPlaceholderWhenTranslationIsMissing(t *testing.T) {
	handler, s, _, _ := newBooksHandler(t)
	bookID, chapters := seedStoredBook(t, s, "")

	req := httptest.NewRequest(http.MethodGet, "/books/1/chapters/2/preview", nil)
	req.SetPathValue("id", strconv.FormatInt(bookID, 10))
	req.SetPathValue("chapterID", strconv.FormatInt(chapters[1].ID, 10))
	rr := httptest.NewRecorder()
	handler.Preview(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	page := rr.Body.String()
	if !strings.Contains(page, "Hello two.") {
		t.Error("preview sem o original do capítulo pendente")
	}
	if !strings.Contains(page, "Ainda sem tradução.") {
		t.Error("preview sem o placeholder da tradução vazia")
	}
}

func TestBooksPreview_notFoundWhenChapterBelongsToAnotherBook(t *testing.T) {
	handler, s, _, _ := newBooksHandler(t)
	firstID, firstChapters := seedStoredBook(t, s, "")
	secondID, err := s.CreateBook(models.Book{
		Title: "Outro", TargetLanguage: "pt-BR", Filename: "outro.epub",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateChapters(secondID, []models.Chapter{
		{Position: 1, DocPath: "ch01.xhtml", Title: "X", SourceText: "<p>Secret</p>"},
	}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/books/1/chapters/99/preview", nil)
	req.SetPathValue("id", strconv.FormatInt(firstID, 10))
	req.SetPathValue("chapterID", strconv.FormatInt(firstChapters[0].ID+99, 10))
	rr := httptest.NewRecorder()
	handler.Preview(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("capítulo inexistente: status = %d", rr.Code)
	}

	other, err := s.ListChapters(secondID)
	if err != nil {
		t.Fatal(err)
	}
	cross := httptest.NewRequest(http.MethodGet, "/books/1/chapters/3/preview", nil)
	cross.SetPathValue("id", strconv.FormatInt(firstID, 10))
	cross.SetPathValue("chapterID", strconv.FormatInt(other[0].ID, 10))
	denied := httptest.NewRecorder()
	handler.Preview(denied, cross)
	if denied.Code != http.StatusNotFound {
		t.Errorf("capítulo de outro livro: status = %d", denied.Code)
	}
	if strings.Contains(denied.Body.String(), "Secret") {
		t.Error("não deve vazar o corpo de outro livro")
	}
}

func TestBooksPreview_dialogHeaderReturnsSpreadWithoutStudioChrome(t *testing.T) {
	handler, s, _, _ := newBooksHandler(t)
	bookID, chapters := seedStoredBook(t, s, "")
	_ = s.SetChapterDone(chapters[0].ID, "<p>Olá um.</p>")

	req := httptest.NewRequest(http.MethodGet, "/books/1/chapters/1/preview", nil)
	req.SetPathValue("id", strconv.FormatInt(bookID, 10))
	req.SetPathValue("chapterID", strconv.FormatInt(chapters[0].ID, 10))
	req.Header.Set("X-Chapter-Preview", "dialog")
	rr := httptest.NewRecorder()
	handler.Preview(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	page := rr.Body.String()
	if !strings.Contains(page, `class="chapter-preview-spread"`) {
		t.Error("fragmento da modal sem o spread")
	}
	if !strings.Contains(page, "Hello one.") || !strings.Contains(page, "Olá um.") {
		t.Error("fragmento da modal sem original e traduzido")
	}
	for _, chrome := range []string{"studio-sidebar", "Voltar à biblioteca", "id=\"amarra-main\""} {
		if strings.Contains(page, chrome) {
			t.Errorf("fragmento da modal ainda tem o chrome %q", chrome)
		}
	}
}

func TestBooksPreview_rendersSafeChapterMarkupAsHTML(t *testing.T) {
	handler, s, _, _ := newBooksHandler(t)
	bookID, chapters := seedStoredBook(t, s, "")
	_ = s.SetChapterDone(chapters[0].ID, "<p>Olá um.</p>")

	req := httptest.NewRequest(http.MethodGet, "/books/1/chapters/1/preview", nil)
	req.SetPathValue("id", strconv.FormatInt(bookID, 10))
	req.SetPathValue("chapterID", strconv.FormatInt(chapters[0].ID, 10))
	rr := httptest.NewRecorder()
	handler.Preview(rr, req)

	page := rr.Body.String()
	if !strings.Contains(page, "<p>Hello one.</p>") {
		t.Error("preview deve renderizar o XHTML original")
	}
	if !strings.Contains(page, "<p>Olá um.</p>") {
		t.Error("preview deve renderizar o XHTML traduzido")
	}
	if strings.Contains(page, "&lt;p&gt;Hello one.") {
		t.Error("XHTML seguro não deve aparecer escapado")
	}
}

func TestBooksPreview_escapesUnsafeMarkupFromTheEPUB(t *testing.T) {
	handler, s, _, _ := newBooksHandler(t)
	bookID, err := s.CreateBook(models.Book{
		Title: "Livro", TargetLanguage: "pt-BR", Filename: "xss.epub",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateChapters(bookID, []models.Chapter{
		{Position: 1, DocPath: "ch01.xhtml", Title: "Um", SourceText: `<p>Hello</p><script>alert(1)</script>`},
	}); err != nil {
		t.Fatal(err)
	}
	chapters, _ := s.ListChapters(bookID)
	_ = s.SetChapterDone(chapters[0].ID, `<p>Olá</p><img src=x onerror="alert(1)">`)

	req := httptest.NewRequest(http.MethodGet, "/books/1/chapters/1/preview", nil)
	req.SetPathValue("id", strconv.FormatInt(bookID, 10))
	req.SetPathValue("chapterID", strconv.FormatInt(chapters[0].ID, 10))
	rr := httptest.NewRecorder()
	handler.Preview(rr, req)

	page := rr.Body.String()
	if strings.Contains(page, "<script>alert(1)</script>") {
		t.Error("script do EPUB não foi escapado")
	}
	if strings.Contains(page, `onerror="alert(1)"`) {
		t.Error("handler do EPUB não foi escapado")
	}
	if !strings.Contains(page, "Hello") || !strings.Contains(page, "Olá") {
		t.Error("preview perdeu o texto depois de escapar")
	}
}

func TestBooksShow_keepsPreviewDialogOutsideChapterList(t *testing.T) {
	handler, s, _, _ := newBooksHandler(t)
	bookID, _ := seedStoredBook(t, s, "")

	req := httptest.NewRequest(http.MethodGet, "/books/1", nil)
	rr := httptest.NewRecorder()
	handler.Show(rr, req, bookID)
	page := rr.Body.String()

	if !strings.Contains(page, `id="chapter-preview-dialog"`) {
		t.Error("página do livro sem a modal de preview")
	}
	listStart := strings.Index(page, `id="chapter-list"`)
	listEnd := strings.Index(page[listStart:], "</div>")
	if listStart < 0 || listEnd < 0 {
		t.Fatal("não achei #chapter-list")
	}
	listHTML := page[listStart : listStart+listEnd]
	if strings.Contains(listHTML, `id="chapter-preview-dialog"`) {
		t.Error("a modal não pode ficar dentro de #chapter-list — o SSE apagaria ela")
	}
}

func TestRenderChapterPreview_emptyPageAnchorsStayInsideEachColumn(t *testing.T) {
	spread, err := renderChapterPreview(
		models.Book{SourceLanguage: "en", TargetLanguage: "pt-BR"},
		models.Chapter{
			Position:       6,
			Title:          "Total Recall",
			SourceText:     `<p>Hello original.<a id="page_20"/></p>`,
			TranslatedText: `<p>Olá traduzido.<a id="page_20"/></p>`,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	kids := previewGridItems(t, spread)
	if len(kids) != 2 {
		t.Fatalf("preview-pages children = %v, âncora vazia do EPUB vazou para o grid", kids)
	}
	if kids[0] != "section.preview-page preview-verso" || kids[1] != "section.preview-page preview-recto" {
		t.Fatalf("colunas = %v", kids)
	}
	if strings.Contains(spread, `<a id="page_20"/>`) {
		t.Fatal("âncora ainda está self-closing; o HTML5 deixa o <a> aberto")
	}
}

func previewGridItems(t *testing.T, spread string) []string {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(spread))
	if err != nil {
		t.Fatal(err)
	}
	pages := findHTMLClass(doc, "preview-pages")
	if pages == nil {
		t.Fatal("spread sem .preview-pages")
	}
	var kids []string
	for child := pages.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}
		kids = append(kids, child.Data+classAttr(child))
	}
	return kids
}

func findHTMLClass(n *html.Node, class string) *html.Node {
	if n.Type == html.ElementNode && hasClass(n, class) {
		return n
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if found := findHTMLClass(child, class); found != nil {
			return found
		}
	}
	return nil
}

func hasClass(n *html.Node, class string) bool {
	for _, attr := range n.Attr {
		if attr.Key == "class" {
			for _, item := range strings.Fields(attr.Val) {
				if item == class {
					return true
				}
			}
		}
	}
	return false
}

func classAttr(n *html.Node) string {
	for _, attr := range n.Attr {
		if attr.Key == "class" && attr.Val != "" {
			return "." + attr.Val
		}
	}
	return ""
}
