---
source: docs/cli.md
source_hash: ae57dbf4d976c526653ef8a9dcc31b96f7fc9a66
---
# CLI 레퍼런스

> 명령별 전체 플래그 및 인자 레퍼런스는 [CLI 레퍼런스](cli/deck.md)에 자동 생성되어 있습니다. 이 페이지는 개요와 공통 규약만 다룹니다.

`deck` CLI는 의도적으로 작게 설계되었습니다.

이 CLI는 간단한 운영자 흐름을 지원합니다: 워크플로 작성, 린트, 번들 콘텐츠 준비, 번들 빌드, 로컬 실행.

## 기본 로컬 흐름

- `init`: `workflows/` 아래에 시작용 워크플로 파일 생성
- `lint`: 워크플로 파일 또는 워크스페이스를 워크플로 및 스텝 스키마에 대해 검증 (`-o text|json`)
- `prepare`: 아티팩트를 `outputs/`로 수집하고, 로컬 `deck` 런처를 작성하며, `.deck/manifest.json`을 작성
- `bundle build`: 현재 워크스페이스를 이동 가능한 아카이브로 패키징
- `apply`: `apply` 워크플로를 로컬에서 실행

## 추가 헬퍼

- `plan`: 실행 전에 어떤 apply 스텝이 실행되거나 건너뛰어지는지 확인 (`-o text|json`)
- `list`: 로컬 워크스페이스 또는 저장된 원격 서버에서 사용 가능한 시나리오 나열
- `cache list` / `cache clean`: 캐시된 아티팩트 항목 확인 또는 정리
- `server remote set/show/unset`: 기본 원격 서버 URL 관리
- `server up` / `server down`: 준비된 번들 루트를 HTTP로 노출하거나, 데몬화된 서버 중지
- `server health`: 명시적 서버 또는 저장된 원격 서버 URL에서 `/healthz` 확인 (`-o text|json`)
- `server logs`: 로컬 서버 감사 로그를 파일 또는 저널에서 읽기
- `state show/list/clear`: 저장된 apply 상태 확인 및 제거
- `version`: 현재 `deck` 빌드 버전 및 메타데이터 표시 (`-o text|json`)
- `completion`: bash, zsh, fish, PowerShell용 셸 자동 완성 생성

## 작성 헬퍼

- `ask`: LLM 기반 작성 어시스턴트를 사용하여 현재 워크스페이스의 워크플로를 질문, 설명, 리뷰, 초안 작성, 다듬기 하는 실험적 헬퍼

`ask`는 실험적 기능이며 표준 `deck` 바이너리의 일부로 제공됩니다.

`deck ask`의 구성 및 사용에 대한 작업 지향 가이드는 [deck ask 사용하기](ask.md)를 참고하세요.

`ask`는 생성 전에 요청을 라우팅합니다. `--create`, `--edit`, `--review` 같은 명시적 작성 및 리뷰 플래그는 강제 오버라이드로 동작합니다. 다른 요청은 LLM 보조 라우트 분류를 거치며, 모호한 요청은 생성으로 흘러가는 대신 명확화를 위해 멈출 수 있습니다.

모델 접근이 불가능할 때, `ask`는 완전한 추론으로 답하는 척 조용히 가장하지 않고 명시적으로 기능을 낮춥니다. `explain`은 대상 파일의 로컬 구조 요약으로 폴백하고, `review`는 로컬 발견 사항으로 폴백하며, 생성 라우트는 로컬 검증이 모델 출력을 대체할 수 없으므로 빠르게 실패합니다.

OpenAI 호환 프로바이더 지원은 현재 다음을 대상으로 합니다: `openai`, `openrouter`, `gemini`.

명령별 `ask` 플래그 및 서브커맨드는 [deck ask](cli/deck_ask.md)를 참고하세요.

## 출력 형식

`deck lint -o json`은 검증된 워크플로 목록, 요약 카운트, 지원되는 워크플로 컨트랙트, 그리고 불투명한 `Command` 스텝이나 무결성 검사가 없는 원격 아티팩트 같은 경고 수준의 `findings`를 담은 구조화된 리포트를 반환합니다.

`deck plan -o json`은 해석된 워크플로 경로, 상태 경로, 런타임 var 키, 스텝별 동작, 그리고 요약 섹션을 반환합니다.

