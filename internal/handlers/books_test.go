package handlers

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/puppe1990/amarra-cais/pkg/cais"

	"github.com/puppe1990/epub-translator/internal/models"
	"github.com/puppe1990/epub-translator/internal/pipeline"
	"github.com/puppe1990/epub-translator/internal/store"
)

// --- fixtures ---------------------------------------------------------------

// writeTestEPUB monta um epub pequeno com dois documentos XHTML.
func writeTestEPUB(t *testing.T, dir string) string {
	t.Helper()
	target := filepath.Join(dir, "origem.epub")
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
	write("OEBPS/content.opf", `<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Livro Teste</dc:title><dc:creator>Autora</dc:creator><dc:language>en</dc:language></metadata><manifest><item id="ch01" href="ch01.xhtml" media-type="application/xhtml+xml"/><item id="ch02" href="ch02.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="ch01"/><itemref idref="ch02"/></spine></package>`)
	write("OEBPS/ch01.xhtml", `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><head><title>Um</title></head><body><p>Hello one.</p></body></html>`)
	write("OEBPS/ch02.xhtml", `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><head><title>Dois</title></head><body><p>Hello two.</p></body></html>`)
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return target
}

type fakeTranslator struct{}

func (fakeTranslator) TranslateBody(_ context.Context, _, body string) (string, error) {
	return strings.NewReplacer("Hello", "Olá").Replace(body), nil
}

func newBooksHandler(t *testing.T) (*BooksHandler, store.Store, *pipeline.Runner, string) {
	t.Helper()
	s := setupTestStore(t)
	runner := pipeline.NewRunner(s, fakeTranslator{})
	booksDir := t.TempDir()
	handler := NewBooksHandler(setupTestViews(t), s, runner, testSite(), nil, cais.Config{}, booksDir)
	return handler, s, runner, booksDir
}

func multipartEPUB(t *testing.T, path string) (io.Reader, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("epub", "livro.epub")
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	_ = writer.WriteField("target_language", "pt-BR")
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return &body, writer.FormDataContentType()
}

// --- upload -----------------------------------------------------------------

