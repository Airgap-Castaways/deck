---
source: docs/workflow-model.md
source_hash: 81c5cf3ed22fadcd0016bfa2fd919dfb4fe3110a
---
<!-- BEGIN 윤문 -->
# 워크플로 모델

deck은 YAML 워크플로 모델을 사용하며, 규모가 큰 절차도 검토하기 쉬운 상태로 유지합니다. 목표는 새로운 DSL을 만드는 것이 아니라, 에어갭(망분리) 운영 작업에서 점점 늘어나는 셸 스크립트보다 명확한 구조를 부여하는 데 있습니다. 타입이 지정된 스텝은 의도를 분명히 드러내고, 이름 붙은 단계는 운영자가 세부 내용을 다 읽지 않아도 절차가 무엇을 하는지 한눈에 보여줍니다.

## 최상위 필드

- `version`: 현재 `v1alpha1`
- `vars`: 선택적 변수 맵
- `steps`: 최상위 스텝 목록
- `phases`: 더 구조화된 실행을 위한 이름 붙은 단계 목록

스키마는 한 번에 한 가지 실행 모드만 허용합니다.

- 최상위 `steps`
- 이름 붙은 `phases`

단계 임포트는 `workflows/components/`를 기준으로 해석합니다. `../components/k8s/prereq.yaml`이 아니라 `k8s/prereq.yaml`처럼 컴포넌트 기준 상대 경로로 작성합니다.

`workflows/components/` 파일은 스텝 프래그먼트입니다. 이 파일은 `steps:`만 담으며 공유된 `vars.*`를 참조할 수 있지만, 공유 기본값은 `workflows/vars.yaml`이나 임포트하는 시나리오의 `vars:` 블록에 두어야 합니다.

<!-- BEGIN GENERATED:WORKFLOW_SCHEMA_CONTRACT -->
## Workflow Schema Contract {#workflow-schema-contract}

deck 워크플로를 작성할 때 참고하는 최상위 레퍼런스입니다.

- schema: `../schemas/deck-workflow.schema.json`

### Example

```yaml
version: v1alpha1
steps:
  - id: write-config
    apiVersion: deck/v1alpha1
    kind: WriteFile
    spec:
      path: /etc/example.conf
      content: hello
```

### Fields

| Key | Type | Required | Default | Enum | Description | Example |
|---|---|---:|---|---|---|---|
| `phases` | `array<object>` | no | `` | `` | 순서가 지정된 실행 단계입니다. 각 단계는 임포트, 스텝, 또는 둘 다를 담을 수 있습니다. | `[{name:install,steps:[...]}]` |
| `steps` | `array<object>` | no | `` | `` | 이름 붙은 단계가 필요 없는 워크플로를 위한 평면 스텝 목록입니다. 실행 시 이 스텝들은 암시적인 `default` 단계로 정규화됩니다. | `[{id:configure-runtime,kind:WriteContainerdConfig,spec:{...}}]` |
| `vars` | `object` | no | `map[]` | `` |  | `map[]` |
| `version` | `string` | yes | `` | `v1alpha1` |  | `v1alpha1` |

### Validation Rules

- 최상위 그룹 `phases`와 `steps` 중 최소 하나는 있어야 합니다.
- 최상위 `phases`와 최상위 `steps`는 같은 워크플로에서 함께 설정할 수 없습니다.

### Notes

- 워크플로는 `phases`나 `steps` 중 최소 하나를 정의해야 합니다.
- 워크플로는 최상위 `phases`와 최상위 `steps`를 동시에 정의할 수 없습니다.
- 최상위 `steps`는 `default`라는 이름의 암시적 단계로 실행됩니다.
- 임포트는 `phases[].imports` 아래에서만 지원하며 `workflows/components/`를 기준으로 해석합니다.
- 스텝이 `apiVersion`을 생략하면, deck은 스키마와 역할 검사가 실행되기 전에 최상위 워크플로 `version`에서 이를 해석합니다.
- 워크플로 모드는 파일 안의 `role` 필드가 아니라 명령 맥락이나 파일 위치로 결정됩니다.
- 각 스텝은 최상위 워크플로 스키마를 통과한 뒤에도 자신의 종류별 스키마로 다시 검증됩니다.
<!-- END GENERATED:WORKFLOW_SCHEMA_CONTRACT -->

## Variables {#variables}

안내가 곁들여진 설명은 [변수와 템플릿화](guides/variables-and-templating.md)를 참고하세요.

