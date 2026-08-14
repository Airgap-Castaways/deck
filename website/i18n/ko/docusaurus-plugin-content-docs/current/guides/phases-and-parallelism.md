---
source: docs/guides/phases-and-parallelism.md
source_hash: 52719ef40b82820bc9c7d6b3c5a65564c159ceb6
---
# 단계와 병렬성

이 가이드는 평면적인 스텝 목록 대신 명명된 단계를 언제 사용해야 하는지, 운영자가
읽기 쉽도록 단계를 어떻게 명명하고 임포트하는지, 그리고 `parallelGroup`과
`maxParallelism`을 사용해 단계 내에서 안전한 스텝을 동시에 실행하는 방법을
설명합니다.

---

## 평면적 스텝 vs. 명명된 단계

워크플로는 모든 작업을 하나의 `steps:` 목록으로 표현하거나 명명된 `phases:`로
나눌 수 있습니다. 두 형식은 상호 배타적입니다; 워크플로는 최상위 수준에서
하나를 선택해야 합니다.

```yaml
# Flat, fine for small, self-contained procedures.
version: v1alpha1
steps:
  - id: ensure-state-dir
    kind: EnsureDirectory
    spec:
      path: /var/lib/deck
      mode: "0755"
  - id: write-config
    kind: WriteFile
    spec:
      path: /etc/myapp.conf
      content: "mode: production\n"
```

평면적 스텝은 `default`라는 이름의 암묵적 단계로 실행됩니다. 절차가 몇 개의 스텝을
넘어서기 전까지는 괜찮습니다. 그 지점부터는 단계를 명명하면 운영자가 모든 스텝을
읽기 전에 의도를 드러낼 수 있습니다.

명명된 단계는 다음의 경우에 사용하세요:

- 절차에 두 개 이상의 자연스러운 경계가 있을 때(예: 호스트 사전 요구사항 →
  런타임 설치 → 애플리케이션 부트스트랩).
- 의미 있는 재개 체크포인트를 원할 때(아래 참고).
- `workflows/components/`에서 재사용 가능한 컴포넌트 프래그먼트를 구성할 때.

---

## 재개 체크포인트로서의 단계

단계는 `deck apply`의 영속화된 재개 경계입니다.

- 완료된 단계는 상태가 존재할 때 다음 실행에서 건너뜁니다.
- 실패한 단계는 다음 실행에서 **첫 번째 스텝**부터 다시 실행됩니다. 실패한 단계
  내부의 부분적 진행 상황은 재사용되지 않습니다.
- 스텝 수준 재개는 지원되지 않습니다. 단계 수준 체크포인트만 영속화됩니다.

이는 단계가 전체가 완료되거나 처음부터 다시 시작되는 의미 있는 작업 단위를
나타낸다는 의미입니다. 단계 경계는 자연스러운 "여기서부터 재시작해도 안전한"
지점에서 선택하세요: 패키지가 설치된 후, 런타임이 구성된 후, `kubeadm init` 같은
단방향 작업 전에.

영속화되는 내용에 대한 전체 세부 사항은 [Apply State](../apply-state.md)를
참고하세요.

---

## 운영자가 읽기 쉽도록 단계 명명하기

간결하고 동작 중심의 이름을 선택하세요. `deck plan` 출력을 읽는 운영자는 절차를
한눈에 이해할 수 있어야 합니다:

```yaml
version: v1alpha1
phases:
  - name: host-prereqs        # OS-level checks and packages
  - name: runtime             # containerd, kubelet
  - name: image-source        # configure containerd registry mirror
  - name: bootstrap           # kubeadm init, announce join
  - name: verify              # readiness checks
```

