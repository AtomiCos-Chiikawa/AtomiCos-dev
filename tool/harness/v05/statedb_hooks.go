// AtomiCos observation hooks for cosmos/evm v0.5-era main (GHSA-8pfh family) (copied into x/vm/statedb by inject.sh).
//
// AtomicosHook is nil unless the AtomiCos harness installs it (ATOMICOS_TRACE is set).
// With a nil hook every function below returns immediately and execution is unchanged.

package statedb

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// AtomicosWriteBackMode names how this version reconciles a StateDB balance with x/bank at
// commit: keeper.SetBalance mints/burns until the spendable balance equals the StateDB balance.
const AtomicosWriteBackMode = "balance=bank_spendable"

// AtomicosAccount is a loaded StateDB account as the harness sees it.
type AtomicosAccount struct {
	Balance        *big.Int
	LockedSnapshot *big.Int // nil when this version keeps no snapshot
}

// AtomicosView gives the harness read access to one StateDB without loading anything new.
type AtomicosView interface {
	Key() any // identity of the StateDB instance
	Ctx() sdk.Context
	CacheCtx() (sdk.Context, bool)
	KeeperRef() any
	Loaded() map[common.Address]AtomicosAccount
	WriteBackMode() string
	// FreshBalance is the balance this StateDB would load for addr from ctx right now
	// (the version's own keeper.GetAccount, so decimals and spendable/total match).
	FreshBalance(ctx sdk.Context, addr common.Address) *big.Int
}

// AtomicosObserver receives the observation points O2, O4, O5, O6 (docs/rules.md §1).
type AtomicosObserver interface {
	Snapshot(v AtomicosView, id int)
	Reverted(v AtomicosView, id int)
	Balance(v AtomicosView, op string, addr common.Address, before, amount *big.Int)
	PrecompileEnter(v AtomicosView, precompile, caller common.Address, input []byte) int
	PrecompileExit(v AtomicosView, call int, recovered any, err error)
	Committed(v AtomicosView, recovered any, err error)
}

// AtomicosHook is installed by the harness. nil = disabled.
var AtomicosHook AtomicosObserver

type atomicosView struct{ s *StateDB }

func (v atomicosView) Key() any              { return v.s }
func (v atomicosView) Ctx() sdk.Context      { return v.s.ctx }
func (v atomicosView) KeeperRef() any        { return v.s.keeper }
func (v atomicosView) WriteBackMode() string { return AtomicosWriteBackMode }
func (v atomicosView) CacheCtx() (sdk.Context, bool) {
	return v.s.cacheCtx, v.s.writeCache != nil
}

func (v atomicosView) FreshBalance(ctx sdk.Context, addr common.Address) *big.Int {
	acct := v.s.keeper.GetAccount(ctx, addr)
	if acct == nil || acct.Balance == nil {
		return new(big.Int)
	}
	return acct.Balance.ToBig()
}

func (v atomicosView) Loaded() map[common.Address]AtomicosAccount {
	out := make(map[common.Address]AtomicosAccount, len(v.s.stateObjects))
	for addr, obj := range v.s.stateObjects {
		if obj == nil {
			continue
		}
		out[addr] = AtomicosAccount{
			Balance:        obj.Balance().ToBig(),
			LockedSnapshot: nil, // this version keeps no locked snapshot
		}
	}
	return out
}

// AtomicosEnabled reports whether a harness is attached.
func AtomicosEnabled() bool { return AtomicosHook != nil }

func atomicosOnSnapshot(s *StateDB, id int) {
	if AtomicosHook != nil {
		AtomicosHook.Snapshot(atomicosView{s}, id)
	}
}

func atomicosAfterRevert(s *StateDB, id int) func() {
	if AtomicosHook == nil {
		return func() {}
	}
	return func() { AtomicosHook.Reverted(atomicosView{s}, id) }
}

func atomicosOnBalance(s *StateDB, op string, addr common.Address, amount *uint256.Int) {
	if AtomicosHook == nil {
		return
	}
	var amt *big.Int
	if amount != nil {
		amt = amount.ToBig()
	}
	// The balance operation itself loads the account, so reading it here changes nothing.
	AtomicosHook.Balance(atomicosView{s}, op, addr, s.GetBalance(addr).ToBig(), amt)
}

// atomicosAfterCommit must be deferred directly (it calls recover to see a failed commit
// and re-panics with the same value). err points to Commit's (named) result, so the error a
// failed commit returns is recorded too.
func atomicosAfterCommit(s *StateDB, err *error) {
	if AtomicosHook == nil {
		return
	}
	if r := recover(); r != nil {
		AtomicosHook.Committed(atomicosView{s}, r, nil)
		panic(r)
	}
	AtomicosHook.Committed(atomicosView{s}, nil, *err)
}

// AtomicosPrecompileEnter is called by precompiles/common after the pre-call flush.
func AtomicosPrecompileEnter(s *StateDB, precompile, caller common.Address, input []byte) int {
	if AtomicosHook == nil {
		return 0
	}
	return AtomicosHook.PrecompileEnter(atomicosView{s}, precompile, caller, input)
}

// AtomicosPrecompileExit is called by precompiles/common when the precompile returns or panics.
func AtomicosPrecompileExit(s *StateDB, call int, recovered any, err error) {
	if AtomicosHook != nil {
		AtomicosHook.PrecompileExit(atomicosView{s}, call, recovered, err)
	}
}
