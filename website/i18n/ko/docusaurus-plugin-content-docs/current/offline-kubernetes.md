---
source: docs/offline-kubernetes.md
source_hash: c2b99d95367a5b3013551660b0d7f2a9f51c8062
---
# 오프라인 Kubernetes 튜토리얼

이 튜토리얼은 `deck`이 설계 목적인 환경 — 인터넷 없음, SSH 기반 오케스트레이션 없음, 각 노드의 로컬 운영자 — 에서 Kubernetes 유지보수 세션에 어떻게 들어맞는지 보여줍니다.

Kubernetes 호스트 준비 및 부트스트랩 절차는 빠르게 늘어납니다. 그 규모에서 순수 셸은 검토하기 어려워집니다. `deck`은 절차를 구조화된 상태로 유지하고, 전송 전에 검증하며, 노드가 필요로 하는 모든 것을 단일 아카이브로 번들링합니다.

## 목표

연결된 환경에서 이식 가능한 번들을 빌드하고, 에어갭(망분리) 환경으로 옮긴 다음, 각 대상 노드에서 Kubernetes 중심 워크플로를 로컬로 실행합니다.

## 워크스페이스

이 튜토리얼의 참조 워크스페이스는 `examples/offline-kubernetes/`입니다. 준비 워크플로, 부트스트랩·조인·리셋용 시나리오 워크플로, 그리고 시나리오가 임포트하는 재사용 가능한 컴포넌트 프래그먼트를 포함합니다.

```
examples/offline-kubernetes/
  workflows/
    vars.yaml          # shared site variables
    prepare.yaml       # gather packages, binaries, images
    scenarios/
      bootstrap.yaml   # control-plane bootstrap
      join.yaml        # worker join
      reset.yaml       # node reset
    components/        # imported by scenarios
```

`vars.yaml`을 사이트에 맞게 조정하세요. 기본값은 RFC 5737 TEST-NET 주소(`192.0.2.0/24`)를 사용하므로, 사용 전에 실제 IP로 교체하세요.

## 1. 연결된 환경에서 아티팩트 준비

준비 워크플로를 실행하여 노드가 필요로 하는 모든 것을 다운로드합니다:

```bash
deck prepare
```

이는 `workflows/prepare.yaml`을 실행하며, 다음을 다운로드합니다:

- Debian 패키지 (conntrack, socat, iptables 및 기타 호스트 선행 요건)
- Kubernetes 바이너리: `kubelet`, `kubeadm`, `kubectl`, `crictl`, `containerd`, `runc`, CNI 플러그인
- kubeadm 컨테이너 이미지: `kube-apiserver`, `kube-controller-manager`, `kube-scheduler`, `kube-proxy`, `etcd`, `coredns`, `pause`
- Calico CNI 이미지 (`quay.io/calico/*`, `quay.io/tigera/operator`)

공유 파일을 편집하지 않고 사이트별 값을 재정의합니다:

```bash
deck prepare -f vars/lab.yaml --var kubernetesVersion=v1.30.1 --var cluster.controlPlaneEndpoint=10.0.1.10:6443
```

## 2. 번들 빌드

준비된 모든 아티팩트를 이식 가능한 아카이브로 패키징합니다:

```bash
deck bundle build --out ./bundle.tar
```

번들에는 `outputs/packages/`, `outputs/images/`, `outputs/files/`, `outputs/bin/`, `workflows/` 트리, 루트 `deck` 런처, 그리고 `.deck/manifest.json` 체크섬이 포함됩니다.

## 3. 에어갭(망분리) 사이트로 번들 전송

`bundle.tar`을 해당 환경에서 승인된 경로 — 이동식 매체, 통제된 게이트웨이, 또는 사이트에서 승인한 다른 전달 방식 — 를 통해 옮기세요. 적용을 실행하기 전에 대상 쪽에서 압축을 풉니다.

## 4. 컨트롤 플레인 노드 부트스트랩

컨트롤 플레인 노드(예: `192.0.2.10`의 `cp-1`)에서 번들 압축을 풀고 부트스트랩 시나리오를 실행합니다:

```bash
tar -xf bundle.tar
cd bundle
./deck apply scenarios/bootstrap.yaml
```

부트스트랩 시나리오는 다음 단계를 순서대로 실행합니다:

1. **host-prereqs** — 노드 선택을 검증하고, 오프라인 Debian repo를 적용하며, OS 패키지를 설치합니다
2. **runtime** — Kubernetes 바이너리, containerd, kubelet을 설치합니다
3. **image-source** — 번들 서버를 가리키는 containerd 레지스트리 미러를 구성합니다
4. **bootstrap** — `kubeadm init`을 실행하고, 암호화된 조인 블록을 출력하며, kubeconfig를 구성합니다
5. **verify** — 클러스터 준비 상태를 확인합니다

### 암호화된 조인 블록

`bootstrap` 단계 동안 운영자에게 패스프레이즈를 입력하라는 프롬프트가 표시됩니다. 그런 다음 kubeadm 조인 명령은 **암호화된 형태로만** 로그에 출력됩니다:

```
----- BEGIN ENCRYPTED JOIN -----
<base64-encoded ciphertext>
----- END ENCRYPTED JOIN -----
```

