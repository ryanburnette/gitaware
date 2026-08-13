# gitaware recovery

The modern `internal/app/app.go` was destroyed. This directory is the recovery
plan and automation.

## Prerequisites

1. **Baseline on `main`** committed and pushed (source + `recovery/gitaware.good` + todo/scripts).
2. Clean worktree (`git status` empty).
3. `pi` auth for synthetic + xai models.

## Start here

```sh
cd ~/git/ryanburnette/gitaware
git checkout main
git pull   # if remote exists
./scripts/recovery-loop.sh
```

Each run creates:

- **ID:** `YYYYMMDDTHHMMSSZ-<pid>` (override with `RECOVERY_ID=...`)
- **Branch:** `recovery/<id>` from `main`
- **State dir:** `todo/state/<id>/` (gitignored)

Optional env:

```sh
RECOVERY_MAX_ITER=20 \
RECOVERY_WRITER_MODEL='synthetic/hf:zai-org/GLM-5.2' \
RECOVERY_REVIEWER_MODEL='xai/grok-4.5' \
  ./scripts/recovery-loop.sh
```

## Layout

| Path | Purpose |
|------|---------|
| `00-DAMAGE.md` | What broke |
| `01-FEATURES.md` | Full feature catalog from good binary |
| `02-UNIT-TESTS.md` | Required tests |
| `03-APP-API.md` | `app` package rebuild checklist |
| `04-RED-GREEN.md` | Loop process |
| `05-ANTI-CHEAT.md` | No cheating tests/gates |
| `baselines/` | Frozen help/doctor/JSON from good binary |
| `state/` | Created by the loop (iteration logs) |

## Good binary (do not overwrite)

- `recovery/gitaware.good`
- `~/.local/share/gitaware-recovery/gitaware.good`
- Checksum: `recovery/gitaware.good.sha256`

## Loop models

1. **Writer:** synthetic GLM-5.2 — implements fixes  
2. **Reviewer:** xai grok-4.5 — audits; must emit `VERDICT: PASS` or `FAIL`

Stops when unit tests, build, feature checks, and reviewer all pass, or max
iterations (default 15).
