---
source: docs/guides/capturing-output.md
source_hash: 39f4b5de108363c3fa91f796875718dc9d1395e0
---
# register로 스텝 출력 캡처하기

일부 스텝은 이후 스텝이 필요로 하는 값을 생성합니다, kubeadm join 파일 경로,
운영자가 제공한 IP 주소, 암호화된 join 블록 등입니다. `register`는 셸 변수
꼼수나 하드코딩된 경로 없이 그러한 값을 전달하는 메커니즘입니다.

---

## register가 해결하는 문제

원시 셸 스크립트에서는 명령의 출력을 변수에 캡처해 이후에 참조할 수 있습니다.
deck 워크플로에서 스텝은 격리된 타입 지정 연산이며, 공유되는 셸 상태가
없습니다. `register`는 한 스텝의 출력에서 이후 스텝의 입력으로 이어지는 타입
지정된 이름 있는 다리를 제공합니다.

`register` 없이:

```yaml
# BAD: join file path hard-coded in two places, fragile, easy to break.
- id: init-cluster
  kind: InitKubeadm
  spec:
    outputJoinFile: /tmp/deck/join.txt

- id: join-worker
  kind: JoinKubeadm
  spec:
    joinFile: /tmp/deck/join.txt   # duplicated literal
```

`register` 사용:

```yaml
- id: init-cluster
  kind: InitKubeadm
  register:
    joinFile: joinFile             # export the step's "joinFile" output as runtime.joinFile
  spec:
    outputJoinFile: "{{ .vars.join.file }}"

- id: join-worker
  kind: JoinKubeadm
  spec:
    joinFile: "{{ .runtime.joinFile }}"   # consumed via template
```

---

## 문법

```yaml
register:
  <runtime-name>: <output-key>
```

- `<runtime-name>`은 사용자가 선택하는 이름입니다. CEL 표현식(`when:`)에서는
  `runtime.<runtime-name>`으로, Go 템플릿 필드(`spec:` 값)에서는
  `.runtime.<runtime-name>`으로 사용할 수 있게 됩니다.
- `<output-key>`는 스텝 종류가 선언하는 출력 이름입니다. 스텝 종류가 명시적으로
  선언한 출력 이름만 등록할 수 있습니다. 선언되지 않은 키를 등록하려고 하면
  `deck lint` 시점에 검증이 실패합니다.

### 선언된 출력을 찾는 위치

각 스텝 종류의 레퍼런스 페이지는 요약 헤더에 `outputs`를 나열합니다. 예를 들면:

| Step kind | Declared outputs |
|-----------|-----------------|
| `InitKubeadm` | `joinFile` |
| `Input` | `value` |

선언된 출력이 없는 스텝은 검증 과정에서 비어 있지 않은 `register` 매핑을
거부합니다.

---

## 등록된 값 소비하기

### 스텝 `spec` 필드에서: Go 템플릿

이중 중괄호 템플릿 표현식 안에서 `.runtime.<name>`을 사용합니다:

```yaml
- id: announce-encrypted-join
  kind: Command
  spec:
    env:
      DECK_JOIN_PASS: "{{ .runtime.joinPass }}"
    command:
      - bash
      - -lc
      - |
        join_cmd="$(kubeadm token create --print-join-command)"
        printf '%s\n' "${join_cmd}" \
          | openssl enc -aes-256-cbc -pbkdf2 -salt -base64 -A -pass env:DECK_JOIN_PASS
```

### `when` 조건에서: CEL 표현식

CEL 표현식에서는 `runtime.<name>`(점 접두사 없음, 중괄호 없음)을 사용합니다:

```yaml
- id: skip-if-no-passphrase
  kind: Message
  when: runtime.joinPass != ""
  spec:
    level: info
    message: "Passphrase received; proceeding with encryption."
```

두 형식 모두 동일한 runtime 네임스페이스를 가리킵니다. 문법이 다른 것은
`spec` 필드가 Go 템플릿을 사용하고 `when`이 CEL을 사용하기 때문입니다.

---

## 시크릿 값

`Input`을 `secret: true`와 함께 사용하면 캡처된 값은:

- 적용 상태 파일에 절대 기록되지 않습니다(실행 간 유지되지 않음).
- 입력 중 터미널에 출력되지 않습니다.
- 동일한 실행 중에는 이후 스텝에서 메모리상으로 사용할 수 있습니다.

따라서 `Input` + `register` + `secret: true`는 패스프레이즈, 토큰, 그 밖에
디스크에 닿아서는 안 되는 모든 값에 대한 올바른 패턴입니다.

```yaml
- id: join-passphrase
  kind: Input
  register:
    joinPass: value          # "value" is the sole output of Input
  spec:
    message: "Passphrase used to encrypt the join command"
    secret: true
    required: true
```

