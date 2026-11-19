#!/usr/bin/env bash
# Builds results/coverage.md from the checker statistics in results/<suite>.check.txt
# (the "stats" block atomicos-check prints). Run after ./run.sh.
#   ./coverage.sh            -> results/coverage.md          (v0.7.4)
#   ./coverage.sh v0.5.1     -> results/v0.5.1/coverage.md   (FP_VERSION=v0.5.1 ./run.sh)
set -euo pipefail
OUT="$(cd "$(dirname "$0")" && pwd)/results${1:+/$1}"
[ -d "$OUT" ] || { echo "no results directory $OUT" >&2; exit 2; }

row() { # row <set> <suite>
  awk -v set="$1" -v suite="$2" '
    /^verdict:/                 { sub(/^verdict: */, ""); sub(/ \(.*/, ""); verdict = $0 }
    /^  R2-A +balance ops/      { for (i = 1; i <= NF; i++) { if ($i == "checked") a = $(i+1); if ($i == "amount") { nz = $(i+1); sub(/\).*/, "", nz) } } }
    /^  R2-W +comparisons/      { for (i = 1; i <= NF; i++) { if ($i == "enter" && e == "") { e = $(i+1); sub(/,/, "", e) } if ($i == "exit" && x == "") { x = $(i+1); sub(/,/, "", x) } } }
    /^  R1 +reverts/            { r = $3; sub(/:/, "", r); for (i = 1; i <= NF; i++) if ($i == "candidates") { c = $(i+1); sub(/,/, "", c) } }
    /^  R1 +commits/            { for (i = 1; i <= NF; i++) if ($i == "judged") { j = $(i+1); sub(/ .*/, "", j) } }
    /^  R3 +exits/              { for (i = 1; i <= NF; i++) { if ($i == "R3-D") { d = $(i+1); sub(/,/, "", d) } if ($i == "R3-U") { u = $(i+1); sub(/,/, "", u) } if ($i == "R3-C") { cc = $(i+1); sub(/;/, "", cc) } } }
    END { printf "| %s | %s | %s | %s (%s) | %s | %s | %s | %s | %s / %s / %s |\n", set, suite, verdict, a, nz, e + x, r, c, j, d, u, cc }
  ' "$OUT/$2.check.txt"
}

{
  echo "# 규칙이 실제로 검사된 횟수"
  echo
  echo "\`coverage.sh${1:+ $1}\`가 \`results/${1:+$1/}<suite>.check.txt\`의 판정기 통계에서 만든다."
  echo
  echo "| set | suite | 판정 | R2-A 검사 (금액이 0이 아닌 것) | R2-W 비교 | 되돌림 | R1 후보가 생긴 되돌림 | R1 판정한 커밋 | R3-D / R3-U / R3-C |"
  echo "|---|---|---|---|---|---|---|---|---|"
  for s in staking distribution; do [ -f "$OUT/$s.check.txt" ] && row A "$s"; done
  for s in bank erc20 werc20 gov slashing ics20; do [ -f "$OUT/$s.check.txt" ] && row B "$s"; done
  echo
  echo "R1 후보가 생긴 되돌림: 되돌린 직후 상태나 이벤트가 스냅샷 때와 달랐던 경우. 판정기는 이 후보가 커밋된 결과에 남는지를 본다."
} > "$OUT/coverage.md"
echo "wrote $OUT/coverage.md"
