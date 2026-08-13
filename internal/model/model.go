package model

import "time"

// Signal is a status condition on a repo or missing clone.
type Signal string

const (
	SignalDirty          Signal = "dirty"
	SignalUntracked      Signal = "untracked"
	SignalAhead          Signal = "ahead"
	SignalBehind         Signal = "behind"
	SignalNoUpstream     Signal = "no_upstream"
	SignalNotDefault     Signal = "not_default"
	SignalDetached       Signal = "detached"
	SignalStash          Signal = "stash"
	SignalNoRemote       Signal = "no_remote"
	SignalNotRepo        Signal = "not_repo"
	SignalOpenPR         Signal = "open_pr"
	SignalMissingClone   Signal = "missing_clone"
	SignalRemoteMismatch Signal = "remote_mismatch"
	SignalRemoteNewer    Signal = "remote_newer" // remote tip differs (ls-remote; may need fetch for full count)
	SignalError          Signal = "error"
)

// PR is an open pull request on the current branch.
type PR struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
}

// Repo is one local working tree under a configured root.
type Repo struct {
	// Identity: prefer origin remote (host/org/name). Path* is filesystem hint only.
	Host     string `json:"host,omitempty"` // from origin, e.g. github.com
	Org      string `json:"org,omitempty"`  // remote owner, else path org
	Name     string `json:"name"`           // remote repo name, else folder
	PathOrg  string `json:"path_org,omitempty"`
	PathName string `json:"path_name,omitempty"`
	Label    string `json:"label,omitempty"` // table display name
	Path     string `json:"path"`
	IsRepo   bool   `json:"is_repo"`

	Branch        string `json:"branch,omitempty"`
	Upstream      string `json:"upstream,omitempty"`
	DefaultBranch string `json:"default_branch,omitempty"`
	Ahead         int    `json:"ahead"`
	Behind        int    `json:"behind"`
	// RemoteChecked is true when a non-mutating ls-remote freshness check ran.
	RemoteChecked bool `json:"remote_checked,omitempty"`
	// RemoteNewer means remote branch tip ≠ HEAD (from ls-remote, no fetch).
	RemoteNewer bool     `json:"remote_newer,omitempty"`
	Dirty       int      `json:"dirty"`
	Untracked   int      `json:"untracked"`
	Stash       int      `json:"stash"`
	HasRemote   bool     `json:"has_remote"`
	RemoteURL   string   `json:"remote_url,omitempty"`
	RemoteOwner string   `json:"remote_owner,omitempty"` // same as Org when from remote
	RemoteName  string   `json:"remote_name,omitempty"`  // same as Name when from remote
	Detached    bool     `json:"detached"`
	PR          *PR      `json:"pr,omitempty"`
	Signals     []Signal `json:"signals"`
	OK          bool     `json:"ok"`
	Err         string   `json:"error,omitempty"`
}

// MissingRepo is on GitHub but not cloned under the local root.
type MissingRepo struct {
	Org           string `json:"org"`
	Name          string `json:"name"`
	URL           string `json:"url,omitempty"`
	DefaultBranch string `json:"default_branch,omitempty"`
	IsFork        bool   `json:"is_fork,omitempty"`
	IsArchived    bool   `json:"is_archived,omitempty"`
}

// Org groups local repos (and optional missing clones) by organization.
type Org struct {
	Name    string        `json:"name"`
	Repos   []Repo        `json:"repos"`
	Missing []MissingRepo `json:"missing,omitempty"`
}

// Summary is aggregate counts for a report.
type Summary struct {
	Repos   int `json:"repos"`
	Issues  int `json:"issues"`
	Missing int `json:"missing"`
	OK      int `json:"ok"`
}

// Report is the full scan result.
type Report struct {
	Root        string        `json:"root"`
	Mode        string        `json:"mode"`
	GeneratedAt time.Time     `json:"generated_at"`
	Orgs        []Org         `json:"orgs"`
	Missing     []MissingRepo `json:"missing,omitempty"`
	Summary     Summary       `json:"summary"`
}

// HasSignal reports whether r already carries s.
func (r Repo) HasSignal(s Signal) bool {
	for _, sig := range r.Signals {
		if sig == s {
			return true
		}
	}
	return false
}

// DisplayName is the table label, falling back to host/org/name.
func (r Repo) DisplayName() string {
	if r.Label != "" {
		return r.Label
	}
	if r.Org != "" && r.Name != "" {
		if r.Host != "" && r.Host != "github.com" {
			return r.Host + "/" + r.Org + "/" + r.Name
		}
		return r.Org + "/" + r.Name
	}
	if r.Name != "" {
		return r.Name
	}
	return r.Path
}

// DisplayName is org/name for a missing repo.
func (m MissingRepo) DisplayName() string {
	return m.Org + "/" + m.Name
}
