package gitlocal

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Runner executes git commands. Tests may substitute a fake.
type Runner struct {
	GitBin  string
	Timeout time.Duration
}

// New returns a Runner using git on PATH.
func New() *Runner {
	return &Runner{
		GitBin:  "git",
		Timeout: 30 * time.Second,
	}
}

func (r *Runner) bin() string {
	if r.GitBin == "" {
		return "git"
	}
	return r.GitBin
}

func (r *Runner) timeout() time.Duration {
	if r.Timeout <= 0 {
		return 30 * time.Second
	}
	return r.Timeout
}

// run executes git -C dir args...
func (r *Runner) run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout())
	defer cancel()

	full := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(ctx, r.bin(), full...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return stdout.Bytes(), fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return stdout.Bytes(), nil
}

// IsWorkTree returns true if dir is a git working tree.
func (r *Runner) IsWorkTree(ctx context.Context, dir string) bool {
	out, err := r.run(ctx, dir, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}

// Status runs porcelain v2 branch status.
func (r *Runner) Status(ctx context.Context, dir string) (Status, error) {
	out, err := r.run(ctx, dir, "status", "--porcelain=v2", "--branch", "--untracked-files=normal")
	if err != nil {
		return Status{}, err
	}
	return ParsePorcelainV2(out)
}

// StashCount returns the number of stash entries.
func (r *Runner) StashCount(ctx context.Context, dir string) (int, error) {
	out, err := r.run(ctx, dir, "stash", "list")
	if err != nil {
		// stash may fail in odd repos; treat as zero
		return 0, nil
	}
	return ParseStashCount(out), nil
}

// RemoteURL returns origin fetch URL, or empty if missing.
func (r *Runner) RemoteURL(ctx context.Context, dir string) (string, bool, error) {
	out, err := r.run(ctx, dir, "remote")
	if err != nil {
		return "", false, err
	}
	remotes := strings.Fields(string(out))
	if len(remotes) == 0 {
		return "", false, nil
	}
	// prefer origin
	name := remotes[0]
	for _, rm := range remotes {
		if rm == "origin" {
			name = "origin"
			break
		}
	}
	urlOut, err := r.run(ctx, dir, "remote", "get-url", name)
	if err != nil {
		return "", true, nil
	}
	return strings.TrimSpace(string(urlOut)), true, nil
}

// Fetch runs git fetch --all --prune --quiet.
func (r *Runner) Fetch(ctx context.Context, dir string) error {
	_, err := r.run(ctx, dir, "fetch", "--all", "--prune", "--quiet")
	return err
}

// IgnoreConfigured is true when gitaware.ignore=true in local config.
func (r *Runner) IgnoreConfigured(ctx context.Context, dir string) bool {
	out, err := r.run(ctx, dir, "config", "--bool", "--get", "gitaware.ignore")
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}

// Available reports whether git is executable.
func (r *Runner) Available() error {
	cmd := exec.Command(r.bin(), "version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git not available: %w", err)
	}
	return nil
}
