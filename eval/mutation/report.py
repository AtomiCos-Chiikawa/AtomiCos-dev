#!/usr/bin/env python3
"""Aggregate eval/mutation results into results/report.md (definitions: README.md §4).

    report.py <eval/mutation dir> [results dir name, default results]
"""
import collections
import math
import os
import sys

ABL = ["O2", "O4", "O5", "O6"]
# GHSA-8pfh is a design / sanity-check case only and is excluded from evaluation (README §3.1)
EXCLUDED = {"A02", "A03"}
KILLED_BY_TESTS = {"FAIL", "PANIC", "TIMEOUT", "MEMOUT"}


def wilson(k, n, z=1.96):
    if n == 0:
        return (0.0, 0.0)
    p = k / n
    d = 1 + z * z / n
    c = p + z * z / (2 * n)
    r = z * math.sqrt(p * (1 - p) / n + z * z / (4 * n * n))
    return (max(0.0, (c - r) / d), min(1.0, (c + r) / d))


def pct(k, n):
    if n == 0:
        return "– (0)"
    lo, hi = wilson(k, n)
    return "%d/%d = **%.0f%%** (95%% CI %.0f–%.0f%%)" % (k, n, 100 * k / n, 100 * lo, 100 * hi)


def tsv(path):
    rows = [l.rstrip("\n").split("\t") for l in open(path, encoding="utf-8") if l.strip()]
    return rows


