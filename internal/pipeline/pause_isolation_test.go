package pipeline

import "testing"

func TestPauseBookDoesNotRequeueOtherBooks(t *testing.T) {
	s := newTestStore(t)
	firstBook, secondBook := seedBook(t, s), seedBook(t, s)
	firstChapters, _ := s.ListChapters(firstBook)
	secondChapters, _ := s.ListChapters(secondBook)
	_ = s.SetChapterRunning(firstChapters[0].ID)
	_ = s.SetChapterRunning(secondChapters[0].ID)
	if err := s.PauseBook(firstBook); err != nil {
		t.Fatal(err)
	}
	first, _ := s.FindChapter(firstChapters[0].ID)
	second, _ := s.FindChapter(secondChapters[0].ID)
	if first.Status != "paused" || second.Status != "running" {
		t.Errorf("pausa deve afetar só seu livro: primeiro=%q, segundo=%q", first.Status, second.Status)
	}
}
