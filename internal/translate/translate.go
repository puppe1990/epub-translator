// Package translate chama um endpoint OpenAI-compatível (OpenAI, Gemini,
// OpenRouter…) para traduzir o corpo de um documento e valida a resposta
// contra o original antes de aceitá-la.
package translate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ChatClient é o contrato do provedor — o Service (retry) e a pipeline usam
// só isto, e os testes injetam um fake.
type ChatClient interface {
	Translate(ctx context.Context, targetLanguage, content string) (string, error)
}

// Client fala o dialeto OpenAI /chat/completions, que Gemini, OpenRouter e
// Groq também expõem.
type Client struct {
	BaseURL        string
	APIKey         string
	Model          string
	SourceLanguage string
	HTTP           *http.Client
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Messages    []chatMessage `json:"messages"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func (c Client) Translate(ctx context.Context, targetLanguage, content string) (string, error) {
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 240 * time.Second}
	}
	messages, tags := c.messages(targetLanguage, content)
	payload, err := json.Marshal(chatRequest{
		Model:       c.Model,
		Temperature: 0.2,
		MaxTokens:   c.maxOutputTokens(),
		Messages:    messages,
	})
	if err != nil {
		return "", fmt.Errorf("montar requisição: %w", err)
	}

	url := strings.TrimRight(c.BaseURL, "/") + "/chat/completions"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("requisição: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+c.APIKey)
	request.Header.Set("Content-Type", "application/json")

	response, err := httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("chamar provedor: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return "", fmt.Errorf("provedor respondeu %d: %s", response.StatusCode, snippet(string(raw), 200))
	}

	var decoded chatResponse
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return "", fmt.Errorf("resposta ilegível do provedor: %w", err)
	}
	if len(decoded.Choices) == 0 {
		return "", fmt.Errorf("provedor não devolveu tradução")
	}
	out := strings.TrimSpace(decoded.Choices[0].Message.Content)
	if len(tags) > 0 {
		out = strings.TrimSpace(unmaskHTMLTags(out, tags))
	}
	return out, nil
}

func (c Client) maxOutputTokens() int {
	if strings.Contains(c.Model, "riva-translate") {
		return 2048
	}
	return 0
}

func (c Client) messages(targetLanguage, content string) ([]chatMessage, []string) {
	if strings.Contains(c.Model, "riva-translate") {
		masked, tags := maskHTMLTags(rivaSourceText(content))
		return []chatMessage{
			{Role: "system", Content: rivaLanguagePair(c.SourceLanguage, targetLanguage)},
			{Role: "user", Content: masked},
		}, tags
	}
	system := SystemPrompt(targetLanguage)
	user := content
	var tags []string
	if strings.Contains(strings.ToLower(c.Model), "gemini") {
		user, tags = maskHTMLTags(content)
		system += "\n6. Os marcadores ⟦tN⟧ são as tags HTML: copie cada um exatamente, sem traduzir nem omitir."
	}
	return []chatMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}, tags
}

// Service converte erros de validação em retentativas, com um lembrete extra
// quando a primeira resposta veio torta.
type Service struct {
	Client        ChatClient
	Attempts      int
	MaxInputChars int
}

func (s Service) TranslateBody(ctx context.Context, targetLanguage, body string) (string, error) {
	parts := splitBody(body, s.MaxInputChars)
	var combined strings.Builder
	for _, part := range parts {
		out, err := s.translateChunk(ctx, targetLanguage, part)
		if err != nil {
			return "", err
		}
		combined.WriteString(out)
	}
	result := combined.String()
	if len(parts) > 1 {
		if err := ValidateBody(body, result); err != nil {
			return "", fmt.Errorf("tradução inválida após juntar %d partes: %w", len(parts), err)
		}
	}
	return result, nil
}

func (s Service) translateChunk(ctx context.Context, targetLanguage, body string) (string, error) {
	attempts := s.Attempts
	if attempts < 1 {
		attempts = 3
	}
	base := fmt.Sprintf("Traduza para %s:\n\n%s", LanguageName(targetLanguage), body)

	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		content := base
		if attempt > 0 {
			content += "\n\nATENÇÃO: a resposta anterior veio com texto fora do HTML, tags alteradas " +
				"ou sem traduzir — responda só com o XHTML traduzido, com as mesmas tags."
		}
		out, err := s.Client.Translate(ctx, targetLanguage, content)
		if err != nil {
			if isContextOverflow(err) {
				return "", err
			}
			lastErr = err
			continue
		}
		if err := ValidateBody(body, out); err != nil {
			lastErr = err
			continue
		}
		return out, nil
	}
	return "", fmt.Errorf("tradução inválida após %d tentativas: %w", attempts, lastErr)
}

func isContextOverflow(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "maximum context length") || strings.Contains(msg, "context length")
}

// SystemPrompt monta as regras; o idioma-alvo entra por extenso para o modelo
// não confundir "pt" com espanhol nem traduzir para o idioma errado.
func SystemPrompt(targetLanguage string) string {
	return "Você é um tradutor literário profissional. Traduza fragmentos de livro (XHTML) para " +
		LanguageName(targetLanguage) + ".\n" +
		"Regras obrigatórias:\n" +
		"1. Mantenha TODAS as tags HTML exatamente como estão (mesmos nomes, estrutura e atributos). " +
		"Traduza apenas o texto visível e os valores de alt.\n" +
		"2. Não adicione, remova ou comente nada — nem notas do tradutor, nem texto antes ou depois do " +
		"HTML. Traduza o fragmento INTEIRO: nenhuma frase pode permanecer no idioma original.\n" +
		"3. Nomes próprios permanecem como no original; referências bibliográficas e títulos de obras " +
		"citadas também.\n" +
		"4. Português natural e fluente, registro de livro publicado, quando o destino for pt-BR.\n" +
		"5. Responda somente com o XHTML traduzido."
}

// Riva v2 espera o par de idiomas no system (ex. en-pt-br) e o texto cru no
// user. Sem esse formato o modelo não traduz como o fine-tune.
func rivaLanguagePair(source, target string) string {
	src := rivaLangCode(source)
	if src == "" {
		src = "en"
	}
	tgt := rivaLangCode(target)
	if tgt == "" {
		tgt = "pt-br"
	}
	return src + "-" + tgt
}

func rivaLangCode(tag string) string {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(tag), "_", "-"))
	switch {
	case normalized == "":
		return ""
	case strings.HasPrefix(normalized, "pt-pt"):
		return "pt-pt"
	case strings.HasPrefix(normalized, "pt"):
		return "pt-br"
	case strings.HasPrefix(normalized, "zh-tw") || strings.HasPrefix(normalized, "zh-hant"):
		return "zh-tw"
	case strings.HasPrefix(normalized, "zh"):
		return "zh-cn"
	case strings.HasPrefix(normalized, "es-us") || strings.HasPrefix(normalized, "es-419"):
		return "es-us"
	case strings.HasPrefix(normalized, "es"):
		return "es-es"
	default:
		if i := strings.Index(normalized, "-"); i > 0 {
			return normalized[:i]
		}
		return normalized
	}
}

func rivaSourceText(content string) string {
	body := content
	if strings.HasPrefix(content, "Traduza para ") {
		if i := strings.Index(content, "\n\n"); i >= 0 {
			body = content[i+2:]
		}
	}
	if i := strings.Index(body, "\n\nATENÇÃO:"); i >= 0 {
		body = body[:i]
	}
	return body
}

// LanguageName mapeia a tag para o nome por extenso usado no prompt.
func LanguageName(tag string) string {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(tag), "_", "-"))
	switch {
	case strings.HasPrefix(normalized, "pt"):
		return "português do Brasil"
	case strings.HasPrefix(normalized, "en"):
		return "inglês"
	case strings.HasPrefix(normalized, "es"):
		return "espanhol"
	case strings.HasPrefix(normalized, "fr"):
		return "francês"
	case strings.HasPrefix(normalized, "de"):
		return "alemão"
	case strings.HasPrefix(normalized, "it"):
		return "italiano"
	case strings.HasPrefix(normalized, "ja"):
		return "japonês"
	default:
		return tag
	}
}

var (
	tagRE      = regexp.MustCompile(`<(/?)([a-zA-Z][a-zA-Z0-9]*)([^>]*)>`)
	entitiesRE = strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`, "&#39;", "'")
)

