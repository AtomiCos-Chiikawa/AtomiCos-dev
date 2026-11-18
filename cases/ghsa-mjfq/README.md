# GHSA-mjfq-3qr2-6g84 (ISA-2025-004): 낮은 gas로 인한 precompile 부분 상태 쓰기

| 항목 | 값 |
|---|---|
| 권고 | https://github.com/cosmos/evm/security/advisories/GHSA-mjfq-3qr2-6g84 |
| 취약 커밋 | `5456ce8dd3d1bed0c7442ab2f0d4146a23efe624` (수정 직전, v0.1.0-26) |
| 수정 커밋 | `0fff8c144b24effbcb3addd666150ba5989d631c` |
| 재현 방식 | 수정 커밋의 회귀 테스트(`tryClaimRewards`)를 수정 직전 커밋에 넣어 두 버전에서 실행 |
| 실행 | `./cases/ghsa-mjfq/run.sh` → `results/summary.txt`, `results/{vuln,fixed}.log` |

## 분석 노트

### 1. 무엇이 잘못됐나
precompile은 실행을 시작할 때(`RunSetup`) StateDB의 SDK 캐시 스냅샷(`s.MultiStore`, `s.Events`)을 떠 둔다. 그런데 이 스냅샷을 journal에 등록하는 `AddJournalEntries` → `stateDB.AddPrecompileFn(...)`은 실행이 성공했을 때만 불렸다. 실패 경로는 두 가지였다.

- keeper 실행 중 SDK gas 미터가 out of gas로 panic: `HandleGasError`가 panic을 잡아 `vm.ErrOutOfGas`로 바꾸기만 하고 SDK 캐시는 되돌리지 않았다.
- 오류 반환, 또는 실행 후 `contract.UseGas(cost)` 실패: 그대로 `return nil, err`했다.

두 경우 모두 그때까지 SDK 캐시에 쓴 변경이 남았다. journal 항목도 없어서 호출한 EVM 프레임이 되돌려져도 이 변경은 되돌려지지 않았다.

### 2. 언제 터지나
- 계정: 보상이 쌓인 일반 위임자(EOA)
- 호출 방식: 컨트랙트가 `try/catch`로 distribution `claimRewards`를 호출한다. 실패해도 바깥 거래는 성공한다.
- gas 조건: 거래 gas limit 80,000. keeper 실행 도중이나 직후에 precompile gas가 부족해진다.

### 3. 어떤 상태가 어긋나나
- Cosmos 쪽: 호출은 실패했는데도 보상 일부(검증인 한 명 몫)가 distribution 모듈 계정에서 위임자의 bank 잔고로 지급된다. 그런데 distribution의 청구 가능 보상은 줄지 않는다. 7단계 기록(`results/judge/vuln.trace.jsonl.gz`)에서 위임자의 청구 가능 보상은 스냅샷(seq 92), 되돌림 직후(seq 95), 커밋(seq 96)에서 모두 그대로였다. 위임자 잔고가 늘어난 만큼 distribution 모듈 계정 잔고는 줄었다.
- EVM 쪽: precompile 호출은 실패(`catch`)로 처리되고 반환값은 `false`다.
- 결과: "실패한 호출"이 SDK 상태를 바꿨다. 권고는 "it could cause funds to be transferred to a user without resetting the claimable rewards to 0"이라고 경고했다. 우리 재현에서 관찰한 상태(보상은 잔고로 지급됐는데 청구 가능 보상은 그대로)가 정확히 이 상황이다.

### 4. 언제 보이나
**거래 전후만 봐도 보인다.** 위임자의 bank 잔고가 `초기 잔고 − gas 비용`보다 보상 중 검증인 한 명 몫만큼 많다. 회귀 테스트도 정확히 이 값을 비교한다(`expectedGasCost = 79_415_000_000_000`). 커밋된 실행에서 이 차이는 10,536,620,465,327,065로, 위임자에게 쌓인 보상 28,779,770,773,615,504의 약 37%다. 어느 검증인 몫이 지급되는지는 실행마다 달라서 다른 검증인 몫인 7,706,529,842,961,374가 나오기도 한다. 두 값 모두 7단계 기록에 있는 이 위임자의 검증인별 청구 가능 보상이다.

### 5. 어떻게 고쳤나
- `HandleGasError`가 out of gas를 잡을 때 `stateDB.RevertMultiStore(snapshot.MultiStore, snapshot.Events)`로 SDK 캐시를 되돌린다.
- 새 `RunAtomic(snapshot, stateDB, fn)`이 오류가 반환되면 같은 방식으로 되돌린다. 모든 상태 변경 precompile의 `Run`을 이걸로 감쌌다.
- journal의 `precompileCallChange.Revert`도 같은 `RevertMultiStore`를 쓰도록 정리했다.

### 6. 어떤 불변식이 깨졌나
> **실패한 precompile 호출이 만든 SDK 상태 변경과 이벤트는 그 호출과 함께 되돌려져야 한다.** (규칙 1: 호출 단위 원자성)

