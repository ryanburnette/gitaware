#!/bin/sh
# recovery-loop.sh — pi writer/reviewer loop to restore gitaware
# Writer: synthetic GLM-5.2 | Reviewer: xai grok-4.5
#
# Each run gets a unique RECOVERY_ID and works on branch recovery/<id>.
# Requires main baseline already committed (and preferably pushed).
set -eu

g_root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
g_state_root="$g_root/todo/state"
g_good="$g_root/recovery/gitaware.good"
g_sum="$g_root/recovery/gitaware.good.sha256"
g_max=${RECOVERY_MAX_ITER:-15}
g_writer_model=${RECOVERY_WRITER_MODEL:-synthetic/hf:zai-org/GLM-5.2}
g_reviewer_model=${RECOVERY_REVIEWER_MODEL:-xai/grok-4.5}
g_id=${RECOVERY_ID:-}
g_skip_branch=${RECOVERY_SKIP_BRANCH:-0}

fn_ts() { date -u '+%Y-%m-%dT%H:%M:%SZ'; }

fn_die() {
	printf 'error: %s\n' "$*" >&2
	exit 2
}

fn_verify_good() {
	test -f "$g_good" || fn_die "missing $g_good"
	test -f "$g_sum" || fn_die "missing $g_sum"
	shasum -a 256 -c "$g_sum" >/dev/null 2>&1 || fn_die "good binary checksum failed"
	if test -f "$HOME/.local/share/gitaware-recovery/gitaware.good.sha256"; then
		shasum -a 256 -c "$HOME/.local/share/gitaware-recovery/gitaware.good.sha256" >/dev/null 2>&1 \
			|| printf 'warn: off-repo backup checksum mismatch\n' >&2
	fi
}

fn_require_main_baseline() {
	cd "$g_root" || fn_die "cd $g_root"
	git rev-parse --verify main >/dev/null 2>&1 || fn_die "no main branch — commit baseline to main first"
	b_head=$(git rev-parse main)
	test -n "$b_head" || fn_die "main has no commits"
	# Detached or dirty unrelated work: require clean tree before branch switch
	if test -n "$(git status --porcelain)"; then
		fn_die "worktree not clean — commit or stash before starting recovery loop"
	fi
}

fn_run_pi() {
	a_model=$1
	a_prompt=$2
	a_out=$3
	shift 3
	pi -p \
		--model "$a_model" \
		--session-dir "$g_session_dir" \
		--no-session \
		--approve \
		"$@" \
		@"$a_prompt" \
		"Repository: $g_root. Recovery ID: $g_id. Branch: $g_branch. State: $g_state. Meet or beat good-binary UI beauty." \
		>"$a_out" 2>"$a_out.err" || {
		printf 'pi failed model=%s log=%s\n' "$a_model" "$a_out.err" >&2
		tail -n 40 "$a_out.err" >&2 || true
		return 1
	}
	return 0
}

fn_verify_good
fn_require_main_baseline

# Unique run id + branch
if test -z "$g_id"; then
	g_id=$(date -u '+%Y%m%dT%H%M%SZ')-$$
fi
g_branch="recovery/${g_id}"
g_state="$g_state_root/$g_id"
g_session_dir="$g_state/pi-sessions"
g_meta="$g_state/meta.env"

mkdir -p "$g_state" "$g_session_dir" "$g_root/recovery"

if test "$g_skip_branch" != 1; then
	# Always start recovery work from main tip
	git checkout main >/dev/null 2>&1 || fn_die "checkout main failed"
	git branch "backup/pre-recovery-${g_id}" main 2>/dev/null || true
	if git show-ref --verify --quiet "refs/heads/${g_branch}"; then
		fn_die "branch already exists: $g_branch — set a new RECOVERY_ID"
	fi
	git checkout -b "$g_branch" main || fn_die "create branch $g_branch failed"
fi

# Record meta
{
	printf 'RECOVERY_ID=%s\n' "$g_id"
	printf 'RECOVERY_BRANCH=%s\n' "$g_branch"
	printf 'STARTED_AT=%s\n' "$(fn_ts)"
	printf 'BASE_MAIN=%s\n' "$(git rev-parse main)"
	printf 'WRITER_MODEL=%s\n' "$g_writer_model"
	printf 'REVIEWER_MODEL=%s\n' "$g_reviewer_model"
} >"$g_meta"

# Point shared “current” pointer for check scripts
printf '%s\n' "$g_id" >"$g_state_root/CURRENT_ID"
printf '%s\n' "$g_state" >"$g_state_root/CURRENT_STATE_DIR"

# Freeze hashes for this run
find "$g_root" -name '*_test.go' -print0 | sort -z | xargs -0 shasum -a 256 >"$g_state/test-hashes-start"
shasum -a 256 "$g_root/scripts/recovery-feature-check.sh" | awk '{print $1}' >"$g_state/feature-check-hash-start"
printf '# Test change log\n\n' >"$g_state/test-change-log.md"
: >"$g_state/gate-log.md"
printf '%s\n' "0" >"$g_state/iteration"

