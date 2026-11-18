package checker

import (
	"fmt"
	"math/big"
	"sort"
	"strings"
)

// Severity of a finding.
const (
	SevViolation = "violation" // a rule in docs/rules.md §2 is broken
	SevGuarded   = "guarded"   // the offending change was attempted but the system rejected it
	SevAux       = "aux"       // auxiliary check (docs/rules.md, A1/A2); never a verdict on its own
)

// Finding is one rule violation (or auxiliary observation).
type Finding struct {
	Rule     string   `json:"rule"`
	Severity string   `json:"severity"`
	Point    string   `json:"point"`
	Seq      int      `json:"seq"`
	DB       string   `json:"db"`
	Call     int      `json:"call,omitempty"`
	Snap     int      `json:"snap,omitempty"`
	Subject  string   `json:"subject"`
	Expected string   `json:"expected"`
	Actual   string   `json:"actual"`
	Detail   string   `json:"detail,omitempty"`
	Outcome  string   `json:"outcome,omitempty"`
	Repeats  int      `json:"repeats,omitempty"` // identical findings later in the same DB that were folded in
	Stack    []string `json:"stack,omitempty"`
}

var two256 = new(big.Int).Lsh(big.NewInt(1), 256)

// Check applies every rule to the trace and returns the findings ordered by sequence number.
func Check(recs []Record) []Finding {
	var all []Finding
	all = append(all, checkR1(recs)...)
	all = append(all, checkR2A(recs)...)
	all = append(all, checkR2W(recs)...)
	all = append(all, checkR3(recs)...)
	all = append(all, checkA1(recs)...)
	sort.SliceStable(all, func(i, j int) bool { return all[i].Seq < all[j].Seq })
	return fold(all)
}

// fold merges repeated findings (same rule, DB and subject) into the first one.
func fold(in []Finding) []Finding {
	type key struct{ rule, db, subject string }
	idx := map[key]int{}
	var out []Finding
	for _, f := range in {
		k := key{f.Rule, f.DB, f.Subject}
		if i, ok := idx[k]; ok {
			out[i].Repeats++
			continue
		}
		idx[k] = len(out)
		out = append(out, f)
	}
	return out
}

// ---------------------------------------------------------------- R1: call-level atomicity
//
// R1 is judged on what gets committed, not on what a StateDB happens to show internally:
// versions differ in how they undo precompile writes (v0.7 restores a snapshot store, v0.1
// drops the whole cache at commit, v0.5 swaps the event list), so "visible right after the
// revert" is only a candidate. A candidate is confirmed when the top-level StateDB commits.
//
//   1. At each revert, compare the StateDB's SDK view and events with the matching snapshot.
//      Differences are leftovers of the reverted frame (candidates).
//   2. A StateDB that first appears while another StateDB is inside a precompile call is
//      nested (keeper.CallEVM from a precompile); its candidates are carried to its top-level
//      StateDB, because a nested context may or may not reach the transaction result.
//   3. At a successful commit of a top-level StateDB (its base context, i.e. the result):
//      R1-S: a candidate value that is still there (equal to the reverted-frame value and not
//            to the snapshot value) is a violation;
//      R1-E: a candidate event that appears in the committed events (beyond the events that
//            existed when the StateDB was first observed) is a violation (USENIX Sec'26 I2).

type snapKey struct {
	db   string
	snap int
}

type leftover struct {
	snapVal, revVal string
	seq             int
}

