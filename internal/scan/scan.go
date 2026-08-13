package scan

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ryanburnette/gitaware/internal/config"
	"github.com/ryanburnette/gitaware/internal/model"
)

// SkipDirNames are never descended into during discover.
var SkipDirNames = map[string]bool{
	"node_modules": true,
	"vendor":       true,
	"target":       true,
	"dist":         true,
	"build":        true,
	".cache":       true,
	".git":         true,
	".svn":         true,
	".hg":          true,
	"Library":      true, // macOS home noise if someone points at ~
}

// Entry is a discovered repo (or non-repo dir when warn).
type Entry struct {
	Root   string // configured root this came from
	Org    string // from path when layout provides it; may be empty
	Name   string // directory basename
	Path   string
	Rel    string // path relative to root
	IsRepo bool
}

// DiscoverAll scans every root with the given layout.
func DiscoverAll(opts config.Options) ([]Entry, error) {
	var all []Entry
	seen := map[string]bool{}
	for _, root := range opts.Roots {
		ents, err := DiscoverRoot(root, opts)
		if err != nil {
			// skip missing optional roots; error if single forced root
			if len(opts.Roots) == 1 {
				return nil, err
			}
			continue
		}
		for _, e := range ents {
			if seen[e.Path] {
				continue
			}
			seen[e.Path] = true
			all = append(all, e)
		}
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].Org != all[j].Org {
			return all[i].Org < all[j].Org
		}
		if all[i].Name != all[j].Name {
			return all[i].Name < all[j].Name
		}
		return all[i].Path < all[j].Path
	})
	return all, nil
}

// DiscoverRoot scans one root according to layout.
func DiscoverRoot(root string, opts config.Options) ([]Entry, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("root %s: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("root %s is not a directory", root)
	}

	layout := opts.Layout
	if layout == "" {
		layout = config.LayoutDiscover
	}

	var entries []Entry
	switch layout {
	case config.LayoutFlat:
		entries, err = scanFlat(root, opts)
	case config.LayoutOrgRepo:
		entries, err = scanOrgRepo(root, opts)
	case config.LayoutHostOrgRepo:
		entries, err = scanHostOrgRepo(root, opts)
	default: // discover
		depth := opts.Depth
		if depth <= 0 {
			depth = 3
		}
		entries, err = scanDiscover(root, depth, opts)
	}
	if err != nil {
		return nil, err
	}
	return filterIgnored(entries, opts.Ignore), nil
}

func scanFlat(root string, opts config.Options) ([]Entry, error) {
	kids, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, k := range kids {
		if !k.IsDir() || strings.HasPrefix(k.Name(), ".") || SkipDirNames[k.Name()] {
			continue
		}
		p := filepath.Join(root, k.Name())
		isRepo := looksLikeGit(p)
		if !isRepo && !opts.WarnNonRepos {
			continue
		}
		out = append(out, Entry{
			Root: root, Name: k.Name(), Path: p, Rel: k.Name(), IsRepo: isRepo,
		})
	}
	return out, nil
}

func scanOrgRepo(root string, opts config.Options) ([]Entry, error) {
	orgs, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, oe := range orgs {
		if !oe.IsDir() || strings.HasPrefix(oe.Name(), ".") || SkipDirNames[oe.Name()] {
			continue
		}
		org := oe.Name()
		if opts.OrgFilter != "" && org != opts.OrgFilter {
			continue
		}
		orgPath := filepath.Join(root, org)
		kids, err := os.ReadDir(orgPath)
		if err != nil {
			continue
		}
		for _, k := range kids {
			if !k.IsDir() || strings.HasPrefix(k.Name(), ".") || SkipDirNames[k.Name()] {
				continue
			}
			name := k.Name()
			p := filepath.Join(orgPath, name)
			isRepo := looksLikeGit(p)
			if !isRepo && !opts.WarnNonRepos {
				continue
			}
			out = append(out, Entry{
				Root: root, Org: org, Name: name, Path: p,
				Rel: filepath.Join(org, name), IsRepo: isRepo,
			})
		}
	}
	return out, nil
}

func scanHostOrgRepo(root string, opts config.Options) ([]Entry, error) {
	hosts, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, h := range hosts {
		if !h.IsDir() || strings.HasPrefix(h.Name(), ".") {
			continue
		}
		hostPath := filepath.Join(root, h.Name())
		// reuse org/repo under each host
		sub, err := scanOrgRepo(hostPath, opts)
		if err != nil {
			continue
		}
		for i := range sub {
			sub[i].Root = root
			sub[i].Rel = filepath.Join(h.Name(), sub[i].Rel)
			sub[i].Path = filepath.Join(hostPath, sub[i].Org, sub[i].Name)
		}
		out = append(out, sub...)
	}
	return out, nil
}

