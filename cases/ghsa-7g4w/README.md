# GHSA-7g4w-cg88-2cq2: 베스팅 계정 위임 시 StateDB 잔고 언더플로

| 항목 | 값 |
|---|---|
| 권고 | https://github.com/cosmos/evm/security/advisories/GHSA-7g4w-cg88-2cq2 |
| 사후 보고서 | https://raw.githubusercontent.com/cosmos/security/main/communications/cosmos_evm_GHSA-7g4w-cg88-2cq2_post_mortem.md |
| 취약 커밋 | `0fe56c036b72aeaf60e5862b2a8bd6d9893a5ef9` (release/v0.7.x, 수정 직전, v0.7.1-2) |
| 수정 커밋 | `0182da198f1f84dc0e4c937fca5e5f175d1be986` (PR #1254, main의 #1176 = `264aa70f` 백포트, v0.7.2) |
| 재현 방식 | 수정 커밋의 회귀 테스트가 StateDB 단위 테스트뿐이라 직접 작성 (`test/`) |
| 실행 | `./cases/ghsa-7g4w/run.sh` |

## 분석 노트

### 1. 무엇이 잘못됐나
- `x/vm/keeper/statedb.go` `GetAccount`: StateDB가 보는 잔고는 `SpendableCoin`(사용 가능 잔고)뿐이다. 잠긴 잔고는 로드할 때 스냅샷(`lockedBalance`)으로 따로 보관했다가 커밋할 때 다시 더한다(`SetBalanceWithLocked`).
- x/staking은 베스팅 계정이 잠긴 코인도 위임할 수 있게 한다(`TrackDelegation`). 이때 bank는 위임 금액 전체를 `coin_spent` 이벤트로 낸다.
- `precompiles/common/balance_handler.go` `AfterBalanceChange`는 이 이벤트 금액을 그대로 `stateDB.SubBalance(위임자, 금액)`으로 옮긴다.
- `x/vm/statedb/state_object.go` `SubBalance`는 `new(uint256.Int).Sub(balance, amount)`를 검사 없이 계산했다. 그래서 `금액 > 사용 가능 잔고`이면 잔고가 약 2²⁵⁶으로 wrap됐다.

### 2. 언제 터지나
- 계정: 잠긴 잔고가 있는 베스팅 계정이 위임자일 때. 공격자는 미리 계산한 주소에 베스팅 계정을 만들고 같은 주소에 컨트랙트를 배포했다. 사후 보고서는 베스팅 계정을 만든 방법을 밝히지 않았다. `MsgCreateVestingAccount`는 우리 재현(아래 "재현 방법" 2번)에서 쓴 방법이다.
- 호출 방식: 위임자 본인이 staking precompile `delegate(위임자, 검증인, 금액)`을 직접 호출한다(`msg.sender == 위임자` 검사 통과).
- 금액 조건: `사용 가능 잔고 < 금액 ≤ 총잔고`
- gas 조건: 없음

### 3. 어떤 상태가 어긋나나
- Cosmos 쪽: 위임은 정상 처리된다. bank 총잔고가 금액만큼 줄고 위임 기록이 생기고 `DelegatedVesting`이 늘어난다.
- EVM 쪽: StateDB의 위임자 잔고가 `2²⁵⁶ − (금액 − 사용 가능 잔고)`로 wrap된다. EVM 안에서 이 계정은 잔고가 거의 무한한 것처럼 보인다.
- 사후 보고서에 따르면, 공격자는 이 잔고를 피해자 주소로 보냈다. 피해자 쪽 덧셈이 오버플로해 서로 상쇄됐고 결과적으로 피해자 잔고를 가져가면서 총발행량 변화는 0이었다.

### 4. 언제 보이나
- **실행 도중을 봐야 확실히 보인다.** 언더플로는 precompile 호출이 끝나는 순간 StateDB 안에서 일어난다. 사후 보고서의 공격처럼 같은 거래 안에서 잔고를 옮겨 상쇄하면, 거래 전후의 총발행량 비교로는 보이지 않는다.
- 실행 결과: 단순 시나리오(상쇄 없음)에서는 취약 버전도 커밋 단계에서 거래가 실패한다. 그래서 거래 전후로는 수정 버전과 구별되지 않는다(아래 "결과" 참고).
- 버전에 따라 거래 종료 후 모습이 다르다(사후 보고서). 0.6.x는 Mint/Burn을 써서 거대한 양을 발행하고 그 때문에 체인이 정지한다. 0.7.x는 bank에 직접 기록한다.

### 5. 어떻게 고쳤나
- `stateObject.SubBalance`: `balance < amount`이면 `panic("state balance underflow ...")`한다.
- `ParseAmount`: 기본 denom과 확장 denom 금액을 합산하도록 바꿨다(같은 PR의 이벤트 금액 처리 강화).
- 회귀 테스트: `TestSubBalanceUnderflowPanics`, `TestParseAmount` 케이스 추가(단위 테스트).
- 범위: 이 수정 커밋은 언더플로(`SubBalance`)와 `ParseAmount`만 다룬다. 사후 보고서가 설명한 두 번째 결함(피해자 잔고 덧셈의 오버플로)은 이 커밋에서 고치지 않았다. `AddBalance`의 오버플로 panic은 나중에 GHSA-367m 수정(`StateDB.Commit` 원자화)과 같은 보안 릴리스(v0.6.3 / v0.7.3)에서 들어갔다.

### 6. 어떤 불변식이 깨졌나
> **잔고를 빼거나 더할 때 언더플로·오버플로가 일어나면 안 된다. 베스팅 계정은 사용 가능 잔고와 잠긴 잔고를 구분하고 기능이 허용한 이동만 반영해야 한다.** (규칙 2: 잔고 전이의 유효성)

## 재현 방법 (직접 작성한 테스트)
`test/vesting_delegate_underflow_test.go`를 두 worktree의 `evmd/tests/integration/atomicos_ghsa7g4w/`에 넣고 실행한다.

1. 배포자의 다음 CREATE 주소 P를 계산한다.
2. `MsgCreateVestingAccount`(delayed, 4e18 잠김)로 P에 베스팅 계정을 만들고 bank 송금으로 사용 가능 잔고 1e18을 넣는다.
3. 관찰용 컨트랙트 `contracts/VestingDelegateProbe.sol`을 배포해 P에 놓이게 한다. P가 여전히 베스팅 계정인지 확인한다.
   - `probe(validator, amount)`는 `address(this).balance`를 읽고 staking `delegate(address(this), validator, amount)`를 호출한 뒤 다시 `address(this).balance`를 읽어 두 값을 반환한다. 다른 곳으로 자금을 옮기지 않는다.
4. `eth_call`(커밋하지 않음)로 실행한다.
   - 대조군: `금액 = 사용 가능 잔고 / 2`일 때 위임이 성공해야 한다.
   - 대상: `금액 = 사용 가능 잔고 + 잠긴 잔고 / 2`일 때 `after > before`(wrap)면 실패한다.
5. (참고, assertion 없음) 같은 호출을 실제 거래로 보내고 거래 전후의 총발행량과 P의 bank 잔고만 기록한다. "거래 전후만 보는 검사기"가 무엇을 보게 되는지 확인하는 용도다.

기대 결과: 취약 버전 FAIL(after가 2²⁵⁶ 근처로 wrap), 수정 버전 PASS(`state balance underflow` panic으로 호출 실패).

수정 버전은 이 위임을 panic으로 거부한다. 즉 위임이 성공하면서 언더플로가 사라지는 것이 아니라, 위임이 거부돼서 언더플로가 일어나지 않는다. 테스트의 assertion은 잔고가 wrap되지 않았는지만 보고 호출이 실패한 이유는 검사하지 않는다. 실패 이유가 `state balance underflow` panic이라는 것은 `results/fixed.log`의 `REPRO` 줄로 확인한다.

컨트랙트를 바꾸면 `SOLC=<solc 0.8.24 경로> contracts/gen.sh`로 `test/probe_contract.go`를 다시 생성한다.

## 결과 (2026-11-19 실행, `results/summary.txt`)

공통 준비 상태(두 버전 동일): P는 `DelayedVestingAccount`이고 bank 총잔고 5e18 = 사용 가능 1e18 + 잠김 4e18. 컨트랙트를 배포한 뒤에도 베스팅 계정 유형이 유지됐다.

| 관찰 | 취약 `0fe56c0` | 수정 `0182da1` |
|---|---|---|
| 대조군: `delegate(0.5e18)` (eth_call) | 성공, StateDB 잔고 1e18 → 0.5e18 | 같음 |
| **대상: `delegate(3e18)`** (eth_call) | 성공, StateDB 잔고 1e18 → **2²⁵⁶ − 2e18** (wrap) | 호출 실패: `panic: state balance underflow ... have=1e18 sub=3e18` |
| 판정 | FAIL (기대 FAIL과 일치) | PASS (기대 PASS와 일치) |
| 참고: 같은 호출을 실제 거래로 | 거래 실패 (code 111222, 커밋 시 `NewIntFromBigInt() out of bound`) | 거래 실패 (code 111222, `state balance underflow`) |
| 참고: 거래 전후 총발행량 / P의 bank 잔고 | 변화 없음 / 변화 없음 | 변화 없음 / 변화 없음 |

**설계에 중요한 관찰 (분석 노트 4번 확정):**
- 실행 도중 StateDB 값을 보면 두 버전이 확실히 갈린다(wrap vs panic).
- **거래 전후만 보면 두 버전이 똑같이 "거래 실패, 상태 변화 없음"으로 보인다.** 이 단순 시나리오에서 취약 버전은 wrap된 잔고를 커밋하다가 실패해서 거래 전체가 되돌려졌다. 공격자는 같은 거래 안에서 이 잔고를 옮겨 상쇄했다(사후 보고서).
- 그래서 거래 종료 후 잔고·총발행량 비교만으로는 이 버그를 볼 수 없고 precompile 호출 직후 시점의 관찰 지점이 필요하다는 근거가 된다.

실행 메모: `-tags=test`가 필요하다(업스트림 Makefile과 같음).

## 7단계: 판정기 동작 확인 (`./run.sh --judge`, `results/judge/`)

4단계와 같은 입력에 하네스(v07 계열: O2, O4, O5, O6)와 판정기를 붙여 실행했다(2026-11-19).

| 버전 | 4단계 재현 테스트 | 7단계 판정기 | 기대 | 결과 |
|---|---|---|---|---|
| 취약 `0fe56c0` | FAIL (StateDB 잔고 wrap) | **규칙 적중 4회** (R2-A ×2, R2-W ×2), 원인 1건 | 위반 ≥ 1 | 기대와 일치 |
| 수정 `0182da1` | PASS | **위반 0건** (guarded R2-A ×2) | 위반 0 | 기대와 일치 |

취약 버전에서 판정기가 보고한 내용. 언더플로 1건을 산술 규칙 R2-A가 잔고 연산 시점에, 되쓰기 규칙 R2-W가 다음 동기화 지점(precompile 종료)에서 잡았다. 같은 호출이 eth_call 모의 실행(`db22`, 커밋 없음)과 실제 거래(`db23`, 커밋 시 panic)에서 한 번씩 실행돼서 규칙마다 2회다:
- R2-A (O4): `SubBalance(3e18)`인데 변경 전 값은 1e18이다. 호출 위치는 `precompiles/common/balance_handler.go` `AfterBalanceChange` → `x/vm/statedb/statedb.go` `SubBalance`다.
- R2-W (O2 종료): 쓰기값 (2²⁵⁶ − 2e18) + 4e18이 bank 기준값 2e18과 다르다.

수정 버전에서는 같은 `SubBalance(3e18)` 시도가 `state balance underflow` panic으로 거부됐다. 판정기는 이를 guarded로 기록하고 위반으로 세지 않았다.

대조군(사용 가능 잔고 안의 정상 위임 0.5e18): 두 버전 모두 R2-W, R3-D 위반이 없었다. R2-W가 베스팅 계정의 정상 흐름에서 성립한다는 실측이다.

거래 전후만 보는 방식과 비교: 4단계에서 확인했듯 실제 거래는 두 버전 모두 "실패, 총발행량·잔고 변화 없음"으로 끝났다. 판정기는 실행 도중 관찰(O4, O2)로 두 버전을 구별했다.

설계 대응표와 비교: 예상 규칙 R2-A, R2-W가 모두 잡혔다.