func checkR1(recs []Record) []Finding {
	var out []Finding
	snaps := map[snapKey]Record{}
	writes := map[string]map[string]leftover{} // top-level db -> state key -> leftover
	events := map[string]map[string]int{}      // top-level db -> event sig -> count
	firstEv := map[string]map[string]int{}
	base := map[string]map[string]int{}
	parent := map[string]string{}
	var open []string
	root := func(db string) string {
		for parent[db] != "" {
			db = parent[db]
		}
		return db
	}
	seen := map[string]bool{}
	for _, r := range recs {
		if !seen[r.DB] {
			seen[r.DB] = true
			if n := len(open); n > 0 && open[n-1] != r.DB {
				parent[r.DB] = open[n-1]
			}
		}
		if r.State != nil {
			if _, ok := base[r.DB]; !ok {
				base[r.DB] = multiset(r.State.Events.CtxSigs)
			}
		}
		switch r.Point {
		case PointPCEnter:
			open = append(open, r.DB)
		case PointPCExit:
			for i := len(open) - 1; i >= 0; i-- {
				if open[i] == r.DB {
					open = append(open[:i], open[i+1:]...)
					break
				}
			}
		case PointSnapshot:
			snaps[snapKey{r.DB, r.Snap}] = r
		case PointRevert:
			s, ok := snaps[snapKey{r.DB, r.Snap}]
			if !ok || s.State == nil || r.State == nil {
				continue
			}
			top := root(r.DB)
			if writes[top] == nil {
				writes[top], events[top], firstEv[top] = map[string]leftover{}, map[string]int{}, map[string]int{}
			}
			for _, d := range diffSDK(s.State, r.State) {
				lo, ok := writes[top][d.key]
				if !ok {
					lo = leftover{snapVal: d.before, seq: r.Seq}
				}
				lo.revVal = d.after
				writes[top][d.key] = lo
			}
			before := multiset(append(append([]string{}, s.State.Events.CtxSigs...), s.State.Events.CacheSigs...))
			after := multiset(append(append([]string{}, r.State.Events.CtxSigs...), r.State.Events.CacheSigs...))
			for sig, n := range after {
				if extra := n - before[sig]; extra > 0 {
					events[top][sig] += extra
					if _, ok := firstEv[top][sig]; !ok {
						firstEv[top][sig] = r.Seq
					}
				}
			}
		case PointCommit:
			if parent[r.DB] != "" || r.Err != "" || r.State == nil {
				continue
			}
			for _, k := range sortedKeys(writes[r.DB]) {
				lo := writes[r.DB][k]
				got := valueOf(r.State, k)
				if numEqual(got, numOrZero(lo.revVal)) && !numEqual(got, numOrZero(lo.snapVal)) {
					out = append(out, Finding{
						Rule: "R1-S", Severity: SevViolation, Point: PointCommit,
						Seq: r.Seq, DB: r.DB, Subject: k,
						Expected: lo.snapVal + " (value before the reverted frame)",
						Actual:   got + " (committed; equals the value the reverted frame left behind)",
						Detail:   fmt.Sprintf("SDK write of a reverted frame was committed (revert at seq %d)", lo.seq),
					})
				}
			}
			final := multiset(r.State.Events.CtxSigs)
			for _, sig := range sortedKeys(events[r.DB]) {
				emitted := final[sig] - base[r.DB][sig]
				if emitted <= 0 {
					continue
				}
				out = append(out, Finding{
					Rule: "R1-E", Severity: SevViolation, Point: PointCommit,
					Seq: r.Seq, DB: r.DB, Subject: "event " + sig,
					Expected: "events of reverted frames are absent from the committed events",
					Actual: fmt.Sprintf("appears %d time(s) in the committed events; %d copy(ies) were left behind by reverts (first at seq %d)",
						emitted, events[r.DB][sig], firstEv[r.DB][sig]),
				})
			}
		}
	}
	return out
}

