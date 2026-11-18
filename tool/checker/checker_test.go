package checker

import (
	"strings"
	"testing"
)

// Synthetic traces only. Numbers are arbitrary and unrelated to any advisory.

const (
	alice = "0x00000000000000000000000000000000000000a1"
	bob   = "0x00000000000000000000000000000000000000b2"
	distr = "0x00000000000000000000000000000000000000d1"
	val   = "valoper1test"
)

func st(bank map[string]string) *State {
	return &State{Bank: bank, Supply: "1000", Events: EventsRec{Ctx: 2, Cache: -1}}
}

func rules(fs []Finding, sev string) []string {
	var out []string
	for _, f := range fs {
		if sev == "" || f.Severity == sev {
			out = append(out, f.Rule)
		}
	}
	return out
}

func expectRules(t *testing.T, fs []Finding, sev string, want ...string) {
	t.Helper()
	got := strings.Join(rules(fs, sev), ",")
	if got != strings.Join(want, ",") {
		t.Fatalf("%s findings = [%s], want [%s]\n%+v", sev, got, strings.Join(want, ","), fs)
	}
}

func TestR1S_RevertRestoresSDKState(t *testing.T) {
	recs := []Record{
		{Seq: 1, Point: PointSnapshot, DB: "db1", Snap: 3, State: st(map[string]string{alice: "100"})},
		{Seq: 2, Point: PointRevert, DB: "db1", Snap: 3, State: st(map[string]string{alice: "100"})},
		{Seq: 3, Point: PointCommit, DB: "db1", State: st(map[string]string{alice: "100"})},
	}
	expectRules(t, Check(recs), "")
}

func TestR1S_WriteSurvivesRevertAndIsCommitted(t *testing.T) {
	recs := []Record{
		{Seq: 1, Point: PointSnapshot, DB: "db1", Snap: 3, State: st(map[string]string{alice: "100"})},
		{Seq: 2, Point: PointRevert, DB: "db1", Snap: 3, State: st(map[string]string{alice: "130"})},
		{Seq: 3, Point: PointCommit, DB: "db1", State: st(map[string]string{alice: "130"})},
	}
	fs := Check(recs)
	expectRules(t, fs, SevViolation, "R1-S")
	if fs[0].Subject != "bank:"+alice || fs[0].Seq != 3 {
		t.Fatalf("finding = %+v", fs[0])
	}
}

// Visible after the revert but dropped at commit (e.g. the cache is discarded): not a violation.
func TestR1S_VisibleButNotCommitted(t *testing.T) {
	recs := []Record{
		{Seq: 1, Point: PointSnapshot, DB: "db1", Snap: 3, State: st(map[string]string{alice: "100"})},
		{Seq: 2, Point: PointRevert, DB: "db1", Snap: 3, State: st(map[string]string{alice: "130"})},
		{Seq: 3, Point: PointCommit, DB: "db1", State: st(map[string]string{alice: "100"})},
	}
	expectRules(t, Check(recs), "")
}

func TestR1S_NoCommitNoVerdict(t *testing.T) {
	recs := []Record{
		{Seq: 1, Point: PointSnapshot, DB: "db1", Snap: 1, State: st(map[string]string{})},
		{Seq: 2, Point: PointRevert, DB: "db1", Snap: 1, State: st(map[string]string{bob: "7"})},
	}
	expectRules(t, Check(recs), "")
}

func TestR1S_SnapshotsArePairedPerDB(t *testing.T) {
	recs := []Record{
		{Seq: 1, Point: PointSnapshot, DB: "db1", Snap: 1, State: st(map[string]string{alice: "100"})},
		{Seq: 2, Point: PointSnapshot, DB: "db2", Snap: 1, State: st(map[string]string{alice: "130"})},
		{Seq: 3, Point: PointRevert, DB: "db2", Snap: 1, State: st(map[string]string{alice: "130"})},
		{Seq: 4, Point: PointRevert, DB: "db1", Snap: 1, State: st(map[string]string{alice: "100"})},
		{Seq: 5, Point: PointCommit, DB: "db1", State: st(map[string]string{alice: "130"})},
	}
	expectRules(t, Check(recs), "")
}

func ev(ctx, cache []string) EventsRec {
	c := -1
	if cache != nil {
		c = len(cache)
	}
	return EventsRec{Ctx: len(ctx), Cache: c, CtxSigs: ctx, CacheSigs: cache}
}

func evState(ctx, cache []string) *State {
	s := st(map[string]string{alice: "1"})
	s.Events = ev(ctx, cache)
	return s
}

