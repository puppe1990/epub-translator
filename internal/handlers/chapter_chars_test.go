package handlers

import (
	"strings"
	"testing"

	"github.com/puppe1990/epub-translator/internal/models"
)

func TestChapterListShowsFormattedSourceChars(t *testing.T) {
	_, list, err := RenderBookFragments(models.Book{ID: 1}, []models.Chapter{
		{ID: 1, Position: 1, Title: "Curto", Status: "done", SourceChars: 10},
		{ID: 2, Position: 2, Title: "Longo", Status: "pending", SourceChars: 12345},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Caracteres", "10", "12.345"} {
		if !strings.Contains(list, want) {
			t.Errorf("listagem sem %q", want)
		}
	}
}

func TestFormatChapterCharsUsesBrazilianThousands(t *testing.T) {
	cases := map[int]string{0: "0", 9: "9", 999: "999", 1000: "1.000", 12345: "12.345", 1234567: "1.234.567"}
	for n, want := range cases {
		if got := formatChapterChars(n); got != want {
			t.Errorf("formatChapterChars(%d) = %q, want %q", n, got, want)
		}
	}
}
