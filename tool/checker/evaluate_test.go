package checker

import (
	"reflect"
	"strings"
	"testing"
)

// Synthetic traces only (see checker_test.go for the helpers). These tests cover what the
// checker examined (Stats), the inconclusive outcome, state.error and severity-aware folding.

func expectVerdict(t *testing.T, res Result, want string) {
	t.Helper()
	if got := res.Verdict(); got != want {
		t.Fatalf("verdict = %q, want %q\ninconclusive: %q\nfindings: %+v", got, want, res.Inconclusive, res.Findings)
	}
}

func expectReason(t *testing.T, res Result, substrs ...string) {
	t.Helper()
	all := strings.Join(res.Inconclusive, "\n")
	for _, s := range substrs {
		if !strings.Contains(all, s) {
			t.Fatalf("inconclusive reasons %q do not mention %q", res.Inconclusive, s)
		}
	}
}

// ---------------------------------------------------------------- inconclusive

func TestInconclusive_EmptyTrace(t *testing.T) {
	for _, recs := range [][]Record{nil, {}} {
		res := Evaluate(recs)
		expectVerdict(t, res, VerdictInconclusive)
		expectReason(t, res, "no records")
		if res.Stats.Records != 0 || len(res.Findings) != 0 {
			t.Fatalf("res = %+v", res)
		}
	}
}

func TestInconclusive_OnlyUnknownPoints(t *testing.T) {
	recs := []Record{
		{Seq: 1, Point: "O3.enter", DB: "db1"},
		{Seq: 2, Point: "O2.Enter", DB: "db1", State: st(map[string]string{alice: "1"})}, // wrong case: not O2.enter
	}
	res := Evaluate(recs)
	expectVerdict(t, res, VerdictInconclusive)
	expectReason(t, res, "none of the 2 records is at a known observation point")
	if res.Stats.UnknownPoints != 2 || res.Stats.Points["O3.enter"] != 1 || res.Stats.Points["O2.Enter"] != 1 {
		t.Fatalf("stats = %+v", res.Stats)
	}
}

// A rule that has nothing to look at is not a reason to be inconclusive: a v0.1.x-like trace
// (no O2 records) is judged by R1 and R2-A only, and that is visible in the stats.
func TestInconclusive_NotApplicableIsNot(t *testing.T) {
	recs := []Record{
		{Seq: 1, Point: PointSnapshot, DB: "db1", Snap: 1, State: st(map[string]string{alice: "100"})},
		{Seq: 2, Point: PointBalance, DB: "db1", Op: "sub", Addr: alice, Before: "100", Amount: "30"},
		{Seq: 3, Point: PointRevert, DB: "db1", Snap: 1, State: st(map[string]string{alice: "100"})},
		{Seq: 4, Point: PointCommit, DB: "db1", State: st(map[string]string{alice: "100"})},
	}
	res := Evaluate(recs)
	expectVerdict(t, res, VerdictNoViolation)
	s := res.Stats
	if len(res.Inconclusive) != 0 || s.R2W.Enter+s.R2W.Exit != 0 || s.R3.Evaluated != (R3Evals{}) ||
		s.R1.Compared != 1 || s.R1.Judged != 1 || s.R2A.Checked != 1 {
		t.Fatalf("inconclusive=%q stats=%+v", res.Inconclusive, s)
	}
}

// Candidates that never meet a successful commit are counted, not a reason to be inconclusive.
func TestInconclusive_DiscardedCandidatesAreOnlyCounted(t *testing.T) {
	recs := []Record{
		{Seq: 1, Point: PointSnapshot, DB: "db1", Snap: 1, State: evState([]string{"tx:a"}, nil)},
		{Seq: 2, Point: PointRevert, DB: "db1", Snap: 1, State: evState([]string{"tx:a"}, []string{"coin_received:x"})},
		{Seq: 3, Point: PointCommit, DB: "db1", Err: "panic: x", State: evState([]string{"tx:a", "coin_received:x"}, nil)},
	}
	res := Evaluate(recs)
	expectVerdict(t, res, VerdictNoViolation)
	r1 := res.Stats.R1
	if r1.EventCandidates != 1 || r1.Discarded != 1 || r1.CommitsFailed != 1 || r1.Judged != 0 {
		t.Fatalf("r1 = %+v", r1)
	}
}