// Events left behind by a revert that reach the committed events: violation.
func TestR1E_RevertedEventsReachCommit(t *testing.T) {
	recs := []Record{
		{Seq: 1, Point: PointSnapshot, DB: "db1", Snap: 1, State: evState([]string{"tx:a"}, nil)},
		{Seq: 2, Point: PointRevert, DB: "db1", Snap: 1, State: evState([]string{"tx:a"}, []string{"coin_received:x", "coin_spent:y"})},
		{Seq: 3, Point: PointCommit, DB: "db1", State: evState([]string{"tx:a", "coin_received:x", "coin_spent:y"}, []string{"coin_received:x", "coin_spent:y"})},
	}
	fs := Check(recs)
	expectRules(t, fs, SevViolation, "R1-E", "R1-E")
}

// Events left in the cache after a revert but not emitted at commit: fine.
func TestR1E_LeftoverNotEmitted(t *testing.T) {
	recs := []Record{
		{Seq: 1, Point: PointSnapshot, DB: "db1", Snap: 1, State: evState([]string{"tx:a"}, nil)},
		{Seq: 2, Point: PointRevert, DB: "db1", Snap: 1, State: evState([]string{"tx:a"}, []string{"coin_received:x"})},
		{Seq: 3, Point: PointCommit, DB: "db1", State: evState([]string{"tx:a", "other:z"}, []string{"coin_received:x"})},
	}
	expectRules(t, Check(recs), "")
}

// Same event content emitted before the StateDB started is not attributed to it.
func TestR1E_BaselineIsExcluded(t *testing.T) {
	recs := []Record{
		{Seq: 1, Point: PointSnapshot, DB: "db1", Snap: 1, State: evState([]string{"coin_received:x"}, nil)},
		{Seq: 2, Point: PointRevert, DB: "db1", Snap: 1, State: evState([]string{"coin_received:x"}, []string{"coin_received:x"})},
		{Seq: 3, Point: PointCommit, DB: "db1", State: evState([]string{"coin_received:x"}, nil)},
	}
	expectRules(t, Check(recs), "")
}

// Leftovers of a nested StateDB are judged at its top-level StateDB's commit.
func TestR1E_NestedJudgedAtTopLevel(t *testing.T) {
	nested := func(topCommitted []string) []Record {
		return []Record{
			{Seq: 1, Point: PointPCEnter, DB: "db1", Call: 1, State: evState([]string{"tx:a"}, []string{})},
			{Seq: 2, Point: PointSnapshot, DB: "db2", Snap: 1, State: evState(nil, nil)},
			{Seq: 3, Point: PointRevert, DB: "db2", Snap: 1, State: evState(nil, []string{"coin_received:x"})},
			// the nested StateDB commits the leftover into a context the outer may not see
			{Seq: 4, Point: PointCommit, DB: "db2", State: evState([]string{"coin_received:x"}, nil)},
			{Seq: 5, Point: PointPCExit, DB: "db1", Call: 1, State: evState([]string{"tx:a"}, []string{})},
			{Seq: 6, Point: PointCommit, DB: "db1", State: evState(topCommitted, nil)},
		}
	}
	expectRules(t, Check(nested([]string{"tx:a"})), "")
	expectRules(t, Check(nested([]string{"tx:a", "coin_received:x"})), SevViolation, "R1-E")
}

// A failed commit is not a result.
func TestR1E_FailedCommitIgnored(t *testing.T) {
	recs := []Record{
		{Seq: 1, Point: PointSnapshot, DB: "db1", Snap: 1, State: evState(nil, nil)},
		{Seq: 2, Point: PointRevert, DB: "db1", Snap: 1, State: evState(nil, []string{"coin_received:x"})},
		{Seq: 3, Point: PointCommit, DB: "db1", Err: "panic: x", State: evState([]string{"coin_received:x"}, nil)},
	}
	expectRules(t, Check(recs), "")
}

func TestR2A_Underflow(t *testing.T) {
	recs := []Record{
		{Seq: 1, Point: PointPCEnter, DB: "db1", Call: 1},
		{Seq: 2, Point: PointBalance, DB: "db1", Call: 1, Op: "sub", Addr: alice, Before: "10", Amount: "25"},
		{Seq: 3, Point: PointPCExit, DB: "db1", Call: 1},
	}
	fs := Check(recs)
	expectRules(t, fs, SevViolation, "R2-A")
	if fs[0].Outcome != "applied" {
		t.Fatalf("outcome = %q", fs[0].Outcome)
	}
}

