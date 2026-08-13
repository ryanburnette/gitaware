package progress

import (
	"bytes"
	"strings"
	"testing"
)

func TestProgressSteps(t *testing.T) {
	var buf bytes.Buffer
	p := New(&buf, Options{Enabled: true, Color: false})
	p.Start("prs")
	p.Step("scan", "39 repos")
	p.StepDone("scan", "39 repos")
	p.End("2 open PRs")

	out := buf.String()
	for _, want := range []string{"gitaware", "prs", "scan", "39 repos", "2 open PRs"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestProgressDisabled(t *testing.T) {
	var buf bytes.Buffer
	p := New(&buf, Options{Enabled: false})
	p.Start("prs")
	p.Step("scan", "x")
	p.End("done")
	if buf.Len() != 0 {
		t.Fatalf("expected silence, got %q", buf.String())
	}
}
