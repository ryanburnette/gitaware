package scan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ryanburnette/gitaware/internal/config"
	"github.com/ryanburnette/gitaware/internal/model"
)

func TestDiscoverOrgRepo(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "acme", "one", ".git"))
	mustMkdir(t, filepath.Join(root, "acme", "two")) // not a repo
	mustMkdir(t, filepath.Join(root, "beta", "three", ".git"))
	mustMkdir(t, filepath.Join(root, "beta", "four"))
	if err := os.WriteFile(filepath.Join(root, "beta", "four", ".git"), []byte("gitdir: /tmp/fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := config.Options{Layout: config.LayoutOrgRepo, Roots: []string{root}}
	entries, err := DiscoverRoot(root, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("len=%d %+v", len(entries), entries)
	}

	opts.OrgFilter = "acme"
	opts.WarnNonRepos = true
	entries, err = DiscoverRoot(root, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("acme with warn: %d", len(entries))
	}
}

func TestDiscoverFlat(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "app", ".git"))
	mustMkdir(t, filepath.Join(root, "lib", ".git"))
	opts := config.Options{Layout: config.LayoutFlat}
	entries, err := DiscoverRoot(root, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("len=%d", len(entries))
	}
	if entries[0].Org != "" {
		t.Fatalf("flat should have empty org: %+v", entries[0])
	}
}

func TestDiscoverDiscover(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "Projects", "app", ".git"))
	mustMkdir(t, filepath.Join(root, "other", "nested", "repo", ".git"))
	opts := config.Options{Layout: config.LayoutDiscover, Depth: 3}
	entries, err := DiscoverRoot(root, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("len=%d %+v", len(entries), entries)
	}
}

func TestGroupByOrg(t *testing.T) {
	repos := []model.Repo{
		{Org: "b", Name: "1"},
		{Org: "a", Name: "1"},
		{Org: "a", Name: "2"},
		{Name: "solo", RemoteOwner: "z"},
	}
	orgs := GroupByOrg(repos)
	if len(orgs) != 3 {
		t.Fatalf("%+v", orgs)
	}
}

func TestDisplayName(t *testing.T) {
	r := model.Repo{
		Org: "other", Name: "app", PathOrg: "acme", PathName: "app",
		RemoteOwner: "other", RemoteName: "app", Host: "github.com", HasRemote: true,
	}
	if g := DisplayName(r, config.NamePath); g != "acme/app" {
		t.Fatal(g)
	}
	if g := DisplayName(r, config.NameRemote); g != "other/app" {
		t.Fatal(g)
	}
	if g := DisplayName(r, config.NameFolder); g != "app" {
		t.Fatal(g)
	}
	// auto prefers remote identity
	if g := DisplayName(r, config.NameAuto); g != "other/app" {
		t.Fatal(g)
	}
	// non-github host included
	gl := model.Repo{Host: "gitlab.com", RemoteOwner: "g", RemoteName: "p", HasRemote: true}
	if g := DisplayName(gl, config.NameRemote); g != "gitlab.com/g/p" {
		t.Fatal(g)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}
