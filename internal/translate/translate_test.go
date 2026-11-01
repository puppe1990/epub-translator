package translate

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

type errorClient struct {
	err   error
	calls int
}

func (e *errorClient) Translate(context.Context, string, string) (string, error) {
	e.calls++
	return "", e.err
}

func TestValidateBody_acceptsTranslatedSameTags(t *testing.T) {
	source := `<h1>Chapter One</h1><p>Hello <i>world</i>, this is a test.</p>`
	out := `<h1>Capítulo Um</h1><p>Olá <i>mundo</i>, isto é um teste.</p>`
	if err := ValidateBody(source, out); err != nil {
		t.Errorf("validou errado: %v", err)
	}
}

func TestValidateBody_rejectsLostTag(t *testing.T) {
	source := `<h1>Chapter One</h1><p>Hello <i>world</i>.</p>`
	out := `<h1>Capítulo Um</h1><p>Olá mundo.</p>`
	if err := ValidateBody(source, out); err == nil {
		t.Error("aceitou resposta sem <i>")
	}
}

func TestValidateBody_rejectsAddedTag(t *testing.T) {
	source := `<p>Hello world, good morning everyone.</p>`
	out := `<p>Olá mundo, bom dia a todas as pessoas.<b>extra</b></p>`
	if err := ValidateBody(source, out); err == nil {
		t.Error("aceitou resposta com tag nova")
	}
}

func TestValidateBody_rejectsPreamble(t *testing.T) {
	source := `<p>Hello world, good morning everyone.</p>`
	out := "Aqui está a tradução:\n\n<p>Olá mundo, bom dia a todas as pessoas.</p>"
	if err := ValidateBody(source, out); err == nil {
		t.Error("aceitou preâmbulo")
	}
}

func TestValidateBody_rejectsUntranslated(t *testing.T) {
	source := `<p>Hello world, good morning everyone.</p>`
	if err := ValidateBody(source, source); err == nil {
		t.Error("aceitou texto idêntico (não traduzido)")
	}
}

func TestValidateBody_acceptsUntranslatedPhotoCaptionName(t *testing.T) {
	// Cap. 55/128 do Total Recall: só o nome na legenda da foto.
	source := `<div class="group"><p class="image"><img alt="images" src="x.jpeg"/></p><p class="caption"><span class="txit">Peter Brenner</span></p></div>`
	if err := ValidateBody(source, source); err != nil {
		t.Fatalf("legenda que é só nome próprio deveria passar: %v", err)
	}
}

func TestValidateBody_rejectsMixedLanguageChapter(t *testing.T) {
	// Recorte do cap. 26: o Riva ecoa o primeiro corte em inglês e traduz o resto.
	source := mixedChapterSource()
	out := mixedChapterEchoThenPortuguese()
	err := ValidateBody(source, out)
	if err == nil {
		t.Fatal("aceitou capítulo com começo em inglês e resto em português")
	}
	if !strings.Contains(err.Error(), "misturado") {
		t.Errorf("erro = %v, queria misturado", err)
	}
}

func TestValidateBody_acceptsTranslatedBookIndex(t *testing.T) {
	// INDEX real: "Mr." quebra o splitter em blocos de nomes (Jaccard alto) e
	// entradas traduzidas (fresh). O join via ValidateBody via isso como
	// "texto misturado" mesmo com cada <p> traduzido.
	var src, out strings.Builder
	src.WriteString(`<h2>INDEX</h2>`)
	out.WriteString(`<h2>ÍNDICE</h2>`)
	for i := 0; i < 90; i++ {
		fmt.Fprintf(&src, `<p>Gerstl, Fredi, 26, 27, 47, 60, 133, 143, 291. Mr. Universe contest %d, 49, 69</p>`, i)
		fmt.Fprintf(&out, `<p>Gerstl, Fredi, 26, 27, 47, 60, 133, 143, 291. Mr. Universe concurso %d, 49, 69</p>`, i)
	}
	for i := 0; i < 90; i++ {
		fmt.Fprintf(&src, `<p>See also specific movie. Action movies, 337, 360, 402, 410, 439</p>`)
		fmt.Fprintf(&out, `<p>Ver também filme específico. Filmes de ação, 337, 360, 402, 410, 439</p>`)
	}
	if err := ValidateBody(src.String(), out.String()); err != nil {
		t.Fatalf("índice com nomes próprios mantidos deveria passar: %v", err)
	}
}

