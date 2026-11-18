#!/usr/bin/env bash
# inject.sh <family> <cosmos/evm worktree>
#
# Adds the AtomiCos observation hooks and the harness package to a cosmos/evm tree.
#   family v07: release/v0.7.x   (GHSA-7g4w)  O2 O4 O5 O6
#   family v05: v0.5-era main    (GHSA-8pfh)  O2 O4 O5 O6
#   family v01: v0.1.x           (GHSA-mjfq)  O4 O5 O6  (no runNativeAction -> no O2)
#
# Every insertion is checked to happen exactly once; the script fails otherwise.
# With ATOMICOS_TRACE unset the inserted code does nothing (the hook stays nil).
set -euo pipefail

FAM="${1:?family (v07|v05|v01)}"
WT="${2:?worktree path}"
HERE="$(cd "$(dirname "$0")" && pwd)"
[ -d "$HERE/$FAM" ] || { echo "inject: unknown family $FAM"; exit 1; }

SD="$WT/x/vm/statedb/statedb.go"
PC="$WT/precompiles/common/precompile.go"
APP="$WT/evmd/app.go"

# harness package files are always refreshed; code insertions happen only once
refresh_package() {
  mkdir -p "$WT/evmd/atomicos"
  cp "$HERE/observer/"*.go "$WT/evmd/atomicos/"
  cp "$HERE/$FAM/abis.go" "$WT/evmd/atomicos/abis.go"
  cp "$HERE/$FAM/statedb_hooks.go" "$WT/x/vm/statedb/atomicos_hooks.go"
  # the store types package moved between versions: use whatever precompiles/common imports
  local storeimp
  storeimp="$(grep -ho 'storetypes "[^"]*"' "$PC" | head -1)"
  [ -n "$storeimp" ] || { echo "inject: cannot find the storetypes import in $PC"; exit 1; }
  # perl, not `sed -i`: works with GNU and BSD userlands (BSD sed -i needs a suffix argument).
  # \Q..\E: the import line is matched literally; the values are passed through the environment.
  ATOMICOS_FROM='storetypes "cosmossdk.io/store/types"' ATOMICOS_TO="$storeimp" \
    perl -pi -e 's/\Q$ENV{ATOMICOS_FROM}\E/$ENV{ATOMICOS_TO}/' "$WT/evmd/atomicos/"*.go
}

if grep -q "atomicosOnSnapshot" "$SD"; then
  refresh_package
  echo "inject: $WT already instrumented (harness package refreshed)"; exit 0
fi

# insert_after <file> <literal line prefix> <text> : insert text after the first line that starts with prefix
insert_after() {
  local file="$1" prefix="$2" text="$3"
  awk -v p="$prefix" -v t="$text" '{ print } !done && index($0, p) == 1 { print t; done = 1 }' "$file" > "$file.atomicos"
  mv "$file.atomicos" "$file"
}

# must_count <file> <literal> <n>
must_count() {
  local n
  n="$(grep -cF -- "$2" "$1" || true)"
  [ "$n" -eq "$3" ] || { echo "inject: expected $3 x '$2' in $1, found $n"; exit 1; }
}

# ---- 1. StateDB hooks (O4, O5, O6)
cp "$HERE/$FAM/statedb_hooks.go" "$WT/x/vm/statedb/atomicos_hooks.go"
insert_after "$SD" 'func (s *StateDB) Snapshot() int {'               $'\tatomicosOnSnapshot(s, s.nextRevisionID)'
insert_after "$SD" 'func (s *StateDB) RevertToSnapshot(revid int) {'  $'\tdefer atomicosAfterRevert(s, revid)()'
insert_after "$SD" 'func (s *StateDB) AddBalance('                    $'\tatomicosOnBalance(s, "add", addr, amount)'
insert_after "$SD" 'func (s *StateDB) SubBalance('                    $'\tatomicosOnBalance(s, "sub", addr, amount)'
insert_after "$SD" 'func (s *StateDB) Commit() error {'               $'\tdefer atomicosAfterCommit(s)'
must_count "$SD" 'atomicosOnSnapshot(s, s.nextRevisionID)' 1
must_count "$SD" 'defer atomicosAfterRevert(s, revid)()' 1
must_count "$SD" 'atomicosOnBalance(s, "add", addr, amount)' 1
must_count "$SD" 'atomicosOnBalance(s, "sub", addr, amount)' 1
must_count "$SD" 'defer atomicosAfterCommit(s)' 1
if grep -q '^func (s \*StateDB) SetBalance(' "$SD"; then
  insert_after "$SD" 'func (s *StateDB) SetBalance(' $'\tatomicosOnBalance(s, "set", addr, amount)'
  must_count "$SD" 'atomicosOnBalance(s, "set", addr, amount)' 1
fi

# ---- 2. precompile entry/exit (O2), only where runNativeAction exists
if [ "$FAM" != "v01" ]; then
  awk '
    /^func \(p Precompile\) runNativeAction\(/ { infn = 1 }
    infn && $0 == "\tinitialGas := ctx.GasMeter().GasConsumed()" {
      print "\tif statedb.AtomicosEnabled() {"
      print "\t\tatomicosCall := statedb.AtomicosPrecompileEnter(stateDB, contract.Address(), contract.Caller(), contract.Input)"
      print "\t\tdefer func() {"
      print "\t\t\tif r := recover(); r != nil {"
      print "\t\t\t\tstatedb.AtomicosPrecompileExit(stateDB, atomicosCall, r, err)"
      print "\t\t\t\tpanic(r)"
      print "\t\t\t}"
      print "\t\t\tstatedb.AtomicosPrecompileExit(stateDB, atomicosCall, nil, err)"
      print "\t\t}()"
      print "\t}"
      infn = 0
    }
    { print }
  ' "$PC" > "$PC.atomicos" && mv "$PC.atomicos" "$PC"
  must_count "$PC" 'statedb.AtomicosPrecompileEnter(stateDB' 1
fi

# ---- 3. harness package and registration in evmd.NewExampleApp
refresh_package

insert_after "$APP" 'import (' $'\tatomicos "github.com/cosmos/evm/evmd/atomicos"'
awk '
  /^func NewExampleApp\(/ { infn = 1 }
  infn && $0 == "\treturn app" {
    print "\tatomicos.Register(atomicos.Keepers{"
    print "\t\tEVM:     app.EVMKeeper,"
    print "\t\tChainID: app.ChainID,"
    print "\t\tAccount: app.AccountKeeper,"
    print "\t\tBank:    app.BankKeeper,"
    print "\t\tStaking: app.StakingKeeper,"
    print "\t\tDistr:   app.DistrKeeper,"
    print "\t})"
    infn = 0
  }
  { print }
' "$APP" > "$APP.atomicos" && mv "$APP.atomicos" "$APP"
must_count "$APP" 'atomicos.Register(atomicos.Keepers{' 1
must_count "$APP" 'atomicos "github.com/cosmos/evm/evmd/atomicos"' 1

echo "inject: $FAM hooks added to $WT"
