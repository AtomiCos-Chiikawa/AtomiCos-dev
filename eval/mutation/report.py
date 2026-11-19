#!/usr/bin/env python3
"""Aggregate eval/mutation results into results/report.md (definitions: README.md).

Reads results/runs/<id>/rows.tsv of every mutant with a DONE marker. If results-old-harness/
exists (the run with the pre-fix harness and checker), adds a before/after comparison.

rows.tsv columns (15; the old harness run has the first 11):
  id suite test_verdict violations rules a1 ablO2 ablO4 ablO5 ablO6 seconds
  atomicos_commit checker_verdict state_errors inconclusive_reasons
"""
import collections
import math
import os
import re
import sys

ABL = ["O2", "O4", "O5", "O6"]
# GHSA-8pfh is a design / sanity-check case only and is excluded from evaluation (README §3.1)
EXCLUDED = {"A02", "A03"}
# A01 reverts the GHSA-mjfq fix, a bug family used to design the rules: reported apart from
# the performance numbers
DESIGN_SEPARATE = {"A01"}
# the first DEV_N run targets in the shuffled order were run before the harness fix and looked
# at: development set; the rest is the evaluation set (README "개발용과 평가용")
DEV_N = 105
KILLED_BY_TESTS = {"FAIL", "PANIC", "TIMEOUT", "MEMOUT"}
NOT_RUN = {"APPLY_FAIL", "INJECT_FAIL", "BUILD_FAIL"}


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


def md(s):
    return str(s).replace("|", "\\|")


def tsv(path):
    return [l.rstrip("\n").split("\t") for l in open(path, encoding="utf-8") if l.strip()]


def load_runs(rdir):
    runs, running = {}, set()
    if not os.path.isdir(rdir):
        return runs, running
    for mid in os.listdir(rdir):
        p = os.path.join(rdir, mid)
        if not os.path.isdir(p):
            continue
        if not os.path.exists(os.path.join(p, "DONE")):
            running.add(mid)
            continue
        rows = os.path.join(p, "rows.tsv")
        if os.path.exists(rows):
            runs[mid] = tsv(rows)
    return runs, running


def col(r, i, default=""):
    return r[i] if len(r) > i else default


def build_reason(path):
    """First compiler error of a failed build, without positions, numbers and identifiers."""
    if not os.path.exists(path):
        return "(build.log 없음)"
    for line in open(path, encoding="utf-8", errors="replace"):
        m = re.search(r"\.go:\d+:\d+: (.*)", line)
        if m:
            msg = m.group(1)
            msg = re.sub(r"declared and not used: .*", "declared and not used", msg)
            msg = re.sub(r'"[^"]*"', "X", msg)
            msg = re.sub(r"\d+", "N", msg)
            return msg[:90]
    return "(컴파일러 오류 줄 없음)"


def classify(rs, rundir):
    vs = {r[2] for r in rs}
    if "RUN_ERROR" in vs:
        return {"status": "제외: RUN_ERROR (메모리 부족 등 외부 종료)"}
    bad = vs & NOT_RUN
    if bad:
        k = sorted(bad)[0]
        f = {"status": "제외: " + k}
        if k == "BUILD_FAIL":
            f["build_reason"] = build_reason(os.path.join(rundir, "build.log"))
        return f
    checker = any(int(r[3]) > 0 for r in rs)
    inconcl = [r for r in rs if col(r, 12) == "inconclusive"]
    return {
        "status": "실행 완료",
        "tests": any(r[2] in KILLED_BY_TESTS for r in rs),
        "checker": checker,
        # inconclusive is never a detection; a mutant counts as inconclusive only when no suite
        # reported a violation and at least one suite's trace was inconclusive
        "inconclusive": (not checker) and bool(inconcl),
        "reasons": [x for r in inconcl for x in col(r, 14, "-").split("|") if x and x != "-"],
        "state_errors": sum(int(col(r, 13, "0") or 0) for r in rs),
        "a1": any(int(r[5]) > 0 for r in rs),
        "abl": {p: any(int(r[6 + i]) > 0 for r in rs) for i, p in enumerate(ABL)},
        "rules": sorted({x for r in rs for x in r[4].split(",") if x != "-"}),
        "how": sorted({r[2] for r in rs if r[2] in KILLED_BY_TESTS}),
        "commits": sorted({col(r, 11) for r in rs if col(r, 11)}),
    }


