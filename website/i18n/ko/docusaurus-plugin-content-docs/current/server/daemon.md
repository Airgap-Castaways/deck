---
source: docs/server/daemon.md
source_hash: 3e46c166432be5b4e15018371fcb3181f069a8df
---
# 서버 데몬 모드

`deck server up --daemon`은 콘텐츠 서버를 백그라운드에서 시작합니다. 그
메커니즘은 플랫폼에 따라 다릅니다. Linux는 일시적 systemd 서비스를 사용하고,
macOS와 Windows는 pid 파일로 추적되는 분리된(detached) 백그라운드 프로세스를
사용합니다.

## Linux — 일시적 systemd 서비스

Linux에서 `deck server up --daemon`은 `systemd-run`을 호출하여 일시적
서비스 유닛을 실행합니다. `systemd-run`과 `systemctl`이 모두 `PATH`에
있어야 하며, 그렇지 않으면 명령은 무언가를 시작하기 전에 실패합니다.

유닛 이름은 `--unit`으로 설정합니다(기본값 `deck-server`). 이 값은
정규화됩니다. 사용 전에 후행 `.service` 접미사가 제거된 다음, 필요에 따라
다시 추가됩니다. 작업 디렉터리는 `WorkingDirectory=` 속성을 통해 유닛으로
전달됩니다.

데몬을 중지하려면:

```
deck server down [--unit deck-server]
```

`deck server down`은 `systemctl stop <unit>.service`를 호출합니다. 로그는
저널을 통해 확인할 수 있습니다:

```
journalctl -u deck-server.service
```

또는 `deck server logs --source journal --unit deck-server.service`를
통해서도 확인할 수 있습니다.

## macOS 및 Windows — 분리된 백그라운드 프로세스

macOS와 Windows에서 `deck server up --daemon`은 분리된 자식 프로세스를
생성합니다:

- **macOS:** 자식 프로세스가 새 세션을 시작합니다(`Setsid: true`).
- **Windows:** 자식 프로세스가 `CREATE_NEW_PROCESS_GROUP`과
  `DETACHED_PROCESS` 생성 플래그를 사용합니다.

자식 프로세스의 stdout과 stderr는 모두 로그 파일로 리디렉션됩니다.

### 상태 파일 위치

pid 파일과 로그 파일은 XDG 상태 루트(`$XDG_STATE_HOME`이 설정되어 있으면
`$XDG_STATE_HOME/deck`, 그렇지 않으면 `~/.local/state/deck`) 아래에
기록됩니다:

```
~/.local/state/deck/server/<unit>.pid
~/.local/state/deck/server/<unit>.log
```

여기서 `<unit>`은 정규화된 `--unit` 값입니다(기본값 `deck-server`).
시작 시 `deck server up`은 로그 파일 경로를 stdout에 출력합니다:

```
server up: ok (deck-server, pid 12345)
server log: /home/user/.local/state/deck/server/deck-server.log
```

### 데몬 중지

```
deck server down [--unit deck-server]
```

`deck server down`은 pid 파일을 읽고, `SIGTERM`을 보내거나(macOS)
`Process.Kill`을 호출한 다음(Windows), pid 파일을 제거합니다. pid 파일을
찾을 수 없으면 명령은 다음과 같이 실패합니다:

```
server down: pid file not found: <path>
```

프로세스가 이미 사라진 경우(오래된 pid), 중지는 여전히 성공하고 pid 파일은
정리됩니다.

## 서버 상태 확인

어느 플랫폼에서든 데몬을 시작한 후, `deck server health`를 사용하여 서버가
응답하는지 확인합니다:

```
deck server health --server http://127.0.0.1:8080
```

이는 `<server>/healthz`로 GET 요청을 보내고 HTTP 200일 때 `health: ok`를
보고합니다.

## 관련 항목

- [deck server up](../cli/deck_server_up.md)
- [Server Registry](registry.md)