func TestValidateBody_acceptsFullyTranslatedLongChapter(t *testing.T) {
	source := mixedChapterSource()
	out := mixedChapterAllPortuguese()
	if err := ValidateBody(source, out); err != nil {
		t.Fatalf("tradução inteira em português deveria passar: %v", err)
	}
}

func mixedChapterSource() string {
	return `<p>IN BODYBUILDING I WAS king of the mountain, but back in everyday LA I was still just another immigrant struggling to learn English and make a life. My mind was so fixed on what I was doing in America that I rarely gave a thought to Austria or Germany. If I was competing in Europe, I'd go home to visit, and I kept in touch with Fredi Gerstl in Graz and Albert Busek in Munich. Often I would cross paths with Albert and other European friends on the bodybuilding circuit.</p>` +
		`<p>I regularly sent pictures and letters home to my parents, telling them what I was doing. Whenever I won a championship, I'd send them the trophy, because I didn't need it in my apartment, and I wanted them to be proud. I'm not sure any of this meant much to them at the start, but after awhile they put up the photos and built a special shelf in their home to show off the trophies. My dad would answer my letters for both of them. He always enclosed my original letter marked up in red ink, correcting my mistakes in spelling and grammar.</p>` +
		`<p>Artie took me to a Hungarian Jewish restaurant where the dishes were just like Austrian food. The owner came over to take our order and I told him I saw something here that I like, give me a little of your junk. That kind of joke made it easy to laugh with strangers in the new country. We stayed until late talking about training, work, and the people we had left behind in Europe.</p>` +
		`<p>He always made time to sit with me after dinner and talk through the words I had gotten wrong during the week. Those evenings taught me more English than any classroom I had tried before coming to America. I wrote everything down and read it again in the morning before going to the gym.</p>`
}

func mixedChapterEchoThenPortuguese() string {
	return `<p>IN BODYBUILDING I WAS king of the mountain, but back in everyday LA I was still just another immigrant struggling to learn English and make a life. My mind was so fixed on what I was doing in America that I rarely gave a thought to Austria or Germany. If I was competing in Europe, I'd go home to visit, and I kept in touch with Fredi Gerstl in Graz and Albert Busek in Munich. Often I would cross paths with Albert and other European friends on the bodybuilding circuit.</p>` +
		`<p>I regularly sent pictures and letters home to my parents, telling them what I was doing. Whenever I won a championship, I'd send them the trophy, because I didn't need it in my apartment, and I wanted them to be proud. I'm not sure any of this meant much to them at the start, but after a while they put up the photos and built a special shelf in their home to show off the trophies. My dad would answer my letters for both of them. He always enclosed my original letter marked up in red ink, correcting my mistakes in spelling and grammar.</p>` +
		`<p>Como um presente, Artie me levou a um restaurante judeu-húngaro onde os pratos eram iguais à comida austríaca. O proprietário veio tomar nosso pedido e eu disse que vi uma coisa aqui de que gosto, dê um pouco do seu lixo. Esse tipo de piada tornava fácil rir com desconhecidos no país novo. Ficamos até tarde falando de treino, trabalho e das pessoas que tínhamos deixado para trás na Europa.</p>` +
		`<p>Ele sempre encontrava tempo para sentar comigo depois do jantar e revisar as palavras que eu tinha errado durante a semana. Aquelas noites me ensinaram mais inglês do que qualquer sala de aula que eu tinha tentado antes de chegar à América. Eu escrevia tudo e lia de novo de manhã antes de ir para a academia.</p>`
}

