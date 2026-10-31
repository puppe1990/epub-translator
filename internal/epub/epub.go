// Package epub lê um EPUB (zip) o suficiente para traduzir: metadados, a lista
// de documentos XHTML (spine + nav) e o corpo de cada um. O rebuild troca
// apenas o miolo de <body> de cada documento traduzido e o dc:language do OPF;
// todo o resto do arquivo original passa intacto (byte a byte).
package epub

import (
	"archive/zip"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"
)

var (
	rootfileRE = regexp.MustCompile(`full-path="([^"]+)"`)
	itemRE     = regexp.MustCompile(`<item\s[^>]*>`)
	idRE       = regexp.MustCompile(`id="([^"]*)"`)
	hrefRE     = regexp.MustCompile(`href="([^"]*)"`)
	spineRE    = regexp.MustCompile(`<itemref\s[^>]*idref="([^"]*)"`)
	titleRE    = regexp.MustCompile(`(?s)<title[^>]*>(.*?)</title>`)
	bodyRE     = regexp.MustCompile(`(?s)<body[^>]*>(.*)</body>`)
	dcTitleRE  = regexp.MustCompile(`(?s)<dc:title[^>]*>(.*?)</dc:title>`)
	dcCreaRE   = regexp.MustCompile(`(?s)<dc:creator[^>]*>(.*?)</dc:creator>`)
	dcLangRE   = regexp.MustCompile(`(?s)<dc:language[^>]*>(.*?)</dc:language>`)
	tagStripRE = regexp.MustCompile(`(?s)<[^>]+>`)
)

// Document é um arquivo XHTML do EPUB que pode ser traduzido.
type Document struct {
	Path  string // caminho dentro do zip ("OEBPS/text/ch01.xhtml")
	Title string // <title> do documento (fallback: nome do arquivo)
	Body  string // conteúdo interno de <body>
}

// Book são os metadados e os documentos de um EPUB.
type Book struct {
	Title     string
	Author    string
	Language  string
	Documents []Document
}

// Parse abre o epub e devolve metadados e documentos na ordem do spine
// (mais o nav, quando não está no spine).
func Parse(srcPath string) (Book, error) {
	reader, err := zip.OpenReader(srcPath)
	if err != nil {
		return Book{}, fmt.Errorf("abrir epub: %w", err)
	}
	defer func() { _ = reader.Close() }()

	files := map[string]*zip.File{}
	for _, file := range reader.File {
		files[file.Name] = file
	}

	opfPath, err := findOPF(files)
	if err != nil {
		return Book{}, err
	}
	opf, err := readFile(files[opfPath])
	if err != nil {
		return Book{}, err
	}

	book := Book{
		Title:    firstGroup(dcTitleRE, opf),
		Author:   firstGroup(dcCreaRE, opf),
		Language: firstGroup(dcLangRE, opf),
	}
	book.Title = unescape(book.Title)
	book.Author = unescape(book.Author)

	manifest := manifestHrefs(opf)
	base := path.Dir(opfPath)
	if base == "." {
		base = ""
	}

	for _, idref := range spineRE.FindAllStringSubmatch(opf, -1) {
		doc, err := loadDocument(files, joinPath(base, manifest[idref[1]]))
		if err == nil {
			book.Documents = append(book.Documents, doc)
		}
	}
	for _, href := range manifest {
		if !strings.Contains(href, ".xhtml") && !strings.Contains(href, ".html") {
			continue
		}
		doc, err := loadDocument(files, joinPath(base, href))
		if err != nil || containsPath(book.Documents, doc.Path) {
			continue
		}
		book.Documents = append(book.Documents, doc)
	}
	if len(book.Documents) == 0 {
		return Book{}, fmt.Errorf("nenhum documento XHTML encontrado no epub")
	}
	return book, nil
}

