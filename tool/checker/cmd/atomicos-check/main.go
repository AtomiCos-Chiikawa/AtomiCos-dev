// Command atomicos-check judges an AtomiCos trace (JSON Lines) and prints the findings, the
// verdict and what the checker examined (stats).
//
//	atomicos-check [-json] [-drop O4,O5] trace.jsonl[.gz]
//
// Exit status:
//
//	0  no violation
//	1  at least one violation (even if the trace is also inconclusive)
//	2  usage or read error
//	3  inconclusive: no violation, but the trace cannot support a "no violation" verdict
//	   (no records, state.error, unparsable numbers, unclosed O2.enter, revert without snapshot)
//
// "guarded" (attempted but rejected) and "aux" findings do not change the exit status.
package main

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"atomicos/tool/checker"
)

const (
	exitNoViolation  = 0
	exitViolation    = 1
	exitError        = 2
	exitInconclusive = 3
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("atomicos-check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "print findings as JSON")
	drop := fs.String("drop", "", "comma-separated observation points to ignore, e.g. O4,O5 (ablation, eval/mutation)")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: atomicos-check [-json] [-drop O4,O5] trace.jsonl[.gz]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitNoViolation // as flag.ExitOnError did
		}
		return exitError
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return exitError
	}
	recs, err := readFile(fs.Arg(0))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitError
	}
	if *drop != "" {
		recs = dropPoints(recs, strings.Split(*drop, ","))
	}
	res := checker.Evaluate(recs)

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(struct {
			Records      int               `json:"records"`
			Verdict      string            `json:"verdict"`
			Inconclusive []string          `json:"inconclusive"`
			Stats        checker.Stats     `json:"stats"`
			Findings     []checker.Finding `json:"findings"`
		}{len(recs), res.Verdict(), res.Inconclusive, res.Stats, res.Findings}); err != nil {
			fmt.Fprintln(stderr, err)
			return exitError
		}
	} else {
		printText(stdout, len(recs), res)
	}
	switch res.Verdict() {
	case checker.VerdictViolation:
		return exitViolation
	case checker.VerdictInconclusive:
		return exitInconclusive
	}
	return exitNoViolation
}

func readFile(path string) ([]checker.Record, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var r io.Reader = f
	if strings.HasSuffix(path, ".gz") {
		zr, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer zr.Close()
		r = zr
	}
	return checker.ReadTrace(r)
}

// dropPoints removes the records of the given observation points ("O4" removes O4.balance),
// as if the harness had not observed them. Used for the ablation in eval/mutation.
func dropPoints(recs []checker.Record, points []string) []checker.Record {
	out := recs[:0:0]
	for _, r := range recs {
		keep := true
		for _, p := range points {
			if p = strings.TrimSpace(p); p != "" && strings.HasPrefix(r.Point, p+".") {
				keep = false
				break
			}
		}
		if keep {
			out = append(out, r)
		}
	}
	return out
}

// printText writes the human-readable report. Scripts (cases/lib.sh) count findings from the
// lines that start with "  violation ", "  guarded " or "  aux "; no other line may.
func printText(w io.Writer, n int, res checker.Result) {
	fs := res.Findings
	count := map[string]int{}
	for _, x := range fs {
		count[x.Severity+" "+x.Rule]++
	}
	fmt.Fprintf(w, "records: %d, findings: %d\n", n, len(fs))
	keys := make([]string, 0, len(count))
	for k := range count {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(w, "  %-22s %d\n", k, count[k])
	}
	switch res.Verdict() {
	case checker.VerdictViolation:
		if len(res.Inconclusive) > 0 {
			fmt.Fprintln(w, "verdict: violation (the trace is also inconclusive, see below)")
		} else {
			fmt.Fprintln(w, "verdict: violation")
		}
	case checker.VerdictInconclusive:
		fmt.Fprintln(w, "verdict: inconclusive (no violation found, but this trace cannot support a no-violation verdict)")
	default:
		fmt.Fprintln(w, "verdict: no violation")
	}
	for _, why := range res.Inconclusive {
		fmt.Fprintf(w, "  inconclusive: %s\n", why)
	}
	printStats(w, res.Stats)

	for _, x := range fs {
		loc := fmt.Sprintf("seq %d %s %s", x.Seq, x.DB, x.Point)
		if x.Call != 0 {
			loc += fmt.Sprintf(" call %d", x.Call)
		}
		if x.Snap != 0 {
			loc += fmt.Sprintf(" snap %d", x.Snap)
		}
		fmt.Fprintf(w, "\n[%s] %s  (%s)\n", strings.ToUpper(x.Severity), x.Rule, loc)
		fmt.Fprintf(w, "  subject:  %s\n  expected: %s\n  actual:   %s\n", x.Subject, x.Expected, x.Actual)
		if x.Detail != "" {
			fmt.Fprintf(w, "  detail:   %s\n", x.Detail)
		}
		if x.Outcome != "" {
			fmt.Fprintf(w, "  outcome:  %s\n", x.Outcome)
		}
		if x.Repeats > 0 {
			seqs := make([]string, len(x.Seqs))
			for i, s := range x.Seqs {
				seqs[i] = fmt.Sprint(s)
			}
			fmt.Fprintf(w, "  repeats:  %d more in the same DB (seqs %s)\n", x.Repeats, strings.Join(seqs, ", "))
		}
		for i, s := range x.Stack {
			if i >= 4 {
				break
			}
			fmt.Fprintf(w, "  at %s\n", s)
		}
	}
}

