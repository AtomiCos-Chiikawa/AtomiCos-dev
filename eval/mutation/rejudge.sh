#!/usr/bin/env bash
# Re-judge saved mutation traces with the checker of this commit (checker v2, README 변경 기록
# 2026-11-20). Tests are not re-run: the test verdict and run time of each row are copied from the
# source rows, only the checker columns are recomputed (full judgment and the O2/O4/O5/O6
# ablations, same line filter as run.sh checker_drop).
#
#   ./eval/mutation/rejudge.sh <source results/runs> [-j N] [id ...]
#
# Source: a results/runs directory of run.sh (only mutants with DONE are re-judged; without ids,
# all of them in split-v2.tsv order: test set first, then dev set, CLEAN first of all).
# Output: results-v2/runs/<id>/{rows.tsv,<suite>.check.txt,DONE}, same 15 columns as run.sh.
# Already re-judged mutants (DONE in results-v2) are skipped. The checker must be committed
# (frozen): the script refuses to run with uncommitted changes under tool/.
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
SRC="${1:?source results/runs directory}"; shift
JOBS=1
if [ "${1:-}" = -j ]; then JOBS="$2"; shift 2; fi
OUT="$HERE/results-v2"
ABLATIONS=(O2 O4 O5 O6)

if [ -n "$(git -C "$ROOT" status --porcelain -- tool)" ]; then
  echo "rejudge: uncommitted changes under tool/; the v2 checker must be frozen at a commit" >&2; exit 2
fi
ATOMICOS_COMMIT="$(git -C "$ROOT" rev-parse --short HEAD)"
CHECKER="$ROOT/.work/bin/atomicos-check-$ATOMICOS_COMMIT"
mkdir -p "$ROOT/.work/bin" "$OUT/runs"
[ -x "$CHECKER" ] || (cd "$ROOT/tool" && GOTOOLCHAIN=local go build -o "$CHECKER" ./checker/cmd/atomicos-check)

# --- same as run.sh (checker_drop, judge), except that the trace is a decompressed copy
checker_drop() {
  local tmp
  tmp="$(mktemp "${TMPDIR:-/tmp}/atomicos-drop.XXXXXX")"
  grep -vE "^\{\"seq\":[0-9]+,\"point\":\"$1\." "$2" > "$tmp" || true
  "$CHECKER" "$tmp" 2>&1 || true
  rm -f "$tmp"
}
judge() {
  local trace="$1" txt="$2" v rules a1 out abl st cv se why
  set +e; out="$("$CHECKER" "$trace" 2>&1)"; st=$?; set -e
  printf '%s\n' "$out" > "$txt"
  case $st in 0) cv=none ;; 1) cv=violation ;; 3) cv=inconclusive ;; *) cv="error$st" ;; esac
  v="$( (grep -E '^  violation ' <<<"$out" || true) | awk '{s+=$3} END {print s+0}')"
  rules="$( (grep -E '^  violation ' <<<"$out" || true) | awk '{printf "%s%s", (NR>1?",":""), $2}')"
  a1="$( (grep -E '^  aux +A1 ' <<<"$out" || true) | awk '{s+=$3} END {print s+0}')"
  se="$( (grep -oE 'records with state.error [0-9]+' <<<"$out" || true) | awk '{print $NF+0}' | head -1)"
  why="$( (grep -E '^  inconclusive: ' <<<"$out" || true) | sed -E 's/^  inconclusive: //; s/\(first at seq [^)]*\)//; s/[0-9]+/N/g; s/[[:space:]]+/ /g; s/ ;/;/g' | paste -sd'|' -)"
  abl=""
  if [ "$v" = 0 ]; then abl=$'0\t0\t0\t0'
  else
    for p in "${ABLATIONS[@]}"; do
      local o
      o="$(checker_drop "$p" "$trace")"
      abl+="$( (grep -E '^  violation ' <<<"$o" || true) | awk '{s+=$3} END {print s+0}')"$'\t'
    done
    abl="${abl%$'\t'}"
  fi
  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s' "$v" "${rules:--}" "$a1" "$abl" "$cv" "${se:-0}" "${why:--}"
}

rejudge_one() {
  local id="$1" src="$SRC/$1" out="$OUT/runs/$1" tmp
  [ -f "$src/DONE" ] || { echo "skip $id (not DONE in source)"; return 0; }
  [ -f "$out/DONE" ] && return 0
  rm -rf "$out"; mkdir -p "$out"
  tmp="$(mktemp -d "${TMPDIR:-/tmp}/atomicos-rejudge.XXXXXX")"
  # columns: id suite test_verdict violations rules a1 ablO2..O6 seconds atomicos_commit
  #          checker_verdict state_errors inconclusive_reasons
  while IFS=$'\t' read -r rid s verdict _v _r _a _o2 _o4 _o5 _o6 secs _c _cv _se _why; do
    if [ -s "$src/$s.trace.jsonl.gz" ]; then
      gzip -dc "$src/$s.trace.jsonl.gz" > "$tmp/t.jsonl"
      j="$(judge "$tmp/t.jsonl" "$out/$s.check.txt")"
    else
      # no trace (APPLY/INJECT/BUILD_FAIL rows, or a run without a trace): same as run.sh
      j=$'0\t-\t0\t0\t0\t0\t0\tnotrace\t0\t-'
      [ "$s" = - ] && j="$_v"$'\t'"$_r"$'\t'"$_a"$'\t'"$_o2"$'\t'"$_o4"$'\t'"$_o5"$'\t'"$_o6"$'\t'"$_cv"$'\t'"$_se"$'\t'"$_why"
    fi
    printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$rid" "$s" "$verdict" "$(cut -f1-7 <<<"$j")" \
      "$secs" "$ATOMICOS_COMMIT" "$(cut -f8-10 <<<"$j")" >> "$out/rows.tsv"
  done < "$src/rows.tsv"
  rm -rf "$tmp"
  touch "$out/DONE"
  echo "$id $(awk -F'\t' '{v+=$4} END {print v+0}' "$out/rows.tsv") violations"
}

if [ $# -gt 0 ]; then ids="$*"
else
  ids="CLEAN $(awk -F'\t' '$3=="test" {print $2}' "$HERE/split-v2.tsv") $(awk -F'\t' '$3=="dev" {print $2}' "$HERE/split-v2.tsv")"
fi
export -f rejudge_one judge checker_drop
export SRC OUT CHECKER ATOMICOS_COMMIT
export ABLATIONS_STR="${ABLATIONS[*]}"
# bash cannot export arrays: rebuild ABLATIONS inside each job
printf '%s\n' $ids | xargs -P "$JOBS" -I{} bash -c 'ABLATIONS=($ABLATIONS_STR); rejudge_one "$1"' _ {}
