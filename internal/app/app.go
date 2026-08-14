package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/ryanburnette/gitaware/internal/cache"
	"github.com/ryanburnette/gitaware/internal/config"
	"github.com/ryanburnette/gitaware/internal/filter"
	"github.com/ryanburnette/gitaware/internal/ghonline"
	"github.com/ryanburnette/gitaware/internal/gitlocal"
	"github.com/ryanburnette/gitaware/internal/model"
	"github.com/ryanburnette/gitaware/internal/progress"
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

// errIssues signals exit code 1.
var errIssues = fmt.Errorf("issues found")

// IsIssues reports whether err is the issues sentinel.
func IsIssues(err error) bool {
	return err == errIssues
}

// SetupVerbose enables debug logging to stderr.
func SetupVerbose(verbose bool) {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
}

// progress builds a stderr progress reporter for a command run.
// Progress is silenced under --json and on non-TTY stderr stays line-based
// (never in-place \r). The returned Progress is already started.
func (a *App) progress(opts config.Options, command string) *progress.Progress {
	enabled := !opts.JSON
	stderrTTY := writerIsTTY(a.Stderr)
	color := config.ColorEnabled(opts.NoColor, stderrTTY)
	p := progress.New(a.Stderr, progress.Options{
		Enabled: enabled,
		Color:   color,
		Verbose: opts.Verbose,
		TTY:     stderrTTY,
	})
	p.Start(command)
	return p
}

// writerIsTTY reports whether w is an interactive terminal.
func writerIsTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return render.IsTTY(f)
}

// commandName maps a filter mode to its progress label.
func commandName(mode filter.Mode) string {
	switch mode {
	case filter.ModeLeave:
		return "leave"
	case filter.ModeArrive:
		return "arrive"
	default:
		return "status"
	}
}

// modeLabel renders the report mode tag shown in the title and JSON.
func modeLabel(opts config.Options, mode filter.Mode) string {
	switch {
	case opts.Fetch && opts.Online:
		return "online+fetch"
	case opts.Fetch:
		return "offline+fetch"
	case opts.Online && opts.CheckRemote:
		return "online+ls-remote"
	case opts.Online:
		return "online"
	default:
		return "offline"
	}
}

// applyModeDefaults sets per-mode option defaults before enrichment.
func applyModeDefaults(opts *config.Options, mode filter.Mode) {
	switch mode {
	case filter.ModeLeave:
		opts.StrictBranch = true
		opts.IncludeBehind = false
	case filter.ModeArrive:
		// Catch-up after switching machines: online, behind matters, strict
		// branch. Freshness defaults to non-mutating ls-remote; --fetch opts in.
		opts.Online = true
		opts.IncludeBehind = true
		opts.StrictBranch = true
		if !opts.Fetch {
			opts.CheckRemote = true
		}
		// Arrive deliberately does NOT set IncludeMissing — use `missing`.
	case filter.ModeStatus:
		opts.IncludeBehind = opts.Fetch || opts.CheckRemote || opts.Online
	}
}

// RunStatus builds and prints a status report.
func (a *App) RunStatus(ctx context.Context, opts config.Options, mode filter.Mode) error {
	p := a.progress(opts, commandName(mode))
	report, err := a.BuildReport(ctx, opts, mode, p)
	if err != nil {
		p.Fail(err.Error())
		return err
	}
	p.End(summaryEnd(report, mode))

	issuesOnly := opts.IssuesOnly
	if opts.All {
		issuesOnly = false
	}
	filter.Apply(&report, issuesOnly, mode)

	if opts.JSON {
		return render.JSON(a.Stdout, report)
	}

	colorOn := config.ColorEnabled(opts.NoColor, render.IsTTY(os.Stdout))
	render.Tree(a.Stdout, report, render.NewThemeOpts(colorOn, colorOn))

	if report.Summary.Issues > 0 || report.Summary.Missing > 0 {
		return errIssues
	}
	return nil
}

// summaryEnd composes the progress.End summary line.
func summaryEnd(report model.Report, mode filter.Mode) string {
	s := report.Summary
	if s.Issues == 0 && s.Missing == 0 {
		return fmt.Sprintf("all clear · %d repos · %s", s.Repos, report.Mode)
	}
	return fmt.Sprintf("%d issues · %d repos · %s", s.Issues, s.Repos, report.Mode)
}

