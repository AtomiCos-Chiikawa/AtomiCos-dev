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
	Repeats  int      `json:"repeats,omitempty"` // other findings with the same rule, DB and subject that were folded into this one
	Seqs     []int    `json:"seqs,omitempty"`    // sequence numbers of every folded finding, this one included (only when Repeats > 0)
	Stack    []string `json:"stack,omitempty"`
}

var two256 = new(big.Int).Lsh(big.NewInt(1), 256)

// Verdicts of a trace. "inconclusive" means no violation was found but the trace cannot
// support a "no violation" verdict (see Result.Inconclusive).
const (
	VerdictViolation    = "violation"
	VerdictInconclusive = "inconclusive"
	VerdictNoViolation  = "no violation"
)

// Result is the outcome of judging one trace.
type Result struct {
	Findings []Finding `json:"findings"`
	Stats    Stats     `json:"stats"`
	// Inconclusive lists why this trace cannot support a "no violation" verdict; empty when it
	// can. The findings are valid either way, and a violation takes precedence.
	Inconclusive []string `json:"inconclusive"`
}

// Verdict is "violation" if any finding is a violation, otherwise "inconclusive" if the trace
// cannot support a verdict, otherwise "no violation".
func (r Result) Verdict() string {
	for _, f := range r.Findings {
		if f.Severity == SevViolation {
			return VerdictViolation
		}
	}
	if len(r.Inconclusive) > 0 {
		return VerdictInconclusive
	}
	return VerdictNoViolation
}

// Stats counts what the checker actually examined. A rule that examined nothing finds nothing,
// so "no findings" has to be read together with these numbers. A rule that does not apply to a
// trace (e.g. no O2 records, so R2-W and R3 evaluate 0 times) is not an error; it is only visible here.
type Stats struct {
	Records       int            `json:"records"`
	Points        map[string]int `json:"points"`        // records per observation point (unknown names included)
	UnknownPoints int            `json:"unknownPoints"` // records whose point is not an observation point of trace.go
	StateDBs      int            `json:"stateDBs"`      // distinct StateDB ids
	StateErrors   int            `json:"stateErrors"`   // records whose state carries state.error; such states are never used to judge
	Unparsable    int            `json:"unparsable"`    // sum of the rules' unparsable counters below
	O2            O2Stats        `json:"o2"`
	R1            R1Stats        `json:"r1"`
	R2A           R2AStats       `json:"r2a"`
	R2W           R2WStats       `json:"r2w"`
	R3            R3Stats        `json:"r3"`
	A1            A1Stats        `json:"a1"`
}

// O2Stats counts precompile entries and exits.
type O2Stats struct {
	Enters   int `json:"enters"`
	Exits    int `json:"exits"`
	Unclosed int `json:"unclosed"` // O2.enter with no O2.exit of the same StateDB and call by the end of the trace
}

// R1Stats counts what R1-S/R1-E examined.
type R1Stats struct {
	Reverts              int `json:"reverts"`              // O5.revert records
	NoSnapshot           int `json:"noSnapshot"`           // reverts with no earlier O5.snapshot of the same StateDB and id
	NoState              int `json:"noState"`              // reverts whose own or snapshot state is missing or has state.error
	Compared             int `json:"compared"`             // reverts compared with their snapshot
	WithCandidates       int `json:"withCandidates"`       // compared reverts that produced at least one candidate
	StateCandidates      int `json:"stateCandidates"`      // distinct (top-level StateDB, SDK key) candidates
	EventCandidates      int `json:"eventCandidates"`      // distinct (top-level StateDB, event signature) candidates
	Commits              int `json:"commits"`              // O6.commit records
	CommitsNested        int `json:"commitsNested"`        // not judged: nested StateDB (its candidates go to its top-level StateDB)
	CommitsFailed        int `json:"commitsFailed"`        // not judged: the commit returned an error
	CommitsNoState       int `json:"commitsNoState"`       // not judged: state missing or has state.error
	Judged               int `json:"judged"`               // successful top-level commits at which candidates were checked
	JudgedWithCandidates int `json:"judgedWithCandidates"` // ... that had at least one candidate to check
	Discarded            int `json:"discarded"`            // candidates not followed by a judged commit of their top-level StateDB
	Unparsable           int `json:"unparsable"`           // numeric values that could not be parsed (compared as strings)
}

