---
source: docs/cli.md
source_hash: 09cd57c696031bee8af2d0dfe5aba4b249fc8aa4
---
# CLI 레퍼런스

> 명령별 전체 플래그와 인자 레퍼런스는 [CLI 레퍼런스](cli/deck.md)에 자동 생성됩니다. 이 페이지는 개요와 공통 규칙만 다룹니다. 작업 중심 안내는 [가이드](guides/authoring-workflows.md)를 참고하십시오.

`deck` CLI는 의도적으로 작게 설계되었습니다. 워크플로를 작성하고, 린트하고, 번들 콘텐츠를 준비하고, 번들을 빌드하고, 로컬에서 실행하는 단순한 운영자 흐름을 지원합니다.

## 개요

### 기본 로컬 흐름

- `init`: `workflows/` 아래에 시작용 워크플로 파일을 생성합니다
- `lint`: 워크플로 파일이나 워크스페이스를 워크플로 및 스텝 스키마와 대조해 검증합니다(`-o text|json`)
- `prepare`: 아티팩트를 `outputs/`로 모으고, 로컬 `deck` 런처를 작성하며, `.deck/manifest.json`을 생성합니다
- `bundle build`: 현재 워크스페이스를 옮길 수 있는 아카이브로 패키징합니다
- `apply`: 적용 시나리오를 로컬에서 실행합니다

### 추가 헬퍼

- `plan`: 실행에 앞서 어떤 적용 스텝이 실행되고 어떤 스텝이 건너뛰어지는지 미리 확인합니다(`-o text|json`)
- `list`: 로컬 워크스페이스나 저장된 원격 서버에서 사용 가능한 시나리오를 나열합니다
- `cache list` / `cache clean`: 캐시된 아티팩트 항목을 확인하거나 정리합니다
- `server remote set/show/unset`: 기본 원격 서버 URL을 관리합니다
- `server up` / `server down`: 준비된 번들 루트를 HTTP로 노출하거나, 데몬화된 서버를 중지합니다
- `server health`: 명시한 서버나 저장된 원격 서버 URL의 `/healthz`를 확인합니다(`-o text|json`)
- `server logs`: 파일이나 저널에서 로컬 서버 감사 로그를 읽습니다
- `state show/list/clear`: 저장된 적용 상태를 확인하고 제거합니다
- `version`: 현재 `deck` 빌드 버전과 메타데이터를 표시합니다(`-o text|json`)
- `completion`: bash, zsh, fish, PowerShell용 셸 자동완성을 생성합니다

### 작성 헬퍼

- `ask`: LLM 기반 작성 도우미로 현재 워크스페이스의 워크플로를 질문, 설명, 검토, 초안 작성, 다듬기 할 수 있는 실험적 헬퍼입니다

`ask`는 실험적 기능이며 표준 `deck` 바이너리에 포함되어 제공됩니다.

`deck ask`를 설정하고 사용하는 작업 중심 가이드는 [deck ask 사용하기](ask.md)를 참고하십시오.

`ask`는 생성에 앞서 요청을 라우팅합니다. `--create`, `--edit`, `--review` 같은 명시적인 작성 및 검토 플래그는 강제 오버라이드로 동작합니다. 그 밖의 요청은 LLM 기반 라우트 분류를 거치며, 모호한 요청은 곧장 생성으로 넘어가지 않고 명확화를 위해 멈출 수 있습니다.

모델에 접근할 수 없을 때 `ask`는 충분히 추론한 척 조용히 넘어가지 않고 동작을 명시적으로 낮춥니다. `explain`은 대상 파일의 로컬 구조 요약으로 폴백하고, `review`는 로컬 분석 결과로 폴백하며, 생성 라우트는 로컬 검증이 모델 출력을 대신할 수 없으므로 곧바로 실패합니다.

OpenAI 호환 프로바이더 지원은 현재 `openai`, `openrouter`, `gemini`를 대상으로 합니다.

명령별 `ask` 플래그와 하위 명령은 [deck ask](cli/deck_ask.md)를 참고하십시오.

## 출력 형식

`deck lint -o json`은 검증된 워크플로 목록, 요약 카운트, 지원되는 워크플로 컨트랙트, 그리고 불투명한 `Command` 스텝이나 무결성 검사가 없는 원격 아티팩트 같은 경고 수준 `findings`를 담은 구조화된 보고서를 반환합니다.

`deck plan -o json`은 해석된 워크플로 경로, 상태 경로, 런타임 var 키, 스텝별 동작, 요약 섹션을 반환합니다.

`deck plan vars -o json`은 실행 입력 스냅샷을 반환합니다. 여기에는 유효한 `vars`, 해석된 `context`, 실행 전에 알려진 초기 `runtime` 값, 이후 스텝이 register할 수 있는 계획된 `runtime` 키가 포함됩니다.

`deck server health -o json`은 해석된 서버 URL, `/healthz` URL, HTTP 상태를 반환합니다.

`deck bundle verify -o json`은 검증된 번들 경로와 최종 상태를 반환합니다.