// ValidateBody confere a resposta contra o original: sequência de tags
// idêntica (normalizada), fragmento limpo (sem preâmbulo), tamanho plausível,
// texto de fato traduzido e sem mistura de idioma original com tradução.
func ValidateBody(source, out string) error {
	if strings.TrimSpace(out) == "" {
		return fmt.Errorf("resposta vazia")
	}
	// Compara nomes de tags como multiconjunto: a ordem pode mudar quando a
	// tradução reorganiza a frase e atributos de apresentação (class, epub:type,
	// role) podem sumir sem quebrar a estrutura — o que não pode é perder ou
	// inventar elementos.
	sourceNames, outNames := tagNames(source), tagNames(out)
	if strings.Join(sourceNames, "|") != strings.Join(outNames, "|") {
		return fmt.Errorf("tags alteradas (fonte %d, resposta %d): faltando %v, sobrando %v",
			len(sourceNames), len(outNames), diffNames(sourceNames, outNames), diffNames(outNames, sourceNames))
	}

	sourceText, outText := plainText(source), plainText(out)
	if sourceText != "" && outText == sourceText {
		return fmt.Errorf("texto idêntico ao original (não traduziu)")
	}
	if sourceText != "" {
		ratio := float64(len(strings.Fields(outText))) / float64(len(strings.Fields(sourceText)))
		if ratio < 0.3 || ratio > 2.5 {
			return fmt.Errorf("tamanho implausível: proporção %.2f", ratio)
		}
	}

	sourceLead, outLead := leadingText(source), leadingText(out)
	switch {
	case strings.TrimSpace(sourceLead) == "" && strings.TrimSpace(outLead) != "":
		return fmt.Errorf("preâmbulo antes do HTML: %q", snippet(outLead, 40))
	case strings.TrimSpace(sourceLead) != "" &&
		len(strings.Fields(outLead)) > len(strings.Fields(sourceLead))*3:
		return fmt.Errorf("texto inicial muito maior que o original")
	}
	if sourceText != "" {
		if err := rejectMixedLanguage(sourceText, outText); err != nil {
			return err
		}
	}
	return nil
}

