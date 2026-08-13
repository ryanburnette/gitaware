package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ryanburnette/gitaware/internal/app"
	"github.com/ryanburnette/gitaware/internal/config"
	"github.com/ryanburnette/gitaware/internal/confirm"
	"github.com/ryanburnette/gitaware/internal/filter"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) > 0 {
		switch args[0] {
		case "-V", "-version", "--version", "version":
			fmt.Printf("gitaware %s\n", app.Version)
			if app.BuildTime != "unknown" {
				fmt.Printf("Built: %s\n", app.BuildTime)
			}
			return 0
		case "help", "--help":
			printVersionUsage()
			return 0
		}
	}

	cmd := "status"
	rest := args
	if len(args) > 0 && !isFlag(args[0]) {
		cmd = args[0]
		rest = args[1:]
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a := app.New()

	var err error
	switch cmd {
	case "status":
		err = cmdStatus(ctx, a, rest, filter.ModeStatus)
	case "leave":
		err = cmdStatus(ctx, a, rest, filter.ModeLeave)
	case "arrive":
		err = cmdArrive(ctx, a, rest)
	case "missing":
		err = cmdMissing(ctx, a, rest)
	case "prs":
		err = cmdPRs(ctx, a, rest)
	case "fetch":
		err = cmdFetch(ctx, a, rest)
	case "clone-missing":
		err = cmdCloneMissing(ctx, a, rest)
	case "init":
		err = cmdInit(a, rest)
	case "orgs":
		err = cmdOrgs(a, rest)
	case "doctor":
		err = cmdDoctor(a, rest)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		printUsage(os.Stderr)
		return 2
	}

	if err == nil {
		return 0
	}
	if err == errAborted {
		return 0
	}
	if app.IsIssues(err) {
		return 1
	}
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	return 2
}

func isFlag(s string) bool {
	return len(s) > 0 && s[0] == '-'
}

// commonFlags are modifiers shared by most commands.
func commonFlags(fs *flag.FlagSet, opts *config.Options) {
	// Discovery — walk dirs for .git; host/org/repo identity comes from origin remotes
	fs.StringVar(&opts.Root, "d", "", "directory to walk (default: config roots, else cwd)")
	fs.StringVar(&opts.Root, "C", "", "same as -d")
	fs.StringVar(&opts.Root, "root", "", "same as -d")
	fs.StringVar(&opts.Layout, "layout", "", "how to walk: discover (default) | flat | org/repo | host/org/repo")
	fs.IntVar(&opts.Depth, "depth", 0, "max depth when layout=discover (default 3)")
	fs.StringVar(&opts.OrgFilter, "org", "", "limit to one org (remote owner)")
	fs.StringVar(&opts.NameMode, "name", "", "label: remote (default) | path | folder | auto")

	// What to show
	fs.BoolVar(&opts.AllFlag, "a", false, "show all repos including clean (○)")
	fs.BoolVar(&opts.AllFlag, "all", false, "same as -a")
	fs.BoolVar(&opts.IssuesFlag, "issues", false, "show only repos with issues (default)")

	// Network (read-only unless --fetch)
	fs.BoolVar(&opts.CheckRemote, "check-remote", false, "non-mutating: ls-remote to see if remote has updates")
	fs.BoolVar(&opts.Fetch, "fetch", false, "MUTATING: git fetch before status (asks to confirm)")
	fs.BoolVar(&opts.Online, "online", false, "use gh (PRs, default branch)")
	fs.BoolVar(&opts.IncludeMissing, "missing", false, "include uncloned GitHub repos (noisy; prefer: gitaware missing)")
	fs.BoolVar(&opts.Refresh, "refresh", false, "ignore gh cache")
	fs.BoolVar(&opts.Yes, "y", false, "skip confirmation for mutating operations")
	fs.BoolVar(&opts.Yes, "yes", false, "same as -y")

	// Output
	fs.BoolVar(&opts.JSON, "json", false, "JSON on stdout (no progress)")
	fs.BoolVar(&opts.Verbose, "v", false, "verbose progress on stderr")
	fs.BoolVar(&opts.NoColor, "no-color", false, "disable colors")
	fs.BoolVar(&opts.WarnNonRepos, "w", false, "include non-git directories")
	fs.IntVar(&opts.Workers, "workers", 0, "parallel workers")
	fs.BoolVar(&opts.NoForks, "no-forks", false, "exclude forks from missing")
	fs.BoolVar(&opts.IncludeArchived, "archived", false, "include archived in missing")
}