// R2AStats counts the O4 balance changes R2-A examined.
type R2AStats struct {
	Ops        int `json:"ops"`        // O4.balance records
	Checked    int `json:"checked"`    // add/sub with a parsable before and amount
	NonZero    int `json:"nonZero"`    // checked ops with a non-zero amount
	Set        int `json:"set"`        // set: not an arithmetic change, not checked
	OtherOp    int `json:"otherOp"`    // unknown op, not checked
	Unparsable int `json:"unparsable"` // add/sub whose before or amount could not be parsed (not checked)
}

// R2WStats counts the write-back comparisons R2-W made at the synchronisation points.
type R2WStats struct {
	SyncEnter  int `json:"syncEnter"`  // O2.enter records with a usable state
	SyncExit   int `json:"syncExit"`   // successful O2.exit records with a usable state
	FailedExit int `json:"failedExit"` // O2.exit with an error: not a synchronisation point
	NoState    int `json:"noState"`    // O2 records at a synchronisation point whose state is missing or has state.error
	Enter      int `json:"enter"`      // account comparisons at O2.enter
	Exit       int `json:"exit"`       // account comparisons at O2.exit
	Blocked    int `json:"blocked"`    // accounts skipped as blocked addresses
	Unparsable int `json:"unparsable"` // accounts whose write-back or bank reference could not be parsed (not compared)
}

// R3Stats counts the precompile calls R3 examined.
type R3Stats struct {
	Exits      int     `json:"exits"`      // O2.exit records
	Evaluated  R3Evals `json:"evaluated"`  // successful calls judged, per sub-rule
	NotCovered int     `json:"notCovered"` // successful calls of a method R3 has no formula for
	Skipped    R3Skips `json:"skipped"`
}

// R3Evals counts the calls judged per sub-rule.
type R3Evals struct {
	D int `json:"R3-D"`
	U int `json:"R3-U"`
	C int `json:"R3-C"`
}

// R3Skips counts the calls R3 could not judge, by reason. MissingModule is not a skip of the
// call: the call is judged, but the R3-C module-balance check is skipped and R3-D/U take the
// auto-withdrawn rewards as 0.
type R3Skips struct {
	CallError     int `json:"callError"`     // the call exited with an error (left to R1)
	NoEnter       int `json:"noEnter"`       // O2.exit without a matching O2.enter
	NoState       int `json:"noState"`       // enter or exit state missing or has state.error
	MissingArgs   int `json:"missingArgs"`   // delegator or amount argument absent
	Unparsable    int `json:"unparsable"`    // a number the formulas need could not be parsed
	MissingModule int `json:"missingModule"` // no distribution module address in the enter state
}

// A1Stats counts the total-supply comparisons.
type A1Stats struct {
	Compared   int `json:"compared"`   // commit / O1.after states compared with the StateDB's first state
	Unparsable int `json:"unparsable"` // comparisons with an unparsable supply (compared as strings)
}

// conditions that make a trace inconclusive; eval.first keeps the lowest seq of each
const (
	condStateError = "stateError"
	condUnparsable = "unparsable"
	condUnclosed   = "unclosed"
	condNoSnapshot = "noSnapshot"
)

// eval carries the statistics while the rules run.
type eval struct {
	st    Stats
	first map[string]int
}

func (e *eval) mark(cond string, seq int) {
	if cur, ok := e.first[cond]; !ok || seq < cur {
		e.first[cond] = seq
	}
}

// unparsable adds n to a rule's counter and remembers where it happened.
func (e *eval) unparsable(counter *int, n, seq int) {
	if n > 0 {
		*counter += n
		e.mark(condUnparsable, seq)
	}
}

var knownPoints = map[string]bool{
	PointSnapshot: true, PointRevert: true, PointBalance: true, PointPCEnter: true,
	PointPCExit: true, PointCommit: true, PointTxBefore: true, PointTxAfter: true,
}

// usable reports whether a record's state may be used to judge. A state with state.error is
// partial (the harness could not read all of it), and a missing entry would read as 0.
func usable(r Record) bool {
	return r.State != nil && r.State.Error == ""
}

// Check applies every rule to the trace and returns the findings ordered by sequence number.
// Evaluate also reports what was examined and whether the trace supports a verdict.
func Check(recs []Record) []Finding {
	return Evaluate(recs).Findings
}

