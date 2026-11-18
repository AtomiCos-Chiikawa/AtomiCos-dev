#!/usr/bin/env bash
# Shared helpers for cases/<case>/run.sh scripts.
#
# Each case script sets:
#   CASE       case id (e.g. ghsa-mjfq)
#   VULN       full commit hash of the vulnerable version (parent of the fix)
#   FIXED      full commit hash of the fix
#   GO_TOOLCHAIN  Go toolchain matching the commit's go.mod (e.g. go1.23.6)
#   RAN_PATTERN   extended regex proving the test actually ran (see run_case)
#   EXPECT_FAIL   bash array of extended regexes: the failure signature of the vulnerable
#                 version (see expected_failure_seen)
# and then calls: prepare_worktrees, inject_*, run_case (or judge_case).
# run_case and judge_case return non-zero when any result is UNEXPECTED.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK="$ROOT/.work"
UPSTREAM="https://github.com/cosmos/evm.git"
MIRROR="$WORK/evm"

# Prefer a locally installed Go; GOTOOLCHAIN then fetches the exact version each commit needs.
if [ -x "$HOME/.local/go/bin/go" ]; then
  export PATH="$HOME/.local/go/bin:$PATH"
fi
command -v go >/dev/null || { echo "go not found (need Go >= 1.21 for GOTOOLCHAIN)"; exit 1; }

log() { printf '[%s] %s\n' "$(date +%H:%M:%S)" "$*"; }

ensure_mirror() {
  if [ ! -d "$MIRROR/.git" ]; then
    log "cloning cosmos/evm into $MIRROR"
    mkdir -p "$WORK"
    git clone --quiet "$UPSTREAM" "$MIRROR"
  fi
  # make sure both commits are present
  for c in "$VULN" "$FIXED"; do
    git -C "$MIRROR" cat-file -e "$c^{commit}" 2>/dev/null || git -C "$MIRROR" fetch --quiet origin
  done
}

# worktree path for a variant (vuln|fixed)
wt() { echo "$WORK/$CASE-$1"; }

prepare_worktrees() {
  ensure_mirror
  for variant in vuln fixed; do
    local dir commit
    dir="$(wt "$variant")"
    commit="$([ "$variant" = vuln ] && echo "$VULN" || echo "$FIXED")"
    if [ ! -d "$dir" ]; then
      git -C "$MIRROR" worktree add --detach --quiet "$dir" "$commit"
    fi
    # always start from a clean checkout of the exact commit
    git -C "$dir" checkout --detach --quiet --force "$commit"
    git -C "$dir" clean -fdq
    log "$variant worktree: $dir @ $(git -C "$dir" log -1 --format=%h)"
  done
}

# copy a file as it exists in the FIXED commit into both worktrees
inject_from_fixed() {
  local path="$1"
  for variant in vuln fixed; do
    local dir
    dir="$(wt "$variant")"
    mkdir -p "$(dirname "$dir/$path")"
    git -C "$MIRROR" show "$FIXED:$path" > "$dir/$path"
  done
}

# copy a file from this case directory into both worktrees
inject_local() {
  local src="$1" dst="$2"
  for variant in vuln fixed; do
    local dir
    dir="$(wt "$variant")"
    mkdir -p "$(dirname "$dir/$dst")"
    cp "$src" "$dir/$dst"
  done
}

# ------------------------------------------------------------------ verdict helpers

require_expect_fail() {
  if ! declare -p EXPECT_FAIL >/dev/null 2>&1 || [ "${#EXPECT_FAIL[@]}" -eq 0 ]; then
    echo "case script must set EXPECT_FAIL (failure signature of the vulnerable version)" >&2
    exit 1
  fi
}

# A Go test binary panic or a Ginkgo [PANICKED] spec is never part of an expected failure.
UNRELATED_PANIC='^panic: |\[PANICKED\]'