def main():
    d = sys.argv[1]
    res = os.path.join(d, sys.argv[2] if len(sys.argv) > 2 else "results")
    mrows = tsv(os.path.join(d, "mutants.tsv"))
    mh = mrows[0]
    muts = {r[mh.index("id")]: dict(zip(mh, r)) for r in mrows[1:]}
    plan = {r[1]: r[2] for r in tsv(os.path.join(d, "plan.tsv"))[1:]}
    runs = collections.defaultdict(list)
    running = set()
    rdir = os.path.join(res, "runs")
    for mid in os.listdir(rdir) if os.path.isdir(rdir) else []:
        rows = os.path.join(rdir, mid, "rows.tsv")
        if not os.path.exists(os.path.join(rdir, mid, "DONE")):
            running.add(mid)
            continue
        if os.path.exists(rows):
            runs[mid] = tsv(rows)

    out = []
    w = out.append
    w("# 변이 평가 결과 (자동 생성: report.py)\n")
    clean = runs.get("CLEAN", [])
    if clean:
        cv = sum(int(r[3]) for r in clean)
        bad = [r[1] for r in clean if r[2] != "PASS"]
        w("**대조군 (변이 없음):** 스위트 %d개, 테스트 %s, 판정기 위반 %d건, A1 %d건\n" % (
            len(clean), "모두 PASS" if not bad else "실패: " + ",".join(bad), cv, sum(int(r[5]) for r in clean)))

    status = collections.Counter()
    final = {}
    for mid, m in muts.items():
        rs = runs.get(mid)
        if mid in EXCLUDED:
            status["제외: 8pfh 계열 (설계·동작 확인용 사례)"] += 1
            continue
        if plan.get(mid, "-") == "-":
            status["도달 못 함 (실행 안 함)"] += 1
            continue
        if not rs:
            status["실행 중 (미완료)" if mid in running else "아직 실행 안 됨"] += 1
            continue
        vs = {r[2] for r in rs}
        if "RUN_ERROR" in vs:
            status["제외: RUN_ERROR (메모리 부족 등 외부 종료)"] += 1
            continue
        if vs & {"APPLY_FAIL", "INJECT_FAIL", "BUILD_FAIL"}:
            status["제외: " + sorted(vs & {"APPLY_FAIL", "INJECT_FAIL", "BUILD_FAIL"})[0]] += 1
            continue
        status["실행 완료"] += 1
        final[mid] = {
            "tests": any(r[2] in KILLED_BY_TESTS for r in rs),
            "checker": any(int(r[3]) > 0 for r in rs),
            "a1": any(int(r[5]) > 0 for r in rs),
            "abl": {p: any(int(r[6 + i]) > 0 for r in rs) for i, p in enumerate(ABL)},
            "rules": sorted({x for r in rs for x in r[4].split(",") if x != "-"}),
            "how": sorted({r[2] for r in rs if r[2] in KILLED_BY_TESTS}),
        }
    w("## 1. 변이 현황\n")
    w("| 상태 | 개수 |\n|---|---|")
    w("| 생성 | %d |" % len(muts))
    for k in sorted(status):
        w("| %s | %d |" % (k, status[k]))
    w("")

    def table(title, keyf):
        groups = collections.defaultdict(list)
        for mid, f in final.items():
            groups[keyf(muts[mid])].append(f)
        w("### %s\n" % title)
        w("| 묶음 | 실행 | 판정기 | 기존 테스트 | 거래 전후(A1) | 기존 테스트가 놓친 것 중 판정기 |")
        w("|---|---|---|---|---|---|")
        for g in sorted(groups):
            fs = groups[g]
            n = len(fs)
            missed = [f for f in fs if not f["tests"]]
            w("| %s | %d | %s | %s | %s | %s |" % (
                g.replace("|", "\\|"), n, pct(sum(f["checker"] for f in fs), n), pct(sum(f["tests"] for f in fs), n),
                pct(sum(f["a1"] for f in fs), n), pct(sum(f["checker"] for f in missed), len(missed))))
        w("")

    n = len(final)
    fs = list(final.values())
    w("## 2. 탐지율 (실행 완료한 변이 기준)\n")
    w("| 방법 | 탐지 |\n|---|---|")
    w("| 판정기 | %s |" % pct(sum(f["checker"] for f in fs), n))
    w("| 기존 테스트 assertion | %s |" % pct(sum(f["tests"] for f in fs), n))
    w("| 거래 전후 총발행량 비교 (A1) | %s |" % pct(sum(f["a1"] for f in fs), n))
    w("| 판정기 또는 기존 테스트 | %s |" % pct(sum(f["checker"] or f["tests"] for f in fs), n))
    w("")
    both = collections.Counter((f["checker"], f["tests"]) for f in fs)
    w("| | 기존 테스트가 잡음 | 기존 테스트가 놓침 |\n|---|---|---|")
    w("| **판정기가 잡음** | %d | **%d** |" % (both[(True, True)], both[(True, False)]))
    w("| **판정기가 놓침** | %d | %d |" % (both[(False, True)], both[(False, False)]))
    w("")
    how = collections.Counter(h for f in fs for h in f["how"])
    w("기존 테스트가 잡은 방식 (한 변이가 여러 스위트에서 다르게 잡히면 각각 셈): " + (", ".join(
        "%s %d" % (k, how[k]) for k in ["FAIL", "PANIC", "TIMEOUT", "MEMOUT"] if how[k]) or "없음") + "\n")
    w("## 3. 묶음별\n")
    table("변이 묶음 (A 공개 수정 되돌리기 / B1 단순 연산자 / B2 조건부)", lambda m: m["group"])
    table("연산자", lambda m: m["group"] + " " + m["op"])
    table("미리 적은 예상 규칙", lambda m: m["expected"])
    table("파일", lambda m: m["file"])

    w("## 4. 관찰 지점 빼 보기 (같은 기록을 해당 관찰 지점 없이 다시 판정)\n")
    det = [f for f in fs if f["checker"]]
    w("판정기가 잡은 변이 %d개 중, 관찰 지점을 하나 뺐을 때도 잡히는 개수:\n" % len(det))
    w("| 뺀 관찰 지점 | 여전히 탐지 | 놓치게 되는 변이 |\n|---|---|---|")
    for p in ABL:
        k = sum(f["abl"][p] for f in det)
        w("| %s | %d/%d | %d |" % (p, k, len(det), len(det) - k))
    w("")

    w("## 5. 변이별 결과\n")
    w("| id | 묶음 | 연산자 | 문맥 | 파일:줄 | 예상 | 판정기 | 규칙 | 테스트 | A1 |")
    w("|---|---|---|---|---|---|---|---|---|---|")
    for mid in sorted(final, key=lambda x: int(muts[x]["order"])):
        m, f = muts[mid], final[mid]
        w("| %s | %s | %s | %s | %s:%s | %s | %s | %s | %s | %s |" % (
            mid, m["group"], m["op"], m["ctx"], m["file"], m["line"], m["expected"].replace("|", "\\|"),
            "✅" if f["checker"] else "–", ",".join(f["rules"]) or "–",
            "✅" if f["tests"] else "–", "✅" if f["a1"] else "–"))
    with open(os.path.join(res, "report.md"), "w", encoding="utf-8", newline="\n") as fh:
        fh.write("\n".join(out) + "\n")
    print("\n".join(out[:40]))


if __name__ == "__main__":
    main()
