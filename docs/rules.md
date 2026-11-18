# 판정 규칙 명세 (5단계)

작성일: 2026-11-19 · 근거와 사례 대응표: [`design-rationale.md`](design-rationale.md)

이 문서는 6단계 판정기(`tool/checker`)와 관찰 지점(`tool/harness`)이 구현할 규칙의 정의, 관찰 값, 관찰 시점, 판정식을 정한다.
판정기는 특정 사례의 숫자나 공격 절차를 모른다. 아래 규칙만 검사한다.

---

## 0. 기준과 용어

**코드 기준.** 관찰 지점의 위치는 cosmos/evm v0.7.x(`release/v0.7.x`, 7g4w 수정 커밋 `0182da1` 근방) 기준이다. 다른 버전(mjfq의 v0.1.x, 8pfh의 v0.5 이전 main)은 함수 이름과 흐름이 달라서 버전별 어댑터로 같은 값을 뽑는다. 예: v0.1.x에는 `BalanceHandler`가 없고 `journalEntries`로 잔고를 맞춘다.

| 용어 | 뜻 |
|---|---|
| 호출 프레임 | EVM의 CALL/DELEGATECALL/STATICCALL/CREATE 한 번. precompile 호출도 프레임 하나다. 프레임은 성공하거나 `REVERT` 또는 오류(out of gas 등)로 실패한다. |
| 되돌려진 프레임 | 자신이 실패했거나 조상 프레임 중 하나가 실패한 프레임. 성공한 precompile 호출도 바깥 프레임이 되돌려지면 여기에 속한다. |
| SDK 상태 | StateDB의 캐시 컨텍스트(`cacheCtx`) 위에서 보이는 Cosmos 모듈 저장소 값과 `EventManager`의 이벤트 목록 |
| 감시 대상 W | 호출에 관련된 계정 집합(호출자, `tx.origin`, precompile 인자에 나온 주소, bank 이벤트의 spender/receiver)과 그 값. 값은 bank 잔고(EVM denom, 18자리로 환산), StateDB 잔고, 잠긴 잔고 스냅샷, 위임, 언본딩, 재위임, 청구 가능 보상, 관련 모듈 계정 잔고, 총발행량이다. |
| 동기화 지점 | StateDB와 SDK 상태가 서로 맞아야 하는 순간. precompile 진입 직후(`FlushToCacheCtx` 다음)와 precompile 정상 종료 직후(`AfterBalanceChange` 다음)다. |

**공통 제외.** 아래는 규칙 위반으로 보지 않는다.
- 거래 수수료 차감과 gas 환불 (EVM 실행 전후의 ante 단계)
- nonce 변화, fee collector 계정
- 블록 단위 효과 (BeginBlock/EndBlock의 발행, 보상 누적, 언본딩 만기). 관찰은 같은 블록 컨텍스트 안에서 한다.
- EVM denom이 아닌 코인
- R2-W에서는 차단 주소(`BlockedAddr`). 모듈 계정을 따로 빼지는 않는다. 세 사례의 trace에서는 관찰된 모듈 계정이 모두 차단 주소 목록(`blocked`)에 들어 있어서 함께 빠졌다. BalanceHandler도 차단 주소의 bank 이벤트를 건너뛴다.

---

## 1. 관찰 지점

행 번호는 cosmos/evm `0182da1`(v0.7.x 계열, 7g4w 수정 커밋) 기준이다.