# expected_failure_seen <log>: true iff every EXPECT_FAIL pattern matches the log and the log
# has no unrelated panic. ANSI colour codes (Ginkgo) are removed before matching.
expected_failure_seen() {
  local tmp p rc=0
  tmp="$(mktemp "${TMPDIR:-/tmp}/atomicos-log.XXXXXX")"
  perl -pe 's/\e\[[0-9;]*m//g' "$1" > "$tmp"
  for p in "${EXPECT_FAIL[@]}"; do
    grep -Eq -e "$p" "$tmp" || { rc=1; break; }
  done
  if [ "$rc" -eq 0 ] && grep -Eq -e "$UNRELATED_PANIC" "$tmp"; then rc=1; fi
  rm -f "$tmp"
  return "$rc"
}

# test_verdict <variant> <go test exit status> <log>: prints
#   PASS        exit 0 and RAN_PATTERN found in the log
#   FAIL        exit != 0 and RAN_PATTERN found; for the vulnerable version the log must also
#               show the expected failure (expected_failure_seen)
#   FAIL-OTHER  the vulnerable version failed, but not with the expected failure (setup error,
#               another assertion, unrelated panic...): never counted as the expected FAIL
#   ERROR       RAN_PATTERN not found: build failure, toolchain download failure, nothing ran...
# RAN_PATTERN (extended regex) must be set by the case script, e.g. 'Ran [1-9][0-9]* of' for Ginkgo.
test_verdict() {
  local variant="$1" status="$2" logf="$3"
  # -e: patterns such as '--- FAIL: ...' start with '-' and would otherwise be read as options
  if ! grep -Eq -e "$RAN_PATTERN" "$logf"; then
    echo ERROR
  elif [ "$status" -eq 0 ]; then
    echo PASS
  elif [ "$variant" != vuln ] || expected_failure_seen "$logf"; then
    echo FAIL
  else
    echo FAIL-OTHER
  fi
}

# count_lines <file>: line count without padding (BSD wc pads), 0 if the file does not exist
count_lines() {
  if [ -f "$1" ]; then awk 'END {print NR}' "$1"; else echo 0; fi
}

# run_case <subdir-in-repo> <go test args...>
# Runs the same command in both worktrees, writes results/{vuln,fixed}.log and results/summary.txt.
# Verdicts: see test_verdict. Expected: vuln FAIL (with its EXPECT_FAIL signature), fixed PASS.
# An ERROR or FAIL-OTHER is never counted as the expected FAIL. Returns 1 if a line is UNEXPECTED.
run_case() {
  : "${RAN_PATTERN:?case script must set RAN_PATTERN}"
  require_expect_fail
  local subdir="$1"; shift
  local results="$ROOT/cases/$CASE/results"
  mkdir -p "$results"
  local summary="$results/summary.txt"
  {
    echo "case:    $CASE"
    echo "date:    $(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "vuln:    $VULN"
    echo "fixed:   $FIXED"
    echo "go:      $GO_TOOLCHAIN (CGO_ENABLED=${CGO_ENABLED:-default})"
    echo "command: (cd $subdir && go test $*)"
    echo "expect:  vuln FAIL with its known failure signature (EXPECT_FAIL in run.sh), fixed PASS"
    echo
  } > "$summary"

  local unexpected=0 variant
  for variant in vuln fixed; do
    local dir status v verdict expected ok
    dir="$(wt "$variant")"
    log "running $variant ..."
    set +e
    (cd "$dir/$subdir" && GOTOOLCHAIN="$GO_TOOLCHAIN" GOFLAGS="${GOFLAGS:-} -p=4" go test "$@") > "$results/$variant.log" 2>&1
    status=$?
    set -e
    v="$(test_verdict "$variant" "$status" "$results/$variant.log")"
    case "$v" in
      PASS)       verdict=PASS ;;
      FAIL)       verdict="FAIL (exit $status)" ;;
      FAIL-OTHER) verdict="FAIL-OTHER (exit $status)" ;;
      *)          verdict="ERROR (test did not run; see log)" ;;
    esac
    expected="$([ "$variant" = vuln ] && echo FAIL || echo PASS)"
    ok="as expected"
    if [ "$v" != "$expected" ]; then ok="UNEXPECTED"; unexpected=1; fi
    printf '%-6s %-34s expected %-5s -> %s\n' "$variant" "$verdict" "$expected" "$ok" | tee -a "$summary"
    if [ "$v" = FAIL-OTHER ]; then
      echo "       failed, but the log does not show the expected failure (EXPECT_FAIL) or has an unrelated panic" | tee -a "$summary"
    fi
  done
  echo >> "$summary"
  echo "observations (lines tagged REPRO in the logs):" >> "$summary"
  for variant in vuln fixed; do
    grep -h "REPRO" "$results/$variant.log" | sed "s/^/  [$variant] /" >> "$summary" || true
  done
  log "summary: $summary"
  if [ "$unexpected" -ne 0 ]; then
    log "UNEXPECTED result(s), see $summary"
    return 1
  fi
}

