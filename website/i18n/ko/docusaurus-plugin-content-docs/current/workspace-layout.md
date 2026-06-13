---
source: docs/workspace-layout.md
source_hash: 063484b95e8b5763ac3c4edfb39dc6397a55f968
---
# 워크스페이스 레이아웃

이 문서는 `deck` 프로젝트의 표준 디렉터리 구조를 설명합니다. `deck init`을 사용하면 이 레이아웃을 자동으로 생성할 수 있습니다.

`deck` 워크스페이스는 세 가지 주요 기능 영역으로 구성됩니다: **워크플로**, **준비된 출력물(Prepared Outputs)**, **메타데이터**.

## 디렉터리 구조

```text
.
├── workflows/
│   ├── prepare.yaml    # Prepare entry workflow
│   ├── scenarios/      # Apply scenario entry workflows
│   ├── components/     # Reusable step fragments (Component Fragments)
│   └── vars.yaml       # Shared variable definitions
├── outputs/            # Prepared artifacts and runtime binaries
│   ├── files/          # Prepared file payloads
│   ├── packages/       # Prepared package payloads
│   ├── images/         # Prepared image payloads
│   └── bin/            # Prepared runtime binaries by os/arch
└── .deck/              # (Internal) Checksums and manifest
```

## 워크플로 (`workflows/`)

`workflows/` 디렉터리에는 모든 운영 로직이 들어 있습니다.

### Prepare 진입점 (`workflows/prepare.yaml`)
`workflows/prepare.yaml`은 `deck prepare`의 고정 진입 워크플로입니다.

### 시나리오 (`workflows/scenarios/`)
시나리오는 `deck apply`의 주요 진입점입니다.
- 여기에 있는 각 파일은 `version`과 함께 `steps` 또는 `phases` 중 하나를 갖춘 완전한 워크플로여야 합니다.
- 일반적인 파일명: `apply.yaml`, `bootstrap.yaml`, `worker-join.yaml`.

### 컴포넌트 (`workflows/components/`)
컴포넌트는 **컴포넌트 프래그먼트**입니다—시나리오로 가져올 수 있는 재사용 가능한 스텝 집합입니다.
- 여기에 있는 파일은 **Component Fragment Schema**를 따릅니다.
- `steps:` 목록만 포함합니다.
- 시나리오의 `phases[].imports`를 통해 가져옵니다.
- **예시**: `workflows/components/k8s/runtime.yaml`은 `k8s/runtime.yaml`로 가져옵니다.

<!-- BEGIN GENERATED:COMPONENT_FRAGMENT_CONTRACT -->
#### Component Fragment Contract

`workflows/components/` 아래에 위치한 재사용 가능한 워크플로 컴포넌트 프래그먼트에 대한 참조입니다.

- schema: `../schemas/deck-component-fragment.schema.json`

##### 예시

```yaml
steps:
  - id: write-config
    kind: WriteFile
    spec:
      path: /etc/example.conf
      content: hello
  - id: restart-service
    kind: ManageService
    spec:
      name: example
      state: restarted
```

##### 필드

| Key | Type | Required | Default | Enum | Description | Example |
|---|---|---:|---|---|---|---|
| `steps` | `array<object>` | yes | `` | `` | 이 프래그먼트에 포함된 워크플로 스텝의 순서가 있는 목록. | `[{id:write-config,kind:WriteFile,spec:{path:/etc/example.conf,content:hello}}]` |

##### 참고

- 컴포넌트 프래그먼트는 워크스페이스의 `workflows/components/` 디렉터리에 저장됩니다.
- `steps:` 목록만 포함하며, 전체 시나리오에 비해 제한된 스키마를 따릅니다.
- 프래그먼트는 `phases[].imports`를 사용해 시나리오 단계로 가져옵니다.
- 함께 제공되는 워크스페이스 레이아웃 문서는 컴포넌트 프래그먼트가 표준 프로젝트 구조에 어떻게 들어맞는지 설명합니다.
<!-- END GENERATED:COMPONENT_FRAGMENT_CONTRACT -->

### 변수 (`workflows/vars.yaml`)
공유 기본값을 담는 중앙 YAML 파일입니다. 여기에 정의된 값은 `{{ .vars.NAME }}` 구문을 통해 모든 워크플로와 컴포넌트에서 사용할 수 있습니다.

노드별 실행을 위해 `vars.yaml`은 `all:` 기본값과 로컬 호스트명으로 선택되는 `hosts:` 오버레이를 포함할 수 있습니다. 우선순위 및 호스트명 매칭에 대한 자세한 내용은 [Workflow Model](workflow-model.md#variables)을 참조하세요.

## 준비된 출력물 (`outputs/`)

이 디렉터리에는 `apply`가 소비하는 준비된 소스 자료가 들어 있습니다.
- `deck prepare` 중에 아티팩트는 워크플로에 선언된 로컬 또는 원격 소스에서 수집되어 정규(canonical) `outputs/files/`, `outputs/packages/`, 또는 `outputs/images/` 루트에 배치됩니다.
- 오프라인 실행을 위한 런타임 바이너리는 `outputs/bin/<os>/<arch>/deck` 아래에 기록되며, 워크스페이스 루트의 `deck` 파일은 런처 스크립트입니다.
- 일단 번들링되면 이 소스는 더 이상 필요하지 않으며, 대상 노드는 최종 번들 콘텐츠만 보게 됩니다.

## 내부 메타데이터 (`.deck/`)

이 디렉터리는 `deck`이 관리하므로 수동으로 편집해서는 안 됩니다.
- `.deck/manifest.json` — 정규 준비된 출력물(`outputs/{files,packages,images,bin}`)의 다이제스트 매니페스트. `workflows/`와 deck 런처 자체는 추적되지 않습니다. `bundle verify`와 apply 시작 시 무결성 기준선으로 사용됩니다.
- `.deck/state/apply/` — 단계 기반 apply 상태 (see [apply-state.md](apply-state.md)). Apply 실행 로그(record.json / events.jsonl)는 워크스페이스 내부가 아니라 `$XDG_STATE_HOME/deck/runs/<run-id>/`에 기록됩니다 (see [apply-runlogs.md](apply-runlogs.md)).

## 관련 참조

- [Workflow Model](workflow-model.md)
- [Bundle Layout](bundle-layout.md)
- [Component Fragment Contract](#component-fragment-contract)