func TestR2A_UnderflowRejectedByPanic(t *testing.T) {
	recs := []Record{
		{Seq: 1, Point: PointPCEnter, DB: "db1", Call: 1},
		{Seq: 2, Point: PointBalance, DB: "db1", Call: 1, Op: "sub", Addr: alice, Before: "10", Amount: "25"},
		{Seq: 3, Point: PointPCExit, DB: "db1", Call: 1, Err: "panic: balance underflow"},
	}
	fs := Check(recs)
	expectRules(t, fs, SevViolation)
	expectRules(t, fs, SevGuarded, "R2-A")
}

func TestR2A_NormalAndOverflow(t *testing.T) {
	max := "115792089237316195423570985008687907853269984665640564039457584007913129639935" // 2^256-1
	recs := []Record{
		{Seq: 1, Point: PointBalance, DB: "db1", Op: "sub", Addr: alice, Before: "10", Amount: "10"},
		{Seq: 2, Point: PointBalance, DB: "db1", Op: "add", Addr: alice, Before: "0", Amount: max},
		{Seq: 3, Point: PointBalance, DB: "db1", Op: "add", Addr: bob, Before: max, Amount: "1"},
		{Seq: 4, Point: PointBalance, DB: "db1", Op: "set", Addr: bob, Before: "1", Amount: "0"},
	}
	fs := Check(recs)
	expectRules(t, fs, SevViolation, "R2-A")
	if fs[0].Subject != bob {
		t.Fatalf("subject = %s", fs[0].Subject)
	}
}

func TestR2W_WriteBackMatchesAtSyncPoints(t *testing.T) {
	s := st(map[string]string{alice: "50"})
	s.StateDB = map[string]StateDBRec{alice: {Balance: "10", WriteBack: "50", BankRef: "50", Mode: "test"}}
	recs := []Record{{Seq: 1, Point: PointPCExit, DB: "db1", Call: 1, State: s}}
	expectRules(t, Check(recs), "")
}

func TestR2W_Mismatch(t *testing.T) {
	s := st(map[string]string{alice: "20"})
	s.StateDB = map[string]StateDBRec{alice: {Balance: "999", WriteBack: "1039", BankRef: "20", Mode: "test"}}
	recs := []Record{
		{Seq: 1, Point: PointPCExit, DB: "db1", Call: 1, State: s},
		{Seq: 2, Point: PointPCEnter, DB: "db1", Call: 2, State: s}, // same account again: folded
	}
	fs := Check(recs)
	expectRules(t, fs, SevViolation, "R2-W")
	if fs[0].Repeats != 1 {
		t.Fatalf("repeats = %d", fs[0].Repeats)
	}
}

func TestR2W_SkipsFailedExitAndBlocked(t *testing.T) {
	s := st(map[string]string{alice: "20", bob: "3"})
	s.Blocked = []string{bob}
	s.StateDB = map[string]StateDBRec{
		alice: {Balance: "1", WriteBack: "1", BankRef: "20"},
		bob:   {Balance: "1", WriteBack: "1", BankRef: "3"},
	}
	recs := []Record{{Seq: 1, Point: PointPCExit, DB: "db1", Call: 1, Err: "out of gas", State: s}}
	expectRules(t, Check(recs), "")
	recs[0].Err = ""
	fs := Check(recs)
	expectRules(t, fs, SevViolation, "R2-W")
	if fs[0].Subject != alice {
		t.Fatalf("blocked address must be skipped, got %s", fs[0].Subject)
	}
}

func delegateTrace(bankAfter, delegAfter, valAfter string) []Record {
	a := st(map[string]string{alice: "100"})
	a.Deleg = map[string]string{alice + "|" + val: "0"}
	a.ValTokens = map[string]string{val: "1000"}
	b := st(map[string]string{alice: bankAfter})
	b.Deleg = map[string]string{alice + "|" + val: delegAfter}
	b.ValTokens = map[string]string{val: valAfter}
	args := map[string]string{"delegatorAddress": alice, "validatorAddress": val, "amount": "30"}
	return []Record{
		{Seq: 1, Point: PointPCEnter, DB: "db1", Call: 1, Method: "delegate", Args: args, State: a},
		{Seq: 2, Point: PointPCExit, DB: "db1", Call: 1, State: b},
	}
}

func TestR3D_Delegate(t *testing.T) {
	expectRules(t, Check(delegateTrace("70", "30", "1030")), "")
	expectRules(t, Check(delegateTrace("80", "30", "1030")), SevViolation, "R3-D")
	expectRules(t, Check(delegateTrace("70", "20", "1020")), SevViolation, "R3-D", "R3-D")
}