func TestInconclusive_RevertWithoutSnapshot(t *testing.T) {
	recs := []Record{
		{Seq: 1, Point: PointRevert, DB: "db1", Snap: 4, State: st(map[string]string{alice: "100"})},
		{Seq: 2, Point: PointCommit, DB: "db1", State: st(map[string]string{alice: "100"})},
	}
	res := Evaluate(recs)
	expectVerdict(t, res, VerdictInconclusive)
	expectReason(t, res, "1 O5.revert record(s) have no matching O5.snapshot (first at seq 1)")
	// a snapshot of another StateDB with the same id does not match
	recs = append([]Record{{Seq: 0, Point: PointSnapshot, DB: "db2", Snap: 4, State: st(nil)}}, recs...)
	expectVerdict(t, Evaluate(recs), VerdictInconclusive)
}

func TestInconclusive_UnclosedEnter(t *testing.T) {
	recs := []Record{
		{Seq: 1, Point: PointPCEnter, DB: "db1", Call: 1, State: st(nil)},
		{Seq: 2, Point: PointPCEnter, DB: "db1", Call: 2, State: st(nil)},
		{Seq: 3, Point: PointPCExit, DB: "db1", Call: 2, State: st(nil)},
		{Seq: 4, Point: PointPCEnter, DB: "db2", Call: 1, State: st(nil)}, // same call id, other StateDB
	}
	res := Evaluate(recs)
	expectVerdict(t, res, VerdictInconclusive)
	expectReason(t, res, "2 O2.enter record(s) have no O2.exit", "first at seq 1")
	if o2 := res.Stats.O2; o2 != (O2Stats{Enters: 3, Exits: 1, Unclosed: 2}) {
		t.Fatalf("o2 = %+v", o2)
	}
}

func TestInconclusive_UnparsableNumbers(t *testing.T) {
	cases := map[string][]Record{
		"R2-A before": {{Seq: 7, Point: PointBalance, DB: "db1", Op: "sub", Addr: alice, Before: "1O", Amount: "1"}},
		"R2-A amount": {{Seq: 7, Point: PointBalance, DB: "db1", Op: "add", Addr: alice, Before: "1"}},
		"R2-W": func() []Record {
			s := st(nil)
			s.StateDB = map[string]StateDBRec{alice: {WriteBack: "x", BankRef: "1"}}
			return []Record{{Seq: 7, Point: PointPCEnter, DB: "db1", Call: 1, State: s}, {Seq: 8, Point: PointPCExit, DB: "db1", Call: 1, State: st(nil)}}
		}(),
		"R1": {
			{Seq: 6, Point: PointSnapshot, DB: "db1", Snap: 1, State: st(map[string]string{alice: "100"})},
			{Seq: 7, Point: PointRevert, DB: "db1", Snap: 1, State: st(map[string]string{alice: "n/a"})},
		},
		"A1": func() []Record {
			s := st(nil)
			s.Supply = "?"
			return []Record{{Seq: 6, Point: PointSnapshot, DB: "db1", Snap: 1, State: st(nil)}, {Seq: 7, Point: PointCommit, DB: "db1", State: s}}
		}(),
	}
	for name, recs := range cases {
		res := Evaluate(recs)
		if res.Stats.Unparsable == 0 || res.Verdict() == VerdictNoViolation {
			t.Errorf("%s: unparsable=%d verdict=%q", name, res.Stats.Unparsable, res.Verdict())
			continue
		}
		expectReason(t, res, "could not parse", "first at seq 7")
	}
}

