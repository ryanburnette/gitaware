package app

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"

	"github.com/ryanburnette/gitaware/internal/config"
	"github.com/ryanburnette/gitaware/internal/filter"
	"github.com/ryanburnette/gitaware/internal/ghonline"
	"github.com/ryanburnette/gitaware/internal/gitlocal"
	"github.com/ryanburnette/gitaware/internal/model"
	"github.com/ryanburnette/gitaware/internal/progress"
)

// enrichLocal fills git status fields for each repo in parallel.
func enrichLocal(ctx context.Context, repos []model.Repo, git *gitlocal.Runner, opts config.Options, p *progress.Progress) []model.Repo {
	out := make([]model.Repo, len(repos))
	copy(out, repos)

	workers := int64(opts.Workers)
	if workers < 1 {
		workers = int64(config.DefaultWorkers())
	}
	sem := semaphore.NewWeighted(workers)
	var mu sync.Mutex
	var done atomic.Int64
	total := 0
	for _, r := range out {
		if r.IsRepo {
			total++
		}
	}
	if total > 0 {
		action := "status"
		if opts.Fetch {
			action = "fetch+status"
		} else if opts.CheckRemote {
			action = "status+remote"
		}
		p.Stepf(action, "0/%d", total)
	}

	g, ctx := errgroup.WithContext(ctx)
	for i := range out {
		i := i
		r := &out[i]
		if !r.IsRepo {
			continue
		}
		g.Go(func() error {
			if err := sem.Acquire(ctx, 1); err != nil {
				return err
			}
			defer sem.Release(1)

			if git.IgnoreConfigured(ctx, r.Path) {
				mu.Lock()
				r.Err = ""
				r.OK = true
				r.Branch = "(ignored)"
				mu.Unlock()
				n := done.Add(1)
				p.Livef("status", "%d/%d", n, total)
				return nil
			}

			if opts.Fetch {
				if err := git.Fetch(ctx, r.Path); err != nil {
					p.VerboseDetail(fmt.Sprintf("fetch %s: %v", r.DisplayName(), err))
				}
			}

			st, err := git.Status(ctx, r.Path)
			if err != nil {
				mu.Lock()
				r.Err = err.Error()
				mu.Unlock()
				n := done.Add(1)
				p.Livef("status", "%d/%d", n, total)
				return nil
			}

			var stash int
			if opts.IncludeStash {
				stash, _ = git.StashCount(ctx, r.Path)
			}

			url, hasRemote, _ := git.RemoteURL(ctx, r.Path)
			var host, owner, rname string
			if hasRemote && url != "" {
				if id, ok := ghonline.ParseRemote(url); ok {
					host, owner, rname = id.Host, id.Owner, id.Name
				}
			}

			var fresh gitlocal.RemoteFreshness
			var freshErr error
			if opts.CheckRemote && !opts.Fetch && hasRemote && !st.Detached {
				fresh, freshErr = git.CheckRemote(ctx, r.Path, st.Upstream)
				if freshErr != nil {
					p.VerboseDetail(fmt.Sprintf("ls-remote %s: %v", r.DisplayName(), freshErr))
				}
			}

			mu.Lock()
			r.Branch = st.Branch
			r.Upstream = st.Upstream
			r.Ahead = st.Ahead
			r.Behind = st.Behind
			r.Dirty = st.Dirty
			r.Untracked = st.Untracked
			r.Detached = st.Detached
			r.Stash = stash
			r.HasRemote = hasRemote
			r.RemoteURL = url
			r.Host = host
			r.RemoteOwner = owner
			r.RemoteName = rname
			// Identity from remote; path fields stay as PathOrg/PathName.
			if owner != "" {
				if ghonline.RemoteMismatch(url, r.PathOrg, r.PathName) {
					r.Signals = append(r.Signals, model.SignalRemoteMismatch)
				}
				r.Org = owner
				r.Name = rname
			} else if r.Name == "" {
				r.Name = r.PathName
				if r.Name == "" {
					r.Name = filepath.Base(r.Path)
				}
			}
			// Apply non-mutating remote freshness (overrides stale tracking ahead/behind when known).
			if opts.CheckRemote && !opts.Fetch && freshErr == nil && fresh.RemoteSHA != "" {
				r.RemoteChecked = true
				if fresh.InSync {
					r.Ahead, r.Behind = 0, 0
					r.RemoteNewer = false
				} else if !fresh.ObjectMiss && fresh.Ahead >= 0 && fresh.Behind >= 0 {
					r.Ahead, r.Behind = fresh.Ahead, fresh.Behind
					r.RemoteNewer = fresh.Behind > 0
				} else if fresh.Behind > 0 || fresh.ObjectMiss {
					// Differ from remote; full count needs objects (optional later fetch).
					r.RemoteNewer = fresh.RemoteSHA != fresh.HeadSHA
					if fresh.Behind > 0 {
						r.Behind = fresh.Behind
					}
				}
			}
			mu.Unlock()

			n := done.Add(1)
			p.Livef("status", "%d/%d %s", n, total, r.DisplayName())
			return nil
		})
	}
	_ = g.Wait()

	if total > 0 {
		label := "status"
		if opts.Fetch {
			label = "fetch+status"
		} else if opts.CheckRemote {
			label = "status+remote"
		}
		p.StepDone(label, fmt.Sprintf("%d repos", total))
	}

	// drop ignored
	filtered := out[:0]
	for _, r := range out {
		if r.Branch == "(ignored)" {
			continue
		}
		filtered = append(filtered, r)
	}
	return filtered
}