func parseCommon(name string, args []string) (config.Options, []string, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var opts config.Options
	commonFlags(fs, &opts)

	if err := helpGuard(fs, name, args); err != nil {
		return opts, nil, err
	}
	if err := fs.Parse(args); err != nil {
		return opts, nil, err
	}
	app.SetupVerbose(opts.Verbose)
	if err := config.Resolve(&opts); err != nil {
		return opts, nil, err
	}
	return opts, fs.Args(), nil
}

type helpExit struct{}

func (helpExit) Error() string { return "help" }

func helpGuard(fs *flag.FlagSet, name string, args []string) error {
	for _, a := range args {
		if a == "-h" || a == "-help" || a == "--help" {
			printCmdHelp(name, fs)
			return helpExit{}
		}
	}
	return nil
}

func printCmdHelp(name string, fs *flag.FlagSet) {
	fmt.Fprintf(os.Stdout, "Usage: gitaware %s [flags]\n\n", name)
	switch name {
	case "status":
		fmt.Fprintln(os.Stdout, "Show a status table of local git repos.")
		fmt.Fprintln(os.Stdout, "Default: only repos with issues. Use -a to include clean repos.")
	case "leave":
		fmt.Fprintln(os.Stdout, "Offline checklist before you walk away (dirty, unpushed, wrong branch, stash).")
		fmt.Fprintln(os.Stdout, "Does not treat \"behind\" as a leave-blocker.")
	case "arrive":
		fmt.Fprintln(os.Stdout, "Online catch-up after switching machines.")
		fmt.Fprintln(os.Stdout, "Default: non-mutating ls-remote freshness + local drift (no fetch).")
		fmt.Fprintln(os.Stdout, "Does not list uncloned repos — use: gitaware missing")
		fmt.Fprintln(os.Stdout, "Use --fetch to update remote-tracking refs (mutates; confirms).")
	case "prs":
		fmt.Fprintln(os.Stdout, "List open GitHub PRs for each repo's current branch (read-only).")
	case "missing":
		fmt.Fprintln(os.Stdout, "List GitHub repos not cloned under your roots (read-only).")
	case "fetch":
		fmt.Fprintln(os.Stdout, "MUTATING: git fetch in every local repo (confirms unless -y).")
	case "clone-missing":
		fmt.Fprintln(os.Stdout, "MUTATING: clone missing GitHub repos into the first root (confirms unless -y).")
	case "init":
		fmt.Fprintln(os.Stdout, "Write ~/.config/gitaware/config.json with detected defaults.")
	case "doctor":
		fmt.Fprintln(os.Stdout, "Print effective config, git/gh status, and repo count.")
	case "orgs":
		fmt.Fprintln(os.Stdout, "List org names discovered under roots.")
	}
	fmt.Fprintln(os.Stdout, "\nFlags:")
	fs.SetOutput(os.Stdout)
	fs.PrintDefaults()
	fmt.Fprintln(os.Stdout, `
Commands choose what to do. Flags modify how.

  gitaware              # status, issues only
  gitaware -a           # status, every repo (clean = ○)
  gitaware leave        # don't-walk-away checklist
  gitaware leave -a     # leave rules, but list clean too
  gitaware arrive       # online + fetch
  gitaware prs          # open PRs on current branches`)
}

func cmdStatus(ctx context.Context, a *app.App, args []string, mode filter.Mode) error {
	opts, _, err := parseCommon(modeName(mode), args)
	if err != nil {
		if _, ok := err.(helpExit); ok {
			return nil
		}
		return err
	}
	if err := confirmMutating(opts, "git fetch in scanned repos", "flag --fetch"); err != nil {
		return err
	}
	return a.RunStatus(ctx, opts, mode)
}

func modeName(m filter.Mode) string {
	switch m {
	case filter.ModeLeave:
		return "leave"
	case filter.ModeArrive:
		return "arrive"
	default:
		return "status"
	}
}

func cmdArrive(ctx context.Context, a *app.App, args []string) error {
	opts, _, err := parseCommon("arrive", args)
	if err != nil {
		if _, ok := err.(helpExit); ok {
			return nil
		}
		return err
	}
	opts.Online = true
	// Default: CheckRemote (set in BuildReport). --fetch is opt-in mutate.
	if err := confirmMutating(opts, "git fetch in scanned repos", "arrive --fetch"); err != nil {
		return err
	}
	return a.RunStatus(ctx, opts, filter.ModeArrive)
}

func cmdMissing(ctx context.Context, a *app.App, args []string) error {
	opts, _, err := parseCommon("missing", args)
	if err != nil {
		if _, ok := err.(helpExit); ok {
			return nil
		}
		return err
	}
	opts.IncludeMissing = true
	opts.Online = true
	return a.RunMissing(ctx, opts)
}

