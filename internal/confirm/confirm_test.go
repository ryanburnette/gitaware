package confirm

import (
	"bytes"
	"strings"
	"testing"
)

func TestAskYesFlag(t *testing.T) {
	if !Ask(ioDiscard{}, ioDiscard{}, nil, true, "fetch") {
		t.Fatal("yes flag should skip prompt")
	}
}

func TestAskDecline(t *testing.T) {
	in := strings.NewReader("n\n")
	var errBuf bytes.Buffer
	if Ask(ioDiscard{}, &errBuf, in, false, "fetch everything") {
		t.Fatal("expected decline")
	}
	if !strings.Contains(errBuf.String(), "WARNING") {
		t.Fatalf("missing warning: %s", errBuf.String())
	}
}

func TestAskAccept(t *testing.T) {
	in := strings.NewReader("Y\n")
	var errBuf bytes.Buffer
	if !Ask(ioDiscard{}, &errBuf, in, false, "fetch") {
		t.Fatal("expected accept")
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }
