---
source: docs/troubleshooting.md
source_hash: 4223f04f91a7e3d16a14cfa1ab423be8963fc49b
---
# 문제 해결

이 가이드는 에러 코드 카탈로그를 증상 중심으로 보완하는 문서입니다. 가장 흔한 실패 모드를 짚어 가며 각각이 무엇을 의미하는지 설명하고 해결 방법을 보여줍니다. 안정적인 에러 코드 전체 목록은 [diagnostics/error-codes.md](diagnostics/error-codes.md)를 참고하세요.

## deck 에러를 읽는 법

deck은 에러를 `CODE: message` 형식으로 렌더링합니다. 예를 들면 다음과 같습니다:

```
E_BUNDLE_INTEGRITY: artifact outputs/files/kubeadm.conf: sha256 mismatch (expected a3f1…, got 9b2d…)
```

`CODE` 부분은 안정적이고 기계가 읽을 수 있는 식별자입니다. `message` 부분에는 해당 실패에 대한 구체적인 맥락이 담겨 있습니다.

### 더 자세한 정보 얻기

`--v=<n>`을 사용하면 stdout으로 가는 출력은 바꾸지 않으면서 stderr의 진단 상세도를 높일 수 있습니다:

| 레벨 | 얻는 정보 |
|-------|-------------|
| `--v=0` | 결과 중심 출력. `apply`와 `prepare`는 여전히 단계 및 스텝 진행 상황을 표시 |
| `--v=1` | 워크플로/소스/경로 결정, 진행 이벤트, 상위 수준 실행 맥락 |
| `--v=2` | 실행 계획, 상태 스냅샷, 단계/배치 계획, 스텝별 메타데이터 |
| `--v=3` | 워크플로 해시, 상태 키, 컨텍스트 키, 스텝 계약 키에 대한 키 수준 추적 |

`--log-format=json`을 사용하면 기계 처리를 위해 진단을 stderr에 JSON Lines로 내보냅니다:

```bash
deck apply --root . --scenario apply --v=2 --log-format=json 2>apply.log
```

이는 grep으로 검색하거나 `jq`로 파이핑할 때 유용합니다. JSON 이벤트 스키마는 텍스트 로그 이벤트와 필드 단위로 일치합니다.

`apply`와 `prepare`의 진행 상황에서 `--v=1`은 kind, 소요 시간, 실패 세부 정보를 추가하며, `--v=2`는 배치, 병렬 처리, 시도, 호출 상관관계 필드를 추가합니다.

