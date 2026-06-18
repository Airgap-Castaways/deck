---
source: docs/guides/running-the-server.md
source_hash: 37c1f4a27031995b740b6e3202a7cc823d5e048c
---
# 콘텐츠 서버 실행

`deck server up`은 준비된 번들 루트를 공유 HTTP 엔드포인트로 전환합니다.
에어갭(망분리) 사이트의 모든 노드는 외부 네트워크 접근 없이 동일한 소스에서
번들을 가져오고, 사용 가능한 시나리오를 탐색하고, containerd가 컨테이너
이미지를 가져오도록 할 수 있습니다.

---

## 서버를 실행해야 할 때(그리고 실행하지 말아야 할 때)

여러 노드가 모두 동일한 번들에서 워크플로를 적용해야 할 때 **서버를
실행**하세요. 모든 노드에 번들을 복사하는 대신, 한 노드(일반적으로
컨트롤 플레인)가 콘텐츠를 호스팅하고 다른 모든 노드는 `--server`로 그
노드를 가리킵니다.

단일 노드에서 로컬 번들로 워크플로를 실행할 때는 **서버를 건너뛰세요**. 이
경우 `deck apply --root . --scenario apply`가 로컬 파일시스템에 대해 직접
동작합니다.

---

## 서버 시작하기

서버는 번들 루트 — `workflows/`와 `outputs/`를 포함하는 디렉터리(`deck
prepare`의 출력) — 가 필요합니다.

### 포그라운드 모드

```bash
deck server up --root /path/to/bundle --addr :5000
```

서버는 터미널을 점유하고 요청을 stderr와 `<root>/.deck/logs/server-audit.log`의
감사 로그에 모두 기록합니다. 요청을 실시간으로 관찰하려는 초기 설정 단계나,
프로세스가 외부에서 관리되는 단명 자동화(컨테이너, 슈퍼바이저, CI 스텝)에서
포그라운드 모드를 사용하세요.

`Ctrl-C`로 중지합니다.

### 데몬 모드

```bash
deck server up --root /path/to/bundle --addr :5000 --daemon
```

`--daemon`은 서버를 백그라운드에서 시작합니다. 메커니즘은 플랫폼별로
다릅니다:

- **Linux**: `deck server up --daemon`은 임시 `systemd-run` 유닛을
  실행합니다. `systemd-run`과 `systemctl`이 모두 `PATH`에 있어야 합니다.
- **macOS / Windows**: deck는 분리된 자식 프로세스를 생성하고
  `~/.local/state/deck/server/` 아래에 pid 파일을 작성합니다.

시작 시 명령은 유닛 이름(Linux) 또는 pid와 로그 파일 경로(macOS/Windows)를
출력합니다:

```
server up: ok (deck-server, pid 12345)
server log: /home/ubuntu/.local/state/deck/server/deck-server.log
```

워커 노드에서 `deck apply` 호출 사이에 서버가 계속 실행되어야 할 때 데몬
모드를 사용하세요 — 예를 들어 여러 날에 걸친 롤아웃 동안 모든 워커에
서비스를 제공하는 컨트롤 플레인 노드에서 사용합니다.

Linux의 저널 접근을 포함한 전체 플랫폼 세부 정보는
[Server Daemon Mode](../server/daemon.md)를 참조하세요.

---

## TLS 옵션

기본적으로 서버는 평문 HTTP로 수신합니다. 암호화된 전송이 필요한 사이트를
위해 두 가지 옵션을 사용할 수 있습니다.

### 자체 서명 인증서

```bash
deck server up --root /path/to/bundle --addr :5443 --tls-self-signed
```

Deck는 시작 시 자체 서명 인증서를 생성합니다. 워커 노드는 containerd
레지스트리 미러 항목에서 `skipVerify: true`를 구성해야 합니다(offline-kubernetes
예제에서 이미 이렇게 하고 있습니다 —
`vars.runtime.containerd.mirrorHosts[*].skipVerify` 참조).

### 자체 인증서 사용

```bash
deck server up \
  --root /path/to/bundle \
  --addr :5443 \
  --tls-cert /etc/deck/server.crt \
  --tls-key  /etc/deck/server.key
```

