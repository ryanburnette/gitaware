# gitaware

Multi-repo git status for switching machines.

```text
gitaware -a

┌───┬──────────────────────────────┬────────┬───────┬───────┬...
│   │ Repository                   │ Branch │ Dirty │ Untrk │
├───┼──────────────────────────────┼────────┼───────┼───────┼
│ ● │ cogburnbros/app              │ main   │       │   1   │
│ ○ │ ryanburnette/authn           │ main   │       │       │
└───┴──────────────────────────────┴────────┴───────┴───────┴
```

- `●` needs attention · `○` clean  
- Default: **issues only**. Use **`-a`** to show everything.

## Install

```sh
go install github.com/ryanburnette/gitaware/cmd/gitaware@latest
# or from a clone:
go install ./cmd/gitaware
```

Needs `git` on `PATH`. Online commands also need authenticated [`gh`](https://cli.github.com/).

## Commands vs flags

| | Role | Examples |
|---|------|----------|
| **Command** | What to do | `status`, `leave`, `arrive`, `prs` |
| **Flag** | How to do it | `-a`, `-d`, `--online`, `--json` |

```text
gitaware                 # command: status (default). issues only
gitaware -a              # same command, flag: include clean repos
gitaware leave           # different command (stricter offline filter)
gitaware leave -a        # leave rules + list clean too
gitaware arrive          # online catch-up (ls-remote + gh)
gitaware prs             # list open PRs (read-only, clickable links)
```

### Commands

| Command | Network | Purpose |
|---------|---------|---------|
| `status` | no* | Status table (default) |
| `leave` | no | Unfinished work before you walk away |
| `arrive` | yes (read) | ls-remote freshness + local drift; no fetch; no missing list |
| `prs` | yes (read) | Open PRs on current branches |
| `missing` | yes (read) | GitHub repos not cloned (only place that lists them by default) |
| `fetch` | **write** | `git fetch` every local repo (confirms) |
| `clone-missing` | **write** | Clone missing into first root (confirms) |
| `init` | no | Write `~/.config/gitaware/config.json` |
| `doctor` | no | Effective config + git/gh |
| `orgs` | no | List org names |

\* `status --online` / `--check-remote` opt into network reads. `--fetch` is opt-in mutate.

### Common flags

| Flag | Meaning |
|------|---------|
| `-a`, `--all` | Show **all** repos (clean = `○`) |
| `--issues` | Show only issues (default) |
| `-d`, `-C` `DIR` | Directory to walk (default: config roots, else **cwd**) |
| `--depth N` | Max depth for discover (default 3) |
| `--layout MODE` | Walk shape: `discover` (default) \| `flat` \| `org/repo` \| `host/org/repo` |
| `--name MODE` | Label: `remote` (default) \| `path` \| `folder` \| `auto` |
| `--org NAME` | Filter by remote owner |
| `--check-remote` | Non-mutating: `ls-remote` to detect remote updates |
| `--fetch` | **Mutating:** `git fetch` (asks `Y`, or `-y`) |
| `-y`, `--yes` | Skip mutating confirmation |
| `--online` | gh signals on status |
| `--json` | JSON on stdout |
| `-v` | Verbose progress on stderr |

### Read vs write

Most commands only **read** (local git + optional `ls-remote` / `gh`).

To see if you are behind **without** updating refs: `arrive` or `--check-remote` (`git ls-remote`). That never moves `origin/*`.

**Mutating** — `fetch`, `clone-missing`, and any use of `--fetch` — prints a warning and requires `Y` (or `-y`).

```sh
gitaware -h              # overview
gitaware leave -h        # flags for one command
```

## Roots and identity

**Where to walk** (first match wins):

1. `-d` / `-C` / `--root`
2. `GITAWARE_ROOT`
3. `roots` in config
4. **current working directory**

**Who the repo is** comes from the **origin remote** (`host` / `owner` / `name`), not from folder names. Folder layout is only a walk hint.

```sh
gitaware -d ~/git -a          # walk ~/git
cd ~/git && gitaware -a       # same if no config roots
gitaware --org ryanburnette   # filter by remote owner
```

| Layout (optional) | Path shape when walking |
|-------------------|-------------------------|
| `discover` | any `.git` up to `--depth` (default) |
| `flat` | `root/repo` |
| `org/repo` | `root/org/repo` |
| `host/org/repo` | `root/github.com/org/repo` |

Labels default to **remote** identity (`owner/repo`, or `host/owner/repo` when host is not github.com).

## Config

`~/.config/gitaware/config.json` — create with `gitaware init`.

```json
{
  "version": 1,
  "roots": ["~/git"],
  "depth": 3,
  "display": {
    "name": "remote",
    "show_all": false
  }
}
```

Omit `roots` to walk the current directory. `display.show_all: true` is the same as always passing `-a`. Flags win over config.

## Columns

| Column | Meaning |
|--------|---------|
| (dot) | `●` issue · `○` clean |
| Repository | label from path and/or remote |
| Branch | current branch |
| Dirty / Untrk / Ahead / Behind / Stash | counts (colored pills when > 0) |
| Notes | `no upstream`, `not default branch`, clickable `PR #N`, … |

## Progress

Long commands print steps on **stderr**. Stdout stays the table/JSON/links.

## Exit codes

- `0` — no issues  
- `1` — one or more issues  
- `2` — tool error  

## Ignore a repo

```sh
git -C path/to/repo config gitaware.ignore true
```

Or list names under `ignore` in config.

## License

MIT
