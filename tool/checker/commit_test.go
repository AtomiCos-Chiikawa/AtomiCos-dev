package checker

import "testing"

// Synthetic traces only (commit checks, commit.go).

const pool = "0x00000000000000000000000000000000000000f1" // a module account, blocked

func commitState(wb, ref string, ctx, cache []string) *State {
	s := evState(ctx, cache)
	s.Blocked = []string{pool}
	s.Modules = map[string]string{"pool": pool}
	s.StateDB = map[string]StateDBRec{alice: {Balance: wb, WriteBack: wb, BankRef: ref, Mode: "test"}}
	return s
}

// R2-W at a successful commit: write-back that the committed bank does not match.
func TestR2WCommit_Mismatch(t *testing.T) {
	ok := []Record{{Seq: 1, Point: PointCommit, DB: "db1", State: commitState("30", "30", nil, nil)}}
	expectRules(t, Check(ok), "")
	bad := []Record{{Seq: 1, Point: PointCommit, DB: "db1", State: commitState("30", "20", nil, nil)}}
	fs := Check(bad)
	expectRules(t, fs, SevViolation, "R2-W")
	if fs[0].Point != PointCommit || fs[0].Subject != alice {
		t.Fatalf("finding = %+v", fs[0])
	}
}

// Blocked accounts are skipped at O6 as at the synchronisation points.
func TestR2WCommit_SkipsBlocked(t *testing.T) {
	s := commitState("30", "30", nil, nil)
	s.StateDB[pool] = StateDBRec{Balance: "5", WriteBack: "5", BankRef: "0"}
	res := Evaluate([]Record{{Seq: 1, Point: PointCommit, DB: "db1", State: s}})
	expectRules(t, res.Findings, "")
	if res.Stats.R2W.Blocked != 1 || res.Stats.R2W.Commit != 1 {
		t.Fatalf("r2w stats = %+v", res.Stats.R2W)
	}
}

// A commit with a recorded error is not judged.
func TestCommit_FailedNotJudged(t *testing.T) {
	recs := []Record{{Seq: 1, Point: PointCommit, DB: "db1", Err: "unauthorized", ErrKnown: true,
		State: commitState("30", "20", nil, []string{"coin_spent:y"})}}
	res := Evaluate(recs)
	expectRules(t, res.Findings, "")
	if res.Stats.O6.Failed != 1 || res.Stats.O6.Judged != 0 {
		t.Fatalf("o6 stats = %+v", res.Stats.O6)
	}
}

// Without errKnown, a non-zero balance change of a module/blocked account in the same StateDB
// means the keeper may have refused the write-back: the commit is not judged. With errKnown and
// no error, it is judged.
func TestCommit_InferredRefusal(t *testing.T) {
	recs := func(amount string, known bool) []Record {
		return []Record{
			{Seq: 1, Point: PointBalance, DB: "db1", Op: "add", Addr: pool, Before: "0", Amount: amount},
			{Seq: 2, Point: PointBalance, DB: "db2", Op: "add", Addr: alice, Before: "0", Amount: "9"},
			{Seq: 3, Point: PointCommit, DB: "db1", ErrKnown: known, State: commitState("30", "20", nil, nil)},
		}
	}
	res := Evaluate(recs("15", false))
	expectRules(t, res.Findings, "")
	if res.Stats.O6.MaybeFailed != 1 {
		t.Fatalf("o6 stats = %+v", res.Stats.O6)
	}
	expectRules(t, Check(recs("0", false)), SevViolation, "R2-W") // zero amount: no write-back change
	expectRules(t, Check(recs("15", true)), SevViolation, "R2-W") // harness says it succeeded
}

// R1-C: events of the final precompile cache must reach the committed events.
func TestR1C_EventsDroppedAtCommit(t *testing.T) {
	ok := []Record{{Seq: 1, Point: PointCommit, DB: "db1",
		State: commitState("1", "1", []string{"tx:a", "coin_spent:y", "coin_spent:y"}, []string{"coin_spent:y", "coin_spent:y"})}}
	expectRules(t, Check(ok), "")
	// one of two identical copies is missing (multiset)
	bad := []Record{{Seq: 1, Point: PointCommit, DB: "db1",
		State: commitState("1", "1", []string{"tx:a", "coin_spent:y"}, []string{"coin_spent:y", "coin_spent:y"})}}
	expectRules(t, Check(bad), SevViolation, "R1-C")
	// no precompile cache: nothing to check
	none := []Record{{Seq: 1, Point: PointCommit, DB: "db1", State: commitState("1", "1", nil, nil)}}
	res := Evaluate(none)
	expectRules(t, res.Findings, "")
	if res.Stats.O6.EventsJudged != 0 || res.Stats.O6.Judged != 1 {
		t.Fatalf("o6 stats = %+v", res.Stats.O6)
	}
}

// Copies left behind by a revert (R1 event candidates) may be dropped at commit (v0.1, v0.5).
func TestR1C_RevertLeftoversNotExpected(t *testing.T) {
	recs := []Record{
		{Seq: 1, Point: PointSnapshot, DB: "db1", Snap: 1, State: evState(nil, []string{"coin_spent:y"})},
		{Seq: 2, Point: PointRevert, DB: "db1", Snap: 1, State: evState(nil, []string{"coin_spent:y", "coin_received:x"})},
		{Seq: 3, Point: PointCommit, DB: "db1", State: evState([]string{"coin_spent:y"}, []string{"coin_spent:y", "coin_received:x"})},
	}
	expectRules(t, Check(recs), "")
	// the copy from before the snapshot is still expected
	recs[2].State = evState(nil, []string{"coin_spent:y", "coin_received:x"})
	expectRules(t, Check(recs), SevViolation, "R1-C")
}
