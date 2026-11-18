# tool: 생성기, 러너, 관찰 지점, 판정기 (6·8단계)

규칙 정의는 [`../docs/rules.md`](../docs/rules.md)에 있다. 각 부분은 파일로만 주고받아서 서로 바꿔 끼울 수 있다.

```
tool/generator ──> scenario/*.json ──(tool/harness/runner: cosmos/evm 안에서 실행)
                                          └─(tool/harness 훅)──> trace.jsonl ──(tool/checker)──> 위반 목록
```

지금 저장소에 있는 것은 판정기(`checker`)와 관찰 지점(`harness`)이다. 생성기와 러너는 8단계 설계이고 아직 저장소에 없다(아래 각 절).

## checker: 판정 부분 (`atomicos/tool` Go 모듈, 표준 라이브러리만 사용)

| 파일 | 내용 |
|---|---|
| `checker/trace.go` | 기록 형식(JSON Lines). 관찰 지점 이름, `State`, `ReadTrace` |
| `checker/checker.go` | 규칙: R1-S/R1-E(커밋 시점 판정, 중첩 StateDB 처리), R2-A, R2-W, R3-D/U/C, 보조 A1 |
| `checker/checker_test.go` | 합성 기록으로 만든 단위 테스트. 규칙마다 위반·정상 경우를 둔다(권고 사례의 숫자는 쓰지 않음) |
| `checker/cmd/atomicos-check` | CLI. 종료 코드 0 위반 없음, 1 위반, 3 판정 불가(빈 trace, 상태 읽기 실패 등). 규칙별 검사 횟수(`stats`)도 출력 |

판정기는 기록만 읽는다. 어떤 사례, 버전, 공격인지 모르고 사례별 숫자도 갖고 있지 않다.

```bash
cd tool && go test ./...
```

```bash
cd tool && go run ./checker/cmd/atomicos-check [-json] trace.jsonl
```

## harness: 관찰 부분 (cosmos/evm 트리에 넣어서 빌드)

`tool/harness`는 별도 `go.mod`로 `atomicos/tool` 빌드에서 빠져 있다. 이 소스는 `inject.sh`가 cosmos/evm worktree에 복사한 뒤 그 버전에 맞춰 컴파일된다.

| 파일 | 넣는 위치 | 내용 |
|---|---|---|
| `v07/statedb_hooks.go` | `x/vm/statedb/atomicos_hooks.go` | v0.7.x 훅. 쓰기 방식: 잔고 + 잠긴 잔고 스냅샷 = bank 총잔고 |
| `v05/statedb_hooks.go` | 〃 | v0.5 계열(8pfh) 훅. 쓰기 방식: 잔고 = 사용 가능 잔고 |
| `v01/statedb_hooks.go` | 〃 | v0.1.x(mjfq) 훅. `*big.Int` 잔고. O2는 아직 구현하지 않음(`runNativeAction`은 없지만 `RunSetup`·`HandleGasError`에 붙일 수 있다). 쓰기 방식: 잔고 = 총잔고 |
| `observer/observer.go`, `observer/state.go` | `evmd/atomicos/` | 공통 기록기: 상태 읽기, JSONL 쓰기, 앱 식별 |
| `v07/abis.go` (v05 동일), `v01/abis.go` | `evmd/atomicos/abis.go` | precompile 메서드·인자 해석 |
| `inject.sh` | - | 훅 호출을 `statedb.go`, `precompile.go`, `evmd/app.go`에 끼워 넣는다. 끼워 넣은 곳마다 정확히 한 번 들어갔는지 검사한다 |

```bash
tool/harness/inject.sh v07 .work/ghsa-7g4w-vuln
```

```bash
cd .work/ghsa-7g4w-vuln/evmd && ATOMICOS_TRACE=/tmp/trace.jsonl go test -tags=test ./tests/integration/atomicos_ghsa7g4w/ -run TestGHSA7g4w -count=1
```

