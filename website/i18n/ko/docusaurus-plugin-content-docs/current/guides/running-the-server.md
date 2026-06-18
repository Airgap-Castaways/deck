---
source: docs/guides/running-the-server.md
source_hash: 37c1f4a27031995b740b6e3202a7cc823d5e048c
---
# 콘텐츠 서버 실행하기

`deck server up`은 준비된 번들 루트를 공유 HTTP 엔드포인트로 바꿔 줍니다. 에어갭(망분리) 사이트의 모든 노드는 외부 네트워크 없이도 이 엔드포인트에서 번들을 내려받고, 사용 가능한 시나리오를 둘러보며, containerd가 동일한 소스에서 컨테이너 이미지를 가져오도록 할 수 있습니다.

---

## 서버가 필요한 경우와 필요 없는 경우

여러 노드가 같은 번들의 워크플로를 적용해야 한다면 **서버를 실행합니다**. 번들을 노드마다 복사하는 대신, 한 노드(보통 컨트롤 플레인)가 콘텐츠를 호스팅하고 나머지 노드는 모두 `--server`로 그 노드를 가리킵니다.

단일 노드에서 로컬 번들로 워크플로를 실행한다면 **서버가 필요 없습니다**. 이때는 `deck apply --root . --scenario apply`로 로컬 파일 시스템을 직접 사용하면 됩니다.

---

## 서버 시작하기

서버에는 번들 루트가 필요합니다. 번들 루트는 `workflows/`와 `outputs/`를 담고 있는 디렉터리로, `deck prepare`의 결과물입니다.

### 포그라운드 모드

```bash
deck server up --root /path/to/bundle --addr :5000
```

서버는 터미널을 점유한 채 요청을 stderr와 `<root>/.deck/logs/server-audit.log`의 감사 로그에 함께 기록합니다. 초기 설정 단계에서 요청을 실시간으로 확인하거나, 프로세스를 외부에서 관리하는 단기 자동화(컨테이너, 슈퍼바이저, CI 스텝)에서 포그라운드 모드를 사용합니다.

`Ctrl-C`로 중지합니다.

### 데몬 모드

```bash
deck server up --root /path/to/bundle --addr :5000 --daemon
```

`--daemon`은 서버를 백그라운드에서 시작합니다. 동작 방식은 플랫폼마다 다릅니다.

- **Linux**: `deck server up --daemon`은 일회성 `systemd-run` 유닛을 띄웁니다. `systemd-run`과 `systemctl`이 모두 `PATH`에 있어야 합니다.
- **macOS / Windows**: deck은 분리된 자식 프로세스를 생성하고 `~/.local/state/deck/server/` 아래에 pid 파일을 기록합니다.

시작 시 명령은 유닛 이름(Linux) 또는 pid와 로그 파일 경로(macOS/Windows)를 출력합니다.

```
server up: ok (deck-server, pid 12345)
server log: /home/ubuntu/.local/state/deck/server/deck-server.log
```

워커 노드에서 `deck apply`를 여러 차례 실행하는 동안 서버가 계속 떠 있어야 한다면 데몬 모드를 사용합니다. 며칠에 걸친 롤아웃 동안 모든 워커에 콘텐츠를 제공하는 컨트롤 플레인 노드가 그 예입니다.

Linux의 저널 접근을 포함한 플랫폼별 상세 내용은 [Server Daemon Mode](../server/daemon.md)를 참고하십시오.

---

## TLS 옵션

서버는 기본적으로 평문 HTTP로 수신 대기합니다. 암호화된 전송이 필요한 사이트를 위해 두 가지 옵션을 제공합니다.

### 자체 서명 인증서

```bash
deck server up --root /path/to/bundle --addr :5443 --tls-self-signed
```

deck은 시작할 때 자체 서명 인증서를 생성합니다. 워커 노드는 containerd 레지스트리 미러 항목에 `skipVerify: true`를 설정해야 합니다(offline-kubernetes 예제는 이미 이렇게 구성되어 있습니다. `vars.runtime.containerd.mirrorHosts[*].skipVerify`를 참고하십시오).

### 직접 준비한 인증서 사용

```bash
deck server up \
  --root /path/to/bundle \
  --addr :5443 \
  --tls-cert /etc/deck/server.crt \
  --tls-key  /etc/deck/server.key
```