// tagNames devolve os nomes das tags (abertura e fechamento) ordenados, com
// span de fora — spans carregam só estilo (drop-cap, smallcaps) e os modelos
// ora os fundem, ora os mantêm.
func tagNames(html string) []string {
	matches := tagRE.FindAllStringSubmatch(html, -1)
	names := make([]string, 0, len(matches))
	for _, match := range matches {
		if strings.EqualFold(match[2], "span") {
			continue
		}
		names = append(names, strings.ToLower(match[1]+match[2]))
	}
	sort.Strings(names)
	return names
}

// diffNames lista as tags de a que sobram em relação a b (mensagem de erro).
func diffNames(a, b []string) []string {
	remaining := map[string]int{}
	for _, name := range b {
		remaining[name]++
	}
	diff := []string{}
	for _, name := range a {
		if remaining[name] > 0 {
			remaining[name]--
			continue
		}
		diff = append(diff, name)
	}
	return diff
}

func plainText(html string) string {
	text := tagRE.ReplaceAllString(html, " ")
	text = entitiesRE.Replace(text)
	return strings.Join(strings.Fields(text), " ")
}

func leadingText(html string) string {
	if index := strings.Index(html, "<"); index >= 0 {
		return html[:index]
	}
	return html
}

func snippet(text string, limit int) string {
	clean := strings.Join(strings.Fields(text), " ")
	if len(clean) <= limit {
		return clean
	}
	return clean[:limit] + "…"
}
