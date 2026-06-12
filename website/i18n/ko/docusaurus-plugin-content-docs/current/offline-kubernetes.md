---
source: docs/offline-kubernetes.md
source_hash: 48a6ca09d847fe5b43eb6dccbd4d61b4e7f9cf31
---
# 오프라인 Kubernetes 튜토리얼

이 튜토리얼은 `deck`이 설계된 환경, 즉 인터넷도 없고 SSH 기반 오케스트레이션도 없으며 각 노드에 로컬 운영자가 있는 환경에서 Kubernetes 유지보수 세션에 어떻게 들어맞는지 보여줍니다.

Kubernetes 호스트 준비 및 부트스트랩 절차는 빠르게 늘어납니다. 그 규모에서는 원시 셸 스크립트를 검토하기가 어려워집니다. `deck`은 절차를 구조화된 상태로 유지하고, 전송 전에 이를 검증하며, 노드에 필요한 모든 것을 단일 아카이브로 번들링합니다.

## 목표

연결된 환경에서 이식 가능한 번들을 빌드하고, 이를 에어갭(망분리) 환경으로 옮긴 다음, 각 대상 노드에서 Kubernetes 지향 워크플로를 로컬로 실행합니다.

## 1. 함께 제공되는 예제에서 시작하기

번들로 제공되는 워크플로 트리에서 시작하여 사이트에 맞게 조정하세요. 정확한 워크플로 및 스텝 계약이 필요할 때는 전송 전에 레퍼런스 문서와 교차 확인하세요.

## 2. 두 작업을 분리해서 유지하기

멘탈 모델은 단순합니다:

```text
prepare artifacts -> build bundle -> transfer bundle -> run locally on each node
```

`prepare`는 전송 전에 사이트에 필요한 것을 수집합니다. `apply`는 노드에서 절차를 로컬로 실행합니다. 이 분리 덕분에 에어갭 반대편의 운영자는 번들만 있으면 됩니다. 즉, 루트 `deck` 런처와 `outputs/bin/` 아래의 일치하는 런타임 바이너리만 있으면 되며, 실행 시점에 외부 의존성이 필요하지 않습니다.

## 3. 절차를 명확하게 모델링하기

스텝과 단계를 사용하여 절차가 무엇을 하는지 보여주세요. Kubernetes 워크플로에서의 일반적인 경계는 다음과 같습니다:

- 호스트 준비
- 패키지 또는 이미지 설정
- 런타임 구성
- kubeadm 부트스트랩 또는 join
- 검증

가능하면 타입이 지정된 스텝을 우선하세요. `Command`는 아직 모델링되지 않은 경계 사례를 위해 사용할 수 있습니다. `when`, `parallelGroup`, `register`, `metadata`, `retry`, `timeout` 같은 공유 스텝 필드는 [Step Envelope Contract](workflow-model.md#step-envelope-contract)를 사용하세요.

Kubernetes 워크플로에 유용한 그룹 엔트리포인트:

- [DownloadPackage](step-kinds/download-package.md)
- [DownloadImage](step-kinds/download-image.md)
- [CheckHost](step-kinds/check-host.md)
- [InstallPackage](step-kinds/install-package.md)
- [WriteContainerdConfig](step-kinds/write-containerd-config.md)
- [ManageService](step-kinds/manage-service.md)
- [InitKubeadm](step-kinds/init-kubeadm.md)
- [WaitForService](step-kinds/wait-for-service.md)

## 4. 연결된 환경에서 번들 준비하기

패키지, 컨테이너 이미지, 파일, 템플릿을 수집하는 `prepare` 워크플로를 작성하세요. 그런 다음 번들을 빌드합니다:

```bash
deck prepare
deck bundle build --out ./bundle.tar
```

로컬 테스트 중에는 사이트별 변경마다 공유 vars 파일을 편집하는 대신, 반복 가능한 `-f, --vars-file` 오버레이나 `--var key=value` 오버라이드를 사용하세요:

```bash
deck prepare -f vars/lab.yaml --var kubernetesVersion=v1.30.1 --var registryHost=mirror.local
```

번들에는 표준 워크스페이스 입력이 포함됩니다: `outputs/packages/`, `outputs/images/`, `outputs/files/`, `outputs/bin/`, `workflows/`, 루트 `deck` 런처, 그리고 `.deck/manifest.json` 체크섬.

## 5. 번들을 오프라인 사이트로 옮기기

`bundle.tar`를 해당 환경에 승인된 경로(이동식 미디어, 통제된 게이트웨이, 또는 사이트에서 승인한 다른 인계 방식)를 통해 전송하세요. 적용을 실행하기 전에 대상 측에서 압축을 해제하세요. `deck`은 이 단계에서 원격 제어 서비스를 필요로 하지 않습니다.

## 6. 대상 노드에서 워크플로를 로컬로 실행하기

오프라인 사이트에서는 대상 머신 자체에서 실행하세요:

```bash
tar -xf bundle.tar
cd bundle
./deck apply
```

워크스페이스의 control-plane 및 worker 워크플로를 kubeadm 기반 부트스트랩과 후속 유지보수의 출발점으로 사용하세요.

## 7. 실제 문제를 해결할 때만 사이트 지원 추가하기

일부 사이트는 에어갭 내부의 임시 공유 번들 소스로 이점을 얻을 수 있습니다. 이는 동일한 에어갭 내에서 여러 노드가 동일한 릴리스를 필요로 할 때 도움이 될 수 있습니다.

예시:

```bash
deck server up --root ./bundle --addr :8080
deck server up --root ./bundle --addr :8443 --tls-self-signed
deck server up --root ./bundle --addr :8080 --daemon --unit deck-server
```

이 선택을 명시적이고 부차적인 것으로 유지하세요. 핵심 워크플로는 각 노드에서의 로컬 `deck` 실행을 중심으로 합니다.

## 8. 전송 및 실행 전에 검증하기

```bash
deck lint
deck lint --workflow ./workflows/scenarios/apply.yaml
```

계획 및 진단을 위해서는 다음도 검토하세요:

- [Workflow model](workflow-model.md)
- [Step Kinds](step-kinds.md)
- [Workspace Layout](workspace-layout.md)
- [Server audit log](server-audit-log.md)
- [CLI Reference](cli.md)
