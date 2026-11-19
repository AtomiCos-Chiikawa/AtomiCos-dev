# 변이 평가 결과 (자동 생성: report.py)

**대조군 (변이 없음):** 스위트 8개, 테스트 모두 PASS, 판정기 위반 0건, A1 0건

## 1. 변이 현황

| 상태 | 개수 |
|---|---|
| 생성 | 398 |
| 도달 못 함 (실행 안 함) | 152 |
| 실행 완료 | 223 |
| 제외: 8pfh 계열 (설계·동작 확인용 사례) | 2 |
| 제외: BUILD_FAIL | 21 |

## 2. 탐지율 (실행 완료한 변이 기준)

| 방법 | 탐지 |
|---|---|
| 판정기 | 38/223 = **17%** (95% CI 13–23%) |
| 기존 테스트 assertion | 103/223 = **46%** (95% CI 40–53%) |
| 거래 전후 총발행량 비교 (A1) | 0/223 = **0%** (95% CI 0–2%) |
| 판정기 또는 기존 테스트 | 107/223 = **48%** (95% CI 42–55%) |

| | 기존 테스트가 잡음 | 기존 테스트가 놓침 |
|---|---|---|
| **판정기가 잡음** | 34 | **4** |
| **판정기가 놓침** | 69 | 116 |

기존 테스트가 잡은 방식 (한 변이가 여러 스위트에서 다르게 잡히면 각각 셈): FAIL 103, MEMOUT 2

## 3. 묶음별

### 변이 묶음 (A 공개 수정 되돌리기 / B1 단순 연산자 / B2 조건부)

| 묶음 | 실행 | 판정기 | 기존 테스트 | 거래 전후(A1) | 기존 테스트가 놓친 것 중 판정기 |
|---|---|---|---|---|---|
| A | 8 | 2/8 = **25%** (95% CI 7–59%) | 2/8 = **25%** (95% CI 7–59%) | 0/8 = **0%** (95% CI 0–32%) | 1/6 = **17%** (95% CI 3–56%) |
| B1 | 162 | 25/162 = **15%** (95% CI 11–22%) | 89/162 = **55%** (95% CI 47–62%) | 0/162 = **0%** (95% CI 0–2%) | 2/73 = **3%** (95% CI 1–9%) |
| B2 | 53 | 11/53 = **21%** (95% CI 12–33%) | 12/53 = **23%** (95% CI 13–36%) | 0/53 = **0%** (95% CI 0–7%) | 1/41 = **2%** (95% CI 0–13%) |

### 연산자

| 묶음 | 실행 | 판정기 | 기존 테스트 | 거래 전후(A1) | 기존 테스트가 놓친 것 중 판정기 |
|---|---|---|---|---|---|
| A FIX-REVERT | 8 | 2/8 = **25%** (95% CI 7–59%) | 2/8 = **25%** (95% CI 7–59%) | 0/8 = **0%** (95% CI 0–32%) | 1/6 = **17%** (95% CI 3–56%) |
| B1 DEL | 40 | 8/40 = **20%** (95% CI 10–35%) | 21/40 = **52%** (95% CI 37–67%) | 0/40 = **0%** (95% CI 0–9%) | 0/19 = **0%** (95% CI 0–17%) |
| B1 ERR | 19 | 5/19 = **26%** (95% CI 12–49%) | 14/19 = **74%** (95% CI 51–88%) | 0/19 = **0%** (95% CI 0–17%) | 2/5 = **40%** (95% CI 12–77%) |
| B1 GUARD | 91 | 4/91 = **4%** (95% CI 2–11%) | 42/91 = **46%** (95% CI 36–56%) | 0/91 = **0%** (95% CI 0–4%) | 0/49 = **0%** (95% CI 0–7%) |
| B1 LOCK | 3 | 2/3 = **67%** (95% CI 21–94%) | 3/3 = **100%** (95% CI 44–100%) | 0/3 = **0%** (95% CI 0–56%) | – (0) |
| B1 SWAP | 9 | 6/9 = **67%** (95% CI 35–88%) | 9/9 = **100%** (95% CI 70–100%) | 0/9 = **0%** (95% CI 0–30%) | – (0) |
| B2 DEL | 17 | 6/17 = **35%** (95% CI 17–59%) | 7/17 = **41%** (95% CI 22–64%) | 0/17 = **0%** (95% CI 0–18%) | 0/10 = **0%** (95% CI 0–28%) |
| B2 ERR | 14 | 2/14 = **14%** (95% CI 4–40%) | 2/14 = **14%** (95% CI 4–40%) | 0/14 = **0%** (95% CI 0–22%) | 1/12 = **8%** (95% CI 1–35%) |
| B2 GUARD | 22 | 3/22 = **14%** (95% CI 5–33%) | 3/22 = **14%** (95% CI 5–33%) | 0/22 = **0%** (95% CI 0–15%) | 0/19 = **0%** (95% CI 0–17%) |

### 미리 적은 예상 규칙

