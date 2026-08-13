package gitlocal

import "testing"

func TestParseLSRemoteSHA(t *testing.T) {
	out := "abcdef0123456789abcdef0123456789abcdef01\trefs/heads/main\n"
	got := parseLSRemoteSHA(out, "refs/heads/main")
	if got != "abcdef0123456789abcdef0123456789abcdef01" {
		t.Fatalf("got %q", got)
	}
}

func TestSplitUpstream(t *testing.T) {
	r, b, ok := splitUpstream("origin/feature/x")
	if !ok || r != "origin" || b != "feature/x" {
		t.Fatalf("%s %s %v", r, b, ok)
	}
}