변수, 런타임 값, 실행 컨텍스트는 서로 다른 출처에서 옵니다.

정적 `vars`는 다음 네 가지 출처에서 흘러오며, 우선순위 순서는 다음과 같습니다.

1. CLI `--var` 오버라이드
2. 시나리오 파일의 `vars:` 블록
3. 노드 범위 선택 전에 공유 vars로 병합되는 CLI `-f, --vars-file` 오버레이
4. `workflows/vars.yaml` 공유 기본값

`deck lint`, `deck prepare`, `deck plan`, `deck apply`는 `workflows/vars.yaml`의 노드 범위 공유 변수도 지원합니다. `hosts:`가 있으면 deck은 실행 시점에 로컬 호스트명을 감지하고, 선택적인 `all:` 값을 공유 기본값으로 적용한 뒤, 일치하는 호스트 항목을 최상위 `vars`로 병합합니다. 호스트명이 목록에 없으면 일반 `vars.yaml` 값과 `all:` 값으로 실행을 이어 갑니다.

노드 범위 `vars.yaml` 예시:

```yaml
all:
  kubernetesVersion: v1.35.5
  podCIDR: 10.244.0.0/16

hosts:
  k8s-cp1:
    ip: 192.168.81.211
    role: control-plane

  k8s-worker1:
    ip: 192.168.81.221
    role: worker
```

`deck lint`, `deck prepare`, `deck plan`, `deck apply`의 경우, deck은 먼저 `workflows/vars.yaml`과 `prepare`, `plan`, `apply`에 전달된 `-f, --vars-file` 오버레이로 공유 vars 문서를 구성합니다. 뒤에 오는 vars 파일은 같은 키에 대해 `vars.yaml`과 동일한 깊은 병합 방식으로 앞선 파일을 덮어씁니다. 실제 변수 우선순위는 다음과 같습니다.

1. `hosts:`가 있을 때 `all`과 `hosts`를 제외한 일반 공유 vars 값
2. 공유 vars `all:` 값
3. 공유 vars `hosts.<hostname>`에서 선택된 호스트 값
4. 선택된 워크플로 또는 시나리오 `vars:` 값
5. CLI `--var` 오버라이드

호스트명 매칭은 감지된 호스트명을 먼저 시도하고, 그다음 첫 `.` 앞의 짧은 호스트명을 시도합니다. 호스트 항목이 없어도 치명적이지 않습니다. 호스트별 필드로 분기하는 워크플로는 `all:`에 `role: ""` 같은 안전한 기본값을 제공한 뒤 `vars.role == "control-plane"` 같은 조건을 사용해야 합니다.

런타임 값은 `register` 출력과 `runtime.host` 같은 내장 런타임 팩트를 통해 별도로 흘러갑니다. `context.*` 아래의 실행 컨텍스트 값은 deck이 명령 실행 시점에 해석하는 메타데이터로, 명령 이름, 워크플로 소스, 워크플로 경로, 번들 루트, 출력 루트, 상태 파일 등이 있습니다.

`prepare`나 `apply`를 실행하기 전에 입력 스냅숏을 확인하려면 `deck plan vars`를 사용하세요. 이 명령은 실제 `vars`, 해석된 `context`, 실행 전에 알려진 초기 `runtime` 값, 그리고 워크플로 스텝이 등록할 수 있는 계획된 런타임 키를 출력합니다. 등록되는 런타임 값은 이를 생성하는 스텝이 실행되기 전에는 예측할 수 없습니다.

**`workflows/vars.yaml`**: 공유 기본값을 한 번 정의합니다.

```yaml
clusterName: prod-k8s
```

**시나리오 `vars:` 블록**: 특정 시나리오에 맞춰 덮어쓰거나 확장합니다.

```yaml
version: v1alpha1
vars:
  clusterName: staging-k8s   # overrides vars.yaml
```

**CLI vars 파일**: `workflows/vars.yaml`을 편집하지 않고 사이트나 노드 값을 오버레이합니다.

```bash
deck apply --root . --scenario apply -f vars/site.yaml -f vars/cp1.yaml
```

vars 파일 경로는 `vars.yaml`이 있는 동일한 `workflows/` 위치를 기준으로 하는 상대 경로입니다.

