// Package atomicos is the AtomiCos observation harness. inject.sh copies it into
// evmd/atomicos of a cosmos/evm worktree. evmd.NewExampleApp calls Register; when the
// environment variable ATOMICOS_TRACE names a file, the harness installs the StateDB hook
// and appends one JSON line per observation (format: tool/checker/trace.go).
//
// The harness only reads state. Every read happens in an isolated context (isolatedReadCtx:
// a throw-away cache branch, or a snapshot that is reverted on the precompile cache's layered
// store) with an infinite gas meter, so gas accounting, events and stores of the traced
// execution do not change.
package atomicos

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/common"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/cosmos/evm/x/vm/statedb"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// EnvTrace names the trace file. Unset = harness disabled.
const EnvTrace = "ATOMICOS_TRACE"

var (
	mu       sync.Mutex
	apps     []Keepers // one per app (IBC tests create several)
	out      *os.File
	seq      int
	dbIDs    = map[any]string{}
	calls    = map[any][]int{} // StateDB -> stack of active precompile call ids
	nextCall int
	obs      observer
)

// Register records the keepers of one app (several apps can exist, e.g. IBC tests).
// If ATOMICOS_TRACE is set, tracing starts immediately into that file.
func Register(k Keepers) {
	mu.Lock()
	defer mu.Unlock()
	apps = append(apps, k)
	path := os.Getenv(EnvTrace)
	if path == "" || out != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		panic(fmt.Sprintf("atomicos: cannot open %s: %v", path, err))
	}
	out = f
	statedb.AtomicosHook = obs
}

// SetTrace switches tracing to a new file (one per scenario) and restarts the numbering of
// records, StateDBs and precompile calls. An empty path stops tracing (the hook is removed).
// Used by the scenario runner.
func SetTrace(path string) error {
	mu.Lock()
	defer mu.Unlock()
	if out != nil {
		_ = out.Close()
		out = nil
	}
	seq, nextCall = 0, 0
	dbIDs, calls = map[any]string{}, map[any][]int{}
	if path == "" {
		statedb.AtomicosHook = nil
		return nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	out = f
	statedb.AtomicosHook = obs
	return nil
}

// ResetApps forgets the registered apps so that apps of finished scenarios can be freed.
// Call it before building a new network.
func ResetApps() {
	mu.Lock()
	defer mu.Unlock()
	apps = nil
}

type observer struct{}

// record is the JSON line; it mirrors checker.Record.
type record struct {
	Seq        int               `json:"seq"`
	Point      string            `json:"point"`
	DB         string            `json:"db"`
	Snap       int               `json:"snap,omitempty"`
	Call       int               `json:"call,omitempty"`
	Precompile string            `json:"precompile,omitempty"`
	Caller     string            `json:"caller,omitempty"`
	Method     string            `json:"method,omitempty"`
	Args       map[string]string `json:"args,omitempty"`
	Err        string            `json:"err,omitempty"`
	ErrKnown   bool              `json:"errKnown,omitempty"`
	Op         string            `json:"op,omitempty"`
	Addr       string            `json:"addr,omitempty"`
	Before     string            `json:"before,omitempty"`
	Amount     string            `json:"amount,omitempty"`
	State      *state            `json:"state,omitempty"`
	Stack      []string          `json:"stack,omitempty"`
}

// emit must be called with mu held.
func emit(r record) {
	seq++
	r.Seq = seq
	b, err := json.Marshal(r)
	if err != nil {
		b, _ = json.Marshal(record{Seq: seq, Point: r.Point, DB: r.DB, Err: "atomicos: marshal: " + err.Error()})
	}
	_, _ = out.Write(append(b, '\n'))
}

// appOf finds the app a StateDB belongs to: by EVM keeper pointer first; some query paths
// (e.g. EthCall, value receiver) build the StateDB with a pointer to a keeper copy, so fall
// back to the chain id of the context, then to the only registered app.
func appOf(v statedb.AtomicosView) (Keepers, bool) {
	ref := v.KeeperRef()
	for _, k := range apps {
		if k.EVM == ref {
			return k, true
		}
	}
	if len(apps) == 1 {
		return apps[0], true
	}
	// Several apps (IBC tests, and a new pair per sub-test): the right one is the app whose
	// store keys exist in this context's multistore. Try the newest first.
	for i := len(apps) - 1; i >= 0; i-- {
		if ownsContext(apps[i], v.Ctx()) {
			return apps[i], true
		}
	}
	return Keepers{}, false
}

// ownsContext reports whether a read through k works on ctx (it panics on foreign store keys).
func ownsContext(k Keepers, ctx sdk.Context) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	if ctx.MultiStore() == nil {
		return false
	}
	// same isolation as readState: on a layered precompile-cache store a CacheContext would push
	// a layer that is never popped
	c, release := isolatedReadCtx(ctx)
	defer release()
	k.Bank.GetSupply(c, evmtypes.GetEVMCoinDenom())
	return true
}

