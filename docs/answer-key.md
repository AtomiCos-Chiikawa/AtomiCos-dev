# 정답지 (재현 대상 사례)

작성일: 2026-11-19 · 근거: GitHub 권고 페이지, 수정 커밋(`git show`로 직접 확인), 7g4w 사후 보고서

이 문서는 구현 순서 1단계(후보 사례 확정)와 2단계(정답지 표)의 산출물이다.
수정 커밋 해시는 모두 `cosmos/evm` 저장소에서 `git log`/`git show`로 확인했다.

---

## 1. 후보 사례 확정

### 선정 (3건)

| 권고 | 권고 페이지 | 세부 내용 공개 여부 | 선정 이유 |
|---|---|---|---|
| GHSA-mjfq-3qr2-6g84 (ISA-2025-004) | 있음 | 공개됨 (원인, 공격 조건, 수정 방식) | 규칙 1(호출 단위 원자성). 회귀 테스트가 있음 |
| GHSA-8pfh-j44r-f654 | 있음 | 공식 원인 미공개 ("More information will be disclosed at a later point"). 수정 커밋 메시지와 회귀 테스트로 원인 추론 가능 | 규칙 1의 다른 유형(try/catch 되돌림 + 중첩 precompile). 회귀 테스트가 있음. 설계·동작 확인용 사례(11단계 평가에서 제외, 아래 메모) |
| GHSA-7g4w-cg88-2cq2 | 있음 | 공개됨 (권고 + [사후 보고서](https://raw.githubusercontent.com/cosmos/security/main/communications/cosmos_evm_GHSA-7g4w-cg88-2cq2_post_mortem.md)) | 규칙 2(잔고 전이 유효성). 실제 공격 사례. 베스팅 계정 필요 |

> 8pfh 표기 원칙: 보고서와 발표에서 원인을 "수정 커밋 79089fe와 회귀 테스트를 근거로 한 추론"으로 표기한다. 공식 원인처럼 쓰지 않는다.
>
> 8pfh의 위치: 확정된 주제 문서는 세부 원인과 재현 방법이 공개되지 않았다는 이유로 GHSA-8pfh를 평가 사례에 넣지 않는다. 그래서 8pfh는 설계·동작 확인용 사례로 두고 11단계 평가에서는 뺀다.
>
> 수정 커밋과 권고의 연결 근거: GitHub Advisory Database의 GHSA-8pfh 항목은 참조 링크에 `79089fe`를 넣었고 Go vulndb GO-2025-4041은 이 커밋을 수정(`FIX`) 참조로 적었다. 권고의 패치 버전 v0.4.2와 v0.3.2에는 같은 패치를 각 버전 구조에 맞춘 백포트(`fe790fb`, `ba20ebc`)가 들어 있다. 커밋 메시지, 릴리스 노트, CHANGELOG에는 권고 ID가 나오지 않는다.
>
> 재현에 쓴 커밋: 취약 커밋 `f463746`은 출시 전 main 스냅샷(v0.5.0-rc.0 이후 85커밋)이고 권고가 영향 버전으로 적은 범위에 들지 않는다. 영향 범위 안의 짝은 v0.4.1/v0.4.2다.

### 제외 (사유)

| 권고 | 내용 | 제외 사유 |
|---|---|---|
| ASA-2026-002 (GHSA-54gx-3cgr-7mfm) | ICS20 precompile 중첩 호출 시 안쪽 변경이 바깥에 반영 안 됨. Saga EVM 약 700만 달러 | 권고는 영향 구성 요소를 ICS20 precompile로, 원인을 "ICS20 precompile이 관여하는 중첩 EVM 실행 경로의 잘못된 상태 처리"로만 밝힌다(수정 위치는 밝히지 않음). IBC 환경이 필요. 권고에 회귀 테스트 언급 없음. 첫 범위(staking·distribution) 밖 |
| GHSA-c6xv-pppp-rjw6 | staking precompile로 언본딩을 만든 컨트랙트가 self-destruct하면 staking 기록이 남음 | 규칙 3 사례로 가치 있음. 하지만 2026-11-02 공개라 공개 코드 취급에 주의가 필요하고 회귀 테스트가 mock 단위 테스트뿐이라 컨트랙트를 새로 짜야 함. 4번째 후보로 보류 |
| GHSA-367m-g444-9mg3 | `StateDB.Commit` 비원자적 기록 + x/erc20 IBC 환불 콜백이 오류를 삼킴 | 공격 경로가 precompile이 아니라 IBC 환불 경로. 후속 범위 |
| GHSA-p8x8-4gg6-prcc | 커밋·flush 오류 시 내부 EVM gas 미청구 | 가용성(DoS) 문제. 잔고 상태 전이 오류가 아님 |
| GHSA-wm4f-qpxp-rch4 | precompile 호출 횟수 카운터 `uint8` wraparound | 가용성(DoS) 문제 |
| GHSA-2m6r / GHSA-7xxp / GHSA-9xwc | fee checker panic / EIP-712 서명 범위 / mempool panic | precompile·상태 전이와 무관 |
| Evmos GHSA-3fp5, GHSA-68fc, GHSA-xgr7 (2024) | precompile 전후 저장소 쓰기 누락 / bank와 EVM 잔고 불일치 / ICS20 컨트랙트 잔고 미차감 | 주제와 직접 관련 있음. 하지만 `evmos/evmos` 저장소라 도구를 다른 코드베이스로 옮겨야 함. 후속 범위 |
| Evmos GHSA-q6hg, GHSA-pxv8, GHSA-7hrh, GHSA-m99c, GHSA-v6rw, GHSA-5jgq | 권한 검사, 베스팅 계정 잔고 계산, precompile을 거친 ante 검사 우회(GHSA-pxv8의 "Missing precompile checks"), authz 등 | precompile과 SDK 사이의 상태 전이 문제가 아님 |

> 범위 메모: 8pfh의 회귀 테스트는 바깥 precompile로 ICS20을 쓰고 프로세스 안에서 IBC 테스트 체인 2개를 띄운다. 되돌려지는 안쪽 호출은 distribution precompile이고 버그 자체는 모든 precompile이 쓰는 공통 코드(`precompiles/common`, `x/vm/statedb`)에 있다. ASA-2026-002를 제외한 사유는 "IBC가 필요해서"만이 아니라 "권고가 영향 구성 요소로 ICS20 precompile을 지목하고(수정 위치는 밝히지 않음), 권고에 회귀 테스트 언급이 없어서"이다.

---

## 2. 정답지 표

### 기본

| | GHSA-mjfq | GHSA-8pfh | GHSA-7g4w |
|---|---|---|---|
| 권고 ID | GHSA-mjfq-3qr2-6g84 (ISA-2025-004) | GHSA-8pfh-j44r-f654 (GO-2025-4041) | GHSA-7g4w-cg88-2cq2 |
| 공개일 | 2025-05-13 (저장소 권고 기준. GitHub 전역 DB는 2025-05-14) | 2025-10-21 | 2026-08-28 (사고 2026-08-20~25) |
| 저장소 | cosmos/evm | cosmos/evm | cosmos/evm |
| 심각도 | Critical (저장소 권고 기준. GitHub 전역 DB는 High, CVSS v4 8.3) | Critical | Critical |

### 버전

| | GHSA-mjfq | GHSA-8pfh | GHSA-7g4w |
|---|---|---|---|
| 영향 버전 (권고) | Cosmos EVM 0.1.0, Evmos > 13.0.0. 패치 버전 필드는 0.6.0으로 표기돼 있음 | 0.3.0~0.3.1, 0.4.0~0.4.1 (패치: 0.3.2, 0.4.2. 본문은 v0.5.0도 언급) | < 0.6.2, 0.7.0~0.7.1 (패치: 0.6.2, 0.7.2) |
| 취약 커밋 (재현에 사용) | `5456ce8dd3d1bed0c7442ab2f0d4146a23efe624` (v0.1.0-26, 수정 직전) | `f4637461878001f6f41acdedad214db247d4f3dd` (main, v0.5.0-rc.0-85, 수정 직전) | `0fe56c036b72aeaf60e5862b2a8bd6d9893a5ef9` (release/v0.7.x, v0.7.1-2, 수정 직전) |
| 수정 커밋 (재현에 사용) | `0fff8c144b24effbcb3addd666150ba5989d631c` ("Merge commit from fork". 태그 v0.2.0과 같은 커밋) | `79089feebe79ce1f35250ba457cbd436e6bfff8b` ("Merge commit from fork") | `0182da198f1f84dc0e4c937fca5e5f175d1be986` (PR #1254, #1176의 백포트) |
| 다른 수정 커밋 | - | 릴리스 태그 v0.4.1 → v0.4.2 (`fe790fb`), v0.3.1 → v0.3.2 (`ba20ebc`). 같은 패치의 백포트 | main: `264aa70f18f4217b0354855a0c056e19c2f51b51` (#1176, 2026-05-15); v0.6.x: `82b3ef6c80887f5b3fb95b1f6ecb519f06202a7e` (#1253, 부모 `d5494fde`) |
| Go 버전 (go.mod) | 1.23.6 | 1.23.8 | 1.25.9 |

### 위치

| | GHSA-mjfq | GHSA-8pfh | GHSA-7g4w |
|---|---|---|---|
| precompile / 모듈 | 모든 상태 변경 precompile (예시: distribution) | 공통 BalanceHandler + StateDB 스냅샷. 회귀 테스트 경로: ICS20 → ERC20 hook → distribution | staking precompile + StateDB + 베스팅 계정(x/auth/vesting) |
| 관련 파일·함수 | `precompiles/common/precompile.go`: `HandleGasError`, `RunAtomic`(신규); `x/vm/statedb/statedb.go`: `RevertMultiStore`(신규); `x/vm/statedb/journal.go`: `precompileCallChange.Revert`; 각 precompile의 `Run` | `precompiles/common/balance_handler.go`: `BalanceHandler`(수정 커밋에서 `BalanceHandlerFactory`로 바뀜); `precompiles/common/precompile.go`: `runNativeAction`; `x/vm/statedb/statedb.go`: `Snapshot`, `RevertToSnapshot` | `x/vm/statedb/state_object.go`: `stateObject.SubBalance`; `precompiles/common/balance_handler.go`: `AfterBalanceChange`; `x/vm/keeper/statedb.go`: `GetAccount`(SpendableCoin), `SetBalanceWithLocked`; `precompiles/common/utils.go`: `ParseAmount` |

### 조건

| | GHSA-mjfq | GHSA-8pfh | GHSA-7g4w |
|---|---|---|---|
| 계정 유형 | 일반 (위임해서 보상이 쌓인 EOA) | 컨트랙트 계정 (위임자이자 보상 수령자) | 베스팅 계정 (잠긴 잔고 보유). 공격에서는 베스팅 계정 주소에 컨트랙트를 배포 |
| 호출 문맥 | 컨트랙트가 `try/catch`로 precompile 호출 + 낮은 gas (거래 gas 80,000) | 중첩: 바깥 precompile(ICS20) 실행 중 EVM 콜백, 이어서 `try/catch` 안에서 distribution `claimRewards` 후 revert (5회) | 직접 호출: 위임자 본인이 staking `delegate(amount > 사용 가능 잔고)` |

### 근거

| | GHSA-mjfq | GHSA-8pfh | GHSA-7g4w |
|---|---|---|---|
| 원인 요약 | precompile이 실패(오류 반환이나 out of gas)하면 그때까지 SDK 캐시에 쓴 변경을 되돌리지 않았다. 성공했을 때만 journal 항목을 추가했기 때문에 호출한 EVM 프레임이 되돌려져도 SDK 변경은 남았다 | (추론) ① 되돌린 스냅샷의 SDK 이벤트가 지워지지 않아 바깥 ICS20 precompile의 BalanceHandler가 되돌려진 안쪽 distribution 호출의 bank 이벤트를 StateDB 잔고에 다시 반영했다. ② 수정 커밋은 precompile 인스턴스마다 하나씩 두던 BalanceHandler를 호출마다 새로 만드는 팩토리(`BalanceHandlerFactory`)로도 바꿨다. 수정 전에도 바깥 ICS20과 안쪽 distribution의 BalanceHandler는 서로 다른 인스턴스였다. ②가 이 회귀 테스트의 실패에 기여했는지는 확인하지 않았다 | StateDB는 사용 가능 잔고만 들고 있다. 그런데 잠긴 잔고까지 위임하면 bank는 전체 금액의 `coin_spent` 이벤트를 낸다. BalanceHandler가 이 금액을 사용 가능 잔고에서 그대로 빼서 검사 없는 uint256 뺄셈이 언더플로(≈2²⁵⁶)했다 |
| 회귀 테스트·PoC | 있음: `precompiles/distribution/integration_test.go`의 `tryClaimRewards` + `DistributionCaller.testTryClaimRewards` | 있음: `evmd/tests/ibc/ics20_recursive_precompile_calls_test.go` (되돌림 있음/없음 2개) + 재귀 ERC20 컨트랙트 2개. 그 밖에 debug precompile 테스트(이벤트 개수만 확인) | 단위 테스트만 있음 (`TestSubBalanceUnderflowPanics`, `ParseAmount`). 베스팅·staking 경로는 없어서 직접 작성 |

### 판단

| | GHSA-mjfq | GHSA-8pfh | GHSA-7g4w |
|---|---|---|---|
| 재현 난이도 | 하: 회귀 테스트를 그대로 사용 | 중: 회귀 테스트는 있지만 IBC 테스트 체인 2개와 ERC20 등록이 필요 | 중: 시나리오(베스팅 계정 주소에 컨트랙트 배포)를 직접 짜야 하고 거래 중간 값을 봐야 함 |
| 재현 방식 | 수정 커밋의 회귀 테스트를 수정 직전 커밋에 넣어 실행 | 같음 | 직접 작성한 테스트를 두 버전에 넣어 실행 (`cases/ghsa-7g4w/test/`) |
| 제외 사유 | - | - | - |

사례별 분석 노트: [`cases/ghsa-mjfq/README.md`](../cases/ghsa-mjfq/README.md), [`cases/ghsa-8pfh/README.md`](../cases/ghsa-8pfh/README.md), [`cases/ghsa-7g4w/README.md`](../cases/ghsa-7g4w/README.md)