**동작 원칙**
- `ATOMICOS_TRACE`가 없으면 훅은 `nil`이고 끼워 넣은 코드는 아무 일도 하지 않는다.
- 상태 읽기는 격리된 컨텍스트(새 `EventManager`)에서 무한 gas 미터와 0으로 둔 KV gas 설정으로 한다. 그래서 gas, 이벤트, 저장소는 바뀌지 않는다. v0.5와 v0.7 계열의 precompile 캐시(`snapshotmulti.Store`)는 `CacheMultiStore()`가 분기를 만들지 않으므로 스냅샷을 잡고 읽은 뒤 되돌린다. 그 밖의 저장소는 `CacheContext` 분기에서 읽는다(`observer/state.go` `isolatedReadCtx`, 자세한 내용과 확인 방법은 [`../docs/rules.md`](../docs/rules.md) §1). 계정을 새로 로드하지 않는다(이미 로드된 StateDB 계정만 본다).
- precompile 종료 훅은 panic을 잡아 기록한 뒤 같은 값으로 다시 panic한다(동작 동일).
- 앱 식별: EVM 키퍼 포인터로 찾는다. 실패하면 앱이 하나일 때 그 앱을 쓴다. 여러 개면 그 컨텍스트에서 읽기가 성공하는 앱을 쓴다. `EthCall`은 키퍼 복사본으로 StateDB를 만들고 IBC 테스트는 앱이 여러 개라서 이 처리가 필요하다.

## 버전 계열 대응

| 사례 | 계열 | 관찰 지점 |
|---|---|---|
| GHSA-7g4w (`release/v0.7.x`) | v07 | O2 O4 O5 O6 |
| GHSA-8pfh (v0.5 이전 main) | v05 | O2 O4 O5 O6 |
| GHSA-mjfq (v0.1.x) | v01 | O4 O5 O6 (O2는 아직 구현하지 않음) |

## generator: 입력 생성기 (8단계, `atomicos/tool/generator`)

> **8단계 설계다. `tool/generator`는 아직 저장소에 없다.** 아래는 만들 내용이다.

시나리오(JSON) 하나는 "새 네트워크 + driver 컨트랙트 계정(선택적으로 베스팅) + 거래 목록"이다. 거래 하나에는 precompile 호출 단계가 여러 개 들어가고 각 단계에는 호출 문맥이 있다.

| 호출 문맥 (`mode`) | 의미 |
|---|---|
| `direct` | driver가 precompile을 직접 호출 |
| `try_revert` | 호출한 뒤 되돌려지는 프레임 안에서 호출 (`try/catch`) |
| `low_gas` | gas를 제한해서 호출 |
| `nested` | 한 단계 깊은 프레임에서 호출 |

거래 옵션 `revertAtEnd`를 켜면 모든 단계를 마친 뒤 거래 전체를 되돌린다. 형식은 `generator/scenario.go`에 둘 예정이다.

**전략**

| 이름 | 쓰는 정보 | 쓰지 않는 정보 |
|---|---|---|
| `random` (8단계, 기준선) | ABI 타입, 시나리오 문법(계정 유형·호출 문맥) | 소스 분석, 어떤 메서드가 상태를 바꾸는지, 어떤 값이 경계값인지 |

`random`은 메서드를 균등하게 고른다(인자가 단순 타입인 것만, 조회 메서드 포함). 정수 인자는 비트 길이를 균등하게 뽑는다(log-uniform). 주소는 시나리오 계정, 검증인, 무작위 주소 중에서, 문자열은 검증인 주소와 무작위 문자열 중에서 고른다. driver가 베스팅 계정일 확률은 0.5다. 같은 seed는 항상 같은 시나리오를 만든다.

```bash
cd tool && go run ./generator/cmd/atomicos-gen -strategy random -n 100 -seed 1 \
  -abi staking=<staking abi.json> -abi distribution=<distribution abi.json> -out <dir>
```

## runner: 시나리오 실행기 (8단계, `tool/harness/runner`)

> **8단계 설계다. `tool/harness/runner`, `run_scenarios.sh`, `summarize.py`는 아직 저장소에 없다.** 아래는 만들 내용이다.

| 파일 | 내용 |
|---|---|
| `runner/contracts/AtomicosDriver.sol` | 호출을 문맥별로 전달만 하는 컨트랙트(공격 코드 없음). `gen.sh`가 `v07/driver_contract.go`로 만든다 |
| `runner/v07/runner_test.go` | 시나리오마다 새 네트워크를 만들고 계정을 준비한다. 시나리오 거래만 기록하고 `<id>.result.json`을 쓴다 |
| `runner/testdata/` | 러너 점검용 시나리오 3개(생성기 출력 아님) |
| `run_scenarios.sh` | 하네스·러너 주입 → 실행 → 시나리오별 판정 → `summary.txt`/`summary.tsv` |
| `summarize.py` | 집계(위반, 거래 성공 수, precompile 메서드별 호출 수와 성공 수) |

```bash
tool/harness/run_scenarios.sh v07 .work/ghsa-7g4w-vuln <scenario dir> <out dir>
```

러너는 v0.7.x 계열부터 만들 계획이다. v0.5와 v0.1 계열은 테스트 유틸 API가 달라서 따로 옮겨야 한다.