func loadDocument(files map[string]*zip.File, href string) (Document, error) {
	file, ok := files[href]
	if !ok {
		return Document{}, fmt.Errorf("documento %s ausente", href)
	}
	content, err := readFile(file)
	if err != nil {
		return Document{}, err
	}
	match := bodyRE.FindStringSubmatchIndex(content)
	if match == nil {
		return Document{}, fmt.Errorf("%s sem <body>", href)
	}
	body := content[match[2]:match[3]]
	if strings.TrimSpace(tagStripRE.ReplaceAllString(body, "")) == "" {
		return Document{}, fmt.Errorf("%s sem texto", href)
	}
	title := unescape(strings.TrimSpace(firstGroup(titleRE, content)))
	if title == "" {
		title = path.Base(href)
	}
	return Document{Path: href, Title: title, Body: body}, nil
}

func findOPF(files map[string]*zip.File) (string, error) {
	container, ok := files["META-INF/container.xml"]
	if ok {
		content, err := readFile(container)
		if err == nil {
			if match := rootfileRE.FindStringSubmatch(content); match != nil {
				return match[1], nil
			}
		}
	}
	for name := range files {
		if strings.HasSuffix(name, ".opf") {
			return name, nil
		}
	}
	return "", fmt.Errorf("OPF não encontrado no epub")
}

func manifestHrefs(opf string) map[string]string {
	hrefs := map[string]string{}
	for _, item := range itemRE.FindAllString(opf, -1) {
		id, href := firstGroup(idRE, item), firstGroup(hrefRE, item)
		if id != "" && href != "" {
			hrefs[id] = href
		}
	}
	return hrefs
}

func readFile(file *zip.File) (string, error) {
	handle, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("abrir %s: %w", file.Name, err)
	}
	defer func() { _ = handle.Close() }()
	content, err := io.ReadAll(handle)
	if err != nil {
		return "", fmt.Errorf("ler %s: %w", file.Name, err)
	}
	return string(content), nil
}

func firstGroup(re *regexp.Regexp, content string) string {
	if match := re.FindStringSubmatch(content); match != nil {
		return match[1]
	}
	return ""
}

func unescape(text string) string {
	replacer := strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`, "&#39;", "'")
	return strings.TrimSpace(replacer.Replace(text))
}

func containsPath(docs []Document, target string) bool {
	for _, doc := range docs {
		if doc.Path == target {
			return true
		}
	}
	return false
}

func joinPath(base, href string) string {
	if base == "" {
		return href
	}
	return base + "/" + href
}

// Rebuild escreve um novo epub com os corpos traduzidos (mapa path→body) e o
// dc:language atualizado. Documentos sem tradução saem inalterados; demais
// arquivos são copiados como estão.
func Rebuild(srcPath string, translations map[string]string, language string, out io.Writer) error {
	reader, err := zip.OpenReader(srcPath)
	if err != nil {
		return fmt.Errorf("abrir epub: %w", err)
	}
	defer func() { _ = reader.Close() }()

	files := map[string]*zip.File{}
	for _, file := range reader.File {
		files[file.Name] = file
	}
	opfPath, _ := findOPF(files)

	writer := zip.NewWriter(out)
	for _, file := range reader.File {
		content, err := readFile(file)
		if err != nil {
			return err
		}
		switch {
		case file.Name == "mimetype":
			// O mimetype precisa ser a primeira entrada e sem compressão.
			header := &zip.FileHeader{Name: "mimetype", Method: zip.Store}
			entry, err := writer.CreateHeader(header)
			if err != nil {
				return err
			}
			if _, err := entry.Write([]byte(content)); err != nil {
				return err
			}
			continue
		case file.Name == opfPath && language != "":
			content = replaceLanguage(content, language)
		case translations[file.Name] != "":
			content = replaceBody(content, translations[file.Name])
		}
		entry, err := writer.Create(file.Name)
		if err != nil {
			return err
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			return err
		}
	}
	return writer.Close()
}

func replaceBody(content, translation string) string {
	match := bodyRE.FindStringSubmatchIndex(content)
	if match == nil {
		return content
	}
	return content[:match[2]] + translation + content[match[3]:]
}

func replaceLanguage(content, language string) string {
	if !dcLangRE.MatchString(content) {
		return content
	}
	return dcLangRE.ReplaceAllString(content, "<dc:language>"+language+"</dc:language>")
}

// HasText informa se o corpo tem texto traduzível (ignora só-imagem).
func HasText(body string) bool {
	return strings.TrimSpace(tagStripRE.ReplaceAllString(body, "")) != ""
}