| 묶음 | 실행 | 판정기 | 기존 테스트 | 거래 전후(A1) | 기존 테스트가 놓친 것 중 판정기 |
|---|---|---|---|---|---|
| - | 66 | 2/66 = **3%** (95% CI 1–10%) | 32/66 = **48%** (95% CI 37–60%) | 0/66 = **0%** (95% CI 0–6%) | 0/34 = **0%** (95% CI 0–10%) |
| R1 | 19 | 8/19 = **42%** (95% CI 23–64%) | 8/19 = **42%** (95% CI 23–64%) | 0/19 = **0%** (95% CI 0–17%) | 1/11 = **9%** (95% CI 2–38%) |
| R1-S | 1 | 0/1 = **0%** (95% CI 0–79%) | 0/1 = **0%** (95% CI 0–79%) | 0/1 = **0%** (95% CI 0–79%) | 0/1 = **0%** (95% CI 0–79%) |
| R1-S\|R2-W | 1 | 0/1 = **0%** (95% CI 0–79%) | 0/1 = **0%** (95% CI 0–79%) | 0/1 = **0%** (95% CI 0–79%) | 0/1 = **0%** (95% CI 0–79%) |
| R1\|R2-W | 27 | 7/27 = **26%** (95% CI 13–45%) | 6/27 = **22%** (95% CI 11–41%) | 0/27 = **0%** (95% CI 0–12%) | 2/21 = **10%** (95% CI 3–29%) |
| R2-A | 20 | 0/20 = **0%** (95% CI 0–16%) | 0/20 = **0%** (95% CI 0–16%) | 0/20 = **0%** (95% CI 0–16%) | 0/20 = **0%** (95% CI 0–16%) |
| R2-W | 66 | 20/66 = **30%** (95% CI 21–42%) | 40/66 = **61%** (95% CI 49–71%) | 0/66 = **0%** (95% CI 0–6%) | 1/26 = **4%** (95% CI 1–19%) |
| R3 | 23 | 1/23 = **4%** (95% CI 1–21%) | 17/23 = **74%** (95% CI 54–87%) | 0/23 = **0%** (95% CI 0–14%) | 0/6 = **0%** (95% CI 0–39%) |

### 파일

| 묶음 | 실행 | 판정기 | 기존 테스트 | 거래 전후(A1) | 기존 테스트가 놓친 것 중 판정기 |
|---|---|---|---|---|---|
| precompiles/bank/bank.go | 1 | 0/1 = **0%** (95% CI 0–79%) | 0/1 = **0%** (95% CI 0–79%) | 0/1 = **0%** (95% CI 0–79%) | 0/1 = **0%** (95% CI 0–79%) |
| precompiles/common/balance_handler.go | 5 | 2/5 = **40%** (95% CI 12–77%) | 4/5 = **80%** (95% CI 38–96%) | 0/5 = **0%** (95% CI 0–43%) | 0/1 = **0%** (95% CI 0–79%) |
| precompiles/common/precompile.go | 35 | 8/35 = **23%** (95% CI 12–39%) | 11/35 = **31%** (95% CI 19–48%) | 0/35 = **0%** (95% CI 0–10%) | 3/24 = **12%** (95% CI 4–31%) |
| precompiles/common/utils.go | 2 | 0/2 = **0%** (95% CI 0–66%) | 0/2 = **0%** (95% CI 0–66%) | 0/2 = **0%** (95% CI 0–66%) | 0/2 = **0%** (95% CI 0–66%) |
| precompiles/distribution/tx.go | 11 | 0/11 = **0%** (95% CI 0–26%) | 8/11 = **73%** (95% CI 43–90%) | 0/11 = **0%** (95% CI 0–26%) | 0/3 = **0%** (95% CI 0–56%) |
| precompiles/erc20/approve.go | 1 | 0/1 = **0%** (95% CI 0–79%) | 0/1 = **0%** (95% CI 0–79%) | 0/1 = **0%** (95% CI 0–79%) | 0/1 = **0%** (95% CI 0–79%) |
| precompiles/erc20/tx.go | 3 | 0/3 = **0%** (95% CI 0–56%) | 3/3 = **100%** (95% CI 44–100%) | 0/3 = **0%** (95% CI 0–56%) | – (0) |
| precompiles/gov/tx.go | 8 | 0/8 = **0%** (95% CI 0–32%) | 6/8 = **75%** (95% CI 41–93%) | 0/8 = **0%** (95% CI 0–32%) | 0/2 = **0%** (95% CI 0–66%) |
| precompiles/ics20/tx.go | 13 | 0/13 = **0%** (95% CI 0–23%) | 0/13 = **0%** (95% CI 0–23%) | 0/13 = **0%** (95% CI 0–23%) | 0/13 = **0%** (95% CI 0–23%) |
| precompiles/slashing/tx.go | 3 | 0/3 = **0%** (95% CI 0–56%) | 1/3 = **33%** (95% CI 6–79%) | 0/3 = **0%** (95% CI 0–56%) | 0/2 = **0%** (95% CI 0–66%) |
| precompiles/staking/tx.go | 12 | 1/12 = **8%** (95% CI 1–35%) | 9/12 = **75%** (95% CI 47–91%) | 0/12 = **0%** (95% CI 0–24%) | 0/3 = **0%** (95% CI 0–56%) |
| precompiles/werc20/tx.go | 2 | 0/2 = **0%** (95% CI 0–66%) | 1/2 = **50%** (95% CI 9–91%) | 0/2 = **0%** (95% CI 0–66%) | 0/1 = **0%** (95% CI 0–79%) |
| x/vm/keeper/statedb.go | 18 | 4/18 = **22%** (95% CI 9–45%) | 13/18 = **72%** (95% CI 49–88%) | 0/18 = **0%** (95% CI 0–18%) | 0/5 = **0%** (95% CI 0–43%) |
| x/vm/statedb/journal.go | 10 | 3/10 = **30%** (95% CI 11–60%) | 4/10 = **40%** (95% CI 17–69%) | 0/10 = **0%** (95% CI 0–28%) | 0/6 = **0%** (95% CI 0–39%) |
| x/vm/statedb/state_object.go | 35 | 6/35 = **17%** (95% CI 8–33%) | 13/35 = **37%** (95% CI 23–54%) | 0/35 = **0%** (95% CI 0–10%) | 0/22 = **0%** (95% CI 0–15%) |
| x/vm/statedb/statedb.go | 64 | 14/64 = **22%** (95% CI 14–33%) | 30/64 = **47%** (95% CI 35–59%) | 0/64 = **0%** (95% CI 0–6%) | 1/34 = **3%** (95% CI 1–15%) |

