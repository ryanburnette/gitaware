# Unit tests required

## Rules (anti-cheat)

1. **Prefer adding tests** over deleting/weakening them.
2. Changing an existing test body requires a written reason in
   `todo/state/test-change-log.md` that iteration (reviewer verifies).
3. Forbidden without reviewer **PASS + explicit approve**: deleting `Test*`,
   renaming away assertions, `t.Skip` on recovery-critical tests, always-true asserts.
4. Baseline: hash of all `*_test.go` at loop start; each iteration diffs hashes.

---

## Existing tests (must keep passing)

### config

- `TestLoadMissingReturnsDefaults`
- `TestLoadAndResolveShowAll` — config `show_all`; `--issues` wins
- `TestResolveRootFlag` — `-d`/`Root` single root

### confirm

- `TestAskYesFlag`, `TestAskDecline`, `TestAskAccept`

### filter

- `TestLeaveIgnoresBehind` — **critical leave invariant**
- `TestLeaveCatchesDirtyAndAhead`
- `TestLeaveStrictBranch`
- `TestArriveCatchesBehind`
- `TestStatusCleanOK`

### ghonline

- `TestParseRemote` — host/owner/repo for git@, https, gitlab
- `TestRemoteMismatch` — empty path ≠ mismatch
- `TestMissingClones` — forks/archived filters

### gitlocal

- Porcelain: clean main, dirty ahead, no upstream, detached, stash count
- Remote: `TestParseLSRemoteSHA`, `TestSplitUpstream`

### progress

- `TestProgressSteps`, `TestProgressDisabled`

### render

- `TestTreeTable`, `TestNotesOmitsCounts`
- `TestHyperlinkEnabled`, `TestHyperlinkDisabled`, `TestPRLink`

### scan

- `TestDiscoverOrgRepo`, `TestDiscoverFlat`, `TestDiscoverDiscover`
- `TestGroupByOrg`, `TestDisplayName` (remote first; non-github host)

---

## Tests to add if missing (recovery should ensure)

### config

- [ ] `TestResolveCwdWhenNoRoots` — empty config roots → cwd
- [ ] `TestResolveNameDefaultRemote`
- [ ] `TestExpandHome`

### filter

- [ ] `TestStatusBehindOnlyOfflineNotIssue` (unless check-remote/online)
- [ ] `TestRemoteNewerIsArriveIssue`

### app (package tests OK; may use temp dirs + fake git later)

- [ ] `TestModeLabel` — offline / online+ls-remote / online+fetch
- [ ] `TestBuildReportArriveNoMissingByDefault` — can use stub or skip network with opts
- [ ] `TestIsIssues`

### scan

- [ ] `TestFilterByOrg`
- [ ] `TestApplyIdentityFromRemoteURL` if exported path used

### gitlocal

- [ ] Table tests already cover parse; optional CheckRemote integration skipped without network

### cmd (optional lightweight)

- [ ] Help contains `check-remote`, `MUTATING`, arrive “Does not list uncloned”

---

## Gate command

```sh
go test ./...
go vet ./...
```

All packages must compile. `internal/app` must have tests once API is restored
(at least `modeLabel` / `IsIssues`).
