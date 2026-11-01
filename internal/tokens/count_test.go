package tokens

import "testing"

func TestCount_helloIsOneCl100kToken(t *testing.T) {
	if got := Count("hello"); got != 1 {
		t.Errorf("Count(%q) = %d, want 1", "hello", got)
	}
}