시크릿 `Input` 스텝 이후에 실행이 중단되면 그 값은 사라집니다, 애초에 유지된
적이 없습니다. 다음 실행에서 해당 스텝은 운영자에게 다시 입력을 요청합니다.

### 암호화된 join 패턴

offline-kubernetes 예제는 두 시나리오에 걸쳐 이 패턴을 사용합니다:

1. **부트스트랩**(`components/bootstrap/announce-join.yaml`): 운영자가
   패스프레이즈를 선택합니다(`Input`, `secret: true`, `joinPass`로 등록).
   kubeadm join 명령은 패스프레이즈로 암호화되어 암호문으로만 로그에
   출력됩니다. 평문은 로그나 상태에 절대 나타나지 않습니다.

2. **Join**(`components/join/input-join.yaml`): 워커 노드에서 운영자가
   암호문을 붙여넣고(`Input`, `joinCipher`로 등록) 패스프레이즈를 다시
   입력합니다(`Input`, `secret: true`, `joinPass`로 등록). `Command` 스텝이
   join 명령을 로컬에서 복호화합니다. 등록된 두 값은 모두 환경 변수로
   전달되며, 명령 문자열에 인라인되거나 상태에 기록되지 않습니다.

전체 예제는
[examples/README.md](../examples/README.md#encrypted-join-pattern)를 참조하세요.

---

## 병렬 배치 제한

`register` 값을 생성하는 스텝이 `parallelGroup` 배치 안에서 실행되면, 그
출력은 **배치 전체가 완료된 후에만** 보입니다. 동일한 배치의 스텝들은 같은
runtime 스냅샷에서 시작하며 서로의 `register` 출력을 볼 수 없습니다.

```yaml
# This is invalid, both steps are in the same batch.
steps:
  - id: get-passphrase
    kind: Input
    parallelGroup: setup
    register:
      pass: value
    spec:
      message: "Enter passphrase"
      secret: true

  - id: use-passphrase
    kind: Command
    parallelGroup: setup        # WRONG: cannot consume pass from the same batch
    spec:
      env:
        PASS: "{{ .runtime.pass }}"
      command: [echo, "encrypting..."]
```

해결: 두 스텝에서 `parallelGroup`을 제거하거나, 생성자(producer)를 더 이른
배치나 더 이른 단계에 두세요.

전체 병렬 배치 규칙은 [단계와 병렬성](phases-and-parallelism.md)을 참조하세요.

---

## 흔한 실수

### 선언되지 않은 출력 키 등록

```yaml
# BAD: Command does not declare an output named "stdout".
- id: get-ip
  kind: Command
  register:
    nodeIP: stdout     # Command has no declared outputs, this fails validation
  spec:
    command: [hostname, -I]
```

`deck lint`는 실행 전에 이를 잡아냅니다. 스텝 종류의 레퍼런스 페이지에서 선언된
출력을 확인하세요.

### 동일한 병렬 배치에서 register 소비

위에서 보았듯이, 스텝은 같은 배치에서 생성된 `register` 값을 읽을 수 없습니다.
소비자(consumer)를 이후 스텝(다른 배치)이나 이후 단계에 배치하세요.

### 시크릿 값이 재개(resume) 후에도 남아 있으리라 기대

시크릿 `Input` 값은 절대 유지되지 않습니다. `Input` 스텝이 완료된 후 그 값을
소비하는 이후 스텝 전에 실행이 중단되면, 재개 시 그 값은 사라집니다. 해당
단계가 다시 실행될 때 deck은 시크릿을 다시 입력하라고 요청합니다.

---

## 작동 예제: Input → Command 파이프라인

운영자에게 레지스트리 주소를 묻고 이를 이후 스텝에서 사용하는 간단한
워크플로입니다:

```yaml
version: v1alpha1
steps:
  # Step 1: ask the operator for the registry host.
  - id: get-registry-host
    kind: Input
    register:
      registryHost: value      # "value" is Input's only declared output
    spec:
      message: "Registry host (e.g. 192.0.2.10:5000)"
      required: true

  # Step 2: write a containerd mirror configuration using the registered value.
  - id: configure-mirror
    kind: WriteFile
    spec:
      path: /etc/containerd/certs.d/registry.k8s.io/hosts.toml
      content: |
        server = "https://registry.k8s.io"
        [host."http://{{ .runtime.registryHost }}"]
          capabilities = ["pull", "resolve"]
          skip_verify = true
```

---

## 관련 레퍼런스

- [Workflow Model, register](../workflow-model.md#register--capture-step-output)
- [Workflow Model, Step Envelope Contract](../workflow-model.md#step-envelope-contract)
- [단계와 병렬성](phases-and-parallelism.md)
- [Input step kind](../step-kinds/input.md)
- [InitKubeadm step kind](../step-kinds/init-kubeadm.md)
- [오프라인 Kubernetes 예제](../examples/README.md)