`deck cache list -o json`과 `deck server logs -o json`은 기계 판독용 출력을 stdout으로 유지하며, `--v=<n>`은 경로 및 카운트 진단을 stderr로 보냅니다.

## 상세도(`--v`) {#verbosity---v}

전역 `--v=<n>`은 stdout 결과 컨트랙트를 바꾸지 않은 채 진단만 stderr에 기록합니다. 지원 수준은 0–3입니다:

- `--v=0`: 결과 중심 출력. 오래 실행되는 `apply` 및 `prepare` 실행은 이때도 단계와 스텝 진행 상황을 stderr에 표시합니다
- `--v=1`: 워크플로/소스/경로 결정, 진행 이벤트, 상위 수준 실행 컨텍스트
- `--v=2`: 적용 실행 계획/상태/스텝 메타데이터, ask 디버그 로그, 계획 평가 세부 정보, 더 깊은 번들/prepare/health 검사 카운트
- `--v=3`: apply/prepare/bundle/list/server/cache의 키 수준 추적, ask 추적 아티팩트, 컨트랙트 노트, lint 분석 힌트, 가장 상세한 plan/lint 추적

실제 사용 예:

- `deck plan --v=3`은 워크플로/런타임 var 추적과 스텝별 평가 세부 정보를 더해 보여줍니다
- `deck apply --v=2`는 실행 계획, 상태 스냅샷, 단계/배치 계획, 스텝별 메타데이터를 더해 보여줍니다
- `deck apply --v=3`은 spec 값은 기록하지 않으면서 워크플로 해시/상태 키, 컨텍스트 키, 워크플로 var 키, 런타임 상태 키, 스텝 컨트랙트 키를 더해 보여줍니다
- `deck prepare --v=2`는 아티팩트 그룹과 캐시 재사용/페치 진단을 더해 보여줍니다
- `deck prepare --v=3`은 spec 값은 기록하지 않으면서 워크플로, 단계, 스텝, 런타임 바이너리 키 추적을 더해 보여줍니다
- `deck bundle build --v=3`과 `deck bundle verify --v=3`은 매니페스트 항목별 경로/카테고리/크기/해시 접두사 추적을 더해 보여줍니다
- `deck list --v=3`, `deck cache ... --v=3`, `deck server ... --v=3`은 해당하는 경우 항목별 추적이나 요청/응답/윈도 추적을 더해 보여줍니다

특히 `deck ask`의 경우:

- `--v=0`: ask 진단 없음
- `--v=1`: stderr에 라우트, 프로바이더, 진행 요약
- `--v=2`: 라우트/프로바이더 요약과 함께 사용자 명령 및 MCP 이벤트
- `--v=3`: 디버그 로그와 분류기/라우트 시스템 프롬프트 및 사용자 프롬프트. 아울러 `.deck/ask/runs/<run-id>/` 아래에 프롬프트와 응답 페이로드 아티팩트를 작성합니다

## 로그 형식(`--log-format`)

전역 `--log-format=text|json`은 마이그레이션된 진단 로그를 stderr에 렌더링하는 방식을 제어합니다.

- `--log-format=text`: 사람이 읽기 위한 한 줄짜리 구조화 텍스트 로그
- `--log-format=json`: 동일한 이벤트 스키마를 기계 처리용으로 담은 stderr의 JSON Lines

현재 마이그레이션된 명령군은 `ask`, `prepare`, `apply`, `server`, `list`, `cache`입니다.

텍스트 진단은 `phase`, `step`, `status`, `reason`, `kind`, `duration_ms`, `batch`, `invocation_id`, 경로/위치 필드처럼 신호가 강한 필드를 낮은 우선순위 속성보다 앞에 둡니다. JSON 진단은 기계 처리를 위해 전체 이벤트 필드를 유지합니다.

`apply` 및 `prepare` 진행 로그의 경우 텍스트 출력은 상세도에 따라 필드를 확장합니다. `--v=0`은 핵심 진행 필드를 표시하고, `--v=1`은 kind/duration/실패 세부 정보를 더하며, `--v>=2`는 batch, parallelism, attempt, invocation 상관 필드를 포함합니다. JSON 진단은 항상 전체 이벤트 필드를 유지합니다.

신규 진단의 이벤트 명명 규칙:

- `*_requested`: 명령 의도 수신
- `*_selected` 또는 `*_resolved`: 입력 경로, 소스, 설정 해석
- `*_planned`: 작업 계획 계산
- `*_started`, `*_succeeded`, `*_failed`: 단위 수준 실행 진행
- `*_completed`: `status`와 `duration_ms`를 담은 명령 수준 완료 요약
- `*_summary`: 집계 카운트
- `*_trace`: `--v=3`을 위한 키 수준 세부 정보

신규 필드는 기간에 `duration_ms`, 바이트 크기에 `*_bytes`, 새 카운트 필드에 `*_count`, 존재 여부 검사에 `has_*` 불리언을 사용합니다. URL 진단은 userinfo, 쿼리 값, 프래그먼트를 가립니다.

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

