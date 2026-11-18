#!/usr/bin/env bash
# GHSA-7g4w-cg88-2cq2: balance underflow in EVM StateDB (vesting account + staking precompile).
# The fix's own regression tests are unit tests on StateDB.SubBalance / ParseAmount, which do not
# exercise the vesting + staking path, so this case uses a hand-written test (test/).
#   expected: vuln FAIL (StateDB balance wraps around), fixed PASS
set -euo pipefail
CASE=ghsa-7g4w
VULN=0fe56c036b72aeaf60e5862b2a8bd6d9893a5ef9   # parent of the fix on release/v0.7.x (v0.7.1-2)
FIXED=0182da198f1f84dc0e4c937fca5e5f175d1be986  # backport of #1176 (PR #1254), shipped in v0.7.2
GO_TOOLCHAIN=go1.25.9                            # go.mod: go 1.25.9
HERE="$(cd "$(dirname "$0")" && pwd)"
source "$HERE/../lib.sh"

prepare_worktrees

PKG=evmd/tests/integration/atomicos_ghsa7g4w
inject_local "$HERE/test/vesting_delegate_underflow_test.go" "$PKG/vesting_delegate_underflow_test.go"
inject_local "$HERE/test/probe_contract.go"                  "$PKG/probe_contract.go"

RAN_PATTERN='--- (PASS|FAIL): TestGHSA7g4w_VestingDelegateBeyondSpendable'
# -tags=test: test config (x/vm/types/*_testing.go) only exists under the 'test' build tag,
#             as in the upstream Makefile.
ARGS=(-tags=test ./tests/integration/atomicos_ghsa7g4w/ -run TestGHSA7g4w -count=1 -v -timeout 30m)

# ./run.sh          step 4: pure reproduction (results/summary.txt)
# ./run.sh --judge  step 7: same input + AtomiCos harness (v07) + checker (results/judge/)
if [ "${1:-}" = "--judge" ]; then
  judge_case v07 evmd "${ARGS[@]}"
else
  run_case evmd "${ARGS[@]}"
fi
