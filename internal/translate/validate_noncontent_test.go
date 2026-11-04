package translate

import (
	"strings"
	"testing"
)

func TestValidateBody_ignoresHTMLCommentsInMixedGate(t *testing.T) {
	comment := `<!-- PROD: Uncomment the following sentence if appropriate and add it to the above paragraph. This book is not intended as legal, medical, financial, or professional advice, and readers should consult a qualified professional when they need it. Marketing and sales teams should coordinate any promotional use with the publisher before printing. -->`
	source := `<p>Harness Engineering by Nicole Koenigstein.</p>
<p>Copyright © 2028 O'Reilly Media, Inc. All rights reserved.</p>
<p>Published by O'Reilly Media, Inc., 141 Stony Circle, Suite 195, Santa Rosa, CA 95401.</p>
` + comment + `
<p>O'Reilly books may be purchased for educational, business, or sales promotional use, and online editions are available for most titles through the publisher catalog, while corporate customers can reach the sales department directly.</p>`

	// O modelo repassa o comentário intacto (não é conteúdo visível) e traduz
	// o corpo; endereço/ISBN permanecem por natureza.
	out := `<p>Harness Engineering por Nicole Koenigstein.</p>
<p>Copyright © 2028 O'Reilly Media, Inc. Todos os direitos reservados.</p>
<p>Publicado pela O'Reilly Media, Inc., 141 Stony Circle, Suite 195, Santa Rosa, CA 95401.</p>
` + comment + `
<p>Os livros da O'Reilly podem ser adquiridos para uso educacional, comercial ou promocional de vendas, e as edições online estão disponíveis para a maioria dos títulos pelo catálogo da editora, enquanto clientes corporativos podem falar diretamente com o departamento de vendas.</p>`

	if err := ValidateBody(source, out); err != nil {
		t.Fatalf("página de copyright traduzida deveria passar: %v", err)
	}
}

func TestValidateBody_ignoresCodeBlocksInMixedGate(t *testing.T) {
	code := `<pre>system: You are a careful reviewer for a production codebase.
Read the pull request diff and list every risk you find.
Explain why each risk matters for production users before suggesting changes.
Never propose a change without a rollback plan and a test that proves the fix.
Prefer small, reversible steps that keep the harness green at all times.</pre>`
	source := `<p>A retrieval pipeline should keep the scoring function small, predictable, and easy to test, because every downstream agent inherits its ranking behavior and any hidden state inside it becomes a debugging nightmare for the whole team.</p>
` + code + `
<p>Once the retrieval step is stable you can measure precision with a small golden set of questions and expected passages, rerunning the harness after every prompt change so regressions surface immediately instead of in production.</p>`

	// Código e prompts não se traduzem: o modelo os repassa idênticos e
	// traduz só a prosa.
	out := `<p>Uma pipeline de retrieval deve manter a função de pontuação pequena, previsível e fácil de testar, porque todo agente a jusante herda o seu comportamento de ranking e qualquer estado escondido dentro dela vira um pesadelo de depuração para o time inteiro.</p>
` + code + `
<p>Assim que o passo de retrieval estiver estável, dá para medir a precisão com um pequeno conjunto de ouro de perguntas e trechos esperados, rodando o harness de novo após cada mudança de prompt para que as regressões apareçam na hora em vez de em produção.</p>`

	if err := ValidateBody(source, out); err != nil {
		t.Fatalf("capítulo técnico com código intacto deveria passar: %v", err)
	}
}

func TestPlainText_stripsCommentsAndCode(t *testing.T) {
	html := `<p>Visível</p><!-- comentário longo que não é conteúdo --><code>fmt.Println("x")</code><pre>code line one two three</pre>`
	text := plainText(html)
	if text != "Visível" {
		t.Fatalf("plainText = %q, want apenas o texto visível", text)
	}
	if !strings.Contains(text, "Visível") {
		t.Fatal("texto visível sumiu")
	}
}