이 다섯 단계 구조는
[offline-kubernetes 예제](https://github.com/Airgap-Castaways/deck/blob/main/docs/examples/offline-kubernetes/workflows/scenarios/bootstrap.yaml)에서
직접 가져온 것입니다. 단계 이름만 읽어도 운영자는 컴포넌트 파일을 하나도 열기 전에
배포 흐름에 대한 유용한 정신적 모델을 얻을 수 있습니다.

---

## 컴포넌트 프래그먼트 임포트하기

단계는 `workflows/components/`에서 재사용 가능한 스텝 파일을 임포트할 수 있습니다.
경로는 항상 `components/` 디렉터리를 기준으로 한 상대 경로이며, 절대 `../` 경로가
아닙니다.

```yaml
phases:
  - name: host-prereqs
    imports:
      - path: host-prereqs.yaml
      - path: repo/offline-repo-debian.yaml
        when: runtime.host.os.family == "debian"
  - name: runtime
    imports:
      - path: runtime/k8s-binaries.yaml
      - path: runtime/containerd.yaml
      - path: runtime/kubelet.yaml
  - name: image-source
    imports:
      - path: runtime/registry-mirror.yaml
```

컴포넌트 파일은 `steps:` 목록만 포함합니다. 자체 `phases:`나 `vars:`를 가질 수
없습니다, 공유 기본값은 `workflows/vars.yaml`이나 임포트하는 시나리오의 `vars:`
블록에 속합니다.

### 조건부 임포트

각 임포트 항목은 선택적인 `when` CEL 조건을 받습니다. Deck는 임포트 조건을 각
스텝 자체의 `when`과 AND로 결합합니다:

- 임포트에만 `when`이 있으면, 해당 파일의 모든 스텝이 이를 상속합니다.
- 스텝에만 `when`이 있으면, 자체 조건을 유지합니다.
- 둘 다 있으면, 스텝은 `(import-when) && (step-when)`일 때만 실행됩니다.

```yaml
phases:
  - name: host-prereqs
    imports:
      - path: repo/offline-repo-debian.yaml
        when: runtime.host.os.family == "debian"   # all steps in this file
      - path: repo/offline-repo-rhel.yaml
        when: runtime.host.os.family == "rhel"
      - path: host-prereqs.yaml                    # no filter, all nodes
```

단계는 또한 임포트와 인라인 `steps:`를 혼합할 수도 있습니다:

```yaml
phases:
  - name: verify
    imports:
      - path: verify/bootstrap-cluster.yaml
    steps:
      - id: check-node-ready
        kind: Command
        spec:
          command: [kubectl, get, nodes]
```

---

## `parallelGroup`을 사용한 병렬 배치

단계 내에서, 같은 `parallelGroup` 값을 공유하는 연속된 스텝들은 하나의 동시 배치로
실행됩니다. 다른 그룹에 속하거나 그룹이 없는 스텝은 순차적으로 실행됩니다.

```yaml
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
            release: ubuntu2404
          repo:
            type: deb-flat
          backend:
            mode: container
            runtime: docker
            image: ubuntu:24.04

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

      # This step runs after the batch above completes, sequentially.
      - id: verify-downloads
        kind: Command
        spec:
          command: [ls, -lh, outputs/packages/]
```

### `parallelGroup` 규칙

1. **연속된 스텝만 해당.** 같은 값을 가진 연속된 스텝만 같은 배치에 속합니다. 한
   번 배치가 닫히면(다른 그룹이거나 그룹이 없는 스텝이 나타나기 때문에), 같은 그룹
   이름은 단계 내에서 나중에 다시 열 수 없습니다.

2. **적용 시점 종류 허용 목록.** 적용 시점에는 특정한 스텝 종류 집합만 병렬 배치에
   나타날 수 있습니다: `Command`, `CopyFile`, `EnsureDirectory`, `ExtractArchive`,
   `WaitForCommand`, `WaitForFile`, `WaitForMissingFile`, `WaitForService`,
   `WaitForTCPPort`, `WaitForMissingTCPPort`, `WriteFile`. 다른 종류는 순차적으로
   실행되어야 합니다.

3. **공유 출력 경로 금지.** 같은 배치의 적용 스텝은 동일한 리터럴 출력 경로나 노드
   경로를 대상으로 할 수 없습니다. 같은 배치의 준비 스텝은 동일한 준비된 루트
   경로에 쓸 수 없습니다.

4. **배치 간 `register` 소비 금지.** 스텝은 같은 배치의 다른 스텝이 생성한
   `register` 값을 소비할 수 없습니다. 배치에서 등록된 값은 전체 배치가 성공한
   후에만 보이게 됩니다. 이 제약에 대한 세부 사항은
   [register로 스텝 출력 캡처하기](capturing-output.md)를 참고하세요.

### 단계의 `maxParallelism`

`maxParallelism`은 배치 내에서 동시에 실행되는 스텝 수를 제한합니다. 이 제한은
단계 내 배치별로 적용됩니다. 이것이 없으면 배치의 모든 스텝이 한 번에
시작됩니다.

```yaml
phases:
  - name: runtime
    maxParallelism: 2          # run at most 2 batch steps at a time
    imports:
      - path: runtime/containerd.yaml
```

이는 리소스가 제한된 머신에서 많은 병렬 다운로드를 실행할 때 유용합니다, 작업을
완전히 직렬화하지 않으면서 동시성을 제한하려면 `maxParallelism: 4`로
설정하세요.

---

## 실전 예제: 병렬 다운로드 그룹이 있는 다단계 시나리오

다음 시나리오는 세 개의 단계를 사용합니다. `prepare-artifacts` 단계는 두 개의
다운로드를 동시에 실행한 다음, `install`과 `verify` 단계를 순차적으로
진행합니다.

```yaml
# workflows/scenarios/apply.yaml
version: v1alpha1
phases:
  - name: prepare-artifacts
    maxParallelism: 2
    steps:
      # Both steps share parallelGroup "downloads", they start at the same time.
      - id: extract-containerd
        kind: ExtractArchive
        parallelGroup: downloads
        spec:
          src: "{{ .context.paths.bundleRoot }}/files/bin/linux/amd64/containerd.tar.gz"
          dest: /opt/containerd-k8s
          strip: 1

      - id: extract-cni-plugins
        kind: ExtractArchive
        parallelGroup: downloads
        spec:
          src: "{{ .context.paths.bundleRoot }}/files/bin/linux/amd64/cni-plugins.tgz"
          dest: /opt/cni/bin
          strip: 0

      # Runs sequentially after the batch above completes.
      - id: set-cni-permissions
        kind: Command
        spec:
          command: [chmod, -R, "0755", /opt/cni/bin]

  - name: install
    imports:
      - path: runtime/kubelet.yaml

  - name: verify
    steps:
      - id: check-kubelet
        kind: WaitForService
        spec:
          name: kubelet
          timeout: 2m
          interval: 5s
```

**재개 동작:** `install` 단계가 중간에 실패하고 `deck apply`를 다시 실행하면,
`prepare-artifacts` 단계는 건너뛰며(이미 완료됨), `install`은 첫 번째 스텝부터
다시 시작됩니다.

---

## 안티 패턴

### 단계 과도하게 분할하기

두 줄짜리 단일 스텝만 있는 단계는 이점 없이 형식만 추가합니다. 모든 스텝에 각자의
단계를 부여하기보다는 논리적으로 관련된 스텝을 하나의 단계로 병합하세요. 좋은
경험칙: 단계는 의미 있는 이름과 함께 30초에서 수 분 정도의 작업을 나타내야
합니다.

### 출력 또는 순서 의존성을 공유하는 스텝 병렬화하기

스텝 B가 스텝 A가 쓴 파일을 읽는다면, 둘은 순차적으로 실행되어야 합니다. 이들을
같은 `parallelGroup`에 두면 규칙을 위반하며 경로 충돌 오류로 실패하거나 경쟁
상태를 일으킵니다.

### 같은 배치에서 `register` 값 소비하기

```yaml
steps:
  # BAD: both steps are in the same batch.
  # "join-node" cannot see joinFile until after the batch completes.
  - id: init-cluster
    kind: InitKubeadm
    parallelGroup: kube-init
    register:
      joinFile: joinFile
    spec:
      outputJoinFile: /tmp/deck/join.txt

  - id: join-node
    kind: JoinKubeadm
    parallelGroup: kube-init          # WRONG, same batch
    spec:
      joinFile: "{{ .runtime.joinFile }}"
```

수정: 두 스텝 모두에서 `parallelGroup`을 제거하거나, 별도의 단계에 두세요.

---

## 관련 참고 자료

- [Workflow Model, Phases](../workflow-model.md#phases)
- [Workflow Model, Parallel batches](../workflow-model.md#parallel-batches)
- [Apply State, Phase-based resume](../apply-state.md#phase-based-resume)
- [register로 스텝 출력 캡처하기](capturing-output.md)
