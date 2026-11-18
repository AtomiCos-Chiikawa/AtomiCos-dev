# 설계 근거 (5단계)

작성일: 2026-11-19 · 규칙 정의: [`rules.md`](rules.md)

판정 규칙은 세 출처를 합쳐 정했다. 명세(EVM, Cosmos SDK, cosmos/evm 구현), 선행연구(원문 확인), 사례 분석(3단계 노트와 4단계 실행 결과)이다.

---

## 1. 사례 × 규칙 대응표

| 사례 | 깨진 불변식 (3단계 노트 6번) | 규칙 | 위반이 처음 보이는 관찰 지점 | 거래 전후(O1)만으로 보이나 (4단계 결과) |
|---|---|---|---|---|
| GHSA-mjfq | 실패한 precompile 호출의 SDK 변경은 되돌려져야 한다 | R1-S (권고가 말한 보상 증상은 실패한 호출에서 생기므로 R3-C가 아니라 R1-S 대상이다) | v0.1.x에는 O2가 없다. O5 되돌림 직후(후보): 스냅샷 때보다 distribution 모듈 bank가 줄고 위임자 bank가 같은 만큼 늘어난 값이 남아 있다(trace seq 92 → 95). O6 커밋(확정): 그 값이 그대로 커밋됐다(seq 96). 청구 가능 보상은 바뀌지 않았다 | 보인다. 실패한 호출 후 잔고 +10,536,620,465,327,065 (수정 버전 0). 실행마다 두 값 중 하나가 나온다(다른 값은 +7,706,529,842,961,374). 두 값 모두 이 trace에 있는 위임자의 검증인별 청구 가능 보상이다. 2026-11-19 독립 재실행에서는 9회 중 4회가 앞의 값이었다 |
| GHSA-8pfh | 되돌려진 호출의 SDK 변경과 이벤트(및 그 이벤트에서 나온 EVM 잔고 변화)는 함께 되돌려져야 한다 | R1-E, R2-W | O5 되돌림 직후: 이벤트가 남음(R1-E). 이어서 바깥 precompile의 O2 종료: StateDB 잔고 + 스냅샷 ≠ bank(R2-W) | 결과는 보인다. 컨트랙트 잔고 200 (기대 0), 이벤트 49개 vs 20개. 원인(남은 이벤트)은 O5에서만 보인다 |
| GHSA-7g4w | 잔고 가감에서 언더플로가 일어나면 안 된다 (베스팅: 사용 가능·잠긴 잔고 구분) | R2-A, R2-W | O4: `SubBalance(3e18)`인데 변경 전 값이 1e18(R2-A). 이어서 O2 종료: (2²⁵⁶ − 2e18) + 4e18 ≠ 2e18(R2-W) | 보이지 않는다. 단순 시나리오에서 취약·수정 버전 모두 "거래 실패, 총발행량·잔고 변화 없음"으로 끝났다 |

이 표에서 나온 설계 결정:
1. 거래 전후 관찰(O1)만으로는 부족하다. 7g4w가 실험으로 보여 줬다. O2(precompile 진입·종료)와 O4(잔고 변경)가 필요하다.
2. 이벤트도 상태로 취급한다. 8pfh에서 돈의 결과를 만든 것은 되돌려지지 않은 이벤트였다(R1-E).
3. 잔고 동기화는 "구현이 커밋 때 쓰는 식"으로 검사한다(R2-W). 8pfh와 7g4w를 같은 규칙으로 잡는다. 두 사례는 원인이 전혀 다르다(이벤트 정리 누락 vs 산술 언더플로).
4. 총발행량은 보조 검사로 둔다(A1). 7g4w 공격과 우리 재현 모두 총발행량 변화가 0이었다.

---

## 2. 규칙별 근거