PEM으로 인코딩된 인증서와 개인 키의 경로를 전달합니다. `--tls-cert`와 `--tls-key`는 항상 함께 지정해야 합니다.

---

## 서버가 제공하는 것

| 경로 접두사 | 제공 내용 |
|-------------|-----------------|
| `/workflows/` | 시나리오 파일. `deck apply`와 `deck plan`에서 `--server` + `--scenario`로 해석합니다. |
| `/v2/` | `outputs/images/` 아래의 이미지 tarball을 기반으로 하는 읽기 전용 OCI Distribution v2 레지스트리. |
| Browse UI | 사용 가능한 번들을 둘러보는 `/`의 정적 사이트(사람이 읽기 위한 용도). |
| `/healthz` | 헬스 프로브 엔드포인트. 서버가 준비되면 `200 OK`를 반환합니다. |

엔드포인트 목록, 별칭 규칙, 읽기 전용 강제 등 레지스트리 상세 내용은 [Server Registry](../server/registry.md)를 참고하십시오.

---

## containerd를 레지스트리 미러로 향하게 하기

서버의 `/v2` 레지스트리는 모든 노드의 containerd가 이를 업스트림 레지스트리의 미러로 취급하도록 구성했을 때 가장 유용합니다. 이렇게 하면 kubeadm이 인터넷에 접속하지 않고도 이미지를 가져올 수 있습니다.

### `WriteContainerdRegistryHosts` 스텝

offline-kubernetes 예제는 `components/runtime/registry-mirror.yaml`에서 `WriteContainerdRegistryHosts`로 미러를 구성합니다.

```yaml
# workflows/components/runtime/registry-mirror.yaml
steps:
  - id: configure-containerd-registry-mirrors
    kind: WriteContainerdRegistryHosts
    spec:
      path: "{{ .vars.runtime.containerd.certsDir }}"
      registryHosts: "{{ .vars.runtime.containerd.mirrorHosts }}"
  - id: restart-containerd-after-registry-mirror
    kind: ManageService
    spec:
      name: "{{ .vars.runtime.containerd.serviceName }}"
      state: restarted
  - id: wait-containerd-after-registry-mirror
    kind: WaitForService
    spec:
      name: "{{ .vars.runtime.containerd.serviceName }}"
      timeout: 5m
      interval: 2s
```

예제의 `vars.yaml`에 있는 `vars.runtime.containerd.mirrorHosts`는 각 업스트림 레지스트리(`registry.k8s.io`, `quay.io`, `docker.io`, `ghcr.io`)를 deck 서버 호스트로 매핑합니다. 예제는 `skipVerify: true`와 함께 `http://192.0.2.10:5000`을 사용합니다.

이미지가 필요한 부트스트랩 또는 조인 스텝에 앞서, 모든 노드에서 이 컴포넌트를 `image-source` 단계의 일부로 실행합니다.

```yaml
phases:
  - name: image-source
    imports:
      - path: runtime/registry-mirror.yaml
```

`WriteContainerdRegistryHosts`가 실행되고 containerd가 재시작되면, `imagePullPolicy: Never`를 설정한 `kubeadm init` / `kubeadm join`은 인터넷 대신 서버의 `/v2` 레지스트리에서 이미지를 해석합니다.

### 워커의 apply 실행을 서버로 향하게 하기

워커 노드가 원격 시나리오를 대상으로 `deck apply`를 실행할 때는 `--server <host:port>`로 deck이 워크플로를 가져올 위치를 지정합니다.

```bash
deck apply --server 192.0.2.10:5000 --scenario join
```

deck은 시나리오 파일을 `http://192.0.2.10:5000/workflows/scenarios/join.yaml`에서 해석하고, apply 상태를 번들 디렉터리가 아니라 사용자 로컬 XDG 상태 루트에 저장합니다(워크플로 소스가 원격이기 때문입니다).

서버 주소를 영구 기본값으로 저장해 두면 매번 입력하지 않아도 됩니다.

```bash
deck server remote set http://192.0.2.10:5000
deck apply --scenario join          # 저장된 원격 주소를 사용합니다
```

---

## 헬스 체크

서버를 시작한 뒤(어느 모드든) 노드를 서버로 향하게 하기 전에 응답 여부를 확인합니다.

```bash
deck server health --server http://192.0.2.10:5000
```

```bash
# 기계가 읽을 수 있는 출력:
deck server health --server http://192.0.2.10:5000 -o json
```