// A violation is still a violation when the trace is also inconclusive; the reasons are kept.
func TestInconclusive_ViolationTakesPrecedence(t *testing.T) {
	recs := []Record{
		{Seq: 1, Point: PointSnapshot, DB: "db1", Snap: 3, State: st(map[string]string{alice: "100"})},
		{Seq: 2, Point: PointRevert, DB: "db1", Snap: 3, State: st(map[string]string{alice: "130"})},
		{Seq: 3, Point: PointCommit, DB: "db1", State: st(map[string]string{alice: "130"})},
		{Seq: 4, Point: PointSnapshot, DB: "db1", Snap: 4, State: &State{Error: "atomicos: read failed"}},
	}
	res := Evaluate(recs)
	expectVerdict(t, res, VerdictViolation)
	expectReason(t, res, "state.error")
}

// ---------------------------------------------------------------- state.error

// A failed read at commit has an empty bank map, which reads as 0. If the reverted frame left 0
// behind, the old checker reported R1-S on that missing data.
func TestStateError_CommitIsNotJudged(t *testing.T) {
	trace := func(commit *State) []Record {
		return []Record{
			{Seq: 1, Point: PointSnapshot, DB: "db1", Snap: 1, State: st(map[string]string{alice: "100"})},
			{Seq: 2, Point: PointRevert, DB: "db1", Snap: 1, State: st(map[string]string{alice: "0"})},
			{Seq: 3, Point: PointCommit, DB: "db1", State: commit},
		}
	}
	// control: the leftover really committed
	expectRules(t, Check(trace(st(map[string]string{alice: "0"}))), SevViolation, "R1-S")

	res := Evaluate(trace(&State{Error: "atomicos: read failed: boom", Events: EventsRec{Cache: -1}}))
	expectRules(t, res.Findings, "")
	expectVerdict(t, res, VerdictInconclusive)
	expectReason(t, res, "1 record(s) carry state.error (first at seq 3)")
	if r1 := res.Stats.R1; r1.CommitsNoState != 1 || r1.Judged != 0 || r1.Discarded != 1 || res.Stats.StateErrors != 1 {
		t.Fatalf("stats = %+v", res.Stats)
	}
}

// A failed read at a revert is not compared with the snapshot (its missing entries would look
// like leftovers of the reverted frame).
func TestStateError_RevertIsNotCompared(t *testing.T) {
	recs := []Record{
		{Seq: 1, Point: PointSnapshot, DB: "db1", Snap: 1, State: st(map[string]string{alice: "100"})},
		{Seq: 2, Point: PointRevert, DB: "db1", Snap: 1, State: &State{Error: "atomicos: validators: x", Supply: "1000"}},
		{Seq: 3, Point: PointCommit, DB: "db1", State: st(map[string]string{alice: "0"})},
	}
	res := Evaluate(recs)
	expectRules(t, res.Findings, "")
	expectVerdict(t, res, VerdictInconclusive)
	if r1 := res.Stats.R1; r1.NoState != 1 || r1.Compared != 0 || r1.StateCandidates != 0 {
		t.Fatalf("r1 = %+v", r1)
	}
}

// R2-W, R3 and A1 do not use a state with state.error either.
func TestStateError_O2IsNotUsed(t *testing.T) {
	recs := delegateTrace("80", "20", "1020") // violates R3-D three times
	recs[1].State.Error = "atomicos: delegations: x"
	recs[1].State.StateDB = map[string]StateDBRec{alice: {WriteBack: "1", BankRef: "2"}} // would violate R2-W
	recs[1].State.Supply = "1"                                                           // would trigger A1 at a commit
	recs = append(recs, Record{Seq: 3, Point: PointCommit, DB: "db1", State: recs[1].State})
	res := Evaluate(recs)
	expectRules(t, res.Findings, "")
	expectVerdict(t, res, VerdictInconclusive)
	s := res.Stats
	if s.R3.Skipped.NoState != 1 || s.R2W.NoState != 1 || s.R2W.Exit != 0 || s.A1.Compared != 0 || s.StateErrors != 2 {
		t.Fatalf("stats = %+v", s)
	}
}

