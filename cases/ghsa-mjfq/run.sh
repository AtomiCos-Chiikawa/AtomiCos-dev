#!/usr/bin/env bash
# GHSA-mjfq-3qr2-6g84 (ISA-2025-004): partial precompile state writes under low gas.
# Pure reproduction: the fix commit's own regression test, run on the fix's parent and on the fix.
#   expected: vuln FAIL, fixed PASS
set -euo pipefail
CASE=ghsa-mjfq
VULN=5456ce8dd3d1bed0c7442ab2f0d4146a23efe624   # parent of the fix (v0.1.0-26)
FIXED=0fff8c144b24effbcb3addd666150ba5989d631c  # "Merge commit from fork" (2025-05-13)
GO_TOOLCHAIN=go1.23.6                            # go.mod: go 1.23.6
source "$(cd "$(dirname "$0")/.." && pwd)/lib.sh"

prepare_worktrees

# 1) regression test + caller contract exactly as in the fix commit
inject_from_fixed precompiles/distribution/integration_test.go
inject_from_fixed precompiles/testutil/contracts/DistributionCaller.sol
inject_from_fixed precompiles/testutil/contracts/DistributionCaller.json

# 2) observation only (no assertion changed): print the values the test compares.
#    Inserted right after `expectedGasCost := ...` inside the "tryClaimRewards" context.
for variant in vuln fixed; do
  f="$(wt "$variant")/precompiles/distribution/integration_test.go"
  #    perl, not `sed -i .../a\`: works with GNU and BSD userlands (BSD sed -i needs a suffix
  #    argument and drops the leading tabs of appended text). The inserted line (3 tabs + Printf)
  #    is byte-for-byte what the former GNU sed command produced.
  perl -pi -e '$_ .= "\t\t\tGinkgoWriter.Printf(\"REPRO tryClaimRewards(gasLimit=80000): accruedRewardsBefore=%s initialBalance=%s finalBalance=%s gasCost=%s extra(final-(initial-gas))=%s\\n\", accruedRewardsAmt, initialBalance.Amount, finalBalance.Amount, expectedGasCost, finalBalance.Amount.Sub(initialBalance.Amount.Sub(expectedGasCost)))\n" if /expectedGasCost := math.NewInt\(79_415_000_000_000\)/' "$f"
  grep -q 'REPRO tryClaimRewards' "$f" || { echo "observation hook not inserted in $variant"; exit 1; }
done

RAN_PATTERN='Ran [1-9][0-9]* of [0-9]+ Specs'   # Ginkgo summary line: the focused spec actually executed
# Failure signature of the vulnerable version (all must match, ANSI colours removed; see lib.sh):
# the tryClaimRewards spec fails on its balance assertion, at the line of that assertion.
ASSERT_LINE="$(grep -n 'expected final balance must be initial balance minus any gas spent' \
  "$(wt vuln)/precompiles/distribution/integration_test.go" | cut -d: -f1)"
case "$ASSERT_LINE" in ''|*[!0-9]*) echo "cannot locate the balance assertion in the injected test"; exit 1;; esac
EXPECT_FAIL=(
  '\[FAILED\] expected final balance must be initial balance minus any gas spent$'
  "In \\[It\\] at: .*/precompiles/distribution/integration_test\\.go:$ASSERT_LINE "
  '\[FAIL\] Calling distribution precompile from another contract tryClaimRewards \[It\] should claim rewards successfully'
)
ARGS=(-run TestPrecompileIntegrationTestSuite -count=1 -timeout 60m -ginkgo.focus "tryClaimRewards" -ginkgo.v)

# ./run.sh          step 4: pure reproduction (results/summary.txt)
# ./run.sh --judge  step 7: same input + AtomiCos harness (v01) + checker (results/judge/)
if [ "${1:-}" = "--judge" ]; then
  judge_case v01 precompiles/distribution "${ARGS[@]}"
else
  run_case precompiles/distribution "${ARGS[@]}"
fi