func mixedChapterAllPortuguese() string {
	return `<p>No fisiculturismo eu era o rei da montanha, mas na vida comum de Los Angeles eu ainda era só mais um imigrante lutando para aprender inglês e construir uma vida. Minha cabeça estava tão presa no que eu fazia na América que quase não pensava na Áustria ou na Alemanha. Se eu competia na Europa, ia visitar casa e mantinha contato com Fredi Gerstl em Graz e Albert Busek em Munique. Muitas vezes cruzava com Albert e outros amigos europeus no circuito de fisiculturismo.</p>` +
		`<p>Eu mandava fotos e cartas para meus pais, contando o que estava fazendo. Sempre que ganhava um campeonato, enviava o troféu, porque não precisava dele no apartamento e queria que eles sentissem orgulho. No começo eu não tinha certeza se isso importava para eles, mas depois de um tempo eles colocaram as fotos e montaram uma prateleira especial para exibir os troféus. Meu pai respondia as cartas dos dois. Sempre devolvia a carta original marcada em tinta vermelha, corrigindo meus erros de ortografia e gramática.</p>` +
		`<p>Como um presente, Artie me levou a um restaurante judeu-húngaro onde os pratos eram iguais à comida austríaca. O proprietário veio tomar nosso pedido e eu disse que vi uma coisa aqui de que gosto, dê um pouco do seu lixo. Esse tipo de piada tornava fácil rir com desconhecidos no país novo. Ficamos até tarde falando de treino, trabalho e das pessoas que tínhamos deixado para trás na Europa.</p>` +
		`<p>Ele sempre encontrava tempo para sentar comigo depois do jantar e revisar as palavras que eu tinha errado durante a semana. Aquelas noites me ensinaram mais inglês do que qualquer sala de aula que eu tinha tentado antes de chegar à América. Eu escrevia tudo e lia de novo de manhã antes de ir para a academia.</p>`
}

func TestValidateBody_rejectsRatioBlowup(t *testing.T) {
	source := `<p>Hello world, good morning everyone.</p>`
	out := `<p>` + strings.Repeat("palavra ", 60) + `</p>`
	if err := ValidateBody(source, out); err == nil {
		t.Error("aceitou resposta com tamanho implausível")
	}
}

func TestValidateBody_allowsTextFirstFragment(t *testing.T) {
	// Fragmentos de página podem começar com texto solto (parágrafo cortado).
	source := `FOREWORD <span><img alt="x" src="a.jpg"/></span><p>Years ago a mobile phone was expensive.</p>`
	out := `PREFÁCIO <span><img alt="x" src="a.jpg"/></span><p>Anos atrás, um telefone celular era caro.</p>`
	if err := ValidateBody(source, out); err != nil {
		t.Errorf("validou errado: %v", err)
	}
}

func TestValidateBody_ignoresSpanDifferences(t *testing.T) {
	// Modelos fundem spans de drop-cap; visualmente inofensivo (o texto fica).
	source := `<h1>PART ONE</h1><p><span>G</span>ETTING ready for <span>it</span>.</p>`
	out := `<h1>PARTE UM</h1><p>PREPARANDO-se para isso.</p>`
	if err := ValidateBody(source, out); err != nil {
		t.Errorf("diferença de span deveria passar: %v", err)
	}
}

func TestValidateBody_stillRejectsLostNonSpanTag(t *testing.T) {
	source := `<p><b>Bold</b> and <i>italic</i> keep both tags.</p>`
	out := `<p><b>Negrito</b> e itálico mantêm as duas tags.</p>`
	if err := ValidateBody(source, out); err == nil {
		t.Error("perda de <i> deve continuar barrada")
	}
}

