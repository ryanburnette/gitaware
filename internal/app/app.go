package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/ryanburnette/gitaware/internal/cache"
	"github.com/ryanburnette/gitaware/internal/config"
	"github.com/ryanburnette/gitaware/internal/filter"
	"github.com/ryanburnette/gitaware/internal/ghonline"
	"github.com/ryanburnette/gitaware/internal/gitlocal"
	"github.com/ryanburnette/gitaware/internal/model"
	"github.com/ryanburnette/gitaware/internal/render"
	"github.com/ryanburnette/gitaware/internal/scan"
)

// Version is set via ldflags.
var Version = "dev"

// BuildTime is set via ldflags.
var BuildTime = "unknown"

// App holds shared dependencies.
type App struct {
	Git    *gitlocal.Runner
	Stdout io.Writer
	Stderr io.Writer
}

// New constructs an App with defaults.
func New() *App {
	return &App{
		Git:    gitlocal.New(),
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
}

// RunStatus builds and prints a status report.
func (a *App) RunStatus(ctx context.Context, opts config.Options, mode filter.Mode) error {
	report, err := a.BuildReport(ctx, opts, mode)
	if err != nil {
		return err
	}

	issuesOnly := opts.IssuesOnly && !opts.All
	filter.Apply(&report, issuesOnly, mode)

	if opts.JSON {
		return render.JSON(a.Stdout, report)
	}
	colorOn := config.ColorEnabled(opts.NoColor, render.IsTTY(os.Stdout))
	render.Tree(a.Stdout, report, render.NewColor(colorOn))

	if report.Summary.Issues > 0 || report.Summary.Missing > 0 {
		return errIssues
	}
	return nil
}

// errIssues signals exit code 1.
var errIssues = fmt.Errorf("issues found")

// IsIssues reports whether err is the issues sentinel.
func IsIssues(err error) bool {
	return err == errIssues
}

// BuildReport scans and enriches without rendering.
func (a *App) BuildReport(ctx context.Context, opts config.Options, mode filter.Mode) (model.Report, error) {
	root := config.ExpandRoot(opts.Root)
	opts.Root = root
	if opts.Workers <= 0 {
		opts.Workers = config.DefaultWorkers()
	}
	// stash on by default
	if !opts.JSON {
		opts.IncludeStash = true
	} else {
		opts.IncludeStash = true
	}

	entries, err := scan.Discover(root, opts.OrgFilter, opts.WarnNonRepos, a.Git)
	if err != nil {
		return model.Report{}, err
	}
	repos := scan.ToRepos(entries)
	repos = enrichLocal(ctx, repos, a.Git, opts)

	var missing []model.MissingRepo
	modeName := "offline"
	if opts.Fetch && opts.Online {
		modeName = "online+fetch"
	} else if opts.Fetch {
		modeName = "offline+fetch"
	} else if opts.Online {
		modeName = "online"
	}

	if opts.Online {
		store := cache.New(config.CacheDir())
		gh := ghonline.New(store)
		var err error
		repos, missing, err = enrichOnline(ctx, repos, opts, gh)
		if err != nil {
			return model.Report{}, err
		}
	}

	// leave never cares about behind as "fresh" but we still derive
	if mode == filter.ModeLeave {
		opts.StrictBranch = true
		opts.IncludeBehind = false
	}
	if mode == filter.ModeArrive {
		opts.IncludeBehind = true
		opts.StrictBranch = true
		opts.IncludeMissing = true
	}
	if mode == filter.ModeStatus {
		opts.IncludeBehind = opts.Fetch || opts.Online
	}

	repos = deriveAll(repos, opts, mode)
	orgs := scan.GroupByOrg(repos)

	// attach missing to orgs
	if len(missing) > 0 {
		byOrg := map[string][]model.MissingRepo{}
		for _, m := range missing {
			byOrg[m.Org] = append(byOrg[m.Org], m)
		}
		seen := map[string]bool{}
		for i := range orgs {
			if ms, ok := byOrg[orgs[i].Name]; ok {
				orgs[i].Missing = ms
				seen[orgs[i].Name] = true
			}
		}
		for orgName, ms := range byOrg {
			if seen[orgName] {
				continue
			}
			orgs = append(orgs, model.Org{Name: orgName, Missing: ms})
		}
	}

	report := model.Report{
		Root:        root,
		Mode:        modeName,
		GeneratedAt: time.Now().UTC(),
		Orgs:        orgs,
		Missing:     missing,
	}
	filter.Apply(&report, false, mode) // summary only
	return report, nil
}

// RunDoctor checks environment.
func (a *App) RunDoctor(opts config.Options) error {
	root := config.ExpandRoot(opts.Root)
	fmt.Fprintf(a.Stdout, "gitaware %s\n", Version)
	fmt.Fprintf(a.Stdout, "root: %s\n", root)

	if err := a.Git.Available(); err != nil {
		fmt.Fprintf(a.Stdout, "git: %v\n", err)
	} else {
		fmt.Fprintf(a.Stdout, "git: ok\n")
	}

	gh := ghonline.New(nil)
	if err := gh.Available(); err != nil {
		fmt.Fprintf(a.Stdout, "gh: %v\n", err)
	} else {
		fmt.Fprintf(a.Stdout, "gh: ok\n")
		u, err := gh.WhoAmI(context.Background())
		if err != nil {
			fmt.Fprintf(a.Stdout, "gh auth: %v\n", err)
		} else {
			fmt.Fprintf(a.Stdout, "gh user: %s\n", u.Login)
		}
	}

	st, err := os.Stat(root)
	if err != nil {
		fmt.Fprintf(a.Stdout, "root exists: no (%v)\n", err)
		return fmt.Errorf("doctor failed")
	}
	if !st.IsDir() {
		fmt.Fprintf(a.Stdout, "root exists: not a directory\n")
		return fmt.Errorf("doctor failed")
	}
	fmt.Fprintf(a.Stdout, "root exists: yes\n")
	fmt.Fprintf(a.Stdout, "cache: %s\n", config.CacheDir())
	return nil
}

// RunOrgs lists org directories under root.
func (a *App) RunOrgs(opts config.Options) error {
	root := config.ExpandRoot(opts.Root)
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() || e.Name()[0] == '.' {
			continue
		}
		fmt.Fprintln(a.Stdout, e.Name())
	}
	return nil
}

