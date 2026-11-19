#!/usr/bin/env bash
# Mutation evaluation of the AtomiCos checker on cosmos/evm v0.7.4 (pre-registered: README.md).
#
#   ./eval/mutation/run.sh gen         generate mutants.tsv + patches/ from clean v0.7.4
#   ./eval/mutation/run.sh coverage    clean run of the 8 suites with -coverprofile (no harness)
#   ./eval/mutation/run.sh plan        map every mutant to the suites that reach it -> plan.tsv
#   ./eval/mutation/run.sh run [W]     clean control + mutants in plan order, W parallel workers (resumable)
#   ./eval/mutation/run.sh report      aggregate -> results/report.md
#
# Per mutant: clean tree -> git apply patch -> harness (inject.sh v07) -> build -> every suite that
# reaches the mutant, with ATOMICOS_TRACE -> test outcome + checker verdicts (full and ablations).
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
REF=v0.7.4
GO_TOOLCHAIN=go1.25.9
export GOTOOLCHAIN="$GO_TOOLCHAIN"
WT="$ROOT/.work/mut-base"
RES="$HERE/results"
TIMEOUT_SUITE=25m   # outer kill; go test itself gets -timeout 20m (its default 10m is too short under load)
HEAVY_SLOTS=2       # at most this many erc20 runs at once (each needs up to ~2 GB with the harness)
MEM_LIMIT_KB=$((3 * 1024 * 1024))  # a test binary above 3 GB RSS is a runaway (clean max: erc20 ~2.1 GB) -> MEMOUT
# name|test function (same suites as eval/fp-upstream)
SUITES=(
  "staking|TestStakingPrecompileIntegrationTestSuite"
  "distribution|TestDistributionPrecompileIntegrationTestSuite"
  "bank|TestBankPrecompileIntegrationTestSuite"
  "erc20|TestErc20IntegrationTestSuite"
  "werc20|TestWERC20PrecompileIntegrationTestSuite"
  "gov|TestGovPrecompileIntegrationTestSuite"
  "slashing|TestStakingPrecompileIntegrationTestSuite"
  "ics20|TestICS20PrecompileIntegrationTestSuite"
)
ABLATIONS=(O2 O4 O5 O6)
# GHSA-8pfh is a design / sanity-check case only and is excluded from step-11 evaluation
# (README.md principle, docs/answer-key.md): its fix-revert mutants are generated but not run.
EXCLUDED=" A02 A03 "

CASE=mutation VULN=$REF FIXED=$REF
source "$ROOT/cases/lib.sh"
export GOFLAGS="-p=4"

testfn() { local e; for e in "${SUITES[@]}"; do [ "${e%%|*}" = "$1" ] && { echo "${e#*|}"; return; }; done; }

clean_tree() {
  ensure_mirror
  COMMIT="$(git -C "$MIRROR" rev-parse "$REF^{commit}")"
  [ -d "$WT" ] || git -C "$MIRROR" worktree add --detach --quiet "$WT" "$COMMIT"
  git -C "$WT" checkout --detach --quiet --force "$COMMIT"
  git -C "$WT" clean -fdq
}

# AtomiCos commit the run uses (written next to the tree by the sync step; the WSL copy has no .git)
ATOMICOS_COMMIT="$(cat "$ROOT/ATOMICOS_COMMIT" 2>/dev/null || git -C "$ROOT" rev-parse --short HEAD 2>/dev/null || echo unknown)"
# mutants run first (README change log 2026-11-19 23시): the 12 the pre-fix checker caught, A04, M154
PRIORITY="A01 C028 M202 M246 M103 M012 C153 M095 M284 M212 C116 M213 A04 M154"

