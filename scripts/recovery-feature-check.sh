#!/bin/sh
# recovery-feature-check.sh — compare candidate CLI to good-binary contracts
# Usage: recovery-feature-check.sh [candidate-path]
set -eu

g_root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
g_cand=${1:-"$g_root/recovery/gitaware.candidate"}
g_good="$g_root/recovery/gitaware.good"
g_fail=0

fn_ok() { printf '  OK  %s\n' "$*"; }
fn_bad() {
	printf '  BAD %s\n' "$*"
	g_fail=1
}

fn_need_cmd() {
	a_bin=$1
	a_sub=$2
	a_pat=$3
	if "$a_bin" $a_sub -h 2>&1 | grep -q "$a_pat"; then
		fn_ok "help $a_sub ~ /$a_pat/"
	else
		fn_bad "help $a_sub missing /$a_pat/"
	fi
}

if test ! -x "$g_cand"; then
	echo "candidate not executable: $g_cand" >&2
	exit 1
fi
if test ! -x "$g_good"; then
	echo "good binary missing: $g_good" >&2
	exit 1
fi

echo "feature-check candidate=$g_cand"

# Must be a real go build artifact path under recovery (not the .good file)
if test "$g_cand" = "$g_good"; then
	fn_bad "candidate must not be the good binary path"
fi

# version
if "$g_cand" version >/dev/null 2>&1; then
	fn_ok version
else
	fn_bad version
fi

# doctor
b_doc=$("$g_cand" doctor 2>&1 || true)
echo "$b_doc" | grep -q 'config:' && fn_ok 'doctor config' || fn_bad 'doctor config'
echo "$b_doc" | grep -q 'roots:' && fn_ok 'doctor roots' || fn_bad 'doctor roots'
echo "$b_doc" | grep -q 'repos found:' && fn_ok 'doctor repos found' || fn_bad 'doctor repos found'
echo "$b_doc" | grep -q 'git: ok' && fn_ok 'doctor git' || fn_bad 'doctor git'

# help contracts
fn_need_cmd "$g_cand" status 'check-remote'
fn_need_cmd "$g_cand" status 'MUTATING'
fn_need_cmd "$g_cand" status 'directory to walk'
fn_need_cmd "$g_cand" arrive 'Does not list uncloned'
fn_need_cmd "$g_cand" arrive 'ls-remote'
fn_need_cmd "$g_cand" fetch 'MUTATING'
fn_need_cmd "$g_cand" fetch 'arrive --fetch'
fn_need_cmd "$g_cand" leave 'behind'

# fetch confirm abort
b_fetch=$(printf 'n\n' | "$g_cand" fetch 2>&1 || true)
echo "$b_fetch" | grep -q 'WARNING' && fn_ok 'fetch warning' || fn_bad 'fetch warning'
echo "$b_fetch" | grep -qi 'abort' && fn_ok 'fetch abort' || fn_bad 'fetch abort'

# JSON offline shape
b_json=$("$g_cand" --json 2>/dev/null || true)
if test -z "$b_json"; then
	fn_bad 'status --json empty'
else
	echo "$b_json" | grep -q '"orgs"' && fn_ok 'json orgs' || fn_bad 'json orgs'
	echo "$b_json" | grep -q '"summary"' && fn_ok 'json summary' || fn_bad 'json summary'
	# remote identity fields present somewhere
	echo "$b_json" | grep -q 'remote_owner\|"host"' && fn_ok 'json remote identity' || fn_bad 'json remote identity'
	echo "$b_json" | grep -q 'path_org\|path_name' && fn_ok 'json path hints' || fn_bad 'json path hints'
fi

# arrive: no missing dump (network; allow fail soft if offline gh broken)
b_arr=$("$g_cand" arrive --json 2>/dev/null || true)
if test -n "$b_arr"; then
	# missing key absent or empty array
	if echo "$b_arr" | grep -q '"missing": \[\]' || ! echo "$b_arr" | grep -q '"missing"'; then
		fn_ok 'arrive missing empty'
	else
		# count entries roughly
		b_n=$(echo "$b_arr" | grep -c '"name":' || true)
		# if missing section has many names beyond local repos, fail
		# stricter: any missing_clone signal or top-level missing with objects
		if echo "$b_arr" | grep -q '"missing": \[' && echo "$b_arr" | grep -A2 '"missing"' | grep -q '"org"'; then
			fn_bad 'arrive lists missing clones'
		else
			fn_ok 'arrive missing empty'
		fi
	fi
	echo "$b_arr" | grep -q 'online' && fn_ok 'arrive mode online' || fn_bad 'arrive mode online'
else
	fn_bad 'arrive --json empty/failed'
fi

# discover finds repos with user config
b_n=$("$g_cand" -a --json 2>/dev/null | grep -c '"is_repo": true' || true)
if test "${b_n:-0}" -gt 0; then
	fn_ok "discovered repos is_repo_true~$b_n"
else
	fn_bad 'discovered zero repos'
fi

# UI beauty: table chrome present (lipgloss borders / box drawing)
b_table=$("$g_cand" -a 2>/dev/null | head -n 40 || true)
if echo "$b_table" | grep -q 'Repository'; then
	fn_ok 'ui has Repository header'
else
	fn_bad 'ui missing Repository header'
fi
if echo "$b_table" | grep -qE '│|┌|├|└|┃|┏'; then
	fn_ok 'ui has table borders'
else
	fn_bad 'ui missing table borders (not as polished as good binary)'
fi
if echo "$b_table" | grep -qE '●|○'; then
	fn_ok 'ui has status dots'
else
	fn_bad 'ui missing ●/○ status dots'
fi
# good binary also has these chrome markers
b_good_table=$("$g_good" -a 2>/dev/null | head -n 20 || true)
if test -n "$b_good_table" && echo "$b_good_table" | grep -qE '│|┌'; then
	fn_ok 'good binary UI baseline readable'
fi

if test "$g_fail" -ne 0; then
	echo "feature-check RESULT FAIL"
	exit 1
fi
echo "feature-check RESULT PASS"
exit 0
