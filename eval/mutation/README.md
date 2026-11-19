# 변이 평가: 판정기 탐지력 측정 (사전 정의)

작성일: 2026-11-19 · 대상: cosmos/evm **v0.7.4** (`ba66d31`) · 실행: [`run.sh`](run.sh)

> **이 문서와 `gen_mutants.py`, `mutants.tsv`, `patches/`는 변이를 실행하기 전에 커밋한다.**
> 결과를 본 뒤에는 연산자, 대상, 예상 규칙, 탐지 정의, 실행 순서를 바꾸지 않는다.
> 바꿔야 할 일이 생기면 이 문서 끝의 "변경 기록"에 이유와 함께 남기고, 바꾸기 전 결과도 같이 보고한다.

## 1. 무엇을 재나

보고서에서 약속한 판정기 성능 평가다. 규칙은 GHSA 세 사례를 보고 설계했으므로, 그 세 사례로 확인한 7단계 결과는 성능 근거가 될 수 없다. 여기서는 버그를 **기계적으로** 심은 v0.7.4 변형 버전을 많이 만들고, 다음을 잰다.

| 지표 | 정의 |
|---|---|
| 판정기 탐지율 | 실행을 마친 변이 중 판정기가 위반(`violation`)을 1건 이상 보고한 비율 |
| 기존 테스트 탐지율 | 같은 실행에서 업스트림 테스트가 실패(FAIL·panic·시간 초과)한 비율 |
| 거래 전후 비교 탐지율 | 같은 기록에서 보조 검사 A1(최상위 StateDB의 처음 관찰 vs 커밋 직후 총발행량)이 변화를 보고한 비율 |
| 판정기만 잡은 변이 수 | 기존 테스트는 통과했는데 판정기가 위반을 보고한 변이 |
| 관찰 지점 빼 보기 | 판정기가 잡은 변이를 O2·O4·O5·O6 중 하나를 뺀 기록으로 다시 판정했을 때 여전히 잡는 수 |

판정기와 기존 테스트는 **같은 실행, 같은 입력**을 본다. 입력은 업스트림 정상 스위트 8묶음이다(`eval/fp-upstream`과 같음). 그래서 이 평가는 **판정기 성능**을 재는 것이고, 입력 생성기 성능은 8단계 이후 같은 변이 세트로 다시 잰다.

## 2. 대상 파일

`x/vm/statedb/{statedb,journal,state_object}.go`, `x/vm/keeper/statedb.go`, `precompiles/common/{precompile,balance_handler,utils}.go`, 그리고 precompile 상태 변경 파일 `precompiles/{staking,distribution,erc20,werc20,gov,slashing,ics20}/tx.go`, `precompiles/erc20/approve.go`, `precompiles/bank/bank.go`. 테스트·mock 파일은 넣지 않는다.

## 3. 변이 묶음과 연산자

### 3.1 A: 공개 수정 되돌리기 (`FIX-REVERT`)
v0.7.4까지 들어간 보안 수정을 v0.7.4 코드 위에서 되돌린다. 수정 하나가 여러 곳을 고쳤으면 고친 곳마다 따로 만든다.

| id | 수정 | 되돌린 내용 | 예상 규칙 |
|---|---|---|---|
| A01 | GHSA-mjfq `0fff8c14` | journal 항목을 precompile 호출이 성공한 뒤에만 추가 | R1 |
| A02 (제외) | GHSA-8pfh `79089fe` (1/2) | 되돌릴 때 이벤트를 복원하지 않음 | R1-E, R2-W |
| A03 (제외) | GHSA-8pfh `79089fe` (2/2) | 중첩 호출이 BalanceHandler 하나를 공유 | R2-W |
| A04 | #1187 `ae2db4f1` | 잠긴 잔고를 커밋 시점에 다시 읽음 (불러올 때의 스냅샷을 쓰지 않음) | R2-W |
| A05 | `fb82a3f8` | 모듈·차단 계정 검사를 잔고가 늘 때만 함 | R2-W |
| A06 | GHSA-7g4w `0182da19` | `SubBalance` 언더플로 검사 제거 | R2-A |
| A07 | GHSA-367m `c3ae9067` (1/3) | precompile 없는 Commit이 거래 컨텍스트에 바로 기록 | R1-S |
| A08 | GHSA-367m `c3ae9067` (2/3) | precompile 있는 Commit이 캐시를 먼저 쓰고 나머지를 따로 기록 | R1-S, R2-W |
| A09 | `c3ae9067` (3/3) | `AddBalance` 오버플로 검사 제거 | R2-A |
| A10 | GHSA-wm4f `4f50aab2` | precompile 호출 횟수 제한 미적용 (DoS 계열) | 없음 (규칙 밖, 미탐 예상) |
| A11 | GHSA-c6xv `8214c816` | staking 기록이 있는 컨트랙트의 self-destruct 허용 | 없음 (R3-L 미구현, 미탐 예상) |

