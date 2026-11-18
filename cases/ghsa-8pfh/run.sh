#!/usr/bin/env bash
# GHSA-8pfh-j44r-f654: stale bank events from reverted precompile calls + shared BalanceHandler.
# Official root cause is still undisclosed; this case relies on the fix commit and its
# regression test (see README.md).
# Pure reproduction: the fix commit's ICS20 recursive-precompile regression test, run on the
# fix's parent and on the fix (in-process IBC test chains, nothing leaves the machine).
#   expected: vuln FAIL ("with reverts" sub-test), fixed PASS
set -euo pipefail
CASE=ghsa-8pfh
VULN=f4637461878001f6f41acdedad214db247d4f3dd   # parent of the fix on main (v0.5.0-rc.0-85)
FIXED=79089feebe79ce1f35250ba457cbd436e6bfff8b  # "Merge commit from fork" (2025-10-21)
GO_TOOLCHAIN=go1.23.8                            # go.mod: go 1.23.8
source "$(cd "$(dirname "$0")/.." && pwd)/lib.sh"

prepare_worktrees

# 1) regression test + its contracts, exactly as in the fix commit
for f in \
  contracts/erc20_recursive_reverting.go \
  contracts/erc20_recursive_non_reverting.go \
  contracts/solidity/ERC20RecursiveRevertingPrecompileCall.sol \
  contracts/solidity/ERC20RecursiveRevertingPrecompileCall.json \
  contracts/solidity/ERC20RecursiveNonRevertingPrecompileCall.sol \
  contracts/solidity/ERC20RecursiveNonRevertingPrecompileCall.json \
  evmd/tests/ibc/ics20_recursive_precompile_calls_test.go
do
  inject_from_fixed "$f"
done
# Not used: evmd/tests/integration/balance_handler/* needs a debug precompile written against the
# fixed API (BalanceHandlerFactory) and only counts events, so it cannot run on the parent as is.

# 2) observation only (no assertion changed): print the contract's bond-denom balance and the
#    event count right before each post-check compares them.
#    Only the two post-check closures are indented with 4 tabs; the same assertion at 3 tabs
#    (initial-state check, where eventAmount does not exist yet) is left untouched.
for variant in vuln fixed; do
  f="$(wt "$variant")/evmd/tests/ibc/ics20_recursive_precompile_calls_test.go"
  sed -i 's#^\(\t\t\t\t\)suite\.Require()\.Equal(contractBondDenomBalance\.Amount, sdkmath\.NewInt(\(0\|50\)))$#\1fmt.Printf("REPRO %s: contract bond-denom balance after transfer=%s (test expects \2), tx events=%d\\n", suite.T().Name(), contractBondDenomBalance.Amount, eventAmount)\n&#' "$f"
  [ "$(grep -c 'REPRO %s: contract bond-denom balance' "$f")" -eq 2 ] || { echo "observation hooks not inserted in $variant"; exit 1; }
done

RAN_PATTERN='--- (PASS|FAIL): TestICS20RecursivePrecompileCallsTestSuite'
# -tags=test: this version's test config (x/vm/types/*_testing.go) only exists under the 'test'
#             build tag; the upstream Makefile runs evmd tests the same way.
ARGS=(-tags=test ./tests/ibc/ -run TestICS20RecursivePrecompileCallsTestSuite -count=1 -v -timeout 30m)

# ./run.sh          step 4: pure reproduction (results/summary.txt)
# ./run.sh --judge  step 7: same input + AtomiCos harness (v05) + checker (results/judge/)
if [ "${1:-}" = "--judge" ]; then
  judge_case v05 evmd "${ARGS[@]}"
else
  run_case evmd "${ARGS[@]}"
fi
