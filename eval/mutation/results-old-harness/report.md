# 변이 평가 결과 (자동 생성: report.py)

**대조군 (변이 없음):** 스위트 8개, 테스트 모두 PASS, 판정기 위반 0건, A1 0건

## 1. 변이 현황

| 상태 | 개수 |
|---|---|
| 생성 | 398 |
| 도달 못 함 (실행 안 함) | 152 |
| 실행 완료 | 97 |
| 실행 중 (미완료) | 4 |
| 아직 실행 안 됨 | 135 |
| 제외: 8pfh 계열 (설계·동작 확인용 사례) | 2 |
| 제외: BUILD_FAIL | 8 |

## 2. 탐지율 (실행 완료한 변이 기준)

| 방법 | 탐지 |
|---|---|
| 판정기 | 12/97 = **12%** (95% CI 7–20%) |
| 기존 테스트 assertion | 35/97 = **36%** (95% CI 27–46%) |
| 거래 전후 총발행량 비교 (A1) | 0/97 = **0%** (95% CI 0–4%) |
| 판정기 또는 기존 테스트 | 37/97 = **38%** (95% CI 29–48%) |

| | 기존 테스트가 잡음 | 기존 테스트가 놓침 |
|---|---|---|
| **판정기가 잡음** | 10 | **2** |
| **판정기가 놓침** | 25 | 60 |

기존 테스트가 잡은 방식 (한 변이가 여러 스위트에서 다르게 잡히면 각각 셈): FAIL 35, MEMOUT 2

## 3. 묶음별

### 변이 묶음 (A 공개 수정 되돌리기 / B1 단순 연산자 / B2 조건부)

| 묶음 | 실행 | 판정기 | 기존 테스트 | 거래 전후(A1) | 기존 테스트가 놓친 것 중 판정기 |
|---|---|---|---|---|---|
| A | 8 | 1/8 = **12%** (95% CI 2–47%) | 2/8 = **25%** (95% CI 7–59%) | 0/8 = **0%** (95% CI 0–32%) | 1/6 = **17%** (95% CI 3–56%) |
| B1 | 69 | 8/69 = **12%** (95% CI 6–21%) | 30/69 = **43%** (95% CI 32–55%) | 0/69 = **0%** (95% CI 0–5%) | 0/39 = **0%** (95% CI 0–9%) |
| B2 | 20 | 3/20 = **15%** (95% CI 5–36%) | 3/20 = **15%** (95% CI 5–36%) | 0/20 = **0%** (95% CI 0–16%) | 1/17 = **6%** (95% CI 1–27%) |

### 연산자

| 묶음 | 실행 | 판정기 | 기존 테스트 | 거래 전후(A1) | 기존 테스트가 놓친 것 중 판정기 |
|---|---|---|---|---|---|
| A FIX-REVERT | 8 | 1/8 = **12%** (95% CI 2–47%) | 2/8 = **25%** (95% CI 7–59%) | 0/8 = **0%** (95% CI 0–32%) | 1/6 = **17%** (95% CI 3–56%) |
| B1 DEL | 17 | 2/17 = **12%** (95% CI 3–34%) | 5/17 = **29%** (95% CI 13–53%) | 0/17 = **0%** (95% CI 0–18%) | 0/12 = **0%** (95% CI 0–24%) |
| B1 ERR | 5 | 1/5 = **20%** (95% CI 4–62%) | 4/5 = **80%** (95% CI 38–96%) | 0/5 = **0%** (95% CI 0–43%) | 0/1 = **0%** (95% CI 0–79%) |
| B1 GUARD | 42 | 2/42 = **5%** (95% CI 1–16%) | 16/42 = **38%** (95% CI 25–53%) | 0/42 = **0%** (95% CI 0–8%) | 0/26 = **0%** (95% CI 0–13%) |
| B1 LOCK | 1 | 1/1 = **100%** (95% CI 21–100%) | 1/1 = **100%** (95% CI 21–100%) | 0/1 = **0%** (95% CI 0–79%) | – (0) |
| B1 SWAP | 4 | 2/4 = **50%** (95% CI 15–85%) | 4/4 = **100%** (95% CI 51–100%) | 0/4 = **0%** (95% CI 0–49%) | – (0) |
| B2 DEL | 8 | 2/8 = **25%** (95% CI 7–59%) | 2/8 = **25%** (95% CI 7–59%) | 0/8 = **0%** (95% CI 0–32%) | 0/6 = **0%** (95% CI 0–39%) |
| B2 ERR | 4 | 1/4 = **25%** (95% CI 5–70%) | 1/4 = **25%** (95% CI 5–70%) | 0/4 = **0%** (95% CI 0–49%) | 1/3 = **33%** (95% CI 6–79%) |
| B2 GUARD | 8 | 0/8 = **0%** (95% CI 0–32%) | 0/8 = **0%** (95% CI 0–32%) | 0/8 = **0%** (95% CI 0–32%) | 0/8 = **0%** (95% CI 0–32%) |

