package atomicos

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"

	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	vestingexported "github.com/cosmos/cosmos-sdk/x/auth/vesting/exported"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/cosmos/evm/x/vm/statedb"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// Keepers are the read-only views the harness needs. Field types are small interfaces so the
// same source compiles against every cosmos/evm version we trace.
type Keepers struct {
	EVM     any           // *evmkeeper.Keeper, used only as an identity to find the app of a StateDB
	ChainID func() string // the app's chain id (read lazily: it can be set after construction)
	Account interface {
		IterateAccounts(ctx context.Context, cb func(account sdk.AccountI) (stop bool))
	}
	Bank interface {
		GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin
		SpendableCoin(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin
		LockedCoins(ctx context.Context, addr sdk.AccAddress) sdk.Coins
		GetSupply(ctx context.Context, denom string) sdk.Coin
		BlockedAddr(addr sdk.AccAddress) bool
	}
	Staking interface {
		GetAllDelegations(ctx context.Context) ([]stakingtypes.Delegation, error)
		GetAllValidators(ctx context.Context) ([]stakingtypes.Validator, error)
		GetAllUnbondingDelegations(ctx context.Context, delegator sdk.AccAddress) ([]stakingtypes.UnbondingDelegation, error)
	}
	Distr distrkeeper.Keeper
}

// state mirrors checker.State.
type state struct {
	Denom     string                `json:"denom"`
	Bank      map[string]string     `json:"bank"`
	Spend     map[string]string     `json:"spendable"`
	Supply    string                `json:"supply"`
	Modules   map[string]string     `json:"modules,omitempty"`
	Blocked   []string              `json:"blocked,omitempty"`
	Deleg     map[string]string     `json:"deleg,omitempty"`
	Unbonding map[string]string     `json:"ubd,omitempty"`
	ValTokens map[string]string     `json:"valTokens,omitempty"`
	Vesting   map[string]vestingRec `json:"vesting,omitempty"`
	Rewards   map[string]string     `json:"rewards,omitempty"`
	Withdraw  map[string]string     `json:"withdraw,omitempty"`
	Events    eventsRec             `json:"events"`
	StateDB   map[string]stateDBRec `json:"statedb,omitempty"`
	Error     string                `json:"error,omitempty"`
}

type vestingRec struct {
	DelegatedVesting string `json:"dv"`
	DelegatedFree    string `json:"df"`
	Locked           string `json:"locked"`
}

type eventsRec struct {
	Ctx       int      `json:"ctx"`
	Cache     int      `json:"cache"`
	CtxSigs   []string `json:"ctxSigs,omitempty"`
	CacheSigs []string `json:"cacheSigs,omitempty"`
}

type stateDBRec struct {
	Balance   string `json:"bal"`
	WriteBack string `json:"writeback"`
	BankRef   string `json:"bankref"`
	Mode      string `json:"mode"`
}

// readState reads everything the rules need. committed=false: as the StateDB currently sees
// the SDK state (its precompile cache context if it has one, otherwise its base context).
// committed=true: the base context only, i.e. what a finished Commit has produced.
func readState(v statedb.AtomicosView, committed bool) (s *state) {
	s = &state{}
	defer func() {
		if r := recover(); r != nil {
			s.Error = fmt.Sprintf("atomicos: read failed: %v", r)
		}
	}()

	k, ok := appOf(v)
	if !ok {
		s.Error = "atomicos: StateDB belongs to an unregistered app"
		return s
	}

	base := v.Ctx()
	s.Events = eventsRec{Cache: -1}
	s.Events.CtxSigs = eventSigs(base)
	s.Events.Ctx = len(s.Events.CtxSigs)
	view := base
	if cache, ok := v.CacheCtx(); ok {
		s.Events.CacheSigs = eventSigs(cache)
		s.Events.Cache = len(s.Events.CacheSigs)
		if !committed {
			view = cache
		}
	}
	if view.MultiStore() == nil {
		s.Error = "atomicos: context has no multistore"
		return s
	}
	// Throw-away branch, infinite gas, no KV gas: reads cannot affect the traced execution.
	ctx, _ := view.CacheContext()
	ctx = ctx.WithGasMeter(storetypes.NewInfiniteGasMeter()).
		WithKVGasConfig(storetypes.GasConfig{}).
		WithTransientKVGasConfig(storetypes.GasConfig{})

	denom := evmtypes.GetEVMCoinDenom()
	s.Denom = denom
	s.Bank, s.Spend = map[string]string{}, map[string]string{}
	s.Modules, s.Vesting = map[string]string{}, map[string]vestingRec{}
	s.Supply = k.Bank.GetSupply(ctx, denom).Amount.String()

	var accs []sdk.AccountI
	k.Account.IterateAccounts(ctx, func(a sdk.AccountI) bool {
		accs = append(accs, a)
		return false
	})
	for _, a := range accs {
		addr := a.GetAddress()
		h := hexAddr(common.BytesToAddress(addr))
		s.Bank[h] = k.Bank.GetBalance(ctx, addr, denom).Amount.String()
		s.Spend[h] = k.Bank.SpendableCoin(ctx, addr, denom).Amount.String()
		if k.Bank.BlockedAddr(addr) {
			s.Blocked = append(s.Blocked, h)
		}
		if m, ok := a.(sdk.ModuleAccountI); ok {
			s.Modules[m.GetName()] = h
		}
		if va, ok := a.(vestingexported.VestingAccount); ok {
			s.Vesting[h] = vestingRec{
				DelegatedVesting: va.GetDelegatedVesting().AmountOf(denom).String(),
				DelegatedFree:    va.GetDelegatedFree().AmountOf(denom).String(),
				Locked:           k.Bank.LockedCoins(ctx, addr).AmountOf(denom).String(),
			}
		}
	}
	sort.Strings(s.Blocked)

	readStaking(ctx, k, s, denom, accs)

	// StateDB side: loaded accounts and their write-back reconciliation.
	s.StateDB = map[string]stateDBRec{}
	mode := v.WriteBackMode()
	for addr, acct := range v.Loaded() {
		h := hexAddr(addr)
		// fresh = what this StateDB would load now (version's own GetAccount: spendable or total,
		// already in EVM units), so the comparison is decimal-correct.
		fresh := v.FreshBalance(ctx, addr)
		rec := stateDBRec{Balance: acct.Balance.String(), Mode: mode}
		switch mode {
		case "balance+locked_snapshot=bank_total":
			// commit writes balance + locked snapshot as the bank total; now the total is
			// fresh spendable + currently locked.
			snap := acct.LockedSnapshot
			if snap == nil {
				snap = new(big.Int)
			}
			lockedNow := k.Bank.LockedCoins(ctx, sdk.AccAddress(addr.Bytes())).AmountOf(denom).BigInt()
			rec.WriteBack = new(big.Int).Add(acct.Balance, snap).String()
			rec.BankRef = new(big.Int).Add(fresh, lockedNow).String()
		default: // "balance=bank_spendable", "balance=bank_total": commit makes the fresh read equal the balance
			rec.WriteBack, rec.BankRef = acct.Balance.String(), fresh.String()
		}
		s.StateDB[h] = rec
	}
	return s
}

func readStaking(ctx sdk.Context, k Keepers, s *state, denom string, accs []sdk.AccountI) {
	vals, err := k.Staking.GetAllValidators(ctx)
	if err != nil {
		s.Error = "atomicos: validators: " + err.Error()
		return
	}
	byOp := map[string]stakingtypes.Validator{}
	s.ValTokens = map[string]string{}
	for _, v := range vals {
		byOp[v.GetOperator()] = v
		s.ValTokens[v.GetOperator()] = v.GetTokens().String()
	}
	dels, err := k.Staking.GetAllDelegations(ctx)
	if err != nil {
		s.Error = "atomicos: delegations: " + err.Error()
		return
	}
	s.Deleg, s.Rewards, s.Withdraw, s.Unbonding = map[string]string{}, map[string]string{}, map[string]string{}, map[string]string{}
	q := distrkeeper.NewQuerier(k.Distr)
	// unbonding entries of every account (also those with no delegation left)
	for _, a := range accs {
		ubds, err := k.Staking.GetAllUnbondingDelegations(ctx, a.GetAddress())
		if err != nil {
			continue
		}
		dh := hexAddr(common.BytesToAddress(a.GetAddress()))
		for _, u := range ubds {
			total := new(big.Int)
			for _, e := range u.Entries {
				total.Add(total, e.Balance.BigInt())
			}
			s.Unbonding[dh+"|"+u.ValidatorAddress] = total.String()
		}
	}
	seen := map[string]bool{}
	for _, d := range dels {
		delAcc, err := sdk.AccAddressFromBech32(d.DelegatorAddress)
		if err != nil {
			continue
		}
		dh := hexAddr(common.BytesToAddress(delAcc))
		key := dh + "|" + d.ValidatorAddress
		if v, ok := byOp[d.ValidatorAddress]; ok {
			s.Deleg[key] = v.TokensFromShares(d.Shares).TruncateInt().String()
		}
		if res, err := q.DelegationRewards(ctx, &distrtypes.QueryDelegationRewardsRequest{
			DelegatorAddress: d.DelegatorAddress, ValidatorAddress: d.ValidatorAddress,
		}); err == nil {
			s.Rewards[key] = res.Rewards.AmountOf(denom).String()
		}
		if !seen[dh] {
			seen[dh] = true
			if w, err := k.Distr.GetDelegatorWithdrawAddr(ctx, delAcc); err == nil {
				s.Withdraw[dh] = hexAddr(common.BytesToAddress(w))
			}
		}
	}
}

// eventSigs returns a short content hash per event (type and attributes, in order).
func eventSigs(ctx sdk.Context) []string {
	if ctx.EventManager() == nil {
		return nil
	}
	evs := ctx.EventManager().Events()
	out := make([]string, len(evs))
	for i, e := range evs {
		h := sha256.New()
		h.Write([]byte(e.Type))
		for _, a := range e.Attributes {
			h.Write([]byte{0})
			h.Write([]byte(a.Key))
			h.Write([]byte{'='})
			h.Write([]byte(a.Value))
		}
		out[i] = e.Type + ":" + hex.EncodeToString(h.Sum(nil)[:6])
	}
	return out
}
