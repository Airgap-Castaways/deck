---
source: docs/guides/conditions-and-cel.md
source_hash: 2cb24974f76ee7d2c227a1131a769eddeb5f78c2
---
# `when`을 사용한 조건 (CEL)

이 가이드는 특정 호스트에서 어떤 스텝이 실행될지 제어하기 위해 `when:`
필드를 사용하는 방법과, 사용 가능한 네임스페이스를 활용해 올바른 CEL
표현식을 작성하는 방법을 설명합니다.

## `when:`이 하는 일

워크플로의 모든 스텝은 선택적인 `when:` 필드를 받습니다. 이 값은 deck이
스텝 실행 전에 평가하는 [CEL](https://cel.dev) 표현식입니다:

- `when:`이 `true`로 평가되거나 (또는 없을 경우), 스텝은 정상적으로
  실행됩니다.
- `when:`이 `false`로 평가되면, 스텝은 **건너뜁니다**: 이는 오류로
  취급되지 않으며, 다음 스텝으로 실행이 계속됩니다.
- `when:`이 평가에 실패하면 (예를 들어, 참조된 변수의 타입이 잘못된
  경우), deck은 `E_CONDITION_EVAL`을 보고하고 중단합니다.

`when:`은 정당한 선택성을 표현하는 데 사용하세요: 특정 호스트 유형이나
역할에서만, 또는 특정 조건이 충족된 후에만 실행되어야 하는 스텝 등입니다.
전제 조건 실패를 가리는 데 사용하지 마세요. 강제적인 적합성 게이트에는
`CheckHost`를 사용하세요.

## 사용 가능한 네임스페이스

CEL 표현식은 세 가지 네임스페이스를 참조할 수 있습니다: `vars.*`,
`runtime.*`, `context.*`. 이들은 중괄호 없이 사용하세요, `when:`은 Go
템플릿 문법이 아닙니다.

### `vars.*`: 정적 입력 변수

`vars.yaml`, `-f` 오버레이, 시나리오 `vars:` 블록, `--var` 플래그(우선순위
오름차순)로부터 빌드된 완전히 병합된 변수 맵을 참조합니다. 노드 범위의
`hosts:` 선택은 `when:`이 평가될 시점에는 이미 실행되었으므로, `vars.role`는
현재 노드의 역할을 반영합니다.

```yaml
- id: init-control-plane
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

```yaml
- id: start-deck-server
  kind: ManageService
  spec:
    name: deck-server
    state: started
    enabled: true
  when: vars.deckServer == true
```

### `runtime.*`: 감지된 사실과 등록된 출력

`runtime.*`는 두 종류의 값을 제공합니다:

**`runtime.host` 아래의 내장 호스트 사실**: 어떤 스텝이 실행되기 전에
로컬 OS로부터 자동으로 채워집니다. 이를 채우기 위해 `CheckHost` 스텝이
필요하지 않으며, 항상 사용할 수 있습니다.

| 필드 | 예시 값 |
|---|---|
| `runtime.host.os.name` | `"linux"` |
| `runtime.host.os.id` | `"ubuntu"` |
| `runtime.host.os.family` | `"debian"` 또는 `"rhel"` |
| `runtime.host.os.version` | `"Ubuntu 24.04.2 LTS"` |
| `runtime.host.os.versionId` | `"24.04"` |
| `runtime.host.os.release` | `"24.04"` (`versionId`의 별칭) |
| `runtime.host.os.idLike` | `"debian"` |
| `runtime.host.arch` | `"amd64"` 또는 `"arm64"` |
| `runtime.host.kernel.release` | `"6.8.0-60-generic"` |

```yaml
- id: configure-apt-repo
  kind: ConfigureRepository
  spec:
    format: deb
    repositories:
      - id: offline-base
        baseurl: file:///srv/offline-repo
        trusted: true
  when: runtime.host.os.family == "debian"

- id: configure-dnf-repo
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

**`runtime.<name>` 아래의 등록된 스텝 출력**: 앞선 스텝이 `register:`를
사용해 출력을 내보낼 때 채워집니다. 등록된 값은 동일 단계의 모든 이후
스텝, 또는 이후 단계에서 사용할 수 있습니다. 출력을 생성하는 스텝이
`parallelGroup` 안에 있는 경우, 그 값은 전체 배치가 성공한 후에만
보입니다.

```yaml
steps:
  - id: collect-join-input
    kind: Input
    register:
      joinCipher: value      # runtime.joinCipher is available to later steps
    spec:
      message: "Paste the encrypted join block"
      required: true

  - id: use-join-value
    kind: Command
    spec:
      command: [echo, "got a join block"]
    when: runtime.joinCipher != ""   # gate on the registered value
```

### `context.*`: deck 실행 메타데이터

`context.*`는 현재 호출에 대한 deck이 제공하는 메타데이터를 담습니다. 이
값들은 명령이 시작될 때 결정되며 실행 중에는 변경되지 않습니다.

| 필드 | 사용 가능 시점 | 설명 |
|---|---|---|
| `context.command` | prepare, apply | 현재 명령: `"prepare"` 또는 `"apply"` |
| `context.workflow.source` | prepare, apply | `"filesystem"` 또는 `"server"` |
| `context.workflow.isServer` | prepare, apply | 소스가 `"server"`일 때 `true` |
| `context.workflow.path` | prepare, apply | 해석된 워크플로 파일 경로 또는 URL |
| `context.workflow.scenario` | apply 전용 | apply가 시나리오를 해석했을 때의 시나리오 이름 |
| `context.paths.bundleRoot` | prepare, apply | 준비된 출력 루트 (prepare) 또는 번들 루트 (apply) |
| `context.paths.outputRoot` | prepare 전용 | 준비된 출력 루트 |
| `context.paths.stateFile` | apply 전용 | apply 상태 파일 경로 |

```yaml
- id: announce-server-mode
  kind: Message
  spec:
    message: "Running from deck server, fetching artifacts remotely"
  when: context.workflow.isServer == true
```

## 일반적인 패턴

### OS 계열 분기

가장 일반적인 `when:` 패턴입니다. Debian 계열 또는 RHEL 계열 노드를
대상으로 하려면 `runtime.host.os.family`를 사용하세요:

```yaml
phases:
  - name: host-prereqs
    imports:
      - path: repo/offline-repo-debian.yaml
        when: runtime.host.os.family == "debian"
      - path: repo/offline-repo-rhel.yaml
        when: runtime.host.os.family == "rhel"
```

### 역할 기반 스텝

`vars.yaml`이 `hosts:`를 통해 노드별로 역할을 할당하는 경우 `vars.role`을
사용하세요:

```yaml
- id: init-first-control-plane
  kind: InitKubeadm
  spec:
    outputJoinFile: "{{ .vars.join.file }}"
  when: vars.role == "control-plane"

- id: join-cluster
  kind: JoinKubeadm
  spec:
    joinFile: "{{ .vars.join.file }}"
  when: vars.role == "worker"
```

매칭되지 않은 호스트가 `E_CONDITION_EVAL`을 일으키지 않도록, 항상
`vars.yaml`의 `all:` 섹션에 안전한 기본값을 제공하세요:

```yaml
all:
  role: ""   # skips both steps above on hosts not in hosts:
```

### 등록된 값에 대한 게이트

스텝은 앞선 스텝이 등록한 값에 대해 게이트를 걸 수 있습니다. 등록된 값은
동일 단계의 다음 스텝부터 보입니다(또는 생성자가 배치 안에 있는 경우 전체
병렬 배치 이후에 보입니다):

```yaml
steps:
  - id: ask-passphrase
    kind: Input
    register:
      passphrase: value
    spec:
      message: "Enter the join passphrase (leave blank to skip join)"
      secret: true

  - id: decrypt-join-command
    kind: Command
    spec:
      env:
        DECK_JOIN_PASS: "{{ .runtime.passphrase }}"
      command: [bash, -lc, "openssl enc -d -aes-256-cbc ..."]
    when: runtime.passphrase != ""
```

## 조건부 import

`phases[].imports` 항목은 선택적인 `when:` 필드를 받습니다. Deck는 import의
조건을 import된 파일의 모든 스텝과 AND로 결합합니다:

- import에 `when:`이 있지만 스텝에는 없는 경우, 스텝은 import 조건을
  상속받습니다.
- 둘 다 `when:`이 있는 경우, 유효 조건은
  `(import-when) && (step-when)`입니다.
- import에 `when:`이 없는 경우, 스텝은 자신의 조건을 변경 없이
  유지합니다.

```yaml
phases:
  - name: host-prereqs
    imports:
      - path: repo/offline-repo-debian.yaml
        when: runtime.host.os.family == "debian"
        # Every step in offline-repo-debian.yaml is now gated on this condition.
        # If a step inside also has its own when:, both conditions must be true.

      - path: host-prereqs.yaml
        # No import-level when:, steps inside keep their own conditions.
```

이는 import 수준에서 넓은 가드(OS 계열, 역할)를 적용하고 스텝 수준에서 더
좁은 가드를 사용할 수 있으며, 모든 스텝에서 넓은 가드를 중복할 필요가
없음을 의미합니다.

## `deck plan`으로 조건 미리보기

`deck plan`은 실행될 단계와 스텝을 보여주며, 어떤 스텝에 `when:` 조건이
있는지도 포함합니다. 오타와 잘못된 네임스페이스 참조를 잡기 위해 `apply`
전에 실행하세요:

```bash
deck plan
```

조건에 사용되는 변수에 대해서는 다음도 실행하세요:

```bash
deck plan vars
```

이는 유효한 `vars`와 초기 `runtime` 값을 보여줍니다, `vars.role`가 예상한
값으로 해석되었는지, `runtime.host.os.family`가 대상 호스트의 OS와
일치하는지 확인하세요.

`when:` 표현식이 런타임에 실패하면 deck은 `E_CONDITION_EVAL`을 보고하고
실행을 멈춥니다. 조건 관련 코드 전체 목록은 error codes 레퍼런스를
참고하세요.

## 제한 사항과 주의점

CEL은 타입이 있는 표현식 언어이며, 범용 스크립팅 언어가 아닙니다. 셸
확장, 문자열에 대한 산술 연산, 또는 CEL에 내장되지 않은 함수 호출을
지원하지 않습니다. 조건은 단순하게 유지하세요: 동등 비교, 비교 연산, 불
and/or, 문자열 포함 여부(`has()`).

타입 규칙은 엄격합니다. `vars.deckServer == true`는 `deckServer`가
`vars.yaml`에서 불리언일 때 동작합니다. 만약 문자열 `"true"`라면, 비교는
조용히 실패하고 스텝은 건너뜁니다. deck이 해석한 Go 타입을 확인하려면
`deck plan vars`를 확인하세요.

미정의 변수 처리. `vars.role`이 전혀 설정되지 않은 경우, `all:`에
기본값이 없고 호스트가 `hosts:`에 없기 때문에, CEL 표현식은
`E_CONDITION_EVAL`을 발생시킵니다. 분기 대상이 되는 모든 필드에 대해 항상
`all:` 기본값을 제공하세요.

**`register`로부터 온 `runtime.*` 값은 생성하는 스텝이 실행되기 전에는
사용할 수 없습니다.** 스텝 B가 `runtime.joinCipher`에 게이트를 걸고 스텝
A가 `joinCipher`를 등록하지만, A가 건너뛰어졌거나 아직 실행되지 않은
경우, 그 값은 미정의이며 `E_CONDITION_EVAL`을 일으킵니다. 생성자가 먼저
실행되도록 보장하려면 단계나 직렬 순서를 사용하세요.

컴포넌트 프래그먼트 내의 `when:`은 import하는 시나리오의 컨텍스트에서
평가됩니다. 프래그먼트는 `vars.*`와 `runtime.*`를 자유롭게 참조할 수
있지만, 자체적인 변수 스코프를 갖지는 않습니다.

## 관련 레퍼런스

- [Workflow Model, `when`](../workflow-model.md#when--conditional-execution): 표준 `when` 및 조건부 import 레퍼런스
- [Workflow Model, `register`](../workflow-model.md#register--capture-step-output): 스텝 출력을 `runtime.*`로 내보내는 방법
- [Workflow Model, Built-In Runtime Fields](../workflow-model.md#built-in-runtime-fields): 전체 `runtime.host` 필드 테이블
- [Variables and templating](variables-and-templating.md): `vars.*`가 어떻게 빌드되고 우선순위가 어떻게 동작하는지
- [Authoring workflows](authoring-workflows.md): `when:`을 전체 시나리오에 적용하기
- [Troubleshooting](../troubleshooting.md): `E_CONDITION_EVAL` 및 관련 오류 진단