## 워크플로 소스 로케이터 {#workflow-source-locators}

`plan`, `apply`, `state`의 경우 워크플로 소스와 진입점을 따로 선택하는 방식을 권장합니다:

```bash
deck apply --root ./demo --scenario apply
deck plan --server https://server --scenario apply
deck state show --server https://server --scenario apply
```

- `--root <path>`는 `workflows/`를 포함하는 로컬 워크플로 트리나 번들 루트를 선택합니다.
- `--server <url>`는 원격 워크플로 서버를 선택하고 `<url>/workflows/scenarios/` 아래에서 시나리오를 해석합니다.
- `--scenario <name>`은 선택된 소스 아래의 `workflows/scenarios/<name>.yaml`을 선택합니다.
- `--workflow <path-or-url>`은 명시적인 워크플로 파일 탈출구로 계속 사용할 수 있습니다.
- `--root`와 `--server`는 함께 쓸 수 없습니다.
- `--workflow`와 `--scenario`는 함께 쓸 수 없습니다.
- 기존 `--source server --scenario <name>`도 `DECK_SERVER`나 `deck server remote set`과 함께 여전히 동작하지만, `--server <url> --scenario <name>`이 더 명확한 인라인 형식입니다.

로컬 워크플로의 적용 상태는 `./.deck/state/apply/` 아래에 보관되고, 원격 워크플로의 적용 상태는 사용자 로컬 XDG 상태 루트 아래의 `deck/state/apply/`를 사용합니다. 워크스페이스 로컬 메타데이터는 `./.deck/` 아래에 보관되며, 사용자 전역 설정, 원격 워크플로 상태, 캐시, 실행 이력은 표준 XDG 위치를 사용합니다.

## 변수 오버라이드

`lint`, `prepare`, `plan`, `apply`는 여러 번 지정할 수 있는 `-f, --vars-file` YAML 오버레이를 지원합니다. `prepare`, `plan`, `apply`는 한 번의 호출에서 여러 번 지정할 수 있는 `--var key=value` 오버라이드도 지원합니다.

- vars 파일은 제공된 순서대로 `workflows/vars.yaml` 위에 병합됩니다.
- 노드 범위의 `all:`과 `hosts:` 값은 vars 파일 오버레이를 병합한 뒤에 선택됩니다.
- 워크플로 `vars:`는 노드 범위 선택 이후에 적용됩니다.
- `--var` 오버라이드는 가장 마지막에 적용되며 우선순위가 가장 높습니다.

vars 파일 경로는 선택된 워크플로 루트를 기준으로 합니다. 예를 들어 `--root ./demo -f vars/site.yaml`은 `./demo/workflows/vars/site.yaml`을 읽습니다.

## 런타임 바이너리 선택

준비 시 `--bundle-binary-source=auto`(기본값)는 `release`로 해석됩니다. 개발 빌드에서는 `--bundle-binary-dir`로 로컬 바이너리를 선택하지 않는 한 최신 GitHub Release를 가져옵니다. `--bundle-binary-dir` 없이 `--bundle-binary-source=local`을 쓰면 현재 호스트 튜플에 대해 현재 실행 파일을 사용합니다. 선택된 바이너리는 `outputs/bin/`으로 원자적으로 게시됩니다.

## 셸 자동완성

`deck completion`이 유일한 자동완성 진입점입니다. 지원 셸은 `bash`, `zsh`, `fish`, `powershell`입니다.

현재 셸 세션에서 자동완성을 활성화하려면:

```bash
source <(deck completion bash)
source <(deck completion zsh)
deck completion fish | source
```

영구적으로 등록하려면 셸 초기화 파일(예: `~/.bashrc`, `~/.zshrc`)에 sourcing 명령을 추가하십시오. PowerShell의 경우 `$PROFILE`에 `deck completion powershell | Out-String | Invoke-Expression`을 추가하십시오.

## 공통 예시

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

- `deck ask`는 작성 라우트에서 워크플로 파일을 직접 작성합니다. 작성 의도를 명확히 하려면 `--create`나 `--edit`를 사용하십시오.
- `deck ask plan`은 재사용 가능한 계획 아티팩트를 `./.deck/plan/` 아래에 저장합니다.
<!-- END GENERATED:ASK_CLI_CONTEXT -->

<!-- BEGIN GENERATED:ASK_AUTHORING_CONTEXT -->
## Ask 작성 컨텍스트

- deck 워크플로를 위한 최상위 워크플로 작성 레퍼런스입니다.
- 임포트는 phases[].imports 아래에서만 유효하며, 컴포넌트 상대 경로를 사용해 workflows/components/에서 해석됩니다.
- 여러 스텝이나 파일에 인라인으로 반복될 만한 구성 가능한 값은 workflows/vars.yaml에 두는 것을 권장합니다.
- 먼저 타입이 지정된 스텝 그룹부터 시작하십시오. 해당하는 타입 스텝이 있으면 `Command`보다 타입 스텝을 우선하십시오.
<!-- END GENERATED:ASK_AUTHORING_CONTEXT -->
