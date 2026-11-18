# GHSA-8pfh-j44r-f654: 되돌려진 호출의 bank 이벤트 + 공유 BalanceHandler

> 주의: **공식 원인은 아직 공개되지 않았다** (권고: "More information will be disclosed at a later point").
> 아래 원인은 수정 커밋 `79089fe`의 diff, 커밋 메시지, 회귀 테스트를 근거로 한 추론이다. 보고서에도 이렇게 표기한다.
> 공식 상세가 아직 공개되지 않았으므로 이 재현 코드를 공개 저장소에 올리기 전에 팀에서 공개 여부를 정한다
> (필요하면 security@cosmos.network에 문의).
>
> 사례의 위치: 주제 문서는 원인이 공식 공개되지 않았다는 이유로 8pfh를 평가 사례에서 뺐다. 그래서 이 사례는 **설계·동작 확인용 사례**로 두고 11단계 평가에서는 제외한다. 원인은 위와 같이 수정 커밋을 근거로 한 추론이다.

| 항목 | 값 |
|---|---|
| 권고 | https://github.com/cosmos/evm/security/advisories/GHSA-8pfh-j44r-f654 |
| 취약 커밋 | `f4637461878001f6f41acdedad214db247d4f3dd` (main, 수정 직전) |
| 수정 커밋 | `79089feebe79ce1f35250ba457cbd436e6bfff8b` |
| 재현 방식 | 수정 커밋의 회귀 테스트(`ics20_recursive_precompile_calls_test.go`)와 컨트랙트를 수정 직전 커밋에 넣어 두 버전에서 실행 |
| 실행 | `./cases/ghsa-8pfh/run.sh` |

## 분석 노트

### 1. 무엇이 잘못됐나 (추론)
수정 커밋은 독립된 두 가지를 고쳤다.

1. 되돌린 스냅샷의 이벤트가 남음. `StateDB.Snapshot()`은 journal 위치만 기억했고 `RevertToSnapshot()`은 SDK `EventManager`를 되돌리지 않았다. 그래서 `try/catch`로 되돌린 precompile 호출이 낸 bank 이벤트(`coin_spent`, `coin_received`)가 이벤트 목록에 그대로 남았다.
2. BalanceHandler 공유. precompile마다 `BalanceHandler` 인스턴스가 하나뿐이었다(`Precompile.BalanceHandler`). 중첩 호출이 같은 인스턴스의 `prevEventsLen`을 덮어쓸 수 있었다.

BalanceHandler는 precompile이 끝날 때(`AfterBalanceChange`) 이벤트 목록의 `prevEventsLen` 이후 구간을 훑는다. 그리고 `coin_spent`/`coin_received`를 StateDB의 `SubBalance`/`AddBalance`로 옮긴다. 그래서 바깥 precompile이 실행되는 동안 안쪽에서 되돌려진 호출의 이벤트가 바깥 precompile의 구간에 들어가면, 되돌려진 잔고 변화가 StateDB에 다시 반영된다.

### 2. 언제 터지나
- 계정: 위임해서 보상이 쌓인 컨트랙트 계정 (회귀 테스트에서는 ERC20 컨트랙트 자신)
- 호출 방식: 중첩 + try/catch 되돌림
  1. 사용자가 ICS20 precompile `transfer`로 이 ERC20 토큰을 보낸다 (바깥 precompile, BalanceHandler 있음).
  2. 토큰 변환 중 ERC20 `_beforeTokenTransfer` hook이 실행된다 (EVM 콜백).
  3. hook이 `try this.claimRewardsAndRevert() {} catch {}`를 5번 반복한다. 안쪽에서 distribution `claimRewards`가 성공한 뒤 `revert()`된다.
- gas 조건: 없음

