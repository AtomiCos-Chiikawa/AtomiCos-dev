# AtomiCos

Cosmos EVM precompile이 EVM 상태와 Cosmos SDK 모듈 상태를 함께 바꿀 때 생기는 상태 전이 오류를 자동으로 찾아내고 재현하는 감사 도구.

> 2026-2학기 블록체인특성화대학 PBL · 팀 취익카와

## 현재 단계

| 단계 | 산출물 | 상태 |
|---|---|---|
| 1. 후보 사례 확정 | [`docs/answer-key.md`](docs/answer-key.md) §1 | 완료 |
| 2. 정답지 표 | [`docs/answer-key.md`](docs/answer-key.md) §2 | 완료 |
| 3. 사례별 코드 분석 | `cases/<id>/README.md` | 완료 |
| 4. 순수 재현 | `cases/<id>/run.sh`, `cases/<id>/results/` | 완료. 세 사례 모두 취약 FAIL / 수정 PASS (2026-11-19) |
| 5. 설계 근거 | [`docs/rules.md`](docs/rules.md), [`docs/design-rationale.md`](docs/design-rationale.md) | 완료. 초안: 규칙 R1-S/E, R2-A/W, R3-D/U/C, 보조 A1/A2, 관찰 지점 O1~O6 |
| 6. 판정기 | [`tool/`](tool/README.md): `checker`(판정), `harness`(관찰 훅, 버전 계열 3개) | 완료. 단위 테스트 통과. 관찰 지점 O1, O3과 보조 검사 A2는 미구현이다. v0.1.x 계열(mjfq) 하네스에는 O2가 없어서 mjfq에는 R2-W와 R3를 적용하지 않았다(구현 상태 표: [`docs/rules.md`](docs/rules.md) §1) |
| 7. 재현 사례로 동작 확인 | `cases/<id>/run.sh --judge` → `cases/<id>/results/judge/` | 완료. 세 사례 모두 취약 버전 위반 보고 / 수정 버전 위반 0건. 추가로 업스트림 정상 스위트에서 오탐 사전 측정을 했다([`eval/fp-upstream`](eval/fp-upstream/README.md)). 설계에 쓰지 않은 스펙 289개에서 0건이다. 단, 의미 있게 검사된 규칙은 R2(R2-A, R2-W)뿐이다. R1은 후보가 0회라 시험되지 않았고 R3에 해당하는 호출은 set B에 거의 없다. 수정된 업스트림 v0.7.4 스위트에서 잰 오탐 측정이지 검출 능력 측정이 아니다 |
| 8~11. 생성기, 소스 분석기, 평가 | `tool/generator`, `tool/analyzer`, `eval/` | 시작 전 |

## 사례

| 사례 | 규칙 | 재현 방식 | 4단계 재현 (취약 / 수정) | 7단계 판정기 (취약 / 수정) | 핵심 관찰 |
|---|---|---|---|---|---|
| [GHSA-mjfq](cases/ghsa-mjfq/README.md) | 1 원자성 (낮은 gas) | 수정 커밋의 회귀 테스트 | FAIL / PASS | 규칙 적중 6회 (R1-S ×2, R1-E ×4), 원인 1건 / 0 | 실패한 호출 후 검증인 한 명 몫의 보상이 잔고에 커밋됨. 금액은 실행마다 두 값 중 하나(커밋된 로그: +0.0105, 다른 실행: +0.0077) |
| [GHSA-8pfh](cases/ghsa-8pfh/README.md) (설계·동작 확인용) | 1 원자성 (try/catch 되돌림 + 이벤트) | 수정 커밋의 회귀 테스트 | FAIL / PASS | 규칙 적중 6회 (R1-E ×5, R2-W ×1), 원인 1건. 보조 A1 ×1 / 0 | 되돌린 보상 청구 이벤트가 4벌 커밋되어 잔고가 200이 됨 |
| [GHSA-7g4w](cases/ghsa-7g4w/README.md) | 2 잔고 전이 (베스팅 언더플로) | 직접 작성한 테스트 | FAIL / PASS | 규칙 적중 4회 (R2-A ×2, R2-W ×2), 원인 1건 / 0 (guarded R2-A ×2) | StateDB 잔고 1e18 → 2²⁵⁶−2e18. 거래 전후만 보면 두 버전이 똑같음 |

규칙 적중 수는 원인 1건을 서로 다른 규칙이 각자의 관점에서 잡은 횟수다. mjfq는 되돌려지지 않은 보상 송금 1번을 저장소 규칙 R1-S(잔고 키 2개)와 이벤트 규칙 R1-E(이벤트 종류 4개)가 함께 잡았다. 8pfh는 이벤트 규칙 R1-E가 되돌린 보상 청구의 이벤트 5종을, 되쓰기 규칙 R2-W가 그 결과인 잔고 불일치를 잡았다. 7g4w는 산술 규칙 R2-A가 잔고 연산에서, R2-W가 다음 동기화 지점에서 잡았다. eth_call 모의 실행과 실제 거래 양쪽에서 잡혀서 규칙마다 2회다.

수정 버전의 0건 중 mjfq와 8pfh는 같은 수정 버전 기록에서 나온 오탐(mjfq R1-S 2건, 8pfh R1-E 5건)을 보고 R1 판정 시점을 커밋 결과로 바꾼 뒤의 값이다(`docs/rules.md` R1, `docs/design-rationale.md` §5). 그래서 세 사례의 수정 버전 0건은 동작 확인이지 독립 근거가 아니다.