A는 설계에 쓴 버그 계열(mjfq·7g4w)을 포함하므로 **B와 따로 집계**한다.

**A02·A03은 생성만 하고 실행·집계에서 뺀다.** 레포 원칙(README "원칙", `docs/answer-key.md`)에 따라 GHSA-8pfh는 원인이 공식 공개되지 않아 설계·동작 확인용 사례로만 쓰고 11단계 평가에서 제외하기 때문이다. 이 결정은 변이를 실행하기 전에 내렸다.

### 3.2 B1: 단순 연산자 (대상 파일의 모든 해당 위치에 자동 적용)

| 연산자 | 적용 위치 | 바꾸는 내용 |
|---|---|---|
| `DEL` | 단독 호출문 (여러 줄 포함). 로그·추적·`panic`·`fmt`·이벤트 발행 호출은 제외 | 그 문장을 지움 |
| `ERR` | `if … err != nil {` 블록 안의 단일 `return` | `return`을 지워 오류를 삼킴 |
| `GUARD` | 블록이 `return`/`panic`/`continue`/`break`로 끝나는 `if COND {` (본문 12줄 이하, `err != nil` 단독 조건은 `ERR`이 다룸) | `if false && (COND) {` |
| `SWAP` | `.AddBalance(`↔`.SubBalance(`, `.AddOverflow(`→`.SubOverflow(`, `.Add(`↔`.Sub(` | 한 줄에 한 곳 |
| `LOCK` | `x/vm/keeper/statedb.go`의 잔고 종류 3곳 (명시 목록) | 사용 가능 잔고 대신 총잔고 / 잠긴 잔고를 0으로 / 커밋 때 잠긴 잔고 누락 |

### 3.3 B2: 조건부 변이 (특정 문맥에서만 버그가 드러남)
실제 버그 세 건은 모두 특정 문맥(낮은 gas, 중첩 호출, 베스팅 계정)에서만 터졌다. 그래서 B1의 `DEL`·`ERR`·`GUARD`를 핵심 상태 전이 함수(`gen_mutants.py`의 `B2_FUNCS`) 안에서 아래 문맥일 때만 일어나도록 감싼 변이를 함께 만든다.

| 문맥 | 조건식 | 쓸 수 있는 곳 |
|---|---|---|
| `NEST` | 이 거래에서 precompile 호출이 2번 이상 일어난 뒤 (`precompileCallsCounter > 1`) | StateDB·stateObject 메서드 |
| `LOCKED` | 잠긴(베스팅) 잔고가 있는 계정 | stateObject 메서드, `SetBalanceWithLocked` |
| `LOWGAS` | precompile에 넘어온 gas가 100,000 미만 | `runNativeAction` |
| `CONTRACT` | precompile을 EOA가 아니라 컨트랙트가 호출 | `runNativeAction` |

### 3.4 예상 규칙 (변이마다 `mutants.tsv`의 `expected` 칸)
파일·함수로 기계적으로 정한다(`expected_rule`). 되돌림 경로는 R1, 잔고·커밋·BalanceHandler·keeper는 R2-W, 산술 검사는 R2-A, staking·distribution precompile은 R3다. **나머지 precompile(bank·erc20·werc20·gov·slashing·ics20)은 기능 규칙이 없어서 `-`(미탐 예상)로 적는다.**

