#!/usr/bin/env bash
# Re-run one suite of one mutant (or CLEAN) N times in its own worktree, to check that a verdict
# is reproducible and not a side effect of machine load. Results go to results/recheck/.
#
#   ./eval/mutation/recheck.sh <id|CLEAN> <suite> [N]
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
ID="$1" S="$2" N="${3:-1}"
REF=v0.7.4
export GOTOOLCHAIN=go1.25.9
CASE=mutation VULN=$REF FIXED=$REF
source "$ROOT/cases/lib.sh"
export GOFLAGS="-p=4"
WT="$ROOT/.work/mut-recheck"
OUT="$HERE/results/recheck"
mkdir -p "$OUT"
declare -A FN=([staking]=TestStakingPrecompileIntegrationTestSuite [distribution]=TestDistributionPrecompileIntegrationTestSuite
  [bank]=TestBankPrecompileIntegrationTestSuite [erc20]=TestErc20IntegrationTestSuite [werc20]=TestWERC20PrecompileIntegrationTestSuite
  [gov]=TestGovPrecompileIntegrationTestSuite [slashing]=TestStakingPrecompileIntegrationTestSuite [ics20]=TestICS20PrecompileIntegrationTestSuite)

ensure_mirror
COMMIT="$(git -C "$MIRROR" rev-parse "$REF^{commit}")"
[ -d "$WT" ] || git -C "$MIRROR" worktree add --detach --quiet "$WT" "$COMMIT"
git -C "$WT" checkout --detach --quiet --force "$COMMIT"
git -C "$WT" clean -fdq
[ "$ID" = CLEAN ] || git -C "$WT" apply "$HERE/patches/$ID.patch"
"$ROOT/tool/harness/inject.sh" v07 "$WT" > /dev/null
build_checker > /dev/null
for i in $(seq 1 "$N"); do
  t="$OUT/$ID.$S.$i.trace.jsonl"
  set +e
  (cd "$WT/evmd" && ATOMICOS_TRACE="$t" go test -tags=test "./tests/integration/precompiles/$S/" \
     -run "^${FN[$S]}\$" -count=1 -timeout 20m -v) > "$OUT/$ID.$S.$i.test.log" 2>&1
  st=$?
  set -e
  ran="$(sed 's/\x1b\[[0-9;]*m//g' "$OUT/$ID.$S.$i.test.log" | grep -aoE 'Ran [0-9]+ of [0-9]+ Specs|[0-9]+ Passed \| [0-9]+ Failed' | tr '\n' ' ' || true)"
  v="$("$CHECKER" "$t" 2>&1 | (grep -E '^  violation ' || true) | awk '{printf "%s=%s ", $2, $3}')"
  printf '%s\t%s\t%d\texit=%d\t%s\tviolations: %s\n' "$ID" "$S" "$i" "$st" "$ran" "${v:-none}" | tee -a "$OUT/summary.tsv"
  [ -s "$t" ] && gzip -f "$t"
  [ $st -eq 0 ] && rm -f "$OUT/$ID.$S.$i.test.log" || gzip -f "$OUT/$ID.$S.$i.test.log"
done
git -C "$WT" checkout --detach --quiet --force "$COMMIT"; git -C "$WT" clean -fdq