| ID | 시점 | v0.7.x 코드 위치 | 기록할 값 | 연결 방법 (6단계) |
|---|---|---|---|---|
| **O1** | 거래 전 / 거래 후 | 테스트 하네스의 블록 실행 전후 | W 전체, 거래 결과(code, gasUsed), 총발행량 | 하네스 (패치 없음) |
| **O2** | precompile 진입 / 종료 | `precompiles/common/precompile.go` `runNativeAction`: 진입은 `FlushToCacheCtx` 다음(83행 근방), 종료는 `AfterBalanceChange` 다음(121행 근방) 또는 오류 반환 지점 | 프레임 ID, precompile 주소·메서드, W의 SDK 값, W의 StateDB 잔고와 잠긴 잔고 스냅샷, 이벤트 수, 반환 오류 | 작은 패치 (`runNativeAction`에 진입·종료 콜백) |
| **O3** | EVM 프레임 진입 / 종료 | geth tracer `OnEnter` / `OnExit` | 깊이, from/to, 입력 선택자, 종료 시 `reverted` 여부와 오류 | tracer (패치 없음) |
| **O4** | StateDB 잔고 변경 | `x/vm/statedb/statedb.go` `AddBalance`(495행), `SubBalance`(504행), `SetBalance`(543행) | 주소, 변경 전 값, 변경량, 변경 후 값, 호출 위치(Go 스택) | 작은 패치. **이 버전의 StateDB는 잔고 변경 때 tracer 훅을 부르지 않는다.** |
| **O5** | 스냅샷 / 되돌림 | `statedb.go` `Snapshot`(691행), `RevertToSnapshot`(699행) | 스냅샷 시점 W의 SDK 값과 이벤트 목록, 되돌림 직후 같은 값 | 작은 패치 |
| **O6** | flush / 커밋 | `statedb.go` `FlushToCacheCtx`(729행), `Commit`(717행) → `keeper.SetAccount` / `SetBalanceWithLocked` | 계정별로 기록하려는 잔고와 실제 기록 결과, 오류 | 작은 패치 |

6단계 구현 상태 (`tool/harness`):

| ID | 구현 | 비고 |
|---|---|---|
| O1 | 미구현 | 테스트가 직접 남길 수 있게 형식만 정의했다(`O1.before/after`) |
| O2 | v0.7·v0.5 계열 | v0.1.x는 아직 구현하지 않았다. `runNativeAction`은 없지만 공통 진입·종료 지점(`5456ce8`의 `precompiles/common/precompile.go` `RunSetup`과 `HandleGasError` 클로저)이 있어서 붙일 수 있다 |
| O3 | 미구현 | 프레임 중첩은 O2 진입·종료와 StateDB 인스턴스로 판단한다(R1). 프레임별 revert 정보가 필요해지면 추가한다 |
| O4 | 세 계열 모두 | 호출 위치(Go 스택)를 함께 기록한다 |
| O5 | 세 계열 모두 | 이벤트는 내용 해시 목록으로 기록한다 |
| O6 | 세 계열 모두 (`Commit`만) | 커밋 직후 기본 컨텍스트(커밋된 결과)를 읽는다. 커밋 중 panic이면 `err`에 남긴다. 훅은 panic만 본다. `Commit`이 error를 반환하면 `err` 없이 기록되고(`statedb_hooks.go` `atomicosAfterCommit`) 판정기는 그 커밋을 성공한 커밋으로 본다. `FlushToCacheCtx`는 아직 관찰하지 않는다 |

모든 읽기는 격리된 컨텍스트에서 무한 gas 미터와 0으로 둔 KV gas 설정, 새 `EventManager`로 한다(`tool/harness/observer/state.go` `isolatedReadCtx`). 그래서 관찰은 gas, 이벤트, 저장소를 바꾸지 않는다. 격리 방법은 저장소 종류에 따라 다르다.
- v0.5와 v0.7 계열의 precompile 캐시 저장소(cosmos/evm `snapshotmulti.Store`): 이 저장소의 `CacheMultiStore()`는 분기를 만들지 않고 스냅샷만 올린 뒤 자기 자신을 돌려준다. 그래서 `CacheContext`로는 격리되지 않는다. 읽기 전에 스냅샷을 잡고 읽은 뒤 그 스냅샷으로 되돌린다. 보상 조회(`DelegationRewards`)는 받은 컨텍스트에 `IncrementValidatorPeriod`로 distribution 값을 쓰는데, 이 쓰기도 되돌림으로 사라진다.
- 그 밖의 저장소(v0.1.x 등): `CacheContext`로 만든 분기에서 읽고 버린다.

