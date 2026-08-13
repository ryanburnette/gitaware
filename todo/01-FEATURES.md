# Feature catalog (must match good binary)

Evidence: `recovery/gitaware.good`, baselines under `todo/baselines/`,
user config `~/.config/gitaware/config.json`.

Legend: **R** = read-only, **W** = mutating (confirm unless `-y`).

---

## Global invariants

1. **Read-only by default.** Network reads OK (`ls-remote`, `gh`). Disk/git writes only via explicit mutate path.
2. **Identity** (`host`/`org`/`name`) from **origin remote**, not folder layout. Path kept as `path_org`/`path_name`.
3. **Roots precedence:** `-d`/`-C`/`--root` > `GITAWARE_ROOT` > config `roots` > **cwd**.
4. **Default walk:** `layout=discover`, `depth=3`, `name=remote`.
5. **Default filter:** issues only; `-a`/`--all` shows clean (`○`).
6. **Exit codes:** `0` clean, `1` issues, `2` tool error. Mutating abort → `0`.
7. **Progress** on stderr; table/JSON/links on stdout. `--json` hides progress.
8. **Offline** unless `--online`, `--check-remote`, `--fetch`, or online commands.
9. **Config file:** `~/.config/gitaware/config.json` (or `GITAWARE_CONFIG`). Flags > env > file > defaults.

### User config (live)

```json
{
  "version": 1,
  "roots": ["~/git"],
  "layout": "discover",
  "depth": 3,
  "display": { "name": "remote", "show_all": false }
}
```

---

## Commands

### `status` (default)

| | |
|--|--|
| Network | no* |
| Purpose | Neutral status table of local repos |
| Default view | Issues only |
| * | `--check-remote` / `--online` / `--fetch` opt in |

Must:

- Discover repos under roots
- Fill dirty/untracked/ahead/behind/stash/branch/upstream/remote fields
- Derive signals; `●` issue / `○` clean when `-a`
- Support `-a`, `--issues`, `-d`, `--org`, `--layout`, `--depth`, `--name`, `--json`, `-v`, `--no-color`, `--workers`
- Behind is **not** an issue offline unless `--check-remote` / `--online` / `--fetch` / `--issues` with include-behind rules

### `leave`

| | |
|--|--|
| Network | no |
| Purpose | Don’t-walk-away checklist |

Issues: dirty, untracked, ahead, no_upstream, detached, stash, no_remote, error, remote_mismatch; not_default if strict.

**Must NOT** treat behind-only as leave issue.

### `arrive`

| | |
|--|--|
| Network | yes (read) |
| Purpose | Catch-up after switching machines |

Must:

- Force online + **CheckRemote** (ls-remote) unless `--fetch`
- Include behind / remote_newer as issues
- Show notes like `remote has updates`
- Mode label includes `ls-remote` or `fetch` as appropriate (e.g. `online+ls-remote`)
- **Must NOT list uncloned repos** by default (`missing` array empty / no “not cloned” dump)
- `--missing` may opt in; prefer dedicated `missing` command
- `--fetch` is mutating → confirm unless `-y`

### `prs`

| | |
|--|--|
| Network | yes (gh) |
| Purpose | Open PRs on current branch per repo |

Must use **remote owner/name** for `gh`, not path. Clickable PR links when TTY supports OSC 8.

### `missing`

| | |
|--|--|
| Network | yes |
| Purpose | List GitHub repos not cloned under roots |

Only command that lists missing by default. Sets `IncludeMissing`.

### `fetch` **W**

- `git fetch --all --prune` every local repo
- Always confirm unless `-y`
- Warning banner: mutates git state

### `clone-missing` **W**

- Clone missing into first root
- Always confirm unless `-y`
- Uses `gh repo clone`

### `init`

- Write config if missing (or `--force`)
- Prefer `SampleConfig` (roots `~/git` if exists, discover, name remote)

### `doctor`

Must print roughly:

```text
gitaware <version>
config: <path> (loaded|...)
layout: ... depth: ... name: ... show_all: ...
roots:
  ...
git: ok
gh: ok
gh user: ...
cache: ...
repos found: N
```

### `orgs`

List org names from discovery/identity grouping.

### `version` / `help`

- `version` / `-V` / `--version`
- `help` / `--help` usage overview

---

## Flags (common)

