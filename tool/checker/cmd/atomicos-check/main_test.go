package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"atomicos/tool/checker"
)

const alice = "0x00000000000000000000000000000000000000a1"

func st(bal string) *checker.State {
	return &checker.State{Bank: map[string]string{alice: bal}, Supply: "1000", Events: checker.EventsRec{Ctx: 0, Cache: -1}}
}

// revertTrace: a frame changed alice 100 -> leftover, was reverted, and the StateDB committed.
func revertTrace(leftover, committed string) []checker.Record {
	return []checker.Record{
		{Seq: 1, Point: checker.PointSnapshot, DB: "db1", Snap: 1, State: st("100")},
		{Seq: 2, Point: checker.PointRevert, DB: "db1", Snap: 1, State: st(leftover)},
		{Seq: 3, Point: checker.PointCommit, DB: "db1", State: st(committed)},
	}
}

// guardedThenViolation: the same underflow is first rejected by a panic, then applied.
func guardedThenViolation() []checker.Record {
	return []checker.Record{
		{Seq: 1, Point: checker.PointPCEnter, DB: "db1", Call: 1},
		{Seq: 2, Point: checker.PointBalance, DB: "db1", Call: 1, Op: "sub", Addr: alice, Before: "10", Amount: "25"},
		{Seq: 3, Point: checker.PointPCExit, DB: "db1", Call: 1, Err: "panic: balance underflow"},
		{Seq: 4, Point: checker.PointPCEnter, DB: "db1", Call: 2},
		{Seq: 5, Point: checker.PointBalance, DB: "db1", Call: 2, Op: "sub", Addr: alice, Before: "10", Amount: "25"},
		{Seq: 6, Point: checker.PointPCExit, DB: "db1", Call: 2},
	}
}

func writeFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if strings.HasSuffix(name, ".gz") {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		zw.Write(data)
		zw.Close()
		data = buf.Bytes()
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeTrace(t *testing.T, name string, recs []checker.Record) string {
	t.Helper()
	var buf bytes.Buffer
	for _, r := range recs {
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}
	return writeFile(t, name, buf.Bytes())
}

func runCLI(args ...string) (code int, stdout, stderr string) {
	var o, e bytes.Buffer
	code = run(args, &o, &e)
	return code, o.String(), e.String()
}

func TestRun_ExitCodes(t *testing.T) {
	withStateError := append(revertTrace("130", "130"),
		checker.Record{Seq: 4, Point: checker.PointSnapshot, DB: "db1", Snap: 2, State: &checker.State{Error: "atomicos: read failed"}})
	cases := []struct {
		name string
		path string
		want int
		out  string // must appear in stdout
	}{
		{"no violation", writeTrace(t, "ok.jsonl", revertTrace("100", "100")), 0, "verdict: no violation"},
		{"violation", writeTrace(t, "v.jsonl", revertTrace("130", "130")), 1, "verdict: violation\n"},
		{"violation gz", writeTrace(t, "v.jsonl.gz", revertTrace("130", "130")), 1, "[VIOLATION] R1-S"},
		{"guarded then violation", writeTrace(t, "fold.jsonl", guardedThenViolation()), 1, "repeats:  1 more in the same DB (seqs 2, 5)"},
		{"violation and state.error", writeTrace(t, "vi.jsonl", withStateError), 1, "inconclusive: 1 record(s) carry state.error (first at seq 4)"},
		{"empty file", writeFile(t, "empty.jsonl", nil), 3, "inconclusive: the trace has no records"},
		{"blank lines only", writeFile(t, "blank.jsonl", []byte("\n\n")), 3, "records: 0, findings: 0"},
		{"state.error, no violation", writeTrace(t, "err.jsonl", []checker.Record{
			{Seq: 1, Point: checker.PointCommit, DB: "db1", State: &checker.State{Error: "atomicos: StateDB belongs to an unregistered app"}},
		}), 3, "verdict: inconclusive"},
		{"unknown points only", writeTrace(t, "unk.jsonl", []checker.Record{{Seq: 1, Point: "O3.enter", DB: "db1"}}), 3, "unknown point 1"},
	}
	for _, c := range cases {
		code, out, errOut := runCLI(c.path)
		if code != c.want || !strings.Contains(out, c.out) {
			t.Errorf("%s: exit %d (want %d), stdout contains %q: %v\nstdout:\n%s\nstderr:\n%s", c.name, code, c.want, c.out, strings.Contains(out, c.out), out, errOut)
		}
		// -json gives the same exit status
		if code, _, _ := runCLI("-json", c.path); code != c.want {
			t.Errorf("%s -json: exit %d, want %d", c.name, code, c.want)
		}
	}
}

func TestRun_UsageAndReadErrors(t *testing.T) {
	good := writeTrace(t, "ok.jsonl", revertTrace("100", "100"))
	for name, args := range map[string][]string{
		"no argument":    {},
		"two arguments":  {good, good},
		"unknown flag":   {"-x", good},
		"missing file":   {filepath.Join(t.TempDir(), "nope.jsonl")},
		"malformed json": {writeFile(t, "bad.jsonl", []byte("{\"seq\":1,\"point\":\"O6.commit\"}\n{bad\n"))},
		"not gzip": {func() string {
			p := filepath.Join(t.TempDir(), "plain.jsonl.gz")
			os.WriteFile(p, []byte("{\"seq\":1}\n"), 0o644)
			return p
		}()},
		"truncated gzip": {func() string {
			p := writeTrace(t, "t.jsonl.gz", revertTrace("100", "100"))
			b, _ := os.ReadFile(p)
			os.WriteFile(p, b[:len(b)/2], 0o644)
			return p
		}()},
	} {
		code, out, errOut := runCLI(args...)
		if code != 2 || errOut == "" {
			t.Errorf("%s: exit %d (want 2), stderr %q, stdout %q", name, code, errOut, out)
		}
	}
}

func TestRun_JSON(t *testing.T) {
	code, out, _ := runCLI("-json", writeFile(t, "empty.jsonl", nil))
	var v struct {
		Records      int               `json:"records"`
		Verdict      string            `json:"verdict"`
		Inconclusive []string          `json:"inconclusive"`
		Stats        checker.Stats     `json:"stats"`
		Findings     []checker.Finding `json:"findings"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if code != 3 || v.Verdict != checker.VerdictInconclusive || len(v.Inconclusive) != 1 || v.Records != 0 || v.Findings != nil {
		t.Fatalf("exit %d, %+v", code, v)
	}

	code, out, _ = runCLI("-json", writeTrace(t, "fold.jsonl", guardedThenViolation()))
	v.Findings = nil
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatal(err)
	}
	if code != 1 || v.Verdict != checker.VerdictViolation || len(v.Findings) != 1 ||
		v.Findings[0].Severity != checker.SevViolation || len(v.Findings[0].Seqs) != 2 ||
		v.Stats.R2A.Checked != 2 || v.Stats.R2A.NonZero != 2 || v.Inconclusive == nil {
		t.Fatalf("exit %d, %s", code, out)
	}
}

// cases/lib.sh and eval/fp-upstream/run.sh count findings from the text lines that start with
// "  violation ", "  guarded " or "  aux ". The stats and verdict lines must not match.
func TestRun_TextKeepsScriptContract(t *testing.T) {
	recs := append(guardedThenViolation(), revertTrace("130", "130")...)
	for i := range recs {
		recs[i].Seq = i + 1
	}
	_, out, _ := runCLI(writeTrace(t, "mix.jsonl", recs))
	re := regexp.MustCompile(`(?m)^  (violation|guarded|aux) .*$`)
	got := re.FindAllString(out, -1)
	want := []string{"  violation R1-S         1", "  violation R2-A         1"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("count lines = %q, want %q\n%s", got, want, out)
	}
	if !strings.HasPrefix(out, "records: 9, findings: 2\n") {
		t.Fatalf("first line changed:\n%s", out)
	}
}