전체 상세도 표는 [CLI 레퍼런스](cli.md#verbosity---v)를 참고하세요.

---

## `deck lint` 실패

`deck lint`는 워크플로가 패키징되거나 실행되기 전에 워크플로 YAML 구조와 각 스텝의 스키마를 검증합니다. 여기서 실수를 잡는 것이 에어갭(망분리) 내부에서 발견하는 것보다 비용이 적습니다.

### 스키마 에러 (`E_SCHEMA_INVALID`)

**증상:** `E_SCHEMA_INVALID: step "write-config": field "spec.path" is required`

**원인:** 스텝 필드에 잘못되었거나 더 이상 사용되지 않는 값이 있거나, 필수 필드가 누락되었습니다.

**해결:** 해당 스텝 종류의 스키마를 확인하세요. `deck lint -o json`을 실행하면 `findings` 목록이 포함된 구조화된 보고서를 얻을 수 있으며, 각 항목은 실패한 스텝, 필드, 제약 조건을 명시합니다. 각 종류에 허용되는 필드는 [step-kinds.md](step-kinds.md)를 참고하세요.

### 역할/모드 불일치 (`E_KIND_ROLE_MISMATCH`)

**증상:** `E_KIND_ROLE_MISMATCH: step "fetch-kubeadm": kind DownloadFile is not valid in apply role`

**원인:** 준비 단계에 속하는 스텝 종류(`DownloadFile`, `DownloadImage`, `DownloadPackage` 등)가 apply 시나리오에 나타났거나 그 반대의 경우입니다. 워크플로 역할은 파일 내 필드가 아니라 명령 맥락과 파일 위치로 결정됩니다.

**해결:** 스텝을 올바른 워크플로 파일로 옮기세요. 준비 전용 종류는 `workflows/prepare.yaml`에 들어갑니다. apply 종류는 `workflows/scenarios/<name>.yaml` 또는 거기서 가져온 컴포넌트 프래그먼트에 들어갑니다. [step-kinds.md](step-kinds.md)의 단계/그룹 색인을 참고하세요.

### 중복된 스텝 ID (`E_DUPLICATE_STEP_ID`)

**증상:** `E_DUPLICATE_STEP_ID: step id "install-packages" is used more than once`

**원인:** 둘 이상의 스텝이 같은 파일 안에서 또는 import가 확장된 후에 동일한 `id` 값을 공유합니다.

**해결:** 각 스텝에 고유한 `id`를 부여하세요. 컴포넌트 프래그먼트가 여러 단계로 가져와질 때, 각 프래그먼트는 결합된 워크플로 전체에서 고유한 ID를 사용해야 합니다. 충돌을 피하려면 `<component>-<action>` 같은 접두사 규칙을 사용하세요.

### 중복된 단계 이름 (`E_DUPLICATE_PHASE_NAME`)

**증상:** `E_DUPLICATE_PHASE_NAME: phase name "install" is used more than once`

**원인:** 같은 워크플로 내 두 단계가 동일한 `name`을 공유합니다(또는 둘 다 빈 이름을 가집니다).

**해결:** 각 단계에 고유하고 비어 있지 않은 `name`을 부여하세요.

### 알 수 없는 스텝 종류

**증상:** `E_SCHEMA_INVALID: step "my-step": unknown kind "InstallKubeconfig"`

**원인:** `kind` 값이 등록된 어떤 스텝 종류와도 일치하지 않습니다. 흔한 원인은 오타와 워크플로와 deck 바이너리 간의 버전 드리프트입니다.

**해결:** [step-kinds.md](step-kinds.md)에서 정확한 종류 이름을 확인하세요. 종류 이름은 대소문자를 구분합니다.

### 비연속 `parallelGroup` (`E_PARALLEL_GROUP_DISCONTIGUOUS`)

**증상:** `E_PARALLEL_GROUP_DISCONTIGUOUS: parallelGroup "images" is not contiguous in phase "load"`

**원인:** `parallelGroup` 레이블을 공유하는 스텝들이 단계 내에서 인접해 있지 않습니다. 병렬 배치가 한번 닫히면, 같은 `parallelGroup` 레이블이 단계 후반부에 다시 나타날 수 없습니다.

**해결:** `parallelGroup` 값을 공유하는 모든 스텝을 단계 내에서 연속되도록 옮기세요. 두 번째 그룹이 중간에 끼어드는 스텝들 이후에 실행되어야 한다면, 다른 `parallelGroup` 레이블을 사용하세요.

### 출력이 없는 종류에 `register` 사용 (`E_REGISTER_OUTPUT_NOT_FOUND`)

**증상:** `E_REGISTER_OUTPUT_NOT_FOUND: step "write-config": kind WriteFile declares no outputs; register key "result" is not valid`

**원인:** `register` 블록이 스텝 종류가 생성하지 않는 출력 키를 명명했습니다. 출력을 명시적으로 선언하는 스텝 종류만 `register`를 지원합니다.

**해결:** `register` 블록을 제거하거나, 필요한 출력을 생성하는 스텝 종류로 전환하세요(예: `InitKubeadm`은 `joinFile`을 생성). 스텝 종류의 [step-kinds.md](step-kinds.md) 항목에 선언된 출력이 나열되어 있습니다.

### 예약된 런타임 변수 이름 (`E_RUNTIME_VAR_RESERVED`, `E_REGISTER_VAR_INVALID`)

**증상:** `E_RUNTIME_VAR_RESERVED: register name "host" conflicts with built-in runtime.host`

**원인:** `register` 키가 내장 런타임 네임스페이스(`host`)와 충돌하거나, 요구되는 명명 패턴과 일치하지 않습니다.

**해결:** 내장 이름을 가리지 않는 register 키를 선택하세요. 내장 필드는 `runtime.host.*`와 `context.*` 아래에 있습니다. 사용자 정의 register 출력은 `runtime.<your-key>`가 됩니다.

---

## `deck prepare` 실패

`deck prepare`는 네트워크 또는 로컬 소스에서 아티팩트를 가져와 `outputs/` 아래에 씁니다. 여기서의 실패는 번들을 아직 빌드할 수 없음을 의미합니다.

### 네트워크 및 다운로드 에러 (`E_PREPARE_SOURCE_NOT_FOUND`, `E_PREPARE_OFFLINE_POLICY_BLOCK`)

**증상:** `E_PREPARE_SOURCE_NOT_FOUND: step "fetch-kubeadm": source.path "bin/kubeadm" not resolved to any configured fetch source`

**원인:** 소스 경로가 구성된 어떤 fetch 소스와도 일치하지 않거나, URL 다운로드가 오프라인 정책에 의해 차단되었습니다.

**해결:** `source.path` 또는 `source.url`이 도달 가능한 소스와 일치하는지 확인하세요. `prepare`를 실행하는 머신이 오프라인이고 스텝에 URL이 필요하다면, 로컬 미러를 구성하거나 네트워크 접근이 가능한 머신에서 `prepare`를 실행하세요. 실행 전에 무엇이 가져와질지 확인하려면 `deck prepare --dry-run`을 사용하세요.

### 체크섬 불일치 (`E_PREPARE_CHECKSUM_MISMATCH`)

**증상:** `E_PREPARE_CHECKSUM_MISMATCH: step "fetch-containerd": sha256 mismatch (expected …, got …)`

**원인:** 다운로드된 아티팩트가 예상되는 SHA-256 다이제스트와 일치하지 않습니다. 가능한 원인으로는 손상된 다운로드, 업스트림 콘텐츠 변경, 오래된 로컬 캐시 등이 있습니다.

**해결:** `deck prepare --refresh`를 실행하여 재사용을 우회하고 모든 아티팩트를 다시 다운로드하세요. 불일치가 지속되면, 워크플로의 예상 다이제스트가 업스트림 소스와 일치하는지 확인하세요.

### 리졸버 메타데이터 불일치

**원인:** 이전 `prepare` 실행이 기록한 재사용 메타데이터(이미지의 경우 `outputs/images/.deck-cache-images.json`, 패키지의 경우 패키지 인덱스 메타데이터)가 더 이상 디스크에 있는 내용과 일치하지 않습니다.

**해결:** `deck prepare --refresh`를 실행하여 캐시된 메타데이터를 무시하고 새 복사본을 가져오세요. 쓰기 전에 준비된 outputs 디렉터리를 제거하려면 `deck prepare --clean`을 사용하세요 — outputs 트리가 일관되지 않은 상태일 때 가장 철저한 초기화 방법입니다.

### 컨테이너 런타임 누락 (`E_PREPARE_RUNTIME_NOT_FOUND`, `E_PREPARE_RUNTIME_UNSUPPORTED`)

**증상:** `E_PREPARE_RUNTIME_NOT_FOUND: no supported container runtime (docker/podman) found`

**원인:** `backend.mode: container`를 사용하는 `DownloadPackage` 스텝은 `docker` 또는 `podman`을 필요로 하는데 둘 다 찾을 수 없었습니다. 이 스텝은 컨테이너 안에서 OS 패키지를 빌드하므로, `prepare`를 실행하는 머신에 작동하는 컨테이너 런타임이 있어야 합니다.

**해결:** prepare 머신에 Docker 또는 Podman을 설치하세요. 또는 워크플로가 허용한다면 `backend.mode`를 컨테이너가 아닌 모드로 전환하되, 대상 배포판에 어떤 모드가 사용 가능한지 [step-kinds.md](step-kinds.md)의 `DownloadPackage` 항목에서 확인하세요.

---

## apply 시작 시 번들 무결성 실패

워크플로 단계를 실행하기 전에, `deck apply`는 번들 매니페스트를 검증합니다. 검증 실패는 즉시 실행을 중단합니다.

### `E_MANIFEST_MISSING`

**증상:** `E_MANIFEST_MISSING: .deck/manifest.json not found in bundle`

**원인:** 번들이 `.deck/manifest.json` 파일 없이 전송되었거나, 매니페스트를 쓰는 `deck prepare`를 먼저 실행하지 않고 번들이 조립되었습니다.

**해결:**
1. 번들을 tarball로 전송했다면, 아카이브에 `.deck/manifest.json`이 포함되어 있는지 확인하세요: `tar -tf bundle.tar | grep manifest.json`.
2. 매니페스트가 누락된 경우, 번들을 다시 빌드하세요: `deck prepare`를 실행한 뒤 `deck bundle build --out ./bundle.tar`을 실행합니다.
3. 완전한 아카이브를 대상으로 다시 전송하세요.

### `E_MANIFEST_EMPTY`

**증상:** `E_MANIFEST_MISSING: manifest exists but contains no tracked entries` (또는 `E_MANIFEST_EMPTY`)

**원인:** 매니페스트 파일은 존재하지만 `entries` 배열이 비어 있습니다. 이는 일반적으로 `deck prepare`가 실행되었으나 아티팩트를 생성하지 않았거나(빈 `outputs/` 트리), 전송 중 매니페스트가 잘렸음을 의미합니다.

**해결:** `deck prepare`를 다시 실행하여 prepare 워크플로가 선언한 아티팩트로 `outputs/`를 채우세요. 그런 다음 번들을 다시 빌드하고 다시 전송하세요.

### `E_BUNDLE_INTEGRITY`

**증상:** `E_BUNDLE_INTEGRITY: artifact outputs/files/kubeadm.conf: sha256 mismatch`

**원인:** 아티팩트가 누락되었거나, SHA-256 다이제스트 또는 파일 크기가 매니페스트와 일치하지 않습니다. 흔한 원인: 부분 전송, 파일 손상, `prepare` 이후 `outputs/`에 대한 수동 편집.

**해결:**
1. `deck bundle verify --file ./bundle.tar`(또는 `deck bundle verify --file ./bundle-dir`)를 실행하여 불일치하는 모든 항목에 대한 전체 보고서를 얻으세요.
2. 번들 디렉터리가 파일 단위로 전송되었다면, `rsync -c`나 검증된 아카이브처럼 체크섬을 보존하는 방법으로 다시 전송하세요.
3. 번들 자체는 올바르지만 아카이브가 잘못 추출되었다면, 다시 추출하고 다시 검증하세요.
4. 아티팩트가 의도적으로 변경되었다면, 처음부터 번들을 다시 빌드하세요: `deck prepare` 다음 `deck bundle build`.

매니페스트가 다루는 경로의 전체 목록은 [bundle-layout.md](bundle-layout.md#apply-time-manifest-verification)를 참고하세요.

---

## apply 실패와 재개

### 재개 작동 방식

`deck apply`는 워크플로, vars, 실행 컨텍스트의 지문(fingerprint)을 키로 하는 상태 파일에 진행 상황을 저장합니다. 완료된 단계가 기록됩니다. 다음 비-fresh 실행에서는 완료된 단계를 건너뛰고 첫 번째 미완료 단계에서 실행을 재개합니다.

실패한 단계는 다음 실행에서 첫 스텝부터 다시 실행됩니다. 실패한 단계 내부의 부분 진행 상황은 재사용되지 않으며 — 단계 전체가 다시 실행됩니다.

```bash
# 무엇이 완료되었고 다음에 무엇이 실행될지 확인
deck state show --root . --scenario apply

# 그런 다음 재개
deck apply --root . --scenario apply
```

전체 상태 모델과 파일 위치는 [apply-state.md](apply-state.md)를 참고하세요.

### `--fresh`를 사용해야 할 때

저장된 상태와 무관하게 모든 단계를 다시 실행하려면 `deck apply --fresh`를 사용하세요:

```bash
deck apply --root . --scenario apply --fresh
```

`--fresh`는 선택된 상태 키만 지웁니다. 같은 디렉터리의 다른 상태 파일은 보존됩니다. `--fresh`와 `--dry-run`은 함께 사용할 수 없다는 점에 유의하세요.

### 상태가 초기화될 수 있는 이유 (새 상태 키)

워크플로 파일 내용, 유효한 vars, 또는 실행 컨텍스트가 변경되면, deck은 다른 상태 키를 계산합니다. 기존 상태는 삭제되지 않으며 — 단지 새 키로는 더 이상 매칭되지 않을 뿐입니다. 이는 워크플로를 편집한 후 재개한 실행이 처음부터 시작됨을 의미합니다.

적용 전에 `deck plan --root . --scenario apply`를 사용하여 해석된 상태 키와 어떤 스텝이 실행되거나 건너뛰어질지 확인하세요. 디스크에 있는 모든 상태 파일을 보려면 `deck state list`를 사용하세요.

### 상태 검사 및 삭제

```bash
# 워크플로의 현재 상태 표시
deck state show --root . --scenario apply

# 저장된 모든 상태 파일 나열
deck state list

# 특정 워크플로의 상태 삭제 (--yes 필요)
deck state clear --root . --scenario apply --yes

# 저장된 모든 상태 삭제
deck state clear --all --yes
```

---

## 템플릿 및 CEL 실수

### 단일 중괄호 문법 (`E_TEMPLATE_SINGLE_BRACE`)

**증상:** `E_TEMPLATE_SINGLE_BRACE: step "write-config": template uses {vars.name}; use {{.vars.name}}`

**원인:** 템플릿 표현식이 요구되는 이중 중괄호 문법 대신 단일 중괄호 문법(`{var}`)을 사용합니다. deck은 문자열 보간에 Go 템플릿을 사용합니다.

**해결:** `{vars.name}`을 `{{ .vars.name }}`으로 바꾸세요. 앞에 붙는 점에 유의하세요: 템플릿 내에서 vars는 `.vars.NAME`으로, 런타임 값은 `.runtime.NAME`으로, 컨텍스트 값은 `.context.NAME`으로 접근합니다.

```yaml
# 잘못됨
content: "cluster: {vars.clusterName}"

# 올바름
content: "cluster: {{ .vars.clusterName }}"
```

### CEL 타입 에러와 `E_CONDITION_EVAL`

**증상:** `E_CONDITION_EVAL: step "install-rhel-packages": when: type error: expected bool, got string`

**원인:** `when` 필드는 Go 템플릿이 아니라 CEL 표현식을 받습니다. CEL은 다른 문법을 사용합니다: 중괄호 없음, 앞에 붙는 점 없음, 엄격한 타입.

**해결:** CEL 표현식을 확인하세요. `when`의 변수 참조는 `vars.NAME`, `runtime.NAME`, `context.NAME`을 사용합니다(중괄호 없음, 앞에 붙는 점 없음):

```yaml
# 잘못됨 — CEL 필드에 Go 템플릿 문법
when: "{{ .vars.role == \"control-plane\" }}"

# 올바름 — CEL 표현식
when: vars.role == "control-plane"
```

런타임 호스트 사실의 경우 `runtime.host.os.family`, `runtime.host.arch` 등을 사용하세요([workflow-model.md](workflow-model.md#built-in-runtime-fields) 참고).

### register 키 충돌 (`E_RUNTIME_VAR_REDEFINED`, `E_PARALLEL_OUTPUT_CONFLICT`)

**증상:** `E_RUNTIME_VAR_REDEFINED: register key "joinFile" is defined more than once`

**원인:** 두 스텝이 같은 키 아래에 출력을 register하여 모호한 런타임 값을 만듭니다.

**해결:** 각 스텝에 서로 다른 register 키 이름을 사용하세요. 두 스텝이 의도적으로 같은 논리적 값을 생성하는 경우(예: 분기 워크플로), `when:` 조건을 사용하여 하나만 실행되도록 보장하세요.

### 같은 병렬 배치 내에서 register된 값 소비 (`E_PARALLEL_RUNTIME_DEPENDENCY`)

**증상:** `E_PARALLEL_RUNTIME_DEPENDENCY: step "join-node" reads runtime.joinFile produced by step "get-join-cmd" in the same parallelGroup`

**원인:** `parallelGroup` 배치 내의 스텝이 같은 배치 내 다른 스텝이 register한 `runtime.*` 값을 소비하려고 시도합니다. 병렬 배치의 register된 값은 전체 배치가 성공한 이후에만 보이게 됩니다.

**해결:** 생성하는 스텝(`register:`가 있는 스텝)을 병렬 배치 바깥의 더 이른 단계나 더 이른 순차 스텝으로 옮기세요. 소비하는 스텝은 병렬 배치에 남아 있거나 그 이후에 실행될 수 있습니다.

병렬 배치 제약의 전체 집합은 [workflow-model.md](workflow-model.md#parallel-batches)를 참고하세요.

---

## `deck ask` 문제

### 프로바이더 미구성

**증상:** `deck ask`가 누락된 프로바이더나 모델에 관한 에러로 응답하거나, 저하된 로컬 전용 응답을 반환합니다.

**원인:** 프로바이더가 구성되지 않았거나, 저장된 구성이 불완전합니다.

**해결:** 프로바이더를 구성하세요:

```bash
deck ask config set \
  --provider openai \
  --model gpt-5.4 \
  --api-key "$DECK_ASK_API_KEY"
```

지원되는 프로바이더: `openai`, `openrouter`, `gemini`. 현재 구성을 검사하세요:

```bash
deck ask config show
```

### OAuth 또는 전송(transport) 실패

**증상:** `deck ask`가 전송 시작 실패, MCP 초기화 실패, 또는 도구 목록 불일치를 보고합니다.

**해결:** 헬스 체크를 실행하여 전송 문제와 기능(capability) 격차를 구분하세요:

```bash
deck ask config health
```

이것은 프로바이더 엔드포인트에 도달 가능한지, MCP 서버가 시작되고 있는지, 필요한 기능이 존재하는지 확인하는 가장 빠른 방법입니다.

특히 OpenAI OAuth 세션의 경우:

```bash
deck ask status --provider openai   # 저장된 세션 확인
deck ask login --provider openai    # 브라우저 로그인 시작
```

### 작성(authoring) 경로는 즉시 실패

**원인:** `deck ask` 작성 경로(`--create`, `--edit`)는 모델 접근에 의존합니다. 이들은 로컬 생성으로 폴백하지 않습니다.

**해결:** `deck ask config health`로 프로바이더 연결성을 확인하세요. 모델에 도달할 수 없다면(예: 외부 증거 프로바이더가 없는 에어갭 내부), 제한된 로컬 폴백을 갖는 `deck ask --review` 또는 `deck ask` 질문/설명 경로를 사용하거나, 연결성이 있는 머신에서 워크플로 파일을 미리 생성하여 정적 파일로 들여오세요.

### `deck ask` 진단

상세도를 높여 경로 선택과 프로바이더 이벤트를 추적하세요:

```bash
deck ask --v=1 "explain workflows/scenarios/apply.yaml"   # 경로 + 프로바이더 요약
deck ask --v=2 "explain workflows/scenarios/apply.yaml"   # + 사용자 명령 + MCP 이벤트
deck ask --v=3 "review this workspace"                    # 전체 디버그 + 프롬프트 아티팩트
```

`--v=3`에서는 프롬프트와 응답 아티팩트가 `.deck/ask/runs/<run-id>/` 아래에 기록됩니다. 세션 상태도 `.deck/ask/last-agent-session.json` 아래에 있습니다.

전체 ask 진단 레퍼런스는 [ask.md](ask.md#diagnostics-and-troubleshooting)를 참고하세요.

---

## 진단 정보 얻기

### 상세도 레벨

```bash
deck apply --root . --scenario apply --v=1   # kind 및 소요 시간 포함 단계/스텝 진행
deck apply --root . --scenario apply --v=2   # + 실행 계획 및 상태 스냅샷
deck apply --root . --scenario apply --v=3   # + 상태 키, 컨텍스트 키, 스텝 계약 키
```

상세도 레벨은 모든 명령에 걸쳐 적용됩니다. `deck prepare --v=2`는 아티팩트 그룹과 캐시 재사용/가져오기 진단을 추가하며, `deck bundle verify --v=3`은 항목별 경로, 카테고리, 크기, 해시 추적을 추가합니다.

### 구조화된 JSON 로그

```bash
deck apply --root . --scenario apply --v=2 --log-format=json 2>apply.log
jq 'select(.event == "step_failed")' apply.log
```

stderr의 JSON Lines는 모든 이벤트 필드를 보존하며 스크립팅에 안정적입니다.

### apply 실행 로그

모든 `deck apply` 호출은 XDG 상태 루트 아래에 호출별 실행 로그를 씁니다:

```text
$XDG_STATE_HOME/deck/runs/<run-id>/
~/.local/state/deck/runs/<run-id>/    (기본값)
```

각 실행 디렉터리에는 `record.json`(구조화된 요약, 각 스텝 이후 업데이트됨)과 `events.jsonl`(스텝별 추가 전용 이벤트 스트림)이 들어 있습니다. 이들은 완료되었거나 중단된 apply에 대한 스텝 타이밍, 순서, 에러 세부 정보의 가장 신뢰할 수 있는 출처입니다.

실행 로그 스키마와 필드 레퍼런스는 [apply-runlogs.md](apply-runlogs.md)를 참고하세요.
