# Recovery writer task

You are restoring **gitaware** after `internal/app/app.go` was overwritten with an ancient MVP copy. Most other packages are intact and already pass tests.

## Read first (in order)

1. `todo/00-DAMAGE.md`
2. `todo/01-FEATURES.md`
3. `todo/03-APP-API.md`
4. `todo/02-UNIT-TESTS.md`
5. `todo/05-ANTI-CHEAT.md`
6. `AGENTS.md`
7. `cmd/gitaware/main.go` (call sites)
8. `internal/app/enrich.go` (signatures you must call)
9. Current broken `internal/app/app.go`
10. `todo/baselines/` as needed

## Goal this turn

Make the project compile and converge on good-binary behavior:

- Rebuild modern `internal/app/app.go` (primary)
- Touch other files only if required for compile/API match
- Prefer **adding** tests over changing existing ones
- If you must change a test, append reason to `todo/state/test-change-log.md`

## Hard rules

- **Never** modify, delete, or overwrite `recovery/gitaware.good` or its sha256
- **Never** `go install` to `~/go/bin` during recovery
- **Never** weaken tests (see anti-cheat)
- Arrive must **not** list missing clones by default
- Mutating ops require confirm unless `-y`
- Identity from git remote; walk roots via discover/config/`-d`
- Run: `gofmt -w` on edited Go files, then `go test ./...` and `go build -o recovery/gitaware.candidate ./cmd/gitaware`

## UI quality bar (non-negotiable)

Output must be **as beautiful and organized as the good binary, or better**:

- Lipgloss bordered table, column alignment, `●`/`○` dots
- Colored count pills; notes column with OSC 8 PR links when TTY allows
- Title + summary lines; progress only on stderr
- No regressions to plain unstyled dumps

Side-by-side check when possible:
`recovery/gitaware.good -a` vs `recovery/gitaware.candidate -a` (stderr discarded).

## Done when

`go test ./...` passes and `recovery/gitaware.candidate` builds. Summarize what you changed in a few bullets at the end.