`deck plan vars -o json`은 실행 입력 스냅샷을 반환합니다: 유효 `vars`, 해석된 `context`, 실행 전에 알려진 초기 `runtime` 값, 그리고 이후 스텝이 등록할 수 있는 계획된 `runtime` 키.

`deck server health -o json`은 해석된 서버 URL, `/healthz` URL, HTTP 상태를 반환합니다.

`deck bundle verify -o json`은 검증된 번들 경로와 최종 상태를 반환합니다.

`deck cache list -o json`과 `deck server logs -o json`은 기계 판독 가능한 출력을 stdout에 유지하면서, `--v=<n>`은 경로 및 카운트 진단을 stderr로 보냅니다.

## 상세도 (`--v`)

전역 `--v=<n>`은 stdout 결과 컨트랙트를 변경하지 않고 진단을 stderr에 작성합니다. 지원 수준은 0–3입니다:

- `--v=0`: 결과 중심 출력; 장시간 실행되는 `apply` 및 `prepare` 실행은 여전히 단계 및 스텝 진행 상황을 stderr에 표시합니다
- `--v=1`: 워크플로/소스/경로 결정, 진행 이벤트, 상위 수준 실행 컨텍스트
- `--v=2`: apply 실행 계획/상태/스텝 메타데이터, ask 디버그 로그, plan 평가 세부 정보, 더 깊은 번들/prepare/health 검사 카운트
- `--v=3`: apply/prepare/bundle/list/server/cache의 키 수준 트레이스, ask 트레이스 아티팩트, 컨트랙트 노트, lint 발견 힌트, 그리고 가장 상세한 plan/lint 트레이스

실제 사용 예:

- `deck plan --v=3`은 워크플로/런타임 var 트레이스와 스텝별 평가 세부 정보를 추가합니다
- `deck apply --v=2`는 실행 계획, 상태 스냅샷, 단계/배치 계획, 스텝별 메타데이터를 추가합니다
- `deck apply --v=3`은 spec 값을 로깅하지 않고 워크플로 해시/상태 키, 컨텍스트 키, 워크플로 var 키, 런타임 상태 키, 스텝 컨트랙트 키를 추가합니다
- `deck prepare --v=2`는 아티팩트 그룹 및 캐시 재사용/페치 진단을 추가합니다
- `deck prepare --v=3`은 spec 값을 로깅하지 않고 워크플로, 단계, 스텝, 런타임 바이너리 키 트레이스를 추가합니다
- `deck bundle build --v=3` 및 `deck bundle verify --v=3`은 매니페스트 항목별 경로/카테고리/크기/해시 접두사 트레이스를 추가합니다
- `deck list --v=3`, `deck cache ... --v=3`, `deck server ... --v=3`은 해당하는 경우 항목별 또는 요청/응답/윈도우 트레이스를 추가합니다

`deck ask`에 한해서는:

- `--v=0`: ask 진단 없음
- `--v=1`: stderr에 라우트, 프로바이더, 진행 요약
- `--v=2`: 라우트/프로바이더 요약과 사용자 명령 및 MCP 이벤트
- `--v=3`: 디버그 로그와 분류기/라우트 시스템 프롬프트 및 사용자 프롬프트; 또한 `.deck/ask/runs/<run-id>/` 아래에 프롬프트 및 응답 페이로드 아티팩트 작성

## 로그 형식 (`--log-format`)

전역 `--log-format=text|json`은 마이그레이션된 진단 로그가 stderr에 렌더링되는 방식을 제어합니다.

- `--log-format=text`: 사람을 위한 한 줄 구조화 텍스트 로그
- `--log-format=json`: 기계 처리를 위해 동일한 이벤트 스키마를 가진 stderr상의 JSON Lines

현재 마이그레이션된 명령 패밀리에는 `ask`, `prepare`, `apply`, `server`, `list`, `cache`가 포함됩니다.

텍스트 진단은 `phase`, `step`, `status`, `reason`, `kind`, `duration_ms`, `batch`, `invocation_id` 및 경로/위치 필드 같은 고신호 필드를 우선순위가 낮은 속성보다 앞에 둡니다. JSON 진단은 기계 처리를 위해 전체 이벤트 필드를 유지합니다.