| 규칙 | 명세 근거 | 선행연구 근거 | 사례 근거 |
|---|---|---|---|
| R1-S | EVM: 호출이 실패하면 그 호출의 상태 변경은 되돌려진다 (EIP-140, geth `evm.Call`). Cosmos SDK: 상태는 모든 메시지가 성공할 때만 기록된다 (`BaseApp.runTx`, `CacheContext`) | USENIX I3 Storage (커밋 경계의 원자성)를 호출 프레임 경계로 좁힌 것. CertiK ①②③ | mjfq |
| R1-E | Cosmos SDK `CacheContext`: 캐시 컨텍스트의 이벤트는 write할 때만 부모로 전달된다 | USENIX I2 Event: "되돌려지거나 중단된 경로의 이벤트는 최종 결과에 나타나면 안 된다" | 8pfh |
| R2-A | uint256 잔고 의미론(음수·2²⁵⁶ 이상 불가). SDK `NonnegativeBalanceInvariant`(v0.50) | (직접 대응 없음) | 7g4w (언더플로 쪽. 수정 `0182da1`은 `SubBalance` 언더플로 panic만 추가했다). 오버플로 쪽은 GHSA-367m 수정(v0.6.3·v0.7.3)에서 들어간 `AddBalance` 오버플로 panic이 근거다 |
| R2-W | cosmos/evm 구현: StateDB는 사용 가능 잔고를 읽고(`GetAccount`→`SpendableCoin`), 커밋 때 `잔고 + 잠긴 잔고 스냅샷`을 기록한다(`SetBalanceWithLocked`). precompile 전에 `FlushToCacheCtx`, 후에 `BalanceHandler`로 동기화한다. SDK: `DelegateCoins`는 총잔고로 검사하고 전체 금액의 `coin_spent`를 낸다. `LockedCoins` = 위임되지 않은 베스팅 코인 | CertiK ② (EVM과 Cosmos 상태의 충돌) | 7g4w, 8pfh |
| R3 | SDK v0.50 모듈 불변식 (`DelegatorSharesInvariant`, `ModuleAccountInvariants`, `CanWithdrawInvariant`, `ModuleAccountInvariant`). v0.53에서 이 모듈 불변식들이 제거되고 x/crisis는 deprecated가 됐다(x/crisis는 v0.53.4에는 있고 v0.54.3에는 없다) | mjfq 권고문: "it could cause funds to be transferred to a user without resetting the claimable rewards to 0" | mjfq (권고 시나리오) |
| A1, A2 | SDK `TotalSupply` 불변식(v0.50) | - | 7g4w 사후 보고서: 총발행량 변화가 0이라 단독 기준으로 부족 |

---

## 3. 출처 (원문 확인)

