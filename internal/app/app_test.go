package app

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ryanburnette/gitaware/internal/config"
	"github.com/ryanburnette/gitaware/internal/filter"
	"github.com/ryanburnette/gitaware/internal/gitlocal"
	"github.com/ryanburnette/gitaware/internal/progress"
)

func TestModeLabel(t *testing.T) {
	cases := []struct {
		name string
		opts config.Options
		mode filter.Mode
		want string
	}{
		{"offline status", config.Options{}, filter.ModeStatus, "offline"},
		{"online no remote check", config.Options{Online: true}, filter.ModeStatus, "online"},
		{"online ls-remote", config.Options{Online: true, CheckRemote: true}, filter.ModeArrive, "online+ls-remote"},
		{"online fetch", config.Options{Online: true, Fetch: true}, filter.ModeArrive, "online+fetch"},
		{"offline fetch", config.Options{Fetch: true}, filter.ModeStatus, "offline+fetch"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := modeLabel(c.opts, c.mode); got != c.want {
				t.Fatalf("modeLabel = %q, want %q", got, c.want)
			}
		})
	}
}

func TestIsIssues(t *testing.T) {
	if IsIssues(nil) {
		t.Fatal("nil should not be issues")
	}
	if !IsIssues(errIssues) {
		t.Fatal("errIssues should be issues")
	}
	if IsIssues(errors.New("some other error")) {
		t.Fatal("other errors should not be issues")
	}
}

// TestApplyModeDefaultsArriveNoMissing verifies the critical invariant that
// arrive mode does not opt into listing uncloned repos by default.
func TestApplyModeDefaultsArriveNoMissing(t *testing.T) {
	opts := config.Options{}
	applyModeDefaults(&opts, filter.ModeArrive)
	if opts.IncludeMissing {
		t.Fatal("arrive must not set IncludeMissing by default")
	}
	if !opts.Online {
		t.Fatal("arrive must be online")
	}
	if !opts.CheckRemote {
		t.Fatal("arrive must default to ls-remote freshness")
	}
	if opts.Fetch {
		t.Fatal("arrive must not force fetch")
	}
}

// TestApplyModeDefaultsArriveFetchDropsCheckRemote: --fetch already mutates, so
// ls-remote is redundant.
func TestApplyModeDefaultsArriveFetchDropsCheckRemote(t *testing.T) {
	opts := config.Options{Fetch: true}
	applyModeDefaults(&opts, filter.ModeArrive)
	if opts.CheckRemote {
		t.Fatal("arrive --fetch should not also set CheckRemote")
	}
}

// TestApplyModeDefaultsLeaveIgnoresBehind encodes the leave invariant: behind
// is never a leave-blocker.
func TestApplyModeDefaultsLeaveIgnoresBehind(t *testing.T) {
	opts := config.Options{}
	applyModeDefaults(&opts, filter.ModeLeave)
	if opts.IncludeBehind {
		t.Fatal("leave must not include behind")
	}
	if !opts.StrictBranch {
		t.Fatal("leave should be strict about branch by default")
	}
}

// TestBuildReportStatusLocal is a lightweight integration test: a temp git
// repo with dirty + untracked work produces an issue in offline status mode.
func TestBuildReportStatusLocal(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	repo := filepath.Join(root, "acme", "widget")
	mkdirT(t, filepath.Join(repo, ".git"))

	ctx := context.Background()
	git := gitlocal.New()
	mustRun(t, ctx, git, repo, "init", "-q")
	mustRun(t, ctx, git, repo, "config", "user.email", "t@t")
	mustRun(t, ctx, git, repo, "config", "user.name", "Test")
	writeT(t, filepath.Join(repo, "README.md"), "hello\n")
	mustRun(t, ctx, git, repo, "add", "README.md")
	mustRun(t, ctx, git, repo, "commit", "-q", "-m", "init")
	// dirty + untracked
	writeT(t, filepath.Join(repo, "README.md"), "changed\n")
	writeT(t, filepath.Join(repo, "new.txt"), "untracked\n")

	a := &App{Git: git}
	opts := config.Options{
		Roots:        []string{root},
		Root:         root,
		Layout:       config.LayoutDiscover,
		Depth:        3,
		NameMode:     config.NameRemote,
		Workers:      2,
		IncludeStash: true,
	}
	p := progress.New(nil, progress.Options{})
	report, err := a.BuildReport(ctx, opts, filter.ModeStatus, p)
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}
	if report.Summary.Repos != 1 {
		t.Fatalf("repos=%d want 1", report.Summary.Repos)
	}
	if report.Summary.Issues != 1 {
		t.Fatalf("issues=%d want 1 (dirty+untracked)", report.Summary.Issues)
	}
	if report.Mode != "offline" {
		t.Fatalf("mode=%q want offline", report.Mode)
	}
}

func mkdirT(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeT(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustRun(t *testing.T, ctx context.Context, _ *gitlocal.Runner, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(ctx, "git", full...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", args[0], err, out)
	}
}