PEM으로 인코딩된 인증서와 개인 키의 경로를 전달합니다. `--tls-cert`와
`--tls-key`는 반드시 함께 제공해야 합니다.

---

## 서버가 제공하는 것

| 경로 접두사 | 제공 내용 |
|-------------|-----------------|
| `/workflows/` | 시나리오 파일; `deck apply`와 `deck plan`에서 `--server` + `--scenario`로 해석됨. |
| `/v2/` | `outputs/images/` 아래의 이미지 tarball을 백엔드로 하는 읽기 전용 OCI Distribution v2 레지스트리. |
| Browse UI | 사용 가능한 번들을 탐색하기 위한 `/`의 정적 사이트(사람이 읽을 수 있음). |
| `/healthz` | 헬스 프로브 엔드포인트; 서버가 준비되면 `200 OK`를 반환. |

레지스트리 세부 정보 — 엔드포인트 목록, 별칭 규칙, 읽기 전용 강제 — 는
[Server Registry](../server/registry.md)를 참조하세요.

---

## containerd를 레지스트리 미러로 향하게 하기

서버의 `/v2` 레지스트리는 모든 노드의 containerd가 이를 업스트림
레지스트리의 미러로 취급하도록 구성될 때 가장 유용합니다. 이를 통해
kubeadm은 인터넷에 접근하지 않고도 이미지를 가져올 수 있습니다.

### `WriteContainerdRegistryHosts` 스텝

offline-kubernetes 예제는 `components/runtime/registry-mirror.yaml`에서
`WriteContainerdRegistryHosts`를 사용하여 미러를 구성합니다:

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

예제의 `vars.yaml`에 있는 `vars.runtime.containerd.mirrorHosts`는 각
업스트림 레지스트리(`registry.k8s.io`, `quay.io`, `docker.io`, `ghcr.io`)를
deck 서버 호스트에 매핑합니다. 예제는 `skipVerify: true`와 함께
`http://192.0.2.10:5000`을 사용합니다.

이미지가 필요한 부트스트랩 또는 조인 스텝 이전에 모든 노드에서 이 컴포넌트를
`image-source` 단계의 일부로 실행하세요:

```yaml
phases:
  - name: image-source
    imports:
      - path: runtime/registry-mirror.yaml
```

`WriteContainerdRegistryHosts`가 실행되고 containerd가 재시작된 후,
`imagePullPolicy: Never`로 실행하는 `kubeadm init` / `kubeadm join`은
인터넷 대신 서버의 `/v2` 레지스트리에서 이미지를 해석합니다.

### 워커 apply 실행을 서버로 향하게 하기

워커 노드가 원격 시나리오에 대해 `deck apply`를 실행할 때, `--server
<host:port>`를 사용하여 deck에 워크플로를 가져올 위치를 알려주세요:

```bash
deck apply --server 192.0.2.10:5000 --scenario join
```

Deck는 `http://192.0.2.10:5000/workflows/scenarios/join.yaml`에서 시나리오
파일을 해석하고, 적용 상태를 사용자 로컬 XDG 상태 루트에
저장합니다(워크플로 소스가 원격이므로 번들 디렉터리가 아님).

서버 주소를 영구 기본값으로 저장하여 매 호출마다 입력하지 않아도 되도록 할
수도 있습니다:

```bash
deck server remote set http://192.0.2.10:5000
deck apply --scenario join          # 저장된 원격을 사용
```

---

## 헬스 체크

서버를 시작한 후(어느 모드든), 노드를 향하게 하기 전에 응답하는지
확인하세요:

```bash
deck server health --server http://192.0.2.10:5000
```

```bash
# 기계가 읽을 수 있는 출력:
deck server health --server http://192.0.2.10:5000 -o json
```

`deck server health`는 `/healthz`로 `GET`을 보내고 HTTP 200에서 `health:
ok`를 보고합니다. 서버가 아직 준비되지 않았다면 몇 초 후에 재시도하세요.

---

## 로그 및 감사 기록