# judge <trace> <check.txt> -> 10 tab separated fields:
#   violations rules a1 ablO2 ablO4 ablO5 ablO6 checker_verdict state_errors inconclusive_reasons
# checker exit status: 0 no violation, 1 violation, 3 inconclusive (not a detection), 2 error
judge() {
  local trace="$1" txt="$2" v rules a1 out abl st cv se why
  set +e; out="$("$CHECKER" "$trace" 2>&1)"; st=$?; set -e
  printf '%s\n' "$out" > "$txt"
  case $st in 0) cv=none ;; 1) cv=violation ;; 3) cv=inconclusive ;; *) cv="error$st" ;; esac
  v="$( (grep -E '^  violation ' <<<"$out" || true) | awk '{s+=$3} END {print s+0}')"
  rules="$( (grep -E '^  violation ' <<<"$out" || true) | awk '{printf "%s%s", (NR>1?",":""), $2}')"
  a1="$( (grep -E '^  aux +A1 ' <<<"$out" || true) | awk '{s+=$3} END {print s+0}')"
  se="$( (grep -oE 'records with state.error [0-9]+' <<<"$out" || true) | awk '{print $NF+0}' | head -1)"
  # reasons without numbers, so the report can count them by kind
  why="$( (grep -E '^  inconclusive: ' <<<"$out" || true) | sed -E 's/^  inconclusive: //; s/\(first at seq [^)]*\)//; s/[0-9]+/N/g; s/[[:space:]]+/ /g; s/ ;/;/g' | paste -sd'|' -)"
  abl=""
  # ablations only matter for traces the full checker flagged (same result, much less time)
  if [ "$v" = 0 ]; then abl=$'0\t0\t0\t0'
  else
    for p in "${ABLATIONS[@]}"; do
      local o
      o="$("$CHECKER" -drop "$p" "$trace" 2>&1 || true)"
      abl+="$( (grep -E '^  violation ' <<<"$o" || true) | awk '{s+=$3} END {print s+0}')"$'\t'
    done
    abl="${abl%$'\t'}"
  fi
  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s' "$v" "${rules:--}" "$a1" "$abl" "$cv" "${se:-0}" "${why:--}"
}

# fail_row <id> <kind>: one row for a mutant that could not be run (same 15 columns as run_suite)
fail_row() {
  printf '%s\t-\t%s\t0\t-\t0\t0\t0\t0\t0\t0\t%s\t-\t0\t-\n' "$1" "$2" "$ATOMICOS_COMMIT" >> "$RES/runs/$1/rows.tsv"
}

# run_suite <id> <suite> -> appends one row to results/runs/<id>/rows.tsv
run_suite() {
  local id="$1" s="$2" out="$RES/runs/$1" trace st t0 verdict
  mkdir -p "$out"
  trace="$out/$s.trace.jsonl"
  local fd=""
  if [ "$s" = erc20 ]; then fd="$(acquire_heavy)"; fi
  t0=$(date +%s)
  rm -f "$out/$s.memout"
  watch_mem "$trace" "$out/$s.memout" & local wpid=$!
  set +e
  (cd "$WT/evmd" && ATOMICOS_TRACE="$trace" timeout "$TIMEOUT_SUITE" \
     go test -tags=test "./tests/integration/precompiles/$s/" -run "^$(testfn "$s")\$" -count=1 -timeout 20m -v) > "$out/$s.test.log" 2>&1
  st=$?
  kill "$wpid" 2>/dev/null; wait "$wpid" 2>/dev/null
  set -e
  [ -n "$fd" ] && rm -f "$fd"
  local log; log="$(sed 's/\x1b\[[0-9;]*m//g' "$out/$s.test.log")"
  if [ -f "$out/$s.memout" ]; then verdict=MEMOUT  # runaway memory, killed by watch_mem (counted like TIMEOUT)
  elif [ $st -eq 124 ] || grep -q 'test timed out after' <<<"$log"; then verdict=TIMEOUT
  elif [ $st -eq 137 ] || grep -q 'signal: killed' <<<"$log"; then verdict=RUN_ERROR  # OOM or external kill: not a test verdict
  elif grep -q '\[build failed\]\|\[setup failed\]' <<<"$log"; then verdict=BUILD_FAIL
  elif [ $st -eq 0 ]; then verdict=PASS
  elif grep -qE 'Ran [0-9]+ of [0-9]+ Specs' <<<"$log"; then verdict=FAIL
  elif grep -q '^panic:' <<<"$log"; then verdict=PANIC
  else verdict=FAIL; fi
  local j=$'0\t-\t0\t0\t0\t0\t0\tnotrace\t0\t-'
  [ -s "$trace" ] && j="$(judge "$trace" "$out/$s.check.txt")"
  # columns: id suite test_verdict violations rules a1 ablO2 ablO4 ablO5 ablO6 seconds
  #          atomicos_commit checker_verdict state_errors inconclusive_reasons
  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$id" "$s" "$verdict" "$(cut -f1-7 <<<"$j")" \
    "$(( $(date +%s) - t0 ))" "$ATOMICOS_COMMIT" "$(cut -f8-10 <<<"$j")" >> "$out/rows.tsv"
  # keep every trace (re-judging after a checker change must not need a re-run), compressed
  if [ -s "$trace" ]; then gzip -f "$trace"; else rm -f "$trace"; fi
  grep -aE -- '--- (FAIL|PASS)|Ran [0-9]+ of|panic:' "$out/$s.test.log" | head -5 > "$out/$s.test.tail" || true
  # keep the full log of every non-PASS run (evidence for why the tests failed)
  if [ "$verdict" = PASS ]; then rm -f "$out/$s.test.log"; else gzip -f "$out/$s.test.log"; fi
}