// valueOf reads a diffSDK key ("bank:0x…", "deleg:…", "vesting:…|dv", "supply") from a state.
func valueOf(s *State, key string) string {
	pick := func(m map[string]string, k string) string { return numOrZero(m[k]) }
	switch {
	case key == "supply":
		return numOrZero(s.Supply)
	case strings.HasPrefix(key, "bank:"):
		return pick(s.Bank, strings.TrimPrefix(key, "bank:"))
	case strings.HasPrefix(key, "deleg:"):
		return pick(s.Deleg, strings.TrimPrefix(key, "deleg:"))
	case strings.HasPrefix(key, "ubd:"):
		return pick(s.Unbonding, strings.TrimPrefix(key, "ubd:"))
	case strings.HasPrefix(key, "valTokens:"):
		return pick(s.ValTokens, strings.TrimPrefix(key, "valTokens:"))
	case strings.HasPrefix(key, "rewards:"):
		return pick(s.Rewards, strings.TrimPrefix(key, "rewards:"))
	case strings.HasPrefix(key, "vesting:"):
		k := strings.TrimPrefix(key, "vesting:")
		i := strings.LastIndex(k, "|")
		if i < 0 {
			return "0"
		}
		v := s.Vesting[k[:i]]
		if k[i+1:] == "dv" {
			return numOrZero(v.DelegatedVesting)
		}
		return numOrZero(v.DelegatedFree)
	}
	return "0"
}

func multiset(xs []string) map[string]int {
	m := make(map[string]int, len(xs))
	for _, x := range xs {
		m[x]++
	}
	return m
}

type diff struct{ key, before, after string }

// diffSDK compares the SDK-side parts of two states. Missing numeric entries count as zero.
func diffSDK(a, b *State) []diff {
	var out []diff
	cmp := func(prefix string, x, y map[string]string) {
		for _, k := range unionKeys(x, y) {
			vx, vy := numOrZero(x[k]), numOrZero(y[k])
			if !numEqual(vx, vy) {
				out = append(out, diff{prefix + k, vx, vy})
			}
		}
	}
	cmp("bank:", a.Bank, b.Bank)
	cmp("deleg:", a.Deleg, b.Deleg)
	cmp("ubd:", a.Unbonding, b.Unbonding)
	cmp("valTokens:", a.ValTokens, b.ValTokens)
	cmp("rewards:", a.Rewards, b.Rewards)
	vest := func(s *State) map[string]string {
		m := map[string]string{}
		for k, v := range s.Vesting {
			m[k+"|dv"] = v.DelegatedVesting
			m[k+"|df"] = v.DelegatedFree
		}
		return m
	}
	cmp("vesting:", vest(a), vest(b))
	if !numEqual(numOrZero(a.Supply), numOrZero(b.Supply)) {
		out = append(out, diff{"supply", a.Supply, b.Supply})
	}
	return out
}

// ---------------------------------------------------------------- R2-A: arithmetic validity

func checkR2A(recs []Record) []Finding {
	var out []Finding
	for i, r := range recs {
		if r.Point != PointBalance || r.Op == "set" {
			continue
		}
		before, ok1 := parseInt(r.Before)
		amount, ok2 := parseInt(r.Amount)
		if !ok1 || !ok2 {
			continue
		}
		var f *Finding
		switch r.Op {
		case "sub":
			if amount.Cmp(before) > 0 {
				f = &Finding{
					Expected: fmt.Sprintf("sub amount <= current balance %s", before),
					Actual: fmt.Sprintf("sub %s from %s (unchecked result would be 2^256 - %s)",
						amount, before, new(big.Int).Sub(amount, before)),
					Detail: "balance underflow",
				}
			}
		case "add":
			if new(big.Int).Add(before, amount).Cmp(two256) >= 0 {
				f = &Finding{
					Expected: "balance + amount < 2^256",
					Actual:   fmt.Sprintf("add %s to %s", amount, before),
					Detail:   "balance overflow",
				}
			}
		}
		if f == nil {
			continue
		}
		f.Rule, f.Severity, f.Point = "R2-A", SevViolation, PointBalance
		f.Seq, f.DB, f.Call, f.Subject, f.Stack = r.Seq, r.DB, r.Call, r.Addr, r.Stack
		f.Outcome = balanceOutcome(recs, i)
		if strings.HasPrefix(f.Outcome, "rejected") {
			f.Severity = SevGuarded
		}
		out = append(out, *f)
	}
	return out
}

