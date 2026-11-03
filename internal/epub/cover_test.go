package epub

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const testContainer = `<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`

func testChapter() string {
	return `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><head><title>Um</title></head><body><p>Hello.</p></body></html>`
}

// buildEPUB monta um zip com os arquivos dados (nome→conteúdo).
func buildEPUB(t *testing.T, files map[string]string) string {
	t.Helper()
	target := filepath.Join(t.TempDir(), "fixture.epub")
	file, err := os.Create(target)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	writer := zip.NewWriter(file)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return target
}

func TestCover_metaCoverFromEPUB2(t *testing.T) {
	coverBytes := "\xff\xd8\xff\xe0-foto-da-capa"
	path := buildEPUB(t, map[string]string{
		"META-INF/container.xml": testContainer,
		"OEBPS/content.opf": `<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="2.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Teste</dc:title><meta name="cover" content="cover-jpg"/></metadata>
  <manifest>
    <item id="cover-jpg" href="images/capa.jpg" media-type="image/jpeg"/>
    <item id="ch01" href="ch01.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine><itemref idref="ch01"/></spine>
</package>`,
		"OEBPS/images/capa.jpg": coverBytes,
		"OEBPS/ch01.xhtml":      testChapter(),
	})

	data, mime, err := Cover(path)
	if err != nil {
		t.Fatal(err)
	}
	if mime != "image/jpeg" || string(data) != coverBytes {
		t.Errorf("cover = %q (%s), want image/jpeg com %q", data, mime, coverBytes)
	}
}

func TestCover_propertiesCoverImageEPUB3(t *testing.T) {
	coverBytes := "\x89PNG-fake"
	path := buildEPUB(t, map[string]string{
		"META-INF/container.xml": testContainer,
		"OEBPS/content.opf": `<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Teste</dc:title></metadata>
  <manifest>
    <item id="figura" href="../Images/cover.png" media-type="image/png" properties="cover-image"/>
    <item id="ch01" href="ch01.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine><itemref idref="ch01"/></spine>
</package>`,
		"Images/cover.png": coverBytes,
		"OEBPS/ch01.xhtml": testChapter(),
	})

	data, mime, err := Cover(path)
	if err != nil {
		t.Fatal(err)
	}
	if mime != "image/png" || string(data) != coverBytes {
		t.Errorf("cover = %q (%s), want image/png com %q", data, mime, coverBytes)
	}
}

func TestCover_fallsBackToCoverLikeName(t *testing.T) {
	coverBytes := "jpeg-sem-meta"
	path := buildEPUB(t, map[string]string{
		"META-INF/container.xml": testContainer,
		"OEBPS/content.opf": `<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Teste</dc:title></metadata>
  <manifest>
    <item id="imagem" href="FrontCover.jpeg" media-type="image/jpeg"/>
    <item id="ch01" href="ch01.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine><itemref idref="ch01"/></spine>
</package>`,
		"OEBPS/FrontCover.jpeg": coverBytes,
		"OEBPS/ch01.xhtml":      testChapter(),
	})

	data, mime, err := Cover(path)
	if err != nil {
		t.Fatal(err)
	}
	if mime != "image/jpeg" || string(data) != coverBytes {
		t.Errorf("cover = %q (%s), want image/jpeg com %q", data, mime, coverBytes)
	}
}

func TestCover_readsImageFromCoverXHTML(t *testing.T) {
	coverBytes := "foto-de-abertura"
	path := buildEPUB(t, map[string]string{
		"META-INF/container.xml": testContainer,
		"OEBPS/content.opf": `<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="2.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Teste</dc:title></metadata>
  <manifest>
    <item id="capa" href="cover.xhtml" media-type="application/xhtml+xml"/>
    <item id="foto" href="front.jpg" media-type="image/jpeg"/>
    <item id="ch01" href="ch01.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine><itemref idref="capa"/><itemref idref="ch01"/></spine>
</package>`,
		"OEBPS/cover.xhtml": `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><head><title>Capa</title></head><body><div><img src="front.jpg" alt="Capa"/></div></body></html>`,
		"OEBPS/front.jpg":   coverBytes,
		"OEBPS/ch01.xhtml":  testChapter(),
	})

	data, mime, err := Cover(path)
	if err != nil {
		t.Fatal(err)
	}
	if mime != "image/jpeg" || string(data) != coverBytes {
		t.Errorf("cover = %q (%s), want image/jpeg com %q", data, mime, coverBytes)
	}
}

func TestCover_missingReturnsErrCoverNotFound(t *testing.T) {
	path := buildEPUB(t, map[string]string{
		"META-INF/container.xml": testContainer,
		"OEBPS/content.opf": `<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Teste</dc:title></metadata>
  <manifest><item id="ch01" href="ch01.xhtml" media-type="application/xhtml+xml"/></manifest>
  <spine><itemref idref="ch01"/></spine>
</package>`,
		"OEBPS/ch01.xhtml": testChapter(),
	})

	if _, _, err := Cover(path); !errors.Is(err, ErrCoverNotFound) {
		t.Errorf("err = %v, want ErrCoverNotFound", err)
	}
	if HasCover(path) {
		t.Error("HasCover = true num epub sem capa")
	}
}

func TestHasCover_trueWhenCoverExists(t *testing.T) {
	path := buildEPUB(t, map[string]string{
		"META-INF/container.xml": testContainer,
		"OEBPS/content.opf": `<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Teste</dc:title></metadata>
  <manifest>
    <item id="cover" href="cover.jpg" media-type="image/jpeg" properties="cover-image"/>
    <item id="ch01" href="ch01.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine><itemref idref="ch01"/></spine>
</package>`,
		"OEBPS/cover.jpg":  "capa",
		"OEBPS/ch01.xhtml": testChapter(),
	})

	if !HasCover(path) {
		t.Error("HasCover = false num epub com capa")
	}
}

func TestCover_missingFileReturnsError(t *testing.T) {
	if _, _, err := Cover(filepath.Join(t.TempDir(), "nao-existe.epub")); err == nil {
		t.Error("Cover aceitou um arquivo inexistente")
	}
	if HasCover(filepath.Join(t.TempDir(), "nao-existe.epub")) {
		t.Error("HasCover = true num arquivo inexistente")
	}
}