`apply` 및 `prepare` 진행 로그의 경우, 텍스트 출력은 상세도에 따라 필드를 확장합니다: `--v=0`은 핵심 진행 필드를 표시하고, `--v=1`은 kind/duration/failure 세부 정보를 추가하며, `--v>=2`는 batch, parallelism, attempt, invocation 상관 필드를 포함합니다. JSON 진단은 항상 전체 이벤트 필드를 유지합니다.

새 진단의 이벤트 명명 규칙:

- `*_requested`: 명령 의도 수신
- `*_selected` 또는 `*_resolved`: 입력 경로, 소스, 또는 구성 해석
- `*_planned`: 작업 계획 계산됨
- `*_started`, `*_succeeded`, `*_failed`: 단위 수준 실행 진행
- `*_completed`: `status` 및 `duration_ms`를 포함한 명령 수준 완료 요약
- `*_summary`: 집계 카운트
- `*_trace`: `--v=3`을 위한 키 수준 세부 정보

새 필드는 기간에 `duration_ms`, 바이트 크기에 `*_bytes`, 새 카운트 필드에 `*_count`, 존재 여부 확인에 `has_*` 불리언을 사용합니다. URL 진단은 userinfo, 쿼리 값, 프래그먼트를 가립니다.

텍스트 진단 예시:

```text
ts=2026-04-02T09:20:00Z level=info component=prepare event=batch_started phase=prepare batch=prepare:downloads parallel_group=downloads batch_size=2 max_parallelism=2 status=started
ts=2026-04-02T09:20:00Z level=info component=prepare event=step_started phase=prepare batch=prepare:downloads step=download-runc kind=DownloadFile attempt=1 status=started
```

JSON 진단 예시:

```json
{"ts":"2026-04-02T09:20:00Z","level":"info","component":"prepare","event":"batch_started","phase":"prepare","batch":"prepare:downloads","parallel_group":"downloads","batch_size":2,"max_parallelism":2,"status":"started"}
{"ts":"2026-04-02T09:20:00Z","level":"info","component":"prepare","event":"step_started","phase":"prepare","batch":"prepare:downloads","step":"download-runc","kind":"DownloadFile","attempt":1,"status":"started"}
```

## 워크플로 소스 로케이터

`plan`, `apply`, `state`의 경우, 워크플로 소스와 진입점을 별도로 선택하는 것을 권장합니다:

```bash
deck apply --root ./demo --scenario apply
deck plan --server https://server --scenario apply
deck state show --server https://server --scenario apply
```

- `--root <path>`는 `workflows/`를 포함하는 로컬 워크플로 트리 또는 번들 루트를 선택합니다.
- `--server <url>`는 원격 워크플로 서버를 선택하고 `<url>/workflows/scenarios/` 아래의 시나리오를 해석합니다.
- `--scenario <name>`은 선택된 소스 아래의 `workflows/scenarios/<name>.yaml`을 선택합니다.
- `--workflow <path-or-url>`은 명시적 워크플로 파일 탈출구로 계속 사용 가능합니다.
- `--root`와 `--server`는 상호 배타적입니다.
- `--workflow`와 `--scenario`는 상호 배타적입니다.
- 기존 `--source server --scenario <name>`은 `DECK_SERVER` 또는 `deck server remote set`과 함께 여전히 동작하지만, `--server <url> --scenario <name>`이 더 명확한 인라인 형태입니다.

로컬 워크플로 apply 상태는 `./.deck/state/apply/` 아래에 유지됩니다. 원격 워크플로 apply 상태는 `deck/state/apply/` 아래의 사용자 로컬 XDG 상태 루트를 사용합니다. 워크스페이스 로컬 메타데이터는 `./.deck/` 아래에 유지되며, 사용자 전역 구성, 원격 워크플로 상태, 캐시, 실행 이력은 표준 XDG 위치를 사용합니다.

## 런타임 바이너리 선택

준비할 때, `--bundle-binary-source=auto`(기본값)는 `release`로 해석됩니다. dev 빌드에서는 `--bundle-binary-dir`이 로컬 바이너리를 선택하지 않는 한 최신 GitHub Release를 페치합니다. `--bundle-binary-dir` 없는 `--bundle-binary-source=local`은 현재 호스트 튜플에 대해 현재 실행 파일을 사용합니다. 선택된 바이너리는 `outputs/bin/`에 원자적으로 게시됩니다.

