#!/bin/sh
# recovery-check.sh — red/green gates for gitaware recovery
# Exit 0 only if compile, tests, vet, good-bin checksum, and feature-check pass.
set -eu

g_root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
g_state_root="$g_root/todo/state"
# Prefer per-run state dir from recovery-loop
if test -n "${RECOVERY_STATE_DIR:-}" && test -d "$RECOVERY_STATE_DIR"; then
	g_state=$RECOVERY_STATE_DIR
elif test -f "$g_state_root/CURRENT_STATE_DIR"; then
	g_state=$(cat "$g_state_root/CURRENT_STATE_DIR")
else
	g_state=$g_state_root
fi
g_good="$g_root/recovery/gitaware.good"
g_sum="$g_root/recovery/gitaware.good.sha256"
g_cand="$g_root/recovery/gitaware.candidate"
g_log="$g_state/gate-log.md"
g_fail=0

fn_ts() {
	date -u '+%Y-%m-%dT%H:%M:%SZ'
}

fn_log() {
	printf '%s\n' "$*" | tee -a "$g_log"
}

fn_gate() {
	# a_name a_cmd...
	a_name=$1
	shift
	fn_log "### GATE $a_name"
	if "$@" >>"$g_log" 2>&1; then
		fn_log "OK $a_name"
		return 0
	fi
	fn_log "FAIL $a_name"
	g_fail=1
	return 1
}

mkdir -p "$g_state" "$g_root/recovery"
touch "$g_log"
fn_log ""
fn_log "## gates $(fn_ts)"

cd "$g_root" || exit 2

# G5 first — refuse to continue if good binary is compromised
fn_log "### GATE G5 good-binary-sha256"
if test ! -f "$g_good" || test ! -f "$g_sum"; then
	fn_log "FAIL G5 missing good binary or checksum"
	exit 2
fi
if ! shasum -a 256 -c "$g_sum" >>"$g_log" 2>&1; then
	fn_log "FAIL G5 checksum mismatch — STOP. Do not continue recovery."
	exit 2
fi
fn_log "OK G5"

# Test / feature-check script hash drift (informational + soft fail if no log)
fn_log "### GATE G0 test-hash-integrity"
if test -f "$g_state/test-hashes-start"; then
	find "$g_root" -name '*_test.go' -print0 | sort -z | xargs -0 shasum -a 256 >"$g_state/test-hashes-now"
	if ! cmp -s "$g_state/test-hashes-start" "$g_state/test-hashes-now"; then
		fn_log "WARN test file hashes changed since loop start"
		if test ! -s "$g_state/test-change-log.md"; then
			fn_log "FAIL G0 tests changed without test-change-log.md"
			g_fail=1
		else
			fn_log "OK G0 change logged"
		fi
	else
		fn_log "OK G0 tests unchanged"
	fi
else
	fn_log "SKIP G0 no start hash yet"
fi

if test -f "$g_state/feature-check-hash-start"; then
	b_fc=$(shasum -a 256 "$g_root/scripts/recovery-feature-check.sh" | awk '{print $1}')
	b_start=$(cat "$g_state/feature-check-hash-start")
	if test "$b_fc" != "$b_start"; then
		fn_log "FAIL G0b feature-check script hash changed (anti-cheat)"
		g_fail=1
	else
		fn_log "OK G0b feature-check script unchanged"
	fi
fi

fn_gate G1-build go build -o "$g_cand" ./cmd/gitaware || true
fn_gate G2-test go test ./... || true
fn_gate G3-vet go vet ./... || true

fn_log "### GATE G4 gofmt"
b_fmt=$(gofmt -l . | grep -v '^recovery/' | grep '\.go$' || true)
if test -n "$b_fmt"; then
	fn_log "FAIL G4 unformatted:"
	fn_log "$b_fmt"
	g_fail=1
else
	fn_log "OK G4"
fi

if test -x "$g_cand"; then
	fn_gate G6-features "$g_root/scripts/recovery-feature-check.sh" "$g_cand" || true
else
	fn_log "FAIL G6 no candidate binary"
	g_fail=1
fi

if test "$g_fail" -ne 0; then
	fn_log "RESULT RED"
	exit 1
fi
fn_log "RESULT GREEN"
exit 0
