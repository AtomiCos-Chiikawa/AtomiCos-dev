// Command atomicos-check judges an AtomiCos trace (JSON Lines) and prints the findings.
//
//	atomicos-check [-json] [-drop O4,O5] trace.jsonl[.gz]
//
// Exit status: 0 no violation, 1 at least one violation, 2 usage or read error.
// "guarded" (attempted but rejected) and "aux" findings do not change the exit status.
package main

import (
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"atomicos/tool/checker"
)

func main() {
	asJSON := flag.Bool("json", false, "print findings as JSON")
	drop := flag.String("drop", "", "comma-separated observation points to ignore, e.g. O4,O5 (ablation)")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: atomicos-check [-json] [-drop O4,O5] trace.jsonl")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	f, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer f.Close()
	var r io.Reader = f
	if strings.HasSuffix(flag.Arg(0), ".gz") {
		zr, err := gzip.NewReader(f)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		defer zr.Close()
		r = zr
	}
	recs, err := checker.ReadTrace(r)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *drop != "" {
		recs = dropPoints(recs, strings.Split(*drop, ","))
	}
	findings := checker.Check(recs)

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(struct {
			Records  int               `json:"records"`
			Findings []checker.Finding `json:"findings"`
		}{len(recs), findings})
	} else {
		printText(len(recs), findings)
	}
	for _, x := range findings {
		if x.Severity == checker.SevViolation {
			os.Exit(1)
		}
	}
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

func printText(n int, fs []checker.Finding) {
	count := map[string]int{}
	for _, x := range fs {
		count[x.Severity+" "+x.Rule]++
	}
	fmt.Printf("records: %d, findings: %d\n", n, len(fs))
	keys := make([]string, 0, len(count))
	for k := range count {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("  %-22s %d\n", k, count[k])
	}
	for _, x := range fs {
		loc := fmt.Sprintf("seq %d %s %s", x.Seq, x.DB, x.Point)
		if x.Call != 0 {
			loc += fmt.Sprintf(" call %d", x.Call)
		}
		if x.Snap != 0 {
			loc += fmt.Sprintf(" snap %d", x.Snap)
		}
		fmt.Printf("\n[%s] %s  (%s)\n", strings.ToUpper(x.Severity), x.Rule, loc)
		fmt.Printf("  subject:  %s\n  expected: %s\n  actual:   %s\n", x.Subject, x.Expected, x.Actual)
		if x.Detail != "" {
			fmt.Printf("  detail:   %s\n", x.Detail)
		}
		if x.Outcome != "" {
			fmt.Printf("  outcome:  %s\n", x.Outcome)
		}
		if x.Repeats > 0 {
			fmt.Printf("  repeats:  %d more in the same DB\n", x.Repeats)
		}
		for i, s := range x.Stack {
			if i >= 4 {
				break
			}
			fmt.Printf("  at %s\n", s)
		}
	}
}