# ------------------------------------------------------------------ step 7: judge mode
#
# judge_case <harness family> <subdir-in-repo> <go test args...>
# Same inputs as run_case, plus the AtomiCos harness (tool/harness/inject.sh) and the checker
# (tool/checker). Writes results/judge/: {vuln,fixed}.test.log, .trace.jsonl.gz, .check.txt,
# .check.json and summary.txt.
#   expected, vuln:  test FAIL (with the EXPECT_FAIL signature), checker exit 1, >= 1 violation
#   expected, fixed: test PASS, checker exit 0, 0 violations ("guarded" and "aux" are not violations)
#   both: a non-empty trace that has every mandatory observation point of the family
# Any other checker exit code (2 = error, 3 = inconclusive, ...) is UNEXPECTED.
# Results are built in results/.judge.tmp and moved to results/judge only when the run completes,
# so an aborted run leaves the previous results in place. Returns 1 if a line is UNEXPECTED.
TOOL="$ROOT/tool"
CHECKER="$WORK/bin/atomicos-check"

build_checker() {
  mkdir -p "$WORK/bin"
  (cd "$TOOL" && GOTOOLCHAIN=local go build -o "$CHECKER" ./checker/cmd/atomicos-check)
}

# Mandatory observation points per harness family (docs/rules.md §1). v0.1.x has no
# runNativeAction, so its family has no O2.
required_points() {
  case "$1" in
    v01) echo "O4 O5 O6" ;;
    *)   echo "O2 O4 O5 O6" ;;
  esac
}

# missing_points <trace> <point...>: prints the points with no record in the trace
missing_points() {
  local trace="$1" p miss=""
  shift
  for p in "$@"; do
    grep -qF "\"point\":\"$p." "$trace" 2>/dev/null || miss="$miss $p"
  done
  echo "${miss# }"
}

# atomicos_rev: commit of this AtomiCos checkout and whether its code differs from that commit
# (generated results/ directories are not counted)
atomicos_rev() {
  local rev n h
  rev="$(git -C "$ROOT" rev-parse HEAD 2>/dev/null || true)"
  if [ -z "$rev" ]; then echo "unknown (not a git checkout)"; return; fi
  n="$( (git -C "$ROOT" status --porcelain --untracked-files=all -- . \
          ':(exclude)cases/*/results/*' ':(exclude)eval/*/results/*' 2>/dev/null || true) | awk 'END {print NR}')"
  if [ "$n" -eq 0 ]; then
    echo "$rev (tree clean outside results/)"
  else
    h="$( (git -C "$ROOT" diff HEAD -- . ':(exclude)cases/*/results/*' ':(exclude)eval/*/results/*' 2>/dev/null || true) | git hash-object --stdin)"
    echo "$rev (tree DIRTY: $n changed or untracked paths outside results/; hash of git diff HEAD: $h)"
  fi
}