### 명세
| 출처 | 확인한 내용 |
|---|---|
| [EIP-140 (REVERT)](https://eips.ethereum.org/EIPS/eip-140) | "execution is aborted, considered as failed, and state changes are rolled back"; 호출한 쪽에는 `0`이 반환된다 |
| geth `core/vm/evm.go` `(*EVM).Call` (cosmos/go-ethereum v1.17.2-cosmos-0) | precompile 호출이든 아니든 `err != nil`이면 `evm.StateDB.RevertToSnapshot(snapshot)`. `ErrExecutionReverted`가 아니면 남은 gas도 0 |
| Cosmos SDK v0.54.3 `baseapp/baseapp.go` `runTx` | "State only gets persisted if all messages get executed successfully". 해당 코드: `if err == nil { … msCache.Write() }` |
| Cosmos SDK v0.54.3 `types/context.go` `CacheContext` | 새 `EventManager`를 가진 캐시 컨텍스트를 만든다. 이벤트는 write할 때만 부모로 전달된다 |
| Cosmos SDK v0.54.3 `x/bank/keeper/keeper.go` `DelegateCoins` | `balance.IsLT(coin)`으로 총잔고를 검사하고 `NewCoinSpentEvent(delegatorAddr, amt)`로 전체 금액을 이벤트로 낸다 |
| Cosmos SDK v0.54.3 `x/auth/vesting/types/vesting_account.go` | `LockedCoins`: "the vesting coins that are not delegated" |
| Cosmos SDK v0.50.13 `x/{bank,staking,distribution}/keeper/invariants.go` | `NonnegativeBalanceInvariant`, `TotalSupply`, `ModuleAccountInvariants`, `DelegatorSharesInvariant`, `CanWithdrawInvariant`, `ModuleAccountInvariant` 등. v0.53.4와 v0.54.3에는 이 파일이 없다 |
| cosmos/evm `0182da1` `precompiles/common/precompile.go` `runNativeAction` | 스냅샷 → journal 등록(`AddPrecompileFn`) → `FlushToCacheCtx` → 실행 → gas 정산 → `AfterBalanceChange` |
| cosmos/evm `0182da1` `x/vm/keeper/statedb.go` | `GetAccount`는 `SpendableCoin`을 쓰고 `SetBalanceWithLocked`는 `잔고 + locked`를 bank에 기록한다 |
| cosmos/evm `0182da1` `x/vm/statedb/statedb.go` | `AddBalance`/`SubBalance`/`SetBalance`에서 tracer 잔고 훅을 부르지 않으므로 O4는 패치가 필요하다 |

### 선행연구
| 출처 | 확인한 내용 | 기획서 서술과 다른 점 |
|---|---|---|
| Ma et al., "The Perils of Flexibility: Uncovering Application-Layer Vulnerabilities in Cosmos SDK Customization Points", USENIX Security 2026 ([페이지](https://www.usenix.org/conference/usenixsecurity26/presentation/ma), [PDF](https://www.usenix.org/system/files/usenixsecurity26-ma.pdf)) | §3.3 불변식 4개: I1 Gas, I2 Event, I3 Storage, I4 Execution Sequence. I3: "Storage updates must satisfy atomicity at the commit boundary. Execution either commits all writes or rolls back all writes, with no partial effects." I2: "if an execution path is reverted or aborted, any events produced along that path must not appear in the final transaction result." 패턴 8개(P1~P8). P5는 Evmos x/evm의 `CommitStateDB`가 다중 메시지 거래에서 되돌려지지 않는 사례(§4.3). 탐지는 "a two-stage process that combines pattern matching with manual validation"(§5) | 기획서 내용과 일치. 추출한 본문에서 "precompile"이라는 단어는 찾지 못했다(PDF 텍스트 추출 기준) |
| CertiK, "EVM – Cosmos Convergence Research From Security Base: Part 2" (2025-06-09, [링크](https://www.certik.com/blog/evm-cosmos-convergence-research-from-security-base-part-2)) | "Security Issues By Inconsistency in States" 아래 소제목 3개: ① "Precompiles Executions Reflects Partial State Transition Changes"(GHSA-3fp5) ② "Precompiles Execution Reversion Collides States Between EVM and Cosmos"(GHSA-68fc) ③ "Precompiles Out-of-Gas Leads To Partial State Write"(GHSA-mjfq). 요약: "synchronization issues, partial reversion, and problems with state writing". 자동화 도구는 제시하지 않고 수동 감사와 `RunAtomic` 패치를 권고 | **기획서와 구현 계획의 "위험 4유형"은 원문과 다르다.** 원문은 소제목 3개다. "장부 동기화 실패"는 별도 소제목이 아니라 요약 문구("synchronization issues")에 해당한다. 인용할 때 "3개 유형"으로 고친다 |

### 사례
- 3단계 분석 노트: [`cases/ghsa-mjfq`](../cases/ghsa-mjfq/README.md), [`cases/ghsa-8pfh`](../cases/ghsa-8pfh/README.md), [`cases/ghsa-7g4w`](../cases/ghsa-7g4w/README.md)
- 4단계 실행 결과: 각 사례의 `results/summary.txt`
- 7g4w 사후 보고서: [cosmos/security](https://raw.githubusercontent.com/cosmos/security/main/communications/cosmos_evm_GHSA-7g4w-cg88-2cq2_post_mortem.md)

---

## 4. 7단계 동작 확인 결과 (2026-11-19)

같은 입력(4단계)에 하네스와 판정기를 붙인 결과다. 실행은 `cases/<id>/run.sh --judge`, 결과는 `cases/<id>/results/judge/`에 있다.

| 사례 | 4단계 재현 (취약 / 수정) | 7단계 판정기: 취약 | 7단계 판정기: 수정 | 설계 대응표의 예상 규칙 | 일치 |
|---|---|---|---|---|---|
| GHSA-mjfq | FAIL / PASS | R1-S ×2, R1-E ×4 | 0 | R1-S | 예 (R1-E도 잡힘) |
| GHSA-8pfh | FAIL / PASS | R1-E ×5, R2-W ×1 (+A1) | 0 | R1-E, R2-W | 예 |
| GHSA-7g4w | FAIL / PASS | R2-A ×2, R2-W ×2 | 0 (guarded R2-A ×2) | R2-A, R2-W | 예 |

표의 수는 규칙 적중 횟수다. 사례마다 원인이 되는 결함은 1건이고 판정기는 그 하나를 서로 다른 규칙으로 여러 번 잡았다(`cases/<id>/results/judge/vuln.check.txt`).
- mjfq: 규칙 적중 6회, 원인 1건. 되돌려진 보상 송금 하나에 저장소 규칙 R1-S(bank 키 2개: distribution 모듈 계정과 위임자)와 이벤트 규칙 R1-E(이벤트 종류 4개: `coin_spent`, `coin_received`, `transfer`, `message`)가 함께 걸렸다.
- 8pfh: 규칙 적중 6회(+A1 1회), 원인 1건. 바깥 ICS20 호출 안에서 distribution `claimRewards`를 5번 부르고 되돌렸다. 그 이벤트 5종류(`coin_spent`, `coin_received`, `transfer`, `message`, `withdraw_rewards`)가 커밋에 남았고(R1-E ×5), 그 결과 바깥 ICS20 호출 종료 시점에 컨트랙트의 StateDB 잔고가 bank와 어긋났다(R2-W ×1).
- 7g4w: 규칙 적중 4회, 원인 1건. 같은 언더플로가 두 실행(eth_call 시뮬레이션과 실제 거래)에서 일어났다. 실행마다 잔고 연산 시점에 산술 규칙 R2-A가, 다음 동기화 지점(O2 종료)에 되쓰기 규칙 R2-W가 걸렸다.

판정기가 못 잡은 것: 세 사례 모두 잡혔다. 다만 아래 두 가지는 판정하지 못했다.
- mjfq의 R3-C(보상 지급과 청구 가능 보상의 일치): v0.1.x 계열에는 O2 관찰 지점이 없다. 그리고 권고 시나리오의 `claimRewards`는 실패한 호출(out of gas)이라 O2가 있어도 R3 대상이 아니다(R3는 성공한 호출에만 적용). mjfq trace에는 권고가 경고한 증상(bank는 지급됐고 청구 가능 보상은 그대로)이 남아 있지만 실패한 호출의 흔적이므로 R1-S가 다루고 실제로 R1-S가 잡았다. R3는 세 취약 사례 어디에서도 위반을 잡지 않았다. R3가 검사된 것은 성공한 호출뿐이다: 7g4w의 `delegate`(취약 3회, 수정 1회), 8pfh의 `claimRewards`와 `delegate`(22회), 업스트림 스위트.
- 7g4w 실제 거래의 커밋 실패(`NewIntFromBigInt() out of bound`) 자체: R1·R2가 아니라 커밋 오류다. O6이 `err`로 기록할 뿐 규칙은 없다.

업스트림 정상 스위트에서 오탐 사전 측정 (`eval/fp-upstream/`, v0.7.4):
- set A (staking·distribution, 스펙 169개): 처음에 R3 106건이 나왔다. 원인은 판정식 오류(자동 인출 보상 누락)였고 고친 뒤 0건이 됐다.
- set B (bank·erc20·werc20·gov·slashing·ics20, 스펙 289개, 설계에 쓰지 않음): 0건이다. R2-A는 47,230회, R2-W는 6,354회 검사됐다(`eval/fp-upstream/results/coverage.md`의 기록 수). R2-A 수에는 금액이 0인 잔고 가감도 들어 있고 0인 가감에서는 R2-A가 걸릴 수 없다. trace와 `coverage.md`를 만든 스크립트가 커밋되지 않아서 저장소만으로는 이 수를 다시 셀 수 없다.
- **R1은 업스트림에서 시험되지 않았다.** 되돌림 15,215회 중 후보가 생긴 경우가 0회였다(v0.7.4는 되돌리는 즉시 정확히 복원).

## 5. 결과를 보고 규칙·관찰을 고친 이력 (자기 채점 주의)

6단계와 7단계에서 실행 결과를 보고 아래를 고쳤다. 대부분 같은 세 사례(또는 set A)의 결과를 보고 고쳤으므로 고친 뒤의 "0건"은 그 데이터에서는 독립 근거가 아니다.

| # | 무엇을 | 계기 | 바꾼 내용 | 근거 |
|---|---|---|---|---|
| 1 | R1-E 판정 시점 | 8pfh 수정 버전 R1-E 5건 | 되돌린 직후 이벤트 수에서 최상위 StateDB 커밋 결과로 바꿈. 중첩 StateDB 후보는 위로 넘김 | USENIX I2("최종 거래 결과에 나타나면 안 된다") |
| 2 | R1-S 판정 시점 | mjfq 수정 버전 R1-S 2건 | 되돌린 직후 캐시 값에서 커밋된 값으로 바꿈 | 버전마다 되돌리는 방식이 다름(v0.1.x는 커밋할 때 캐시를 버림) |
| 3 | R2-W 기준값 | 8pfh(6자리 denom 체인) 단위 불일치 | bank 원시값에서 버전 자신의 `keeper.GetAccount` 읽기로 바꿈 | 구현이 커밋 때 쓰는 식 |
| 4 | R3-D/U | set A 106건 | 자동 인출 보상 r 반영 | SDK `BeforeDelegationSharesModified` 훅 |
| (관찰) | 앱 식별 | EthCall과 IBC에서 상태 읽기 실패 | 키퍼 포인터에서 읽기 시험으로 바꿈 | - |

1~4번은 모두 "구현·명세가 실제로 하는 일"에 맞춰 규칙을 정확하게 만든 방향이고 사례의 숫자나 공격 절차를 넣은 것은 없다. 그래도 일반화를 주장하려면 설계에 쓰지 않은 데이터가 필요하다. 지금까지는 set B(R2만 의미 있게 검사됨)가 유일하다.

## 6. 평가에 대한 주의
- 규칙은 세 사례를 보고 설계했다. 그래서 7단계의 세 사례 확인은 **"동작 확인"이지 성능 평가가 아니다.**
- 일반화의 근거는 다른 곳에서 찾는다.
  - 규칙이 사례가 아니라 명세와 선행연구에서 나왔는가. 위 §2 표가 이걸 보여 준다.
  - 설계에 쓰지 않은 사례에서 작동하는가. 후보: GHSA-c6xv(R3-L), Evmos GHSA-68fc·3fp5, 버그 심기(11단계).
- 8pfh의 위치: 확정된 주제 문서는 세부 원인과 재현 방법이 공개되지 않았다는 이유로 GHSA-8pfh를 평가 사례에 넣지 않는다. 그래서 8pfh는 설계·동작 확인용 사례로 두고 11단계 평가에서는 뺀다.
- R2-W는 이 프로젝트가 새로 정의한 규칙이다. 정상 흐름에서 성립하는지(오탐 0)를 수정 버전과 정상 시나리오로 먼저 확인해야 한다(`rules.md` §4).