재현성: 처음 결과(커밋 e8d8e34)는 Linux amd64에서 만들었다. 2026-11-19에 macOS arm64에서 같은 코드로 독립적으로 다시 돌렸을 때 세 사례 모두 판정(취약 FAIL / 수정 PASS), 규칙 적중, trace 기록 수가 같았고, 커밋된 trace 6개를 다시 판정한 결과도 바이트 단위로 같았다. 그 뒤 판정기와 하네스를 보완했다(판정 불가 구분과 검사 통계, 관찰 읽기 격리, 판정 조건 강화). 지금 저장소의 결과 파일은 보완한 코드로 macOS arm64에서 다시 만든 것이다. 세 사례의 판정, REPRO 수치, 규칙 적중은 보완 전과 같다. 판정기 단위 테스트 43개가 모두 통과한다. 업스트림 오탐 측정은 두 세트 모두 위반 0건이다. mjfq의 지급 금액(위 표)과 업스트림 측정의 기록 수, 호출 수, 되돌림 수는 실행마다 조금씩 달라진다.

각 사례의 `results/summary.txt`에 실행 커밋, 명령, 판정, 관찰 값(`REPRO` 줄)이 있다. 결과 파일의 시각은 UTC(2026-11-18)이고 문서의 날짜는 한국 시간(2026-11-19)이다. 정답지는 [`docs/answer-key.md`](docs/answer-key.md).

## 디렉터리

```
atomicos/
├── README.md
├── docs/
│   ├── architecture.png       # 시스템 구성도 (예정: 아직 없음. 팀원 작성본을 여기에 둔다)
│   ├── answer-key.md          # 정답지 (1·2단계)
│   ├── rules.md               # 판정 규칙 명세 (5단계)
│   └── design-rationale.md    # 사례 × 규칙 대응표, 근거 (5단계)
├── cases/                     # 사례별 (3·4단계)
│   ├── lib.sh                 # 공통: 클론, worktree 준비, 파일 주입, 두 버전 실행, 판정 요약
│   ├── ghsa-mjfq/             # README.md(분석 노트), run.sh, results/
│   ├── ghsa-8pfh/             # README.md, run.sh, results/
│   └── ghsa-7g4w/             # README.md, run.sh, results/, test/(직접 작성한 테스트), contracts/
├── tool/                      # 6단계부터 (Go 모듈)
│   ├── harness/  checker/  generator/  analyzer/   # generator/는 예정(아직 없음), analyzer/는 예정(빈 폴더만 있음)
├── eval/fp-upstream/          # 오탐 사전 측정 (업스트림 정상 스위트)
└── .work/                     # cosmos/evm 클론과 worktree (git 무시)
```

## 실행

필요 환경:
- Go 1.21 이상 (각 커밋의 `go.mod`에 맞는 툴체인 1.23.6 / 1.23.8 / 1.25.9를 `GOTOOLCHAIN`이 자동으로 받는다)
- **C 컴파일러 (gcc, `build-essential`) 필수:** cgo 없이는 세 버전 모두 빌드되지 않는다.
- Linux와 macOS 기본 환경(BSD sed, bash 3.2)에서 모두 실행된다. 파일 치환은 `perl`로 한다.
- 네트워크, 디스크 수 GB, 메모리 8GB 정도. 처음 실행할 때는 빌드에 10분 이상 걸린다.

```bash
./cases/ghsa-mjfq/run.sh
```

```bash
./cases/ghsa-8pfh/run.sh
```

```bash
./cases/ghsa-7g4w/run.sh
```

판정기까지 붙여 실행(7단계):

```bash
./cases/ghsa-7g4w/run.sh --judge
```

각 스크립트는 다음 순서로 동작한다.
1. `cosmos/evm`을 `.work/evm`에 클론한다(처음 한 번).
2. 수정 직전 커밋(취약)과 수정 커밋을 각각 worktree로 깨끗하게 체크아웃한다.
3. 같은 테스트를 두 worktree에 넣고 같은 `go test`를 실행한다.
4. `results/summary.txt`, `results/{vuln,fixed}.log`를 남긴다. 저장소에 커밋된 `cases/<id>/results/`의 파일을 그 자리에서 덮어쓴다(`--judge`는 `results/judge/`를 지우고 다시 만든다).

판정은 세 가지다.
- PASS / FAIL: 테스트가 실제로 실행된 경우
- ERROR: 빌드나 다운로드 실패 등으로 테스트가 돌지 않은 경우. **ERROR는 기대한 FAIL로 세지 않는다.**

## 원칙
- 모든 실행은 cosmos/evm 통합 테스트 프레임워크의 프로세스 내 체인에서만 한다. 공개 네트워크로 거래를 보내지 않는다.
- 회귀 테스트를 가져올 때 assertion은 바꾸지 않는다. 비교하는 값을 남기는 `REPRO` 출력 줄만 추가한다(각 `run.sh`에 명시).
- GHSA-8pfh의 원인은 공식 미공개라 "수정 커밋 기반 추론"으로 표기한다. 주제 문서가 같은 이유로 8pfh를 평가 사례에서 뺐으므로 8pfh는 설계·동작 확인용 사례로 두고 11단계 평가에서는 제외한다. 재현 코드 공개 여부는 팀이 정한다.