func printStats(w io.Writer, s checker.Stats) {
	pts := make([]string, 0, len(s.Points))
	for _, p := range sortedKeys(s.Points) {
		pts = append(pts, fmt.Sprintf("%s %d", p, s.Points[p]))
	}
	if len(pts) == 0 {
		pts = append(pts, "(none)")
	}
	r1, a, rw, r3 := s.R1, s.R2A, s.R2W, s.R3
	fmt.Fprintln(w, "stats (what the checker examined):")
	fmt.Fprintf(w, "  points  %s; unknown point %d\n", strings.Join(pts, ", "), s.UnknownPoints)
	fmt.Fprintf(w, "  trace   statedbs %d, records with state.error %d, unparsable numbers %d\n", s.StateDBs, s.StateErrors, s.Unparsable)
	fmt.Fprintf(w, "  O2      enters %d, exits %d, enters never closed %d\n", s.O2.Enters, s.O2.Exits, s.O2.Unclosed)
	fmt.Fprintf(w, "  R1      reverts %d: compared %d, with candidates %d, no matching snapshot %d, no usable state %d\n",
		r1.Reverts, r1.Compared, r1.WithCandidates, r1.NoSnapshot, r1.NoState)
	fmt.Fprintf(w, "  R1      candidates: state keys %d, events %d; discarded (no judged top-level commit followed) %d\n",
		r1.StateCandidates, r1.EventCandidates, r1.Discarded)
	fmt.Fprintf(w, "  R1      commits %d: top-level judged %d (with candidates %d); not judged: nested %d, failed %d, no usable state %d; unparsable %d\n",
		r1.Commits, r1.Judged, r1.JudgedWithCandidates, r1.CommitsNested, r1.CommitsFailed, r1.CommitsNoState, r1.Unparsable)
	fmt.Fprintf(w, "  R2-A    balance ops %d: add/sub checked %d (non-zero amount %d); not checked: set %d, other op %d, unparsable %d\n",
		a.Ops, a.Checked, a.NonZero, a.Set, a.OtherOp, a.Unparsable)
	fmt.Fprintf(w, "  R2-W    comparisons enter %d, exit %d (sync points enter %d, exit %d); not compared: blocked %d, unparsable %d; failed exits %d, no usable state %d\n",
		rw.Enter, rw.Exit, rw.SyncEnter, rw.SyncExit, rw.Blocked, rw.Unparsable, rw.FailedExit, rw.NoState)
	fmt.Fprintf(w, "  R3      exits %d: evaluated R3-D %d, R3-U %d, R3-C %d; method not covered %d\n",
		r3.Exits, r3.Evaluated.D, r3.Evaluated.U, r3.Evaluated.C, r3.NotCovered)
	fmt.Fprintf(w, "  R3      skipped: call exited with error %d, missing arguments %d, unparsable number %d, no usable state %d, no matching enter %d; missing distribution module address %d\n",
		r3.Skipped.CallError, r3.Skipped.MissingArgs, r3.Skipped.Unparsable, r3.Skipped.NoState, r3.Skipped.NoEnter, r3.Skipped.MissingModule)
	fmt.Fprintf(w, "  A1      comparisons %d, unparsable %d\n", s.A1.Compared, s.A1.Unparsable)
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