### 미리 적은 예상 규칙

| 묶음 | 실행 | 판정기 | 기존 테스트 | 거래 전후(A1) | 기존 테스트가 놓친 것 중 판정기 |
|---|---|---|---|---|---|
| - | 25 | 2/25 = **8%** (95% CI 2–25%) | 11/25 = **44%** (95% CI 27–63%) | 0/25 = **0%** (95% CI 0–13%) | 0/14 = **0%** (95% CI 0–22%) |
| R1 | 10 | 3/10 = **30%** (95% CI 11–60%) | 2/10 = **20%** (95% CI 6–51%) | 0/10 = **0%** (95% CI 0–28%) | 1/8 = **12%** (95% CI 2–47%) |
| R1-S | 1 | 0/1 = **0%** (95% CI 0–79%) | 0/1 = **0%** (95% CI 0–79%) | 0/1 = **0%** (95% CI 0–79%) | 0/1 = **0%** (95% CI 0–79%) |
| R1-S\|R2-W | 1 | 0/1 = **0%** (95% CI 0–79%) | 0/1 = **0%** (95% CI 0–79%) | 0/1 = **0%** (95% CI 0–79%) | 0/1 = **0%** (95% CI 0–79%) |
| R1\|R2-W | 8 | 1/8 = **12%** (95% CI 2–47%) | 1/8 = **12%** (95% CI 2–47%) | 0/8 = **0%** (95% CI 0–32%) | 1/7 = **14%** (95% CI 3–51%) |
| R2-A | 14 | 0/14 = **0%** (95% CI 0–22%) | 0/14 = **0%** (95% CI 0–22%) | 0/14 = **0%** (95% CI 0–22%) | 0/14 = **0%** (95% CI 0–22%) |
| R2-W | 30 | 5/30 = **17%** (95% CI 7–34%) | 16/30 = **53%** (95% CI 36–70%) | 0/30 = **0%** (95% CI 0–11%) | 0/14 = **0%** (95% CI 0–22%) |
| R3 | 8 | 1/8 = **12%** (95% CI 2–47%) | 5/8 = **62%** (95% CI 31–86%) | 0/8 = **0%** (95% CI 0–32%) | 0/3 = **0%** (95% CI 0–56%) |

### 파일