// balanceOutcome reports whether the offending balance change was applied or rejected,
// based on what the trace shows next for the same StateDB.
func balanceOutcome(recs []Record, i int) string {
	r := recs[i]
	for _, n := range recs[i+1:] {
		if n.DB != r.DB {
			continue
		}
		if n.Point == PointPCExit && n.Call == r.Call && strings.HasPrefix(n.Err, "panic:") {
			return "rejected (panic: " + strings.TrimSpace(strings.TrimPrefix(n.Err, "panic:")) + ")"
		}
		if n.Point == PointPCExit && n.Call == r.Call {
			return "applied"
		}
		if n.Point == PointBalance || n.Point == PointPCEnter || n.Point == PointCommit {
			return "applied"
		}
	}
	return "unknown (trace ends)"
}

// ---------------------------------------------------------------- R2-W: write-back consistency

func checkR2W(recs []Record) []Finding {
	var out []Finding
	for _, r := range recs {
		if r.State == nil {
			continue
		}
		sync := r.Point == PointPCEnter || (r.Point == PointPCExit && r.Err == "")
		if !sync {
			continue
		}
		blocked := map[string]bool{}
		for _, b := range r.State.Blocked {
			blocked[strings.ToLower(b)] = true
		}
		for _, addr := range sortedKeys(r.State.StateDB) {
			if blocked[addr] {
				continue
			}
			a := r.State.StateDB[addr]
			wb, ok1 := parseInt(a.WriteBack)
			ref, ok2 := parseInt(a.BankRef)
			if !ok1 || !ok2 || wb.Cmp(ref) == 0 {
				continue
			}
			out = append(out, Finding{
				Rule: "R2-W", Severity: SevViolation, Point: r.Point,
				Seq: r.Seq, DB: r.DB, Call: r.Call, Subject: addr,
				Expected: fmt.Sprintf("write-back %s == bank reference", a.BankRef),
				Actual:   fmt.Sprintf("write-back %s (StateDB balance %s, mode %s)", a.WriteBack, a.Balance, a.Mode),
				Detail:   "the StateDB balance would not reconcile with the bank at commit",
			})
		}
	}
	return out
}

// ---------------------------------------------------------------- R3: feature-level consistency

type callKey struct {
	db   string
	call int
}

func checkR3(recs []Record) []Finding {
	var out []Finding
	enters := map[callKey]Record{}
	for _, r := range recs {
		switch r.Point {
		case PointPCEnter:
			enters[callKey{r.DB, r.Call}] = r
		case PointPCExit:
			e, ok := enters[callKey{r.DB, r.Call}]
			if !ok || r.Err != "" || e.State == nil || r.State == nil {
				continue
			}
			out = append(out, r3For(e, r)...)
		}
	}
	return out
}