// Delegating to a validator the delegator already delegates to withdraws the accrued rewards
// first (x/distribution hook): the delegator's balance drops by amount - rewards.
func TestR3D_AutoWithdrawnRewards(t *testing.T) {
	withDistr := func(bankAfter, distrAfter string) []Record {
		recs := delegateTrace(bankAfter, "30", "1030")
		recs[0].State.Bank[distr] = "500"
		recs[0].State.Modules = map[string]string{"distribution": distr}
		recs[1].State.Bank[distr] = distrAfter
		recs[1].State.Modules = recs[0].State.Modules
		return recs
	}
	expectRules(t, Check(withDistr("75", "495")), "")                   // -30 + 5 rewards
	expectRules(t, Check(withDistr("70", "495")), SevViolation, "R3-D") // rewards left the module but never arrived
}

func TestR3U_AutoWithdrawnRewards(t *testing.T) {
	trace := func(bankAfter, distrAfter string) []Record {
		a := st(map[string]string{alice: "100", distr: "500"})
		a.Modules = map[string]string{"distribution": distr}
		a.Deleg = map[string]string{alice + "|" + val: "30"}
		b := st(map[string]string{alice: bankAfter, distr: distrAfter})
		b.Modules = a.Modules
		b.Deleg = map[string]string{alice + "|" + val: "10"}
		b.Unbonding = map[string]string{alice + "|" + val: "20"}
		args := map[string]string{"delegatorAddress": alice, "validatorAddress": val, "amount": "20"}
		return []Record{
			{Seq: 1, Point: PointPCEnter, DB: "db1", Call: 1, Method: "undelegate", Args: args, State: a},
			{Seq: 2, Point: PointPCExit, DB: "db1", Call: 1, State: b},
		}
	}
	expectRules(t, Check(trace("100", "500")), "")
	expectRules(t, Check(trace("103", "497")), "")
	expectRules(t, Check(trace("120", "500")), SevViolation, "R3-U") // principal paid out before maturity
}

func TestR3D_FailedCallIsLeftToR1(t *testing.T) {
	recs := delegateTrace("80", "30", "1030")
	recs[1].Err = "execution reverted"
	expectRules(t, Check(recs), "")
}

func claimTrace(paid, distrDelta, rewardsAfter string) []Record {
	a := st(map[string]string{alice: "100", distr: "500"})
	a.Modules = map[string]string{"distribution": distr}
	a.Rewards = map[string]string{alice + "|" + val: "40.5"}
	b := st(map[string]string{alice: add(100, paid), distr: add(500, distrDelta)})
	b.Modules = a.Modules
	b.Rewards = map[string]string{alice + "|" + val: rewardsAfter}
	args := map[string]string{"delegatorAddress": alice}
	return []Record{
		{Seq: 1, Point: PointPCEnter, DB: "db1", Call: 1, Method: "claimRewards", Args: args, State: a},
		{Seq: 2, Point: PointPCExit, DB: "db1", Call: 1, State: b},
	}
}

func add(base int, delta string) string {
	var d int
	neg := strings.HasPrefix(delta, "-")
	for _, c := range strings.TrimPrefix(delta, "-") {
		d = d*10 + int(c-'0')
	}
	if neg {
		d = -d
	}
	return itoa(base + d)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}

func TestR3C_Claim(t *testing.T) {
	expectRules(t, Check(claimTrace("40", "-40", "0")), "")
	// paid but claimable rewards were not reduced
	expectRules(t, Check(claimTrace("40", "-40", "40.5")), SevViolation, "R3-C")
	// paid more than the distribution module lost
	expectRules(t, Check(claimTrace("40", "-30", "0")), SevViolation, "R3-C")
}

func TestA1_SupplyChangeIsAuxiliary(t *testing.T) {
	a := st(nil)
	b := st(nil)
	b.Supply = "999"
	recs := []Record{
		{Seq: 1, Point: PointSnapshot, DB: "db1", Snap: 1, State: a},
		{Seq: 2, Point: PointCommit, DB: "db1", State: b},
	}
	fs := Check(recs)
	expectRules(t, fs, SevViolation)
	expectRules(t, fs, SevAux, "A1")
}

func TestReadTrace(t *testing.T) {
	in := `{"seq":1,"point":"O4.balance","db":"db1","op":"sub","addr":"0xa","before":"1","amount":"2"}

{"seq":2,"point":"O2.exit","db":"db1","call":1,"err":"panic: x"}
`
	recs, err := ReadTrace(strings.NewReader(in))
	if err != nil || len(recs) != 2 || recs[1].Err != "panic: x" {
		t.Fatalf("recs=%+v err=%v", recs, err)
	}
	if _, err := ReadTrace(strings.NewReader("{bad")); err == nil {
		t.Fatal("expected a parse error")
	}
}