judge_case() {
  : "${RAN_PATTERN:?case script must set RAN_PATTERN}"
  require_expect_fail
  local fam="$1" subdir="$2"; shift 2
  local out="$ROOT/cases/$CASE/results/judge"
  local tmp="$ROOT/cases/$CASE/results/.judge.tmp"
  rm -rf "$tmp"; mkdir -p "$tmp"
  # an aborted run (checker build, inject or harness failure, interrupt) only loses the temp dir
  trap "rm -rf '$tmp'" EXIT
  build_checker
  local points
  points="$(required_points "$fam")"
  local summary="$tmp/summary.txt"
  {
    echo "case:    $CASE (step 7: same input as step 4 + harness $fam + checker)"
    echo "date:    $(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "atomicos: $(atomicos_rev)"
    echo "vuln:    $VULN"
    echo "fixed:   $FIXED"
    echo "go:      $GO_TOOLCHAIN"
    echo "command: (cd $subdir && ATOMICOS_TRACE=<trace> go test $*)"
    echo "expect:  vuln: test FAIL (EXPECT_FAIL signature), checker exit 1, >=1 violation"
    echo "         fixed: test PASS, checker exit 0, 0 violations"
    echo "         both: records > 0 and observation points $points present"
    echo
  } > "$summary"

  local unexpected=0 variant
  for variant in vuln fixed; do
    local dir trace tstatus test cstatus records counts violations missing expected ok why
    dir="$(wt "$variant")"
    trace="$tmp/$variant.trace.jsonl"
    rm -f "$trace"   # the observer appends (O_APPEND): never mix with an old trace
    "$TOOL/harness/inject.sh" "$fam" "$dir" > "$tmp/$variant.inject.log"
    log "judging $variant ..."
    set +e
    (cd "$dir/$subdir" && ATOMICOS_TRACE="$trace" GOTOOLCHAIN="$GO_TOOLCHAIN" GOFLAGS="${GOFLAGS:-} -p=4" go test "$@") > "$tmp/$variant.test.log" 2>&1
    tstatus=$?
    set -e
    test="$(test_verdict "$variant" "$tstatus" "$tmp/$variant.test.log")"

    set +e
    "$CHECKER" "$trace" > "$tmp/$variant.check.txt" 2>&1
    cstatus=$?
    "$CHECKER" -json "$trace" > "$tmp/$variant.check.json" 2>/dev/null
    set -e
    records="$(count_lines "$trace")"
    # shellcheck disable=SC2086  # $points is a list of words
    missing="$(missing_points "$trace" $points)"
    # grep finds nothing when there is no finding: keep pipefail/set -e from aborting here
    counts="$( (grep -E '^  (violation|guarded|aux) ' "$tmp/$variant.check.txt" || true) | awk '{printf "%s%s %s x%s", (NR>1?", ":""), $1, $2, $3}')"
    violations="$( (grep -E '^  violation ' "$tmp/$variant.check.txt" || true) | awk '{s+=$3} END {print s+0}')"
    if [ -f "$trace" ]; then gzip -f "$trace"; fi

    why=""
    if [ "$variant" = vuln ]; then
      expected=">=1 violation"
      [ "$test" = FAIL ] || why="$why; test $test, expected FAIL"
      [ "$cstatus" -eq 1 ] || why="$why; checker exit $cstatus, expected 1"
      [ "$violations" -ge 1 ] || why="$why; no violation"
    else
      expected="0 violations"
      [ "$test" = PASS ] || why="$why; test $test, expected PASS"
      [ "$cstatus" -eq 0 ] || why="$why; checker exit $cstatus, expected 0"
      [ "$violations" -eq 0 ] || why="$why; $violations violation(s)"
    fi
    [ "$records" -gt 0 ] || why="$why; empty trace"
    [ -z "$missing" ] || why="$why; observation points missing: $missing"
    if [ -z "$why" ]; then
      ok="as expected"
    else
      ok="UNEXPECTED (${why#; })"
      unexpected=1
    fi
    printf '%-6s test %-5s records %-4s violations %-3s checker exit %s  expected %-14s -> %s\n' \
      "$variant" "$test" "$records" "$violations" "$cstatus" "$expected" "$ok" | tee -a "$summary"
    echo "       findings: ${counts:-none}" | tee -a "$summary"
  done
  echo >> "$summary"
  echo "details: results/judge/{vuln,fixed}.check.txt  (re-judge: atomicos-check results/judge/<variant>.trace.jsonl.gz)" >> "$summary"

  # the run completed: replace results/judge with it
  rm -rf "$out.old"
  if [ -d "$out" ]; then mv "$out" "$out.old"; fi
  mv "$tmp" "$out"
  rm -rf "$out.old"
  trap - EXIT
  log "summary: $out/summary.txt"
  if [ "$unexpected" -ne 0 ]; then
    log "UNEXPECTED result(s), see $out/summary.txt"
    return 1
  fi
}
