# Red / green recovery process

## Philosophy

Rebuild to the **good binary’s observed behavior** and the **intact source packages**.
Do not invent a new product. Green = tests + build + feature gates + reviewer PASS.

## Run identity

Each `./scripts/recovery-loop.sh` invocation:

1. Requires clean worktree and existing `main` baseline commit
2. Allocates `RECOVERY_ID` (timestamp + pid) unless set
3. Creates branch `recovery/<id>` from `main`
4. Writes state under `todo/state/<id>/`
5. Commits writer progress on that branch; pushes branch if `origin` exists

Never commit recovery work directly on `main` inside the loop.

## Loop (one iteration)

```text
RED  1. Measure: go test, go build, feature-check vs recovery/gitaware.good
     2. Writer (synthetic GLM-5.2): fix compile + implement missing app API/features + UI polish
GREEN 3. Re-measure gates (includes UI table/dots checks)
     4. Reviewer (xai grok-4.5): audit diff, tests, anti-cheat, feature parity, UI beauty
     5. If reviewer PASS and all gates green → STOP success
     6. Else append reviewer notes → next iteration
```

## Gates (all required)

| Gate | Command / check |
|------|-----------------|
| G1 compile | `go build -o recovery/gitaware.candidate ./cmd/gitaware` |
| G2 unit | `go test ./...` |
| G3 vet | `go vet ./...` |
| G4 fmt | `gofmt -l .` empty (or format in writer turn) |
| G5 good bin safe | sha256 match `recovery/gitaware.good` |
| G6 features | `scripts/recovery-feature-check.sh` |
| G7 reviewer | `todo/state/review-ITER.md` contains `VERDICT: PASS` |

## Feature check (G6) high level

Against **candidate** binary (never replace `.good`):

1. `version` / `doctor` exit 0; doctor mentions config + roots + repos found
2. Help text contains: `check-remote`, `MUTATING`, `-d`, `discover`, arrive “Does not list uncloned”
3. `leave --json`: behind-only clean repos can be `ok: true` (spot-check via jq)
4. `arrive --json`: `missing` empty or absent length 0; mode matches `/online/` and ls-remote or fetch
5. `--json` repo has `host` or `remote_owner`, `path_org` fields when applicable
6. `printf n \| fetch` prints WARNING and aborts
7. Candidate discovers repos under configured root (count > 0 with user config)

## Writer constraints (GLM)

- Read `todo/01-FEATURES.md`, `03-APP-API.md`, `AGENTS.md`
- Prefer editing `internal/app/app.go` first until build is green
- Do not weaken tests; add tests if needed
- Do not touch `recovery/gitaware.good`
- Do not `go install` to `~/go/bin` during recovery (optional final step only)
- Run `gofmt` on edited Go files

## Reviewer constraints (Grok)

- Read-only preferred; may run tests
- Must reject: deleted tests, vacuous asserts, arrive listing missing by default,
  fetch without confirm, path-based identity ignoring remotes
- Output format required:

```markdown
## Review iteration N
### Gates
### Test integrity
### Feature parity
### Code quality
### Required fixes (if any)
VERDICT: PASS|FAIL
```

## Max iterations

Default **15**. Env `RECOVERY_MAX_ITER` overrides.

## Stop conditions

- **Success:** G1–G7 all green
- **Fail:** max iterations or G5 (good binary checksum) fails
- **Abort:** operator kills script

## State files

```text
todo/state/
  iteration          # current number
  gate-log.md        # append-only
  test-hashes-start  # sha256 of *_test.go at loop start
  test-change-log.md # required if tests modified
  review-N.md        # reviewer output
  writer-N.md        # optional writer summary
```
