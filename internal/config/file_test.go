package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingReturnsDefaults(t *testing.T) {
	f, ok, err := LoadFile(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if f.Layout != LayoutDiscover {
		t.Fatalf("layout=%s", f.Layout)
	}
}

func TestLoadAndResolveShowAll(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	f := SampleOrgRepo()
	f.Display.ShowAll = true
	if err := WriteFile(path, f); err != nil {
		t.Fatal(err)
	}
	// point loader via env
	t.Setenv("GITAWARE_CONFIG", path)
	// ensure root exists
	git := filepath.Join(dir, "git")
	if err := os.MkdirAll(filepath.Join(git, "o", "r"), 0o755); err != nil {
		t.Fatal(err)
	}
	// rewrite roots to temp
	f.Roots = []string{git}
	if err := WriteFile(path, f); err != nil {
		t.Fatal(err)
	}

	var opts Options
	if err := Resolve(&opts); err != nil {
		t.Fatal(err)
	}
	if !opts.All || opts.IssuesOnly {
		t.Fatalf("show_all not applied: all=%v issues=%v", opts.All, opts.IssuesOnly)
	}

	opts = Options{IssuesFlag: true}
	if err := Resolve(&opts); err != nil {
		t.Fatal(err)
	}
	if opts.All {
		t.Fatal("--issues should win")
	}
}

func TestResolveRootFlag(t *testing.T) {
	dir := t.TempDir()
	var opts Options
	opts.Root = dir
	if err := Resolve(&opts); err != nil {
		t.Fatal(err)
	}
	if len(opts.Roots) != 1 || opts.Roots[0] != ExpandRoot(dir) {
		t.Fatalf("%v", opts.Roots)
	}
}
