// AtomiCos observation hooks for cosmos/evm v0.1.x (GHSA-mjfq family; copied into x/vm/statedb
// by inject.sh). This version uses *big.Int balances and has no runNativeAction, so only
// O4 (balance), O5 (snapshot/revert) and O6 (commit) are observed; O2 is not available.
//
// AtomicosHook is nil unless the AtomiCos harness installs it (ATOMICOS_TRACE is set).
// With a nil hook every function below returns immediately and execution is unchanged.

package statedb

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// AtomicosWriteBackMode names how this version reconciles a StateDB balance with x/bank at
// commit: keeper.SetBalance mints/burns until the bank total equals the StateDB balance.
const AtomicosWriteBackMode = "balance=bank_total"

// AtomicosAccount is a loaded StateDB account as the harness sees it.
type AtomicosAccount struct {
	Balance        *big.Int
	LockedSnapshot *big.Int // always nil in this version
}

// AtomicosView gives the harness read access to one StateDB without loading anything new.
type AtomicosView interface {
	Key() any
	Ctx() sdk.Context
	CacheCtx() (sdk.Context, bool)
	KeeperRef() any
	Loaded() map[common.Address]AtomicosAccount
	WriteBackMode() string
	// FreshBalance is the balance this StateDB would load for addr from ctx right now
	// (the version's own keeper.GetAccount, so decimals and spendable/total match).
	FreshBalance(ctx sdk.Context, addr common.Address) *big.Int
}

// AtomicosObserver receives the observation points (docs/rules.md §1).
type AtomicosObserver interface {
	Snapshot(v AtomicosView, id int)
	Reverted(v AtomicosView, id int)
	Balance(v AtomicosView, op string, addr common.Address, before, amount *big.Int)
	PrecompileEnter(v AtomicosView, precompile, caller common.Address, input []byte) int
	PrecompileExit(v AtomicosView, call int, recovered any, err error)
	Committed(v AtomicosView, recovered any)
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
	return new(big.Int).Set(acct.Balance)
}

func (v atomicosView) Loaded() map[common.Address]AtomicosAccount {
	out := make(map[common.Address]AtomicosAccount, len(v.s.stateObjects))
	for addr, obj := range v.s.stateObjects {
		if obj == nil {
			continue
		}
		out[addr] = AtomicosAccount{Balance: new(big.Int).Set(obj.Balance())}
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

func atomicosOnBalance(s *StateDB, op string, addr common.Address, amount *big.Int) {
	if AtomicosHook == nil {
		return
	}
	var amt *big.Int
	if amount != nil {
		amt = new(big.Int).Set(amount)
	}
	AtomicosHook.Balance(atomicosView{s}, op, addr, new(big.Int).Set(s.GetBalance(addr)), amt)
}

// atomicosAfterCommit must be deferred directly (it calls recover to see a failed commit
// and re-panics with the same value).
func atomicosAfterCommit(s *StateDB) {
	if AtomicosHook == nil {
		return
	}
	if r := recover(); r != nil {
		AtomicosHook.Committed(atomicosView{s}, r)
		panic(r)
	}
	AtomicosHook.Committed(atomicosView{s}, nil)
}

// AtomicosPrecompileEnter / Exit exist for API parity; this version has no call site for them.
func AtomicosPrecompileEnter(s *StateDB, precompile, caller common.Address, input []byte) int {
	if AtomicosHook == nil {
		return 0
	}
	return AtomicosHook.PrecompileEnter(atomicosView{s}, precompile, caller, input)
}

func AtomicosPrecompileExit(s *StateDB, call int, recovered any, err error) {
	if AtomicosHook != nil {
		AtomicosHook.PrecompileExit(atomicosView{s}, call, recovered, err)
	}
}