### 3. 어떤 상태가 어긋나나
- Cosmos 쪽: distribution 상태는 되돌려져서 보상은 여전히 청구할 수 있다(회귀 테스트 기대값 `50.000000000000000000`). 다만 취약 버전의 4단계 테스트는 그 앞의 잔고 assertion에서 멈추므로 이 부분은 테스트로는 확인되지 않았다. 근거는 7단계 기록이다. 최상위 커밋 시점(`results/judge/vuln.trace.jsonl.gz` seq 151)에 컨트랙트의 청구 가능 보상은 50으로 남아 있고 bank 잔고는 200이다.
- EVM 쪽: 바깥 ICS20 precompile의 BalanceHandler가 되돌려진 `claimRewards`의 `coin_received`를 다시 반영한다. 그래서 컨트랙트의 StateDB 잔고가 늘고 커밋할 때 bank에 기록된다.
- 결과: 보상은 그대로 남아 있는데 잔고도 늘어나서 같은 보상이 두 번 존재하는 상태가 된다. 회귀 테스트는 컨트랙트 잔고가 0이어야 한다고 본다.

### 4. 언제 보이나
- 돈의 결과는 **거래 전후만 봐도 보인다**: 컨트랙트의 bond-denom 잔고(기대값 0)와 청구 가능 보상(기대값 그대로).
- 원인(되돌려진 이벤트가 바깥 precompile의 구간에 남는 것)은 **실행 도중**에만 보인다. 이벤트 목록과 BalanceHandler 구간은 거래가 끝나면 사라지기 때문이다.

### 5. 어떻게 고쳤나
- `revision`에 그 시점의 `EventManager().Events()`를 저장한다. `RevertToSnapshot`에서 EventManager를 그 이벤트로 다시 만든다 (`x/vm/statedb/statedb.go`).
- `BalanceHandler`를 `BalanceHandlerFactory`로 바꿔서 `runNativeAction` 호출마다 새 인스턴스를 만든다 (`precompiles/common/{balance_handler,precompile}.go`, 각 precompile 생성자).

### 6. 어떤 불변식이 깨졌나
> **되돌려진 호출이 만든 SDK 상태 변경과 이벤트(그리고 그 이벤트에서 나온 EVM 잔고 변화)는 함께 되돌려져야 한다.** (규칙 1: 호출 단위 원자성. "이벤트도 되돌려져야 한다"는 부분이 mjfq와 다르다)

## 재현 방법
`run.sh`는 수정 커밋에서 아래 파일을 그대로 가져와 두 worktree에 넣는다.
- `evmd/tests/ibc/ics20_recursive_precompile_calls_test.go` (하위 테스트 2개: "with reverts"는 버그 경로, "without reverts"는 대조군)
- `contracts/erc20_recursive_{reverting,non_reverting}.go`, `contracts/solidity/ERC20Recursive*PrecompileCall.{sol,json}`

assertion은 바꾸지 않았다. 잔고를 비교하기 직전에 값과 이벤트 수를 출력하는 `REPRO` 줄만 넣는다.
`evmd/tests/integration/balance_handler/*`(debug precompile 테스트)는 수정 후 API(`BalanceHandlerFactory`)로 작성돼 있어서 수정 직전 커밋에서는 컴파일되지 않는다. 또 이벤트 개수만 확인하는 테스트라 사용하지 않았다.

기대 결과: 취약 버전 FAIL("with reverts"에서 컨트랙트 잔고가 0이 아님), 수정 버전 PASS.

## 결과 (2026-11-19 실행, `results/summary.txt`)

| 버전 | 판정 | 기대 | "with reverts" 거래 후 컨트랙트 잔고 (기대 0) | "without reverts" (대조군, 기대 50) | 거래 이벤트 수 (with / without) |
|---|---|---|---|---|---|
| 취약 `f463746` | FAIL | FAIL (일치) | **200** | 50 (PASS) | 49 / 29 |
| 수정 `79089fe` | PASS | PASS (일치) | 0 | 50 (PASS) | 20 / 29 |

- 취약 버전은 "with reverts" 하위 테스트에서만 실패했다. 대조군 "without reverts"는 두 버전 모두 통과했다. 실패 원인이 "되돌림" 경로에 있다는 뜻이다.
- 되돌려진 `claimRewards`는 지급되지 않아야 하는데 취약 버전에서는 보상(50)의 4배인 200이 컨트랙트 잔고로 들어왔다. 이벤트도 29개 더 남았다(49 vs 20). 7단계 기록으로 나눠 보면 20개는 되돌린 보상 청구의 이벤트(5종 × 4벌)이고 9개는 커밋이 StateDB의 200을 bank에 기록하면서 새로 생긴 이벤트(`coinbase`, `fractional_balance_change` 등)다. 앞의 20개는 되돌린 호출의 이벤트가 지워지지 않았다는 추론(분석 노트 1번)과 맞는다. 4배가 되는 경로는 6단계 판정기 실행에서 확인됐다(아래 "6단계 관찰").