// RunMissing lists missing clones (online).
func (a *App) RunMissing(ctx context.Context, opts config.Options) error {
	opts.Online = true
	opts.All = true
	opts.IssuesOnly = false
	report, err := a.BuildReport(ctx, opts, filter.ModeArrive)
	if err != nil {
		return err
	}
	if opts.JSON {
		type out struct {
			Missing []model.MissingRepo `json:"missing"`
		}
		return render.JSON(a.Stdout, model.Report{Missing: report.Missing, Root: report.Root, Mode: report.Mode, GeneratedAt: report.GeneratedAt})
	}
	if len(report.Missing) == 0 {
		fmt.Fprintln(a.Stdout, "no missing clones")
		return nil
	}
	for _, m := range report.Missing {
		fmt.Fprintln(a.Stdout, m.DisplayName())
	}
	return errIssues
}

// RunPRs lists open PRs on current branches (online).
func (a *App) RunPRs(ctx context.Context, opts config.Options) error {
	opts.Online = true
	opts.All = true
	report, err := a.BuildReport(ctx, opts, filter.ModeStatus)
	if err != nil {
		return err
	}
	count := 0
	for _, org := range report.Orgs {
		for _, r := range org.Repos {
			if r.PR == nil {
				continue
			}
			count++
			if opts.JSON {
				continue
			}
			fmt.Fprintf(a.Stdout, "%s\tPR#%d\t%s\t%s\n", r.DisplayName(), r.PR.Number, r.Branch, r.PR.URL)
		}
	}
	if opts.JSON {
		return render.JSON(a.Stdout, report)
	}
	if count == 0 {
		fmt.Fprintln(a.Stdout, "no open PRs on current branches")
		return nil
	}
	return nil
}

// RunFetch fetches all repos.
func (a *App) RunFetch(ctx context.Context, opts config.Options) error {
	root := config.ExpandRoot(opts.Root)
	entries, err := scan.Discover(root, opts.OrgFilter, false, a.Git)
	if err != nil {
		return err
	}
	opts.Fetch = true
	repos := scan.ToRepos(entries)
	_ = enrichLocal(ctx, repos, a.Git, opts)
	fmt.Fprintf(a.Stdout, "fetched %d repos under %s\n", countRepos(repos), root)
	if opts.All || true {
		// optional status after
	}
	return nil
}

func countRepos(repos []model.Repo) int {
	n := 0
	for _, r := range repos {
		if r.IsRepo {
			n++
		}
	}
	return n
}

// RunCloneMissing clones missing repos.
func (a *App) RunCloneMissing(ctx context.Context, opts config.Options, names []string) error {
	opts.Online = true
	report, err := a.BuildReport(ctx, opts, filter.ModeArrive)
	if err != nil {
		return err
	}
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	store := cache.New(config.CacheDir())
	gh := ghonline.New(store)

	var toClone []model.MissingRepo
	for _, m := range report.Missing {
		if len(want) > 0 && !want[m.DisplayName()] && !want[m.Name] {
			continue
		}
		toClone = append(toClone, m)
	}
	if len(toClone) == 0 {
		fmt.Fprintln(a.Stdout, "nothing to clone")
		return nil
	}

	root := config.ExpandRoot(opts.Root)
	for _, m := range toClone {
		dest := filepath.Join(root, m.Org, m.Name)
		if st, err := os.Stat(dest); err == nil && st.IsDir() {
			fmt.Fprintf(a.Stderr, "skip exists: %s\n", dest)
			continue
		}
		if err := os.MkdirAll(filepath.Join(root, m.Org), 0o755); err != nil {
			return err
		}
		fmt.Fprintf(a.Stdout, "cloning %s → %s\n", m.DisplayName(), dest)
		if err := gh.Clone(ctx, m.Org, m.Name, dest); err != nil {
			fmt.Fprintf(a.Stderr, "error: %v\n", err)
			continue
		}
	}
	return nil
}

// SetupVerbose enables debug logging to stderr.
func SetupVerbose(verbose bool) {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
}
