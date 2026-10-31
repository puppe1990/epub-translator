package epub

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFixture monta um epub mínimo: capa implícita via spine com dois
// capítulos, nav fora do spine, css e mimetype sem compressão.
func writeFixture(t *testing.T, dir string) string {
	t.Helper()
	target := filepath.Join(dir, "fixture.epub")
	file, err := os.Create(target)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()

	writer := zip.NewWriter(file)
	writeStored := func(name, content string) {
		header := &zip.FileHeader{Name: name, Method: zip.Store}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	write := func(name, content string) {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}

	writeStored("mimetype", "application/epub+zip")
	write("META-INF/container.xml", `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>`)
	write("OEBPS/content.opf", `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Livro Teste</dc:title>
    <dc:creator>Autora Teste</dc:creator>
    <dc:language>en</dc:language>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="ch01" href="text/ch01.xhtml" media-type="application/xhtml+xml"/>
    <item id="ch02" href="text/ch02.xhtml" media-type="application/xhtml+xml"/>
    <item id="css" href="styles/book.css" media-type="text/css"/>
  </manifest>
  <spine><itemref idref="ch01"/><itemref idref="ch02"/></spine>
</package>`)
	write("OEBPS/nav.xhtml", `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><head><title>Sumário</title></head><body><nav><a href="text/ch01.xhtml">Capítulo 1</a></nav></body></html>`)
	write("OEBPS/text/ch01.xhtml", `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><head><title>Capítulo 1</title></head><body><h1>Chapter One</h1><p>Hello <i>world</i>.</p></body></html>`)
	write("OEBPS/text/ch02.xhtml", `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><head><title>Capítulo 2</title></head><body><p>Second page.</p></body></html>`)
	write("OEBPS/styles/book.css", "body { font-family: serif; }")

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return target
}

func TestParse_metadataAndDocuments(t *testing.T) {
	path := writeFixture(t, t.TempDir())

	book, err := Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if book.Title != "Livro Teste" || book.Author != "Autora Teste" || book.Language != "en" {
		t.Errorf("metadata = %q/%q/%q", book.Title, book.Author, book.Language)
	}

	var paths []string
	for _, doc := range book.Documents {
		paths = append(paths, doc.Path)
	}
	want := []string{"OEBPS/text/ch01.xhtml", "OEBPS/text/ch02.xhtml", "OEBPS/nav.xhtml"}
	if strings.Join(paths, ",") != strings.Join(want, ",") {
		t.Errorf("documents = %v, want %v", paths, want)
	}
	first := book.Documents[0]
	if first.Title != "Capítulo 1" {
		t.Errorf("doc title = %q", first.Title)
	}
	if !strings.Contains(first.Body, "Hello <i>world</i>") {
		t.Errorf("body = %q", first.Body)
	}
}

func TestParse_rejectsGarbage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nao-epub.epub")
	if err := os.WriteFile(path, []byte("isto não é um zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(path); err == nil {
		t.Error("Parse aceitou um arquivo que não é epub")
	}
}

func TestRebuild_replacesBodiesAndLanguage(t *testing.T) {
	path := writeFixture(t, t.TempDir())

	var out bytes.Buffer
	translations := map[string]string{
		"OEBPS/text/ch01.xhtml": `<h1>Capítulo Um</h1><p>Olá <i>mundo</i>.</p>`,
	}
	if err := Rebuild(path, translations, "pt-BR", &out); err != nil {
		t.Fatal(err)
	}

	rebuilt := filepath.Join(t.TempDir(), "traduzido.epub")
	if err := os.WriteFile(rebuilt, out.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	reader, err := zip.OpenReader(rebuilt)
	if err != nil {
		t.Fatalf("epub reconstruído ilegível: %v", err)
	}
	defer func() { _ = reader.Close() }()
	if reader.File[0].Name != "mimetype" || reader.File[0].Method != zip.Store {
		t.Errorf("mimetype deve ser a primeira entrada sem compressão")
	}

	book, err := Parse(rebuilt)
	if err != nil {
		t.Fatal(err)
	}
	if book.Language != "pt-BR" {
		t.Errorf("language = %q, want pt-BR", book.Language)
	}
	bodies := map[string]string{}
	for _, doc := range book.Documents {
		bodies[doc.Path] = doc.Body
	}
	if !strings.Contains(bodies["OEBPS/text/ch01.xhtml"], "Olá <i>mundo</i>") {
		t.Errorf("ch01 body = %q", bodies["OEBPS/text/ch01.xhtml"])
	}
	if !strings.Contains(bodies["OEBPS/text/ch02.xhtml"], "Second page.") {
		t.Errorf("ch02 deve ficar intacto, body = %q", bodies["OEBPS/text/ch02.xhtml"])
	}
	if !strings.Contains(bodies["OEBPS/nav.xhtml"], "Capítulo 1") {
		t.Errorf("nav deve ficar intacto, body = %q", bodies["OEBPS/nav.xhtml"])
	}
}