func r3For(enter, exit Record) []Finding {
	var out []Finding
	a, b := enter.State, exit.State
	add := func(rule, subject, expected, actual, detail string) {
		out = append(out, Finding{
			Rule: rule, Severity: SevViolation, Point: PointPCExit,
			Seq: exit.Seq, DB: exit.DB, Call: exit.Call, Subject: subject,
			Expected: expected, Actual: actual,
			Detail: fmt.Sprintf("%s.%s returned success; %s", short(enter.Precompile), enter.Method, detail),
		})
	}
	delta := func(m1, m2 map[string]string, k string) *big.Int {
		x, _ := parseInt(numOrZero(m1[k]))
		y, _ := parseInt(numOrZero(m2[k]))
		return new(big.Int).Sub(y, x)
	}
	d := strings.ToLower(enter.Args["delegatorAddress"])
	v := enter.Args["validatorAddress"]
	amt, haveAmt := parseInt(enter.Args["amount"])
	// Changing a delegation first withdraws the rewards accrued on it (x/distribution hook
	// BeforeDelegationSharesModified -> withdrawDelegationRewards), paid from the distribution
	// module account to the delegator's withdraw address. paid is that amount (0 if none).
	w := strings.ToLower(a.Withdraw[d])
	if w == "" {
		w = d
	}
	paid := new(big.Int)
	if distr := strings.ToLower(a.Modules["distribution"]); distr != "" {
		paid.Neg(delta(a.Bank, b.Bank, distr))
	}
	// expected change of the delegator's bank balance, given the principal movement
	expectDelegator := func(principal *big.Int) *big.Int {
		e := new(big.Int).Set(principal)
		if w == d {
			e.Add(e, paid)
		}
		return e
	}
	checkWithdrawAddr := func(rule string) {
		if w != d {
			if got := delta(a.Bank, b.Bank, w); got.Cmp(paid) != 0 {
				add(rule, "bank:"+w, "Δbank(withdraw address) = auto-withdrawn rewards "+paid.String(), "Δ = "+got.String(), "auto-withdrawn rewards did not reach the withdraw address")
			}
		}
	}

	switch enter.Method {
	case "delegate":
		if d == "" || !haveAmt {
			return nil
		}
		want := expectDelegator(new(big.Int).Neg(amt))
		if got := delta(a.Bank, b.Bank, d); got.Cmp(want) != 0 {
			add("R3-D", "bank:"+d, fmt.Sprintf("Δbank(delegator) = -%s + auto-withdrawn rewards %s = %s", amt, paid, want), "Δ = "+got.String(), "delegator balance does not match the delegated amount")
		}
		checkWithdrawAddr("R3-D")
		if got := delta(a.Deleg, b.Deleg, d+"|"+v); !closeTo(got, amt, 1) {
			add("R3-D", "deleg:"+d+"|"+v, "Δdelegation tokens = +"+amt.String(), "Δ = "+got.String(), "delegation did not grow by the delegated amount")
		}
		if got := delta(a.ValTokens, b.ValTokens, v); !closeTo(got, amt, 1) {
			add("R3-D", "valTokens:"+v, "Δvalidator tokens = +"+amt.String(), "Δ = "+got.String(), "validator tokens did not grow by the delegated amount")
		}
		if va, ok := a.Vesting[d]; ok {
			vb := b.Vesting[d]
			x := sum(va.DelegatedVesting, va.DelegatedFree)
			y := sum(vb.DelegatedVesting, vb.DelegatedFree)
			if got := new(big.Int).Sub(y, x); got.Cmp(amt) != 0 {
				add("R3-D", "vesting:"+d, "Δ(delegated vesting + delegated free) = +"+amt.String(), "Δ = "+got.String(), "vesting delegation tracking does not match the delegated amount")
			}
		}
	case "undelegate":
		if d == "" || !haveAmt {
			return nil
		}
		if got := delta(a.Deleg, b.Deleg, d+"|"+v); !closeTo(got, new(big.Int).Neg(amt), 1) {
			add("R3-U", "deleg:"+d+"|"+v, "Δdelegation tokens = -"+amt.String(), "Δ = "+got.String(), "delegation did not shrink by the undelegated amount")
		}
		if got := delta(a.Unbonding, b.Unbonding, d+"|"+v); !closeTo(got, amt, 1) {
			add("R3-U", "ubd:"+d+"|"+v, "Δunbonding = +"+amt.String(), "Δ = "+got.String(), "unbonding entry does not match the undelegated amount")
		}
		want := expectDelegator(new(big.Int))
		if got := delta(a.Bank, b.Bank, d); got.Cmp(want) != 0 {
			add("R3-U", "bank:"+d, "Δbank(delegator) = auto-withdrawn rewards only ("+want.String()+") before maturity", "Δ = "+got.String(), "delegator balance changed on undelegate beyond the auto-withdrawn rewards")
		}
		checkWithdrawAddr("R3-U")
	case "claimRewards", "withdrawDelegatorRewards":
		if d == "" {
			return nil
		}
		distr := strings.ToLower(a.Modules["distribution"])
		paid := delta(a.Bank, b.Bank, w)
		if distr != "" {
			if got := delta(a.Bank, b.Bank, distr); new(big.Int).Neg(got).Cmp(paid) != 0 {
				add("R3-C", "bank:"+w, "Δbank(withdraw address) = -Δbank(distribution module) = "+new(big.Int).Neg(got).String(), "Δ = "+paid.String(), "paid rewards are not balanced by the distribution module account")
			}
		}
		// claimable rewards must drop by at least what was paid (payout truncates decimals)
		before, after := rewardsOf(a, d, v), rewardsOf(b, d, v)
		drop := new(big.Rat).Sub(before, after)
		if drop.Cmp(new(big.Rat).SetInt(paid)) < 0 {
			add("R3-C", "rewards:"+d, "claimable rewards drop >= paid "+paid.String(), "drop = "+drop.FloatString(6), "rewards were paid without reducing claimable rewards")
		}
	}
	return out
}