// ---------------------------------------------------------------- fold

func TestFold_KeepsHighestSeverityAndAllSeqs(t *testing.T) {
	in := []Finding{
		{Rule: "R2-A", Severity: SevGuarded, Seq: 1, DB: "db1", Subject: alice, Outcome: "rejected (panic: x)"},
		{Rule: "R2-W", Severity: SevViolation, Seq: 2, DB: "db1", Subject: alice},
		{Rule: "R2-A", Severity: SevViolation, Seq: 3, DB: "db1", Subject: alice, Outcome: "applied"},
		{Rule: "R2-A", Severity: SevGuarded, Seq: 4, DB: "db1", Subject: alice, Outcome: "rejected (panic: y)"},
		{Rule: "R2-A", Severity: SevViolation, Seq: 5, DB: "db1", Subject: alice, Outcome: "applied"},
	}
	out := fold(in)
	want := []Finding{
		{Rule: "R2-W", Severity: SevViolation, Seq: 2, DB: "db1", Subject: alice},
		{Rule: "R2-A", Severity: SevViolation, Seq: 3, DB: "db1", Subject: alice, Outcome: "applied", Repeats: 3, Seqs: []int{1, 3, 4, 5}},
	}
	if !reflect.DeepEqual(out, want) {
		t.Fatalf("fold =\n%+v\nwant\n%+v", out, want)
	}
	// same severity: the first finding is kept, as before
	same := fold([]Finding{
		{Rule: "R2-W", Severity: SevViolation, Seq: 1, DB: "db1", Subject: alice, Actual: "first"},
		{Rule: "R2-W", Severity: SevViolation, Seq: 2, DB: "db1", Subject: alice, Actual: "second"},
	})
	if len(same) != 1 || same[0].Actual != "first" || same[0].Repeats != 1 || !reflect.DeepEqual(same[0].Seqs, []int{1, 2}) {
		t.Fatalf("fold = %+v", same)
	}
}

func underflowCalls(firstErr, secondErr string) []Record {
	return []Record{
		{Seq: 1, Point: PointPCEnter, DB: "db1", Call: 1},
		{Seq: 2, Point: PointBalance, DB: "db1", Call: 1, Op: "sub", Addr: alice, Before: "10", Amount: "25"},
		{Seq: 3, Point: PointPCExit, DB: "db1", Call: 1, Err: firstErr},
		{Seq: 4, Point: PointPCEnter, DB: "db1", Call: 2},
		{Seq: 5, Point: PointBalance, DB: "db1", Call: 2, Op: "sub", Addr: alice, Before: "10", Amount: "25"},
		{Seq: 6, Point: PointPCExit, DB: "db1", Call: 2, Err: secondErr},
	}
}

// A rejected attempt followed by an applied underflow on the same account is a violation.
// The old fold kept the first (guarded) finding and the trace passed.
func TestFold_GuardedThenViolation(t *testing.T) {
	res := Evaluate(underflowCalls("panic: balance underflow", ""))
	expectRules(t, res.Findings, "", "R2-A")
	f := res.Findings[0]
	if f.Severity != SevViolation || f.Seq != 5 || f.Outcome != "applied" || f.Repeats != 1 || !reflect.DeepEqual(f.Seqs, []int{2, 5}) {
		t.Fatalf("finding = %+v", f)
	}
	expectVerdict(t, res, VerdictViolation)

	res = Evaluate(underflowCalls("", "panic: balance underflow"))
	f = res.Findings[0]
	if len(res.Findings) != 1 || f.Severity != SevViolation || f.Seq != 2 || !reflect.DeepEqual(f.Seqs, []int{2, 5}) {
		t.Fatalf("findings = %+v", res.Findings)
	}
	// both rejected: still guarded, still one finding
	res = Evaluate(underflowCalls("panic: a", "panic: b"))
	expectRules(t, res.Findings, SevGuarded, "R2-A")
	expectVerdict(t, res, VerdictNoViolation)
}