| 묶음 | 실행 | 판정기 | 기존 테스트 | 거래 전후(A1) | 기존 테스트가 놓친 것 중 판정기 |
|---|---|---|---|---|---|
| precompiles/common/balance_handler.go | 3 | 0/3 = **0%** (95% CI 0–56%) | 2/3 = **67%** (95% CI 21–94%) | 0/3 = **0%** (95% CI 0–56%) | 0/1 = **0%** (95% CI 0–79%) |
| precompiles/common/precompile.go | 11 | 2/11 = **18%** (95% CI 5–48%) | 3/11 = **27%** (95% CI 10–57%) | 0/11 = **0%** (95% CI 0–26%) | 2/8 = **25%** (95% CI 7–59%) |
| precompiles/common/utils.go | 1 | 0/1 = **0%** (95% CI 0–79%) | 0/1 = **0%** (95% CI 0–79%) | 0/1 = **0%** (95% CI 0–79%) | 0/1 = **0%** (95% CI 0–79%) |
| precompiles/distribution/tx.go | 3 | 0/3 = **0%** (95% CI 0–56%) | 1/3 = **33%** (95% CI 6–79%) | 0/3 = **0%** (95% CI 0–56%) | 0/2 = **0%** (95% CI 0–66%) |
| precompiles/erc20/approve.go | 1 | 0/1 = **0%** (95% CI 0–79%) | 0/1 = **0%** (95% CI 0–79%) | 0/1 = **0%** (95% CI 0–79%) | 0/1 = **0%** (95% CI 0–79%) |
| precompiles/erc20/tx.go | 2 | 0/2 = **0%** (95% CI 0–66%) | 2/2 = **100%** (95% CI 34–100%) | 0/2 = **0%** (95% CI 0–66%) | – (0) |
| precompiles/gov/tx.go | 1 | 0/1 = **0%** (95% CI 0–79%) | 1/1 = **100%** (95% CI 21–100%) | 0/1 = **0%** (95% CI 0–79%) | – (0) |
| precompiles/ics20/tx.go | 5 | 0/5 = **0%** (95% CI 0–43%) | 0/5 = **0%** (95% CI 0–43%) | 0/5 = **0%** (95% CI 0–43%) | 0/5 = **0%** (95% CI 0–43%) |
| precompiles/slashing/tx.go | 1 | 0/1 = **0%** (95% CI 0–79%) | 0/1 = **0%** (95% CI 0–79%) | 0/1 = **0%** (95% CI 0–79%) | 0/1 = **0%** (95% CI 0–79%) |
| precompiles/staking/tx.go | 5 | 1/5 = **20%** (95% CI 4–62%) | 4/5 = **80%** (95% CI 38–96%) | 0/5 = **0%** (95% CI 0–43%) | 0/1 = **0%** (95% CI 0–79%) |
| precompiles/werc20/tx.go | 1 | 0/1 = **0%** (95% CI 0–79%) | 1/1 = **100%** (95% CI 21–100%) | 0/1 = **0%** (95% CI 0–79%) | – (0) |
| x/vm/keeper/statedb.go | 10 | 2/10 = **20%** (95% CI 6–51%) | 7/10 = **70%** (95% CI 40–89%) | 0/10 = **0%** (95% CI 0–28%) | 0/3 = **0%** (95% CI 0–56%) |
| x/vm/statedb/journal.go | 4 | 0/4 = **0%** (95% CI 0–49%) | 0/4 = **0%** (95% CI 0–49%) | 0/4 = **0%** (95% CI 0–49%) | 0/4 = **0%** (95% CI 0–49%) |
| x/vm/statedb/state_object.go | 19 | 3/19 = **16%** (95% CI 6–38%) | 5/19 = **26%** (95% CI 12–49%) | 0/19 = **0%** (95% CI 0–17%) | 0/14 = **0%** (95% CI 0–22%) |
| x/vm/statedb/statedb.go | 30 | 4/30 = **13%** (95% CI 5–30%) | 9/30 = **30%** (95% CI 17–48%) | 0/30 = **0%** (95% CI 0–11%) | 0/21 = **0%** (95% CI 0–15%) |

## 4. 관찰 지점 빼 보기 (같은 기록을 해당 관찰 지점 없이 다시 판정)

판정기가 잡은 변이 12개 중, 관찰 지점을 하나 뺐을 때도 잡히는 개수:

| 뺀 관찰 지점 | 여전히 탐지 | 놓치게 되는 변이 |
|---|---|---|
| O2 | 4/12 | 8 |
| O4 | 11/12 | 1 |
| O5 | 9/12 | 3 |
| O6 | 9/12 | 3 |

## 5. 변이별 결과

| id | 묶음 | 연산자 | 문맥 | 파일:줄 | 예상 | 판정기 | 규칙 | 테스트 | A1 |
|---|---|---|---|---|---|---|---|---|---|
| A01 | A | FIX-REVERT | - | precompiles/common/precompile.go:0 | R1 | ✅ | R1-E,R1-S | – | – |
| A04 | A | FIX-REVERT | - | x/vm/keeper/statedb.go:0 | R2-W | – | – | ✅ | – |
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
| M154 | B1 | GUARD | - | x/vm/statedb/statedb.go:723 | R2-W | – | – | ✅ | – |
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
| M158 | B1 | DEL | - | x/vm/statedb/statedb.go:729 | R2-W | – | – | ✅ | – |
| M017 | B1 | GUARD | - | precompiles/common/precompile.go:61 | R1\|R2-W | – | – | – | – |
| M242 | B1 | SWAP | - | x/vm/keeper/statedb.go:192 | R2-W | – | – | ✅ | – |
| M199 | B1 | GUARD | - | x/vm/statedb/state_object.go:138 | R2-A | – | – | – | – |
| M213 | B1 | DEL | - | x/vm/statedb/state_object.go:177 | R2-W | ✅ | R2-W | ✅ | – |
