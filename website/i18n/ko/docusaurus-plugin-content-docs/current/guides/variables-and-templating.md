---
source: docs/guides/variables-and-templating.md
source_hash: 9a9bc9ed49b247e226033f4118a1d28d0748a52f
---
# 변수와 템플릿

이 가이드는 deck가 변수를 어떻게 해석하는지, 변수를 여러 파일과 CLI 플래그에 걸쳐 어떻게 구성하는지, 워크플로 스텝 안에서 어떻게 안전하게 사용하는지 설명합니다.

## 네 가지 변수 소스와 우선순위

정적 `vars`는 네 가지 소스에서 옵니다. **나중 소스가 이전 소스를 이깁니다.**
우선순위가 낮은 것부터 높은 것 순서는 다음과 같습니다:

1. `workflows/vars.yaml` — 공유 워크스페이스 기본값
2. CLI `-f, --vars-file` 오버레이 — 실행 시 전달되는 사이트 또는 노드 파일
3. 시나리오 `vars:` 블록 — 워크플로 파일 안에 선언된 재정의
4. CLI `--var key=value` — 호출별 재정의

**이 순서가 중요한 이유:**

`--var`로 설정한 값은 항상 `vars.yaml`이나 시나리오 파일의 값을 이깁니다.
이는 다음을 의미합니다:

- 클러스터 전역에서 안정적인 기본값은 `vars.yaml`에 둡니다.
- `-f`를 사용해 `vars.yaml`을 편집하지 않고도 사이트별 또는 환경별 파일을
  그 기본값 위에 레이어로 쌓습니다.
- 시나리오 `vars:` 블록을 사용해 공유 상태를 건드리지 않고 특정 시나리오
  하나에 맞게 값을 조정합니다.
- `--var`는 디버깅이나 단계적 롤아웃 중의 일회성 재정의에 사용합니다.

