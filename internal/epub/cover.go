package epub

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"regexp"
	"strings"
)

// ErrCoverNotFound indica que o EPUB não traz uma imagem de capa localizável.
var ErrCoverNotFound = errors.New("epub sem imagem de capa")

var (
	metaTagRE = regexp.MustCompile(`(?i)<meta\s[^>]*>`)
	imgTagRE  = regexp.MustCompile(`(?i)<img\s[^>]*>`)
	tagAttrRE = regexp.MustCompile(`(?i)([a-z][a-z0-9:_-]*)\s*=\s*"(?:([^"]*))"`)
)

var coverImageExts = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".svg":  "image/svg+xml",
}

type manifestItem struct {
	ID         string
	Href       string
	MediaType  string
	Properties string
}

// Cover devolve os bytes da imagem de capa do EPUB e o seu mime type. A
// ordem de busca cobre EPUB3 (properties="cover-image"), EPUB2
// (<meta name="cover">) e o resto por nome do arquivo; sem isso, procura uma
// página cover.xhtml com <img>.
func Cover(srcPath string) ([]byte, string, error) {
	reader, files, err := openZip(srcPath)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = reader.Close() }()

	zipPath, mime, err := locateCover(files)
	if err != nil {
		return nil, "", err
	}
	file, ok := files[zipPath]
	if !ok {
		return nil, "", fmt.Errorf("capa %s ausente do zip", zipPath)
	}
	handle, err := file.Open()
	if err != nil {
		return nil, "", fmt.Errorf("abrir capa %s: %w", zipPath, err)
	}
	defer func() { _ = handle.Close() }()
	data, err := io.ReadAll(handle)
	if err != nil {
		return nil, "", fmt.Errorf("ler capa %s: %w", zipPath, err)
	}
	return data, mime, nil
}

// HasCover informa se o EPUB tem capa sem carregar a imagem.
func HasCover(srcPath string) bool {
	reader, files, err := openZip(srcPath)
	if err != nil {
		return false
	}
	defer func() { _ = reader.Close() }()
	_, _, err = locateCover(files)
	return err == nil
}

func openZip(srcPath string) (*zip.ReadCloser, map[string]*zip.File, error) {
	reader, err := zip.OpenReader(srcPath)
	if err != nil {
		return nil, nil, fmt.Errorf("abrir epub: %w", err)
	}
	files := map[string]*zip.File{}
	for _, file := range reader.File {
		files[file.Name] = file
	}
	return reader, files, nil
}

func locateCover(files map[string]*zip.File) (string, string, error) {
	opfPath, err := findOPF(files)
	if err != nil {
		return "", "", err
	}
	opf, err := readFile(files[opfPath])
	if err != nil {
		return "", "", err
	}
	base := path.Dir(opfPath)
	if base == "." {
		base = ""
	}
	items := manifestItems(opf)
	if item, ok := coverItem(opf, items); ok {
		zipPath := resolveZipPath(base, item.Href)
		if _, exists := files[zipPath]; exists {
			return zipPath, coverMime(item, item.Href), nil
		}
	}
	if zipPath, mime, ok := coverImageFromXHTML(files, items, base); ok {
		return zipPath, mime, nil
	}
	return "", "", ErrCoverNotFound
}

func manifestItems(opf string) []manifestItem {
	var items []manifestItem
	for _, tag := range itemRE.FindAllString(opf, -1) {
		attrs := tagAttrs(tag)
		items = append(items, manifestItem{
			ID:         attrs["id"],
			Href:       attrs["href"],
			MediaType:  attrs["media-type"],
			Properties: attrs["properties"],
		})
	}
	return items
}

func tagAttrs(tag string) map[string]string {
	attrs := map[string]string{}
	for _, match := range tagAttrRE.FindAllStringSubmatch(tag, -1) {
		attrs[strings.ToLower(match[1])] = match[2]
	}
	return attrs
}

// coverItem aplica a precedência dos padrões de capa do EPUB.
func coverItem(opf string, items []manifestItem) (manifestItem, bool) {
	for _, item := range items {
		if hasProperty(item.Properties, "cover-image") {
			return item, true
		}
	}
	if id := metaCoverID(opf); id != "" {
		for _, item := range items {
			if item.ID == id {
				return item, true
			}
		}
	}
	for _, item := range items {
		if isImageItem(item) && strings.Contains(strings.ToLower(item.ID+path.Base(item.Href)), "cover") {
			return item, true
		}
	}
	return manifestItem{}, false
}

func hasProperty(properties, wanted string) bool {
	for _, property := range strings.Fields(properties) {
		if strings.EqualFold(property, wanted) {
			return true
		}
	}
	return false
}

func metaCoverID(opf string) string {
	for _, tag := range metaTagRE.FindAllString(opf, -1) {
		attrs := tagAttrs(tag)
		if strings.EqualFold(attrs["name"], "cover") {
			return attrs["content"]
		}
	}
	return ""
}

func isImageItem(item manifestItem) bool {
	if strings.HasPrefix(strings.ToLower(item.MediaType), "image/") {
		return true
	}
	_, ok := coverImageExts[strings.ToLower(path.Ext(item.Href))]
	return ok
}

func coverMime(item manifestItem, href string) string {
	if strings.HasPrefix(strings.ToLower(item.MediaType), "image/") {
		return item.MediaType
	}
	if mime, ok := coverImageExts[strings.ToLower(path.Ext(href))]; ok {
		return mime
	}
	return "application/octet-stream"
}

// coverImageFromXHTML cobre EPUBs antigos cuja capa é uma página própria com
// <img> apontando para uma imagem sem "cover" no nome.
func coverImageFromXHTML(files map[string]*zip.File, items []manifestItem, base string) (string, string, bool) {
	for _, item := range items {
		if !strings.Contains(strings.ToLower(path.Base(item.Href)), "cover") {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(item.Href), ".xhtml") && !strings.HasSuffix(strings.ToLower(item.Href), ".html") {
			continue
		}
		pagePath := resolveZipPath(base, item.Href)
		content, err := readFile(files[pagePath])
		if err != nil {
			continue
		}
		for _, tag := range imgTagRE.FindAllString(content, -1) {
			src := tagAttrs(tag)["src"]
			if src == "" {
				continue
			}
			zipPath := resolveZipPath(path.Dir(pagePath), src)
			if _, exists := files[zipPath]; exists {
				return zipPath, coverMime(manifestItem{}, src), true
			}
		}
	}
	return "", "", false
}

// resolveZipPath resolve o href do OPF contra o diretório do OPF (aceita
// ../, %20 e âncora/query).
func resolveZipPath(base, href string) string {
	cleaned := strings.TrimSpace(href)
	cleaned = strings.SplitN(cleaned, "#", 2)[0]
	cleaned = strings.SplitN(cleaned, "?", 2)[0]
	if unescaped, err := url.PathUnescape(cleaned); err == nil {
		cleaned = unescaped
	}
	return strings.TrimPrefix(path.Join(base, cleaned), "./")
}