// rewardsOf sums claimable rewards of delegator d (for validator v only, if v is set).
func rewardsOf(s *State, d, v string) *big.Rat {
	total := new(big.Rat)
	for k, val := range s.Rewards {
		parts := strings.SplitN(k, "|", 2)
		if len(parts) != 2 || parts[0] != d || (v != "" && parts[1] != v) {
			continue
		}
		if x, ok := new(big.Rat).SetString(numOrZero(val)); ok {
			total.Add(total, x)
		}
	}
	return total
}

// ---------------------------------------------------------------- A1: total supply (auxiliary)

func checkA1(recs []Record) []Finding {
	var out []Finding
	first := map[string]Record{}
	for _, r := range recs {
		if r.State == nil {
			continue
		}
		if _, ok := first[r.DB]; !ok {
			first[r.DB] = r
		}
		if r.Point != PointCommit && r.Point != PointTxAfter {
			continue
		}
		f := first[r.DB]
		if !numEqual(numOrZero(f.State.Supply), numOrZero(r.State.Supply)) {
			out = append(out, Finding{
				Rule: "A1", Severity: SevAux, Point: r.Point, Seq: r.Seq, DB: r.DB,
				Subject:  "supply",
				Expected: "unchanged unless explained by a mint/burn feature",
				Actual:   fmt.Sprintf("%s -> %s", f.State.Supply, r.State.Supply),
			})
		}
	}
	return out
}

// ---------------------------------------------------------------- helpers

func parseInt(s string) (*big.Int, bool) {
	if s == "" {
		return nil, false
	}
	x, ok := new(big.Int).SetString(s, 10)
	return x, ok
}

func numOrZero(s string) string {
	if s == "" {
		return "0"
	}
	return s
}

// numEqual compares integer or decimal strings numerically.
func numEqual(a, b string) bool {
	x, ok1 := new(big.Rat).SetString(a)
	y, ok2 := new(big.Rat).SetString(b)
	if !ok1 || !ok2 {
		return a == b
	}
	return x.Cmp(y) == 0
}

func closeTo(got, want *big.Int, tol int64) bool {
	d := new(big.Int).Sub(got, want)
	return d.Abs(d).Cmp(big.NewInt(tol)) <= 0
}

func sum(xs ...string) *big.Int {
	t := new(big.Int)
	for _, x := range xs {
		if v, ok := parseInt(numOrZero(x)); ok {
			t.Add(t, v)
		}
	}
	return t
}

func unionKeys(a, b map[string]string) []string {
	set := map[string]bool{}
	for k := range a {
		set[k] = true
	}
	for k := range b {
		set[k] = true
	}
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func short(addr string) string {
	if len(addr) > 10 {
		return "0x…" + addr[len(addr)-4:]
	}
	return addr
}