처음 구현(e8d8e34)은 모든 계열에서 `CacheContext`를 써서 v0.5·v0.7에서는 보상 조회의 쓰기가 실행 중인 precompile 캐시에 남았다. 같은 거래를 하네스 없이, 수정 전 하네스로, 수정 후 하네스로 커밋해 모든 KV 저장소 값의 해시와 gasUsed를 비교했다. 수정 전 하네스는 gasUsed와 distribution 저장소를 바꿨고(예: 7g4w 수정 버전 위임 128,569 → 126,630), 수정 후 하네스는 하네스 없는 실행과 같았다. 세 사례의 판정, REPRO 수치, 규칙 적중은 수정 전후가 같았다. 업스트림 set A의 기록 수와 호출 수는 조금 달라졌다(`eval/fp-upstream/README.md`).

보상 조회 하나가 실패하거나 panic하면 그 항목만 빼고 나머지 상태는 기록한다. out of gas로 중간에 멈춘 precompile 호출의 종료 시점(O2 종료)에는 distribution 기록이 덜 써진 상태라 SDK의 보상 계산이 nil 참조로 panic한다. 처음 구현은 이 panic 때문에 그 시점의 상태 전체를 읽기 실패(`state.error`)로 남겼다.

사례별 적용 규칙 (`cases/<id>/results/judge/*.trace.jsonl.gz` 기준):

| 사례 | 계열 | 적용한 규칙 | 적용하지 못한 규칙 | 실제 검사량 |
|---|---|---|---|---|
| mjfq | v0.1.x (O2 없음) | R1-S, R1-E, R2-A | R2-W, R3 (둘 다 O2가 필요하다) | O4 기록 54개(취약·수정 각각)가 모두 금액 0이라 R2-A는 사실상 시험되지 않았다. O2를 붙이면 R2-W를 동기화 지점에서 검사하고 실패한 precompile 호출을 직접 볼 수 있다. 다만 mjfq의 `claimRewards`는 실패한 호출이라 O2가 있어도 R3 대상이 아니다 |
| 8pfh | v0.5 계열 | R1, R2-A, R2-W, R3 | - | R3 평가(오류 없이 끝난 O2 진입·종료 짝) 22회: `claimRewards` 20, `delegate` 2. 취약·수정 버전 같음 |
| 7g4w | v0.7.x | R1, R2-A, R2-W, R3 | - | R3 평가: 취약 3회, 수정 1회(모두 `delegate`. 수정 버전의 나머지 2회는 panic으로 끝나 빠졌다) |

---

## 2. 규칙

### R1. 호출 단위 원자성

> 되돌려진 프레임이 만든 SDK 상태 변경과 이벤트는 커밋된 결과에 남으면 안 된다.

**판정 시점 (6단계 구현에서 확정):** 되돌린 직후가 아니라 최상위 StateDB가 커밋한 직후에 판정한다. 버전마다 precompile 쓰기를 되돌리는 방식이 다르기 때문이다.
- v0.7.x: snapshot 저장소를 복원한다.
- v0.1.x: 커밋할 때 캐시를 통째로 버리므로 되돌린 직후 캐시에 보이는 값도 커밋되지 않는다.
- v0.5 계열: 커밋할 때 내보낼 이벤트 목록을 바꿔치기한다.

그래서 "되돌린 직후 보이는 것"은 후보로만 쓴다. 이 기준으로 바꾸기 전에는 수정 버전에서 오탐이 났다(mjfq R1-S 2건, 8pfh R1-E 5건).

공통 절차:
1. O5 되돌림 직후의 SDK 값과 이벤트를 짝이 되는 스냅샷 시점과 비교한다. 달라진 것은 되돌린 프레임이 남긴 후보다.
2. 다른 StateDB가 precompile 호출 중(O2 진입~종료)일 때 처음 나타난 StateDB는 중첩(precompile 안의 `keeper.CallEVM`)으로 본다. 중첩 StateDB의 후보는 최상위 StateDB로 넘긴다. 중첩 StateDB의 컨텍스트가 바깥으로 이어지는지는 버전마다 다르기 때문이다(8pfh 수정 버전은 끊는다).
3. 최상위 StateDB의 O6 커밋 직후, 기본 컨텍스트(커밋된 결과)에서 후보를 확인한다.

