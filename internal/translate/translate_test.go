package translate

import (
	"context"
	"strings"
	"testing"
)

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
