#!/usr/bin/env bash
# Pre-measurement of false positives (step 7 extra, ahead of step 11).
#
# The rules were designed while looking at the three cases, so "0 violations on the fixed
# versions of the same three inputs" is weak evidence. Here the harness and the checker run
# on upstream test suites that were NOT used for the design, on the latest patched release of
# the v0.7.x line. Every finding here is either a false positive or a real problem, and has
# to be explained.
#
#   ./eval/fp-upstream/run.sh [A|B]       -> eval/fp-upstream/results/   (default: both sets)
#   FP_VERSION=v0.5.1 ./eval/fp-upstream/run.sh [A|B]
#                                         -> eval/fp-upstream/results/v0.5.1/
#   FP_VERSION selects a row of the version table below (default v0.7.4).
#
# Exit status: 0 = every suite ran and passed and the checker found no violation;
#              1 = runs fine, but the checker reported violations (to be explained);
#              2 = a run problem: a suite's tests failed or did not run, an empty trace, or a
#                  checker exit code other than 0 / 1.
#
# Set A (staking, distribution) was used to correct R3 (auto-withdrawn rewards), so after that
# fix it is no longer independent. Set B was never looked at while designing or fixing rules.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# Every version is pinned by commit hash: a tag can be moved; the commit cannot.
#   v0.7.4: latest patched release on release/v0.7.x at the time of writing (default).
#   v0.6.5: latest patched release on release/v0.6.x. It has the locked-balance snapshot
#           write-back (SetBalanceWithLocked) of v0.7.x, so it uses harness family v07.
#   v0.5.1: latest patched release on release/v0.5.x (harness family v05).
#   v0.2.0: the v0.1.x line has a single tag, v0.1.0, which predates the GHSA-mjfq fix and the
#           example_chain -> evmd rename (inject.sh needs evmd/). v0.2.0 is the GHSA-mjfq fix
#           commit itself (the base of harness family v01). Its precompile integration suites
#           live in precompiles/<module>/ (root module); slashing and ics20 have none.
FP_VERSION="${FP_VERSION:-v0.7.4}"
# set|package dir|test function
case "$FP_VERSION" in
  v0.7.4|v0.6.5|v0.5.1)
    SUITES=(
      "A|staking|TestStakingPrecompileIntegrationTestSuite"
      "A|distribution|TestDistributionPrecompileIntegrationTestSuite"
      "B|bank|TestBankPrecompileIntegrationTestSuite"
      "B|erc20|TestErc20IntegrationTestSuite"
      "B|werc20|TestWERC20PrecompileIntegrationTestSuite"
      "B|gov|TestGovPrecompileIntegrationTestSuite"
      "B|slashing|TestStakingPrecompileIntegrationTestSuite"
      "B|ics20|TestICS20PrecompileIntegrationTestSuite"
    )
    TEST_DIR=evmd PKG_PREFIX=./tests/integration/precompiles ;;
  v0.2.0)
    SUITES=(
      "A|staking|TestPrecompileIntegrationTestSuite"
      "A|distribution|TestPrecompileIntegrationTestSuite"
      "B|bank|TestIntegrationSuite"
      "B|erc20|TestIntegrationSuite"
      "B|werc20|TestPrecompileIntegrationTestSuite"
      "B|gov|TestKeeperIntegrationTestSuite"
    )
    TEST_DIR=. PKG_PREFIX=./precompiles ;;
esac
case "$FP_VERSION" in
  v0.7.4) COMMIT=ba66d3106a52f84b1c1ede8d9683c2df98d742ef FAM=v07 GO_TOOLCHAIN=go1.25.9 ;;
  v0.6.5) COMMIT=e04dc041978ba3ea5289a2c8398e9939ba8e957e FAM=v07 GO_TOOLCHAIN=go1.23.8 ;;
  v0.5.1) COMMIT=e2f6e47c70b77bfb375ae27b7aa24082aabe6884 FAM=v05 GO_TOOLCHAIN=go1.23.8 ;;
  v0.2.0) COMMIT=0fff8c144b24effbcb3addd666150ba5989d631c FAM=v01 GO_TOOLCHAIN=go1.23.6 ;;
  *) echo "unknown FP_VERSION $FP_VERSION (v0.7.4 | v0.6.5 | v0.5.1 | v0.2.0)" >&2; exit 2 ;;
esac
REF=$FP_VERSION
WT="$ROOT/.work/fp-$(echo "$FP_VERSION" | tr -d .)"
# v0.7.4 keeps its results where they were first written
if [ "$FP_VERSION" = v0.7.4 ]; then
  OUT="$ROOT/eval/fp-upstream/results"
else
  OUT="$ROOT/eval/fp-upstream/results/$FP_VERSION"
fi
ONLY="${1:-}"

