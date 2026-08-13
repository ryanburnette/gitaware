package gitlocal

import (
	"testing"
)

func TestParsePorcelainV2_CleanMain(t *testing.T) {
	in := []byte(`# branch.oid abcdef
# branch.head main
# branch.upstream origin/main
# branch.ab +0 -0
`)
	st, err := ParsePorcelainV2(in)
	if err != nil {
		t.Fatal(err)
	}
	if st.Branch != "main" {
		t.Fatalf("branch=%q", st.Branch)
	}
	if st.Upstream != "origin/main" {
		t.Fatalf("upstream=%q", st.Upstream)
	}
	if st.Ahead != 0 || st.Behind != 0 {
		t.Fatalf("ab=%d %d", st.Ahead, st.Behind)
	}
	if st.Dirty != 0 || st.Untracked != 0 {
		t.Fatalf("dirty/untracked")
	}
}

func TestParsePorcelainV2_DirtyAheadFeature(t *testing.T) {
	in := []byte(`# branch.oid abcdef
# branch.head feature/x
# branch.upstream origin/feature/x
# branch.ab +2 -1
1 .M N...
1 M. N...
? foo.txt
? bar.txt
`)
	st, err := ParsePorcelainV2(in)
	if err != nil {
		t.Fatal(err)
	}
	if st.Branch != "feature/x" {
		t.Fatalf("branch=%q", st.Branch)
	}
	if st.Ahead != 2 || st.Behind != 1 {
		t.Fatalf("ab=%d %d", st.Ahead, st.Behind)
	}
	if st.Dirty != 2 {
		t.Fatalf("dirty=%d", st.Dirty)
	}
	if st.Untracked != 2 {
		t.Fatalf("untracked=%d", st.Untracked)
	}
}

func TestParsePorcelainV2_NoUpstream(t *testing.T) {
	in := []byte(`# branch.oid abcdef
# branch.head wip
`)
	st, err := ParsePorcelainV2(in)
	if err != nil {
		t.Fatal(err)
	}
	if st.Upstream != "" {
		t.Fatalf("upstream=%q", st.Upstream)
	}
	if st.HasAB {
		t.Fatal("expected no ab")
	}
}

func TestParsePorcelainV2_Detached(t *testing.T) {
	in := []byte(`# branch.oid abcdef
# branch.head (detached)
`)
	st, err := ParsePorcelainV2(in)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Detached {
		t.Fatal("expected detached")
	}
}

func TestParseStashCount(t *testing.T) {
	in := []byte("stash@{0}: WIP\nstash@{1}: other\n")
	if n := ParseStashCount(in); n != 2 {
		t.Fatalf("n=%d", n)
	}
	if n := ParseStashCount(nil); n != 0 {
		t.Fatalf("n=%d", n)
	}
}
