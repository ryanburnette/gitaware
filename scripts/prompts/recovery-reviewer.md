# Recovery reviewer task

You review a recovery iteration of **gitaware**. Be skeptical. Prefer FAIL when unsure.

## Read

1. `todo/01-FEATURES.md`, `todo/03-APP-API.md`, `todo/05-ANTI-CHEAT.md`
2. `todo/state/gate-log.md` (latest gate results)
3. `todo/state/test-change-log.md` if present
4. Diff of concern: `internal/app/app.go` and any changed `*_test.go`
5. Optionally run `go test ./...` and `scripts/recovery-feature-check.sh`

## Check

1. Gates G1–G6 actually green (or note which failed)
2. Test integrity — no cheating
3. Feature parity with good binary catalog (especially arrive/missing/fetch confirm/remote identity)
4. `app.go` uses `DiscoverAll`, progress, `NewThemeOpts`, mode defaults correct
5. Candidate was built from source this iteration (not a copy of `.good`)
6. **UI beauty:** table layout, dots, pills, notes, title/summary match or beat `recovery/gitaware.good`. FAIL plain/ugly output.

## Output format (mandatory)

Write the full review to stdout as markdown ending with exactly one of:

```text
VERDICT: PASS
```

or

```text
VERDICT: FAIL
```

If FAIL, list concrete required fixes the writer must do next.
