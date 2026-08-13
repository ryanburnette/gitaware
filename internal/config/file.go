package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Layout names.
const (
	LayoutDiscover    = "discover"
	LayoutFlat        = "flat"
	LayoutOrgRepo     = "org/repo"
	LayoutHostOrgRepo = "host/org/repo"
)

// DisplayName modes.
const (
	NameAuto   = "auto"
	NamePath   = "path"
	NameRemote = "remote"
	NameFolder = "folder"
)

// File is the on-disk config at ~/.config/gitaware/config.json.
type File struct {
	Version         int      `json:"version"`
	Roots           []string `json:"roots,omitempty"`
	Layout          string   `json:"layout,omitempty"`
	Depth           int      `json:"depth,omitempty"`
	DefaultBranches []string `json:"default_branches,omitempty"`
	GitHubOrgs      []string `json:"github_orgs,omitempty"`
	Workers         int      `json:"workers,omitempty"`
	CacheTTLHours   int      `json:"cache_ttl_hours,omitempty"`
	Display         Display  `json:"display"`
	Ignore          Ignore   `json:"ignore"`
}

// Display controls table labeling and default filters.
type Display struct {
	// Name: auto | path | remote | folder
	Name string `json:"name,omitempty"`
	// ShowAll includes clean repos (same as -a).
	ShowAll bool `json:"show_all,omitempty"`
}

// Ignore filters discovery.
type Ignore struct {
	Orgs      []string `json:"orgs,omitempty"`
	Repos     []string `json:"repos,omitempty"` // org/name or name
	PathGlobs []string `json:"path_globs,omitempty"`
}

// ConfigPath returns ~/.config/gitaware/config.json (or $GITAWARE_CONFIG).
func ConfigPath() string {
	if v := os.Getenv("GITAWARE_CONFIG"); v != "" {
		return expandHome(v)
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "gitaware", "config.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", "config.json")
	}
	return filepath.Join(home, ".config", "gitaware", "config.json")
}

// DefaultFile is built-in config when no file exists.
// Empty Roots means "use the current working directory".
func DefaultFile() File {
	return File{
		Version: 1,
		Roots:   nil,
		Layout:  LayoutDiscover,
		Depth:   3,
		Display: Display{
			Name:    NameRemote,
			ShowAll: false,
		},
		DefaultBranches: append([]string{}, DefaultBranches...),
		Workers:         DefaultWorkers(),
		CacheTTLHours:   24,
	}
}

// defaultRootCandidates are common project dirs (only existing ones are used at scan time).
func defaultRootCandidates() []string {
	return []string{
		"~/git",
		"~/Projects",
		"~/projects",
		"~/Developer",
		"~/developer",
		"~/src",
		"~/code",
		"~/work",
		"~/repos",
	}
}

// LoadFile reads config from path. Missing file returns defaults + ErrNotExist wrapper?
// Returns defaults and false if missing.
func LoadFile(path string) (File, bool, error) {
	def := DefaultFile()
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return def, false, nil
		}
		return def, false, fmt.Errorf("read config: %w", err)
	}
	var f File
	if err := json.Unmarshal(b, &f); err != nil {
		return def, true, fmt.Errorf("parse config: %w", err)
	}
	merged := mergeFile(def, f)
	return merged, true, nil
}

func mergeFile(base, over File) File {
	out := base
	if over.Version != 0 {
		out.Version = over.Version
	}
	if len(over.Roots) > 0 {
		out.Roots = over.Roots
	}
	if over.Layout != "" {
		out.Layout = over.Layout
	}
	if over.Depth > 0 {
		out.Depth = over.Depth
	}
	if len(over.DefaultBranches) > 0 {
		out.DefaultBranches = over.DefaultBranches
	}
	if len(over.GitHubOrgs) > 0 {
		out.GitHubOrgs = over.GitHubOrgs
	}
	if over.Workers > 0 {
		out.Workers = over.Workers
	}
	if over.CacheTTLHours > 0 {
		out.CacheTTLHours = over.CacheTTLHours
	}
	if over.Display.Name != "" {
		out.Display.Name = over.Display.Name
	}
	// ShowAll: only override if file explicitly set — JSON false is zero value.
	// We treat presence of display object with show_all via raw merge: always take over.Display.ShowAll
	// when Display was unmarshaled. That's correct: default false, file can set true.
	out.Display.ShowAll = over.Display.ShowAll
	if over.Display.Name == "" && base.Display.Name != "" && over.Display.ShowAll == false {
		// keep name from base if over didn't set name - already handled
	}
	if over.Display.Name != "" {
		out.Display.Name = over.Display.Name
	} else if base.Display.Name != "" {
		out.Display.Name = base.Display.Name
	}
	out.Ignore.Orgs = append([]string{}, over.Ignore.Orgs...)
	out.Ignore.Repos = append([]string{}, over.Ignore.Repos...)
	out.Ignore.PathGlobs = append([]string{}, over.Ignore.PathGlobs...)
	if out.Display.Name == "" {
		out.Display.Name = NameAuto
	}
	if out.Layout == "" {
		out.Layout = LayoutDiscover
	}
	return out
}

// WriteFile writes f as indented JSON, creating parent dirs.
func WriteFile(path string, f File) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if f.Version == 0 {
		f.Version = 1
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}

// ExpandRoots expands ~ and makes absolute; drops empty.
func ExpandRoots(roots []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, r := range roots {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		abs := ExpandRoot(r)
		if seen[abs] {
			continue
		}
		seen[abs] = true
		out = append(out, abs)
	}
	return out
}

// ExistingRoots returns only roots that exist as directories.
func ExistingRoots(roots []string) []string {
	var out []string
	for _, r := range ExpandRoots(roots) {
		st, err := os.Stat(r)
		if err != nil || !st.IsDir() {
			continue
		}
		out = append(out, r)
	}
	return out
}

// SampleConfig is a starter config: walk ~/git, label from remotes.
func SampleConfig() File {
	return File{
		Version: 1,
		Roots:   []string{"~/git"},
		Layout:  LayoutDiscover,
		Depth:   3,
		Display: Display{
			Name:    NameRemote,
			ShowAll: false,
		},
		DefaultBranches: append([]string{}, DefaultBranches...),
		Workers:         DefaultWorkers(),
		CacheTTLHours:   24,
	}
}

// SampleOrgRepo is kept for older docs; prefer SampleConfig.
func SampleOrgRepo() File {
	f := SampleConfig()
	f.Layout = LayoutOrgRepo
	f.Depth = 2
	return f
}
