#!/usr/bin/env bash
# Shared helpers for cases/<case>/run.sh scripts.
#
# Each case script sets:
#   CASE       case id (e.g. ghsa-mjfq)
#   VULN       full commit hash of the vulnerable version (parent of the fix)
#   FIXED      full commit hash of the fix
#   GO_TOOLCHAIN  Go toolchain matching the commit's go.mod (e.g. go1.23.6)
# and then calls: prepare_worktrees, inject_*, run_case.

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

# run_case <subdir-in-repo> <go test args...>
# Runs the same command in both worktrees, writes results/{vuln,fixed}.log and results/summary.txt.
#
# Verdicts:
#   PASS   go test exit 0 and RAN_PATTERN found in the log
#   FAIL   go test exit != 0 and RAN_PATTERN found (the test actually ran and failed)
#   ERROR  RAN_PATTERN not found: build failure, toolchain download failure, nothing ran...
# RAN_PATTERN (extended regex) must be set by the case script, e.g. 'Ran [1-9][0-9]* of' for Ginkgo.
# An ERROR is never counted as the expected FAIL.
run_case() {
  : "${RAN_PATTERN:?case script must set RAN_PATTERN}"
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
    echo
  } > "$summary"

  for variant in vuln fixed; do
    local dir status
    dir="$(wt "$variant")"
    log "running $variant ..."
    set +e
    (cd "$dir/$subdir" && GOTOOLCHAIN="$GO_TOOLCHAIN" GOFLAGS="${GOFLAGS:-} -p=4" go test "$@") > "$results/$variant.log" 2>&1
    status=$?
    set -e
    local verdict
    # -e: patterns such as '--- FAIL: ...' start with '-' and would otherwise be read as options
    if ! grep -Eq -e "$RAN_PATTERN" "$results/$variant.log"; then
      verdict="ERROR (test did not run; see log)"
    elif [ $status -eq 0 ]; then
      verdict=PASS
    else
      verdict="FAIL (exit $status)"
    fi
    local expected
    expected="$([ "$variant" = vuln ] && echo FAIL || echo PASS)"
    local ok="as expected"
    case "$verdict" in "$expected"*) ;; *) ok="UNEXPECTED";; esac
    printf '%-6s %-34s expected %-5s -> %s\n' "$variant" "$verdict" "$expected" "$ok" | tee -a "$summary"
  done
  echo >> "$summary"
  echo "observations (lines tagged REPRO in the logs):" >> "$summary"
  for variant in vuln fixed; do
    grep -h "REPRO" "$results/$variant.log" | sed "s/^/  [$variant] /" >> "$summary" || true
  done
  log "summary: $summary"
}

# ------------------------------------------------------------------ step 7: judge mode
#
# judge_case <harness family> <subdir-in-repo> <go test args...>
# Same inputs as run_case, plus the AtomiCos harness (tool/harness/inject.sh) and the checker
# (tool/checker). Writes results/judge/: {vuln,fixed}.test.log, .trace.jsonl.gz, .check.txt,
# .check.json and summary.txt.
#   expected: vuln >= 1 violation, fixed 0 violations ("guarded" and "aux" are not violations)
TOOL="$ROOT/tool"
CHECKER="$WORK/bin/atomicos-check"

build_checker() {
  mkdir -p "$WORK/bin"
  (cd "$TOOL" && GOTOOLCHAIN=local go build -o "$CHECKER" ./checker/cmd/atomicos-check)
}

judge_case() {
  : "${RAN_PATTERN:?case script must set RAN_PATTERN}"
  local fam="$1" subdir="$2"; shift 2
  local out="$ROOT/cases/$CASE/results/judge"
  rm -rf "$out"; mkdir -p "$out"
  build_checker
  local summary="$out/summary.txt"
  {
    echo "case:    $CASE (step 7: same input as step 4 + harness $fam + checker)"
    echo "date:    $(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "vuln:    $VULN"
    echo "fixed:   $FIXED"
    echo "go:      $GO_TOOLCHAIN"
    echo "command: (cd $subdir && ATOMICOS_TRACE=<trace> go test $*)"
    echo
  } > "$summary"

  for variant in vuln fixed; do
    local dir trace tstatus test cstatus
    dir="$(wt "$variant")"
    trace="$out/$variant.trace.jsonl"
    "$TOOL/harness/inject.sh" "$fam" "$dir" > "$out/$variant.inject.log"
    log "judging $variant ..."
    set +e
    (cd "$dir/$subdir" && ATOMICOS_TRACE="$trace" GOTOOLCHAIN="$GO_TOOLCHAIN" GOFLAGS="${GOFLAGS:-} -p=4" go test "$@") > "$out/$variant.test.log" 2>&1
    tstatus=$?
    set -e
    if ! grep -Eq -e "$RAN_PATTERN" "$out/$variant.test.log"; then
      test="ERROR"
    elif [ $tstatus -eq 0 ]; then test="PASS"; else test="FAIL"; fi

    set +e
    "$CHECKER" "$trace" > "$out/$variant.check.txt" 2>&1
    cstatus=$?
    "$CHECKER" -json "$trace" > "$out/$variant.check.json" 2>/dev/null
    set -e
    local records counts violations
    records="$(wc -l < "$trace" 2>/dev/null || echo 0)"
    # grep finds nothing when there is no finding: keep pipefail/set -e from aborting here
    counts="$( (grep -E '^  (violation|guarded|aux) ' "$out/$variant.check.txt" || true) | awk '{printf "%s%s %s x%s", (NR>1?", ":""), $1, $2, $3}')"
    violations="$( (grep -E '^  violation ' "$out/$variant.check.txt" || true) | awk '{s+=$3} END {print s+0}')"
    gzip -f "$trace"

    local expected ok
    if [ "$variant" = vuln ]; then
      expected=">=1 violation"
      if [ "$test" != ERROR ] && [ "$cstatus" -ne 2 ] && [ "$violations" -ge 1 ]; then ok="as expected"; else ok="UNEXPECTED"; fi
    else
      expected="0 violations"
      if [ "$test" != ERROR ] && [ "$cstatus" -ne 2 ] && [ "$violations" -eq 0 ]; then ok="as expected"; else ok="UNEXPECTED"; fi
    fi
    printf '%-6s test %-5s records %-4s violations %-3s expected %-14s -> %s\n' \
      "$variant" "$test" "$records" "$violations" "$expected" "$ok" | tee -a "$summary"
    echo "       findings: ${counts:-none}" | tee -a "$summary"
  done
  echo >> "$summary"
  echo "details: results/judge/{vuln,fixed}.check.txt  (re-judge: atomicos-check results/judge/<variant>.trace.jsonl.gz)" >> "$summary"
  log "summary: $summary"
}