- 로컬 워크플로에서는 `-f vars/site.yaml`이 `workflows/vars/site.yaml`로 해석됩니다.
- 원격 워크플로에서는 같은 상대 경로가 원격 `workflows/` URL을 기준으로 해석됩니다.
- 뒤에 오는 파일은 같은 키에 대해 `vars.yaml`과 동일한 깊은 병합 방식으로 앞선 파일을 덮어씁니다.
- deck은 vars 파일 오버레이를 병합한 뒤 `all:`과 `hosts:` 노드 범위 값을 추출합니다.
- `--var`은 우선순위가 가장 높은 최종 오버라이드로 남습니다.

**템플릿 보간**: 문자열 필드 안에서 `{{ .vars.NAME }}`을 사용합니다.

```yaml
- id: write-hostname
  kind: WriteFile
  spec:
    path: /etc/hostname
    content: "{{ .vars.clusterName }}\n"
```

**CEL 표현식**: `when:` 조건에서 `vars.NAME`, `runtime.NAME`, `context.NAME`을 (중괄호 없이) 사용합니다.

```yaml
- id: install-rhel-packages
  kind: InstallPackage
  spec:
    packages: [kubeadm, kubelet, kubectl]
  when: runtime.host.os.family == "rhel"
```

노드 범위 vars는 계획과 상태 해싱 전에 선택되는 정적 입력입니다. `runtime.host`은 감지된 OS, 아키텍처, 커널 데이터를 담는 런타임 팩트 네임스페이스로 남습니다.

<!-- BEGIN GENERATED:SYSTEM_VARIABLES -->
### Built-In Runtime Fields {#built-in-runtime-fields}

`runtime.host`은 prepare와 apply 모두에서 사용하는 내장 예약 런타임 네임스페이스입니다. OS 계열, 배포판 ID, 버전, 아키텍처, 커널 릴리스처럼 감지된 호스트 팩트에 사용합니다. 감지된 로컬 호스트 팩트를 정적 `vars` 값으로 모델링하지 마세요.

| Field | Type | Description |
|---|---|---|
| `runtime.host.os.name` | `string` | Go 런타임이 보고한 운영체제 이름입니다. |
| `runtime.host.os.id` | `string` | `/etc/os-release`의 `ID`에서 가져온 배포판 ID로, 소문자입니다. |
| `runtime.host.os.family` | `string` | `debian`이나 `rhel`처럼 추론된 배포판 계열이며, 알 수 없으면 빈 값입니다. |
| `runtime.host.os.version` | `string` | `/etc/os-release`의 `VERSION`에서 가져온 배포판 버전입니다. |
| `runtime.host.os.versionId` | `string` | `/etc/os-release`의 `VERSION_ID`에서 가져온 배포판 버전 ID입니다. |
| `runtime.host.os.release` | `string` | 기존 워크플로를 위해 유지되는 `runtime.host.os.versionId`의 별칭입니다. |
| `runtime.host.os.idLike` | `string` | `/etc/os-release`의 `ID_LIKE`에서 가져온 배포판 호환 ID로, 소문자입니다. |
| `runtime.host.arch` | `string` | `amd64`나 `arm64`처럼 정규화된 호스트 아키텍처입니다. |
| `runtime.host.kernel.release` | `string` | `/proc/sys/kernel/osrelease`에서 가져온 커널 릴리스입니다. |

### Execution Context Fields {#execution-context-fields}

`context`는 `when` 표현식과 템플릿 모두에서 사용할 수 있습니다. 표준 필드는 다음과 같습니다.

| Field | Type | Prepare | Apply | Description |
|---|---|---:|---:|---|
| `context.command` | `string` | yes | yes | 현재 명령으로, `prepare` 또는 `apply`입니다. |
| `context.workflow.source` | `string` | yes | yes | 워크플로 소스로, `filesystem` 또는 `server`입니다. |
| `context.workflow.isServer` | `boolean` | yes | yes | `context.workflow.source == "server"`에서 파생된 편의용 불리언 값입니다. |
| `context.workflow.path` | `string` | yes | yes | 해석된 워크플로 파일 경로 또는 URL입니다. |
| `context.workflow.scenario` | `string` | no | yes | apply가 시나리오를 해석했을 때의 시나리오 이름입니다. |
| `context.paths.bundleRoot` | `string` | yes | yes | prepare 중에는 준비된 출력 루트, apply 중에는 선택된 번들 루트입니다. |
| `context.paths.outputRoot` | `string` | yes | no | 준비된 출력 루트입니다. |
| `context.paths.stateFile` | `string` | no | yes | apply 상태 파일 경로입니다. |

기존 템플릿을 위해 레거시 별칭도 계속 사용할 수 있습니다. `context.bundleRoot`은 `context.paths.bundleRoot`에, `context.stateFile`은 `context.paths.stateFile`에 매핑됩니다.