# watch_mem <trace> <marker>: kill the test binary of this run (found by its ATOMICOS_TRACE) once its
# RSS exceeds MEM_LIMIT_KB, so one runaway mutant cannot push the other workers into the OOM killer
watch_mem() {
  local p r
  while sleep 10; do
    for p in $(pgrep -f '\.test -test\.paniconexit0' || true); do
      tr '\0' '\n' < "/proc/$p/environ" 2>/dev/null | grep -qxF "ATOMICOS_TRACE=$1" || continue
      r="$(ps -o rss= -p "$p" 2>/dev/null | tr -d ' ')"
      if [ "${r:-0}" -gt "$MEM_LIMIT_KB" ]; then echo "$r" > "$2"; kill -9 "$p" 2>/dev/null || true; fi
    done
  done
}

# acquire_heavy -> prints the slot file it holds (mkdir-free atomic claim via noclobber)
acquire_heavy() {
  local k f
  while true; do
    for k in $(seq 1 "$HEAVY_SLOTS"); do
      f="$RES/heavy.$k.slot"
      if ( set -o noclobber; echo "$$" > "$f" ) 2>/dev/null; then echo "$f"; return; fi
    done
    sleep 5
  done
}

# Every mutant adds its own compiled packages to the Go build cache; keep the disk bounded.
CACHE_LIMIT_KB=$((40 * 1024 * 1024))
trim_cache() {
  local c kb
  c="$(go env GOCACHE)"
  kb="$(du -sk "$c" 2>/dev/null | cut -f1)"
  if [ "${kb:-0}" -gt "$CACHE_LIMIT_KB" ]; then
    # entries in use are touched at least hourly; drop those untouched for 2 hours
    # (packages of finished mutants) instead of wiping the cache under the other workers
    log "build cache $((kb / 1024 / 1024)) GB > 40 GB: removing entries unused for 2 hours"
    find "$c" -type f -mmin +120 -delete 2>/dev/null || true
  fi
}

run_mutant() {
  run_mutant_body "$@"
  touch "$RES/runs/$1/DONE"
}

