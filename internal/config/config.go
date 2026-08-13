package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// DefaultBranches used when GitHub default is unknown.
var DefaultBranches = []string{"main", "master"}

// Options are the resolved runtime settings (flags + env + file).
type Options struct {
	// Discovery
	Roots     []string // effective scan roots
	Root      string   // -C single-root override (if set, Roots = [Root])
	Layout    string
	Depth     int
	OrgFilter string

	// Filters / mode
	All             bool // -a include clean
	IssuesOnly      bool // effective: !All after resolve
	IssuesFlag      bool // --issues explicitly set
	AllFlag         bool // -a explicitly set
	Fetch           bool // mutating: git fetch (confirm unless Yes)
	CheckRemote     bool // non-mutating: ls-remote freshness
	Online          bool
	Refresh         bool
	CheckAllPRs     bool
	Yes             bool // -y skip mutating confirmation
	WarnNonRepos    bool
	NoForks         bool
	IncludeArchived bool
	StrictBranch    bool
	IncludeBehind   bool
	IncludeMissing  bool
	IncludeStash    bool

	// Display
	NameMode string // auto|path|remote|folder
	JSON     bool
	Verbose  bool
	NoColor  bool

	// Engine
	Workers         int
	DefaultBranches []string
	GitHubOrgs      []string
	Ignore          Ignore

	// Meta
	ConfigPath string
	ConfigOK   bool // file was loaded
}

// DefaultWorkers returns a sensible parallel worker count.
func DefaultWorkers() int {
	n := runtime.NumCPU()
	if n < 4 {
		return 4
	}
	if n > 16 {
		return 16
	}
	return n
}

// CacheDir is ~/.cache/gitaware (or $GITAWARE_CACHE).
func CacheDir() string {
	if v := os.Getenv("GITAWARE_CACHE"); v != "" {
		return expandHome(v)
	}
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "gitaware")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".cache", "gitaware")
	}
	return filepath.Join(home, ".cache", "gitaware")
}

// ColorEnabled respects NO_COLOR and --no-color, and TTY on stdout.
func ColorEnabled(noColorFlag bool, stdoutIsTTY bool) bool {
	if noColorFlag {
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if strings.EqualFold(os.Getenv("TERM"), "dumb") {
		return false
	}
	return stdoutIsTTY
}

// IsDefaultBranch reports whether branch is a known default.
func IsDefaultBranch(branch string, defaults []string) bool {
	if branch == "" {
		return false
	}
	if len(defaults) == 0 {
		defaults = DefaultBranches
	}
	for _, d := range defaults {
		if branch == d {
			return true
		}
	}
	return false
}

func expandHome(p string) string {
	if p == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return p
		}
		return home
	}
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return p
		}
		return filepath.Join(home, p[2:])
	}
	return p
}

// ExpandRoot normalizes a single path.
func ExpandRoot(root string) string {
	if root == "" {
		return ""
	}
	root = expandHome(root)
	abs, err := filepath.Abs(root)
	if err != nil {
		return root
	}
	return abs
}

// DefaultRoot is the first existing default candidate, or ~/git.
func DefaultRoot() string {
	if v := os.Getenv("GITAWARE_ROOT"); v != "" {
		return ExpandRoot(v)
	}
	existing := ExistingRoots(defaultRootCandidates())
	if len(existing) > 0 {
		return existing[0]
	}
	return ExpandRoot("~/git")
}

// Resolve loads config file and applies flag/env precedence into opts.
// Call after parsing flags into opts (Root, AllFlag, etc. already set).
func Resolve(opts *Options) error {
	path := ConfigPath()
	opts.ConfigPath = path
	file, ok, err := LoadFile(path)
	if err != nil {
		return err
	}
	opts.ConfigOK = ok

	// Start from file
	if opts.Layout == "" {
		opts.Layout = file.Layout
	}
	if opts.Depth <= 0 {
		opts.Depth = file.Depth
	}
	if opts.NameMode == "" {
		opts.NameMode = file.Display.Name
	}
	if opts.Workers <= 0 {
		opts.Workers = file.Workers
	}
	if len(opts.DefaultBranches) == 0 {
		opts.DefaultBranches = file.DefaultBranches
	}
	if len(opts.GitHubOrgs) == 0 {
		opts.GitHubOrgs = file.GitHubOrgs
	}
	opts.Ignore = file.Ignore

	// show_all from config unless flags override
	showAll := file.Display.ShowAll
	if opts.AllFlag {
		showAll = true
	}
	if opts.IssuesFlag {
		showAll = false
	}
	opts.All = showAll
	opts.IssuesOnly = !showAll

	// Roots: -d/-C > GITAWARE_ROOT > config roots > cwd
	// Identity (host/org/repo) comes from git remotes, not from path layout.
	if opts.Root != "" {
		opts.Roots = []string{ExpandRoot(opts.Root)}
	} else if env := os.Getenv("GITAWARE_ROOT"); env != "" {
		opts.Roots = []string{ExpandRoot(env)}
		opts.Root = opts.Roots[0]
	} else if len(file.Roots) > 0 {
		opts.Roots = ExistingRoots(file.Roots)
		if len(opts.Roots) == 0 {
			// config listed roots but none exist — keep expanded for clear error
			opts.Roots = ExpandRoots(file.Roots)
		}
		opts.Root = opts.Roots[0]
	} else {
		// No config roots: walk the current working directory.
		cwd, err := os.Getwd()
		if err != nil {
			cwd = ExpandRoot("~")
		}
		opts.Roots = []string{cwd}
		opts.Root = cwd
	}

	// Discovery always finds .git dirs. Path layouts are optional speed/shape hints.
	if opts.Layout == "" {
		opts.Layout = LayoutDiscover
	}
	if opts.Depth <= 0 {
		opts.Depth = 3
	}
	// Labels default to remote identity (owner/repo).
	if opts.NameMode == "" {
		opts.NameMode = NameRemote
	}
	if opts.Workers <= 0 {
		opts.Workers = DefaultWorkers()
	}
	if len(opts.DefaultBranches) == 0 {
		opts.DefaultBranches = append([]string{}, DefaultBranches...)
	}
	opts.IncludeStash = true
	return nil
}
