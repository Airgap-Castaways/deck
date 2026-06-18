---
source: docs/guides/authoring-workflows.md
source_hash: 4aa76a5cdd6a9bf2ab915fd6a6baef333376bc3e
---
# 워크플로 작성하기

이 가이드는 deck 워크플로를 처음부터 작성하는 과정을 안내합니다 — `deck init`이
생성하는 파일에서 시작해 컴포넌트 프래그먼트를 임포트하고 호스트 유형에 따라
분기하는 다중 단계 시나리오까지 다룹니다.

## `deck init`에서 시작하기

`deck init`을 실행해 워크스페이스를 스캐폴딩합니다:

```bash
deck init --out ./my-workspace
cd ./my-workspace
```

다음과 같은 레이아웃이 생성됩니다:

```text
my-workspace/
├── workflows/
│   ├── prepare.yaml          # entry workflow for deck prepare
│   ├── vars.yaml             # shared variable defaults
│   ├── scenarios/
│   │   └── apply.yaml        # entry scenario for deck apply
│   └── components/
│       └── example-apply.yaml
└── outputs/
    ├── files/
    ├── packages/
    └── images/
```

**어떤 파일을 먼저 편집할까요?**

- `workflows/vars.yaml`부터 시작해 모든 시나리오가 읽을 공유 기본값을
  정의합니다 (Kubernetes 버전, 클러스터 IP, 역할 플래그).
- `workflows/scenarios/apply.yaml`을 편집해 대상 노드에서 실행되는 스텝을
  기술합니다.
- 여러 시나리오에서 스텝 그룹을 재사용하려면 `workflows/components/` 아래에
  스텝 프래그먼트를 추가합니다.
- 오프라인 전달을 위해 아티팩트(바이너리, 패키지, 이미지)를 스테이징해야 할
  때만 `workflows/prepare.yaml`을 편집합니다.

## 시나리오의 구조

시나리오는 `workflows/scenarios/` 아래에 위치한 완전한 워크플로 파일입니다.
`version`과 `steps` 또는 `phases` 중 최소 하나를 반드시 포함해야 합니다.

### 최소 주석 예시

```yaml
# workflows/scenarios/apply.yaml

version: v1alpha1          # required; currently always v1alpha1

vars:                      # optional; scenario-level overrides for vars.yaml values
  clusterName: k8s-lab

steps:
  - id: write-motd         # required; stable, unique step identifier
    kind: WriteFile        # required; typed step kind (see step-kinds index)
    # apiVersion: deck/v1alpha1   # optional; deck infers it from version above
    spec:                  # required; kind-specific payload
      path: /etc/motd
      content: |
        deck maintenance session in progress — {{ .vars.clusterName }}
    when: ""               # optional; CEL expression — step is skipped when false
    register: {}           # optional; export step outputs as runtime.NAME
    timeout: 30s           # optional; duration string
    metadata:              # optional; free-form annotation map
      owner: platform-team
```

### 스텝 엔벨로프

모든 스텝은 종류에 관계없이 동일한 외부 엔벨로프를 공유합니다:

| 필드 | 필수 | 목적 |
|---|---|---|
| `id` | 예 | 안정적인 스텝 식별자; 워크플로 전체에서 고유해야 함 |
| `kind` | 예 | 타입이 지정된 스텝 이름 (`WriteFile`, `InstallPackage`, …) |
| `spec` | 예 | 스텝 종류별 페이로드 |
| `apiVersion` | 아니오 | 생략 시 deck가 `version`에서 추론 |
| `when` | 아니오 | CEL 표현식; `false`로 평가되면 스텝을 건너뜀 (오류 아님) |
| `register` | 아니오 | 런타임 변수 이름을 스텝 출력 키에 매핑 |
| `timeout` | 아니오 | `30s` 또는 `5m` 같은 기간 문자열 |
| `retry` | 아니오 | 실패 시 재시도 횟수 |
| `parallelGroup` | 아니오 | 동일한 값을 가진 연속 스텝이 한 배치로 실행됨 |
| `metadata` | 아니오 | 툴링이나 감사 컨텍스트를 위한 자유 형식 어노테이션 맵 |

## `Command`보다 타입 지정 스텝 선택하기

`deck`는 일반적인 작업을 위한 목적 특화 스텝 종류를 제공합니다. 다음 이유로
`Command`보다 이들을 선호하세요:

- 실행 전에 스키마에 대해 검증됩니다.
- `deck plan` 출력에서 의도를 명확하게 표현합니다.
- 인라인 셸보다 린트와 리뷰가 쉽습니다.

**타입 지정 스텝을 사용해야 하는 경우:**

| 하려는 작업 … | 사용 |
|---|---|
| 파일 쓰기 | `WriteFile` |
| 준비된 아티팩트 복사 | `CopyFile` |
| 패키지 설치 | `InstallPackage` / `InstallAptPackage` / `InstallDnfPackage` |
| systemd 서비스 설정 | `WriteSystemdUnit` + `ManageService` |
| Kubernetes 컨트롤 플레인 부트스트랩 | `InitKubeadm` |
| 조건 대기 | `WaitForService`, `WaitForCommand`, `WaitForTCPPort`, … |
| 호스트 적합성 확인 | `CheckHost` |

**`Command`를 사용해야 하는 경우:**

해당 작업을 모델링하는 타입 지정 스텝이 없을 때 — 예를 들어 벤더 특화 CLI
호출이나 deck가 아직 직접 지원하지 않는 일회성 프로브. `Command`는 기본
선택지가 아니라 탈출구입니다.