`BEGIN`과 `END` 마커 사이의 텍스트(base64 한 줄)를 복사하세요. 평문 조인 자격 증명은 로그, 적용 상태, 또는 제공되는 어떤 파일에도 기록되지 않습니다.

### Calico CNI

Calico 이미지는 준비되어 번들의 이미지 저장소에서 사용할 수 있습니다. CNI 매니페스트(Tigera operator 또는 `calico.yaml`)는 부트스트랩 이후 **대역 외(out-of-band)** 로 적용됩니다 — 예를 들어, 매니페스트를 번들에 넣어두고 `kubeconfig`를 사용할 수 있게 된 후 클러스터 접근이 가능한 터미널에서 `kubectl apply -f`를 실행하는 방식입니다.

## 5. 워커 노드 조인

각 워커 노드(예: `worker-1`)에서 번들 압축을 풀고 조인 시나리오를 실행합니다:

```bash
tar -xf bundle.tar
cd bundle
./deck apply scenarios/join.yaml --server 192.0.2.10:5000
```

조인 시나리오는 운영자에게 두 가지 입력을 요청합니다:

1. **암호문(Ciphertext)** — 암호화된 조인 블록을 한 줄로 붙여넣습니다(줄 바꿈 없음).
2. **패스프레이즈(Passphrase)** — 부트스트랩 시 선택한 패스프레이즈로, 시크릿으로 입력됩니다(에코되지 않음).

워커는 조인 명령을 로컬에서 복호화하여 `JoinKubeadm`이 사용하는 임시 파일에 기록합니다. 평문은 네트워크를 통과하지 않으며 적용 상태나 로그에 저장되지 않습니다.

조인 단계 이후 verify 단계가 노드의 클러스터 조인 여부를 확인합니다.

## 6. 전송 및 실행 전 검증

```bash
deck lint --root examples/offline-kubernetes
```

개별 시나리오의 경우:

```bash
deck lint --workflow examples/offline-kubernetes/workflows/scenarios/bootstrap.yaml
```

## 7. 사이트 서버 (선택)

일부 사이트에서는 에어갭(망분리) 내부에 임시 공유 번들 소스를 두어 여러 노드가 개별 전송 없이 동일한 릴리스를 가져올 수 있도록 하는 것이 유용합니다:

```bash
deck server up --root ./bundle --addr :5000
deck server up --root ./bundle --addr :5000 --daemon --unit deck-server
```

vars 기본값(`server.url: 192.0.2.10:5000`)이 이 패턴과 일치합니다. 이 선택은 명시적이고 부차적으로 유지하세요. 핵심 워크플로는 각 노드에서의 로컬 `deck` 실행을 중심으로 합니다.

## 8. 노드 리셋

노드를 다시 조인 가능한 상태로 되돌리려면 리셋 시나리오를 실행합니다:

```bash
./deck apply scenarios/reset.yaml
```

리셋 시나리오는 `kubeadm reset`을 실행하고, CRI를 중지하며, 네트워크 상태를 초기화하고, deck 아티팩트를 제거합니다. 마지막에 운영자 대상 범위 고지로 끝납니다.

### 리셋 범위

리셋은 다음을 수행하지 **않습니다**:

- 클러스터 API에서 Node 객체 제거. 노드를 드레인하고 삭제하려면 컨트롤 플레인 노드에서 실행하세요:
  ```bash
  kubectl drain <node> --ignore-daemonsets --delete-emptydir-data
  kubectl delete node <node>
  ```
- 컨테이너 런타임 또는 Kubernetes 바이너리 제거. 이들은 노드가 빠르게 다시 조인할 수 있도록 유지됩니다. 전체 해체는 수동입니다:
  ```bash
  systemctl disable --now containerd-k8s
  rm -rf /opt/containerd-k8s /data/containerd-k8s /etc/containerd-k8s
  ```
- swap 재활성화, 오프라인 repo 제거, OS 패키지 제거.

워크플로 내 리셋 범위 고지는 `examples/offline-kubernetes/workflows/components/reset/scope-notice.yaml`에 있습니다.

## 이 워크스페이스에서 사용하는 스텝 종류

이 워크스페이스가 의존하는 스텝 종류에 대한 유용한 참조 항목:

- [DownloadPackage](step-kinds/download-package.md)
- [DownloadImage](step-kinds/download-image.md)
- [DownloadFile](step-kinds/download-file.md)
- [CheckHost](step-kinds/check-host.md)
- [InstallPackage](step-kinds/install-package.md)
- [WriteContainerdConfig](step-kinds/write-containerd-config.md)
- [ManageService](step-kinds/manage-service.md)
- [InitKubeadm](step-kinds/init-kubeadm.md)
- [WaitForService](step-kinds/wait-for-service.md)

공유 스텝 필드(`when`, `parallelGroup`, `register`, `metadata`, `retry`, `timeout`)에 대해서는 [Step Envelope Contract](workflow-model.md#step-envelope-contract)를 참조하세요.

계획 수립 및 진단을 위해 다음도 검토하세요:

- [Workflow model](workflow-model.md)
- [Step Kinds](step-kinds.md)
- [Workspace Layout](workspace-layout.md)
- [Server audit log](server-audit-log.md)
- [CLI Reference](cli.md)