// Evaluate applies every rule to the trace.
func Evaluate(recs []Record) Result {
	e := &eval{first: map[string]int{}}
	e.survey(recs)
	var all []Finding
	all = append(all, checkR1(recs, e)...)
	all = append(all, checkR2A(recs, e)...)
	all = append(all, checkR2W(recs, e)...)
	all = append(all, checkR3(recs, e)...)
	all = append(all, checkA1(recs, e)...)
	sort.SliceStable(all, func(i, j int) bool { return all[i].Seq < all[j].Seq })
	st := &e.st
	st.Unparsable = st.R1.Unparsable + st.R2A.Unparsable + st.R2W.Unparsable + st.R3.Skipped.Unparsable + st.A1.Unparsable
	return Result{Findings: fold(all), Stats: e.st, Inconclusive: e.inconclusive()}
}

// survey counts records, points, StateDBs, state errors and unclosed precompile calls.
func (e *eval) survey(recs []Record) {
	st := &e.st
	st.Records = len(recs)
	st.Points = map[string]int{}
	dbs := map[string]bool{}
	open := map[callKey]int{}
	for _, r := range recs {
		st.Points[r.Point]++
		if !knownPoints[r.Point] {
			st.UnknownPoints++
		}
		dbs[r.DB] = true
		if r.State != nil && r.State.Error != "" {
			st.StateErrors++
			e.mark(condStateError, r.Seq)
		}
		switch r.Point {
		case PointPCEnter:
			st.O2.Enters++
			open[callKey{r.DB, r.Call}] = r.Seq
		case PointPCExit:
			st.O2.Exits++
			delete(open, callKey{r.DB, r.Call})
		}
	}
	st.StateDBs = len(dbs)
	st.O2.Unclosed = len(open)
	for _, seq := range open {
		e.mark(condUnclosed, seq)
	}
}

// inconclusive lists why the trace cannot support a "no violation" verdict.
func (e *eval) inconclusive() []string {
	st := &e.st
	out := []string{}
	switch {
	case st.Records == 0:
		out = append(out, "the trace has no records")
	case st.UnknownPoints == st.Records:
		out = append(out, fmt.Sprintf("none of the %d records is at a known observation point", st.Records))
	}
	if st.StateErrors > 0 {
		out = append(out, fmt.Sprintf("%d record(s) carry state.error (first at seq %d); those states were not used to judge",
			st.StateErrors, e.first[condStateError]))
	}
	if st.Unparsable > 0 {
		out = append(out, fmt.Sprintf("a rule could not parse a number it needed %d time(s) (first at seq %d; R1 %d, R2-A %d, R2-W %d, R3 %d, A1 %d)",
			st.Unparsable, e.first[condUnparsable], st.R1.Unparsable, st.R2A.Unparsable, st.R2W.Unparsable, st.R3.Skipped.Unparsable, st.A1.Unparsable))
	}
	if st.O2.Unclosed > 0 {
		out = append(out, fmt.Sprintf("%d O2.enter record(s) have no O2.exit by the end of the trace (first at seq %d); the trace may be truncated",
			st.O2.Unclosed, e.first[condUnclosed]))
	}
	if st.R1.NoSnapshot > 0 {
		out = append(out, fmt.Sprintf("%d O5.revert record(s) have no matching O5.snapshot (first at seq %d)",
			st.R1.NoSnapshot, e.first[condNoSnapshot]))
	}
	return out
}

func sevRank(s string) int {
	switch s {
	case SevViolation:
		return 3
	case SevGuarded:
		return 2
	case SevAux:
		return 1
	}
	return 0
}