단계와 그룹별로 정리된 전체 색인은 [스텝 종류](../step-kinds.md)를 참고하세요.

## 다중 단계 시나리오 구조화하기

절차에 자연스러운 경계가 있을 때 `phases`를 사용합니다. 단계는 `deck apply`의
재개 경계이기도 합니다 — 실행이 중단되면 마지막으로 완료되지 않은 단계부터
재개됩니다.

### 컴포넌트 프래그먼트 임포트하기

컴포넌트는 `workflows/components/` 아래에 위치합니다. 이들은 `steps:` 목록만
포함합니다. `phases[].imports`를 사용해 단계로 임포트합니다. 경로는
`workflows/components/`를 기준으로 합니다 — `../components/k8s/prereq.yaml`이
아니라 `k8s/prereq.yaml`로 작성하세요.

```yaml
# workflows/scenarios/bootstrap.yaml

version: v1alpha1
phases:
  - name: host-prereqs          # human-readable phase label
    imports:
      - path: host-prereqs.yaml         # resolves from workflows/components/
      - path: repo/offline-repo.yaml

  - name: runtime
    imports:
      - path: runtime/containerd.yaml
      - path: runtime/kubelet.yaml

  - name: verify
    steps:
      - id: check-cluster-ready         # inline step alongside imports
        kind: CheckKubernetesCluster
        spec:
          timeout: 10m
          interval: 10s
          nodes:
            total: 1
          kubeSystem:
            readyPrefixes:
              - etcd-
              - kube-apiserver-
              - kube-controller-manager-
              - kube-scheduler-
```

**컴포넌트 프래그먼트** (`workflows/components/host-prereqs.yaml`):

```yaml
steps:
  - id: disable-swap
    kind: Swap
    spec:
      disable: true
      persist: true

  - id: load-overlay-module
    kind: KernelModule
    spec:
      names: [overlay, br_netfilter]
      load: true
      persist: true
      persistFile: /etc/modules-load.d/kubernetes.conf
```

컴포넌트 프래그먼트에는 `version`이나 `vars`가 없습니다 — 이들은 임포트하는
시나리오에 속합니다.

## `when`으로 분기하기

해당되지 않는 호스트에서 스텝을 건너뛰려면 `when` 필드를 사용합니다. 값은 CEL
표현식입니다. `when`이 `false`로 평가되면 스텝을 건너뜁니다 (오류 아님).

### OS 계열 분기

```yaml
steps:
  - id: configure-apt-repo
    kind: ConfigureRepository
    spec:
      format: deb
      repositories:
        - id: offline-base
          baseurl: file:///srv/offline-repo
          trusted: true
    when: runtime.host.os.family == "debian"   # skipped on RHEL nodes

  - id: configure-yum-repo
    kind: ConfigureRepository
    spec:
      format: rpm
      repositories:
        - id: offline-base
          name: offline-base
          baseurl: file:///srv/offline-repo
          enabled: true
          gpgcheck: false
    when: runtime.host.os.family == "rhel"     # skipped on Debian nodes
```

### 역할 기반 분기

`vars.yaml`이 `hosts:`를 사용해 노드별 `role`을 할당하는 경우, `when`을
사용하면 컨트롤 플레인 또는 워커 스텝만 대상으로 지정할 수 있습니다:

```yaml
  - id: kubeadm-init
    kind: InitKubeadm
    spec:
      outputJoinFile: "{{ .vars.join.file }}"
    when: vars.role == "control-plane"

  - id: join-worker
    kind: JoinKubeadm
    spec:
      joinFile: "{{ .vars.join.file }}"
    when: vars.role == "worker"
```

조건부 임포트와 register로 게이팅된 조건을 포함한 더 많은 패턴은
[when으로 조건 지정하기 (CEL)](conditions-and-cel.md)를 참고하세요.

## 커밋하기 전에 검증하기

워크플로를 패키징하거나 전송하기 전에 항상 린트하세요:

```bash
# lint the default scenario
deck lint

# lint a specific workflow file
deck lint --workflow ./workflows/scenarios/bootstrap.yaml
```

`deck lint`는 다음을 확인합니다:

- 최상위 워크플로 스키마 (`version`, `phases`/`steps` 상호 배타성, 필수 필드)
- 각 타입 지정 스텝 종류의 스키마
- 예약된 런타임 키 및 워크플로 호환성 규칙

**아무것도 실행하지 않고 실행 순서 미리보기:**

```bash
deck plan
deck plan vars   # show the fully-resolved variable snapshot
```

`deck plan`은 실행될 단계와 스텝을 출력하며, 가능한 경우 조건을 평가합니다.
`deck plan vars`는 유효한 `vars`, 해석된 `context`, 초기 `runtime` 값을
출력합니다 — `apply`를 실행하기 전에 노드 범위 vars가 올바르게 해석되었는지
확인하는 데 유용합니다.

## 관련 참고 자료

- [워크플로 모델](../workflow-model.md) — 권위 있는 스키마 및 필드 참조
- [스텝 종류](../step-kinds.md) — 단계와 그룹별 전체 색인
- [변수와 템플릿](variables-and-templating.md) — vars 우선순위 및 `{{ .vars.NAME }}` 문법
- [when으로 조건 지정하기 (CEL)](conditions-and-cel.md) — 전체 `when` 참조
- [워크스페이스 레이아웃](../workspace-layout.md) — 컴포넌트와 시나리오를 위한 디렉터리 계약
