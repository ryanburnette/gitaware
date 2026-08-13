package render

import (
	"strings"
	"testing"
)

func TestHyperlinkEnabled(t *testing.T) {
	got := Hyperlink(true, "https://example.com/x", "PR #3")
	if !strings.Contains(got, "https://example.com/x") {
		t.Fatalf("missing url: %q", got)
	}
	if !strings.Contains(got, "PR #3") {
		t.Fatalf("missing text: %q", got)
	}
	if !strings.Contains(got, "\033]8;;") {
		t.Fatalf("missing osc8: %q", got)
	}
}

func TestHyperlinkDisabled(t *testing.T) {
	got := Hyperlink(false, "https://example.com/x", "PR #3")
	if got != "PR #3" {
		t.Fatalf("got %q", got)
	}
}

func TestPRLink(t *testing.T) {
	th := NewTheme(true)
	got := PRLink(th, 3, "https://github.com/o/r/pull/3", true)
	if !strings.Contains(got, "pull/3") {
		t.Fatalf("missing url: %q", got)
	}
	if !strings.Contains(got, "PR #3") {
		t.Fatalf("missing label: %q", got)
	}
}