## 재현 방법
`run.sh`는 수정 커밋에서 아래 파일을 그대로 가져와 두 worktree(취약·수정)에 넣는다.
- `precompiles/distribution/integration_test.go` (`tryClaimRewards` 컨텍스트 추가본)
- `precompiles/testutil/contracts/DistributionCaller.sol` / `.json` (`testTryClaimRewards` 추가본)

assertion은 바꾸지 않았다. 비교하는 값을 로그에 남기기 위해 `GinkgoWriter.Printf("REPRO ...")` 한 줄만 추가한다. 실행 명령은 `go test ./precompiles/distribution -run TestPrecompileIntegrationTestSuite -ginkgo.focus tryClaimRewards`이다.

기대 결과: 취약 버전 FAIL(잔고가 보상만큼 많음), 수정 버전 PASS.

## 결과 (2026-11-19 실행, `results/summary.txt`)

| 버전 | 판정 | 기대 | 거래 후 잔고 − (초기 잔고 − gas 비용) |
|---|---|---|---|
| 취약 `5456ce8` | FAIL | FAIL (일치) | **+10,536,620,465,327,065** (≈ 0.0105) |
| 수정 `0fff8c1` | PASS | PASS (일치) | 0 |

- 두 버전 모두 같은 입력이다. 거래 gas limit 80,000, gas 비용 79,415,000,000,000, 청구 가능 보상 28,779,770,773,615,504(≈ 0.0288).
- 취약 버전에서 precompile 호출은 실패했다(`catch`). 그런데도 보상 일부(검증인 한 명 몫, 이 실행에서는 전체의 약 37%)가 잔고에 들어왔다. 회귀 테스트 환경의 검증인은 3명이다. 그래서 검증인 하나의 보상이 지급된 뒤 out of gas가 난 것으로 보인다. 말 그대로 "부분 쓰기"다. 분석 노트 4번의 판단대로 거래 전후 잔고 비교만으로 보인다.
- 지급되는 금액은 실행마다 두 값 중 하나다. 이 실행의 10,536,620,465,327,065 말고 2026-11-19 다른 기계에서 다시 돌렸을 때는 7,706,529,842,961,374(다른 검증인 몫)가 나오기도 했다. 판정(FAIL / PASS)과 7단계 규칙 적중 수는 매번 같았다.
- 첫 시도는 툴체인 다운로드 타임아웃과 C 컴파일러 부재로 테스트가 돌지 않았다. 그 결과는 버렸다. `cases/lib.sh`는 이런 경우를 FAIL이 아니라 ERROR로 판정한다.

## 7단계: 판정기 동작 확인 (`./run.sh --judge`, `results/judge/`)

4단계와 같은 입력에 하네스(v01 계열: O4, O5, O6)와 판정기를 붙여 실행했다(2026-11-19).

| 버전 | 4단계 재현 테스트 | 7단계 판정기 | 기대 | 결과 |
|---|---|---|---|---|
| 취약 `5456ce8` | FAIL (잔고 +10,536,620,465,327,065) | **규칙 적중 6회** (R1-S ×2, R1-E ×4), 원인 1건 | 위반 ≥ 1 | 기대와 일치 |
| 수정 `0fff8c1` | PASS | **위반 0건** | 위반 0 | 기대와 일치 |

취약 버전에서 판정기가 보고한 내용 (최상위 StateDB 커밋 시점):
- R1-S: 위임자 bank 잔고와 distribution 모듈 계정 잔고. 두 값 모두 되돌린 프레임이 남긴 값 그대로 커밋됐다. 위임자 증가분 99,998,007,101,617,353,674,122 − 99,997,996,564,996,888,347,057 = 10,536,620,465,327,065로 4단계 재현의 "추가 잔고"와 정확히 같다(검증인 한 명 몫. 실행에 따라 7,706,529,842,961,374가 나오기도 한다). 같은 기록에서 위임자의 청구 가능 보상은 커밋 시점에도 줄지 않았다.
- R1-E: 실패한 보상 청구의 `coin_received`, `coin_spent`, `message`, `transfer` 이벤트가 커밋된 이벤트에 1번씩 들어갔다.
- 두 규칙은 같은 원인(되돌려지지 않은 보상 송금 1번)을 저장소 쪽(잔고 키 2개)과 이벤트 쪽(종류 4개)에서 각각 잡았다.

설계 대응표(`docs/design-rationale.md` §1)와 비교: 예상 규칙은 R1-S였다. 실제로는 R1-S에 더해 R1-E도 잡혔다(이벤트도 되돌려지지 않았기 때문). 이 버전의 하네스(v01 계열)에는 O2 관찰 지점이 없다. 그래서 O2 기록을 쓰는 R2-W와 R3는 이 사례에 적용되지 않았다. R2-A는 O4 기록으로 적용됐지만 이 기록의 O4 잔고 변경 54건은 변경량이 모두 0이어서 실제로 검사된 것이 없다. 또 R3는 성공한 precompile 호출만 판정하므로(`tool/checker/checker.go` `checkR3`) 실패한 이 호출은 O2가 있어도 R3 대상이 아니다. 이 사례의 증상은 R1-S가 잡았다.
