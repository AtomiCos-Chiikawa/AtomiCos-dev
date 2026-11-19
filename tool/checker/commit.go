package checker

import (
	"fmt"
	"strings"
)

// ---------------------------------------------------------------- commit checks (added after the step-11 dev analysis)
//
// At a successful O6 commit the StateDB has written everything back, so two things that R1/R2-W
// only check at synchronisation points must also hold in the committed context:
//
//	R2-W (at O6): every loaded, non-blocked account's write-back equals its bank reference.
//	R1-C:         every SDK event in the StateDB's final precompile cache (after reverts) is in
//	              the committed context events (multiset containment): successful precompile
//	              changes are not dropped at commit. Copies that R1 found left behind by reverts
//	              (its event candidates of the same top-level StateDB) are not expected: v0.1 and
//	              v0.5 keep them in the cache and drop them only at commit.
//
// Both apply only to commits that succeeded. A commit is failed when the O6 record carries an
// error. Traces whose harness records the error Commit returns mark that with errKnown; for
// older traces (no errKnown) a commit is treated as possibly failed, and skipped, when the
// StateDB changed the balance of an address the keeper refuses to write back (a module account
// or a blocked address: cosmos/evm keeper SetBalanceWithLocked returns ErrUnauthorized for
// module accounts and for blocked addresses whose balance changes). The inference is
// conservative: such a commit may still have succeeded, it is then just not judged.

func checkCommit(recs []Record, e *eval) []Finding {
	st := &e.st.O6
	var out []Finding
	touched := map[string]map[string]bool{} // db -> addresses whose StateDB balance this StateDB changed
	for _, r := range recs {
		switch r.Point {
		case PointBalance:
			if r.Op == "set" || r.Op == "add" || r.Op == "sub" {
				if r.Op == "set" || numOrZero(r.Amount) != "0" {
					if touched[r.DB] == nil {
						touched[r.DB] = map[string]bool{}
					}
					touched[r.DB][strings.ToLower(r.Addr)] = true
				}
			}
			continue
		case PointCommit:
		default:
			continue
		}
		st.Commits++
		switch {
		case r.Err != "":
			st.Failed++
			continue
		case !usable(r):
			st.NoState++
			continue
		case !r.ErrKnown && mayBeRefused(r.State, touched[r.DB]):
			st.MaybeFailed++
			continue
		}
		st.Judged++
		out = append(out, r2wAt(r, &e.st.R2W, e)...)
		if r.State.Events.Cache < 0 {
			continue
		}
		st.EventsJudged++
		ctx := multiset(r.State.Events.CtxSigs)
		var left map[string]int
		if e.root != nil {
			left = e.revertEvents[e.root(r.DB)]
		}
		missing := map[string]int{}
		n := 0
		for sig, c := range multiset(r.State.Events.CacheSigs) {
			if d := c - left[sig] - ctx[sig]; d > 0 {
				missing[sig] = d
				n += d
			}
		}
		if n == 0 {
			continue
		}
		sigs := sortedKeys(missing)
		if len(sigs) > 3 {
			sigs = append(sigs[:3], "…")
		}
		out = append(out, Finding{
			Rule: "R1-C", Severity: SevViolation, Point: PointCommit, Seq: r.Seq, DB: r.DB,
			Subject:  "committed events",
			Expected: fmt.Sprintf("the %d event(s) of the final precompile cache, minus copies left by reverts, are in the committed events", len(r.State.Events.CacheSigs)),
			Actual:   fmt.Sprintf("%d missing (committed %d): %s", n, len(r.State.Events.CtxSigs), strings.Join(sigs, ", ")),
			Detail:   "events of successful precompile calls were dropped at commit",
		})
	}
	return out
}

// mayBeRefused reports whether the StateDB changed the balance of a module account or blocked
// address, whose write-back the keeper refuses (the commit may then have returned an error).
func mayBeRefused(s *State, touched map[string]bool) bool {
	if len(touched) == 0 {
		return false
	}
	for _, b := range s.Blocked {
		if touched[strings.ToLower(b)] {
			return true
		}
	}
	for _, m := range s.Modules {
		if touched[strings.ToLower(m)] {
			return true
		}
	}
	return false
}

// r2wAt compares write-back and bank reference of every loaded, non-blocked account of a
// record (same comparison as R2-W at the synchronisation points).
func r2wAt(r Record, st *R2WStats, e *eval) []Finding {
	var out []Finding
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
		switch r.Point {
		case PointPCEnter:
			st.Enter++
		case PointPCExit:
			st.Exit++
		case PointCommit:
			st.Commit++
		}
		if wb.Cmp(ref) == 0 {
			continue
		}
		detail := "the StateDB balance would not reconcile with the bank at commit"
		if r.Point == PointCommit {
			detail = "the committed bank balance differs from what the StateDB wrote back"
		}
		out = append(out, Finding{
			Rule: "R2-W", Severity: SevViolation, Point: r.Point,
			Seq: r.Seq, DB: r.DB, Call: r.Call, Subject: addr,
			Expected: fmt.Sprintf("write-back %s == bank reference", a.BankRef),
			Actual:   fmt.Sprintf("write-back %s (StateDB balance %s, mode %s)", a.WriteBack, a.Balance, a.Mode),
			Detail:   detail,
		})
	}
	return out
}