run_mutant_body() {
  local id="$1" suites="$2" out="$RES/runs/$1"
  trim_cache
  rm -rf "$out"; mkdir -p "$out"
  clean_tree
  if [ "$id" != CLEAN ]; then
    if ! git -C "$WT" apply "$HERE/patches/$id.patch" 2> "$out/apply.log"; then
      fail_row "$id" APPLY_FAIL; return
    fi
  fi
  if ! "$ROOT/tool/harness/inject.sh" v07 "$WT" > "$out/inject.log" 2>&1; then
    fail_row "$id" INJECT_FAIL; return
  fi
  if ! (cd "$WT" && go build ./x/vm/... ./precompiles/...) > "$out/build.log" 2>&1; then
    fail_row "$id" BUILD_FAIL; return
  fi
  rm -f "$out/build.log"
  local s
  for s in ${suites//,/ }; do run_suite "$id" "$s"; done
}

case "${1:-}" in
  gen)
    clean_tree
    rm -rf "$HERE/patches"
    python3 "$HERE/gen_mutants.py" "$WT" "$HERE"
    ;;
  coverage)
    clean_tree
    mkdir -p "$RES/coverage"
    for e in "${SUITES[@]}"; do
      s="${e%%|*}"; log "coverage: $s"
      set +e
      (cd "$WT/evmd" && go test -tags=test "./tests/integration/precompiles/$s/" -run "^${e#*|}\$" -count=1 \
         -coverpkg=github.com/cosmos/evm/x/vm/statedb,github.com/cosmos/evm/x/vm/keeper,github.com/cosmos/evm/precompiles/... \
         -coverprofile="$RES/coverage/$s.out") > "$RES/coverage/$s.log" 2>&1
      echo "  exit $?: $(tail -1 "$RES/coverage/$s.log")"
      set -e
    done
    ;;
  plan)
    python3 "$HERE/plan.py" "$HERE"
    ;;
  run)
    # run [WORKERS]: workers take mutants from plan.tsv in order (atomic claim), each in its own worktree
    WORKERS="${2:-1}"
    build_checker
    mkdir -p "$RES/runs"; rm -rf "$RES/claims"; mkdir -p "$RES/claims"; rm -f "$RES"/heavy.*.slot
    all="$(printf '%s,' "${SUITES[@]%%|*}")"
    log "AtomiCos commit $ATOMICOS_COMMIT"
    if [ ! -f "$RES/runs/CLEAN/DONE" ]; then log "clean control"; run_mutant CLEAN "${all%,}"; fi
    # gate: no mutant runs unless the clean control is all PASS, 0 violations, 0 inconclusive, 0 state.error
    bad="$(awk -F'\t' '$3!="PASS" || $4!=0 || $13!="none" || $14!=0 {print $2": test "$3", violations "$4", checker "$13", state.error "$14}' "$RES/runs/CLEAN/rows.tsv")"
    if [ -n "$bad" ] || [ "$(wc -l < "$RES/runs/CLEAN/rows.tsv")" -ne "${#SUITES[@]}" ]; then
      log "CLEAN GATE FAILED: mutants not run"; printf '%s\n' "$bad"; exit 3
    fi
    log "clean gate passed"
    # run order: PRIORITY first, then plan.tsv order (recorded in results/queue.tsv)
    { head -1 "$HERE/plan.tsv"
      for p in $PRIORITY; do awk -F'\t' -v id="$p" '$2==id' "$HERE/plan.tsv"; done
      tail -n +2 "$HERE/plan.tsv" | awk -F'\t' -v pr=" $PRIORITY " 'index(pr, " "$2" ")==0'
    } > "$RES/queue.tsv"
    worker() {
      local k="$1"
      [ "$k" -eq 1 ] || WT="$ROOT/.work/mut-w$k"
      tail -n +2 "$RES/queue.tsv" | while IFS=$'\t' read -r order id suites; do
        [ "$suites" = "-" ] && continue
        case "$EXCLUDED" in *" $id "*) continue ;; esac
        [ -f "$RES/runs/$id/DONE" ] && continue
        mkdir "$RES/claims/$id" 2>/dev/null || continue
        log "w$k [$order] $id  suites: $suites"
        run_mutant "$id" "$suites"
      done
      clean_tree
      log "w$k finished"
    }
    for k in $(seq 1 "$WORKERS"); do worker "$k" & sleep 30; done
    wait
    ;;
  report)
    python3 "$HERE/report.py" "$HERE"
    ;;
  *) sed -n '2,12p' "$0"; exit 2 ;;
esac