// BuildReport scans and enriches without rendering.
func (a *App) BuildReport(ctx context.Context, opts config.Options, mode filter.Mode, p *progress.Progress) (model.Report, error) {
	if p == nil {
		p = progress.New(io.Discard, progress.Options{})
	}
	if opts.Workers <= 0 {
		opts.Workers = config.DefaultWorkers()
	}
	// Mode defaults must be applied before enrichment so that CheckRemote/Fetch
	// drive ls-remote/fetch during local status.
	applyModeDefaults(&opts, mode)

	// Discover repos under all effective roots.
	p.Stepf("scan", "discovering under %s", rootList(opts.Roots))
	entries, err := scan.DiscoverAll(opts)
	if err != nil {
		return model.Report{}, err
	}
	repos := scan.ToRepos(entries)
	p.StepDone("scan", fmt.Sprintf("%d repos", countRepos(repos)))

	// Local git status (and optional fetch / ls-remote freshness).
	repos = enrichLocal(ctx, repos, a.Git, opts, p)

	// Identity: apply display names from resolved name mode.
	scan.ApplyDisplayNames(repos, opts.NameMode)

	// Filter by remote owner after identity is applied.
	if opts.OrgFilter != "" {
		repos = scan.FilterByOrg(repos, opts.OrgFilter)
	}

	var missing []model.MissingRepo
	if opts.Online {
		store := cache.New(config.CacheDir())
		gh := ghonline.New(store)
		repos, missing, err = enrichOnline(ctx, repos, opts, gh, p)
		if err != nil {
			return model.Report{}, err
		}
	}

	repos = deriveAll(repos, opts, mode)
	orgs := scan.GroupByOrg(repos)

	// Attach missing clones to their orgs (or append as org-only groups).
	if len(missing) > 0 {
		byOrg := map[string][]model.MissingRepo{}
		var orgOrder []string
		for _, m := range missing {
			if _, ok := byOrg[m.Org]; !ok {
				orgOrder = append(orgOrder, m.Org)
			}
			byOrg[m.Org] = append(byOrg[m.Org], m)
		}
		seen := map[string]bool{}
		for i := range orgs {
			if ms, ok := byOrg[orgs[i].Name]; ok {
				orgs[i].Missing = ms
				seen[orgs[i].Name] = true
			}
		}
		for _, orgName := range orgOrder {
			if seen[orgName] {
				continue
			}
			orgs = append(orgs, model.Org{Name: orgName, Missing: byOrg[orgName]})
		}
	}

	report := model.Report{
		Root:        reportRoot(opts),
		Mode:        modeLabel(opts, mode),
		GeneratedAt: time.Now().UTC(),
		Orgs:        orgs,
		Missing:     missing,
	}
	filter.Apply(&report, false, mode) // summary only
	return report, nil
}

