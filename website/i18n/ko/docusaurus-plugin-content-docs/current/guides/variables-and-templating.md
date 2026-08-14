---
source: docs/guides/variables-and-templating.md
source_hash: 48f8b107b6488084084ef251ac5181d2f8f5f0b4
---
# 변수와 템플릿화

이 가이드에서는 deck이 변수를 해석하는 방식, 여러 파일과 CLI 플래그에 걸쳐 변수를 구성하는 방법, 그리고 워크플로 스텝 안에서 변수를 안전하게 사용하는 방법을 설명합니다.

## 네 가지 변수 소스와 우선순위

정적 `vars`는 네 가지 소스에서 옵니다. **뒤쪽 소스가 앞쪽 소스를 덮어씁니다.** 우선순위가 낮은 것에서 높은 것 순서는 다음과 같습니다.

1. `workflows/vars.yaml` — 워크스페이스 공통 기본값
2. CLI `-f, --vars-file` 오버레이 — 실행 시점에 전달하는 사이트별 또는 노드별 파일
3. 시나리오 `vars:` 블록 — 워크플로 파일 안에서 선언하는 재정의
4. CLI `--var key=value` — 호출 단위 재정의

**이 순서가 중요한 이유는 다음과 같습니다.**

`--var`로 설정한 값은 `vars.yaml`이나 시나리오 파일의 값을 항상 이깁니다. 이를 활용하는 방법은 다음과 같습니다.

- 클러스터 전체에 적용할 안정적인 기본값은 `vars.yaml`에 둡니다.
- `vars.yaml`을 수정하지 않고도 `-f`로 사이트별 또는 환경별 파일을 기본값 위에 얹습니다.
- 특정 시나리오 하나만 조정할 때는 공유 상태를 건드리지 않고 시나리오 `vars:` 블록을 사용합니다.
- 디버깅이나 단계적 롤아웃 중의 일회성 재정의에는 `--var`를 사용합니다.

