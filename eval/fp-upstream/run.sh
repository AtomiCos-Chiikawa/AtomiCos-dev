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
#
# Set A (staking, distribution) was used to correct R3 (auto-withdrawn rewards), so after that
# fix it is no longer independent. Set B was never looked at while designing or fixing rules.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
REF=v0.7.4                     # latest patched release on release/v0.7.x at the time of writing
GO_TOOLCHAIN=go1.25.9
WT="$ROOT/.work/fp-v074"
OUT="$ROOT/eval/fp-upstream/results"
# set|package dir|test function
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
ONLY="${1:-}"

# reuse helpers (clone, checker build); CASE/VULN/FIXED are only needed by the mirror check
CASE=fp-upstream VULN=$REF FIXED=$REF
source "$ROOT/cases/lib.sh"
ensure_mirror
COMMIT="$(git -C "$MIRROR" rev-parse "$REF^{commit}")"
[ -d "$WT" ] || git -C "$MIRROR" worktree add --detach --quiet "$WT" "$COMMIT"
git -C "$WT" checkout --detach --quiet --force "$COMMIT"
git -C "$WT" clean -fdq
"$ROOT/tool/harness/inject.sh" v07 "$WT"
build_checker

mkdir -p "$OUT"
{
  echo "false-positive pre-measurement: upstream precompile integration suites"
  echo "date:   $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "tree:   cosmos/evm $REF ($COMMIT) + harness v07"
  echo
} > "$OUT/summary${ONLY:+-$ONLY}.txt"
for entry in "${SUITES[@]}"; do
  IFS='|' read -r set s testfn <<< "$entry"
  [ -z "$ONLY" ] || [ "$ONLY" = "$set" ] || continue
  trace="$OUT/$s.trace.jsonl"
  log "suite $s ..."
  set +e
  (cd "$WT/evmd" && ATOMICOS_TRACE="$trace" GOTOOLCHAIN="$GO_TOOLCHAIN" GOFLAGS="-p=4" \
     go test -tags=test "./tests/integration/precompiles/$s/" -run "^${testfn}\$" -count=1 -timeout 120m -v) \
     > "$OUT/$s.test.log" 2>&1
  set -e
  "$CHECKER" "$trace" > "$OUT/$s.check.txt" 2>&1 || true
  "$CHECKER" -json "$trace" > "$OUT/$s.check.json" 2>/dev/null || true
  ran="$( (grep -aoE 'Ran [0-9]+ of [0-9]+ Specs' "$OUT/$s.test.log" || true) | tail -1)"
  res="$( (sed 's/\x1b\[[0-9;]*m//g' "$OUT/$s.test.log" | grep -aoE '[0-9]+ Passed \| [0-9]+ Failed' || true) | tail -1)"
  records="$(wc -l < "$trace" 2>/dev/null || echo 0)"
  dbs="$( (grep -o '"db":"db[0-9]*"' "$trace" || true) | sort -u | wc -l)"
  pcs="$(grep -c '"point":"O2.enter"' "$trace" || true)"
  reverts="$(grep -c '"point":"O5.revert"' "$trace" || true)"
  findings="$( (grep -E '^  (violation|guarded|aux) ' "$OUT/$s.check.txt" || true) | awk '{printf "%s%s %s x%s", (NR>1?", ":""), $1, $2, $3}')"
  gzip -f "$trace"
  {
    echo "[set $set: $s] ${ran:-no Ginkgo summary} (${res:-?})"
    echo "      records $records, StateDBs $dbs, precompile calls $pcs, reverts $reverts"
    echo "      findings: ${findings:-none}"
  } | tee -a "$OUT/summary${ONLY:+-$ONLY}.txt"
done
log "summary: $OUT/summary${ONLY:+-$ONLY}.txt"