func TestLanguageName(t *testing.T) {
	cases := map[string]string{
		"pt-BR": "português do Brasil",
		"pt":    "português do Brasil",
		"en":    "inglês",
		"es":    "espanhol",
		"fr":    "francês",
		"de":    "alemão",
		"it":    "italiano",
		"ja":    "japonês",
	}
	for tag, want := range cases {
		if got := LanguageName(tag); got != want {
			t.Errorf("LanguageName(%q) = %q, want %q", tag, got, want)
		}
	}
}

func TestRivaLanguagePair(t *testing.T) {
	cases := []struct{ source, target, want string }{
		{"", "pt-BR", "en-pt-br"},
		{"en", "pt", "en-pt-br"},
		{"en-US", "es", "en-es-es"},
		{"ja", "zh-CN", "ja-zh-cn"},
	}
	for _, tc := range cases {
		if got := rivaLanguagePair(tc.source, tc.target); got != tc.want {
			t.Errorf("rivaLanguagePair(%q, %q) = %q, want %q", tc.source, tc.target, got, tc.want)
		}
	}
}

type fakeClient struct {
	responses []string
	calls     int
}

func (f *fakeClient) Translate(_ context.Context, _, _ string) (string, error) {
	response := f.responses[min(f.calls, len(f.responses)-1)]
	f.calls++
	return response, nil
}

func TestService_retriesUntilValid(t *testing.T) {
	client := &fakeClient{responses: []string{
		"Aqui está a tradução:\n<p>Olá mundo, bom dia a todas as pessoas.</p>",
		`<p>Olá mundo, bom dia a todas as pessoas.</p>`,
	}}
	service := Service{Client: client, Attempts: 3}

	out, err := service.TranslateBody(context.Background(), "pt-BR", `<p>Hello world, good morning everyone.</p>`)
	if err != nil {
		t.Fatal(err)
	}
	if out != `<p>Olá mundo, bom dia a todas as pessoas.</p>` {
		t.Errorf("out = %q", out)
	}
	if client.calls != 2 {
		t.Errorf("calls = %d, want 2", client.calls)
	}
}

type recordingClient struct {
	payloads []string
}

func (r *recordingClient) Translate(_ context.Context, _, content string) (string, error) {
	body := rivaSourceText(content)
	r.payloads = append(r.payloads, body)
	return strings.ReplaceAll(body, "Hello", "Olá"), nil
}

func TestService_translatesLongBodyInChunksUnderBudget(t *testing.T) {
	first := "<p>" + strings.Repeat("Hello world. ", 6) + "</p>"
	second := "<p>" + strings.Repeat("Hello there. ", 6) + "</p>"
	body := first + second
	client := &recordingClient{}
	service := Service{Client: client, Attempts: 1, MaxInputChars: utf8.RuneCountInString(first) + 8}

	out, err := service.TranslateBody(context.Background(), "pt-BR", body)
	if err != nil {
		t.Fatal(err)
	}
	if len(client.payloads) < 2 {
		t.Fatalf("chamadas = %d, capítulo grande precisa de mais de uma", len(client.payloads))
	}
	for i, payload := range client.payloads {
		if utf8.RuneCountInString(payload) > service.MaxInputChars {
			t.Errorf("payload %d tem %d runes, orçamento %d", i, utf8.RuneCountInString(payload), service.MaxInputChars)
		}
	}
	if err := ValidateBody(body, out); err != nil {
		t.Errorf("resultado juntado inválido: %v", err)
	}
}

// cappingClient imita o teto de saída do Gemini: devolve só os primeiros N
// nomes de tag (o INDEX do Total Recall morre assim em 64k tokens).
type cappingClient struct {
	maxTags int
	calls   int
}

func (c *cappingClient) Translate(_ context.Context, _, content string) (string, error) {
	c.calls++
	body := rivaSourceText(content)
	translated := strings.ReplaceAll(body, "topic", "tema")
	return keepFirstNonSpanTags(translated, c.maxTags), nil
}