| Flag | Behavior |
|------|----------|
| `-a` / `--all` | Show clean repos |
| `--issues` | Force issues-only |
| `-d` / `-C` / `--root` | Single walk root |
| `--layout` | discover \| flat \| org/repo \| host/org/repo |
| `--depth` | discover max depth (default 3) |
| `--name` | remote \| path \| folder \| auto |
| `--org` | Filter by remote owner (after identity) |
| `--check-remote` | Non-mutating ls-remote freshness |
| `--fetch` | Mutating fetch + confirm |
| `--missing` | Opt-in uncloned list on online commands |
| `--online` | gh signals |
| `--refresh` | bypass gh cache |
| `-y` / `--yes` | Skip mutate confirm |
| `--json` | JSON stdout |
| `-v` | Verbose progress |
| `--no-color` | No colors |
| `-w` | Warn/include non-git dirs |
| `--workers` | Parallelism |
| `--no-forks` / `--archived` | missing filters |

---

## JSON contract (`--json`)

Top-level keys: `root`, `mode`, `generated_at`, `orgs`, `summary`, optional `missing`.

Repo object must support (good binary sample):

`host`, `org`, `name`, `path_org`, `path_name`, `label`, `path`, `is_repo`,
`branch`, `upstream`, `ahead`, `behind`, `dirty`, `untracked`, `stash`,
`has_remote`, `remote_url`, `remote_owner`, `remote_name`, `detached`,
`signals`, `ok`, optional `pr`, `remote_checked`, `remote_newer`, `default_branch`, `error`.

Summary: `repos`, `issues`, `missing`, `ok`.

Arrive baseline meta: `mode` contains online + ls-remote; `missing_len` = 0.

---

## UI (beauty bar = good binary, meet or beat)

The repaired CLI must look **as polished as `recovery/gitaware.good`, or better**. Do not ship a plain text dump.

### Required look and feel

- **Lipgloss bordered table** with aligned columns (Repository, Branch, Dirty, Untrk, Ahead, Behind, Stash, Notes)
- Status dots: `●` issue (attention) · `○` clean when `-a`
- **Count pills** (colored chips) for dirty/untrk/ahead/behind/stash when > 0; empty cell when 0
- Branch styling: warn on non-default / detached
- Notes: calm, readable — `no upstream`, `not default branch`, `remote has updates`, `no remote`, OSC 8 clickable `PR #N`
- Title line: `gitaware  <root>  ·  <mode>`
- Summary line: `N issues  ·  M repos  ·  <mode>` (or all-clear)
- Progress on **stderr** only: stepped `•` / `✓` (scan, status+remote, github, prs); no progress noise in `--json`
- Colors respect `NO_COLOR` / `--no-color`; still readable without color
- Wide labels truncate cleanly; table does not explode layout on long branch names

### Reviewer UI gate

Compare candidate table output to good binary on the same machine (`-a` sample). FAIL if candidate is clearly uglier, unaligned, missing pills/dots, or dumps unstructured lines where the good binary shows a table.

---

## Layouts (walk only)

| Layout | Shape |
|--------|--------|
| discover | walk for `.git` to depth |
| flat | `root/repo` |
| org/repo | `root/org/repo` |
| host/org/repo | `root/host/org/repo` |

Identity still from remote after enrich.

---

## Signals

`dirty`, `untracked`, `ahead`, `behind`, `no_upstream`, `not_default`, `detached`,
`stash`, `no_remote`, `not_repo`, `open_pr`, `missing_clone`, `remote_mismatch`,
`remote_newer`, `error`.

---

## Package API surface (from good binary symbols)

`internal/app` must provide:

- `New`, `SetupVerbose`, `IsIssues`, `Version`, `BuildTime`
- `(*App).RunStatus`, `BuildReport`, `RunDoctor`, `RunOrgs`, `RunMissing`,
  `RunPRs`, `RunFetch`, `RunCloneMissing`, `RunInit`
- helpers used internally: `progress`, `modeLabel`, `countRepos`, `enrichLocal`,
  `enrichOnline`, `deriveAll`, `ghRepo`

`BuildReport` signature expected by modern code:

```go
BuildReport(ctx, opts, mode, p *progress.Progress) (model.Report, error)
```

`RunStatus` uses progress and `render.NewThemeOpts` (not deleted `NewColor`).

Discovery: `scan.DiscoverAll` / `DiscoverRoot` (not old `scan.Discover`).