// ---------------------------------------------------------------- R3 robustness

// Before: x, _ := parseInt(...) returned nil and big.Int arithmetic on it panicked (CLI exit 2).
func TestR3_UnparsableIsACountedSkip(t *testing.T) {
	mutate := map[string]func([]Record){
		"bank":    func(r []Record) { r[1].State.Bank[alice] = "7O" },
		"deleg":   func(r []Record) { r[0].State.Deleg[alice+"|"+val] = "-" },
		"amount":  func(r []Record) { r[0].Args["amount"] = "30.5" },
		"vesting": func(r []Record) { r[0].State.Vesting = map[string]VestingRec{alice: {DelegatedVesting: "x"}} },
		"rewards": func(r []Record) {
			r[0].Method = "claimRewards"
			r[0].State.Rewards = map[string]string{alice + "|" + val: "1..5"}
		},
	}
	for name, m := range mutate {
		recs := delegateTrace("80", "20", "1020") // would violate R3-D
		m(recs)
		res := Evaluate(recs)
		if len(res.Findings) != 0 || res.Stats.R3.Skipped.Unparsable != 1 || res.Stats.R3.Evaluated != (R3Evals{}) {
			t.Errorf("%s: findings=%+v r3=%+v", name, res.Findings, res.Stats.R3)
			continue
		}
		expectVerdict(t, res, VerdictInconclusive)
	}
}

func TestR3_SkipReasons(t *testing.T) {
	enter := func(seq, call int, method string, args map[string]string) Record {
		s := st(map[string]string{alice: "100"})
		return Record{Seq: seq, Point: PointPCEnter, DB: "db1", Call: call, Method: method, Args: args, State: s}
	}
	exit := func(seq, call int, err string) Record {
		return Record{Seq: seq, Point: PointPCExit, DB: "db1", Call: call, Err: err, State: st(map[string]string{alice: "100"})}
	}
	full := map[string]string{"delegatorAddress": alice, "validatorAddress": val, "amount": "0"}
	recs := []Record{
		enter(1, 1, "delegate", full), exit(2, 1, "execution reverted"), // call error
		enter(3, 2, "delegate", map[string]string{"delegatorAddress": alice}), exit(4, 2, ""), // no amount
		enter(5, 3, "undelegate", map[string]string{"amount": "1"}), exit(6, 3, ""), // no delegator
		enter(7, 4, "claimRewards", map[string]string{"delegatorAddress": alice}), exit(8, 4, ""), // judged, no module
		exit(9, 5, ""),                                // no enter
		enter(10, 6, "approve", nil), exit(11, 6, ""), // not covered
		enter(12, 7, "undelegate", full), exit(13, 7, ""), // judged (zero amount, nothing moves), no module
	}
	res := Evaluate(recs)
	want := R3Stats{
		Exits:      7,
		Evaluated:  R3Evals{U: 1, C: 1},
		NotCovered: 1,
		Skipped:    R3Skips{CallError: 1, NoEnter: 1, MissingArgs: 2, MissingModule: 2},
	}
	if res.Stats.R3 != want {
		t.Fatalf("r3 = %+v\nwant %+v", res.Stats.R3, want)
	}
	expectRules(t, res.Findings, "")
	expectVerdict(t, res, VerdictNoViolation) // missing arguments / module are counted, not inconclusive
}

// ---------------------------------------------------------------- stats on a small trace