`workflows/vars.yaml`에 `hosts:` 섹션이 포함된 경우, deck는 위의 네 소스
이후에 추가 선택 단계를 적용합니다. 전체 순서는 아래
[노드 범위 vars](#node-scoped-vars)를 참조하세요.

## `workflows/vars.yaml` — 공유 워크스페이스 기본값

워크스페이스의 모든 시나리오가 공유하는 값을 정의합니다:

```yaml
# workflows/vars.yaml

kubernetesVersion: v1.36.1
arch: amd64

cluster:
  podCIDR: 10.244.0.0/16
  serviceCIDR: 10.96.0.0/12
  controlPlaneEndpoint: 192.0.2.10:6443

registry:
  host: 192.0.2.10:5000
  scheme: http
```

모든 시나리오와 컴포넌트 프래그먼트는 `{{ .vars.kubernetesVersion }}`,
`{{ .vars.cluster.podCIDR }}` 등으로 이 값들을 참조할 수 있습니다.

## 시나리오 수준 `vars:` 블록

특정 시나리오 하나에 대해 공유 기본값을 재정의하거나 확장합니다:

```yaml
# workflows/scenarios/staging-bootstrap.yaml

version: v1alpha1
vars:
  kubernetesVersion: v1.36.1  # pin to a specific patch for staging
  cluster:
    controlPlaneEndpoint: 192.0.2.20:6443   # staging endpoint

phases:
  - name: bootstrap
    imports:
      - path: bootstrap/kubeadm.yaml
```

시나리오 `vars:` 블록은 `vars.yaml` 값 위에 깊은 병합(deep-merge)되므로,
재정의하려는 키만 나열하면 됩니다. 다른 모든 `vars.yaml` 키는 그대로 사용
가능합니다.

## CLI vars 파일 — `-f, --vars-file`

`vars.yaml`을 편집하지 않고 실행 시점에 사이트별 또는 환경별 값을 레이어로
쌓습니다:

```bash
# Apply site overrides first, then node-specific overrides on top
deck apply --root . --scenario bootstrap -f vars/site.yaml -f vars/cp1.yaml
```

파일 경로는 `workflows/` 디렉터리를 기준으로 합니다:
- `-f vars/site.yaml`은 `workflows/vars/site.yaml`로 해석됩니다.
- 목록에서 나중에 오는 파일이 이전 파일을 재정의합니다(`vars.yaml`과 동일한
  깊은 병합 동작).

```yaml
# workflows/vars/site.yaml
registry:
  host: 10.0.1.5:5000
```

```yaml
# workflows/vars/cp1.yaml
cluster:
  controlPlaneEndpoint: 10.0.1.5:6443
```

## CLI `--var` — 단일 키 재정의

파일을 만들지 않고 키 하나를 재정의합니다:

```bash
deck apply --root . --scenario bootstrap --var kubernetesVersion=v1.36.1
```

`--var`는 가장 높은 우선순위를 가지며 일회성 디버깅이나 단계적 롤아웃을 위한
것이지 영구적인 구성을 위한 것이 아닙니다.

## 노드 범위 vars {#node-scoped-vars}

`vars.yaml`에 `hosts:` 섹션이 포함된 경우, deck는 실행 시점에 로컬 호스트명과
일치하는 항목을 선택하여 유효 vars에 깊은 병합합니다. 이를 통해 단일
`vars.yaml`이 클러스터의 모든 노드에 대한 노드별 구성을 담을 수 있습니다.

### `hosts:` 사용 시 전체 우선순위

`hosts:`가 있을 때 유효 우선순위(낮은 것부터 높은 것)는 다음과 같습니다:

1. 일반 최상위 `vars.yaml` 값(`all:`과 `hosts:` 외부의 키)
2. `vars.yaml`의 `all:` 기본값
3. 일치하는 `vars.yaml`의 `hosts.<hostname>` 항목
4. 시나리오 `vars:` 블록
5. CLI `--var` 재정의

(`-f` 오버레이는 위 1단계 이전에 공유 vars 문서에 병합되므로, 1–3단계에
도달하는 값에 영향을 줍니다.)

### 예시

```yaml
# workflows/vars.yaml

all:
  kubernetesVersion: v1.36.1
  arch: amd64
  role: ""               # safe default; prevent CEL errors on unmatched hosts

hosts:
  cp-1:
    ip: 192.0.2.10
    role: control-plane
    deckServer: true

  worker-1:
    ip: 192.0.2.21
    role: worker
    deckServer: false
```

`deck apply`가 호스트명이 `cp-1`인 머신에서 실행될 때:

1. 최상위 `vars.yaml` 값이 로드됩니다.
2. `all:` 기본값이 적용됩니다(`kubernetesVersion`, `arch`, `role: ""`).
3. `cp-1` 호스트 항목이 병합되어 `role`은 `"control-plane"`이 되고
   `ip`는 `192.0.2.10`이 됩니다.
4. 시나리오 `vars:` 블록(있는 경우)이 그 위에 병합됩니다.
5. 모든 `--var` 플래그가 마지막에 적용됩니다.

로컬 호스트명이 `hosts:`에 나타나지 않으면, 실행은 `all:` 값만 사용하여
계속됩니다. 워크플로가 분기 조건으로 삼는 모든 필드에 대해 `all:`에 안전한
기본값을 제공하세요 — 예를 들어 `role: ""` — 일치하지 않는 호스트에서 CEL
평가 오류를 방지하기 위함입니다.

호스트명 매칭은 먼저 탐지된 전체 호스트명을 시도하고, 그다음 짧은
호스트명(첫 번째 `.` 앞부분)을 시도합니다.

## 스텝에서 변수 사용하기: `{{ .vars.NAME }}`

스텝 `spec` 문자열 필드 안에서는 이중 중괄호와 함께 Go 템플릿 구문을
사용합니다:

```yaml
- id: write-cluster-config
  kind: WriteFile
  spec:
    path: /etc/kubernetes/cluster.conf
    content: |
      clusterName: {{ .vars.cluster.controlPlaneEndpoint }}
      kubernetesVersion: {{ .vars.kubernetesVersion }}
      podCIDR: {{ .vars.cluster.podCIDR }}
```

중첩된 YAML 키는 점 표기법을 사용합니다: `{{ .vars.cluster.podCIDR }}`는
`cluster` 맵 안의 `podCIDR` 필드를 읽습니다.

템플릿 보간은 `spec` 문자열 필드에서 사용할 수 있습니다. 등록된 런타임
출력을 참조하려면 `.runtime.NAME`을 사용합니다(
[register](../workflow-model.md#register--capture-step-output) 참조).

### `when:`의 CEL 표현식은 다른 구문을 사용합니다

`when:` 조건에서는 중괄호 없이 CEL 네임스페이스를 사용합니다:

```yaml
when: vars.role == "control-plane"         # CEL — no braces
```

다음은 틀립니다:

```yaml
when: "{{ .vars.role }} == control-plane"  # wrong — this is template syntax
```

전체 레퍼런스는 [when을 사용한 조건 (CEL)](conditions-and-cel.md)을
참조하세요.

## 해석된 vars 검사하기: `deck plan vars`

`prepare`나 `apply`를 실행하기 전에 유효한 변수 스냅샷을 확인하세요:

```bash
deck plan vars
```

이 명령은 다음을 출력합니다:

- 완전히 병합된 `vars` 맵(우선순위를 통과해 살아남은 값을 보여줌).
- 해석된 `context` 필드(명령, 워크플로 소스, 경로).
- 실행 전에 알려진 초기 `runtime` 값(로컬 OS에서 탐지된 `runtime.host`
  팩트 포함).
- 실행 중 스텝이 등록할 계획된 런타임 키(값 자체는 예측되지 않음).

우선순위 문제를 디버깅하거나 노드 범위 호스트 항목이 올바르게 해석되었는지
확인할 때 언제든 `deck plan vars`를 사용하세요.

## 흔한 실수

### 잘못된 우선순위 가정

**실수:** 시나리오 `vars:` 블록에 설정된 값을 재정의하려고 `vars.yaml`을
편집하면서 `vars.yaml`이 이길 것이라 기대하는 것.

**해결:** 시나리오 `vars:`가 `vars.yaml`을 이긴다는 점을 기억하세요. 모든
시나리오에 걸쳐 값을 공유하려면 `vars.yaml`에 두고 시나리오 블록에서는
생략하세요.

### 일치하지 않는 호스트에 대한 `all:` 기본값 누락

**실수:** `all: role: ""` 기본값 없이 `vars.role`로 분기하는 것.
로컬 호스트명이 어떤 `hosts:` 항목과도 일치하지 않으면 `vars.role`은
정의되지 않고 CEL 표현식이 `E_CONDITION_EVAL`을 발생시킵니다.

**해결:** `all:`에 안전한 기본값을 추가하세요:

```yaml
all:
  role: ""   # safe default prevents CEL errors on unmatched hosts
```

### 단일 중괄호 템플릿 구문

**실수:** `{{ .vars.name }}` 대신 `{vars.name}`이나 `{ .vars.name }`을
작성하는 것.

**해결:** 항상 이중 중괄호를 사용하세요. 단일 중괄호 구문은 스텝 검증 중에
`E_TEMPLATE_SINGLE_BRACE`를 발생시킵니다.

```yaml
# wrong
content: "cluster: {vars.clusterName}"

# correct
content: "cluster: {{ .vars.clusterName }}"
```

## 관련 레퍼런스

- [워크플로 모델 — 변수](../workflow-model.md#variables) — 정식 우선순위 규칙과 노드 범위 vars
- [워크플로 작성하기](authoring-workflows.md) — 시나리오를 처음부터 구성하는 방법
- [when을 사용한 조건 (CEL)](conditions-and-cel.md) — CEL `when:` 표현식에서 vars 사용하기
- [트러블슈팅](../troubleshooting.md) — 변수 해석 오류 진단하기