apply 상태 키를 계산할 때, deck은 다른 컨텍스트 값에서 파생된 필드(예: `context.workflow.isServer`, 그리고 상태 키 자체에서 파생되는 `context.paths.stateFile`)를 제외하고 실행 컨텍스트의 지문을 포함합니다.
<!-- END GENERATED:SYSTEM_VARIABLES -->

## 최소 워크플로

```yaml
version: v1alpha1
steps:
  - id: prepare-state-dir
    kind: EnsureDirectory
    spec:
      path: /var/lib/deck
      mode: "0755"
```

## 최소 prepare 워크플로

```yaml
version: v1alpha1
steps:
  - id: fetch-kubeadm
    kind: DownloadFile
    spec:
      source:
        url: https://example.local/kubeadm
      mode: "0755"
```

prepare 다운로드 스텝이 출력 위치를 명시하지 않으면, deck은 스텝 종류의 기본 준비 경로를 사용합니다.

- `DownloadFile`: `files/<basename>`
- `DownloadImage`: `images/`
- `DownloadPackage`: `packages/`, 또는 `repo.type`이 설정된 경우 `packages/deb/<release>`와 `packages/rpm/<release>`

## Step Envelope Contract {#step-envelope-contract}

모든 워크플로 스텝은 종류별 `spec` 검증이 실행되기 전에 동일한 외부 엔벨로프를 사용합니다.

필수 필드:

- `id`: 안정적인 스텝 식별자로, 워크플로 스텝 id 패턴과 일치해야 합니다
- `kind`: `WriteFile`이나 `CheckKubernetesCluster` 같은 타입 지정 스텝 이름
- `spec`: 해당 스텝의 스키마로 검증되는 종류별 페이로드

선택적 공유 필드:

- `apiVersion`: 스텝 api 버전으로, 생략하면 deck이 최상위 워크플로 `version`에서 해석합니다
- `when`: CEL 표현식으로, false로 평가되면 스텝을 건너뜁니다
- `parallelGroup`: 값이 같은 연속된 스텝들은 한 단계 안에서 한 배치로 실행할 수 있습니다
- `retry`: 실패 시 재시도 횟수
- `timeout`: `30s`나 `5m` 같은 기간 문자열
- `register`: 선언된 스텝 출력을 이후 런타임 값으로 내보냅니다
- `metadata`: 도구나 감사 목적 맥락을 위한 자유 형식 주석 맵

공유 엔벨로프 규칙:

- `register` 키는 CEL에서 `runtime.<name>`, 템플릿에서 `.runtime.<name>`으로 사용할 수 있게 됩니다
- 스텝이 병렬 배치 안에서 실행되면, 그 `register` 출력은 전체 배치가 성공한 뒤에야 보입니다
- `spec`은 공유 엔벨로프를 통과한 뒤 선택된 스텝 종류로 항상 다시 검증됩니다

### `when`: conditional execution {#when--conditional-execution}

안내가 곁들여진 설명은 [when으로 조건 지정하기 (CEL)](guides/conditions-and-cel.md)를 참고하세요.

`when`은 CEL 표현식을 받습니다. `vars:`나 `vars.yaml`에 정의된 입력 변수를 참조하려면 `vars.`을, 실행 중 앞서 등록된 스텝 출력과 `runtime.host` 아래의 내장 호스트 팩트를 참조하려면 `runtime.`을, deck이 제공하는 실행 메타데이터를 참조하려면 `context.`을 사용합니다.

```yaml
steps:
  - id: add-debian-repo
    kind: ConfigureRepository
    spec:
      format: deb
      repositories:
        - id: offline-base
          baseurl: file:///srv/offline-repo
          trusted: true
    when: runtime.host.os.family == "debian"

  - id: add-rhel-repo
    kind: ConfigureRepository
    spec:
      format: rpm
      repositories:
        - id: offline-base
          name: offline-base
          baseurl: file:///srv/offline-repo
          enabled: true
          gpgcheck: false
    when: runtime.host.os.family == "rhel"
```

`swap`, `kernelModules`, 필수 바이너리처럼 호스트 적합성 검사에서 워크플로가 빠르게 실패해야 할 때는 `CheckHost`를 사용하세요. `CheckHost`는 그러한 조건을 검증하지만, 워크플로에 `CheckHost` 스텝이 없어도 `runtime.host`은 존재합니다.

### `register`: capture step output {#register--capture-step-output}