func TestStats_Counters(t *testing.T) {
	enterSt := st(map[string]string{alice: "100", bob: "3"})
	enterSt.Deleg = map[string]string{alice + "|" + val: "0"}
	enterSt.ValTokens = map[string]string{val: "1000"}
	enterSt.Blocked = []string{bob}
	enterSt.StateDB = map[string]StateDBRec{
		alice: {Balance: "100", WriteBack: "100", BankRef: "100"},
		bob:   {Balance: "1", WriteBack: "1", BankRef: "3"}, // blocked: not compared
	}
	exitSt := st(map[string]string{alice: "70", bob: "3"})
	exitSt.Deleg = map[string]string{alice + "|" + val: "30"}
	exitSt.ValTokens = map[string]string{val: "1030"}
	exitSt.StateDB = map[string]StateDBRec{alice: {Balance: "70", WriteBack: "70", BankRef: "70"}}
	args := map[string]string{"delegatorAddress": alice, "validatorAddress": val, "amount": "30"}
	recs := []Record{
		{Seq: 1, Point: PointSnapshot, DB: "db1", Snap: 1, State: evState([]string{"tx:a"}, nil)},
		{Seq: 2, Point: PointPCEnter, DB: "db1", Call: 1, Method: "delegate", Args: args, State: enterSt},
		{Seq: 3, Point: PointBalance, DB: "db1", Call: 1, Op: "sub", Addr: alice, Before: "100", Amount: "30"},
		{Seq: 4, Point: PointBalance, DB: "db1", Call: 1, Op: "add", Addr: bob, Before: "0", Amount: "0"},
		{Seq: 5, Point: PointBalance, DB: "db1", Call: 1, Op: "set", Addr: alice, Before: "70", Amount: "70"},
		{Seq: 6, Point: PointPCExit, DB: "db1", Call: 1, State: exitSt},
		// leaves bank:alice 1 -> 70 and one event behind (2 candidates)
		{Seq: 7, Point: PointRevert, DB: "db1", Snap: 1, State: func() *State {
			s := st(map[string]string{alice: "70"})
			s.Events = ev([]string{"tx:a"}, []string{"coin_spent:y"})
			return s
		}()},
		{Seq: 8, Point: PointRevert, DB: "db1", Snap: 9, State: st(nil)}, // no snapshot 9
		{Seq: 9, Point: PointCommit, DB: "db1", State: evState([]string{"tx:a"}, nil)},
		{Seq: 10, Point: "O9.future", DB: "db1"},
		{Seq: 11, Point: PointPCEnter, DB: "db2", Call: 2, Method: "approve", State: st(nil)}, // never exits
	}
	res := Evaluate(recs)
	want := Stats{
		Records: 11,
		Points: map[string]int{
			PointSnapshot: 1, PointPCEnter: 2, PointBalance: 3, PointPCExit: 1,
			PointRevert: 2, PointCommit: 1, "O9.future": 1,
		},
		UnknownPoints: 1,
		StateDBs:      2,
		O2:            O2Stats{Enters: 2, Exits: 1, Unclosed: 1},
		R1: R1Stats{
			Reverts: 2, NoSnapshot: 1, Compared: 1, WithCandidates: 1,
			StateCandidates: 1, EventCandidates: 1,
			Commits: 1, Judged: 1, JudgedWithCandidates: 1,
		},
		R2A: R2AStats{Ops: 3, Checked: 2, NonZero: 1, Set: 1},
		R2W: R2WStats{SyncEnter: 2, SyncExit: 1, Enter: 1, Exit: 1, Blocked: 1},
		R3:  R3Stats{Exits: 1, Evaluated: R3Evals{D: 1}, Skipped: R3Skips{MissingModule: 1}},
		A1:  A1Stats{Compared: 1},
	}
	if !reflect.DeepEqual(res.Stats, want) {
		t.Fatalf("stats =\n%+v\nwant\n%+v", res.Stats, want)
	}
	expectRules(t, res.Findings, "")
	expectVerdict(t, res, VerdictInconclusive)
	if len(res.Inconclusive) != 2 {
		t.Fatalf("inconclusive = %q", res.Inconclusive)
	}
	expectReason(t, res, "O5.revert record(s) have no matching O5.snapshot (first at seq 8)", "no O2.exit by the end of the trace (first at seq 11)")
}