### 감사 로그 보기

모든 HTTP 요청은 다음 위치의 JSONL 감사 로그에 작성됩니다:

```
<root>/.deck/logs/server-audit.log
```

다음으로 읽습니다:

```bash
deck server logs --root /path/to/bundle --source file
```

또는 Linux에서 저널을 스트리밍합니다:

```bash
deck server logs --source journal --unit deck-server.service
```

감사 로그는 구성된 크기 제한을 초과하면 자동으로 회전합니다(기본 50 MB,
최대 10개 파일 보관). `deck server up`에서 `--audit-max-size-mb`와
`--audit-max-files`로 조정합니다.

전체 감사 기록 스키마는 [Server Audit Log](../server-audit-log.md)를
참조하세요.

### 상세 수준

`deck server up`에 `--v=1` 또는 `--v=2`를 추가하면 감사 로그와 함께
구조화된 진단 정보를 stderr로 방출합니다.

---

## 데몬 중지하기

```bash
deck server down
```

Linux에서는 `systemctl stop deck-server.service`를 호출합니다.
macOS/Windows에서는 추적된 프로세스에 `SIGTERM`을 보내고 pid 파일을
제거합니다. 프로세스가 이미 사라진 경우(오래된 pid), 중지는 여전히
성공하고 pid 파일이 정리됩니다.

사용자 지정 `--unit`으로 시작한 데몬을 중지하려면:

```bash
deck server down --unit my-bundle-server
```

---

## 실전 예제: 컨트롤 플레인 서버 + 워커 조인

이 엔드투엔드 안내는 offline-kubernetes 예제 레이아웃과 일치합니다.

### 컨트롤 플레인 노드에서(cp-1, 192.0.2.10)

```bash
# 1. 연결된 환경에서 전송된 번들을 추출합니다.
tar -xf bundle.tar

# 2. 포트 5000에서 서버를 데몬으로 시작합니다.
deck server up --root . --addr :5000 --daemon

# 3. 준비되었는지 확인합니다.
deck server health --server http://192.0.2.10:5000

# 4. 부트스트랩 시나리오를 적용합니다(서버가 아닌 로컬 파일시스템 사용).
deck apply --root . --scenario bootstrap
```

부트스트랩 중에 운영자는 암호문구를 입력하도록 요청받습니다. kubeadm 조인
명령은 암호화된 블록으로만 터미널에 출력됩니다. `BEGIN ENCRYPTED JOIN`과
`END ENCRYPTED JOIN` 사이의 텍스트를 복사하세요.

### 각 워커 노드에서(worker-1, 192.0.2.21)

```bash
# 1. 번들을 추출합니다(컨트롤 플레인과 동일한 번들).
tar -xf bundle.tar

# 2. 컨트롤 플레인 서버를 가리키며 조인 시나리오를 적용합니다.
#    deck는 http://192.0.2.10:5000에서 join.yaml 워크플로를 가져옵니다.
deck apply --server 192.0.2.10:5000 --scenario join
```

조인 시나리오:
1. 암호화된 조인 블록을 입력하도록 요청합니다(암호문을 붙여넣기).
2. 암호문구를 입력하도록 요청합니다.
3. containerd 레지스트리 미러를 구성하여(`image-source` 단계) kubeadm이
   인터넷 접근 없이 `192.0.2.10:5000/v2`에서 이미지를 가져올 수 있도록
   합니다.
4. 조인 명령을 로컬에서 복호화하고 `kubeadm join`을 실행합니다.

### 모든 워커가 조인한 후 종료

```bash
# cp-1에서, 모든 워커가 성공적으로 조인한 후:
deck server down
```

---

## 관련 참조

- [Server Registry](../server/registry.md) — OCI `/v2` 엔드포인트 세부 정보
- [Server Daemon Mode](../server/daemon.md) — systemd 및 pid 파일 메커니즘
- [Server Audit Log](../server-audit-log.md) — 감사 기록 스키마
- [Offline Kubernetes example](../examples/README.md) — 전체 멀티 노드 안내
- [CLI Reference — deck server up](../cli/deck_server_up.md)