def main():
    sys.stdout.reconfigure(encoding="utf-8")
    d = sys.argv[1]
    res = os.path.join(d, "results")
    mrows = tsv(os.path.join(d, "mutants.tsv"))
    mh = mrows[0]
    muts = {r[mh.index("id")]: dict(zip(mh, r)) for r in mrows[1:]}
    plan_rows = tsv(os.path.join(d, "plan.tsv"))[1:]
    plan = {r[1]: r[2] for r in plan_rows}
    targets = [r[1] for r in plan_rows if r[2] != "-" and r[1] not in EXCLUDED]
    pos = {m: i + 1 for i, m in enumerate(targets)}
    split = {m: ("개발용" if pos[m] <= DEV_N else "평가용") for m in targets}

    runs, running = load_runs(os.path.join(res, "runs"))
    old_runs, _ = load_runs(os.path.join(d, "results-old-harness", "runs"))

    out = []
    w = out.append
    w("# 변이 평가 결과 (자동 생성: report.py)\n")
    w("- 탐지율의 신뢰 구간은 95% Wilson 구간이다.")
    w("- 동작이 원본과 같은 변이(등가 변이)는 걸러 내지 않았다. 탐지율은 보수적인 값이다.")
    w("- 묶음·연산자·예상 규칙·파일별 숫자는 표본이 작아 **참고용**이다.")
    w("- 판정 불가(판정기 종료 코드 3)는 탐지로 세지 않고 따로 센다.")
    w("- A01(GHSA-mjfq 수정 되돌리기)은 규칙 설계에 쓴 버그 계열이므로 성능 숫자에서 빼고 따로 적는다.")
    w("- 개발용: 섞은 실행 순서 1~%d번째(하네스 수정 전 실행에서 결과를 본 변이). 평가용: 그 뒤.\n" % DEV_N)

    # ------------------------------------------------------------ clean control
    clean = runs.get("CLEAN", [])
    w("## 1. 대조군 (변이 없는 v0.7.4, 오탐 측정)\n")
    if clean:
        w("| 묶음 | 테스트 | 판정기 위반 | 판정기 판정 | state.error | 판정 불가 사유 | 커밋 |")
        w("|---|---|---|---|---|---|---|")
        for r in clean:
            w("| %s | %s | %s | %s | %s | %s | %s |" % (r[1], r[2], r[3], col(r, 12, "?"), col(r, 13, "?"),
                                                 md(col(r, 14, "-")), col(r, 11, "?")))
        w("\n**판정기 위반 합계(오탐): %d건**, A1 %d건\n" % (sum(int(r[3]) for r in clean), sum(int(r[5]) for r in clean)))
    else:
        w("(아직 없음)\n")

    # ------------------------------------------------------------ status
    status = collections.Counter()
    final = {}
    build_reasons = collections.Counter()
    for mid in muts:
        if mid in EXCLUDED:
            status["제외: 8pfh 계열 (설계·동작 확인용 사례)"] += 1
            continue
        if plan.get(mid, "-") == "-":
            status["도달 못 함 (실행 안 함)"] += 1
            continue
        rs = runs.get(mid)
        if not rs:
            status["실행 중 (미완료)" if mid in running else "아직 실행 안 됨"] += 1
            continue
        f = classify(rs, os.path.join(res, "runs", mid))
        status[f["status"]] += 1
        if "build_reason" in f:
            build_reasons[f["build_reason"]] += 1
        if f["status"] == "실행 완료":
            final[mid] = f
    done_pos = sorted(pos[m] for m in final) or [0]
    w("## 2. 변이 현황\n")
    w("| 상태 | 개수 |\n|---|---|")
    w("| 생성 | %d |" % len(muts))
    for k in sorted(status):
        w("| %s | %d |" % (k, status[k]))
    w("\n실행 대상 %d개 중 섞은 순서로 %d번째까지 시작했다(우선 실행한 변이 포함).\n" % (len(targets), max(done_pos)))
    commits = sorted({c for f in final.values() for c in f["commits"]} | {col(r, 11) for r in clean if col(r, 11)})
    w("결과에 기록된 AtomiCos 커밋: %s\n" % (", ".join(commits) or "(없음)"))
    w("### 빌드 실패 사유\n")
    if build_reasons:
        w("| 사유 (첫 컴파일 오류) | 개수 |\n|---|---|")
        for k, n in build_reasons.most_common():
            w("| %s | %d |" % (md(k), n))
    else:
        w("없음")
    w("")
    w("### 판정 불가 사유 (묶음 단위, 위반 없이 판정 불가로 끝난 변이 %d개)\n" % sum(f["inconclusive"] for f in final.values()))
    reasons = collections.Counter(x for f in final.values() for x in f["reasons"])
    if reasons:
        w("| 사유 | 묶음 수 |\n|---|---|")
        for k, n in reasons.most_common():
            w("| %s | %d |" % (md(k), n))
    else:
        w("없음")
    w("")

    # ------------------------------------------------------------ detection
    def summary(title, mids):
        fs = [final[m] for m in mids]
        n = len(fs)
        w("### %s (실행 완료 %d개)\n" % (title, n))
        w("| 방법 | 탐지 |\n|---|---|")
        w("| 판정기 | %s |" % pct(sum(f["checker"] for f in fs), n))
        w("| 기존 테스트 assertion | %s |" % pct(sum(f["tests"] for f in fs), n))
        w("| 거래 전후 총발행량 비교(A1) | %s |" % pct(sum(f["a1"] for f in fs), n))
        w("| 판정기 또는 기존 테스트 | %s |" % pct(sum(f["checker"] or f["tests"] for f in fs), n))
        w("")
        both = collections.Counter((f["checker"], f["tests"]) for f in fs)
        w("| 판정기 × 기존 테스트 | 개수 |\n|---|---|")
        w("| 둘 다 잡음 | %d |" % both[(True, True)])
        w("| **판정기만 잡음** | **%d** |" % both[(True, False)])
        w("| 테스트만 잡음 | %d |" % both[(False, True)])
        w("| 둘 다 못 잡음 | %d |" % both[(False, False)])
        w("")
        how = collections.Counter(h for f in fs for h in f["how"])
        w("기존 테스트가 잡은 방식 (여러 묶음에서 다르게 잡히면 각각 셈): " + (", ".join(
            "%s %d" % (k, how[k]) for k in ["FAIL", "PANIC", "TIMEOUT", "MEMOUT"] if how[k]) or "없음"))
        w("판정기 판정 불가로 끝난 변이(탐지로 세지 않음): %d개\n" % sum(f["inconclusive"] for f in fs))
        only = [m for m in mids if final[m]["checker"] and not final[m]["tests"]]
        if only:
            w("판정기만 잡은 변이: " + ", ".join("%s(%s)" % (m, ",".join(final[m]["rules"])) for m in only) + "\n")
        det = [final[m] for m in mids if final[m]["checker"]]
        w("관찰 지점 하나를 빼고 다시 판정 (판정기가 잡은 %d개 기준):\n" % len(det))
        w("| 뺀 관찰 지점 | 여전히 탐지 | 놓치게 되는 변이 |\n|---|---|---|")
        for p in ABL:
            k = sum(f["abl"][p] for f in det)
            w("| %s | %d/%d | %d |" % (p, k, len(det), len(det) - k))
        w("")

    def table(title, mids, keyf):
        groups = collections.defaultdict(list)
        for m in mids:
            groups[keyf(muts[m])].append(final[m])
        w("#### %s (참고용)\n" % title)
        w("| 묶음 | 실행 | 판정기 | 기존 테스트 | 거래 전후 총발행량 비교(A1) | 기존 테스트가 놓친 것 중 판정기 |")
        w("|---|---|---|---|---|---|")
        for g in sorted(groups):
            fs = groups[g]
            n = len(fs)
            missed = [f for f in fs if not f["tests"]]
            w("| %s | %d | %s | %s | %s | %s |" % (
                md(g), n, pct(sum(f["checker"] for f in fs), n), pct(sum(f["tests"] for f in fs), n),
                pct(sum(f["a1"] for f in fs), n), pct(sum(f["checker"] for f in missed), len(missed))))
        w("")

    perf = [m for m in final if m not in DESIGN_SEPARATE]
    ev = sorted([m for m in perf if split[m] == "평가용"], key=pos.get)
    dv = sorted([m for m in perf if split[m] == "개발용"], key=pos.get)
    w("## 3. 탐지력과 비교 (A01 제외)\n")
    summary("평가용", ev)
    summary("개발용", dv)
    summary("전체 (개발용 + 평가용)", ev + dv)
    w("## 4. A01 (GHSA-mjfq 수정 되돌리기, 설계에 쓴 계열이라 따로)\n")
    for m in sorted(DESIGN_SEPARATE):
        if m in final:
            f = final[m]
            w("- %s: 판정기 %s (%s), 기존 테스트 %s, A1 %s, 판정 불가 %s\n" % (
                m, "잡음" if f["checker"] else "못 잡음", ",".join(f["rules"]) or "-",
                "잡음" if f["tests"] else "못 잡음", "잡음" if f["a1"] else "못 잡음", "예" if f["inconclusive"] else "아니오"))
        else:
            w("- %s: 아직 결과 없음\n" % m)

    w("## 5. 나눠 보기 (참고용)\n")
    for title, mids in [("평가용", ev), ("개발용", dv)]:
        w("### %s\n" % title)
        table("변이 묶음 (A / B1 / B2)", mids, lambda m: m["group"])
        table("연산자", mids, lambda m: m["group"] + " " + m["op"])
        table("미리 적은 예상 규칙", mids, lambda m: m["expected"])
        table("파일", mids, lambda m: m["file"])

    # ------------------------------------------------------------ before / after
    w("## 6. 하네스·판정기 수정 전과 후 비교\n")
    if old_runs:
        oldf = {}
        for m, rs in old_runs.items():
            if m in muts and m not in EXCLUDED:
                f = classify(rs, "")
                if f["status"] == "실행 완료":
                    oldf[m] = f
        common = sorted(set(oldf) & set(final), key=lambda m: pos.get(m, 0))
        w("수정 전(results-old-harness)과 수정 후 모두 실행을 마친 변이 %d개 (A01 포함).\n" % len(common))
        w("| | 수정 전 | 수정 후 |\n|---|---|---|")
        for k, name in [("checker", "판정기"), ("tests", "기존 테스트"), ("a1", "거래 전후 총발행량 비교(A1)")]:
            w("| %s 탐지 | %d | %d |" % (name, sum(oldf[m][k] for m in common), sum(final[m][k] for m in common)))
        w("| 판정기 판정 불가 | (구분 없음) | %d |" % sum(final[m]["inconclusive"] for m in common))
        w("")
        changed = [m for m in common if oldf[m]["checker"] != final[m]["checker"] or oldf[m]["tests"] != final[m]["tests"]]
        w("탐지가 달라진 변이 %d개:\n" % len(changed))
        if changed:
            w("| id | 묶음 | 판정기 전 → 후 | 기존 테스트 전 → 후 | 수정 후 규칙 | 수정 후 판정 불가 |")
            w("|---|---|---|---|---|---|")
            yn = lambda b: "✅" if b else "–"
            for m in changed:
                o, f = oldf[m], final[m]
                w("| %s | %s | %s → %s | %s → %s | %s | %s |" % (
                    m, split.get(m, "-"), yn(o["checker"]), yn(f["checker"]), yn(o["tests"]), yn(f["tests"]),
                    ",".join(f["rules"]) or "–", "예" if f["inconclusive"] else "–"))
        w("")
    else:
        w("(results-old-harness 없음)\n")

    # ------------------------------------------------------------ per mutant
    w("## 7. 변이별 결과\n")
    w("| 순서 | id | 구분 | 묶음 | 연산자 | 문맥 | 파일:줄 | 예상 | 판정기 | 규칙 | 테스트 | A1 |")
    w("|---|---|---|---|---|---|---|---|---|---|---|---|")
    for m in sorted(final, key=lambda x: pos[x]):
        mm, f = muts[m], final[m]
        cv = "✅" if f["checker"] else ("판정 불가" if f["inconclusive"] else "–")
        w("| %d | %s | %s | %s | %s | %s | %s:%s | %s | %s | %s | %s | %s |" % (
            pos[m], m, "따로(A01)" if m in DESIGN_SEPARATE else split[m], mm["group"], mm["op"], mm["ctx"],
            mm["file"], mm["line"], md(mm["expected"]), cv, ",".join(f["rules"]) or "–",
            "✅" if f["tests"] else "–", "✅" if f["a1"] else "–"))
    with open(os.path.join(res, "report.md"), "w", encoding="utf-8", newline="\n") as fh:
        fh.write("\n".join(out) + "\n")
    print("\n".join(out[:60]))


if __name__ == "__main__":
    main()