안내가 곁들여진 설명은 [register로 스텝 출력 캡처하기](guides/capturing-output.md)를 참고하세요.

`register`는 런타임 변수 이름을 스텝 출력 키에 매핑합니다. 내보낸 값은 CEL의 `runtime.`과 템플릿의 `.runtime`을 통해 이후 스텝에서 사용할 수 있습니다. 스텝이 병렬 배치 안에서 실행되면, 그 값은 전체 배치가 성공한 뒤에 보입니다.

```yaml
steps:
  - id: get-join-cmd
    kind: InitKubeadm
    spec:
      outputJoinFile: "{{ .vars.joinFile }}"
    register:
      joinFile: joinFile

  - id: join-node
    kind: JoinKubeadm
    spec:
      joinFile: "{{ .runtime.joinFile }}"
      extraArgs: ["--cri-socket", "unix:///run/containerd/containerd.sock", "--ignore-preflight-errors=Swap,FileExisting-crictl,FileExisting-conntrack,FileExisting-socat"]
```

`register`는 선택된 스텝 종류가 명시적으로 선언한 출력 이름만 내보낼 수 있습니다. 예를 들어 `InitKubeadm`은 `joinFile`을 내보낼 수 있는 반면, 선언된 출력이 없는 스텝은 검증 중에 비어 있지 않은 `register` 매핑을 거부합니다.

## Phases {#phases}

안내가 곁들여진 설명은 [단계와 병렬 처리](guides/phases-and-parallelism.md)를 참고하세요.

절차에 자연스러운 경계가 있을 때, 예를 들어 런타임 블록 전에 완료되어야 하는 호스트 사전 요구 블록이 있을 때 단계를 사용하세요. 스텝이 몇 개뿐인 단순한 apply 워크플로에는 평면 `steps:`로 충분합니다.

각 단계는 컴포넌트 프래그먼트를 임포트하거나, 인라인 스텝을 포함하거나, 둘 다 할 수 있습니다. 단계는 `apply`에서 지속되는 재개 경계이기도 합니다.

```yaml
version: v1alpha1
phases:
  - name: host-prereqs
    imports:
      - path: k8s/prereq.yaml
      - path: repo/offline-repo.yaml
  - name: runtime
    maxParallelism: 2
    imports:
      - path: k8s/containerd-kubelet.yaml
  - name: verify
    steps:
      - id: check-node-ready
        kind: Command
        spec:
          command: [kubectl, get, nodes]
```

임포트 경로는 `workflows/components/`를 기준으로 하는 상대 경로입니다. `../components/k8s/prereq.yaml`이 아니라 `k8s/prereq.yaml`로 작성합니다.

### Conditional imports (`imports[].when`)

각 임포트 항목은 선택적인 `when` CEL 조건을 받습니다. deck은 임포트의 `when`을 그 파일에서 로드된 모든 스텝에 AND로 결합합니다. 결합 형태는 `(import-when) && (step-when)`입니다.

- 임포트에 `when`이 없으면, 스텝은 자신의 `when`을 그대로 유지합니다.
- 스텝에 자신의 `when`이 없으면, 임포트의 `when`을 그대로 상속합니다.
- 둘 다 있으면, 결과는 `(import-when) && (step-when)`입니다.

```yaml
phases:
  - name: gpu-setup
    imports:
      - path: gpu/setup.yaml
        when: "vars.gpu == true"   # applied (AND) to every step imported from gpu/setup.yaml
      - path: base.yaml
        # no when, steps from base.yaml keep their own conditions unchanged
```

정적 변수를 검사하려면 `vars.`을, 런타임 팩트를 검사하려면 `runtime.`을 사용하며, 이는 스텝 수준 `when`과 동일합니다.

최상위 `steps:`도 여전히 유효합니다. 실행 시 이 스텝들은 `default`라는 이름의 암시적 단계로 정규화됩니다.

## Parallel batches {#parallel-batches}

몇 개의 연속된 스텝을 함께 실행해도 안전할 때 `parallelGroup`을 사용합니다.

```yaml
version: v1alpha1
phases:
  - name: packages
    maxParallelism: 2
    steps:
      - id: download-ubuntu
        kind: DownloadPackage
        parallelGroup: distro-downloads
        spec:
          packages: [containerd]
          distro:
            family: debian
            release: ubuntu2204
          repo:
            type: deb-flat
          backend:
            mode: container
            runtime: docker
            image: ubuntu:22.04

      - id: download-rhel
        kind: DownloadPackage
        parallelGroup: distro-downloads
        spec:
          packages: [containerd]
          distro:
            family: rhel
            release: rhel9
          repo:
            type: rpm
          backend:
            mode: container
            runtime: docker
            image: rockylinux:9
```