func TestBooksUpload_createsBookWithChaptersAndTranslates(t *testing.T) {
	handler, s, runner, booksDir := newBooksHandler(t)
	source := writeTestEPUB(t, t.TempDir())

	body, contentType := multipartEPUB(t, source)
	req := httptest.NewRequest(http.MethodPost, "/books", body)
	req.Header.Set("Content-Type", contentType)
	rr := httptest.NewRecorder()
	handler.Upload(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if location := rr.Header().Get("Location"); location != "/books/1" {
		t.Errorf("Location = %q", location)
	}

	book, err := s.FindBook(1)
	if err != nil {
		t.Fatal(err)
	}
	if book.Title != "Livro Teste" || book.TargetLanguage != "pt-BR" || book.TotalChapters != 2 {
		t.Errorf("book = %+v", book)
	}
	if !strings.HasPrefix(book.SourcePath, booksDir) {
		t.Errorf("source guardado fora de %s: %q", booksDir, book.SourcePath)
	}

	runner.Wait(1)
	chapters, _ := s.ListChapters(1)
	for _, chapter := range chapters {
		if chapter.Status != "done" {
			t.Errorf("capítulo %d = %+v", chapter.Position, chapter)
		}
	}
	book, _ = s.FindBook(1)
	if book.Status != "done" {
		t.Errorf("status do livro = %q", book.Status)
	}
}

func TestBooksUpload_rejectsFileThatIsNotEPUB(t *testing.T) {
	handler, s, _, _ := newBooksHandler(t)

	garbage := filepath.Join(t.TempDir(), "nao-epub.epub")
	if err := os.WriteFile(garbage, []byte("isto não é um epub"), 0o644); err != nil {
		t.Fatal(err)
	}
	body, contentType := multipartEPUB(t, garbage)
	req := httptest.NewRequest(http.MethodPost, "/books", body)
	req.Header.Set("Content-Type", contentType)
	rr := httptest.NewRecorder()
	handler.Upload(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "não consegui ler o EPUB") {
		t.Error("erro sem mensagem clara para o usuário")
	}
	books, _ := s.ListBooks()
	if len(books) != 0 {
		t.Errorf("livro criado mesmo com upload inválido: %+v", books)
	}
}

// --- página do livro, retry e download --------------------------------------

func seedStoredBook(t *testing.T, s store.Store, sourcePath string) (int64, []models.Chapter) {
	t.Helper()
	bookID, err := s.CreateBook(models.Book{
		Title: "Livro Teste", Author: "Autora", SourceLanguage: "en",
		TargetLanguage: "pt-BR", Filename: "livro.epub", SourcePath: sourcePath,
	})
	if err != nil {
		t.Fatal(err)
	}
	chapters := []models.Chapter{
		{Position: 1, DocPath: "OEBPS/ch01.xhtml", Title: "Um", SourceText: "<p>Hello one.</p>"},
		{Position: 2, DocPath: "OEBPS/ch02.xhtml", Title: "Dois", SourceText: "<p>Hello two.</p>"},
	}
	if err := s.CreateChapters(bookID, chapters); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListChapters(bookID)
	if err != nil {
		t.Fatal(err)
	}
	return bookID, list
}

func TestBooksShow_rendersProgressChaptersAndStream(t *testing.T) {
	handler, s, _, _ := newBooksHandler(t)
	bookID, chapters := seedStoredBook(t, s, "")
	_ = s.SetChapterDone(chapters[0].ID, "<p>Olá um.</p>")
	_ = s.SetChapterFailed(chapters[1].ID, "provedor respondeu 429")
	_ = s.SetBookStatus(bookID, "failed")

	req := httptest.NewRequest(http.MethodGet, "/books/1", nil)
	rr := httptest.NewRecorder()
	handler.Show(rr, req, bookID)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	page := rr.Body.String()
	for _, want := range []string{
		"Livro Teste",
		"Capítulo",
		"Caracteres",
		`id="book-progress"`,
		`id="chapter-list"`,
		`data-amarra-stream="/books/1/events"`,
		"Traduzido",
		"Falhou",
		"provedor respondeu 429",
		"Retentar",
		"/books/1/chapters/" + strconv.FormatInt(chapters[1].ID, 10) + "/retry",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("página sem %q", want)
		}
	}

	missing := httptest.NewRecorder()
	handler.Show(missing, httptest.NewRequest(http.MethodGet, "/books/99", nil), 99)
	if missing.Code != http.StatusNotFound {
		t.Errorf("livro inexistente: status = %d", missing.Code)
	}
}

type orderTranslator struct {
	mu    sync.Mutex
	order []string
}

func (o *orderTranslator) TranslateBody(_ context.Context, _, body string) (string, error) {
	o.mu.Lock()
	o.order = append(o.order, body)
	o.mu.Unlock()
	return strings.NewReplacer("Hello", "Olá").Replace(body), nil
}