**6단계 관찰 (판정기 사전 실행).** 취약 버전 기록에서 아래가 보였다.
- 되돌린 보상 청구의 이벤트 5종(`coin_received`, `coin_spent`, `message`, `transfer`, `withdraw_rewards`)이 각각 4번씩 최종 커밋 이벤트에 들어갔다(R1-E). 5번 되돌렸는데 4번만 남은 이유는 다음과 같다. 매 precompile 호출이 진입할 때 캐시 이벤트 목록을 저장하는데 이 목록에 그 전에 되돌린 호출의 이벤트가 섞여 있다. 그래서 마지막 호출의 되돌림 목록에 앞선 4번분이 실려 중첩 StateDB에서 커밋됐다. 이 20개(5종 × 4벌)는 수정 버전에서도 중첩 StateDB의 커밋에 똑같이 들어간다(수정 버전 기록 seq 139, 커밋 이벤트 20개). 두 버전의 차이는 이 이벤트가 바깥 ICS20 precompile의 컨텍스트로 넘어가 최상위 커밋에 남는지다(취약 20개 모두 남음, 수정 0개).
- 바깥 ICS20 precompile이 끝날 때, StateDB에는 bank에 없는 200(EVM 단위 2e14)이 들어가 있었다(R2-W). 바깥 BalanceHandler가 이 4번분의 `coin_received`를 잔고로 반영한 결과다.
- 수정 버전에서는 같은 입력에서 위반이 0건이었다.
- 실행 메모: 이 버전의 evmd 테스트는 `-tags=test`가 필요하다(업스트림 Makefile과 같음). 처음에는 이 태그가 빠져 SetupTest에서 panic이 났고 그 결과는 버렸다.

## 7단계: 판정기 동작 확인 (`./run.sh --judge`, `results/judge/`)

4단계와 같은 입력에 하네스(v05 계열: O2, O4, O5, O6)와 판정기를 붙여 실행했다(2026-11-19).

| 버전 | 4단계 재현 테스트 | 7단계 판정기 | 기대 | 결과 |
|---|---|---|---|---|
| 취약 `f463746` | FAIL (컨트랙트 잔고 200, 기대 0) | **규칙 적중 6회** (R1-E ×5, R2-W ×1), 원인 1건 (+ 보조 A1 ×1) | 위반 ≥ 1 | 기대와 일치 |
| 수정 `79089fe` | PASS | **위반 0건** | 위반 0 | 기대와 일치 |

취약 버전에서 판정기가 보고한 내용:
- R2-W (바깥 ICS20 precompile 종료 시점): 컨트랙트의 StateDB 잔고가 2e14(EVM 단위, 보상 200)인데 bank 기준값은 0이다. 이 상태로 커밋하면 bank에 없는 200이 생긴다.
- R1-E (최상위 StateDB 커밋 시점): 되돌린 보상 청구의 이벤트 5종이 각각 4번씩 커밋된 이벤트에 들어갔다. 5번 되돌린 흔적은 중첩 StateDB에서 나왔는데 판정기가 이것을 최상위 StateDB로 넘겨 판정했다.
- 보조 A1: 거래 동안 총발행량이 200 늘었다.

설계 대응표와 비교: 예상 규칙 R1-E, R2-W가 모두 잡혔다. 같은 원인을 이벤트 규칙 R1-E는 되돌린 보상 청구의 이벤트에서, 되쓰기 규칙 R2-W는 그 결과인 잔고 불일치에서 각각 잡았다. R1-S는 보고되지 않았다. 저장소(distribution 상태)는 제대로 되돌려졌고 문제는 이벤트와 그 이벤트로 바뀐 EVM 잔고였기 때문이다. 분석 노트의 추론과 맞는다.
