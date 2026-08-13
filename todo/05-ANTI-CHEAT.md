# Anti-cheat rules for recovery loop

## Hard fails (reviewer must FAIL)

1. Delete or empty any pre-existing `Test*` without replacement that preserves intent
2. `t.Skip`, `t.Skipf` on recovery-critical tests (filter leave/behind, parse remote, etc.)
3. Assertions that cannot fail (`if true`, empty want slices ignored, etc.)
4. Changing `TestLeaveIgnoresBehind` to expect behind as leave issue
5. Making `IncludeMissing` default true on arrive to “match” a wrong test
6. Overwriting or deleting `recovery/gitaware.good`
7. Feature-check script edited to always succeed (diff against start hash)
8. Binary candidate is just a copy of `.good` without rebuilding from source
   (`recovery-feature-check` verifies candidate is newer build from tree via
   `go build` in the same iteration)

## Soft fails (reviewer FAIL unless justified in test-change-log)

1. Any diff in `*_test.go` vs start-of-loop hash without log entry
2. Lowering coverage by removing edge cases
3. Widening timeouts / ignoring errors in tests

## Allowed

1. **Add** new tests
2. Fix tests that called removed APIs **only if** behavior is preserved and logged
3. gofmt / imports on test files without logic change (still log “format only”)

## Automated checks

- `scripts/recovery-check.sh` hashes all `*_test.go` and `scripts/recovery-feature-check.sh`
- Compares to `todo/state/test-hashes-start` and `todo/state/feature-check-hash-start`
- Non-empty unexpected diff → gate fail until `test-change-log.md` updated **and**
  reviewer PASS
