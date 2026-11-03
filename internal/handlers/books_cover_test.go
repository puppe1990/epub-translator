package handlers

import (
	"archive/zip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/puppe1990/epub-translator/internal/models"
)

const testCoverBytes = "\xff\xd8\xff\xe0-capa-de-teste"

// writeCoverEPUB monta um epub de teste com (ou sem) imagem de capa no
// manifest — o handler precisa servir a capa extraída do zip.
func writeCoverEPUB(t *testing.T, dir string, withCover bool) string {
	t.Helper()
	target := filepath.Join(dir, "com-capa.epub")
	file, err := os.Create(target)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	writer := zip.NewWriter(file)
	write := func(name, content string) {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	header := &zip.FileHeader{Name: "mimetype", Method: zip.Store}
	entry, _ := writer.CreateHeader(header)
	_, _ = entry.Write([]byte("application/epub+zip"))
	write("META-INF/container.xml", `<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`)

	coverItem := ""
	if withCover {
		coverItem = `<item id="cover" href="images/cover.jpg" media-type="image/jpeg" properties="cover-image"/>`
	}
	write("OEBPS/content.opf", `<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Livro Teste</dc:title><dc:creator>Autora</dc:creator><dc:language>en</dc:language></metadata><manifest>`+coverItem+`<item id="ch01" href="ch01.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="ch01"/></spine></package>`)
	write("OEBPS/ch01.xhtml", `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><head><title>Um</title></head><body><p>Hello one.</p></body></html>`)
	if withCover {
		write("OEBPS/images/cover.jpg", testCoverBytes)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return target
}

func coverRequest(bookID int64) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/books/1/cover", nil)
	req.SetPathValue("id", strconv.FormatInt(bookID, 10))
	return req
}

func TestBooksCover_servesExtractedImage(t *testing.T) {
	handler, s, _, _ := newBooksHandler(t)
	source := writeCoverEPUB(t, t.TempDir(), true)
	bookID, _ := seedStoredBook(t, s, source)

	rr := httptest.NewRecorder()
	handler.Cover(rr, coverRequest(bookID), bookID)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if contentType := rr.Header().Get("Content-Type"); contentType != "image/jpeg" {
		t.Errorf("Content-Type = %q", contentType)
	}
	if rr.Body.String() != testCoverBytes {
		t.Errorf("body = %q, want a capa extraída do zip", rr.Body.String())
	}
	if cache := rr.Header().Get("Cache-Control"); !strings.Contains(cache, "max-age") {
		t.Errorf("Cache-Control = %q, queria cache do navegador", cache)
	}
}

func TestBooksCover_withoutCoverReturnsNotFound(t *testing.T) {
	handler, s, _, _ := newBooksHandler(t)
	bookID, _ := seedStoredBook(t, s, writeTestEPUB(t, t.TempDir()))

	rr := httptest.NewRecorder()
	handler.Cover(rr, coverRequest(bookID), bookID)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, queria 404 para livro sem capa", rr.Code)
	}
}

func TestBooksCover_unknownBookReturnsNotFound(t *testing.T) {
	handler, _, _, _ := newBooksHandler(t)

	rr := httptest.NewRecorder()
	handler.Cover(rr, coverRequest(99), 99)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, queria 404 para livro inexistente", rr.Code)
	}
}

func TestBooksIndex_rendersCoverImageWhenAvailable(t *testing.T) {
	handler, s, _, _ := newBooksHandler(t)
	withCover, _ := seedStoredBook(t, s, writeCoverEPUB(t, t.TempDir(), true))
	_ = s.SetBookStatus(withCover, "done")
	withoutCover, err := s.CreateBook(models.Book{
		Title: "Sem Capa", Author: "Autora", SourceLanguage: "en",
		TargetLanguage: "pt-BR", Filename: "outra.epub", SourcePath: writeTestEPUB(t, t.TempDir()),
	})
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler.Index(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	page := rr.Body.String()

	if !strings.Contains(page, `/books/`+strconv.FormatInt(withCover, 10)+`/cover`) {
		t.Error("card sem a imagem da capa do livro com capa")
	}
	if strings.Contains(page, `/books/`+strconv.FormatInt(withoutCover, 10)+`/cover`) {
		t.Error("card apontando para capa de livro sem capa")
	}
}

func TestBooksShow_rendersCoverImageWhenAvailable(t *testing.T) {
	handler, s, _, _ := newBooksHandler(t)
	bookID, _ := seedStoredBook(t, s, writeCoverEPUB(t, t.TempDir(), true))

	rr := httptest.NewRecorder()
	handler.Show(rr, httptest.NewRequest(http.MethodGet, "/books/1", nil), bookID)
	page := rr.Body.String()

	if !strings.Contains(page, `src="/books/1/cover"`) {
		t.Error("página do livro sem a imagem da capa")
	}
}

func TestBooksShow_keepsPlaceholderWithoutCover(t *testing.T) {
	handler, s, _, _ := newBooksHandler(t)
	bookID, _ := seedStoredBook(t, s, writeTestEPUB(t, t.TempDir()))

	rr := httptest.NewRecorder()
	handler.Show(rr, httptest.NewRequest(http.MethodGet, "/books/1", nil), bookID)
	page := rr.Body.String()

	if strings.Contains(page, `src="/books/1/cover"`) {
		t.Error("página do livro apontando para capa inexistente")
	}
	if !strings.Contains(page, "detail-cover") {
		t.Error("placeholder da capa sumiu")
	}
}
