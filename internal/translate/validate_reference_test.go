package translate

import (
	"strings"
	"testing"
)

// fixture de prosa: sem marcadores de bibliografia (densidade ~0).
func proseSource() string {
	return strings.Repeat(`<p>The meeting dragged on as both sides repeated their positions without listening to the other, and the room filled with the particular silence of an argument that had stopped being about the subject and had become about who would speak last in that discussion.</p>
`, 10)
}

// fixture de notas: citações com anos, periódicos e editoras (densidade alta).
func notesSource() string {
	return strings.Repeat(`<p>12. See R. B. Cialdini, Influence: Science and Practice, 5th ed. (Boston: Pearson, 2009), pp. 45-48.</p>
<p>13. S. Milgram, Behavioral Study of Obedience, Journal of Abnormal and Social Psychology 67 (1963): 371-78.</p>
<p>14. L. Festinger, A Theory of Cognitive Dissonance (Stanford, CA: Stanford University Press, 1957), 3-12.</p>
`, 8)
}

const translatedClosingNote = "As notas desta edição foram reorganizadas para acompanhar a numeração dos capítulos do livro, e cada entrada remete à página correspondente da edição impressa original, de modo que o leitor consiga localizar rapidamente a passagem citada sempre que quiser conferir o contexto completo da pesquisa mencionada pelo autor ao longo do texto e das notas de rodapé."

func TestValidateBody_skipsMixedGateForReferenceDenseSource(t *testing.T) {
	source := notesSource()

	// Resposta legítima: o corpo é traduzido, mas títulos, periódicos e
	// editoras ficam no original (regra 3 do prompt) — 40+ palavras copiadas.
	// Troca de "See"→"Ver" e uma nota longa traduzida no lugar de uma citação,
	// mantendo o mesmo número de tags.
	lines := strings.Split(notesSource(), "\n")
	outLines := make([]string, len(lines))
	for i, line := range lines {
		outLines[i] = strings.ReplaceAll(line, "See ", "Ver ")
	}
	outLines[len(outLines)-2] = "<p>" + translatedClosingNote + "</p>"
	out := strings.Join(outLines, "\n")

	if err := ValidateBody(source, out); err != nil {
		t.Fatalf("notas com citações em inglês deveriam passar: %v", err)
	}
}

func TestValidateBody_stillRejectsMixedLanguageInProse(t *testing.T) {
	source := proseSource()

	// Eco parcial em prosa: um parágrafo inteiro volta igual, o resto traduz.
	paragraphs := strings.Split(source, "</p>")
	out := paragraphs[0] + "</p>" + strings.Repeat("<p>A reunião se arrastou enquanto os dois lados repetiam suas posições sem ouvir o outro, e a sala se encheu daquele silêncio particular de uma discussão que havia deixado de ser sobre o assunto e passado a ser sobre quem falaria por último.</p>", len(paragraphs)-2)

	if err := ValidateBody(source, out); err == nil || !strings.Contains(err.Error(), "texto misturado") {
		t.Fatalf("eco em prosa deveria continuar barrado, err = %v", err)
	}
}

func TestReferenceDense(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		{"prosa comum", plainText(proseSource()), false},
		{"notas com citações", plainText(notesSource()), true},
		{"poucas palavras com poucos marcadores", "Kotler, P. Marketing Management. 2003. Prentice Hall.", false},
	}
	for _, tc := range cases {
		if got := referenceDense(tc.text); got != tc.want {
			t.Errorf("%s: referenceDense = %v, want %v", tc.name, got, tc.want)
		}
	}
}
