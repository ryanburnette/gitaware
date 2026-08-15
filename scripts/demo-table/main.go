// Command demo-table prints a fictional gitaware status table using the real
// renderer. For README screenshots only — no disk scan, no network.
//
//	go run ./scripts/demo-table
//	go run ./scripts/demo-table --no-color
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ryanburnette/gitaware/internal/model"
	"github.com/ryanburnette/gitaware/internal/render"
)

func main() {
	noColor := flag.Bool("no-color", false, "plain table (no ANSI)")
	flag.Parse()

	color := !*noColor && os.Getenv("NO_COLOR") == ""
	// Force color even when stdout is piped (for scripted captures).
	if color && os.Getenv("CLICOLOR_FORCE") == "" {
		_ = os.Setenv("CLICOLOR_FORCE", "1")
	}
	th := render.NewThemeOpts(color, color)

	report := demoReport()
	render.Tree(os.Stdout, report, th)

	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "demo-table: fictional data via real renderer.")
	fmt.Fprintln(os.Stderr, "Screenshot this terminal (color on). Only real name: ryanburnette/gitaware.")
}

func demoReport() model.Report {
	// One real public repo (this project). Everything else is made-up example/*.
	repos := []model.Repo{
		{
			Host: "github.com", Org: "ryanburnette", Name: "gitaware",
			Label: "ryanburnette/gitaware", Path: "~/git/ryanburnette/gitaware",
			IsRepo: true, Branch: "main", DefaultBranch: "main", HasRemote: true,
			OK: true,
		},
		{
			Host: "github.com", Org: "example", Name: "api",
			Label: "example/api", Path: "~/git/example/api",
			IsRepo: true, Branch: "main", DefaultBranch: "main", HasRemote: true,
			Dirty: 3, Untracked: 2,
			Signals: []model.Signal{model.SignalDirty, model.SignalUntracked},
		},
		{
			Host: "github.com", Org: "example", Name: "web",
			Label: "example/web", Path: "~/git/example/web",
			IsRepo: true, Branch: "feature/checkout", DefaultBranch: "main", HasRemote: true,
			Ahead: 2, Upstream: "origin/feature/checkout",
			PR: &model.PR{
				Number: 42,
				URL:    "https://github.com/example/web/pull/42",
			},
			Signals: []model.Signal{model.SignalAhead, model.SignalNotDefault, model.SignalOpenPR},
		},
		{
			Host: "github.com", Org: "example", Name: "workers",
			Label: "example/workers", Path: "~/git/example/workers",
			IsRepo: true, Branch: "main", DefaultBranch: "main", HasRemote: true,
			Behind: 4, RemoteChecked: true, RemoteNewer: true,
			Signals: []model.Signal{model.SignalBehind, model.SignalRemoteNewer},
		},
		{
			Host: "github.com", Org: "example", Name: "docs",
			Label: "example/docs", Path: "~/git/example/docs",
			IsRepo: true, Branch: "main", DefaultBranch: "main", HasRemote: true,
			Stash: 1, Dirty: 1,
			Signals: []model.Signal{model.SignalStash, model.SignalDirty},
		},
		{
			Host: "github.com", Org: "example", Name: "cli",
			Label: "example/cli", Path: "~/git/example/cli",
			IsRepo: true, Branch: "wip/experiments", DefaultBranch: "main", HasRemote: true,
			Signals: []model.Signal{model.SignalNoUpstream, model.SignalNotDefault},
		},
		{
			Org: "example", Name: "scratch",
			Label: "example/scratch", Path: "~/git/example/scratch",
			IsRepo: true, Branch: "main", DefaultBranch: "main", HasRemote: false,
			Signals: []model.Signal{model.SignalNoRemote},
		},
		{
			Host: "github.com", Org: "example", Name: "legacy",
			Label: "example/legacy", Path: "~/git/example/legacy",
			IsRepo: true, Detached: true, HasRemote: true,
			Signals: []model.Signal{model.SignalDetached},
		},
	}

	issues, okN := 0, 0
	for i := range repos {
		if repos[i].OK && len(repos[i].Signals) == 0 {
			okN++
			continue
		}
		repos[i].OK = false
		issues++
	}

	return model.Report{
		Root:        "~/git",
		Mode:        "online+ls-remote",
		GeneratedAt: time.Now().UTC(),
		Orgs: []model.Org{
			{Name: "ryanburnette", Repos: repos[:1]},
			{Name: "example", Repos: repos[1:]},
		},
		Summary: model.Summary{
			Repos:  len(repos),
			Issues: issues,
			OK:     okN,
		},
	}
}