# Export for child scripts
export RECOVERY_STATE_DIR="$g_state"
export RECOVERY_ID="$g_id"

printf 'recovery-loop start %s\n' "$(fn_ts)"
printf '  id       %s\n' "$g_id"
printf '  branch   %s\n' "$g_branch"
printf '  state    %s\n' "$g_state"
printf '  root     %s\n' "$g_root"
printf '  max      %s\n' "$g_max"
printf '  writer   %s\n' "$g_writer_model"
printf '  reviewer %s\n' "$g_reviewer_model"
printf '  good     %s\n' "$g_good"
printf '  base     %s\n' "$(git rev-parse --short main)"

b_iter=0
while test "$b_iter" -lt "$g_max"; do
	b_iter=$((b_iter + 1))
	printf '%s\n' "$b_iter" >"$g_state/iteration"
	printf '\n======== ITERATION %s / %s id=%s %s ========\n' "$b_iter" "$g_max" "$g_id" "$(fn_ts)"

	fn_verify_good

	printf '--- pre-writer gates ---\n'
	"$g_root/scripts/recovery-check.sh" || true

	printf '--- writer (%s) ---\n' "$g_writer_model"
	b_wout="$g_state/writer-$b_iter.md"
	fn_run_pi "$g_writer_model" \
		"$g_root/scripts/prompts/recovery-writer.md" \
		"$b_wout" || printf 'writer pi error; continuing to gates\n' >&2

	gofmt -w "$g_root/internal/app" "$g_root/cmd" 2>/dev/null || true

	# Commit iteration snapshot on recovery branch (safe history)
	if test -n "$(git status --porcelain)"; then
		git add \
			cmd/gitaware \
			internal \
			scripts \
			todo/00-DAMAGE.md todo/01-FEATURES.md todo/02-UNIT-TESTS.md \
			todo/03-APP-API.md todo/04-RED-GREEN.md todo/05-ANTI-CHEAT.md \
			todo/README.md todo/baselines \
			AGENTS.md README.md go.mod go.sum .gitignore \
			2>/dev/null || true
		# never add candidate binary or state runtime noise except meta pointer files we want
		mkdir -p "$g_root/tmp"
		printf 'recovery(%s): writer iteration %s\n' "$g_id" "$b_iter" >"$g_root/tmp/recovery-commit-msg.txt"
		git commit -F "$g_root/tmp/recovery-commit-msg.txt" || true
	fi

	printf '--- post-writer gates ---\n'
	b_gates_ok=0
	if "$g_root/scripts/recovery-check.sh"; then
		b_gates_ok=1
	fi

	printf '--- reviewer (%s) ---\n' "$g_reviewer_model"
	b_rout="$g_state/review-$b_iter.md"
	fn_run_pi "$g_reviewer_model" \
		"$g_root/scripts/prompts/recovery-reviewer.md" \
		"$b_rout" \
		--append-system-prompt "UI must match or beat recovery/gitaware.good. VERDICT PASS only if gates green, no cheat, features+UI ok." \
		|| true

	b_verdict=FAIL
	if test -f "$b_rout" && grep -q '^VERDICT: PASS' "$b_rout"; then
		b_verdict=PASS
	fi
	printf 'reviewer verdict: %s\n' "$b_verdict"

	# Push recovery branch if remote exists (feature branch — allowed)
	if git remote get-url origin >/dev/null 2>&1; then
		git push -u origin "$g_branch" || printf 'warn: push %s failed\n' "$g_branch" >&2
	fi

	if test "$b_gates_ok" -eq 1 && test "$b_verdict" = PASS; then
		fn_verify_good
		printf 'SUCCESS=1\nFINISHED_AT=%s\n' "$(fn_ts)" >>"$g_meta"
		printf '\nSUCCESS id=%s iteration=%s branch=%s\n' "$g_id" "$b_iter" "$g_branch"
		printf 'candidate: %s\n' "$g_root/recovery/gitaware.candidate"
		printf 'Inspect UI:\n'
		printf '  recovery/gitaware.good -a 2>/dev/null | head\n'
		printf '  recovery/gitaware.candidate -a 2>/dev/null | head\n'
		printf 'Optional install after you approve:\n'
		printf '  cp recovery/gitaware.candidate "$(go env GOPATH)/bin/gitaware"\n'
		exit 0
	fi

	printf 'iteration %s incomplete (gates_ok=%s verdict=%s)\n' "$b_iter" "$b_gates_ok" "$b_verdict"
done

printf 'SUCCESS=0\nFINISHED_AT=%s\n' "$(fn_ts)" >>"$g_meta"
printf '\nFAILED: max iterations %s id=%s\n' "$g_max" "$g_id" >&2
exit 1
