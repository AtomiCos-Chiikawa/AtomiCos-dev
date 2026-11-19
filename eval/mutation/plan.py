#!/usr/bin/env python3
"""Map each mutant to the upstream suites whose clean run executes the mutated lines.

    python3 plan.py <mutation dir>

Reads <dir>/mutants.tsv, <dir>/patches/*.patch and <dir>/results/coverage/<suite>.out
(go test -coverprofile of the clean tree), writes <dir>/plan.tsv:
    order  id  suites            (comma list, or "-" when no suite reaches the mutant)
"""
import collections
import os
import re
import sys

MODULE = "github.com/cosmos/evm/"


def old_lines(patch):
    """Line numbers of the original file that a patch removes or touches."""
    out = set()
    old = None
    last_ctx = None
    for line in patch.splitlines():
        m = re.match(r"^@@ -(\d+)(?:,\d+)? \+\d+(?:,\d+)? @@", line)
        if m:
            old = int(m.group(1))
            continue
        if old is None or line.startswith(("---", "+++")):
            continue
        if line.startswith("-"):
            out.add(old)
            old += 1
        elif line.startswith("+"):
            if last_ctx is not None:
                out.add(last_ctx)  # pure insertion: attribute to the line before it
        else:
            last_ctx = old
            old += 1
    return out


def load_cov(path):
    """file -> list of (start, end, count)."""
    blocks = collections.defaultdict(list)
    with open(path) as f:
        for line in f:
            m = re.match(r"^(.+?):(\d+)\.\d+,(\d+)\.\d+ \d+ (\d+)$", line.strip())
            if not m or not m.group(1).startswith(MODULE):
                continue
            blocks[m.group(1)[len(MODULE):]].append((int(m.group(2)), int(m.group(3)), int(m.group(4))))
    return blocks


def main():
    d = sys.argv[1]
    covdir = os.path.join(d, "results", "coverage")
    suites = sorted(f[:-4] for f in os.listdir(covdir) if f.endswith(".out"))
    cov = {s: load_cov(os.path.join(covdir, s + ".out")) for s in suites}
    rows = [l.rstrip("\n").split("\t") for l in open(os.path.join(d, "mutants.tsv"), encoding="utf-8")]
    head, rows = rows[0], rows[1:]
    iid, ifile = head.index("id"), head.index("file")
    reached = 0
    with open(os.path.join(d, "plan.tsv"), "w", encoding="utf-8", newline="\n") as out:
        out.write("order\tid\tsuites\n")
        for r in rows:
            patch = open(os.path.join(d, "patches", r[iid] + ".patch"), encoding="utf-8").read()
            lines = old_lines(patch)
            hit = []
            for s in suites:
                for (a, b, n) in cov[s].get(r[ifile], []):
                    if n > 0 and any(a <= x <= b for x in lines):
                        hit.append(s)
                        break
            reached += bool(hit)
            out.write("%s\t%s\t%s\n" % (r[0], r[iid], ",".join(hit) or "-"))
    print("plan: %d mutants, %d reached by at least one suite, %d unreached" % (len(rows), reached, len(rows) - reached))


if __name__ == "__main__":
    main()