# reuse helpers (clone, checker build); CASE/VULN/FIXED are only needed by the mirror check
CASE=fp-upstream VULN=$COMMIT FIXED=$COMMIT
source "$ROOT/cases/lib.sh"
ensure_mirror
# the tag is informational only: say so if it no longer names the pinned commit
tagged="$(git -C "$MIRROR" rev-parse -q --verify "refs/tags/$REF^{commit}" 2>/dev/null || true)"
if [ -n "$tagged" ] && [ "$tagged" != "$COMMIT" ]; then
  log "warning: tag $REF now points to $tagged, not to the pinned $COMMIT; using the pinned commit"
fi
[ -d "$WT" ] || git -C "$MIRROR" worktree add --detach --quiet "$WT" "$COMMIT"
git -C "$WT" checkout --detach --quiet --force "$COMMIT"
git -C "$WT" clean -fdq
"$ROOT/tool/harness/inject.sh" "$FAM" "$WT"
build_checker

mkdir -p "$OUT"
SUMMARY="$OUT/summary${ONLY:+-$ONLY}.txt"
{
  echo "false-positive pre-measurement: upstream precompile integration suites"
  echo "date:   $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "tree:   cosmos/evm $REF ($COMMIT, pinned by hash) + harness $FAM"
  echo "atomicos: $(atomicos_rev)"
  echo
} > "$SUMMARY"
problems=0 violated=0
for entry in "${SUITES[@]}"; do
  IFS='|' read -r set s testfn <<< "$entry"
  [ -z "$ONLY" ] || [ "$ONLY" = "$set" ] || continue
  trace="$OUT/$s.trace.jsonl"
  # the observer opens the trace with O_APPEND: never append to a stale trace
  rm -f "$trace" "$trace.gz"
  log "suite $s ..."
  set +e
  (cd "$WT/$TEST_DIR" && ATOMICOS_TRACE="$trace" GOTOOLCHAIN="$GO_TOOLCHAIN" GOFLAGS="-p=4" \
     go test -tags=test "$PKG_PREFIX/$s/" -run "^${testfn}\$" -count=1 -timeout 120m -v) \
     > "$OUT/$s.test.log" 2>&1
  tstatus=$?
  "$CHECKER" "$trace" > "$OUT/$s.check.txt" 2>&1
  cstatus=$?
  "$CHECKER" -json "$trace" > "$OUT/$s.check.json" 2>/dev/null
  set -e
  ran="$( (grep -aoE 'Ran [0-9]+ of [0-9]+ Specs' "$OUT/$s.test.log" || true) | tail -1)"
  # perl, not GNU sed's \x1b: strip ANSI colours portably (GNU and BSD userlands)
  res="$( (perl -pe 's/\e\[[0-9;]*m//g' "$OUT/$s.test.log" | grep -aoE '[0-9]+ Passed \| [0-9]+ Failed' || true) | tail -1)"
  if [ -z "$ran" ]; then test=ERROR; elif [ "$tstatus" -eq 0 ]; then test=PASS; else test=FAIL; fi
  [ "$test" = PASS ] || problems=1
  case "$cstatus" in
    0) checker="0 (no violation)" ;;
    1) checker="1 (violations)"; violated=1 ;;
    *) checker="$cstatus (checker ERROR)"; problems=1 ;;
  esac
  records="$(count_lines "$trace")"
  [ "$records" -gt 0 ] || problems=1
  # counts without BSD wc padding; 0 when the trace is missing
  dbs="$( (grep -o '"db":"db[0-9]*"' "$trace" 2>/dev/null || true) | sort -u | awk 'END {print NR}')"
  pcs="$(grep -c '"point":"O2.enter"' "$trace" 2>/dev/null || true)"
  reverts="$(grep -c '"point":"O5.revert"' "$trace" 2>/dev/null || true)"
  findings="$( (grep -E '^  (violation|guarded|aux) ' "$OUT/$s.check.txt" || true) | awk '{printf "%s%s %s x%s", (NR>1?", ":""), $1, $2, $3}')"
  if [ -f "$trace" ]; then gzip -f "$trace"; fi
  {
    echo "[set $set: $s] ${ran:-no Ginkgo summary} (${res:-?})"
    echo "      go test exit $tstatus ($test), checker exit $checker"
    echo "      records $records, StateDBs $dbs, precompile calls ${pcs:-0}, reverts ${reverts:-0}"
    echo "      findings: ${findings:-none}"
  } | tee -a "$SUMMARY"
done
if [ "$problems" -ne 0 ]; then
  echo "result: RUN PROBLEM (a suite failed or did not run, an empty trace, or a checker error; see above)" | tee -a "$SUMMARY"
  log "summary: $SUMMARY"
  exit 2
elif [ "$violated" -ne 0 ]; then
  echo "result: the checker reported violations; each one must be explained" | tee -a "$SUMMARY"
  log "summary: $SUMMARY"
  exit 1
fi
echo "result: every suite passed, no violation" | tee -a "$SUMMARY"
log "summary: $SUMMARY"
