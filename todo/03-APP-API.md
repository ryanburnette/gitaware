# `internal/app` rebuild checklist

Primary broken file: `internal/app/app.go`.
Keep `internal/app/enrich.go` (already modern) unless a compile fix is required.

## Must match `cmd/gitaware/main.go`

| Caller | Method |
|--------|--------|
| status/leave/arrive | `RunStatus(ctx, opts, mode)` |
| missing | `RunMissing(ctx, opts)` |
| prs | `RunPRs(ctx, opts)` |
| fetch | alias → `RunStatus(ctx, opts, ModeArrive)` with `Fetch=true` (no separate `RunFetch`) |
| clone-missing | `RunCloneMissing(ctx, opts, names)` |
| init | `RunInit(opts, force bool)` |
| orgs | `RunOrgs(opts)` |
| doctor | `RunDoctor(opts)` |
| parseCommon | `SetupVerbose` |
| main exit | `IsIssues(err)` |
| version | `Version`, `BuildTime` |

## `BuildReport` flow (target)

1. `config.Resolve(&opts)`
2. Apply **mode defaults before enrich**:
   - leave: StrictBranch, IncludeBehind=false
   - arrive: Online, IncludeBehind, StrictBranch, CheckRemote if !Fetch; **do not** set IncludeMissing
   - status: IncludeBehind if Fetch\|\|CheckRemote\|\|Online
3. Progress step `scan` → `scan.DiscoverAll` → `ToRepos`
4. `enrichLocal(ctx, repos, git, opts, p)`
5. Optional `FilterByOrg` after identity
6. `ApplyDisplayNames`
7. If Online: `enrichOnline` (missing only if IncludeMissing)
8. `deriveAll` → `GroupByOrg` → attach missing to orgs if any
9. Summary via `filter.Apply(..., false, mode)` or equivalent
10. Return `model.Report` with Root, Mode (`modeLabel`), GeneratedAt, Orgs, Missing, Summary

## `RunStatus`

- Start progress with command name
- BuildReport
- filter.Apply issuesOnly
- End progress
- JSON or Tree with `render.NewThemeOpts`
- Return `errIssues` if issues/missing > 0

## Other runners

- **RunDoctor:** effective config + git/gh + repo count (DiscoverAll)
- **RunInit:** SampleConfig, write ConfigPath, --force
- **RunOrgs:** list org names from discovered/enriched repos
- **RunMissing:** Online + IncludeMissing; print names or JSON
- **RunPRs:** Online + CheckAllPRs; print or JSON
- **RunFetch:** Fetch=true enrichLocal only (or dedicated fetch loop) + message
- **RunCloneMissing:** IncludeMissing; gh.Clone into first root

## Do not

- Call removed `scan.Discover` or `render.NewColor`
- Call enrich* without progress (use `progress.New(io.Discard, ...)` if needed)
- Default IncludeMissing on arrive
- Overwrite `recovery/gitaware.good`