func cmdPRs(ctx context.Context, a *app.App, args []string) error {
	opts, _, err := parseCommon("prs", args)
	if err != nil {
		if _, ok := err.(helpExit); ok {
			return nil
		}
		return err
	}
	return a.RunPRs(ctx, opts)
}

func cmdFetch(ctx context.Context, a *app.App, args []string) error {
	opts, _, err := parseCommon("fetch", args)
	if err != nil {
		if _, ok := err.(helpExit); ok {
			return nil
		}
		return err
	}
	opts.Fetch = true // command always fetches
	if err := confirmMutating(opts, "git fetch --all --prune in every local repo", "fetch"); err != nil {
		return err
	}
	return a.RunFetch(ctx, opts)
}

func cmdCloneMissing(ctx context.Context, a *app.App, args []string) error {
	opts, pos, err := parseCommon("clone-missing", args)
	if err != nil {
		if _, ok := err.(helpExit); ok {
			return nil
		}
		return err
	}
	opts.IncludeMissing = true
	opts.Online = true
	// Always confirm clone (mutates disk), even though Fetch may be false.
	if !confirm.Ask(os.Stdout, os.Stderr, os.Stdin, opts.Yes,
		"clone missing repositories into the first configured root",
		"creates new directories and runs: gh repo clone",
	) {
		return errAborted
	}
	return a.RunCloneMissing(ctx, opts, pos)
}

var errAborted = fmt.Errorf("aborted")

// confirmMutating prompts when opts.Fetch is set (the only shared mutating flag).
func confirmMutating(opts config.Options, action string, via string) error {
	if !opts.Fetch {
		return nil
	}
	if !confirm.Ask(os.Stdout, os.Stderr, os.Stdin, opts.Yes, action, "via "+via) {
		return errAborted
	}
	return nil
}

func cmdInit(a *app.App, args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	force := fs.Bool("force", false, "overwrite existing config")
	if err := helpGuard(fs, "init", args); err != nil {
		if _, ok := err.(helpExit); ok {
			return nil
		}
		return err
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	return a.RunInit(config.Options{}, *force)
}

func cmdOrgs(a *app.App, args []string) error {
	opts, _, err := parseCommon("orgs", args)
	if err != nil {
		if _, ok := err.(helpExit); ok {
			return nil
		}
		return err
	}
	return a.RunOrgs(opts)
}

func cmdDoctor(a *app.App, args []string) error {
	opts, _, err := parseCommon("doctor", args)
	if err != nil {
		if _, ok := err.(helpExit); ok {
			return nil
		}
		return err
	}
	return a.RunDoctor(opts)
}

func printVersionUsage() {
	fmt.Printf("gitaware %s\n\n", app.Version)
	printUsage(os.Stdout)
}

func printUsage(w *os.File) {
	fmt.Fprintln(w, `gitaware — multi-repo git status for switching machines

Usage:
  gitaware [command] [flags]

Commands (what to do):
  status          Status table (default if you omit the command)
  leave           Offline: unfinished work before you leave
  arrive          Online: fetch, behind, missing clones, drift
  prs             Online: open PRs on current branches (read-only)
  missing         Online: GitHub repos not cloned
  fetch           git fetch in every local repo
  clone-missing   Clone missing repos into the first root
  init            Write ~/.config/gitaware/config.json
  doctor          Effective config + git/gh checks
  orgs            List org names under roots
  version         Print version
  help            Show this help

Flags (how — common ones):
  -a, --all         Show every repo (clean repos get a green ○)
  --issues          Show only repos with issues (default)
  -C, --root DIR    Scan only this directory (overrides config roots)
  --layout MODE     flat | org/repo | host/org/repo | discover
  --check-remote    Non-mutating: ls-remote to detect remote updates
  --fetch           MUTATING: git fetch (confirms; use -y to skip)
  -y, --yes         Skip mutating confirmation
  --online          Enable gh signals on status
  --json            JSON on stdout (hides progress)
  -v                Verbose progress on stderr

Read vs write:
  Read-only by default (status, leave, arrive, prs, missing).
  arrive uses ls-remote for freshness — does not update local refs.
  Mutating: fetch, clone-missing, and any command with --fetch.
  Those print a WARNING and require Y (or -y).

Examples:
  gitaware              # issues only
  gitaware -a           # everything, ○ = clean
  gitaware leave         # safe-to-leave checklist
  gitaware arrive        # catch-up (ls-remote + gh, no fetch)
  gitaware arrive --fetch -y
  gitaware fetch -y     # update all remote-tracking refs
  gitaware prs          # clickable PR links

Config: ~/.config/gitaware/config.json
Exit: 0 no issues · 1 issues · 2 error

Commands pick the job. Flags only change scope, roots, and output.
Use gitaware <command> -h for command-specific flags.`)
}