func TestBooksRetryChapter_translatesClickedChapterBeforeEarlierPending(t *testing.T) {
	s := setupTestStore(t)
	translator := &orderTranslator{}
	runner := pipeline.NewRunner(s, translator)
	handler := NewBooksHandler(setupTestViews(t), s, runner, testSite(), nil, cais.Config{}, t.TempDir())

	bookID, err := s.CreateBook(models.Book{Title: "Livro", TargetLanguage: "pt-BR"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateChapters(bookID, []models.Chapter{
		{Position: 1, DocPath: "ch01.xhtml", Title: "Um", SourceText: "<p>Hello one</p>"},
		{Position: 2, DocPath: "ch02.xhtml", Title: "Dois", SourceText: "<p>Hello two</p>"},
		{Position: 3, DocPath: "ch03.xhtml", Title: "Três", SourceText: "<p>Hello three</p>"},
	}); err != nil {
		t.Fatal(err)
	}
	chapters, _ := s.ListChapters(bookID)
	if err := s.SetChapterDone(chapters[0].ID, "<p>Olá one</p>"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetChapterFailed(chapters[2].ID, "429"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/books/1/chapters/3/retry", nil)
	req.SetPathValue("id", strconv.FormatInt(bookID, 10))
	req.SetPathValue("chapterID", strconv.FormatInt(chapters[2].ID, 10))
	handler.RetryChapter(httptest.NewRecorder(), req)
	runner.Wait(bookID)

	translator.mu.Lock()
	order := append([]string{}, translator.order...)
	translator.mu.Unlock()
	if len(order) == 0 || order[0] != "<p>Hello three</p>" {
		t.Fatalf("Retentar da linha traduziu %v, queria o capítulo 3 primeiro", order)
	}
}

func TestBooksRetryChapter_putsFailedBackInQueue(t *testing.T) {
	handler, s, runner, _ := newBooksHandler(t)
	bookID, chapters := seedStoredBook(t, s, "")
	_ = s.SetChapterFailed(chapters[1].ID, "429")

	req := httptest.NewRequest(http.MethodPost, "/books/1/chapters/2/retry", nil)
	req.SetPathValue("id", "1")
	req.SetPathValue("chapterID", strconv.FormatInt(chapters[1].ID, 10))
	rr := httptest.NewRecorder()
	handler.RetryChapter(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	runner.Wait(bookID)
	book, _ := s.FindBook(bookID)
	if book.Status != "done" || book.DoneChapters != 2 {
		t.Errorf("book = %+v", book)
	}
	updated, _ := s.FindChapter(chapters[1].ID)
	if updated.Attempts != 1 {
		t.Errorf("attempts = %d (o histórico da falha deve ficar)", updated.Attempts)
	}
}

func TestBooksDownload_servesRebuiltEPUB(t *testing.T) {
	handler, s, _, _ := newBooksHandler(t)
	source := writeTestEPUB(t, t.TempDir())
	_, chapters := seedStoredBook(t, s, source)
	_ = s.SetChapterDone(chapters[0].ID, "<p>Olá um, traduzido.</p>")

	req := httptest.NewRequest(http.MethodGet, "/books/1/download", nil)
	rr := httptest.NewRecorder()
	handler.Download(rr, req, 1)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	if contentType := rr.Header().Get("Content-Type"); contentType != "application/epub+zip" {
		t.Errorf("Content-Type = %q", contentType)
	}
	if disposition := rr.Header().Get("Content-Disposition"); !strings.Contains(disposition, "livro-teste-pt-br.epub") {
		t.Errorf("Content-Disposition = %q", disposition)
	}

	reader, err := zip.NewReader(bytes.NewReader(rr.Body.Bytes()), int64(rr.Body.Len()))
	if err != nil {
		t.Fatalf("resposta não é um zip: %v", err)
	}
	var ch01 string
	for _, file := range reader.File {
		if file.Name == "OEBPS/ch01.xhtml" {
			handle, _ := file.Open()
			content, _ := io.ReadAll(handle)
			_ = handle.Close()
			ch01 = string(content)
		}
	}
	if !strings.Contains(ch01, "Olá um, traduzido.") {
		t.Errorf("ch01 sem tradução: %q", ch01)
	}
}

func TestBooksEvents_streamsSSE(t *testing.T) {
	handler, s, _, _ := newBooksHandler(t)
	seedStoredBook(t, s, "")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, "/books/1/events", nil).WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.Events(rr, req, 1)

	if contentType := rr.Header().Get("Content-Type"); !strings.Contains(contentType, "text/event-stream") {
		t.Errorf("Content-Type = %q", contentType)
	}
}