// fold merges findings with the same rule, DB and subject into one. The merged finding is the
// first one of the highest severity (violation > guarded > aux), so a later violation is never
// hidden behind an earlier guarded finding. Repeats counts the findings folded into it and
// Seqs lists the sequence numbers of all of them.
func fold(in []Finding) []Finding {
	type key struct{ rule, db, subject string }
	idx := map[key]int{}
	var out []Finding
	for _, f := range in {
		k := key{f.Rule, f.DB, f.Subject}
		i, ok := idx[k]
		if !ok {
			idx[k] = len(out)
			out = append(out, f)
			continue
		}
		cur := &out[i]
		seqs := cur.Seqs
		if seqs == nil {
			seqs = []int{cur.Seq}
		}
		seqs = append(seqs, f.Seq)
		repeats := cur.Repeats + 1
		if sevRank(f.Severity) > sevRank(cur.Severity) {
			*cur = f
		}
		cur.Repeats, cur.Seqs = repeats, seqs
	}
	// a finding replaced by a later, more severe one carries the later seq
	sort.SliceStable(out, func(i, j int) bool { return out[i].Seq < out[j].Seq })
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
//
// States carrying state.error are treated as absent: they are never compared, judged or used
// as the event baseline.

type snapKey struct {
	db   string
	snap int
}

type leftover struct {
	snapVal, revVal string
	seq             int
}

func checkR1(recs []Record, e *eval) []Finding {
	st := &e.st.R1
	var out []Finding
	snaps := map[snapKey]Record{}
	writes := map[string]map[string]leftover{} // top-level db -> state key -> leftover
	events := map[string]map[string]int{}      // top-level db -> event sig -> count
	firstEv := map[string]map[string]int{}
	base := map[string]map[string]int{}
	parent := map[string]string{}
	pending := map[string]map[string]bool{} // top-level db -> candidates not yet checked at a judged commit (stats only)
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
		if usable(r) {
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
			st.Reverts++
			s, ok := snaps[snapKey{r.DB, r.Snap}]
			if !ok {
				st.NoSnapshot++
				e.mark(condNoSnapshot, r.Seq)
				continue
			}
			if !usable(s) || !usable(r) {
				st.NoState++
				continue
			}
			st.Compared++
			top := root(r.DB)
			if writes[top] == nil {
				writes[top], events[top], firstEv[top] = map[string]leftover{}, map[string]int{}, map[string]int{}
			}
			if pending[top] == nil {
				pending[top] = map[string]bool{}
			}
			diffs, bad := diffSDK(s.State, r.State)
			e.unparsable(&st.Unparsable, bad, r.Seq)
			produced := len(diffs) > 0
			for _, d := range diffs {
				lo, ok := writes[top][d.key]
				if !ok {
					lo = leftover{snapVal: d.before, seq: r.Seq}
				}
				lo.revVal = d.after
				writes[top][d.key] = lo
				pending[top]["state "+d.key] = true
			}
			before := multiset(append(append([]string{}, s.State.Events.CtxSigs...), s.State.Events.CacheSigs...))
			after := multiset(append(append([]string{}, r.State.Events.CtxSigs...), r.State.Events.CacheSigs...))
			for sig, n := range after {
				if extra := n - before[sig]; extra > 0 {
					events[top][sig] += extra
					if _, ok := firstEv[top][sig]; !ok {
						firstEv[top][sig] = r.Seq
					}
					pending[top]["event "+sig] = true
					produced = true
				}
			}
			if produced {
				st.WithCandidates++
			}
		case PointCommit:
			st.Commits++
			switch {
			case parent[r.DB] != "":
				st.CommitsNested++
				continue
			case r.Err != "":
				st.CommitsFailed++
				continue
			case !usable(r):
				st.CommitsNoState++
				continue
			}
			st.Judged++
			if len(writes[r.DB])+len(events[r.DB]) > 0 {
				st.JudgedWithCandidates++
			}
			delete(pending, r.DB)
			for _, k := range sortedKeys(writes[r.DB]) {
				lo := writes[r.DB][k]
				got := valueOf(r.State, k)
				e.unparsable(&st.Unparsable, notNum(got), r.Seq)
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
	for top, w := range writes {
		st.StateCandidates += len(w)
		st.EventCandidates += len(events[top])
	}
	for _, p := range pending {
		st.Discarded += len(p)
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
// unparsable counts the compared values that are not numbers (they are compared as strings).
func diffSDK(a, b *State) (out []diff, unparsable int) {
	cmp := func(prefix string, x, y map[string]string) {
		for _, k := range unionKeys(x, y) {
			vx, vy := numOrZero(x[k]), numOrZero(y[k])
			unparsable += notNum(vx) + notNum(vy)
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
	sa, sb := numOrZero(a.Supply), numOrZero(b.Supply)
	unparsable += notNum(sa) + notNum(sb)
	if !numEqual(sa, sb) {
		out = append(out, diff{"supply", a.Supply, b.Supply})
	}
	return out, unparsable
}

// ---------------------------------------------------------------- R2-A: arithmetic validity

func checkR2A(recs []Record, e *eval) []Finding {
	st := &e.st.R2A
	var out []Finding
	for i, r := range recs {
		if r.Point != PointBalance {
			continue
		}
		st.Ops++
		switch r.Op {
		case "add", "sub":
		case "set":
			st.Set++
			continue
		default:
			st.OtherOp++
			continue
		}
		before, ok1 := parseInt(r.Before)
		amount, ok2 := parseInt(r.Amount)
		if !ok1 || !ok2 {
			e.unparsable(&st.Unparsable, 1, r.Seq)
			continue
		}
		st.Checked++
		if amount.Sign() != 0 {
			st.NonZero++
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

func checkR2W(recs []Record, e *eval) []Finding {
	st := &e.st.R2W
	var out []Finding
	for _, r := range recs {
		if r.Point != PointPCEnter && r.Point != PointPCExit {
			continue
		}
		if r.Point == PointPCExit && r.Err != "" {
			st.FailedExit++
			continue
		}
		if !usable(r) {
			st.NoState++
			continue
		}
		enter := r.Point == PointPCEnter
		if enter {
			st.SyncEnter++
		} else {
			st.SyncExit++
		}
		blocked := map[string]bool{}
		for _, b := range r.State.Blocked {
			blocked[strings.ToLower(b)] = true
		}
		for _, addr := range sortedKeys(r.State.StateDB) {
			if blocked[addr] {
				st.Blocked++
				continue
			}
			a := r.State.StateDB[addr]
			wb, ok1 := parseInt(a.WriteBack)
			ref, ok2 := parseInt(a.BankRef)
			if !ok1 || !ok2 {
				e.unparsable(&st.Unparsable, 1, r.Seq)
				continue
			}
			if enter {
				st.Enter++
			} else {
				st.Exit++
			}
			if wb.Cmp(ref) == 0 {
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

func checkR3(recs []Record, e *eval) []Finding {
	st := &e.st.R3
	var out []Finding
	enters := map[callKey]Record{}
	for _, r := range recs {
		switch r.Point {
		case PointPCEnter:
			enters[callKey{r.DB, r.Call}] = r
		case PointPCExit:
			st.Exits++
			en, ok := enters[callKey{r.DB, r.Call}]
			switch {
			case !ok:
				st.Skipped.NoEnter++
				continue
			case r.Err != "":
				st.Skipped.CallError++
				continue
			case !usable(en) || !usable(r):
				st.Skipped.NoState++
				continue
			}
			fs, handled, noModule := r3For(en, r)
			if noModule {
				st.Skipped.MissingModule++
			}
			switch handled {
			case "R3-D":
				st.Evaluated.D++
			case "R3-U":
				st.Evaluated.U++
			case "R3-C":
				st.Evaluated.C++
			case r3NotCovered:
				st.NotCovered++
			case r3MissingArgs:
				st.Skipped.MissingArgs++
			case r3Unparsable:
				e.unparsable(&st.Skipped.Unparsable, 1, r.Seq)
			}
			out = append(out, fs...)
		}
	}
	return out
}

// how r3For handled a call, besides the sub-rule it judged it under
const (
	r3NotCovered  = "not covered"
	r3MissingArgs = "missing arguments"
	r3Unparsable  = "unparsable number"
)

// r3For judges one successful precompile call. handled is the sub-rule the call was judged
// under ("R3-D", "R3-U", "R3-C") or why it was not; noModule reports that the enter state has
// no distribution module address. A number the formulas need that does not parse makes the
// whole call a skip (no findings), never a panic or a silent 0.
func r3For(enter, exit Record) (out []Finding, handled string, noModule bool) {
	var rule string
	switch enter.Method {
	case "delegate":
		rule = "R3-D"
	case "undelegate":
		rule = "R3-U"
	case "claimRewards", "withdrawDelegatorRewards":
		rule = "R3-C"
	default:
		return nil, r3NotCovered, false
	}
	a, b := enter.State, exit.State
	add := func(rule, subject, expected, actual, detail string) {
		out = append(out, Finding{
			Rule: rule, Severity: SevViolation, Point: PointPCExit,
			Seq: exit.Seq, DB: exit.DB, Call: exit.Call, Subject: subject,
			Expected: expected, Actual: actual,
			Detail: fmt.Sprintf("%s.%s returned success; %s", short(enter.Precompile), enter.Method, detail),
		})
	}
	// bad is set when a number the formulas read does not parse; the call is then skipped.
	bad := false
	num := func(s string) *big.Int {
		x, ok := parseInt(numOrZero(s))
		if !ok {
			bad = true
			return new(big.Int)
		}
		return x
	}
	delta := func(m1, m2 map[string]string, k string) *big.Int {
		x := num(m1[k])
		y := num(m2[k])
		return new(big.Int).Sub(y, x)
	}
	d := strings.ToLower(enter.Args["delegatorAddress"])
	v := enter.Args["validatorAddress"]
	if d == "" {
		return nil, r3MissingArgs, false
	}
	var amt *big.Int
	if rule != "R3-C" {
		raw := enter.Args["amount"]
		if raw == "" {
			return nil, r3MissingArgs, false
		}
		x, ok := parseInt(raw)
		if !ok {
			return nil, r3Unparsable, false
		}
		amt = x
	}
	// Changing a delegation first withdraws the rewards accrued on it (x/distribution hook
	// BeforeDelegationSharesModified -> withdrawDelegationRewards), paid from the distribution
	// module account to the delegator's withdraw address. paid is that amount (0 if none).
	w := strings.ToLower(a.Withdraw[d])
	if w == "" {
		w = d
	}
	paid := new(big.Int)
	distr := strings.ToLower(a.Modules["distribution"])
	noModule = distr == ""
	if distr != "" && rule != "R3-C" {
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

	switch rule {
	case "R3-D":
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
			x := new(big.Int).Add(num(va.DelegatedVesting), num(va.DelegatedFree))
			y := new(big.Int).Add(num(vb.DelegatedVesting), num(vb.DelegatedFree))
			if got := new(big.Int).Sub(y, x); got.Cmp(amt) != 0 {
				add("R3-D", "vesting:"+d, "Δ(delegated vesting + delegated free) = +"+amt.String(), "Δ = "+got.String(), "vesting delegation tracking does not match the delegated amount")
			}
		}
	case "R3-U":
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
	case "R3-C":
		paid := delta(a.Bank, b.Bank, w)
		if distr != "" {
			if got := delta(a.Bank, b.Bank, distr); new(big.Int).Neg(got).Cmp(paid) != 0 {
				add("R3-C", "bank:"+w, "Δbank(withdraw address) = -Δbank(distribution module) = "+new(big.Int).Neg(got).String(), "Δ = "+paid.String(), "paid rewards are not balanced by the distribution module account")
			}
		}
		// claimable rewards must drop by at least what was paid (payout truncates decimals)
		before, ok1 := rewardsOf(a, d, v)
		after, ok2 := rewardsOf(b, d, v)
		if !ok1 || !ok2 {
			bad = true
		}
		drop := new(big.Rat).Sub(before, after)
		if drop.Cmp(new(big.Rat).SetInt(paid)) < 0 {
			add("R3-C", "rewards:"+d, "claimable rewards drop >= paid "+paid.String(), "drop = "+drop.FloatString(6), "rewards were paid without reducing claimable rewards")
		}
	}
	if bad {
		return nil, r3Unparsable, noModule
	}
	return out, rule, noModule
}

// rewardsOf sums claimable rewards of delegator d (for validator v only, if v is set).
// ok is false if one of the summed values is not a number.
func rewardsOf(s *State, d, v string) (total *big.Rat, ok bool) {
	total, ok = new(big.Rat), true
	for k, val := range s.Rewards {
		parts := strings.SplitN(k, "|", 2)
		if len(parts) != 2 || parts[0] != d || (v != "" && parts[1] != v) {
			continue
		}
		x, good := new(big.Rat).SetString(numOrZero(val))
		if !good {
			ok = false
			continue
		}
		total.Add(total, x)
	}
	return total, ok
}

// ---------------------------------------------------------------- A1: total supply (auxiliary)

func checkA1(recs []Record, e *eval) []Finding {
	st := &e.st.A1
	var out []Finding
	first := map[string]Record{}
	for _, r := range recs {
		if !usable(r) {
			continue
		}
		if _, ok := first[r.DB]; !ok {
			first[r.DB] = r
		}
		if r.Point != PointCommit && r.Point != PointTxAfter {
			continue
		}
		f := first[r.DB]
		x, y := numOrZero(f.State.Supply), numOrZero(r.State.Supply)
		st.Compared++
		if notNum(x)+notNum(y) > 0 {
			e.unparsable(&st.Unparsable, 1, r.Seq)
		}
		if !numEqual(x, y) {
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

// notNum is 1 if s is not an integer or decimal string, else 0.
func notNum(s string) int {
	if _, ok := new(big.Rat).SetString(s); ok {
		return 0
	}
	return 1
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
