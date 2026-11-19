#!/usr/bin/env python3
"""Checker v1 vs v2 on the same mutant runs, split into the dev and test sets (split-v2.tsv).

    compare.py <v1 results/runs> <v2 results-v2/runs> [out.md]

Only mutants finished in both directories are counted. A mutant is "실행 완료" and counted as in
report.py (RUN_ERROR and APPLY/INJECT/BUILD_FAIL excluded); detection = a violation in any suite.
"""
import collections
import os
import sys

from report import KILLED_BY_TESTS, pct, tsv


def load(rdir):
    out = {}
    for mid in os.listdir(rdir):
        p = os.path.join(rdir, mid)
        if mid == "CLEAN" or not os.path.exists(os.path.join(p, "DONE")):
            continue
        rows = tsv(os.path.join(p, "rows.tsv"))
        vs = {r[2] for r in rows}
        if "RUN_ERROR" in vs or vs & {"APPLY_FAIL", "INJECT_FAIL", "BUILD_FAIL"}:
            out[mid] = None
            continue
        out[mid] = {
            "checker": any(int(r[3]) > 0 for r in rows),
            "tests": any(r[2] in KILLED_BY_TESTS for r in rows),
            "rules": sorted({x for r in rows for x in r[4].split(",") if x != "-"}),
        }
    return out


def main():
    here = os.path.dirname(os.path.abspath(__file__))
    v1, v2 = load(sys.argv[1]), load(sys.argv[2])
    split = {r[1]: r[2] for r in tsv(os.path.join(here, "split-v2.tsv"))[1:]}
    out = []
    w = out.append
    w("# 판정기 v1 vs v2 (같은 실행의 기록을 다시 판정, compare.py)\n")
    w("| 집합 | 끝난 변이 | 실행 완료 | 판정기 v1 | 판정기 v2 | 기존 테스트 | v2 또는 기존 테스트 | v2만 새로 잡은 변이 |")
    w("|---|---|---|---|---|---|---|---|")
    newly = collections.defaultdict(list)
    for s in ["test", "dev"]:
        ids = [m for m in split if split[m] == s and m in v1 and m in v2]
        ok = [m for m in ids if v1[m] is not None and v2[m] is not None]
        n = len(ok)
        d1 = sum(v1[m]["checker"] for m in ok)
        d2 = sum(v2[m]["checker"] for m in ok)
        t = sum(v2[m]["tests"] for m in ok)
        either = sum(v2[m]["checker"] or v2[m]["tests"] for m in ok)
        newly[s] = [m for m in ok if v2[m]["checker"] and not v1[m]["checker"]]
        lost = [m for m in ok if v1[m]["checker"] and not v2[m]["checker"]]
        w("| %s | %d | %d | %s | %s | %s | %s | %d%s |" % (
            s, len(ids), n, pct(d1, n), pct(d2, n), pct(t, n), pct(either, n), len(newly[s]),
            (" (v1만 잡고 v2가 놓친 변이 %d: %s)" % (len(lost), ",".join(lost))) if lost else ""))
    w("")
    for s in ["test", "dev"]:
        if newly[s]:
            w("**%s: v2만 새로 잡은 변이** (규칙, 기존 테스트 탐지 여부)\n" % s)
            for m in sorted(newly[s]):
                w("- %s: %s; 기존 테스트 %s" % (m, ",".join(v2[m]["rules"]), "잡음" if v2[m]["tests"] else "놓침"))
            w("")
    text = "\n".join(out) + "\n"
    if len(sys.argv) > 3:
        with open(sys.argv[3], "w", encoding="utf-8", newline="\n") as fh:
            fh.write(text)
    print(text)


if __name__ == "__main__":
    main()
