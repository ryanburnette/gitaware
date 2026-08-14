# gitaware

CLI that reports git status across one or more project roots.

## Skills

- `skill:go-develop`
- `skill:shell-scripting` when touching scripts
- `skill:prose` for README and user-facing text

## Layout

```text
cmd/gitaware/          entrypoint, flag dispatch, usage text
internal/app/          commands, enrich pipeline
internal/config/       flags + ~/.config/gitaware/config.json
internal/scan/         multi-root layouts (flat, org/repo, discover, …)
internal/gitlocal/     git exec + porcelain v2 parse
internal/ghonline/     gh exec + cache-backed lists
internal/filter/       leave/arrive/status issue rules
internal/render/       table + JSON + OSC 8 links (lipgloss)
internal/progress/     multi-step stderr progress
internal/model/        shared types
internal/cache/        ~/.cache/gitaware JSON TTL cache
```

## Commands vs flags

- **Commands** = what to do (`status`, `leave`, `arrive`, `prs`, …).
- **Flags** = how (`-a`, `-C`, `--online`, `--layout`, …).
- Default command is `status` when the first arg is a flag or missing.
- Usage must keep this distinction obvious (`-a` = show clean repos).

## Invariants

- **Read-only by default.** Network reads (`ls-remote`, `gh`) are fine; disk/git mutations are not silent.
- Mutating ops (`arrive --fetch`, alias `fetch`, other `--fetch`) require interactive `Y` or `-y`.
- Out-of-date check without mutate: `git ls-remote` (`--check-remote` / `arrive` default), not fetch.
- `fetch` command is a thin alias of `arrive --fetch` (same report path).
- Offline local status by default. Online: `--online`, `arrive`, `prs`, `missing`.
- Shell out to `git` and `gh`; do not add go-git unless there is a clear win.
- Identity (host/org/repo) from **origin remote**, not path layout.
- Walk roots: `-d` > `GITAWARE_ROOT` > config `roots` > **cwd**. Default layout `discover`.
- `flag.FlagSet` subcommands — no Cobra/Viper.
- No Makefile (go-develop).
- Precedence: flags > env > config file > built-in defaults.

## Pre-commit

```sh
go fmt ./...
goimports -w .
go build ./...
go test ./...
go vet ./...
```

## Gotchas

- `GroupByOrg` must not keep a pointer across `append` (use index map).
- `leave` must not treat behind-only as an issue.
- `arrive` defaults to **ls-remote** freshness (no fetch). `--fetch` is opt-in and confirms; prefer that over the `fetch` alias.
- `arrive` does **not** list uncloned repos; that is only `missing` / `--missing`.
- Porcelain v2 `branch.ab` absent ⇒ `no_upstream`, not ahead/behind 0.
- GitHub API uses **origin** owner/name, not folder path alone.
- `gh pr list --head owner:branch` may return empty; try bare branch first.
- Progress on **stderr**; live `\r` only when stderr is a TTY; off for `--json`.
- `-a` / `--all` and config `display.show_all` include clean `○` rows; `--issues` forces issues-only.