func scanDiscover(root string, maxDepth int, opts config.Options) ([]Entry, error) {
	var out []Entry
	var walk func(dir, rel string, depth int) error
	walk = func(dir, rel string, depth int) error {
		if depth > maxDepth {
			return nil
		}
		// If this dir is a git repo, take it and do not descend.
		if depth > 0 && looksLikeGit(dir) {
			org, name := splitOrgName(rel)
			if opts.OrgFilter != "" && org != opts.OrgFilter && org != "" {
				return nil
			}
			if opts.OrgFilter != "" && org == "" {
				// filter by remote later; keep for now
			}
			base := filepath.Base(dir)
			if name == "" {
				name = base
			}
			out = append(out, Entry{
				Root: root, Org: org, Name: name, Path: dir, Rel: rel, IsRepo: true,
			})
			return nil
		}
		kids, err := os.ReadDir(dir)
		if err != nil {
			return nil
		}
		for _, k := range kids {
			if !k.IsDir() {
				continue
			}
			n := k.Name()
			if strings.HasPrefix(n, ".") || SkipDirNames[n] {
				continue
			}
			child := filepath.Join(dir, n)
			childRel := n
			if rel != "" {
				childRel = filepath.Join(rel, n)
			}
			_ = walk(child, childRel, depth+1)
		}
		return nil
	}
	if err := walk(root, "", 0); err != nil {
		return nil, err
	}
	// depth 0: root itself might be a single repo
	if looksLikeGit(root) {
		out = append(out, Entry{
			Root: root, Name: filepath.Base(root), Path: root, Rel: ".", IsRepo: true,
		})
	}
	return out, nil
}

// splitOrgName turns "acme/app" into org=acme name=app; "app" into name=app.
func splitOrgName(rel string) (org, name string) {
	rel = filepath.ToSlash(rel)
	parts := strings.Split(rel, "/")
	switch len(parts) {
	case 0:
		return "", ""
	case 1:
		return "", parts[0]
	case 2:
		return parts[0], parts[1]
	default:
		// host/org/repo or deeper → last two
		return parts[len(parts)-2], parts[len(parts)-1]
	}
}

func looksLikeGit(path string) bool {
	st, err := os.Stat(filepath.Join(path, ".git"))
	if err != nil {
		return false
	}
	return st.IsDir() || st.Mode().IsRegular()
}