// enrichOnline adds default branches, PRs, missing clones.
func enrichOnline(ctx context.Context, repos []model.Repo, opts config.Options, gh *ghonline.Client, p *progress.Progress) ([]model.Repo, []model.MissingRepo, error) {
	if gh == nil {
		return repos, nil, fmt.Errorf("gh client required for online mode")
	}
	if err := gh.Available(); err != nil {
		return repos, nil, err
	}

	// group local names by GitHub owner (path org, remote owner, or config)
	type orgSet struct {
		names map[string]bool
	}
	byOrg := map[string]*orgSet{}
	var orgOrder []string
	addOrg := func(org string) *orgSet {
		if org == "" {
			return nil
		}
		if _, ok := byOrg[org]; !ok {
			byOrg[org] = &orgSet{names: map[string]bool{}}
			orgOrder = append(orgOrder, org)
		}
		return byOrg[org]
	}
	for _, org := range opts.GitHubOrgs {
		addOrg(org)
	}
	for _, r := range repos {
		owner, name := ghRepo(&r)
		if owner == "" {
			owner = r.Org
			name = r.Name
		}
		s := addOrg(owner)
		if s != nil && r.IsRepo && name != "" {
			s.names[name] = true
		}
		// also index path org so folder-local names match missing diffs
		if r.Org != "" && r.Org != owner {
			if ps := addOrg(r.Org); ps != nil && r.IsRepo {
				ps.names[r.Name] = true
			}
		}
	}

	var allMissing []model.MissingRepo
	defMaps := map[string]map[string]string{}

	p.Stepf("github", "listing %d orgs", len(orgOrder))
	for i, org := range orgOrder {
		p.Livef("github", "%d/%d %s", i+1, len(orgOrder), org)
		remote, err := gh.ListOrgRepos(ctx, org, opts.Refresh)
		if err != nil {
			p.Warn(fmt.Sprintf("list %s: %v", org, err))
			continue
		}
		defMaps[org] = ghonline.DefaultBranchMap(remote)
		// Missing clones only when explicitly requested (missing command / --missing).
		// Arrive/online status should not dump every uncloned org repo.
		if opts.IncludeMissing {
			miss := ghonline.MissingClones(org, remote, byOrg[org].names, opts.NoForks, opts.IncludeArchived)
			allMissing = append(allMissing, miss...)
		}
	}
	if opts.IncludeMissing {
		p.StepDone("github", fmt.Sprintf("%d missing clones", len(allMissing)))
	} else {
		p.StepDone("github", fmt.Sprintf("%d orgs", len(orgOrder)))
	}

	// apply default branches
	for i := range repos {
		if dm, ok := defMaps[repos[i].Org]; ok {
			if d, ok := dm[repos[i].Name]; ok {
				repos[i].DefaultBranch = d
			}
		}
	}

	// Collect PR candidates
	type cand struct {
		idx         int
		owner, name string
		branch      string
		display     string
	}
	var cands []cand
	for i := range repos {
		r := &repos[i]
		if !r.IsRepo || r.Detached || r.Branch == "" {
			continue
		}
		owner, name := ghRepo(r)
		if owner == "" || name == "" {
			continue
		}
		if !opts.CheckAllPRs {
			// skip PR lookup for clean default branch to save API calls
			isDef := config.IsDefaultBranch(r.Branch, config.DefaultBranches)
			if r.DefaultBranch != "" {
				isDef = r.Branch == r.DefaultBranch
			}
			if isDef && r.Dirty == 0 && r.Ahead == 0 {
				continue
			}
		}
		cands = append(cands, cand{
			idx: i, owner: owner, name: name, branch: r.Branch,
			display: owner + "/" + name,
		})
	}

	if len(cands) == 0 {
		p.Step("prs", "nothing to check")
		return repos, allMissing, nil
	}

	p.Stepf("prs", "0/%d branches", len(cands))
	workers := int64(opts.Workers)
	if workers < 1 {
		workers = 8
	}
	sem := semaphore.NewWeighted(workers)
	var mu sync.Mutex
	var done atomic.Int64
	var found atomic.Int64
	var failed atomic.Int64

	g, ctx := errgroup.WithContext(ctx)
	for _, c := range cands {
		c := c
		g.Go(func() error {
			if err := sem.Acquire(ctx, 1); err != nil {
				return err
			}
			defer sem.Release(1)

			pr, err := gh.OpenPR(ctx, c.owner, c.name, c.branch)
			n := done.Add(1)
			if err != nil {
				failed.Add(1)
				p.VerboseDetail(fmt.Sprintf("%s: %v", c.display, err))
				p.Livef("prs", "%d/%d %s", n, len(cands), c.display)
				return nil
			}
			if pr != nil {
				found.Add(1)
				mu.Lock()
				repos[c.idx].PR = pr
				mu.Unlock()
				p.VerboseDetail(fmt.Sprintf("%s PR #%d", c.display, pr.Number))
			}
			p.Livef("prs", "%d/%d %s", n, len(cands), c.display)
			return nil
		})
	}
	_ = g.Wait()

	detail := fmt.Sprintf("%d open", found.Load())
	if failed.Load() > 0 {
		detail += fmt.Sprintf(" · %d failed", failed.Load())
	}
	p.StepDone("prs", detail)

	return repos, allMissing, nil
}

// ghRepo returns owner/name for GitHub API calls (prefer origin URL over path).
func ghRepo(r *model.Repo) (owner, name string) {
	if r.RemoteOwner != "" && r.RemoteName != "" {
		return r.RemoteOwner, r.RemoteName
	}
	if r.RemoteURL != "" {
		if o, n, ok := ghonline.ParseOwnerRepo(r.RemoteURL); ok {
			return o, n
		}
	}
	// last resort: directory layout
	return r.Org, r.Name
}

// deriveAll runs signal derivation for every repo.
func deriveAll(repos []model.Repo, opts config.Options, mode filter.Mode) []model.Repo {
	for i := range repos {
		preserved := repos[i].Signals
		repos[i].Signals = nil
		for _, s := range preserved {
			if s == model.SignalRemoteMismatch {
				repos[i].Signals = append(repos[i].Signals, s)
			}
		}
		filter.DeriveSignals(&repos[i], opts, mode)
	}
	return repos
}
