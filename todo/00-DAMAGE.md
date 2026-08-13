# Damage report (read-only facts)

## What is lost

- Modern `internal/app/app.go` was overwritten by `git checkout --` against an
  **uncommitted** repo. Git only had an early MVP staged copy (~354 lines).
- That file orchestrated: multi-root scan, progress, mode defaults (leave/arrive),
  doctor/init/orgs/fetch/clone-missing/prs/missing, and wiring to `enrich.go`.

## What still exists (source)

| Area | State |
|------|--------|
| `cmd/gitaware/main.go` | Modern CLI surface |
| `internal/app/enrich.go` | Modern (expects progress + new APIs) |
| `internal/scan`, `config`, `filter`, `gitlocal`, `ghonline`, `render`, `progress`, `confirm`, `model`, `cache` | Modern; most unit tests **pass** |
| `internal/app/app.go` | **Broken** old MVP; does not compile with the rest |

## What is preserved (binary)

| Copy | Path |
|------|------|
| In-repo | `recovery/gitaware.good` |
| Off-repo backup | `~/.local/share/gitaware-recovery/gitaware.good` |
| Checksum | `recovery/gitaware.good.sha256` |

Binary mtime: **2026-08-13 14:27** (pre-destruction build that includes
`--missing` opt-in and arrive-without-missing-list).

**Do not overwrite, strip, or `go install` over these until recovery is done.**
The PATH binary (`~/go/bin/gitaware`) may still match; treat `recovery/gitaware.good`
as the only trusted reference.

## Evidence sources for rebuild

1. Good binary behavior (`todo/baselines/*`, `strings` symbols)
2. Intact packages + their tests
3. `cmd/gitaware/main.go` call sites (`RunStatus`, `RunInit`, `BuildReport` via modes, etc.)
4. `enrich.go` signatures (`enrichLocal`/`enrichOnline` need `*progress.Progress`)
5. User config: `todo/baselines/user-config.json`

## Compile failure today

```text
internal/app/app.go: undefined render.NewColor, scan.Discover
enrichLocal/enrichOnline: missing progress argument
```