func filterIgnored(entries []Entry, ign config.Ignore) []Entry {
	if len(ign.Orgs) == 0 && len(ign.Repos) == 0 && len(ign.PathGlobs) == 0 {
		return entries
	}
	orgSet := toSet(ign.Orgs)
	repoSet := toSet(ign.Repos)
	var out []Entry
	for _, e := range entries {
		if e.Org != "" && orgSet[e.Org] {
			continue
		}
		key := e.Name
		if e.Org != "" {
			key = e.Org + "/" + e.Name
		}
		if repoSet[key] || repoSet[e.Name] {
			continue
		}
		skip := false
		for _, g := range ign.PathGlobs {
			ok, _ := filepath.Match(g, e.Path)
			ok2, _ := filepath.Match(g, e.Rel)
			if ok || ok2 {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		out = append(out, e)
	}
	return out
}

func toSet(ss []string) map[string]bool {
	m := map[string]bool{}
	for _, s := range ss {
		m[s] = true
	}
	return m
}

// ToRepos converts entries to model.Repo shells.
// Path-derived org/name are hints only; ApplyIdentity overwrites from remote.
func ToRepos(entries []Entry) []model.Repo {
	out := make([]model.Repo, 0, len(entries))
	for _, e := range entries {
		out = append(out, model.Repo{
			Org:      e.Org,
			Name:     e.Name,
			PathOrg:  e.Org,
			PathName: e.Name,
			Path:     e.Path,
			IsRepo:   e.IsRepo,
		})
	}
	return out
}

// ApplyIdentity sets Host/Org/Name from the origin remote when available.
// PathOrg/PathName stay as filesystem hints. Call after RemoteURL is filled.
func ApplyIdentity(repos []model.Repo) {
	for i := range repos {
		r := &repos[i]
		if r.RemoteURL == "" {
			// keep path identity; ensure Name is set
			if r.Name == "" {
				r.Name = filepath.Base(r.Path)
			}
			continue
		}
		id, ok := parseRemoteLocal(r.RemoteURL)
		if !ok {
			continue
		}
		r.Host = id.host
		r.RemoteOwner = id.owner
		r.RemoteName = id.name
		r.Org = id.owner
		r.Name = id.name
	}
}

// remoteBits avoids importing ghonline from scan (identity applied in app too).
// scan.ApplyIdentity is a thin helper; app also sets fields during enrich.
type remoteBits struct{ host, owner, name string }

func parseRemoteLocal(remoteURL string) (remoteBits, bool) {
	// Duplicate minimal parse so scan tests don't need ghonline.
	// Full parse lives in ghonline.ParseRemote; enrich uses that.
	remoteURL = strings.TrimSpace(remoteURL)
	if remoteURL == "" {
		return remoteBits{}, false
	}
	if strings.HasPrefix(remoteURL, "git@") {
		rest := strings.TrimPrefix(remoteURL, "git@")
		host, path, found := strings.Cut(rest, ":")
		if !found {
			return remoteBits{}, false
		}
		path = strings.TrimSuffix(path, ".git")
		parts := strings.Split(path, "/")
		if len(parts) >= 2 {
			return remoteBits{host, parts[0], parts[len(parts)-1]}, true
		}
		return remoteBits{}, false
	}
	// https://host/owner/repo.git — light parse
	const scheme = "://"
	if i := strings.Index(remoteURL, scheme); i >= 0 {
		rest := remoteURL[i+len(scheme):]
		host, path, found := strings.Cut(rest, "/")
		if !found {
			return remoteBits{}, false
		}
		if hi := strings.IndexByte(host, '@'); hi >= 0 {
			host = host[hi+1:]
		}
		path = strings.TrimSuffix(path, ".git")
		parts := strings.Split(path, "/")
		if len(parts) >= 2 {
			return remoteBits{host, parts[0], parts[len(parts)-1]}, true
		}
	}
	return remoteBits{}, false
}

// FilterByOrg keeps repos whose Org or PathOrg matches (after identity).
func FilterByOrg(repos []model.Repo, org string) []model.Repo {
	if org == "" {
		return repos
	}
	var out []model.Repo
	for _, r := range repos {
		if r.Org == org || r.PathOrg == org || r.RemoteOwner == org {
			out = append(out, r)
		}
	}
	return out
}

// GroupByOrg builds Org slices. Uses identity Org (remote owner preferred).
func GroupByOrg(repos []model.Repo) []model.Org {
	if len(repos) == 0 {
		return nil
	}
	index := map[string]int{}
	var orgs []model.Org
	for _, r := range repos {
		org := r.Org
		if org == "" {
			org = r.RemoteOwner
		}
		if org == "" {
			org = r.PathOrg
		}
		if org == "" {
			org = "_"
		}
		i, ok := index[org]
		if !ok {
			orgs = append(orgs, model.Org{Name: org})
			i = len(orgs) - 1
			index[org] = i
		}
		orgs[i].Repos = append(orgs[i].Repos, r)
	}
	sort.Slice(orgs, func(a, b int) bool { return orgs[a].Name < orgs[b].Name })
	return orgs
}

// ApplyDisplayNames sets Label on each repo from name mode.
func ApplyDisplayNames(repos []model.Repo, mode string) {
	for i := range repos {
		repos[i].Label = DisplayName(repos[i], mode)
	}
}

// DisplayName picks the repository column text.
// Default (auto/remote): identity from git remote — host/org/repo when host ≠ github.com.
func DisplayName(r model.Repo, mode string) string {
	remoteLabel := identityLabel(r)
	pathLabel := ""
	if r.PathOrg != "" && r.PathName != "" {
		pathLabel = r.PathOrg + "/" + r.PathName
	} else if r.PathName != "" {
		pathLabel = r.PathName
	}
	folder := r.PathName
	if folder == "" {
		folder = filepath.Base(r.Path)
	}
	if r.Name != "" && folder == "" {
		folder = r.Name
	}

	switch mode {
	case config.NamePath:
		if pathLabel != "" {
			return pathLabel
		}
		if remoteLabel != "" {
			return remoteLabel
		}
		return folder
	case config.NameFolder:
		return folder
	case config.NameRemote:
		if remoteLabel != "" {
			return remoteLabel
		}
		return firstNonEmpty(pathLabel, folder)
	default: // auto: remote first, then path
		if remoteLabel != "" {
			return remoteLabel
		}
		return firstNonEmpty(pathLabel, folder)
	}
}

func identityLabel(r model.Repo) string {
	owner, name := r.RemoteOwner, r.RemoteName
	if owner == "" {
		owner = r.Org
	}
	if name == "" {
		name = r.Name
	}
	// Only treat as remote identity when we actually have a remote.
	if r.RemoteOwner == "" && r.RemoteName == "" && r.Host == "" {
		if r.Org != "" && r.Name != "" && r.HasRemote {
			// identity already applied into Org/Name
		} else if !r.HasRemote {
			return ""
		}
	}
	if owner == "" || name == "" {
		return ""
	}
	// Prefer remote-backed labels.
	if r.RemoteOwner != "" || r.Host != "" {
		if r.Host != "" && r.Host != "github.com" {
			return r.Host + "/" + owner + "/" + name
		}
		return owner + "/" + name
	}
	return ""
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}