## 4. 관찰 지점 빼 보기 (같은 기록을 해당 관찰 지점 없이 다시 판정)

판정기가 잡은 변이 38개 중, 관찰 지점을 하나 뺐을 때도 잡히는 개수:

| 뺀 관찰 지점 | 여전히 탐지 | 놓치게 되는 변이 |
|---|---|---|
| O2 | 23/38 | 15 |
| O4 | 37/38 | 1 |
| O5 | 35/38 | 3 |
| O6 | 24/38 | 14 |

## 5. 변이별 결과

| id | 묶음 | 연산자 | 문맥 | 파일:줄 | 예상 | 판정기 | 규칙 | 테스트 | A1 |
|---|---|---|---|---|---|---|---|---|---|
| A01 | A | FIX-REVERT | - | precompiles/common/precompile.go:0 | R1 | ✅ | R1-E,R1-S,R2-W | – | – |
| A04 | A | FIX-REVERT | - | x/vm/keeper/statedb.go:0 | R2-W | ✅ | R2-W | ✅ | – |
| A05 | A | FIX-REVERT | - | x/vm/keeper/statedb.go:0 | R2-W | – | – | ✅ | – |
| A06 | A | FIX-REVERT | - | x/vm/statedb/state_object.go:0 | R2-A | – | – | – | – |
| A07 | A | FIX-REVERT | - | x/vm/statedb/statedb.go:0 | R1-S | – | – | – | – |
| A08 | A | FIX-REVERT | - | x/vm/statedb/statedb.go:0 | R1-S\|R2-W | – | – | – | – |
| A09 | A | FIX-REVERT | - | x/vm/statedb/state_object.go:0 | R2-A | – | – | – | – |
| A10 | A | FIX-REVERT | - | x/vm/statedb/statedb.go:0 | - | – | – | – | – |
| M385 | B1 | GUARD | - | precompiles/ics20/tx.go:111 | - | – | – | – | – |
| M149 | B1 | DEL | - | x/vm/statedb/statedb.go:676 | - | – | – | – | – |
| C114 | B2 | GUARD | NEST | x/vm/statedb/statedb.go:467 | R1 | – | – | – | – |
| M065 | B1 | GUARD | - | precompiles/common/balance_handler.go:96 | R2-W | – | – | ✅ | – |
| M202 | B1 | SWAP | - | x/vm/statedb/state_object.go:142 | R2-W | ✅ | R2-W | ✅ | – |
| M379 | B1 | GUARD | - | precompiles/ics20/tx.go:51 | - | – | – | – | – |
| M246 | B1 | DEL | - | x/vm/keeper/statedb.go:218 | R2-W | ✅ | R2-W | ✅ | – |
| M240 | B1 | GUARD | - | x/vm/keeper/statedb.go:188 | R2-W | – | – | ✅ | – |
| M303 | B1 | GUARD | - | precompiles/distribution/tx.go:50 | R3 | – | – | – | – |
| M230 | B1 | DEL | - | x/vm/statedb/state_object.go:300 | R2-W | – | – | ✅ | – |
| C175 | B2 | DEL | NEST | x/vm/statedb/statedb.go:777 | R2-W | – | – | – | – |
| M103 | B1 | GUARD | - | x/vm/statedb/statedb.go:387 | - | ✅ | R2-W | ✅ | – |
| M196 | B1 | GUARD | - | x/vm/statedb/state_object.go:54 | R2-W | – | – | ✅ | – |
| M283 | B1 | GUARD | - | precompiles/staking/tx.go:171 | R3 | – | – | ✅ | – |
| M329 | B1 | ERR | - | precompiles/distribution/tx.go:231 | R3 | – | – | – | – |
| M206 | B1 | GUARD | - | x/vm/statedb/state_object.go:158 | R2-A | – | – | – | – |
| M349 | B1 | GUARD | - | precompiles/werc20/tx.go:71 | - | – | – | ✅ | – |
| C119 | B2 | GUARD | NEST | x/vm/statedb/statedb.go:500 | R2-A | – | – | – | – |
| M226 | B1 | GUARD | - | x/vm/statedb/state_object.go:268 | R2-W | – | – | – | – |
| M233 | B1 | GUARD | - | x/vm/keeper/statedb.go:61 | R2-W | – | – | – | – |
| M315 | B1 | GUARD | - | precompiles/distribution/tx.go:130 | R3 | – | – | ✅ | – |
| M194 | B1 | DEL | - | x/vm/statedb/journal.go:281 | R1 | – | – | – | – |
| M092 | B1 | DEL | - | x/vm/statedb/statedb.go:272 | - | – | – | – | – |
| M118 | B1 | GUARD | - | x/vm/statedb/statedb.go:500 | R2-A | – | – | – | – |
| M012 | B1 | LOCK | - | x/vm/keeper/statedb.go:0 | R2-W | ✅ | R2-W | ✅ | – |
| M288 | B1 | GUARD | - | precompiles/staking/tx.go:218 | R3 | – | – | ✅ | – |
| M100 | B1 | GUARD | - | x/vm/statedb/statedb.go:348 | - | – | – | ✅ | – |
| C153 | B2 | DEL | NEST | x/vm/statedb/statedb.go:714 | R1 | ✅ | R1-E,R1-S | ✅ | – |
| M188 | B1 | DEL | - | x/vm/statedb/journal.go:197 | R1 | – | – | – | – |
| M088 | B1 | GUARD | - | x/vm/statedb/statedb.go:234 | - | – | – | – | – |
| M381 | B1 | GUARD | - | precompiles/ics20/tx.go:64 | - | – | – | – | – |
| M191 | B1 | DEL | - | x/vm/statedb/journal.go:232 | R1 | – | – | – | – |
| C204 | B2 | GUARD | NEST | x/vm/statedb/state_object.go:143 | R2-A | – | – | – | – |
| M095 | B1 | GUARD | - | x/vm/statedb/statedb.go:302 | - | ✅ | R2-A | ✅ | – |
| M154 | B1 | GUARD | - | x/vm/statedb/statedb.go:723 | R2-W | ✅ | R1-C | ✅ | – |
| M293 | B1 | GUARD | - | precompiles/staking/tx.go:268 | R3 | – | – | ✅ | – |
| M016 | B1 | GUARD | - | precompiles/common/precompile.go:50 | - | – | – | ✅ | – |
| C048 | B2 | ERR | LOWGAS | precompiles/common/precompile.go:122 | R1\|R2-W | – | – | – | – |
| M141 | B1 | DEL | - | x/vm/statedb/statedb.go:636 | - | – | – | – | – |
| C205 | B2 | GUARD | LOCKED | x/vm/statedb/state_object.go:143 | R2-A | – | – | – | – |
| M284 | B1 | ERR | - | precompiles/staking/tx.go:177 | R3 | ✅ | R3-D | ✅ | – |
| C036 | B2 | DEL | LOWGAS | precompiles/common/precompile.go:106 | R1\|R2-W | – | – | – | – |
| C200 | B2 | GUARD | NEST | x/vm/statedb/state_object.go:138 | R2-A | – | – | – | – |
| M337 | B1 | GUARD | - | precompiles/erc20/tx.go:96 | - | – | – | ✅ | – |
| C183 | B2 | DEL | NEST | x/vm/statedb/statedb.go:791 | R2-W | – | – | – | – |
| M372 | B1 | GUARD | - | precompiles/slashing/tx.go:36 | - | – | – | – | – |
| M247 | B1 | ERR | - | x/vm/keeper/statedb.go:221 | R2-W | – | – | ✅ | – |
| M353 | B1 | ERR | - | precompiles/gov/tx.go:47 | - | – | – | ✅ | – |
| M384 | B1 | GUARD | - | precompiles/ics20/tx.go:105 | - | – | – | – | – |
| M107 | B1 | DEL | - | x/vm/statedb/statedb.go:415 | - | – | – | ✅ | – |
| M346 | B1 | GUARD | - | precompiles/erc20/approve.go:78 | - | – | – | – | – |
| M336 | B1 | SWAP | - | precompiles/erc20/tx.go:95 | - | – | – | ✅ | – |
| M032 | B1 | DEL | - | precompiles/common/precompile.go:98 | R1\|R2-W | – | – | – | – |
| M106 | B1 | DEL | - | x/vm/statedb/statedb.go:414 | - | – | – | – | – |
| C039 | B2 | ERR | LOWGAS | precompiles/common/precompile.go:111 | R1\|R2-W | – | – | ✅ | – |
| M069 | B1 | GUARD | - | precompiles/common/utils.go:19 | R2-W | – | – | – | – |
| C028 | B2 | ERR | CONTRACT | precompiles/common/precompile.go:84 | R1\|R2-W | ✅ | R2-W | – | – |
| M150 | B1 | GUARD | - | x/vm/statedb/statedb.go:707 | R1 | – | – | – | – |
| C177 | B2 | DEL | NEST | x/vm/statedb/statedb.go:779 | R2-W | – | – | – | – |
| M227 | B1 | GUARD | - | x/vm/statedb/state_object.go:274 | R2-W | – | – | – | – |
| M123 | B1 | GUARD | - | x/vm/statedb/statedb.go:512 | R2-A | – | – | – | – |
| C161 | B2 | ERR | NEST | x/vm/statedb/statedb.go:736 | R2-W | – | – | – | – |
| M078 | B1 | DEL | - | x/vm/statedb/statedb.go:124 | - | – | – | – | – |
| M121 | B1 | GUARD | - | x/vm/statedb/statedb.go:509 | R2-A | – | – | – | – |
| M058 | B1 | GUARD | - | precompiles/common/balance_handler.go:69 | R2-W | – | – | – | – |
| M060 | B1 | GUARD | - | precompiles/common/balance_handler.go:79 | R2-W | – | – | ✅ | – |
| M212 | B1 | SWAP | - | x/vm/statedb/state_object.go:170 | R2-W | ✅ | R2-W | ✅ | – |
| M113 | B1 | GUARD | - | x/vm/statedb/statedb.go:467 | R1 | – | – | – | – |
| C163 | B2 | DEL | NEST | x/vm/statedb/statedb.go:738 | R2-W | – | – | – | – |
| C208 | B2 | GUARD | LOCKED | x/vm/statedb/state_object.go:158 | R2-A | – | – | – | – |
| C215 | B2 | DEL | LOCKED | x/vm/statedb/state_object.go:177 | R2-W | – | – | – | – |
| M298 | B1 | GUARD | - | precompiles/staking/tx.go:316 | R3 | – | – | – | – |
| M035 | B1 | DEL | - | precompiles/common/precompile.go:106 | R1\|R2-W | – | – | – | – |
| M244 | B1 | DEL | - | x/vm/keeper/statedb.go:214 | R2-W | – | – | – | – |
| C116 | B2 | DEL | NEST | x/vm/statedb/statedb.go:471 | R1 | ✅ | R1-E,R1-S | ✅ | – |
| M050 | B1 | ERR | - | precompiles/common/precompile.go:160 | - | – | – | ✅ | – |
| C019 | B2 | GUARD | CONTRACT | precompiles/common/precompile.go:61 | R1\|R2-W | – | – | – | – |
| M195 | B1 | DEL | - | x/vm/statedb/journal.go:289 | R1 | – | – | – | – |
| M225 | B1 | GUARD | - | x/vm/statedb/state_object.go:267 | R2-W | – | – | – | – |
| M098 | B1 | GUARD | - | x/vm/statedb/statedb.go:330 | - | – | – | ✅ | – |
| M397 | B1 | GUARD | - | precompiles/ics20/tx.go:202 | - | – | – | – | – |
| M209 | B1 | GUARD | - | x/vm/statedb/state_object.go:162 | R2-A | – | – | – | – |
| M238 | B1 | GUARD | - | x/vm/keeper/statedb.go:174 | R2-W | – | – | – | – |
| C201 | B2 | GUARD | LOCKED | x/vm/statedb/state_object.go:138 | R2-A | – | – | – | – |
| M158 | B1 | DEL | - | x/vm/statedb/statedb.go:729 | R2-W | ✅ | R1-C,R2-W | ✅ | – |
| M017 | B1 | GUARD | - | precompiles/common/precompile.go:61 | R1\|R2-W | – | – | – | – |
| M242 | B1 | SWAP | - | x/vm/keeper/statedb.go:192 | R2-W | – | – | ✅ | – |
| M199 | B1 | GUARD | - | x/vm/statedb/state_object.go:138 | R2-A | – | – | – | – |
| M213 | B1 | DEL | - | x/vm/statedb/state_object.go:177 | R2-W | ✅ | R2-W | ✅ | – |
| M096 | B1 | GUARD | - | x/vm/statedb/statedb.go:311 | - | – | – | ✅ | – |
| C179 | B2 | ERR | NEST | x/vm/statedb/statedb.go:783 | R2-W | – | – | – | – |
| M044 | B1 | GUARD | - | precompiles/common/precompile.go:120 | R1\|R2-W | ✅ | R2-W | ✅ | – |
| C124 | B2 | GUARD | NEST | x/vm/statedb/statedb.go:512 | R2-A | – | – | – | – |
| M395 | B1 | GUARD | - | precompiles/ics20/tx.go:188 | - | – | – | – | – |
| C217 | B2 | DEL | NEST | x/vm/statedb/state_object.go:181 | R2-W | ✅ | R2-W | ✅ | – |
| M364 | B1 | GUARD | - | precompiles/gov/tx.go:129 | - | – | – | ✅ | – |
| M330 | B1 | GUARD | - | precompiles/bank/bank.go:90 | - | – | – | – | – |
| M051 | B1 | GUARD | - | precompiles/common/precompile.go:164 | - | – | – | ✅ | – |
| M108 | B1 | DEL | - | x/vm/statedb/statedb.go:429 | - | – | – | ✅ | – |
| M086 | B1 | DEL | - | x/vm/statedb/statedb.go:229 | R1 | ✅ | R1-S,R2-W | ✅ | – |
| M219 | B1 | GUARD | - | x/vm/statedb/state_object.go:200 | R2-W | – | – | – | – |
| M190 | B1 | DEL | - | x/vm/statedb/journal.go:224 | R1 | – | – | – | – |
| M308 | B1 | SWAP | - | precompiles/distribution/tx.go:77 | R3 | – | – | ✅ | – |
| M084 | B1 | GUARD | - | x/vm/statedb/statedb.go:213 | - | – | – | ✅ | – |
| M052 | B1 | GUARD | - | precompiles/common/precompile.go:169 | - | – | – | ✅ | – |
| M328 | B1 | ERR | - | precompiles/distribution/tx.go:227 | R3 | – | – | ✅ | – |
| M071 | B1 | GUARD | - | precompiles/common/utils.go:33 | R2-W | – | – | – | – |
| M272 | B1 | GUARD | - | precompiles/staking/tx.go:68 | R3 | – | – | – | – |
| M063 | B1 | SWAP | - | precompiles/common/balance_handler.go:89 | R2-W | ✅ | R2-W | ✅ | – |
| M224 | B1 | DEL | - | x/vm/statedb/state_object.go:243 | R2-W | – | – | ✅ | – |
| M140 | B1 | GUARD | - | x/vm/statedb/statedb.go:628 | - | – | – | – | – |
| M075 | B1 | DEL | - | x/vm/statedb/statedb.go:93 | - | – | – | – | – |
| C046 | B2 | GUARD | CONTRACT | precompiles/common/precompile.go:120 | R1\|R2-W | ✅ | R2-W | ✅ | – |
| M099 | B1 | GUARD | - | x/vm/statedb/statedb.go:339 | - | – | – | ✅ | – |
| M278 | B1 | GUARD | - | precompiles/staking/tx.go:125 | R3 | – | – | ✅ | – |
| M371 | B1 | GUARD | - | precompiles/slashing/tx.go:31 | - | – | – | – | – |
| C087 | B2 | DEL | NEST | x/vm/statedb/statedb.go:229 | R1 | ✅ | R1-S | ✅ | – |
| M216 | B1 | DEL | - | x/vm/statedb/state_object.go:181 | R2-W | ✅ | R2-W | ✅ | – |
| M182 | B1 | DEL | - | x/vm/statedb/statedb.go:791 | R2-W | – | – | ✅ | – |
| C021 | B2 | ERR | LOWGAS | precompiles/common/precompile.go:68 | R1\|R2-W | – | – | – | – |
| C027 | B2 | ERR | LOWGAS | precompiles/common/precompile.go:84 | R1\|R2-W | – | – | – | – |
| M340 | B1 | GUARD | - | precompiles/erc20/tx.go:124 | - | – | – | ✅ | – |
| M068 | B1 | SWAP | - | precompiles/common/balance_handler.go:106 | R2-W | ✅ | R2-W | ✅ | – |
| M014 | B1 | LOCK | - | x/vm/keeper/statedb.go:0 | R2-W | – | – | ✅ | – |
| C211 | B2 | GUARD | LOCKED | x/vm/statedb/state_object.go:162 | R2-A | – | – | – | – |
| M203 | B1 | GUARD | - | x/vm/statedb/state_object.go:143 | R2-A | – | – | – | – |
| M228 | B1 | GUARD | - | x/vm/statedb/state_object.go:285 | R2-W | – | – | ✅ | – |
| M319 | B1 | GUARD | - | precompiles/distribution/tx.go:160 | R3 | – | – | ✅ | – |
| M234 | B1 | GUARD | - | x/vm/keeper/statedb.go:86 | R2-W | – | – | ✅ | – |
| M189 | B1 | DEL | - | x/vm/statedb/journal.go:216 | R1 | ✅ | R2-W | ✅ | – |
| M368 | B1 | GUARD | - | precompiles/gov/tx.go:158 | - | – | – | ✅ | – |
| M311 | B1 | GUARD | - | precompiles/distribution/tx.go:101 | R3 | – | – | ✅ | – |
| M304 | B1 | GUARD | - | precompiles/distribution/tx.go:55 | R3 | – | – | ✅ | – |
| M289 | B1 | ERR | - | precompiles/staking/tx.go:225 | R3 | – | – | ✅ | – |
| M178 | B1 | ERR | - | x/vm/statedb/statedb.go:783 | R2-W | – | – | ✅ | – |
| M396 | B1 | ERR | - | precompiles/ics20/tx.go:190 | - | – | – | – | – |
| M387 | B1 | GUARD | - | precompiles/ics20/tx.go:126 | - | – | – | – | – |
| M351 | B1 | ERR | - | precompiles/gov/tx.go:37 | - | – | – | ✅ | – |
| M245 | B1 | DEL | - | x/vm/keeper/statedb.go:216 | R2-W | – | – | ✅ | – |
| C025 | B2 | ERR | CONTRACT | precompiles/common/precompile.go:78 | R1\|R2-W | – | – | – | – |
| M164 | B1 | ERR | - | x/vm/statedb/statedb.go:747 | R2-W | ✅ | R2-W | – | – |
| M056 | B1 | GUARD | - | precompiles/common/precompile.go:243 | - | – | – | – | – |
| M185 | B1 | DEL | - | x/vm/statedb/journal.go:79 | R1 | ✅ | R1-E,R1-S,R2-W | ✅ | – |
| C155 | B2 | GUARD | NEST | x/vm/statedb/statedb.go:723 | R2-W | ✅ | R1-C | ✅ | – |
| M327 | B1 | GUARD | - | precompiles/distribution/tx.go:221 | R3 | – | – | ✅ | – |
| M383 | B1 | GUARD | - | precompiles/ics20/tx.go:85 | - | – | – | – | – |
| M360 | B1 | GUARD | - | precompiles/gov/tx.go:100 | - | – | – | ✅ | – |
| M187 | B1 | DEL | - | x/vm/statedb/journal.go:178 | R1 | ✅ | R1-S,R2-W | ✅ | – |
| M273 | B1 | GUARD | - | precompiles/staking/tx.go:73 | R3 | – | – | ✅ | – |
| C207 | B2 | GUARD | NEST | x/vm/statedb/state_object.go:158 | R2-A | – | – | – | – |
| C042 | B2 | GUARD | LOWGAS | precompiles/common/precompile.go:116 | R1\|R2-W | – | – | – | – |
| C159 | B2 | DEL | NEST | x/vm/statedb/statedb.go:729 | R2-W | ✅ | R1-C,R2-W | ✅ | – |
| C157 | B2 | ERR | NEST | x/vm/statedb/statedb.go:727 | R2-W | – | – | – | – |
| C033 | B2 | DEL | LOWGAS | precompiles/common/precompile.go:98 | R1\|R2-W | – | – | – | – |
| M356 | B1 | GUARD | - | precompiles/gov/tx.go:71 | - | – | – | – | – |
| M357 | B1 | ERR | - | precompiles/gov/tx.go:76 | - | – | – | ✅ | – |
| C214 | B2 | DEL | NEST | x/vm/statedb/state_object.go:177 | R2-W | – | – | ✅ | – |
| M128 | B1 | GUARD | - | x/vm/statedb/statedb.go:539 | - | – | – | ✅ | – |
| M220 | B1 | GUARD | - | x/vm/statedb/state_object.go:204 | R2-W | – | – | – | – |
| M176 | B1 | DEL | - | x/vm/statedb/statedb.go:779 | R2-W | – | – | ✅ | – |
| C241 | B2 | GUARD | LOCKED | x/vm/keeper/statedb.go:188 | R2-W | – | – | – | – |
| M146 | B1 | DEL | - | x/vm/statedb/statedb.go:651 | - | – | – | – | – |
| M222 | B1 | DEL | - | x/vm/statedb/state_object.go:228 | R2-W | – | – | ✅ | – |
| M229 | B1 | GUARD | - | x/vm/statedb/state_object.go:296 | R2-W | – | – | – | – |
| M277 | B1 | GUARD | - | precompiles/staking/tx.go:120 | R3 | – | – | – | – |
| M091 | B1 | DEL | - | x/vm/statedb/statedb.go:258 | - | – | – | ✅ | – |
| C045 | B2 | GUARD | LOWGAS | precompiles/common/precompile.go:120 | R1\|R2-W | ✅ | R2-W | ✅ | – |
| M186 | B1 | DEL | - | x/vm/statedb/journal.go:84 | R1 | – | – | ✅ | – |
| C018 | B2 | GUARD | LOWGAS | precompiles/common/precompile.go:61 | R1\|R2-W | – | – | – | – |
| M232 | B1 | GUARD | - | x/vm/keeper/statedb.go:39 | R2-W | – | – | ✅ | – |
| M104 | B1 | GUARD | - | x/vm/statedb/statedb.go:392 | - | – | – | ✅ | – |
| C049 | B2 | ERR | CONTRACT | precompiles/common/precompile.go:122 | R1\|R2-W | – | – | – | – |
| C151 | B2 | GUARD | NEST | x/vm/statedb/statedb.go:707 | R1 | – | – | – | – |
| M038 | B1 | ERR | - | precompiles/common/precompile.go:111 | R1\|R2-W | ✅ | R3-D,R3-U | ✅ | – |
| C218 | B2 | DEL | LOCKED | x/vm/statedb/state_object.go:181 | R2-W | ✅ | R2-W | ✅ | – |
| M223 | B1 | DEL | - | x/vm/statedb/state_object.go:239 | R2-W | – | – | – | – |
| M102 | B1 | GUARD | - | x/vm/statedb/statedb.go:366 | - | – | – | ✅ | – |
| C024 | B2 | ERR | LOWGAS | precompiles/common/precompile.go:78 | R1\|R2-W | – | – | – | – |
| M192 | B1 | DEL | - | x/vm/statedb/journal.go:240 | R1 | – | – | – | – |
| M125 | B1 | SWAP | - | x/vm/statedb/statedb.go:515 | R2-W | ✅ | R2-W | ✅ | – |
| M348 | B1 | GUARD | - | precompiles/werc20/tx.go:63 | - | – | – | – | – |
| C034 | B2 | DEL | CONTRACT | precompiles/common/precompile.go:98 | R1\|R2-W | – | – | – | – |
| M126 | B1 | DEL | - | x/vm/statedb/statedb.go:522 | - | – | – | ✅ | – |
| M394 | B1 | ERR | - | precompiles/ics20/tx.go:184 | - | – | – | – | – |
| M237 | B1 | GUARD | - | x/vm/keeper/statedb.go:153 | R2-W | – | – | ✅ | – |
| M231 | B1 | DEL | - | x/vm/statedb/state_object.go:305 | R2-W | – | – | ✅ | – |
| M057 | B1 | GUARD | - | precompiles/common/precompile.go:248 | - | – | – | – | – |
| M147 | B1 | DEL | - | x/vm/statedb/statedb.go:661 | - | – | – | – | – |
| M120 | B1 | SWAP | - | x/vm/statedb/statedb.go:503 | R2-W | ✅ | R2-W | ✅ | – |
| M386 | B1 | GUARD | - | precompiles/ics20/tx.go:118 | - | – | – | – | – |
| M013 | B1 | LOCK | - | x/vm/keeper/statedb.go:0 | R2-W | ✅ | R2-W | ✅ | – |
| C210 | B2 | GUARD | NEST | x/vm/statedb/state_object.go:162 | R2-A | – | – | – | – |
| M097 | B1 | GUARD | - | x/vm/statedb/statedb.go:321 | - | – | – | ✅ | – |
| C043 | B2 | GUARD | CONTRACT | precompiles/common/precompile.go:116 | R1\|R2-W | – | – | – | – |
| M015 | B1 | GUARD | - | precompiles/common/precompile.go:38 | - | – | – | ✅ | – |
| M041 | B1 | GUARD | - | precompiles/common/precompile.go:116 | R1\|R2-W | – | – | – | – |
| M393 | B1 | GUARD | - | precompiles/ics20/tx.go:178 | - | – | – | – | – |
| M294 | B1 | ERR | - | precompiles/staking/tx.go:274 | R3 | – | – | ✅ | – |
| M299 | B1 | ERR | - | precompiles/staking/tx.go:321 | R3 | – | – | ✅ | – |
| C165 | B2 | ERR | NEST | x/vm/statedb/statedb.go:747 | R2-W | – | – | – | – |
| C181 | B2 | DEL | NEST | x/vm/statedb/statedb.go:789 | R2-W | – | – | – | – |
| M026 | B1 | ERR | - | precompiles/common/precompile.go:84 | R1\|R2-W | ✅ | R2-W | – | – |
| M156 | B1 | ERR | - | x/vm/statedb/statedb.go:727 | R2-W | ✅ | R2-W | ✅ | – |
| M105 | B1 | DEL | - | x/vm/statedb/statedb.go:397 | - | – | – | ✅ | – |
| M352 | B1 | GUARD | - | precompiles/gov/tx.go:41 | - | – | – | – | – |
| C037 | B2 | DEL | CONTRACT | precompiles/common/precompile.go:106 | R1\|R2-W | – | – | – | – |
| M323 | B1 | GUARD | - | precompiles/distribution/tx.go:190 | R3 | – | – | – | – |
| C040 | B2 | ERR | CONTRACT | precompiles/common/precompile.go:111 | R1\|R2-W | ✅ | R3-D | ✅ | – |
| C239 | B2 | GUARD | LOCKED | x/vm/keeper/statedb.go:174 | R2-W | – | – | – | – |
| M142 | B1 | DEL | - | x/vm/statedb/statedb.go:638 | - | – | – | – | – |
| C022 | B2 | ERR | CONTRACT | precompiles/common/precompile.go:68 | R1\|R2-W | – | – | – | – |
| C122 | B2 | GUARD | NEST | x/vm/statedb/statedb.go:509 | R2-A | – | – | – | – |
| M324 | B1 | ERR | - | precompiles/distribution/tx.go:196 | R3 | – | – | ✅ | – |
| M377 | B1 | GUARD | - | precompiles/ics20/tx.go:41 | - | – | – | – | – |
| M373 | B1 | GUARD | - | precompiles/slashing/tx.go:41 | - | – | – | ✅ | – |