첫 버전의 규칙:

- `parallelGroup` 값이 같은 연속된 스텝만 같은 배치에 속합니다
- 배치가 한 번 닫히면, 같은 `parallelGroup` 값은 그 단계에서 다시 나타날 수 없습니다
- 단계는 여전히 순서대로 실행됩니다
- apply 시점의 병렬 배치는 의도적으로 작은 허용 목록으로 제한됩니다: `Command`, `CopyFile`, `EnsureDirectory`, `ExtractArchive`, `WaitForCommand`, `WaitForFile`, `WaitForMissingFile`, `WaitForService`, `WaitForTCPPort`, `WaitForMissingTCPPort`, `WriteFile`
- 같은 배치의 apply 스텝은 동일한 리터럴 출력 경로나 노드 경로를 대상으로 삼을 수 없습니다
- 같은 배치의 prepare 스텝은 동일한 리터럴 준비 루트 경로(예: 동일한 `files/...`, `images/...`, `packages/...` 목적지)에 쓸 수 없습니다
- 같은 배치의 스텝은 `runtime.*`을 통해 서로의 `register` 출력을 소비할 수 없습니다
- 병렬 배치의 `register` 출력은 이후 배치나 이후 단계에만 보입니다

한 스텝이 다른 스텝의 런타임 출력을 소비해야 하는 워크플로라면, 그 스텝들을 별도의 배치나 별도의 단계에 두세요.

## Step kinds

타입 지정 스텝은 워크플로를 훑어보고, 검증하고, 발전시키기 쉽게 만듭니다. 지원되는 종류가 맞지 않을 때만 `Command`를 사용하세요.

공개 스텝 종류 레퍼런스는 워크플로 단계와 작업 지향 그룹으로 구성됩니다.

- `Host Prep`
- `Files and Archives`
- `Packages and Repositories`
- `Container Images`
- `Container Runtime`
- `Services and Systemd`
- `Kubernetes Lifecycle`
- `Waits and Polling`
- `Operator Interaction`
- `Advanced`

현재 단계/그룹 색인과 정확한 지원 종류 목록은 [Step Kinds](step-kinds.md)를 참고하세요.

## Prepare semantics {#prepare-semantics}

`prepare`는 `apply`와 동일한 스텝 문법을 사용하지만, 어떤 종류가 유효한지는 명령 맥락이 결정합니다.

- `DownloadFile`은 prepare 전용이며 `outputPath`는 표준 `files/` 루트 아래에 있어야 합니다
- `DownloadImage`는 prepare 전용이며 `outputDir`는 `images/`나 `images/...` 하위 디렉터리 아래에 있어야 합니다
- `DownloadPackage`는 prepare 전용이며 `outputDir`는 `packages/`나 `packages/...` 하위 디렉터리 아래에 있어야 합니다
- 이후 apply 스텝을 위해 안정적인 커스텀 위치가 필요하지 않다면 `outputPath`나 `outputDir`를 생략하세요
- 컨테이너 기반 `DownloadPackage`는 apt/dnf 패키지 관리자 캐시 디렉터리를 바인드 마운트하는 대신, 성공적으로 익스포트한 뒤 호스트가 소유한 익스포트 아티팩트 캐시를 재사용합니다
- `workflows/prepare.yaml`은 prepare 워크플로의 고정된 진입점입니다

## When to use Command

아직 맞는 스텝 종류가 없을 때 `Command`를 사용하세요. 이는 탈출구일 뿐 이상적인 작성 경로가 아닙니다. 워크플로가 `Command`에 크게 의존한다면, 그 절차는 여전히 원시 셸에 너무 가까울 수 있습니다.

## Validation model

`deck lint`는 다음을 검사합니다.

- 최상위 워크플로 스키마
- 참조된 각 스텝 종류의 스키마
- 예약된 런타임 키와 워크플로 호환성 규칙

전송 전에 검증할 수 있다는 점은 셸 파일을 주고받는 대신 워크플로 모델을 사용하는 주된 이유 중 하나입니다.

## Related references

- `core-concepts/why-deck.md`
- [Workspace Layout](workspace-layout.md#component-fragment-contract)
- `bundle-layout.md`
- `../../schemas/deck-workflow.schema.json`
<!-- END 윤문 -->
