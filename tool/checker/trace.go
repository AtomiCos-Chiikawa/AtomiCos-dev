// Package checker judges AtomiCos traces against the rules in docs/rules.md.
//
// The checker only reads trace records (JSON Lines written by tool/harness). It does not
// know which case, version or attack produced the trace, and it contains no case-specific
// numbers. All values are decimal strings so that 256-bit and wrapped values survive JSON.
package checker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

// Observation points (docs/rules.md §1).
const (
	PointSnapshot = "O5.snapshot" // StateDB.Snapshot(id) is about to be taken
	PointRevert   = "O5.revert"   // StateDB.RevertToSnapshot(id) has completed
	PointBalance  = "O4.balance"  // StateDB Add/Sub/SetBalance is about to be applied
	PointPCEnter  = "O2.enter"    // precompile entry, after the pre-call flush
	PointPCExit   = "O2.exit"     // precompile exit (success, error or panic)
	PointCommit   = "O6.commit"   // StateDB.Commit has completed
	PointTxBefore = "O1.before"   // optional: emitted by a test around a whole transaction
	PointTxAfter  = "O1.after"    // optional
)

// Record is one line of a trace.
type Record struct {
	Seq   int    `json:"seq"`
	Point string `json:"point"`
	// DB identifies the StateDB instance ("db1", "db2", ...). Nested EVM executions started
	// by a precompile (keeper.CallEVM) get their own StateDB and therefore their own DB.
	DB string `json:"db"`

	Snap int `json:"snap,omitempty"` // O5: snapshot id
	Call int `json:"call,omitempty"` // O2, O4: precompile call id (0 = outside any precompile)

	// O2
	Precompile string            `json:"precompile,omitempty"`
	Caller     string            `json:"caller,omitempty"`
	Method     string            `json:"method,omitempty"`
	Args       map[string]string `json:"args,omitempty"`
	Err        string            `json:"err,omitempty"`
	// O6: true when the harness records the error Commit returned in Err (Err empty = the
	// commit succeeded). Older harnesses recorded only panics; see commit.go.
	ErrKnown bool `json:"errKnown,omitempty"`

	// O4
	Op     string `json:"op,omitempty"` // add | sub | set
	Addr   string `json:"addr,omitempty"`
	Before string `json:"before,omitempty"`
	Amount string `json:"amount,omitempty"`

	State *State   `json:"state,omitempty"`
	Stack []string `json:"stack,omitempty"` // Go call sites that led to the observation
}

// State is everything the harness could read at an observation point.
// Keys of account maps are lower-case 0x-hex addresses.
type State struct {
	Denom   string            `json:"denom"`
	Bank    map[string]string `json:"bank"`              // total balance in the EVM denom
	Spend   map[string]string `json:"spendable"`         // spendable balance in the EVM denom
	Supply  string            `json:"supply"`            // total supply of the EVM denom
	Modules map[string]string `json:"modules,omitempty"` // module name -> address
	Blocked []string          `json:"blocked,omitempty"` // addresses that cannot receive funds

	// staking / vesting / distribution, keyed by "delegator|validator"
	Deleg     map[string]string     `json:"deleg,omitempty"` // delegation tokens
	Unbonding map[string]string     `json:"ubd,omitempty"`   // sum of unbonding entry balances
	ValTokens map[string]string     `json:"valTokens,omitempty"`
	Vesting   map[string]VestingRec `json:"vesting,omitempty"`  // by delegator address
	Rewards   map[string]string     `json:"rewards,omitempty"`  // claimable rewards (decimal)
	Withdraw  map[string]string     `json:"withdraw,omitempty"` // delegator -> withdraw address

	// events visible to the StateDB: in its base context and in its precompile cache context
	Events EventsRec `json:"events"`

	// StateDB accounts that are loaded in this StateDB instance
	StateDB map[string]StateDBRec `json:"statedb,omitempty"`

	Error string `json:"error,omitempty"` // the harness could not read part of the state
}

// VestingRec holds the delegation tracking of a vesting account.
type VestingRec struct {
	DelegatedVesting string `json:"dv"`
	DelegatedFree    string `json:"df"`
	Locked           string `json:"locked"`
}

// EventsRec describes the SDK events visible to a StateDB: in its base context (Ctx, these
// reach the transaction result) and in its precompile cache context (Cache, -1 = no cache yet).
// Sigs are short content hashes ("type:hash") in emission order.
type EventsRec struct {
	Ctx       int      `json:"ctx"`
	Cache     int      `json:"cache"`
	CtxSigs   []string `json:"ctxSigs,omitempty"`
	CacheSigs []string `json:"cacheSigs,omitempty"`
}

// StateDBRec describes one loaded StateDB account and its write-back reconciliation.
//
// WriteBack is the bank amount this StateDB balance will produce when it is written back at
// commit, and BankRef is the bank amount it must match right now. The harness computes both
// with the version's own write-back formula (e.g. v0.7.x: balance + locked snapshot vs bank
// total; older versions: balance vs spendable or total). Mode names the formula.
type StateDBRec struct {
	Balance   string `json:"bal"`
	WriteBack string `json:"writeback"`
	BankRef   string `json:"bankref"`
	Mode      string `json:"mode"`
}

// ReadTrace parses a JSON Lines trace.
func ReadTrace(r io.Reader) ([]Record, error) {
	var out []Record
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
	line := 0
	for sc.Scan() {
		line++
		b := sc.Bytes()
		if len(b) == 0 {
			continue
		}
		var rec Record
		if err := json.Unmarshal(b, &rec); err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		out = append(out, rec)
	}
	return out, sc.Err()
}
