#!/usr/bin/env bash
# Mutation evaluation of the AtomiCos checker on cosmos/evm v0.7.4 (pre-registered: README.md).
#
#   ./eval/mutation/run.sh gen         generate mutants.tsv + patches/ from clean v0.7.4
#   ./eval/mutation/run.sh coverage    clean run of the 8 suites with -coverprofile (no harness)
#   ./eval/mutation/run.sh plan        map every mutant to the suites that reach it -> plan.tsv
#   ./eval/mutation/run.sh run [N]     clean control + mutants in plan order (resumable, N = max mutants)
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
TIMEOUT_SUITE=20m
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

# judge <trace> -> "violations rules a1 abl..." (one line, tab separated)
judge() {
  local trace="$1" v rules a1 out abl
  out="$("$CHECKER" "$trace" 2>&1 || true)"
  v="$( (grep -E '^  violation ' <<<"$out" || true) | awk '{s+=$3} END {print s+0}')"
  rules="$( (grep -E '^  violation ' <<<"$out" || true) | awk '{printf "%s%s", (NR>1?",":""), $2}')"
  a1="$( (grep -E '^  aux +A1 ' <<<"$out" || true) | awk '{s+=$3} END {print s+0}')"
  abl=""
  for p in "${ABLATIONS[@]}"; do
    local o
    o="$("$CHECKER" -drop "$p" "$trace" 2>&1 || true)"
    abl+="$( (grep -E '^  violation ' <<<"$o" || true) | awk '{s+=$3} END {print s+0}')"$'\t'
  done
  printf '%s\t%s\t%s\t%s' "$v" "${rules:--}" "$a1" "${abl%$'\t'}"
}

# run_suite <id> <suite> -> appends one row to results/runs.tsv
run_suite() {
  local id="$1" s="$2" out="$RES/runs/$1" trace st t0 verdict
  mkdir -p "$out"
  trace="$out/$s.trace.jsonl"
  t0=$(date +%s)
  set +e
  (cd "$WT/evmd" && ATOMICOS_TRACE="$trace" timeout "$TIMEOUT_SUITE" \
     go test -tags=test "./tests/integration/precompiles/$s/" -run "^$(testfn "$s")\$" -count=1 -v) > "$out/$s.test.log" 2>&1
  st=$?
  set -e
  local log; log="$(sed 's/\x1b\[[0-9;]*m//g' "$out/$s.test.log")"
  if [ $st -eq 124 ]; then verdict=TIMEOUT
  elif grep -q '\[build failed\]\|\[setup failed\]' <<<"$log"; then verdict=BUILD_FAIL
  elif [ $st -eq 0 ]; then verdict=PASS
  elif grep -qE 'Ran [0-9]+ of [0-9]+ Specs' <<<"$log"; then verdict=FAIL
  elif grep -q '^panic:' <<<"$log"; then verdict=PANIC
  else verdict=FAIL; fi
  local j="0	-	0	0	0	0	0"
  [ -s "$trace" ] && j="$(judge "$trace")"
  printf '%s\t%s\t%s\t%s\t%s\n' "$id" "$s" "$verdict" "$j" "$(( $(date +%s) - t0 ))" >> "$RES/runs.tsv"
  # keep the trace only when the checker reported something (evidence), compressed
  if [ -s "$trace" ] && [ "$(cut -f1 <<<"$j")" != "0" ]; then gzip -f "$trace"; else rm -f "$trace"; fi
  grep -aE -- '--- (FAIL|PASS)|Ran [0-9]+ of|panic:' "$out/$s.test.log" | head -5 > "$out/$s.test.tail" || true
  rm -f "$out/$s.test.log"
}

# Every mutant adds its own compiled packages to the Go build cache; keep the disk bounded.
CACHE_LIMIT_KB=$((40 * 1024 * 1024))
trim_cache() {
  local c kb
  c="$(go env GOCACHE)"
  kb="$(du -sk "$c" 2>/dev/null | cut -f1)"
  if [ "${kb:-0}" -gt "$CACHE_LIMIT_KB" ]; then
    log "build cache $((kb / 1024 / 1024)) GB > 40 GB: cleaning"
    go clean -cache
  fi
}

run_mutant() {
  local id="$1" suites="$2" out="$RES/runs/$1"
  trim_cache
  rm -rf "$out"; mkdir -p "$out"
  sed -i "/^$id\t/d" "$RES/runs.tsv" 2>/dev/null || true
  clean_tree
  if [ "$id" != CLEAN ]; then
    if ! git -C "$WT" apply "$HERE/patches/$id.patch" 2> "$out/apply.log"; then
      printf '%s\t-\tAPPLY_FAIL\t0\t-\t0\t0\t0\t0\t0\t0\n' "$id" >> "$RES/runs.tsv"; return
    fi
  fi
  if ! "$ROOT/tool/harness/inject.sh" v07 "$WT" > "$out/inject.log" 2>&1; then
    printf '%s\t-\tINJECT_FAIL\t0\t-\t0\t0\t0\t0\t0\t0\n' "$id" >> "$RES/runs.tsv"; return
  fi
  if ! (cd "$WT" && go build ./x/vm/... ./precompiles/...) > "$out/build.log" 2>&1; then
    printf '%s\t-\tBUILD_FAIL\t0\t-\t0\t0\t0\t0\t0\t0\n' "$id" >> "$RES/runs.tsv"; return
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
    MAX="${2:-100000}"
    build_checker
    mkdir -p "$RES/runs"; touch "$RES/runs.tsv"
    all="$(printf '%s,' "${SUITES[@]%%|*}")"
    if ! grep -q "^CLEAN	" "$RES/runs.tsv"; then log "clean control"; run_mutant CLEAN "${all%,}"; fi
    n=0
    tail -n +2 "$HERE/plan.tsv" | while IFS=$'\t' read -r order id suites; do
      [ "$suites" = "-" ] && continue
      case "$EXCLUDED" in *" $id "*) continue ;; esac
      grep -q "^$id	" "$RES/runs.tsv" && continue
      [ "$n" -ge "$MAX" ] && break
      n=$((n + 1))
      log "[$order] $id  suites: $suites"
      run_mutant "$id" "$suites"
    done
    clean_tree
    ;;
  report)
    python3 "$HERE/report.py" "$HERE"
    ;;
  *) sed -n '2,12p' "$0"; exit 2 ;;
esac