## 변수 오버라이드

`lint`, `prepare`, `plan`, `apply`는 반복 가능한 `-f, --vars-file` YAML 오버레이를 지원합니다. `prepare`, `plan`, `apply`는 또한 단일 호출에 대해 반복 가능한 `--var key=value` 오버라이드를 지원합니다.

- vars 파일은 제공된 순서대로 `workflows/vars.yaml` 위에 병합됩니다.
- 노드 스코프 `all:` 및 `hosts:` 값은 vars 파일 오버레이가 병합된 후에 선택됩니다.
- 워크플로 `vars:`는 노드 스코프 선택 이후에 적용됩니다.
- `--var` 오버라이드는 마지막에 적용되며 가장 높은 우선순위를 가집니다.

vars 파일 경로는 선택된 워크플로 루트를 기준으로 합니다. 예를 들어, `--root ./demo -f vars/site.yaml`은 `./demo/workflows/vars/site.yaml`을 읽습니다.

## 셸 자동 완성

`deck completion`이 유일한 자동 완성 진입점입니다. 지원 셸: `bash`, `zsh`, `fish`, `powershell`.

현재 셸 세션에 대해 자동 완성을 활성화하려면:

```bash
source <(deck completion bash)
source <(deck completion zsh)
deck completion fish | source
```

영구 등록을 위해서는, 소싱 명령을 셸의 초기화 파일(예: `~/.bashrc`, `~/.zshrc`)에 추가하세요. PowerShell의 경우, `deck completion powershell | Out-String | Invoke-Expression`을 `$PROFILE`에 추가하세요.

## 일반 예시

```bash
deck init --out ./demo
deck version
deck version -o json
deck list --source local
deck lint --workflow ./demo/workflows/scenarios/apply.yaml
deck lint --workflow ./demo/workflows/scenarios/apply.yaml -o json

cd ./demo
deck prepare
deck prepare -f vars/site.yaml --var registryHost=mirror.local --var kubernetesVersion=v1.30.1
deck bundle build --out ./bundle.tar
deck plan --root . --scenario apply
deck plan --root . --scenario apply -o json
deck apply --root . --scenario apply
deck apply --root . --scenario apply -f vars/cp1.yaml --var role=control-plane --var nodeIP=10.0.0.10
deck cache clean --older-than 30d --dry-run
```

선택적 사이트 로컬 서버 예시:

```bash
deck server remote set http://127.0.0.1:8080
deck server up --root ./bundle --addr :8080
deck server up --root ./bundle --addr :8443 --tls-self-signed
deck server health --server http://127.0.0.1:8080 -o json
deck plan --server http://127.0.0.1:8080 --scenario apply
deck bundle verify --file ./bundle -o json
```

선택적 `ask` 예시:

```bash
deck ask config set --provider openai --model gpt-5.4 --api-key "$DECK_ASK_API_KEY"
deck ask "explain what workflows/scenarios/apply.yaml does"
deck ask --create "create an air-gapped rhel9 single-node kubeadm workflow"
deck ask --review
```

<!-- BEGIN GENERATED:ASK_CLI_CONTEXT -->
## Ask CLI 컨텍스트

- `deck ask`는 작성 라우트에 대해 워크플로 파일을 직접 작성합니다. 작성 의도를 명시하려면 `--create` 또는 `--edit`을 사용하세요.
- `deck ask plan`은 재사용 가능한 plan 아티팩트를 `./.deck/plan/` 아래에 저장합니다.
<!-- END GENERATED:ASK_CLI_CONTEXT -->

<!-- BEGIN GENERATED:ASK_AUTHORING_CONTEXT -->
## Ask 작성 컨텍스트

- deck 워크플로를 위한 최상위 워크플로 작성 레퍼런스입니다.
- import는 phases[].imports 아래에서만 유효하며 컴포넌트 상대 경로를 사용하여 workflows/components/에서 해석됩니다.
- 스텝이나 파일 전반에 인라인으로 반복될 만한 구성 가능한 값에는 workflows/vars.yaml을 선호하세요.
- 타입이 지정된 스텝 그룹부터 시작하세요. 타입이 지정된 스텝이 존재할 경우 `Command`보다 그것을 선호하세요.
<!-- END GENERATED:ASK_AUTHORING_CONTEXT -->
