package ghonline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"github.com/ryanburnette/gitaware/internal/cache"
	"github.com/ryanburnette/gitaware/internal/model"
)

// Client shells out to gh.
type Client struct {
	GHBin   string
	Timeout time.Duration
	Cache   *cache.Store
}

// New returns a Client using gh on PATH.
func New(c *cache.Store) *Client {
	return &Client{
		GHBin:   "gh",
		Timeout: 60 * time.Second,
		Cache:   c,
	}
}

func (c *Client) bin() string {
	if c.GHBin == "" {
		return "gh"
	}
	return c.GHBin
}

// Available checks gh is on PATH.
func (c *Client) Available() error {
	cmd := exec.Command(c.bin(), "version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gh not available: %w", err)
	}
	return nil
}

// User is the authenticated GitHub user.
type User struct {
	Login string `json:"login"`
}

// WhoAmI returns the logged-in gh user.
func (c *Client) WhoAmI(ctx context.Context) (User, error) {
	var u User
	if err := c.ghJSON(ctx, &u, "api", "user"); err != nil {
		return u, err
	}
	return u, nil
}

// RemoteRepo is a subset of gh repo list JSON.
type RemoteRepo struct {
	Name             string `json:"name"`
	URL              string `json:"url"`
	IsFork           bool   `json:"isFork"`
	IsArchived       bool   `json:"isArchived"`
	DefaultBranchRef *struct {
		Name string `json:"name"`
	} `json:"defaultBranchRef"`
}

// ListOrgRepos lists repos for an org or user namespace.
func (c *Client) ListOrgRepos(ctx context.Context, org string, refresh bool) ([]RemoteRepo, error) {
	key := "repos-" + org
	var repos []RemoteRepo
	if c.Cache != nil && c.Cache.Get(key, &repos, refresh) {
		return repos, nil
	}

	// gh repo list works for users and orgs
	err := c.ghJSON(ctx, &repos,
		"repo", "list", org,
		"--limit", "1000",
		"--json", "name,url,isFork,isArchived,defaultBranchRef",
	)
	if err != nil {
		return nil, fmt.Errorf("gh repo list %s: %w", org, err)
	}
	if c.Cache != nil {
		_ = c.Cache.Put(key, repos)
	}
	return repos, nil
}

// PR is a pull request summary.
type PR struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
	State  string `json:"state"`
}

// OpenPR returns an open PR for head branch on org/repo, if any.
// Uses the git remote owner/name (not the local folder path). Tries bare
// branch head first; owner:branch is a fallback for cross-fork PRs.
func (c *Client) OpenPR(ctx context.Context, org, repo, branch string) (*model.PR, error) {
	if branch == "" || branch == "(detached)" {
		return nil, nil
	}
	ref := org + "/" + repo

	// Bare branch matches same-repo PRs (the common case). owner:branch often
	// returns an empty list even when a PR exists, so it is only a fallback.
	var prs []PR
	err := c.ghJSON(ctx, &prs,
		"pr", "list",
		"--repo", ref,
		"--head", branch,
		"--state", "open",
		"--json", "number,url,state",
		"--limit", "1",
	)
	if err != nil || len(prs) == 0 {
		var prs2 []PR
		err2 := c.ghJSON(ctx, &prs2,
			"pr", "list",
			"--repo", ref,
			"--head", org+":"+branch,
			"--state", "open",
			"--json", "number,url,state",
			"--limit", "1",
		)
		if err2 == nil && len(prs2) > 0 {
			prs = prs2
			err = nil
		} else if err != nil {
			return nil, err
		}
	}
	if len(prs) == 0 {
		return nil, nil
	}
	return &model.PR{Number: prs[0].Number, URL: prs[0].URL}, nil
}

// DefaultBranchMap returns name -> default branch for an org listing.
func DefaultBranchMap(repos []RemoteRepo) map[string]string {
	m := make(map[string]string, len(repos))
	for _, r := range repos {
		if r.DefaultBranchRef != nil && r.DefaultBranchRef.Name != "" {
			m[r.Name] = r.DefaultBranchRef.Name
		}
	}
	return m
}

// MissingClones diffs remote list against local repo names.
func MissingClones(org string, remote []RemoteRepo, localNames map[string]bool, noForks, includeArchived bool) []model.MissingRepo {
	var out []model.MissingRepo
	for _, r := range remote {
		if r.IsArchived && !includeArchived {
			continue
		}
		if r.IsFork && noForks {
			continue
		}
		if localNames[r.Name] {
			continue
		}
		def := ""
		if r.DefaultBranchRef != nil {
			def = r.DefaultBranchRef.Name
		}
		out = append(out, model.MissingRepo{
			Org:           org,
			Name:          r.Name,
			URL:           r.URL,
			DefaultBranch: def,
			IsFork:        r.IsFork,
			IsArchived:    r.IsArchived,
		})
	}
	return out
}

// RemoteID is host/owner/repo parsed from a git remote URL.
type RemoteID struct {
	Host  string // github.com, gitlab.com, …
	Owner string
	Name  string
}

// ParseRemote extracts host/owner/repo from a remote URL (any git host).
func ParseRemote(remoteURL string) (RemoteID, bool) {
	var id RemoteID
	remoteURL = strings.TrimSpace(remoteURL)
	if remoteURL == "" {
		return id, false
	}
	// git@host:owner/repo.git
	if strings.HasPrefix(remoteURL, "git@") {
		rest := strings.TrimPrefix(remoteURL, "git@")
		host, path, found := strings.Cut(rest, ":")
		if !found {
			return id, false
		}
		id.Host = host
		path = strings.TrimSuffix(path, ".git")
		parts := strings.Split(path, "/")
		if len(parts) >= 2 {
			id.Owner, id.Name = parts[0], parts[len(parts)-1]
			return id, true
		}
		return id, false
	}
	// ssh://git@host/owner/repo.git
	// https://host/owner/repo.git
	u, err := url.Parse(remoteURL)
	if err != nil {
		return id, false
	}
	id.Host = u.Hostname()
	p := strings.TrimPrefix(u.Path, "/")
	p = strings.TrimSuffix(p, ".git")
	parts := strings.Split(p, "/")
	if len(parts) >= 2 {
		id.Owner, id.Name = parts[0], parts[len(parts)-1]
		return id, true
	}
	return id, false
}

// ParseOwnerRepo extracts owner/repo from a remote URL.
func ParseOwnerRepo(remoteURL string) (owner, repo string, ok bool) {
	id, ok := ParseRemote(remoteURL)
	return id.Owner, id.Name, ok
}

// RemoteMismatch is true when origin owner/repo != path-derived org/name.
// Empty path org/name means no path identity — not a mismatch.
func RemoteMismatch(remoteURL, pathOrg, pathName string) bool {
	if pathOrg == "" && pathName == "" {
		return false
	}
	id, ok := ParseRemote(remoteURL)
	if !ok {
		return false
	}
	if pathOrg != "" && pathOrg != id.Owner {
		return true
	}
	if pathName != "" && pathName != id.Name {
		return true
	}
	return false
}

// Clone runs gh repo clone owner/name into dest (parent must exist).
func (c *Client) Clone(ctx context.Context, owner, name, dest string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.bin(), "repo", "clone", owner+"/"+name, dest)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("gh repo clone %s/%s: %s", owner, name, msg)
	}
	return nil
}

func (c *Client) ghJSON(ctx context.Context, dest any, args ...string) error {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.bin(), args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s", msg)
	}
	if err := json.Unmarshal(stdout.Bytes(), dest); err != nil {
		return fmt.Errorf("decode gh json: %w", err)
	}
	return nil
}
