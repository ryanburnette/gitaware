package gitlocal

import (
	"context"
	"fmt"
	"strings"
)

// RemoteFreshness is a non-mutating comparison of local HEAD vs remote tip.
// Uses git ls-remote (does not update refs or the object DB intentionally;
// if the tip object is already present locally we can count commits).
type RemoteFreshness struct {
	Remote     string // e.g. origin
	Branch     string // remote branch name
	RemoteSHA  string
	HeadSHA    string
	InSync     bool
	Ahead      int // commits local has that remote tip lacks; -1 = unknown
	Behind     int // commits remote has that local lacks; -1 = unknown
	NoRemote   bool
	NoBranch   bool
	ObjectMiss bool // remote tip not in local object DB (need fetch to count)
}

// CheckRemote compares HEAD to the remote-tracking upstream branch tip via ls-remote.
// Does not run fetch; safe for read-only "are we out of date?" checks.
func (r *Runner) CheckRemote(ctx context.Context, dir, upstream string) (RemoteFreshness, error) {
	var out RemoteFreshness
	out.Ahead, out.Behind = -1, -1

	if upstream == "" {
		// fall back to origin + current branch
		br, err := r.run(ctx, dir, "branch", "--show-current")
		if err != nil || strings.TrimSpace(string(br)) == "" {
			out.NoBranch = true
			return out, nil
		}
		upstream = "origin/" + strings.TrimSpace(string(br))
	}

	remote, branch, ok := splitUpstream(upstream)
	if !ok {
		out.NoBranch = true
		return out, nil
	}
	out.Remote, out.Branch = remote, branch

	headOut, err := r.run(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return out, err
	}
	out.HeadSHA = strings.TrimSpace(string(headOut))

	// Non-mutating probe of the remote.
	ls, err := r.run(ctx, dir, "ls-remote", "--refs", remote, "refs/heads/"+branch)
	if err != nil {
		return out, fmt.Errorf("ls-remote %s: %w", remote, err)
	}
	sha := parseLSRemoteSHA(string(ls), "refs/heads/"+branch)
	if sha == "" {
		// branch may not exist on remote yet (unpushed new branch)
		out.NoBranch = true
		return out, nil
	}
	out.RemoteSHA = sha

	if sha == out.HeadSHA {
		out.InSync = true
		out.Ahead, out.Behind = 0, 0
		return out, nil
	}

	// If we already have the object (e.g. from an earlier fetch), count without fetching.
	if !r.objectExists(ctx, dir, sha) {
		out.ObjectMiss = true
		// Still know we differ; treat as behind-ish for awareness (remote moved or we diverged).
		out.Behind = 1
		return out, nil
	}

	aheadOut, err := r.run(ctx, dir, "rev-list", "--count", sha+"..HEAD")
	if err != nil {
		out.ObjectMiss = true
		out.Behind = 1
		return out, nil
	}
	behindOut, err := r.run(ctx, dir, "rev-list", "--count", "HEAD.."+sha)
	if err != nil {
		out.ObjectMiss = true
		out.Behind = 1
		return out, nil
	}
	fmt.Sscanf(strings.TrimSpace(string(aheadOut)), "%d", &out.Ahead)
	fmt.Sscanf(strings.TrimSpace(string(behindOut)), "%d", &out.Behind)
	return out, nil
}

func (r *Runner) objectExists(ctx context.Context, dir, sha string) bool {
	_, err := r.run(ctx, dir, "cat-file", "-e", sha+"^{commit}")
	return err == nil
}

func splitUpstream(upstream string) (remote, branch string, ok bool) {
	// origin/main or origin/feature/x
	upstream = strings.TrimSpace(upstream)
	i := strings.IndexByte(upstream, '/')
	if i <= 0 || i == len(upstream)-1 {
		return "", "", false
	}
	return upstream[:i], upstream[i+1:], true
}

func parseLSRemoteSHA(out, wantRef string) string {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// <sha>TAB<ref>
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		if parts[1] == wantRef {
			return parts[0]
		}
	}
	// single-line result for exact ref query
	parts := strings.Fields(strings.TrimSpace(out))
	if len(parts) >= 1 && len(parts[0]) >= 40 {
		return parts[0]
	}
	return ""
}