#### R1-S (SDK 저장소)
- **판정식:** 후보 키 k에 대해 `커밋값(k) = 되돌린 직후 값(k) ≠ 스냅샷 값(k)`이면 위반
- **관찰:** O2(중첩 판단), O5, O6
- **보조 판정 (거래 단위, 생성기 단계):** O1만 쓸 수 있을 때는 같은 초기 상태에서 "되돌려진 단계를 뺀 시나리오"를 한 번 더 실행하고 수수료를 보정한 W를 비교한다.
- **한계:** 되돌린 뒤 정상 호출이 같은 키를 우연히 같은 값으로 바꾸면 오탐, 다른 값으로 바꾸면 미탐이 될 수 있다.

#### R1-E (이벤트)
- **판정식:** 후보 이벤트(내용 해시 `type:hash`)가 커밋된 이벤트에 나타나면 위반. 이때 StateDB를 처음 관찰했을 때 이미 있던 이벤트는 빼고 센다. (USENIX Sec'26 I2: "되돌려지거나 중단된 경로의 이벤트는 최종 결과에 나타나면 안 된다")
- **관찰:** O2(중첩 판단), O5, O6
- **한계:** 정상 경로가 내용이 똑같은 이벤트를 내면 구별할 수 없다. 판정기는 되돌리지 않은 호출이 정상적으로 낸 이벤트 수를 빼지 않는다. 되돌린 이벤트와 내용 해시가 같은 이벤트가 커밋에 하나라도 있으면 위반으로 본다.

### R2. 잔고 전이의 유효성

> 잔고 가감은 산술적으로 유효해야 하고 동기화 지점에서 StateDB가 나중에 기록할 잔고는 bank의 실제 잔고와 맞아야 한다.

#### R2-A (산술)
- **정의:** StateDB 잔고 변경에서 언더플로(`빼는 양 > 현재 값`)와 오버플로(`결과 ≥ 2²⁵⁶`)는 일어나면 안 된다.
- **판정식:** O4에서 `SubBalance(a, x)`이고 `x > before`이면 위반, `AddBalance(a, x)`이고 `before + x ≥ 2²⁵⁶`이면 위반
- **관찰:** O4
- **비고:** 수정 버전은 언더플로 시 panic한다(`0182da1`). 이 경우에도 O4에서 "시도"를 기록한다. 다만 같은 precompile 호출이 panic으로 끝났으면 등급을 guarded(시도했지만 거부됨)로 보고하고 위반으로 세지 않는다.
  - 한계: guarded 여부는 같은 호출의 O2 종료 오류 문자열이 `panic:`으로 시작하는지로만 정한다(`checker.go` `balanceOutcome`). 그래서 panic 대신 오류를 반환하는 수정은 위반으로 보고되고 같은 호출에서 무관한 panic이 나도 guarded로 보고된다.

#### R2-W (되쓰기 일치)
- **정의:** 동기화 지점(O2 진입 직후, O2 정상 종료 직후)에서, 차단 주소가 아닌 각 계정 a(StateDB에 로드된 계정. 모듈 계정도 차단 주소가 아니면 검사한다)에 대해 "StateDB가 커밋 때 a에 기록할 bank 잔고"는 "지금 SDK 상태의 a의 bank 잔고"와 같아야 한다.
  - 버전별 식(어댑터, `tool/harness/<계열>/statedb_hooks.go`):

    | 계열 | 커밋 때 동작 | 쓰기값 | 기준값 |
    |---|---|---|---|
    | v0.7.x | `SetBalanceWithLocked`: 잔고 + 잠긴 잔고 스냅샷을 bank 총잔고로 기록 | `StateDB(a) + 잠긴 스냅샷(a)` | `지금 읽은 사용 가능 잔고(a) + 지금 잠긴 잔고(a)` |
    | v0.5 계열 | `SetBalance`: 사용 가능 잔고가 StateDB 값이 되도록 mint/burn | `StateDB(a)` | `지금 읽은 사용 가능 잔고(a)` |
    | v0.1.x | `SetBalance`: 총잔고가 StateDB 값이 되도록 mint/burn | `StateDB(a)` | `지금 읽은 총잔고(a)` |

  - "지금 읽은 값"은 버전 자신의 `keeper.GetAccount`로 읽으므로 6자리 denom 체인에서도 단위가 맞는다. 처음엔 bank 값을 그대로 썼다가 8pfh 테스트 체인(6자리)에서 단위가 어긋났다.
- **판정식:** `쓰기값(a) ≠ 기준값(a)`이면 위반
- **관찰:** O2, O4
- **"두 장부가 항상 같다"는 설명이 아닌 이유:** 사용 가능 잔고와 총잔고를 비교하지 않는다. 비교 대상은 구현이 커밋 때 쓰는 식이고 비교 시점은 구현이 동기화를 보장하는 지점뿐이다. 그래서 베스팅 계정에서 EVM 잔고(사용 가능)와 bank 총잔고가 다른 것은 정상으로 본다.
  - 예(7g4w 대조군): 사용 가능 1e18, 잠김 4e18인 계정이 0.5e18을 위임하면 StateDB 0.5e18 + 스냅샷 4e18 = 4.5e18 = bank 총잔고(5e18 − 0.5e18)이므로 위반이 아니다.
  - 6단계 실측: 이 대조군에서 취약·수정 두 버전 모두 R2-W 위반 0건이었다.

### R3. 기능별 상태 일치

> 성공한 precompile 호출은 그 기능이 정한 대로만 SDK 상태를 바꿔야 한다.

O2 진입·종료 값으로 판정하고 성공한 호출에만 적용한다(실패한 호출은 R1이 다룬다).

| ID | 기능 | 판정식 (Δ = 종료 − 진입) |
|---|---|---|
| R3-D | staking `delegate(d, v, x)` | Δbank총잔고(d) = −x + r; Δ(d의 v 위임 tokens) = +x; Δ(검증인 v의 tokens) = +x; 베스팅 계정이면 Δ(DelegatedVesting + DelegatedFree)(d) = x |
| R3-U | staking `undelegate(d, v, x)` | Δ(d의 v 위임 tokens) = −x; 언본딩 기록 +x; Δbank(d) = r (만기 전에는 원금이 오지 않음) |
| R3-C | distribution `claimRewards` / `withdrawDelegatorRewards` | Δbank(인출 주소) = Δ(distribution 모듈 계정 잔고)의 부호 반대; 청구 가능 보상 감소분 ≥ 지급액 Δbank(인출 주소) (검증인 인자가 있으면 그 검증인, 없으면 위임자의 모든 검증인 합. 지급 때 소수점 이하를 버리므로 ≥) |

- **r (자동 인출 보상):** 위임액이 바뀌기 전에 x/distribution 훅(`BeforeDelegationSharesModified` → `withdrawDelegationRewards`)이 그 위임에 쌓인 보상을 인출 주소로 보낸다. r = −Δbank(distribution 모듈 계정)이다. 인출 주소가 위임자와 다르면 위임자 식에서 r을 빼고 Δbank(인출 주소) = r을 따로 확인한다. r에는 진입 때 청구 가능 보상이라는 상한을 두지 않는다(`checker.go` `r3For`). 그래서 distribution 모듈이 청구 가능한 것보다 많이 내보내도 R3-D/U는 알아채지 못한다.
  - 7단계에서 고친 부분: 처음에는 r을 넣지 않아서 업스트림 정상 스위트(set A: staking·distribution)에서 R3-D/U 오탐이 106건 났다. 위반 하나하나의 차이가 정확히 자동 인출 보상과 같았다. 고친 뒤 같은 기록을 다시 판정하니 0건이었다(`eval/fp-upstream/results/summary-A.txt`).
- **근거:** Cosmos SDK v0.50 모듈 불변식(`DelegatorSharesInvariant`, `ModuleAccountInvariants`, `CanWithdrawInvariant`, `ModuleAccountInvariant`)을 호출 단위로 좁힌 것이다. SDK v0.53에서 bank·staking·distribution 모듈 불변식이 제거되고 x/crisis는 deprecated가 됐다. x/crisis는 v0.53.4에는 아직 있고 v0.54.3에는 없다.
- **R3-C의 한계 (구현 기준):** 판정기는 위 표의 두 식만 검사한다. 그래서 지급한 만큼만 청구 가능 보상을 줄이고 나머지를 남긴 청구도, 아무것도 지급하지 않고 아무것도 바꾸지 않은 "성공" 청구도 통과한다. "인출한 검증인들에 대한 청구 가능 보상 = 0" 검사는 아직 구현하지 않았다.
- **후속 후보 R3-L (계정 수명):** EVM에서 삭제된 계정은 위임, 재위임, 언본딩을 가지면 안 된다. GHSA-c6xv 대상이고 아직 범위 밖이다.

### 보조 검사 (단독 판정 기준으로 쓰지 않음)

| ID | 내용 | 관찰 | 쓰는 이유 / 한계 |
|---|---|---|---|
| A1 | 총발행량 보존: 거래 전후 EVM denom 총발행량 변화는 mint/burn 기능 호출로 설명돼야 한다 | O6 커밋 직후 (O1은 미구현) | 7g4w 공격의 총발행량 변화는 0이었다(사후 보고서). 우리 재현에서도 거래 전후로는 변화가 없었다. 단독으로는 부족하다. 구현(`checker.go` `checkA1`)은 StateDB를 처음 관찰했을 때와 O6 커밋 직후의 총발행량이 다르면 aux로 보고만 한다. "mint/burn 기능 호출로 설명" 부분은 구현하지 않았다. |
| A2 | SDK 전역 불변식: bank `TotalSupply`, `NonnegativeBalance`, staking `ModuleAccountInvariants`, distribution `CanWithdraw` | O1 | 미구현. SDK v0.53+에는 없어서 v0.50 정의를 재구현해야 한다. 비용이 크면 거래 끝에서만 돌린다. |

---

## 3. 판정 결과 형식 (6단계 출력)

```json
{
  "rule": "R2-A",
  "point": "O4",
  "frame": ["0 CALL eoa->probe", "1 CALL probe->0x…0800 delegate"],
  "account": "0x…",
  "expected": "sub <= 1000000000000000000",
  "actual": "sub = 3000000000000000000 (after = 2^256 - 2e18)",
  "go_stack": ["x/vm/statedb/statedb.go:504", "precompiles/common/balance_handler.go:…"],
  "severity": "violation | guarded | aux",
  "outcome": "applied | rejected (panic: …)"
}
```

`severity`: `violation`은 규칙 위반이다. `guarded`는 위반이 시도됐지만 시스템이 거부한 경우다(위반 수에 넣지 않음). `aux`는 보조 검사 결과다(판정 기준 아님). CLI(`atomicos-check`)의 판정과 종료 코드: `violation`이 하나라도 있으면 "violation"(1), 없고 trace가 판정 근거로 충분하면 "no violation"(0), 위반은 없지만 trace가 판정 근거가 못 되면 "inconclusive"(3)다. 판정 불가가 되는 경우는 빈 trace, 관찰 지점을 하나도 알아보지 못한 trace, 상태 읽기에 실패한 기록(`state.error`)이 있는 trace다. 출력의 `stats`에는 규칙마다 실제로 검사한 횟수가 나온다.

---

## 4. 7단계에서 확인할 오탐 위험

| 위험 | 관련 규칙 | 확인 방법 |
|---|---|---|
| 잠긴 잔고가 블록 시간에 따라 바뀌는 계정(연속 베스팅) | R2-W | 같은 블록 안에서만 비교하는지 확인하고 연속 베스팅 계정으로 정상 위임 실행 |
| 6자리 denom 체인의 확장 denom 환산(precisebank) | R2-W | 기준값을 버전 자신의 `keeper.GetAccount`로 읽어 단위를 맞췄다(§2 R2-W). 8pfh 테스트 체인(6자리)에서 수정 버전 R2-W 위반은 0건이었다. 6자리 단위로 나누어떨어지지 않는 금액은 이번 trace에 없어서 그 경우는 시험되지 않았다 |
| precompile이 다른 EVM 호출을 부르는 경우(ICS20 → ERC20 hook) 중첩 O2 | R1, R2-W | 8pfh 수정 버전에서 위반 0인지 확인 |
| 수수료·환불이 W에 섞이는 경우 | R1-S 보조 판정, A1 | 수수료를 fee collector 변화량으로 보정 |
| 수정 버전의 panic 거부를 위반으로 볼지 | R2-A | `outcome=rejected`로 따로 집계 |