// reportRoot is the display root (first root, or cwd).
func reportRoot(opts config.Options) string {
	if len(opts.Roots) > 0 {
		return opts.Roots[0]
	}
	if opts.Root != "" {
		return opts.Root
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

func rootList(roots []string) string {
	if len(roots) == 0 {
		return "cwd"
	}
	if len(roots) == 1 {
		return roots[0]
	}
	return fmt.Sprintf("%d roots", len(roots))
}

// RunDoctor prints effective config and environment checks.
func (a *App) RunDoctor(opts config.Options) error {
	out := a.Stdout
	fmt.Fprintf(out, "gitaware %s\n", Version)

	path := opts.ConfigPath
	status := "missing"
	if opts.ConfigOK {
		status = "loaded"
	}
	fmt.Fprintf(out, "config: %s (%s)\n", path, status)
	fmt.Fprintf(out, "layout: %s  depth: %d  name: %s  show_all: %t\n",
		layoutLabel(opts.Layout), opts.Depth, nameLabel(opts.NameMode), opts.All)
	fmt.Fprintln(out, "roots:")
	for _, r := range opts.Roots {
		fmt.Fprintf(out, "  %s\n", r)
	}

	if err := a.Git.Available(); err != nil {
		fmt.Fprintf(out, "git: %v\n", err)
	} else {
		fmt.Fprintln(out, "git: ok")
	}

	gh := ghonline.New(nil)
	if err := gh.Available(); err != nil {
		fmt.Fprintf(out, "gh: %v\n", err)
	} else {
		fmt.Fprintln(out, "gh: ok")
		if u, err := gh.WhoAmI(context.Background()); err != nil {
			fmt.Fprintf(out, "gh user: %v\n", err)
		} else {
			fmt.Fprintf(out, "gh user: %s\n", u.Login)
		}
	}

	fmt.Fprintf(out, "cache: %s\n", config.CacheDir())

	// Repo count uses discovery only (no git enrichment); stays silent/fast.
	entries, err := scan.DiscoverAll(opts)
	repos := 0
	if err == nil {
		repos = countRepos(scan.ToRepos(entries))
	}
	fmt.Fprintf(out, "repos found: %d\n", repos)
	return nil
}

func layoutLabel(layout string) string {
	if layout == "" {
		return config.LayoutDiscover
	}
	return layout
}

func nameLabel(mode string) string {
	if mode == "" {
		return config.NameRemote
	}
	return mode
}

// RunOrgs lists org names that contain at least one discovered repo.
func (a *App) RunOrgs(opts config.Options) error {
	entries, err := scan.DiscoverAll(opts)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	var orgs []string
	for _, e := range entries {
		if e.Org == "" {
			continue
		}
		if seen[e.Org] {
			continue
		}
		seen[e.Org] = true
		orgs = append(orgs, e.Org)
	}
	sort.Strings(orgs)
	for _, o := range orgs {
		fmt.Fprintln(a.Stdout, o)
	}
	return nil
}

// RunMissing lists GitHub repos not cloned under the roots (online).
func (a *App) RunMissing(ctx context.Context, opts config.Options) error {
	opts.Online = true
	opts.IncludeMissing = true
	opts.All = true
	opts.IssuesOnly = false
	p := a.progress(opts, "missing")
	report, err := a.BuildReport(ctx, opts, filter.ModeArrive, p)
	if err != nil {
		p.Fail(err.Error())
		return err
	}
	p.End(fmt.Sprintf("%d missing clones", len(report.Missing)))

	if opts.JSON {
		// Minimal report: just the missing list + provenance.
		out := model.Report{
			Root:        report.Root,
			Mode:        report.Mode,
			GeneratedAt: report.GeneratedAt,
			Missing:     report.Missing,
		}
		return render.JSON(a.Stdout, out)
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
	opts.CheckAllPRs = true
	opts.All = true
	opts.IssuesOnly = false
	p := a.progress(opts, "prs")
	report, err := a.BuildReport(ctx, opts, filter.ModeStatus, p)
	if err != nil {
		p.Fail(err.Error())
		return err
	}
	p.End("done")

	if opts.JSON {
		return render.JSON(a.Stdout, report)
	}

	colorOn := config.ColorEnabled(opts.NoColor, render.IsTTY(os.Stdout))
	th := render.NewThemeOpts(colorOn, colorOn)

	type prRow struct {
		r model.Repo
	}
	var rows []prRow
	for _, org := range report.Orgs {
		for _, r := range org.Repos {
			if r.PR != nil {
				rows = append(rows, prRow{r})
			}
		}
	}
	if len(rows) == 0 {
		fmt.Fprintln(a.Stdout, "no open PRs on current branches")
		return nil
	}

	fmt.Fprintln(a.Stdout)
	for _, row := range rows {
		r := row.r
		dot := th.StatusIssue.Render("●")
		label := styleLabel(r.DisplayName(), th)
		branch := styleBranchCell(r, th)
		pr := render.PRLink(th, r.PR.Number, r.PR.URL, false)
		url := th.Meta.Render(r.PR.URL)
		fmt.Fprintf(a.Stdout, "%s  %s  %s  %s  %s\n", dot, label, branch, pr, url)
	}
	return nil
}

// styleLabel dims the org/ prefix of an owner/repo label.
func styleLabel(label string, th render.Theme) string {
	if i := strings.LastIndex(label, "/"); i > 0 && i < len(label)-1 {
		return th.Org.Render(label[:i+1]) + th.Repo.Render(label[i+1:])
	}
	return th.Repo.Render(label)
}

// styleBranchCell highlights non-default / detached branches.
func styleBranchCell(r model.Repo, th render.Theme) string {
	plain := r.Branch
	if r.Detached {
		plain = "DETACHED"
	}
	if plain == "" {
		plain = "—"
		return th.Meta.Render(plain)
	}
	defaults := config.DefaultBranches
	if r.DefaultBranch != "" {
		defaults = []string{r.DefaultBranch}
	}
	if r.Detached {
		return th.Danger.Render(plain)
	}
	if config.IsDefaultBranch(r.Branch, defaults) {
		return th.Branch.Render(plain)
	}
	return th.BranchHL.Render(plain)
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

// RunInit writes a sample config if none exists (or with --force).
func (a *App) RunInit(opts config.Options, force bool) error {
	path := config.ConfigPath()
	if !force {
		if _, err := os.Stat(path); err == nil {
			fmt.Fprintf(a.Stdout, "config exists: %s (use --force to overwrite)\n", path)
			return nil
		}
	}
	f := config.SampleConfig()
	// Prefer ~/git if it exists as a root candidate.
	existing := config.ExistingRoots([]string{"~/git"})
	if len(existing) > 0 {
		f.Roots = []string{"~/git"}
	}
	if err := config.WriteFile(path, f); err != nil {
		return err
	}
	fmt.Fprintf(a.Stdout, "wrote %s\n", path)
	return nil
}
