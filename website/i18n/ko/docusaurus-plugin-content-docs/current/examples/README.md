---
source: docs/examples/README.md
source_hash: 9687e8035337de8c6084787389454eb8ce29cdf8
sidebar_label: "Examples"
---

# 예제

이 디렉터리의 파일들은 실제 절차를 위한 출발점입니다. 타입이 지정된 스텝이 어떻게 운영 의도를 표현하는지, 그리고 단계가 어떻게 더 큰 워크플로를 한눈에 파악할 수 있게 유지하는지를 보여줍니다.

## 이 예제를 사용하는 방법

- 적용할 구체적인 워크플로가 필요할 때 여기서부터 시작하세요.
- 세부 사항을 더하기 전에 전체 구조를 명확하게 유지하세요.
- 이미 적합한 스텝 종류가 있다면 반복적인 셸을 타입이 지정된 스텝으로 교체하세요.
- 패키징하거나 전송하기 전에 결과를 검증하세요.

## Command 정책

- 이 예제들은 의도적으로 타입 우선(typed-first) 방식을 유지하며, 내장 스텝이 이미 동작을 모델링하는 경우 `Command` 사용을 피합니다.
- `Command`는 deck이 직접 모델링하지 않는 벤더 도구, 커스텀 프로브, 또는 일회성 로컬 명령에만 사용하세요.
- 워크플로에 서비스 라이프사이클 변경, 파일 작업, 아카이브 추출, sysctl 변경, 스왑 제어, 커널 모듈, 또는 심볼릭 링크 관리가 필요한 경우, 전용 타입 스텝을 우선 사용하세요.

## offline-kubernetes/ 워크스페이스

`offline-kubernetes/`는 전체 오프라인 Kubernetes 라이프사이클을 보여주는 완전한 멀티 노드 워크스페이스입니다. deck으로 실제 에어갭(망분리) 배포를 어떻게 구성하는지에 대한 대표 예제입니다.

### 레이아웃

```
offline-kubernetes/
  workflows/
    vars.yaml                       # shared variables (versions, IPs, paths)
    prepare.yaml                    # prepare workflow: packages, binaries, images
    scenarios/
      bootstrap.yaml                # control-plane bootstrap
      join.yaml                     # worker join
      reset.yaml                    # node reset
    components/                     # reusable phase fragments imported by scenarios
      bootstrap/
        announce-join.yaml          # encrypted join block output
        kubeadm.yaml
      host-prereqs.yaml
      join/
        input-join.yaml             # operator pastes ciphertext + passphrase
        join-node.yaml
      kube/user-access.yaml
      node/verify-selection.yaml
      repo/offline-repo-debian.yaml
      reset/
        artifacts.yaml
        cri.yaml
        kubeadm.yaml
        network.yaml
        scope-notice.yaml           # reset scope documentation step
      runtime/
        containerd.yaml
        k8s-binaries.yaml
        kubelet.yaml
        packages-debian.yaml
        registry-mirror.yaml
      verify/
        bootstrap-cluster.yaml
        node.yaml
```

### 설치 흐름

1. **준비** — 연결된 환경에서 `prepare.yaml`에 대해 `deck prepare`를 실행합니다. 이 단계는 Debian 패키지, Kubernetes 바이너리(kubelet, kubeadm, kubectl, containerd, runc, CNI 플러그인), kubeadm 이미지, Calico CNI 이미지를 `outputs/`로 다운로드합니다.
2. **빌드** — `deck bundle build`가 모든 것을 이식 가능한 `bundle.tar`로 패키징합니다.
3. **전송** — `bundle.tar`를 승인된 경로를 통해 에어갭(망분리) 사이트로 이동합니다.
4. **부트스트랩** — control-plane 노드(예: `cp-1`, `192.0.2.10`)에서 번들을 풀고 다음을 실행합니다:
   ```bash
   ./deck apply scenarios/bootstrap.yaml
   ```
   부트스트랩 중에 운영자는 패스프레이즈를 입력하라는 프롬프트를 받습니다. 그런 다음 kubeadm join 명령은 **암호화된 블록**으로만 로그에 출력됩니다 — 평문 join 파일은 제공되지 않습니다. `BEGIN ENCRYPTED JOIN`과 `END ENCRYPTED JOIN` 사이의 텍스트를 복사하세요.
5. **조인** — 각 워커 노드에서 다음을 실행합니다:
   ```bash
   ./deck apply scenarios/join.yaml --server 192.0.2.10:5000
   ```
   프롬프트가 나타나면 암호문을 한 줄로 붙여넣고, 부트스트랩에서 선택한 패스프레이즈를 입력합니다. 워커는 join 명령을 로컬에서 복호화하며 로그나 apply 상태에 절대 기록하지 않습니다.
6. **검증** — 두 시나리오 모두 클러스터와 노드 준비 상태를 확인하는 verify 단계를 포함합니다.

### 암호화 조인 패턴 {#encrypted-join-pattern}

부트스트랩은 `openssl enc -aes-256-cbc -pbkdf2`를 통해 join 명령을 암호화된 형태로만 출력합니다. 패스프레이즈는 부트스트랩 운영자가 대화형으로 입력하며 절대 저장되지 않습니다. 워커 운영자는 암호문(`Input` 스텝)을 붙여넣고 패스프레이즈(`Input`, secret)를 별도로 입력합니다. 복호화는 워커에서 로컬로 이루어지며, 평문은 `JoinKubeadm`이 사용하는 임시 파일에 기록된 후 제거됩니다. 평문 join 자격 증명은 네트워크를 거치지 않습니다.

### Calico CNI

Calico 이미지(`quay.io/calico/*`, `quay.io/tigera/operator`)는 `prepare.yaml`에 의해 다운로드되어 번들의 이미지 저장소에서 제공됩니다. CNI 매니페스트(Tigera operator 또는 `calico.yaml`)는 부트스트랩 이후 **대역 외(out-of-band)**로 적용됩니다 — 일반적으로 연결된 터미널에서 `kubectl apply -f`를 사용하거나, 매니페스트를 번들에 넣고 수동으로 실행합니다.

### 리셋 범위

`scenarios/reset.yaml`은 노드를 **재조인 가능한(re-joinable)** 상태로 되돌립니다. `kubeadm reset`을 실행하고, CRI를 중지하며, 네트워크 상태와 아티팩트를 정리합니다. 다음은 수행하지 **않습니다**:

- 클러스터 API에서 Node 객체를 제거하지 않습니다. 노드를 드레인하고 삭제하려면 control-plane 노드에서 다음을 실행하세요:
  ```bash
  kubectl drain <node> --ignore-daemonsets --delete-emptydir-data
  kubectl delete node <node>
  ```
- 컨테이너 런타임이나 Kubernetes 바이너리를 제거하지 않습니다(빠른 재조인을 위해 유지).
- 스왑을 다시 활성화하거나, 오프라인 레포를 제거하거나, OS 패키지를 제거하지 않습니다.

워크플로 내 운영자 알림은 `offline-kubernetes/workflows/components/reset/scope-notice.yaml`을 참조하세요.

### 검증

```bash
deck lint --root docs/examples/offline-kubernetes
```

연습 위주의 맥락을 원한다면 [Quick Start](../quick-start.md)와 [Offline Kubernetes Tutorial](../offline-kubernetes.md)부터 시작하세요.