## 4. 실행과 판정

1. **커버리지:** 깨끗한 v0.7.4에서 8묶음을 `-coverprofile`로 한 번 돌린다. 변이가 바꾼 줄을 실행한 스위트만 그 변이의 입력으로 쓴다. 어느 스위트도 실행하지 않는 줄의 변이는 **도달 못 함**으로 분류하고 실행하지 않는다.
2. **대조군:** 변이 없는 트리에 하네스를 붙여 8묶음을 돌린다. 테스트는 모두 PASS, 판정기 위반은 0이어야 한다.
3. **변이마다:** 깨끗한 트리 → `git apply` → `inject.sh v07` → 빌드 → 해당 스위트 실행(`ATOMICOS_TRACE`) → 테스트 결과 + 판정(전체, O2/O4/O5/O6 각각 뺀 판정).
4. **제외:** 패치 적용 실패, 하네스 삽입 실패, 빌드 실패. 개수는 보고한다.
5. **실행 순서:** A 먼저, 그다음 B1·B2를 고정 seed(`20261119`)로 섞은 순서. **시간 안에 다 돌리지 못하면, 섞은 순서의 앞부분까지 실행한 결과를 그대로 보고한다**(무작위 표본). 골라서 돌리지 않는다.
6. **보고:** 탐지율은 "실행 완료" 변이를 분모로 하고 95% 신뢰 구간(Wilson)을 붙인다. 도달 못 한 변이 수도 함께 적는다. 결과는 묶음(A/B1/B2), 연산자, 예상 규칙별로 나눈다.

```bash
./eval/mutation/run.sh gen
```

```bash
./eval/mutation/run.sh coverage
```

```bash
./eval/mutation/run.sh plan
```

```bash
./eval/mutation/run.sh run
```

```bash
./eval/mutation/run.sh report
```

## 생성 결과 (실행 전, 2026-11-19)

`run.sh gen`·`coverage`·`plan` 결과다. 변이별 내용은 [`mutants.tsv`](mutants.tsv)와 [`patches/`](patches/), 변이별 실행 스위트는 [`plan.tsv`](plan.tsv), 커버리지는 [`results/coverage/`](results/coverage/)에 있다.

| 묶음 | 연산자 | 생성 | 도달 |
|---|---|---|---|
| A | FIX-REVERT | 11 | 10 (A11 self-destruct 경로는 스위트가 지나가지 않음). 그중 A02·A03은 8pfh 계열이라 실행하지 않음 → 실행 대상 8 |
| B1 | DEL | 88 | 59 |
| B1 | ERR | 111 | 19 |
| B1 | GUARD | 114 | 91 |
| B1 | LOCK | 3 | 3 |
| B1 | SWAP | 11 | 9 |
| B2 | DEL | 21 | 19 |
| B2 | ERR | 16 | 14 |
| B2 | GUARD | 23 | 22 |
| **합계** | | **398** | **246** (실행 대상 244) |

ERR은 도달률이 낮다(111개 중 19개). 업스트림 정상 테스트가 오류 경로를 거의 지나가지 않기 때문이다. 이 자체가 입력 생성기가 필요한 이유 중 하나다.

## 5. 해석할 때 주의
- **아무도 못 잡은 변이**에는 동작이 원본과 같은 변이(등가 변이)가 섞여 있다. 따로 걸러 내지 않았으므로 탐지율은 보수적인 값이다.
- **입력이 닿았다고 버그가 드러나는 것은 아니다.** 커버리지는 줄 단위라, B2처럼 특정 문맥이 필요한 변이는 줄은 실행돼도 조건이 성립하지 않을 수 있다. 이 몫은 생성기(8단계)가 채울 부분이다.
- 기존 테스트의 탐지에는 panic과 시간 초과도 포함한다(일반적인 변이 테스트 관례).
- 거래 전후 비교는 O1(거래 전후 관찰)이 아직 없어서, 같은 기록에서 보조 검사 A1(총발행량)로 대신한다. 잔고 하나하나의 전후 비교는 정답 값을 알아야 하므로 기준선으로 쓰지 않았다.

## 변경 기록
- (없음)