`deck server health`는 `/healthz`로 `GET`을 보내고 HTTP 200이면 `health: ok`를 보고합니다. 서버가 아직 준비되지 않았다면 몇 초 후에 다시 시도하십시오.

---

## 로그와 감사 기록

### 감사 로그 보기

모든 HTTP 요청은 다음 위치의 JSONL 감사 로그에 기록됩니다.

```
<root>/.deck/logs/server-audit.log
```

다음 명령으로 읽습니다.

```bash
deck server logs --root /path/to/bundle --source file
```

Linux에서는 저널을 스트리밍할 수도 있습니다.

```bash
deck server logs --source journal --unit deck-server.service
```

감사 로그는 설정된 크기 한도를 넘으면 자동으로 회전합니다(기본 50 MB, 최대 10개 파일 보관). `deck server up`의 `--audit-max-size-mb`와 `--audit-max-files`로 조정합니다.

전체 감사 기록 스키마는 [Server Audit Log](../server-audit-log.md)를 참고하십시오.

### 상세 출력 수준

`deck server up`에 `--v=1` 또는 `--v=2`를 추가하면 감사 로그와 함께 구조화된 진단 정보를 stderr로 내보냅니다.

---

## 데몬 중지하기

```bash
deck server down
```

Linux에서는 `systemctl stop deck-server.service`를 호출합니다. macOS/Windows에서는 추적 중인 프로세스에 `SIGTERM`을 보내고 pid 파일을 제거합니다. 프로세스가 이미 사라진 경우(오래된 pid)에도 중지는 성공하며 pid 파일이 정리됩니다.

사용자 지정 `--unit`으로 시작한 데몬을 중지하려면 다음과 같이 합니다.

```bash
deck server down --unit my-bundle-server
```

---

## 실전 예제: 컨트롤 플레인 서버 + 워커 조인

이 엔드투엔드 안내는 offline-kubernetes 예제의 레이아웃을 따릅니다.

### 컨트롤 플레인 노드(cp-1, 192.0.2.10)에서

```bash
# 1. 연결된 환경에서 전송한 번들을 추출합니다.
tar -xf bundle.tar

# 2. 5000번 포트에서 서버를 데몬으로 시작합니다.
deck server up --root . --addr :5000 --daemon

# 3. 준비 여부를 확인합니다.
deck server health --server http://192.0.2.10:5000

# 4. 부트스트랩 시나리오를 적용합니다(서버가 아닌 로컬 파일 시스템 사용).
deck apply --root . --scenario bootstrap
```

부트스트랩 도중 운영자에게 패스프레이즈를 묻습니다. kubeadm 조인 명령은 암호화된 블록 형태로만 터미널에 출력됩니다. `BEGIN ENCRYPTED JOIN`과 `END ENCRYPTED JOIN` 사이의 텍스트를 복사하십시오.

### 각 워커 노드(worker-1, 192.0.2.21)에서

```bash
# 1. 번들을 추출합니다(컨트롤 플레인과 동일한 번들).
tar -xf bundle.tar

# 2. 컨트롤 플레인 서버를 가리켜 조인 시나리오를 적용합니다.
#    deck은 http://192.0.2.10:5000에서 join.yaml 워크플로를 가져옵니다.
deck apply --server 192.0.2.10:5000 --scenario join
```

조인 시나리오는 다음을 수행합니다.
1. 암호화된 조인 블록을 입력하도록 요청합니다(암호문을 붙여넣습니다).
2. 패스프레이즈를 입력하도록 요청합니다.
3. containerd 레지스트리 미러를 구성하여(`image-source` 단계) kubeadm이 인터넷 없이 `192.0.2.10:5000/v2`에서 이미지를 가져오도록 합니다.
4. 조인 명령을 로컬에서 복호화하고 `kubeadm join`을 실행합니다.

### 모든 워커가 조인한 뒤 종료

```bash
# cp-1에서 모든 워커가 성공적으로 조인한 뒤:
deck server down
```

---

## 관련 참고 자료

- [Server Registry](../server/registry.md) — OCI `/v2` 엔드포인트 상세
- [Server Daemon Mode](../server/daemon.md) — systemd와 pid 파일 동작 방식
- [Server Audit Log](../server-audit-log.md) — 감사 기록 스키마
- [Offline Kubernetes example](../examples/README.md) — 전체 멀티 노드 안내
- [CLI Reference — deck server up](../cli/deck_server_up.md)