`workflows/vars.yaml`에 `hosts:` 섹션이 있으면 deck은 위 네 소스에 더해 추가 선택 단계를 적용합니다. 전체 순서는 아래 [노드 스코프 변수](#node-scoped-vars)를 참고하십시오.

## `workflows/vars.yaml` — 워크스페이스 공통 기본값

워크스페이스의 모든 시나리오가 공유하는 값을 정의합니다.

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

모든 시나리오와 컴포넌트 프래그먼트에서 `{{ .vars.kubernetesVersion }}`, `{{ .vars.cluster.podCIDR }}` 같은 표현으로 이 값들을 참조할 수 있습니다.

## 시나리오 수준 `vars:` 블록

특정 시나리오 하나에 한해 공유 기본값을 재정의하거나 확장합니다.

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

시나리오 `vars:` 블록은 `vars.yaml` 값 위에 깊은 병합(deep-merge)됩니다. 따라서 재정의할 키만 나열하면 되고, 나머지 `vars.yaml` 키는 그대로 사용됩니다.

## CLI 변수 파일 — `-f, --vars-file`

`vars.yaml`을 수정하지 않고 실행 시점에 사이트별 또는 환경별 값을 얹습니다.

```bash
# Apply site overrides first, then node-specific overrides on top
deck apply --root . --scenario bootstrap -f vars/site.yaml -f vars/cp1.yaml
```

파일 경로는 `workflows/` 디렉터리를 기준으로 합니다.
- `-f vars/site.yaml`은 `workflows/vars/site.yaml`로 해석됩니다.
- 목록에서 뒤에 오는 파일이 앞선 파일을 재정의합니다(`vars.yaml`과 동일한 깊은 병합 동작).

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

파일을 만들지 않고 키 하나만 재정의합니다.

```bash
deck apply --root . --scenario bootstrap --var kubernetesVersion=v1.36.1
```

`--var`는 우선순위가 가장 높으며, 영구 설정이 아니라 일회성 디버깅이나 단계적 롤아웃을 위한 용도입니다.

## 노드 스코프 변수 {#node-scoped-vars}

`vars.yaml`에 `hosts:` 섹션이 있으면 deck은 실행 시점에 로컬 호스트명과 일치하는 항목을 골라 유효 변수에 깊은 병합합니다. 이렇게 하면 단일 `vars.yaml` 하나로 클러스터의 모든 노드에 대한 노드별 설정을 담을 수 있습니다.

### `hosts:`를 포함한 전체 우선순위

`hosts:`가 있을 때 유효 우선순위는 낮은 것에서 높은 것 순으로 다음과 같습니다.

1. 일반 최상위 `vars.yaml` 값(`all:`과 `hosts:` 바깥의 키)
2. `vars.yaml`의 `all:` 기본값
3. 일치하는 `vars.yaml`의 `hosts.<hostname>` 항목
4. 시나리오 `vars:` 블록
5. CLI `--var` 재정의

(`-f` 오버레이는 위 1단계보다 먼저 공유 변수 문서에 병합되므로, 1~3단계에 도달하는 값에 영향을 줍니다.)

### 예제

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

호스트명이 `cp-1`인 머신에서 `deck apply`를 실행하면 다음 순서로 처리됩니다.

1. 최상위 `vars.yaml` 값을 로드합니다.
2. `all:` 기본값을 적용합니다(`kubernetesVersion`, `arch`, `role: ""`).
3. `cp-1` 호스트 항목을 병합하여 `role`은 `"control-plane"`, `ip`는 `192.0.2.10`이 됩니다.
4. 시나리오 `vars:` 블록이 있으면 그 위에 병합합니다.
5. `--var` 플래그를 마지막으로 적용합니다.

로컬 호스트명이 `hosts:`에 없으면 `all:` 값만으로 실행을 이어 갑니다. 워크플로가 분기 기준으로 삼는 필드에는 `all:`에 안전한 기본값을 두십시오. 예를 들어 `role: ""`을 지정하면 일치하지 않는 호스트에서 CEL 평가 오류를 방지할 수 있습니다.

호스트명 매칭은 먼저 감지된 전체 호스트명을 시도하고, 그다음 짧은 호스트명(첫 `.` 앞부분)을 시도합니다.

## 스텝에서 변수 사용하기: `{{ .vars.NAME }}`

스텝 `spec`의 문자열 필드 안에서는 이중 중괄호를 쓰는 Go 템플릿 문법을 사용합니다.

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

중첩된 YAML 키는 점 표기법으로 접근합니다. `{{ .vars.cluster.podCIDR }}`은 `cluster` 맵 안의 `podCIDR` 필드를 읽습니다.

템플릿 보간은 `spec` 문자열 필드에서 사용할 수 있습니다. register된 런타임 출력을 참조하려면 `.runtime.NAME`을 사용합니다([register](../workflow-model.md#register--capture-step-output) 참고).

### `when:`의 CEL 표현식은 문법이 다릅니다

`when:` 조건에서는 중괄호 없이 CEL 네임스페이스를 사용합니다.

```yaml
when: vars.role == "control-plane"         # CEL — no braces
```

다음과 같이 쓰면 안 됩니다.

```yaml
when: "{{ .vars.role }} == control-plane"  # wrong — this is template syntax
```

전체 레퍼런스는 [when 조건 (CEL)](conditions-and-cel.md)을 참고하십시오.

## 해석된 변수 확인하기: `deck plan vars`

`prepare`나 `apply`를 실행하기 전에 유효 변수 스냅숏을 확인하십시오.

```bash
deck plan vars
```

이 명령은 다음을 출력합니다.

- 완전히 병합된 `vars` 맵(우선순위를 거쳐 살아남은 값을 보여 줍니다).
- 해석된 `context` 필드(명령, 워크플로 소스, 경로).
- 실행 전에 알 수 있는 초기 `runtime` 값(로컬 OS에서 감지한 `runtime.host` 팩트 포함).
- 실행 중 스텝이 register할 런타임 키(값 자체는 예측하지 않습니다).

우선순위 문제를 디버깅하거나 노드 스코프 호스트 항목이 제대로 해석되었는지 확인할 때는 언제든 `deck plan vars`를 사용하십시오.

## 흔한 실수

### 우선순위에 대한 잘못된 가정

**실수:** 시나리오 `vars:` 블록에서 설정한 값을 재정의하려고 `vars.yaml`을 수정하면서 `vars.yaml`이 이기리라 기대하는 경우입니다.

**해결:** 시나리오 `vars:`가 `vars.yaml`을 이긴다는 점을 기억하십시오. 모든 시나리오에 값을 공유하려면 그 값을 `vars.yaml`에 두고 시나리오 블록에서는 생략하십시오.

### 일치하지 않는 호스트를 위한 `all:` 기본값 누락

**실수:** `all: role: ""` 기본값 없이 `vars.role`로 분기하는 경우입니다. 로컬 호스트명이 어떤 `hosts:` 항목과도 일치하지 않으면 `vars.role`이 정의되지 않아 CEL 표현식이 `E_CONDITION_EVAL`을 일으킵니다.

**해결:** `all:`에 안전한 기본값을 추가하십시오.

```yaml
all:
  role: ""   # safe default prevents CEL errors on unmatched hosts
```

### 단일 중괄호 템플릿 문법

**실수:** `{{ .vars.name }}` 대신 `{vars.name}`이나 `{ .vars.name }`으로 쓰는 경우입니다.

**해결:** 항상 이중 중괄호를 사용하십시오. 단일 중괄호 문법은 스텝 검증 중 `E_TEMPLATE_SINGLE_BRACE`를 일으킵니다.

```yaml
# wrong
content: "cluster: {vars.clusterName}"

# correct
content: "cluster: {{ .vars.clusterName }}"
```

## 관련 레퍼런스

- [워크플로 모델 — 변수](../workflow-model.md#variables) — 우선순위 규칙과 노드 스코프 변수의 표준 정의
- [워크플로 작성하기](authoring-workflows.md) — 시나리오를 처음부터 구성하는 방법
- [when 조건 (CEL)](conditions-and-cel.md) — CEL `when:` 표현식에서 변수 사용하기
- [문제 해결](../troubleshooting.md) — 변수 해석 오류 진단하기