func keepFirstNonSpanTags(html string, max int) string {
	matches := tagRE.FindAllStringIndex(html, -1)
	count := 0
	cut := len(html)
	for _, match := range matches {
		sub := tagRE.FindStringSubmatch(html[match[0]:match[1]])
		if len(sub) >= 3 && strings.EqualFold(sub[2], "span") {
			continue
		}
		count++
		if count >= max {
			cut = match[1]
			break
		}
	}
	return html[:cut]
}

func TestService_indexSurvivesOutputCapWhenChunked(t *testing.T) {
	body := indexLikeBody(80)
	sourceTags := len(tagNames(body))
	capTags := sourceTags * 2 / 3
	if capTags < 10 {
		t.Fatalf("fixture pequena demais: %d tags", sourceTags)
	}
	client := &cappingClient{maxTags: capTags}

	_, err := (Service{Client: client, Attempts: 1, MaxInputChars: 0}).TranslateBody(context.Background(), "pt-BR", body)
	if err == nil {
		t.Fatal("índice inteiro acima do teto de saída tinha que falhar")
	}
	if !strings.Contains(err.Error(), "tags alteradas") {
		t.Fatalf("queríamos tags alteradas, veio %v", err)
	}

	budget := 1500
	client = &cappingClient{maxTags: capTags}
	out, err := (Service{Client: client, Attempts: 1, MaxInputChars: budget}).TranslateBody(context.Background(), "pt-BR", body)
	if err != nil {
		t.Fatalf("fatiado abaixo do teto tinha que passar: %v", err)
	}
	if client.calls < 2 {
		t.Fatalf("calls = %d, índice precisa de mais de uma parte", client.calls)
	}
	if err := ValidateBody(body, out); err != nil {
		t.Fatalf("juntado inválido: %v", err)
	}
}

func TestService_doesNotRetryContextLengthError(t *testing.T) {
	client := &errorClient{err: fmt.Errorf("provedor respondeu 400: maximum context length is 8192 tokens")}
	_, err := (Service{Client: client, Attempts: 3}).TranslateBody(context.Background(), "pt-BR", "<p>Hello world.</p>")
	if err == nil {
		t.Fatal("esperava erro de contexto")
	}
	if client.calls != 1 {
		t.Errorf("calls = %d, overflow de contexto não deve repetir o mesmo payload", client.calls)
	}
}

func TestService_doesNotRetryOpenRouterNoUpstreams(t *testing.T) {
	client := &errorClient{err: fmt.Errorf(`provedor respondeu 503: {"error":{"message":"Provider returned error","code":503,"metadata":{"raw":"{\"error\":\"no upstreams available\"}","provider_name":"Recursive"}}}`)}
	_, err := (Service{Client: client, Attempts: 3}).TranslateBody(context.Background(), "pt-BR", "<p>Hello world.</p>")
	if err == nil {
		t.Fatal("esperava o 503 do provedor")
	}
	if client.calls != 1 {
		t.Errorf("calls = %d, sem GPU no upstream não deve queimar as 3 tentativas", client.calls)
	}
	if strings.Contains(err.Error(), "tradução inválida") {
		t.Errorf("503 não é tradução inválida: %v", err)
	}
	if !strings.Contains(err.Error(), "503") || !strings.Contains(err.Error(), "no upstreams") {
		t.Errorf("erro precisa continuar sendo o 503: %v", err)
	}
}

func TestService_failsAfterAttempts(t *testing.T) {
	// Sempre com preâmbulo: inválido em todas as tentativas.
	client := &fakeClient{responses: []string{"Aqui está:\n<p>Olá mundo.</p>"}}
	service := Service{Client: client, Attempts: 2}

	_, err := service.TranslateBody(context.Background(), "pt-BR", `<p>Hello world.</p>`)
	if err == nil {
		t.Fatal("esperava erro após as tentativas")
	}
	if client.calls != 2 {
		t.Errorf("calls = %d, want 2", client.calls)
	}
	if !strings.Contains(err.Error(), "tradução inválida") {
		t.Errorf("erro sem contexto: %v", err)
	}
}