func dbID(v statedb.AtomicosView) string {
	k := v.Key()
	if id, ok := dbIDs[k]; ok {
		return id
	}
	id := fmt.Sprintf("db%d", len(dbIDs)+1)
	dbIDs[k] = id
	return id
}

func currentCall(v statedb.AtomicosView) int {
	st := calls[v.Key()]
	if len(st) == 0 {
		return 0
	}
	return st[len(st)-1]
}

func hexAddr(a common.Address) string { return strings.ToLower(a.Hex()) }

func intStr(x *big.Int) string {
	if x == nil {
		return ""
	}
	return x.String()
}

func (observer) Snapshot(v statedb.AtomicosView, id int) {
	mu.Lock()
	defer mu.Unlock()
	emit(record{Point: "O5.snapshot", DB: dbID(v), Snap: id, Call: currentCall(v), State: readState(v, false)})
}

func (observer) Reverted(v statedb.AtomicosView, id int) {
	mu.Lock()
	defer mu.Unlock()
	emit(record{Point: "O5.revert", DB: dbID(v), Snap: id, Call: currentCall(v), State: readState(v, false)})
}

func (observer) Balance(v statedb.AtomicosView, op string, addr common.Address, before, amount *big.Int) {
	mu.Lock()
	defer mu.Unlock()
	emit(record{
		Point: "O4.balance", DB: dbID(v), Call: currentCall(v),
		Op: op, Addr: hexAddr(addr), Before: intStr(before), Amount: intStr(amount),
		Stack: callSites(),
	})
}

func (observer) PrecompileEnter(v statedb.AtomicosView, precompile, caller common.Address, input []byte) int {
	mu.Lock()
	defer mu.Unlock()
	nextCall++
	id := nextCall
	calls[v.Key()] = append(calls[v.Key()], id)
	method, args := decodeMethod(precompile, input)
	emit(record{
		Point: "O2.enter", DB: dbID(v), Call: id,
		Precompile: hexAddr(precompile), Caller: hexAddr(caller), Method: method, Args: args,
		State: readState(v, false),
	})
	return id
}

func (observer) PrecompileExit(v statedb.AtomicosView, call int, recovered any, err error) {
	mu.Lock()
	defer mu.Unlock()
	st := calls[v.Key()]
	if n := len(st); n > 0 && st[n-1] == call {
		calls[v.Key()] = st[:n-1]
	}
	msg := ""
	switch {
	case recovered != nil:
		msg = fmt.Sprintf("panic: %v", recovered)
	case err != nil:
		msg = err.Error()
	}
	emit(record{Point: "O2.exit", DB: dbID(v), Call: call, Err: msg, State: readState(v, false)})
}

func (observer) Committed(v statedb.AtomicosView, recovered any, err error) {
	mu.Lock()
	defer mu.Unlock()
	msg := ""
	switch {
	case recovered != nil:
		msg = fmt.Sprintf("panic: %v", recovered)
	case err != nil:
		msg = err.Error()
	}
	// ErrKnown: Err holds both a panic and a returned error, so an empty Err means success
	emit(record{Point: "O6.commit", DB: dbID(v), Err: msg, ErrKnown: true, State: readState(v, true)})
}

// callSites returns the cosmos/evm frames that led to a balance change (outermost last).
func callSites() []string {
	pcs := make([]uintptr, 32)
	n := runtime.Callers(3, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	var out []string
	for {
		f, more := frames.Next()
		file := f.File
		if i := strings.Index(file, "/x/vm/"); i >= 0 {
			file = file[i+1:]
		} else if i := strings.Index(file, "/precompiles/"); i >= 0 {
			file = file[i+1:]
		} else {
			file = ""
		}
		if file != "" && !strings.Contains(file, "atomicos") {
			fn := f.Function
			if i := strings.LastIndex(fn, "/"); i >= 0 {
				fn = fn[i+1:]
			}
			out = append(out, fmt.Sprintf("%s:%d %s", file, f.Line, fn))
			if len(out) == 6 {
				break
			}
		}
		if !more {
			break
		}
	}
	return out
}
